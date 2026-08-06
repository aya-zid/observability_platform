// internal/service/report_service.go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

var (
	ErrReportNotFound        = errors.New("report not found")
	ErrReportBadDates        = errors.New("invalid report dates")
	errReportUnsupportedType = errors.New("unsupported report type")
)

const reportGenerationTimeout = 240 * time.Second
const reportStaleGenerating = 5 * time.Minute

// ReportService orchestrates AI report generation: it gathers the compact
// numeric payload per UTC region, computes comparison deltas, calls Gemini and
// persists the result. Generation runs in a background goroutine so the POST
// returns immediately with status "generating".
type ReportService struct {
	reportRepo      *repository.ReportRepository
	alertRuleRepo   *repository.AlertRuleRepository
	hostRepo        *repository.HostRepository
	incidentGrouper *IncidentGroupingService
	ai              AIProvider
	gathererBase    *NginxDashboardService
}

func NewReportService(
	reportRepo *repository.ReportRepository,
	alertRuleRepo *repository.AlertRuleRepository,
	hostRepo *repository.HostRepository,
	incidentGrouper *IncidentGroupingService,
	ai AIProvider,
	gathererBase *NginxDashboardService,
) *ReportService {
	return &ReportService{
		reportRepo:      reportRepo,
		alertRuleRepo:   alertRuleRepo,
		hostRepo:        hostRepo,
		incidentGrouper: incidentGrouper,
		ai:              ai,
		gathererBase:    gathererBase,
	}
}

// Generate creates a report. When force is false it first reuses an existing
// ready report with the same dedup key. It always returns immediately and the
// heavy work happens in a background goroutine. The returned bool reports
// whether a brand-new report row was created.
func (s *ReportService) Generate(ctx context.Context, tenantID, userID uint, tenantUUID, typ, dateA, dateB string, force bool) (*domain.Report, bool, error) {
	if typ != "today" && typ != "compare" {
		return nil, false, errReportUnsupportedType
	}
	if !force {
		existing, err := s.reportRepo.FindReadyByKey(ctx, tenantID, typ, dateA, dateB)
		if err != nil {
			return nil, false, err
		}
		if existing != nil {
			return existing, false, nil
		}
	}

	report := &domain.Report{
		TenantID:    tenantID,
		Type:        typ,
		DateA:       dateA,
		DateB:       dateB,
		Status:      "generating",
		GeneratedBy: userID,
	}
	if err := s.reportRepo.Create(ctx, report); err != nil {
		return nil, false, err
	}
	go s.runGeneration(report.ID, tenantID, tenantUUID, typ, dateA, dateB)
	return report, true, nil
}

// Regenerate forces a fresh AI generation for an existing report, overwriting
// its content in place.
func (s *ReportService) Regenerate(ctx context.Context, id, tenantID uint, tenantUUID string) (*domain.Report, error) {
	report, err := s.reportRepo.GetByIDTenant(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, ErrReportNotFound
	}
	if err := s.reportRepo.MarkGenerating(ctx, id); err != nil {
		return nil, err
	}
	report.Status = "generating"
	report.Error = ""
	report.FinishedAt = nil
	go s.runGeneration(report.ID, tenantID, tenantUUID, report.Type, report.DateA, report.DateB)
	return report, nil
}

func (s *ReportService) GetByIDTenant(ctx context.Context, id, tenantID uint) (*domain.Report, error) {
	report, err := s.reportRepo.GetByIDTenant(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, ErrReportNotFound
	}
	// Staleness guard: a "generating" row that has not been touched for 5
	// minutes is lazily failed (server restarts / crashed goroutines). The
	// check uses UpdatedAt (bumped by every MarkGenerating/MarkReady/MarkError),
	// not CreatedAt, so a regenerate on an old row is not falsely timed out.
	if report.Status == "generating" && time.Since(report.UpdatedAt) > reportStaleGenerating {
		_ = s.reportRepo.MarkError(context.Background(), report.ID, "generation timed out")
		report.Status = "error"
		report.Error = "generation timed out"
	}
	return report, nil
}

