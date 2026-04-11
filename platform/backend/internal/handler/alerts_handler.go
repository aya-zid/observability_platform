package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type AlertsHandler struct {
	alertsRepo *repository.AlertsRepository
}

func NewAlertsHandler(alertsRepo *repository.AlertsRepository) *AlertsHandler {
	return &AlertsHandler{alertsRepo: alertsRepo}
}

func (h *AlertsHandler) ListVulnerabilityAlerts(c echo.Context) error {
	claimsAny := c.Get("claims")
	claims, ok := claimsAny.(*infrastructure.Claims)
	if !ok || claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	items, err := h.alertsRepo.ListVulnerabilityAlertsByTenantCtx(c.Request().Context(), claims.TenantID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load alerts"})
	}
	return c.JSON(http.StatusOK, items)
}
