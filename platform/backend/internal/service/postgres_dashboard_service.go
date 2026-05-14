// internal/service/postgres_dashboard_service.go
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

type PostgresDashboardService struct {
	base  *NginxDashboardService
	redis *redis.Client
}

// ── Structs ──────────────────────────────────────────────────────────────────

type PostgresDashboardSummary struct {
	Up                      *float64 `json:"up,omitempty"`
	ScrapeError             *float64 `json:"scrape_error,omitempty"`
	MaxConnections          *float64 `json:"max_connections,omitempty"`
	ConnectionSaturationPct *float64 `json:"connection_saturation_pct,omitempty"`
	CacheHitRatioPct        *float64 `json:"cache_hit_ratio_pct,omitempty"`
	DatabaseSizeBytes       *float64 `json:"database_size_bytes,omitempty"`
	WalSizeBytes            *float64 `json:"wal_size_bytes,omitempty"`
	ReplicationLagSeconds   *float64 `json:"replication_lag_seconds,omitempty"`
	MaxTxDurationSeconds    *float64 `json:"max_tx_duration_seconds,omitempty"`
	RollbackRatePct         *float64 `json:"rollback_rate_pct,omitempty"`
	DeadlocksPerMin         *float64 `json:"deadlocks_per_min,omitempty"`
	ArchiveFailed           *float64 `json:"archive_failed,omitempty"`
	SharedBuffersBytes      *float64 `json:"shared_buffers_bytes,omitempty"`
	WorkMemBytes            *float64 `json:"work_mem_bytes,omitempty"`
	EffectiveCacheSizeBytes *float64 `json:"effective_cache_size_bytes,omitempty"`

	ConnectionsTimeseries []TimePoint `json:"connections_timeseries"`
	ActivityTimeseries    []TimePoint `json:"activity_timeseries"`
	TransactionTimeseries []TimePoint `json:"transaction_timeseries"`
	CacheTimeseries       []TimePoint `json:"cache_timeseries"`
	TempTimeseries        []TimePoint `json:"temp_timeseries"`
	LocksTimeseries       []TimePoint `json:"locks_timeseries"`
	SizeTimeseries        []TimePoint `json:"size_timeseries"`
	WalTimeseries         []TimePoint `json:"wal_timeseries"`
	RowOpsTimeseries      []TimePoint `json:"row_ops_timeseries"`
	SeqVsIndexTimeseries  []TimePoint `json:"seq_vs_index_timeseries"`
	BgwriterTimeseries    []TimePoint `json:"bgwriter_timeseries"`
	ArchiverTimeseries    []TimePoint `json:"archiver_timeseries"`
	ConnByDbTimeseries    []TimePoint `json:"conn_by_db_timeseries"`

	DatabaseSizes  []LabelCount             `json:"database_sizes"`
	LocksByMode    []LabelCount             `json:"locks_by_mode"`
	ConnByDatabase []LabelCount             `json:"conn_by_database"`
	DatabaseHealth []PostgresDatabaseHealth `json:"database_health"`

	LogSummary    PostgresLogSummary `json:"log_summary"`
	RecentLogs    []PostgresLogEvent `json:"recent_logs"`
	SlowQueryLogs []PostgresLogEvent `json:"slow_query_logs"`
}

type PostgresDatabaseHealth struct {
	Database           string  `json:"database"`
	SizeBytes          float64 `json:"size_bytes"`
	Connections        float64 `json:"connections"`
	CacheHitRatioPct   float64 `json:"cache_hit_ratio_pct"`
	TempFilesPerSecond float64 `json:"temp_files_per_second"`
	TempBytesPerSecond float64 `json:"temp_bytes_per_second"`
	RollbackRatePct    float64 `json:"rollback_rate_pct"`
}

type PostgresLogSummary struct {
	SlowQueries    float64 `json:"slow_queries"`
	Errors         float64 `json:"errors"`
	AuthFailures   float64 `json:"auth_failures"`
	Connections    float64 `json:"connections"`
	Disconnections float64 `json:"disconnections"`
	LockWaits      float64 `json:"lock_waits"`
	TempFiles      float64 `json:"temp_files"`
	SchemaChanges  float64 `json:"schema_changes"`
	Checkpoints    float64 `json:"checkpoints"`
}

type PostgresLogEvent struct {
	Timestamp  string            `json:"timestamp"`
	Severity   string            `json:"severity"`
	User       string            `json:"user"`
	Database   string            `json:"database"`
	Client     string            `json:"client"`
	App        string            `json:"app"`
	Category   string            `json:"category"`
	DurationMs *float64          `json:"duration_ms,omitempty"`
	Message    string            `json:"message"`
	Labels     map[string]string `json:"labels,omitempty"`
}

