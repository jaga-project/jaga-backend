package database

import (
	"context"
	"database/sql"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

func ValidateAPIKeyAndGetUser(ctx context.Context, db *sql.DB, apiKey string) (*User, error) {
    rows, err := db.QueryContext(ctx, "SELECT user_id, key_hash FROM service_api_keys")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var validUserID string

    for rows.Next() {
        var userID, keyHash string
        if err := rows.Scan(&userID, &keyHash); err != nil {
            return nil, err
        }

        err := bcrypt.CompareHashAndPassword([]byte(keyHash), []byte(apiKey))
        if err == nil {
            validUserID = userID
            break
        }
    }

    if validUserID == "" {
        return nil, errors.New("invalid API key")
    }

    // Setelah mendapatkan UserID, ambil detail lengkap user tersebut menggunakan fungsi yang ada.
    user, err := FindUserByID(db, validUserID, ctx)
    if err != nil {
        return nil, err
    }

    // Update last_used_at (opsional, tapi praktik yang baik).
    // Dijalankan sebagai goroutine agar tidak memblokir response.
    go db.ExecContext(context.Background(), "UPDATE service_api_keys SET last_used_at = NOW() WHERE user_id = $1", validUserID)

    return user, nil
}