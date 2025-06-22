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

// VehicleInfo adalah struct ringkas untuk detail kendaraan dalam respons.
type VehicleInfo struct {
    VehicleName string `json:"vehicle_name"`
    PlateNumber string `json:"plate_number"`
}

type LostReportResponse struct {
    LostID                 int          `json:"lost_id"`
    UserID                 string       `json:"user_id"`
    Timestamp              time.Time    `json:"timestamp"`
    VehicleID              int          `json:"vehicle_id"`
    Address                string       `json:"address"`
    Latitude               *float64     `json:"latitude,omitempty"`  // Ditambahkan
    Longitude              *float64     `json:"longitude,omitempty"` // Ditambahkan
    Status                 string       `json:"status"`
    MotorEvidenceImageURL  *string      `json:"motor_evidence_image_url,omitempty"`
    PersonEvidenceImageURL *string      `json:"person_evidence_image_url,omitempty"`
    Vehicle                *VehicleInfo `json:"vehicle,omitempty"` // Menggunakan struct ringkas
}

// Helper untuk mengubah database.LostReportWithVehicleInfo menjadi LostReportResponse
func (s *Server) toLostReportResponse(ctx context.Context, dbQuerier database.Querier, lr *database.LostReportWithVehicleInfo) LostReportResponse {
    response := LostReportResponse{
        LostID:    lr.LostID,
        UserID:    lr.UserID,
        Timestamp: lr.Timestamp,
        VehicleID: lr.VehicleID,
        Address:   lr.Address,
        Latitude:  lr.Latitude,  // Ditambahkan
        Longitude: lr.Longitude, // Ditambahkan
        Status:    lr.Status,
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

    // Mengisi info kendaraan jika ada (hasil dari LEFT JOIN valid)
    if lr.VehicleName.Valid {
        response.Vehicle = &VehicleInfo{
            VehicleName: lr.VehicleName.String,
            PlateNumber: lr.PlateNumber.String,
        }
    }

    return response
}

func (s *Server) handleCreateLostReport() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        const maxEvidenceFileSize = 5 * 1024 * 1024
        const extraFormDataSize = 1 * 1024 * 1024
        maxTotalSize := extraFormDataSize + 2*maxEvidenceFileSize

        if err := r.ParseMultipartForm(int64(maxTotalSize)); err != nil {
            writeJSONError(w, "Request too large or invalid multipart form: "+err.Error(), http.StatusBadRequest)
            return
        }

        var lr database.LostReport
        var err error

        requestingUserID, ok := r.Context().Value(middleware.UserIDContextKey).(string)
        if !ok || requestingUserID == "" {
            writeJSONError(w, "Unauthorized: User ID not found in token", http.StatusUnauthorized)
            return
        }
        lr.UserID = requestingUserID

        timestampStr := r.FormValue("timestamp")
        if timestampStr == "" {
            lr.Timestamp = time.Now()
        } else {
            lr.Timestamp, err = time.Parse(time.RFC3339, timestampStr)
            if err != nil {
                writeJSONError(w, fmt.Sprintf("Invalid timestamp format. Use RFC3339. Error: %v", err), http.StatusBadRequest)
                return
            }
            if lr.Timestamp.After(time.Now().Add(5 * time.Minute)) {
                writeJSONError(w, "timestamp (waktu kejadian) cannot be unreasonably in the future", http.StatusBadRequest)
                return
            }
        }

        vehicleIDStr := r.FormValue("vehicle_id")
        if vehicleIDStr == "" {
            writeJSONError(w, "vehicle_id is required", http.StatusBadRequest)
            return
        }
        lr.VehicleID, err = strconv.Atoi(vehicleIDStr)
        if err != nil {
            writeJSONError(w, "Invalid vehicle_id: must be an integer", http.StatusBadRequest)
            return
        }

        lr.Address = r.FormValue("address")
        if lr.Address == "" {
            writeJSONError(w, "address is required", http.StatusBadRequest)
            return
        }

        // --- Validasi Latitude & Longitude ---
        latStr := r.FormValue("latitude")
        lonStr := r.FormValue("longitude")

        if latStr != "" && lonStr != "" {
            lat, errLat := strconv.ParseFloat(latStr, 64)
            lon, errLon := strconv.ParseFloat(lonStr, 64)

            if errLat != nil || errLon != nil {
                writeJSONError(w, "Invalid format for latitude or longitude. They must be numbers.", http.StatusBadRequest)
                return
            }
            if lat < -90 || lat > 90 {
                writeJSONError(w, "Latitude must be between -90 and 90.", http.StatusBadRequest)
                return
            }
            if lon < -180 || lon > 180 {
                writeJSONError(w, "Longitude must be between -180 and 180.", http.StatusBadRequest)
                return
            }
            lr.Latitude = &lat
            lr.Longitude = &lon
        } else if latStr != "" || lonStr != "" {
            // Jika hanya salah satu yang diisi
            writeJSONError(w, "Both latitude and longitude must be provided together.", http.StatusBadRequest)
            return
        }
        // --- Akhir Validasi ---

        statusStr := r.FormValue("status")
        if statusStr != "" {
            lr.Status = statusStr
        } else {
            lr.Status = database.StatusLostReportBelumDiproses
        }

        tx, err := s.db.Get().BeginTx(r.Context(), nil)
        if err != nil {
            writeJSONError(w, "Failed to start database transaction", http.StatusInternalServerError)
            return
        }
        var txErr error
        var motorEvidenceImageStoragePath string
        var personEvidenceImageStoragePath string

        defer func() {
            if p := recover(); p != nil {
                _ = tx.Rollback()
                if motorEvidenceImageStoragePath != "" {
                    _ = os.Remove(motorEvidenceImageStoragePath)
                }
                if personEvidenceImageStoragePath != "" {
                    _ = os.Remove(personEvidenceImageStoragePath)
                }
                panic(p)
            } else if txErr != nil {
                _ = tx.Rollback()
                if motorEvidenceImageStoragePath != "" {
                    _ = os.Remove(motorEvidenceImageStoragePath)
                }
                if personEvidenceImageStoragePath != "" {
                    _ = os.Remove(personEvidenceImageStoragePath)
                }
            }
        }()

        motorFile, motorHandler, errMotorFile := r.FormFile("motor_evidence_image")
        if errMotorFile == nil {
            defer motorFile.Close()
            imgIDResult, storagePath, errUpload := s.uploadAndCreateImageRecordLr(r.Context(), tx, motorFile, motorHandler, "motor_evidence_image")
            if errUpload != nil {
                txErr = fmt.Errorf("failed to process motor_evidence_image: %w", errUpload)
                writeJSONError(w, txErr.Error(), determineImageUploadErrorStatusCode(errUpload))
                return
            }
            motorEvidenceImageStoragePath = storagePath
            if imgIDResult.Valid {
                lr.MotorEvidenceImageID = &imgIDResult.Int64
            }
        } else if !errors.Is(errMotorFile, http.ErrMissingFile) {
            txErr = fmt.Errorf("error retrieving motor_evidence_image: %w", errMotorFile)
            writeJSONError(w, txErr.Error(), http.StatusBadRequest)
            return
        }

        personFile, personHandler, errPersonFile := r.FormFile("person_evidence_image")
        if errPersonFile == nil {
            defer personFile.Close()
            imgIDResult, storagePath, errUpload := s.uploadAndCreateImageRecordLr(r.Context(), tx, personFile, personHandler, "person_evidence_image")
            if errUpload != nil {
                txErr = fmt.Errorf("failed to process person_evidence_image: %w", errUpload)
                writeJSONError(w, txErr.Error(), determineImageUploadErrorStatusCode(errUpload))
                return
            }
            personEvidenceImageStoragePath = storagePath
            if imgIDResult.Valid {
                lr.PersonEvidenceImageID = &imgIDResult.Int64
            }
        } else if !errors.Is(errPersonFile, http.ErrMissingFile) {
            txErr = fmt.Errorf("error retrieving person_evidence_image: %w", errPersonFile)
            writeJSONError(w, txErr.Error(), http.StatusBadRequest)
            return
        }

        txErr = database.CreateLostReportTx(r.Context(), tx, &lr)
        if txErr != nil {
            writeJSONError(w, "Failed to create lost report record: "+txErr.Error(), http.StatusInternalServerError)
            return
        }

        txErr = tx.Commit()
        if txErr != nil {
            writeJSONError(w, "Failed to commit database transaction", http.StatusInternalServerError)
            return
        }

        createdLRFromDB, errGet := database.GetLostReportWithVehicleInfoByID(r.Context(), s.db.Get(), lr.LostID)
        if errGet != nil {
            fmt.Printf("WARN: handleCreateLostReport - Failed to retrieve created report for full response: %v\n", errGet)
            // Fallback: create response from the data we have, without vehicle/image details
            fallbackResponse := LostReportResponse{
                LostID:    lr.LostID,
                UserID:    lr.UserID,
                Timestamp: lr.Timestamp,
                VehicleID: lr.VehicleID,
                Address:   lr.Address,
                Status:    lr.Status,
            }
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusCreated)
            json.NewEncoder(w).Encode(fallbackResponse)
            return
        }
        response := s.toLostReportResponse(r.Context(), s.db.Get(), createdLRFromDB)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(response)
    }
}

