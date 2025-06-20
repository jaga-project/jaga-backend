package auth

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

var jwtKey []byte

func init() {
	err := godotenv.Load()
	if err != nil {
		// Tidak dianggap error fatal jika .env tidak ada,
		// karena variabel mungkin sudah di-set di environment sistem.
		log.Println("auth: .env file not found, relying on system environment variables")
	}

	// Gunakan satu nama variabel environment yang konsisten, misalnya JWT_SECRET
	keyStr := os.Getenv("JWT_SECRET")
	if keyStr == "" {
		log.Fatal("FATAL: JWT_SECRET environment variable not set. Application cannot start securely.")
	}
	jwtKey = []byte(keyStr)
	log.Println("auth: JWT Secret Key loaded successfully.")
}

type Claims struct {
	UserID string `json:"user_id"`
	IsAdmin bool   `json:"is_admin"`
	jwt.RegisteredClaims
}

// GenerateJWT membuat token JWT baru untuk UserID yang diberikan.
func GenerateJWT(userID string, isAdmin bool) (string, time.Time, error) {
	if len(jwtKey) == 0 {
		return "", time.Time{}, errors.New("JWT secret key is not initialized")
	}

	expirationTime := time.Now().Add(24 * time.Hour) // Token berlaku selama 24 jam

	claims := &Claims{
		UserID:  userID,
		IsAdmin: isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expirationTime, nil
}

// ValidateJWT memvalidasi token string dan mengembalikan claims jika valid.
func ValidateJWT(tokenString string) (*Claims, error) {
	// Pastikan jwtKey sudah diinisialisasi.
	if len(jwtKey) == 0 {
		return nil, errors.New("JWT secret key is not initialized")
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Gunakan variabel jwtKey yang sudah diinisialisasi.
		return jwtKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errors.New("token has expired")
		}
		return nil, errors.New("invalid token")
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}