type PostgresLogsPage struct {
	Events   []PostgresLogEvent `json:"events"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	HasMore  bool               `json:"has_more"`
}

type PostgresConfigSnapshot struct {
	GeneratedAt string                `json:"generated_at"`
	Groups      []PostgresConfigGroup `json:"groups"`
}

type PostgresConfigGroup struct {
	Key      string                  `json:"key"`
	Title    string                  `json:"title"`
	Settings []PostgresConfigSetting `json:"settings"`
}

type PostgresConfigSetting struct {
	Name        string   `json:"name"`
	Value       *float64 `json:"value,omitempty"`
	Unit        string   `json:"unit"`
	Description string   `json:"description"`
	Warn        bool     `json:"warn"`
}

type postgresLogHeader struct {
	Timestamp string
	PID       string
	User      string
	Database  string
	Client    string
	App       string
	Message   string
}

// ── Constructor ───────────────────────────────────────────────────────────────

func NewPostgresDashboardService(cfg *config.Config, redisClient *redis.Client) *PostgresDashboardService {
	return &PostgresDashboardService{
		base:  NewNginxDashboardService(cfg, redisClient),
		redis: redisClient,
	}
}

func (s *PostgresDashboardService) GetConfigSnapshot(
	ctx context.Context,
	tenantID uint,
) (*PostgresConfigSnapshot, error) {
	now := time.Now()
	result := &PostgresConfigSnapshot{
		GeneratedAt: now.Format(time.RFC3339),
		Groups:      postgresConfigCatalog(),
	}

	for groupIndex := range result.Groups {
		for settingIndex := range result.Groups[groupIndex].Settings {
			setting := &result.Groups[groupIndex].Settings[settingIndex]
			series, err := s.base.vmQueryInstant(ctx, tenantID, setting.Name, now.Unix())
			if err != nil {
				continue
			}
			if value := latestVMValue(series); value != nil {
				setting.Value = value
				setting.Warn = postgresConfigSettingWarn(setting.Name, *value)
			}
		}
	}
	return result, nil
}

// ── Dashboard summary ────────────────────────────────────────────────────────

func (s *PostgresDashboardService) GetDashboardSummary(
	ctx context.Context,
	tenantUUID string,
	tenantID uint,
	serviceName string,
	startNs, endNs int64,
	step string,
) (*PostgresDashboardSummary, error) {
	startSec := startNs / int64(1e9)
	endSec := endNs / int64(1e9)
	cacheKey := fmt.Sprintf("postgres:dash:%s:%s:%d:%d:%s", tenantUUID, serviceName, startSec, endSec, step)

	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var result PostgresDashboardSummary
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

func (s *PostgresDashboardService) fetchDashboard(
	ctx context.Context,
	tenantUUID string,
	tenantID uint,
	serviceName string,
	startNs, endNs int64,
	step string,
) (*PostgresDashboardSummary, error) {
	startSec := startNs / int64(1e9)
	endSec := endNs / int64(1e9)
	rangeWindow := step
	if strings.TrimSpace(rangeWindow) == "" {
		rangeWindow = "30s"
	}

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
		"up":                "pg_up",
		"scrape_error":      "pg_exporter_last_scrape_error",
		"max_connections":   "pg_settings_max_connections",
		"conn_saturation":   "100 * sum(pg_stat_database_numbackends) / max(pg_settings_max_connections)",
		"cache_ratio":       "100 * sum(rate(pg_stat_database_blks_hit_total[" + rangeWindow + "])) / clamp_min(sum(rate(pg_stat_database_blks_hit_total[" + rangeWindow + "]) + rate(pg_stat_database_blks_read_total[" + rangeWindow + "])), 1)",
		"db_size_total":     "sum(pg_database_size_bytes)",
		"wal_size":          "pg_wal_size_bytes",
		"replication_lag":   "pg_replication_lag_seconds",
		"connections":       "sum(pg_stat_database_numbackends)",
		"activity":          "sum by (state) (pg_stat_activity_count)",
		"transactions":      "sum(rate(pg_stat_database_xact_commit_total[" + rangeWindow + "])) or vector(0)",
		"rollbacks":         "sum(rate(pg_stat_database_xact_rollback_total[" + rangeWindow + "])) or vector(0)",
		"deadlocks":         "sum(rate(pg_stat_database_deadlocks_total[" + rangeWindow + "])) or vector(0)",
		"cache_reads":       "sum(rate(pg_stat_database_blks_read_total[" + rangeWindow + "])) or vector(0)",
		"cache_hits":        "sum(rate(pg_stat_database_blks_hit_total[" + rangeWindow + "])) or vector(0)",
		"temp_files":        "sum(rate(pg_stat_database_temp_files_total[" + rangeWindow + "])) or vector(0)",
		"temp_bytes":        "sum(rate(pg_stat_database_temp_bytes_total[" + rangeWindow + "])) or vector(0)",
		"temp_files_by_db":  "sum by (datname) (rate(pg_stat_database_temp_files_total[" + rangeWindow + "]))",
		"temp_bytes_by_db":  "sum by (datname) (rate(pg_stat_database_temp_bytes_total[" + rangeWindow + "]))",
		"cache_ratio_by_db": "100 * sum by (datname) (rate(pg_stat_database_blks_hit_total[" + rangeWindow + "])) / clamp_min(sum by (datname) (rate(pg_stat_database_blks_hit_total[" + rangeWindow + "]) + rate(pg_stat_database_blks_read_total[" + rangeWindow + "])), 1)",
		"rollback_by_db":    "100 * sum by (datname) (rate(pg_stat_database_xact_rollback_total[" + rangeWindow + "])) / clamp_min(sum by (datname) (rate(pg_stat_database_xact_commit_total[" + rangeWindow + "]) + rate(pg_stat_database_xact_rollback_total[" + rangeWindow + "])), 1)",
		"locks":             "sum by (mode) (pg_locks_count)",
		"db_size_by_name":   "pg_database_size_bytes",
		"wal_timeseries":    "pg_wal_size_bytes",
		"max_tx_duration":   "max(pg_stat_activity_max_tx_duration{state='active'}) or vector(0)",
		"conn_by_db":        "pg_stat_database_numbackends",
		"row_inserts":       "sum(rate(pg_stat_database_tup_inserted_total[" + rangeWindow + "])) or vector(0)",
		"row_updates":       "sum(rate(pg_stat_database_tup_updated_total[" + rangeWindow + "])) or vector(0)",
		"row_deletes":       "sum(rate(pg_stat_database_tup_deleted_total[" + rangeWindow + "])) or vector(0)",
		"tup_returned":      "sum(rate(pg_stat_database_tup_returned_total[" + rangeWindow + "])) or vector(0)",
		"tup_fetched":       "sum(rate(pg_stat_database_tup_fetched_total[" + rangeWindow + "])) or vector(0)",
		"buffers_clean":     "sum(rate(pg_stat_bgwriter_buffers_clean_total[" + rangeWindow + "])) or vector(0)",
		"buffers_alloc":     "sum(rate(pg_stat_bgwriter_buffers_alloc_total[" + rangeWindow + "])) or vector(0)",
		"maxwritten_clean":  "sum(rate(pg_stat_bgwriter_maxwritten_clean_total[" + rangeWindow + "])) or vector(0)",
		"archive_ok":        "sum(rate(pg_stat_archiver_archived_count_total[" + rangeWindow + "])) or vector(0)",
		"archive_fail":      "sum(rate(pg_stat_archiver_failed_count_total[" + rangeWindow + "])) or vector(0)",
		"rollback_rate":     "100 * sum(rate(pg_stat_database_xact_rollback_total[" + rangeWindow + "])) / clamp_min(sum(rate(pg_stat_database_xact_commit_total[" + rangeWindow + "]) + rate(pg_stat_database_xact_rollback_total[" + rangeWindow + "])), 1)",
		"deadlock_rate_min": "sum(rate(pg_stat_database_deadlocks_total[5m])) * 60",
		"config_shared_buf": "pg_settings_shared_buffers_bytes",
		"config_work_mem":   "pg_settings_work_mem_bytes",
		"config_eff_cache":  "pg_settings_effective_cache_size_bytes",
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

	baseSelector := postgresLokiSelector(serviceName)
	logQueries := buildPostgresLogQueries(baseSelector)

	const dashboardPreviewLimit = 20
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

	summary := &PostgresDashboardSummary{
		Up:                      latestVMValue(results["up"].data),
		ScrapeError:             latestVMValue(results["scrape_error"].data),
		MaxConnections:          latestVMValue(results["max_connections"].data),
		ConnectionSaturationPct: latestVMValue(results["conn_saturation"].data),
		CacheHitRatioPct:        latestVMValue(results["cache_ratio"].data),
		DatabaseSizeBytes:       latestVMValue(results["db_size_total"].data),
		WalSizeBytes:            latestVMValue(results["wal_size"].data),
		ReplicationLagSeconds:   latestVMValue(results["replication_lag"].data),
		MaxTxDurationSeconds:    latestVMValue(results["max_tx_duration"].data),
		RollbackRatePct:         latestVMValue(results["rollback_rate"].data),
		DeadlocksPerMin:         latestVMValue(results["deadlock_rate_min"].data),
		ArchiveFailed:           latestVMValue(results["archive_fail"].data),
		SharedBuffersBytes:      latestVMValue(results["config_shared_buf"].data),
		WorkMemBytes:            latestVMValue(results["config_work_mem"].data),
		EffectiveCacheSizeBytes: latestVMValue(results["config_eff_cache"].data),
	}

	summary.ConnectionsTimeseries = singleVMSeries(results["connections"].data, "connections")
	summary.ActivityTimeseries = mergeVMSeriesByLabel(results["activity"].data, "state")
	summary.TransactionTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"commits":   results["transactions"].data,
		"rollbacks": results["rollbacks"].data,
		"deadlocks": results["deadlocks"].data,
	})
	summary.CacheTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"hit_ratio": results["cache_ratio"].data,
		"hits":      results["cache_hits"].data,
		"reads":     results["cache_reads"].data,
	})
	summary.TempTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"files": results["temp_files"].data,
		"bytes": results["temp_bytes"].data,
	})
	summary.LocksTimeseries = mergeVMSeriesByLabel(results["locks"].data, "mode")
	summary.SizeTimeseries = mergeVMSeriesByLabel(results["db_size_by_name"].data, "datname")
	summary.WalTimeseries = singleVMSeries(results["wal_timeseries"].data, "bytes")
	summary.RowOpsTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"inserts": results["row_inserts"].data,
		"updates": results["row_updates"].data,
		"deletes": results["row_deletes"].data,
	})
	summary.SeqVsIndexTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"returned": results["tup_returned"].data,
		"fetched":  results["tup_fetched"].data,
	})
	summary.BgwriterTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"clean":      results["buffers_clean"].data,
		"alloc":      results["buffers_alloc"].data,
		"maxwritten": results["maxwritten_clean"].data,
	})
	summary.ArchiverTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"archived": results["archive_ok"].data,
		"failed":   results["archive_fail"].data,
	})
	summary.ConnByDbTimeseries = mergeVMSeriesByLabel(results["conn_by_db"].data, "datname")

	summary.DatabaseSizes = latestLabelCounts(results["db_size_by_name"].data, "datname", 10)
	summary.LocksByMode = latestLabelCounts(results["locks"].data, "mode", 12)
	summary.ConnByDatabase = latestLabelCounts(results["conn_by_db"].data, "datname", 10)
	summary.DatabaseHealth = postgresDatabaseHealth(
		results["db_size_by_name"].data,
		results["conn_by_db"].data,
		results["cache_ratio_by_db"].data,
		results["temp_files_by_db"].data,
		results["temp_bytes_by_db"].data,
		results["rollback_by_db"].data,
		15,
	)

	summary.LogSummary = s.countParsedLogCategories(ctx, tenantUUID, serviceName, startNs, endNs)

	summary.RecentLogs = mergePostgresLogEvents(dashboardPreviewLimit,
		logResults["errors"].entries,
		logResults["auth_failures"].entries,
		logResults["lock_waits"].entries,
		logResults["schema_changes"].entries,
	)
	summary.SlowQueryLogs = mergePostgresLogEvents(dashboardPreviewLimit,
		logResults["slow"].entries,
	)

	return summary, nil
}

var monitoringUsers = map[string]struct{}{
	"postgres_exporter": {},
	"pgexporter":        {},
	"monitoring":        {},
}

// ── Dedicated Logs Endpoint ──────────────────────────────────────────────────

func (s *PostgresDashboardService) GetLogs(
	ctx context.Context,
	tenantUUID string,
	serviceName string,
	startNs, endNs int64,
	category string,
	search string,
	includeMonitoring bool,
	page int,
	pageSize int,
) (*PostgresLogsPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 100
	}

	baseSelector := postgresLokiSelector(serviceName)
	lokiQuery := postgresLogsLokiQuery(baseSelector, category)

	// Fetch all matching logs using our helper
	entries, err := s.lokiQueryAllLogEntries(ctx, tenantUUID, lokiQuery, startNs, endNs)
	if err != nil {
		return nil, fmt.Errorf("loki fetch: %w", err)
	}

	events := make([]PostgresLogEvent, 0, len(entries))
	for _, e := range entries {
		events = append(events, parsePostgresLogEvent(e))
	}

	// Category filter (after parsing)
	if category != "" && category != "all" {
		filtered := events[:0]
		for _, ev := range events {
			if ev.Category == category {
				filtered = append(filtered, ev)
			}
		}
		events = filtered
	}

	// Search filter
	if needle := strings.ToLower(strings.TrimSpace(search)); needle != "" {
		filtered := events[:0]
		for _, ev := range events {
			if postgresLogMatchesSearch(ev, needle) {
				filtered = append(filtered, ev)
			}
		}
		events = filtered
	}

	// Hide monitoring user connection/disconnection events by default
	if !includeMonitoring {
		// Build port-to-user map so we can correlate connection received
		// lines (which have no user) to the full session. All connection
		// and disconnection events carry the client host:port in the
		// Client field (e.g. "::1(58254)"), even when the message body
		// does not contain "port=XXXXX".
		portToUser := make(map[string]string)
		for _, ev := range events {
			if ev.Category == "connection" || ev.Category == "disconnection" {
				if ev.User != "" && ev.User != "-" {
					if matches := clientPortRE.FindStringSubmatch(ev.Client); len(matches) == 2 {
						portToUser[matches[1]] = ev.User
					}
				}
			}
		}

		filtered := events[:0]
		for _, ev := range events {
			if ev.Category == "connection" || ev.Category == "disconnection" {
				effectiveUser := ev.User
				if effectiveUser == "" || effectiveUser == "-" {
					if matches := clientPortRE.FindStringSubmatch(ev.Client); len(matches) == 2 {
						if u, ok := portToUser[matches[1]]; ok {
							effectiveUser = u
						}
					}
				}
				if isMonitoringUser(effectiveUser) {
					continue
				}
			}
			filtered = append(filtered, ev)
		}
		events = filtered
	}

	total := len(events)
	offset := (page - 1) * pageSize
	if offset >= total {
		return &PostgresLogsPage{
			Events:   []PostgresLogEvent{},
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

	return &PostgresLogsPage{
		Events:   events[offset:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  end < total,
	}, nil
}

func (s *PostgresDashboardService) countParsedLogCategories(
	ctx context.Context,
	tenantUUID string,
	serviceName string,
	startNs, endNs int64,
) PostgresLogSummary {
	entries, err := s.lokiQueryAllLogEntries(ctx, tenantUUID, postgresLokiSelector(serviceName), startNs, endNs)
	if err != nil {
		return PostgresLogSummary{}
	}

	var summary PostgresLogSummary
	for _, entry := range entries {
		ev := parsePostgresLogEvent(entry)
		switch ev.Category {
		case "slow query":
			summary.SlowQueries++
		case "error":
			summary.Errors++
		case "auth failure":
			summary.AuthFailures++
		case "connection":
			summary.Connections++
		case "disconnection":
			summary.Disconnections++
		case "lock wait":
			summary.LockWaits++
		case "temp file":
			summary.TempFiles++
		case "schema change":
			summary.SchemaChanges++
		case "checkpoint":
			summary.Checkpoints++
		}
	}
	return summary
}

// ── Loki pagination helper (local to Postgres service) ───────────────────────

func (s *PostgresDashboardService) lokiQueryAllLogEntries(
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

	// Sort ascending so each continuation line immediately follows its primary entry.
	sort.SliceStable(all, func(i, j int) bool {
		ti, _ := strconv.ParseInt(all[i].TimestampNs, 10, 64)
		tj, _ := strconv.ParseInt(all[j].TimestampNs, 10, 64)
		return ti < tj
	})

	// Merge multi-line PostgreSQL log entries (e.g. multi-line SQL statements,
	// DETAIL / HINT / CONTEXT continuation lines) into a single entry each.
	all = mergePGMultilineEntries(all)

	// Sort descending (newest first) for the UI.
	sortLokiLogEntriesDesc(all)
	return all, nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// mergePGMultilineEntries merges PostgreSQL continuation lines into their
// preceding primary entry. A continuation line is any line that does NOT carry
// the full PostgreSQL log header ([pid] user@db client app SEVERITY:).
//
// PostgreSQL writes multi-line statements (e.g. CREATE TABLE with column
// definitions) as separate physical lines. Loki ingests each line individually,
// so without this step they appear as orphaned single-line events in the UI.
//
// The function also handles headered DETAIL / HINT / CONTEXT lines emitted by
// PostgreSQL as follow-on lines to an ERROR or WARNING entry. LOG: statement:
// entries intentionally remain primary log rows.
//
// Input must be sorted ascending by timestamp; the caller re-sorts descending
// afterwards for the UI.
func mergePGMultilineEntries(entries []LokiLogEntry) []LokiLogEntry {
	if len(entries) == 0 {
		return entries
	}
	merged := make([]LokiLogEntry, 0, len(entries))
	pendingContinuations := make(map[string][]LokiLogEntry)
	lastPrimaryPID := ""
	lastPrimaryTimestamp := ""
	for _, e := range entries {
		line := strings.TrimSpace(e.Line)
		header, hasHeader := parsePostgresHeader(line)
		if !hasHeader && len(merged) > 0 {
			// Strip any bare collection-timestamp prefix the log pipeline may
			// have prepended (e.g. "2026-05-12 17:36:50.201 "), then append
			// the fragment to the last primary entry.
			fragment := strings.TrimSpace(continuationLeadingTsRE.ReplaceAllString(line, ""))
			if fragment != "" {
				merged[len(merged)-1].Line += "\n" + fragment
			}
			continue
		}

		if hasHeader {
			message := strings.TrimSpace(header.Message)
			key := postgresContinuationKey(header.Timestamp, header.PID)
			if len(merged) > 0 && pgHeaderContinuationRE.MatchString(message) &&
				header.Timestamp == lastPrimaryTimestamp && header.PID == lastPrimaryPID {
				merged[len(merged)-1].Line += "\n" + message
				continue
			}
			if pgHeaderContinuationRE.MatchString(message) {
				pendingContinuations[key] = append(pendingContinuations[key], e)
				continue
			}
			lastPrimaryTimestamp = header.Timestamp
			lastPrimaryPID = header.PID
			merged = append(merged, e)
			if pending, ok := pendingContinuations[key]; ok {
				for _, continuation := range pending {
					if continuationHeader, ok := parsePostgresHeader(strings.TrimSpace(continuation.Line)); ok {
						merged[len(merged)-1].Line += "\n" + strings.TrimSpace(continuationHeader.Message)
					}
				}
				delete(pendingContinuations, key)
			}
		} else {
			merged = append(merged, e)
			lastPrimaryTimestamp = ""
			lastPrimaryPID = ""
		}
	}
	for _, pending := range pendingContinuations {
		merged = append(merged, pending...)
	}
	return merged
}

func postgresContinuationKey(timestamp, pid string) string {
	return timestamp + "\x00" + pid
}

func parsePostgresHeader(line string) (postgresLogHeader, bool) {
	if matches := postgresPrefixRE.FindStringSubmatch(line); len(matches) == 8 {
		return postgresLogHeader{
			Timestamp: matches[1],
			PID:       matches[2],
			User:      unknownToEmpty(matches[3]),
			Database:  unknownToEmpty(matches[4]),
			Client:    unknownToEmpty(matches[5]),
			App:       unknownToEmpty(matches[6]),
			Message:   matches[7],
		}, true
	}
	if matches := postgresServerPrefixRE.FindStringSubmatch(line); len(matches) == 5 {
		return postgresLogHeader{
			Timestamp: matches[1],
			PID:       matches[2],
			Message:   matches[4],
		}, true
	}
	return postgresLogHeader{}, false
}

func postgresLokiSelector(serviceName string) string {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return `{service_name=~"postgres.*|postgresql.*"}`
	}
	return fmt.Sprintf(`{service_name="%s"}`, lokiLabelValue(serviceName))
}

func postgresLogsLokiQuery(baseSelector, category string) string {
	if postgresLogCategoryNeedsRawLines(category) {
		return baseSelector
	}
	filters := map[string]string{
		"auth failure":  `|~ "(?i)(password authentication failed|authentication failed|no pg_hba.conf entry)"`,
		"connection":    `|~ "connection received|connection authorized|connection authenticated"`,
		"disconnection": `|~ "disconnection:"`,
		"error":         `|~ "(?i)\b(ERROR|FATAL|PANIC):"`,
		"lock wait":     `|~ "(?i)still waiting for|deadlock detected|could not obtain lock"`,
		"schema change": `|~ "(?i)statement:\s*(CREATE|ALTER|DROP|TRUNCATE|GRANT|REVOKE)"`,
		"slow query":    `|~ "(?i)duration: [0-9.]+ ms"`,
		"temp file":     `|~ "(?i)temporary file:"`,
		"checkpoint":    `|~ "(?i)checkpoint complete|checkpoint starting"`,
	}
	if f, ok := filters[category]; ok {
		return baseSelector + " " + f
	}
	return baseSelector
}

func postgresLogCategoryNeedsRawLines(category string) bool {
	// Category filters must agree exactly with the parsed "All categories" view.
	// PostgreSQL continuation lines and server-process headers make Loki-only
	// prefilters too easy to under-fetch, so the logs page filters after parsing.
	return strings.TrimSpace(strings.ToLower(category)) != ""
}

func isMonitoringUser(user string) bool {
	_, ok := monitoringUsers[user]
	return ok
}

func postgresLogMatchesSearch(ev PostgresLogEvent, needle string) bool {
	for _, field := range []string{ev.Timestamp, ev.Severity, ev.User, ev.Database, ev.Client, ev.App, ev.Category, ev.Message} {
		if strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}
	return false
}

func buildPostgresLogQueries(baseSelector string) map[string]string {
	return map[string]string{
		"slow":           baseSelector + ` |~ "(?i)duration: [0-9.]+ ms"`,
		"errors":         baseSelector + ` |~ "(?i)\b(ERROR|FATAL|PANIC)\b"`,
		"auth_failures":  baseSelector + ` |~ "(?i)(password authentication failed|authentication failed|no pg_hba.conf entry)"`,
		"connections":    baseSelector + ` |~ "connection received|connection authorized|connection authenticated"`,
		"disconnections": baseSelector + ` |~ "disconnection:"`,
		"lock_waits":     baseSelector + ` |~ "(?i)still waiting for|deadlock detected|could not obtain lock"`,
		"temp_files":     baseSelector + ` |~ "(?i)temporary file:"`,
		"schema_changes": baseSelector + ` |~ "(?i)statement:\s*(CREATE|ALTER|DROP|TRUNCATE|GRANT|REVOKE)"`,
		"checkpoints":    baseSelector + ` |~ "(?i)checkpoint complete|checkpoint starting"`,
	}
}

// ── VM / common helpers (re-exported for convenience) ────────────────────────

func singleVMSeries(series []VMSeries, key string) []TimePoint {
	if len(series) == 0 {
		return []TimePoint{}
	}
	points := make([]TimePoint, 0, len(series[0].Values))
	for _, sample := range series[0].Values {
		points = append(points, TimePoint{"time": sample[0], key: sample[1]})
	}
	return points
}

func mergeNamedVMSeries(seriesByName map[string][]VMSeries) []TimePoint {
	timeMap := map[float64]TimePoint{}
	for name, series := range seriesByName {
		for _, item := range series {
			for _, sample := range item.Values {
				if _, ok := timeMap[sample[0]]; !ok {
					timeMap[sample[0]] = TimePoint{"time": sample[0]}
				}
				timeMap[sample[0]][name] = sample[1]
			}
		}
	}
	points := make([]TimePoint, 0, len(timeMap))
	for _, pt := range timeMap {
		points = append(points, pt)
	}
	sortTimePoints(points)
	return points
}

func mergeVMSeriesByLabel(series []VMSeries, labelKey string) []TimePoint {
	timeMap := map[float64]TimePoint{}
	for _, item := range series {
		label := item.Metric[labelKey]
		if label == "" {
			label = "unknown"
		}
		for _, sample := range item.Values {
			if _, ok := timeMap[sample[0]]; !ok {
				timeMap[sample[0]] = TimePoint{"time": sample[0]}
			}
			timeMap[sample[0]][label] = sample[1]
		}
	}
	points := make([]TimePoint, 0, len(timeMap))
	for _, pt := range timeMap {
		points = append(points, pt)
	}
	sortTimePoints(points)
	return points
}

func latestLabelCounts(series []VMSeries, labelKey string, limit int) []LabelCount {
	counts := map[string]float64{}
	for _, item := range series {
		label := item.Metric[labelKey]
		if label == "" {
			label = "unknown"
		}
		counts[label] += lastValue(item.Values)
	}
	return topN(counts, limit)
}

func postgresDatabaseHealth(
	sizeSeries []VMSeries,
	connSeries []VMSeries,
	cacheSeries []VMSeries,
	tempFileSeries []VMSeries,
	tempByteSeries []VMSeries,
	rollbackSeries []VMSeries,
	limit int,
) []PostgresDatabaseHealth {
	rows := map[string]*PostgresDatabaseHealth{}
	ensure := func(db string) *PostgresDatabaseHealth {
		if db == "" {
			db = "unknown"
		}
		if _, ok := rows[db]; !ok {
			rows[db] = &PostgresDatabaseHealth{Database: db}
		}
		return rows[db]
	}
	apply := func(series []VMSeries, fn func(*PostgresDatabaseHealth, float64)) {
		for _, item := range series {
			db := item.Metric["datname"]
			row := ensure(db)
			fn(row, lastValue(item.Values))
		}
	}

	apply(sizeSeries, func(row *PostgresDatabaseHealth, value float64) { row.SizeBytes = value })
	apply(connSeries, func(row *PostgresDatabaseHealth, value float64) { row.Connections = value })
	apply(cacheSeries, func(row *PostgresDatabaseHealth, value float64) { row.CacheHitRatioPct = value })
	apply(tempFileSeries, func(row *PostgresDatabaseHealth, value float64) { row.TempFilesPerSecond = value })
	apply(tempByteSeries, func(row *PostgresDatabaseHealth, value float64) { row.TempBytesPerSecond = value })
	apply(rollbackSeries, func(row *PostgresDatabaseHealth, value float64) { row.RollbackRatePct = value })

	out := make([]PostgresDatabaseHealth, 0, len(rows))
	for _, row := range rows {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].SizeBytes > out[j].SizeBytes
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// ── Log parsing ──────────────────────────────────────────────────────────────

var (
	postgresPrefixRE       = regexp.MustCompile(`(?:^|.*?)(\d{4}-\d{2}-\d{2} [0-9:.]+ \S+) \[(\d+)\] ([^@ ]+)@([^ ]+) ([^ ]*) ([^ ]*) (.*)$`)
	postgresServerPrefixRE = regexp.MustCompile(`(?:^|.*?)(\d{4}-\d{2}-\d{2} [0-9:.]+ \S+) \[(\d+)\] (@\s+)?(.*)$`)
	durationRE             = regexp.MustCompile(`duration: ([0-9.]+) ms`)
	postgresSeverityRE     = regexp.MustCompile(`\b(DEBUG|INFO|NOTICE|WARNING|ERROR|LOG|FATAL|PANIC):\s+`)
	pgHeaderContinuationRE = regexp.MustCompile(`(?i)^(DETAIL|HINT|CONTEXT|LOCATION|SCHEMA NAME|TABLE NAME|COLUMN NAME|DATATYPE NAME|CONSTRAINT NAME):`)
	// Matches a bare OTel/Loki collection-timestamp prefix on continuation lines,
	// e.g. "2026-05-12 17:36:50.201 " — stripped before appending to the primary entry.
	continuationLeadingTsRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} [0-9:.]+ `)
	clientPortRE            = regexp.MustCompile(`\((\d+)\)`)
)

