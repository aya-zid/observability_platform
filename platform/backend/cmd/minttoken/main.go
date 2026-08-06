// Command minttoken prints a signed JWT for local API testing against the dev
// platform backend. It reuses the same infrastructure package the backend uses,
// so the token is guaranteed to validate.
//
// Usage:
//
//	go run ./cmd/minttoken [userID] [tenantID] [email] [role]
//
// Defaults match the local dev seed (admin on tenant 1). Token is valid for 60
// minutes and is used as:  Authorization: Bearer <token>
package main

import (
	"fmt"
	"os"
	"strconv"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
)

func main() {
	userID := uintArg(1, 1)
	tenantID := uintArg(2, 1)
	email := strArg(3, "azid28278@gmail.com")
	role := strArg(4, "admin")

	infrastructure.InitJWT(&config.Config{
		JWTSecret:             envOr("JWT_SECRET", "super-secret-change-me-in-prod-32-chars-min"),
		JWTIssuer:             envOr("JWT_ISSUER", "obs-platform"),
		JWTAudience:           envOr("JWT_AUDIENCE", "obs-api"),
		AccessTokenTTLMinutes: 60,
	})

	tok, err := infrastructure.GenerateToken(userID, tenantID, email, role)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println(tok)
}

func uintArg(i int, def uint) uint {
	if i < len(os.Args) {
		if v, err := strconv.ParseUint(os.Args[i], 10, 64); err == nil {
			return uint(v)
		}
	}
	return def
}

func strArg(i int, def string) string {
	if i < len(os.Args) && os.Args[i] != "" {
		return os.Args[i]
	}
	return def
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
