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
	"errors"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
	"github.com/jaga-project/jaga-backend/internal/middleware"
	"golang.org/x/crypto/bcrypt"
)

func generateUniqueFilenameLocal(originalFilename string) string {
	timestamp := time.Now().UnixNano()
	randomUUID := uuid.New().String()
	extension := filepath.Ext(originalFilename)
	base := strings.TrimSuffix(originalFilename, extension)
	safeBase := strings.ReplaceAll(strings.ToLower(base), " ", "_")
	if len(safeBase) > 50 { 
		safeBase = safeBase[:50]
	}
	return fmt.Sprintf("%s_%d_%s%s", safeBase, timestamp, randomUUID, extension)
}

func (s *Server) handleCreateUser() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        const maxKTPFileSize = 5 * 1024 * 1024
        const extraFormDataSizeUser = 1 * 1024 * 1024
        maxTotalUserFormSize := int64(extraFormDataSizeUser + maxKTPFileSize)

        if err := r.ParseMultipartForm(maxTotalUserFormSize); err != nil {
            writeJSONError(w, "Request too large or invalid multipart form: "+err.Error(), http.StatusBadRequest)
            return
        }

        newUser := database.User{
            UserID:    uuid.New().String(),
            CreatedAt: time.Now(),
            Name:      r.FormValue("name"),
            Email:     r.FormValue("email"),
            Phone:     r.FormValue("phone"),
            NIK:       r.FormValue("nik"),
        }
        password := r.FormValue("password")

        if newUser.Name == "" || newUser.Email == "" || password == "" || newUser.NIK == "" {
            writeJSONError(w, "Missing required user fields (name, email, password, nik)", http.StatusBadRequest)
            return
        }

        hashedPasswordBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
        if err != nil {
            writeJSONError(w, "Failed to hash password: "+err.Error(), http.StatusInternalServerError)
            return
        }
        newUser.Password = string(hashedPasswordBytes)

        tx, err := s.db.Get().BeginTx(r.Context(), nil)
        if err != nil {
            writeJSONError(w, "Failed to start database transaction: "+err.Error(), http.StatusInternalServerError)
            return
        }
        defer tx.Rollback()

        file, handler, err := r.FormFile("ktp_image")
        if err != nil && err != http.ErrMissingFile {
            writeJSONError(w, "Error retrieving KTP image: "+err.Error(), http.StatusBadRequest)
            return
        }

        if handler != nil { 
            defer file.Close()

            validatedMimeType, errMime := ValidateMimeType(file, handler, DefaultAllowedMimeTypes)
            if errMime != nil {
                writeJSONError(w, fmt.Sprintf("KTP image MIME type validation failed: %v", errMime), http.StatusBadRequest)
                return
            }

            if handler.Size > maxKTPFileSize {
                writeJSONError(w, fmt.Sprintf("KTP image file size exceeds %dMB limit.", maxKTPFileSize/(1024*1024)), http.StatusBadRequest)
                return
            }

            uniqueFilename := generateUniqueFilenameLocal(handler.Filename)
            ktpStoragePath := filepath.Join(imageUploadPath, uniqueFilename)

            if err := saveUploadedFile(file, ktpStoragePath); err != nil {
                writeJSONError(w, "Failed to save KTP image file: "+err.Error(), http.StatusInternalServerError)
                return
            }

            imgRecord := database.Image{
                StoragePath:      filepath.ToSlash(ktpStoragePath),
                FilenameOriginal: handler.Filename,
                MimeType:         validatedMimeType,
                SizeBytes:        handler.Size,
            }
            if err := database.CreateImageTx(r.Context(), tx, &imgRecord); err != nil {
                os.Remove(ktpStoragePath) 
                writeJSONError(w, "Failed to save KTP image metadata: "+err.Error(), http.StatusInternalServerError)
                return
            }
            newUser.KTPImageID = &imgRecord.ImageID
        }

        if err := database.CreateUserTx(r.Context(), tx, &newUser); err != nil {
            if newUser.KTPImageID != nil {
                path, _ := database.GetImageStoragePath(r.Context(), tx, *newUser.KTPImageID)
                if path != "" {
                    os.Remove(path)
                }
            }
            writeJSONError(w, "Failed to create user: "+err.Error(), http.StatusInternalServerError)
            return
        }

        if err := tx.Commit(); err != nil {
            if newUser.KTPImageID != nil {
                path, _ := database.GetImageStoragePath(r.Context(), s.db.Get(), *newUser.KTPImageID)
                if path != "" {
                    os.Remove(path)
                }
            }
            writeJSONError(w, "Failed to commit database transaction: "+err.Error(), http.StatusInternalServerError)
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
        writeJSONError(w, "Failed to retrieve users: "+err.Error(), http.StatusInternalServerError)
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
				writeJSONError(w, "User not found", http.StatusNotFound)

			} else {
				writeJSONError(w, "Failed to retrieve user: "+err.Error(), http.StatusInternalServerError)
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
			writeJSONError(w, "User ID is required", http.StatusBadRequest)
			return
		}

		user, err := database.FindUserByID(s.db.Get(), userID, r.Context())
		if err != nil {
			if err == sql.ErrNoRows || err.Error() == "user not found" {
				writeJSONError(w, "User not found", http.StatusNotFound)
      } else {
        writeJSONError(w, "Failed to retrieve user: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}
		user.Password = ""
		json.NewEncoder(w).Encode(user)
	}
}

func (s *Server) handleUpdateUser() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        targetUserID := mux.Vars(r)["id"]

        requestingUserID, ok := r.Context().Value(middleware.UserIDContextKey).(string)
        if !ok {
            writeJSONError(w, "Unauthorized: User ID not found in token", http.StatusUnauthorized)
            return
        }
        isRequestingUserAdmin, _ := r.Context().Value(middleware.AdminStatusContextKey).(bool)

        if !isRequestingUserAdmin && requestingUserID != targetUserID {
            writeJSONError(w, "Forbidden: You can only update your own profile", http.StatusForbidden)
            return
        }

        var updates map[string]interface{}
        if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
            writeJSONError(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
            return
        }

        if password, ok := updates["password"].(string); ok && password != "" {
            hashedPasswordBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
            if err != nil {
                writeJSONError(w, "Failed to hash new password: "+err.Error(), http.StatusInternalServerError)
                return
            }
            updates["password"] = string(hashedPasswordBytes)
        } else if ok && password == "" {
            delete(updates, "password")
        }

        if len(updates) == 0 {
            writeJSONError(w, "No update fields provided", http.StatusBadRequest)
            return
        }

        tx, err := s.db.Get().BeginTx(r.Context(), nil)
        if err != nil {
            writeJSONError(w, "Failed to start transaction", http.StatusInternalServerError)
            return
        }
        defer tx.Rollback()

        if err := database.UpdateUserTx(r.Context(), tx, targetUserID, updates); err != nil {
            if errors.Is(err, sql.ErrNoRows) {
                writeJSONError(w, "User not found or no effective changes made", http.StatusNotFound)
            } else {
                writeJSONError(w, "Failed to update user: "+err.Error(), http.StatusInternalServerError)
            }
            return
        }

        if err := tx.Commit(); err != nil {
            writeJSONError(w, "Failed to commit transaction", http.StatusInternalServerError)
            return
        }

        updatedUser, err := database.FindUserByID(s.db.Get(), targetUserID, r.Context())
        if err != nil {
            writeJSONError(w, "User updated, but failed to retrieve new data", http.StatusInternalServerError)
            return
        }
        updatedUser.Password = "" 

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(updatedUser)
    }
}