func (s *ReportService) ListByTenant(ctx context.Context, tenantID uint, typ string, limit, offset int) ([]domain.Report, error) {
	return s.reportRepo.ListByTenant(ctx, tenantID, typ, limit, offset)
}

func (s *ReportService) Delete(ctx context.Context, id, tenantID uint) error {
	return s.reportRepo.Delete(ctx, id, tenantID)
}

// ─── Async generation ─────────────────────────────────────────────────────────

func (s *ReportService) runGeneration(reportID, tenantID uint, tenantUUID, typ, dateA, dateB string) {
	genCtx, cancel := context.WithTimeout(context.Background(), reportGenerationTimeout)
	defer cancel()

	dayA, err := time.Parse("2006-01-02", dateA)
	if err != nil {
		s.markError(reportID, "invalid date_a: "+err.Error())
		return
	}
	var dayB time.Time
	if typ == "compare" {
		dayB, err = time.Parse("2006-01-02", dateB)
		if err != nil {
			s.markError(reportID, "invalid date_b: "+err.Error())
			return
		}
	}

	payload, err := s.buildNumbersPayload(genCtx, tenantID, tenantUUID, typ, dayA, dayB)
	if err != nil {
		s.markError(reportID, "telemetry gathering failed: "+err.Error())
		return
	}
	numbersJSON, err := json.Marshal(payload)
	if err != nil {
		s.markError(reportID, "marshal numbers payload: "+err.Error())
		return
	}

	schema := TodayReportSchema()
	if typ == "compare" {
		schema = CompareReportSchema()
	}
	aiText, err := s.ai.Generate(genCtx, s.systemPrompt(typ), string(numbersJSON), schema)
	if err != nil {
		msg := err.Error()
		if errors.Is(err, errGeminiNotConfigured) {
			msg = errGeminiNotConfigured.Error()
		}
		s.markError(reportID, msg)
		return
	}

	s.markReady(reportID, json.RawMessage(numbersJSON), json.RawMessage(aiText))
}

func (s *ReportService) markReady(id uint, numbers, ai json.RawMessage) {
	_ = s.reportRepo.MarkReady(context.Background(), id, numbers, ai)
}

func (s *ReportService) markError(id uint, msg string) {
	_ = s.reportRepo.MarkError(context.Background(), id, msg)
}

// ─── Numeric payload ──────────────────────────────────────────────────────────

type reportNumbersPayload struct {
	Type          string                       `json:"type"`
	DayA          string                       `json:"day_a"`
	DayB          string                       `json:"day_b,omitempty"`
	GeneratedAt   string                       `json:"generated_at"`
	Regions       []reportRegionPayload        `json:"regions"`
	AlertsA       []reportAlertPayload         `json:"alerts_a"`
	AlertsB       []reportAlertPayload         `json:"alerts_b,omitempty"`
	Incidents     []reportIncidentPayload      `json:"incidents"`
	ServiceHealth []reportServiceHealthPayload `json:"service_health"`
	Hosts         reportHostsPayload           `json:"hosts"`
	Fleet         *ReportFleetMetrics          `json:"fleet"`
	FleetDayB     *ReportFleetMetrics          `json:"fleet_day_b,omitempty"`
	Deltas        []reportDeltaPayload         `json:"deltas,omitempty"`
}

type reportRegionPayload struct {
	Region string               `json:"region"`
	DayA   reportWindowPayload  `json:"day_a"`
	DayB   *reportWindowPayload `json:"day_b,omitempty"`
}

type reportWindowPayload struct {
	Services    map[string]*ReportServiceMetrics `json:"services"`
	Fleet       *ReportFleetMetrics              `json:"fleet"`
	LogSignals  map[string]*ReportLogSignals     `json:"log_signals"`
	AlertsFired int                              `json:"alerts_fired"`
}

