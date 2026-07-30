package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

// ─── Response DTOs ───────────────────────────────────────────────────────────

type OverviewResponse struct {
	Global    GlobalHealth   `json:"global"`
	Incidents []IncidentCard `json:"incidents"`
	Hosts     []HostOverview `json:"hosts"`
	Timeline  TimelineData   `json:"timeline"`
}

type GlobalHealth struct {
	HostsOnline       int `json:"hosts_online"`
	HostsTotal        int `json:"hosts_total"`
	ServicesHealthy   int `json:"services_healthy"`
	ServicesDegraded  int `json:"services_degraded"`
	ServicesDown      int `json:"services_down"`
	ActiveSignals     int `json:"active_signals"`
	ActiveCritical    int `json:"active_critical"`
	ActiveWarning     int `json:"active_warning"`
	ContainersHealthy int `json:"containers_healthy"`
	ContainersTotal   int `json:"containers_total"`
	CVEExposure       int `json:"cve_exposure"`
}

type HostOverview struct {
	MachineID string        `json:"machine_id"`
	Hostname  string        `json:"hostname"`
	OS        string        `json:"os"`
	Status    string        `json:"status"`
	LastSeen  string        `json:"last_seen"`
	CPUPct    *float64      `json:"cpu_pct,omitempty"`
	MemPct    *float64      `json:"mem_pct,omitempty"`
	DiskPct   *float64      `json:"disk_pct,omitempty"`
	Summary   string        `json:"summary,omitempty"`
	Services  []ServiceChip `json:"services"`
}

type ServiceChip struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Port   *int   `json:"port,omitempty"`
}

type TimelineData struct {
	Events []TimelineEvent `json:"events"`
}

type TimelineEvent struct {
	Timestamp string `json:"timestamp"`
	Hostname  string `json:"hostname"`
	Service   string `json:"service"`
	EventType string `json:"event_type"`
	Message   string `json:"message"`
}

// ─── Service ─────────────────────────────────────────────────────────────────

type OverviewService struct {
	cfg             *config.Config
	redis           *redis.Client
	httpClient      *http.Client
	hostRepo        *repository.HostRepository
	discRepo        *repository.DiscoveryRepository
	alertRuleRepo   *repository.AlertRuleRepository
	alertsRepo      *repository.AlertsRepository
	incidentGrouper *IncidentGroupingService
}

func NewOverviewService(
	cfg *config.Config,
	redisClient *redis.Client,
	hostRepo *repository.HostRepository,
	discRepo *repository.DiscoveryRepository,
	alertRuleRepo *repository.AlertRuleRepository,
	alertsRepo *repository.AlertsRepository,
	incidentGrouper *IncidentGroupingService,
) *OverviewService {
	return &OverviewService{
		cfg:             cfg,
		redis:           redisClient,
		httpClient:      &http.Client{Timeout: 45 * time.Second},
		hostRepo:        hostRepo,
		discRepo:        discRepo,
		alertRuleRepo:   alertRuleRepo,
		alertsRepo:      alertsRepo,
		incidentGrouper: incidentGrouper,
	}
}

