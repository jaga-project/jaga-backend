package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
	"github.com/jaga-project/jaga-backend/internal/middleware"
)

// LostReportResponse adalah struct untuk respons JSON dengan URL gambar.
type LostReportResponse struct {
    LostID                int       `json:"lost_id"`
    UserID                string    `json:"user_id"`
    Timestamp             time.Time `json:"timestamp"`
    VehicleID             int       `json:"vehicle_id"`
    Address               string    `json:"address"`
    Status                string    `json:"status"`
    // DetectedID            *int      `json:"detected_id,omitempty"` // Dihapus
    MotorEvidenceImageURL *string   `json:"motor_evidence_image_url,omitempty"`
    PersonEvidenceImageURL *string  `json:"person_evidence_image_url,omitempty"`
}

// Helper untuk mengubah database.LostReport menjadi LostReportResponse
func (s *Server) toLostReportResponse(ctx context.Context, dbQuerier database.Querier, lr *database.LostReport) LostReportResponse {
    response := LostReportResponse{
        LostID:     lr.LostID,
        UserID:     lr.UserID,
        Timestamp:  lr.Timestamp,
        VehicleID:  lr.VehicleID,
        Address:    lr.Address,
        Status:     lr.Status,
        // DetectedID: lr.DetectedID, // Dihapus
    }

    if lr.MotorEvidenceImageID != nil && *lr.MotorEvidenceImageID > 0 {
        path, err := database.GetImageStoragePath(ctx, dbQuerier, *lr.MotorEvidenceImageID)
        if err == nil && path != "" {
            url := "/" + strings.TrimPrefix(path, "/")
            response.MotorEvidenceImageURL = &url
        } else if err != nil && !errors.Is(err, sql.ErrNoRows) {
            fmt.Printf("WARN: Failed to get motor evidence image path for ID %d: %v\n", *lr.MotorEvidenceImageID, err)
        }
    }

    if lr.PersonEvidenceImageID != nil && *lr.PersonEvidenceImageID > 0 {
        path, err := database.GetImageStoragePath(ctx, dbQuerier, *lr.PersonEvidenceImageID)
        if err == nil && path != "" {
            url := "/" + strings.TrimPrefix(path, "/")
            response.PersonEvidenceImageURL = &url
        } else if err != nil && !errors.Is(err, sql.ErrNoRows) {
            fmt.Printf("WARN: Failed to get person evidence image path for ID %d: %v\n", *lr.PersonEvidenceImageID, err)
        }
    }
    return response
}

