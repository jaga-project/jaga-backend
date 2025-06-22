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

		// Cek status admin SEKALI di sini
		isAdmin, err := database.IsUserAdmin(s.db.Get(), user.UserID)
		if err != nil {
			// Tangani error, log saja dan anggap bukan admin untuk keamanan
			log.Printf("Failed to check admin status for %s during login: %v", user.UserID, err)
			isAdmin = false
		}

		tokenString, expiresAt, err := auth.GenerateJWT(user.UserID, isAdmin)
		if err != nil {
			writeJSONError(w, "Failed to generate token: "+err.Error(), http.StatusInternalServerError)
			return
		}

		//token dan detail user
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

// RegisterAuthRoutes mendaftarkan rute untuk autentikasi.
func (s *Server) RegisterAuthRoutes(r *mux.Router) {
	r.HandleFunc("/auth/login", s.handleLogin()).Methods("POST")

	// Rute registrasi user (handleCreateUser) bisa tetap di RegisterUserRoutes
	// atau dipindahkan ke sini jika Anda ingin semua yang terkait auth ada di satu tempat.
	// Jika tetap di RegisterUserRoutes, pastikan itu adalah rute publik.
	// Contoh:
	// r.HandleFunc("/users", s.handleCreateUser()).Methods("POST") // Endpoint registrasi publik
}



