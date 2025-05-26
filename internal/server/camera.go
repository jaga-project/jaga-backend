package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
)

func (s *Server) handleCreateCamera() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var cam database.Camera
        if err := json.NewDecoder(r.Body).Decode(&cam); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        if err := database.CreateCamera(r.Context(), s.db.Get(), &cam); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(cam)
    }
}

func (s *Server) handleGetCamera() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        idStr := mux.Vars(r)["id"]
        if idStr != "" {
            id, err := strconv.ParseInt(idStr, 10, 64)
            if err != nil {
                http.Error(w, "invalid camera_id", http.StatusBadRequest)
                return
            }
            cam, err := database.GetCameraByID(r.Context(), s.db.Get(), id)
            if err != nil {
                http.Error(w, err.Error(), http.StatusNotFound)
                return
            }
            json.NewEncoder(w).Encode(cam)
            return
        }
        list, err := database.ListCameras(r.Context(), s.db.Get())
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        json.NewEncoder(w).Encode(list)
    }
}

func (s *Server) handleUpdateCamera() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        idStr := mux.Vars(r)["id"]
        id, err := strconv.ParseInt(idStr, 10, 64)
        if err != nil {
            http.Error(w, "invalid camera_id", http.StatusBadRequest)
            return
        }
        var cam database.Camera
        if err := json.NewDecoder(r.Body).Decode(&cam); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        if err := database.UpdateCamera(r.Context(), s.db.Get(), id, &cam); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusOK)
    }
}

func (s *Server) handleDeleteCamera() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        idStr := mux.Vars(r)["id"]
        id, err := strconv.ParseInt(idStr, 10, 64)
        if err != nil {
            http.Error(w, "invalid camera_id", http.StatusBadRequest)
            return
        }
        if err := database.DeleteCamera(r.Context(), s.db.Get(), id); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusNoContent)
    }
}

// RegisterCameraRoutes registers all camera-related routes
func (s *Server) RegisterCameraRoutes(r *mux.Router) {
    r.HandleFunc("/cameras", s.handleCreateCamera()).Methods("POST")
    r.HandleFunc("/cameras", s.handleGetCamera()).Methods("GET")
    r.HandleFunc("/cameras/{id}", s.handleGetCamera()).Methods("GET")
    r.HandleFunc("/cameras/{id}", s.handleUpdateCamera()).Methods("PUT")
    r.HandleFunc("/cameras/{id}", s.handleDeleteCamera()).Methods("DELETE")
}