package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
    

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
	// "github.com/jaga-project/jaga-backend/internal/middleware" // Jika ingin mengambil UserID pengupload
)


const imageUploadPath = "./uploads/images" 
const maxUploadSize = 10 * 1024 * 1024 // 10 MB

var DefaultAllowedMimeTypes = map[string]bool{
    "image/jpeg": true,
    "image/png":  true,
    "image/jpg":  true,
    "image/gif":  true,
}

// ValidateMimeType memeriksa apakah tipe MIME file valid berdasarkan header dan konten.
// Mengembalikan tipe MIME yang terdeteksi dan tervalidasi, atau error jika tidak valid.
// file pointer akan di-reset ke awal setelah pembacaan untuk deteksi.
func ValidateMimeType(file multipart.File, handler *multipart.FileHeader, allowedMimeTypes map[string]bool) (string, error) {
    if file == nil || handler == nil {
        return "", fmt.Errorf("file or handler cannot be nil for MIME validation")
    }

    // 1. Coba validasi berdasarkan header Content-Type dari handler
    headerMimeType := handler.Header.Get("Content-Type")
    fmt.Printf("DEBUG ValidateMimeType: Header MIME for '%s': %s\n", handler.Filename, headerMimeType)

    for allowed := range allowedMimeTypes {
        if strings.HasPrefix(headerMimeType, allowed) {
            fmt.Printf("DEBUG ValidateMimeType: '%s' validated by header as %s (prefix match with %s)\n", handler.Filename, headerMimeType, allowed)
            // Tidak perlu reset pointer jika validasi dari header sudah cukup
            // Namun, untuk konsistensi dan jika deteksi konten selalu diinginkan, reset bisa dilakukan di sini juga.
            // Untuk saat ini, kita anggap validasi header sudah cukup jika berhasil.
            return headerMimeType, nil
        }
    }

    // 2. Jika header tidak cocok atau untuk keamanan tambahan, deteksi dari konten file
    // Simpan posisi pointer saat ini
    currentPos, errSeek := file.Seek(0, io.SeekCurrent)
    if errSeek != nil {
        return "", fmt.Errorf("failed to get current file position for '%s' before MIME detection: %w", handler.Filename, errSeek)
    }

    // Reset ke awal untuk membaca buffer deteksi
    if _, errSeek := file.Seek(0, io.SeekStart); errSeek != nil {
        return "", fmt.Errorf("failed to reset file pointer for '%s' for MIME detection: %w", handler.Filename, errSeek)
    }

    buffer := make([]byte, 512) // Standar untuk http.DetectContentType
    n, err := file.Read(buffer)
    if err != nil && err != io.EOF {
        // Kembalikan pointer ke posisi semula jika terjadi error baca
        file.Seek(currentPos, io.SeekStart)
        return "", fmt.Errorf("failed to read file buffer for MIME detection ('%s'): %w", handler.Filename, err)
    }

    // Penting: Kembalikan file pointer ke posisi semula setelah membaca buffer,
    // agar operasi file selanjutnya (seperti io.Copy) tidak terpengaruh jika validasi gagal di sini,
    // atau jika pemanggil mengharapkan pointer tetap.
    // Jika validasi berhasil dan tipe MIME terdeteksi dari konten, pemanggil mungkin ingin pointer di awal.
    // Untuk fungsi helper ini, lebih aman mengembalikan pointer ke posisi sebelum dipanggil,
    // atau secara eksplisit menyatakan bahwa pointer akan berada di awal setelah fungsi ini.
    // Di sini, kita akan mengembalikannya ke awal jika validasi berhasil, karena file akan segera di-copy.
    // Jika tidak, kembalikan ke posisi semula.

    detectedMimeType := http.DetectContentType(buffer[:n])
    fmt.Printf("DEBUG ValidateMimeType: Content-detected MIME for '%s': %s\n", handler.Filename, detectedMimeType)

    for allowed := range allowedMimeTypes {
        if strings.HasPrefix(detectedMimeType, allowed) {
            fmt.Printf("DEBUG ValidateMimeType: '%s' validated by content as %s (prefix match with %s)\n", handler.Filename, detectedMimeType, allowed)
            // Reset pointer ke awal karena file akan segera diproses (misalnya, io.Copy)
            if _, errSeekReset := file.Seek(0, io.SeekStart); errSeekReset != nil {
                return "", fmt.Errorf("failed to reset file pointer for '%s' after successful content MIME detection: %w", handler.Filename, errSeekReset)
            }
            return detectedMimeType, nil // Valid berdasarkan konten
        }
    }

    // Jika tidak ada yang cocok, kembalikan pointer ke posisi semula
    if _, errSeekRestore := file.Seek(currentPos, io.SeekStart); errSeekRestore != nil {
        // Log error ini, tapi error validasi MIME utama lebih penting
        fmt.Printf("WARNING ValidateMimeType: Failed to restore file pointer for '%s' after failed MIME detection: %v\n", handler.Filename, errSeekRestore)
    }

    allowedKeys := make([]string, 0, len(allowedMimeTypes))
    for k := range allowedMimeTypes {
        allowedKeys = append(allowedKeys, k)
    }
    return "", fmt.Errorf("invalid file type for '%s'. Header: '%s', Detected: '%s'. Allowed: %s", handler.Filename, headerMimeType, detectedMimeType, strings.Join(allowedKeys, ", "))
}

