// internal/handler/nginx_dashboard_handler.go

package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
)

type NginxDashboardHandler struct {
	svc           *service.NginxDashboardService
	discoveryRepo *repository.DiscoveryRepository
	tenantRepo    *repository.TenantRepository
}

func NewNginxDashboardHandler(
	svc *service.NginxDashboardService,
	discoveryRepo *repository.DiscoveryRepository,
	tenantRepo *repository.TenantRepository,
) *NginxDashboardHandler {
	return &NginxDashboardHandler{
		svc:           svc,
		discoveryRepo: discoveryRepo,
		tenantRepo:    tenantRepo,
	}
}

// Dashboard returns the full dashboard summary in one call.
//
// GET /api/nginx/:id/dashboard
//
//	?start=<unix_ns>&end=<unix_ns>&step=<duration>&min_rt=<seconds>
//
// Response: NginxDashboardSummary JSON (see nginx_dashboard_service.go)
//
// The backend caches the result for 20s in Redis.
// The frontend should poll this endpoint every 20–30s.
//
// min_rt (optional): slow-request threshold in seconds. Default is 1.0.
func (h *NginxDashboardHandler) Dashboard(c echo.Context) error {
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

	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil || tenant == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "tenant not found"})
	}

	startNs, endNs, step, err := parseTimeParams(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// Parse configurable slow-request threshold (default 1.0s)
	minRT := 0.0
	if minRTStr := c.QueryParam("min_rt"); minRTStr != "" {
		minRT, _ = strconv.ParseFloat(minRTStr, 64)
	}
	rtPercentiles := parsePercentileParam(c.QueryParam("percentiles"))

	summary, err := h.svc.GetDashboardSummary(
		c.Request().Context(),
		tenant.UUID,
		claims.TenantID,
		svc.ServiceName,
		c.QueryParam("domain"),
		startNs,
		endNs,
		step,
		minRT,
		rtPercentiles,
	)
	if err != nil {
		c.Logger().Errorf("[nginx-dashboard] summary failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch dashboard data"})
	}

	return c.JSON(http.StatusOK, summary)
}

func parsePercentileParam(raw string) []float64 {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]float64, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(part, "p")), 64)
		if err != nil {
			continue
		}
		out = append(out, value)
	}
	return out
}

// SlowRequests returns a paginated list of slow requests for the "View More" page.
//
// GET /api/nginx/:id/slow-requests
//
//	?start=<unix_ns>&end=<unix_ns>&min_rt=<seconds>&page=<int>&per_page=<int>
//
// Response: SlowRequestsListResult JSON (see nginx_dashboard_service.go)
//
// NOT cached — user-driven pagination, always fetches fresh.
func (h *NginxDashboardHandler) SlowRequests(c echo.Context) error {
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

	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil || tenant == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "tenant not found"})
	}

	startNs, endNs, _, err := parseTimeParams(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// Parse configurable slow-request threshold (default 1.0s)
	minRT := 0.0
	if minRTStr := c.QueryParam("min_rt"); minRTStr != "" {
		minRT, _ = strconv.ParseFloat(minRTStr, 64)
	}

	// Pagination params
	page := 1
	if pageStr := c.QueryParam("page"); pageStr != "" {
		if parsed, parseErr := strconv.Atoi(pageStr); parseErr == nil && parsed > 0 {
			page = parsed
		}
	}
	perPage := 100
	if perPageStr := c.QueryParam("per_page"); perPageStr != "" {
		if parsed, parseErr := strconv.Atoi(perPageStr); parseErr == nil && parsed > 0 {
			perPage = parsed
		}
	}

	result, err := h.svc.GetSlowRequestsList(
		c.Request().Context(),
		tenant.UUID,
		svc.ServiceName,
		c.QueryParam("domain"),
		startNs,
		endNs,
		minRT,
		page,
		perPage,
	)
	if err != nil {
		c.Logger().Errorf("[nginx-slow-requests] query failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch slow requests"})
	}

	return c.JSON(http.StatusOK, result)
}

// Explorer runs on-demand filtered queries for the drilldown/research tab.
//
// GET /api/nginx/:id/explorer
//
//	?start=<unix_ns>&end=<unix_ns>&step=<duration>
//	&ip=<exact_ip>            (optional)
//	&path=<path_prefix>       (optional, e.g. /api)
//	&status=<2xx|3xx|4xx|5xx> (optional)
//	&min_rt=<seconds>         (optional, e.g. 1.0)
//
// NOT cached — each filter combination is unique and user-driven.
func (h *NginxDashboardHandler) Explorer(c echo.Context) error {
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

	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil || tenant == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "tenant not found"})
	}

	startNs, endNs, step, err := parseTimeParams(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	minRT := 0.0
	if minRTStr := c.QueryParam("min_rt"); minRTStr != "" {
		minRT, _ = strconv.ParseFloat(minRTStr, 64)
	}

	filter := service.NginxExplorerFilter{
		Domain:      c.QueryParam("domain"),
		IP:          c.QueryParam("ip"),
		PathPrefix:  c.QueryParam("path"),
		StatusClass: c.QueryParam("status"),
		MinRT:       minRT,
	}

	result, err := h.svc.GetExplorerResult(
		c.Request().Context(),
		tenant.UUID,
		svc.ServiceName,
		startNs,
		endNs,
		step,
		filter,
	)
	if err != nil {
		c.Logger().Errorf("[nginx-explorer] query failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to run explorer query"})
	}

	return c.JSON(http.StatusOK, result)
}

