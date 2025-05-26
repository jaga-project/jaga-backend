package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/jaga-project/jaga-backend/internal/auth" // Paket auth kita
)

// contextKey adalah tipe kustom untuk kunci context agar tidak bentrok.
type contextKey string

// UserIDContextKey adalah kunci untuk menyimpan UserID di context.
const UserIDContextKey = contextKey("userID")
// AdminStatusContextKey adalah kunci untuk menyimpan status admin di context.
const AdminStatusContextKey = contextKey("isAdmin")

// JWTMiddleware memverifikasi token JWT dari Authorization header.
// Jika valid, UserID (dan info lain seperti status admin) akan disimpan di context request.
func JWTMiddleware(next http.Handler) http.Handler {
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
			http.Error(w, err.Error(), http.StatusUnauthorized) // ValidateJWT sudah memberikan pesan error yang sesuai
			return
		}

		// Simpan UserID ke context request.
		ctx := context.WithValue(r.Context(), UserIDContextKey, claims.UserID)

		// Anda bisa menambahkan pengecekan role dari database di sini jika claims tidak menyimpannya
		// dan menyimpannya ke context juga.
		// Misalnya, cek apakah claims.UserID adalah admin:
		// isAdmin, _ := database.IsUserAdmin(s.db.Get(), claims.UserID) // Anda perlu fungsi ini
		// ctx = context.WithValue(ctx, AdminStatusContextKey, isAdmin)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}