type reportAlertPayload struct {
	Severity      string   `json:"severity"`
	RuleName      string   `json:"rule_name"`
	HostMachineID string   `json:"host_machine_id"`
	Service       string   `json:"service"`
	Status        string   `json:"status"`
	TriggeredAt   string   `json:"triggered_at"`
	ResolvedAt    string   `json:"resolved_at,omitempty"`
	ActualValue   *float64 `json:"actual_value"`
	Domain        string   `json:"domain,omitempty"`
	Pool          string   `json:"pool,omitempty"`
	ContainerName string   `json:"container_name,omitempty"`
}

type reportIncidentPayload struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Status      string `json:"status"`
	Hostname    string `json:"hostname"`
	FiringSince string `json:"firing_since"`
	ResolvedAt  string `json:"resolved_at,omitempty"`
	Members     int    `json:"members"`
}

type reportServiceHealthPayload struct {
	Service     string `json:"service"`
	Present     bool   `json:"present"`
	UpInstances int    `json:"up_instances"`
}

type reportHostsPayload struct {
	Total  int `json:"total"`
	Online int `json:"online"`
}

type reportDeltaPayload struct {
	Region    string   `json:"region"`
	Service   string   `json:"service"`
	Metric    string   `json:"metric"`
	DayAValue *float64 `json:"day_a_value"`
	DayBValue *float64 `json:"day_b_value"`
	DeltaPct  *float64 `json:"delta_pct"`
}

func (s *ReportService) buildNumbersPayload(ctx context.Context, tenantID uint, tenantUUID, typ string, dayA, dayB time.Time) (*reportNumbersPayload, error) {
	g := &reportMetricsGatherer{base: s.gathererBase, tenantUUID: tenantUUID, tenantID: tenantID}

	dayAExt, err := s.extractDay(ctx, g, dayA)
	if err != nil {
		return nil, err
	}
	hasB := typ == "compare"
	dayBExt := dayExtraction{}
	if hasB {
		dayBExt, err = s.extractDay(ctx, g, dayB)
		if err != nil {
			return nil, err
		}
	}

	dayAStart := midnightUTC(dayA)
	dayAEnd := dayAStart.Add(24 * time.Hour)

	payload := &reportNumbersPayload{
		Type:          typ,
		DayA:          dayA.Format("2006-01-02"),
		GeneratedAt:   time.Now().Format(time.RFC3339),
		Regions:       make([]reportRegionPayload, 0, len(reportRegions)),
		AlertsA:       mapAlerts(dayAExt.alerts),
		ServiceHealth: s.serviceHealth(ctx, g, dayAEnd.Unix()),
		Incidents:     mapIncidents(s.incidentCards(ctx, tenantID), dayAStart, dayAEnd),
		Hosts:         s.hostsSummary(ctx, tenantID),
		Fleet:         dayAExt.fullDay.fleet,
	}
	if hasB {
		payload.DayB = dayB.Format("2006-01-02")
		payload.AlertsB = mapAlerts(dayBExt.alerts)
		payload.FleetDayB = dayBExt.fullDay.fleet
		payload.Deltas = computeDeltas(dayAExt, dayBExt)
	}

	for _, name := range reportRegions {
		wa := dayAExt.windows[name]
		r := reportRegionPayload{
			Region: name,
			DayA: reportWindowPayload{
				Services:    wa.services,
				Fleet:       wa.fleet,
				LogSignals:  wa.logSignals,
				AlertsFired: wa.alertsFired,
			},
		}
		if hasB {
			wb := dayBExt.windows[name]
			r.DayB = &reportWindowPayload{
				Services:    wb.services,
				Fleet:       wb.fleet,
				LogSignals:  wb.logSignals,
				AlertsFired: wb.alertsFired,
			}
		}
		payload.Regions = append(payload.Regions, r)
	}

	return payload, nil
}

// dayExtraction holds the gathered scalars for one calendar day.
type dayExtraction struct {
	windows map[string]reportWindowResult
	fullDay reportWindowResult
	alerts  []domain.AlertEvent
}