func (s *Server) handleDeleteUser() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := mux.Vars(r)["id"]

        tx, err := s.db.Get().BeginTx(r.Context(), nil)
        if err != nil {
            writeJSONError(w, "Failed to start transaction", http.StatusInternalServerError)
            return
        }
        defer tx.Rollback()

        _ = database.DeleteAdminTx(r.Context(), tx, userID) 

        if err := database.DeleteUserTx(r.Context(), tx, userID); err != nil {
            if errors.Is(err, sql.ErrNoRows) {
                writeJSONError(w, "User not found", http.StatusNotFound)
            } else {
                writeJSONError(w, "Failed to delete user: "+err.Error(), http.StatusInternalServerError)
            }
            return
        }

        if err := tx.Commit(); err != nil {
            writeJSONError(w, "Failed to commit transaction", http.StatusInternalServerError)
            return
        }

        w.WriteHeader(http.StatusNoContent)
    }
}

func saveUploadedFile(file io.ReadSeeker, path string) error {
    if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
        return err
    }

    dst, err := os.Create(path)
    if err != nil {
        return err
    }
    defer dst.Close()

    if _, err := file.Seek(0, io.SeekStart); err != nil {
        return err
    }

    _, err = io.Copy(dst, file)
    return err
}

func (s *Server) RegisterUserRoutes(r *mux.Router) {
	r.HandleFunc("/users", s.handleCreateUser()).Methods("POST")
}

func (s *Server) RegisterUserProtectedRoutes(r *mux.Router) {
	adminOnlyMiddleware := middleware.AdminOnlyMiddleware()
	r.Handle("/users", adminOnlyMiddleware(s.handleGetUser())).Methods("GET")   
	r.HandleFunc("/users/{id}", s.handleGetUserByID()).Methods("GET") 
	r.HandleFunc("/users/{id}", s.handleUpdateUser()).Methods("PUT")
	r.Handle("/users/{id}", adminOnlyMiddleware(s.handleDeleteUser())).Methods("DELETE")
}
