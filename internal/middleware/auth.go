package middleware

import (
	"context"
	"encoding/json"
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
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "Authorization header required"})
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "Invalid Authorization header format (must be Bearer {token})"})
				return
			}
			tokenString := parts[1]

			claims, err := auth.ValidateJWT(tokenString)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}

			// Ambil UserID dan IsAdmin langsung dari claims token yang sudah tervalidasi
			ctx := context.WithValue(r.Context(), UserIDContextKey, claims.UserID)
			ctx = context.WithValue(ctx, AdminStatusContextKey, claims.IsAdmin)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func AdminOnlyMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Ambil status admin dari konteks yang sudah di-set oleh JWTMiddleware.
            isAdmin, ok := r.Context().Value(AdminStatusContextKey).(bool)

            // Jika status tidak ada (bukan boolean) atau false, tolak akses.
            if !ok || !isAdmin {
                w.Header().Set("Content-Type", "application/json")
                w.WriteHeader(http.StatusForbidden)
                json.NewEncoder(w).Encode(map[string]string{"error": "Forbidden: Administrator access required"})
                return
            }

            // Jika user adalah admin, lanjutkan ke handler berikutnya.
            next.ServeHTTP(w, r)
        })
    }
}