// GetOverview builds the full overview + correlation response for a tenant.
// timelineStart and timelineEnd are optional nanosecond epoch values; if zero,
// a rolling 5-minute window is used for the incident timeline.
func (s *OverviewService) GetOverview(
	ctx context.Context, tenantID uint, tenantUUID string,
	timelineStartNs, timelineEndNs int64,
) (*OverviewResponse, error) {
	hosts, err := s.hostRepo.ListByTenantCtx(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list hosts: %w", err)
	}

	services, err := s.discRepo.ListServicesByTenantCtx(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}

	now := time.Now()
	global := GlobalHealth{}

	if containers, err := s.discRepo.ListContainerServicesByTenantCtx(ctx, tenantID); err == nil {
		global.ContainersTotal = len(containers)
		for _, c := range containers {
			if now.Sub(c.LastSeenAt) <= 7*time.Minute {
				global.ContainersHealthy++
			}
		}
	}
	if s.alertRuleRepo != nil {
		if counts, err := s.alertRuleRepo.CountFiringEventsByTenantCtx(ctx, tenantID); err == nil {
			global.ActiveSignals = counts.Total
			global.ActiveCritical = counts.Critical
			global.ActiveWarning = counts.Warning
		}
	}
	if s.alertsRepo != nil {
		if cveExposure, err := s.alertsRepo.CountHighRiskVulnerableServicesCtx(ctx, tenantID); err == nil {
			global.CVEExposure = cveExposure
		}
	}

	svcsByHost := map[string][]serviceBrief{}
	for _, svc := range services {
		svcsByHost[svc.MachineID] = append(svcsByHost[svc.MachineID], serviceBrief{
			Name:       svc.ServiceName,
			Port:       svc.Port,
			LogCollOn:  svc.LogCollectionOn,
			MetricColl: svc.MetricCollectionOn,
		})
	}

	type hostBrief struct {
		machineID string
		hostname  string
		osName    string
		lastSeen  time.Time
	}
	hostBriefs := make([]hostBrief, 0, len(hosts))
	for _, h := range hosts {
		hostBriefs = append(hostBriefs, hostBrief{
			machineID: h.MachineID,
			hostname:  h.Hostname,
			osName:    h.OSName,
			lastSeen:  h.LastSeenAt,
		})
	}

	// Timeline window: default to 5min if not specified.
	if timelineStartNs == 0 || timelineEndNs == 0 {
		timelineEndNs = now.UnixNano()
		timelineStartNs = timelineEndNs - int64(5*time.Minute)
	}

	var (
		hostCards []HostOverview
		timeline  TimelineData
		mu        sync.Mutex
		wg        sync.WaitGroup
	)

	timeSec := now.Unix()
	timeNs := now.UnixNano()

	for _, hb := range hostBriefs {
		wg.Add(1)
		go func(hb hostBrief) {
			defer wg.Done()

			online := now.Sub(hb.lastSeen) < 5*time.Minute
			hostStatus := "offline"
			if online {
				hostStatus = "online"
			}

			cpu := s.queryVMInstantSafe(ctx, tenantID,
				fmt.Sprintf(`(1 - avg(system_cpu_utilization_ratio{host_id="%s",state="idle"}) > 0) * 100`, hb.machineID), timeSec)
			mem := s.queryVMInstantSafe(ctx, tenantID,
				fmt.Sprintf(`sum(system_memory_usage_bytes{host_id="%s",state="used"}) / sum(system_memory_usage_bytes{host_id="%s",state=~"used|free|buffered|cached"}) * 100`, hb.machineID, hb.machineID), timeSec)
			disk := s.queryVMInstantSafe(ctx, tenantID,
				fmt.Sprintf(`system_filesystem_utilization_ratio{host_id="%s",mountpoint="/"} * 100`, hb.machineID), timeSec)

			chips := s.getServiceChips(ctx, tenantID, tenantUUID, hb.machineID, svcsByHost[hb.machineID], timeSec, timeNs-5*time.Minute.Nanoseconds(), timeNs)
			healthyCount, degradedCount, downCount := 0, 0, 0
			summaryParts := []string{}
			for _, c := range chips {
				switch c.Status {
				case "healthy":
					healthyCount++
				case "degraded":
					degradedCount++
					summaryParts = append(summaryParts, fmt.Sprintf("%s: degraded", c.Name))
				case "down":
					downCount++
					summaryParts = append(summaryParts, fmt.Sprintf("%s: down", c.Name))
				}
			}
			if !online {
				summaryParts = append(summaryParts, "host offline")
				downCount += len(chips)
			}
			summary := ""
			if len(summaryParts) > 0 {
				summary = strings.Join(summaryParts, ", ")
			}

			// Build timeline events from logs using the requested window.
			hostTimeline := s.getTimelineEvents(ctx, tenantUUID, hb.machineID, hb.hostname, svcsByHost[hb.machineID], timelineStartNs, timelineEndNs)

			mu.Lock()
			global.HostsTotal++
			if online {
				global.HostsOnline++
			}
			global.ServicesHealthy += healthyCount
			global.ServicesDegraded += degradedCount
			global.ServicesDown += downCount

			hostCards = append(hostCards, HostOverview{
				MachineID: hb.machineID,
				Hostname:  hb.hostname,
				OS:        hb.osName,
				Status:    hostStatus,
				LastSeen:  hb.lastSeen.Format(time.RFC3339),
				CPUPct:    cpu,
				MemPct:    mem,
				DiskPct:   disk,
				Summary:   summary,
				Services:  chips,
			})
			timeline.Events = append(timeline.Events, hostTimeline...)
			mu.Unlock()
		}(hb)
	}
	wg.Wait()

	// Sort timeline: newest first.
	sort.Slice(timeline.Events, func(i, j int) bool {
		return timeline.Events[i].Timestamp > timeline.Events[j].Timestamp
	})
	if len(timeline.Events) > 100 {
		timeline.Events = timeline.Events[:100]
	}

	incidents := []IncidentCard{}
	if s.incidentGrouper != nil {
		if grouped, err := s.incidentGrouper.GroupIncidents(ctx, tenantID); err == nil {
			incidents = grouped
		} else {
			slog.Warn("incident grouping failed", "tenant_id", tenantID, "error", err)
		}
	}

	return &OverviewResponse{
		Global:    global,
		Incidents: incidents,
		Hosts:     hostCards,
		Timeline:  timeline,
	}, nil
}


