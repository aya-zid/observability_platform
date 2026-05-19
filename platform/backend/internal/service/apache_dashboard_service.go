// internal/service/apache_dashboard_service.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
)

// ─── Response types ───────────────────────────────────────────────────────────

// ApacheDashboardSummary is the single JSON payload returned to the frontend.
// Every field that the old ApacheDashboardPage needed is present here.
type ApacheDashboardSummary struct {
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

	// ── apache_workers timeseries (from VictoriaMetrics) ──
	// Each TimePoint: {"time": unixSeconds, "busy": n, "idle": n}
	WorkersTimeseries []TimePoint `json:"workers_timeseries"`

	// ── apache_traffic_bytes_total timeseries ──
	// Each TimePoint: {"time": unixSeconds, "bytes": n}
	TrafficTimeseries []TimePoint `json:"traffic_timeseries"`

	// ── Load averages timeseries ──
	// Each TimePoint: {"time": unixSeconds, "load1": n, "load5": n, "load15": n}
	LoadTimeseries []TimePoint `json:"load_timeseries"`

	// ── VM cumulative timeseries ──
	VMRequestsTimeseries []TimePoint `json:"vm_requests_timeseries"` // {"time", "requests"}

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

	// ── VM scalar values (for stat cards) ──
	VMRequestsTotal  *float64 `json:"vm_requests_total,omitempty"`
	UptimeSeconds    *float64 `json:"uptime_seconds,omitempty"`
	CPULoadPercent   *float64 `json:"cpu_load_percent,omitempty"`
	BusyWorkers      *float64 `json:"busy_workers,omitempty"`
	IdleWorkers      *float64 `json:"idle_workers,omitempty"`
	TrafficBytesTotal *float64 `json:"traffic_bytes_total,omitempty"`
	Load1            *float64 `json:"load_1,omitempty"`
	Load5            *float64 `json:"load_5,omitempty"`
	Load15           *float64 `json:"load_15,omitempty"`

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

// ─── Explorer types ───────────────────────────────────────────────────────────

type ApacheExplorerResult struct {
	RTTimeseries []TimePoint  `json:"rt_timeseries"`
	RequestRate  []TimePoint  `json:"request_rate"`
	TopStatuses  []LabelCount `json:"top_statuses"`
	TopPaths     []LabelCount `json:"top_paths"`
	TotalCount   float64      `json:"total_count"`
}

type ApacheDomainItem struct {
	Domain      string `json:"domain"`
	Enabled     bool   `json:"enabled"`
	IsDefault   bool   `json:"is_default"`
	IsSharedLog bool   `json:"is_shared_log"`
}

type ApacheExplorerFilter struct {
	Domain      string
	IP          string
	PathPrefix  string
	StatusClass string
	MinRT       float64
}

// ─── Service ──────────────────────────────────────────────────────────────────

type ApacheDashboardService struct {
	cfg   *config.Config
	base  *NginxDashboardService
	redis *redis.Client
}

func NewApacheDashboardService(cfg *config.Config, redisClient *redis.Client) *ApacheDashboardService {
	return &ApacheDashboardService{
		cfg:   cfg,
		base:  NewNginxDashboardService(cfg, redisClient),
		redis: redisClient,
	}
}

func (s *ApacheDashboardService) GetDashboardSummary(
	ctx context.Context,
	tenantUUID string,
	tenantID uint,
	serviceName string,
	domain string,
	startNs, endNs int64,
	step string,
	minRT float64,
	extraPercentiles []float64,
) (*ApacheDashboardSummary, error) {
	if minRT <= 0 {
		minRT = 1000.0
	}
	extraPercentiles = normalizeRTPercentiles(extraPercentiles)
	startSec := startNs / int64(1e9)
	endSec := endNs / int64(1e9)
	cacheKey := fmt.Sprintf("apache:dash:%s:%s:%s:%d:%d:%s:%.3f:%s", tenantUUID, serviceName, domain, startSec, endSec, step, minRT, percentileCacheKey(extraPercentiles))

	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var result ApacheDashboardSummary
			if json.Unmarshal(cached, &result) == nil {
				return &result, nil
			}
		}
	}

	summary, err := s.fetchDashboard(ctx, tenantUUID, tenantID, serviceName, domain, startNs, endNs, step, minRT, extraPercentiles)
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

