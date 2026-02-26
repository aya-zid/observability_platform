// internal/infrastructure/jwt.go
package infrastructure

import (
	"context" // added for future context-aware usage
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
)

var cfg *config.Config

type Claims struct {
	UserID    uint   `json:"user_id"`
	TenantID  uint   `json:"tenant_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	jwt.RegisteredClaims
}

func InitJWT(c *config.Config) {
	cfg = c
}

// GenerateToken creates a new JWT
func GenerateToken(userID, tenantID uint, email, role string) (string, error) {
	claims := Claims{
		UserID:   userID,
		TenantID: tenantID,
		Email:    email,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecret))
}

// ParseToken validates and returns claims
func ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrSignatureInvalid
}

// ParseTokenWithContext (future-proof for tracing / cancellation)
func ParseTokenWithContext(_ context.Context, tokenStr string) (*Claims, error) {
	return ParseToken(tokenStr)
}