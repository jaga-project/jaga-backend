package database

import (
    "fmt"
    "log"
    "os"

    "github.com/joho/godotenv"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

// Connect membuka koneksi ke Postgres dan meng-returns *gorm.DB
func Connect() *gorm.DB {
    // load .env (jika ada)
    if err := godotenv.Load(); err != nil {
        log.Println("⚠️  .env not found, reading env vars directly")
    }

    // jika kamu pakai DATABASE_URL:
    if url := os.Getenv("DATABASE_URL"); url != "" {
        db, err := gorm.Open(postgres.Open(url), &gorm.Config{})
        if err != nil {
            log.Fatalf("failed to connect database: %v", err)
        }
        return db
    }

    // atau build DSN manual
    dsn := fmt.Sprintf(
        "host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
        os.Getenv("DB_HOST"),
        os.Getenv("DB_USER"),
        os.Getenv("DB_PASSWORD"),
        os.Getenv("DB_NAME"),
        os.Getenv("DB_PORT"),
    )

    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        log.Fatalf("failed to connect database: %v", err)
    }

    // Optional: auto‐migrate semua model
    // db.AutoMigrate(&User{}, &Admin{}, &Vehicle{}, &Camera{}, &Detected{}, &LostReport{})

    return db
}
