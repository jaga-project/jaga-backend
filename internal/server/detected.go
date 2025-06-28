package server

import (
	"context" 
	"database/sql"
	"encoding/json"
	"errors" 
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings" 
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
    "github.com/jaga-project/jaga-backend/internal/middleware"
)

const maxFileSizeDetected = 5 * 1024 * 1024 // 5 MB

type DetectedResponse struct {
    DetectedID        int       `json:"detected_id"`
    CameraID          int       `json:"camera_id"`
    Timestamp         time.Time `json:"timestamp"`
    PersonImageURL    *string   `json:"person_image_url,omitempty"`
    MotorcycleImageURL *string  `json:"motorcycle_image_url,omitempty"`
}

func (s *Server) toDetectedResponse(ctx context.Context, dbQuerier database.Querier, d *database.Detected) DetectedResponse {
    response := DetectedResponse{
        DetectedID: d.DetectedID,
        CameraID:   d.CameraID,
        Timestamp:  d.Timestamp,
    }

    if d.PersonImageID.Valid {
        path, err := database.GetImageStoragePath(ctx, dbQuerier, d.PersonImageID.Int64)
        if err == nil && path != "" {
            url := "/" + strings.TrimPrefix(path, "/")
            response.PersonImageURL = &url
        } else if err != nil && !errors.Is(err, sql.ErrNoRows) {
            fmt.Printf("WARN: Failed to get person image path for detected ID %d, image ID %d: %v\n", d.DetectedID, d.PersonImageID.Int64, err)
        }
    }

    if d.MotorcycleImageID.Valid {
        path, err := database.GetImageStoragePath(ctx, dbQuerier, d.MotorcycleImageID.Int64)
        if err == nil && path != "" {
            url := "/" + strings.TrimPrefix(path, "/")
            response.MotorcycleImageURL = &url
        } else if err != nil && !errors.Is(err, sql.ErrNoRows) {
            fmt.Printf("WARN: Failed to get motorcycle image path for detected ID %d, image ID %d: %v\n", d.DetectedID, d.MotorcycleImageID.Int64, err)
        }
    }
    return response
}

func processImageUpload(r *http.Request, formFieldName string, tx *sql.Tx) (sql.NullInt64, string, error) {
    file, handler, err := r.FormFile(formFieldName)
    if err != nil {
        if err == http.ErrMissingFile {
            fmt.Printf("DEBUG processImageUpload (detected): File not provided for field '%s'\n", formFieldName)
            return sql.NullInt64{Valid: false}, "", nil
        }
        fmt.Printf("ERROR processImageUpload (detected): Error retrieving file for field '%s': %v\n", formFieldName, err)
        return sql.NullInt64{Valid: false}, "", fmt.Errorf("error retrieving %s: %w", formFieldName, err)
    }
    defer file.Close()

    fmt.Printf("DEBUG processImageUpload (detected): Processing file '%s' for field '%s', size: %d, header MIME: %s\n", handler.Filename, formFieldName, handler.Size, handler.Header.Get("Content-Type"))

    if handler.Size == 0 {
        return sql.NullInt64{}, "", fmt.Errorf("file for %s is empty", formFieldName)
    }

    if handler.Size > maxFileSizeDetected {
        return sql.NullInt64{}, "", fmt.Errorf("%s file size (%d bytes) exceeds %dMB limit", formFieldName, handler.Size, maxFileSizeDetected/(1024*1024))
    }

    validatedMimeType, errMime := ValidateMimeType(file, handler, DefaultAllowedMimeTypes)
    if errMime != nil {
        return sql.NullInt64{}, "", fmt.Errorf("MIME type validation failed for %s: %w", formFieldName, errMime)
    }

    originalFilename := handler.Filename
    uniqueFilename := generateUniqueFilenameLocal(originalFilename)
    storageDir := imageUploadPath
    storagePath := filepath.Join(storageDir, uniqueFilename)

    if err := os.MkdirAll(storageDir, os.ModePerm); err != nil {
        return sql.NullInt64{Valid: false}, "", fmt.Errorf("failed to create upload directory '%s' for %s: %w", storageDir, formFieldName, err)
    }

    dst, err := os.Create(storagePath)
    if err != nil {
        return sql.NullInt64{Valid: false}, storagePath, fmt.Errorf("failed to create destination file '%s' for %s: %w", storagePath, formFieldName, err)
    }
    defer dst.Close()

    if _, errSeek := file.Seek(0, io.SeekStart); errSeek != nil {
        os.Remove(storagePath)
        return sql.NullInt64{}, storagePath, fmt.Errorf("failed to reset file pointer before copy for %s: %w", formFieldName, errSeek)
    }

    bytesCopied, err := io.Copy(dst, file)
    if err != nil {
        os.Remove(storagePath)
        return sql.NullInt64{Valid: false}, storagePath, fmt.Errorf("failed to copy %s file content to '%s': %w", formFieldName, storagePath, err)
    }
    fmt.Printf("DEBUG processImageUpload (detected): Successfully saved %s to %s (%d bytes copied)\n", formFieldName, storagePath, bytesCopied)

    imgRecord := database.Image{
        StoragePath:      filepath.ToSlash(storagePath),
        FilenameOriginal: originalFilename,
        MimeType:         validatedMimeType,
        SizeBytes:        bytesCopied,
    }

    if err := database.CreateImageTx(r.Context(), tx, &imgRecord); err != nil {
        os.Remove(storagePath)
        return sql.NullInt64{Valid: false}, storagePath, fmt.Errorf("failed to save %s image metadata: %w", formFieldName, err)
    }
    fmt.Printf("DEBUG processImageUpload (detected): Successfully created image record for '%s'. ImageID: %d\n", formFieldName, imgRecord.ImageID)
    return sql.NullInt64{Int64: imgRecord.ImageID, Valid: true}, storagePath, nil
}

