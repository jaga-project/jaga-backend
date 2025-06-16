package server

import (
	"encoding/json"
	"net/http"
	"time"
	"log"
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
			http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
			return
		}

		if req.Email == "" || req.Password == "" {
			http.Error(w, "Email and password are required", http.StatusBadRequest)
			return
		}

		user, err := database.FindSingleUser(s.db.Get(), req.Email, r.Context())
		if err != nil {
			if err.Error() == "user not found" {
				http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			} else {
				http.Error(w, "Error finding user: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
		if err != nil {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}

		tokenString, expiresAt, err := auth.GenerateJWT(user.UserID)
		if err != nil {
			http.Error(w, "Failed to generate token: "+err.Error(), http.StatusInternalServerError)
			return
		}

		isAdmin := false
		_, err = database.GetAdminByUserID(r.Context(), s.db.Get(), user.UserID)
		if err == nil { 
			isAdmin = true
		} else if err.Error() != "admin not found" { 
			log.Printf("Error checking admin status for user %s: %v", user.UserID, err)
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
	authRouter := r.PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/login", s.handleLogin()).Methods("POST")

	// Rute registrasi user (handleCreateUser) bisa tetap di RegisterUserRoutes
	// atau dipindahkan ke sini jika Anda ingin semua yang terkait auth ada di satu tempat.
	// Jika tetap di RegisterUserRoutes, pastikan itu adalah rute publik.
	// Contoh:
	// userRouter := r.PathPrefix("/users").Subrouter() // Ini akan menjadi publik
	// userRouter.HandleFunc("", s.handleCreateUser()).Methods("POST") // Endpoint registrasi
}



