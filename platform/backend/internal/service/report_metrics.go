// internal/service/report_metrics.go
package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ─── Time regions (UTC) ───────────────────────────────────────────────────────

// reportRegions is the fixed order of the four UTC regions.
var reportRegions = []string{"night", "morning", "afternoon", "evening"}

// reportRegionOffsets maps region name -> start hour offset (UTC).
var reportRegionOffsets = map[string]int{
	"night":     0,
	"morning":   6,
	"afternoon": 12,
	"evening":   18,
}

// regionWindows returns the four fixed UTC windows [startNs, endNs) for the
// given day. `day` must already be UTC midnight (the caller resolves date_a/b
// with time.Date(y, m, d, 0, 0, 0, 0, time.UTC)). No tenant timezone is used.
func regionWindows(day time.Time) map[string][2]int64 {
	base := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	windows := make(map[string][2]int64, len(reportRegions))
	for _, name := range reportRegions {
		start := base.Add(time.Duration(reportRegionOffsets[name]) * time.Hour)
		end := base.Add(time.Duration(reportRegionOffsets[name]+6) * time.Hour)
		windows[name] = [2]int64{start.UnixNano(), end.UnixNano()}
	}
	return windows
}

// ─── Payload structs (compact scalars fed to the AI) ─────────────────────────

// ReportServiceMetrics holds the compact per-region scalars for one service kind.
// Every field is a pointer so an absent metric serializes as null ("no data").
type ReportServiceMetrics struct {
	// web (nginx / apache) — from Loki access logs
	TotalRequests  *float64 `json:"total_requests"`
	AvgReqRate     *float64 `json:"avg_req_rate"`
	Total5xx       *float64 `json:"total_5xx"`
	Total4xx       *float64 `json:"total_4xx"`
	P95RTMs        *float64 `json:"p95_rt_ms"`
	BandwidthBytes *float64 `json:"bandwidth_bytes"`
	BotRate        *float64 `json:"bot_rate"`
	TopErrorPath   string   `json:"top_error_path,omitempty"`

	// postgres
	CacheHitRatioPct        *float64 `json:"cache_hit_ratio_pct"`
	SlowQueries             *float64 `json:"slow_queries"`
	Errors                  *float64 `json:"errors"`
	AuthFailures            *float64 `json:"auth_failures"`
	LockWaits               *float64 `json:"lock_waits"`
	DeadlocksPerMin         *float64 `json:"deadlocks_per_min"`
	ConnectionSaturationPct *float64 `json:"connection_saturation_pct"`
	DbSizeBytes             *float64 `json:"db_size_bytes"`

	// mariadb
	QPS                   *float64 `json:"qps"`
	SlowQueriesPerMin     *float64 `json:"slow_queries_per_min"`
	BufferPoolHitRatioPct *float64 `json:"buffer_pool_hit_ratio_pct"`
	ThreadsConnected      *float64 `json:"threads_connected"`
	AbortedConnectsPerMin *float64 `json:"aborted_connects_per_min"`
	TableScanRatioPct     *float64 `json:"table_scan_ratio_pct"`

	// redis
	MemoryUsagePct      *float64 `json:"memory_usage_pct"`
	CommandsPerSec      *float64 `json:"commands_per_sec"`
	ConnectedClients    *float64 `json:"connected_clients"`
	ClientSaturationPct *float64 `json:"client_saturation_pct"`
	EvictRate           *float64 `json:"evict_rate"`
	ErrorRepliesPerSec  *float64 `json:"error_replies_per_sec"`
	SlowLogCount        *float64 `json:"slow_log_count"`

	// php-fpm
	WorkerSaturationPct *float64 `json:"worker_saturation_pct"`
	AcceptedConnPerSec  *float64 `json:"accepted_conn_per_sec"`
	SlowRequests        *float64 `json:"slow_requests"`
	Status5xx           *float64 `json:"status_5xx"`
	LatencyP95Ms        *float64 `json:"latency_p95_ms"`
	OpcacheHitRate      *float64 `json:"opcache_hit_rate"`
	MaxChildrenReached  *float64 `json:"max_children_reached"`
}

