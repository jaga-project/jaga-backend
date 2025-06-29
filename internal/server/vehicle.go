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

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
	"github.com/jaga-project/jaga-backend/internal/middleware"
)

const maxFileSizeVehicle = 5 * 1024 * 1024       
const extraFormDataSizeVehicle = 5 * 1024 * 1024 

type VehicleResponse struct {
    VehicleID    int64                   `json:"vehicle_id"`
    VehicleName  string                  `json:"vehicle_name"`
    Color        string                  `json:"color"`
    UserID       string                  `json:"user_id"`
    PlateNumber  string                  `json:"plate_number"`
    STNKImageURL *string                 `json:"stnk_image_url,omitempty"`
    KKImageURL   *string                 `json:"kk_image_url,omitempty"`
    Ownership    *database.OwnershipType `json:"ownership,omitempty"`
}

func (s *Server) toVehicleResponse(ctx context.Context, dbQuerier database.Querier, v *database.Vehicle) VehicleResponse {
    response := VehicleResponse{
        VehicleID:   v.VehicleID,
        VehicleName: v.VehicleName,
        Color:       v.Color,
        UserID:      v.UserID,
        PlateNumber: v.PlateNumber,
    }

		if v.Ownership.Valid {
        ownershipValue := database.OwnershipType(v.Ownership.String)
        response.Ownership = &ownershipValue
    } else {
        response.Ownership = nil 
    }

    if v.STNKImageID.Valid {
        path, err := database.GetImageStoragePath(ctx, dbQuerier, v.STNKImageID.Int64)
        if err == nil && path != "" {
            url := "/" + strings.TrimPrefix(path, "/")
            response.STNKImageURL = &url
        } else if err != nil && !errors.Is(err, sql.ErrNoRows) {
            fmt.Printf("WARN: Failed to get STNK image path for vehicle ID %d, image ID %d: %v\n", v.VehicleID, v.STNKImageID.Int64, err)
        }
    }

    if v.KKImageID.Valid {
        path, err := database.GetImageStoragePath(ctx, dbQuerier, v.KKImageID.Int64)
        if err == nil && path != "" {
            url := "/" + strings.TrimPrefix(path, "/")
            response.KKImageURL = &url
        } else if err != nil && !errors.Is(err, sql.ErrNoRows) {
            fmt.Printf("WARN: Failed to get KK image path for vehicle ID %d, image ID %d: %v\n", v.VehicleID, v.KKImageID.Int64, err)
        }
    }
    return response
}

