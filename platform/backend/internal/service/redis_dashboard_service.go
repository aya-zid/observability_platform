// internal/service/redis_dashboard_service.go
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

type RedisDashboardService struct {
	base  *NginxDashboardService
	redis *redis.Client
}

type RedisDashboardSummary struct {
	Up                     *float64 `json:"up,omitempty"`
	ScrapeError            *float64 `json:"scrape_error,omitempty"`
	MemoryUsedBytes        *float64 `json:"memory_used_bytes,omitempty"`
	MemoryRssBytes         *float64 `json:"memory_rss_bytes,omitempty"`
	MemoryPeakBytes        *float64 `json:"memory_peak_bytes,omitempty"`
	MemoryFragRatio        *float64 `json:"memory_frag_ratio,omitempty"`
	MaxMemoryBytes         *float64 `json:"max_memory_bytes,omitempty"`
	SystemMemoryBytes      *float64 `json:"system_memory_bytes,omitempty"`
	MemoryUsagePct         *float64 `json:"memory_usage_pct,omitempty"`
	ConnectedClients       *float64 `json:"connected_clients,omitempty"`
	MaxClients             *float64 `json:"max_clients,omitempty"`
	ClientSaturationPct    *float64 `json:"client_saturation_pct,omitempty"`
	BlockedClients         *float64 `json:"blocked_clients,omitempty"`
	CommandsPerSec         *float64 `json:"commands_per_sec,omitempty"`
	CacheHitRatioPct       *float64 `json:"cache_hit_ratio_pct,omitempty"`
	KeysTotal              *float64 `json:"keys_total,omitempty"`
	EvictRate              *float64 `json:"evict_rate,omitempty"`
	ExpireRate             *float64 `json:"expire_rate,omitempty"`
	ErrorRepliesPerSec     *float64 `json:"error_replies_per_sec,omitempty"`
	UnexpectedErrorsPerSec *float64 `json:"unexpected_errors_per_sec,omitempty"`
	CommandFailuresPerSec  *float64 `json:"command_failures_per_sec,omitempty"`
	CommandRejectsPerSec   *float64 `json:"command_rejects_per_sec,omitempty"`
	NetInputBytesPerSec    *float64 `json:"net_input_bytes_per_sec,omitempty"`
	NetOutputBytesPerSec   *float64 `json:"net_output_bytes_per_sec,omitempty"`
	UptimeSeconds          *float64 `json:"uptime_seconds,omitempty"`
	SlowLogLength          *float64 `json:"slow_log_length,omitempty"`
	RejectedConnsPerSec    *float64 `json:"rejected_conns_per_sec,omitempty"`
	CpuSysPerSec           *float64 `json:"cpu_sys_per_sec,omitempty"`
	CpuUserPerSec          *float64 `json:"cpu_user_per_sec,omitempty"`
	ConnectedSlaves        *float64 `json:"connected_slaves,omitempty"`
	LatestForkSeconds      *float64 `json:"latest_fork_seconds,omitempty"`
	RDBChangesSinceSave    *float64 `json:"rdb_changes_since_last_save,omitempty"`
	RDBSaveInProgress      *float64 `json:"rdb_save_in_progress,omitempty"`
	AOFEnabled             *float64 `json:"aof_enabled,omitempty"`
	AOFRewriteInProgress   *float64 `json:"aof_rewrite_in_progress,omitempty"`

	MemoryTimeseries      []TimePoint `json:"memory_timeseries"`
	ClientsTimeseries     []TimePoint `json:"clients_timeseries"`
	CommandsTimeseries    []TimePoint `json:"commands_timeseries"`
	CacheTimeseries       []TimePoint `json:"cache_timeseries"`
	ErrorTimeseries       []TimePoint `json:"error_timeseries"`
	NetTimeseries         []TimePoint `json:"net_timeseries"`
	CpuTimeseries         []TimePoint `json:"cpu_timeseries"`
	KeysTimeseries        []TimePoint `json:"keys_timeseries"`
	EvictExpireTimeseries []TimePoint `json:"evict_expire_timeseries"`
	FragTimeseries        []TimePoint `json:"frag_timeseries"`
	ConnRateTimeseries    []TimePoint `json:"conn_rate_timeseries"`
	PersistenceTimeseries []TimePoint `json:"persistence_timeseries"`

	KeysByDb     []LabelCount `json:"keys_by_db"`
	CommandsTop  []LabelCount `json:"commands_top"`
	CommandRates []LabelCount `json:"command_rates"`

	LogSummary RedisLogSummary `json:"log_summary"`
	RecentLogs []RedisLogEvent `json:"recent_logs"`
}

