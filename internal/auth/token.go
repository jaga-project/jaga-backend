package auth

import (
	"errors"
	"fmt"
	"time"
	"log" 
  "os"  

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
)

var jwtKey []byte 

// init akan dipanggil secara otomatis ketika package auth di-load.
func init() {
    // Coba load variabel dari .env file. Ini berguna untuk pengembangan lokal.
    // Di lingkungan produksi, variabel environment biasanya di-set langsung oleh sistem.
    err := godotenv.Load()
    if err != nil {
        // Tidak perlu menganggap ini error fatal jika .env tidak ada,
        // karena variabel mungkin sudah di-set di environment sistem.
        log.Println("auth: .env file not found or error loading, relying on system environment variables")
    }

    keyStr := os.Getenv("JWT_SECRET_KEY")
    if keyStr == "" {
        // Jika JWT_SECRET_KEY tidak di-set, ini adalah masalah keamanan serius.
        // Untuk produksi, Anda harus menghentikan aplikasi.
        // Untuk pengembangan, Anda bisa fallback ke default yang tidak aman, TAPI INI TIDAK DIREKOMENDASIKAN.
        log.Fatal("FATAL: JWT_SECRET_KEY environment variable not set. Application cannot start securely.")
        // Alternatif (TIDAK AMAN UNTUK PRODUKSI, HANYA UNTUK DEBUGGING SANGAT AWAL):
        // log.Println("WARNING: JWT_SECRET_KEY not set, using a default insecure key. DO NOT USE IN PRODUCTION.")
        // keyStr = "fallback_insecure_default_key_replace_immediately"
    }
    jwtKey = []byte(keyStr)
    log.Println("auth: JWT Secret Key loaded.") 
}

// Claims adalah struktur untuk data yang akan disimpan dalam token JWT.
type Claims struct {
	UserID string `json:"user_id"`
	// Anda bisa menambahkan role atau klaim lain di sini jika perlu
	// Role string `json:"role,omitempty"`
	jwt.RegisteredClaims
}

// GenerateJWT membuat token JWT baru untuk UserID yang diberikan.
func GenerateJWT(userID string) (string, time.Time, error) {
	expirationTime := time.Now().Add(24 * time.Hour) // Token berlaku selama 24 jam

	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "jaga-app", // Nama aplikasi Anda (opsional)
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expirationTime, nil
}

// ValidateJWT memvalidasi token string dan mengembalikan claims jika token valid.
func ValidateJWT(tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Pastikan metode signing adalah yang diharapkan (HS256)
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errors.New("token has expired")
		}
		if errors.Is(err, jwt.ErrTokenMalformed) {
			return nil, errors.New("malformed token")
		}
		if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
			return nil, errors.New("invalid token signature")
		}
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}