func (s *Server) uploadAndCreateImageRecordLr(ctx context.Context, tx *sql.Tx, file multipart.File, handler *multipart.FileHeader, formFieldName string) (sql.NullInt64, string, error) {
    if handler.Size == 0 {
        return sql.NullInt64{}, "", fmt.Errorf("file for %s is empty", formFieldName)
    }

    const maxEvidenceFileSize = 5 * 1024 * 1024 // 5 MB
    if handler.Size > maxEvidenceFileSize {
        return sql.NullInt64{}, "", fmt.Errorf("%s file size (%d bytes) exceeds %dMB limit", formFieldName, handler.Size, maxEvidenceFileSize/(1024*1024))
    }

    validatedMimeType, errMime := ValidateMimeType(file, handler, DefaultAllowedMimeTypes)
    if errMime != nil {
        return sql.NullInt64{}, "", fmt.Errorf("MIME type validation failed for %s: %w", formFieldName, errMime)
    }

    originalFilename := handler.Filename
    fileExtension := filepath.Ext(originalFilename)
    uniqueFilename := fmt.Sprintf("%s%s", uuid.New().String(), fileExtension)

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

    if _, errSeek := file.Seek(0, io.SeekStart); errSeek != nil {
        _ = os.Remove(storagePath)
        return sql.NullInt64{}, storagePath, fmt.Errorf("failed to reset file pointer before copy for %s: %w", formFieldName, errSeek)
    }

    bytesCopied, err := io.Copy(dst, file)
    if err != nil {
        _ = os.Remove(storagePath)
        return sql.NullInt64{}, storagePath, fmt.Errorf("failed to copy %s file content to '%s': %w", formFieldName, storagePath, err)
    }

    imgRecord := database.Image{
        StoragePath:      filepath.ToSlash(storagePath),
        FilenameOriginal: originalFilename,
        MimeType:         validatedMimeType,
        SizeBytes:        bytesCopied,
    }

    if err := database.CreateImageTx(ctx, tx, &imgRecord); err != nil {
        _ = os.Remove(storagePath)
        return sql.NullInt64{}, storagePath, fmt.Errorf("failed to save %s image metadata to DB: %w", formFieldName, err)
    }
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
                writeJSONError(w, fmt.Sprintf("Invalid status filter. Valid statuses are: %s, %s, %s", database.StatusLostReportBelumDiproses, database.StatusLostReportSedangDiproses, database.StatusLostReportSudahDitemukan), http.StatusBadRequest)
                return
            }
        }

        list, err := database.ListLostReportsWithVehicleInfo(r.Context(), db, statusFilter)
        if err != nil {
            writeJSONError(w, "Failed to list lost reports: "+err.Error(), http.StatusInternalServerError)
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
            writeJSONError(w, "invalid lost_id: must be an integer", http.StatusBadRequest)
            return
        }

        db := s.db.Get()
        lr, err := database.GetLostReportWithVehicleInfoByID(r.Context(), db, id)
        if err != nil {
            if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "not found") {
                writeJSONError(w, "Lost report not found", http.StatusNotFound)
            } else {
                writeJSONError(w, "Failed to get lost report: "+err.Error(), http.StatusInternalServerError)
            }
            return
        }

        requestingUserID, _ := r.Context().Value(middleware.UserIDContextKey).(string)
        isAdmin, _ := r.Context().Value(middleware.AdminStatusContextKey).(bool)

        if !isAdmin && lr.UserID != requestingUserID {
            writeJSONError(w, "Forbidden: You can only view your own reports or you must be an administrator.", http.StatusForbidden)
            return
        }

        response := s.toLostReportResponse(r.Context(), db, lr)
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(response)
    }
}

