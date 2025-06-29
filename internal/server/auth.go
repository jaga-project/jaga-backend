package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/auth"
	"github.com/jaga-project/jaga-backend/internal/database"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token      string    `json:"token"`
	ExpiresAt  time.Time `json:"expires_at"`
	UserID     string    `json:"user_id"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	IsAdmin    bool      `json:"is_admin"`
	KTPImageID *int64    `json:"ktp_image_id,omitempty"`
	NIK        string    `json:"nik"`
	Phone      string    `json:"phone"`
}

func (s *Server) handleLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
			return
		}

		if req.Email == "" || req.Password == "" {
			writeJSONError(w, "Email and password are required", http.StatusBadRequest)
			return
		}

		user, err := database.FindSingleUser(s.db.Get(), req.Email, r.Context())
		if err != nil {
			if err.Error() == "user not found" {
				writeJSONError(w, "Invalid email or password", http.StatusUnauthorized)
			} else {
				writeJSONError(w, "Error finding user: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
		if err != nil {
			writeJSONError(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}

		isAdmin, err := database.IsUserAdmin(s.db.Get(), user.UserID)
		if err != nil {
			log.Printf("Failed to check admin status for %s during login: %v", user.UserID, err)
			isAdmin = false
		}

		tokenString, expiresAt, err := auth.GenerateJWT(user.UserID, isAdmin)
		if err != nil {
			writeJSONError(w, "Failed to generate token: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LoginResponse{
			Token:      tokenString,
			ExpiresAt:  expiresAt,
			UserID:     user.UserID,
			Name:       user.Name,
			Email:      user.Email,
			IsAdmin:    isAdmin,
			KTPImageID: user.KTPImageID,
			NIK:        user.NIK,
			Phone:      user.Phone,
		})
	}
}

func (s *Server) RegisterAuthRoutes(r *mux.Router) {
	r.HandleFunc("/auth/login", s.handleLogin()).Methods("POST")
}



