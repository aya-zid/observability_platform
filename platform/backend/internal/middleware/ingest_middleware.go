// internal/middleware/ingest_middleware.go
package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/cache"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

var lastUsedTracker = make(map[uint]time.Time)

const TenantContextKey = "ingest_tenant"

// IngestMiddleware authenticates agent requests via API key.
//
// Flow:
//  1. Extract API key from Authorization: Basic header (key used as both user and password)
//  2. Check Redis cache (TTL = 5 min) — ~0ms hit after first request
//  3. On miss: hit Postgres, populate cache
//  4. Attach CachedTenant to Echo context for proxy handlers
func IngestMiddleware(tenantCache *cache.TenantCache, tenantRepo *repository.TenantRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			rawAPIKey, _, ok := c.Request().BasicAuth()
			if !ok || rawAPIKey == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "missing or invalid Authorization header — use Basic auth with your API key",
				})
			}

			ctx := c.Request().Context()

			// ── 1. Redis cache lookup ──────────────────────────────────
			cached, err := tenantCache.Get(ctx, rawAPIKey)
			if err != nil {
				// Redis is down — log and fall through to DB (degraded mode)
				c.Logger().Errorf("[ingest] redis get error: %v", err)
			}

		if cached != nil {
			if !cached.IsActive || !cached.APIKeyEnabled {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "api key disabled"})
			}
			c.Set(TenantContextKey, cached)
			// Async last-used tracking (debounced to 1 min)
			updateLastUsed(tenantRepo, cached.ID)
			return next(c)
		}

			// ── 2. DB lookup on cache miss ─────────────────────────────
			tenant, err := tenantRepo.FindByAPIKey(ctx, rawAPIKey)
			if err != nil {
				c.Logger().Errorf("[ingest] tenant lookup error: %v", err)
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid API key"})
			}
			if tenant == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid API key"})
			}
			if !tenant.IsActive || !tenant.APIKeyEnabled {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "api key disabled"})
			}

			resolved := &cache.CachedTenant{
				ID:            tenant.ID,
				UUID:          tenant.UUID,
				IsActive:      tenant.IsActive,
				APIKeyEnabled: tenant.APIKeyEnabled,
			}

			// ── 3. Populate cache (best-effort — don't fail on Redis error) ──
			if cacheErr := tenantCache.Set(ctx, rawAPIKey, resolved); cacheErr != nil {
				c.Logger().Warnf("[ingest] redis set error (non-fatal): %v", cacheErr)
			}

			c.Set(TenantContextKey, resolved)
			// Async last-used tracking (debounced to 1 min)
			updateLastUsed(tenantRepo, resolved.ID)
			return next(c)
		}
	}
}

func updateLastUsed(tenantRepo *repository.TenantRepository, tenantID uint) {
	now := time.Now()
	last, ok := lastUsedTracker[tenantID]
	if ok && now.Sub(last) < time.Minute {
		return
	}
	lastUsedTracker[tenantID] = now
	go func() {
		_ = tenantRepo.UpdateOrgSettingsCtx(context.Background(), tenantID, map[string]interface{}{
			"api_key_last_used_at": now,
		})
	}()
}

// GetTenantFromContext is a helper used by proxy handlers to extract the resolved tenant.
func GetTenantFromContext(c echo.Context) *cache.CachedTenant {
	v := c.Get(TenantContextKey)
	if v == nil {
		return nil
	}
	t, _ := v.(*cache.CachedTenant)
	return t
}