func (s *Server) handleGetUserLostReports() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        requestingUserID, ok := r.Context().Value(middleware.UserIDContextKey).(string)
        if !ok || requestingUserID == "" {
            writeJSONError(w, "Unauthorized: User ID not found in token", http.StatusUnauthorized)
            return
        }

        db := s.db.Get()
        // NOTE: This assumes a function `ListLostReportsWithVehicleInfoByUserID` exists in your database package.
        list, err := database.ListLostReportsWithVehicleInfoByUserID(r.Context(), db, requestingUserID)
        if err != nil {
            writeJSONError(w, "Failed to list your lost reports: "+err.Error(), http.StatusInternalServerError)
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

func (s *Server) handleUpdateLostReport() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        idStr := mux.Vars(r)["id"]
        id, err := strconv.Atoi(idStr)
        if err != nil {
            writeJSONError(w, "invalid lost_id: must be an integer", http.StatusBadRequest)
            return
        }

        var updates struct {
            Timestamp *string  `json:"timestamp"`
            Address   *string  `json:"address"`
            VehicleID *int     `json:"vehicle_id"`
            Status    *string  `json:"status"`
            Latitude  *float64 `json:"latitude"`  // Ditambahkan
            Longitude *float64 `json:"longitude"` // Ditambahkan
        }

        if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
            writeJSONError(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
            return
        }

        requestingUserID, _ := r.Context().Value(middleware.UserIDContextKey).(string)
        isAdmin, _ := r.Context().Value(middleware.AdminStatusContextKey).(bool)

        existingLR, err := database.GetLostReportWithVehicleInfoByID(r.Context(), s.db.Get(), id)
        if err != nil {
            if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "not found") {
                writeJSONError(w, "Lost report not found", http.StatusNotFound)
            } else {
                writeJSONError(w, "Failed to retrieve existing report: "+err.Error(), http.StatusInternalServerError)
            }
            return
        }

        isOwner := existingLR.UserID == requestingUserID
        if !isOwner && !isAdmin {
            writeJSONError(w, "Forbidden: You do not have permission to update this report.", http.StatusForbidden)
            return
        }

        reportToUpdate := database.LostReport{
            LostID:                existingLR.LostID,
            UserID:                existingLR.UserID,
            Timestamp:             existingLR.Timestamp,
            VehicleID:             existingLR.VehicleID,
            Address:               existingLR.Address,
            Latitude:              existingLR.Latitude,  // Ditambahkan
            Longitude:             existingLR.Longitude, // Ditambahkan
            Status:                existingLR.Status,
            MotorEvidenceImageID:  existingLR.MotorEvidenceImageID,
            PersonEvidenceImageID: existingLR.PersonEvidenceImageID,
        }

        anythingChanged := false

        if isAdmin && updates.Status != nil {
            validStatuses := []string{database.StatusLostReportBelumDiproses, database.StatusLostReportSedangDiproses, database.StatusLostReportSudahDitemukan}
            isValidNewStatus := false
            for _, vs := range validStatuses {
                if *updates.Status == vs {
                    isValidNewStatus = true
                    break
                }
            }
            if !isValidNewStatus {
                writeJSONError(w, "Invalid status value.", http.StatusBadRequest)
                return
            }
            if reportToUpdate.Status != *updates.Status {
                reportToUpdate.Status = *updates.Status
                anythingChanged = true
            }
        }

        if isOwner {
            if updates.Timestamp != nil {
                parsedTime, err := time.Parse(time.RFC3339, *updates.Timestamp)
                if err != nil {
                    writeJSONError(w, "Invalid timestamp format. Use RFC3339.", http.StatusBadRequest)
                    return
                }
                if parsedTime.After(time.Now().Add(5 * time.Minute)) {
                    writeJSONError(w, "timestamp (waktu kejadian) cannot be unreasonably in the future", http.StatusBadRequest)
                    return
                }
                if reportToUpdate.Timestamp != parsedTime {
                    reportToUpdate.Timestamp = parsedTime
                    anythingChanged = true
                }
            }
            if updates.Address != nil && reportToUpdate.Address != *updates.Address {
                reportToUpdate.Address = *updates.Address
                anythingChanged = true
            }
            if updates.VehicleID != nil && reportToUpdate.VehicleID != *updates.VehicleID {
                reportToUpdate.VehicleID = *updates.VehicleID
                anythingChanged = true
            }
            if updates.Latitude != nil && updates.Longitude != nil {
                if *updates.Latitude < -90 || *updates.Latitude > 90 {
                    writeJSONError(w, "Latitude must be between -90 and 90.", http.StatusBadRequest)
                    return
                }
                if *updates.Longitude < -180 || *updates.Longitude > 180 {
                    writeJSONError(w, "Longitude must be between -180 and 180.", http.StatusBadRequest)
                    return
                }
                reportToUpdate.Latitude = updates.Latitude
                reportToUpdate.Longitude = updates.Longitude
                anythingChanged = true
            } else if updates.Latitude != nil || updates.Longitude != nil {
                writeJSONError(w, "Both latitude and longitude must be provided together for an update.", http.StatusBadRequest)
                return
            }
        }

        if !anythingChanged {
            response := s.toLostReportResponse(r.Context(), s.db.Get(), existingLR)
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(response)
            return
        }

        if err := database.UpdateLostReport(r.Context(), s.db.Get(), id, &reportToUpdate); err != nil {
            writeJSONError(w, "Failed to update lost report: "+err.Error(), http.StatusInternalServerError)
            return
        }

        updatedReport, err := database.GetLostReportWithVehicleInfoByID(r.Context(), s.db.Get(), id)
        if err != nil {
            writeJSONError(w, "Failed to retrieve updated report after update: "+err.Error(), http.StatusInternalServerError)
            return
        }

        response := s.toLostReportResponse(r.Context(), s.db.Get(), updatedReport)
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(response)
    }
}

