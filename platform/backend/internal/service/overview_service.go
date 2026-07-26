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
	Global   GlobalHealth   `json:"global"`
	Hosts    []HostOverview `json:"hosts"`
	Timeline TimelineData   `json:"timeline"`
	DepGraph DepGraphData   `json:"dep_graph"`
}

type GlobalHealth struct {
	HostsOnline      int `json:"hosts_online"`
	HostsTotal       int `json:"hosts_total"`
	ServicesHealthy  int `json:"services_healthy"`
	ServicesDegraded int `json:"services_degraded"`
	ServicesDown     int `json:"services_down"`
	ActiveSignals    int `json:"active_signals"`
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
		global    GlobalHealth
		hostCards []HostOverview
		timeline  TimelineData
		depGraph  DepGraphData
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

			// Build dep graph nodes + edges.
			hostDepNodes, hostDepEdges := s.buildDepGraph(hb.machineID, hb.hostname, chips)

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
			depGraph.Nodes = append(depGraph.Nodes, hostDepNodes...)
			depGraph.Edges = append(depGraph.Edges, hostDepEdges...)
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

	return &OverviewResponse{
		Global:   global,
		Hosts:    hostCards,
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

func (s *OverviewService) buildDepGraph(machineID, hostname string, chips []ServiceChip) ([]DepNode, []DepEdge) {
	nodes := []DepNode{
		{ID: machineID + ":host", Label: hostname, Status: "healthy", Type: "host"},
	}
	var edges []DepEdge

	hasWeb := false
	hasApp := false
	hasDB := false
	hasCache := false

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

	// Second pass: build nodes and edges.
	for _, c := range chips {
		switch c.Name {
		case "nginx", "apache":
			webID := machineID + ":web"
			nodes = append(nodes, DepNode{ID: webID, Label: c.Name, Status: c.Status, Type: "web"})
			edges = append(edges, DepEdge{Source: webID, Target: machineID + ":host", Status: c.Status})
		case "phpfpm":
			appID := machineID + ":app"
			nodes = append(nodes, DepNode{ID: appID, Label: c.Name, Status: c.Status, Type: "app"})
			edges = append(edges, DepEdge{Source: appID, Target: machineID + ":host", Status: c.Status})
		case "mariadb", "postgres":
			dbID := machineID + ":db"
			nodes = append(nodes, DepNode{ID: dbID, Label: c.Name, Status: c.Status, Type: "db"})
			edges = append(edges, DepEdge{Source: dbID, Target: machineID + ":host", Status: c.Status})
		case "redis":
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

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}