func (s *Server) handleCreateVehicle() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        maxTotalSize := extraFormDataSizeVehicle + 2*maxFileSizeVehicle
        if err := r.ParseMultipartForm(int64(maxTotalSize)); err != nil {
            writeJSONError(w, "Request too large or invalid multipart form: "+err.Error(), http.StatusBadRequest)
            return
        }

        var newVehicleDB database.Vehicle
        var err error

        userIDFromCtx, ok := r.Context().Value(middleware.UserIDContextKey).(string)
        if !ok || userIDFromCtx == "" {
            writeJSONError(w, "Unauthorized: User ID not found in context.", http.StatusUnauthorized)
            return
        }

        newVehicleDB.VehicleName = r.FormValue("vehicle_name")
        newVehicleDB.Color = r.FormValue("color")
        newVehicleDB.UserID = userIDFromCtx
        newVehicleDB.PlateNumber = r.FormValue("plate_number")
        ownershipStr := r.FormValue("ownership")

        if newVehicleDB.VehicleName == "" || newVehicleDB.PlateNumber == "" {
            writeJSONError(w, "vehicle_name and plate_number are required", http.StatusBadRequest)
            return
        }

        if ownershipStr != "" {
            if ownershipStr == string(database.OwnershipPribadi) || ownershipStr == string(database.OwnershipKeluarga) {
                newVehicleDB.Ownership = sql.NullString{String: ownershipStr, Valid: true}
            } else {
                writeJSONError(w, fmt.Sprintf("Invalid ownership value. Must be '%s' or '%s'", database.OwnershipPribadi, database.OwnershipKeluarga), http.StatusBadRequest)
                return
            }
        } else {
            newVehicleDB.Ownership = sql.NullString{Valid: false}
        }

        tx, err := s.db.Get().BeginTx(r.Context(), nil)
        if err != nil {
            fmt.Printf("ERROR: Failed to start database transaction for vehicle creation: %v\n", err)
            writeJSONError(w, "Failed to start database transaction", http.StatusInternalServerError)
            return
        }
        var txErr error
        var stnkImageStoragePath string
        var kkImageStoragePath string

        defer func() {
            if p := recover(); p != nil {
                tx.Rollback()
                if stnkImageStoragePath != "" { os.Remove(stnkImageStoragePath) }
                if kkImageStoragePath != "" { os.Remove(kkImageStoragePath) }
                panic(p)
            } else if txErr != nil {
                tx.Rollback()
                if stnkImageStoragePath != "" { os.Remove(stnkImageStoragePath) }
                if kkImageStoragePath != "" { os.Remove(kkImageStoragePath) }
            }
        }()

        stnkFile, stnkHandler, errSTNK := r.FormFile("stnk_image")
        if errSTNK == nil {
            defer stnkFile.Close()
            stnkImageID, tempPath, errUpload := s.uploadAndCreateImageRecord(r.Context(), tx, stnkFile, stnkHandler, "stnk_image", maxFileSizeVehicle)
            if errUpload != nil {
                txErr = fmt.Errorf("failed to process stnk_image: %w", errUpload)
                statusCode := http.StatusInternalServerError
                errMsg := strings.ToLower(errUpload.Error())
                if strings.Contains(errMsg, "file is empty") || strings.Contains(errMsg, "exceeds") || strings.Contains(errMsg, "invalid type") || strings.Contains(errMsg, "mime type validation failed") {
                    statusCode = http.StatusBadRequest
                }
                writeJSONError(w, txErr.Error(), statusCode)
                return
            }
            newVehicleDB.STNKImageID = stnkImageID
            stnkImageStoragePath = tempPath
        } else if errSTNK != http.ErrMissingFile {
            txErr = fmt.Errorf("error retrieving stnk_image: %w", errSTNK)
            writeJSONError(w, txErr.Error(), http.StatusBadRequest)
            return
        }

        kkFile, kkHandler, errKK := r.FormFile("kk_image")
        if errKK == nil {
            defer kkFile.Close()
            kkImageID, tempPath, errUpload := s.uploadAndCreateImageRecord(r.Context(), tx, kkFile, kkHandler, "kk_image", maxFileSizeVehicle)
            if errUpload != nil {
                txErr = fmt.Errorf("failed to process kk_image: %w", errUpload)
                statusCode := http.StatusInternalServerError
                errMsg := strings.ToLower(errUpload.Error())
                if strings.Contains(errMsg, "file is empty") || strings.Contains(errMsg, "exceeds") || strings.Contains(errMsg, "invalid type") || strings.Contains(errMsg, "mime type validation failed") {
                    statusCode = http.StatusBadRequest
                }
                writeJSONError(w, txErr.Error(), statusCode)
                return
            }
            newVehicleDB.KKImageID = kkImageID
            kkImageStoragePath = tempPath
        } else if errKK != http.ErrMissingFile {
            txErr = fmt.Errorf("error retrieving kk_image: %w", errKK)
            writeJSONError(w, txErr.Error(), http.StatusBadRequest)
            return
        }

        txErr = database.CreateVehicleTx(r.Context(), tx, &newVehicleDB)
        if txErr != nil {
            fmt.Printf("ERROR: Failed to create vehicle record in transaction: %v\n", txErr)
            writeJSONError(w, "Failed to create vehicle record: "+txErr.Error(), http.StatusInternalServerError)
            return
        }

        txErr = tx.Commit()
        if txErr != nil {
            fmt.Printf("ERROR: Failed to commit database transaction for vehicle creation: %v\n", txErr)
            writeJSONError(w, "Failed to commit database transaction", http.StatusInternalServerError)
            return
        }
        createdVehicle, errGet := database.GetVehicleByID(r.Context(), s.db.Get(), newVehicleDB.VehicleID)
        if errGet != nil {
            fmt.Printf("WARN: Vehicle created (ID: %d), but failed to retrieve for full response: %v\n", newVehicleDB.VehicleID, errGet)
            response := s.toVehicleResponse(r.Context(), s.db.Get(), &newVehicleDB)
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusCreated)
            json.NewEncoder(w).Encode(response)
            return
        }

        response := s.toVehicleResponse(r.Context(), s.db.Get(), createdVehicle)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(response)
    }
}

