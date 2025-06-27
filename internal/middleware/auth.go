package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jaga-project/jaga-backend/internal/auth"
	"github.com/jaga-project/jaga-backend/internal/database"
)

type contextKey string

const UserIDContextKey = contextKey("userID")
const AdminStatusContextKey = contextKey("isAdmin")

func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func UnifiedAuthMiddleware(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey != "" {
				user, err := database.ValidateAPIKeyAndGetUser(r.Context(), db, apiKey)
				if err != nil {
					writeJSONError(w, "Forbidden: Invalid API Key", http.StatusForbidden)
					return
				}
				
				isAdmin, err := database.IsUserAdmin(db, user.UserID)
				if err != nil {
					writeJSONError(w, "Failed to verify admin status for API key user", http.StatusInternalServerError)
					return
				}
				ctx := context.WithValue(r.Context(), UserIDContextKey, user.UserID)
				ctx = context.WithValue(ctx, AdminStatusContextKey, isAdmin) 
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSONError(w, "Unauthorized: Missing Authorization header", http.StatusUnauthorized)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenStr == authHeader {
				writeJSONError(w, "Unauthorized: Invalid token format", http.StatusUnauthorized)
				return
			}

			claims, err := auth.ValidateJWT(tokenStr)
			if err != nil {
				writeJSONError(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDContextKey, claims.UserID)
			ctx = context.WithValue(ctx, AdminStatusContextKey, claims.IsAdmin)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func AdminOnlyMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			isAdmin, ok := r.Context().Value(AdminStatusContextKey).(bool)

			if !ok || !isAdmin {
				writeJSONError(w, "Forbidden: Administrator access required", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}