type RedisLogSummary struct {
	Warnings     float64 `json:"warnings"`
	Errors       float64 `json:"errors"`
	Shutdowns    float64 `json:"shutdowns"`
	Startups     float64 `json:"startups"`
	Persistence  float64 `json:"persistence"`
	AuthFailures float64 `json:"auth_failures"`
	Security     float64 `json:"security"`
}

type RedisLogEvent struct {
	Timestamp string            `json:"timestamp"`
	Severity  string            `json:"severity"`
	Role      string            `json:"role"`
	PID       string            `json:"pid"`
	Category  string            `json:"category"`
	Message   string            `json:"message"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type RedisLogsPage struct {
	Events   []RedisLogEvent `json:"events"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	HasMore  bool            `json:"has_more"`
}

func NewRedisDashboardService(cfg *config.Config, redisClient *redis.Client) *RedisDashboardService {
	return &RedisDashboardService{
		base:  NewNginxDashboardService(cfg, redisClient),
		redis: redisClient,
	}
}

func (s *RedisDashboardService) GetDashboardSummary(
	ctx context.Context,
	tenantUUID string,
	tenantID uint,
	serviceName string,
	startNs, endNs int64,
	step string,
) (*RedisDashboardSummary, error) {
	startSec := startNs / int64(1e9)
	endSec := endNs / int64(1e9)
	cacheKey := fmt.Sprintf("redis:dash:%s:%s:%d:%d:%s", tenantUUID, serviceName, startSec, endSec, step)

	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var result RedisDashboardSummary
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

func (s *RedisDashboardService) fetchDashboard(
	ctx context.Context,
	tenantUUID string,
	tenantID uint,
	serviceName string,
	startNs, endNs int64,
	step string,
) (*RedisDashboardSummary, error) {
	startSec := startNs / int64(1e9)
	endSec := endNs / int64(1e9)
	rateWindow := "5m"

	type vR struct {
		data []VMSeries
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
	logQuery := func(q string, limit int) logR {
		entries, err := s.base.lokiQueryLogEntries(ctx, tenantUUID, q, startNs, endNs, limit)
		return logR{entries: entries, err: err}
	}

	results := make(map[string]vR)
	logResults := make(map[string]logR)
	var resultsMu sync.Mutex
	var wg sync.WaitGroup

	queries := map[string]string{
		"up":                "redis_up",
		"scrape_error":      "redis_exporter_last_scrape_error",
		"memory_used":       "redis_memory_used_bytes",
		"memory_rss":        "redis_memory_used_rss_bytes",
		"memory_peak":       "redis_memory_used_peak_bytes",
		"frag_ratio":        "redis_mem_fragmentation_ratio",
		"max_memory":        "redis_memory_max_bytes",
		"system_memory":     "redis_total_system_memory_bytes",
		"clients":           "redis_connected_clients",
		"max_clients":       "redis_config_maxclients",
		"blocked_clients":   "redis_blocked_clients",
		"commands_rate":     "rate(redis_commands_processed_total[" + rateWindow + "])",
		"commands_total":    "redis_commands_processed_total",
		"cmd_rate_by_cmd":   "sum by (cmd) (rate(redis_commands_total[" + rateWindow + "]))",
		"keys_hits":         "rate(redis_keyspace_hits_total[" + rateWindow + "]) or vector(0)",
		"keys_misses":       "rate(redis_keyspace_misses_total[" + rateWindow + "]) or vector(0)",
		"hits_total":        "redis_keyspace_hits_total",
		"misses_total":      "redis_keyspace_misses_total",
		"keys_total":        "sum(redis_db_keys) or vector(0)",
		"keys_by_db":        "redis_db_keys",
		"evict_rate":        "rate(redis_evicted_keys_total[" + rateWindow + "]) or vector(0)",
		"expire_rate":       "rate(redis_expired_keys_total[" + rateWindow + "]) or vector(0)",
		"error_replies":     "rate(redis_error_replies_total[" + rateWindow + "]) or vector(0)",
		"unexpected_errors": "rate(redis_unexpected_error_replies_total[" + rateWindow + "]) or vector(0)",
		"cmd_failures":      "sum(rate(redis_commands_failed_calls_total[" + rateWindow + "])) or vector(0)",
		"cmd_rejects":       "sum(rate(redis_commands_rejected_calls_total[" + rateWindow + "])) or vector(0)",
		"net_input":         "rate(redis_net_input_bytes_total[" + rateWindow + "]) or vector(0)",
		"net_output":        "rate(redis_net_output_bytes_total[" + rateWindow + "]) or vector(0)",
		"cpu_sys":           "rate(redis_cpu_sys_seconds_total[" + rateWindow + "]) or vector(0)",
		"cpu_user":          "rate(redis_cpu_user_seconds_total[" + rateWindow + "]) or vector(0)",
		"uptime":            "redis_uptime_in_seconds",
		"slowlog_len":       "redis_slowlog_length",
		"rejected_conns":    "rate(redis_rejected_connections_total[" + rateWindow + "]) or vector(0)",
		"conn_received":     "rate(redis_connections_received_total[" + rateWindow + "]) or vector(0)",
		"connected_slaves":  "redis_connected_slaves",
		"latest_fork":       "redis_latest_fork_seconds",
		"rdb_changes":       "redis_rdb_changes_since_last_save",
		"rdb_save":          "redis_rdb_bgsave_in_progress",
		"aof_enabled":       "redis_aof_enabled",
		"aof_rewrite":       "redis_aof_rewrite_in_progress",
		"commands_duration": "redis_commands_duration_seconds_total",
	}

	for name, q := range queries {
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

	baseSelector := redisLokiSelector(serviceName)
	const dashboardPreviewLimit = 20

	logQueries := map[string]string{
		"warnings":  baseSelector + ` |~ "^#"`,
		"startups":  baseSelector + ` |~ "Redis is starting|Ready to accept connections|Server initialized"`,
		"shutdowns": baseSelector + ` |~ "User requested shutdown|bye bye"`,
		"persist":   baseSelector + ` |~ "DB saved|Loading RDB|Saving the final RDB"`,
		"auth":      baseSelector + ` |~ "(?i)(AUTH failed|authentication failed|invalid password|WRONGPASS|NOAUTH)"`,
		"security":  baseSelector + ` |~ "(?i)(Possible SECURITY ATTACK|protected mode|cross protocol scripting|DENIED)"`,
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

	memoryUsage := latestVMValue(results["memory_used"].data)
	sysMem := latestVMValue(results["system_memory"].data)
	var memPct *float64
	if memoryUsage != nil && sysMem != nil && *sysMem > 0 {
		v := 100 * (*memoryUsage) / (*sysMem)
		memPct = &v
	}
	maxMemory := latestVMValue(results["max_memory"].data)
	if memoryUsage != nil && maxMemory != nil && *maxMemory > 0 {
		v := 100 * (*memoryUsage) / (*maxMemory)
		memPct = &v
	}

	clients := latestVMValue(results["clients"].data)
	maxClients := latestVMValue(results["max_clients"].data)
	var clientPct *float64
	if clients != nil && maxClients != nil && *maxClients > 0 {
		v := 100 * (*clients) / (*maxClients)
		clientPct = &v
	}

	hits := lastValueFromSeries(results["hits_total"].data)
	misses := lastValueFromSeries(results["misses_total"].data)
	var hitRatio *float64
	if hits+misses > 0 {
		v := 100 * hits / (hits + misses)
		hitRatio = &v
	}

	summary := &RedisDashboardSummary{
		Up:                     latestVMValue(results["up"].data),
		ScrapeError:            latestVMValue(results["scrape_error"].data),
		MemoryUsedBytes:        memoryUsage,
		MemoryRssBytes:         latestVMValue(results["memory_rss"].data),
		MemoryPeakBytes:        latestVMValue(results["memory_peak"].data),
		MemoryFragRatio:        latestVMValue(results["frag_ratio"].data),
		MaxMemoryBytes:         maxMemory,
		SystemMemoryBytes:      sysMem,
		MemoryUsagePct:         memPct,
		ConnectedClients:       clients,
		MaxClients:             maxClients,
		ClientSaturationPct:    clientPct,
		BlockedClients:         latestVMValue(results["blocked_clients"].data),
		CommandsPerSec:         latestVMValue(results["commands_rate"].data),
		CacheHitRatioPct:       hitRatio,
		KeysTotal:              latestVMValue(results["keys_total"].data),
		EvictRate:              latestVMValue(results["evict_rate"].data),
		ExpireRate:             latestVMValue(results["expire_rate"].data),
		ErrorRepliesPerSec:     latestVMValue(results["error_replies"].data),
		UnexpectedErrorsPerSec: latestVMValue(results["unexpected_errors"].data),
		CommandFailuresPerSec:  latestVMValue(results["cmd_failures"].data),
		CommandRejectsPerSec:   latestVMValue(results["cmd_rejects"].data),
		NetInputBytesPerSec:    latestVMValue(results["net_input"].data),
		NetOutputBytesPerSec:   latestVMValue(results["net_output"].data),
		UptimeSeconds:          latestVMValue(results["uptime"].data),
		SlowLogLength:          latestVMValue(results["slowlog_len"].data),
		RejectedConnsPerSec:    latestVMValue(results["rejected_conns"].data),
		CpuSysPerSec:           latestVMValue(results["cpu_sys"].data),
		CpuUserPerSec:          latestVMValue(results["cpu_user"].data),
		ConnectedSlaves:        latestVMValue(results["connected_slaves"].data),
		LatestForkSeconds:      latestVMValue(results["latest_fork"].data),
		RDBChangesSinceSave:    latestVMValue(results["rdb_changes"].data),
		RDBSaveInProgress:      latestVMValue(results["rdb_save"].data),
		AOFEnabled:             latestVMValue(results["aof_enabled"].data),
		AOFRewriteInProgress:   latestVMValue(results["aof_rewrite"].data),
	}

	summary.MemoryTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"used": results["memory_used"].data,
		"rss":  results["memory_rss"].data,
		"peak": results["memory_peak"].data,
	})
	summary.ClientsTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"clients": results["clients"].data,
		"blocked": results["blocked_clients"].data,
	})
	summary.CommandsTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"commands": results["commands_rate"].data,
	})
	summary.CacheTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"hits":   results["keys_hits"].data,
		"misses": results["keys_misses"].data,
	})
	summary.ErrorTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"errors":     results["error_replies"].data,
		"unexpected": results["unexpected_errors"].data,
		"failed":     results["cmd_failures"].data,
		"rejected":   results["cmd_rejects"].data,
	})
	summary.NetTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"input":  results["net_input"].data,
		"output": results["net_output"].data,
	})
	summary.CpuTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"user": results["cpu_user"].data,
		"sys":  results["cpu_sys"].data,
	})
	summary.KeysTimeseries = singleVMSeries(results["keys_total"].data, "keys")
	summary.EvictExpireTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"evicted": results["evict_rate"].data,
		"expired": results["expire_rate"].data,
	})
	summary.FragTimeseries = singleVMSeries(results["frag_ratio"].data, "ratio")
	summary.ConnRateTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"received": results["conn_received"].data,
		"rejected": results["rejected_conns"].data,
	})
	summary.PersistenceTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"rdb_changes": results["rdb_changes"].data,
		"fork_sec":    results["latest_fork"].data,
	})

	summary.KeysByDb = latestLabelCounts(results["keys_by_db"].data, "db", 16)
	summary.CommandsTop = topNCommands(results["commands_duration"].data, 10)
	summary.CommandRates = latestLabelCounts(results["cmd_rate_by_cmd"].data, "cmd", 10)

	summary.LogSummary = s.countRedisLogCategories(ctx, tenantUUID, serviceName, startNs, endNs)

	summary.RecentLogs = mergeRedisLogEvents(dashboardPreviewLimit,
		logResults["warnings"].entries,
		logResults["startups"].entries,
		logResults["shutdowns"].entries,
		logResults["persist"].entries,
		logResults["auth"].entries,
		logResults["security"].entries,
	)

	return summary, nil
}