func ensureUploadDir(dir string) error {
    err := os.MkdirAll(dir, os.ModePerm) 
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

        r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
        if err := r.ParseMultipartForm(maxUploadSize); err != nil {
            if err.Error() == "http: request body too large" {
                http.Error(w, fmt.Sprintf("File too large. Maximum upload size is %dMB", maxUploadSize/(1024*1024)), http.StatusRequestEntityTooLarge)
            } else {
                http.Error(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
            }
            return
        }

        file, handler, err := r.FormFile("imageFile") // "imageFile" adalah nama field yang diharapkan
        if err != nil {
            http.Error(w, "Invalid image file form field ('imageFile'): "+err.Error(), http.StatusBadRequest)
            return
        }
        defer file.Close()

        // Validasi tipe MIME menggunakan fungsi helper
        // file pointer akan di-reset oleh ValidateMimeType jika validasi berhasil melalui deteksi konten
        mimeType, errMime := ValidateMimeType(file, handler, DefaultAllowedMimeTypes)
        if errMime != nil {
            http.Error(w, fmt.Sprintf("MIME type validation failed: %v", errMime), http.StatusBadRequest)
            return
        }
        // file pointer sekarang berada di awal jika ValidateMimeType berhasil melalui deteksi konten,
        // atau tidak berubah jika berhasil melalui header. Untuk io.Copy, kita perlu memastikan itu di awal.
        if _, errSeek := file.Seek(0, io.SeekStart); errSeek != nil {
            log.Printf("Error seeking to start of file after MIME validation: %v", errSeek)
            http.Error(w, "Internal server error: could not process file", http.StatusInternalServerError)
            return
        }

        originalFilename := handler.Filename
        fileExtension := filepath.Ext(originalFilename)
        uniqueFilename := fmt.Sprintf("%s%s", uuid.New().String(), fileExtension)
        storagePath := filepath.Join(imageUploadPath, uniqueFilename)

        dst, err := os.Create(storagePath)
        if err != nil {
            log.Printf("Error creating destination file %s: %v", storagePath, err)
            http.Error(w, "Internal server error: could not save file", http.StatusInternalServerError)
            return
        }
        defer dst.Close()

        fileSize, err := io.Copy(dst, file) // file sudah di-seek ke awal
        if err != nil {
            log.Printf("Error copying uploaded file to %s: %v", storagePath, err)
            os.Remove(storagePath) // Hapus file parsial jika copy gagal
            http.Error(w, "Internal server error: could not copy file content", http.StatusInternalServerError)
            return
        }

        // Tidak perlu membuka ulang file untuk deteksi MIME karena sudah dilakukan oleh ValidateMimeType

        // Ambil UserID pengupload dari context jika diperlukan (jika Image.UploaderUserID diaktifkan)
        // uploaderUserID, ok := r.Context().Value(middleware.UserIDContextKey).(string)
        // if !ok && <uploaderUserID_is_mandatory> { // Sesuaikan kondisi jika wajib
        // 	os.Remove(storagePath)
        // 	http.Error(w, "Unauthorized: User ID not found for upload", http.StatusUnauthorized)
        // 	return
        // }

        dbImg := &database.Image{
            StoragePath:      filepath.ToSlash(storagePath), // Simpan dengan forward slashes
            FilenameOriginal: originalFilename,
            MimeType:         mimeType, // Gunakan mimeType yang divalidasi
            SizeBytes:        fileSize,
            // UploaderUserID:   &uploaderUserID, // Jika UploaderUserID diaktifkan dan valid
        }

        // Mulai transaksi
        tx, err := s.db.Get().BeginTx(r.Context(), nil)
        if err != nil {
            log.Printf("Error starting transaction for image upload: %v", err)
            os.Remove(storagePath)
            http.Error(w, "Internal server error: could not process image upload", http.StatusInternalServerError)
            return
        }
        var txErr error // Variabel untuk error di dalam scope transaksi
        defer func() {
            if p := recover(); p != nil {
                tx.Rollback()
                // Hapus file jika panic terjadi setelah file disimpan tapi sebelum commit
                // (meskipun os.Remove sudah ada di beberapa jalur error, ini untuk catch-all)
                if _, statErr := os.Stat(storagePath); statErr == nil {
                    os.Remove(storagePath)
                }
                panic(p)
            } else if txErr != nil {
                tx.Rollback()
                // Hapus file jika error terjadi setelah file disimpan tapi sebelum commit
                if _, statErr := os.Stat(storagePath); statErr == nil {
                    os.Remove(storagePath)
                }
            }
        }()

        txErr = database.CreateImageTx(r.Context(), tx, dbImg)
        if txErr != nil {
            log.Printf("Error creating image record in DB for %s: %v", storagePath, txErr)
            // txErr sudah di-set, defer akan rollback dan menghapus file
            http.Error(w, "Internal server error: could not save image metadata", http.StatusInternalServerError)
            return
        }

        txErr = tx.Commit()
        if txErr != nil {
            log.Printf("Error committing transaction for image upload: %v", txErr)
            // txErr sudah di-set, defer akan rollback dan menghapus file
            http.Error(w, "Internal server error: could not finalize image upload", http.StatusInternalServerError)
            return
        }

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

        cleanStoragePath := filepath.Clean(imgData.StoragePath)

        // Periksa apakah file ada di disk
        if _, err := os.Stat(cleanStoragePath); os.IsNotExist(err) {
            log.Printf("Image file not found on disk: %s (DB ID: %d)", cleanStoragePath, imageID)
            http.Error(w, "Image file not found on disk", http.StatusNotFound)
            return
        }
        
        if imgData.MimeType != "" {
            w.Header().Set("Content-Type", imgData.MimeType)
        } else {
            w.Header().Set("Content-Type", "application/octet-stream")
        }

        http.ServeFile(w, r, cleanStoragePath)
    }
}

