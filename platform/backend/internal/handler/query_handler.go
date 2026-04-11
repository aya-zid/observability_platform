// internal/handler/query_handler.go
package handler

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/labstack/echo/v4"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type QueryHandler struct {
	cfg        *config.Config
	tenantRepo *repository.TenantRepository
	httpClient *http.Client
}

func NewQueryHandler(cfg *config.Config, tenantRepo *repository.TenantRepository) *QueryHandler {
	return &QueryHandler{
		cfg:        cfg,
		tenantRepo: tenantRepo,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// QueryMetrics proxies to VictoriaMetrics vmselect with tenant isolation.
// GET /api/query/metrics?query=<promql>&start=<unix>&end=<unix>&step=<s>&type=range|instant
func (h *QueryHandler) QueryMetrics(c echo.Context) error {
	claims, ok := getClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	q := c.QueryParam("query")
	start := c.QueryParam("start")
	end := c.QueryParam("end")
	step := c.QueryParam("step")
	queryType := c.QueryParam("type")
	if queryType == "" {
		queryType = "range"
	}
	if q == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "query is required"})
	}

	endpoint := "query_range"
	if queryType == "instant" {
		endpoint = "query"
	}

	// Tenant isolation: ID in URL path — never visible to the browser
	upstream := fmt.Sprintf("http://%s:%s/select/%d/prometheus/api/v1/%s",
		h.cfg.VMSelectHost, h.cfg.VMSelectPort, claims.TenantID, endpoint)

	params := url.Values{}
	params.Set("query", q)
	if start != "" {
		params.Set("start", start)
	}
	if end != "" {
		params.Set("end", end)
	}
	if step != "" {
		params.Set("step", step)
	}

	return h.proxyGet(c, upstream+"?"+params.Encode(), nil)
}

// QueryLogsRange proxies to Loki query_range.
//
// This endpoint is used for BOTH:
//   - Log-stream queries (returning log lines) — direction matters, default "backward"
//   - Metric queries (rate, quantile_over_time, etc.) — direction is ignored by Loki
//
// The frontend sets direction only when fetching log lines; metric queries omit it,
// so the backend must NOT force direction when the caller does not ask for it.
//
// GET /api/query/logs/range?query=<logql>&start=<ns>&end=<ns>&step=<dur>&limit=<n>&direction=<forward|backward>
func (h *QueryHandler) QueryLogsRange(c echo.Context) error {
	claims, ok := getClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil || tenant == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "tenant not found"})
	}

	q := c.QueryParam("query")
	start := c.QueryParam("start")
	end := c.QueryParam("end")
	step := c.QueryParam("step")
	limit := c.QueryParam("limit")
	direction := c.QueryParam("direction") // may be empty — do NOT default for metric queries

	if q == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "query is required"})
	}
	if limit == "" {
		limit = "1000"
	}

	upstream := fmt.Sprintf("http://%s:%s/loki/api/v1/query_range",
		h.cfg.LokiHost, h.cfg.LokiPort)

	params := url.Values{}
	params.Set("query", q)
	params.Set("limit", limit)
	if direction != "" {
		params.Set("direction", direction)
	} // only set when caller requests it
	if start != "" {
		params.Set("start", start)
	}
	if end != "" {
		params.Set("end", end)
	}
	if step != "" {
		params.Set("step", step)
	}

	headers := map[string]string{"X-Scope-OrgID": tenant.UUID}
	return h.proxyGet(c, upstream+"?"+params.Encode(), headers)
}

// QueryLogs proxies to Loki query_range for log lines (not metric queries).
// Always uses direction=backward (newest first) and a time range.
//
// GET /api/query/logs?query=<logql>&limit=<n>&start=<ns>&end=<ns>&direction=<backward|forward>
func (h *QueryHandler) QueryLogs(c echo.Context) error {
	claims, ok := getClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil || tenant == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "tenant not found"})
	}

	q := c.QueryParam("query")
	limit := c.QueryParam("limit")
	start := c.QueryParam("start")
	end := c.QueryParam("end")
	direction := c.QueryParam("direction")

	if q == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "query is required"})
	}
	if limit == "" {
		limit = "100"
	}
	if direction == "" {
		direction = "backward"
	}

	// Default to last hour if no range provided
	if start == "" || end == "" {
		now := time.Now().UnixNano()
		oneHourAgo := now - int64(60*60*1e9)
		start = fmt.Sprintf("%d", oneHourAgo)
		end = fmt.Sprintf("%d", now)
	}

	upstream := fmt.Sprintf("http://%s:%s/loki/api/v1/query_range",
		h.cfg.LokiHost, h.cfg.LokiPort)

	params := url.Values{}
	params.Set("query", q)
	params.Set("limit", limit)
	params.Set("direction", direction)
	params.Set("start", start)
	params.Set("end", end)

	headers := map[string]string{"X-Scope-OrgID": tenant.UUID}
	return h.proxyGet(c, upstream+"?"+params.Encode(), headers)
}

