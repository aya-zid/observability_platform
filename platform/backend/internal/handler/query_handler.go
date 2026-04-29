package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
)

type QueryHandler struct {
	queryService *service.QueryService
}

func NewQueryHandler(queryService *service.QueryService) *QueryHandler {
	return &QueryHandler{queryService: queryService}
}

func (h *QueryHandler) QueryMetrics(c echo.Context) error {
	claims, ok := getClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	params := service.QueryMetricsParams{
		Query: c.QueryParam("query"),
		Start: c.QueryParam("start"),
		End:   c.QueryParam("end"),
		Step:  c.QueryParam("step"),
		Type:  c.QueryParam("type"),
	}
	if params.Type == "" {
		params.Type = "range"
	}
	if params.Query == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "query is required"})
	}

	resp, err := h.queryService.QueryMetrics(c.Request().Context(), claims.TenantID, params)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "upstream query failed: " + err.Error()})
	}
	return writeProxiedResponse(c, resp)
}

func (h *QueryHandler) QueryLogsRange(c echo.Context) error {
	claims, ok := getClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	params := service.QueryLogsRangeParams{
		Query:     c.QueryParam("query"),
		Start:     c.QueryParam("start"),
		End:       c.QueryParam("end"),
		Step:      c.QueryParam("step"),
		Limit:     c.QueryParam("limit"),
		Direction: c.QueryParam("direction"),
	}
	if params.Query == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "query is required"})
	}

	resp, err := h.queryService.QueryLogsRange(c.Request().Context(), claims.TenantID, params)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "upstream query failed: " + err.Error()})
	}
	return writeProxiedResponse(c, resp)
}

func (h *QueryHandler) QueryLogs(c echo.Context) error {
	claims, ok := getClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	params := service.QueryLogsParams{
		Query:     c.QueryParam("query"),
		Limit:     c.QueryParam("limit"),
		Start:     c.QueryParam("start"),
		End:       c.QueryParam("end"),
		Direction: c.QueryParam("direction"),
	}
	if params.Query == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "query is required"})
	}

	resp, err := h.queryService.QueryLogs(c.Request().Context(), claims.TenantID, params)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "upstream query failed: " + err.Error()})
	}
	return writeProxiedResponse(c, resp)
}

func (h *QueryHandler) QueryLogsLabels(c echo.Context) error {
	claims, ok := getClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	resp, err := h.queryService.QueryLogLabels(c.Request().Context(), claims.TenantID, service.QueryLogLabelsParams{
		Name:  c.QueryParam("name"),
		Query: c.QueryParam("query"),
		Start: c.QueryParam("start"),
		End:   c.QueryParam("end"),
	})
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "upstream query failed: " + err.Error()})
	}
	return writeProxiedResponse(c, resp)
}

func (h *QueryHandler) GrafanaDashboardURL(c echo.Context) error {
	_, ok := getClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"url": h.queryService.GrafanaDashboardURL(
			c.QueryParam("dashboard"),
			c.QueryParam("from"),
			c.QueryParam("to"),
		),
	})
}

func writeProxiedResponse(c echo.Context, resp *service.ProxiedResponse) error {
	if resp.ContentType != "" {
		c.Response().Header().Set("Content-Type", resp.ContentType)
	}
	return c.Blob(resp.StatusCode, resp.ContentType, resp.Body)
}

func getClaims(c echo.Context) (*infrastructure.Claims, bool) {
	claimsAny := c.Get("claims")
	claims, ok := claimsAny.(*infrastructure.Claims)
	return claims, ok && claims != nil
}