// ─── Service health ───────────────────────────────────────────────────────────

type serviceBrief struct {
	Name       string
	Port       *int
	LogCollOn  bool
	MetricColl bool
}

var supportedServiceKinds = map[string]bool{
	"nginx":      true,
	"apache":     true,
	"phpfpm":     true,
	"mariadb":    true,
	"postgresql": true,
	"redis":      true,
	"container":  true,
}

func serviceKind(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch {
	case n == "nginx":
		return "nginx"
	case n == "apache" || n == "apache2" || n == "httpd":
		return "apache"
	case n == "phpfpm" || strings.Contains(n, "fpm"):
		return "phpfpm"
	case n == "mariadb" || n == "mysql":
		return "mariadb"
	case n == "postgres" || n == "postgresql" || strings.HasPrefix(n, "postgresql"):
		return "postgresql"
	case n == "redis" || n == "redis-server":
		return "redis"
	default:
		return n
	}
}

func isSupportedService(name string) bool {
	return supportedServiceKinds[serviceKind(name)]
}

func (s *OverviewService) getServiceChips(
	ctx context.Context, tenantID uint, tenantUUID, machineID string,
	svcs []serviceBrief, timeSec int64, startNs, endNs int64,
) []ServiceChip {
	chips := make([]ServiceChip, 0, len(svcs))
	for _, svc := range svcs {
		if !isSupportedService(svc.Name) {
			continue
		}
		status := s.getServiceHealth(ctx, tenantID, tenantUUID, machineID, svc, timeSec, startNs, endNs)
		chips = append(chips, ServiceChip{
			Name:   svc.Name,
			Status: status,
			Port:   svc.Port,
		})
	}
	return chips
}