// ReportLogSignals is the per-service-kind error/warning log line count for a
// window (reused from timelineProfiles classification).
type ReportLogSignals struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
}

// ─── Gatherer ─────────────────────────────────────────────────────────────────

// reportMetricsGatherer issues tenant-wide scalar queries to Loki and
// VictoriaMetrics, reusing the dashboard service helpers. No host_id filter is
// applied, so every query aggregates across all hosts in the tenant.
type reportMetricsGatherer struct {
	base       *NginxDashboardService
	tenantUUID string
	tenantID   uint
}

const (
	reportRateWindow = "5m"
	reportVMStep     = "15m"
)

// serviceNameRegex returns a Loki service_name label matcher that matches every
// discovered instance of a service kind tenant-wide.
func serviceNameRegex(kind string) string {
	switch kind {
	case "nginx":
		return `nginx.*`
	case "apache":
		return `apache.*|apache2.*|httpd.*`
	case "phpfpm":
		return `phpfpm.*`
	case "mariadb":
		return `mysql.*|mariadb.*|mysqld.*`
	case "postgresql":
		return `postgres.*|postgresql.*`
	case "redis":
		return `redis.*`
	case "host":
		return `journald`
	default:
		return kind + `.*`
	}
}

// reportWebSelector builds the tenant-wide Loki selector for web access logs.
func reportWebSelector(kind, logKind string) string {
	sel := fmt.Sprintf(`{service_name=~"%s"}`, serviceNameRegex(kind))
	if logKind != "" {
		sel += fmt.Sprintf(` | log_kind="%s"`, lokiLabelValue(logKind))
	}
	return sel
}

// ─── VM scalar helpers ────────────────────────────────────────────────────────

type vmAggMode int

const (
	vmAggAvg vmAggMode = iota
	vmAggMax
	vmAggLatest
)

// vmRangeScalar runs a range query over the window and aggregates the samples.
func (g *reportMetricsGatherer) vmRangeScalar(ctx context.Context, query string, startSec, endSec int64, mode vmAggMode) *float64 {
	series, err := g.base.vmQueryRange(ctx, g.tenantID, query, startSec, endSec, reportVMStep, "", "report")
	if err != nil {
		return nil
	}
	if len(series) == 0 {
		return nil
	}
	if mode == vmAggLatest {
		if v := latestVMValue(series); v != nil {
			return v
		}
		return nil
	}
	agg := func(init float64, better func(a, b float64) bool) *float64 {
		val := init
		found := false
		for _, s := range series {
			for _, pt := range s.Values {
				if !found || better(pt[1], val) {
					val = pt[1]
					found = true
				}
			}
		}
		if !found {
			return nil
		}
		return &val
	}
	if mode == vmAggMax {
		return agg(0, func(a, b float64) bool { return a > b })
	}
	// vmAggAvg
	sum := 0.0
	count := 0
	for _, s := range series {
		for _, pt := range s.Values {
			sum += pt[1]
			count++
		}
	}
	if count == 0 {
		return nil
	}
	v := sum / float64(count)
	return &v
}

// vmInstantScalar runs an instant query at the window end.
func (g *reportMetricsGatherer) vmInstantScalar(ctx context.Context, query string, endSec int64) *float64 {
	series, err := g.base.vmQueryInstant(ctx, g.tenantID, query, endSec, "", "report")
	if err != nil {
		return nil
	}
	return vmInstValue(series)
}

// ratio computes a/b*100 guarding division by zero; nil when a or b missing.
func ratioPct(a, b *float64) *float64 {
	if a == nil || b == nil || *b == 0 {
		return nil
	}
	v := 100 * (*a) / (*b)
	return &v
}