type reportWindowResult struct {
	services    map[string]*ReportServiceMetrics
	fleet       *ReportFleetMetrics
	logSignals  map[string]*ReportLogSignals
	alertsFired int
}

func (s *ReportService) extractDay(ctx context.Context, g *reportMetricsGatherer, day time.Time) (dayExtraction, error) {
	startNs, endNs := dayExtentNs(day)
	alerts, err := s.alertRuleRepo.ListEventsByTenantInRange(ctx, g.tenantID, midnightUTC(day), midnightUTC(day).Add(24*time.Hour))
	if err != nil {
		return dayExtraction{}, fmt.Errorf("list alert events: %w", err)
	}

	windows := regionWindows(day)
	regions := make(map[string]reportWindowResult, len(reportRegions))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, name := range reportRegions {
		n := name
		w := windows[n]
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := s.extractWindow(ctx, g, w[0], w[1])
			r.alertsFired = countAlertsInRange(alerts, w[0], w[1])
			mu.Lock()
			regions[n] = r
			mu.Unlock()
		}()
	}

	var fullDay reportWindowResult
	wg.Add(1)
	go func() {
		defer wg.Done()
		fullDay = s.extractWindow(ctx, g, startNs, endNs)
	}()

	wg.Wait()
	return dayExtraction{windows: regions, fullDay: fullDay, alerts: alerts}, nil
}

func (s *ReportService) extractWindow(ctx context.Context, g *reportMetricsGatherer, startNs, endNs int64) reportWindowResult {
	res := reportWindowResult{
		services:   make(map[string]*ReportServiceMetrics),
		logSignals: make(map[string]*ReportLogSignals),
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}
	runService := func(kind string, fn func(context.Context, int64, int64) *ReportServiceMetrics) {
		run(func() {
			m := fn(ctx, startNs, endNs)
			mu.Lock()
			res.services[kind] = m
			mu.Unlock()
		})
	}

	runService("nginx", func(c context.Context, a, b int64) *ReportServiceMetrics { return g.extractWeb(c, "nginx", a, b) })
	runService("apache", func(c context.Context, a, b int64) *ReportServiceMetrics { return g.extractWeb(c, "apache", a, b) })
	runService("postgresql", g.extractPostgres)
	runService("mariadb", g.extractMariaDB)
	runService("redis", g.extractRedis)
	runService("phpfpm", g.extractPHPFPM)

	run(func() { res.fleet = g.extractFleet(ctx, startNs, endNs) })

	for _, kind := range []string{"nginx", "apache", "postgresql", "mariadb", "redis", "phpfpm"} {
		k := kind
		run(func() {
			ls := g.countLogSignals(ctx, k, startNs, endNs)
			mu.Lock()
			res.logSignals[k] = ls
			mu.Unlock()
		})
	}

	wg.Wait()
	return res
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func midnightUTC(day time.Time) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
}

func dayExtentNs(day time.Time) (int64, int64) {
	start := midnightUTC(day)
	end := start.Add(24 * time.Hour)
	return start.UnixNano(), end.UnixNano()
}

func countAlertsInRange(alerts []domain.AlertEvent, startNs, endNs int64) int {
	n := 0
	for _, a := range alerts {
		t := a.TriggeredAt.UnixNano()
		if t >= startNs && t < endNs {
			n++
		}
	}
	return n
}

func mapAlerts(alerts []domain.AlertEvent) []reportAlertPayload {
	out := make([]reportAlertPayload, 0, len(alerts))
	for _, a := range alerts {
		sev := ""
		if a.Rule != nil {
			sev = a.Rule.DefaultSeverity
		}
		if a.Threshold != nil && a.Threshold.Severity != "" {
			sev = a.Threshold.Severity
		}
		ruleName := ""
		if a.Rule != nil {
			ruleName = a.Rule.TemplateName
		}
		p := reportAlertPayload{
			Severity:      sev,
			RuleName:      ruleName,
			HostMachineID: a.HostMachineID,
			Service:       a.MetricString,
			Status:        a.Status,
			TriggeredAt:   a.TriggeredAt.Format(time.RFC3339),
			ActualValue:   a.ActualValue,
			Domain:        a.Domain,
			Pool:          a.Pool,
			ContainerName: a.ContainerName,
		}
		if a.ResolvedAt != nil {
			p.ResolvedAt = a.ResolvedAt.Format(time.RFC3339)
		}
		out = append(out, p)
	}
	return out
}

