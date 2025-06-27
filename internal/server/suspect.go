package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
)

// handleCreateSuspect menangani pembuatan satu data suspect.
func (s *Server) handleCreateSuspect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var suspect database.Suspect
		if err := json.NewDecoder(r.Body).Decode(&suspect); err != nil {
			writeJSONError(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		suspect.CreatedAt = time.Now()
		if err := database.CreateSuspect(r.Context(), s.db.Get(), &suspect); err != nil {
			log.Printf("ERROR: Failed to create suspect: %v", err)
			writeJSONError(w, "Failed to create suspect", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(suspect)
	}
}

// handleCreateManySuspects menangani pembuatan banyak suspect sekaligus (batch).
func (s *Server) handleCreateManySuspects() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var suspects []*database.Suspect
		if err := json.NewDecoder(r.Body).Decode(&suspects); err != nil {
			writeJSONError(w, "Invalid request body: expected an array of suspects", http.StatusBadRequest)
			return
		}

		if len(suspects) == 0 {
			writeJSONError(w, "Request body must contain at least one suspect", http.StatusBadRequest)
			return
		}

		// Setel waktu pembuatan untuk setiap suspect
		now := time.Now()
		for _, suspect := range suspects {
			suspect.CreatedAt = now
		}

		// Panggil fungsi database untuk membuat banyak data sekaligus
		err := database.CreateManySuspects(r.Context(), s.db.Get(), suspects)
		if err != nil {
			log.Printf("ERROR: Failed to create many suspects: %v", err)
			writeJSONError(w, fmt.Sprintf("Failed to create suspects in database: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"message": fmt.Sprintf("%d suspects created successfully", len(suspects))})
	}
}

// handleListSuspects menangani permintaan untuk melihat semua data suspect.
func (s *Server) handleListSuspects() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := database.ListSuspects(r.Context(), s.db.Get())
		if err != nil {
			log.Printf("ERROR: Failed to list suspects: %v", err)
			writeJSONError(w, fmt.Sprintf("Failed to retrieve suspects: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	}
}

// handleGetSuspectByID menangani permintaan untuk melihat satu suspect berdasarkan ID.
func (s *Server) handleGetSuspectByID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := mux.Vars(r)["id"]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeJSONError(w, "Invalid suspect ID", http.StatusBadRequest)
			return
		}

		suspect, err := database.GetSuspectByID(r.Context(), s.db.Get(), id)
		if err != nil {
			if err.Error() == "suspect not found" {
				writeJSONError(w, err.Error(), http.StatusNotFound)
			} else {
				log.Printf("ERROR: Failed to get suspect by ID %d: %v", id, err)
				writeJSONError(w, "Failed to retrieve suspect", http.StatusInternalServerError)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(suspect)
	}
}

// handleUpdateSuspect menangani pembaruan data suspect.
func (s *Server) handleUpdateSuspect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := mux.Vars(r)["id"]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeJSONError(w, "Invalid suspect ID", http.StatusBadRequest)
			return
		}
		var suspect database.Suspect
		if err := json.NewDecoder(r.Body).Decode(&suspect); err != nil {
			writeJSONError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Catatan: CreatedAt tidak diubah saat update.
		if err := database.UpdateSuspect(r.Context(), s.db.Get(), id, &suspect); err != nil {
			log.Printf("ERROR: Failed to update suspect %d: %v", id, err)
			writeJSONError(w, "Failed to update suspect", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "Suspect updated successfully"})
	}
}

// handleDeleteSuspect menangani penghapusan data suspect.
func (s *Server) handleDeleteSuspect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := mux.Vars(r)["id"]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeJSONError(w, "Invalid suspect ID", http.StatusBadRequest)
			return
		}
		if err := database.DeleteSuspect(r.Context(), s.db.Get(), id); err != nil {
			log.Printf("ERROR: Failed to delete suspect %d: %v", id, err)
			writeJSONError(w, "Failed to delete suspect", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// RegisterSuspectRoutes mendaftarkan semua rute yang berhubungan dengan suspect.
func (s *Server) RegisterSuspectRoutes(r *mux.Router) {
	r.HandleFunc("/suspects", s.handleCreateSuspect()).Methods("POST")
	r.HandleFunc("/suspects/batch", s.handleCreateManySuspects()).Methods("POST")
	r.HandleFunc("/suspects", s.handleListSuspects()).Methods("GET")
	r.HandleFunc("/suspects/{id:[0-9]+}", s.handleGetSuspectByID()).Methods("GET")
	r.HandleFunc("/suspects/{id:[0-9]+}", s.handleUpdateSuspect()).Methods("PUT")
	r.HandleFunc("/suspects/{id:[0-9]+}", s.handleDeleteSuspect()).Methods("DELETE")
}

