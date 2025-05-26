package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	// "time" // Tidak digunakan secara langsung di sini, UploadedAt dari DB

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
	// "github.com/jaga-project/jaga-backend/internal/middleware" // Jika ingin mengambil UserID pengupload
)

// imageUploadPath adalah direktori tempat menyimpan gambar yang diunggah.
// TODO: Jadikan ini dapat dikonfigurasi, mis., melalui variabel environment atau konfigurasi server.
const imageUploadPath = "./uploads/images" // Pastikan direktori ini ada atau dapat dibuat.
const maxUploadSize = 10 * 1024 * 1024 // 10 MB

// ensureUploadDir memeriksa apakah direktori upload ada, dan membuatnya jika tidak.
func ensureUploadDir(dir string) error {
    err := os.MkdirAll(dir, os.ModePerm) // os.ModePerm (0777) mungkin terlalu permisif untuk produksi. Pertimbangkan 0755.
    if err != nil {
        return fmt.Errorf("failed to create upload directory %s: %w", dir, err)
    }
    return nil
}

func (s *Server) handleImageUpload() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if err := ensureUploadDir(imageUploadPath); err != nil {
            log.Printf("Error ensuring upload directory: %v", err)
            http.Error(w, "Internal server error: could not prepare upload directory", http.StatusInternalServerError)
            return
        }

        // Batasi ukuran request body sebelum parsing multipart form
        r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
        if err := r.ParseMultipartForm(maxUploadSize); err != nil {
            if err.Error() == "http: request body too large" {
                http.Error(w, fmt.Sprintf("File too large. Maximum upload size is %dMB", maxUploadSize/(1024*1024)), http.StatusRequestEntityTooLarge)
            } else {
                http.Error(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
            }
            return
        }

        file, handler, err := r.FormFile("imageFile") // "imageFile" adalah nama field form untuk file
        if err != nil {
            http.Error(w, "Invalid image file: "+err.Error(), http.StatusBadRequest)
            return
        }
        defer file.Close()

        originalFilename := handler.Filename
        fileExtension := filepath.Ext(originalFilename)
        // Buat nama file unik untuk menghindari penimpaan dan untuk keamanan
        uniqueFilename := fmt.Sprintf("%s%s", uuid.New().String(), fileExtension)
        // Simpan path relatif atau absolut sesuai kebutuhan. Relatif ke root aplikasi lebih portabel.
        storagePath := filepath.Join(imageUploadPath, uniqueFilename)

        dst, err := os.Create(storagePath)
        if err != nil {
            log.Printf("Error creating destination file %s: %v", storagePath, err)
            http.Error(w, "Internal server error: could not save file", http.StatusInternalServerError)
            return
        }
        defer dst.Close()

        fileSize, err := io.Copy(dst, file)
        if err != nil {
            log.Printf("Error copying uploaded file to %s: %v", storagePath, err)
            http.Error(w, "Internal server error: could not copy file content", http.StatusInternalServerError)
            return
        }

        // Deteksi MIME type dari file yang disimpan untuk akurasi
        // Perlu membuka kembali file setelah disimpan untuk deteksi yang benar
        savedFile, err := os.Open(storagePath)
        if err != nil {
            log.Printf("Error opening saved file for MIME detection %s: %v", storagePath, err)
            http.Error(w, "Internal server error: could not process file for type detection", http.StatusInternalServerError)
            return
        }
        defer savedFile.Close()

        buffer := make([]byte, 512) // Baca 512 byte pertama untuk menentukan tipe konten
        bytesRead, err := savedFile.Read(buffer)
        if err != nil && err != io.EOF {
            log.Printf("Error reading saved file for MIME detection %s: %v", storagePath, err)
            http.Error(w, "Internal server error: could not determine file type", http.StatusInternalServerError)
            return
        }
        mimeType := http.DetectContentType(buffer[:bytesRead])

        // Validasi dasar untuk tipe MIME yang diizinkan (opsional tapi direkomendasikan)
        allowedMimeTypes := map[string]bool{
            "image/jpeg": true,
            "image/png":  true,
            "image/gif":  true,
            // "image/webp": true, // Tambahkan jika didukung
        }
        if !allowedMimeTypes[mimeType] {
            os.Remove(storagePath) // Hapus file yang tidak valid
            http.Error(w, fmt.Sprintf("Unsupported file type: %s. Allowed types: %s", mimeType, getKeysFromMap(allowedMimeTypes)), http.StatusBadRequest)
            return
        }

        // Ambil UserID pengupload dari context jika diperlukan (jika Image.UploaderUserID diaktifkan)
        // uploaderUserID, ok := r.Context().Value(middleware.UserIDContextKey).(string)
        // if !ok && <uploaderUserID_is_mandatory> { // Sesuaikan kondisi jika wajib
        // 	os.Remove(storagePath)
        // 	http.Error(w, "Unauthorized: User ID not found for upload", http.StatusUnauthorized)
        // 	return
        // }

        dbImg := &database.Image{
            StoragePath:      storagePath, // Path tempat file disimpan
            FilenameOriginal: &originalFilename,
            MimeType:         &mimeType,
            SizeBytes:        &fileSize,
            // UploaderUserID:   &uploaderUserID, // Jika UploaderUserID diaktifkan dan valid
        }

        imageID, err := database.CreateImage(r.Context(), s.db.Get(), dbImg)
        if err != nil {
            log.Printf("Error creating image record in DB for %s: %v", storagePath, err)
            os.Remove(storagePath) // Hapus file jika insert DB gagal
            http.Error(w, "Internal server error: could not save image metadata", http.StatusInternalServerError)
            return
        }
        dbImg.ImageID = imageID
        // dbImg.UploadedAt akan diisi oleh database.CreateImage

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(dbImg)
    }
}