func mergePostgresLogEvents(limit int, groups ...[]LokiLogEntry) []PostgresLogEvent {
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
	events := make([]PostgresLogEvent, 0, len(entries))
	for _, entry := range entries {
		events = append(events, parsePostgresLogEvent(entry))
	}
	return events
}

func parsePostgresLogEvent(entry LokiLogEntry) PostgresLogEvent {
	event := PostgresLogEvent{
		Timestamp: entry.TimestampNs,
		Severity:  "LOG",
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

	// For multi-line entries (continuation lines merged in), the PostgreSQL
	// header is only on the first line. Run the header regex against the first
	// line only so that $ matches correctly (Go RE2: $ = end of text, . != \n).
	firstLine := line
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		firstLine = line[:idx]
	}
	if header, ok := parsePostgresHeader(firstLine); ok {
		event.Timestamp = header.Timestamp
		event.User = header.User
		event.Database = header.Database
		event.Client = header.Client
		event.App = header.App
		// matches[7] is the message portion of the first line (e.g.
		// "LOG:  statement: CREATE TABLE test_metrics_table (").
		// If there are continuation lines, append them so the full SQL body
		// is visible in the message field.
		if idx := strings.IndexByte(line, '\n'); idx >= 0 {
			event.Message = header.Message + "\n" + line[idx+1:]
		} else {
			event.Message = header.Message
		}
	}

	if matches := postgresSeverityRE.FindStringSubmatch(event.Message); len(matches) == 2 {
		event.Severity = strings.ToUpper(matches[1])
	}
	if matches := durationRE.FindStringSubmatch(event.Message); len(matches) == 2 {
		if value, err := strconv.ParseFloat(matches[1], 64); err == nil {
			event.DurationMs = &value
		}
	}

	event.Category = classifyPostgresLog(event.Message)
	if len(event.Labels) == 0 {
		event.Labels = nil
	}
	return event
}