func (s *Server) handleCreateLostReport() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        fmt.Println("DEBUG: handleCreateLostReport - Start")

        const maxEvidenceFileSize = 5 * 1024 * 1024 
        const extraFormDataSize = 1 * 1024 * 1024   
        maxTotalSize := extraFormDataSize + 2*maxEvidenceFileSize

        fmt.Printf("DEBUG: handleCreateLostReport - Parsing multipart form with max size: %d bytes\n", maxTotalSize)
        if err := r.ParseMultipartForm(int64(maxTotalSize)); err != nil {
            fmt.Printf("ERROR: handleCreateLostReport - Failed to parse multipart form: %v\n", err)
            http.Error(w, "Request too large or invalid multipart form: "+err.Error(), http.StatusBadRequest)
            return
        }

        var lr database.LostReport
        var err error 

        requestingUserID, ok := r.Context().Value(middleware.UserIDContextKey).(string)
        if !ok || requestingUserID == "" {
            http.Error(w, "Unauthorized: User ID not found in token", http.StatusUnauthorized)
            return
        }
        lr.UserID = requestingUserID

        timestampStr := r.FormValue("timestamp")
        if timestampStr == "" {
            lr.Timestamp = time.Now()
        } else {
            lr.Timestamp, err = time.Parse(time.RFC3339, timestampStr)
            if err != nil {
                http.Error(w, fmt.Sprintf("Invalid timestamp format. Use RFC3339. Error: %v", err), http.StatusBadRequest)
                return
            }
            if lr.Timestamp.After(time.Now().Add(5 * time.Minute)) {
                http.Error(w, "timestamp (waktu kejadian) cannot be unreasonably in the future", http.StatusBadRequest)
                return
            }
        }

        vehicleIDStr := r.FormValue("vehicle_id")
        if vehicleIDStr == "" {
            http.Error(w, "vehicle_id is required", http.StatusBadRequest)
            return
        }
        lr.VehicleID, err = strconv.Atoi(vehicleIDStr)
        if err != nil {
            http.Error(w, "Invalid vehicle_id: must be an integer", http.StatusBadRequest)
            return
        }

        lr.Address = r.FormValue("address")
        if lr.Address == "" {
            http.Error(w, "address is required", http.StatusBadRequest)
            return
        }

        statusStr := r.FormValue("status")
        if statusStr != "" {
            lr.Status = statusStr
        } else {
            lr.Status = database.StatusLostReportBelumDiproses
        }

        // detectedIDStr := r.FormValue("detected_id") // Dihapus
        // if detectedIDStr != "" { // Dihapus
        // 	detectedIDVal, errAtoi := strconv.Atoi(detectedIDStr) // Dihapus
        // 	if errAtoi != nil { // Dihapus
        // 		http.Error(w, "Invalid detected_id: must be an integer", http.StatusBadRequest) // Dihapus
        // 		return // Dihapus
        // 	} // Dihapus
        // 	// lr.DetectedID = &detectedIDVal // Dihapus - field ini sudah tidak ada di database.LostReport
        // } // Dihapus

        fmt.Println("DEBUG: handleCreateLostReport - Form values parsed, starting transaction")
        tx, err := s.db.Get().BeginTx(r.Context(), nil)
        if err != nil {
            fmt.Printf("ERROR: handleCreateLostReport - Failed to start database transaction: %v\n", err)
            http.Error(w, "Failed to start database transaction", http.StatusInternalServerError)
            return
        }
        var txErr error 
        var motorEvidenceImageStoragePath string
        var personEvidenceImageStoragePath string

        defer func() {
            if p := recover(); p != nil {
                fmt.Printf("PANIC: handleCreateLostReport - Rolling back transaction due to panic: %v\n", p)
                tx.Rollback()
                if motorEvidenceImageStoragePath != "" { os.Remove(motorEvidenceImageStoragePath) }
                if personEvidenceImageStoragePath != "" { os.Remove(personEvidenceImageStoragePath) }
                panic(p)
            } else if txErr != nil {
                fmt.Printf("ERROR: handleCreateLostReport - Rolling back transaction due to error: %v\n", txErr)
                tx.Rollback()
                if motorEvidenceImageStoragePath != "" { os.Remove(motorEvidenceImageStoragePath) }
                if personEvidenceImageStoragePath != "" { os.Remove(personEvidenceImageStoragePath) }
            }
        }()

        fmt.Println("DEBUG: handleCreateLostReport - Attempting to get motor_evidence_image")
        motorFile, motorHandler, errMotorFile := r.FormFile("motor_evidence_image")
        if errMotorFile == nil {
            fmt.Printf("DEBUG: handleCreateLostReport - motor_evidence_image found: %s, size: %d\n", motorHandler.Filename, motorHandler.Size)
            defer motorFile.Close()
            imgIDResult, storagePath, errUpload := s.uploadAndCreateImageRecordLr(r.Context(), tx, motorFile, motorHandler, "motor_evidence_image")
            if errUpload != nil {
                txErr = fmt.Errorf("failed to process motor_evidence_image: %w", errUpload)
                fmt.Printf("ERROR: handleCreateLostReport - %v\n", txErr)
                http.Error(w, txErr.Error(), determineImageUploadErrorStatusCode(errUpload))
                return
            }
            motorEvidenceImageStoragePath = storagePath
            if imgIDResult.Valid {
                lr.MotorEvidenceImageID = &imgIDResult.Int64
                fmt.Printf("DEBUG: handleCreateLostReport - motor_evidence_image processed, ID: %d\n", *lr.MotorEvidenceImageID)
            }
        } else if errMotorFile != http.ErrMissingFile {
            txErr = fmt.Errorf("error retrieving motor_evidence_image: %w", errMotorFile)
            fmt.Printf("ERROR: handleCreateLostReport - %v\n", txErr)
            http.Error(w, txErr.Error(), http.StatusBadRequest)
            return
        } else {
            fmt.Println("DEBUG: handleCreateLostReport - motor_evidence_image not provided (optional)")
        }

        fmt.Println("DEBUG: handleCreateLostReport - Attempting to get person_evidence_image")
        personFile, personHandler, errPersonFile := r.FormFile("person_evidence_image")
        if errPersonFile == nil {
            fmt.Printf("DEBUG: handleCreateLostReport - person_evidence_image found: %s, size: %d\n", personHandler.Filename, personHandler.Size)
            defer personFile.Close()
            imgIDResult, storagePath, errUpload := s.uploadAndCreateImageRecordLr(r.Context(), tx, personFile, personHandler, "person_evidence_image")
            if errUpload != nil {
                txErr = fmt.Errorf("failed to process person_evidence_image: %w", errUpload)
                fmt.Printf("ERROR: handleCreateLostReport - %v\n", txErr)
                http.Error(w, txErr.Error(), determineImageUploadErrorStatusCode(errUpload))
                return
            }
            personEvidenceImageStoragePath = storagePath
            if imgIDResult.Valid {
                lr.PersonEvidenceImageID = &imgIDResult.Int64
                fmt.Printf("DEBUG: handleCreateLostReport - person_evidence_image processed, ID: %d\n", *lr.PersonEvidenceImageID)
            }
        } else if errPersonFile != http.ErrMissingFile {
            txErr = fmt.Errorf("error retrieving person_evidence_image: %w", errPersonFile)
            fmt.Printf("ERROR: handleCreateLostReport - %v\n", txErr)
            http.Error(w, txErr.Error(), http.StatusBadRequest)
            return
        } else {
            fmt.Println("DEBUG: handleCreateLostReport - person_evidence_image not provided (optional)")
        }

        fmt.Printf("DEBUG: handleCreateLostReport - Attempting to create lost report record: %+v\n", lr)
        txErr = database.CreateLostReportTx(r.Context(), tx, &lr)
        if txErr != nil {
            fmt.Printf("ERROR: handleCreateLostReport - Failed to create lost report record in transaction: %v\n", txErr)
            http.Error(w, "Failed to create lost report record: "+txErr.Error(), http.StatusInternalServerError)
            return
        }

        fmt.Println("DEBUG: handleCreateLostReport - Attempting to commit transaction")
        txErr = tx.Commit()
        if txErr != nil {
            fmt.Printf("ERROR: handleCreateLostReport - Failed to commit database transaction: %v\n", txErr)
            http.Error(w, "Failed to commit database transaction", http.StatusInternalServerError)
            return
        }

        fmt.Printf("DEBUG: handleCreateLostReport - Successfully created lost report: %+v\n", lr)
        // Menggunakan toLostReportResponse untuk konsistensi dan menyertakan URL gambar jika ada
        createdLRFromDB, errGet := database.GetLostReportByID(r.Context(), s.db.Get(), lr.LostID)
        if errGet != nil {
            // Jika gagal mengambil dari DB setelah create, kirim data yang ada (tanpa URL gambar)
            fmt.Printf("WARN: handleCreateLostReport - Failed to retrieve created report for full response: %v\n", errGet)
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusCreated)
            json.NewEncoder(w).Encode(lr) // lr adalah database.LostReport, bukan LostReportResponse
            fmt.Println("DEBUG: handleCreateLostReport - End (fallback response)")
            return
        }
        response := s.toLostReportResponse(r.Context(), s.db.Get(), createdLRFromDB)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(response)
        fmt.Println("DEBUG: handleCreateLostReport - End")
    }
}

