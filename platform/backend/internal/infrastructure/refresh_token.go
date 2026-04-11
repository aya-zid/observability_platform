package infrastructure

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
)

const (
	refreshTokenPrefix = "rt_"
)

// GenerateRefreshToken returns an opaque refresh token (returned to the client) and its HMAC hash (stored server-side).
func GenerateRefreshToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("rand read: %w", err)
	}
	raw = refreshTokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	hash, err = HashRefreshToken(raw)
	if err != nil {
		return "", "", err
	}
	return raw, hash, nil
}

func HashRefreshToken(raw string) (string, error) {
	if cfg == nil {
		return "", errors.New("config not initialized")
	}
	if cfg.RefreshTokenPepper == "" {
		return "", errors.New("REFRESH_TOKEN_PEPPER is required")
	}
	m := hmac.New(sha256.New, []byte(cfg.RefreshTokenPepper))
	_, _ = m.Write([]byte(raw))
	return hex.EncodeToString(m.Sum(nil)), nil
}