// ─── Loki scalar helpers ──────────────────────────────────────────────────────

// lokiCountInstant returns sum(count_over_time(expr [window])) evaluated at the
// end of the window — a total count over the whole region.
func (g *reportMetricsGatherer) lokiCountInstant(ctx context.Context, expr string, window string, endNs int64) *float64 {
	q := fmt.Sprintf(`sum(count_over_time(%s [%s]))`, expr, window)
	series, err := g.base.lokiQueryInstant(ctx, g.tenantUUID, q, endNs, 1)
	if err != nil {
		return nil
	}
	v := lokiSingleValue(series)
	return &v
}

// lokiRateInstant returns sum(rate(expr [window])) at the window end.
func (g *reportMetricsGatherer) lokiRateInstant(ctx context.Context, expr string, window string, endNs int64) *float64 {
	q := fmt.Sprintf(`sum(rate(%s [%s]))`, expr, window)
	series, err := g.base.lokiQueryInstant(ctx, g.tenantUUID, q, endNs, 1)
	if err != nil {
		return nil
	}
	v := lokiSingleValue(series)
	return &v
}

// lokiQuantileInstant returns quantile_over_time(...) at the window end.
func (g *reportMetricsGatherer) lokiQuantileInstant(ctx context.Context, expr, unwrapLabel, window string, endNs int64) *float64 {
	q := fmt.Sprintf(`quantile_over_time(0.95, %s | unwrap %s [%s]) by ()`, expr, unwrapLabel, window)
	series, err := g.base.lokiQueryInstant(ctx, g.tenantUUID, q, endNs, 1)
	if err != nil {
		return nil
	}
	v := lokiSingleValue(series)
	return &v
}

// ─── Web extractor (nginx / apache) ───────────────────────────────────────────

func (g *reportMetricsGatherer) extractWeb(ctx context.Context, kind string, startNs, endNs int64) *ReportServiceMetrics {
	window := fmt.Sprintf("%ds", (endNs-startNs)/int64(1e9))
	base := reportWebSelector(kind, "access")
	baseParse := base + " " + parseExpr

	type res struct {
		name string
		val  *float64
		path string
	}
	results := make([]res, 0, 8)
	var mu sync.Mutex
	var wg sync.WaitGroup
	collect := func(name string, q string, endNs int64, wantPath bool) {
		series, err := g.base.lokiQueryInstant(ctx, g.tenantUUID, q, endNs, 1)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			results = append(results, res{name: name})
			return
		}
		v := lokiSingleValue(series)
		path := ""
		if wantPath && len(series) > 0 {
			path = series[0].Metric["path"]
		}
		results = append(results, res{name: name, val: &v, path: path})
	}

	wg.Add(7)
	go func() {
		defer wg.Done()
		collect("total_requests", fmt.Sprintf(`sum(count_over_time(%s [%s]))`, base, window), endNs, false)
	}()
	go func() {
		defer wg.Done()
		collect("avg_req_rate", fmt.Sprintf(`sum(rate(%s [%s]))`, base, window), endNs, false)
	}()
	go func() {
		defer wg.Done()
		collect("total_5xx", fmt.Sprintf(`sum(count_over_time(%s | status =~ "5.." [%s]))`, baseParse, window), endNs, false)
	}()
	go func() {
		defer wg.Done()
		collect("total_4xx", fmt.Sprintf(`sum(count_over_time(%s | status =~ "4.." [%s]))`, baseParse, window), endNs, false)
	}()
	go func() {
		defer wg.Done()
		collect("p95_rt_ms", fmt.Sprintf(`quantile_over_time(0.95, %s %s | unwrap rt [%s]) by ()`, base, rtParseExpr, window), endNs, false)
	}()
	go func() {
		defer wg.Done()
		collect("bandwidth_bytes", fmt.Sprintf(`sum(sum_over_time(%s | unwrap bytes [%s]))`, baseParse, window), endNs, false)
	}()
	go func() {
		defer wg.Done()
		collect("bot_rate", fmt.Sprintf("sum(rate(%s |~ `(?i)bot|crawl|spider|curl|wget|python|java|go-http` [%s]))", base, window), endNs, false)
	}()
	wg.Wait()

	// top error path (5xx, top-1 by path)
	topErrSeries, err := g.base.lokiQueryInstant(ctx, g.tenantUUID,
		fmt.Sprintf(`topk(1, sum by (path) (count_over_time(%s | status =~ "5.." [%s])))`, baseParse, window), endNs, 1)
	topErrPath := ""
	if err == nil && len(topErrSeries) > 0 {
		topErrPath = topErrSeries[0].Metric["path"]
	}

	m := &ReportServiceMetrics{TopErrorPath: topErrPath}
	for _, r := range results {
		if r.val == nil {
			continue
		}
		v := *r.val
		switch r.name {
		case "total_requests":
			m.TotalRequests = &v
		case "avg_req_rate":
			m.AvgReqRate = &v
		case "total_5xx":
			m.Total5xx = &v
		case "total_4xx":
			m.Total4xx = &v
		case "p95_rt_ms":
			m.P95RTMs = &v
		case "bandwidth_bytes":
			m.BandwidthBytes = &v
		case "bot_rate":
			m.BotRate = &v
		}
	}
	return m
}

