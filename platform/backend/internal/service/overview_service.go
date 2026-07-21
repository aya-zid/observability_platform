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
	Global          GlobalHealth     `json:"global"`
	Hosts           []HostOverview   `json:"hosts"`
	Issues []DiagnosedIssue `json:"issues"`
	Findings        []FindingVO      `json:"findings"`
	Timeline        TimelineData     `json:"timeline"`
	DepGraph        DepGraphData     `json:"dep_graph"`
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

type FindingVO struct {
	ID                 uint           `json:"id"`
	RuleID             string         `json:"rule_id"`
	Severity           string         `json:"severity"`
	Title              string         `json:"title"`
	Message            string         `json:"message"`
	Evidence           []EvidenceItem `json:"evidence"`
	RecommendedAction  *string        `json:"recommended_action,omitempty"`
	DashboardLink      *string        `json:"dashboard_link,omitempty"`
	ServiceName        *string        `json:"service_name,omitempty"`
	InvestigationLinks []LinkVO       `json:"investigation_links,omitempty"`
	Hostname           string         `json:"hostname"`
	FiredAt            string         `json:"fired_at"`
	Duration           string         `json:"duration"`
}

type LinkVO struct {
	Label string `json:"label"`
	URL   string `json:"url"`
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
	cfg                   *config.Config
	redis                 *redis.Client
	httpClient            *http.Client
	hostRepo              *repository.HostRepository
	discRepo              *repository.DiscoveryRepository
	correlationFindingRepo *repository.CorrelationFindingRepository
	issueDetectionService  *IssueDetectionService
}

