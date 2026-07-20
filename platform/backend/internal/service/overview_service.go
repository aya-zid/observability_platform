package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	Hosts     []HostOverview `json:"hosts"`
	TopIssues []TopIssue     `json:"top_issues"`
	Findings  []FindingVO    `json:"findings"`
	Timeline  TimelineData   `json:"timeline"`
	DepGraph  DepGraphData   `json:"dep_graph"`
}

type GlobalHealth struct {
	HostsOnline      int `json:"hosts_online"`
	HostsTotal       int `json:"hosts_total"`
	ServicesHealthy  int `json:"services_healthy"`
	ServicesDegraded int `json:"services_degraded"`
	ServicesDown     int `json:"services_down"`
	ActiveSignals    int `json:"active_signals"`
	OpenFindings     int `json:"open_findings"`
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

type TopIssue struct {
	MachineID string `json:"machine_id"`
	Hostname  string `json:"hostname"`
	Service   string `json:"service"`
	Issue     string `json:"issue"`
	Value     string `json:"value"`
	Severity  string `json:"severity"`
	Age       string `json:"age"`
}

type FindingVO struct {
	ID        uint           `json:"id"`
	RuleID    string         `json:"rule_id"`
	Severity  string         `json:"severity"`
	Title     string         `json:"title"`
	Message   string         `json:"message"`
	Evidence  []EvidenceItem `json:"evidence"`
	FiredAt   string         `json:"fired_at"`
	Hostname  string         `json:"hostname"`
}

type EvidenceItem struct {
	Metric string `json:"metric"`
	Value  string `json:"value"`
	Status string `json:"status"`
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

type DepGraphData struct {
	Nodes []DepNode `json:"nodes"`
	Edges []DepEdge `json:"edges"`
}

type DepNode struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"`
	Type   string `json:"type"`
}

type DepEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Status string `json:"status"`
}

// ─── Service ─────────────────────────────────────────────────────────────────

type OverviewService struct {
	cfg        *config.Config
	redis      *redis.Client
	httpClient *http.Client
	hostRepo   *repository.HostRepository
	discRepo   *repository.DiscoveryRepository
}

func NewOverviewService(
	cfg *config.Config,
	redisClient *redis.Client,
	hostRepo *repository.HostRepository,
	discRepo *repository.DiscoveryRepository,
) *OverviewService {
	return &OverviewService{
		cfg:        cfg,
		redis:      redisClient,
		httpClient: &http.Client{Timeout: 45 * time.Second},
		hostRepo:   hostRepo,
		discRepo:   discRepo,
	}
}

