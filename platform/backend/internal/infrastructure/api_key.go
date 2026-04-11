package infrastructure

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

const (
	apiKeyPrefixText   = "obs_live_"
	apiKeyEntropyBytes = 32 // 256-bit random secret
)

// GenerateTenantAPIKey returns a new tenant API key.
//
// NOTE: This returns a secret key that will be stored in clear text by request.
func GenerateTenantAPIKey() (rawKey string, err error) {
	b := make([]byte, apiKeyEntropyBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand read: %w", err)
	}

	rawKey = apiKeyPrefixText + base64.RawURLEncoding.EncodeToString(b)
	return rawKey, nil
}