func (s *Server) uploadAndCreateImageRecord(ctx context.Context, tx *sql.Tx, file multipart.File, handler *multipart.FileHeader, formFieldName string, maxFileSize int64) (sql.NullInt64, string, error) {
    if handler.Size == 0 {
        return sql.NullInt64{}, "", fmt.Errorf("file for %s is empty", formFieldName)
    }
    if handler.Size > maxFileSize {
        return sql.NullInt64{}, "", fmt.Errorf("%s file size (%d bytes) exceeds %dMB limit", formFieldName, handler.Size, maxFileSize/(1024*1024))
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
        os.Remove(storagePath)
        return sql.NullInt64{}, storagePath, fmt.Errorf("failed to reset file pointer before copy for %s: %w", formFieldName, errSeek)
    }

    bytesCopied, err := io.Copy(dst, file)
    if err != nil {
        os.Remove(storagePath)
        return sql.NullInt64{}, storagePath, fmt.Errorf("failed to copy %s file content to '%s': %w", formFieldName, storagePath, err)
    }

    imgRecord := database.Image{
        StoragePath:      filepath.ToSlash(storagePath),
        FilenameOriginal: originalFilename,
        MimeType:         validatedMimeType,
        SizeBytes:        bytesCopied,
    }
    if err := database.CreateImageTx(ctx, tx, &imgRecord); err != nil {
        os.Remove(storagePath)
        return sql.NullInt64{}, storagePath, fmt.Errorf("failed to save %s image metadata to DB: %w", formFieldName, err)
    }

    return sql.NullInt64{Int64: imgRecord.ImageID, Valid: true}, storagePath, nil
}

func (s *Server) handleGetVehicle() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        idStr, idExists := vars["id"]
        db := s.db.Get()

        if idExists && idStr != "" {
            id, err := strconv.ParseInt(idStr, 10, 64)
            if err != nil {
                writeJSONError(w, "invalid vehicle_id format", http.StatusBadRequest)
                return
            }
            v, err := database.GetVehicleByID(r.Context(), db, id)
            if err != nil {
                if errors.Is(err, sql.ErrNoRows) || err.Error() == "vehicle not found" {
                    writeJSONError(w, "Vehicle not found", http.StatusNotFound)
                } else {
                    fmt.Printf("ERROR: Failed to get vehicle by ID %d: %v\n", id, err)
                    writeJSONError(w, "Failed to retrieve vehicle", http.StatusInternalServerError)
                }
                return
            }
            response := s.toVehicleResponse(r.Context(), db, v)
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(response)
            return
        }

        vehiclesDB, err := database.ListVehicles(r.Context(), db)
        if err != nil {
            fmt.Printf("ERROR: Failed to list vehicles: %v\n", err)
            writeJSONError(w, "Failed to retrieve vehicles", http.StatusInternalServerError)
            return
        }
        responseList := make([]VehicleResponse, 0, len(vehiclesDB))
        for i := range vehiclesDB {
            responseList = append(responseList, s.toVehicleResponse(r.Context(), db, &vehiclesDB[i]))
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(responseList)
    }
}

func (s *Server) handleGetVehicleByPlate() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        plate := mux.Vars(r)["plate_number"]
        if plate == "" {
            writeJSONError(w, "plate_number is required in path", http.StatusBadRequest)
            return
        }
        db := s.db.Get()
        v, err := database.GetVehicleByPlate(r.Context(), db, plate)
        if err != nil {
            if errors.Is(err, sql.ErrNoRows) || err.Error() == "vehicle not found" {
                writeJSONError(w, "Vehicle not found for plate: "+plate, http.StatusNotFound)
            } else {
                fmt.Printf("ERROR: Failed to get vehicle by plate %s: %v\n", plate, err)
                writeJSONError(w, "Failed to retrieve vehicle by plate", http.StatusInternalServerError)
            }
            return
        }
        response := s.toVehicleResponse(r.Context(), db, v)
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(response)
    }
}

func (s *Server) handleGetUserVehicles() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userIDFromCtx, ok := r.Context().Value(middleware.UserIDContextKey).(string)
        if !ok || userIDFromCtx == "" {
            writeJSONError(w, "Unauthorized: User ID not found in context.", http.StatusUnauthorized)
            return
        }

        db := s.db.Get() 
        vehiclesDB, err := database.ListVehiclesByUserID(r.Context(), db, userIDFromCtx)
        if err != nil {
            fmt.Printf("ERROR: Failed to list vehicles for user %s: %v\n", userIDFromCtx, err)
            writeJSONError(w, "Failed to retrieve user's vehicles", http.StatusInternalServerError)
            return
        }

        responseList := make([]VehicleResponse, 0, len(vehiclesDB))
        for i := range vehiclesDB {
            responseList = append(responseList, s.toVehicleResponse(r.Context(), db, &vehiclesDB[i]))
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(responseList)
    }
}

