// internal/service/nginx_dashboard_service.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
)

// ─── Response types ───────────────────────────────────────────────────────────

// SlowRequest is one entry in the slow-requests table on the dashboard.
type SlowRequest struct {
	Path           string  `json:"path"`
	NormalizedPath string  `json:"normalized_path"`
	Method         string  `json:"method"`
	Status         int     `json:"status"`
	IP             string  `json:"ip"`
	RT             float64 `json:"rt"`
	Referer        string  `json:"referer"`
	UserAgent      string  `json:"user_agent"`
	Timestamp      string  `json:"timestamp"`
}

// NginxDashboardSummary is the single JSON payload returned to the frontend.
// Every field that the old NginxDashboardPage needed is present here.
type NginxDashboardSummary struct {
	// ── Status timeseries ──
	// Each TimePoint: {"time": unixSeconds, "2xx": rate, "3xx": rate, "4xx": rate, "5xx": rate}
	StatusTimeseries []TimePoint `json:"status_timeseries"`

	// ── Response-time percentiles timeseries ──
	// Each TimePoint: {"time": unixSeconds, "p50": ms, "p95": ms, "p99": ms}
	RTTimeseries []TimePoint `json:"rt_timeseries"`

	// ── Request rate timeseries ──
	// Each TimePoint: {"time": unixSeconds, "req": req/s}
	ReqRateTimeseries []TimePoint `json:"req_rate_timeseries"`

	// ── Bandwidth timeseries ──
	// Each TimePoint: {"time": unixSeconds, "bytes": bytes/s}
	BandwidthTimeseries []TimePoint `json:"bandwidth_timeseries"`

	// ── nginx_connections_current timeseries (from VictoriaMetrics) ──
	// Each TimePoint: {"time": unixSeconds, "active": n, "waiting": n, "reading": n, "writing": n}
	ConnTimeseries []TimePoint `json:"conn_timeseries"`

	// ── VM cumulative timeseries ──
	VMTotalConnTimeseries []TimePoint `json:"vm_total_conn_timeseries"` // {"time", "total"}
	VMRequestsTimeseries  []TimePoint `json:"vm_requests_timeseries"`   // {"time", "requests"}

	// ── Bot vs Human timeseries ──
	// Each TimePoint: {"time": unixSeconds, "botRate": req/s, "totalRate": req/s}
	BotTimeseries []TimePoint `json:"bot_timeseries"`

	// ── Scalar totals (for status breakdown cards) ──
	TotalRequests float64 `json:"total_requests"`
	Total2xx      float64 `json:"total_2xx"`
	Total3xx      float64 `json:"total_3xx"`
	Total4xx      float64 `json:"total_4xx"`
	Total5xx      float64 `json:"total_5xx"`

	// ── Aggregation tables ──
	TopPaths      []LabelCount    `json:"top_paths"`
	TopIPs        []LabelCount    `json:"top_ips"`
	TopStatuses   []LabelCount    `json:"top_statuses"`
	TopMethods    []LabelCount    `json:"top_methods"`
	TopErrorPaths []ErrorPathStat `json:"top_error_paths"`
	// ── Normalized path aggregations ──
	TopNormalizedPaths      []LabelCount    `json:"top_normalized_paths"`
	TopNormalizedErrorPaths []ErrorPathStat `json:"top_normalized_error_paths"`
	SlowRequestsNormalized  []SlowRequest   `json:"slow_requests_normalized"`
	SuspiciousIPs           []SuspiciousIP  `json:"suspicious_ips"`

	// ── Latest scalar connection values (for stat cards) ──
	ConnActive  *float64 `json:"conn_active,omitempty"`
	ConnWaiting *float64 `json:"conn_waiting,omitempty"`
	ConnReading *float64 `json:"conn_reading,omitempty"`
	ConnWriting *float64 `json:"conn_writing,omitempty"`

	// ── VM scalar counters (latest values for stat cards) ──
	VMRequestsTotal *float64 `json:"vm_requests_total,omitempty"`
	VMAcceptedTotal *float64 `json:"vm_accepted_total,omitempty"`
	VMHandledTotal  *float64 `json:"vm_handled_total,omitempty"`

	// ── Slow requests table (top 100, rt > minRT) ──
	SlowRequests []SlowRequest `json:"slow_requests"`
	// Total slow requests found in the time range (before truncation to 100).
	// Use this to render the "Showing 100 of N" label and gate the "View More" button.
	SlowRequestsTotal int `json:"slow_requests_total"`
	// ── Normalized slow requests (top 100) ──
	// SlowRequestsNormalized is already defined above; this is just the total count.
	SlowRequestsNormalizedTotal int `json:"slow_requests_normalized_total"`
	// ── The threshold used to compute the above two lists (seconds) ──
	// Echoed back so the frontend can label the section correctly, e.g. "Slow (> 2s)".
	SlowRequestsMinRT float64 `json:"slow_requests_min_rt"`
}

// TimePoint is a generic map for chart data points.
// The "time" key always holds unix seconds (float64).
type TimePoint map[string]interface{}

type LabelCount struct {
	Label string  `json:"label"`
	Count float64 `json:"count"`
}

type ErrorPathStat struct {
	Path   string  `json:"path"`
	Errors float64 `json:"errors"`
	Total  float64 `json:"total"`
	ErrPct float64 `json:"err_pct"`
}

type SuspiciousIP struct {
	IP     string  `json:"ip"`
	Errors float64 `json:"errors"`
	Total  float64 `json:"total"`
	Rate   float64 `json:"rate"`
}

type LokiLogEntry struct {
	Line     string
	Metadata map[string]string
}

// ─── Explorer types ───────────────────────────────────────────────────────────

type NginxExplorerResult struct {
	RTTimeseries []TimePoint  `json:"rt_timeseries"`
	RequestRate  []TimePoint  `json:"request_rate"`
	TopStatuses  []LabelCount `json:"top_statuses"`
	TopPaths     []LabelCount `json:"top_paths"`
	TotalCount   float64      `json:"total_count"`
}

type NginxDomainItem struct {
	Domain      string `json:"domain"`
	Enabled     bool   `json:"enabled"`
	IsDefault   bool   `json:"is_default"`
	IsSharedLog bool   `json:"is_shared_log"`
}

