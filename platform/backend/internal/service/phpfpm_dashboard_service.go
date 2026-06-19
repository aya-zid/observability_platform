package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
)

type PHPFPMDashboardService struct {
	base  *NginxDashboardService
	redis *redis.Client
}

type PHPFPMDashboardSummary struct {
	Up              *float64 `json:"up,omitempty"`
	ScrapeFailures  *float64 `json:"scrape_failures,omitempty"`
	UptimeSeconds   *float64 `json:"uptime_seconds,omitempty"`
	HealthScore     *float64 `json:"health_score,omitempty"`

	ActiveProcesses     *float64 `json:"active_processes,omitempty"`
	IdleProcesses       *float64 `json:"idle_processes,omitempty"`
	TotalProcesses      *float64 `json:"total_processes,omitempty"`
	MaxActiveProcesses  *float64 `json:"max_active_processes,omitempty"`
	WorkerSaturationPct *float64 `json:"worker_saturation_pct,omitempty"`

	ListenQueue         *float64 `json:"listen_queue,omitempty"`
	ListenQueueLen      *float64 `json:"listen_queue_len,omitempty"`
	QueueUtilizationPct *float64 `json:"queue_utilization_pct,omitempty"`
	MaxListenQueue      *float64 `json:"max_listen_queue,omitempty"`

	AcceptedConnections *float64 `json:"accepted_connections,omitempty"`
	AcceptedConnPerSec  *float64 `json:"accepted_conn_per_sec,omitempty"`

	SlowRequests       *float64 `json:"slow_requests,omitempty"`
	MaxChildrenReached *float64 `json:"max_children_reached,omitempty"`
	AvgDurationMs      *float64 `json:"avg_duration_ms,omitempty"`
	AvgMemoryBytes     *float64 `json:"avg_memory_bytes,omitempty"`
	AvgMemoryKb        *float64 `json:"avg_memory_kb,omitempty"`
	AvgCpuPct          *float64 `json:"avg_cpu_pct,omitempty"`
	RunningProcesses   *float64 `json:"running_processes,omitempty"`

	OpcacheEnabled       *float64 `json:"opcache_enabled,omitempty"`
	OpcacheHitRate       *float64 `json:"opcache_hit_rate,omitempty"`
	OpcacheMemoryUsed    *float64 `json:"opcache_memory_used_ratio,omitempty"`
	OpcacheMemoryFree    *float64 `json:"opcache_memory_free_bytes,omitempty"`
	OpcacheMemoryTotal   *float64 `json:"opcache_memory_total_bytes,omitempty"`
	OpcacheWasted        *float64 `json:"opcache_wasted_ratio,omitempty"`
	OpcacheScripts       *float64 `json:"opcache_cached_scripts,omitempty"`
	OpcacheHits          *float64 `json:"opcache_hits_total,omitempty"`
	OpcacheMisses        *float64 `json:"opcache_misses_total,omitempty"`
	OpcacheRestarts      *float64 `json:"opcache_restarts_total,omitempty"`

	TopSlowScripts       []ScriptStat  `json:"top_slow_scripts,omitempty"`
	TopMemScripts        []ScriptStat  `json:"top_mem_scripts,omitempty"`
	TopProcessesDuration []ProcessStat `json:"top_processes_duration,omitempty"`
	TopProcessesMemory   []ProcessStat `json:"top_processes_memory,omitempty"`

	LatencyP50 *float64 `json:"latency_p50,omitempty"`
	LatencyP95 *float64 `json:"latency_p95,omitempty"`
	LatencyP99 *float64 `json:"latency_p99,omitempty"`

	Status2xx *float64     `json:"status_2xx,omitempty"`
	Status4xx *float64     `json:"status_4xx,omitempty"`
	Status5xx *float64     `json:"status_5xx,omitempty"`
	TopURIs      []LabelCount `json:"top_uris,omitempty"`
	TopErrorURIs []LabelCount `json:"top_error_uris,omitempty"`

	WorkersTimeseries    []TimePoint `json:"workers_timeseries"`
	QueueTimeseries      []TimePoint `json:"queue_timeseries"`
	ThroughputTimeseries []TimePoint `json:"throughput_timeseries"`
	DurationTimeseries   []TimePoint `json:"duration_timeseries"`
	MemoryTimeseries     []TimePoint `json:"memory_timeseries"`
	CpuTimeseries        []TimePoint `json:"cpu_timeseries"`

	OpcacheHitRateTimeseries  []TimePoint `json:"opcache_hit_rate_timeseries"`
	OpcacheMemoryTimeseries   []TimePoint `json:"opcache_memory_timeseries"`
	OpcacheScriptsTimeseries  []TimePoint `json:"opcache_scripts_timeseries"`

	StatusTimeseries   []TimePoint `json:"status_timeseries"`
	ReqRateTimeseries  []TimePoint `json:"req_rate_timeseries"`

	LogSummary PHPFPMLogSummary `json:"log_summary"`
	RecentLogs []PHPFPMLogEvent `json:"recent_logs"`
	Pools []string `json:"pools"`
	PoolDetails map[string]PoolDetail `json:"pool_details"`

	MethodDistribution  []LabelCount `json:"method_distribution"`
	StatusDistribution  []LabelCount `json:"status_distribution"`
	DurationHistogram   []LabelCount `json:"duration_histogram"`
	MemoryHistogram     []LabelCount `json:"memory_histogram"`
	CPUHistogram        []LabelCount `json:"cpu_histogram"`
	TopErrorMessages    []LabelCount `json:"top_error_messages"`
	ErrorTypeBreakdown  []LabelCount `json:"error_type_breakdown"`
}

type ScriptStat struct {
	Script      string  `json:"script"`
	AvgDuration float64 `json:"avg_duration_ms"`
	AvgMemory   float64 `json:"avg_memory_kb"`
	Count       float64 `json:"count"`
}

type ProcessStat struct {
	PID      string  `json:"pid"`
	State    string  `json:"state"`
	Duration float64 `json:"duration_ms"`
	Memory   float64 `json:"memory_kb"`
	CPU      float64 `json:"cpu_pct"`
	Requests float64 `json:"requests"`
}

type PHPFPMLogSummary struct {
	Warnings     float64 `json:"warnings"`
	Errors       float64 `json:"errors"`
	Notices      float64 `json:"notices"`
	SlowRequests float64 `json:"slow_requests"`
	Startups     float64 `json:"startups"`
}

