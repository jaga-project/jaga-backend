package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
	"github.com/jaga-project/jaga-backend/internal/middleware"
)

func (s *Server) handleCreateCamera() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var cam database.Camera
		if err := json.NewDecoder(r.Body).Decode(&cam); err != nil {
			writeJSONError(w, "Invalid JSON format: "+err.Error(), http.StatusBadRequest)
			return
		}

		validationErrors := make(map[string]string)
		if strings.TrimSpace(cam.Name) == "" {
			validationErrors["name"] = "Name field is required and cannot be empty."
		}
		if strings.TrimSpace(cam.IPCamera) == "" {
			validationErrors["ip_camera"] = "IP Camera field is required and cannot be empty."
		}
		if cam.Latitude < -90 || cam.Latitude > 90 {
			validationErrors["latitude"] = "Latitude must be between -90 and 90."
		}
		if cam.Longitude < -180 || cam.Longitude > 180 {
			validationErrors["longitude"] = "Longitude must be between -180 and 180."
		}

		if len(validationErrors) > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "Validation failed",
				"details": validationErrors,
			})
			return
		}

		if err := database.CreateCamera(r.Context(), s.db.Get(), &cam); err != nil {
			writeJSONError(w, "Failed to create camera: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(cam)
	}
}

func (s *Server) handleListCameras() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := database.ListCameras(r.Context(), s.db.Get())
		if err != nil {
			writeJSONError(w, "Failed to list cameras: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	}
}

func (s *Server) handleGetCameraByID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := mux.Vars(r)["id"]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeJSONError(w, "Invalid camera_id: must be an integer", http.StatusBadRequest)
			return
		}
		cam, err := database.GetCameraByID(r.Context(), s.db.Get(), id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "not found") {
				writeJSONError(w, "Camera not found", http.StatusNotFound)
			} else {
				writeJSONError(w, "Failed to get camera: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cam)
	}
}

func (s *Server) handleUpdateCamera() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := mux.Vars(r)["id"]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeJSONError(w, "Invalid camera_id: must be an integer", http.StatusBadRequest)
			return
		}
		var cam database.Camera
		if err := json.NewDecoder(r.Body).Decode(&cam); err != nil {
			writeJSONError(w, "Invalid JSON format: "+err.Error(), http.StatusBadRequest)
			return
		}

		validationErrors := make(map[string]string)
		if strings.TrimSpace(cam.Name) == "" {
			validationErrors["name"] = "Name field is required and cannot be empty."
		}
		if strings.TrimSpace(cam.IPCamera) == "" {
			validationErrors["ip_camera"] = "IP Camera field is required and cannot be empty."
		}
		if cam.Latitude < -90 || cam.Latitude > 90 {
			validationErrors["latitude"] = "Latitude must be between -90 and 90."
		}
		if cam.Longitude < -180 || cam.Longitude > 180 {
			validationErrors["longitude"] = "Longitude must be between -180 and 180."
		}

		if len(validationErrors) > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "Validation failed",
				"details": validationErrors,
			})
			return
		}

		if err := database.UpdateCamera(r.Context(), s.db.Get(), id, &cam); err != nil {
			writeJSONError(w, "Failed to update camera: "+err.Error(), http.StatusInternalServerError)
			return
		}

		updatedCam, err := database.GetCameraByID(r.Context(), s.db.Get(), id)
		if err != nil {
			writeJSONError(w, "Failed to retrieve updated camera: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updatedCam)
	}
}

func (s *Server) handleDeleteCamera() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := mux.Vars(r)["id"]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeJSONError(w, "Invalid camera_id: must be an integer", http.StatusBadRequest)
			return
		}
		if err := database.DeleteCamera(r.Context(), s.db.Get(), id); err != nil {
			writeJSONError(w, "Failed to delete camera: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) RegisterPublicCameraRoutes(r *mux.Router) {
    r.HandleFunc("/cameras", s.handleListCameras()).Methods("GET")
    r.HandleFunc("/cameras/{id:[0-9]+}", s.handleGetCameraByID()).Methods("GET")
}

func (s *Server) RegisterProtectedCameraRoutes(r *mux.Router) {
    adminOnlyMiddleware := middleware.AdminOnlyMiddleware()
    r.Handle("/cameras", adminOnlyMiddleware(s.handleCreateCamera())).Methods("POST")
    r.Handle("/cameras/{id:[0-9]+}", adminOnlyMiddleware(s.handleUpdateCamera())).Methods("PUT")
    r.Handle("/cameras/{id:[0-9]+}", adminOnlyMiddleware(s.handleDeleteCamera())).Methods("DELETE")
}