func (s *Server) handleUpdateVehicle() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        idStr := vars["id"]
        id, err := strconv.ParseInt(idStr, 10, 64)
        if err != nil {
            writeJSONError(w, "invalid vehicle_id format", http.StatusBadRequest)
            return
        }

        maxTotalSize := extraFormDataSizeVehicle + 2*maxFileSizeVehicle
        if err := r.ParseMultipartForm(int64(maxTotalSize)); err != nil {
            writeJSONError(w, "Request too large or invalid multipart form: "+err.Error(), http.StatusBadRequest)
            return
        }

        db := s.db.Get()
        existingVehicle, err := database.GetVehicleByID(r.Context(), db, id)
        if err != nil {
            if errors.Is(err, sql.ErrNoRows) {
                writeJSONError(w, "Vehicle not found", http.StatusNotFound)
            } else {
                writeJSONError(w, "Failed to retrieve existing vehicle", http.StatusInternalServerError)
            }
            return
        }

        requestingUserID, _ := r.Context().Value(middleware.UserIDContextKey).(string)
        isAdmin, _ := r.Context().Value(middleware.AdminStatusContextKey).(bool)
        if !isAdmin && existingVehicle.UserID != requestingUserID {
            writeJSONError(w, "Forbidden: You can only update your own vehicles.", http.StatusForbidden)
            return
        }

        updates := make(map[string]interface{})

        if val, ok := r.Form["vehicle_name"]; ok {
            updates["vehicle_name"] = val[0]
        }
        if val, ok := r.Form["color"]; ok {
            updates["color"] = val[0]
        }
        if val, ok := r.Form["plate_number"]; ok {
            updates["plate_number"] = val[0]
        }
        if val, ok := r.Form["ownership"]; ok {
            ownershipStr := val[0]
            if ownershipStr == string(database.OwnershipPribadi) || ownershipStr == string(database.OwnershipKeluarga) {
                updates["ownership"] = ownershipStr
            } else if ownershipStr == "" {
                updates["ownership"] = nil 
            } else {
                writeJSONError(w, fmt.Sprintf("Invalid ownership value. Must be '%s', '%s', or empty", database.OwnershipPribadi, database.OwnershipKeluarga), http.StatusBadRequest)
                return
            }
        }

        tx, err := db.BeginTx(r.Context(), nil)
        if err != nil {
            writeJSONError(w, "Failed to start database transaction", http.StatusInternalServerError)
            return
        }
        var txErr error
        var newStnkImageStoragePath, newKkImageStoragePath string
        oldStnkImageID := existingVehicle.STNKImageID
        oldKkImageID := existingVehicle.KKImageID

        defer func() {
            if p := recover(); p != nil {
                tx.Rollback()
                if newStnkImageStoragePath != "" { os.Remove(newStnkImageStoragePath) }
                if newKkImageStoragePath != "" { os.Remove(newKkImageStoragePath) }
                panic(p)
            } else if txErr != nil {
                tx.Rollback()
                if newStnkImageStoragePath != "" { os.Remove(newStnkImageStoragePath) }
                if newKkImageStoragePath != "" { os.Remove(newKkImageStoragePath) }
            }
        }()

        stnkFile, stnkHandler, errSTNK := r.FormFile("stnk_image")
        if errSTNK == nil {
            defer stnkFile.Close()
            stnkImageID, tempPath, errUpload := s.uploadAndCreateImageRecord(r.Context(), tx, stnkFile, stnkHandler, "stnk_image", maxFileSizeVehicle)
            if errUpload != nil {
                txErr = fmt.Errorf("failed to process new stnk_image: %w", errUpload)
                writeJSONError(w, txErr.Error(), http.StatusBadRequest)
                return
            }
            updates["stnk_image_id"] = stnkImageID.Int64
            newStnkImageStoragePath = tempPath
        } else if errSTNK != http.ErrMissingFile {
            txErr = fmt.Errorf("error retrieving stnk_image for update: %w", errSTNK)
            writeJSONError(w, txErr.Error(), http.StatusBadRequest)
            return
        }

        kkFile, kkHandler, errKK := r.FormFile("kk_image")
        if errKK == nil {
            defer kkFile.Close()
            kkImageID, tempPath, errUpload := s.uploadAndCreateImageRecord(r.Context(), tx, kkFile, kkHandler, "kk_image", maxFileSizeVehicle)
            if errUpload != nil {
                txErr = fmt.Errorf("failed to process new kk_image: %w", errUpload)
                writeJSONError(w, txErr.Error(), http.StatusBadRequest)
                return
            }
            updates["kk_image_id"] = kkImageID.Int64
            newKkImageStoragePath = tempPath
        } else if errKK != http.ErrMissingFile {
            txErr = fmt.Errorf("error retrieving kk_image for update: %w", errKK)
            writeJSONError(w, txErr.Error(), http.StatusBadRequest)
            return
        }

        if len(updates) == 0 {
            writeJSONError(w, "No changes provided for update", http.StatusBadRequest)
            return
        }

        txErr = database.UpdateVehicleTx(r.Context(), tx, id, updates)
        if txErr != nil {
            if errors.Is(txErr, sql.ErrNoRows) {
                writeJSONError(w, "Vehicle not found or no effective changes made", http.StatusNotFound)
            } else {
                writeJSONError(w, "Failed to update vehicle: "+txErr.Error(), http.StatusInternalServerError)
            }
            return
        }

        if _, ok := updates["stnk_image_id"]; ok && oldStnkImageID.Valid {
             if errDel := s.deleteImageRecordAndFile(r.Context(), tx, oldStnkImageID.Int64); errDel != nil {
                fmt.Printf("WARN: Failed to delete old STNK image (ID: %d) after update: %v\n", oldStnkImageID.Int64, errDel)
            }
        }
        if _, ok := updates["kk_image_id"]; ok && oldKkImageID.Valid {
            if errDel := s.deleteImageRecordAndFile(r.Context(), tx, oldKkImageID.Int64); errDel != nil {
                fmt.Printf("WARN: Failed to delete old KK image (ID: %d) after update: %v\n", oldKkImageID.Int64, errDel)
            }
        }

        txErr = tx.Commit()
        if txErr != nil {
            writeJSONError(w, "Failed to commit database transaction", http.StatusInternalServerError)
            return
        }

        updatedVehicleDB, errGet := database.GetVehicleByID(r.Context(), db, id)
        if errGet != nil {
            writeJSONError(w, "Vehicle updated successfully, but failed to retrieve updated data.", http.StatusInternalServerError)
            return
        }
        response := s.toVehicleResponse(r.Context(), db, updatedVehicleDB)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(response)
    }
}

