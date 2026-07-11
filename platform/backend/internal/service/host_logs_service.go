package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
)

type HostLogEvent struct {
	Timestamp   string            `json:"timestamp"`
	TimestampNs string            `json:"timestamp_ns"`
	Message     string            `json:"message"`
	Priority    string            `json:"priority,omitempty"`
	Severity    string            `json:"severity"`
	Unit        string            `json:"unit,omitempty"`
	Identifier  string            `json:"identifier,omitempty"`
	PID         string            `json:"pid,omitempty"`
	Transport   string            `json:"transport,omitempty"`
	Command     string            `json:"command,omitempty"`
	Raw         map[string]string `json:"raw,omitempty"` // Keep only useful fields
	Labels      map[string]string `json:"labels,omitempty"`
}

type HostLogsSummary struct {
	Total     int            `json:"total"`
	ByLevel   map[string]int `json:"by_level"`
	ByUnit    map[string]int `json:"by_unit"`
	Units     []string       `json:"units"`
	FirstSeen string         `json:"first_seen,omitempty"`
	LastSeen  string         `json:"last_seen,omitempty"`
}

type HostLogsPage struct {
	Events   []HostLogEvent  `json:"events"`
	Summary  HostLogsSummary `json:"summary"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	HasMore  bool            `json:"has_more"`
}

// ─────────────────────────────────────────────────────────────
// Log Health Summary — derived metrics from journald logs
// ─────────────────────────────────────────────────────────────

type LogHealthTimeBucket struct {
	Time     int64 `json:"time"`
	Errors   int   `json:"errors"`
	Warnings int   `json:"warnings"`
	Total    int   `json:"total"`
}

type LogHealthRestartEvent struct {
	Time    int64  `json:"time"`
	Unit    string `json:"unit"`
	Failed  bool   `json:"failed"`
	Message string `json:"message"`
}

type LogHealthSSHBucket struct {
	Time   int64 `json:"time"`
	Failed int   `json:"failed"`
	OK     int   `json:"ok"`
}

type LogHealthOOMEvent struct {
	Time    int64  `json:"time"`
	Message string `json:"message"`
}

type LogHealthDiskErrorBucket struct {
	Time  int64 `json:"time"`
	Count int   `json:"count"`
}

type LogHealthSummary struct {
	ErrorRate      []LogHealthTimeBucket     `json:"error_rate"`
	RestartEvents  []LogHealthRestartEvent   `json:"restart_events"`
	SSHAttempts    []LogHealthSSHBucket      `json:"ssh_attempts"`
	OOMKills       []LogHealthOOMEvent       `json:"oom_kills"`
	DiskErrors     []LogHealthDiskErrorBucket `json:"disk_errors"`
	TotalErrors    int                       `json:"total_errors"`
	TotalWarnings  int                       `json:"total_warnings"`
	TotalRestarts  int                       `json:"total_restarts"`
	TotalOOMKills  int                       `json:"total_oom_kills"`
}

type HostLogsService struct {
	base  *NginxDashboardService
	redis *redis.Client
}

func NewHostLogsService(cfg *config.Config, redisClient *redis.Client) *HostLogsService {
	return &HostLogsService{
		base:  NewNginxDashboardService(cfg, redisClient),
		redis: redisClient,
	}
}

func (s *HostLogsService) GetLogs(
	ctx context.Context,
	tenantUUID string,
	machineID string,
	startNs, endNs int64,
	priority string,
	unit string,
	identifier string,
	search string,
	page int,
	pageSize int,
) (*HostLogsPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 100
	}

	cacheKey := fmt.Sprintf(
		"host:logs:%s:%s:%d:%d:%s:%s:%s:%s:%d:%d",
		tenantUUID,
		machineID,
		startNs,
		endNs,
		priority,
		unit,
		identifier,
		search,
		page,
		pageSize,
	)

	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var result HostLogsPage
			if json.Unmarshal(cached, &result) == nil {
				return &result, nil
			}
		}
	}

	query := fmt.Sprintf(`{service_name="journald"} | host_id="%s"`, lokiLabelValue(machineID))

	entries, err := s.queryAllHostLogEntries(ctx, tenantUUID, query, startNs, endNs)
	if err != nil {
		return nil, err
	}

	events := make([]HostLogEvent, 0, len(entries))
	for _, entry := range entries {
		event := parseHostLogEvent(entry)
		if isLowValueHostLogNoise(event) {
			continue
		}
		if priority != "" && event.Priority != priority {
			continue
		}
		if unit != "" && event.Unit != unit {
			continue
		}
		if identifier != "" && event.Identifier != identifier {
			continue
		}
		if search != "" && !hostLogMatchesSearch(event, search) {
			continue
		}
		events = append(events, event)
	}

	summary := buildHostLogsSummary(events)
	total := len(events)
	offset := (page - 1) * pageSize

	if offset >= total {
		result := &HostLogsPage{
			Events:   []HostLogEvent{},
			Summary:  summary,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
			HasMore:  false,
		}
		s.cacheLogsPage(ctx, cacheKey, result)
		return result, nil
	}

	end := offset + pageSize
	if end > total {
		end = total
	}

	result := &HostLogsPage{
		Events:   events[offset:end],
		Summary:  summary,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  end < total,
	}

	s.cacheLogsPage(ctx, cacheKey, result)
	return result, nil
}

func (s *HostLogsService) GetHealthSummary(
	ctx context.Context,
	tenantUUID string,
	machineID string,
	startNs, endNs int64,
	step string,
) (*LogHealthSummary, error) {
	cacheKey := fmt.Sprintf("host:logs:health:%s:%s:%d:%d:%s", tenantUUID, machineID, startNs, endNs, step)

	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var result LogHealthSummary
			if json.Unmarshal(cached, &result) == nil {
				return &result, nil
			}
		}
	}

	baseSelector := fmt.Sprintf(`{service_name="journald"} | host_id="%s"`, lokiLabelValue(machineID))

	// Error rate (PRIORITY 0-3 = emergency..error) bucketed by time
	errorQ := fmt.Sprintf(`sum(count_over_time(%s | PRIORITY=~"0|1|2|3" [%s]))`, baseSelector, step)
	// Warning rate (PRIORITY 4)
	warningQ := fmt.Sprintf(`sum(count_over_time(%s | PRIORITY="4" [%s]))`, baseSelector, step)
	// Total log rate for context
	totalQ := fmt.Sprintf(`sum(count_over_time(%s [%s]))`, baseSelector, step)

	// Service restart/failure detection
	restartQ := fmt.Sprintf(`%s |~ "(?i)Started .*\\.service|Failed with result|Main process exited"`, baseSelector)

	// Failed SSH auth
	sshQ := fmt.Sprintf(`%s | SYSLOG_IDENTIFIER="sshd" |~ "(?i)Failed password|authentication failure"`, baseSelector)

	// Successful SSH auth
	sshOKQ := fmt.Sprintf(`%s | SYSLOG_IDENTIFIER="sshd" |~ "(?i)Accepted publickey|Accepted password"`, baseSelector)

	// OOM kills
	oomQ := fmt.Sprintf(`%s | SYSLOG_IDENTIFIER="kernel" |~ "(?i)Out of memory: Killed process"`, baseSelector)

	// Disk I/O errors
	diskErrQ := fmt.Sprintf(`%s |~ "(?i)I/O error|EXT4-fs error|ata.*error"`, baseSelector)

	startSec := float64(startNs) / 1e9
	endSec := float64(endNs) / 1e9

	var errorRate, warningRate, totalRate []LokiMatrixSeries
	var restartEntries, sshEntries, sshOKEntries, oomEntries, diskErrEntries []LokiLogEntry

	var wg sync.WaitGroup
	wg.Add(7)

	go func() {
		defer wg.Done()
		errorRate, _ = s.base.lokiQueryRange(ctx, tenantUUID, errorQ, startNs, endNs, step, 1000)
	}()
	go func() {
		defer wg.Done()
		warningRate, _ = s.base.lokiQueryRange(ctx, tenantUUID, warningQ, startNs, endNs, step, 1000)
	}()
	go func() {
		defer wg.Done()
		totalRate, _ = s.base.lokiQueryRange(ctx, tenantUUID, totalQ, startNs, endNs, step, 1000)
	}()
	go func() {
		defer wg.Done()
		restartEntries, _ = s.base.lokiQueryLogEntries(ctx, tenantUUID, restartQ, startNs, endNs, 500)
	}()
	go func() {
		defer wg.Done()
		sshEntries, _ = s.base.lokiQueryLogEntries(ctx, tenantUUID, sshQ, startNs, endNs, 500)
	}()
	go func() {
		defer wg.Done()
		sshOKEntries, _ = s.base.lokiQueryLogEntries(ctx, tenantUUID, sshOKQ, startNs, endNs, 500)
	}()
	go func() {
		defer wg.Done()
		oomEntries, _ = s.base.lokiQueryLogEntries(ctx, tenantUUID, oomQ, startNs, endNs, 100)
	}()
	wg.Wait()

	// Also fetch disk errors (sequential, rare)
	diskErrEntries, _ = s.base.lokiQueryLogEntries(ctx, tenantUUID, diskErrQ, startNs, endNs, 200)

	// Merge error + warning rate into time buckets
	errorBuckets := mergeRateBuckets(errorRate, warningRate, totalRate, startSec, endSec, step)

	// Parse restart events
	restarts := make([]LogHealthRestartEvent, 0, len(restartEntries))
	for _, e := range restartEntries {
		ts, _ := strconv.ParseInt(e.TimestampNs, 10, 64)
		line := e.Line
		isFailed := strings.Contains(strings.ToLower(line), "failed with result") ||
			strings.Contains(strings.ToLower(line), "main process exited")
		unit := extractUnitFromLine(line)
		restarts = append(restarts, LogHealthRestartEvent{
			Time:    ts / 1e6,
			Unit:    unit,
			Failed:  isFailed,
			Message: truncate(line, 200),
		})
	}

	// Parse SSH attempts into buckets
	sshBuckets := bucketSSHAttempts(sshEntries, sshOKEntries, startSec, endSec, step)

	// Parse OOM events
	ooms := make([]LogHealthOOMEvent, 0, len(oomEntries))
	for _, e := range oomEntries {
		ts, _ := strconv.ParseInt(e.TimestampNs, 10, 64)
		ooms = append(ooms, LogHealthOOMEvent{
			Time:    ts / 1e6,
			Message: truncate(e.Line, 300),
		})
	}

	// Parse disk errors into buckets
	diskBuckets := bucketDiskErrors(diskErrEntries, startSec, endSec, step)

	// Compute totals
	totalErrors := 0
	for _, b := range errorBuckets {
		totalErrors += b.Errors
	}
	totalWarnings := 0
	for _, b := range errorBuckets {
		totalWarnings += b.Warnings
	}
	totalRestarts := 0
	for _, r := range restarts {
		if r.Failed {
			totalRestarts++
		}
	}

	result := &LogHealthSummary{
		ErrorRate:     errorBuckets,
		RestartEvents: restarts,
		SSHAttempts:   sshBuckets,
		OOMKills:      ooms,
		DiskErrors:    diskBuckets,
		TotalErrors:   totalErrors,
		TotalWarnings: totalWarnings,
		TotalRestarts: totalRestarts,
		TotalOOMKills: len(ooms),
	}

	if s.redis != nil {
		if b, err := json.Marshal(result); err == nil {
			_ = s.redis.Set(ctx, cacheKey, b, 20*time.Second).Err()
		}
	}

	return result, nil
}

func mergeRateBuckets(
	errors, warnings, totals []LokiMatrixSeries,
	startSec, endSec float64,
	step string,
) []LogHealthTimeBucket {
	errMap := make(map[int64]int)
	for _, series := range errors {
		for _, v := range series.Values {
			ts := int64(v[0])
			errMap[ts] = int(v[1])
		}
	}
	warnMap := make(map[int64]int)
	for _, series := range warnings {
		for _, v := range series.Values {
			ts := int64(v[0])
			warnMap[ts] = int(v[1])
		}
	}
	totalMap := make(map[int64]int)
	for _, series := range totals {
		for _, v := range series.Values {
			ts := int64(v[0])
			totalMap[ts] = int(v[1])
		}
	}

	tsSet := make(map[int64]struct{})
	for ts := range errMap {
		tsSet[ts] = struct{}{}
	}
	for ts := range warnMap {
		tsSet[ts] = struct{}{}
	}
	for ts := range totalMap {
		tsSet[ts] = struct{}{}
	}

	// Loki matrix timestamps are in seconds (Prometheus standard).
	// Convert to milliseconds so the frontend `new Date(time)` works directly.
	timestamps := make([]int64, 0, len(tsSet))
	for ts := range tsSet {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })

	buckets := make([]LogHealthTimeBucket, 0, len(timestamps))
	for _, ts := range timestamps {
		buckets = append(buckets, LogHealthTimeBucket{
			Time:     ts * 1000,
			Errors:   errMap[ts],
			Warnings: warnMap[ts],
			Total:    totalMap[ts],
		})
	}
	return buckets
}

func bucketSSHAttempts(
	failed, ok []LokiLogEntry,
	startSec, endSec float64,
	step string,
) []LogHealthSSHBucket {
	stepDur := parseStepDuration(step)
	buckets := make([]LogHealthSSHBucket, 0)
	for t := startSec; t < endSec; t += stepDur {
		buckets = append(buckets, LogHealthSSHBucket{Time: int64(t * 1000)})
	}

	for _, e := range failed {
		ts, _ := strconv.ParseInt(e.TimestampNs, 10, 64)
		sec := float64(ts) / 1e9
		idx := int((sec - startSec) / stepDur)
		if idx >= 0 && idx < len(buckets) {
			buckets[idx].Failed++
		}
	}
	for _, e := range ok {
		ts, _ := strconv.ParseInt(e.TimestampNs, 10, 64)
		sec := float64(ts) / 1e9
		idx := int((sec - startSec) / stepDur)
		if idx >= 0 && idx < len(buckets) {
			buckets[idx].OK++
		}
	}
	return buckets
}

func bucketDiskErrors(
	entries []LokiLogEntry,
	startSec, endSec float64,
	step string,
) []LogHealthDiskErrorBucket {
	stepDur := parseStepDuration(step)
	buckets := make([]LogHealthDiskErrorBucket, 0)
	for t := startSec; t < endSec; t += stepDur {
		buckets = append(buckets, LogHealthDiskErrorBucket{Time: int64(t * 1000)})
	}

	for _, e := range entries {
		ts, _ := strconv.ParseInt(e.TimestampNs, 10, 64)
		sec := float64(ts) / 1e9
		idx := int((sec - startSec) / stepDur)
		if idx >= 0 && idx < len(buckets) {
			buckets[idx].Count++
		}
	}
	return buckets
}

func parseStepDuration(step string) float64 {
	if strings.HasSuffix(step, "m") {
		val, err := strconv.ParseFloat(strings.TrimSuffix(step, "m"), 64)
		if err != nil {
			return 60
		}
		return val * 60
	}
	step = strings.TrimSuffix(step, "s")
	val, err := strconv.ParseFloat(step, 64)
	if err != nil {
		return 30
	}
	return val
}

func extractUnitFromLine(line string) string {
	// Try to extract unit name from systemd log lines
	// e.g. "Started nginx.service" -> "nginx.service"
	lower := strings.ToLower(line)
	for _, prefix := range []string{"started ", "finished ", "stopped "} {
		if idx := strings.Index(lower, prefix); idx >= 0 {
			rest := line[idx+len(prefix):]
			rest = strings.TrimSpace(rest)
			if dotIdx := strings.Index(rest, " "); dotIdx > 0 {
				return rest[:dotIdx]
			}
			return rest
		}
	}
	if idx := strings.Index(lower, "failed with result"); idx >= 0 {
		// Look backwards for the unit name
		prefix := line[:idx]
		prefix = strings.TrimSpace(prefix)
		if spaceIdx := strings.LastIndex(prefix, " "); spaceIdx >= 0 {
			return prefix[spaceIdx+1:]
		}
		return prefix
	}
	return ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func (s *HostLogsService) cacheLogsPage(ctx context.Context, cacheKey string, result *HostLogsPage) {
	if s.redis == nil {
		return
	}
	if b, err := json.Marshal(result); err == nil {
		_ = s.redis.Set(ctx, cacheKey, b, 15*time.Second).Err()
	}
}

func (s *HostLogsService) queryAllHostLogEntries(ctx context.Context, tenantUUID, query string, startNs, endNs int64) ([]LokiLogEntry, error) {
	const batchSize = 2000
	const maxEntries = 20000

	var all []LokiLogEntry
	seen := make(map[string]struct{}, 4096)

	cursorEnd := endNs
	for len(all) < maxEntries && cursorEnd >= startNs {
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

	if len(all) > maxEntries {
		all = all[:maxEntries]
	}

	sortLokiLogEntriesDesc(all)
	return all, nil
}

// ─────────────────────────────────────────────────────────────
// CLEAN PARSER - Remove internal observability metadata
// ─────────────────────────────────────────────────────────────
func parseHostLogEvent(entry LokiLogEntry) HostLogEvent {
	raw := map[string]string{}
	lineTrim := strings.TrimSpace(entry.Line)

	if strings.HasPrefix(lineTrim, "{") {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(lineTrim), &parsed); err == nil {
			for key, value := range parsed {
				raw[key] = fmt.Sprint(value)
			}
		}
	}

	// Clean message extraction
	message := strings.TrimSpace(raw["MESSAGE"])
	if message == "" {
		message = firstNonEmpty(raw["msg"], raw["message"], raw["log"], entry.Line)
	}

	unit := firstNonEmpty(raw["_SYSTEMD_UNIT"], raw["UNIT"], entry.Metadata["_SYSTEMD_UNIT"], entry.Metadata["systemd_unit"])
	identifier := firstNonEmpty(raw["SYSLOG_IDENTIFIER"], raw["_COMM"], entry.Metadata["SYSLOG_IDENTIFIER"], entry.Metadata["syslog_identifier"])
	priority := firstNonEmpty(raw["PRIORITY"], entry.Metadata["PRIORITY"], entry.Metadata["priority"])

	// Clean Raw map - remove internal observability fields
	cleanRaw := cleanHostLogRaw(raw, entry.Metadata)

	return HostLogEvent{
		Timestamp:   formatUnixNs(entry.TimestampNs),
		TimestampNs: entry.TimestampNs,
		Message:     message,
		Priority:    priority,
		Severity:    journaldSeverity(priority),
		Unit:        unit,
		Identifier:  identifier,
		PID:         firstNonEmpty(raw["_PID"], entry.Metadata["_PID"], entry.Metadata["pid"]),
		Transport:   firstNonEmpty(raw["_TRANSPORT"], entry.Metadata["_TRANSPORT"]),
		Command:     firstNonEmpty(raw["_CMDLINE"], entry.Metadata["_CMDLINE"], entry.Metadata["cmdline"]),
		Raw:         cleanRaw,
		Labels:      entry.Metadata, // Keep full labels for debugging if needed
	}
}

// Removes noisy internal fields from Raw
func cleanHostLogRaw(raw, metadata map[string]string) map[string]string {
	exclude := map[string]bool{
		"tenant_id":             true,
		"agent_version":         true,
		"detected_level":        true,
		"host_id":               true,
		"job":                   true,
		"observed_timestamp":    true,
		"__CURSOR":              true,
		"__MONOTONIC_TIMESTAMP": true,
		"__SEQNUM":              true,
		"__SEQNUM_ID":           true,
		"service_name":          true,
	}

	out := make(map[string]string)
	for k, v := range raw {
		if !exclude[k] && !strings.HasPrefix(k, "__") {
			out[k] = v
		}
	}
	for k, v := range metadata {
		if !exclude[k] && !strings.HasPrefix(k, "__") && out[k] == "" {
			out[k] = v
		}
	}
	return out
}

func mergeHostLogRaw(raw, metadata map[string]string) map[string]string {
	// Deprecated - we now use cleanHostLogRaw
	return cleanHostLogRaw(raw, metadata)
}

func buildHostLogsSummary(events []HostLogEvent) HostLogsSummary {
	summary := HostLogsSummary{
		Total:   len(events),
		ByLevel: map[string]int{},
		ByUnit:  map[string]int{},
		Units:   []string{},
	}
	unitSet := map[string]struct{}{}

	for i, event := range events {
		level := event.Severity
		if level == "" {
			level = "unknown"
		}
		summary.ByLevel[level]++

		if event.Unit != "" {
			summary.ByUnit[event.Unit]++
			unitSet[event.Unit] = struct{}{}
		}

		if i == 0 {
			summary.LastSeen = event.Timestamp
		}
		summary.FirstSeen = event.Timestamp
	}

	for unit := range unitSet {
		summary.Units = append(summary.Units, unit)
	}
	sort.Strings(summary.Units)
	return summary
}

func hostLogMatchesSearch(event HostLogEvent, search string) bool {
	needle := strings.ToLower(strings.TrimSpace(search))
	if needle == "" {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		event.Message,
		event.Unit,
		event.Identifier,
		event.PID,
		event.Transport,
		event.Command,
	}, " "))

	if strings.Contains(haystack, needle) {
		return true
	}

	for key, value := range event.Raw {
		if strings.Contains(strings.ToLower(key+" "+value), needle) {
			return true
		}
	}
	return false
}

func isLowValueHostLogNoise(event HostLogEvent) bool {
	if priority, err := strconv.Atoi(strings.TrimSpace(event.Priority)); err == nil && priority <= 3 {
		return false
	}

	unit := strings.TrimSpace(event.Unit)
	identifier := strings.TrimSpace(event.Identifier)
	transport := strings.TrimSpace(event.Transport)

	noisyUnits := map[string]bool{
		"systemd-udevd.service":     true,
		"systemd-resolved.service":  true,
		"systemd-timesyncd.service": true,
	}
	if noisyUnits[unit] {
		return true
	}

	return identifier == "kernel" || transport == "kernel"
}

func journaldSeverity(priority string) string {
	switch strings.TrimSpace(priority) {
	case "0":
		return "emergency"
	case "1":
		return "alert"
	case "2":
		return "critical"
	case "3":
		return "error"
	case "4":
		return "warning"
	case "5":
		return "notice"
	case "6":
		return "info"
	case "7":
		return "debug"
	default:
		return "unknown"
	}
}

func formatUnixNs(ns string) string {
	value, err := strconv.ParseInt(ns, 10, 64)
	if err != nil {
		return ns
	}
	return time.Unix(0, value).Format("2006-01-02 15:04:05.000")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
