package server

import (
	// Ditambahkan untuk database.CreateImageTx
	"database/sql"
	"encoding/json"
	"fmt"
	"io" // Ditambahkan untuk handler di processImageUpload
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	// "strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
)

const maxFileSizeDetected = 5 * 1024 * 1024 // 5 MB

// generateUniqueFilenameLocalDetected dapat tetap spesifik jika diperlukan,
// atau Anda bisa menggunakan generateUniqueFilenameLocal dari image.go jika logikanya sama.
func generateUniqueFilenameLocalDetected(originalFilename string) string {
	// Menggunakan UUID untuk keunikan yang lebih baik, konsisten dengan image.go
	randomUUID := uuid.New().String()
	extension := filepath.Ext(originalFilename)
	// base := strings.TrimSuffix(originalFilename, extension)
	// safeBase := strings.ReplaceAll(strings.ToLower(base), " ", "_")
	// if len(safeBase) > 50 {
	// 	safeBase = safeBase[:50]
	// }
	// return fmt.Sprintf("%s_%d_%s%s", safeBase, time.Now().UnixNano(), randomUUID, extension) // Example if timestamp were used
	return fmt.Sprintf("%s%s", randomUUID, extension) // Lebih sederhana, hanya UUID + ekstensi
}

func processImageUpload(r *http.Request, formFieldName string, tx *sql.Tx) (sql.NullInt64, string, error) {
	file, handler, err := r.FormFile(formFieldName)
	if err != nil {
		if err == http.ErrMissingFile {
			fmt.Printf("DEBUG processImageUpload (detected): File not provided for field '%s'\n", formFieldName)
			return sql.NullInt64{Valid: false}, "", nil // File tidak ada, bukan error aplikasi
		}
		fmt.Printf("ERROR processImageUpload (detected): Error retrieving file for field '%s': %v\n", formFieldName, err)
		return sql.NullInt64{Valid: false}, "", fmt.Errorf("error retrieving %s: %w", formFieldName, err)
	}
	defer file.Close()

	fmt.Printf("DEBUG processImageUpload (detected): Processing file '%s' for field '%s', size: %d, header MIME: %s\n", handler.Filename, formFieldName, handler.Size, handler.Header.Get("Content-Type"))

	if handler.Size == 0 {
		return sql.NullInt64{}, "", fmt.Errorf("file for %s is empty", formFieldName)
	}

	// Validasi ukuran file
	if handler.Size > maxFileSizeDetected {
		return sql.NullInt64{}, "", fmt.Errorf("%s file size (%d bytes) exceeds %dMB limit", formFieldName, handler.Size, maxFileSizeDetected/(1024*1024))
	}

	// Panggil fungsi validasi MIME terpusat
	// DefaultAllowedMimeTypes diasumsikan ada di package server (misalnya, dari image.go)
	validatedMimeType, errMime := ValidateMimeType(file, handler, DefaultAllowedMimeTypes)
	if errMime != nil {
		return sql.NullInt64{}, "", fmt.Errorf("MIME type validation failed for %s: %w", formFieldName, errMime)
	}
	// file pointer sudah di-reset oleh ValidateMimeType jika validasi dari konten,
	// atau tidak berubah jika dari header. Kita akan reset lagi sebelum io.Copy untuk memastikan.

	originalFilename := handler.Filename
	uniqueFilename := generateUniqueFilenameLocalDetected(originalFilename) // Gunakan fungsi yang sesuai
	storageDir := imageUploadPath                                           // imageUploadPath dari image.go
	storagePath := filepath.Join(storageDir, uniqueFilename)

	if err := os.MkdirAll(storageDir, os.ModePerm); err != nil {
		return sql.NullInt64{Valid: false}, "", fmt.Errorf("failed to create upload directory '%s' for %s: %w", storageDir, formFieldName, err)
	}

	dst, err := os.Create(storagePath)
	if err != nil {
		return sql.NullInt64{Valid: false}, storagePath, fmt.Errorf("failed to create destination file '%s' for %s: %w", storagePath, formFieldName, err)
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
		return sql.NullInt64{Valid: false}, storagePath, fmt.Errorf("failed to copy %s file content to '%s': %w", formFieldName, storagePath, err)
	}
	fmt.Printf("DEBUG processImageUpload (detected): Successfully saved %s to %s (%d bytes copied)\n", formFieldName, storagePath, bytesCopied)

	// Simpan metadata gambar ke database
	imgRecord := database.Image{
		StoragePath:      filepath.ToSlash(storagePath), // Simpan dengan forward slashes
		FilenameOriginal: originalFilename,
		MimeType:         validatedMimeType, // Gunakan tipe MIME yang sudah divalidasi
		SizeBytes:        bytesCopied,       // Gunakan bytesCopied
	}

	// Menggunakan r.Context() untuk database.CreateImageTx
	if err := database.CreateImageTx(r.Context(), tx, &imgRecord); err != nil {
		os.Remove(storagePath) // Best effort untuk menghapus file jika insert DB gagal
		return sql.NullInt64{Valid: false}, storagePath, fmt.Errorf("failed to save %s image metadata: %w", formFieldName, err)
	}
	fmt.Printf("DEBUG processImageUpload (detected): Successfully created image record for '%s'. ImageID: %d\n", formFieldName, imgRecord.ImageID)
	return sql.NullInt64{Int64: imgRecord.ImageID, Valid: true}, storagePath, nil
}

