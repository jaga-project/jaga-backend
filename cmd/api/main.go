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
    dbSvc, err := database.New()
    if err != nil {
        log.Fatalf("❌ failed to init database service: %v", err)
    }

    // Health‐check
    if err := dbSvc.Health(context.Background()); err != nil {
        log.Fatalf("❌ database not healthy: %v", err)
    }

    e := server.NewRouter(dbSvc.DB())

    srv := &http.Server{
        Addr:    getAddr(),
        Handler: e,
    }

    go func() {
        log.Printf("🚀 JAGA Backend starting on %s\n", srv.Addr)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("❌ server runtime error: %v", err)
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
    <-quit
    log.Println("🛑 Shutting down server...")

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
