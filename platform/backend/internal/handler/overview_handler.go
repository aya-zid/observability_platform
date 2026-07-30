package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
)

type OverviewHandler struct {
	svc        *service.OverviewService
	tenantRepo *repository.TenantRepository
}

func NewOverviewHandler(svc *service.OverviewService, tenantRepo *repository.TenantRepository) *OverviewHandler {
	return &OverviewHandler{svc: svc, tenantRepo: tenantRepo}
}

func (h *OverviewHandler) GetOverview(c echo.Context) error {
	claims, ok := c.Get("claims").(*infrastructure.Claims)
	if !ok || claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil || tenant == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "tenant not found"})
	}

	// Optional timeline window params (millisecond epoch — converted to ns).
	var timelineStartMs, timelineEndMs int64
	if startStr := c.QueryParam("timeline_start"); startStr != "" {
		var err error
		timelineStartMs, err = strconv.ParseInt(startStr, 10, 64)
		if err != nil {
			c.Logger().Errorf("invalid timeline_start: %v", err)
		}
	}
	if endStr := c.QueryParam("timeline_end"); endStr != "" {
		var err error
		timelineEndMs, err = strconv.ParseInt(endStr, 10, 64)
		if err != nil {
			c.Logger().Errorf("invalid timeline_end: %v", err)
		}
	}
	// Convert millisecond-precision values to nanoseconds for internal use.
	timelineStartNs := timelineStartMs * 1_000_000
	timelineEndNs := timelineEndMs * 1_000_000

	overview, err := h.svc.GetOverview(c.Request().Context(), claims.TenantID, tenant.UUID, timelineStartNs, timelineEndNs)
	if err != nil {
		c.Logger().Errorf("[overview] failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch overview data"})
	}

	return c.JSON(http.StatusOK, overview)
}