// uploadAndCreateImageRecordLr menangani validasi, penyimpanan file, dan pembuatan record di tabel 'images'.
func (s *Server) uploadAndCreateImageRecordLr(ctx context.Context, tx *sql.Tx, file multipart.File, handler *multipart.FileHeader, formFieldName string) (sql.NullInt64, string, error) {
    fmt.Printf("DEBUG: uploadAndCreateImageRecordLr called for %s, filename: %s, size: %d\n", formFieldName, handler.Filename, handler.Size)

    if handler.Size == 0 {
        return sql.NullInt64{}, "", fmt.Errorf("file for %s is empty", formFieldName)
    }

    // Batas ukuran file spesifik untuk bukti laporan kehilangan
    const maxEvidenceFileSize = 5 * 1024 * 1024 // 5 MB (bisa berbeda dari maxFileSizeDetected)
    if handler.Size > maxEvidenceFileSize {
        return sql.NullInt64{}, "", fmt.Errorf("%s file size (%d bytes) exceeds %dMB limit", formFieldName, handler.Size, maxEvidenceFileSize/(1024*1024))
    }

    // Panggil fungsi validasi MIME terpusat
    // DefaultAllowedMimeTypes dan ValidateMimeType diasumsikan ada di package server (misalnya, dari image.go)
    validatedMimeType, errMime := ValidateMimeType(file, handler, DefaultAllowedMimeTypes)
    if errMime != nil {
        return sql.NullInt64{}, "", fmt.Errorf("MIME type validation failed for %s: %w", formFieldName, errMime)
    }
    // file pointer sudah di-reset oleh ValidateMimeType jika validasi dari konten,
    // atau tidak berubah jika dari header. Kita akan reset lagi sebelum io.Copy untuk memastikan.

    originalFilename := handler.Filename
    fileExtension := filepath.Ext(originalFilename)
    uniqueFilename := fmt.Sprintf("%s%s", uuid.New().String(), fileExtension)

    // imageUploadPath diasumsikan sebagai konstanta yang dapat diakses dari package server (misalnya, image.go)
    storageDir := imageUploadPath
    storagePath := filepath.Join(storageDir, uniqueFilename)

    if err := os.MkdirAll(storageDir, os.ModePerm); err != nil {
        return sql.NullInt64{}, "", fmt.Errorf("failed to create upload directory '%s' for %s: %w", storageDir, formFieldName, err)
    }

    dst, err := os.Create(storagePath)
    if err != nil {
        return sql.NullInt64{}, storagePath, fmt.Errorf("failed to create destination file '%s' for %s: %w", storagePath, formFieldName, err)
    }
    defer dst.Close()

    // Pastikan file pointer ada di awal sebelum io.Copy
    if _, errSeek := file.Seek(0, io.SeekStart); errSeek != nil {
        os.Remove(storagePath) // Hapus file yang mungkin sudah dibuat
        return sql.NullInt64{}, storagePath, fmt.Errorf("failed to reset file pointer before copy for %s: %w", formFieldName, errSeek)
    }

	bytesCopied, err := io.Copy(dst, file)
	if err != nil {
		os.Remove(storagePath) // Hapus file parsial jika copy gagal
		return sql.NullInt64{}, storagePath, fmt.Errorf("failed to copy %s file content to '%s': %w", formFieldName, storagePath, err)
	}
	fmt.Printf("DEBUG: Successfully saved %s to %s (%d bytes copied)\n", formFieldName, storagePath, bytesCopied)

    imgRecord := database.Image{
        StoragePath:      filepath.ToSlash(storagePath), // Simpan dengan forward slashes
        FilenameOriginal: originalFilename,
        MimeType:         validatedMimeType, // Gunakan tipe MIME yang sudah divalidasi
        SizeBytes:        bytesCopied,       // Gunakan bytesCopied sebagai ukuran file yang sebenarnya disimpan
    }

    if err := database.CreateImageTx(ctx, tx, &imgRecord); err != nil {
        os.Remove(storagePath) // Hapus file fisik jika insert DB gagal
        return sql.NullInt64{}, storagePath, fmt.Errorf("failed to save %s image metadata to DB: %w", formFieldName, err)
    }
    fmt.Printf("DEBUG: Successfully created image record for %s. ImageID: %d\n", formFieldName, imgRecord.ImageID)

    return sql.NullInt64{Int64: imgRecord.ImageID, Valid: true}, storagePath, nil
}