// GetOverview builds the full overview + correlation response for a tenant.
func (s *OverviewService) GetOverview(ctx context.Context, tenantID uint, tenantUUID string) (*OverviewResponse, error) {
	hosts, err := s.hostRepo.ListByTenantCtx(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list hosts: %w", err)
	}

	services, err := s.discRepo.ListServicesByTenantCtx(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}

	now := time.Now()

	// Group services by machine_id.
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

	var (
		global     GlobalHealth
		hostCards  []HostOverview
		topIssues  []TopIssue
		findings   []FindingVO
		timeline   TimelineData
		depGraph   DepGraphData
		mu         sync.Mutex
		wg         sync.WaitGroup
	)

	timeSec := now.Unix()
	timeNs := now.UnixNano()
	startNs := timeNs - int64(5*time.Minute)

	for _, hb := range hostBriefs {
		wg.Add(1)
		go func(hb hostBrief) {
			defer wg.Done()

			online := now.Sub(hb.lastSeen) < 5*time.Minute
			hostStatus := "offline"
			if online {
				hostStatus = "online"
			}

			// Query host-level metrics (best-effort).
			cpu := s.queryVMInstantSafe(ctx, tenantID,
				fmt.Sprintf(`(1 - avg(system_cpu_utilization_ratio{host_id="%s",state="idle"}) > 0) * 100`, hb.machineID), timeSec)
			mem := s.queryVMInstantSafe(ctx, tenantID,
				fmt.Sprintf(`sum(system_memory_usage_bytes{host_id="%s",state="used"}) / sum(system_memory_usage_bytes{host_id="%s",state=~"used|free|buffered|cached"}) * 100`, hb.machineID, hb.machineID), timeSec)
			disk := s.queryVMInstantSafe(ctx, tenantID,
				fmt.Sprintf(`system_filesystem_utilization_ratio{host_id="%s",mountpoint="/"} * 100`, hb.machineID), timeSec)

			// Determine service chips.
			chips := s.getServiceChips(ctx, tenantID, tenantUUID, hb.machineID, svcsByHost[hb.machineID], timeSec, startNs, timeNs)
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
				downCount += len(chips) // all services are down if host is offline
			}
			summary := ""
			if len(summaryParts) > 0 {
				summary = strings.Join(summaryParts, ", ")
			}

			// Generate top issues from log signals.
			issues := s.getLogIssues(ctx, tenantUUID, hb.machineID, hb.hostname, svcsByHost[hb.machineID], startNs, timeNs)
			// Add metric-based issues.
			metricIssues := s.getMetricIssues(ctx, tenantID, hb.machineID, hb.hostname, svcsByHost[hb.machineID], timeSec)
			issues = append(issues, metricIssues...)

			// Run correlation rules for this host.
			hostFindings := s.evaluateCorrelationRules(ctx, tenantID, tenantUUID, hb.machineID, hb.hostname, svcsByHost[hb.machineID], timeSec, startNs, timeNs)
			hostDepNodes, hostDepEdges := s.buildDepGraph(hb.machineID, hb.hostname, chips)

			// Build timeline events from log signals.
			hostTimeline := s.getTimelineEvents(ctx, tenantUUID, hb.machineID, hb.hostname, svcsByHost[hb.machineID], startNs, timeNs)

			mu.Lock()
			global.HostsTotal++
			if online {
				global.HostsOnline++
			}
			global.ServicesHealthy += healthyCount
			global.ServicesDegraded += degradedCount
			global.ServicesDown += downCount
			global.ActiveSignals += len(issues)

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
			topIssues = append(topIssues, issues...)
			findings = append(findings, hostFindings...)
			timeline.Events = append(timeline.Events, hostTimeline...)
			depGraph.Nodes = append(depGraph.Nodes, hostDepNodes...)
			depGraph.Edges = append(depGraph.Edges, hostDepEdges...)
			mu.Unlock()
		}(hb)
	}
	wg.Wait()

	global.OpenFindings = len(findings)
	global.ActiveSignals += len(findings)

	// Sort: newest timeline first.
	sort.Slice(timeline.Events, func(i, j int) bool {
		return timeline.Events[i].Timestamp > timeline.Events[j].Timestamp
	})
	if len(timeline.Events) > 100 {
		timeline.Events = timeline.Events[:100]
	}

	// Sort issues by severity: critical > warning > info.
	sort.Slice(topIssues, func(i, j int) bool {
		return severityWeight(topIssues[i].Severity) > severityWeight(topIssues[j].Severity)
	})
	if len(topIssues) > 20 {
		topIssues = topIssues[:20]
	}

	return &OverviewResponse{
		Global:    global,
		Hosts:     hostCards,
		TopIssues: topIssues,
		Findings:  findings,
		Timeline:  timeline,
		DepGraph:  depGraph,
	}, nil
}

// ─── Service health ───────────────────────────────────────────────────────────

type serviceBrief struct {
	Name       string
	Port       *int
	LogCollOn  bool
	MetricColl bool
}

var supportedServices = map[string]bool{
	"nginx":    true,
	"apache":   true,
	"phpfpm":   true,
	"mariadb":  true,
	"postgres": true,
	"redis":    true,
}

