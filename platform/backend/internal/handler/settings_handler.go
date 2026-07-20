package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/validator"
)

type SettingsHandler struct {
	tenantRepo *repository.TenantRepository
}

func NewSettingsHandler(tenantRepo *repository.TenantRepository) *SettingsHandler {
	return &SettingsHandler{tenantRepo: tenantRepo}
}

func (h *SettingsHandler) GetOrgSettings(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)
	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil || tenant == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "tenant not found"})
	}

	return c.JSON(http.StatusOK, dto.OrgSettingsResponse{
		CompanyName:  tenant.CompanyName,
		ContactEmail: tenant.ContactEmail,
		Timezone:     tenant.Timezone,
		CreatedAt:    tenant.CreatedAt.Format(time.RFC3339),
	})
}

func (h *SettingsHandler) UpdateOrgSettings(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)

	var req dto.UpdateOrgRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	updates := map[string]interface{}{}
	if req.CompanyName != "" {
		updates["company_name"] = req.CompanyName
	}
	if req.Timezone != "" {
		updates["timezone"] = req.Timezone
	}

	if err := h.tenantRepo.UpdateOrgSettingsCtx(c.Request().Context(), claims.TenantID, updates); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update settings"})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}


