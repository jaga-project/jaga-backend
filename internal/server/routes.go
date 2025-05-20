package server

import (
	"net/http"

	"github.com/gorilla/mux"
)

func (s *Server) RegisterRoutes() http.Handler {
    router := mux.NewRouter()

    // Route root
    router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("Welcome to JAGA"))
    })

    // Route health check / ping
    router.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("pong"))
    })


    s.RegisterUserRoutes(router)

    s.RegisterVehicleRoutes(router)

		s.RegisterDetectedRoutes(router)
		
    // TODO: tambahkan route lain di sini, misal /vehicles, /reports, dsb.
    return router
}