func (s *OverviewService) getServiceChips(
	ctx context.Context, tenantID uint, tenantUUID, machineID string,
	svcs []serviceBrief, timeSec int64, startNs, endNs int64,
) []ServiceChip {
	chips := make([]ServiceChip, 0, len(svcs))
	for _, svc := range svcs {
		if !supportedServices[svc.Name] {
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
		// If metrics not collected, check logs for errors as a fallback.
		errCount := s.queryLogErrorCount(ctx, tenantUUID, machineID, svc.Name, startNs, endNs)
		if errCount > 10 {
			return "degraded"
		}
		return "unknown"
	}

	// Check *_up metric.
	upMetric := fmt.Sprintf(`%s_up{host_id="%s"}`, svc.Name, machineID)
	up := s.queryVMInstantSafe(ctx, tenantID, upMetric, timeSec)
	if up != nil && *up == 0 {
		return "down"
	}

	// Check scrape error.
	scrapeErr := s.queryVMInstantSafe(ctx, tenantID,
		fmt.Sprintf(`%s_scrape_error{host_id="%s"}`, svc.Name, machineID), timeSec)
	if scrapeErr != nil && *scrapeErr > 0 {
		return "down"
	}

	// Service-specific degradation checks.
	switch svc.Name {
	case "phpfpm":
		sat := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`phpfpm_active_workers{host_id="%s"} / phpfpm_total_workers{host_id="%s"} > 0.85`, machineID, machineID), timeSec)
		if sat != nil && *sat > 0.85 {
			return "degraded"
		}
	case "mariadb":
		bp := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`mysql_global_status_innodb_buffer_pool_hit_ratio{host_id="%s"} < 0.95`, machineID), timeSec)
		if bp != nil && *bp < 0.95 {
			return "degraded"
		}
	case "postgres":
		conn := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`pg_stat_activity_count{host_id="%s",datname!~"template.*|postgres"} > 80`, machineID), timeSec)
		if conn != nil && *conn > 80 {
			return "degraded"
		}
	case "redis":
		hitRatio := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`rate(redis_keyspace_hits_total{host_id="%s"}[5m]) / (rate(redis_keyspace_hits_total{host_id="%s"}[5m]) + rate(redis_keyspace_misses_total{host_id="%s"}[5m])) < 0.9`, machineID, machineID, machineID), timeSec)
		if hitRatio != nil && *hitRatio < 0.9 {
			return "degraded"
		}
	case "nginx", "apache":
		err5xx := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`rate(%s_http_requests_total{host_id="%s",status=~"5.."}[5m]) / rate(%s_http_requests_total{host_id="%s"}[5m]) > 0.05`, svc.Name, machineID, svc.Name, machineID), timeSec)
		if err5xx != nil && *err5xx > 0.05 {
			return "degraded"
		}
	}

	// Log-based degradation: check for elevated errors.
	errCount := s.queryLogErrorCount(ctx, tenantUUID, machineID, svc.Name, startNs, endNs)
	if errCount > 50 {
		return "degraded"
	}

	return "healthy"
}

// ─── Log-based issues ─────────────────────────────────────────────────────────

func (s *OverviewService) getLogIssues(
	ctx context.Context, tenantUUID, machineID, hostname string,
	svcs []serviceBrief, startNs, endNs int64,
) []TopIssue {
	var issues []TopIssue
	for _, svc := range svcs {
		if !supportedServices[svc.Name] || !svc.LogCollOn {
			continue
		}

		qErr := fmt.Sprintf(`sum by(service) (count_over_time({service=%q, host_id=%q} |= "error" [5m])) > 0`, svc.Name, machineID)
		rows, err := s.lokiQueryRange(ctx, tenantUUID, qErr, startNs, endNs, "5m", 10)
		if err != nil || len(rows) == 0 {
			continue
		}
		for _, row := range rows {
			if len(row.Values) == 0 {
				continue
			}
			val := row.Values[len(row.Values)-1][1]
			if val > 0 {
				sev := "warning"
				if val > 50 {
					sev = "critical"
				}
				age := "just now"
				issues = append(issues, TopIssue{
					MachineID: machineID,
					Hostname:  hostname,
					Service:   svc.Name,
					Issue:     fmt.Sprintf("Error rate elevated in %s logs", svc.Name),
					Value:     fmt.Sprintf("%.0f/min", val),
					Severity:  sev,
					Age:       age,
				})
			}
		}
	}
	return issues
}

// ─── Metric-based issues ──────────────────────────────────────────────────────