func (s *Server) handleListLostReports() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        db := s.db.Get()
        statusFilter := r.URL.Query().Get("status")
        if statusFilter != "" {
            isValidStatus := false
            validStatuses := []string{database.StatusLostReportBelumDiproses, database.StatusLostReportSedangDiproses, database.StatusLostReportSudahDitemukan}
            for _, vs := range validStatuses {
                if statusFilter == vs {
                    isValidStatus = true
                    break
                }
            }
            if !isValidStatus {
                http.Error(w, fmt.Sprintf("Invalid status filter. Valid statuses are: %s, %s, %s", database.StatusLostReportBelumDiproses, database.StatusLostReportSedangDiproses, database.StatusLostReportSudahDitemukan), http.StatusBadRequest)
                return
            }
        }

        list, err := database.ListLostReports(r.Context(), db, statusFilter)
        if err != nil {
            fmt.Printf("ERROR: Failed to list lost reports (filter: '%s'): %v\n", statusFilter, err)
            http.Error(w, "Failed to list lost reports: "+err.Error(), http.StatusInternalServerError)
            return
        }

        responseList := make([]LostReportResponse, 0, len(list))
        for i := range list {
            responseList = append(responseList, s.toLostReportResponse(r.Context(), db, &list[i]))
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(responseList)
    }
}

