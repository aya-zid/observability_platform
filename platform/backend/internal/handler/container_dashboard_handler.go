package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
)

type ContainerDashboardHandler struct {
	svc           *service.ContainerDashboardService
	discoveryRepo *repository.DiscoveryRepository
	tenantRepo    *repository.TenantRepository
}

func NewContainerDashboardHandler(
	svc *service.ContainerDashboardService,
	discoveryRepo *repository.DiscoveryRepository,
	tenantRepo *repository.TenantRepository,
) *ContainerDashboardHandler {
	return &ContainerDashboardHandler{
		svc:           svc,
		discoveryRepo: discoveryRepo,
		tenantRepo:    tenantRepo,
	}
}

func (h *ContainerDashboardHandler) Dashboard(c echo.Context) error {
	claims, ok := getClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	serviceID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid container id"})
	}

	svc, err := h.discoveryRepo.FindContainerServiceByIDTenantCtx(c.Request().Context(), claims.TenantID, uint(serviceID))
	if err != nil || svc == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "container not found"})
	}

	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil || tenant == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "tenant not found"})
	}

	startNs, endNs, step, err := parseTimeParams(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	summary, err := h.svc.GetDashboardSummary(
		c.Request().Context(),
		tenant.UUID,
		claims.TenantID,
		svc.ContainerName,
		startNs,
		endNs,
		step,
		svc.MachineID,
	)
	if err != nil {
		c.Logger().Errorf("[container-dashboard] summary failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch dashboard data"})
	}

	return c.JSON(http.StatusOK, summary)
}

func (h *ContainerDashboardHandler) Logs(c echo.Context) error {
	claims, ok := getClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	serviceID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid container id"})
	}

	svc, err := h.discoveryRepo.FindContainerServiceByIDTenantCtx(c.Request().Context(), claims.TenantID, uint(serviceID))
	if err != nil || svc == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "container not found"})
	}

	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil || tenant == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "tenant not found"})
	}

	startNs, err := strconv.ParseInt(c.QueryParam("start"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid start param"})
	}
	endNs, err := strconv.ParseInt(c.QueryParam("end"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid end param"})
	}

	search := c.QueryParam("search")

	page := 1
	if p := c.QueryParam("page"); p != "" {
		if v, err2 := strconv.Atoi(p); err2 == nil && v > 0 {
			page = v
		}
	}
	pageSize := 100
	if ps := c.QueryParam("page_size"); ps != "" {
		if v, err2 := strconv.Atoi(ps); err2 == nil && v > 0 && v <= 500 {
			pageSize = v
		}
	}

	result, err := h.svc.GetLogs(
		c.Request().Context(),
		tenant.UUID,
		svc.ContainerName,
		startNs,
		endNs,
		search,
		page,
		pageSize,
		svc.MachineID,
	)
	if err != nil {
		c.Logger().Errorf("[container-logs] fetch failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch logs"})
	}

	return c.JSON(http.StatusOK, result)
}