func (s *RedisDashboardService) GetLogs(
	ctx context.Context,
	tenantUUID string,
	serviceName string,
	startNs, endNs int64,
	category string,
	search string,
	page int,
	pageSize int,
) (*RedisLogsPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 100
	}

	baseSelector := redisLokiSelector(serviceName)
	lokiQuery := redisLogsLokiQuery(baseSelector, category)

	entries, err := s.lokiQueryAllLogEntries(ctx, tenantUUID, lokiQuery, startNs, endNs)
	if err != nil {
		return nil, fmt.Errorf("loki fetch: %w", err)
	}

	events := make([]RedisLogEvent, 0, len(entries))
	for _, e := range entries {
		events = append(events, parseRedisLogEvent(e))
	}

	if category != "" && category != "all" {
		filtered := events[:0]
		for _, ev := range events {
			if ev.Category == category {
				filtered = append(filtered, ev)
			}
		}
		events = filtered
	}

	if needle := strings.ToLower(strings.TrimSpace(search)); needle != "" {
		filtered := events[:0]
		for _, ev := range events {
			if redisLogMatchesSearch(ev, needle) {
				filtered = append(filtered, ev)
			}
		}
		events = filtered
	}

	total := len(events)
	offset := (page - 1) * pageSize
	if offset >= total {
		return &RedisLogsPage{
			Events:   []RedisLogEvent{},
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

	return &RedisLogsPage{
		Events:   events[offset:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  end < total,
	}, nil
}

func (s *RedisDashboardService) lokiQueryAllLogEntries(
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

func (s *RedisDashboardService) countRedisLogCategories(
	ctx context.Context,
	tenantUUID string,
	serviceName string,
	startNs, endNs int64,
) RedisLogSummary {
	entries, err := s.lokiQueryAllLogEntries(ctx, tenantUUID, redisLokiSelector(serviceName), startNs, endNs)
	if err != nil {
		return RedisLogSummary{}
	}

	var summary RedisLogSummary
	for _, entry := range entries {
		ev := parseRedisLogEvent(entry)
		switch ev.Category {
		case "warning":
			summary.Warnings++
		case "error":
			summary.Errors++
		case "shutdown":
			summary.Shutdowns++
		case "startup":
			summary.Startups++
		case "persistence":
			summary.Persistence++
		case "auth":
			summary.AuthFailures++
		case "security":
			summary.Security++
		}
	}
	return summary
}

func redisLokiSelector(serviceName string) string {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return `{service_name=~"redis.*"}`
	}
	return fmt.Sprintf(`{service_name="%s"}`, lokiLabelValue(serviceName))
}

func redisLogsLokiQuery(baseSelector, category string) string {
	filters := map[string]string{
		"warning":     `|~ "^#"`,
		"startup":     `|~ "Redis is starting|Ready to accept connections|Server initialized"`,
		"shutdown":    `|~ "User requested shutdown|bye bye"`,
		"persistence": `|~ "DB saved|Loading RDB|Saving the final RDB"`,
		"auth":        `|~ "(?i)(AUTH failed|authentication failed|invalid password|WRONGPASS|NOAUTH)"`,
		"security":    `|~ "(?i)(Possible SECURITY ATTACK|protected mode|cross protocol scripting|DENIED)"`,
		"error":       `|~ "^#"`,
	}
	if f, ok := filters[category]; ok {
		return baseSelector + " " + f
	}
	return baseSelector
}

func parseRedisLogEvent(entry LokiLogEntry) RedisLogEvent {
	event := RedisLogEvent{
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

	redisPrefixRE := regexp.MustCompile(`^(\d+):([MCS])\s+(\d{1,2} \w+ \d{4} [0-9:.]+)\s+([*#.])\s+(.*)$`)
	if matches := redisPrefixRE.FindStringSubmatch(line); len(matches) == 6 {
		event.PID = matches[1]
		event.Role = matches[2]
		event.Message = strings.TrimSpace(matches[5])
	}

	if containsRedisAuthFailure(line) {
		event.Category = "auth"
		event.Severity = "WARNING"
	} else if containsRedisSecurityEvent(line) {
		event.Category = "security"
		event.Severity = "WARNING"
	} else if strings.Contains(line, "# ") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "warning") || strings.Contains(lower, "oom") || strings.Contains(lower, "overcommit") {
			event.Category = "warning"
			event.Severity = "WARNING"
		} else {
			event.Category = "error"
			event.Severity = "ERROR"
		}
	} else if strings.Contains(line, "* Redis is starting") {
		event.Category = "startup"
		event.Severity = "INFO"
	} else if strings.Contains(line, "* Ready to accept connections") {
		event.Category = "startup"
		event.Severity = "INFO"
	} else if strings.Contains(line, "User requested shutdown") || strings.Contains(line, "bye bye") {
		event.Category = "shutdown"
		event.Severity = "INFO"
	} else if strings.Contains(line, "DB saved") || strings.Contains(line, "Loading RDB") || strings.Contains(line, "Saving the final RDB") {
		event.Category = "persistence"
		event.Severity = "INFO"
	} else {
		event.Category = "event"
	}

	if len(event.Labels) == 0 {
		event.Labels = nil
	}
	return event
}

func redisLogMatchesSearch(ev RedisLogEvent, needle string) bool {
	for _, field := range []string{ev.Timestamp, ev.Severity, ev.Role, ev.PID, ev.Category, ev.Message} {
		if strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}
	return false
}

func containsRedisAuthFailure(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "auth failed") ||
		strings.Contains(lower, "authentication failed") ||
		strings.Contains(lower, "invalid password") ||
		strings.Contains(lower, "wrongpass") ||
		strings.Contains(lower, "noauth")
}

func containsRedisSecurityEvent(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "possible security attack") ||
		strings.Contains(lower, "protected mode") ||
		strings.Contains(lower, "cross protocol scripting") ||
		strings.Contains(lower, "denied")
}

func mergeRedisLogEvents(limit int, groups ...[]LokiLogEntry) []RedisLogEvent {
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
	events := make([]RedisLogEvent, 0, len(entries))
	for _, entry := range entries {
		events = append(events, parseRedisLogEvent(entry))
	}
	return events
}

func lastValueFromSeries(series []VMSeries) float64 {
	for _, s := range series {
		if v := lastValue(s.Values); v > 0 {
			return v
		}
	}
	return 0
}

func topNCommands(series []VMSeries, n int) []LabelCount {
	counts := map[string]float64{}
	for _, item := range series {
		cmd := item.Metric["cmd"]
		if cmd == "" {
			continue
		}
		counts[cmd] += lastValue(item.Values)
	}
	return topN(counts, n)
}
