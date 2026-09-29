package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const tokenTTL = 12 * time.Hour

// Admin is the authenticated administrator carried by a token.
type Admin struct {
	ID    string
	Name  string
	Email string
}

type sessionClaims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	jwt.RegisteredClaims
}

// SignToken creates a signed admin token.
func SignToken(secret string, admin Admin, now time.Time) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("jwt secret is empty")
	}

	claims := sessionClaims{
		Email: admin.Email,
		Name:  admin.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   admin.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// ParseToken validates a signed admin token.
func ParseToken(secret, raw string, now time.Time) (Admin, error) {
	if secret == "" || raw == "" {
		return Admin{}, fmt.Errorf("invalid token")
	}

	claims := &sessionClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	}, jwt.WithTimeFunc(func() time.Time { return now }), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid {
		return Admin{}, fmt.Errorf("invalid token")
	}
	if claims.Subject == "" || claims.Email == "" {
		return Admin{}, fmt.Errorf("invalid token")
	}

	return Admin{
		ID:    claims.Subject,
		Name:  claims.Name,
		Email: claims.Email,
	}, nil
}
