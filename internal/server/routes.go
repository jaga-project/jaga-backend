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
        w.Write([]byte("Welcome to JAGA Backend!"))
    })

    // Route health check / ping
    router.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("pong"))
    })

    // User routes
    s.RegisterUserRoutes(router)

    // TODO: tambahkan route lain di sini, misal /vehicles, /reports, dsb.
    return router
}