// QueryLogsLabels proxies to Loki /label and /label/{name}/values.
// GET /api/query/logs/labels?name=<label>&query=<filter>&start=<ns>&end=<ns>
func (h *QueryHandler) QueryLogsLabels(c echo.Context) error {
	claims, ok := getClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil || tenant == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "tenant not found"})
	}

	labelName := c.QueryParam("name")
	query := c.QueryParam("query")
	start := c.QueryParam("start")
	end := c.QueryParam("end")

	var upstream string
	if labelName != "" {
		upstream = fmt.Sprintf("http://%s:%s/loki/api/v1/label/%s/values",
			h.cfg.LokiHost, h.cfg.LokiPort, labelName)
	} else {
		upstream = fmt.Sprintf("http://%s:%s/loki/api/v1/labels",
			h.cfg.LokiHost, h.cfg.LokiPort)
	}

	params := url.Values{}
	if query != "" {
		params.Set("query", query)
	}
	if start != "" {
		params.Set("start", start)
	}
	if end != "" {
		params.Set("end", end)
	}

	headers := map[string]string{"X-Scope-OrgID": tenant.UUID}
	return h.proxyGet(c, upstream+"?"+params.Encode(), headers)
}

// proxyGet forwards a GET request upstream and streams the response back.
func (h *QueryHandler) proxyGet(c echo.Context, upstreamURL string, extraHeaders map[string]string) error {
	req, err := http.NewRequestWithContext(c.Request().Context(), http.MethodGet, upstreamURL, nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to build upstream request"})
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "upstream query failed: " + err.Error()})
	}
	defer resp.Body.Close()

	c.Response().Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	c.Response().WriteHeader(resp.StatusCode)
	_, _ = io.Copy(c.Response().Writer, resp.Body)
	return nil
}

// GrafanaDashboardURL returns a Grafana dashboard link for the authenticated user.
//
// MULTI-TENANT DESIGN NOTES
// ──────────────────────────
// Grafana's datasource config is static (one set of credentials per datasource).
// This means Grafana itself cannot dynamically switch tenants per user session.
//
// Proper multi-tenant Grafana options are:
//
//	A) One Grafana Organisation per tenant — requires the Grafana HTTP API to
//	   provision orgs, datasources, and dashboards per tenant at signup time.
//	B) Per-tenant Grafana instances (expensive, overkill for most cases).
//	C) Don't expose Grafana to tenants at all — use the React dashboard (this app)
//	   for tenant-isolated views, and keep Grafana as an internal admin tool.
//
// Current implementation: option C.
// The returned URL opens the shared Grafana dashboard (scoped to whichever tenant
// is configured in grafana-datasources.yaml). It is intentionally NOT tenant-aware.
// Tenant IDs / UUIDs are NOT embedded in the URL.
//
// To enable option A in the future:
//  1. At tenant signup, call POST /api/orgs + POST /api/orgs/{id}/users + provision
//     datasources with the tenant's credentials via Grafana API.
//  2. Here, look up the tenant's Grafana org ID and embed ?orgId=<tenant_org_id>.
//  3. Use Grafana's OAuth or a signed JWT to auto-login the user to their org.
//
// GET /api/grafana/dashboard-url?dashboard=<uid>&from=<time>&to=<time>
func (h *QueryHandler) GrafanaDashboardURL(c echo.Context) error {
	_, ok := getClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	dashUID := c.QueryParam("dashboard")
	from := c.QueryParam("from")
	to := c.QueryParam("to")
	if from == "" {
		from = "now-1h"
	}
	if to == "" {
		to = "now"
	}

	// cfg.GrafanaURL must be the PUBLIC address (accessible from the browser).
	// In docker-compose, set GRAFANA_URL=http://<host_ip>:3000 in the backend env.
	grafanaBase := h.cfg.GrafanaURL
	if grafanaBase == "" {
		grafanaBase = "http://localhost:3000"
	}

	params := url.Values{}
	params.Set("orgId", "1")
	params.Set("from", from)
	params.Set("to", to)
	params.Set("refresh", "30s")

	var dashURL string
	if dashUID != "" {
		// kiosk=tv hides Grafana navigation chrome when the user opens the link
		params.Set("kiosk", "tv")
		dashURL = fmt.Sprintf("%s/d/%s?%s", grafanaBase, dashUID, params.Encode())
	} else {
		dashURL = fmt.Sprintf("%s/explore?%s", grafanaBase, params.Encode())
	}

	return c.JSON(http.StatusOK, map[string]string{"url": dashURL})
}

func getClaims(c echo.Context) (*infrastructure.Claims, bool) {
	claimsAny := c.Get("claims")
	claims, ok := claimsAny.(*infrastructure.Claims)
	return claims, ok && claims != nil
}
