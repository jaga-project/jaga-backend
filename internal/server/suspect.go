package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
)

func (s *Server) handleCreateSuspect() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var suspect database.Suspect
        if err := json.NewDecoder(r.Body).Decode(&suspect); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        suspect.CreatedAt = time.Now()
        if err := database.CreateSuspect(r.Context(), s.db.Get(), &suspect); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(suspect)
    }
}

func (s *Server) handleGetSuspect() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        idStr := mux.Vars(r)["id"]
        if idStr != "" {
            id, err := strconv.ParseInt(idStr, 10, 64)
            if err != nil {
                http.Error(w, "invalid suspect_id", http.StatusBadRequest)
                return
            }
            suspect, err := database.GetSuspectByID(r.Context(), s.db.Get(), id)
            if err != nil {
                http.Error(w, err.Error(), http.StatusNotFound)
                return
            }
            json.NewEncoder(w).Encode(suspect)
            return
        }
        list, err := database.ListSuspects(r.Context(), s.db.Get())
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        json.NewEncoder(w).Encode(list)
    }
}

func (s *Server) handleUpdateSuspect() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        idStr := mux.Vars(r)["id"]
        id, err := strconv.ParseInt(idStr, 10, 64)
        if err != nil {
            http.Error(w, "invalid suspect_id", http.StatusBadRequest)
            return
        }
        var suspect database.Suspect
        if err := json.NewDecoder(r.Body).Decode(&suspect); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        suspect.CreatedAt = time.Now()
        if err := database.UpdateSuspect(r.Context(), s.db.Get(), id, &suspect); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusOK)
    }
}

func (s *Server) handleDeleteSuspect() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        idStr := mux.Vars(r)["id"]
        id, err := strconv.ParseInt(idStr, 10, 64)
        if err != nil {
            http.Error(w, "invalid suspect_id", http.StatusBadRequest)
            return
        }
        if err := database.DeleteSuspect(r.Context(), s.db.Get(), id); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusNoContent)
    }
}

// RegisterSuspectRoutes registers all suspect-related routes
func (s *Server) RegisterSuspectRoutes(r *mux.Router) {
    r.HandleFunc("/suspects", s.handleCreateSuspect()).Methods("POST")
    r.HandleFunc("/suspects", s.handleGetSuspect()).Methods("GET")
    r.HandleFunc("/suspects/{id}", s.handleGetSuspect()).Methods("GET")
    r.HandleFunc("/suspects/{id}", s.handleUpdateSuspect()).Methods("PUT")
    r.HandleFunc("/suspects/{id}", s.handleDeleteSuspect()).Methods("DELETE")
}