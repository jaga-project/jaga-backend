package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io" // Ditambahkan
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings" // Ditambahkan

	"github.com/google/uuid" // Ditambahkan
	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
	"github.com/jaga-project/jaga-backend/internal/middleware"
)

const maxFileSizeVehicle = 5 * 1024 * 1024 // 5 MB per file (STNK, KK)
const extraFormDataSizeVehicle = 1 * 1024 * 1024 // 1MB untuk field teks lainnya

func (s *Server) handleCreateVehicle() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        maxTotalSize := extraFormDataSizeVehicle + 2*maxFileSizeVehicle // Untuk 2 file (STNK, KK)
        if err := r.ParseMultipartForm(int64(maxTotalSize)); err != nil { // Perubahan di sini
            http.Error(w, "Request too large or invalid multipart form: "+err.Error(), http.StatusBadRequest)
            return
        }

		var newVehicle database.Vehicle
		var err error

		userIDFromCtx, ok := r.Context().Value(middleware.UserIDContextKey).(string)
		if !ok || userIDFromCtx == "" {
			http.Error(w, "Unauthorized: User ID not found in context.", http.StatusUnauthorized)
			return
		}

		newVehicle.VehicleName = r.FormValue("vehicle_name")
		newVehicle.Color = r.FormValue("color")
		newVehicle.UserID = userIDFromCtx
		newVehicle.PlateNumber = r.FormValue("plate_number")
		ownershipStr := r.FormValue("ownership")

		if newVehicle.VehicleName == "" || newVehicle.PlateNumber == "" {
			http.Error(w, "vehicle_name and plate_number are required", http.StatusBadRequest)
			return
		}

		// Validasi Ownership
		if ownershipStr == string(database.OwnershipPribadi) || ownershipStr == string(database.OwnershipKeluarga) {
			newVehicle.Ownership = database.OwnershipType(ownershipStr)
		} else {
			http.Error(w, fmt.Sprintf("Invalid ownership value. Must be '%s' or '%s'", database.OwnershipPribadi, database.OwnershipKeluarga), http.StatusBadRequest)
			return
		}

		tx, err := s.db.Get().BeginTx(r.Context(), nil)
		if err != nil {
			fmt.Printf("ERROR: Failed to start database transaction for vehicle creation: %v\n", err)
			http.Error(w, "Failed to start database transaction", http.StatusInternalServerError)
			return
		}
		var txErr error
		var stnkImageStoragePath string
		var kkImageStoragePath string

		defer func() {
			if p := recover(); p != nil {
				tx.Rollback()
				if stnkImageStoragePath != "" {
					os.Remove(stnkImageStoragePath)
				}
				if kkImageStoragePath != "" {
					os.Remove(kkImageStoragePath)
				}
				panic(p)
			} else if txErr != nil {
				tx.Rollback()
				if stnkImageStoragePath != "" {
					os.Remove(stnkImageStoragePath)
				}
				if kkImageStoragePath != "" {
					os.Remove(kkImageStoragePath)
				}
			}
		}()

		// Proses upload gambar STNK (opsional)
		stnkFile, stnkHandler, errSTNK := r.FormFile("stnk_image")
		if errSTNK == nil {
			defer stnkFile.Close()
			stnkImageID, tempPath, errUpload := s.uploadAndCreateImageRecord(r.Context(), tx, stnkFile, stnkHandler, "stnk_image", maxFileSizeVehicle)
			if errUpload != nil {
				txErr = fmt.Errorf("failed to process stnk_image: %w", errUpload)
				// Gunakan determineImageUploadErrorStatusCode jika ada, atau default ke Bad Request
				http.Error(w, txErr.Error(), determineImageUploadErrorStatusCode(errUpload))
				return
			}
			newVehicle.STNKImageID = stnkImageID
			stnkImageStoragePath = tempPath
		} else if errSTNK != http.ErrMissingFile {
			txErr = fmt.Errorf("error retrieving stnk_image: %w", errSTNK)
			http.Error(w, txErr.Error(), http.StatusBadRequest)
			return
		}

		// Proses upload gambar KK (opsional)
		kkFile, kkHandler, errKK := r.FormFile("kk_image")
		if errKK == nil {
			defer kkFile.Close()
			kkImageID, tempPath, errUpload := s.uploadAndCreateImageRecord(r.Context(), tx, kkFile, kkHandler, "kk_image", maxFileSizeVehicle)
			if errUpload != nil {
				txErr = fmt.Errorf("failed to process kk_image: %w", errUpload)
				http.Error(w, txErr.Error(), determineImageUploadErrorStatusCode(errUpload))
				return
			}
			newVehicle.KKImageID = kkImageID
			kkImageStoragePath = tempPath
		} else if errKK != http.ErrMissingFile {
			txErr = fmt.Errorf("error retrieving kk_image: %w", errKK)
			http.Error(w, txErr.Error(), http.StatusBadRequest)
			return
		}

		txErr = database.CreateVehicleTx(r.Context(), tx, &newVehicle)
		if txErr != nil {
			fmt.Printf("ERROR: Failed to create vehicle record in transaction: %v\n", txErr)
			http.Error(w, "Failed to create vehicle record: "+txErr.Error(), http.StatusInternalServerError)
			return
		}

		txErr = tx.Commit()
		if txErr != nil {
			fmt.Printf("ERROR: Failed to commit database transaction for vehicle creation: %v\n", txErr)
			http.Error(w, "Failed to commit database transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(newVehicle)
	}
}