func (s *Server) handleCreateDetected() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        fmt.Println("DEBUG: handleCreateDetected POST request received")
        if err := r.ParseMultipartForm(20 << 20); err != nil {
            writeJSONError(w, "Request too large or invalid multipart form: "+err.Error(), http.StatusBadRequest)
            return
        }

        var newDetected database.Detected
        var err error

        cameraIDStr := r.FormValue("camera_id")
        if cameraIDStr == "" {
            writeJSONError(w, "camera_id is required", http.StatusBadRequest)
            return
        }
        newDetected.CameraID, err = strconv.Atoi(cameraIDStr)
        if err != nil {
            writeJSONError(w, "Invalid camera_id: must be an integer", http.StatusBadRequest)
            return
        }

        timestampStr := r.FormValue("timestamp")
        if timestampStr == "" {
            writeJSONError(w, "timestamp is required", http.StatusBadRequest)
            return
        }

        parsedTime, err := time.Parse(time.RFC3339, timestampStr)
        if err != nil {
            writeJSONError(w, fmt.Sprintf("Invalid timestamp format. Use RFC3339. Error: %v", err), http.StatusBadRequest)
            return
        }
        newDetected.Timestamp = parsedTime

        tx, err := s.db.Get().BeginTx(r.Context(), nil)
        if err != nil {
            fmt.Printf("ERROR handleCreateDetected: Failed to start database transaction: %v\n", err)
            writeJSONError(w, "Failed to start database transaction: "+err.Error(), http.StatusInternalServerError)
            return
        }
        var txErr error
        var personImageStoragePath string
        var motorcycleImageStoragePath string

        var wg sync.WaitGroup
        errChan := make(chan error, 2) // Channel untuk menampung error dari goroutine

        var personImageID sql.NullInt64
        var motorcycleImageID sql.NullInt64

        // Mulai goroutine untuk person_image
        wg.Add(1)
        go func() {
            defer wg.Done()
            var err error
            personImageID, personImageStoragePath, err = processImageUpload(r, "person_image", tx)
            if err != nil {
                errChan <- fmt.Errorf("failed to process person_image: %w", err)
            }
        }()

        // Mulai goroutine untuk motorcycle_image
        wg.Add(1)
        go func() {
            defer wg.Done()
            var err error
            motorcycleImageID, motorcycleImageStoragePath, err = processImageUpload(r, "motorcycle_image", tx)
            if err != nil {
                errChan <- fmt.Errorf("failed to process motorcycle_image: %w", err)
            }
        }()

        wg.Wait()
        close(errChan) 

        // Periksa error
        for err := range errChan {
            if err != nil {
                txErr = err 
                statusCode := http.StatusInternalServerError
                errMsg := strings.ToLower(err.Error())
                if strings.Contains(errMsg, "file is empty") ||
                    strings.Contains(errMsg, "size exceeds") ||
                    strings.Contains(errMsg, "invalid type") ||
                    strings.Contains(errMsg, "mime type validation failed") {
                    statusCode = http.StatusBadRequest
                }
                writeJSONError(w, err.Error(), statusCode)
                return
            }
        }

        defer func() {
            if p := recover(); p != nil {
                tx.Rollback()
                if personImageStoragePath != "" { os.Remove(personImageStoragePath) }
                if motorcycleImageStoragePath != "" { os.Remove(motorcycleImageStoragePath) }
                panic(p)
            } else if txErr != nil {
                tx.Rollback()
                if personImageStoragePath != "" { os.Remove(personImageStoragePath) }
                if motorcycleImageStoragePath != "" { os.Remove(motorcycleImageStoragePath) }
            }
        }()

        // Tetapkan ID gambar ke struct setelah goroutine selesai
        newDetected.PersonImageID = personImageID
        newDetected.MotorcycleImageID = motorcycleImageID

        txErr = database.CreateDetectedTx(r.Context(), tx, &newDetected)
        if txErr != nil {
            writeJSONError(w, "Failed to create detected record: "+txErr.Error(), http.StatusInternalServerError)
            return
        }

        txErr = tx.Commit()
        if txErr != nil {
            writeJSONError(w, "Failed to commit database transaction: "+txErr.Error(), http.StatusInternalServerError)
            return
        }

        // Mengembalikan DetectedResponse
        response := s.toDetectedResponse(r.Context(), s.db.Get(), &newDetected) // Gunakan s.db.Get() karena tx sudah di-commit
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(response)
        fmt.Println("DEBUG: Response sent from handleCreateDetected")
    }
}