// ─── fetchDashboard: runs all queries in parallel ─────────────────────────────

func (s *ApacheDashboardService) fetchDashboard(
	ctx context.Context,
	tenantUUID string,
	tenantID uint,
	serviceName string,
	domain string,
	startNs, endNs int64,
	step string,
	minRT float64,
	extraPercentiles []float64,
) (*ApacheDashboardSummary, error) {
	base := lokiSelector(serviceName, domain, "access")
	rangeSeconds := (endNs - startNs) / int64(1e9)

	// ── Loki queries ──────────────────────────────────────────────────────────

	status2xxQ := fmt.Sprintf(`sum(rate(%s %s | status =~ "2.." [%s]))`, base, parseExpr, step)
	status3xxQ := fmt.Sprintf(`sum(rate(%s %s | status =~ "3.." [%s]))`, base, parseExpr, step)
	status4xxQ := fmt.Sprintf(`sum(rate(%s %s | status =~ "4.." [%s]))`, base, parseExpr, step)
	status5xxQ := fmt.Sprintf(`sum(rate(%s %s | status =~ "5.." [%s]))`, base, parseExpr, step)

	rtP50Q := fmt.Sprintf(`quantile_over_time(0.50, %s %s | unwrap rt [%s]) by ()`, base, rtParseExpr, step)
	rtP95Q := fmt.Sprintf(`quantile_over_time(0.95, %s %s | unwrap rt [%s]) by ()`, base, rtParseExpr, step)
	rtP99Q := fmt.Sprintf(`quantile_over_time(0.99, %s %s | unwrap rt [%s]) by ()`, base, rtParseExpr, step)
	extraRTQueries := map[string]string{}
	for _, p := range extraPercentiles {
		extraRTQueries[percentileLabel(p)] = fmt.Sprintf(`quantile_over_time(%.4f, %s %s | unwrap rt [%s]) by ()`, p/100, base, rtParseExpr, step)
	}

	reqRateQ := fmt.Sprintf(`sum(rate(%s [%s]))`, base, step)

	bandwidthQ := fmt.Sprintf(
		`sum(rate(%s %s | unwrap bytes [%s]))`,
		base, parseExpr, step,
	)

	botQ := fmt.Sprintf(
		`sum(rate(%s |~ `+"`"+`(?i)bot|crawl|spider|curl|wget|python|java|go-http`+"`"+` [%s]))`,
		base, step,
	)
	totalRateQ := fmt.Sprintf(`sum(rate(%s [%s]))`, base, step)

	topAggQ := fmt.Sprintf(
		`topk(50, sum by (path, status, method, ip) (count_over_time(%s %s [%ds])))`,
		base, parseExpr, rangeSeconds,
	)
	topNormAggQ := fmt.Sprintf(
		`topk(50, sum by (normalized_path, status, method, ip) (count_over_time(%s | normalized_path!="" %s [%ds])))`,
		base, parseExpr, rangeSeconds,
	)
	slowNormQ := fmt.Sprintf(
		`max by (normalized_path, method, status) (max_over_time(%s | normalized_path!="" %s %s | unwrap rt [%ds]))`,
		base, parseExpr, rtParseExpr, rangeSeconds,
	)
	slowTotalQ := fmt.Sprintf(
		`sum(count_over_time(%s %s | rt >= %.6f [%ds]))`,
		base, rtParseExpr, minRT, rangeSeconds,
	)
	slowNormTotalQ := fmt.Sprintf(
		`count(max by (normalized_path) (max_over_time(%s | normalized_path!="" %s | rt >= %.6f | unwrap rt [%ds])))`,
		base, rtParseExpr, minRT, rangeSeconds,
	)

	slowLogsQ := fmt.Sprintf(`%s %s | rt >= %.6f`, base, rtParseExpr, minRT)

	// ── VM queries ────────────────────────────────────────────────────────────
	workersBusyQ := `apache_workers{state="busy"}`
	workersIdleQ := `apache_workers{state="idle"}`
	requestsTotalQ := `apache_requests_total`
	trafficTotalQ := `apache_traffic_bytes_total`
	cpuLoadQ := `apache_cpu_load_percent`
	load1Q := `apache_load_1_percent`
	load5Q := `apache_load_5_percent`
	load15Q := `apache_load_15_percent`
	uptimeQ := `apache_uptime_seconds_total`

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
		wg               sync.WaitGroup
		status2xx        lR
		status3xx        lR
		status4xx        lR
		status5xx        lR
		rtP50            lR
		rtP95            lR
		rtP99            lR
		reqRate          lR
		bandwidth        lR
		botRate          lR
		totalRate        lR
		topAgg           lR
		topNormAgg       lR
		slowNormAgg      lR
		slowTotalAgg     lR
		slowNormTotalAgg lR
		workersBusy      vR
		workersIdle      vR
		reqTotal         vR
		trafficTotal     vR
		cpuLoad          vR
		load1            vR
		load5            vR
		load15           vR
		uptime           vR
		slowLogs         logR
		extraRT          map[string]lR
		extraRTMu        sync.Mutex
	)

	lq := func(q string) lR {
		d, e := s.base.lokiQueryRange(ctx, tenantUUID, q, startNs, endNs, step, 5000)
		return lR{d, e}
	}
	vq := func(q string) vR {
		d, e := s.base.vmQueryRange(ctx, tenantID, q, startNs/int64(1e9), endNs/int64(1e9), step)
		return vR{d, e}
	}

	extraRT = make(map[string]lR, len(extraRTQueries))

	wg.Add(26 + len(extraRTQueries))
	go func() { defer wg.Done(); status2xx = lq(status2xxQ) }()
	go func() { defer wg.Done(); status3xx = lq(status3xxQ) }()
	go func() { defer wg.Done(); status4xx = lq(status4xxQ) }()
	go func() { defer wg.Done(); status5xx = lq(status5xxQ) }()
	go func() { defer wg.Done(); rtP50 = lq(rtP50Q) }()
	go func() { defer wg.Done(); rtP95 = lq(rtP95Q) }()
	go func() { defer wg.Done(); rtP99 = lq(rtP99Q) }()
	for label, query := range extraRTQueries {
		label := label
		query := query
		go func() {
			defer wg.Done()
			result := lq(query)
			extraRTMu.Lock()
			extraRT[label] = result
			extraRTMu.Unlock()
		}()
	}
	go func() { defer wg.Done(); reqRate = lq(reqRateQ) }()
	go func() { defer wg.Done(); bandwidth = lq(bandwidthQ) }()
	go func() { defer wg.Done(); botRate = lq(botQ) }()
	go func() { defer wg.Done(); totalRate = lq(totalRateQ) }()
	go func() {
		defer wg.Done()
		d, e := s.base.lokiQueryInstant(ctx, tenantUUID, topAggQ, endNs, 5000)
		topAgg = lR{d, e}
	}()
	go func() {
		defer wg.Done()
		d, e := s.base.lokiQueryInstant(ctx, tenantUUID, topNormAggQ, endNs, 5000)
		topNormAgg = lR{d, e}
	}()
	go func() {
		defer wg.Done()
		d, e := s.base.lokiQueryInstant(ctx, tenantUUID, slowNormQ, endNs, 5000)
		slowNormAgg = lR{d, e}
	}()
	go func() {
		defer wg.Done()
		d, e := s.base.lokiQueryInstant(ctx, tenantUUID, slowTotalQ, endNs, 1)
		slowTotalAgg = lR{d, e}
	}()
	go func() {
		defer wg.Done()
		d, e := s.base.lokiQueryInstant(ctx, tenantUUID, slowNormTotalQ, endNs, 1)
		slowNormTotalAgg = lR{d, e}
	}()
	go func() { defer wg.Done(); workersBusy = vq(workersBusyQ) }()
	go func() { defer wg.Done(); workersIdle = vq(workersIdleQ) }()
	go func() { defer wg.Done(); reqTotal = vq(requestsTotalQ) }()
	go func() { defer wg.Done(); trafficTotal = vq(trafficTotalQ) }()
	go func() { defer wg.Done(); cpuLoad = vq(cpuLoadQ) }()
	go func() { defer wg.Done(); load1 = vq(load1Q) }()
	go func() { defer wg.Done(); load5 = vq(load5Q) }()
	go func() { defer wg.Done(); load15 = vq(load15Q) }()
	go func() { defer wg.Done(); uptime = vq(uptimeQ) }()
	go func() {
		defer wg.Done()
		entries, err := s.base.lokiQueryLogEntries(ctx, tenantUUID, slowLogsQ, startNs, endNs, 100)
		slowLogs = logR{entries, err}
	}()
	wg.Wait()

	summary := &ApacheDashboardSummary{}

	// ── Status timeseries ─────────────────────────────────────────────────────
	summary.StatusTimeseries = mergeNamedLokiSeries(map[string][]LokiMatrixSeries{
		"2xx": status2xx.data,
		"3xx": status3xx.data,
		"4xx": status4xx.data,
		"5xx": status5xx.data,
	})

	// ── RT timeseries (merge p50/p95/p99, convert seconds → ms) ──────────────
	rtSeries := map[string][]LokiMatrixSeries{
		"p50": rtP50.data,
		"p95": rtP95.data,
		"p99": rtP99.data,
	}
	for label, result := range extraRT {
		rtSeries[label] = result.data
	}
	summary.RTTimeseries = mergeApacheRTSeries(rtSeries)

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

	// ── Workers timeseries (busy/idle merged by time) ─────────────────────────
	summary.WorkersTimeseries = mergeVMWorkersSeries(workersBusy.data, workersIdle.data)

	// ── Traffic timeseries ────────────────────────────────────────────────────
	if len(trafficTotal.data) > 0 {
		for _, pt := range trafficTotal.data[0].Values {
			summary.TrafficTimeseries = append(summary.TrafficTimeseries,
				TimePoint{"time": pt[0], "bytes": pt[1]})
		}
	}

	// ── Load average timeseries ───────────────────────────────────────────────
	summary.LoadTimeseries = mergeVMLoadSeries(load1.data, load5.data, load15.data)

	// ── VM cumulative timeseries ──────────────────────────────────────────────
	if len(reqTotal.data) > 0 {
		for _, pt := range reqTotal.data[0].Values {
			summary.VMRequestsTimeseries = append(summary.VMRequestsTimeseries,
				TimePoint{"time": pt[0], "requests": pt[1]})
		}
	}

	// ── Latest scalar values for stat cards ───────────────────────────────────
	summary.VMRequestsTotal = latestVMValue(reqTotal.data)
	summary.UptimeSeconds = latestVMValue(uptime.data)
	summary.CPULoadPercent = latestVMValue(cpuLoad.data)
	summary.BusyWorkers = latestVMValue(workersBusy.data)
	summary.IdleWorkers = latestVMValue(workersIdle.data)
	summary.TrafficBytesTotal = latestVMValue(trafficTotal.data)
	summary.Load1 = latestVMValue(load1.data)
	summary.Load5 = latestVMValue(load5.data)
	summary.Load15 = latestVMValue(load15.data)

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
	slowReqs, parsedSlowTotal := parseSlowRequests(slowLogs.entries, 100, minRT)
	summary.SlowRequests = slowReqs
	summary.SlowRequestsTotal = int(lokiSingleValue(slowTotalAgg.data))
	if summary.SlowRequestsTotal == 0 {
		summary.SlowRequestsTotal = parsedSlowTotal
	}
	summary.SlowRequestsMinRT = minRT

	slowNorm, slowNormTotal := buildSlowRequestsNormalized(slowNormAgg.data, 100, minRT)
	summary.SlowRequestsNormalized = slowNorm
	summary.SlowRequestsNormalizedTotal = int(lokiSingleValue(slowNormTotalAgg.data))
	if summary.SlowRequestsNormalizedTotal == 0 {
		summary.SlowRequestsNormalizedTotal = slowNormTotal
	}

	return summary, nil
}

