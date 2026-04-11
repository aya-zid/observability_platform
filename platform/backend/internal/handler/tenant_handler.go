package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/cache"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type TenantHandler struct {
	tenantRepo *repository.TenantRepository
	userRepo   *repository.UserRepository
	tenantCache *cache.TenantCache
}

func NewTenantHandler(
	tenantRepo *repository.TenantRepository,
	userRepo *repository.UserRepository,
	tenantCache *cache.TenantCache,
) *TenantHandler {
	return &TenantHandler{
		tenantRepo: tenantRepo,
		userRepo: userRepo,
		tenantCache: tenantCache,
	}
}

// GetAPIKey returns the tenant API key for the authenticated user.
// NOTE: This is intentionally returning a secret (per request). Not recommended for production.
func (h *TenantHandler) GetAPIKey(c echo.Context) error {
	claimsAny := c.Get("claims")
	claims, ok := claimsAny.(*infrastructure.Claims)
	if !ok || claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load tenant"})
	}
	if tenant == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "tenant not found"})
	}

	return c.JSON(http.StatusOK, dto.APIKeyResponse{APIKey: tenant.APIKey})
}

// RotateAPIKey generates and persists a new tenant API key.
// The old key is invalidated from Redis cache immediately.
func (h *TenantHandler) RotateAPIKey(c echo.Context) error {
	claimsAny := c.Get("claims")
	claims, ok := claimsAny.(*infrastructure.Claims)
	if !ok || claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load tenant"})
	}
	if tenant == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "tenant not found"})
	}

	newAPIKey, err := infrastructure.GenerateTenantAPIKey()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to generate api key"})
	}

	if err := h.tenantRepo.UpdateAPIKeyByIDCtx(c.Request().Context(), tenant.ID, newAPIKey); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to rotate api key"})
	}

	if h.tenantCache != nil && tenant.APIKey != "" {
		_ = h.tenantCache.Invalidate(c.Request().Context(), tenant.APIKey)
	}

	return c.JSON(http.StatusOK, dto.APIKeyResponse{APIKey: newAPIKey})
}

// GetMe returns current authenticated user identity and tenant basic info.
func (h *TenantHandler) GetMe(c echo.Context) error {
	claimsAny := c.Get("claims")
	claims, ok := claimsAny.(*infrastructure.Claims)
	if !ok || claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	user, err := h.userRepo.FindByIDCtx(c.Request().Context(), claims.UserID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load user"})
	}
	if user == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}

	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load tenant"})
	}
	if tenant == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "tenant not found"})
	}

	return c.JSON(http.StatusOK, dto.MeResponse{
		UserID:      user.ID,
		TenantID:    user.TenantID,
		FullName:    user.FullName,
		Email:       user.Email,
		Role:        user.Role,
		CompanyName: tenant.CompanyName,
	})
}