func (s *OverviewService) getMetricIssues(
	ctx context.Context, tenantID uint, machineID, hostname string,
	svcs []serviceBrief, timeSec int64,
) []TopIssue {
	var issues []TopIssue

	for _, svc := range svcs {
		if !supportedServices[svc.Name] || !svc.MetricColl {
			continue
		}

		switch svc.Name {
		case "phpfpm":
			sat := s.queryVMInstantSafe(ctx, tenantID,
				fmt.Sprintf(`phpfpm_active_workers{host_id="%s"} / phpfpm_total_workers{host_id="%s"}`, machineID, machineID), timeSec)
			if sat != nil && *sat > 0.85 {
				issues = append(issues, TopIssue{
					MachineID: machineID,
					Hostname:  hostname,
					Service:   "phpfpm",
					Issue:     "PHP-FPM worker pool saturation",
					Value:     fmt.Sprintf("%.0f%%", *sat*100),
					Severity:  "warning",
					Age:       "just now",
				})
			}
		case "mariadb":
			bp := s.queryVMInstantSafe(ctx, tenantID,
				fmt.Sprintf(`mysql_global_status_innodb_buffer_pool_hit_ratio{host_id="%s"}`, machineID), timeSec)
			if bp != nil && *bp < 0.95 {
				issues = append(issues, TopIssue{
					MachineID: machineID,
					Hostname:  hostname,
					Service:   "mariadb",
					Issue:     "InnoDB buffer pool hit ratio low",
					Value:     fmt.Sprintf("%.1f%%", *bp*100),
					Severity:  "warning",
					Age:       "just now",
				})
			}
			slowQ := s.queryVMInstantSafe(ctx, tenantID,
				fmt.Sprintf(`rate(mysql_global_status_slow_queries{host_id="%s"}[5m])`, machineID), timeSec)
			if slowQ != nil && *slowQ > 0.5 {
				issues = append(issues, TopIssue{
					MachineID: machineID,
					Hostname:  hostname,
					Service:   "mariadb",
					Issue:     "Slow queries detected on MariaDB",
					Value:     fmt.Sprintf("%.1f/s", *slowQ),
					Severity:  "warning",
					Age:       "just now",
				})
			}
		case "postgres":
			slowQ := s.queryVMInstantSafe(ctx, tenantID,
				fmt.Sprintf(`rate(pg_stat_database_conflicts_total{host_id="%s"}[5m])`, machineID), timeSec)
			if slowQ != nil && *slowQ > 0.5 {
				issues = append(issues, TopIssue{
					MachineID: machineID,
					Hostname:  hostname,
					Service:   "postgres",
					Issue:     "Slow queries detected on PostgreSQL",
					Value:     fmt.Sprintf("%.1f/s", *slowQ),
					Severity:  "warning",
					Age:       "just now",
				})
			}
		case "redis":
			hit := s.queryVMInstantSafe(ctx, tenantID,
				fmt.Sprintf(`rate(redis_keyspace_hits_total{host_id="%s"}[5m]) / (rate(redis_keyspace_hits_total{host_id="%s"}[5m]) + rate(redis_keyspace_misses_total{host_id="%s"}[5m])) * 100`, machineID, machineID, machineID), timeSec)
			if hit != nil && *hit < 90 {
				issues = append(issues, TopIssue{
					MachineID: machineID,
					Hostname:  hostname,
					Service:   "redis",
					Issue:     "Redis cache hit ratio low",
					Value:     fmt.Sprintf("%.1f%%", *hit),
					Severity:  "warning",
					Age:       "just now",
				})
			}
		}
	}

	return issues
}

// ─── Correlation rules engine ─────────────────────────────────────────────────