// ─── SlowRequestsList (not cached — used by "View More" page) ────────────────

// GetSlowRequestsList returns a paginated list of slow requests for the "View More" page.
func (s *ApacheDashboardService) GetSlowRequestsList(
	ctx context.Context,
	tenantUUID string,
	serviceName string,
	domain string,
	startNs, endNs int64,
	minRT float64,
	page, perPage int,
) (*SlowRequestsListResult, error) {
	if minRT <= 0 {
		minRT = 1000.0
	}
	if perPage <= 0 || perPage > 500 {
		perPage = 100
	}
	if page <= 0 {
		page = 1
	}
	base := lokiSelector(serviceName, domain, "access")

	rangeSeconds := (endNs - startNs) / int64(1e9)
	if rangeSeconds < 1 {
		rangeSeconds = 1
	}
	filteredLogsQ := fmt.Sprintf(`%s %s | rt >= %.6f`, base, rtParseExpr, minRT)
	totalQ := fmt.Sprintf(`sum(count_over_time(%s %s | rt >= %.6f [%ds]))`, base, rtParseExpr, minRT, rangeSeconds)
	totalSeries, err := s.base.lokiQueryInstant(ctx, tenantUUID, totalQ, endNs, 1)
	if err != nil {
		return nil, fmt.Errorf("loki count query failed: %w", err)
	}
	total := int(lokiSingleValue(totalSeries))

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
		entries, err := s.base.lokiQueryLogEntriesUntil(ctx, tenantUUID, filteredLogsQ, startNs, endNs, end)
		if err != nil {
			return nil, fmt.Errorf("loki log query failed: %w", err)
		}
		all, parsedTotal := parseSlowRequests(entries, len(entries), minRT)
		if total == 0 {
			total = parsedTotal
		}
		if start < len(all) {
			if end > len(all) {
				end = len(all)
			}
			pageItems = all[start:end]
		} else {
			pageItems = []SlowRequest{}
		}
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

func (s *ApacheDashboardService) GetExplorerResult(
	ctx context.Context,
	tenantUUID string,
	serviceName string,
	startNs, endNs int64,
	step string,
	filter ApacheExplorerFilter,
) (*ApacheExplorerResult, error) {
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
		d, e := s.base.lokiQueryRange(ctx, tenantUUID,
			fmt.Sprintf(`quantile_over_time(0.50, %s | unwrap rt [%s])`, rtBase, step),
			startNs, endNs, step, 5000)
		rtP50 = lR{d, e}
	}()
	go func() {
		defer wg.Done()
		d, e := s.base.lokiQueryRange(ctx, tenantUUID,
			fmt.Sprintf(`quantile_over_time(0.95, %s | unwrap rt [%s])`, rtBase, step),
			startNs, endNs, step, 5000)
		rtP95 = lR{d, e}
	}()
	go func() {
		defer wg.Done()
		d, e := s.base.lokiQueryRange(ctx, tenantUUID,
			fmt.Sprintf(`sum(rate(%s [%s]))`, filterChain, step),
			startNs, endNs, step, 5000)
		rate = lR{d, e}
	}()
	go func() {
		defer wg.Done()
		topAggStep := fmt.Sprintf("%ds", rangeSeconds)
		d, e := s.base.lokiQueryRange(ctx, tenantUUID,
			fmt.Sprintf(`topk(20, sum by (path, status) (count_over_time(%s [%ds])))`, filterChain, rangeSeconds),
			startNs, endNs, topAggStep, 5000)
		topAgg = lR{d, e}
	}()
	wg.Wait()

	result := &ApacheExplorerResult{}
	result.RTTimeseries = mergeApacheRTSeries(map[string][]LokiMatrixSeries{
		"p50": rtP50.data,
		"p95": rtP95.data,
	})
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

// GetErrorLogs returns raw apache error log lines for the given service/domain/window.
func (s *ApacheDashboardService) GetErrorLogs(
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
	return s.base.lokiQueryLogLines(ctx, tenantUUID, sel, startNs, endNs, limit)
}

// parseCombinedLog parses the standard apache combined log format.
// Returns nil if the line does not match.
func parseCombinedLog(line string) *SlowRequest {
	fields := splitNginxLog(line)
	if len(fields) < 9 {
		return nil
	}
	ip := fields[0]
	timestamp := fields[3]
	request := fields[4]
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

// ─── IP Investigation Types ────────────────────────────────────────────────────

type ApacheIPPathStat struct {
	Path            string             `json:"path"`
	Method          string             `json:"method"`
	Count           float64            `json:"count"`
	Errors          float64            `json:"errors"`
	AvgRT           float64            `json:"avg_rt"`
	Status          string             `json:"status"`
	StatusBreakdown map[string]float64 `json:"status_breakdown"`
}

type ApacheIPInvestigationResult struct {
	TotalRequests  float64 `json:"total_requests"`
	Total4xx       float64 `json:"total_4xx"`
	Total5xx       float64 `json:"total_5xx"`
	ErrorRate      float64 `json:"error_rate"`
	FirstSeenToday string  `json:"first_seen_today"`
	LastSeenToday  string  `json:"last_seen_today"`

	ReqRateTimeseries []TimePoint `json:"req_rate_timeseries"`
	StatusTimeseries  []TimePoint `json:"status_timeseries"`
	RTTimeseries      []TimePoint `json:"rt_timeseries"`

	Paths    []ApacheIPPathStat `json:"paths"`
	Warnings []string           `json:"warnings,omitempty"`
}

// ─── GetIPInvestigation ────────────────────────────────────────────────────────

func (s *ApacheDashboardService) GetIPInvestigation(
	ctx context.Context,
	tenantUUID string,
	serviceName string,
	ip string,
	domain string,
	startNs, endNs int64,
	step string,
	seenStartNs, seenEndNs int64,
) (*ApacheIPInvestigationResult, error) {

	base := lokiSelector(serviceName, domain, "access")
	ipFilter := fmt.Sprintf(`%s %s | ip = "%s"`, base, parseExpr, lokiLabelValue(ip))
	ipFilterRT := fmt.Sprintf(`%s %s %s | ip = "%s"`, base, parseExpr, rtParseExpr, lokiLabelValue(ip))

	const countInterval = "1m"

	type lR struct {
		data []LokiMatrixSeries
		err  error
	}

	lq := func(q string) lR {
		d, e := s.base.lokiQueryRange(ctx, tenantUUID, q, startNs, endNs, countInterval, 5000)
		return lR{d, e}
	}

	var wg sync.WaitGroup
	var (
		reqRate, st2xx, st3xx, st4xx, st5xx, avgRT lR
		pathCounts, pathErrors, pathAvgRT          lR
		pathMethodCounts, pathStatusCounts         lR
	)

	wg.Add(11)

	go func() {
		defer wg.Done()
		reqRate = lq(fmt.Sprintf(`sum(count_over_time(%s [%s]))`, ipFilter, countInterval))
	}()
	go func() {
		defer wg.Done()
		st2xx = lq(fmt.Sprintf(`sum(count_over_time(%s | status =~ "2.." [%s]))`, ipFilter, countInterval))
	}()
	go func() {
		defer wg.Done()
		st3xx = lq(fmt.Sprintf(`sum(count_over_time(%s | status =~ "3.." [%s]))`, ipFilter, countInterval))
	}()
	go func() {
		defer wg.Done()
		st4xx = lq(fmt.Sprintf(`sum(count_over_time(%s | status =~ "4.." [%s]))`, ipFilter, countInterval))
	}()
	go func() {
		defer wg.Done()
		st5xx = lq(fmt.Sprintf(`sum(count_over_time(%s | status =~ "5.." [%s]))`, ipFilter, countInterval))
	}()
	go func() {
		defer wg.Done()
		avgRT = lq(fmt.Sprintf(`avg(avg_over_time(%s | unwrap rt [%s]))`, ipFilterRT, countInterval))
	}()
	go func() {
		defer wg.Done()
		pathCounts = lq(fmt.Sprintf(`sum by (path) (count_over_time(%s [%s]))`, ipFilter, countInterval))
	}()
	go func() {
		defer wg.Done()
		pathErrors = lq(fmt.Sprintf(`sum by (path) (count_over_time(%s | status =~ "4..|5.." [%s]))`, ipFilter, countInterval))
	}()
	go func() {
		defer wg.Done()
		pathAvgRT = lq(fmt.Sprintf(`avg by (path) (avg_over_time(%s | unwrap rt [%s]))`, ipFilterRT, countInterval))
	}()
	go func() {
		defer wg.Done()
		pathMethodCounts = lq(fmt.Sprintf(`sum by (path, method) (count_over_time(%s [%s]))`, ipFilter, countInterval))
	}()
	go func() {
		defer wg.Done()
		pathStatusCounts = lq(fmt.Sprintf(`sum by (path, status) (count_over_time(%s [%s]))`, ipFilter, countInterval))
	}()

	wg.Wait()

	result := &ApacheIPInvestigationResult{}

	result.TotalRequests = sumLokiSeries(reqRate.data)
	result.Total4xx = sumLokiSeries(st4xx.data)
	result.Total5xx = sumLokiSeries(st5xx.data)
	if result.TotalRequests > 0 {
		result.ErrorRate = ((result.Total4xx + result.Total5xx) / result.TotalRequests) * 100
	}

	if len(reqRate.data) > 0 {
		for _, pt := range reqRate.data[0].Values {
			result.ReqRateTimeseries = append(result.ReqRateTimeseries,
				TimePoint{"time": pt[0], "req": pt[1]})
		}
	}

	result.StatusTimeseries = mergeNamedLokiSeries(map[string][]LokiMatrixSeries{
		"2xx": st2xx.data, "3xx": st3xx.data, "4xx": st4xx.data, "5xx": st5xx.data,
	})

	if len(avgRT.data) > 0 {
		for _, pt := range avgRT.data[0].Values {
			result.RTTimeseries = append(result.RTTimeseries,
				TimePoint{"time": pt[0], "avg_rt": pt[1]})
		}
	}

	result.Paths = buildApachePathStats(
		pathCounts.data,
		pathErrors.data,
		pathAvgRT.data,
		pathMethodCounts.data,
		pathStatusCounts.data,
	)

	if seenStartNs <= 0 || seenEndNs <= seenStartNs {
		seenStartNs = ipInvTodayMidnightNs()
		seenEndNs = time.Now().UnixNano()
	}

	var firstTs, lastTs int64
	var wg2 sync.WaitGroup
	wg2.Add(2)
	go func() {
		defer wg2.Done()
		firstTs, _ = s.base.lokiQueryFirstEntryTs(ctx, tenantUUID, ipFilter, seenStartNs, seenEndNs, "forward")
	}()
	go func() {
		defer wg2.Done()
		lastTs, _ = s.base.lokiQueryFirstEntryTs(ctx, tenantUUID, ipFilter, seenStartNs, seenEndNs, "backward")
	}()
	wg2.Wait()

	if firstTs > 0 {
		result.FirstSeenToday = tsNsToHHMM(firstTs)
	}
	if lastTs > 0 {
		result.LastSeenToday = tsNsToHHMM(lastTs)
	}

	return result, nil
}

func buildApachePathStats(
	countSeries []LokiMatrixSeries,
	errorSeries []LokiMatrixSeries,
	rtSeries []LokiMatrixSeries,
	methodSeries []LokiMatrixSeries,
	statusSeries []LokiMatrixSeries,
) []ApacheIPPathStat {

	type acc struct {
		count    float64
		errors   float64
		rtSum    float64
		rtCount  float64
		methods  map[string]float64
		statuses map[string]float64
	}

	pathMap := make(map[string]*acc)
	get := func(path string) *acc {
		if a, ok := pathMap[path]; ok {
			return a
		}
		a := &acc{
			methods:  make(map[string]float64),
			statuses: make(map[string]float64),
		}
		pathMap[path] = a
		return a
	}

	for _, s := range countSeries {
		path := s.Metric["path"]
		if path == "" {
			continue
		}
		a := get(path)
		for _, pt := range s.Values {
			a.count += pt[1]
		}
	}

	for _, s := range errorSeries {
		path := s.Metric["path"]
		if path == "" {
			continue
		}
		a := get(path)
		for _, pt := range s.Values {
			a.errors += pt[1]
		}
	}

	for _, s := range rtSeries {
		path := s.Metric["path"]
		if path == "" {
			continue
		}
		a := get(path)
		for _, pt := range s.Values {
			a.rtSum += pt[1]
			a.rtCount++
		}
	}

	for _, s := range methodSeries {
		path := s.Metric["path"]
		method := s.Metric["method"]
		if path == "" || method == "" {
			continue
		}
		a := get(path)
		for _, pt := range s.Values {
			a.methods[method] += pt[1]
		}
	}

	for _, s := range statusSeries {
		path := s.Metric["path"]
		status := s.Metric["status"]
		if path == "" || status == "" {
			continue
		}
		a := get(path)
		for _, pt := range s.Values {
			a.statuses[status] += pt[1]
		}
	}

	type entry struct {
		path string
		acc  *acc
	}
	list := make([]entry, 0, len(pathMap))
	for p, a := range pathMap {
		list = append(list, entry{p, a})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].acc.count > list[j].acc.count
	})
	if len(list) > 20 {
		list = list[:20]
	}

	stats := make([]ApacheIPPathStat, 0, len(list))
	for _, e := range list {
		avgRT := 0.0
		if e.acc.rtCount > 0 {
			avgRT = e.acc.rtSum / e.acc.rtCount
		}
		stats = append(stats, ApacheIPPathStat{
			Path:            e.path,
			Count:           e.acc.count,
			Errors:          e.acc.errors,
			Method:          dominantKey(e.acc.methods),
			Status:          dominantKey(e.acc.statuses),
			AvgRT:           avgRT,
			StatusBreakdown: e.acc.statuses,
		})
	}
	return stats
}

// mergeVMWorkersSeries merges 2 VM series (busy/idle) into one timeseries.
func mergeVMWorkersSeries(busy, idle []VMSeries) []TimePoint {
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
	add(busy, "busy")
	add(idle, "idle")

	points := make([]TimePoint, 0, len(timeMap))
	for _, pt := range timeMap {
		points = append(points, pt)
	}
	sortTimePoints(points)
	return points
}

// mergeVMLoadSeries merges 3 VM series (load1/load5/load15) into one timeseries.
func mergeVMLoadSeries(load1, load5, load15 []VMSeries) []TimePoint {
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
	add(load1, "load1")
	add(load5, "load5")
	add(load15, "load15")

	points := make([]TimePoint, 0, len(timeMap))
	for _, pt := range timeMap {
		points = append(points, pt)
	}
	sortTimePoints(points)
	return points
}