func (s *OverviewService) getServiceHealth(
	ctx context.Context, tenantID uint, tenantUUID, machineID string,
	svc serviceBrief, timeSec int64, startNs, endNs int64,
) string {
	if !svc.MetricColl {
		errCount := s.queryLogErrorCount(ctx, tenantUUID, machineID, svc.Name, startNs, endNs)
		if errCount > 10 {
			return "degraded"
		}
		return "unknown"
	}

	switch kind := serviceKind(svc.Name); kind {
	case "phpfpm":
		up := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`phpfpm_up{host_id="%s"}`, machineID), timeSec)
		if up != nil && *up == 0 {
			return "down"
		}
		sat := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`phpfpm_active_processes{host_id="%s"} / clamp_min(phpfpm_total_processes{host_id="%s"}, 1) > 0.85`, machineID, machineID), timeSec)
		if sat != nil && *sat > 0.85 {
			return "degraded"
		}
	case "mariadb":
		up := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`mysql_up{host_id="%s"}`, machineID), timeSec)
		if up != nil && *up == 0 {
			return "down"
		}
		bp := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`rate(mysql_global_status_innodb_buffer_pool_reads{host_id="%s"}[5m]) / rate(mysql_global_status_innodb_buffer_pool_read_requests{host_id="%s"}[5m]) > 0.05`, machineID, machineID), timeSec)
		if bp != nil && *bp > 0.05 {
			return "degraded"
		}
	case "postgresql":
		up := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`pg_up{host_id="%s"}`, machineID), timeSec)
		if up != nil && *up == 0 {
			return "down"
		}
		conn := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`pg_stat_activity_count{host_id="%s",datname!~"template.*|postgres"} > 80`, machineID), timeSec)
		if conn != nil && *conn > 80 {
			return "degraded"
		}
	case "redis":
		up := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`redis_up{host_id="%s"}`, machineID), timeSec)
		if up != nil && *up == 0 {
			return "down"
		}
		hitRatio := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`rate(redis_keyspace_hits_total{host_id="%s"}[5m]) / (rate(redis_keyspace_hits_total{host_id="%s"}[5m]) + rate(redis_keyspace_misses_total{host_id="%s"}[5m])) < 0.9`, machineID, machineID, machineID), timeSec)
		if hitRatio != nil && *hitRatio < 0.9 {
			return "degraded"
		}
	case "nginx":
		total := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`nginx_requests_total{host_id="%s"}`, machineID), timeSec)
		if total == nil {
			return "down"
		}
	case "apache":
		total := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`apache_requests_total{host_id="%s"}`, machineID), timeSec)
		if total == nil {
			return "down"
		}
	}

	errCount := s.queryLogErrorCount(ctx, tenantUUID, machineID, svc.Name, startNs, endNs)
	if errCount > 50 {
		return "degraded"
	}

	return "healthy"
}

// ─── Timeline signal profiles ────────────────────────────────────────────────

type timelineSignalProfile struct {
	Label          string
	LinePattern    string
	ErrorPatterns   []*regexp.Regexp
	WarningPatterns []*regexp.Regexp
}