func (s *Server) handleDeleteVehicle() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        idStr := vars["id"]
        id, err := strconv.ParseInt(idStr, 10, 64)
        if err != nil {
            writeJSONError(w, "invalid vehicle_id format", http.StatusBadRequest)
            return
        }

        db := s.db.Get()
        vehicleToDelete, err := database.GetVehicleByID(r.Context(), db, id)
        if err != nil {
            if errors.Is(err, sql.ErrNoRows) || err.Error() == "vehicle not found" {
                writeJSONError(w, "Vehicle not found", http.StatusNotFound)
            } else {
                writeJSONError(w, "Failed to retrieve vehicle before deletion", http.StatusInternalServerError)
            }
            return
        }

        requestingUserID, _ := r.Context().Value(middleware.UserIDContextKey).(string)
        isAdmin, _ := r.Context().Value(middleware.AdminStatusContextKey).(bool)
        if !isAdmin && vehicleToDelete.UserID != requestingUserID {
            writeJSONError(w, "Forbidden: You can only delete your own vehicles.", http.StatusForbidden)
            return
        }

        tx, err := db.BeginTx(r.Context(), nil)
        if err != nil {
            writeJSONError(w, "Failed to start database transaction", http.StatusInternalServerError)
            return
        }
        var txErr error
        defer func() {
            if p := recover(); p != nil {
                tx.Rollback()
                panic(p)
            } else if txErr != nil {
                tx.Rollback()
            }
        }()

        if vehicleToDelete.STNKImageID.Valid {
            txErr = s.deleteImageRecordAndFile(r.Context(), tx, vehicleToDelete.STNKImageID.Int64)
            if txErr != nil {
                fmt.Printf("WARN: Failed to delete STNK image (ID: %d) during vehicle deletion: %v\n", vehicleToDelete.STNKImageID.Int64, txErr)
                // Tidak menggagalkan commit utama, hanya warning. Atau bisa juga digagalkan.
                // writeJSONError(w, "Failed to delete associated STNK image: "+txErr.Error(), http.StatusInternalServerError)
                // return
                txErr = nil 
            }
        }

        if vehicleToDelete.KKImageID.Valid {
            txErr = s.deleteImageRecordAndFile(r.Context(), tx, vehicleToDelete.KKImageID.Int64)
            if txErr != nil {
                fmt.Printf("WARN: Failed to delete KK image (ID: %d) during vehicle deletion: %v\n", vehicleToDelete.KKImageID.Int64, txErr)
                // writeJSONError(w, "Failed to delete associated KK image: "+txErr.Error(), http.StatusInternalServerError)
                // return
                txErr = nil
            }
        }

        txErr = database.DeleteVehicleTx(r.Context(), tx, id)
        if txErr != nil {
            if errors.Is(txErr, sql.ErrNoRows) || strings.Contains(txErr.Error(), "no vehicle record deleted") {
                writeJSONError(w, "Vehicle not found", http.StatusNotFound)
            } else {
                writeJSONError(w, "Failed to delete vehicle: "+txErr.Error(), http.StatusInternalServerError)
            }
            return
        }

        txErr = tx.Commit()
        if txErr != nil {
            writeJSONError(w, "Failed to commit database transaction", http.StatusInternalServerError)
            return
        }

        w.WriteHeader(http.StatusNoContent)
    }
}

