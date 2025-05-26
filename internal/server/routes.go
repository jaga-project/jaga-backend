package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/middleware" // Import middleware
)

func (s *Server) RegisterRoutes() http.Handler {
	mainRouter := mux.NewRouter()

	// --- Rute Publik ---
	// Rute root dan ping
	mainRouter.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Welcome to JAGA API"))
	}).Methods("GET")
	mainRouter.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	}).Methods("GET")

	// Rute Autentikasi (Login)
	s.RegisterAuthRoutes(mainRouter) // Ini akan mendaftarkan /auth/login

	// Rute Registrasi User (POST /users) - ini harus publik
	// Kita akan mendaftarkannya secara terpisah agar tidak masuk ke subrouter API yang dilindungi
	userPublicRouter := mainRouter.PathPrefix("/users").Subrouter()
	userPublicRouter.HandleFunc("", s.handleCreateUser()).Methods("POST")


	// --- Rute yang Dilindungi (Memerlukan Autentikasi JWT) ---
	apiRouter := mainRouter.PathPrefix("/api").Subrouter()
	apiRouter.Use(middleware.JWTMiddleware) // Terapkan middleware ke semua rute di bawah /api

	// Daftarkan rute-rute lain yang memerlukan autentikasi di bawah apiRouter
	// Pastikan fungsi Register...Routes ini sekarang mendaftarkan ke router yang diberikan (apiRouter)
	// dan tidak lagi menyertakan rute publik seperti registrasi user.

	s.RegisterUserProtectedRoutes(apiRouter) // Fungsi baru untuk rute user yang dilindungi
	s.RegisterVehicleRoutes(apiRouter)       // Asumsi semua rute vehicle dilindungi
	s.RegisterDetectedRoutes(apiRouter)      // Asumsi semua rute detected dilindungi
	s.RegisterAdminRoutes(apiRouter)         // Asumsi semua rute admin dilindungi (mungkin perlu middleware admin tambahan)
	s.RegisterCameraRoutes(apiRouter)        // Asumsi semua rute camera dilindungi
	s.RegisterLostReportRoutes(apiRouter)    // Semua rute lost report sekarang dilindungi
	s.RegisterSuspectRoutes(apiRouter)       // Asumsi semua rute suspect dilindungi
	s.RegisterImageRoutes(apiRouter) // Pastikan baris ini ada dan tidak di-comment

	return mainRouter
}

// RegisterUserProtectedRoutes mendaftarkan rute user yang memerlukan autentikasi.
// Ini tidak termasuk POST /users (registrasi).
func (s *Server) RegisterUserProtectedRoutes(r *mux.Router) {
	r.HandleFunc("/users", s.handleGetUser()).Methods("GET") // GET /api/users (list semua user, mungkin hanya untuk admin)
	r.HandleFunc("/users/{id}", s.handleGetUserByID()).Methods("GET") // GET /api/users/{id} (get user by ID)
	r.HandleFunc("/users/{id}", s.handleUpdateUser()).Methods("PUT") // PUT /api/users/{id} (update user, biasanya user sendiri atau admin)
	r.HandleFunc("/users/{id}", s.handleDeleteUser()).Methods("DELETE") // DELETE /api/users/{id} (delete user, biasanya admin)
}

// Pastikan fungsi Register...Routes lainnya (RegisterVehicleRoutes, dll.)
// sekarang hanya berisi definisi rute dan tidak membuat subrouter sendiri
// atau menerapkan middleware sendiri. Mereka akan menerima router (apiRouter)
// yang sudah memiliki middleware.
// Contoh:
// func (s *Server) RegisterLostReportRoutes(r *mux.Router) {
//     r.HandleFunc("/lost_reports", s.handleCreateLostReport()).Methods("POST")
//     r.HandleFunc("/lost_reports", s.handleGetLostReport()).Methods("GET")
//     r.HandleFunc("/lost_reports/{id}", s.handleGetLostReport()).Methods("GET")
//     r.HandleFunc("/lost_reports/{id}", s.handleUpdateLostReport()).Methods("PUT")
//     r.HandleFunc("/lost_reports/{id}", s.handleDeleteLostReport()).Methods("DELETE")
// }
// Anda perlu menyesuaikan semua fungsi Register...Routes Anda agar sesuai dengan pola ini.