func (s *Server) RegisterImageRoutes(r *mux.Router) {
    if err := ensureUploadDir(imageUploadPath); err != nil {
        log.Printf("WARNING: Could not create initial image upload directory %s: %v. Will attempt per-request.", imageUploadPath, err)
        
    }

    r.HandleFunc("/images", s.handleImageUpload()).Methods("POST")
    r.HandleFunc("/images/{id:[0-9]+}", s.handleGetImage()).Methods("GET")
}

func getKeysFromMap(m map[string]bool) string {
    keys := make([]string, 0, len(m))
    for k := range m {
        keys = append(keys, k)
    }
    return strings.Join(keys, ", ")
}

func (s *Server) handleDeleteImage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// requestingUserID, _ := r.Context().Value(middleware.UserIDContextKey).(string)
		// isAdmin, _ := r.Context().Value(middleware.AdminStatusContextKey).(bool)

		vars := mux.Vars(r)
		idStr := vars["id"]
		imageID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid image ID", http.StatusBadRequest)
			return
		}

		imgData, err := database.GetImageByID(r.Context(), s.db.Get(), imageID)
		if err != nil {
			if err.Error() == "image not found" {
				http.Error(w, "Image not found in database", http.StatusNotFound)
			} else {
				http.Error(w, "Failed to retrieve image metadata", http.StatusInternalServerError)
			}
			return
		}

		// // Otorisasi
		// if !isAdmin && (imgData.UploaderUserID == nil || *imgData.UploaderUserID != requestingUserID) {
		// 	http.Error(w, "Forbidden: You are not authorized to delete this image", http.StatusForbidden)
		// 	return
        // }

        // Hapus file dari disk
        // Mulai transaksi
        tx, err := s.db.Get().BeginTx(r.Context(), nil)
        if err != nil {
            log.Printf("Error starting transaction for image deletion: %v", err)
            http.Error(w, "Internal server error: could not process image deletion", http.StatusInternalServerError)
            return
        }
        // Defer rollback jika terjadi error atau panic
        defer func() {
            if p := recover(); p != nil {
                tx.Rollback()
                panic(p) // re-throw panic setelah Rollback
            } else if err != nil {
                tx.Rollback() // Rollback jika err tidak nil
            }
        }()

        // Hapus file dari disk terlebih dahulu
        // Jika ini gagal, kita mungkin ingin rollback transaksi DB atau melanjutkan tergantung kebutuhan
        // Untuk saat ini, kita log errornya tapi tetap mencoba menghapus dari DB
        if err := os.Remove(imgData.StoragePath); err != nil {
            log.Printf("Error deleting image file %s from disk: %v. Proceeding to delete DB record.", imgData.StoragePath, err)
            // Jika penghapusan file adalah kritikal dan harus menghentikan proses, uncomment baris di bawah dan handle error
            // err = fmt.Errorf("failed to delete image file: %w", err)
            // http.Error(w, "Failed to delete image file from disk", http.StatusInternalServerError)
            // return // Ini akan memicu rollback dari defer func
        }

        // Hapus record dari database menggunakan transaksi
        err = database.DeleteImageTx(r.Context(), tx, imageID)
        if err != nil {
            // err sudah di-set, defer func akan melakukan rollback
            log.Printf("Error deleting image record from DB for ID %d: %v", imageID, err)
            http.Error(w, "Failed to delete image record from database: "+err.Error(), http.StatusInternalServerError)
            return
        }

        // Commit transaksi jika berhasil
        err = tx.Commit()
        if err != nil {
            // err sudah di-set, defer func akan melakukan rollback
            log.Printf("Error committing transaction for image deletion: %v", err)
            http.Error(w, "Internal server error: could not finalize image deletion", http.StatusInternalServerError)
            return
        }

        w.WriteHeader(http.StatusNoContent)
    }
}

// Helper untuk menentukan status code error upload gambar (jika belum ada secara global di server)
// Anda mungkin sudah memiliki ini di image.go atau file helper lain.
func determineImageUploadErrorStatusCode(err error) int {
    if err == nil {
        return http.StatusOK
    }
    errMsg := strings.ToLower(err.Error())
    if strings.Contains(errMsg, "file is empty") ||
        strings.Contains(errMsg, "size exceeds") ||
        strings.Contains(errMsg, "invalid type") ||
        strings.Contains(errMsg, "mime type validation failed") {
        return http.StatusBadRequest
    }
    return http.StatusInternalServerError
}