// ─── Postgres extractor ───────────────────────────────────────────────────────

func (g *reportMetricsGatherer) extractPostgres(ctx context.Context, startNs, endNs int64) *ReportServiceMetrics {
	startSec := startNs / int64(1e9)
	endSec := endNs / int64(1e9)
	window := fmt.Sprintf("%ds", (endNs-startNs)/int64(1e9))
	m := &ReportServiceMetrics{}

	base := postgresLokiSelector("", "")

	type res struct {
		name string
		val  *float64
	}
	results := make([]res, 0, 4)
	var mu sync.Mutex
	var wg sync.WaitGroup
	collect := func(name, q string) {
		series, err := g.base.lokiQueryInstant(ctx, g.tenantUUID, q, endNs, 1)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			results = append(results, res{name: name})
			return
		}
		v := lokiSingleValue(series)
		results = append(results, res{name: name, val: &v})
	}

	wg.Add(4)
	go func() {
		defer wg.Done()
		collect("slow_queries", fmt.Sprintf(`sum(count_over_time(%s |~ "(?i)duration: [0-9.]+ ms" [%s]))`, base, window))
	}()
	go func() {
		defer wg.Done()
		collect("errors", fmt.Sprintf(`sum(count_over_time(%s |~ "(?i)\\b(ERROR|FATAL|PANIC)\\b" [%s]))`, base, window))
	}()
	go func() {
		defer wg.Done()
		collect("auth_failures", fmt.Sprintf(`sum(count_over_time(%s |~ "(?i)(password authentication failed|authentication failed|no pg_hba.conf entry)" [%s]))`, base, window))
	}()
	go func() {
		defer wg.Done()
		collect("lock_waits", fmt.Sprintf(`sum(count_over_time(%s |~ "(?i)still waiting for|deadlock detected|could not obtain lock" [%s]))`, base, window))
	}()
	wg.Wait()
	for _, r := range results {
		if r.val == nil {
			continue
		}
		v := *r.val
		switch r.name {
		case "slow_queries":
			m.SlowQueries = &v
		case "errors":
			m.Errors = &v
		case "auth_failures":
			m.AuthFailures = &v
		case "lock_waits":
			m.LockWaits = &v
		}
	}

	// VM scalars
	cacheQ := fmt.Sprintf(`100 * sum(rate(pg_stat_database_blks_hit_total[%s])) / clamp_min(sum(rate(pg_stat_database_blks_hit_total[%s]) + rate(pg_stat_database_blks_read_total[%s])), 1)`, reportRateWindow, reportRateWindow, reportRateWindow)
	m.CacheHitRatioPct = g.vmRangeScalar(ctx, cacheQ, startSec, endSec, vmAggAvg)
	m.DeadlocksPerMin = g.vmRangeScalar(ctx, fmt.Sprintf(`sum(rate(pg_stat_database_deadlocks_total[%s])) * 60`, reportRateWindow), startSec, endSec, vmAggAvg)
	m.ConnectionSaturationPct = g.vmRangeScalar(ctx, `100 * sum(pg_stat_database_numbackends) / max(pg_settings_max_connections)`, startSec, endSec, vmAggMax)
	m.DbSizeBytes = g.vmInstantScalar(ctx, `sum(pg_database_size_bytes)`, endSec)

	return m
}

