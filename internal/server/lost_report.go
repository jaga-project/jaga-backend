package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
	"github.com/jaga-project/jaga-backend/internal/middleware" // Import middleware untuk context key
)

func (s *Server) handleCreateLostReport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var lr database.LostReport
		if err := json.NewDecoder(r.Body).Decode(&lr); err != nil {
			http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Ambil UserID dari context yang di-set oleh JWTMiddleware
		requestingUserID, ok := r.Context().Value(middleware.UserIDContextKey).(string)
		if !ok || requestingUserID == "" {
			// Ini seharusnya tidak terjadi jika middleware bekerja dengan benar
			http.Error(w, "Unauthorized: User ID not found in token", http.StatusUnauthorized)
			return
		}
		lr.UserID = requestingUserID // Set UserID laporan dengan ID dari token

		// Validasi: Timestamp (waktu kejadian) wajib diisi oleh client.
		if lr.Timestamp.IsZero() {
			http.Error(w, "timestamp (waktu kejadian) is required", http.StatusBadRequest)
			return
		}
		// Validasi tambahan (opsional): Pastikan timestamp tidak di masa depan
		if lr.Timestamp.After(time.Now().Add(5 * time.Minute)) { // Toleransi 5 menit
			http.Error(w, "timestamp (waktu kejadian) cannot be unreasonably in the future", http.StatusBadRequest)
			return
		}

		// Asumsi: lr.EvidenceImageID (jika ada) sudah merupakan ID dari gambar yang valid
		// yang telah diunggah sebelumnya dan disimpan di tabel 'images'.
		// Asumsi: lr.VehicleID juga sudah divalidasi atau akan divalidasi
		// bahwa vehicle tersebut ada dan milik user yang bersangkutan (jika perlu).

		if err := database.CreateLostReport(r.Context(), s.db.Get(), &lr); err != nil {
			http.Error(w, "Failed to create lost report: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(lr)
	}
}

func (s *Server) handleGetLostReport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Ambil UserID dari context untuk potensi filter berdasarkan user atau role
		// requestingUserID, _ := r.Context().Value(middleware.UserIDContextKey).(string)
		// isAdmin, _ := r.Context().Value(middleware.AdminStatusContextKey).(bool)

		idStr := mux.Vars(r)["id"]
		if idStr != "" { // Get by ID
			id, err := strconv.Atoi(idStr)
			if err != nil {
				http.Error(w, "invalid lost_id: must be an integer", http.StatusBadRequest)
				return
			}
			lr, err := database.GetLostReportByID(r.Context(), s.db.Get(), id)
			if err != nil {
				if err.Error() == "lost_report not found" {
					http.Error(w, "Lost report not found", http.StatusNotFound)
				} else {
					http.Error(w, "Failed to get lost report: "+err.Error(), http.StatusInternalServerError)
				}
				return
			}
			// Otorisasi: User hanya bisa melihat laporannya sendiri, kecuali admin
			// if !isAdmin && lr.UserID != requestingUserID {
			// 	http.Error(w, "Forbidden: You can only view your own reports", http.StatusForbidden)
			// 	return
			// }
			json.NewEncoder(w).Encode(lr)
			return
		}

		// List all (mungkin hanya untuk admin atau dengan filter user)
		// if !isAdmin {
		//  // Filter berdasarkan requestingUserID jika bukan admin
		//  // list, err := database.ListLostReportsByUserID(r.Context(), s.db.Get(), requestingUserID)
		// } else {
		// list, err := database.ListLostReports(r.Context(), s.db.Get())
		// }
		list, err := database.ListLostReports(r.Context(), s.db.Get()) // Untuk sekarang, list semua
		if err != nil {
			http.Error(w, "Failed to list lost reports: "+err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(list)
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

		var lrUpdates database.LostReport // Data dari request body
		if err := json.NewDecoder(r.Body).Decode(&lrUpdates); err != nil {
			http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Ambil UserID dan status Admin dari context
		requestingUserID, ok := r.Context().Value(middleware.UserIDContextKey).(string)
		if !ok || requestingUserID == "" {
			http.Error(w, "Unauthorized: User ID not found in token", http.StatusUnauthorized)
			return
		}
		isAdmin, _ := r.Context().Value(middleware.AdminStatusContextKey).(bool) // Default ke false jika tidak ada

		existingLR, err := database.GetLostReportByID(r.Context(), s.db.Get(), id)
		if err != nil {
			if err.Error() == "lost_report not found" {
				http.Error(w, "Lost report not found", http.StatusNotFound)
			} else {
				http.Error(w, "Failed to retrieve existing report: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}

		reportToUpdate := *existingLR // Salin semua field dari existingLR
		updatedByOwner := false
		updatedByAdmin := false

		// Logika untuk Admin: Admin bisa mengubah status
		if isAdmin {
			if lrUpdates.Status != "" && lrUpdates.Status != existingLR.Status {
				// Validasi status jika perlu (misalnya, pastikan status ada dalam daftar enum yang valid)
				// if !isValidStatus(lrUpdates.Status) {
				// 	http.Error(w, "Invalid status value", http.StatusBadRequest)
				// 	return
				// }
				reportToUpdate.Status = lrUpdates.Status
				updatedByAdmin = true
			}
		}

		// Logika untuk Pemilik Laporan: Pemilik bisa mengubah field tertentu
		if existingLR.UserID == requestingUserID {
			// Timestamp
			if !lrUpdates.Timestamp.IsZero() && lrUpdates.Timestamp != existingLR.Timestamp {
				if lrUpdates.Timestamp.After(time.Now().Add(5 * time.Minute)) { // Toleransi 5 menit
					http.Error(w, "timestamp (waktu kejadian) cannot be unreasonably in the future", http.StatusBadRequest)
					return
				}
				reportToUpdate.Timestamp = lrUpdates.Timestamp
				updatedByOwner = true
			}

			// Address
			if lrUpdates.Address != "" && lrUpdates.Address != existingLR.Address {
				reportToUpdate.Address = lrUpdates.Address
				updatedByOwner = true
			}

			// VehicleID
			if lrUpdates.VehicleID != 0 && lrUpdates.VehicleID != existingLR.VehicleID {
				// TODO: Validasi apakah VehicleID ini milik requestingUserID
				// if !isValidVehicleForUser(r.Context(), s.db.Get(), requestingUserID, lrUpdates.VehicleID) {
				// 	http.Error(w, "Invalid vehicle_id for this user", http.StatusBadRequest)
				// 	return
				// }
				reportToUpdate.VehicleID = lrUpdates.VehicleID
				updatedByOwner = true
			}

			// EvidenceImageID
			// Jika lrUpdates.EvidenceImageID adalah nil, itu bisa berarti "hapus" atau "tidak ada perubahan".
			// Kita akan menganggapnya "gunakan nilai dari request jika ada, termasuk null".
			// Ini berarti jika client mengirim "evidence_image_id": null, maka akan di-set ke null.
			// Jika client tidak mengirim field "evidence_image_id", maka reportToUpdate.EvidenceImageID akan
			// tetap dari existingLR.EvidenceImageID karena lrUpdates.EvidenceImageID akan nil (default untuk pointer).
			// Jadi, kita perlu cara untuk tahu apakah field itu *benar-benar* dikirim.
			// Untuk sementara, kita akan update jika lrUpdates.EvidenceImageID berbeda dari existingLR.EvidenceImageID
			// Ini tidak ideal.
			// Mari kita asumsikan jika client mengirim fieldnya, kita pakai.
			// Jika tidak, kita pertahankan yang lama. Ini sulit tanpa struct request khusus.
			// Untuk sekarang, kita akan update jika lrUpdates.EvidenceImageID adalah non-nil.
			// Jika ingin mengizinkan penghapusan dengan mengirim null, maka:
			// reportToUpdate.EvidenceImageID = lrUpdates.EvidenceImageID;
			// Mari kita gunakan ini:
			if lrUpdates.EvidenceImageID != existingLR.EvidenceImageID { // Ini akan menangkap perubahan ke nil atau ke ID baru
				reportToUpdate.EvidenceImageID = lrUpdates.EvidenceImageID
				updatedByOwner = true
			}
		} else {
			// Jika bukan admin dan bukan pemilik, tidak boleh update field pemilik
			if lrUpdates.Timestamp != existingLR.Timestamp && !lrUpdates.Timestamp.IsZero() ||
				lrUpdates.Address != existingLR.Address && lrUpdates.Address != "" ||
				lrUpdates.VehicleID != existingLR.VehicleID && lrUpdates.VehicleID != 0 ||
				lrUpdates.EvidenceImageID != existingLR.EvidenceImageID {
				// Jika ada upaya mengubah field pemilik oleh non-pemilik (dan bukan admin yang mengubah status)
				if !isAdmin || (isAdmin && lrUpdates.Status == "" || lrUpdates.Status == existingLR.Status) { // Jika admin tidak mengubah status
					http.Error(w, "Forbidden: You can only update your own report's details", http.StatusForbidden)
					return
				}
			}
		}

		// Pastikan UserID tidak diubah
		reportToUpdate.UserID = existingLR.UserID
		// Diasumsikan DetectedID tidak diubah melalui endpoint ini oleh user/admin
		reportToUpdate.DetectedID = existingLR.DetectedID

		// Hanya lakukan update ke database jika ada perubahan
		if !updatedByAdmin && !updatedByOwner {
			// Tidak ada perubahan yang diizinkan atau tidak ada data yang relevan untuk diubah
			w.WriteHeader(http.StatusOK) // Atau http.StatusNotModified jika Anda mau
			json.NewEncoder(w).Encode(existingLR) // Kirim data lama karena tidak ada perubahan
			return
		}

		if err := database.UpdateLostReport(r.Context(), s.db.Get(), id, &reportToUpdate); err != nil {
			http.Error(w, "Failed to update lost report: "+err.Error(), http.StatusInternalServerError)
			return
		}

		updatedReport, err := database.GetLostReportByID(r.Context(), s.db.Get(), id)
		if err != nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(reportToUpdate) // Kirim data yang kita punya jika gagal fetch
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(updatedReport)
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
		// isAdmin, _ := r.Context().Value(middleware.AdminStatusContextKey).(bool)

		existingLR, err := database.GetLostReportByID(r.Context(), s.db.Get(), id)
		if err != nil {
			if err.Error() == "lost_report not found" {
				http.Error(w, "Lost report not found", http.StatusNotFound)
			} else {
				http.Error(w, "Failed to retrieve existing report: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}

		// Otorisasi: Hanya pemilik laporan atau admin yang boleh delete
		// if !isAdmin && existingLR.UserID != requestingUserID {
		// 	http.Error(w, "Forbidden: You can only delete your own reports", http.StatusForbidden)
		// 	return
		// }
		if existingLR.UserID != requestingUserID { // Sederhanakan: hanya pemilik
			http.Error(w, "Forbidden: You can only delete your own reports", http.StatusForbidden)
			return
		}

		if err := database.DeleteLostReport(r.Context(), s.db.Get(), id); err != nil {
			http.Error(w, "Failed to delete lost report: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// RegisterLostReportRoutes mendaftarkan semua rute terkait lost_report.
// Sekarang semua rute ini akan dilindungi oleh middleware JWT jika didaftarkan di bawah apiRouter.
func (s *Server) RegisterLostReportRoutes(r *mux.Router) {
	r.HandleFunc("/lost_reports", s.handleCreateLostReport()).Methods("POST")
	r.HandleFunc("/lost_reports", s.handleGetLostReport()).Methods("GET") // List all
	r.HandleFunc("/lost_reports/{id}", s.handleGetLostReport()).Methods("GET") // Get by ID
	r.HandleFunc("/lost_reports/{id}", s.handleUpdateLostReport()).Methods("PUT")
	r.HandleFunc("/lost_reports/{id}", s.handleDeleteLostReport()).Methods("DELETE")
}