func (s *OverviewService) evaluateCorrelationRules(
	ctx context.Context, tenantID uint, tenantUUID, machineID, hostname string,
	svcs []serviceBrief, timeSec int64, startNs, endNs int64,
) []FindingVO {
	var findings []FindingVO

	svcMap := map[string]serviceBrief{}
	for _, svc := range svcs {
		svcMap[svc.Name] = svc
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Rule 1: Web → App — nginx/apache 5xx + phpfpm saturation
	if s.hasAny(svcMap, "nginx", "apache") && s.hasAny(svcMap, "phpfpm") {
		web5xx := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`rate(nginx_http_requests_total{host_id="%s",status=~"5.."}[5m]) / rate(nginx_http_requests_total{host_id="%s"}[5m]) > 0.05`, machineID, machineID), timeSec)
		if web5xx == nil {
			web5xx = s.queryVMInstantSafe(ctx, tenantID,
				fmt.Sprintf(`rate(httpd_requests_total{host_id="%s",status=~"5.."}[5m]) / rate(httpd_requests_total{host_id="%s"}[5m]) > 0.05`, machineID, machineID), timeSec)
		}
		phpSat := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`phpfpm_active_workers{host_id="%s"} / phpfpm_total_workers{host_id="%s"} > 0.85`, machineID, machineID), timeSec)
		if web5xx != nil && *web5xx > 0.05 && phpSat != nil && *phpSat > 0.85 {
			findings = append(findings, FindingVO{
				RuleID:   "web_to_app",
				Severity: "critical",
				Title:    "Web tier errors correlate with PHP-FPM pool exhaustion",
				Message:  fmt.Sprintf("On %s, web server 5xx errors (%.1f%%) are firing alongside PHP-FPM worker saturation (%.0f%%). This suggests the application pool cannot keep up with request volume.", hostname, *web5xx*100, *phpSat*100),
				Evidence: []EvidenceItem{
					{Metric: "Web 5xx rate", Value: fmt.Sprintf("%.1f%%", *web5xx*100), Status: "critical"},
					{Metric: "PHP-FPM saturation", Value: fmt.Sprintf("%.0f%%", *phpSat*100), Status: "warning"},
				},
				FiredAt:  now,
				Hostname: hostname,
			})
		}
	}

	// Rule 2: App → DB — phpfpm slow + DB slow queries
	if s.hasAny(svcMap, "phpfpm") && (s.hasAny(svcMap, "mariadb") || s.hasAny(svcMap, "postgres")) {
		phpSlow := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`phpfpm_request_duration_seconds{host_id="%s",quantile="0.95"} > 1`, machineID), timeSec)
		var dbSlow *float64
		if s.hasAny(svcMap, "mariadb") {
			dbSlow = s.queryVMInstantSafe(ctx, tenantID,
				fmt.Sprintf(`rate(mysql_global_status_slow_queries{host_id="%s"}[5m]) > 0.5`, machineID), timeSec)
		} else {
			dbSlow = s.queryVMInstantSafe(ctx, tenantID,
				fmt.Sprintf(`rate(pg_stat_database_conflicts_total{host_id="%s"}[5m]) > 0.5`, machineID), timeSec)
		}
		if phpSlow != nil && *phpSlow > 1 && dbSlow != nil && *dbSlow > 0.5 {
			findings = append(findings, FindingVO{
				RuleID:   "app_to_db",
				Severity: "warning",
				Title:    "Slow PHP requests correlate with database slow queries",
				Message:  fmt.Sprintf("On %s, PHP-FPM p95 response time is %.1fs and the database is reporting elevated slow queries. The application is likely DB-bound.", hostname, *phpSlow),
				Evidence: []EvidenceItem{
					{Metric: "PHP-FPM p95 RT", Value: fmt.Sprintf("%.1fs", *phpSlow), Status: "warning"},
					{Metric: "DB slow queries", Value: fmt.Sprintf("%.1f/s", *dbSlow), Status: "warning"},
				},
				FiredAt:  now,
				Hostname: hostname,
			})
		}
	}

	// Rule 3: DB → Host — buffer pool pressure + disk I/O
	if s.hasAny(svcMap, "mariadb") {
		bp := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`mysql_global_status_innodb_buffer_pool_hit_ratio{host_id="%s"} < 0.95`, machineID), timeSec)
		ioWait := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`rate(node_disk_read_time_seconds_total{host_id="%s"}[5m]) > 0.1`, machineID), timeSec)
		if bp != nil && *bp < 0.95 && ioWait != nil && *ioWait > 0.1 {
			findings = append(findings, FindingVO{
				RuleID:   "db_to_host",
				Severity: "warning",
				Title:    "Buffer pool pressure correlates with disk I/O saturation",
				Message:  fmt.Sprintf("On %s, InnoDB buffer pool hit ratio is %.1f%% and disk read I/O is elevated (%.1f/s). Consider increasing innodb_buffer_pool_size or checking disk health.", hostname, *bp*100, *ioWait),
				Evidence: []EvidenceItem{
					{Metric: "Buffer pool hit ratio", Value: fmt.Sprintf("%.1f%%", *bp*100), Status: "warning"},
					{Metric: "Disk read I/O", Value: fmt.Sprintf("%.1f/s", *ioWait), Status: "warning"},
				},
				FiredAt:  now,
				Hostname: hostname,
			})
		}
	}

	// Rule 4: Cache miss cascade — redis miss + DB load
	if s.hasAny(svcMap, "redis") && (s.hasAny(svcMap, "mariadb") || s.hasAny(svcMap, "postgres")) {
		cacheHit := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`rate(redis_keyspace_hits_total{host_id="%s"}[5m]) / (rate(redis_keyspace_hits_total{host_id="%s"}[5m]) + rate(redis_keyspace_misses_total{host_id="%s"}[5m])) < 0.9`, machineID, machineID, machineID), timeSec)
		var dbQueryRate *float64
		if s.hasAny(svcMap, "mariadb") {
			dbQueryRate = s.queryVMInstantSafe(ctx, tenantID,
				fmt.Sprintf(`rate(mysql_global_status_queries{host_id="%s"}[5m])`, machineID), timeSec)
		} else {
			dbQueryRate = s.queryVMInstantSafe(ctx, tenantID,
				fmt.Sprintf(`rate(pg_stat_database_tup_fetched{host_id="%s"}[5m])`, machineID), timeSec)
		}
		if cacheHit != nil && *cacheHit < 0.9 && dbQueryRate != nil && *dbQueryRate > 100 {
			findings = append(findings, FindingVO{
				RuleID:   "cache_miss_cascade",
				Severity: "warning",
				Title:    "Falling cache hit ratio correlates with rising DB load",
				Message:  fmt.Sprintf("On %s, Redis cache hit ratio is %.1f%% while database query rate is high (%.0f/s). The cache may be undersized or evicting too aggressively.", hostname, *cacheHit*100, *dbQueryRate),
				Evidence: []EvidenceItem{
					{Metric: "Cache hit ratio", Value: fmt.Sprintf("%.1f%%", *cacheHit*100), Status: "warning"},
					{Metric: "DB query rate", Value: fmt.Sprintf("%.0f/s", *dbQueryRate), Status: "info"},
				},
				FiredAt:  now,
				Hostname: hostname,
			})
		}
	}

	// Rule 5: Host-wide exhaustion — CPU/mem + multiple degraded services
	if len(svcMap) >= 2 {
		cpuSat := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`(1 - avg(system_cpu_utilization_ratio{host_id="%s",state="idle"}) > 0) * 100 > 90`, machineID), timeSec)
		memSat := s.queryVMInstantSafe(ctx, tenantID,
			fmt.Sprintf(`sum(system_memory_usage_bytes{host_id="%s",state="used"}) / sum(system_memory_usage_bytes{host_id="%s",state=~"used|free|buffered|cached"}) * 100 > 90`, machineID, machineID), timeSec)
		if (cpuSat != nil && *cpuSat > 0) || (memSat != nil && *memSat > 0) {
			degradedCount := 0
			for _, svc := range svcMap {
				if !supportedServices[svc.Name] || !svc.MetricColl {
					continue
				}
				h := s.getServiceHealth(ctx, tenantID, tenantUUID, machineID, svc, timeSec, startNs, endNs)
				if h == "degraded" || h == "down" {
					degradedCount++
				}
			}
			if degradedCount >= 2 {
				findings = append(findings, FindingVO{
					RuleID:   "host_exhaustion",
					Severity: "critical",
					Title:    "Host resource exhaustion affecting multiple services",
					Message:  fmt.Sprintf("On %s, system resources are saturated (CPU >90%% or memory >90%%) and %d services are degraded. This is a host-level issue, not individual service bugs.", hostname, degradedCount),
					Evidence: []EvidenceItem{
						{Metric: "CPU saturation", Value: ">90%", Status: "critical"},
						{Metric: "Memory saturation", Value: ">90%", Status: "critical"},
						{Metric: "Degraded services", Value: fmt.Sprintf("%d", degradedCount), Status: "critical"},
					},
					FiredAt:  now,
					Hostname: hostname,
				})
			}
		}
	}

	return findings
}