// ─── MariaDB extractor ────────────────────────────────────────────────────────

func (g *reportMetricsGatherer) extractMariaDB(ctx context.Context, startNs, endNs int64) *ReportServiceMetrics {
	startSec := startNs / int64(1e9)
	endSec := endNs / int64(1e9)
	window := fmt.Sprintf("%ds", (endNs-startNs)/int64(1e9))
	m := &ReportServiceMetrics{}

	base := mariaDBLokiSelector("", "")

	type res struct {
		name string
		val  *float64
	}
	results := make([]res, 0, 4)
	var mu sync.Mutex
	var wg sync.WaitGroup
	collect := func(name, q string) {
		series, err := g.base.lokiQueryInstant(ctx, g.tenantUUID, q, endNs, 1)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			results = append(results, res{name: name})
			return
		}
		v := lokiSingleValue(series)
		results = append(results, res{name: name, val: &v})
	}

	wg.Add(4)
	go func() {
		defer wg.Done()
		collect("slow_queries", fmt.Sprintf(`sum(count_over_time(%s |~ "(?i)(Query_time|slow query)" [%s]))`, base, window))
	}()
	go func() {
		defer wg.Done()
		collect("errors", fmt.Sprintf(`sum(count_over_time(%s |~ "(?i)(\\[ERROR\\]| ERROR |error:|crash|failed)" [%s]))`, base, window))
	}()
	go func() {
		defer wg.Done()
		collect("deadlocks", fmt.Sprintf(`sum(count_over_time(%s |~ "(?i)(deadlock|lock wait timeout)" [%s]))`, base, window))
	}()
	go func() {
		defer wg.Done()
		collect("auth_failures", fmt.Sprintf(`sum(count_over_time(%s |~ "(?i)(access denied|authentication|denied for user)" [%s]))`, base, window))
	}()
	wg.Wait()
	for _, r := range results {
		if r.val == nil {
			continue
		}
		v := *r.val
		switch r.name {
		case "slow_queries":
			m.SlowQueries = &v
		case "errors":
			m.Errors = &v
		case "deadlocks":
			if wm := float64(endNs-startNs) / float64(1e9) / 60; wm > 0 {
				perMin := v / wm
				m.DeadlocksPerMin = &perMin
			}
		case "auth_failures":
			m.AuthFailures = &v
		}
	}

	m.QPS = g.vmRangeScalar(ctx, fmt.Sprintf(`rate(mysql_global_status_questions[%s])`, reportRateWindow), startSec, endSec, vmAggAvg)
	m.SlowQueriesPerMin = g.vmRangeScalar(ctx, fmt.Sprintf(`rate(mysql_global_status_slow_queries[%s]) * 60`, reportRateWindow), startSec, endSec, vmAggAvg)
	m.BufferPoolHitRatioPct = g.vmRangeScalar(ctx, fmt.Sprintf(`100 * (1 - (rate(mysql_global_status_innodb_buffer_pool_reads[%s]) / rate(mysql_global_status_innodb_buffer_pool_read_requests[%s]))) and on() (rate(mysql_global_status_innodb_buffer_pool_read_requests[%s]) > 0)`, reportRateWindow, reportRateWindow, reportRateWindow), startSec, endSec, vmAggAvg)
	m.ConnectionSaturationPct = g.vmRangeScalar(ctx, `100 * mysql_global_status_threads_connected / clamp_min(mysql_global_variables_max_connections, 1)`, startSec, endSec, vmAggMax)
	m.ThreadsConnected = g.vmRangeScalar(ctx, `sum(mysql_global_status_threads_connected)`, startSec, endSec, vmAggAvg)
	m.AbortedConnectsPerMin = g.vmRangeScalar(ctx, fmt.Sprintf(`rate(mysql_global_status_aborted_connects[%s]) * 60`, reportRateWindow), startSec, endSec, vmAggAvg)

	rndNext := g.vmRangeScalar(ctx, fmt.Sprintf(`rate(mysql_global_status_handlers_total{handler="read_rnd_next"}[%s])`, reportRateWindow), startSec, endSec, vmAggAvg)
	readKey := g.vmRangeScalar(ctx, fmt.Sprintf(`rate(mysql_global_status_handlers_total{handler="read_key"}[%s])`, reportRateWindow), startSec, endSec, vmAggAvg)
	if rndNext != nil && readKey != nil && *rndNext+*readKey > 0 {
		v := 100 * (*rndNext) / (*rndNext + *readKey)
		m.TableScanRatioPct = &v
	}

	return m
}