func mapIncidents(cards []IncidentCard, dayStart, dayEnd time.Time) []reportIncidentPayload {
	out := make([]reportIncidentPayload, 0, len(cards))
	for _, c := range cards {
		// Filter by overlap with the report day.
		if c.FiringSince.After(dayEnd) {
			continue
		}
		if c.ResolvedAt != nil && c.ResolvedAt.Before(dayStart) {
			continue
		}
		p := reportIncidentPayload{
			ID:          c.ID,
			Title:       c.Title,
			Severity:    c.Severity,
			Category:    string(c.Category),
			Status:      c.Status,
			Hostname:    c.Hostname,
			FiringSince: c.FiringSince.Format(time.RFC3339),
			Members:     len(c.Members),
		}
		if c.ResolvedAt != nil {
			p.ResolvedAt = c.ResolvedAt.Format(time.RFC3339)
		}
		out = append(out, p)
	}
	return out
}

func (s *ReportService) incidentCards(ctx context.Context, tenantID uint) []IncidentCard {
	if s.incidentGrouper == nil {
		return nil
	}
	cards, err := s.incidentGrouper.GroupIncidents(ctx, tenantID)
	if err != nil {
		return nil
	}
	return cards
}

func (s *ReportService) hostsSummary(ctx context.Context, tenantID uint) reportHostsPayload {
	p := reportHostsPayload{}
	hosts, err := s.hostRepo.ListByTenantCtx(ctx, tenantID)
	if err != nil {
		return p
	}
	p.Total = len(hosts)
	now := time.Now()
	for _, h := range hosts {
		if now.Sub(h.LastSeenAt) <= 5*time.Minute {
			p.Online++
		}
	}
	return p
}

func (s *ReportService) serviceHealth(ctx context.Context, g *reportMetricsGatherer, endSec int64) []reportServiceHealthPayload {
	kinds := []string{"nginx", "apache", "postgresql", "mariadb", "redis", "phpfpm", "container"}
	out := make([]reportServiceHealthPayload, 0, len(kinds))
	for _, k := range kinds {
		q := serviceUpQuery(k)
		if q == "" {
			continue
		}
		v := g.vmInstantScalar(ctx, q, endSec)
		p := reportServiceHealthPayload{Service: k}
		if v != nil {
			p.Present = *v > 0
			if p.Present {
				p.UpInstances = int(*v)
			}
		}
		out = append(out, p)
	}
	return out
}

func serviceUpQuery(kind string) string {
	switch kind {
	case "nginx":
		return `count(nginx_requests_total)`
	case "apache":
		return `count(apache_requests_total)`
	case "postgresql":
		return `count(pg_up)`
	case "mariadb":
		return `count(mysql_up)`
	case "redis":
		return `count(redis_up)`
	case "phpfpm":
		return `count(phpfpm_up)`
	case "container":
		return `count({__name__=~"container_.*_cpu_percent"})`
	default:
		return ""
	}
}

// ─── Deltas (compare) ─────────────────────────────────────────────────────────

type reportMetricRef struct {
	metric string
	get    func(*ReportServiceMetrics) *float64
}

