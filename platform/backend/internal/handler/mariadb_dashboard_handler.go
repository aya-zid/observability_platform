package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
)

type MariaDBDashboardHandler struct {
	svc           *service.MariaDBDashboardService
	discoveryRepo *repository.DiscoveryRepository
	tenantRepo    *repository.TenantRepository
}

func NewMariaDBDashboardHandler(
	svc *service.MariaDBDashboardService,
	discoveryRepo *repository.DiscoveryRepository,
	tenantRepo *repository.TenantRepository,
) *MariaDBDashboardHandler {
	return &MariaDBDashboardHandler{
		svc:           svc,
		discoveryRepo: discoveryRepo,
		tenantRepo:    tenantRepo,
	}
}

func (h *MariaDBDashboardHandler) Dashboard(c echo.Context) error {
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
	if !isMariaDBServiceName(svc.ServiceName) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "service is not MariaDB/MySQL"})
	}

	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil || tenant == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "tenant not found"})
	}

	startNs, endNs, step, err := parseTimeParams(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	minDurationSec := 0.0
	if m := c.QueryParam("min_duration_seconds"); m != "" {
		if v, err2 := strconv.ParseFloat(m, 64); err2 == nil && v >= 0 {
			minDurationSec = v
		}
	}

	summary, err := h.svc.GetDashboardSummary(
		c.Request().Context(),
		tenant.UUID,
		claims.TenantID,
		svc.ServiceName,
		startNs,
		endNs,
		step,
		minDurationSec,
	)
	if err != nil {
		c.Logger().Errorf("[mariadb-dashboard] summary failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch dashboard data"})
	}

	return c.JSON(http.StatusOK, summary)
}

func (h *MariaDBDashboardHandler) Config(c echo.Context) error {
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
	if !isMariaDBServiceName(svc.ServiceName) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "service is not MariaDB/MySQL"})
	}

	snapshot, err := h.svc.GetConfigSnapshot(c.Request().Context(), claims.TenantID)
	if err != nil {
		c.Logger().Errorf("[mariadb-config] fetch failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch config"})
	}

	return c.JSON(http.StatusOK, snapshot)
}

func (h *MariaDBDashboardHandler) Logs(c echo.Context) error {
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
	if !isMariaDBServiceName(svc.ServiceName) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "service is not MariaDB/MySQL"})
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

	category := c.QueryParam("category")
	search := c.QueryParam("search")
	minDurationMs := 0.0
	if m := c.QueryParam("min_duration_ms"); m != "" {
		if v, err2 := strconv.ParseFloat(m, 64); err2 == nil && v >= 0 {
			minDurationMs = v
		}
	}

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
		category,
		search,
		page,
		pageSize,
		minDurationMs,
	)
	if err != nil {
		c.Logger().Errorf("[mariadb-logs] fetch failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch logs"})
	}

	return c.JSON(http.StatusOK, result)
}

func (h *MariaDBDashboardHandler) SlowQueryLogs(c echo.Context) error {
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
	if !isMariaDBServiceName(svc.ServiceName) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "service is not MariaDB/MySQL"})
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

	page := 1
	if p := c.QueryParam("page"); p != "" {
		if v, err2 := strconv.Atoi(p); err2 == nil && v > 0 {
			page = v
		}
	}
	pageSize := 50
	if ps := c.QueryParam("page_size"); ps != "" {
		if v, err2 := strconv.Atoi(ps); err2 == nil && v > 0 && v <= 200 {
			pageSize = v
		}
	}

	minDurationMs := 0.0
	if m := c.QueryParam("min_duration_ms"); m != "" {
		if v, err2 := strconv.ParseFloat(m, 64); err2 == nil && v >= 0 {
			minDurationMs = v
		}
	}

	result, err := h.svc.GetSlowQueryLogs(
		c.Request().Context(),
		tenant.UUID,
		svc.ServiceName,
		startNs,
		endNs,
		page,
		pageSize,
		minDurationMs,
	)
	if err != nil {
		c.Logger().Errorf("[mariadb-slow-query-logs] fetch failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch slow query logs"})
	}

	return c.JSON(http.StatusOK, result)
}

func (h *MariaDBDashboardHandler) ErrorLogs(c echo.Context) error {
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
	if !isMariaDBServiceName(svc.ServiceName) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "service is not MariaDB/MySQL"})
	}

	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil || tenant == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "tenant not found"})
	}

	startNs, endNs, _, err := parseTimeParams(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	limit := 200
	if limitStr := c.QueryParam("limit"); limitStr != "" {
		if parsed, parseErr := strconv.Atoi(limitStr); parseErr == nil && parsed > 0 {
			limit = parsed
		}
	}

	lines, err := h.svc.GetErrorLogs(
		c.Request().Context(),
		tenant.UUID,
		svc.ServiceName,
		startNs,
		endNs,
		limit,
	)
	if err != nil {
		c.Logger().Errorf("[mariadb-error-logs] query failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch error logs"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"lines": lines})
}

func isMariaDBServiceName(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	return normalized == "mysql" || normalized == "mariadb" || normalized == "mysqld" || normalized == "mariadbd" || strings.Contains(normalized, "mysql") || strings.Contains(normalized, "mariadb")
}
