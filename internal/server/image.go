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
    "github.com/jaga-project/jaga-backend/internal/middleware"
)

const imageUploadPath = "./uploads/images" 
const maxUploadSize = 5 * 1024 * 1024 

var DefaultAllowedMimeTypes = map[string]bool{
    "image/jpeg": true,
    "image/png":  true,
    "image/jpg":  true,
    "image/gif":  true,
}

func ValidateMimeType(file multipart.File, handler *multipart.FileHeader, allowedMimeTypes map[string]bool) (string, error) {
    if file == nil || handler == nil {
        return "", fmt.Errorf("file or handler cannot be nil for MIME validation")
    }

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

    currentPos, errSeek := file.Seek(0, io.SeekCurrent)
    if errSeek != nil {
        return "", fmt.Errorf("failed to get current file position for '%s' before MIME detection: %w", handler.Filename, errSeek)
    }

    if _, errSeek := file.Seek(0, io.SeekStart); errSeek != nil {
        return "", fmt.Errorf("failed to reset file pointer for '%s' for MIME detection: %w", handler.Filename, errSeek)
    }

    buffer := make([]byte, 512) 
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
            return detectedMimeType, nil 
        }
    }

    if _, errSeekRestore := file.Seek(currentPos, io.SeekStart); errSeekRestore != nil {
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
            writeJSONError(w, "Internal server error: could not prepare upload directory", http.StatusInternalServerError)
            return
        }

        r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
        if err := r.ParseMultipartForm(maxUploadSize); err != nil {
            if err.Error() == "http: request body too large" {
                writeJSONError(w, fmt.Sprintf("File too large. Maximum upload size is %dMB", maxUploadSize/(1024*1024)), http.StatusRequestEntityTooLarge)
            } else {
                writeJSONError(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
            }
            return
        }

        file, handler, err := r.FormFile("imageFile")
        if err != nil {
            writeJSONError(w, "Invalid image file form field ('imageFile'): "+err.Error(), http.StatusBadRequest)
            return
        }
        defer file.Close()

        mimeType, errMime := ValidateMimeType(file, handler, DefaultAllowedMimeTypes)
        if errMime != nil {
            writeJSONError(w, fmt.Sprintf("MIME type validation failed: %v", errMime), http.StatusBadRequest)
            return
        }
        
        if _, errSeek := file.Seek(0, io.SeekStart); errSeek != nil {
            log.Printf("Error seeking to start of file after MIME validation: %v", errSeek)
            writeJSONError(w, "Internal server error: could not process file", http.StatusInternalServerError)
            return
        }

        originalFilename := handler.Filename
        fileExtension := filepath.Ext(originalFilename)
        uniqueFilename := fmt.Sprintf("%s%s", uuid.New().String(), fileExtension)
        storagePath := filepath.Join(imageUploadPath, uniqueFilename)

        dst, err := os.Create(storagePath)
        if err != nil {
            log.Printf("Error creating destination file %s: %v", storagePath, err)
            writeJSONError(w, "Internal server error: could not save file", http.StatusInternalServerError)
            return
        }
        defer dst.Close()

        fileSize, err := io.Copy(dst, file) // file sudah di-seek ke awal
        if err != nil {
            log.Printf("Error copying uploaded file to %s: %v", storagePath, err)
            os.Remove(storagePath) // Hapus file parsial jika copy gagal
            writeJSONError(w, "Internal server error: could not copy file content", http.StatusInternalServerError)
            return
        }

        dbImg := &database.Image{
            StoragePath:      filepath.ToSlash(storagePath), 
            FilenameOriginal: originalFilename,
            MimeType:         mimeType, 
            SizeBytes:        fileSize,
        }

        tx, err := s.db.Get().BeginTx(r.Context(), nil)
        if err != nil {
            log.Printf("Error starting transaction for image upload: %v", err)
            os.Remove(storagePath)
            writeJSONError(w, "Internal server error: could not process image upload", http.StatusInternalServerError)
            return
        }
        var txErr error 
        defer func() {
            if p := recover(); p != nil {
                tx.Rollback()
                if _, statErr := os.Stat(storagePath); statErr == nil {
                    os.Remove(storagePath)
                }
                panic(p)
            } else if txErr != nil {
                tx.Rollback()
                if _, statErr := os.Stat(storagePath); statErr == nil {
                    os.Remove(storagePath)
                }
            }
        }()

        txErr = database.CreateImageTx(r.Context(), tx, dbImg)
        if txErr != nil {
            log.Printf("Error creating image record in DB for %s: %v", storagePath, txErr)
            writeJSONError(w, "Internal server error: could not save image metadata", http.StatusInternalServerError)
            return
        }

        txErr = tx.Commit()
        if txErr != nil {
            log.Printf("Error committing transaction for image upload: %v", txErr)
            writeJSONError(w, "Internal server error: could not finalize image upload", http.StatusInternalServerError)
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
            writeJSONError(w, "Image ID is required", http.StatusBadRequest)
            return
        }

        imageID, err := strconv.ParseInt(idStr, 10, 64)
        if err != nil {
            writeJSONError(w, "Invalid image ID format", http.StatusBadRequest)
            return
        }

        imgData, err := database.GetImageByID(r.Context(), s.db.Get(), imageID)
        if err != nil {
            if err.Error() == "image not found" {
                writeJSONError(w, "Image not found", http.StatusNotFound)
            } else {
                log.Printf("Error getting image %d from DB: %v", imageID, err)
                writeJSONError(w, "Internal server error retrieving image metadata", http.StatusInternalServerError)
            }
            return
        }

        cleanStoragePath := filepath.Clean(imgData.StoragePath)

        if _, err := os.Stat(cleanStoragePath); os.IsNotExist(err) {
            log.Printf("Image file not found on disk: %s (DB ID: %d)", cleanStoragePath, imageID)
            writeJSONError(w, "Image file not found on disk", http.StatusNotFound)
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

// func getKeysFromMap(m map[string]bool) string {
//     keys := make([]string, 0, len(m))
//     for k := range m {
//         keys = append(keys, k)
//     }
//     return strings.Join(keys, ", ")
// }

func (s *Server) handleDeleteImage() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        idStr := vars["id"]
        imageID, err := strconv.ParseInt(idStr, 10, 64)
        if err != nil {
            writeJSONError(w, "Invalid image ID", http.StatusBadRequest)
            return
        }

        imgData, err := database.GetImageByID(r.Context(), s.db.Get(), imageID)
        if err != nil {
            if err.Error() == "image not found" {
                writeJSONError(w, "Image not found in database", http.StatusNotFound)
            } else {
                writeJSONError(w, "Failed to retrieve image metadata", http.StatusInternalServerError)
            }
            return
        }

        isAdmin, _ := r.Context().Value("isAdmin").(bool)

        // Otorisasi: Hanya admin yang boleh menghapus gambar.
        if !isAdmin {
            writeJSONError(w, "Forbidden: You are not authorized to perform this action", http.StatusForbidden)
            return
        }

       // Mulai transaksi
        tx, err := s.db.Get().BeginTx(r.Context(), nil)
        if err != nil {
            log.Printf("Error starting transaction for image deletion: %v", err)
            writeJSONError(w, "Internal server error: could not process image deletion", http.StatusInternalServerError)
            return
        }
        
        // Hapus record dari database TERLEBIH DAHULU di dalam transaksi
        if err := database.DeleteImageTx(r.Context(), tx, imageID); err != nil {
            tx.Rollback() // Batalkan transaksi jika penghapusan DB gagal
            log.Printf("Error deleting image record from DB for ID %d: %v", imageID, err)
            writeJSONError(w, "Failed to delete image record from database", http.StatusInternalServerError)
            return
        }

        // Commit transaksi jika penghapusan DB berhasil
        if err := tx.Commit(); err != nil {
            log.Printf("Error committing transaction for image deletion: %v", err)
            writeJSONError(w, "Internal server error: could not finalize image deletion", http.StatusInternalServerError)
            return
        }

        // HANYA SETELAH DATABASE BERHASIL DIUBAH, hapus file dari disk.
        if err := os.Remove(imgData.StoragePath); err != nil {
            // Pada titik ini, DB sudah konsisten. Kita hanya perlu mencatat bahwa file gagal dihapus.
            log.Printf("WARNING: DB record for image %d deleted, but failed to delete file on disk: %s. Error: %v", imageID, imgData.StoragePath, err)
        }

        w.WriteHeader(http.StatusNoContent)
    }
}

func (s *Server) RegisterImageRoutes(r *mux.Router) {
    if err := ensureUploadDir(imageUploadPath); err != nil {
        log.Printf("WARNING: Could not create initial image upload directory %s: %v. Will attempt per-request.", imageUploadPath, err)
        
    }
    adminOnlyMiddleware := middleware.AdminOnlyMiddleware()

    r.Handle("/images", adminOnlyMiddleware(s.handleImageUpload())).Methods("POST")
    r.Handle("/images/{id:[0-9]+}", s.handleGetImage()).Methods("GET")
    r.Handle("/images/{id:[0-9]+}", adminOnlyMiddleware(s.handleDeleteImage())).Methods("DELETE")
}