var reportHeadlines = map[string][]reportMetricRef{
	"nginx": {
		{"total_requests", func(m *ReportServiceMetrics) *float64 { return m.TotalRequests }},
		{"avg_req_rate", func(m *ReportServiceMetrics) *float64 { return m.AvgReqRate }},
		{"total_5xx", func(m *ReportServiceMetrics) *float64 { return m.Total5xx }},
		{"total_4xx", func(m *ReportServiceMetrics) *float64 { return m.Total4xx }},
		{"p95_rt_ms", func(m *ReportServiceMetrics) *float64 { return m.P95RTMs }},
		{"bot_rate", func(m *ReportServiceMetrics) *float64 { return m.BotRate }},
	},
	"apache": {
		{"total_requests", func(m *ReportServiceMetrics) *float64 { return m.TotalRequests }},
		{"avg_req_rate", func(m *ReportServiceMetrics) *float64 { return m.AvgReqRate }},
		{"total_5xx", func(m *ReportServiceMetrics) *float64 { return m.Total5xx }},
		{"total_4xx", func(m *ReportServiceMetrics) *float64 { return m.Total4xx }},
		{"p95_rt_ms", func(m *ReportServiceMetrics) *float64 { return m.P95RTMs }},
		{"bot_rate", func(m *ReportServiceMetrics) *float64 { return m.BotRate }},
	},
	"postgresql": {
		{"cache_hit_ratio_pct", func(m *ReportServiceMetrics) *float64 { return m.CacheHitRatioPct }},
		{"slow_queries", func(m *ReportServiceMetrics) *float64 { return m.SlowQueries }},
		{"errors", func(m *ReportServiceMetrics) *float64 { return m.Errors }},
		{"auth_failures", func(m *ReportServiceMetrics) *float64 { return m.AuthFailures }},
		{"deadlocks_per_min", func(m *ReportServiceMetrics) *float64 { return m.DeadlocksPerMin }},
		{"connection_saturation_pct", func(m *ReportServiceMetrics) *float64 { return m.ConnectionSaturationPct }},
	},
	"mariadb": {
		{"qps", func(m *ReportServiceMetrics) *float64 { return m.QPS }},
		{"slow_queries_per_min", func(m *ReportServiceMetrics) *float64 { return m.SlowQueriesPerMin }},
		{"errors", func(m *ReportServiceMetrics) *float64 { return m.Errors }},
		{"deadlocks_per_min", func(m *ReportServiceMetrics) *float64 { return m.DeadlocksPerMin }},
		{"buffer_pool_hit_ratio_pct", func(m *ReportServiceMetrics) *float64 { return m.BufferPoolHitRatioPct }},
		{"connection_saturation_pct", func(m *ReportServiceMetrics) *float64 { return m.ConnectionSaturationPct }},
	},
	"redis": {
		{"memory_usage_pct", func(m *ReportServiceMetrics) *float64 { return m.MemoryUsagePct }},
		{"commands_per_sec", func(m *ReportServiceMetrics) *float64 { return m.CommandsPerSec }},
		{"connected_clients", func(m *ReportServiceMetrics) *float64 { return m.ConnectedClients }},
		{"evict_rate", func(m *ReportServiceMetrics) *float64 { return m.EvictRate }},
		{"error_replies_per_sec", func(m *ReportServiceMetrics) *float64 { return m.ErrorRepliesPerSec }},
		{"slow_log_count", func(m *ReportServiceMetrics) *float64 { return m.SlowLogCount }},
	},
	"phpfpm": {
		{"worker_saturation_pct", func(m *ReportServiceMetrics) *float64 { return m.WorkerSaturationPct }},
		{"accepted_conn_per_sec", func(m *ReportServiceMetrics) *float64 { return m.AcceptedConnPerSec }},
		{"slow_requests", func(m *ReportServiceMetrics) *float64 { return m.SlowRequests }},
		{"status_5xx", func(m *ReportServiceMetrics) *float64 { return m.Status5xx }},
		{"latency_p95_ms", func(m *ReportServiceMetrics) *float64 { return m.LatencyP95Ms }},
		{"opcache_hit_rate", func(m *ReportServiceMetrics) *float64 { return m.OpcacheHitRate }},
	},
}

type reportFleetRef struct {
	metric string
	get    func(*ReportFleetMetrics) *float64
}