// ─── Dependency graph ────────────────────────────────────────────────────────

func (s *OverviewService) buildDepGraph(machineID, hostname string, chips []ServiceChip) ([]DepNode, []DepEdge) {
	nodes := []DepNode{
		{ID: machineID + ":host", Label: hostname, Status: "healthy", Type: "host"},
	}
	var edges []DepEdge

	hasWeb := false
	hasApp := false
	hasDB := false
	hasCache := false

	for _, c := range chips {
		switch c.Name {
		case "nginx", "apache":
			hasWeb = true
			webID := machineID + ":web"
			nodes = append(nodes, DepNode{ID: webID, Label: c.Name, Status: c.Status, Type: "web"})
			edges = append(edges, DepEdge{Source: webID, Target: machineID + ":host", Status: c.Status})
		case "phpfpm":
			hasApp = true
			appID := machineID + ":app"
			nodes = append(nodes, DepNode{ID: appID, Label: c.Name, Status: c.Status, Type: "app"})
			edges = append(edges, DepEdge{Source: appID, Target: machineID + ":host", Status: c.Status})
		case "mariadb", "postgres":
			hasDB = true
			dbID := machineID + ":db"
			nodes = append(nodes, DepNode{ID: dbID, Label: c.Name, Status: c.Status, Type: "db"})
			edges = append(edges, DepEdge{Source: dbID, Target: machineID + ":host", Status: c.Status})
		case "redis":
			hasCache = true
			cacheID := machineID + ":cache"
			nodes = append(nodes, DepNode{ID: cacheID, Label: c.Name, Status: c.Status, Type: "cache"})
			edges = append(edges, DepEdge{Source: cacheID, Target: machineID + ":host", Status: c.Status})
		}
	}

	// Add dependency edges between services.
	if hasWeb && hasApp {
		edges = append(edges, DepEdge{
			Source: machineID + ":web",
			Target: machineID + ":app",
			Status: "healthy",
		})
	}
	if hasApp && hasDB {
		edges = append(edges, DepEdge{
			Source: machineID + ":app",
			Target: machineID + ":db",
			Status: "healthy",
		})
	}
	if hasApp && hasCache {
		edges = append(edges, DepEdge{
			Source: machineID + ":app",
			Target: machineID + ":cache",
			Status: "healthy",
		})
	}

	return nodes, edges
}