// ─── Redis extractor ──────────────────────────────────────────────────────────

func (g *reportMetricsGatherer) extractRedis(ctx context.Context, startNs, endNs int64) *ReportServiceMetrics {
	startSec := startNs / int64(1e9)
	endSec := endNs / int64(1e9)
	m := &ReportServiceMetrics{}

	m.CommandsPerSec = g.vmRangeScalar(ctx, fmt.Sprintf(`rate(redis_commands_processed_total[%s])`, reportRateWindow), startSec, endSec, vmAggAvg)
	m.ConnectedClients = g.vmRangeScalar(ctx, `sum(redis_connected_clients)`, startSec, endSec, vmAggAvg)
	m.EvictRate = g.vmRangeScalar(ctx, fmt.Sprintf(`rate(redis_evicted_keys_total[%s])`, reportRateWindow), startSec, endSec, vmAggAvg)
	m.ErrorRepliesPerSec = g.vmRangeScalar(ctx, fmt.Sprintf(`rate(redis_error_replies_total[%s])`, reportRateWindow), startSec, endSec, vmAggAvg)
	m.SlowLogCount = g.vmInstantScalar(ctx, `sum(redis_slowlog_length)`, endSec)
	m.OpcacheHitRate = nil

	usedMax := g.vmRangeScalar(ctx, `max(redis_memory_used_bytes)`, startSec, endSec, vmAggMax)
	maxMem := g.vmInstantScalar(ctx, `max(redis_memory_max_bytes)`, endSec)
	m.MemoryUsagePct = ratioPct(usedMax, maxMem)

	clientsAvg := g.vmRangeScalar(ctx, `sum(redis_connected_clients)`, startSec, endSec, vmAggAvg)
	maxClients := g.vmInstantScalar(ctx, `max(redis_config_maxclients)`, endSec)
	m.ClientSaturationPct = ratioPct(clientsAvg, maxClients)

	hits := g.vmRangeScalar(ctx, fmt.Sprintf(`rate(redis_keyspace_hits_total[%s])`, reportRateWindow), startSec, endSec, vmAggAvg)
	misses := g.vmRangeScalar(ctx, fmt.Sprintf(`rate(redis_keyspace_misses_total[%s])`, reportRateWindow), startSec, endSec, vmAggAvg)
	if hits != nil && misses != nil && *hits+*misses > 0 {
		v := 100 * (*hits) / (*hits + *misses)
		m.CacheHitRatioPct = &v
	}

	return m
}