// uploadAndCreateImageRecord menangani validasi, penyimpanan file, dan pembuatan record di tabel 'images'.
// Fungsi ini sekarang lebih lengkap dan konsisten dengan implementasi di handler lain.
func (s *Server) uploadAndCreateImageRecord(ctx context.Context, tx *sql.Tx, file multipart.File, handler *multipart.FileHeader, formFieldName string, maxFileSize int64) (sql.NullInt64, string, error) {
	if handler.Size == 0 {
		return sql.NullInt64{}, "", fmt.Errorf("file for %s is empty", formFieldName)
	}
	if handler.Size > maxFileSize {
		return sql.NullInt64{}, "", fmt.Errorf("%s file size (%d bytes) exceeds %dMB limit", formFieldName, handler.Size, maxFileSize/(1024*1024))
	}

	// Panggil fungsi validasi MIME terpusat
	// Asumsi ValidateMimeType dan DefaultAllowedMimeTypes ada di package server (misalnya, dari image.go)
	validatedMimeType, errMime := ValidateMimeType(file, handler, DefaultAllowedMimeTypes)
	if errMime != nil {
		return sql.NullInt64{}, "", fmt.Errorf("MIME type validation failed for %s: %w", formFieldName, errMime)
	}

	originalFilename := handler.Filename
	fileExtension := filepath.Ext(originalFilename)
	uniqueFilename := fmt.Sprintf("%s%s", uuid.New().String(), fileExtension)

	// imageUploadPath diasumsikan sebagai konstanta yang dapat diakses dari package server
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
		os.Remove(storagePath)
		return sql.NullInt64{}, storagePath, fmt.Errorf("failed to reset file pointer before copy for %s: %w", formFieldName, errSeek)
	}

	bytesCopied, err := io.Copy(dst, file)
	if err != nil {
		os.Remove(storagePath)
		return sql.NullInt64{}, storagePath, fmt.Errorf("failed to copy %s file content to '%s': %w", formFieldName,storagePath,err)
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
		w.Header().Set("Content-Type", "application/json")
		vars := mux.Vars(r)
		idStr, idExists := vars["id"]

		if idExists && idStr != "" {
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				http.Error(w, "invalid vehicle_id format", http.StatusBadRequest)
				return
			}
			v, err := database.GetVehicleByID(r.Context(), s.db.Get(), id)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) || err.Error() == "vehicle not found" { // Menggunakan errors.Is lebih baik
					http.Error(w, "Vehicle not found", http.StatusNotFound)
				} else {
					fmt.Printf("ERROR: Failed to get vehicle by ID %d: %v\n", id, err)
					http.Error(w, "Failed to retrieve vehicle", http.StatusInternalServerError)
				}
				return
			}
			json.NewEncoder(w).Encode(v)
			return
		}

		vehicles, err := database.ListVehicles(r.Context(), s.db.Get())
		if err != nil {
			fmt.Printf("ERROR: Failed to list vehicles: %v\n", err)
			http.Error(w, "Failed to retrieve vehicles", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(vehicles)
	}
}

