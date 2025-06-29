package server

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jaga-project/jaga-backend/internal/middleware"
)

func (s *Server) RegisterRoutes() http.Handler {
	mainRouter := mux.NewRouter()

	mainRouter.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json") 
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "Welcome to JAGA API"}) 
	}).Methods("GET")
	mainRouter.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "pong"})
	}).Methods("GET")

	userPublicRouter := mainRouter.PathPrefix("/users").Subrouter()
	userPublicRouter.HandleFunc("", s.handleCreateUser()).Methods("POST")

	fs := http.FileServer(http.Dir("./uploads/"))
	mainRouter.PathPrefix("/uploads/").Handler(http.StripPrefix("/uploads/", fs))

	s.RegisterAuthRoutes(mainRouter)

	publicApiRouter := mainRouter.PathPrefix("/api").Subrouter()
	s.RegisterPublicCameraRoutes(publicApiRouter)

	apiRouter := mainRouter.PathPrefix("/api").Subrouter()
	apiRouter.Use(middleware.UnifiedAuthMiddleware(s.db.Get()))

	adminOnlyMiddleware := middleware.AdminOnlyMiddleware()

	adminRouter := apiRouter.PathPrefix("/admins").Subrouter()
	adminRouter.Use(adminOnlyMiddleware)
	s.RegisterAdminRoutes(adminRouter)


	s.RegisterUserProtectedRoutes(apiRouter)
	s.RegisterVehicleRoutes(apiRouter)
	s.RegisterDetectedRoutes(apiRouter)
	s.RegisterProtectedCameraRoutes(apiRouter)
	s.RegisterLostReportRoutes(apiRouter)
	s.RegisterSuspectRoutes(apiRouter)
	s.RegisterImageRoutes(apiRouter)
	s.RegisterResultRoutes(apiRouter)

	return mainRouter
}







