package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
	"strings"

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

	corsOriginsStr := os.Getenv("CORS_ALLOWED_ORIGINS")
    if corsOriginsStr == "" {
        corsOriginsStr = "http://localhost:3000"
        log.Printf("WARN: CORS_ALLOWED_ORIGINS environment variable not set. Defaulting to '%s'", corsOriginsStr)
    }
    allowedOriginsList := strings.Split(corsOriginsStr, ",")
    log.Printf("INFO: Configuring CORS with allowed origins: %v", allowedOriginsList)

    allowedOrigins := handlers.AllowedOrigins(allowedOriginsList)
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