func (s *Server) handleCreateDetected() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("DEBUG: handleCreateDetected POST request received") // Log awal
		// Batasi ukuran keseluruhan request body
		if err := r.ParseMultipartForm(20 << 20); err != nil { // 20 MB total limit
			http.Error(w, "Request too large or invalid multipart form: "+err.Error(), http.StatusBadRequest)
			return
		}

		var newDetected database.Detected
		var err error

		// Ambil CameraID
		cameraIDStr := r.FormValue("camera_id")
		if cameraIDStr == "" {
			http.Error(w, "camera_id is required", http.StatusBadRequest)
			return
		}
		newDetected.CameraID, err = strconv.Atoi(cameraIDStr)
		if err != nil {
			http.Error(w, "Invalid camera_id: must be an integer", http.StatusBadRequest)
			return
		}

		// Ambil Timestamp, diharapkan dalam format RFC3339
		timestampStr := r.FormValue("timestamp")
		if timestampStr == "" {
			http.Error(w, "timestamp is required", http.StatusBadRequest)
			return
		}

		parsedTime, err := time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid timestamp format. Use RFC3339 (e.g., 2023-01-01T15:04:05Z). Error: %v", err), http.StatusBadRequest)
			return
		}
		newDetected.Timestamp = parsedTime

		// Mulai Transaksi Database
		tx, err := s.db.Get().BeginTx(r.Context(), nil)
		if err != nil {
			fmt.Printf("ERROR handleCreateDetected: Failed to start database transaction: %v\n", err) // Tambahkan log
			http.Error(w, "Failed to start database transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}
		var txErr error
		var personImageStoragePath string
		var motorcycleImageStoragePath string

		defer func() {
			if p := recover(); p != nil {
				tx.Rollback()
				// Hapus file jika sudah tersimpan
				if personImageStoragePath != "" {
					os.Remove(personImageStoragePath)
				}
				if motorcycleImageStoragePath != "" {
					os.Remove(motorcycleImageStoragePath)
				}
				panic(p)
			} else if txErr != nil {
				tx.Rollback()
				if personImageStoragePath != "" {
					os.Remove(personImageStoragePath)
				}
				if motorcycleImageStoragePath != "" {
					os.Remove(motorcycleImageStoragePath)
				}
			}
		}()

		// Proses person_image
		fmt.Println("DEBUG handleCreateDetected: Processing person_image") // Tambahkan log
		var personImageID sql.NullInt64
		personImageID, personImageStoragePath, txErr = processImageUpload(r, "person_image", tx)
		if txErr != nil {
			// processImageUpload sudah melakukan logging error internalnya
			http.Error(w, "Failed to process person_image: "+txErr.Error(), determineImageUploadErrorStatusCode(txErr)) // Gunakan helper status code
			return
		}
		newDetected.PersonImageID = personImageID

		// Proses motorcycle_image
		fmt.Println("DEBUG handleCreateDetected: Processing motorcycle_image") // Tambahkan log
		var motorcycleImageID sql.NullInt64
		motorcycleImageID, motorcycleImageStoragePath, txErr = processImageUpload(r, "motorcycle_image", tx)
		if txErr != nil {
			http.Error(w, "Failed to process motorcycle_image: "+txErr.Error(), determineImageUploadErrorStatusCode(txErr)) // Gunakan helper status code
			return
		}
		newDetected.MotorcycleImageID = motorcycleImageID

		// Simpan record detected
		txErr = database.CreateDetectedTx(r.Context(), tx, &newDetected)
		if txErr != nil {
			http.Error(w, "Failed to create detected record: "+txErr.Error(), http.StatusInternalServerError)
			return
		}

		// Commit Transaksi
		txErr = tx.Commit()
		if txErr != nil {
			http.Error(w, "Failed to commit database transaction: "+txErr.Error(), http.StatusInternalServerError)
			return
		}

		// Tepat sebelum mengirim respons sukses
		fmt.Printf("DEBUG: Attempting to send 201 Created with body: %+v\n", newDetected) // Log sebelum WriteHeader
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		errEncode := json.NewEncoder(w).Encode(newDetected)
		if errEncode != nil {
			fmt.Printf("DEBUG: Error encoding response body: %v\n", errEncode)
			// Pada titik ini, header mungkin sudah terkirim, jadi http.Error mungkin tidak efektif
		}
		fmt.Println("DEBUG: Response sent from handleCreateDetected")
	}
}

