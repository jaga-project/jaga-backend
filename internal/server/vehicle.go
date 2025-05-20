package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
)

func (s *Server) handleCreateVehicle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var v database.Vehicle
		if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := database.CreateVehicle(r.Context(), s.db.Get(), &v); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(v)
	}
}

func (s *Server) handleGetVehicle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := mux.Vars(r)["id"]
		if idStr != "" {
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				http.Error(w, "invalid vehicle_id", http.StatusBadRequest)
				return
			}
			v, err := database.GetVehicleByID(r.Context(), s.db.Get(), id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(v)
			return
		}
		vehicles, err := database.ListVehicles(r.Context(), s.db.Get())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(vehicles)
	}
}

func (s *Server) handleUpdateVehicle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := mux.Vars(r)["id"]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid vehicle_id", http.StatusBadRequest)
			return
		}
		var v database.Vehicle
		if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := database.UpdateVehicle(r.Context(), s.db.Get(), id, &v); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func (s *Server) handleDeleteVehicle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := mux.Vars(r)["id"]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid vehicle_id", http.StatusBadRequest)
			return
		}
		if err := database.DeleteVehicle(r.Context(), s.db.Get(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// RegisterVehicleRoutes registers all vehicle-related routes
func (s *Server) RegisterVehicleRoutes(r *mux.Router) {
	r.HandleFunc("/vehicles", s.handleCreateVehicle()).Methods("POST")
	r.HandleFunc("/vehicles", s.handleGetVehicle()).Methods("GET")
	r.HandleFunc("/vehicles/{id}", s.handleGetVehicle()).Methods("GET")
	r.HandleFunc("/vehicles/{id}", s.handleUpdateVehicle()).Methods("PUT")
	r.HandleFunc("/vehicles/{id}", s.handleDeleteVehicle()).Methods("DELETE")
}