type NginxExplorerFilter struct {
	Domain      string
	IP          string
	PathPrefix  string
	StatusClass string
	MinRT       float64
}

// ─── Service ──────────────────────────────────────────────────────────────────

type NginxDashboardService struct {
	cfg        *config.Config
	redis      *redis.Client
	httpClient *http.Client
}

func NewNginxDashboardService(cfg *config.Config, redisClient *redis.Client) *NginxDashboardService {
	return &NginxDashboardService{
		cfg:        cfg,
		redis:      redisClient,
		httpClient: &http.Client{Timeout: 45 * time.Second},
	}
}

const dashboardCacheTTL = 20 * time.Second

func (s *NginxDashboardService) GetDashboardSummary(
	ctx context.Context,
	tenantUUID string,
	tenantID uint,
	serviceName string,
	domain string,
	startNs, endNs int64,
	step string,
	minRT float64, // slow-request threshold in seconds; 0 means use default (1.0)
) (*NginxDashboardSummary, error) {
	if minRT <= 0 {
		minRT = 1.0
	}
	// Floor startNs/endNs to the nearest second to prevent per-millisecond cache misses.
	// Two fetches 1 second apart for the same range should hit the same cache entry.
	startSec := startNs / int64(1e9)
	endSec := endNs / int64(1e9)
	// Include minRT in cache key so different thresholds don't share a cached result.
	cacheKey := fmt.Sprintf("nginx:dash:%s:%s:%s:%d:%d:%s:%.3f", tenantUUID, serviceName, domain, startSec, endSec, step, minRT)

	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var result NginxDashboardSummary
			if json.Unmarshal(cached, &result) == nil {
				return &result, nil
			}
		}
	}

	summary, err := s.fetchDashboard(ctx, tenantUUID, tenantID, serviceName, domain, startNs, endNs, step, minRT)
	if err != nil {
		return nil, err
	}

	if s.redis != nil {
		if b, err := json.Marshal(summary); err == nil {
			_ = s.redis.Set(ctx, cacheKey, b, dashboardCacheTTL).Err()
		}
	}

	return summary, nil
}

// ─── Parse expressions ────────────────────────────────────────────────────────

// parseExpr extracts ip, method, path, status, bytes from a standard nginx combined log line.
const parseExpr = `| regexp ` + "`" +
	`^(?P<ip>[^ ]+) [^ ]+ [^ ]+ \[[^\]]+\] "(?P<method>[A-Z]+) (?P<path>[^ ]+) [^"]*" (?P<status>\d{3}) (?P<bytes>[\d-]+)` +
	"`"

// rtParseExpr additionally extracts rt=<seconds> appended at end of log line.
const rtParseExpr = `| regexp ` + "`" + `rt=(?P<rt>[0-9.]+)$` + "`"

// ─── fetchDashboard: runs all queries in parallel ─────────────────────────────

