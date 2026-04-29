package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type ProxiedResponse struct {
	StatusCode  int
	ContentType string
	Body        []byte
}

type QueryMetricsParams struct {
	Query string
	Start string
	End   string
	Step  string
	Type  string
}

type QueryLogsRangeParams struct {
	Query     string
	Start     string
	End       string
	Step      string
	Limit     string
	Direction string
}

type QueryLogsParams struct {
	Query     string
	Limit     string
	Start     string
	End       string
	Direction string
}

type QueryLogLabelsParams struct {
	Name  string
	Query string
	Start string
	End   string
}

type QueryService struct {
	cfg        *config.Config
	tenantRepo *repository.TenantRepository
	httpClient *http.Client
}

func NewQueryService(cfg *config.Config, tenantRepo *repository.TenantRepository) *QueryService {
	return &QueryService{
		cfg:        cfg,
		tenantRepo: tenantRepo,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *QueryService) QueryMetrics(ctx context.Context, tenantID uint, params QueryMetricsParams) (*ProxiedResponse, error) {
	endpoint := "query_range"
	if params.Type == "instant" {
		endpoint = "query"
	}

	upstream := fmt.Sprintf("http://%s:%s/select/%d/prometheus/api/v1/%s",
		s.cfg.VMSelectHost, s.cfg.VMSelectPort, tenantID, endpoint)

	queryParams := url.Values{}
	queryParams.Set("query", params.Query)
	if params.Start != "" {
		queryParams.Set("start", params.Start)
	}
	if params.End != "" {
		queryParams.Set("end", params.End)
	}
	if params.Step != "" {
		queryParams.Set("step", params.Step)
	}

	return s.proxyGet(ctx, upstream+"?"+queryParams.Encode(), nil)
}

func (s *QueryService) QueryLogsRange(ctx context.Context, tenantID uint, params QueryLogsRangeParams) (*ProxiedResponse, error) {
	tenant, err := s.tenantRepo.FindByIDCtx(ctx, tenantID)
	if err != nil || tenant == nil {
		return nil, fmt.Errorf("tenant not found")
	}

	limit := params.Limit
	if limit == "" {
		limit = "1000"
	}

	upstream := fmt.Sprintf("http://%s:%s/loki/api/v1/query_range", s.cfg.LokiHost, s.cfg.LokiPort)
	queryParams := url.Values{}
	queryParams.Set("query", params.Query)
	queryParams.Set("limit", limit)
	if params.Direction != "" {
		queryParams.Set("direction", params.Direction)
	}
	if params.Start != "" {
		queryParams.Set("start", params.Start)
	}
	if params.End != "" {
		queryParams.Set("end", params.End)
	}
	if params.Step != "" {
		queryParams.Set("step", params.Step)
	}

	headers := map[string]string{"X-Scope-OrgID": tenant.UUID}
	return s.proxyGet(ctx, upstream+"?"+queryParams.Encode(), headers)
}

func (s *QueryService) QueryLogs(ctx context.Context, tenantID uint, params QueryLogsParams) (*ProxiedResponse, error) {
	tenant, err := s.tenantRepo.FindByIDCtx(ctx, tenantID)
	if err != nil || tenant == nil {
		return nil, fmt.Errorf("tenant not found")
	}

	limit := params.Limit
	if limit == "" {
		limit = "100"
	}
	direction := params.Direction
	if direction == "" {
		direction = "backward"
	}
	start := params.Start
	end := params.End
	if start == "" || end == "" {
		now := time.Now().UnixNano()
		oneHourAgo := now - int64(60*60*1e9)
		start = fmt.Sprintf("%d", oneHourAgo)
		end = fmt.Sprintf("%d", now)
	}

	upstream := fmt.Sprintf("http://%s:%s/loki/api/v1/query_range", s.cfg.LokiHost, s.cfg.LokiPort)
	queryParams := url.Values{}
	queryParams.Set("query", params.Query)
	queryParams.Set("limit", limit)
	queryParams.Set("direction", direction)
	queryParams.Set("start", start)
	queryParams.Set("end", end)

	headers := map[string]string{"X-Scope-OrgID": tenant.UUID}
	return s.proxyGet(ctx, upstream+"?"+queryParams.Encode(), headers)
}

func (s *QueryService) QueryLogLabels(ctx context.Context, tenantID uint, params QueryLogLabelsParams) (*ProxiedResponse, error) {
	tenant, err := s.tenantRepo.FindByIDCtx(ctx, tenantID)
	if err != nil || tenant == nil {
		return nil, fmt.Errorf("tenant not found")
	}

	var upstream string
	if params.Name != "" {
		upstream = fmt.Sprintf("http://%s:%s/loki/api/v1/label/%s/values", s.cfg.LokiHost, s.cfg.LokiPort, params.Name)
	} else {
		upstream = fmt.Sprintf("http://%s:%s/loki/api/v1/labels", s.cfg.LokiHost, s.cfg.LokiPort)
	}

	queryParams := url.Values{}
	if params.Query != "" {
		queryParams.Set("query", params.Query)
	}
	if params.Start != "" {
		queryParams.Set("start", params.Start)
	}
	if params.End != "" {
		queryParams.Set("end", params.End)
	}

	headers := map[string]string{"X-Scope-OrgID": tenant.UUID}
	return s.proxyGet(ctx, upstream+"?"+queryParams.Encode(), headers)
}

func (s *QueryService) GrafanaDashboardURL(dashboard, from, to string) string {
	if from == "" {
		from = "now-1h"
	}
	if to == "" {
		to = "now"
	}

	grafanaBase := s.cfg.GrafanaURL
	if grafanaBase == "" {
		grafanaBase = "http://localhost:3000"
	}

	params := url.Values{}
	params.Set("orgId", "1")
	params.Set("from", from)
	params.Set("to", to)
	params.Set("refresh", "30s")

	if dashboard != "" {
		params.Set("kiosk", "tv")
		return fmt.Sprintf("%s/d/%s?%s", grafanaBase, dashboard, params.Encode())
	}

	return fmt.Sprintf("%s/explore?%s", grafanaBase, params.Encode())
}

func (s *QueryService) proxyGet(ctx context.Context, upstreamURL string, extraHeaders map[string]string) (*ProxiedResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &ProxiedResponse{
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		Body:        body,
	}, nil
}