func (s *Server) handleDeleteLostReport() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        idStr := mux.Vars(r)["id"]
        id, err := strconv.Atoi(idStr)
        if err != nil {
            writeJSONError(w, "invalid lost_id: must be an integer", http.StatusBadRequest)
            return
        }

        requestingUserID, _ := r.Context().Value(middleware.UserIDContextKey).(string)
        isAdmin, _ := r.Context().Value(middleware.AdminStatusContextKey).(bool)

        existingLR, err := database.GetLostReportWithVehicleInfoByID(r.Context(), s.db.Get(), id)
        if err != nil {
            if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "not found") {
                writeJSONError(w, "Lost report not found", http.StatusNotFound)
            } else {
                writeJSONError(w, "Failed to retrieve existing report: "+err.Error(), http.StatusInternalServerError)
            }
            return
        }

        if !isAdmin && existingLR.UserID != requestingUserID {
            writeJSONError(w, "Forbidden: You can only delete your own reports or an admin can delete any report.", http.StatusForbidden)
            return
        }

        // TODO: Consider deleting related images from storage and the 'images' table.
        // This requires transactional logic.

        if err := database.DeleteLostReport(r.Context(), s.db.Get(), id); err != nil {
            writeJSONError(w, "Failed to delete lost report: "+err.Error(), http.StatusInternalServerError)
            return
        }

        w.WriteHeader(http.StatusNoContent)
    }
}

// Regis rute lost report
func (s *Server) RegisterLostReportRoutes(r *mux.Router) {

    adminOnlyMiddleware := middleware.AdminOnlyMiddleware()

    r.Handle("/lost_reports", adminOnlyMiddleware(s.handleListLostReports())).Methods("GET")
    r.HandleFunc("/lost_reports", s.handleCreateLostReport()).Methods("POST")
    r.HandleFunc("/lost_reports/my", s.handleGetUserLostReports()).Methods("GET")
    r.HandleFunc("/lost_reports/{id:[0-9]+}", s.handleGetLostReportByID()).Methods("GET")
    r.HandleFunc("/lost_reports/{id:[0-9]+}", s.handleUpdateLostReport()).Methods("PUT")
    r.HandleFunc("/lost_reports/{id:[0-9]+}", s.handleDeleteLostReport()).Methods("DELETE")
}