func (s *NginxDashboardService) fetchDashboard(
	ctx context.Context,
	tenantUUID string,
	tenantID uint,
	serviceName string,
	domain string,
	startNs, endNs int64,
	step string,
	minRT float64,
) (*NginxDashboardSummary, error) {
	// "access" is the default — dashboard metrics must never touch error logs.
	base := lokiSelector(serviceName, domain, "access")
	rangeSeconds := (endNs - startNs) / int64(1e9) // integer division — keeps exact seconds

	// ── Loki queries ──────────────────────────────────────────────────────────

	// 1. Status class timeseries
	// Run one query per class and merge locally. This avoids relying on label
	// mutation in the Loki pipeline, which can otherwise produce empty series.
	status2xxQ := fmt.Sprintf(`sum(rate(%s %s | status =~ "2.." [%s]))`, base, parseExpr, step)
	status3xxQ := fmt.Sprintf(`sum(rate(%s %s | status =~ "3.." [%s]))`, base, parseExpr, step)
	status4xxQ := fmt.Sprintf(`sum(rate(%s %s | status =~ "4.." [%s]))`, base, parseExpr, step)
	status5xxQ := fmt.Sprintf(`sum(rate(%s %s | status =~ "5.." [%s]))`, base, parseExpr, step)

	// 2. RT percentiles (3 separate goroutines, all parallel)
	// Aggregate into a single output series across all matching streams so the
	// frontend gets one clean p50/p95/p99 line instead of a partial multi-stream merge.
	rtP50Q := fmt.Sprintf(`quantile_over_time(0.50, %s %s | unwrap rt [%s]) by ()`, base, rtParseExpr, step)
	rtP95Q := fmt.Sprintf(`quantile_over_time(0.95, %s %s | unwrap rt [%s]) by ()`, base, rtParseExpr, step)
	rtP99Q := fmt.Sprintf(`quantile_over_time(0.99, %s %s | unwrap rt [%s]) by ()`, base, rtParseExpr, step)

	// 3. Request rate over time
	reqRateQ := fmt.Sprintf(`sum(rate(%s [%s]))`, base, step)

	// 4. Bandwidth (bytes/s) over time
	bandwidthQ := fmt.Sprintf(
		`sum(rate(%s %s | unwrap bytes [%s]))`,
		base, parseExpr, step,
	)

	// 5. Bot detection rate
	botQ := fmt.Sprintf(
		`sum(rate(%s |~ `+"`"+`(?i)bot|crawl|spider|curl|wget|python|java|go-http`+"`"+` [%s]))`,
		base, step,
	)
	totalRateQ := fmt.Sprintf(`sum(rate(%s [%s]))`, base, step)

	// 6. Top aggregation — instant query at endNs with range vector [rangeSeconds].
	//    Anchored at endNs, it covers exactly [endNs - rangeSeconds, endNs] with no
	//    wall-clock snapping. count_over_time([rangeSeconds]) counts every log exactly once.
	topAggQ := fmt.Sprintf(
		`topk(50, sum by (path, status, method, ip) (count_over_time(%s %s [%ds])))`,
		base, parseExpr, rangeSeconds,
	)
	topNormAggQ := fmt.Sprintf(
		`topk(50, sum by (normalized_path, status, method, ip) (count_over_time(%s | normalized_path!="" %s [%ds])))`,
		base, parseExpr, rangeSeconds,
	)
	slowNormQ := fmt.Sprintf(
		`topk(100, max by (normalized_path, method, status) (max_over_time(%s | normalized_path!="" %s %s | unwrap rt [%ds])))`,
		base, parseExpr, rtParseExpr, rangeSeconds,
	)

	// 7. Slow requests — fetch 500 recent log lines to find rt>minRT entries
	//    These are actual log lines (streams), not metric queries.
	slowLogsQ := base

	// ── VM queries ────────────────────────────────────────────────────────────
	connActiveQ := `nginx_connections_current{state="active"}`
	connWaitingQ := `nginx_connections_current{state="waiting"}`
	connReadingQ := `nginx_connections_current{state="reading"}`
	connWritingQ := `nginx_connections_current{state="writing"}`
	requestsTotalQ := `nginx_requests_total`
	acceptedTotalQ := `nginx_connections_accepted_total`
	handledTotalQ := `nginx_connections_handled_total`
	totalConnQ := `sum(nginx_connections_current)`

	// ── Result holders ────────────────────────────────────────────────────────
	type lR struct {
		data []LokiMatrixSeries
		err  error
	}
	type vR struct {
		data []VMSeries
		err  error
	}
	type logR struct {
		entries []LokiLogEntry
		err     error
	}

	var (
		wg          sync.WaitGroup
		status2xx   lR
		status3xx   lR
		status4xx   lR
		status5xx   lR
		rtP50       lR
		rtP95       lR
		rtP99       lR
		reqRate     lR
		bandwidth   lR
		botRate     lR
		totalRate   lR
		topAgg      lR
		topNormAgg  lR
		slowNormAgg lR
		connActive  vR
		connWaiting vR
		connReading vR
		connWriting vR
		reqTotal    vR
		accTotal    vR
		handTotal   vR
		totalConn   vR
		slowLogs    logR
	)

	lq := func(q string) lR {
		d, e := s.lokiQueryRange(ctx, tenantUUID, q, startNs, endNs, step, 5000)
		return lR{d, e}
	}
	vq := func(q string) vR {
		d, e := s.vmQueryRange(ctx, tenantID, q, startNs/int64(1e9), endNs/int64(1e9), step)
		return vR{d, e}
	}

	wg.Add(23)
	go func() { defer wg.Done(); status2xx = lq(status2xxQ) }()
	go func() { defer wg.Done(); status3xx = lq(status3xxQ) }()
	go func() { defer wg.Done(); status4xx = lq(status4xxQ) }()
	go func() { defer wg.Done(); status5xx = lq(status5xxQ) }()
	go func() { defer wg.Done(); rtP50 = lq(rtP50Q) }()
	go func() { defer wg.Done(); rtP95 = lq(rtP95Q) }()
	go func() { defer wg.Done(); rtP99 = lq(rtP99Q) }()
	go func() { defer wg.Done(); reqRate = lq(reqRateQ) }()
	go func() { defer wg.Done(); bandwidth = lq(bandwidthQ) }()
	go func() { defer wg.Done(); botRate = lq(botQ) }()
	go func() { defer wg.Done(); totalRate = lq(totalRateQ) }()
	go func() {
		defer wg.Done()
		d, e := s.lokiQueryInstant(ctx, tenantUUID, topAggQ, endNs, 5000)
		topAgg = lR{d, e}
	}()
	go func() {
		defer wg.Done()
		d, e := s.lokiQueryInstant(ctx, tenantUUID, topNormAggQ, endNs, 5000)
		topNormAgg = lR{d, e}
	}()
	go func() {
		defer wg.Done()
		d, e := s.lokiQueryInstant(ctx, tenantUUID, slowNormQ, endNs, 5000)
		slowNormAgg = lR{d, e}
	}()
	go func() { defer wg.Done(); connActive = vq(connActiveQ) }()
	go func() { defer wg.Done(); connWaiting = vq(connWaitingQ) }()
	go func() { defer wg.Done(); connReading = vq(connReadingQ) }()
	go func() { defer wg.Done(); connWriting = vq(connWritingQ) }()
	go func() { defer wg.Done(); reqTotal = vq(requestsTotalQ) }()
	go func() { defer wg.Done(); accTotal = vq(acceptedTotalQ) }()
	go func() { defer wg.Done(); handTotal = vq(handledTotalQ) }()
	go func() { defer wg.Done(); totalConn = vq(totalConnQ) }()
	go func() {
		defer wg.Done()
		entries, err := s.lokiQueryLogEntries(ctx, tenantUUID, slowLogsQ, startNs, endNs, 500)
		slowLogs = logR{entries, err}
	}()
	wg.Wait()

	// Non-fatal: if individual queries fail, we surface empty data for that section
	// rather than failing the whole dashboard. Log errors are swallowed here;
	// add structured logging as needed.

	summary := &NginxDashboardSummary{}

	// ── Status timeseries ─────────────────────────────────────────────────────
	summary.StatusTimeseries = mergeNamedLokiSeries(map[string][]LokiMatrixSeries{
		"2xx": status2xx.data,
		"3xx": status3xx.data,
		"4xx": status4xx.data,
		"5xx": status5xx.data,
	})

	// ── RT timeseries (merge p50/p95/p99, convert seconds → ms) ──────────────
	summary.RTTimeseries = mergeRTSeries(rtP50.data, rtP95.data, rtP99.data)

	// ── Request rate timeseries ───────────────────────────────────────────────
	if len(reqRate.data) > 0 {
		for _, pt := range reqRate.data[0].Values {
			summary.ReqRateTimeseries = append(summary.ReqRateTimeseries,
				TimePoint{"time": pt[0], "req": pt[1]})
		}
	}

	// ── Bandwidth timeseries ──────────────────────────────────────────────────
	if len(bandwidth.data) > 0 {
		for _, pt := range bandwidth.data[0].Values {
			summary.BandwidthTimeseries = append(summary.BandwidthTimeseries,
				TimePoint{"time": pt[0], "bytes": pt[1]})
		}
	}

	// ── Bot timeseries (merge botRate + totalRate) ────────────────────────────
	summary.BotTimeseries = mergeBotSeries(botRate.data, totalRate.data)

	// ── Connection timeseries (active/waiting/reading/writing merged by time) ─
	summary.ConnTimeseries = mergeVMConnSeries(connActive.data, connWaiting.data, connReading.data, connWriting.data)

	// ── VM cumulative timeseries ──────────────────────────────────────────────
	if len(totalConn.data) > 0 {
		for _, pt := range totalConn.data[0].Values {
			summary.VMTotalConnTimeseries = append(summary.VMTotalConnTimeseries,
				TimePoint{"time": pt[0], "total": pt[1]})
		}
	}
	if len(reqTotal.data) > 0 {
		for _, pt := range reqTotal.data[0].Values {
			summary.VMRequestsTimeseries = append(summary.VMRequestsTimeseries,
				TimePoint{"time": pt[0], "requests": pt[1]})
		}
	}

	// ── Latest scalar values for stat cards ───────────────────────────────────
	summary.ConnActive = latestVMValue(connActive.data)
	summary.ConnWaiting = latestVMValue(connWaiting.data)
	summary.ConnReading = latestVMValue(connReading.data)
	summary.ConnWriting = latestVMValue(connWriting.data)
	summary.VMRequestsTotal = latestVMValue(reqTotal.data)
	summary.VMAcceptedTotal = latestVMValue(accTotal.data)
	summary.VMHandledTotal = latestVMValue(handTotal.data)

	// ── Aggregation from topAgg query ─────────────────────────────────────────
	pathCounts := map[string]float64{}
	ipCounts := map[string]float64{}
	statusCounts := map[string]float64{}
	methodCounts := map[string]float64{}
	pathErrorCounts := map[string]float64{}
	ipErrorCounts := map[string]float64{}

	for _, series := range topAgg.data {
		path := series.Metric["path"]
		ip := series.Metric["ip"]
		status := series.Metric["status"]
		method := series.Metric["method"]
		count := lastValue(series.Values)

		if path != "" {
			pathCounts[path] += count
		}
		if ip != "" {
			ipCounts[ip] += count
		}
		if status != "" {
			statusCounts[status] += count
			switch status[0] {
			case '2':
				summary.Total2xx += count
			case '3':
				summary.Total3xx += count
			case '4':
				summary.Total4xx += count
				if path != "" {
					pathErrorCounts[path] += count
				}
				if ip != "" {
					ipErrorCounts[ip] += count
				}
			case '5':
				summary.Total5xx += count
				if path != "" {
					pathErrorCounts[path] += count
				}
				if ip != "" {
					ipErrorCounts[ip] += count
				}
			}
		}
		if method != "" {
			methodCounts[method] += count
		}
		summary.TotalRequests += count
	}

	summary.TopPaths = topN(pathCounts, 10)
	summary.TopIPs = topN(ipCounts, 10)
	summary.TopStatuses = topN(statusCounts, 20)
	summary.TopMethods = topN(methodCounts, 10)

	// Top error paths by raw path (existing — unchanged)
	for _, pc := range summary.TopPaths {
		errs := pathErrorCounts[pc.Label]
		if errs == 0 {
			continue
		}
		pct := 0.0
		if pc.Count > 0 {
			pct = (errs / pc.Count) * 100
		}
		summary.TopErrorPaths = append(summary.TopErrorPaths, ErrorPathStat{
			Path: pc.Label, Errors: errs, Total: pc.Count, ErrPct: pct,
		})
	}
	sortErrorPaths(summary.TopErrorPaths)

	normPathCounts := map[string]float64{}
	normPathErrorCounts := map[string]float64{}
	for _, series := range topNormAgg.data {
		normPath := series.Metric["normalized_path"]
		status := series.Metric["status"]
		count := lastValue(series.Values)
		if normPath == "" {
			continue
		}
		normPathCounts[normPath] += count
		if status == "" {
			continue
		}
		if status[0] == '4' || status[0] == '5' {
			normPathErrorCounts[normPath] += count
		}
	}

	summary.TopNormalizedPaths = topN(normPathCounts, 10)
	for _, pc := range summary.TopNormalizedPaths {
		errs := normPathErrorCounts[pc.Label]
		if errs == 0 {
			continue
		}
		pct := 0.0
		if pc.Count > 0 {
			pct = (errs / pc.Count) * 100
		}
		summary.TopNormalizedErrorPaths = append(summary.TopNormalizedErrorPaths, ErrorPathStat{
			Path: pc.Label, Errors: errs, Total: pc.Count, ErrPct: pct,
		})
	}
	sortErrorPaths(summary.TopNormalizedErrorPaths)

	// Suspicious IPs (error rate > 30%)
	for ip, errs := range ipErrorCounts {
		total := ipCounts[ip]
		if total == 0 {
			continue
		}
		rate := (errs / total) * 100
		if rate > 30 {
			summary.SuspiciousIPs = append(summary.SuspiciousIPs, SuspiciousIP{
				IP: ip, Errors: errs, Total: total, Rate: rate,
			})
		}
	}
	sortSuspiciousIPs(summary.SuspiciousIPs)

	// ── Slow requests (parse log lines, filter rt > minRT) ───────────────────
	slowReqs, slowTotal := parseSlowRequests(slowLogs.entries, 100, minRT)
	summary.SlowRequests = slowReqs
	summary.SlowRequestsTotal = slowTotal
	summary.SlowRequestsMinRT = minRT

	// Slow normalized requests come directly from Loki's normalized_path metadata,
	// not from regrouping raw paths on the application side.
	slowNorm, slowNormTotal := buildSlowRequestsNormalized(slowNormAgg.data, 100, minRT)
	summary.SlowRequestsNormalized = slowNorm
	summary.SlowRequestsNormalizedTotal = slowNormTotal

	return summary, nil
}

