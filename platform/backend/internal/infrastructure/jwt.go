// internal/infrastructure/jwt.go
package infrastructure

import (
	"context" // added for future context-aware usage
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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

type AdminClaims struct {
	AdminID uint   `json:"admin_id"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	jwt.RegisteredClaims
}

func InitJWT(c *config.Config) {
	cfg = c
}

// GenerateToken creates a new tenant JWT
func GenerateToken(userID, tenantID uint, email, role string) (string, error) {
	if cfg == nil {
		return "", errors.New("config not initialized")
	}
	claims := Claims{
		UserID:   userID,
		TenantID: tenantID,
		Email:    email,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.JWTIssuer,
			Subject:   strconv.FormatUint(uint64(userID), 10),
			Audience:  []string{cfg.JWTAudience},
			ID:        uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(cfg.AccessTokenTTLMinutes) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecret))
}

// GenerateAdminToken creates a new admin JWT
func GenerateAdminToken(adminID uint, email, role string) (string, error) {
	if cfg == nil {
		return "", errors.New("config not initialized")
	}
	claims := AdminClaims{
		AdminID: adminID,
		Email:   email,
		Role:    role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.JWTIssuer + "-admin",
			Subject:   strconv.FormatUint(uint64(adminID), 10),
			Audience:  []string{cfg.JWTAudience + "-admin"},
			ID:        uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(cfg.AccessTokenTTLMinutes) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecret))
}

// ParseToken validates and returns claims
func ParseToken(tokenStr string) (*Claims, error) {
	if cfg == nil {
		return nil, errors.New("config not initialized")
	}
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method == nil || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		if claims.Issuer != cfg.JWTIssuer {
			return nil, jwt.ErrTokenInvalidIssuer
		}
		audOK := false
		for _, a := range claims.Audience {
			if a == cfg.JWTAudience {
				audOK = true
				break
			}
		}
		if !audOK {
			return nil, jwt.ErrTokenInvalidAudience
		}
		return claims, nil
	}

	return nil, jwt.ErrSignatureInvalid
}

// ParseAdminToken validates and returns admin claims
func ParseAdminToken(tokenStr string) (*AdminClaims, error) {
	if cfg == nil {
		return nil, errors.New("config not initialized")
	}
	token, err := jwt.ParseWithClaims(tokenStr, &AdminClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method == nil || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*AdminClaims); ok && token.Valid {
		if claims.Issuer != cfg.JWTIssuer+"-admin" {
			return nil, jwt.ErrTokenInvalidIssuer
		}
		audOK := false
		for _, a := range claims.Audience {
			if a == cfg.JWTAudience+"-admin" {
				audOK = true
				break
			}
		}
		if !audOK {
			return nil, jwt.ErrTokenInvalidAudience
		}
		return claims, nil
	}

	return nil, jwt.ErrSignatureInvalid
}

// ParseTokenWithContext (future-proof for tracing / cancellation)
func ParseTokenWithContext(_ context.Context, tokenStr string) (*Claims, error) {
	return ParseToken(tokenStr)
}
