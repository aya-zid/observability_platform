// internal/middleware/auth_middleware.go
package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/apierror"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

func RequireRole(allowedRoles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claimsAny := c.Get("claims")
			claims, ok := claimsAny.(*infrastructure.Claims)
			if !ok || claims == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
			}
			for _, role := range allowedRoles {
				if claims.Role == role {
					return next(c)
				}
			}
			return c.JSON(apierror.ErrForbidden.Status, map[string]interface{}{
				"error": apierror.ErrForbidden.Message,
				"code":  apierror.ErrForbidden.Code,
			})
		}
	}
}

func RequireAdminForMutation() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claimsAny := c.Get("claims")
			claims, ok := claimsAny.(*infrastructure.Claims)
			if !ok || claims == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
			}
			if claims.Role == "member" {
				switch c.Request().Method {
				case "POST", "PUT", "PATCH", "DELETE":
					return c.JSON(apierror.ErrForbidden.Status, map[string]interface{}{
						"error": apierror.ErrForbidden.Message,
						"code":  apierror.ErrForbidden.Code,
					})
				}
			}
			return next(c)
		}
	}
}

func AuthMiddleware(tenantRepo *repository.TenantRepository, userRepo *repository.UserRepository, subscriptionRepo *repository.SubscriptionRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Prefer HttpOnly cookie (browser auth). Fallback to Bearer header for non-browser clients.
			if cookie, err := c.Cookie("access_token"); err == nil && cookie != nil && cookie.Value != "" {
				claims, err := infrastructure.ParseToken(cookie.Value)
				if err != nil {
					return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid or expired token"})
				}
				if ok, status := authorize(c, claims, tenantRepo, userRepo, subscriptionRepo); !ok {
					return status
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

			claims, err := infrastructure.ParseToken(tokenStr)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid or expired token"})
			}

			if ok, status := authorize(c, claims, tenantRepo, userRepo, subscriptionRepo); !ok {
				return status
			}

			return next(c)
		}
	}
}

func isBillingPath(c echo.Context) bool {
	return strings.Contains(c.Request().URL.Path, "/billing")
}

// authorize validates the tenant is active and the user account is active.
// Deactivated users are rejected on every request, so they cannot keep using
// an already-issued access token after their admin deactivates the account.
func authorize(c echo.Context, claims *infrastructure.Claims, tenantRepo *repository.TenantRepository, userRepo *repository.UserRepository, subscriptionRepo *repository.SubscriptionRepository) (bool, error) {
	tenant, err := tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil || tenant == nil {
		return false, c.JSON(http.StatusUnauthorized, map[string]string{"error": "tenant inactive"})
	}
	if tenant.BannedAt != nil {
		return false, c.JSON(apierror.ErrTenantBanned.Status, map[string]interface{}{
			"error": apierror.ErrTenantBanned.Message,
			"code":  apierror.ErrTenantBanned.Code,
		})
	}
	if !tenant.IsActive {
		return false, c.JSON(http.StatusUnauthorized, map[string]string{"error": "tenant inactive"})
	}

	if tenant.SubscriptionStatus == domain.SubscriptionStatusSuspended && !isBillingPath(c) {
		return false, c.JSON(apierror.ErrAccountSuspended.Status, map[string]interface{}{
			"error": apierror.ErrAccountSuspended.Message,
			"code":  apierror.ErrAccountSuspended.Code,
		})
	}

	user, err := userRepo.FindByIDCtx(c.Request().Context(), claims.UserID)
	if err != nil || user == nil {
		return false, c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}
	if user.Status == string(domain.UserStatusInactive) {
		return false, c.JSON(apierror.ErrUserDeactivated.Status, map[string]interface{}{
			"error": apierror.ErrUserDeactivated.Message,
			"code":  apierror.ErrUserDeactivated.Code,
		})
	}

	if subscriptionRepo != nil {
		sub, _ := subscriptionRepo.FindByTenantCtx(c.Request().Context(), tenant.ID)
		if sub != nil && sub.Status == "expired" && !isBillingPath(c) {
			return false, c.JSON(apierror.ErrSubscriptionExpired.Status, map[string]interface{}{
				"error": apierror.ErrSubscriptionExpired.Message,
				"code":  apierror.ErrSubscriptionExpired.Code,
			})
		}
	}

	c.Set("claims", claims)
	return true, nil
}