// ─── SlowRequestsList (not cached — used by "View More" page) ────────────────

// SlowRequestsListResult is the paginated response for the full slow-requests page.
type SlowRequestsListResult struct {
	Items      []SlowRequest `json:"items"`
	Total      int           `json:"total"`
	Page       int           `json:"page"`
	PerPage    int           `json:"per_page"`
	TotalPages int           `json:"total_pages"`
	MinRT      float64       `json:"min_rt"`
}

// GetSlowRequestsList fetches up to 5 000 log lines and returns a paginated list
// of slow requests. It is the backing service for the "View More" full-page view.
func (s *NginxDashboardService) GetSlowRequestsList(
	ctx context.Context,
	tenantUUID string,
	serviceName string,
	domain string,
	startNs, endNs int64,
	minRT float64,
	page, perPage int,
) (*SlowRequestsListResult, error) {
	if minRT <= 0 {
		minRT = 1.0
	}
	if perPage <= 0 || perPage > 500 {
		perPage = 100
	}
	if page <= 0 {
		page = 1
	}
	base := lokiSelector(serviceName, domain, "access")
	// Fetch a generous batch — 5 000 lines covers most time windows even for busy servers.
	entries, err := s.lokiQueryLogEntries(ctx, tenantUUID, base, startNs, endNs, 5000)
	if err != nil {
		return nil, fmt.Errorf("loki query failed: %w", err)
	}
	// Parse all matching slow requests (no truncation yet)
	all, total := parseSlowRequests(entries, len(entries), minRT)
	// all is already sorted by rt desc
	totalPages := (total + perPage - 1) / perPage
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * perPage
	end := start + perPage
	if end > total {
		end = total
	}
	var pageItems []SlowRequest
	if start < total {
		pageItems = all[start:end]
	} else {
		pageItems = []SlowRequest{}
	}
	return &SlowRequestsListResult{
		Items:      pageItems,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
		MinRT:      minRT,
	}, nil
}

