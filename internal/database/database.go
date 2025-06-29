package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/joho/godotenv/autoload"
	_ "github.com/lib/pq"
)

type Service interface {
	Health() map[string]string
	Get() *sql.DB
}

type service struct {
	db *sql.DB
}

func New() Service {
	connStr := os.Getenv("POSTGRES_URI")
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	return &service{
		db: db,
	}
}

func (s *service) Health() map[string]string {
	err := s.db.Ping()
	if err != nil {
		log.Fatalf("%s", fmt.Sprintf("db down: %v", err))
	}
    log.Printf("db up: %v", err)
	return map[string]string{
		"message": "It's healthy",
	}
}

func (s *service) Get() *sql.DB {
	return s.db
}