type PoolDetail struct {
	ActiveProcesses    *float64     `json:"active_processes"`
	IdleProcesses      *float64     `json:"idle_processes"`
	TotalProcesses     *float64     `json:"total_processes"`
	SaturationPct      *float64     `json:"saturation_pct"`
	AcceptedConnPerSec *float64     `json:"accepted_conn_per_sec"`
	MaxChildrenReached *float64     `json:"max_children_reached"`
	SlowRequests       *float64     `json:"slow_requests"`

	Status2xx  *float64     `json:"status_2xx"`
	Status4xx  *float64     `json:"status_4xx"`
	Status5xx  *float64     `json:"status_5xx"`
	LatencyP50 *float64     `json:"latency_p50"`
	LatencyP95 *float64     `json:"latency_p95"`
	LatencyP99 *float64     `json:"latency_p99"`
	TopSlowScripts []ScriptStat   `json:"top_slow_scripts"`
	TopURIs        []LabelCount   `json:"top_uris"`
	RequestCount   *float64       `json:"request_count"`

	ErrorCount       *float64     `json:"error_count"`
	WarningCount     *float64     `json:"warning_count"`
	NoticeCount      *float64     `json:"notice_count"`
	SlowRequestCount *float64     `json:"slow_request_count"`
	TopErrorMessages []LabelCount `json:"top_error_messages"`
}

type PHPFPMLogEvent struct {
	Timestamp string            `json:"timestamp"`
	Severity  string            `json:"severity"`
	Pool      string            `json:"pool"`
	Category  string            `json:"category"`
	Message   string            `json:"message"`
	Labels    map[string]string `json:"labels,omitempty"`
	Method    string  `json:"method,omitempty"`
	URI       string  `json:"uri,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	Script    string  `json:"script,omitempty"`
	DurationMs float64 `json:"duration_ms,omitempty"`
	MemoryKB  float64 `json:"memory_kb,omitempty"`
	CPU       float64 `json:"cpu,omitempty"`
}

type PHPFPMLogsPage struct {
	Events   []PHPFPMLogEvent `json:"events"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	HasMore  bool             `json:"has_more"`
}

type PHPFPMLogsTypeMeta struct {
	AccessCount int `json:"access_count"`
	ErrorCount  int `json:"error_count"`
	SlowCount   int `json:"slow_count"`
}

func NewPHPFPMDashboardService(cfg *config.Config, redisClient *redis.Client) *PHPFPMDashboardService {
	return &PHPFPMDashboardService{
		base:  NewNginxDashboardService(cfg, redisClient),
		redis: redisClient,
	}
}

func (s *PHPFPMDashboardService) GetDashboardSummary(
	ctx context.Context,
	tenantUUID string,
	tenantID uint,
	serviceName string,
	startNs, endNs int64,
	step string,
) (*PHPFPMDashboardSummary, error) {
	startSec := startNs / int64(1e9)
	endSec := endNs / int64(1e9)
	cacheKey := fmt.Sprintf("phpfpm:dash:%s:%s:%d:%d:%s", tenantUUID, serviceName, startSec, endSec, step)

	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var result PHPFPMDashboardSummary
			if json.Unmarshal(cached, &result) == nil {
				return &result, nil
			}
		}
	}

	summary, err := s.fetchDashboard(ctx, tenantUUID, tenantID, serviceName, startNs, endNs, step)
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

const phpfpmAccessLogRegex = `^(?P<pool>\S+) (?P<pid>\S+) "(?P<method>[A-Z]+) (?P<uri>[^"]*)" (?P<status>\d+) (?P<script>\S+) (?P<duration>[\d.]+) (?P<memory>[\d.]+) (?P<cpu>[\d.]+)$`

var phpfpmAccessLogRE = regexp.MustCompile(`^(\S+)\s+(\S+)\s+"(\S+)\s+(.*?)"\s+(\d+)\s+(\S+)\s+([\d.]+)\s+([\d.]+)\s+([\d.]+)$`)

var phpfpmSlowLogHeaderRE = regexp.MustCompile(`^\[\d{2}-[A-Z][a-z]{2}-\d{4}`)
var phpfpmSlowLogPoolRE = regexp.MustCompile(`\[pool\s+(\S+)\]`)
var phpfpmSlowLogScriptRE = regexp.MustCompile(`script_filename\s*=\s*(\S+)`)
var phpfpmSlowLogBacktraceRE = regexp.MustCompile(`\[0x[0-9a-fA-F]+\]\s+(.+)$`)