// ─── Explorer (not cached) ────────────────────────────────────────────────────

func (s *NginxDashboardService) GetExplorerResult(
	ctx context.Context,
	tenantUUID string,
	serviceName string,
	startNs, endNs int64,
	step string,
	filter NginxExplorerFilter,
) (*NginxExplorerResult, error) {
	// Explorer always operates on access logs — error logs have no parseable fields
	// for status/path/method/ip filtering.
	base := lokiSelector(serviceName, filter.Domain, "access")
	filterChain := base + " " + parseExpr

	if filter.IP != "" {
		filterChain += fmt.Sprintf(` | ip = "%s"`, filter.IP)
	}
	if filter.PathPrefix != "" {
		filterChain += fmt.Sprintf(` | path =~ "%s.*"`, filter.PathPrefix)
	}
	if filter.StatusClass != "" && filter.StatusClass != "all" {
		switch filter.StatusClass {
		case "2xx":
			filterChain += ` | status =~ "2.."`
		case "3xx":
			filterChain += ` | status =~ "3.."`
		case "4xx":
			filterChain += ` | status =~ "4.."`
		case "5xx":
			filterChain += ` | status =~ "5.."`
		}
	}

	rtBase := filterChain
	if filter.MinRT > 0 {
		rtBase += fmt.Sprintf(` %s | rt > %s`, rtParseExpr,
			strconv.FormatFloat(filter.MinRT, 'f', -1, 64))
	} else {
		rtBase += " " + rtParseExpr
	}

	type lR struct {
		data []LokiMatrixSeries
		err  error
	}
	var (
		wg     sync.WaitGroup
		rtP50  lR
		rtP95  lR
		rate   lR
		topAgg lR
	)

	rangeSeconds := (endNs - startNs) / int64(1e9)

	wg.Add(4)
	go func() {
		defer wg.Done()
		d, e := s.lokiQueryRange(ctx, tenantUUID,
			fmt.Sprintf(`quantile_over_time(0.50, %s | unwrap rt [%s])`, rtBase, step),
			startNs, endNs, step, 5000)
		rtP50 = lR{d, e}
	}()
	go func() {
		defer wg.Done()
		d, e := s.lokiQueryRange(ctx, tenantUUID,
			fmt.Sprintf(`quantile_over_time(0.95, %s | unwrap rt [%s])`, rtBase, step),
			startNs, endNs, step, 5000)
		rtP95 = lR{d, e}
	}()
	go func() {
		defer wg.Done()
		d, e := s.lokiQueryRange(ctx, tenantUUID,
			fmt.Sprintf(`sum(rate(%s [%s]))`, filterChain, step),
			startNs, endNs, step, 5000)
		rate = lR{d, e}
	}()
	go func() {
		defer wg.Done()
		// Single-step range query: step = full window so there is exactly ONE
		// evaluation point and count_over_time counts each log exactly once.
		topAggStep := fmt.Sprintf("%ds", rangeSeconds)
		d, e := s.lokiQueryRange(ctx, tenantUUID,
			fmt.Sprintf(`topk(20, sum by (path, status) (count_over_time(%s [%ds])))`, filterChain, rangeSeconds),
			startNs, endNs, topAggStep, 5000)
		topAgg = lR{d, e}
	}()
	wg.Wait()

	result := &NginxExplorerResult{}
	result.RTTimeseries = mergeRTSeries(rtP50.data, rtP95.data, nil)
	if len(rate.data) > 0 {
		for _, pt := range rate.data[0].Values {
			result.RequestRate = append(result.RequestRate, TimePoint{"time": pt[0], "rate": pt[1]})
		}
	}
	pathCounts := map[string]float64{}
	statusCounts := map[string]float64{}
	for _, series := range topAgg.data {
		count := lastValue(series.Values)
		result.TotalCount += count
		if p := series.Metric["path"]; p != "" {
			pathCounts[p] += count
		}
		if st := series.Metric["status"]; st != "" {
			statusCounts[st] += count
		}
	}
	result.TopPaths = topN(pathCounts, 10)
	result.TopStatuses = topN(statusCounts, 20)
	return result, nil
}

// GetErrorLogs returns raw nginx error log lines for the given service/domain/window.
// Error logs are deliberately NOT parsed — they're free-form text and must be
// displayed as-is in the UI. Up to `limit` lines are returned, newest first.
func (s *NginxDashboardService) GetErrorLogs(
	ctx context.Context,
	tenantUUID string,
	serviceName string,
	domain string,
	startNs, endNs int64,
	limit int,
) ([]string, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	sel := lokiSelector(serviceName, domain, "error")
	return s.lokiQueryLogLines(ctx, tenantUUID, sel, startNs, endNs, limit)
}

// By default it targets access logs only (log_kind="access") — error logs have
// a completely different format and would corrupt all metric calculations.
// Pass logKind="error" explicitly to query error logs instead.
func lokiSelector(serviceName, domain, logKind string) string {
	if logKind == "" {
		logKind = "access"
	}
	// log_kind and vhost are structured metadata labels set by the OTel agent.
	// They must be filtered with | label="value" (pipeline filter), NOT inside
	// the stream selector {} — Loki only indexes labels declared at stream
	// ingestion time inside {}, and structured metadata isn't one of them.
	selector := fmt.Sprintf(`{service_name="%s"}`, lokiLabelValue(serviceName))
	selector += fmt.Sprintf(` | log_kind="%s"`, lokiLabelValue(logKind))
	if domain = strings.TrimSpace(domain); domain != "" {
		selector += fmt.Sprintf(` | vhost="%s"`, lokiLabelValue(domain))
	}
	return selector
}

func lokiLabelValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}

// ─── HTTP helpers ─────────────────────────────────────────────────────────────

type LokiMatrixSeries struct {
	Metric map[string]string
	Values [][2]float64
}

type VMSeries struct {
	Metric map[string]string
	Values [][2]float64
}

func (s *NginxDashboardService) lokiQueryRange(
	ctx context.Context,
	tenantUUID, query string,
	startNs, endNs int64,
	step string,
	limit int,
) ([]LokiMatrixSeries, error) {
	upstream := fmt.Sprintf("http://%s:%s/loki/api/v1/query_range", s.cfg.LokiHost, s.cfg.LokiPort)
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", strconv.FormatInt(startNs, 10))
	params.Set("end", strconv.FormatInt(endNs, 10))
	params.Set("step", step)
	params.Set("limit", strconv.Itoa(limit))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Scope-OrgID", tenantUUID)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseLokiMatrixResponse(body)
}

// lokiQueryLogLines fetches actual log lines (stream resultType) from Loki.
// Used for the slow-requests table.
// lokiQueryInstant runs a Loki instant query at the given timestampNs.
// It returns results in the same LokiMatrixSeries shape as lokiQueryRange so
// callers can use them identically. Loki instant queries return resultType
// "vector" (one value per stream) rather than "matrix" (multiple values).
// parseLokiInstantResponse converts that into a single-value series list.
func (s *NginxDashboardService) lokiQueryInstant(
	ctx context.Context,
	tenantUUID, query string,
	timeNs int64,
	limit int,
) ([]LokiMatrixSeries, error) {
	upstream := fmt.Sprintf("http://%s:%s/loki/api/v1/query", s.cfg.LokiHost, s.cfg.LokiPort)
	params := url.Values{}
	params.Set("query", query)
	params.Set("time", strconv.FormatInt(timeNs, 10))
	params.Set("limit", strconv.Itoa(limit))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Scope-OrgID", tenantUUID)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseLokiInstantResponse(body)
}

// parseLokiInstantResponse parses a Loki /loki/api/v1/query response.
// Instant queries return resultType "vector": each result has a single [ts, value]
// pair rather than an array of pairs. We convert to LokiMatrixSeries so the
// rest of the aggregation code doesn't need two separate paths.
func parseLokiInstantResponse(body []byte) ([]LokiMatrixSeries, error) {
	var raw struct {
		Data struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric map[string]string `json:"metric"`
				Value  []interface{}     `json:"value"` // [timestamp, value]
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := make([]LokiMatrixSeries, 0, len(raw.Data.Result))
	for _, r := range raw.Data.Result {
		series := LokiMatrixSeries{Metric: r.Metric}
		if len(r.Value) >= 2 {
			series.Values = [][2]float64{{toFloat(r.Value[0]), toFloat(r.Value[1])}}
		}
		out = append(out, series)
	}
	return out, nil
}

func (s *NginxDashboardService) lokiQueryLogLines(
	ctx context.Context,
	tenantUUID, query string,
	startNs, endNs int64,
	limit int,
) ([]string, error) {
	entries, err := s.lokiQueryLogEntries(ctx, tenantUUID, query, startNs, endNs, limit)
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, entry.Line)
	}
	return lines, nil
}

func (s *NginxDashboardService) lokiQueryLogEntries(
	ctx context.Context,
	tenantUUID, query string,
	startNs, endNs int64,
	limit int,
) ([]LokiLogEntry, error) {
	upstream := fmt.Sprintf("http://%s:%s/loki/api/v1/query_range", s.cfg.LokiHost, s.cfg.LokiPort)
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", strconv.FormatInt(startNs, 10))
	params.Set("end", strconv.FormatInt(endNs, 10))
	params.Set("limit", strconv.Itoa(limit))
	params.Set("direction", "backward")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Scope-OrgID", tenantUUID)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseLokiStreamEntries(body), nil
}

func (s *NginxDashboardService) vmQueryRange(
	ctx context.Context,
	tenantID uint,
	query string,
	startSec, endSec int64,
	step string,
) ([]VMSeries, error) {
	upstream := fmt.Sprintf("http://%s:%s/select/%d/prometheus/api/v1/query_range",
		s.cfg.VMSelectHost, s.cfg.VMSelectPort, tenantID)
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", strconv.FormatInt(startSec, 10))
	params.Set("end", strconv.FormatInt(endSec, 10))
	params.Set("step", step)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
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
	return parseVMMatrixResponse(body)
}

// ─── Response parsers ─────────────────────────────────────────────────────────

func parseLokiMatrixResponse(body []byte) ([]LokiMatrixSeries, error) {
	var raw struct {
		Data struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric map[string]string `json:"metric"`
				Values [][]interface{}   `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := make([]LokiMatrixSeries, 0, len(raw.Data.Result))
	for _, r := range raw.Data.Result {
		series := LokiMatrixSeries{Metric: r.Metric}
		for _, v := range r.Values {
			if len(v) < 2 {
				continue
			}
			series.Values = append(series.Values, [2]float64{toFloat(v[0]), toFloat(v[1])})
		}
		out = append(out, series)
	}
	return out, nil
}

// parseLokiStreamLines extracts raw log line strings from a Loki streams response.
func parseLokiStreamLines(body []byte) []string {
	entries := parseLokiStreamEntries(body)
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, entry.Line)
	}
	return lines
}

func parseLokiStreamEntries(body []byte) []LokiLogEntry {
	var raw struct {
		Data struct {
			Result []struct {
				Values [][]interface{} `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}
	var entries []LokiLogEntry
	for _, stream := range raw.Data.Result {
		for _, v := range stream.Values {
			if len(v) >= 2 {
				entry := LokiLogEntry{
					Line:     fmt.Sprint(v[1]),
					Metadata: map[string]string{},
				}
				if len(v) >= 3 {
					if rawMeta, ok := v[2].(map[string]interface{}); ok {
						for key, value := range rawMeta {
							entry.Metadata[key] = fmt.Sprint(value)
						}
					}
				}
				entries = append(entries, entry)
			}
		}
	}
	return entries
}

