package middleware

import (
	"context"
	"net/http"
	"strings"
	 "database/sql"
	 "log"

	"github.com/jaga-project/jaga-backend/internal/auth" // Paket auth kita
	"github.com/jaga-project/jaga-backend/internal/database"
)


type contextKey string

const UserIDContextKey = contextKey("userID")

const AdminStatusContextKey = contextKey("isAdmin")

// JWTMiddleware memverifikasi token JWT dari Authorization header.
// Jika valid, UserID (dan info lain seperti status admin) akan disimpan di context request.
func JWTMiddleware(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                http.Error(w, "Authorization header required", http.StatusUnauthorized)
                return
            }

            parts := strings.Split(authHeader, " ")
            if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
                http.Error(w, "Invalid Authorization header format (must be Bearer {token})", http.StatusUnauthorized)
                return
            }
            tokenString := parts[1]

            claims, err := auth.ValidateJWT(tokenString)
            if err != nil {
                http.Error(w, err.Error(), http.StatusUnauthorized)
                return
            }

            ctx := context.WithValue(r.Context(), UserIDContextKey, claims.UserID)

            // Cek apakah claims.UserID adalah admin:
            // Sekarang 'db' tersedia dari closure NewJWTMiddleware
            isAdmin, errDb := database.IsUserAdmin(db, claims.UserID) // Menggunakan db yang di-pass
            if errDb != nil {
                // Tangani error database dengan baik, mungkin log dan anggap bukan admin
                // atau kembalikan internal server error jika kritis
                // Untuk sekarang, kita log dan anggap bukan admin jika ada error DB
                log.Printf("Error checking admin status for UserID %s: %v", claims.UserID, errDb)
                isAdmin = false
            }
            ctx = context.WithValue(ctx, AdminStatusContextKey, isAdmin)

            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}