func (s *PHPFPMDashboardService) fetchDashboard(
	ctx context.Context,
	tenantUUID string,
	tenantID uint,
	serviceName string,
	startNs, endNs int64,
	step string,
) (*PHPFPMDashboardSummary, error) {
	startSec := startNs / int64(1e9)
	endSec := endNs / int64(1e9)
	rateWindow := "5m"
	rangeSeconds := endSec - startSec
	if rangeSeconds < 60 {
		rangeSeconds = 60
	}

	type vR struct {
		data []VMSeries
		err  error
	}
	type lR struct {
		data []LokiMatrixSeries
		err  error
	}
	type logR struct {
		entries []LokiLogEntry
		err     error
	}

	query := func(q string) vR {
		data, err := s.base.vmQueryRange(ctx, tenantID, q, startSec, endSec, step)
		return vR{data: data, err: err}
	}
	lokiQuery := func(q string) lR {
		d, e := s.base.lokiQueryRange(ctx, tenantUUID, q, startNs, endNs, step, 5000)
		return lR{d, e}
	}
	lokiInstant := func(q string) lR {
		d, e := s.base.lokiQueryInstant(ctx, tenantUUID, q, endNs, 5000)
		return lR{d, e}
	}
	logQuery := func(q string, limit int) logR {
		entries, err := s.base.lokiQueryLogEntries(ctx, tenantUUID, q, startNs, endNs, limit)
		return logR{entries: entries, err: err}
	}

	results := make(map[string]vR)
	lokiResults := make(map[string]lR)
	logResults := make(map[string]logR)
	var resultsMu sync.Mutex
	var wg sync.WaitGroup

	vmQueries := map[string]string{
		"up":                  "phpfpm_up",
		"scrape_failures":     "phpfpm_scrape_failures_total",
		"start_since":         "phpfpm_start_since_total",
		"active":              "phpfpm_active_processes",
		"idle":                "phpfpm_idle_processes",
		"total":               "phpfpm_total_processes",
		"max_active":          "phpfpm_max_active_processes_total",
		"listen_queue":        "phpfpm_listen_queue",
		"listen_queue_len":    "phpfpm_listen_queue_length",
		"max_listen_queue":    "phpfpm_max_listen_queue_total",
		"accepted":            "phpfpm_accepted_connections_total",
		"accepted_rate":       "rate(phpfpm_accepted_connections_total[" + rateWindow + "])",
		"slow_requests":       "phpfpm_slow_requests_total",
		"max_children":        "phpfpm_max_children_reached_total",
		"children_rate":       "rate(phpfpm_max_children_reached_total[" + rateWindow + "])",
		"proc_duration":       "avg(phpfpm_process_request_duration)",
		"proc_memory":         "avg(phpfpm_process_last_request_memory)",
		"proc_cpu":            "avg(phpfpm_process_last_request_cpu)",
		"running_procs":       "sum(phpfpm_process_state{state=\"Running\"}) or vector(0)",
	}

	for name, q := range vmQueries {
		name, q := name, q
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := query(q)
			resultsMu.Lock()
			results[name] = result
			resultsMu.Unlock()
		}()
	}

	opcacheQueries := map[string]string{
		"opcache_enabled":     "phpfpm_opcache_enabled",
		"opcache_hit_rate":    "phpfpm_opcache_hit_rate",
		"opcache_memory_used": "phpfpm_opcache_memory_used_ratio",
		"opcache_memory_free": "phpfpm_opcache_memory_free_bytes",
		"opcache_memory_total": "phpfpm_opcache_memory_total_bytes",
		"opcache_wasted":      "phpfpm_opcache_wasted_ratio",
		"opcache_scripts":     "phpfpm_opcache_cached_scripts_total",
		"opcache_hits":        "phpfpm_opcache_hits_total",
		"opcache_misses":      "phpfpm_opcache_misses_total",
		"opcache_restarts":    "phpfpm_opcache_restarts_total",
	}

	for name, q := range opcacheQueries {
		name, q := name, q
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := query(q)
			resultsMu.Lock()
			results[name] = result
			resultsMu.Unlock()
		}()
	}

	opcacheTSQueries := map[string]string{
		"opcache_ts_hit":   "phpfpm_opcache_hit_rate",
		"opcache_ts_used":  "phpfpm_opcache_memory_used_ratio",
		"opcache_ts_cache": "phpfpm_opcache_cached_scripts_total",
	}
	for name, q := range opcacheTSQueries {
		name, q := name, q
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := query(q)
			resultsMu.Lock()
			results[name] = result
			resultsMu.Unlock()
		}()
	}

	var topProcDur, topProcMem vR
	wg.Add(1)
	go func() {
		defer wg.Done()
		d, e := s.base.vmQueryInstant(ctx, tenantID, `topk(10, phpfpm_process_request_duration)`, endSec)
		resultsMu.Lock()
		topProcDur = vR{data: d, err: e}
		resultsMu.Unlock()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		d, e := s.base.vmQueryInstant(ctx, tenantID, `topk(10, phpfpm_process_last_request_memory)`, endSec)
		resultsMu.Lock()
		topProcMem = vR{data: d, err: e}
		resultsMu.Unlock()
	}()

	baseSelector := phpfpmLokiSelector(serviceName)
	accessBase := baseSelector + ` | log_file_name="access.log"`

	accessStatusQ := fmt.Sprintf(
		`sum by (status_class) (rate(%s | regexp \`+"`"+phpfpmAccessLogRegex+"`"+` [%s]))`,
		accessBase, step,
	)
	accessRateQ := fmt.Sprintf(`sum(rate(%s [%s]))`, accessBase, step)
	topURIQ := fmt.Sprintf(
		`topk(20, sum by (uri, status) (count_over_time(%s | regexp \`+"`"+phpfpmAccessLogRegex+"`"+` [%ds])))`,
		accessBase, rangeSeconds,
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		result := lokiQuery(accessStatusQ)
		resultsMu.Lock()
		lokiResults["status_codes"] = result
		resultsMu.Unlock()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		result := lokiQuery(accessRateQ)
		resultsMu.Lock()
		lokiResults["req_rate"] = result
		resultsMu.Unlock()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		result := lokiInstant(topURIQ)
		resultsMu.Lock()
		lokiResults["top_uris"] = result
		resultsMu.Unlock()
	}()

	errorBase := baseSelector + ` | log_file_name="error.log"`
	const dashboardPreviewLimit = 20
	logQueries := map[string]string{
		"warnings":     errorBase + ` |= "WARNING"`,
		"errors":       errorBase + ` |~ "(?i)(ERROR|ALERT|PHP Fatal)"`,
		"slow_request": errorBase + ` |= "executing too slow"`,
		"startups":     errorBase + ` |~ "systemd monitor|starting"`,
	}
	for name, q := range logQueries {
		name, q := name, q
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := logQuery(q, dashboardPreviewLimit)
			resultsMu.Lock()
			logResults[name] = result
			resultsMu.Unlock()
		}()
	}

	wg.Add(1)
	var accessLogResult logR
	go func() {
		defer wg.Done()
		entries, err := s.base.lokiQueryLogEntries(ctx, tenantUUID, accessBase, startNs, endNs, 5000)
		resultsMu.Lock()
		accessLogResult = logR{entries: entries, err: err}
		resultsMu.Unlock()
	}()
	wg.Add(1)
	var errorLogAllResult logR
	go func() {
		defer wg.Done()
		entries, err := s.base.lokiQueryLogEntries(ctx, tenantUUID, errorBase, startNs, endNs, 5000)
		resultsMu.Lock()
		errorLogAllResult = logR{entries: entries, err: err}
		resultsMu.Unlock()
	}()

	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	active := latestVMValue(results["active"].data)
	total := latestVMValue(results["total"].data)
	var saturation *float64
	if active != nil && total != nil && *total > 0 {
		v := 100 * (*active) / (*total)
		saturation = &v
	}

	queue := latestVMValue(results["listen_queue"].data)
	queueLen := latestVMValue(results["listen_queue_len"].data)
	var queueUtil *float64
	if queue != nil && queueLen != nil && *queueLen > 0 {
		v := 100 * (*queue) / (*queueLen)
		queueUtil = &v
	}

	procDuration := latestVMValue(results["proc_duration"].data)
	procMemory := latestVMValue(results["proc_memory"].data)
	procCpu := latestVMValue(results["proc_cpu"].data)
	var memKb *float64
	if procMemory != nil {
		v := (*procMemory) / 1024
		memKb = &v
	}

	slowReq := latestVMValue(results["slow_requests"].data)
	childrenRate := latestVMValue(results["children_rate"].data)

	healthScore := computePHPFPMHealthScore(saturation, queueUtil, slowReq, childrenRate)

	topSlowScripts, topMemScripts, latencyP50, latencyP95, latencyP99 := parsePHPFPMAccessLogs(accessLogResult.entries)
	topProcDurStats := parseProcessStats(topProcDur.data, "duration")
	topProcMemStats := parseProcessStats(topProcMem.data, "memory")

	status2xx := lokiSumLabel(lokiResults["status_codes"].data, "2xx")
	status4xx := lokiSumLabel(lokiResults["status_codes"].data, "4xx")
	status5xx := lokiSumLabel(lokiResults["status_codes"].data, "5xx")
	topURIs, topErrorURIs := parseTopPHPFMURIs(lokiResults["top_uris"].data)
	pools := detectPHPFPMPools(results["active"].data)

	methodDist, statusDist, durHist, memHist, cpuHist := computeAccessLogAnalytics(accessLogResult.entries)
	topErrMsgs, errTypeBreakdown := computeErrorLogAnalytics(errorLogAllResult.entries)

	summary := &PHPFPMDashboardSummary{
		Up:                  latestVMValue(results["up"].data),
		ScrapeFailures:      latestVMValue(results["scrape_failures"].data),
		UptimeSeconds:       latestVMValue(results["start_since"].data),
		HealthScore:         healthScore,
		ActiveProcesses:     active,
		IdleProcesses:       latestVMValue(results["idle"].data),
		TotalProcesses:      total,
		MaxActiveProcesses:  latestVMValue(results["max_active"].data),
		WorkerSaturationPct: saturation,
		ListenQueue:         queue,
		ListenQueueLen:      queueLen,
		QueueUtilizationPct: queueUtil,
		MaxListenQueue:      latestVMValue(results["max_listen_queue"].data),
		AcceptedConnections: latestVMValue(results["accepted"].data),
		AcceptedConnPerSec:  latestVMValue(results["accepted_rate"].data),
		SlowRequests:       slowReq,
		MaxChildrenReached: latestVMValue(results["max_children"].data),
		AvgDurationMs:      procDuration,
		AvgMemoryBytes:     procMemory,
		AvgMemoryKb:        memKb,
		AvgCpuPct:          procCpu,
		RunningProcesses: latestVMValue(results["running_procs"].data),
		OpcacheEnabled:    latestVMValue(results["opcache_enabled"].data),
		OpcacheHitRate:    latestVMValue(results["opcache_hit_rate"].data),
		OpcacheMemoryUsed: latestVMValue(results["opcache_memory_used"].data),
		OpcacheMemoryFree: latestVMValue(results["opcache_memory_free"].data),
		OpcacheMemoryTotal: latestVMValue(results["opcache_memory_total"].data),
		OpcacheWasted:     latestVMValue(results["opcache_wasted"].data),
		OpcacheScripts:    latestVMValue(results["opcache_scripts"].data),
		OpcacheHits:       latestVMValue(results["opcache_hits"].data),
		OpcacheMisses:     latestVMValue(results["opcache_misses"].data),
		OpcacheRestarts:   latestVMValue(results["opcache_restarts"].data),
		TopSlowScripts:       topSlowScripts,
		TopMemScripts:        topMemScripts,
		TopProcessesDuration: topProcDurStats,
		TopProcessesMemory:   topProcMemStats,
		LatencyP50: latencyP50,
		LatencyP95: latencyP95,
		LatencyP99: latencyP99,
		Status2xx: status2xx,
		Status4xx: status4xx,
		Status5xx: status5xx,
		TopURIs:      topURIs,
		TopErrorURIs: topErrorURIs,
		Pools: pools,
		MethodDistribution: methodDist,
		StatusDistribution: statusDist,
		DurationHistogram:  durHist,
		MemoryHistogram:    memHist,
		CPUHistogram:       cpuHist,
		TopErrorMessages:   topErrMsgs,
		ErrorTypeBreakdown: errTypeBreakdown,
	}

	summary.WorkersTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"active": results["active"].data,
		"idle":   results["idle"].data,
		"total":  results["total"].data,
	})
	summary.QueueTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"queue": results["listen_queue"].data,
		"max":   results["listen_queue_len"].data,
	})
	summary.ThroughputTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"accepted": results["accepted_rate"].data,
	})
	summary.DurationTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"duration": results["proc_duration"].data,
	})
	summary.MemoryTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"memory": results["proc_memory"].data,
	})
	summary.CpuTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"cpu": results["proc_cpu"].data,
	})
	summary.OpcacheHitRateTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"hit_rate": results["opcache_ts_hit"].data,
	})
	summary.OpcacheMemoryTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"used_ratio": results["opcache_ts_used"].data,
	})
	summary.OpcacheScriptsTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"cached": results["opcache_ts_cache"].data,
	})
	summary.StatusTimeseries = mergeNamedLokiSeries(map[string][]LokiMatrixSeries{
		"2xx": filterStatusClass(lokiResults["status_codes"].data, "2"),
		"4xx": filterStatusClass(lokiResults["status_codes"].data, "4"),
		"5xx": filterStatusClass(lokiResults["status_codes"].data, "5"),
	})
	if len(lokiResults["req_rate"].data) > 0 {
		for _, pt := range lokiResults["req_rate"].data[0].Values {
			summary.ReqRateTimeseries = append(summary.ReqRateTimeseries,
				TimePoint{"time": pt[0], "req": pt[1]})
		}
	}

	summary.PoolDetails = buildPoolDetails(
		results["active"].data, results["idle"].data, results["total"].data,
		results["accepted_rate"].data, results["max_children"].data, results["slow_requests"].data,
		accessLogResult.entries, errorLogAllResult.entries, pools,
	)
	summary.LogSummary = s.countPHPFPMLogCategories(ctx, tenantUUID, serviceName, startNs, endNs)
	summary.RecentLogs = mergePHPFPMLogEvents(dashboardPreviewLimit,
		logResults["warnings"].entries,
		logResults["errors"].entries,
		logResults["slow_request"].entries,
		logResults["startups"].entries,
	)

	return summary, nil
}

func filterStatusClass(series []LokiMatrixSeries, prefix string) []LokiMatrixSeries {
	var out []LokiMatrixSeries
	for _, s := range series {
		status := s.Metric["status_class"]
		if strings.HasPrefix(status, prefix) {
			out = append(out, s)
		}
	}
	return out
}

func lokiSumLabel(series []LokiMatrixSeries, labelValue string) *float64 {
	var sum float64
	for _, s := range series {
		for _, v := range s.Values {
			sum += v[1]
		}
	}
	if len(series) == 0 {
		return nil
	}
	return &sum
}

func detectPHPFPMPools(series []VMSeries) []string {
	seen := map[string]struct{}{}
	for _, s := range series {
		if pool, ok := s.Metric["pool"]; ok && pool != "" {
			seen[pool] = struct{}{}
		}
	}
	pools := make([]string, 0, len(seen))
	for p := range seen {
		pools = append(pools, p)
	}
	sort.Strings(pools)
	return pools
}

func buildPoolDetails(
	activeSeries, idleSeries, totalSeries []VMSeries,
	acceptedRateSeries, maxChildrenSeries, slowReqSeries []VMSeries,
	accessEntries []LokiLogEntry,
	errorEntries []LokiLogEntry,
	pools []string,
) map[string]PoolDetail {
	if len(pools) == 0 {
		return nil
	}
	details := make(map[string]PoolDetail, len(pools))
	poolAccess := parseAccessLogsByPool(accessEntries)
	poolErrors := parseErrorLogsByPool(errorEntries)

	for _, pool := range pools {
		d := PoolDetail{}
		d.ActiveProcesses = poolLatestVMValue(activeSeries, pool)
		d.IdleProcesses = poolLatestVMValue(idleSeries, pool)
		d.TotalProcesses = poolLatestVMValue(totalSeries, pool)
		if d.ActiveProcesses != nil && d.TotalProcesses != nil && *d.TotalProcesses > 0 {
			v := 100 * (*d.ActiveProcesses) / (*d.TotalProcesses)
			d.SaturationPct = &v
		}
		d.AcceptedConnPerSec = poolRateVMValue(acceptedRateSeries, pool)
		d.MaxChildrenReached = poolLatestVMValue(maxChildrenSeries, pool)
		d.SlowRequests = poolLatestVMValue(slowReqSeries, pool)

		if pa, ok := poolAccess[pool]; ok && pa.requests > 0 {
			r := float64(pa.requests)
			s2 := float64(pa.status2xx)
			s4 := float64(pa.status4xx)
			s5 := float64(pa.status5xx)
			d.RequestCount = &r
			d.Status2xx = &s2
			d.Status4xx = &s4
			d.Status5xx = &s5
			d.TopURIs = topN(pa.uris, 5)
			if len(pa.durations) > 0 {
				sort.Float64s(pa.durations)
				p50 := pa.durations[int(float64(len(pa.durations))*0.50)]
				p95 := pa.durations[int(float64(len(pa.durations))*0.95)]
				p99 := pa.durations[int(float64(len(pa.durations))*0.99)]
				d.LatencyP50 = &p50
				d.LatencyP95 = &p95
				d.LatencyP99 = &p99
			}
			slowList := make([]ScriptStat, 0, len(pa.scripts))
			for script, acc := range pa.scripts {
				slowList = append(slowList, ScriptStat{
					Script:      script,
					AvgDuration: acc.totalDur / acc.count,
					AvgMemory:   acc.totalMem / acc.count,
					Count:       acc.count,
				})
			}
			sort.Slice(slowList, func(i, j int) bool { return slowList[i].AvgDuration > slowList[j].AvgDuration })
			if len(slowList) > 5 {
				slowList = slowList[:5]
			}
			d.TopSlowScripts = slowList
		}

		if pe, ok := poolErrors[pool]; ok {
			ec := float64(pe.errorCount)
			wc := float64(pe.warningCount)
			nc := float64(pe.noticeCount)
			sc := float64(pe.slowRequestCount)
			d.ErrorCount = &ec
			d.WarningCount = &wc
			d.NoticeCount = &nc
			d.SlowRequestCount = &sc
			if len(pe.errorMessages) > 0 {
				d.TopErrorMessages = topN(pe.errorMessages, 5)
			}
		}

		details[pool] = d
	}
	return details
}

type poolAccessStats struct {
	requests  int
	status2xx int
	status4xx int
	status5xx int
	durations []float64
	scripts   map[string]*scriptAccum
	uris      map[string]float64
	errorURIs map[string]float64
}

type scriptAccum struct {
	totalDur float64
	totalMem float64
	count    float64
}

func parseAccessLogsByPool(entries []LokiLogEntry) map[string]*poolAccessStats {
	pools := map[string]*poolAccessStats{}
	for _, entry := range entries {
		line := strings.TrimSpace(entry.Line)
		matches := phpfpmAccessLogRE.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		poolName := matches[1]
		if poolName == "" {
			continue
		}
		if _, ok := pools[poolName]; !ok {
			pools[poolName] = &poolAccessStats{
				scripts:   map[string]*scriptAccum{},
				uris:      map[string]float64{},
				errorURIs: map[string]float64{},
			}
		}
		p := pools[poolName]
		p.requests++

		status := matches[5]
		if len(status) > 0 {
			switch status[0] {
			case '2':
				p.status2xx++
			case '4':
				p.status4xx++
			case '5':
				p.status5xx++
			}
		}

		dur, _ := strconv.ParseFloat(matches[7], 64)
		mem, _ := strconv.ParseFloat(matches[8], 64)
		p.durations = append(p.durations, dur)

		uri := matches[4]
		p.uris[uri]++
		if len(status) > 0 && (status[0] == '4' || status[0] == '5') {
			p.errorURIs[uri]++
		}

		script := matches[6]
		if dur > 0 && script != "" && !strings.HasPrefix(script, "-") {
			if _, ok := p.scripts[script]; !ok {
				p.scripts[script] = &scriptAccum{}
			}
			p.scripts[script].totalDur += dur
			p.scripts[script].totalMem += mem
			p.scripts[script].count++
		}
	}
	return pools
}

type poolErrorStats struct {
	errorCount      int
	warningCount    int
	noticeCount     int
	slowRequestCount int
	errorMessages   map[string]float64
}

func parseErrorLogsByPool(entries []LokiLogEntry) map[string]*poolErrorStats {
	pools := map[string]*poolErrorStats{}
	for _, entry := range entries {
		ev := parsePHPFPMLogEvent(entry)
		if ev.Pool == "" {
			continue
		}
		if _, ok := pools[ev.Pool]; !ok {
			pools[ev.Pool] = &poolErrorStats{errorMessages: map[string]float64{}}
		}
		p := pools[ev.Pool]
		switch ev.Category {
		case "error":
			p.errorCount++
		case "warning":
			p.warningCount++
		case "notice":
			p.noticeCount++
		case "slow_request":
			p.slowRequestCount++
		}
		msg := ev.Message
		if msg != "" {
			p.errorMessages[msg]++
		}
	}
	return pools
}

func poolLatestVMValue(series []VMSeries, pool string) *float64 {
	for _, s := range series {
		if s.Metric["pool"] == pool {
			return latestVMValue([]VMSeries{s})
		}
	}
	return nil
}

func poolRateVMValue(series []VMSeries, pool string) *float64 {
	for _, s := range series {
		if s.Metric["pool"] == pool {
			if len(s.Values) == 0 {
				return nil
			}
			v := s.Values[len(s.Values)-1][1]
			return &v
		}
	}
	return nil
}

func computeAccessLogAnalytics(entries []LokiLogEntry) (methodDist, statusDist, durHist, memHist, cpuHist []LabelCount) {
	methods := map[string]float64{}
	statuses := map[string]float64{}
	durBuckets := map[string]float64{
		"0-100ms": 0, "100-500ms": 0, "500ms-1s": 0, "1s-5s": 0, ">5s": 0,
	}
	memBuckets := map[string]float64{
		"0-1024KB": 0, "1024-4096KB": 0, "4096-8192KB": 0, ">8192KB": 0,
	}
	cpuBuckets := map[string]float64{
		"0-1%": 0, "1-5%": 0, "5-25%": 0, ">25%": 0,
	}

	for _, entry := range entries {
		line := strings.TrimSpace(entry.Line)
		matches := phpfpmAccessLogRE.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		methods[matches[3]]++

		statuses[matches[5]]++

		if d, err := strconv.ParseFloat(matches[7], 64); err == nil {
			switch {
			case d <= 100:
				durBuckets["0-100ms"]++
			case d <= 500:
				durBuckets["100-500ms"]++
			case d <= 1000:
				durBuckets["500ms-1s"]++
			case d <= 5000:
				durBuckets["1s-5s"]++
			default:
				durBuckets[">5s"]++
			}
		}
		if m, err := strconv.ParseFloat(matches[8], 64); err == nil {
			switch {
			case m <= 1024:
				memBuckets["0-1024KB"]++
			case m <= 4096:
				memBuckets["1024-4096KB"]++
			case m <= 8192:
				memBuckets["4096-8192KB"]++
			default:
				memBuckets[">8192KB"]++
			}
		}
		if c, err := strconv.ParseFloat(matches[9], 64); err == nil {
			switch {
			case c <= 1:
				cpuBuckets["0-1%"]++
			case c <= 5:
				cpuBuckets["1-5%"]++
			case c <= 25:
				cpuBuckets["5-25%"]++
			default:
				cpuBuckets[">25%"]++
			}
		}
	}

	methodDist = topN(methods, 10)
	statusDist = topN(statuses, 20)
	for k, v := range durBuckets {
		durHist = append(durHist, LabelCount{Label: k, Count: v})
	}
	for k, v := range memBuckets {
		memHist = append(memHist, LabelCount{Label: k, Count: v})
	}
	for k, v := range cpuBuckets {
		cpuHist = append(cpuHist, LabelCount{Label: k, Count: v})
	}
	return
}

func computeErrorLogAnalytics(entries []LokiLogEntry) (topMessages, typeBreakdown []LabelCount) {
	msgs := map[string]float64{}
	types := map[string]float64{}

	for _, entry := range entries {
		ev := parsePHPFPMLogEvent(entry)
		types[ev.Category]++
		msg := strings.TrimSpace(ev.Message)
		if msg != "" {
			// Truncate very long messages to group similar errors
			if len(msg) > 120 {
				msg = msg[:120]
			}
			msgs[msg]++
		}
	}

	topMessages = topN(msgs, 10)
	typeBreakdown = topN(types, 10)
	return
}

func parseTopPHPFMURIs(series []LokiMatrixSeries) (topURIs, topErrorURIs []LabelCount) {
	uriCounts := map[string]float64{}
	uriErrors := map[string]float64{}
	for _, s := range series {
		uri := s.Metric["uri"]
		status := s.Metric["status"]
		count := lastValue(s.Values)
		if uri == "" {
			continue
		}
		uriCounts[uri] += count
		if len(status) > 0 && (status[0] == '4' || status[0] == '5') {
			uriErrors[uri] += count
		}
	}
	topURIs = topN(uriCounts, 10)
	topErrorURIs = topN(uriErrors, 10)
	return topURIs, topErrorURIs
}

// ── Log endpoint with log_file_name filtering + pool filter + slow log consolidation ──

func (s *PHPFPMDashboardService) GetLogs(
	ctx context.Context,
	tenantUUID string,
	serviceName string,
	startNs, endNs int64,
	logFile string,
	pool string,
	search string,
	method string,
	statusCode string,
	minDuration float64, hasMinDuration bool,
	maxDuration float64, hasMaxDuration bool,
	minMemory float64, hasMinMemory bool,
	maxMemory float64, hasMaxMemory bool,
	minCPU float64, hasMinCPU bool,
	maxCPU float64, hasMaxCPU bool,
	page int,
	pageSize int,
) (*PHPFPMLogsPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 100
	}

	baseSelector := phpfpmLokiSelector(serviceName)
	fullQuery := phpfpmLogQuery(baseSelector, logFile)

	entries, err := s.lokiQueryAllLogEntries(ctx, tenantUUID, fullQuery, startNs, endNs)
	if err != nil {
		return nil, fmt.Errorf("loki fetch: %w", err)
	}

	if logFile == "slow" {
		entries = consolidatePHPFPMSlowLogEntries(entries)
	}

	events := make([]PHPFPMLogEvent, 0, len(entries))
	for _, e := range entries {
		ev := parsePHPFPMLogEvent(e)
		ev.Labels = copyMetadataWithPool(e.Metadata)
		if pool != "" && ev.Pool != pool {
			continue
		}
		if method != "" && ev.Method != "" && !strings.EqualFold(ev.Method, method) {
			continue
		}
		if statusCode != "" {
			if ev.StatusCode == 0 {
				continue
			}
			sc := strconv.Itoa(ev.StatusCode)
			if len(statusCode) == 1 {
				// "2" matches 2xx, "3" matches 3xx, etc.
				if len(sc) == 0 || sc[0:1] != statusCode {
					continue
				}
			} else if sc != statusCode {
				continue
			}
		}
		if hasMinDuration && ev.DurationMs < minDuration {
			continue
		}
		if hasMaxDuration && ev.DurationMs > maxDuration {
			continue
		}
		if hasMinMemory && ev.MemoryKB < minMemory {
			continue
		}
		if hasMaxMemory && ev.MemoryKB > maxMemory {
			continue
		}
		if hasMinCPU && ev.CPU < minCPU {
			continue
		}
		if hasMaxCPU && ev.CPU > maxCPU {
			continue
		}
		events = append(events, ev)
	}

	if needle := strings.ToLower(strings.TrimSpace(search)); needle != "" {
		filtered := events[:0]
		for _, ev := range events {
			if phpfpmLogMatchesSearch(ev, needle) {
				filtered = append(filtered, ev)
			}
		}
		events = filtered
	}

	total := len(events)
	offset := (page - 1) * pageSize
	if offset >= total {
		return &PHPFPMLogsPage{
			Events:   []PHPFPMLogEvent{},
			Total:    total,
			Page:     page,
			PageSize: pageSize,
			HasMore:  false,
		}, nil
	}

	end := offset + pageSize
	if end > total {
		end = total
	}

	return &PHPFPMLogsPage{
		Events:   events[offset:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  end < total,
	}, nil
}

func (s *PHPFPMDashboardService) GetLogsMeta(
	ctx context.Context,
	tenantUUID string,
	serviceName string,
	startNs, endNs int64,
) (*PHPFPMLogsTypeMeta, error) {
	baseSelector := phpfpmLokiSelector(serviceName)

	countEntries := func(q string) (int, error) {
		entries, err := s.lokiQueryAllLogEntries(ctx, tenantUUID, q, startNs, endNs)
		if err != nil {
			return 0, err
		}
		return len(entries), nil
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var accessCount, errorCount, slowCount int
	var accessErr, errorErr, slowErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		c, err := countEntries(baseSelector + ` | log_file_name="access.log"`)
		mu.Lock()
		accessCount, accessErr = c, err
		mu.Unlock()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		c, err := countEntries(baseSelector + ` | log_file_name="error.log"`)
		mu.Lock()
		errorCount, errorErr = c, err
		mu.Unlock()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		entries, err := s.lokiQueryAllLogEntries(ctx, tenantUUID, baseSelector+` | log_file_name="slow.log"`, startNs, endNs)
		if err == nil {
			entries = consolidatePHPFPMSlowLogEntries(entries)
		}
		mu.Lock()
		if err != nil {
			slowErr = err
		} else {
			slowCount = len(entries)
		}
		mu.Unlock()
	}()
	wg.Wait()

	meta := &PHPFPMLogsTypeMeta{}
	if accessErr == nil {
		meta.AccessCount = accessCount
	}
	if errorErr == nil {
		meta.ErrorCount = errorCount
	}
	if slowErr == nil {
		meta.SlowCount = slowCount
	}
	return meta, nil
}

func phpfpmLogQuery(baseSelector, logFile string) string {
	switch logFile {
	case "access":
		return baseSelector + ` | log_file_name="access.log"`
	case "error":
		return baseSelector + ` | log_file_name="error.log"`
	case "slow":
		return baseSelector + ` | log_file_name="slow.log"`
	default:
		return baseSelector
	}
}

func copyMetadataWithPool(meta map[string]string) map[string]string {
	if len(meta) == 0 {
		return nil
	}
	out := make(map[string]string, len(meta))
	for k, v := range meta {
		out[k] = v
	}
	return out
}

func consolidatePHPFPMSlowLogEntries(entries []LokiLogEntry) []LokiLogEntry {
	if len(entries) == 0 {
		return entries
	}

	sort.Slice(entries, func(i, j int) bool {
		ti, _ := strconv.ParseInt(entries[i].TimestampNs, 10, 64)
		tj, _ := strconv.ParseInt(entries[j].TimestampNs, 10, 64)
		return ti < tj
	})

	var out []LokiLogEntry
	var cur *LokiLogEntry

	for _, e := range entries {
		line := strings.TrimSpace(e.Line)
		if isPHPSlowLogHeader(line) {
			if cur != nil {
				out = append(out, *cur)
			}
			cp := e
			cur = &cp
		} else if cur != nil {
			cur.Line += "\n" + line
			if e.TimestampNs > cur.TimestampNs {
				cur.TimestampNs = e.TimestampNs
			}
		} else {
			out = append(out, e)
		}
	}
	if cur != nil {
		out = append(out, *cur)
	}

	sortLokiLogEntriesDesc(out)
	return out
}

func isPHPSlowLogHeader(line string) bool {
	return phpfpmSlowLogHeaderRE.MatchString(line)
}

// ── helpers ──

func (s *PHPFPMDashboardService) lokiQueryAllLogEntries(
	ctx context.Context,
	tenantUUID string,
	query string,
	startNs, endNs int64,
) ([]LokiLogEntry, error) {
	const batchSize = 2000
	var all []LokiLogEntry
	seen := make(map[string]struct{}, 4096)

	cursorEnd := endNs
	for {
		batch, err := s.base.lokiQueryLogEntries(ctx, tenantUUID, query, startNs, cursorEnd, batchSize)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}

		minTs := cursorEnd
		for _, e := range batch {
			key := e.TimestampNs + "\x00" + e.Line
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			all = append(all, e)
			if ts, err2 := strconv.ParseInt(e.TimestampNs, 10, 64); err2 == nil && ts < minTs {
				minTs = ts
			}
		}

		if minTs <= startNs || minTs >= cursorEnd {
			break
		}
		cursorEnd = minTs - 1
		if len(batch) < batchSize {
			break
		}
	}

	sort.SliceStable(all, func(i, j int) bool {
		ti, _ := strconv.ParseInt(all[i].TimestampNs, 10, 64)
		tj, _ := strconv.ParseInt(all[j].TimestampNs, 10, 64)
		return ti < tj
	})

	sortLokiLogEntriesDesc(all)
	return all, nil
}

func (s *PHPFPMDashboardService) countPHPFPMLogCategories(
	ctx context.Context,
	tenantUUID string,
	serviceName string,
	startNs, endNs int64,
) PHPFPMLogSummary {
	baseSelector := phpfpmLokiSelector(serviceName)
	errorBase := baseSelector + ` | log_file_name="error.log"`
	entries, err := s.lokiQueryAllLogEntries(ctx, tenantUUID, errorBase, startNs, endNs)
	if err != nil {
		return PHPFPMLogSummary{}
	}

	var summary PHPFPMLogSummary
	for _, entry := range entries {
		ev := parsePHPFPMLogEvent(entry)
		switch ev.Category {
		case "warning":
			summary.Warnings++
		case "error":
			summary.Errors++
		case "notice":
			summary.Notices++
		case "slow_request":
			summary.SlowRequests++
		case "startup":
			summary.Startups++
		}
	}
	return summary
}

func phpfpmLokiSelector(serviceName string) string {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return `{service_name=~"phpfpm.*"}`
	}
	return fmt.Sprintf(`{service_name="%s"}`, lokiLabelValue(serviceName))
}

var phpfpmErrorLogRE = regexp.MustCompile(`^\[(.+?)\]\s+(WARNING|ERROR|NOTICE|ALERT):(?:\s*\[pool\s+(\S+)\])?\s*(.*)$`)

func parsePHPFPMLogEvent(entry LokiLogEntry) PHPFPMLogEvent {
	event := PHPFPMLogEvent{
		Timestamp: entry.TimestampNs,
		Severity:  "info",
		Category:  "event",
		Message:   entry.Line,
		Labels:    map[string]string{},
	}

	for key, value := range entry.Metadata {
		if strings.TrimSpace(value) != "" {
			event.Labels[key] = value
		}
	}

	if ts, err := strconv.ParseInt(entry.TimestampNs, 10, 64); err == nil {
		event.Timestamp = time.Unix(0, ts).Format(time.RFC3339)
	}

	line := strings.TrimSpace(entry.Line)

	if matches := phpfpmErrorLogRE.FindStringSubmatch(line); len(matches) == 5 {
		severity := matches[2]
		pool := matches[3]
		message := strings.TrimSpace(matches[4])

		event.Pool = pool
		event.Message = message

		switch severity {
		case "WARNING":
			event.Severity = "WARNING"
			if strings.Contains(message, "executing too slow") {
				event.Category = "slow_request"
			} else {
				event.Category = "warning"
			}
		case "ERROR", "ALERT":
			event.Severity = "ERROR"
			event.Category = "error"
		case "NOTICE":
			event.Severity = "NOTICE"
			if strings.Contains(message, "systemd monitor") || strings.Contains(message, "starting") {
				event.Category = "startup"
			} else if strings.Contains(message, "stopped for tracing") || strings.Contains(message, "about to trace") || strings.Contains(message, "finished trace") {
				event.Category = "slow_request"
			} else {
				event.Category = "notice"
			}
		}
	} else if phpfpmSlowLogHeaderRE.MatchString(line) {
		event.Category = "slow_request"
		event.Severity = "WARNING"
		event.Message = extractSlowLogMessage(line)
		if pool := extractSlowLogPool(line); pool != "" {
			event.Pool = pool
		}
	} else if matches := phpfpmAccessLogRE.FindStringSubmatch(line); matches != nil {
		event.Category = "access"
		event.Severity = "INFO"
		event.Pool = matches[1]
		event.Method = matches[3]
		event.URI = matches[4]
		event.StatusCode = safeAtoi(matches[5])
		event.Script = matches[6]
		if d, err := strconv.ParseFloat(matches[7], 64); err == nil {
			event.DurationMs = d
		}
		if m, err := strconv.ParseFloat(matches[8], 64); err == nil {
			event.MemoryKB = m
		}
		if c, err := strconv.ParseFloat(matches[9], 64); err == nil {
			event.CPU = c
		}
		event.Message = fmt.Sprintf("%s %s → %d (%sms, %sKB, %s%%)",
			matches[3], matches[4], safeAtoi(matches[5]),
			matches[7], matches[8], matches[9])
	}

	if len(event.Labels) == 0 {
		event.Labels = nil
	}
	return event
}

func extractSlowLogPool(line string) string {
	m := phpfpmSlowLogPoolRE.FindStringSubmatch(line)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

func extractSlowLogMessage(line string) string {
	parts := strings.SplitN(line, "]", 3)
	if len(parts) >= 3 {
		return strings.TrimSpace(parts[2])
	}
	return line
}

func safeAtoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

func phpfpmLogMatchesSearch(ev PHPFPMLogEvent, needle string) bool {
	for _, field := range []string{ev.Timestamp, ev.Severity, ev.Pool, ev.Category, ev.Message} {
		if strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}
	return false
}

func computePHPFPMHealthScore(saturation, queueUtil, slowReqs, childrenRate *float64) *float64 {
	score := 100.0
	if saturation != nil {
		switch {
		case *saturation > 85:
			score -= 30
		case *saturation > 70:
			score -= 15
		case *saturation > 50:
			score -= 5
		}
	}
	if queueUtil != nil && *queueUtil > 0 {
		switch {
		case *queueUtil > 50:
			score -= 25
		case *queueUtil > 10:
			score -= 10
		default:
			score -= 5
		}
	}
	if slowReqs != nil && *slowReqs > 0 {
		score -= 20
	}
	if childrenRate != nil && *childrenRate > 0 {
		score -= 25
	}
	if score < 0 {
		score = 0
	}
	return &score
}

func parsePHPFPMAccessLogs(entries []LokiLogEntry) (topSlow, topMem []ScriptStat, p50, p95, p99 *float64) {
	type scriptAccum struct {
		totalDur float64
		totalMem float64
		count    float64
	}
	scripts := map[string]*scriptAccum{}
	durations := []float64{}

	for _, entry := range entries {
		line := strings.TrimSpace(entry.Line)
		matches := phpfpmAccessLogRE.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		script := matches[6]
		dur, _ := strconv.ParseFloat(matches[7], 64)
		mem, _ := strconv.ParseFloat(matches[8], 64)

		if dur > 0 && script != "" && !strings.HasPrefix(script, "-") {
			if _, ok := scripts[script]; !ok {
				scripts[script] = &scriptAccum{}
			}
			scripts[script].totalDur += dur
			scripts[script].totalMem += mem
			scripts[script].count++
			durations = append(durations, dur)
		}
	}

	slowList := make([]ScriptStat, 0, len(scripts))
	memList := make([]ScriptStat, 0, len(scripts))
	for script, acc := range scripts {
		slowList = append(slowList, ScriptStat{
			Script:      script,
			AvgDuration: acc.totalDur / acc.count,
			AvgMemory:   acc.totalMem / acc.count,
			Count:       acc.count,
		})
		memList = append(memList, ScriptStat{
			Script:      script,
			AvgDuration: acc.totalDur / acc.count,
			AvgMemory:   acc.totalMem / acc.count,
			Count:       acc.count,
		})
	}
	sort.Slice(slowList, func(i, j int) bool { return slowList[i].AvgDuration > slowList[j].AvgDuration })
	sort.Slice(memList, func(i, j int) bool { return memList[i].AvgMemory > memList[j].AvgMemory })
	if len(slowList) > 5 {
		slowList = slowList[:5]
	}
	if len(memList) > 5 {
		memList = memList[:5]
	}

	if len(durations) > 0 {
		sort.Float64s(durations)
		p50v := durations[int(float64(len(durations))*0.50)]
		p95v := durations[int(float64(len(durations))*0.95)]
		p99v := durations[int(float64(len(durations))*0.99)]
		p50 = &p50v
		p95 = &p95v
		p99 = &p99v
	}

	return slowList, memList, p50, p95, p99
}

func parseProcessStats(series []VMSeries, statType string) []ProcessStat {
	stats := make([]ProcessStat, 0, len(series))
	for _, s := range series {
		if len(s.Values) == 0 {
			continue
		}
		val := s.Values[len(s.Values)-1][1]
		ps := ProcessStat{
			PID:   s.Metric["pid"],
			State: s.Metric["state"],
		}
		switch statType {
		case "duration":
			ps.Duration = val
		case "memory":
			ps.Memory = val / 1024
		}
		stats = append(stats, ps)
	}
	sort.Slice(stats, func(i, j int) bool {
		switch statType {
		case "duration":
			return stats[i].Duration > stats[j].Duration
		case "memory":
			return stats[i].Memory > stats[j].Memory
		}
		return false
	})
	if len(stats) > 10 {
		stats = stats[:10]
	}
	return stats
}

func mergePHPFPMLogEvents(limit int, groups ...[]LokiLogEntry) []PHPFPMLogEvent {
	seen := map[string]struct{}{}
	entries := []LokiLogEntry{}
	for _, group := range groups {
		for _, entry := range group {
			key := entry.TimestampNs + "\x00" + entry.Line
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			entries = append(entries, entry)
		}
	}
	sortLokiLogEntriesDesc(entries)
	if len(entries) > limit {
		entries = entries[:limit]
	}
	events := make([]PHPFPMLogEvent, 0, len(entries))
	for _, entry := range entries {
		events = append(events, parsePHPFPMLogEvent(entry))
	}
	return events
}