func (s *Server) handleGetLostReportByID() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        idStr := mux.Vars(r)["id"]
        id, err := strconv.Atoi(idStr)
        if err != nil {
            http.Error(w, "invalid lost_id: must be an integer", http.StatusBadRequest)
            return
        }

        db := s.db.Get()
        lr, err := database.GetLostReportByID(r.Context(), db, id)
        if err != nil {
            if err.Error() == "lost_report not found" || errors.Is(err, sql.ErrNoRows) {
                http.Error(w, "Lost report not found", http.StatusNotFound)
            } else {
                fmt.Printf("ERROR: Failed to get lost report by ID %d: %v\n", id, err)
                http.Error(w, "Failed to get lost report: "+err.Error(), http.StatusInternalServerError)
            }
            return
        }

        // --- LOGIKA OTORISASI ---
        requestingUserID, _ := r.Context().Value(middleware.UserIDContextKey).(string)
        isAdmin, _ := r.Context().Value(middleware.AdminStatusContextKey).(bool)

        // Tolak akses jika pengguna bukan admin DAN bukan pemilik laporan.
        if !isAdmin && lr.UserID != requestingUserID {
            http.Error(w, "Forbidden: You can only view your own reports or you must be an administrator.", http.StatusForbidden)
            return
        }
        // --- AKHIR LOGIKA OTORISASI ---

        response := s.toLostReportResponse(r.Context(), db, lr)
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(response)
    }
}

func (s *Server) handleGetUserLostReports() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Ambil ID pengguna yang membuat permintaan dari token JWT
        requestingUserID, ok := r.Context().Value(middleware.UserIDContextKey).(string)
        if !ok || requestingUserID == "" {
            http.Error(w, "Unauthorized: User ID not found in token", http.StatusUnauthorized)
            return
        }

        db := s.db.Get()

        // Panggil fungsi database untuk mengambil laporan berdasarkan UserID
        // CATATAN: Ini memerlukan fungsi baru `ListLostReportsByUserID` di package database Anda.
        list, err := database.ListLostReportsByUserID(r.Context(), db, requestingUserID)
        if err != nil {
            fmt.Printf("ERROR: Failed to list lost reports for user ID '%s': %v\n", requestingUserID, err)
            http.Error(w, "Failed to list your lost reports: "+err.Error(), http.StatusInternalServerError)
            return
        }

        // Konversi hasil ke format respons
        responseList := make([]LostReportResponse, 0, len(list))
        for i := range list {
            responseList = append(responseList, s.toLostReportResponse(r.Context(), db, &list[i]))
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(responseList)
    }
}