func unknownToEmpty(value string) string {
	if value == "[unknown]" || value == "[unknown]@[unknown]" {
		return ""
	}
	return value
}

func classifyPostgresLog(message string) string {
	lower := strings.ToLower(message)
	compact := strings.Join(strings.Fields(lower), " ")
	switch {
	case strings.Contains(lower, "password authentication failed") || strings.Contains(lower, "authentication failed"):
		return "auth failure"
	case strings.Contains(lower, "error:") || strings.Contains(lower, "fatal:") || strings.Contains(lower, "panic:"):
		return "error"
	case strings.Contains(lower, "duration:"):
		return "slow query"
	case strings.Contains(lower, "deadlock detected") || strings.Contains(lower, "still waiting for") || strings.Contains(lower, "could not obtain lock"):
		return "lock wait"
	case strings.Contains(lower, "temporary file:"):
		return "temp file"
	case strings.Contains(compact, "statement: create") ||
		strings.Contains(compact, "statement: alter") ||
		strings.Contains(compact, "statement: drop") ||
		strings.Contains(compact, "statement: truncate") ||
		strings.Contains(compact, "statement: grant") ||
		strings.Contains(compact, "statement: revoke"):
		return "schema change"
	case strings.Contains(lower, "connection received") || strings.Contains(lower, "connection authorized") || strings.Contains(lower, "connection authenticated"):
		return "connection"
	case strings.Contains(lower, "disconnection:"):
		return "disconnection"
	case strings.Contains(lower, "checkpoint"):
		return "checkpoint"
	default:
		return "event"
	}
}