func (s *Server) handleGetDetected() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        vars := mux.Vars(r)
        idStr, idExists := vars["id"]
        db := s.db.Get()

        // Get by specific ID jika ada di path
        if idExists && idStr != "" {
            id, err := strconv.Atoi(idStr)
            if err != nil {
                writeJSONError(w, "Invalid detected_id: must be an integer", http.StatusBadRequest)
                return
            }
            d, err := database.GetDetectedByID(r.Context(), db, id)
            if err != nil {
                if errors.Is(err, sql.ErrNoRows) || err.Error() == "detected not found" {
                    writeJSONError(w, "Detected record not found", http.StatusNotFound)
                } else {
                    fmt.Printf("ERROR: Failed to get detected by ID %d: %v\n", id, err)
                    writeJSONError(w, "Failed to retrieve detected record", http.StatusInternalServerError)
                }
                return
            }
            response := s.toDetectedResponse(r.Context(), db, d)
            json.NewEncoder(w).Encode(response)
            return
        }

        queryParams := r.URL.Query()
        startTimeStr := queryParams.Get("start_time")
        endTimeStr := queryParams.Get("end_time")
        radiusStr := queryParams.Get("radius_km") 
        latStr := queryParams.Get("lat")
        lonStr := queryParams.Get("lon")

        // Filter gabungan (Waktu + Jarak dari Laporan Kehilangan)
        if startTimeStr != "" && endTimeStr != "" && latStr != "" && lonStr != "" && radiusStr != "" {
            // 1. Parse semua parameter yang diperlukan
            lat, err := strconv.ParseFloat(latStr, 64)
            if err != nil {
                writeJSONError(w, "invalid lat: must be a number", http.StatusBadRequest)
                return
            }
            lon, err := strconv.ParseFloat(lonStr, 64)
            if err != nil {
                writeJSONError(w, "invalid lon: must be a number", http.StatusBadRequest)
                return
            }
            radiusKm, err := strconv.ParseFloat(radiusStr, 64)
            if err != nil || radiusKm <= 0 {
                writeJSONError(w, "invalid radius_km: must be a positive number", http.StatusBadRequest)
                return
            }
            startTime, err := time.Parse(time.RFC3339, startTimeStr)
            if err != nil {
                writeJSONError(w, fmt.Sprintf("Invalid start_time format. Use RFC3339. Error: %v", err), http.StatusBadRequest)
                return
            }
            endTime, err := time.Parse(time.RFC3339, endTimeStr)
            if err != nil {
                writeJSONError(w, fmt.Sprintf("Invalid end_time format. Use RFC3339. Error: %v", err), http.StatusBadRequest)
                return
            }
            if endTime.Before(startTime) {
                writeJSONError(w, "end_time must be after start_time", http.StatusBadRequest)
                return
            }
            // Validasi tambahan untuk lat/lon
            if lat < -90 || lat > 90 {
                writeJSONError(w, "lat must be between -90 and 90", http.StatusBadRequest)
                return
            }
            if lon < -180 || lon > 180 {
                writeJSONError(w, "lon must be between -180 and 180", http.StatusBadRequest)
                return
            }

            // 2. Panggil fungsi database dengan parameter yang sudah diparsing
            detectedListDB, err := database.ListDetectedByProximityAndTimestamp(r.Context(), db, lat, lon, radiusKm, startTime, endTime)
            if err != nil {
                fmt.Printf("ERROR: Failed to list detected by proximity and time: %v\n", err)
                writeJSONError(w, "Failed to retrieve detected records with combined filter", http.StatusInternalServerError)
                return
            }

            // 3. Kirim respons
            responseList := make([]DetectedResponse, 0, len(detectedListDB))
            for i := range detectedListDB {
                responseList = append(responseList, s.toDetectedResponse(r.Context(), db, &detectedListDB[i]))
            }
            json.NewEncoder(w).Encode(responseList)
            return
        }

        // Filter berdasarkan rentang waktu saja
        if startTimeStr != "" && endTimeStr != "" {
            layout := time.RFC3339
            startTime, err := time.Parse(layout, startTimeStr)
            if err != nil {
                writeJSONError(w, fmt.Sprintf("Invalid start_time format. Use %s. Error: %v", layout, err), http.StatusBadRequest)
                return
            }
            endTime, err := time.Parse(layout, endTimeStr)
            if err != nil {
                writeJSONError(w, fmt.Sprintf("Invalid end_time format. Use %s. Error: %v", layout, err), http.StatusBadRequest)
                return
            }
            if endTime.Before(startTime) {
                writeJSONError(w, "end_time must be after start_time", http.StatusBadRequest)
                return
            }
            if layout == "2006-01-02" {
                endTime = endTime.Add(24*time.Hour - time.Nanosecond)
            }

            detectedListDB, err := database.ListDetectedByTimestampRange(r.Context(), db, startTime, endTime)
            if err != nil {
                fmt.Printf("ERROR: Failed to list detected by timestamp range (%s - %s): %v\n", startTimeStr, endTimeStr, err)
                writeJSONError(w, "Failed to retrieve detected records by timestamp", http.StatusInternalServerError)
                return
            }
            responseList := make([]DetectedResponse, 0, len(detectedListDB))
            for i := range detectedListDB {
                responseList = append(responseList, s.toDetectedResponse(r.Context(), db, &detectedListDB[i]))
            }
            json.NewEncoder(w).Encode(responseList)
            return
        }

        // Filter berdasarkan jarak (menggunakan lat/lon manual)
        if latStr != "" && lonStr != "" && radiusStr != "" {
            lat, err := strconv.ParseFloat(latStr, 64)
            if err != nil {
                writeJSONError(w, "invalid lat: must be a number", http.StatusBadRequest)
                return
            }
            lon, err := strconv.ParseFloat(lonStr, 64)
            if err != nil {
                writeJSONError(w, "invalid lon: must be a number", http.StatusBadRequest)
                return
            }
            radiusKm, err := strconv.ParseFloat(radiusStr, 64)
            if err != nil || radiusKm <= 0 {
                writeJSONError(w, "invalid radius_km: must be a positive number", http.StatusBadRequest)
                return
            }
            if lat < -90 || lat > 90 {
                writeJSONError(w, "lat must be between -90 and 90", http.StatusBadRequest)
                return
            }
            if lon < -180 || lon > 180 {
                writeJSONError(w, "lon must be between -180 and 180", http.StatusBadRequest)
                return
            }
            detectedListDB, err := database.ListDetectedByCoordinates(r.Context(), db, lat, lon, radiusKm)
            if err != nil {
                fmt.Printf("ERROR: Failed to list detected by proximity (lat: %f, lon: %f, radius: %fkm): %v\n", lat, lon, radiusKm, err)
                writeJSONError(w, "Failed to retrieve detected records by proximity", http.StatusInternalServerError)
                return
            }
            responseList := make([]DetectedResponse, 0, len(detectedListDB))
            for i := range detectedListDB {
                responseList = append(responseList, s.toDetectedResponse(r.Context(), db, &detectedListDB[i]))
            }
            json.NewEncoder(w).Encode(responseList)
            return
        }

        // Fallback: List semua data jika tidak ada filter yang cocok
        detectedListDB, err := database.ListDetected(r.Context(), db)
        if err != nil {
            fmt.Printf("ERROR: Failed to list all detected records: %v\n", err)
            writeJSONError(w, "Failed to retrieve detected records", http.StatusInternalServerError)
            return
        }
        responseList := make([]DetectedResponse, 0, len(detectedListDB))
        for i := range detectedListDB {
            responseList = append(responseList, s.toDetectedResponse(r.Context(), db, &detectedListDB[i]))
        }
        json.NewEncoder(w).Encode(responseList)
    }
}