func (s *Server) handleUpdateLostReport() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        idStr := mux.Vars(r)["id"]
        id, err := strconv.Atoi(idStr)
        if err != nil {
            http.Error(w, "invalid lost_id: must be an integer", http.StatusBadRequest)
            return
        }

        var lrUpdates database.LostReport 
        
        if err := json.NewDecoder(r.Body).Decode(&lrUpdates); err != nil {
            http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
            return
        }

        requestingUserID, ok := r.Context().Value(middleware.UserIDContextKey).(string)
        if !ok || requestingUserID == "" {
            http.Error(w, "Unauthorized: User ID not found in token", http.StatusUnauthorized)
            return
        }
        isAdmin, _ := r.Context().Value(middleware.AdminStatusContextKey).(bool)

        existingLR, err := database.GetLostReportByID(r.Context(), s.db.Get(), id)
        if err != nil {
            if err.Error() == "lost_report not found" {
                http.Error(w, "Lost report not found", http.StatusNotFound)
            } else {
                http.Error(w, "Failed to retrieve existing report: "+err.Error(), http.StatusInternalServerError)
            }
            return
        }

        reportToUpdate := *existingLR
        updatedByOwner := false
        updatedByAdmin := false

        if isAdmin {
            if lrUpdates.Status != "" && lrUpdates.Status != existingLR.Status {
                validStatuses := []string{database.StatusLostReportBelumDiproses, database.StatusLostReportSedangDiproses, database.StatusLostReportSudahDitemukan}
                isValidNewStatus := false
                for _, vs := range validStatuses {
                    if lrUpdates.Status == vs {
                        isValidNewStatus = true
                        break
                    }
                }
                if !isValidNewStatus {
                    http.Error(w, fmt.Sprintf("Invalid status value. Valid statuses are: %s, %s, %s", database.StatusLostReportBelumDiproses, database.StatusLostReportSedangDiproses, database.StatusLostReportSudahDitemukan), http.StatusBadRequest)
                    return
                }
                reportToUpdate.Status = lrUpdates.Status
                updatedByAdmin = true
            }
        }

        if existingLR.UserID == requestingUserID {
            if !lrUpdates.Timestamp.IsZero() && lrUpdates.Timestamp != existingLR.Timestamp {
                if lrUpdates.Timestamp.After(time.Now().Add(5 * time.Minute)) {
                    http.Error(w, "timestamp (waktu kejadian) cannot be unreasonably in the future", http.StatusBadRequest)
                    return
                }
                reportToUpdate.Timestamp = lrUpdates.Timestamp
                updatedByOwner = true
            }

            if lrUpdates.Address != "" && lrUpdates.Address != existingLR.Address {
                reportToUpdate.Address = lrUpdates.Address
                updatedByOwner = true
            }

            if lrUpdates.VehicleID != 0 && lrUpdates.VehicleID != existingLR.VehicleID {
                reportToUpdate.VehicleID = lrUpdates.VehicleID
                updatedByOwner = true
            }
            
            if lrUpdates.MotorEvidenceImageID != existingLR.MotorEvidenceImageID {
                reportToUpdate.MotorEvidenceImageID = lrUpdates.MotorEvidenceImageID
                updatedByOwner = true
            }
            if lrUpdates.PersonEvidenceImageID != existingLR.PersonEvidenceImageID {
                reportToUpdate.PersonEvidenceImageID = lrUpdates.PersonEvidenceImageID
                updatedByOwner = true
            }
        } else {
            if !isAdmin {
                http.Error(w, "Forbidden: You can only update your own report's details or an admin can update status.", http.StatusForbidden)
                return
            }
            if (lrUpdates.Timestamp != existingLR.Timestamp && !lrUpdates.Timestamp.IsZero()) ||
                (lrUpdates.Address != existingLR.Address && lrUpdates.Address != "") ||
                (lrUpdates.VehicleID != existingLR.VehicleID && lrUpdates.VehicleID != 0) ||
                (lrUpdates.MotorEvidenceImageID != existingLR.MotorEvidenceImageID) ||
                (lrUpdates.PersonEvidenceImageID != existingLR.PersonEvidenceImageID) {
                if !updatedByAdmin {
                    http.Error(w, "Forbidden: Admin can only update status or owner can update details.", http.StatusForbidden)
                    return
                }
            }
        }

        reportToUpdate.UserID = existingLR.UserID 
        // reportToUpdate.DetectedID = existingLR.DetectedID // Dihapus - field ini sudah tidak ada

        if !updatedByAdmin && !updatedByOwner {
            // Menggunakan toLostReportResponse untuk konsistensi
            response := s.toLostReportResponse(r.Context(), s.db.Get(), existingLR)
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusOK)
            json.NewEncoder(w).Encode(response)
            return
        }

        if err := database.UpdateLostReport(r.Context(), s.db.Get(), id, &reportToUpdate); err != nil {
            http.Error(w, "Failed to update lost report: "+err.Error(), http.StatusInternalServerError)
            return
        }

        updatedReport, err := database.GetLostReportByID(r.Context(), s.db.Get(), id)
        if err != nil {
            // Jika gagal mengambil dari DB setelah update, kirim data yang diupdate (tanpa URL gambar yang mungkin baru)
            fmt.Printf("WARN: handleUpdateLostReport - Failed to retrieve updated report for full response: %v\n", err)
            response := s.toLostReportResponse(r.Context(), s.db.Get(), &reportToUpdate) // Gunakan reportToUpdate
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusOK)
            json.NewEncoder(w).Encode(response)
            return
        }
        // Menggunakan toLostReportResponse untuk konsistensi
        response := s.toLostReportResponse(r.Context(), s.db.Get(), updatedReport)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(response)
    }
}