func (h *NginxDashboardHandler) Domains(c echo.Context) error {
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

	rows, err := h.discoveryRepo.ListVirtualHostsByServiceCtx(c.Request().Context(), claims.TenantID, svc.ID)
	if err != nil {
		c.Logger().Errorf("[nginx-domains] list failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list nginx domains"})
	}

	items := make([]service.NginxDomainItem, 0)
	seen := make(map[string]int)
	for _, row := range rows {
		if !row.Enabled {
			continue
		}
		domainName := strings.TrimSpace(row.Domain)
		if domainName == "" {
			domainName = "(default)"
		}
		if idx, ok := seen[domainName]; ok {
			items[idx].IsDefault = items[idx].IsDefault || row.IsDefault
			items[idx].IsSharedLog = items[idx].IsSharedLog || row.IsSharedLog
			continue
		}
		seen[domainName] = len(items)
		items = append(items, service.NginxDomainItem{
			Domain:      domainName,
			Enabled:     row.Enabled,
			IsDefault:   row.IsDefault,
			IsSharedLog: row.IsSharedLog,
		})
	}

	return c.JSON(http.StatusOK, items)
}

// ErrorLogs returns raw nginx error log lines for a service/domain window.
//
// GET /api/nginx/:id/error-logs
//
//	?start=<unix_ns>&end=<unix_ns>&step=<duration>
//	&domain=<vhost>   (optional — omit to query all domains)
//	&limit=<int>      (optional — default 200, max 500)
//
// Response: { "lines": ["...", ...] }
//
// Lines are raw unstructured text (nginx error log format) and are NOT parsed.
// The frontend displays them verbatim.
func (h *NginxDashboardHandler) ErrorLogs(c echo.Context) error {
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
		c.QueryParam("domain"),
		startNs,
		endNs,
		limit,
	)
	if err != nil {
		c.Logger().Errorf("[nginx-error-logs] query failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch error logs"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"lines": lines})
}

// ─── Helpers ───────────────────────────────────────────────────────────────────

func parseTimeParams(c echo.Context) (startNs, endNs int64, step string, err error) {
	startStr := c.QueryParam("start")
	endStr := c.QueryParam("end")
	step = c.QueryParam("step")

	if step == "" {
		step = "30s"
	}

	if startStr == "" || endStr == "" {
		// Default to last 1 hour
		now := time.Now().UnixNano()
		startNs = now - int64(3600)*1e9
		endNs = now
		return startNs, endNs, step, nil
	}

	startNs, err = strconv.ParseInt(startStr, 10, 64)
	if err != nil {
		return 0, 0, "", err
	}
	endNs, err = strconv.ParseInt(endStr, 10, 64)
	if err != nil {
		return 0, 0, "", err
	}
	if startNs <= 0 || endNs <= 0 {
		return 0, 0, "", echo.NewHTTPError(http.StatusBadRequest, "start and end must be positive unix nanoseconds")
	}
	if endNs <= startNs {
		return 0, 0, "", echo.NewHTTPError(http.StatusBadRequest, "end must be after start")
	}
	return startNs, endNs, step, nil
}

// In internal/handler/nginx_dashboard_handler.go — add this method

func (h *NginxDashboardHandler) IPInvestigation(c echo.Context) error {
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

	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil || tenant == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "tenant not found"})
	}

	ip := c.QueryParam("ip")
	if ip == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ip parameter is required"})
	}

	startNs, endNs, step, err := parseTimeParams(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	seenStartNs, seenEndNs := int64(0), int64(0)
	if c.QueryParam("seen_start") != "" || c.QueryParam("seen_end") != "" {
		seenStartNs, err = parseNsParam(c.QueryParam("seen_start"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid seen_start param"})
		}
		seenEndNs, err = parseNsParam(c.QueryParam("seen_end"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid seen_end param"})
		}
	}

	domain := c.QueryParam("domain")

	result, err := h.svc.GetIPInvestigation(
		c.Request().Context(),
		tenant.UUID,
		svc.ServiceName,
		ip,
		domain,
		startNs,
		endNs,
		step,
		seenStartNs,
		seenEndNs,
	)
	if err != nil {
		c.Logger().Errorf("[nginx-ip-investigation] failed for ip=%s: %v", ip, err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch IP investigation data"})
	}

	return c.JSON(http.StatusOK, result)
}
