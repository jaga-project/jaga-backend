package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/jaga-project/jaga-backend/internal/auth"
)

type contextKey string

const UserIDContextKey = contextKey("userID")
const AdminStatusContextKey = contextKey("isAdmin")

// JWTMiddleware sekarang tidak perlu parameter 'db'
func JWTMiddleware() func(http.Handler) http.Handler {
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

			// Ambil UserID dan IsAdmin langsung dari claims token yang sudah tervalidasi
			ctx := context.WithValue(r.Context(), UserIDContextKey, claims.UserID)
			ctx = context.WithValue(ctx, AdminStatusContextKey, claims.IsAdmin)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}