// internal/middleware/admin_auth_middleware.go
package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/apierror"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

const AdminContextKey = "adminClaims"

func AdminAuthMiddleware(adminRepo *repository.AdminRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if cookie, err := c.Cookie("admin_access_token"); err == nil && cookie != nil && cookie.Value != "" {
				claims, err := infrastructure.ParseAdminToken(cookie.Value)
				if err != nil {
					return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid or expired token"})
				}
				if err := authorizeAdmin(c, claims, adminRepo); err != nil {
					return err
				}
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

			claims, err := infrastructure.ParseAdminToken(tokenStr)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid or expired token"})
			}

			if err := authorizeAdmin(c, claims, adminRepo); err != nil {
				return err
			}

			return next(c)
		}
	}
}

func authorizeAdmin(c echo.Context, claims *infrastructure.AdminClaims, adminRepo *repository.AdminRepository) error {
	admin, err := adminRepo.FindByIDCtx(c.Request().Context(), claims.AdminID)
	if err != nil || admin == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}
	if !admin.IsActive {
		return c.JSON(apierror.ErrUserDeactivated.Status, map[string]interface{}{
			"error": apierror.ErrUserDeactivated.Message,
			"code":  apierror.ErrUserDeactivated.Code,
		})
	}

	c.Set(AdminContextKey, claims)
	return nil
}