// ─── Timeline events ─────────────────────────────────────────────────────────

func (s *OverviewService) getTimelineEvents(
	ctx context.Context, tenantUUID, machineID, hostname string,
	svcs []serviceBrief, startNs, endNs int64,
) []TimelineEvent {
	var events []TimelineEvent
	for _, svc := range svcs {
		if !supportedServices[svc.Name] || !svc.LogCollOn {
			continue
		}
		q := fmt.Sprintf(`{service=%q, host_id=%q} |= "error"`, svc.Name, machineID)
		entries, err := s.lokiQueryLogEntries(ctx, tenantUUID, q, startNs, endNs, 20)
		if err != nil {
			continue
		}
		for _, e := range entries {
			tsNs, _ := strconv.ParseInt(e.TimestampNs, 10, 64)
			ts := time.Unix(0, tsNs).UTC().Format(time.RFC3339)
			events = append(events, TimelineEvent{
				Timestamp: ts,
				Hostname:  hostname,
				Service:   svc.Name,
				EventType: "error",
				Message:   truncateString(e.Line, 120),
			})
		}
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

func (s *OverviewService) queryLogErrorCount(ctx context.Context, tenantUUID, machineID, serviceName string, startNs, endNs int64) float64 {
	q := fmt.Sprintf(`count_over_time({service=%q, host_id=%q} |= "error" [5m])`, serviceName, machineID)
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

func (s *OverviewService) hasAny(m map[string]serviceBrief, names ...string) bool {
	for _, n := range names {
		if _, ok := m[n]; ok {
			return true
		}
	}
	return false
}

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func severityWeight(s string) int {
	switch s {
	case "critical":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	}
	return 0
}