func NewOverviewService(
	cfg *config.Config,
	redisClient *redis.Client,
	hostRepo *repository.HostRepository,
	discRepo *repository.DiscoveryRepository,
	correlationFindingRepo *repository.CorrelationFindingRepository,
	issueDetectionService *IssueDetectionService,
) *OverviewService {
	return &OverviewService{
		cfg:                   cfg,
		redis:                 redisClient,
		httpClient:            &http.Client{Timeout: 45 * time.Second},
		hostRepo:              hostRepo,
		discRepo:              discRepo,
		correlationFindingRepo: correlationFindingRepo,
		issueDetectionService:  issueDetectionService,
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
		global     GlobalHealth
		hostCards  []HostOverview
		allIssues  []DiagnosedIssue
		findings   []FindingVO
		timeline   TimelineData
		depGraph   DepGraphData
		mu         sync.Mutex
		wg         sync.WaitGroup
	)

	timeSec := now.Unix()
	timeNs := now.UnixNano()
	startNs := timeNs - int64(5*time.Minute)

	// Pre-load active correlation findings from DB.
	dbFindings, err := s.correlationFindingRepo.ListActiveByTenantCtx(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list findings: %w", err)
	}
	findingsByHost := map[string][]domainFindingBrief{}
	for _, f := range dbFindings {
		machineID := f.MachineID
		serviceName := ""
		if f.ServiceName != nil {
			serviceName = *f.ServiceName
		}
		findingsByHost[machineID] = append(findingsByHost[machineID], domainFindingBrief{
			ruleID:            f.RuleID,
			severity:          f.Severity,
			title:             f.Title,
			message:           f.Message,
			evidence:          f.Evidence,
			recommendedAction: f.RecommendedAction,
			dashboardLink:     f.DashboardLink,
			svcName:           serviceName,
			hostname:          f.Hostname,
			firedAt:           f.FiredAt,
		})
	}

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
				downCount += len(chips)
			}
			summary := ""
			if len(summaryParts) > 0 {
				summary = strings.Join(summaryParts, ", ")
			}

			// Diagnosed issues from IssueDetectionService.
			issues := s.issueDetectionService.DetectHostIssues(
				ctx, tenantID, tenantUUID, hb.machineID, hb.hostname,
				svcsByHost[hb.machineID], timeSec, startNs, timeNs,
			)

			// Build dep graph nodes + edges with coloring from active findings.
			hostFindings := findingsByHost[hb.machineID]
			hostDepNodes, hostDepEdges := s.buildDepGraph(hb.machineID, hb.hostname, chips, hostFindings)

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
			allIssues = append(allIssues, issues...)
			depGraph.Nodes = append(depGraph.Nodes, hostDepNodes...)
			depGraph.Edges = append(depGraph.Edges, hostDepEdges...)
			timeline.Events = append(timeline.Events, hostTimeline...)
			mu.Unlock()
		}(hb)
	}
	wg.Wait()

	// Build FindingsVO from DB findings.
	for _, f := range dbFindings {
		evidence := parseEvidenceJSON(f.Evidence)
		duration := computeDuration(f.FiredAt)
		links := buildInvestigationLinks(f.MachineID, f.RuleID)
		findings = append(findings, FindingVO{
			ID:                 f.ID,
			RuleID:             f.RuleID,
			Severity:           f.Severity,
			Title:              f.Title,
			Message:            f.Message,
			Evidence:           evidence,
			RecommendedAction:  f.RecommendedAction,
			DashboardLink:      f.DashboardLink,
			ServiceName:        f.ServiceName,
			InvestigationLinks: links,
			Hostname:           f.Hostname,
			FiredAt:            f.FiredAt.UTC().Format(time.RFC3339),
			Duration:           duration,
		})
	}

	global.OpenFindings = len(findings)
	global.ActiveSignals += len(findings)

	// Sort timeline: newest first.
	sort.Slice(timeline.Events, func(i, j int) bool {
		return timeline.Events[i].Timestamp > timeline.Events[j].Timestamp
	})
	if len(timeline.Events) > 100 {
		timeline.Events = timeline.Events[:100]
	}

	// Sort issues by severity: critical > warning > info.
	sort.Slice(allIssues, func(i, j int) bool {
		return severityWeight(allIssues[i].Severity) > severityWeight(allIssues[j].Severity)
	})
	if len(allIssues) > 20 {
		allIssues = allIssues[:20]
	}

	return &OverviewResponse{
		Global:   global,
		Hosts:    hostCards,
		Issues:   allIssues,
		Findings: findings,
		Timeline: timeline,
		DepGraph: depGraph,
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
		errCount := s.queryLogErrorCount(ctx, tenantUUID, machineID, svc.Name, startNs, endNs)
		if errCount > 10 {
			return "degraded"
		}
		return "unknown"
	}

	upMetric := fmt.Sprintf(`%s_up{host_id="%s"}`, svc.Name, machineID)
	up := s.queryVMInstantSafe(ctx, tenantID, upMetric, timeSec)
	if up != nil && *up == 0 {
		return "down"
	}

	scrapeErr := s.queryVMInstantSafe(ctx, tenantID,
		fmt.Sprintf(`%s_scrape_error{host_id="%s"}`, svc.Name, machineID), timeSec)
	if scrapeErr != nil && *scrapeErr > 0 {
		return "down"
	}

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

	errCount := s.queryLogErrorCount(ctx, tenantUUID, machineID, svc.Name, startNs, endNs)
	if errCount > 50 {
		return "degraded"
	}

	return "healthy"
}

// ─── Dependency graph with edge coloring ──────────────────────────────────────

type domainFindingBrief struct {
	ruleID            string
	severity          string
	title             string
	message           string
	evidence          string
	recommendedAction *string
	dashboardLink     *string
	svcName           string
	hostname          string
	firedAt           time.Time
}

// ruleToEdgeStatus maps correlation rule IDs to dep graph edge status colors.
func ruleToEdgeStatus(ruleID string) string {
	switch ruleID {
	case "web_app_exhaustion", "auth_brute_force", "mariadb_deadlock_spike":
		return "critical"
	case "app_db_bottleneck", "db_disk_bottleneck", "cache_miss_cascade":
		return "warning"
	case "host_exhaustion":
		return "critical"
	case "http_4xx_high", "postgres_replication_lag":
		return "warning"
	}
	return "warning"
}

