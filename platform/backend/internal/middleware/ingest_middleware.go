// internal/middleware/ingest_middleware.go
package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/cache"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

// contextKey is unexported to avoid collisions with other packages
type contextKey string

const TenantContextKey contextKey = "ingest_tenant"

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
			apiKey, _, ok := c.Request().BasicAuth()
			if !ok || apiKey == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "missing or invalid Authorization header — use Basic auth with your API key",
				})
			}

			ctx := c.Request().Context()

			// ── 1. Redis cache lookup ──────────────────────────────────
			cached, err := tenantCache.Get(ctx, apiKey)
			if err != nil {
				// Redis is down — log and fall through to DB (degraded mode)
				c.Logger().Errorf("[ingest] redis get error: %v", err)
			}

			if cached != nil {
				c.Set(string(TenantContextKey), cached)
				return next(c)
			}

			// ── 2. DB lookup on cache miss ─────────────────────────────
			tenant, err := tenantRepo.FindByAPIKey(ctx, apiKey)
			if err != nil || tenant == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "invalid API key",
				})
			}

			resolved := &cache.CachedTenant{
				ID:   tenant.ID,
				UUID: tenant.UUID,
			}

			// ── 3. Populate cache (best-effort — don't fail on Redis error) ──
			if cacheErr := tenantCache.Set(ctx, apiKey, resolved); cacheErr != nil {
				c.Logger().Warnf("[ingest] redis set error (non-fatal): %v", cacheErr)
			}

			c.Set(string(TenantContextKey), resolved)
			return next(c)
		}
	}
}

// GetTenantFromContext is a helper used by proxy handlers to extract the resolved tenant.
func GetTenantFromContext(c echo.Context) *cache.CachedTenant {
	v := c.Get(string(TenantContextKey))
	if v == nil {
		return nil
	}
	t, _ := v.(*cache.CachedTenant)
	return t
}