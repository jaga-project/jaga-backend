package server

import (
	"encoding/json" 
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/middleware" 
)

func (s *Server) RegisterRoutes() http.Handler {
	mainRouter := mux.NewRouter()

	// Rute root dan ping
	mainRouter.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json") // Set Content-Type header
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "Welcome to JAGA API"}) // Encode and send JSON
	}).Methods("GET")
	mainRouter.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "pong"})
	}).Methods("GET")


	// Rute Registrasi User (POST /users) harus publik
	userPublicRouter := mainRouter.PathPrefix("/users").Subrouter()
	userPublicRouter.HandleFunc("", s.handleCreateUser()).Methods("POST")

	// Rute untuk mengakses file statis (misalnya gambar KTP) di direktori uploads
	fs := http.FileServer(http.Dir("./uploads/"))
  mainRouter.PathPrefix("/uploads/").Handler(http.StripPrefix("/uploads/", fs))

	// --- Rute API yang Dilindungi ---
	apiRouter := mainRouter.PathPrefix("/api").Subrouter()


	// Buat instance middleware JWT dengan meneruskan koneksi database
	// Asumsi s.db.Get() mengembalikan *sql.DB atau tipe yang diharapkan oleh NewJWTMiddleware
  jwtAuthMiddleware := middleware.JWTMiddleware()
  apiRouter.Use(jwtAuthMiddleware) // Gunakan instance middleware yang sudah dibuat

	// Rute Autentikasi (Login)
	s.RegisterAuthRoutes(mainRouter)

	s.RegisterUserProtectedRoutes(apiRouter)
	s.RegisterVehicleRoutes(apiRouter)      
	s.RegisterDetectedRoutes(apiRouter)    
	s.RegisterAdminRoutes(apiRouter)         
	s.RegisterCameraRoutes(apiRouter)        
	s.RegisterLostReportRoutes(apiRouter)    
	s.RegisterSuspectRoutes(apiRouter)       
	s.RegisterImageRoutes(apiRouter) 

	return mainRouter
}