func postgresConfigCatalog() []PostgresConfigGroup {
	return []PostgresConfigGroup{
		{
			Key:   "memory",
			Title: "Memory",
			Settings: []PostgresConfigSetting{
				cfgSetting("pg_settings_shared_buffers_bytes", "bytes", "Shared memory pool used for caching disk pages. Usually 25% of total RAM."),
				cfgSetting("pg_settings_work_mem_bytes", "bytes", "Memory per sort or hash operation. High values speed up complex queries but multiply per connection."),
				cfgSetting("pg_settings_effective_cache_size_bytes", "bytes", "Planner's estimate of OS + buffer cache available. Does not allocate memory, just guides query planning."),
				cfgSetting("pg_settings_maintenance_work_mem_bytes", "bytes", "Memory for VACUUM, CREATE INDEX, and similar maintenance. Can be safely set higher than work_mem."),
				cfgSetting("pg_settings_temp_buffers_bytes", "bytes", "Memory for temporary tables per session."),
				cfgSetting("pg_settings_logical_decoding_work_mem_bytes", "bytes", "Memory for logical replication decoding buffers."),
			},
		},
		{
			Key:   "connections",
			Title: "Connections",
			Settings: []PostgresConfigSetting{
				cfgSetting("pg_settings_max_connections", "number", "Maximum simultaneous client connections. Each connection uses memory; use a pooler like PgBouncer above about 200."),
				cfgSetting("pg_settings_superuser_reserved_connections", "number", "Connections reserved for superusers. Regular users cannot use these slots."),
				cfgSetting("pg_settings_reserved_connections", "number", "Connections reserved for roles with pg_use_reserved_connections privilege."),
				cfgSetting("pg_settings_idle_in_transaction_session_timeout_seconds", "seconds", "Idle open transactions are killed after this delay. Set a low value to prevent lock buildup."),
				cfgSetting("pg_settings_idle_session_timeout_seconds", "seconds", "Fully idle sessions are disconnected after this. Zero means never."),
				cfgSetting("pg_settings_lock_timeout_seconds", "seconds", "A query waiting for a lock longer than this is cancelled. Zero means wait forever."),
				cfgSetting("pg_settings_statement_timeout_seconds", "seconds", "Any single statement running longer than this is cancelled. Zero means no limit."),
			},
		},
		{
			Key:   "wal",
			Title: "Checkpoints and WAL",
			Settings: []PostgresConfigSetting{
				cfgSetting("pg_settings_checkpoint_timeout_seconds", "seconds", "Maximum time between automatic checkpoints. Higher values mean fewer I/O spikes but more work per checkpoint."),
				cfgSetting("pg_settings_checkpoint_completion_target", "number", "Fraction of checkpoint interval to spread writes over. 0.9 means spread over 90% of the gap."),
				cfgSetting("pg_settings_max_wal_size_bytes", "bytes", "WAL grows up to this size before forcing a checkpoint. Higher means less checkpoint pressure."),
				cfgSetting("pg_settings_min_wal_size_bytes", "bytes", "Minimum WAL size kept on disk to avoid recycling files unnecessarily."),
				cfgSetting("pg_settings_wal_buffers_bytes", "bytes", "Memory for WAL data not yet written to disk. -1 means auto-calculated from shared_buffers."),
				cfgSetting("pg_settings_wal_keep_size_bytes", "bytes", "Minimum WAL retained for standby servers to catch up. Zero means no forced retention."),
				cfgSetting("pg_settings_fsync", "boolean", "Whether PostgreSQL forces data to disk for durability. Should be on in production."),
				cfgSetting("pg_settings_full_page_writes", "boolean", "Protects against torn pages after crashes. Should be on in production."),
			},
		},
		{
			Key:   "autovacuum",
			Title: "Autovacuum",
			Settings: []PostgresConfigSetting{
				cfgSetting("pg_settings_autovacuum", "boolean", "Whether autovacuum is enabled. Should almost always be on."),
				cfgSetting("pg_settings_autovacuum_max_workers", "number", "Maximum autovacuum worker processes running simultaneously."),
				cfgSetting("pg_settings_autovacuum_naptime_seconds", "seconds", "How often autovacuum wakes up to check each database."),
				cfgSetting("pg_settings_autovacuum_vacuum_scale_factor", "number", "Fraction of table rows that must be dead before a vacuum is triggered."),
				cfgSetting("pg_settings_autovacuum_analyze_scale_factor", "number", "Fraction of table rows that must change before an analyze is triggered."),
				cfgSetting("pg_settings_autovacuum_vacuum_cost_limit", "number", "Cost limit for autovacuum before it sleeps. Higher value means more aggressive vacuuming."),
				cfgSetting("pg_settings_autovacuum_vacuum_cost_delay_seconds", "seconds", "Sleep time after autovacuum hits its cost limit. Lower means more I/O usage by vacuum."),
			},
		},
		{
			Key:   "planner",
			Title: "Query Planner",
			Settings: []PostgresConfigSetting{
				cfgSetting("pg_settings_random_page_cost", "number", "Planner's estimate of the cost of a non-sequential disk page fetch. Lower values favor index scans."),
				cfgSetting("pg_settings_seq_page_cost", "number", "Planner's estimate of a sequential disk page fetch. Baseline cost unit."),
				cfgSetting("pg_settings_effective_io_concurrency", "number", "Number of concurrent I/O operations the planner assumes the disk can serve. Use higher values for SSDs."),
				cfgSetting("pg_settings_default_statistics_target", "number", "Detail level of column statistics collected by ANALYZE. Higher means better plans but slower ANALYZE."),
				cfgSetting("pg_settings_enable_seqscan", "boolean", "Whether the planner can use sequential scans. Disabling this forces index use and is mainly for debugging."),
				cfgSetting("pg_settings_enable_hashjoin", "boolean", "Whether hash join plans are allowed."),
				cfgSetting("pg_settings_enable_mergejoin", "boolean", "Whether merge join plans are allowed."),
				cfgSetting("pg_settings_enable_nestloop", "boolean", "Whether nested loop join plans are allowed."),
				cfgSetting("pg_settings_jit", "boolean", "Whether JIT compilation is enabled for queries. Can speed up analytical queries but add latency to short OLTP queries."),
				cfgSetting("pg_settings_max_parallel_workers_per_gather", "number", "Maximum parallel workers per query gather node."),
				cfgSetting("pg_settings_max_parallel_workers", "number", "Maximum total parallel worker processes across all queries."),
				cfgSetting("pg_settings_max_worker_processes", "number", "Maximum background worker processes. Must cover parallel, replication, and autovacuum workers."),
			},
		},
		{
			Key:   "replication",
			Title: "Replication",
			Settings: []PostgresConfigSetting{
				cfgSetting("pg_settings_max_wal_senders", "number", "Maximum simultaneous WAL sender processes, one per replica or streaming backup."),
				cfgSetting("pg_settings_max_replication_slots", "number", "Maximum number of replication slots. Unused slots can retain WAL indefinitely."),
				cfgSetting("pg_settings_hot_standby", "boolean", "Whether standbys allow read queries while in recovery. Needed for read replicas."),
				cfgSetting("pg_settings_hot_standby_feedback", "boolean", "Whether standbys inform the primary of queries in progress to prevent vacuum conflicts."),
				cfgSetting("pg_settings_wal_receiver_timeout_seconds", "seconds", "A streaming replication connection is considered dead if no data arrives within this timeout."),
			},
		},
		{
			Key:   "logging",
			Title: "Logging",
			Settings: []PostgresConfigSetting{
				cfgSetting("pg_settings_log_min_duration_statement_seconds", "seconds", "Log any statement that takes longer than this. -1 disables. 0 logs everything."),
				cfgSetting("pg_settings_log_checkpoints", "boolean", "Whether checkpoint events are logged."),
				cfgSetting("pg_settings_log_lock_waits", "boolean", "Whether waits for locks are logged."),
				cfgSetting("pg_settings_log_autovacuum_min_duration_seconds", "seconds", "Log autovacuum runs that take longer than this. -1 disables."),
				cfgSetting("pg_settings_log_temp_files_bytes", "bytes", "Log creation of temp files larger than this. -1 disables."),
				cfgSetting("pg_settings_logging_collector", "boolean", "Whether the built-in logging collector is running. Required for log file output."),
				cfgSetting("pg_settings_log_rotation_age_seconds", "seconds", "Log file is rotated after this age."),
				cfgSetting("pg_settings_log_rotation_size_bytes", "bytes", "Log file is rotated when it reaches this size."),
			},
		},
		{
			Key:   "security",
			Title: "Security and Auth",
			Settings: []PostgresConfigSetting{
				cfgSetting("pg_settings_ssl", "boolean", "Whether SSL connections are accepted."),
				cfgSetting("pg_settings_ssl_prefer_server_ciphers", "boolean", "Whether the server's cipher preferences take priority over the client's."),
				cfgSetting("pg_settings_authentication_timeout_seconds", "seconds", "A client must authenticate within this time or the connection is dropped."),
				cfgSetting("pg_settings_scram_iterations", "number", "SCRAM iteration count for password hashing. Higher is more secure but slower at login."),
				cfgSetting("pg_settings_row_security", "boolean", "Whether row-level security policies are enforced by default."),
			},
		},
		{
			Key:   "constants",
			Title: "Infrastructure Constants",
			Settings: []PostgresConfigSetting{
				cfgSetting("pg_settings_block_size", "bytes", "Database page size in bytes. Fixed at compile time, usually 8192."),
				cfgSetting("pg_settings_max_function_args", "number", "Maximum number of arguments a function can accept."),
				cfgSetting("pg_settings_server_version_num", "number", "Numeric server version, e.g. 160001 means PostgreSQL 16.1."),
				cfgSetting("pg_settings_port", "number", "Port the server listens on."),
				cfgSetting("pg_settings_segment_size_bytes", "bytes", "Maximum size of a single relation segment file on disk."),
			},
		},
	}
}

func cfgSetting(name, unit, description string) PostgresConfigSetting {
	return PostgresConfigSetting{Name: name, Unit: unit, Description: description}
}

func postgresConfigSettingWarn(name string, value float64) bool {
	switch name {
	case "pg_settings_autovacuum",
		"pg_settings_fsync",
		"pg_settings_full_page_writes",
		"pg_settings_log_checkpoints",
		"pg_settings_log_lock_waits":
		return value == 0
	case "pg_settings_ssl":
		return value == 0
	default:
		return false
	}
}
