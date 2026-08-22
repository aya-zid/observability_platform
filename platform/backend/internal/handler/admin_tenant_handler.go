// internal/handler/admin_tenant_handler.go
package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
)

type AdminTenantHandler struct {
	tenantService *service.AdminTenantService
}

func NewAdminTenantHandler(tenantService *service.AdminTenantService) *AdminTenantHandler {
	return &AdminTenantHandler{tenantService: tenantService}
}

func (h *AdminTenantHandler) BanTenant(c echo.Context) error {
	adminClaims := c.Get("adminClaims").(*infrastructure.AdminClaims)
	tenantID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid tenant id"})
	}

	var req dto.AdminBanTenantRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if err := h.tenantService.BanTenant(c.Request().Context(), uint(tenantID), adminClaims.AdminID, req.Reason); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func (h *AdminTenantHandler) UnbanTenant(c echo.Context) error {
	tenantID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid tenant id"})
	}

	if err := h.tenantService.UnbanTenant(c.Request().Context(), uint(tenantID)); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func (h *AdminTenantHandler) ChangePlan(c echo.Context) error {
	tenantID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid tenant id"})
	}

	var req dto.AdminAssignPackRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if err := h.tenantService.AssignPack(c.Request().Context(), uint(tenantID), req.PackID); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func (h *AdminTenantHandler) RenewSubscription(c echo.Context) error {
	tenantID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid tenant id"})
	}

	var req dto.AdminRenewSubscriptionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if err := h.tenantService.RenewSubscription(c.Request().Context(), uint(tenantID), req.DurationMonths); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}
