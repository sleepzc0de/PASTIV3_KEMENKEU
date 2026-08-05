package utils

import (
	"errors"
	"time"

	"pasti-v3-backend/config"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func GenerateAccessToken(userID, username, role string) (string, int, error) {
	cfg := config.Cfg
	expireMinutes := cfg.JWTAccessExpireMin
	expirationTime := time.Now().Add(time.Duration(expireMinutes) * time.Minute)

	claims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "pasti-v3",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		return "", 0, err
	}

	return signed, expireMinutes * 60, nil
}

func ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("metode signing tidak valid")
		}
		return []byte(config.Cfg.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, errors.New("token tidak valid atau sudah kedaluwarsa")
	}

	return claims, nil
}
