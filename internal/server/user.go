package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/database"
	"golang.org/x/crypto/bcrypt"
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

		// Hash password sebelum disimpan ke database
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Failed to hash password", http.StatusInternalServerError)
			return
		}
		user.Password = string(hashedPassword) // Simpan hash password

		if err := database.CreateSingleUser(s.db.Get(), user, r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Sebaiknya jangan kirim balik password, bahkan yang sudah di-hash, ke client
		user.Password = "" // Kosongkan password sebelum mengirim response

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
			// Kosongkan password untuk setiap user sebelum mengirim response
			for i := range users {
				users[i].Password = ""
			}
			json.NewEncoder(w).Encode(users)
			return
		}

		// Get single user by email
		user, err := database.FindSingleUser(s.db.Get(), email, r.Context()) // Perbaikan di sini
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		user.Password = "" // Kosongkan password sebelum mengirim response
		json.NewEncoder(w).Encode(user)
	}
}

func (s *Server) handleGetUserByID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := mux.Vars(r)["id"]
		if userID == "" {
			http.Error(w, "User ID is required", http.StatusBadRequest)
			return
		}

		user, err := database.FindUserByID(s.db.Get(), userID, r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		user.Password = "" // Kosongkan password sebelum mengirim response
		json.NewEncoder(w).Encode(user)
	}
}

func (s *Server) handleUpdateUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := mux.Vars(r)["id"]
		var userUpdates database.User
		if err := json.NewDecoder(r.Body).Decode(&userUpdates); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := database.UpdateSingleUser(s.db.Get(), userID, userUpdates, r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Ambil data user yang sudah diupdate untuk dikirim balik (opsional, tapi baik)
		updatedUser, err := database.FindUserByID(s.db.Get(), userID, r.Context())
		if err != nil {
			// Jika error di sini, setidaknya operasi update sudah berhasil.
			// Kirim status OK saja tanpa body, atau log errornya.
			w.WriteHeader(http.StatusOK)
			return
		}
		updatedUser.Password = "" // Kosongkan password

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(updatedUser)
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
	r.HandleFunc("/users", s.handleGetUser()).Methods("GET") // Untuk ?email=xxx atau list semua
	r.HandleFunc("/users/{id}", s.handleGetUserByID()).Methods("GET") // Untuk get by UserID
	r.HandleFunc("/users/{id}", s.handleUpdateUser()).Methods("PUT")
	r.HandleFunc("/users/{id}", s.handleDeleteUser()).Methods("DELETE")
}