func (s *Server) handleUpdateDetected() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        idStr := mux.Vars(r)["id"]
        id, err := strconv.Atoi(idStr)
        if err != nil {
            writeJSONError(w, "Invalid detected_id: must be an integer", http.StatusBadRequest)
            return
        }
        var dUpdates database.Detected
        if err := json.NewDecoder(r.Body).Decode(&dUpdates); err != nil {
            writeJSONError(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
            return
        }

        // Untuk update, jika ingin mengubah gambar, prosesnya akan lebih kompleks.
        // Handler ini akan mengupdate field non-file dan ID gambar jika disediakan.
        // Jika ID gambar di-set null atau 0 di request, maka akan di-set null di DB.
        // Jika ID gambar baru disediakan, pastikan gambar tersebut sudah ada di tabel images.
        // Atau, jika ingin upload gambar baru saat update, perlu logika ParseMultipartForm dan processImageUpload.
        // Untuk kesederhanaan, kita asumsikan ID gambar yang valid (jika ada) sudah ada di tabel images.

        // Ambil data lama untuk perbandingan atau untuk mengisi field yang tidak diupdate
        existingDetected, err := database.GetDetectedByID(r.Context(), s.db.Get(), id)
        if err != nil {
            if errors.Is(err, sql.ErrNoRows) {
                writeJSONError(w, "Detected record not found for update", http.StatusNotFound)
            } else {
                writeJSONError(w, "Failed to retrieve existing detected record: "+err.Error(), http.StatusInternalServerError)
            }
            return
        }

        // Update field yang diizinkan
        if dUpdates.CameraID != 0 { 
            existingDetected.CameraID = dUpdates.CameraID
        }
        if !dUpdates.Timestamp.IsZero() {
            existingDetected.Timestamp = dUpdates.Timestamp
        }
        // Update ID gambar jika disediakan di payload
        // Jika ingin menghapus gambar, klien harus mengirimkan ID gambar null atau field yang sesuai
        if dUpdates.PersonImageID.Valid {
            existingDetected.PersonImageID = dUpdates.PersonImageID
        } else if r.Body != http.NoBody { // Cek apakah field PersonImageID ada di JSON request (meskipun null)
            // Jika klien mengirim {"person_image_id": null}, maka dUpdates.PersonImageID.Valid akan false
            // dan kita ingin meng-setnya menjadi null.
            // Ini memerlukan cara untuk membedakan field yang tidak ada vs field yang ada tapi null.
            // Untuk saat ini, kita asumsikan jika Valid=false, itu berarti tidak diupdate atau di-set null.
            // Jika ingin lebih presisi, perlu parsing JSON yang lebih canggih atau field pointer di struct request.
            existingDetected.PersonImageID = dUpdates.PersonImageID // Ini akan meng-set null jika Valid=false
        }

        if dUpdates.MotorcycleImageID.Valid {
            existingDetected.MotorcycleImageID = dUpdates.MotorcycleImageID
        } else if r.Body != http.NoBody {
            existingDetected.MotorcycleImageID = dUpdates.MotorcycleImageID
        }


        if err := database.UpdateDetected(r.Context(), s.db.Get(), id, existingDetected); err != nil {
            if errors.Is(err, sql.ErrNoRows) || err.Error() == "no detected record updated or record not found" {
                writeJSONError(w, "Detected record not found or no changes made", http.StatusNotFound)
            } else {
                writeJSONError(w, "Failed to update detected record: "+err.Error(), http.StatusInternalServerError)
            }
            return
        }

        updatedDetected, err := database.GetDetectedByID(r.Context(), s.db.Get(), id)
        if err != nil {
            writeJSONError(w, "Update succeeded but failed to retrieve updated record: "+err.Error(), http.StatusInternalServerError)
            return
        }
        // Mengembalikan DetectedResponse
        response := s.toDetectedResponse(r.Context(), s.db.Get(), updatedDetected)
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(response)
    }
}