var timelineProfiles = map[string]timelineSignalProfile{
	"nginx": {
		Label:       "nginx",
		LinePattern: `(?i)(\[[a-z]*error\]|\b(emerg|alert|crit|error|warn)\b|" [45][0-9]{2} | [45][0-9]{2} )`,
		ErrorPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(\[[a-z]*error\]|\b(emerg|alert|crit|error)\b|" 5[0-9]{2} | 5[0-9]{2} )`),
		},
		WarningPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(\bwarn\b|" 4[0-9]{2} | 4[0-9]{2} )`),
		},
	},
	"apache": {
		Label:       "apache",
		LinePattern: `(?i)(\[[^]]*:(emerg|alert|crit|error|warn)\]|\b(emerg|alert|crit|error|warn)\b|" [45][0-9]{2} | [45][0-9]{2} )`,
		ErrorPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(\[[^]]*:(emerg|alert|crit|error)\]|\b(emerg|alert|crit|error)\b|" 5[0-9]{2} | 5[0-9]{2} )`),
		},
		WarningPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(\[[^]]*:warn\]|\bwarn\b|" 4[0-9]{2} | 4[0-9]{2} )`),
		},
	},
	"phpfpm": {
		Label:       "phpfpm",
		LinePattern: `(?i)(ERROR|WARNING|ALERT|FATAL|failed|timeout|max_children|child exited|segmentation fault)`,
		ErrorPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(ERROR|ALERT|FATAL|failed|timeout|child exited|segmentation fault)`),
		},
		WarningPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(WARNING|max_children)`),
		},
	},
	"mariadb": {
		Label:       "mariadb",
		LinePattern: `(?i)(\[ERROR\]|\[Warning\]|ERROR|Fatal|deadlock|aborted connection|crash|slow query|Query_time)`,
		ErrorPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(\[ERROR\]|ERROR|Fatal|deadlock|crash|aborted connection)`),
		},
		WarningPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(\[Warning\]|warning|slow query|Query_time)`),
		},
	},
	"postgresql": {
		Label:       "postgresql",
		LinePattern: `(?i)(ERROR:|FATAL:|PANIC:|WARNING:|deadlock detected|duration:|slow query|terminating connection|could not)`,
		ErrorPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(ERROR:|FATAL:|PANIC:|deadlock detected|terminating connection|could not)`),
		},
		WarningPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(WARNING:|duration:|slow query)`),
		},
	},
	"redis": {
		Label:       "redis",
		LinePattern: `(?i)(error|fatal|OOM|MISCONF|BUSY|LOADING|warning|slow|blocked|WAIT)`,
		ErrorPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(error|fatal|OOM|MISCONF|BUSY|LOADING)`),
		},
		WarningPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(warning|slow|blocked|WAIT)`),
		},
	},
	"host": {
		Label:       "host",
		LinePattern: `(?i)(error|failed|failure|panic|oom|killed|denied|segfault|critical|warning|timeout)`,
		ErrorPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(error|failed|failure|panic|oom|killed|denied|segfault|critical)`),
		},
		WarningPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(warning|timeout)`),
		},
	},
	"container": {
		Label:       "container",
		LinePattern: `(?i)(error|failed|failure|exception|panic|oom|killed|crash|warning|timeout|unhealthy)`,
		ErrorPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(error|failed|failure|exception|panic|oom|killed|crash)`),
		},
		WarningPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(warning|timeout|unhealthy)`),
		},
	},
}

func serviceLogQuery(service, machineID string) string {
	profile, ok := timelineProfiles[serviceKind(service)]
	if !ok {
		return fmt.Sprintf(`{service_name=%q, host_id=%q} |~ "(?i)(error|warn|failed)"`, service, machineID)
	}
	return fmt.Sprintf(`{service_name=%q, host_id=%q} |~ "%s"`, service, machineID, escapeLokiString(profile.LinePattern))
}

func hostLogQuery(machineID string) string {
	profile := timelineProfiles["host"]
	return fmt.Sprintf(`{service_name="journald"} | host_id=%q |~ "%s"`, machineID, escapeLokiString(profile.LinePattern))
}

func escapeLokiString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

func classifyTimelineEvent(kind, line string) string {
	profile, ok := timelineProfiles[kind]
	if !ok {
		profile = timelineProfiles[serviceKind(kind)]
	}
	for _, pattern := range profile.ErrorPatterns {
		if pattern.MatchString(line) {
			return "error"
		}
	}
	for _, pattern := range profile.WarningPatterns {
		if pattern.MatchString(line) {
			return "warning"
		}
	}
	return "warning"
}

// ─── Timeline events ─────────────────────────────────────────────────────────

func (s *OverviewService) getTimelineEvents(
	ctx context.Context, tenantUUID, machineID, hostname string,
	svcs []serviceBrief, startNs, endNs int64,
) []TimelineEvent {
	var events []TimelineEvent

	appendEntries := func(serviceLabel, kind, query string, limit int) {
		entries, err := s.lokiQueryLogEntries(ctx, tenantUUID, query, startNs, endNs, limit)
		if err != nil {
			slog.Warn("timeline loki query failed", "service", serviceLabel, "host", machineID, "error", err)
			return
		}
		for _, e := range entries {
			tsNs, _ := strconv.ParseInt(e.TimestampNs, 10, 64)
			ts := time.Unix(0, tsNs).UTC().Format(time.RFC3339)
			events = append(events, TimelineEvent{
				Timestamp: ts,
				Hostname:  hostname,
				Service:   serviceLabel,
				EventType: classifyTimelineEvent(kind, e.Line),
				Message:   truncateString(extractTimelineMessage(e.Line), 140),
			})
		}
	}

	appendEntries("host", "host", hostLogQuery(machineID), 12)

	for _, svc := range svcs {
		if !isSupportedService(svc.Name) || !svc.LogCollOn {
			continue
		}
		appendEntries(svc.Name, serviceKind(svc.Name), serviceLogQuery(svc.Name, machineID), 12)
	}
	return events
}