func (s *Server) handleGetVehicleByPlate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		plate := mux.Vars(r)["plate_number"]
		if plate == "" {
			http.Error(w, "plate_number is required in path", http.StatusBadRequest)
			return
		}
		v, err := database.GetVehicleByPlate(r.Context(), s.db.Get(), plate)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || err.Error() == "vehicle not found" {
				http.Error(w, "Vehicle not found for plate: "+plate, http.StatusNotFound)
			} else {
				fmt.Printf("ERROR: Failed to get vehicle by plate %s: %v\n", plate, err)
				http.Error(w, "Failed to retrieve vehicle by plate", http.StatusInternalServerError)
			}
			return
		}
		json.NewEncoder(w).Encode(v)
	}
}

func (s *Server) handleUpdateVehicle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		idStr := vars["id"]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid vehicle_id format", http.StatusBadRequest)
			return
		}

		// Untuk update dengan gambar, kita akan menggunakan multipart/form-data
		maxTotalSize := extraFormDataSizeVehicle + 2*maxFileSizeVehicle // Sama seperti create
		if err := r.ParseMultipartForm(int64(maxTotalSize)); err != nil {
			http.Error(w, "Request too large or invalid multipart form: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Ambil vehicle yang ada untuk perbandingan dan untuk mendapatkan UserID yang benar
		existingVehicle, err := database.GetVehicleByID(r.Context(), s.db.Get(), id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || err.Error() == "vehicle not found" {
				http.Error(w, "Vehicle not found", http.StatusNotFound)
			} else {
				http.Error(w, "Failed to retrieve existing vehicle", http.StatusInternalServerError)
			}
			return
		}

		// Pastikan user yang melakukan update adalah pemilik vehicle atau admin
		requestingUserID, _ := r.Context().Value(middleware.UserIDContextKey).(string)
		isAdmin, _ := r.Context().Value(middleware.AdminStatusContextKey).(bool)
		if !isAdmin && existingVehicle.UserID != requestingUserID {
			http.Error(w, "Forbidden: You can only update your own vehicles.", http.StatusForbidden)
			return
		}

		vehicleToUpdate := *existingVehicle // Salin data yang ada
		changed := false

		// Update field teks jika ada di form
		if val := r.FormValue("vehicle_name"); val != "" {
			vehicleToUpdate.VehicleName = val
			changed = true
		}
		if val := r.FormValue("color"); val != "" {
			vehicleToUpdate.Color = val
			changed = true
		}
		if val := r.FormValue("plate_number"); val != "" {
			vehicleToUpdate.PlateNumber = val
			changed = true
		}
		if ownershipStr := r.FormValue("ownership"); ownershipStr != "" {
			if ownershipStr == string(database.OwnershipPribadi) || ownershipStr == string(database.OwnershipKeluarga) {
				vehicleToUpdate.Ownership = database.OwnershipType(ownershipStr)
				changed = true
			} else {
				http.Error(w, fmt.Sprintf("Invalid ownership value. Must be '%s' or '%s'", database.OwnershipPribadi, database.OwnershipKeluarga), http.StatusBadRequest)
				return
			}
		}

		tx, err := s.db.Get().BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, "Failed to start database transaction", http.StatusInternalServerError)
			return
		}
		var txErr error
		var newStnkImageStoragePath, newKkImageStoragePath string
		// TODO: Simpan path gambar lama untuk dihapus jika gambar baru diupload dan transaksi berhasil.

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

		// Proses STNK Image jika ada file baru
		stnkFile, stnkHandler, errSTNK := r.FormFile("stnk_image")
		if errSTNK == nil {
			defer stnkFile.Close()
			// TODO: Hapus file STNK lama dan record image lama jika ada
			stnkImageID, tempPath, errUpload := s.uploadAndCreateImageRecord(r.Context(), tx, stnkFile, stnkHandler, "stnk_image", maxFileSizeVehicle)
			if errUpload != nil {
				txErr = fmt.Errorf("failed to process new stnk_image: %w", errUpload)
				http.Error(w, txErr.Error(), determineImageUploadErrorStatusCode(errUpload))
				return
			}
			vehicleToUpdate.STNKImageID = stnkImageID
			newStnkImageStoragePath = tempPath
			changed = true
		} else if errSTNK != http.ErrMissingFile {
			txErr = fmt.Errorf("error retrieving stnk_image for update: %w", errSTNK)
			http.Error(w, txErr.Error(), http.StatusBadRequest)
			return
		}

		// Proses KK Image jika ada file baru
		kkFile, kkHandler, errKK := r.FormFile("kk_image")
		if errKK == nil {
			defer kkFile.Close()
			// TODO: Hapus file KK lama dan record image lama jika ada
			kkImageID, tempPath, errUpload := s.uploadAndCreateImageRecord(r.Context(), tx, kkFile, kkHandler, "kk_image", maxFileSizeVehicle)
			if errUpload != nil {
				txErr = fmt.Errorf("failed to process new kk_image: %w", errUpload)
				http.Error(w, txErr.Error(), determineImageUploadErrorStatusCode(errUpload))
				return
			}
			vehicleToUpdate.KKImageID = kkImageID
			newKkImageStoragePath = tempPath
			changed = true
		} else if errKK != http.ErrMissingFile {
			txErr = fmt.Errorf("error retrieving kk_image for update: %w", errKK)
			http.Error(w, txErr.Error(), http.StatusBadRequest)
			return
		}

		if !changed {
			http.Error(w, "No changes provided for update", http.StatusBadRequest)
			return // Tidak perlu rollback karena tidak ada operasi DB
		}

		txErr = database.UpdateVehicleTx(r.Context(), tx, id, &vehicleToUpdate) // Asumsi UpdateVehicle menerima *sql.Tx
		if txErr != nil {
			if errors.Is(txErr, sql.ErrNoRows) || strings.Contains(txErr.Error(), "no vehicle record updated") {
				http.Error(w, "Vehicle not found or no effective changes made", http.StatusNotFound)
			} else {
				http.Error(w, "Failed to update vehicle: "+txErr.Error(), http.StatusInternalServerError)
			}
			return
		}

		txErr = tx.Commit()
		if txErr != nil {
			http.Error(w, "Failed to commit database transaction", http.StatusInternalServerError)
			return
		}

		// TODO: Jika gambar lama diganti, hapus file fisik dan record image lama di sini setelah commit berhasil.

		updatedVehicle, err := database.GetVehicleByID(r.Context(), s.db.Get(), id)
		if err != nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"message": "Vehicle updated successfully, but failed to retrieve updated data."})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(updatedVehicle)
	}
}

