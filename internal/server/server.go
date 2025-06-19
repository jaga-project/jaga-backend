package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/handlers" 
	"github.com/jaga-project/jaga-backend/internal/database"

	_ "github.com/joho/godotenv/autoload"
	_ "github.com/lib/pq"
)

type Server struct {
	port int
	db   database.Service
}

func NewServer() *http.Server {
	portStr := os.Getenv("PORT")
	port, err := strconv.Atoi(portStr)
	if err != nil || port == 0 {
		log.Fatalf("PORT environment variable is not set or invalid: got '%s'", portStr)
	}

	newServer := &Server{
		port: port,
		db:   database.New(),
	}

	allowedOrigins := handlers.AllowedOrigins([]string{"http://localhost:3000", "http://100.115.148.124","http://100.115.148.124:3000","http://100.72.88.10"}) // Tambahkan IP atau domain frontend Anda
	allowedMethods := handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	allowedHeaders := handlers.AllowedHeaders([]string{"Content-Type", "Authorization", "X-Requested-With"})
	allowCredentials := handlers.AllowCredentials()

	mainHandler := newServer.RegisterRoutes()

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", newServer.port),
		Handler:      handlers.CORS(allowedOrigins, allowedMethods, allowedHeaders, allowCredentials)(mainHandler),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server
}