func (s *Server) handleGetImage() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        idStr, ok := vars["id"]
        if !ok {
            http.Error(w, "Image ID is required", http.StatusBadRequest)
            return
        }

        imageID, err := strconv.ParseInt(idStr, 10, 64)
        if err != nil {
            http.Error(w, "Invalid image ID format", http.StatusBadRequest)
            return
        }

        imgData, err := database.GetImageByID(r.Context(), s.db.Get(), imageID)
        if err != nil {
            if err.Error() == "image not found" {
                http.Error(w, "Image not found", http.StatusNotFound)
            } else {
                log.Printf("Error getting image %d from DB: %v", imageID, err)
                http.Error(w, "Internal server error retrieving image metadata", http.StatusInternalServerError)
            }
            return
        }

        // Pastikan path aman dan file ada sebelum disajikan
        // filepath.Clean untuk membersihkan path
        cleanStoragePath := filepath.Clean(imgData.StoragePath)

        // Periksa apakah file ada di disk
        if _, err := os.Stat(cleanStoragePath); os.IsNotExist(err) {
            log.Printf("Image file not found on disk: %s (DB ID: %d)", cleanStoragePath, imageID)
            http.Error(w, "Image file not found on disk", http.StatusNotFound)
            return
        }
        
        if imgData.MimeType != nil && *imgData.MimeType != "" {
            w.Header().Set("Content-Type", *imgData.MimeType)
        } else {
            // Fallback jika tipe MIME tidak disimpan atau kosong
            w.Header().Set("Content-Type", "application/octet-stream")
        }

        http.ServeFile(w, r, cleanStoragePath)
    }
}

// RegisterImageRoutes mendaftarkan rute terkait gambar.
// Rute ini sebaiknya dilindungi oleh middleware JWT jika upload/akses gambar tidak bersifat publik.
func (s *Server) RegisterImageRoutes(r *mux.Router) {
    // Pastikan direktori upload dasar ada saat startup (opsional, bisa juga per-request)
    if err := ensureUploadDir(imageUploadPath); err != nil {
        log.Printf("WARNING: Could not create initial image upload directory %s: %v. Will attempt per-request.", imageUploadPath, err)
        // Tidak fatal di sini, biarkan handleImageUpload mencoba membuatnya.
    }

    r.HandleFunc("/images", s.handleImageUpload()).Methods("POST")
    r.HandleFunc("/images/{id:[0-9]+}", s.handleGetImage()).Methods("GET")
    // Anda bisa menambahkan rute DELETE /images/{id} di sini jika diperlukan
    // yang akan memanggil handleDeleteImage (perlu dibuat)
}

// Helper untuk mendapatkan kunci dari map (untuk pesan error)
func getKeysFromMap(m map[string]bool) string {
    keys := make([]string, 0, len(m))
    for k := range m {
        keys = append(keys, k)
    }
    return strings.Join(keys, ", ")
}

// handleDeleteImage (Contoh jika Anda ingin menambahkan fungsionalitas delete)
// func (s *Server) handleDeleteImage() http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		// Ambil UserID dan isAdmin dari context jika otorisasi diperlukan
// 		// requestingUserID, _ := r.Context().Value(middleware.UserIDContextKey).(string)
// 		// isAdmin, _ := r.Context().Value(middleware.AdminStatusContextKey).(bool)

// 		vars := mux.Vars(r)
// 		idStr := vars["id"]
// 		imageID, err := strconv.ParseInt(idStr, 10, 64)
// 		if err != nil {
// 			http.Error(w, "Invalid image ID", http.StatusBadRequest)
// 			return
// 		}

// 		imgData, err := database.GetImageByID(r.Context(), s.db.Get(), imageID)
// 		if err != nil {
// 			if err.Error() == "image not found" {
// 				http.Error(w, "Image not found in database", http.StatusNotFound)
// 			} else {
// 				http.Error(w, "Failed to retrieve image metadata", http.StatusInternalServerError)
// 			}
// 			return
// 		}

// 		// Otorisasi: Siapa yang boleh menghapus gambar? Pemilik? Admin?
// 		// if !isAdmin && (imgData.UploaderUserID == nil || *imgData.UploaderUserID != requestingUserID) {
// 		// 	http.Error(w, "Forbidden: You are not authorized to delete this image", http.StatusForbidden)
// 		// 	return
// 		// }

// 		// Hapus file dari disk
// 		if err := os.Remove(imgData.StoragePath); err != nil {
// 			log.Printf("Error deleting image file %s from disk: %v", imgData.StoragePath, err)
// 			// Lanjutkan untuk menghapus dari DB, tapi log error ini.
// 			// Atau, putuskan apakah akan mengembalikan error ke client.
// 		}

// 		// Hapus record dari database
// 		if err := database.DeleteImage(r.Context(), s.db.Get(), imageID); err != nil {
// 			http.Error(w, "Failed to delete image record from database: "+err.Error(), http.StatusInternalServerError)
// 			return
// 		}

// 		w.WriteHeader(http.StatusNoContent)
// 	}
// }