func (s *Server) handleDeleteLostReport() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        idStr := mux.Vars(r)["id"]
        id, err := strconv.Atoi(idStr)
        if err != nil {
            http.Error(w, "invalid lost_id: must be an integer", http.StatusBadRequest)
            return
        }

        requestingUserID, ok := r.Context().Value(middleware.UserIDContextKey).(string)
        if !ok || requestingUserID == "" {
            http.Error(w, "Unauthorized: User ID not found in token", http.StatusUnauthorized)
            return
        }
        isAdmin, _ := r.Context().Value(middleware.AdminStatusContextKey).(bool)


        existingLR, err := database.GetLostReportByID(r.Context(), s.db.Get(), id)
        if err != nil {
            if err.Error() == "lost_report not found" {
                http.Error(w, "Lost report not found", http.StatusNotFound)
            } else {
                http.Error(w, "Failed to retrieve existing report: "+err.Error(), http.StatusInternalServerError)
            }
            return
        }

        if !isAdmin && existingLR.UserID != requestingUserID {
            http.Error(w, "Forbidden: You can only delete your own reports or an admin can delete any report.", http.StatusForbidden)
            return
        }

        // TODO: Pertimbangkan untuk menghapus gambar terkait dari storage jika ada (existingLR.MotorEvidenceImageID dan existingLR.PersonEvidenceImageID)
        // dan juga dari tabel 'images'. Ini memerlukan logika tambahan dan transaksi.

        if err := database.DeleteLostReport(r.Context(), s.db.Get(), id); err != nil {
            http.Error(w, "Failed to delete lost report: "+err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusNoContent)
    }
}

// RegisterLostReportRoutes mendaftarkan semua rute terkait lost_report.
func (s *Server) RegisterLostReportRoutes(r *mux.Router) {
    adminOnlyMiddleware := middleware.AdminOnlyMiddleware()

    r.HandleFunc("/lost_reports", s.handleCreateLostReport()).Methods("POST")
    r.Handle("/lost_reports", adminOnlyMiddleware(s.handleListLostReports())).Methods("GET")
    r.HandleFunc("/lost_reports/my", s.handleGetUserLostReports()).Methods("GET")
    r.HandleFunc("/lost_reports/{id}", s.handleGetLostReportByID()).Methods("GET") // Seharusnya id adalah integer
    r.HandleFunc("/lost_reports/{id:[0-9]+}", s.handleUpdateLostReport()).Methods("PUT") // Pastikan id adalah integer
    r.HandleFunc("/lost_reports/{id:[0-9]+}", s.handleDeleteLostReport()).Methods("DELETE") // Pastikan id adalah integer
}