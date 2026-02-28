// internal/cache/tenant_cache.go
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	tenantKeyPrefix = "tenant:apikey:"
	tenantTTL       = 5 * time.Minute
)

// CachedTenant holds only what the proxy needs — resolved once, reused many times
type CachedTenant struct {
	ID   uint   `json:"id"`   // uint PK → used in vminsert URL  /insert/{ID}/prometheus/
	UUID string `json:"uuid"` // stable UUID → used as X-Scope-OrgID for Loki + Tempo
}

type TenantCache struct {
	client *redis.Client
}

func NewTenantCache(client *redis.Client) *TenantCache {
	return &TenantCache{client: client}
}

// Get resolves an API key to a CachedTenant. Returns nil, nil if not found (cache miss).
func (c *TenantCache) Get(ctx context.Context, apiKey string) (*CachedTenant, error) {
	key := tenantKeyPrefix + apiKey
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // cache miss — caller must hit DB
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}

	var tenant CachedTenant
	if err := json.Unmarshal([]byte(val), &tenant); err != nil {
		return nil, fmt.Errorf("unmarshal cached tenant: %w", err)
	}
	return &tenant, nil
}

// Set stores a resolved tenant under its API key with a 5-minute TTL.
func (c *TenantCache) Set(ctx context.Context, apiKey string, tenant *CachedTenant) error {
	key := tenantKeyPrefix + apiKey
	data, err := json.Marshal(tenant)
	if err != nil {
		return fmt.Errorf("marshal tenant: %w", err)
	}
	return c.client.Set(ctx, key, data, tenantTTL).Err()
}

// Invalidate removes an API key from cache immediately (use on key rotation or revocation).
func (c *TenantCache) Invalidate(ctx context.Context, apiKey string) error {
	return c.client.Del(ctx, tenantKeyPrefix+apiKey).Err()
}