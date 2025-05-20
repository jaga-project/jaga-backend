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
                http.Error(w, "invalid detected_id", http.StatusBadRequest)
                return
            }
            d, err := database.GetDetectedByID(r.Context(), s.db.Get(), id)
            if err != nil {
                http.Error(w, err.Error(), http.StatusNotFound)
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
            http.Error(w, "invalid detected_id", http.StatusBadRequest)
            return
        }
        var d database.Detected
        if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        if err := database.UpdateDetected(r.Context(), s.db.Get(), id, &d); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusOK)
    }
}

func (s *Server) handleDeleteDetected() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        idStr := mux.Vars(r)["id"]
        id, err := strconv.Atoi(idStr)
        if err != nil {
            http.Error(w, "invalid detected_id", http.StatusBadRequest)
            return
        }
        if err := database.DeleteDetected(r.Context(), s.db.Get(), id); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusNoContent)
    }
}

// RegisterDetectedRoutes registers all detected-related routes
func (s *Server) RegisterDetectedRoutes(r *mux.Router) {
    r.HandleFunc("/detected", s.handleCreateDetected()).Methods("POST")
    r.HandleFunc("/detected", s.handleGetDetected()).Methods("GET")
    r.HandleFunc("/detected/{id}", s.handleGetDetected()).Methods("GET")
    r.HandleFunc("/detected/{id}", s.handleUpdateDetected()).Methods("PUT")
    r.HandleFunc("/detected/{id}", s.handleDeleteDetected()).Methods("DELETE")
}