// ─── PHP-FPM extractor ────────────────────────────────────────────────────────

func (g *reportMetricsGatherer) extractPHPFPM(ctx context.Context, startNs, endNs int64) *ReportServiceMetrics {
	startSec := startNs / int64(1e9)
	endSec := endNs / int64(1e9)
	window := fmt.Sprintf("%ds", (endNs-startNs)/int64(1e9))
	m := &ReportServiceMetrics{}

	base := phpfpmLokiSelector("", "")
	accessBase := base + ` | log_file_name=~".*\\.access\\.log$"`
	accessRegex := "`" + phpfpmAccessLogRegex + "`"

	m.WorkerSaturationPct = g.vmRangeScalar(ctx, `100 * sum(phpfpm_active_processes) / clamp_min(sum(phpfpm_total_processes), 1)`, startSec, endSec, vmAggMax)
	m.AcceptedConnPerSec = g.vmRangeScalar(ctx, fmt.Sprintf(`sum(rate(phpfpm_accepted_connections_total[%s]))`, reportRateWindow), startSec, endSec, vmAggAvg)
	m.SlowRequests = g.vmRangeScalar(ctx, fmt.Sprintf(`sum(increase(phpfpm_slow_requests_total[%s]))`, window), startSec, endSec, vmAggLatest)
	m.MaxChildrenReached = g.vmRangeScalar(ctx, fmt.Sprintf(`sum(increase(phpfpm_max_children_reached_total[%s]))`, window), startSec, endSec, vmAggLatest)
	m.OpcacheHitRate = g.vmRangeScalar(ctx, `avg(phpfpm_opcache_hit_rate)`, startSec, endSec, vmAggAvg)

	m.Status5xx = g.lokiCountInstant(ctx, fmt.Sprintf(`%s | regexp %s | status =~ "5.."`, accessBase, accessRegex), window, endNs)
	m.LatencyP95Ms = g.lokiQuantileInstant(ctx, fmt.Sprintf(`%s | regexp %s`, accessBase, accessRegex), "duration", window, endNs)

	// errors from error.log
	m.Errors = g.lokiCountInstant(ctx, base+` | log_file_name="error.log" |~ "(?i)(ERROR|ALERT|PHP Fatal)"`, window, endNs)

	return m
}

// ─── Fleet & containers (VM, tenant-wide) ─────────────────────────────────────

// ReportFleetMetrics holds the aggregated host-fleet scalars for a window.
type ReportFleetMetrics struct {
	AvgCPU     *float64 `json:"avg_cpu_pct"`
	AvgMem     *float64 `json:"avg_mem_pct"`
	AvgDisk    *float64 `json:"avg_disk_pct"`
	NetIn      *float64 `json:"net_in_bps"`
	NetOut     *float64 `json:"net_out_bps"`
	LoadAvg1m  *float64 `json:"load_avg_1m"`
	Containers int      `json:"containers"`
	AvgContCPU *float64 `json:"avg_container_cpu_pct"`
	AvgContMem *float64 `json:"avg_container_mem_pct"`
}

