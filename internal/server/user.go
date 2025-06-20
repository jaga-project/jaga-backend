package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
	"github.com/jaga-project/jaga-backend/internal/middleware"
	"golang.org/x/crypto/bcrypt"
)

// Fungsi helper untuk membuat nama file unik (menggantikan utils.GenerateUniqueFilename)
func generateUniqueFilenameLocal(originalFilename string) string {
	timestamp := time.Now().UnixNano()
	randomUUID := uuid.New().String()
	extension := filepath.Ext(originalFilename)
	base := strings.TrimSuffix(originalFilename, extension)
	// Membersihkan base name agar lebih aman untuk nama file
	safeBase := strings.ReplaceAll(strings.ToLower(base), " ", "_")
	if len(safeBase) > 50 { // Batasi panjang base name
		safeBase = safeBase[:50]
	}
	return fmt.Sprintf("%s_%d_%s%s", safeBase, timestamp, randomUUID, extension)
}

func (s *Server) handleCreateUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Batasi ukuran keseluruhan request body (misalnya 10MB untuk multipart)
		// Sesuaikan batas ukuran total form jika perlu, mirip dengan lost_report.go
		// const maxKTPFileSize = 5 * 1024 * 1024 // 5MB untuk KTP
		// const extraFormDataSizeUser = 1 * 1024 * 1024 // 1MB untuk field teks lainnya
		// maxTotalUserFormSize := extraFormDataSizeUser + maxKTPFileSize
		// if err := r.ParseMultipartForm(maxTotalUserFormSize); err != nil {
		if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB, sesuaikan jika perlu
			http.Error(w, "Request too large or invalid multipart form: "+err.Error(), http.StatusBadRequest)
			return
		}

		var newUser database.User
		var ktpImageID sql.NullInt64

		// 1. Proses file KTP jika diunggah
		file, handler, err := r.FormFile("ktp_image") // "ktp_image" adalah nama field dari frontend
		var ktpStoragePath string
		var validatedMimeType string // Untuk menyimpan MIME type yang divalidasi

		if err != nil {
			if err != http.ErrMissingFile {
				http.Error(w, "Error retrieving KTP image: "+err.Error(), http.StatusBadRequest)
				return
			}
			// File KTP tidak ada, lanjutkan tanpa memproses gambar KTP
		} else {
			defer file.Close()

			// Validasi tipe MIME menggunakan fungsi helper terpusat
			// Asumsi ValidateMimeType dan DefaultAllowedMimeTypes ada di package server
			var errMime error
			validatedMimeType, errMime = ValidateMimeType(file, handler, DefaultAllowedMimeTypes)
			if errMime != nil {
				http.Error(w, fmt.Sprintf("KTP image MIME type validation failed: %v", errMime), http.StatusBadRequest)
				return
			}
			// file pointer sudah di-reset oleh ValidateMimeType jika validasi dari konten,
			// atau tidak berubah jika dari header. Kita akan reset lagi sebelum io.Copy untuk memastikan.

			const maxKTPFileSize = 5 * 1024 * 1024 // 5MB, sesuaikan jika perlu
			if handler.Size > maxKTPFileSize {
				http.Error(w, fmt.Sprintf("KTP image file size exceeds %dMB limit.", maxKTPFileSize/(1024*1024)), http.StatusBadRequest)
				return
			}

			uniqueFilename := generateUniqueFilenameLocal(handler.Filename) // Menggunakan fungsi lokal
			ktpStoragePath = filepath.Join(imageUploadPath, uniqueFilename) // imageUploadPath dari image.go

			if err := os.MkdirAll(imageUploadPath, os.ModePerm); err != nil {
				http.Error(w, "Failed to create upload directory: "+err.Error(), http.StatusInternalServerError)
				return
			}

			dst, err := os.Create(ktpStoragePath)
			if err != nil {
				http.Error(w, "Failed to save KTP image file: "+err.Error(), http.StatusInternalServerError)
				return
			}
			defer dst.Close()

			// Pastikan file pointer ada di awal sebelum io.Copy
			if _, errSeek := file.Seek(0, io.SeekStart); errSeek != nil {
				os.Remove(ktpStoragePath)
				http.Error(w, "Internal server error: could not process KTP image file after validation", http.StatusInternalServerError)
				return
			}

			if _, err := io.Copy(dst, file); err != nil {
				os.Remove(ktpStoragePath) // Hapus file parsial jika copy gagal
				http.Error(w, "Failed to copy KTP image file content: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		// 2. Ambil data user lainnya dari form
		newUser.UserID = uuid.New().String()
		newUser.CreatedAt = time.Now()
		newUser.Name = r.FormValue("name")
		newUser.Email = r.FormValue("email")
		newUser.Phone = r.FormValue("phone")
		password := r.FormValue("password")
		newUser.NIK = r.FormValue("nik")

		if newUser.Name == "" || newUser.Email == "" || password == "" || newUser.NIK == "" {
			if ktpStoragePath != "" {
				os.Remove(ktpStoragePath)
			}
			http.Error(w, "Missing required user fields (name, email, password, nik)", http.StatusBadRequest)
			return
		}

		// Hash password (langsung menggunakan bcrypt)
		hashedPasswordBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			if ktpStoragePath != "" {
				os.Remove(ktpStoragePath)
			}
			http.Error(w, "Failed to hash password: "+err.Error(), http.StatusInternalServerError)
			return
		}
		newUser.Password = string(hashedPasswordBytes)

		// 3. Mulai Transaksi Database
		tx, err := s.db.Get().BeginTx(r.Context(), nil)
		if err != nil {
			if ktpStoragePath != "" {
				os.Remove(ktpStoragePath)
			}
			http.Error(w, "Failed to start database transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}
		var txErr error // Variabel untuk menampung error dalam scope defer
		defer func() {
			if p := recover(); p != nil {
				tx.Rollback()
				if ktpStoragePath != "" { os.Remove(ktpStoragePath) }
				panic(p)
			} else if txErr != nil {
				tx.Rollback()
				if ktpStoragePath != "" { // Hapus file jika transaksi gagal
					os.Remove(ktpStoragePath)
				}
			}
		}()

		if ktpStoragePath != "" && handler != nil {
			// Gunakan validatedMimeType yang didapat dari ValidateMimeType
			imgRecord := database.Image{
				StoragePath:      filepath.ToSlash(ktpStoragePath),
				FilenameOriginal: handler.Filename,
				MimeType:         validatedMimeType, // Menggunakan MIME type yang sudah divalidasi
				SizeBytes:        handler.Size,      // handler.Size seharusnya aman di sini
			}
			txErr = database.CreateImageTx(r.Context(), tx, &imgRecord)
			if txErr != nil {
				// txErr akan ditangkap oleh defer func untuk rollback dan penghapusan file
				http.Error(w, "Failed to save KTP image metadata to database: "+txErr.Error(), http.StatusInternalServerError)
				return
			}
			ktpImageID = sql.NullInt64{Int64: imgRecord.ImageID, Valid: true}
		}

		if ktpImageID.Valid {
			newUser.KTPImageID = &ktpImageID.Int64
		} else {
			newUser.KTPImageID = nil
		}

		txErr = database.CreateSingleUserTx(r.Context(), tx, &newUser)
		if txErr != nil {
			http.Error(w, "Failed to create user: "+txErr.Error(), http.StatusInternalServerError)
			return
		}

		txErr = tx.Commit()
		if txErr != nil {
			http.Error(w, "Failed to commit database transaction: "+txErr.Error(), http.StatusInternalServerError)
			return
		}

		newUser.Password = ""
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(newUser)
	}
}

func (s *Server) handleGetUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		email := r.URL.Query().Get("email")
		if email == "" {
			users, err := database.FindManyUser(s.db.Get(), r.Context())
			if err != nil {
				http.Error(w, "Failed to retrieve users: "+err.Error(), http.StatusInternalServerError)
				return
			}
			for i := range users {
				users[i].Password = ""
			}
			json.NewEncoder(w).Encode(users)
			return
		}

		user, err := database.FindSingleUser(s.db.Get(), email, r.Context())
		if err != nil {
			if err == sql.ErrNoRows || err.Error() == "user not found" {
				http.Error(w, "User not found", http.StatusNotFound)
			} else {
				http.Error(w, "Failed to retrieve user: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}
		user.Password = ""
		json.NewEncoder(w).Encode(user)
	}
}

func (s *Server) handleGetUserByID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		userID := mux.Vars(r)["id"]
		if userID == "" {
			http.Error(w, "User ID is required", http.StatusBadRequest)
			return
		}

		user, err := database.FindUserByID(s.db.Get(), userID, r.Context())
		if err != nil {
			if err == sql.ErrNoRows || err.Error() == "user not found" {
				http.Error(w, "User not found", http.StatusNotFound)
			} else {
				http.Error(w, "Failed to retrieve user: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}
		user.Password = ""
		json.NewEncoder(w).Encode(user)
	}
}

func (s *Server) handleUpdateUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Ambil ID user target dari URL
        targetUserID := mux.Vars(r)["id"]

        // Ambil info user yang membuat permintaan dari token JWT (via context)
        requestingUserID, ok := r.Context().Value(middleware.UserIDContextKey).(string)
        if !ok {
            http.Error(w, "Unauthorized: User ID not found in token", http.StatusUnauthorized)
            return
        }
        isRequestingUserAdmin, _ := r.Context().Value(middleware.AdminStatusContextKey).(bool)

        // LOGIKA OTORISASI:
        // Izinkan jika pengguna adalah admin ATAU jika pengguna sedang memperbarui profilnya sendiri.
        if !isRequestingUserAdmin && requestingUserID != targetUserID {
            http.Error(w, "Forbidden: You can only update your own profile", http.StatusForbidden)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        var userUpdates database.User
        if err := json.NewDecoder(r.Body).Decode(&userUpdates); err != nil {
            http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
            return
        }

        if userUpdates.Password != "" {
            hashedPasswordBytes, err := bcrypt.GenerateFromPassword([]byte(userUpdates.Password), bcrypt.DefaultCost)
            if err != nil {
                http.Error(w, "Failed to hash new password: "+err.Error(), http.StatusInternalServerError)
                return
            }
            userUpdates.Password = string(hashedPasswordBytes)
        }

        // Gunakan targetUserID dari URL untuk update, bukan dari body
        if err := database.UpdateSingleUser(s.db.Get(), targetUserID, userUpdates, r.Context()); err != nil {
            if err.Error() == "user not found for update" || err == sql.ErrNoRows {
                http.Error(w, "User not found for update", http.StatusNotFound)
            } else {
                http.Error(w, "Failed to update user: "+err.Error(), http.StatusInternalServerError)
            }
            return
        }

        updatedUser, err := database.FindUserByID(s.db.Get(), targetUserID, r.Context())
        if err != nil {
            w.WriteHeader(http.StatusOK)
            json.NewEncoder(w).Encode(map[string]string{"message": "User updated successfully, but failed to retrieve updated record."})
            return
        }
        updatedUser.Password = ""

        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(updatedUser)
    }
}

func (s *Server) handleDeleteUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := mux.Vars(r)["id"]
		if err := database.DeleteSingleUser(s.db.Get(), userID, r.Context()); err != nil {
			if err.Error() == "user not found for delete" || err == sql.ErrNoRows {
				http.Error(w, "User not found for delete", http.StatusNotFound)
			} else {
				http.Error(w, "Failed to delete user: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// RegisterUserRoutes registers all user-related routes (publik, seperti registrasi)
func (s *Server) RegisterUserRoutes(r *mux.Router) {
	r.HandleFunc("/users", s.handleCreateUser()).Methods("POST")
}

// RegisterUserProtectedRoutes registers routes that require authentication
func (s *Server) RegisterUserProtectedRoutes(r *mux.Router) {
	adminOnlyMiddleware := middleware.AdminOnlyMiddleware()
	r.Handle("/users", adminOnlyMiddleware(s.handleGetUser())).Methods("GET")       // List all atau by email
	r.HandleFunc("/users/{id}", s.handleGetUserByID()).Methods("GET") // Get by UserID
	r.HandleFunc("/users/{id}", s.handleUpdateUser()).Methods("PUT")
	r.Handle("/users/{id}", adminOnlyMiddleware(s.handleDeleteUser())).Methods("DELETE")
}
