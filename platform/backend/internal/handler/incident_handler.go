package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type IncidentHandler struct {
	incidentRepo *repository.IncidentRepository
	tenantRepo   *repository.TenantRepository
}

func NewIncidentHandler(incidentRepo *repository.IncidentRepository, tenantRepo *repository.TenantRepository) *IncidentHandler {
	return &IncidentHandler{incidentRepo: incidentRepo, tenantRepo: tenantRepo}
}

func (h *IncidentHandler) DismissIncident(c echo.Context) error {
	claims, ok := c.Get("claims").(*infrastructure.Claims)
	if !ok || claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	incidentID := c.Param("id")
	if incidentID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "incident id required"})
	}

	if err := h.incidentRepo.DismissIncident(c.Request().Context(), claims.TenantID, incidentID); err != nil {
		c.Logger().Errorf("[incident] dismiss failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to dismiss incident"})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "dismissed"})
}