var reportFleetHeadlines = []reportFleetRef{
	{"avg_cpu_pct", func(f *ReportFleetMetrics) *float64 { return f.AvgCPU }},
	{"avg_mem_pct", func(f *ReportFleetMetrics) *float64 { return f.AvgMem }},
	{"avg_disk_pct", func(f *ReportFleetMetrics) *float64 { return f.AvgDisk }},
	{"net_in_bps", func(f *ReportFleetMetrics) *float64 { return f.NetIn }},
	{"net_out_bps", func(f *ReportFleetMetrics) *float64 { return f.NetOut }},
	{"load_avg_1m", func(f *ReportFleetMetrics) *float64 { return f.LoadAvg1m }},
}

func computeDeltas(a, b dayExtraction) []reportDeltaPayload {
	out := make([]reportDeltaPayload, 0)
	for _, name := range reportRegions {
		ra, okA := a.windows[name]
		rb, okB := b.windows[name]
		if !okA || !okB {
			continue
		}
		out = append(out, compareWindows(name, ra, rb)...)
	}
	out = append(out, compareWindows("day", a.fullDay, b.fullDay)...)
	return out
}

func compareWindows(region string, a, b reportWindowResult) []reportDeltaPayload {
	out := make([]reportDeltaPayload, 0)
	for kind, refs := range reportHeadlines {
		ma, mAok := a.services[kind]
		mb, mBok := b.services[kind]
		if !mAok && !mBok {
			continue
		}
		for _, ref := range refs {
			var va, vb *float64
			if mAok {
				va = ref.get(ma)
			}
			if mBok {
				vb = ref.get(mb)
			}
			if va == nil && vb == nil {
				continue
			}
			out = append(out, reportDeltaPayload{
				Region:    region,
				Service:   kind,
				Metric:    ref.metric,
				DayAValue: va,
				DayBValue: vb,
				DeltaPct:  deltaPct(va, vb),
			})
		}
	}
	if a.fleet != nil || b.fleet != nil {
		for _, ref := range reportFleetHeadlines {
			var va, vb *float64
			if a.fleet != nil {
				va = ref.get(a.fleet)
			}
			if b.fleet != nil {
				vb = ref.get(b.fleet)
			}
			if va == nil && vb == nil {
				continue
			}
			out = append(out, reportDeltaPayload{
				Region:    region,
				Service:   "fleet",
				Metric:    ref.metric,
				DayAValue: va,
				DayBValue: vb,
				DeltaPct:  deltaPct(va, vb),
			})
		}
	}
	return out
}

// deltaPct returns (day_b - day_a) / day_a * 100, nil when either side is
// missing or day_a is zero (division by zero guard).
func deltaPct(a, b *float64) *float64 {
	if a == nil || b == nil || *a == 0 {
		return nil
	}
	v := (*b - *a) / *a * 100
	return &v
}

// ─── System prompt ────────────────────────────────────────────────────────────

func (s *ReportService) systemPrompt(typ string) string {
	if typ == "compare" {
		return s.comparePrompt()
	}
	return s.todayPrompt()
}