func (s *Server) deleteImageRecordAndFile(ctx context.Context, tx *sql.Tx, imageID int64) error {
    imagePath, err := database.GetImageStoragePath(ctx, tx, imageID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil
        }
        return fmt.Errorf("failed to get image path for ID %d: %w", imageID, err)
    }

    err = database.DeleteImageTx(ctx, tx, imageID)
    if err != nil {
        return fmt.Errorf("failed to delete image record for ID %d: %w", imageID, err)
    }

    if imagePath != "" {
        // Pastikan imagePath adalah path yang aman dan relatif terhadap root upload Anda.
        // Jika imagePath adalah "uploads/images/file.jpg" dan imageUploadPath adalah "./"
        // maka fullDiskPath harusnya imagePath itu sendiri.
        // Jika imageUploadPath adalah "./uploads" dan imagePath adalah "images/file.jpg"
        // maka fullDiskPath = filepath.Join(imageUploadPath, imagePath)
        // Asumsi imagePath yang disimpan di DB sudah benar dan relatif dari root proyek atau direktori yang dilayani.
        // Untuk keamanan, pastikan path ini tidak bisa dimanipulasi untuk menghapus file di luar direktori upload.
        // fullDiskPath := filepath.Clean(imagePath) // Membersihkan path
        // if !strings.HasPrefix(fullDiskPath, filepath.Clean(imageUploadPath)) {
        // 	return fmt.Errorf("invalid image path, potential traversal: %s", imagePath)
        // }
        fullDiskPath := imagePath 

        if errOs := os.Remove(fullDiskPath); errOs != nil && !os.IsNotExist(errOs) {
            // Jika file fisik gagal dihapus setelah record DB berhasil dihapus, ini adalah masalah.
            // Anda bisa memilih untuk mengembalikan error ini yang akan menyebabkan rollback jika ini bagian dari transaksi yang lebih besar,
            // atau hanya log sebagai warning.
            return fmt.Errorf("failed to delete image file %s after DB record deletion: %w", fullDiskPath, errOs)
        }
    }
    return nil
}

func (s *Server) RegisterVehicleRoutes(r *mux.Router) {
    adminOnlyMiddleware := middleware.AdminOnlyMiddleware()
    r.Handle("/vehicles", adminOnlyMiddleware(s.handleGetVehicle())).Methods("GET")
    r.Handle("/vehicles/plate/{plate_number}", adminOnlyMiddleware( s.handleGetVehicleByPlate())).Methods("GET")
    r.Handle("/vehicles/{id:[0-9]+}", adminOnlyMiddleware(s.handleGetVehicle())).Methods("GET")
    
    r.HandleFunc("/vehicles", s.handleCreateVehicle()).Methods("POST")
	r.HandleFunc("/vehicles/my", s.handleGetUserVehicles()).Methods("GET")
    r.HandleFunc("/vehicles/{id:[0-9]+}", s.handleUpdateVehicle()).Methods("PUT")
    r.HandleFunc("/vehicles/{id:[0-9]+}", s.handleDeleteVehicle()).Methods("DELETE")
}



