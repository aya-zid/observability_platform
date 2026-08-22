// internal/handler/admin_dashboard_handler.go
package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
)

type AdminDashboardHandler struct {
	dashboardService *service.AdminDashboardService
}

func NewAdminDashboardHandler(dashboardService *service.AdminDashboardService) *AdminDashboardHandler {
	return &AdminDashboardHandler{dashboardService: dashboardService}
}

func (h *AdminDashboardHandler) GetStats(c echo.Context) error {
	stats, err := h.dashboardService.GetStats(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, stats)
}

func (h *AdminDashboardHandler) ListTenants(c echo.Context) error {
	limit := 50
	offset := 0
	search := c.QueryParam("search")

	if limitStr := c.QueryParam("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	if offsetStr := c.QueryParam("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	tenants, total, err := h.dashboardService.ListTenants(c.Request().Context(), limit, offset, search)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"tenants": tenants,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (h *AdminDashboardHandler) GetTenantDetail(c echo.Context) error {
	tenantID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid tenant id"})
	}

	detail, err := h.dashboardService.GetTenantDetail(c.Request().Context(), uint(tenantID))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, detail)
}