func (s *ReportService) todayPrompt() string {
	return `You are a senior SRE / DevOps analyst for a multi-tenant observability platform.
You analyze infrastructure telemetry (metrics + logs + alerts) and produce a professional daily report for an operations team.

INPUT
You receive one JSON payload ("numbers") with monitored telemetry split into fixed UTC time regions:
night (00-06), morning (06-12), afternoon (12-18), evening (18-24). This is a "today" report: one day's data.
Each region's "day_a.services" holds per-service-kind scalar metrics (nginx/apache/postgresql/mariadb/redis/phpfpm),
"day_a.fleet" holds aggregated host metrics, "day_a.log_signals" holds per-service error/warning log line counts,
and "day_a.alerts_fired" is the alert count in that region. Whole-day "alerts_a", "incidents", "service_health",
"hosts" and "fleet" are also provided.

ANALYSIS METHOD — follow this order for every region:
1. Scan for anomalies and extremes: spikes, drops, saturation (>85%), error bursts, deadlocks, auth failures,
   latency regressions, near-capacity resources.
2. Look for correlated signals across services. If one tier degrades (e.g. PHP-FPM saturation) check whether the
   tier above it errored (web 5xx) or the tier below it slowed (DB slow queries). Mention plausible cause->effect
   chains only when the data supports them.
3. Distinguish real problems from noise: a single spike on a low-traffic host is noise; a sustained rise across
   many hosts is a pattern. Say so.

GROUND-TRUTH RULES (non-negotiable):
- ONLY reference numbers that literally appear in the payload. Never invent, approximate, or extrapolate figures.
- Quote values exactly as given, WITH their units (req/s, ms, %, bytes, count).
- If the payload has no data for a service/region (null/empty), say "no data" — do not guess.
- delta_pct may be null (division by zero) — treat null as "not measurable", not 0%.

RECOMMENDATIONS — this section must be the most useful part of the report. For each recommendation provide:
- priority: critical | high | medium | low (impact x urgency, not effort)
- title: one short imperative line ("Raise PHP-FPM pm.max_children", "Enable rt= in nginx log_format",
  "Investigate the 5xx spike on /api/v2/checkout")
- action: the concrete step(s) to take, in plain language
- rationale: tie it to the exact metric(s) and values you observed
- expected_impact: what the team should expect after acting
- effort: low | medium | high
Order recommendations by priority. Skip obvious/generic advice; only recommend what the data actually warrants.

STYLE:
- Professional, concise, plain language a non-expert operator can act on.
- Executive summary: 2-3 sentences max. Region summaries: 1-2 sentences.
- Be specific and numeric; avoid vague praise or filler.

OUTPUT:
- Return ONLY valid JSON matching the schema. No markdown, no prose, no code fences, no commentary outside the JSON object.`
}

func (s *ReportService) comparePrompt() string {
	return `You are a senior SRE / DevOps analyst for a multi-tenant observability platform.
You analyze infrastructure telemetry (metrics + logs + alerts) and produce a professional day-comparison report.

INPUT
You receive one JSON payload ("numbers") with two days' telemetry split into fixed UTC time regions:
night (00-06), morning (06-12), afternoon (12-18), evening (18-24).
- day_a = reference day, day_b = current day.
- Each region's "day_a" and "day_b" hold per-service-kind scalar metrics, fleet aggregates, log signals and
  alerts_fired for that day's region.
- "alerts_a"/"alerts_b", "incidents", "service_health", "hosts", "fleet" and "fleet_day_b" are whole-day.
- "deltas" contains precomputed ground-truth differences: region ("day" = whole-day totals), service, metric,
  day_a_value, day_b_value and delta_pct = (day_b - day_a) / day_a * 100.

ANALYSIS METHOD — follow this order:
1. Rank what changed most using the given delta_pct values. Never recompute or invent percentages.
2. For each region, name the biggest differences (up/down/flat) and what they imply.
3. Look for correlated signals: did traffic rise and DB slow queries rise with it? Did 5xx drop when latency dropped?
4. Distinguish real changes from noise: a small delta on low traffic is noise; a sustained large delta is a pattern.

GROUND-TRUTH RULES (non-negotiable):
- ONLY reference numbers that literally appear in the payload, including delta_pct. Never invent figures.
- Quote values exactly, WITH their units (req/s, ms, %, bytes, count).
- delta_pct null means "not measurable" (division by zero), not 0%.
- If no data for a service/region (null/empty), say "no data".

RECOMMENDATIONS — provide for each: priority (critical|high|medium|low), a short imperative title,
a concrete action, a rationale tied to exact metrics/values you observed, expected_impact, and effort (low|medium|high).
Order by priority; only recommend what the data warrants.

STYLE:
- Professional, concise, plain language a non-expert operator can act on.
- Executive summary: 2-3 sentences max. Region summaries: 1-2 sentences.

OUTPUT:
- Return ONLY valid JSON matching the schema. No markdown, no prose, no code fences, no commentary outside the JSON object.`
}
