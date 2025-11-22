package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenClaims represents the JWT claims
type TokenClaims struct {
	Username string                 `json:"sub"`
	Email    string                 `json:"email,omitempty"`
	Name     string                 `json:"name,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	jwt.RegisteredClaims
}

// CreateAccessToken creates a new JWT access token
func CreateAccessToken(user *User, cfg *AuthConfig) (string, error) {
	expirationTime := time.Now().Add(time.Duration(cfg.JWTExpireMinutes) * time.Minute)
	
	claims := &TokenClaims{
		Username: user.Username,
		Email:    user.Email,
		Name:     user.FullName,
		Metadata: user.Metadata,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "juicytrade",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecretKey))
}

// ValidateToken validates a JWT token and returns the user
func ValidateToken(tokenString string, cfg *AuthConfig) (*User, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.JWTSecretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		return &User{
			Username:  claims.Username,
			Email:     claims.Email,
			FullName:  claims.Name,
			LastLogin: time.Now(), // Approximate
			Metadata:  claims.Metadata,
		}, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}
