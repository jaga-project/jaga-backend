package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/jaga-project/jaga-backend/internal/database"
    "github.com/jaga-project/jaga-backend/internal/server"
)

func main() {
    // 1️⃣ Inisialisasi DB service
    dbSvc, err := database.New()
    if err != nil {
        log.Fatalf("❌ failed to init database service: %v", err)
    }

    // 2️⃣ Health‐check sebelum mulai
    if err := dbSvc.Health(context.Background()); err != nil {
        log.Fatalf("❌ database not healthy: %v", err)
    }

    // 3️⃣ Buat router & inject *gorm.DB
    e := server.NewRouter(dbSvc.DB())

    // 4️⃣ Bungkus Echo ke dalam http.Server untuk graceful shutdown
    srv := &http.Server{
        Addr:    getAddr(),
        Handler: e,
    }

    // 5️⃣ Jalankan server di goroutine
    go func() {
        log.Printf("🚀 JAGA Backend starting on %s\n", srv.Addr)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("❌ server runtime error: %v", err)
        }
    }()

    // 6️⃣ Tunggu sinyal OS (CTRL+C / SIGTERM)
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
    <-quit
    log.Println("🛑 Shutting down server...")

    // 7️⃣ Graceful shutdown dengan timeout
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        log.Fatalf("❌ server forced to shutdown: %v", err)
    }

    log.Println("✅ Server stopped gracefully")
}

func getAddr() string {
    if addr := os.Getenv("SERVER_ADDR"); addr != "" {
        return addr
    }
    return ":8080"
}