func (s *OverviewService) buildDepGraph(machineID, hostname string, chips []ServiceChip, findings []domainFindingBrief) ([]DepNode, []DepEdge) {
	nodes := []DepNode{
		{ID: machineID + ":host", Label: hostname, Status: "healthy", Type: "host"},
	}
	var edges []DepEdge

	hasWeb := false
	hasApp := false
	hasDB := false
	hasCache := false

	// Collect edge status overrides from findings.
	edgeOverrides := map[string]string{} // "source:target" -> status

	// Helper to register finding impact on an edge.
	applyFinding := func(source, target, ruleID string) {
		key := source + ":" + target
		status := ruleToEdgeStatus(ruleID)
		if existing, ok := edgeOverrides[key]; ok {
			if severityWeight(status) > severityWeight(existing) {
				edgeOverrides[key] = status
			}
		} else {
			edgeOverrides[key] = status
		}
	}

	// First pass: determine which service types exist.
	for _, c := range chips {
		switch c.Name {
		case "nginx", "apache":
			hasWeb = true
		case "phpfpm":
			hasApp = true
		case "mariadb", "postgres":
			hasDB = true
		case "redis":
			hasCache = true
		}
	}

	// Apply edge overrides from findings now that flags are set.
	for _, f := range findings {
		switch f.ruleID {
		case "web_app_exhaustion", "auth_brute_force", "http_4xx_high":
			if hasWeb && hasApp {
				applyFinding(machineID+":web", machineID+":app", f.ruleID)
			}
		case "app_db_bottleneck":
			if hasApp && hasDB {
				applyFinding(machineID+":app", machineID+":db", f.ruleID)
			}
		case "cache_miss_cascade":
			if hasApp && hasCache {
				applyFinding(machineID+":app", machineID+":cache", f.ruleID)
			}
		case "db_disk_bottleneck", "mariadb_deadlock_spike":
			if hasDB {
				applyFinding(machineID+":db", machineID+":host", f.ruleID)
			}
		case "postgres_replication_lag":
			if hasDB {
				applyFinding(machineID+":db", machineID+":host", f.ruleID)
			}
		case "host_exhaustion":
			for _, c := range chips {
				nodeID := ""
				switch c.Name {
				case "nginx", "apache":
					nodeID = machineID + ":web"
				case "phpfpm":
					nodeID = machineID + ":app"
				case "mariadb", "postgres":
					nodeID = machineID + ":db"
				case "redis":
					nodeID = machineID + ":cache"
				}
				if nodeID != "" {
					applyFinding(nodeID, machineID+":host", f.ruleID)
				}
			}
		}
	}

	// Second pass: build nodes and edges with overrides.
	for _, c := range chips {
		switch c.Name {
		case "nginx", "apache":
			webID := machineID + ":web"
			nodes = append(nodes, DepNode{ID: webID, Label: c.Name, Status: c.Status, Type: "web"})
			status := c.Status
			if s, ok := edgeOverrides[webID+":"+machineID+":host"]; ok {
				status = s
			}
			edges = append(edges, DepEdge{Source: webID, Target: machineID + ":host", Status: status})
		case "phpfpm":
			appID := machineID + ":app"
			nodes = append(nodes, DepNode{ID: appID, Label: c.Name, Status: c.Status, Type: "app"})
			status := c.Status
			if s, ok := edgeOverrides[appID+":"+machineID+":host"]; ok {
				status = s
			}
			edges = append(edges, DepEdge{Source: appID, Target: machineID + ":host", Status: status})
		case "mariadb", "postgres":
			dbID := machineID + ":db"
			nodes = append(nodes, DepNode{ID: dbID, Label: c.Name, Status: c.Status, Type: "db"})
			status := c.Status
			if s, ok := edgeOverrides[dbID+":"+machineID+":host"]; ok {
				status = s
			}
			edges = append(edges, DepEdge{Source: dbID, Target: machineID + ":host", Status: status})
		case "redis":
			cacheID := machineID + ":cache"
			nodes = append(nodes, DepNode{ID: cacheID, Label: c.Name, Status: c.Status, Type: "cache"})
			status := c.Status
			if s, ok := edgeOverrides[cacheID+":"+machineID+":host"]; ok {
				status = s
			}
			edges = append(edges, DepEdge{Source: cacheID, Target: machineID + ":host", Status: status})
		}
	}

	// Add dependency edges between services (with overrides).
	if hasWeb && hasApp {
		status := "healthy"
		if s, ok := edgeOverrides[machineID+":web:"+machineID+":app"]; ok {
			status = s
		}
		edges = append(edges, DepEdge{
			Source: machineID + ":web",
			Target: machineID + ":app",
			Status: status,
		})
	}
	if hasApp && hasDB {
		status := "healthy"
		if s, ok := edgeOverrides[machineID+":app:"+machineID+":db"]; ok {
			status = s
		}
		edges = append(edges, DepEdge{
			Source: machineID + ":app",
			Target: machineID + ":db",
			Status: status,
		})
	}
	if hasApp && hasCache {
		status := "healthy"
		if s, ok := edgeOverrides[machineID+":app:"+machineID+":cache"]; ok {
			status = s
		}
		edges = append(edges, DepEdge{
			Source: machineID + ":app",
			Target: machineID + ":cache",
			Status: status,
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

func computeDuration(firedAt time.Time) string {
	d := time.Since(firedAt)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	}
	h := int(d.Hours())
	if h == 1 {
		return "1 hour ago"
	}
	return fmt.Sprintf("%d hours ago", h)
}

func buildInvestigationLinks(machineID, ruleID string) []LinkVO {
	switch ruleID {
	case "web_app_exhaustion":
		return []LinkVO{
			{Label: "Nginx Dashboard", URL: fmt.Sprintf("/nginx/%s/dashboard", machineID)},
			{Label: "PHP-FPM Dashboard", URL: fmt.Sprintf("/phpfpm/%s/dashboard", machineID)},
		}
	case "app_db_bottleneck":
		return []LinkVO{
			{Label: "PHP-FPM Slow Logs", URL: fmt.Sprintf("/phpfpm/%s/logs", machineID)},
			{Label: "MariaDB Slow Queries", URL: fmt.Sprintf("/mariadb/%s/slow-query-logs", machineID)},
		}
	case "db_disk_bottleneck":
		return []LinkVO{
			{Label: "MariaDB Dashboard", URL: fmt.Sprintf("/mariadb/%s/dashboard", machineID)},
			{Label: "Host Disk Dashboard", URL: fmt.Sprintf("/hosts/%s/dashboard/disk", machineID)},
		}
	case "cache_miss_cascade":
		return []LinkVO{
			{Label: "Redis Dashboard", URL: fmt.Sprintf("/redis/%s/dashboard", machineID)},
			{Label: "DB Dashboard", URL: fmt.Sprintf("/mariadb/%s/dashboard", machineID)},
		}
	case "host_exhaustion":
		return []LinkVO{
			{Label: "Host CPU", URL: fmt.Sprintf("/hosts/%s/dashboard/cpu", machineID)},
			{Label: "Host Memory", URL: fmt.Sprintf("/hosts/%s/dashboard/memory", machineID)},
		}
	case "auth_brute_force":
		return []LinkVO{
			{Label: "Nginx IP Investigation", URL: fmt.Sprintf("/nginx/%s/ip-investigation", machineID)},
		}
	case "http_4xx_high":
		return []LinkVO{
			{Label: "Nginx Explorer", URL: fmt.Sprintf("/nginx/%s/explorer", machineID)},
		}
	case "mariadb_deadlock_spike":
		return []LinkVO{
			{Label: "MariaDB Dashboard", URL: fmt.Sprintf("/mariadb/%s/dashboard", machineID)},
		}
	case "postgres_replication_lag":
		return []LinkVO{
			{Label: "PostgreSQL Dashboard", URL: fmt.Sprintf("/postgres/%s/dashboard", machineID)},
		}
	}
	return nil
}

func parseEvidenceJSON(evidence string) []EvidenceItem {
	if evidence == "" || evidence == "null" {
		return nil
	}
	// For now, return a single item with the raw JSON as value.
	// The evidence JSON is structured as flat key-value pairs.
	var items []EvidenceItem
	// Simple parser for evidence JSON like `{"key": value, ...}`
	// Strip braces and split by comma.
	inner := strings.TrimSpace(evidence)
	inner = strings.TrimPrefix(inner, "{")
	inner = strings.TrimSuffix(inner, "}")
	parts := strings.Split(inner, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.Trim(strings.TrimSpace(kv[0]), `"`)
		val := strings.TrimSpace(kv[1])
		items = append(items, EvidenceItem{
			Metric: key,
			Value:  val,
			Status: "info",
		})
	}
	if len(items) == 0 {
		items = []EvidenceItem{{Metric: "details", Value: evidence, Status: "info"}}
	}
	return items
}
