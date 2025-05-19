package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
)

func (s *Server) handleCreateUser() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var user database.User
        if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        // Generate UUID and created time
        user.UserID = uuid.New().String()
        user.CreatedAt = time.Now()

        if err := database.CreateSingleUser(s.db.Get(), user, r.Context()); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(user)
    }
}

func (s *Server) handleGetUser() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        email := r.URL.Query().Get("email")
        if email == "" {
            // Get all users if no email specified
            users, err := database.FindManyUser(s.db.Get(), r.Context())
            if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
            json.NewEncoder(w).Encode(users)
            return
        }

        // Get single user by email
        user, err := database.FindSingleUser(s.db.Get(), database.User{Email: email}, r.Context())
        if err != nil {
            http.Error(w, err.Error(), http.StatusNotFound)
            return
        }
        json.NewEncoder(w).Encode(user)
    }
}

func (s *Server) handleUpdateUser() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := mux.Vars(r)["id"]
        var user database.User
        if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        if err := database.UpdateSingleUser(s.db.Get(), userID, user, r.Context()); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        w.WriteHeader(http.StatusOK)
    }
}

func (s *Server) handleDeleteUser() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := mux.Vars(r)["id"]
        if err := database.DeleteSingleUser(s.db.Get(), userID, r.Context()); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusNoContent)
    }
}

// RegisterUserRoutes registers all user-related routes
func (s *Server) RegisterUserRoutes(r *mux.Router) {
    r.HandleFunc("/users", s.handleCreateUser()).Methods("POST")
    r.HandleFunc("/users", s.handleGetUser()).Methods("GET")
    r.HandleFunc("/users/{id}", s.handleUpdateUser()).Methods("PUT")
    r.HandleFunc("/users/{id}", s.handleDeleteUser()).Methods("DELETE")
}

