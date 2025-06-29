package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"fmt"

	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
	"github.com/jaga-project/jaga-backend/internal/middleware"
)

func (s *Server) handleCreateAdmin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var admin database.Admin
		if err := json.NewDecoder(r.Body).Decode(&admin); err != nil {
			writeJSONError(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
			return
		}

		if admin.UserID == "" {
			writeJSONError(w, "user_id is required", http.StatusBadRequest)
			return
		}

		admin.CreatedAt = time.Now()
		tx, err := s.db.Get().BeginTx(r.Context(), nil)
        if err != nil {
            fmt.Printf("ERROR handleCreateDetected: Failed to start database transaction: %v\n", err)
            writeJSONError(w, "Failed to start database transaction: "+err.Error(), http.StatusInternalServerError)
            return
        }
		defer tx.Rollback()

		if err := database.CreateAdminTx(r.Context(), tx, &admin); err != nil {
			if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
				writeJSONError(w, "This user is already an admin", http.StatusConflict)
				return
			}
			writeJSONError(w, "Failed to create admin: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
            writeJSONError(w, "Failed to commit transaction: "+err.Error(), http.StatusInternalServerError)
            return
    }
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(admin)
	}
}

func (s *Server) handleGetAdmin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := mux.Vars(r)["user_id"] 
		if userID != "" {
			admin, err := database.GetAdminByUserID(r.Context(), s.db.Get(), userID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeJSONError(w, "Admin not found", http.StatusNotFound)
				} else {
					writeJSONError(w, "Failed to get admin: "+err.Error(), http.StatusInternalServerError)
				}
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(admin)
			return
		}

		// Jika tidak ada user_id di path, list semua admin
		admins, err := database.ListAdmin(r.Context(), s.db.Get())
		if err != nil {
			writeJSONError(w, "Failed to list admins: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(admins)
	}
}

func (s *Server) handleUpdateAdmin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := mux.Vars(r)["user_id"]
		var adminUpdates database.Admin
		if err := json.NewDecoder(r.Body).Decode(&adminUpdates); err != nil {
			writeJSONError(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
			return
		}

		if err := database.UpdateAdmin(r.Context(), s.db.Get(), userID, &adminUpdates); err != nil {
			writeJSONError(w, "Failed to update admin: "+err.Error(), http.StatusInternalServerError)
			return
		}

		updatedAdmin, err := database.GetAdminByUserID(r.Context(), s.db.Get(), userID)
		if err != nil {
			writeJSONError(w, "Failed to retrieve updated admin data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(updatedAdmin)
	}
}

func (s *Server) handleDeleteAdmin() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := mux.Vars(r)["user_id"]

        if err := database.DeleteAdmin(r.Context(), s.db.Get(), userID); err != nil {
            if errors.Is(err, sql.ErrNoRows) {
                w.WriteHeader(http.StatusNoContent)
                return
            }
            writeJSONError(w, "Failed to demote admin: "+err.Error(), http.StatusInternalServerError)
            return
        }

        w.WriteHeader(http.StatusNoContent)
    }
}

func (s *Server) RegisterAdminRoutes(r *mux.Router) {
	adminOnlyMiddleware := middleware.AdminOnlyMiddleware()
	r.Handle("/", adminOnlyMiddleware(s.handleCreateAdmin())).Methods("POST")
	r.Handle("/", adminOnlyMiddleware(s.handleGetAdmin())).Methods("GET")
	r.Handle("/{user_id}", adminOnlyMiddleware(s.handleGetAdmin())).Methods("GET")
	r.Handle("/{user_id}", adminOnlyMiddleware(s.handleUpdateAdmin())).Methods("PUT")
	r.Handle("/{user_id}", adminOnlyMiddleware(s.handleDeleteAdmin())).Methods("DELETE")
}
