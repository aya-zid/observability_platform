// internal/middleware/auth_middleware.go
package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

func AuthMiddleware(tenantRepo *repository.TenantRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Prefer HttpOnly cookie (browser auth). Fallback to Bearer header for non-browser clients.
			if cookie, err := c.Cookie("access_token"); err == nil && cookie != nil && cookie.Value != "" {
				claims, err := infrastructure.ParseToken(cookie.Value)
				if err != nil {
					return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid or expired token"})
				}
				tenant, err := tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
				if err != nil || tenant == nil || !tenant.IsActive {
					return c.JSON(http.StatusUnauthorized, map[string]string{"error": "tenant inactive"})
				}
				c.Set("claims", claims)
				return next(c)
			}

			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Missing Authorization header"})
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenStr == authHeader {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token format"})
			}

			claims, err := infrastructure.ParseToken(tokenStr)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid or expired token"})
			}

			tenant, err := tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
			if err != nil || tenant == nil || !tenant.IsActive {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "tenant inactive"})
			}

			// Store claims in Echo context
			c.Set("claims", claims)

			return next(c)
		}
	}
}