func (s *Server) handleGetDetected() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		vars := mux.Vars(r)
		idStr, idExists := vars["id"]

		if idExists && idStr != "" { // Get by ID
			id, err := strconv.Atoi(idStr)
			if err != nil {
				http.Error(w, "Invalid detected_id: must be an integer", http.StatusBadRequest)
				return
			}
			d, err := database.GetDetectedByID(r.Context(), s.db.Get(), id)
			if err != nil {
				if err.Error() == "detected not found" { // Sebaiknya gunakan errors.Is atau custom error type
					http.Error(w, "Detected record not found", http.StatusNotFound)
				} else {
					// Log error internal
					fmt.Printf("ERROR: Failed to get detected by ID %d: %v\n", id, err)
					http.Error(w, "Failed to retrieve detected record", http.StatusInternalServerError)
				}
				return
			}
			json.NewEncoder(w).Encode(d)
			return
		}

		// Cek query parameters untuk filter timestamp
		queryParams := r.URL.Query()
		startTimeStr := queryParams.Get("start_time")
		endTimeStr := queryParams.Get("end_time")

		if startTimeStr != "" && endTimeStr != "" {
			layout := time.RFC3339

			startTime, err := time.Parse(layout, startTimeStr)
			if err != nil {
				http.Error(w, fmt.Sprintf("Invalid start_time format. Use %s. Error: %v", layout, err), http.StatusBadRequest)
				return
			}
			endTime, err := time.Parse(layout, endTimeStr)
			if err != nil {
				http.Error(w, fmt.Sprintf("Invalid end_time format. Use %s. Error: %v", layout, err), http.StatusBadRequest)
				return
			}

			// Pastikan endTime adalah setelah startTime
			if endTime.Before(startTime) {
				http.Error(w, "end_time must be after start_time", http.StatusBadRequest)
				return
			}

			// Untuk query range harian yang inklusif pada end_time, Anda mungkin ingin set jam, menit, detik pada endTime ke akhir hari.
			// Contoh: jika endTimeStr adalah "2023-10-27", maka endTime akan menjadi "2023-10-27T00:00:00Z".
			// Jika Anda ingin mencakup seluruh hari "2023-10-27", Anda perlu menyesuaikan endTime menjadi "2023-10-27T23:59:59.999999999Z"
			// atau mengubah query SQL menjadi `timestamp < $2` (jika $2 adalah awal hari berikutnya).
			// Untuk kesederhanaan, contoh ini mengasumsikan timestamp yang diberikan sudah presisi.
			// Jika layout adalah "2006-01-02", maka endTime perlu disesuaikan untuk mencakup seluruh hari.
			if layout == "2006-01-02" {
				endTime = endTime.Add(24*time.Hour - time.Nanosecond) // Akhir hari
			}

			detectedList, err := database.ListDetectedByTimestampRange(r.Context(), s.db.Get(), startTime, endTime)
			if err != nil {
				// Log error internal
				fmt.Printf("ERROR: Failed to list detected by timestamp range (%s - %s): %v\n", startTimeStr, endTimeStr, err)
				http.Error(w, "Failed to retrieve detected records by timestamp", http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(detectedList)
			return
		}

		// List all jika tidak ada ID dan tidak ada filter timestamp yang valid
		detectedList, err := database.ListDetected(r.Context(), s.db.Get())
		if err != nil {
			// Log error internal
			fmt.Printf("ERROR: Failed to list all detected records: %v\n", err)
			http.Error(w, "Failed to retrieve detected records", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(detectedList)
	}
}

func (s *Server) handleUpdateDetected() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		idStr := mux.Vars(r)["id"]
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Invalid detected_id: must be an integer", http.StatusBadRequest)
			return
		}
		var d database.Detected
		if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
			http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Untuk update, jika ingin mengubah gambar, prosesnya akan lebih kompleks.
		// Biasanya, gambar diupdate melalui endpoint terpisah atau dengan logika yang lebih rumit
		// untuk menangani penghapusan gambar lama dan penambahan gambar baru.
		// Handler ini akan mengupdate field non-file dan ID gambar jika disediakan.
		// Timestamp dari body request akan digunakan.

		if err := database.UpdateDetected(r.Context(), s.db.Get(), id, &d); err != nil {
			if err.Error() == "no detected record updated or record not found" {
				http.Error(w, "Detected record not found or no changes made", http.StatusNotFound)
			} else {
				http.Error(w, "Failed to update detected record: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}

		updatedDetected, err := database.GetDetectedByID(r.Context(), s.db.Get(), id)
		if err != nil {
			http.Error(w, "Update succeeded but failed to retrieve updated record: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(updatedDetected)
	}
}

func (s *Server) handleDeleteDetected() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := mux.Vars(r)["id"]
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Invalid detected_id: must be an integer", http.StatusBadRequest)
			return
		}
		// Pertimbangkan untuk menghapus file gambar terkait dari disk jika record detected dihapus.
		// Ini memerlukan pengambilan data detected dulu untuk mendapatkan ID gambar.
		if err := database.DeleteDetected(r.Context(), s.db.Get(), id); err != nil {
			if err.Error() == "no detected record deleted or record not found" {
				http.Error(w, "Detected record not found", http.StatusNotFound)
			} else {
				http.Error(w, "Failed to delete detected record: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// RegisterDetectedRoutes registers all detected-related routes
func (s *Server) RegisterDetectedRoutes(r *mux.Router) {
	r.HandleFunc("/detected", s.handleCreateDetected()).Methods("POST")
	r.HandleFunc("/detected", s.handleGetDetected()).Methods("GET")             // List all
	r.HandleFunc("/detected/{id:[0-9]+}", s.handleGetDetected()).Methods("GET") // Get by ID
	r.HandleFunc("/detected/{id:[0-9]+}", s.handleUpdateDetected()).Methods("PUT")
	r.HandleFunc("/detected/{id:[0-9]+}", s.handleDeleteDetected()).Methods("DELETE")
}