func (g *reportMetricsGatherer) extractFleet(ctx context.Context, startNs, endNs int64) *ReportFleetMetrics {
	startSec := startNs / int64(1e9)
	endSec := endNs / int64(1e9)
	f := &ReportFleetMetrics{}

	f.AvgCPU = g.vmRangeScalar(ctx, `(1 - avg(system_cpu_utilization_ratio{state="idle"})) * 100`, startSec, endSec, vmAggAvg)
	f.AvgMem = g.vmRangeScalar(ctx, `sum(system_memory_usage_bytes{state="used"}) / sum(system_memory_usage_bytes{state=~"used|free|buffered|cached"}) * 100`, startSec, endSec, vmAggAvg)
	f.AvgDisk = g.vmRangeScalar(ctx, `avg(system_filesystem_utilization_ratio{mountpoint="/"}) * 100`, startSec, endSec, vmAggAvg)
	f.NetIn = g.vmRangeScalar(ctx, fmt.Sprintf(`sum(rate(system_network_io_bytes_total{direction="receive"}[%s]))`, reportRateWindow), startSec, endSec, vmAggAvg)
	f.NetOut = g.vmRangeScalar(ctx, fmt.Sprintf(`sum(rate(system_network_io_bytes_total{direction="transmit"}[%s]))`, reportRateWindow), startSec, endSec, vmAggAvg)
	f.LoadAvg1m = g.vmRangeScalar(ctx, `avg(system_cpu_load_average_1m)`, startSec, endSec, vmAggAvg)

	f.Containers = 0
	f.AvgContCPU = g.vmRangeScalar(ctx, `avg({__name__=~"container_.*_cpu_percent"})`, startSec, endSec, vmAggAvg)
	f.AvgContMem = g.vmRangeScalar(ctx, `avg({__name__=~"container_.*_mem_percent"})`, startSec, endSec, vmAggAvg)
	if c := g.vmRangeScalar(ctx, `count({__name__=~"container_.*_cpu_percent"})`, startSec, endSec, vmAggLatest); c != nil {
		f.Containers = int(*c)
	}

	return f
}

// ─── Log signal counts (B3, reuses timelineProfiles) ─────────────────────────

// countLogSignals counts error and warning log lines for a service kind in a
// window, using the same regex profiles the overview timeline uses.
func (g *reportMetricsGatherer) countLogSignals(ctx context.Context, kind string, startNs, endNs int64) *ReportLogSignals {
	window := fmt.Sprintf("%ds", (endNs-startNs)/int64(1e9))
	profile, ok := timelineProfiles[kind]
	if !ok {
		profile = timelineProfiles[serviceKind(kind)]
	}
	selector := fmt.Sprintf(`{service_name=~"%s"}`, serviceNameRegex(kind))

	errExpr := joinRegexPatterns(profile.ErrorPatterns)
	warnExpr := joinRegexPatterns(profile.WarningPatterns)

	out := &ReportLogSignals{}
	if errExpr != "" {
		if v := g.lokiCountInstant(ctx, fmt.Sprintf(`%s |~ "%s"`, selector, escapeLokiString(errExpr)), window, endNs); v != nil {
			out.Errors = int(*v)
		}
	}
	if warnExpr != "" {
		if v := g.lokiCountInstant(ctx, fmt.Sprintf(`%s |~ "%s"`, selector, escapeLokiString(warnExpr)), window, endNs); v != nil {
			out.Warnings = int(*v)
		}
	}
	return out
}

func joinRegexPatterns(patterns []*regexp.Regexp) string {
	if len(patterns) == 0 {
		return ""
	}
	parts := make([]string, 0, len(patterns))
	for _, p := range patterns {
		parts = append(parts, p.String())
	}
	return strings.Join(parts, "|")
}

// ─── Service up (B3 service health) ───────────────────────────────────────────

// serviceUp reports whether at least one live instance of a service kind exists
// at the given instant (used for tenant-wide health counts).
func (g *reportMetricsGatherer) serviceUp(ctx context.Context, kind string, endSec int64) bool {
	var q string
	switch kind {
	case "nginx":
		q = `count(nginx_requests_total)`
	case "apache":
		q = `count(apache_requests_total)`
	case "postgresql":
		q = `sum(pg_up)`
	case "mariadb":
		q = `sum(mysql_up)`
	case "redis":
		q = `sum(redis_up)`
	case "phpfpm":
		q = `sum(phpfpm_up)`
	case "container":
		q = `count({__name__=~"container_.*_cpu_percent"})`
	default:
		return false
	}
	v := g.vmInstantScalar(ctx, q, endSec)
	return v != nil && *v > 0
}