// ─── VM queries ──────────────────────────────────────────────────────────────

func (s *OverviewService) queryVMInstantSafe(ctx context.Context, tenantID uint, query string, timeSec int64) *float64 {
	upstream := fmt.Sprintf("http://%s:%s/select/%d/prometheus/api/v1/query",
		s.cfg.VMSelectHost, s.cfg.VMSelectPort, tenantID)
	params := url.Values{}
	params.Set("query", query)
	params.Set("time", strconv.FormatInt(timeSec, 10))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream+"?"+params.Encode(), nil)
	if err != nil {
		return nil
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	series, err := parseVMInstantResponse(body)
	if err != nil || len(series) == 0 || len(series[0].Values) == 0 {
		return nil
	}
	return &series[0].Values[0][1]
}

func serviceErrorPattern(service string) string {
	switch serviceKind(service) {
	case "nginx", "apache":
		return `(?i)(error|emerg|alert|crit| 5[0-9]{2} )`
	case "phpfpm":
		return `(?i)(error|alert|fatal|timeout|child exited|segmentation)`
	case "mariadb":
		return `(?i)(error|fatal|deadlock|crash|aborted connection)`
	case "postgresql":
		return `(?i)(error:|fatal:|panic:|deadlock detected|terminating connection|could not)`
	case "redis":
		return `(?i)(error|fatal|oom|misconf|busy|loading)`
	default:
		return `error`
	}
}

func (s *OverviewService) queryLogErrorCount(ctx context.Context, tenantUUID, machineID, serviceName string, startNs, endNs int64) float64 {
	pattern := serviceErrorPattern(serviceName)
	var q string
	if pattern == "error" {
		q = fmt.Sprintf(`count_over_time({service_name=%q, host_id=%q} |= "error" [5m])`, serviceName, machineID)
	} else {
		q = fmt.Sprintf(`count_over_time({service_name=%q, host_id=%q} |~ "%s" [5m])`, serviceName, machineID, escapeLokiString(pattern))
	}
	rows, err := s.lokiQueryRange(ctx, tenantUUID, q, startNs, endNs, "5m", 1)
	if err != nil || len(rows) == 0 || len(rows[0].Values) == 0 {
		return 0
	}
	return rows[0].Values[len(rows[0].Values)-1][1]
}

// ─── Loki queries ────────────────────────────────────────────────────────────

func (s *OverviewService) lokiQueryRange(ctx context.Context, tenantUUID, query string, startNs, endNs int64, step string, limit int) ([]LokiMatrixSeries, error) {
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

func (s *OverviewService) lokiQueryLogEntries(ctx context.Context, tenantUUID, query string, startNs, endNs int64, limit int) ([]LokiLogEntry, error) {
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

// ─── Helpers ─────────────────────────────────────────────────────────────────

func extractTimelineMessage(line string) string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "{") {
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err == nil {
			if msg, ok := raw["MESSAGE"].(string); ok && strings.TrimSpace(msg) != "" {
				return strings.TrimSpace(msg)
			}
			if msg, ok := raw["message"].(string); ok && strings.TrimSpace(msg) != "" {
				return strings.TrimSpace(msg)
			}
			if msg, ok := raw["msg"].(string); ok && strings.TrimSpace(msg) != "" {
				return strings.TrimSpace(msg)
			}
		}
	}
	return line
}

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
