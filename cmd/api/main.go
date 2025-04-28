package main

import (
    "log"
    "go-backend/internal/database"
    "jaga-project/internal/server"
)

func main() {
    // buka koneksi ke Postgres
    db := database.Connect()

    // inisialisasi HTTP server + routes
    r := server.NewRouter(db)

    log.Println("🚀 Server running on :8080")
    if err := r.Run(":8080"); err != nil {
        log.Fatalf("could not run server: %v", err)
    }
}
