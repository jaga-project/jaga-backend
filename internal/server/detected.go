package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
)

func (s *Server) handleCreateDetected() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var d database.Detected
		if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Timestamp seharusnya sudah diisi dari request body (misalnya dari sistem ML yang mendeteksi)
		// Jika timestamp perlu di-generate di sini:
		// if d.Timestamp.IsZero() {
		// 	d.Timestamp = time.Now()
		// }

		if err := database.CreateDetected(r.Context(), s.db.Get(), &d); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(d)
	}
}

func (s *Server) handleGetDetected() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := mux.Vars(r)["id"]
		if idStr != "" {
			id, err := strconv.Atoi(idStr)
			if err != nil {
				http.Error(w, "invalid detected_id: must be an integer", http.StatusBadRequest)
				return
			}
			d, err := database.GetDetectedByID(r.Context(), s.db.Get(), id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound) // Asumsi error dari GetByID adalah Not Found jika tidak ada error lain
				return
			}
			json.NewEncoder(w).Encode(d)
			return
		}
		detectedList, err := database.ListDetected(r.Context(), s.db.Get())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(detectedList)
	}
}

func (s *Server) handleUpdateDetected() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := mux.Vars(r)["id"]
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "invalid detected_id: must be an integer", http.StatusBadRequest)
			return
		}
		var d database.Detected
		if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Timestamp dari body request akan digunakan.
		// Jika ingin timestamp diupdate ke waktu sekarang saat operasi update:
		// d.Timestamp = time.Now()

		if err := database.UpdateDetected(r.Context(), s.db.Get(), id, &d); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError) // Bisa juga StatusNotFound jika update gagal karena ID tidak ada
			return
		}
		// Mengambil data yang sudah diupdate untuk dikirim kembali (opsional)
		updatedDetected, err := database.GetDetectedByID(r.Context(), s.db.Get(), id)
		if err != nil {
			// Jika error di sini, setidaknya operasi update sudah berhasil.
			w.WriteHeader(http.StatusOK) // Kirim OK tanpa body jika tidak bisa mengambil data terbaru
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
			http.Error(w, "invalid detected_id: must be an integer", http.StatusBadRequest)
			return
		}
		if err := database.DeleteDetected(r.Context(), s.db.Get(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError) // Bisa juga StatusNotFound jika delete gagal karena ID tidak ada
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// RegisterDetectedRoutes registers all detected-related routes
func (s *Server) RegisterDetectedRoutes(r *mux.Router) {
	r.HandleFunc("/detected", s.handleCreateDetected()).Methods("POST")
	r.HandleFunc("/detected", s.handleGetDetected()).Methods("GET")       // List all
	r.HandleFunc("/detected/{id}", s.handleGetDetected()).Methods("GET")    // Get by ID
	r.HandleFunc("/detected/{id}", s.handleUpdateDetected()).Methods("PUT")
	r.HandleFunc("/detected/{id}", s.handleDeleteDetected()).Methods("DELETE")
}