func (s *Server) handleDeleteVehicle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		idStr := vars["id"]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid vehicle_id format", http.StatusBadRequest)
			return
		}

		// Ambil vehicle yang akan dihapus untuk mendapatkan ID gambar
		vehicleToDelete, err := database.GetVehicleByID(r.Context(), s.db.Get(), id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || err.Error() == "vehicle not found" {
				http.Error(w, "Vehicle not found", http.StatusNotFound)
			} else {
				http.Error(w, "Failed to retrieve vehicle before deletion", http.StatusInternalServerError)
			}
			return
		}

		// Pastikan user yang melakukan delete adalah pemilik vehicle atau admin
		requestingUserID, _ := r.Context().Value(middleware.UserIDContextKey).(string)
		isAdmin, _ := r.Context().Value(middleware.AdminStatusContextKey).(bool)
		if !isAdmin && vehicleToDelete.UserID != requestingUserID {
			http.Error(w, "Forbidden: You can only delete your own vehicles.", http.StatusForbidden)
			return
		}

		tx, err := s.db.Get().BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, "Failed to start database transaction", http.StatusInternalServerError)
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

		// Hapus record vehicle
		txErr = database.DeleteVehicleTx(r.Context(), tx, id) // Asumsi ada DeleteVehicleTx
		if txErr != nil {
			if errors.Is(txErr, sql.ErrNoRows) || strings.Contains(txErr.Error(), "no vehicle record deleted") {
				http.Error(w, "Vehicle not found", http.StatusNotFound)
			} else {
				http.Error(w, "Failed to delete vehicle: "+txErr.Error(), http.StatusInternalServerError)
			}
			return
		}

		// Hapus gambar STNK jika ada
		if vehicleToDelete.STNKImageID.Valid {
			txErr = s.deleteImageRecordAndFile(r.Context(), tx, vehicleToDelete.STNKImageID.Int64)
			if txErr != nil {
				// Log error tapi lanjutkan, mungkin hanya warning
				fmt.Printf("WARN: Failed to delete STNK image (ID: %d) during vehicle deletion: %v\n", vehicleToDelete.STNKImageID.Int64, txErr)
				txErr = nil // Reset error agar tidak rollback transaksi utama jika hanya image delete yg gagal
			}
		}

		// Hapus gambar KK jika ada
		if vehicleToDelete.KKImageID.Valid {
			txErr = s.deleteImageRecordAndFile(r.Context(), tx, vehicleToDelete.KKImageID.Int64)
			if txErr != nil {
				fmt.Printf("WARN: Failed to delete KK image (ID: %d) during vehicle deletion: %v\n", vehicleToDelete.KKImageID.Int64, txErr)
				txErr = nil
			}
		}

		txErr = tx.Commit()
		if txErr != nil {
			http.Error(w, "Failed to commit database transaction", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// deleteImageRecordAndFile adalah helper untuk menghapus record gambar dari DB dan file dari disk.
func (s *Server) deleteImageRecordAndFile(ctx context.Context, tx *sql.Tx, imageID int64) error {
    imagePath, err := database.GetImageStoragePath(ctx, tx, imageID) // Perlu fungsi ini di database/image.go
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil // Record gambar tidak ada, anggap sudah terhapus
        }
        return fmt.Errorf("failed to get image path for ID %d: %w", imageID, err)
    }

    err = database.DeleteImageTx(ctx, tx, imageID) // Perlu fungsi ini di database/image.go
    if err != nil {
        return fmt.Errorf("failed to delete image record for ID %d: %w", imageID, err)
    }

    if imagePath != "" {
        // Hati-hati dengan path traversal jika imagePath bisa dikontrol user.
        // Sebaiknya pastikan imagePath selalu relatif terhadap direktori upload yang aman.
        // Untuk keamanan tambahan, filepath.Clean dan validasi base path bisa digunakan.
        // fullDiskPath := filepath.Join(imageUploadPath, filepath.Base(imagePath)) // Lebih aman
        fullDiskPath := imagePath // Jika imagePath sudah absolut atau relatif dari root yang benar
        if errOs := os.Remove(fullDiskPath); errOs != nil && !os.IsNotExist(errOs) {
            // Log error penghapusan file, tapi jangan gagalkan transaksi utama jika record DB sudah terhapus.
            // Atau, jika penghapusan file kritis, kembalikan error.
            return fmt.Errorf("failed to delete image file %s: %w", fullDiskPath, errOs)
        }
    }
    return nil
}


func (s *Server) RegisterVehicleRoutes(r *mux.Router) {
	r.HandleFunc("/vehicles", s.handleCreateVehicle()).Methods("POST")
	r.HandleFunc("/vehicles", s.handleGetVehicle()).Methods("GET")
	r.HandleFunc("/vehicles/{id:[0-9]+}", s.handleGetVehicle()).Methods("GET")
	r.HandleFunc("/vehicles/plate/{plate_number}", s.handleGetVehicleByPlate()).Methods("GET")
	r.HandleFunc("/vehicles/{id:[0-9]+}", s.handleUpdateVehicle()).Methods("PUT")
	r.HandleFunc("/vehicles/{id:[0-9]+}", s.handleDeleteVehicle()).Methods("DELETE")
}

