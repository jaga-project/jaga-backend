package database

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/joho/godotenv"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

type Service interface {
    Health(ctx context.Context) error
    DB() *gorm.DB
}

type service struct {
    db *gorm.DB
}

func New() (Service, error) {
    // Load .env (sekali) — pakai autoload atau explicit
    _ = godotenv.Load()

    // Ambil DATABASE_URL atau build DSN
    dsn := os.Getenv("DATABASE_URL")
    if dsn == "" {
        dsn = fmt.Sprintf(
            "host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
            os.Getenv("DB_HOST"),
            os.Getenv("DB_USER"),
            os.Getenv("DB_PASSWORD"),
            os.Getenv("DB_NAME"),
            os.Getenv("DB_PORT"),
            os.Getenv("DB_SSLMODE"),
        )
    }

    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        return nil, fmt.Errorf("failed to connect database: %w", err)
    }

    return &service{db: db}, nil
}

// Health melakukan ping ke DB dengan timeout
func (s *service) Health(ctx context.Context) error {
    sqlDB, err := s.db.DB()
    if err != nil {
        return fmt.Errorf("getting sql.DB: %w", err)
    }

    // Pakai konteks timeout agar tidak nunggu selamanya
    ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
    defer cancel()

    if err := sqlDB.PingContext(ctx); err != nil {
        return fmt.Errorf("database ping failed: %w", err)
    }
    return nil
}

// DB expose *gorm.DB
func (s *service) DB() *gorm.DB {
    return s.db
}
