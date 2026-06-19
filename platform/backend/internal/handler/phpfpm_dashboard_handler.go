package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
)

type PHPFPMDashboardHandler struct {
	svc           *service.PHPFPMDashboardService
	discoveryRepo *repository.DiscoveryRepository
	tenantRepo    *repository.TenantRepository
}

func NewPHPFPMDashboardHandler(
	svc *service.PHPFPMDashboardService,
	discoveryRepo *repository.DiscoveryRepository,
	tenantRepo *repository.TenantRepository,
) *PHPFPMDashboardHandler {
	return &PHPFPMDashboardHandler{
		svc:           svc,
		discoveryRepo: discoveryRepo,
		tenantRepo:    tenantRepo,
	}
}

func (h *PHPFPMDashboardHandler) Dashboard(c echo.Context) error {
	claims, ok := getClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	serviceID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid service id"})
	}

	svc, err := h.discoveryRepo.FindServiceByIDTenantCtx(c.Request().Context(), claims.TenantID, uint(serviceID))
	if err != nil || svc == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "service not found"})
	}
	if !isPHPFPMServiceName(svc.ServiceName) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "service is not PHP-FPM"})
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
		svc.ServiceName,
		startNs,
		endNs,
		step,
	)
	if err != nil {
		c.Logger().Errorf("[phpfpm-dashboard] summary failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch dashboard data"})
	}

	return c.JSON(http.StatusOK, summary)
}

func (h *PHPFPMDashboardHandler) Logs(c echo.Context) error {
	claims, ok := getClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	serviceID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid service id"})
	}

	svc, err := h.discoveryRepo.FindServiceByIDTenantCtx(c.Request().Context(), claims.TenantID, uint(serviceID))
	if err != nil || svc == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "service not found"})
	}
	if !isPHPFPMServiceName(svc.ServiceName) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "service is not PHP-FPM"})
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

	logFile := c.QueryParam("log_file")
	pool := c.QueryParam("pool")
	search := c.QueryParam("search")
	method := c.QueryParam("method")
	statusCode := c.QueryParam("status_code")

	parseFloat := func(s string) (float64, bool) {
		if s == "" {
			return 0, false
		}
		v, err := strconv.ParseFloat(s, 64)
		return v, err == nil
	}
	minDur, hasMinDur := parseFloat(c.QueryParam("min_duration"))
	maxDur, hasMaxDur := parseFloat(c.QueryParam("max_duration"))
	minMem, hasMinMem := parseFloat(c.QueryParam("min_memory"))
	maxMem, hasMaxMem := parseFloat(c.QueryParam("max_memory"))
	minCPU, hasMinCPU := parseFloat(c.QueryParam("min_cpu"))
	maxCPU, hasMaxCPU := parseFloat(c.QueryParam("max_cpu"))

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
		svc.ServiceName,
		startNs,
		endNs,
		logFile,
		pool,
		search,
		method,
		statusCode,
		minDur, hasMinDur,
		maxDur, hasMaxDur,
		minMem, hasMinMem,
		maxMem, hasMaxMem,
		minCPU, hasMinCPU,
		maxCPU, hasMaxCPU,
		page,
		pageSize,
	)
	if err != nil {
		c.Logger().Errorf("[phpfpm-logs] fetch failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch logs"})
	}

	return c.JSON(http.StatusOK, result)
}

func (h *PHPFPMDashboardHandler) LogsMeta(c echo.Context) error {
	claims, ok := getClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	serviceID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid service id"})
	}

	svc, err := h.discoveryRepo.FindServiceByIDTenantCtx(c.Request().Context(), claims.TenantID, uint(serviceID))
	if err != nil || svc == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "service not found"})
	}
	if !isPHPFPMServiceName(svc.ServiceName) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "service is not PHP-FPM"})
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

	meta, err := h.svc.GetLogsMeta(c.Request().Context(), tenant.UUID, svc.ServiceName, startNs, endNs)
	if err != nil {
		c.Logger().Errorf("[phpfpm-logs-meta] failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch log counts"})
	}

	return c.JSON(http.StatusOK, meta)
}

func isPHPFPMServiceName(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	return normalized == "phpfpm" || normalized == "php-fpm" || strings.Contains(normalized, "phpfpm") || strings.Contains(normalized, "php-fpm")
}