func (s *Server) handleDeleteDetected() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        idStr := mux.Vars(r)["id"]
        id, err := strconv.Atoi(idStr)
        if err != nil {
            writeJSONError(w, "Invalid detected_id: must be an integer", http.StatusBadRequest)
            return
        }

        // Ambil data detected untuk mendapatkan ID gambar-gambar terkait.
        detectedData, err := database.GetDetectedByID(r.Context(), s.db.Get(), id)
        if err != nil {
            if errors.Is(err, sql.ErrNoRows) {
                writeJSONError(w, "Detected record not found", http.StatusNotFound)
                return
            }
            writeJSONError(w, "Failed to retrieve detected record: "+err.Error(), http.StatusInternalServerError)
            return
        }

        // Mulai transaksi untuk memastikan semua operasi DB atomik.
        tx, err := s.db.Get().BeginTx(r.Context(), nil)
        if err != nil {
            writeJSONError(w, "Failed to start transaction: "+err.Error(), http.StatusInternalServerError)
            return
        }
        defer tx.Rollback() // Rollback otomatis jika terjadi error sebelum commit.

        // Hapus record 'detected' utama.
        if err := database.DeleteDetectedTx(r.Context(), tx, id); err != nil {
            writeJSONError(w, "Failed to delete detected record from DB: "+err.Error(), http.StatusInternalServerError)
            return
        }

        // Kumpulkan path file yang akan dihapus dari disk.
        var imagePathsToDelete []string

        // Hapus record person_image dan dapatkan path-nya.
        if detectedData.PersonImageID.Valid {
            path, err := database.GetImageStoragePathAndDeleteTx(r.Context(), tx, detectedData.PersonImageID.Int64)
            if err != nil {
                // Jika ada error di sini, transaksi akan di-rollback oleh defer.
                writeJSONError(w, "Failed to process person image deletion: "+err.Error(), http.StatusInternalServerError)
                return
            }
            if path != "" {
                imagePathsToDelete = append(imagePathsToDelete, path)
            }
        }

        // Hapus record motorcycle_image dan dapatkan path-nya.
        if detectedData.MotorcycleImageID.Valid {
            path, err := database.GetImageStoragePathAndDeleteTx(r.Context(), tx, detectedData.MotorcycleImageID.Int64)
            if err != nil {
                writeJSONError(w, "Failed to process motorcycle image deletion: "+err.Error(), http.StatusInternalServerError)
                return
            }
            if path != "" {
                imagePathsToDelete = append(imagePathsToDelete, path)
            }
        }

        // Jika semua operasi DB berhasil, commit transaksi.
        if err := tx.Commit(); err != nil {
            writeJSONError(w, "Failed to commit transaction: "+err.Error(), http.StatusInternalServerError)
            return
        }

        // HANYA SETELAH DB BERHASIL, hapus file dari disk di latar belakang.
        go func() {
            for _, path := range imagePathsToDelete {
                if err := os.Remove(path); err != nil {
                    log.Printf("WARN: DB records deleted, but failed to delete image file on disk: %s. Error: %v", path, err)
                }
            }
        }()

        w.WriteHeader(http.StatusNoContent)
    }
}

func (s *Server) RegisterDetectedRoutes(r *mux.Router) {
    adminOnlyMiddleware := middleware.AdminOnlyMiddleware()

    r.Handle("/detected", adminOnlyMiddleware(s.handleCreateDetected())).Methods("POST")
    r.Handle("/detected", adminOnlyMiddleware(s.handleGetDetected())).Methods("GET")
    r.Handle("/detected/{id:[0-9]+}", adminOnlyMiddleware(s.handleGetDetected())).Methods("GET")
    r.Handle("/detected/{id:[0-9]+}", adminOnlyMiddleware(s.handleUpdateDetected())).Methods("PUT")
    r.Handle("/detected/{id:[0-9]+}", adminOnlyMiddleware(s.handleDeleteDetected())).Methods("DELETE")
}