func parseVMMatrixResponse(body []byte) ([]VMSeries, error) {
	var raw struct {
		Data struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Values [][]interface{}   `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := make([]VMSeries, 0, len(raw.Data.Result))
	for _, r := range raw.Data.Result {
		series := VMSeries{Metric: r.Metric}
		for _, v := range r.Values {
			if len(v) < 2 {
				continue
			}
			series.Values = append(series.Values, [2]float64{toFloat(v[0]), toFloat(v[1])})
		}
		out = append(out, series)
	}
	return out, nil
}

// ─── Assembly helpers ─────────────────────────────────────────────────────────

func mergeSeriesByLabel(series []LokiMatrixSeries, labelKey string) []TimePoint {
	timeMap := map[float64]map[string]float64{}
	for _, s := range series {
		label := s.Metric[labelKey]
		if label == "" {
			label = "unknown"
		}
		for _, v := range s.Values {
			if _, ok := timeMap[v[0]]; !ok {
				timeMap[v[0]] = map[string]float64{}
			}
			timeMap[v[0]][label] = v[1]
		}
	}
	points := make([]TimePoint, 0, len(timeMap))
	for t, vals := range timeMap {
		pt := TimePoint{"time": t}
		for k, v := range vals {
			pt[k] = v
		}
		points = append(points, pt)
	}
	sortTimePoints(points)
	return points
}

func mergeNamedLokiSeries(seriesByName map[string][]LokiMatrixSeries) []TimePoint {
	timeMap := map[float64]TimePoint{}
	for name, series := range seriesByName {
		if len(series) == 0 {
			continue
		}
		for _, v := range series[0].Values {
			if _, ok := timeMap[v[0]]; !ok {
				timeMap[v[0]] = TimePoint{"time": v[0]}
			}
			timeMap[v[0]][name] = v[1]
		}
	}
	points := make([]TimePoint, 0, len(timeMap))
	for _, pt := range timeMap {
		points = append(points, pt)
	}
	sortTimePoints(points)
	return points
}

func mergeRTSeries(p50, p95, p99 []LokiMatrixSeries) []TimePoint {
	buildSeriesMap := func(series []LokiMatrixSeries) map[float64]float64 {
		values := map[float64]float64{}
		for _, item := range series {
			for _, sample := range item.Values {
				values[sample[0]] = sample[1] * 1000
			}
		}
		return values
	}

	p50Values := buildSeriesMap(p50)
	p95Values := buildSeriesMap(p95)
	p99Values := buildSeriesMap(p99)

	allTimes := map[float64]struct{}{}
	for ts := range p50Values {
		allTimes[ts] = struct{}{}
	}
	for ts := range p95Values {
		allTimes[ts] = struct{}{}
	}
	for ts := range p99Values {
		allTimes[ts] = struct{}{}
	}

	points := make([]TimePoint, 0, len(allTimes))
	for ts := range allTimes {
		point := TimePoint{"time": ts}
		if value, ok := p50Values[ts]; ok {
			point["p50"] = value
		} else {
			point["p50"] = nil
		}
		if value, ok := p95Values[ts]; ok {
			point["p95"] = value
		} else {
			point["p95"] = nil
		}
		if value, ok := p99Values[ts]; ok {
			point["p99"] = value
		} else {
			point["p99"] = nil
		}
		points = append(points, point)
	}

	sortTimePoints(points)
	return points
}

func mergeBotSeries(bot, total []LokiMatrixSeries) []TimePoint {
	timeMap := map[float64]TimePoint{}
	if len(bot) > 0 {
		for _, v := range bot[0].Values {
			if _, ok := timeMap[v[0]]; !ok {
				timeMap[v[0]] = TimePoint{"time": v[0]}
			}
			timeMap[v[0]]["botRate"] = v[1]
		}
	}
	if len(total) > 0 {
		for _, v := range total[0].Values {
			if _, ok := timeMap[v[0]]; !ok {
				timeMap[v[0]] = TimePoint{"time": v[0]}
			}
			timeMap[v[0]]["totalRate"] = v[1]
		}
	}
	points := make([]TimePoint, 0, len(timeMap))
	for _, pt := range timeMap {
		points = append(points, pt)
	}
	sortTimePoints(points)
	return points
}

// mergeVMConnSeries merges 4 separate VM series (active/waiting/reading/writing)
// into one timeseries of TimePoints, same shape the old frontend expected.
func mergeVMConnSeries(active, waiting, reading, writing []VMSeries) []TimePoint {
	timeMap := map[float64]TimePoint{}
	add := func(series []VMSeries, key string) {
		if len(series) == 0 {
			return
		}
		for _, v := range series[0].Values {
			if _, ok := timeMap[v[0]]; !ok {
				timeMap[v[0]] = TimePoint{"time": v[0]}
			}
			timeMap[v[0]][key] = v[1]
		}
	}
	add(active, "active")
	add(waiting, "waiting")
	add(reading, "reading")
	add(writing, "writing")

	points := make([]TimePoint, 0, len(timeMap))
	for _, pt := range timeMap {
		points = append(points, pt)
	}
	sortTimePoints(points)
	return points
}

// parseSlowRequests scans raw nginx log lines for ones containing rt=<seconds>
// where rt >= minRT, parses them, and returns:
//   - the top limit sorted by rt desc
//   - the total number of matching slow requests (before truncation)
//
// It now properly uses normalized_path from Loki structured metadata.
func parseSlowRequests(entries []LokiLogEntry, limit int, minRT float64) ([]SlowRequest, int) {
	var result []SlowRequest
	for _, entry := range entries {
		line := entry.Line
		// Quick pre-check
		rtIdx := findRT(line)
		if rtIdx < 0 {
			continue
		}
		rt := parseRT(line[rtIdx:])
		if rt < minRT {
			continue
		}

		parsed := parseNginxCombinedLog(line)
		if parsed == nil {
			continue
		}

		// Use normalized_path from OTel structured metadata (most important fix)
		if normPath, exists := entry.Metadata["normalized_path"]; exists && normPath != "" {
			parsed.NormalizedPath = normPath
		} else {
			// Fallback: use original path
			parsed.NormalizedPath = parsed.Path
		}

		parsed.RT = rt
		result = append(result, *parsed)
	}

	// Sort by rt desc
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].RT > result[i].RT {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	total := len(result)
	if len(result) > limit {
		result = result[:limit]
	}
	return result, total
}

// buildSlowRequestsNormalized uses Loki aggregation over the normalized_path
// metadata field directly. Each row is the single worst response time seen for
// that normalized path in the selected window.
// Returns the list (capped at limit) and the total before truncation.
func buildSlowRequestsNormalized(series []LokiMatrixSeries, limit int, minRT float64) ([]SlowRequest, int) {
	best := map[string]SlowRequest{}
	for _, item := range series {
		normalizedPath := item.Metric["normalized_path"]
		if normalizedPath == "" {
			continue
		}

		rt := lastValue(item.Values)
		if rt < minRT {
			continue
		}

		status, _ := strconv.Atoi(item.Metric["status"])
		req := SlowRequest{
			Path:           normalizedPath,
			NormalizedPath: normalizedPath,
			Method:         item.Metric["method"],
			Status:         status,
			RT:             rt,
		}

		if existing, ok := best[normalizedPath]; !ok || req.RT > existing.RT {
			best[normalizedPath] = req
		}
	}

	out := make([]SlowRequest, 0, len(best))
	for _, req := range best {
		out = append(out, req)
	}

	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].RT > out[i].RT {
				out[i], out[j] = out[j], out[i]
			}
		}
	}

	total := len(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, total
}

func findRT(line string) int {
	const marker = "rt="
	for i := 0; i < len(line)-len(marker); i++ {
		if line[i:i+len(marker)] == marker {
			return i + len(marker)
		}
	}
	return -1
}

func parseRT(s string) float64 {
	end := 0
	for end < len(s) && (s[end] == '.' || (s[end] >= '0' && s[end] <= '9')) {
		end++
	}
	if end == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(s[:end], 64)
	return v
}

// parseNginxCombinedLog parses the standard nginx combined log format.
// Returns nil if the line does not match.
func parseNginxCombinedLog(line string) *SlowRequest {
	// Format: IP - - [timestamp] "METHOD path protocol" status bytes "referer" "ua"
	fields := splitNginxLog(line)
	if len(fields) < 9 {
		return nil
	}
	ip := fields[0]
	timestamp := fields[3] // [timestamp]
	request := fields[4]   // "METHOD /path HTTP/1.1" (already unquoted by splitter)
	statusStr := fields[5]
	referer := ""
	userAgent := ""
	if len(fields) > 7 {
		referer = fields[7]
	}
	if len(fields) > 8 {
		userAgent = fields[8]
	}
	if len(request) == 0 {
		return nil
	}
	// Parse request
	parts := splitN(request, " ", 3)
	if len(parts) < 2 {
		return nil
	}
	method := parts[0]
	path := parts[1]
	status := 0
	for _, c := range statusStr {
		if c < '0' || c > '9' {
			break
		}
		status = status*10 + int(c-'0')
	}
	if status == 0 {
		return nil
	}
	return &SlowRequest{
		Path:      path,
		Method:    method,
		Status:    status,
		IP:        ip,
		Referer:   referer,
		UserAgent: userAgent,
		Timestamp: timestamp,
	}
}

func splitNginxLog(line string) []string {
	// Simple tokenizer: space-separated, handles "..." and [...] as single tokens
	var fields []string
	i := 0
	for i < len(line) {
		for i < len(line) && line[i] == ' ' {
			i++
		}
		if i >= len(line) {
			break
		}
		var token string
		switch line[i] {
		case '"':
			i++
			start := i
			for i < len(line) && line[i] != '"' {
				i++
			}
			token = line[start:i]
			if i < len(line) {
				i++ // skip closing "
			}
		case '[':
			i++
			start := i
			for i < len(line) && line[i] != ']' {
				i++
			}
			token = line[start:i]
			if i < len(line) {
				i++ // skip closing ]
			}
		default:
			start := i
			for i < len(line) && line[i] != ' ' {
				i++
			}
			token = line[start:i]
		}
		fields = append(fields, token)
	}
	return fields
}

func splitN(s, sep string, n int) []string {
	var parts []string
	for len(parts) < n-1 {
		idx := -1
		for i := 0; i <= len(s)-len(sep); i++ {
			if s[i:i+len(sep)] == sep {
				idx = i
				break
			}
		}
		if idx < 0 {
			break
		}
		parts = append(parts, s[:idx])
		s = s[idx+len(sep):]
	}
	parts = append(parts, s)
	return parts
}

func lastValue(values [][2]float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return values[len(values)-1][1]
}

func latestVMValue(series []VMSeries) *float64 {
	if len(series) == 0 || len(series[0].Values) == 0 {
		return nil
	}
	v := series[0].Values[len(series[0].Values)-1][1]
	return &v
}

func topN(counts map[string]float64, n int) []LabelCount {
	result := make([]LabelCount, 0, len(counts))
	for k, v := range counts {
		result = append(result, LabelCount{Label: k, Count: v})
	}
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Count > result[i].Count {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	if len(result) > n {
		result = result[:n]
	}
	return result
}

func sortTimePoints(pts []TimePoint) {
	for i := 0; i < len(pts); i++ {
		for j := i + 1; j < len(pts); j++ {
			if toFloat(pts[j]["time"]) < toFloat(pts[i]["time"]) {
				pts[i], pts[j] = pts[j], pts[i]
			}
		}
	}
}

func sortErrorPaths(paths []ErrorPathStat) {
	for i := 0; i < len(paths); i++ {
		for j := i + 1; j < len(paths); j++ {
			if paths[j].ErrPct > paths[i].ErrPct {
				paths[i], paths[j] = paths[j], paths[i]
			}
		}
	}
}

func sortSuspiciousIPs(ips []SuspiciousIP) {
	for i := 0; i < len(ips); i++ {
		for j := i + 1; j < len(ips); j++ {
			if ips[j].Errors > ips[i].Errors {
				ips[i], ips[j] = ips[j], ips[i]
			}
		}
	}
}

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case json.Number:
		f, _ := val.Float64()
		return f
	}
	return 0
}