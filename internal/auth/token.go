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
		log.Println("auth: .env file not found, relying on system environment variables")
	}

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

func GenerateJWT(userID string, isAdmin bool) (string, time.Time, error) {
	if len(jwtKey) == 0 {
		return "", time.Time{}, errors.New("JWT secret key is not initialized")
	}

	expirationTime := time.Now().Add(24 * time.Hour) 

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

func ValidateJWT(tokenString string) (*Claims, error) {
	if len(jwtKey) == 0 {
		return nil, errors.New("JWT secret key is not initialized")
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
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