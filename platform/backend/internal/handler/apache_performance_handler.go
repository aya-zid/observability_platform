// internal/handler/apache_performance_handler.go
package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
)

// ApachePerformanceHandler exposes three performance chart endpoints:
//
//	GET /api/apache/:id/performance/heatmap    → Chart 1: RT Intensity Heatmap
//	GET /api/apache/:id/performance/scatter    → Chart 2: Raw RT Scatter (stub)
//	GET /api/apache/:id/performance/dist       → Chart 3: RT Distribution by Status
type ApachePerformanceHandler struct {
	svc           *service.ApachePerformanceService
	discoveryRepo *repository.DiscoveryRepository
	tenantRepo    *repository.TenantRepository
}

func NewApachePerformanceHandler(
	svc *service.ApachePerformanceService,
	discoveryRepo *repository.DiscoveryRepository,
	tenantRepo *repository.TenantRepository,
) *ApachePerformanceHandler {
	return &ApachePerformanceHandler{
		svc:           svc,
		discoveryRepo: discoveryRepo,
		tenantRepo:    tenantRepo,
	}
}

// resolvePerformanceParams is shared param parsing for all three endpoints.
// Returns (tenantUUID, serviceName, domain, startNs, endNs, error).
func (h *ApachePerformanceHandler) resolvePerformanceParams(c echo.Context) (
	tenantUUID, serviceName, domain string,
	startNs, endNs int64,
	err error,
) {
	claims, ok := getClaims(c)
	if !ok {
		err = echo.NewHTTPError(http.StatusUnauthorized, "unauthenticated")
		return
	}

	serviceID, parseErr := strconv.ParseUint(c.Param("id"), 10, 64)
	if parseErr != nil {
		err = echo.NewHTTPError(http.StatusBadRequest, "invalid service id")
		return
	}

	svc, svcErr := h.discoveryRepo.FindServiceByIDTenantCtx(c.Request().Context(), claims.TenantID, uint(serviceID))
	if svcErr != nil || svc == nil {
		err = echo.NewHTTPError(http.StatusNotFound, "service not found")
		return
	}
	serviceName = svc.ServiceName

	tenant, tenantErr := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if tenantErr != nil || tenant == nil {
		err = echo.NewHTTPError(http.StatusInternalServerError, "tenant not found")
		return
	}
	tenantUUID = tenant.UUID

	domain = c.QueryParam("domain")

	startNs, err = parseNsParam(c.QueryParam("start"))
	if err != nil {
		err = echo.NewHTTPError(http.StatusBadRequest, "invalid start param")
		return
	}
	endNs, err = parseNsParam(c.QueryParam("end"))
	if err != nil {
		err = echo.NewHTTPError(http.StatusBadRequest, "invalid end param")
		return
	}
	return
}

// Heatmap handles GET /api/apache/:id/performance/heatmap
func (h *ApachePerformanceHandler) Heatmap(c echo.Context) error {
	tenantUUID, serviceName, domain, startNs, endNs, err := h.resolvePerformanceParams(c)
	if err != nil {
		return err
	}

	result, err := h.svc.GetRTIntensityHeatmap(
		c.Request().Context(),
		tenantUUID, serviceName, domain,
		startNs, endNs,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, result)
}

// Scatter handles GET /api/apache/:id/performance/scatter
func (h *ApachePerformanceHandler) Scatter(c echo.Context) error {
	tenantUUID, serviceName, domain, startNs, endNs, err := h.resolvePerformanceParams(c)
	if err != nil {
		return err
	}

	result, err := h.svc.GetRTScatter(
		c.Request().Context(),
		tenantUUID, serviceName, domain,
		startNs, endNs,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, result)
}

// Distribution handles GET /api/apache/:id/performance/dist
func (h *ApachePerformanceHandler) Distribution(c echo.Context) error {
	tenantUUID, serviceName, domain, startNs, endNs, err := h.resolvePerformanceParams(c)
	if err != nil {
		return err
	}

	result, err := h.svc.GetRTDistributionByStatus(
		c.Request().Context(),
		tenantUUID, serviceName, domain,
		startNs, endNs,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, result)
}
