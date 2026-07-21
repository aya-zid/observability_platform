package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type CorrelationWorker struct {
	cfg                   *config.Config
	httpClient            *http.Client
	hostRepo              *repository.HostRepository
	discRepo              *repository.DiscoveryRepository
	tenantRepo            *repository.TenantRepository
	correlationFindingRepo *repository.CorrelationFindingRepository
}

func NewCorrelationWorker(
	cfg *config.Config,
	hostRepo *repository.HostRepository,
	discRepo *repository.DiscoveryRepository,
	tenantRepo *repository.TenantRepository,
	correlationFindingRepo *repository.CorrelationFindingRepository,
) *CorrelationWorker {
	return &CorrelationWorker{
		cfg:                   cfg,
		httpClient:            &http.Client{Timeout: 45 * time.Second},
		hostRepo:              hostRepo,
		discRepo:              discRepo,
		tenantRepo:            tenantRepo,
		correlationFindingRepo: correlationFindingRepo,
	}
}

func (w *CorrelationWorker) StartEvaluator(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.evaluate(ctx)
		}
	}
}

func (w *CorrelationWorker) evaluate(ctx context.Context) {
	tenants, err := w.tenantRepo.FindAllCtx(ctx)
	if err != nil {
		log.Printf("[correlation-worker] list tenants: %v", err)
		return
	}
	for _, tenant := range tenants {
		w.evaluateTenant(ctx, tenant.ID, tenant.UUID)
	}
}

// hostEvalContext holds pre-fetched data for a single host evaluation.
type hostEvalContext struct {
	machineID string
	hostname  string
	svcMap    map[string]serviceBrief
}

func (w *CorrelationWorker) evaluateTenant(ctx context.Context, tenantID uint, tenantUUID string) {
	hosts, err := w.hostRepo.ListByTenantCtx(ctx, tenantID)
	if err != nil {
		log.Printf("[correlation-worker] list hosts for tenant %d: %v", tenantID, err)
		return
	}
	services, err := w.discRepo.ListServicesByTenantCtx(ctx, tenantID)
	if err != nil {
		log.Printf("[correlation-worker] list services for tenant %d: %v", tenantID, err)
		return
	}
	svcsByHost := map[string][]serviceBrief{}
	for _, svc := range services {
		if !supportedServices[svc.ServiceName] {
			continue
		}
		svcsByHost[svc.MachineID] = append(svcsByHost[svc.MachineID], serviceBrief{
			Name:       svc.ServiceName,
			Port:       svc.Port,
			LogCollOn:  svc.LogCollectionOn,
			MetricColl: svc.MetricCollectionOn,
		})
	}

	for _, host := range hosts {
		svcs := svcsByHost[host.MachineID]
		if len(svcs) == 0 {
			continue
		}
		svcMap := make(map[string]serviceBrief, len(svcs))
		for _, svc := range svcs {
			svcMap[svc.Name] = svc
		}
		ec := hostEvalContext{
			machineID: host.MachineID,
			hostname:  host.Hostname,
			svcMap:    svcMap,
		}
		w.evaluateHost(ctx, tenantID, tenantUUID, ec)
	}

	// Resolve stale findings older than 10 minutes.
	w.resolveStaleFindings(ctx, tenantID)
}

func (w *CorrelationWorker) resolveStaleFindings(ctx context.Context, tenantID uint) {
	resolved, err := w.correlationFindingRepo.ResolveStale(ctx, tenantID, 10*time.Minute)
	if err != nil {
		log.Printf("[correlation-worker] resolve stale for tenant %d: %v", tenantID, err)
		return
	}
	if resolved > 0 {
		log.Printf("[correlation-worker] resolved %d stale findings for tenant %d", resolved, tenantID)
	}
}

func (w *CorrelationWorker) evaluateHost(ctx context.Context, tenantID uint, tenantUUID string, ec hostEvalContext) {
	now := time.Now()
	timeSec := now.Unix()
	startNs := now.Add(-5 * time.Minute).UnixNano()
	endNs := now.UnixNano()

	activeRules := map[string]bool{}
	svc := ec.svcMap

	// ── Rule 1: web_app_exhaustion — (5xx > 5%) AND (phpfpm active/total > 85%) ──
	if hasAnySvc(svc, "nginx", "apache") && hasAnySvc(svc, "phpfpm") {
		web5xx := w.queryVM(ctx, tenantID,
			fmt.Sprintf(`rate(nginx_http_requests_total{host_id="%s",status=~"5.."}[5m]) / rate(nginx_http_requests_total{host_id="%s"}[5m]) > 0.05`, ec.machineID, ec.machineID), timeSec)
		if web5xx == nil {
			web5xx = w.queryVM(ctx, tenantID,
				fmt.Sprintf(`rate(httpd_requests_total{host_id="%s",status=~"5.."}[5m]) / rate(httpd_requests_total{host_id="%s"}[5m]) > 0.05`, ec.machineID, ec.machineID), timeSec)
		}
		phpSat := w.queryVM(ctx, tenantID,
			fmt.Sprintf(`phpfpm_active_workers{host_id="%s"} / phpfpm_total_workers{host_id="%s"} > 0.85`, ec.machineID, ec.machineID), timeSec)
		if web5xx != nil && *web5xx > 0.05 && phpSat != nil && *phpSat > 0.85 {
			activeRules["web_app_exhaustion"] = true
			svcName := "nginx"
			if !hasAnySvc(svc, "nginx") {
				svcName = "apache"
			}
			action := "Increase pm.max_children or reduce PHP execution time."
			dashLink := fmt.Sprintf("/nginx/%s/dashboard", ec.machineID)
			msg := fmt.Sprintf("Web server 5xx errors (%.1f%%) firing alongside PHP-FPM saturation (%.0f%%). Pool cannot keep up.", *web5xx*100, *phpSat*100)
			w.upsertFinding(ctx, tenantID, ec, "web_app_exhaustion", "critical",
				"Web tier errors correlate with PHP-FPM pool exhaustion", msg,
				fmt.Sprintf(`{"web_5xx_pct": %.1f, "phpfpm_saturation_pct": %.0f}`, *web5xx*100, *phpSat*100),
				&svcName, &action, &dashLink)
		}
	}

	// ── Rule 2: app_db_bottleneck — (phpfpm p95 > 1.5s) AND (DB slow > 0.5/s) ──
	if hasAnySvc(svc, "phpfpm") && (hasAnySvc(svc, "mariadb") || hasAnySvc(svc, "postgres")) {
		phpSlow := w.queryVM(ctx, tenantID,
			fmt.Sprintf(`phpfpm_request_duration_seconds{host_id="%s",quantile="0.95"} > 1.5`, ec.machineID), timeSec)
		var dbName string
		var dbSlow *float64
		if hasAnySvc(svc, "mariadb") {
			dbName = "mariadb"
			dbSlow = w.queryVM(ctx, tenantID,
				fmt.Sprintf(`rate(mysql_global_status_slow_queries{host_id="%s"}[5m]) > 0.5`, ec.machineID), timeSec)
		} else {
			dbName = "postgres"
			dbSlow = w.queryVM(ctx, tenantID,
				fmt.Sprintf(`rate(pg_stat_database_conflicts_total{host_id="%s"}[5m]) > 0.5`, ec.machineID), timeSec)
		}
		if phpSlow != nil && *phpSlow > 1.5 && dbSlow != nil && *dbSlow > 0.5 {
			activeRules["app_db_bottleneck"] = true
			action := "Tune the slowest DB query or add an index."
			dashLink := fmt.Sprintf("/phpfpm/%s/dashboard", ec.machineID)
			msg := fmt.Sprintf("PHP-FPM p95 response time is %.1fs and %s slow queries are elevated (%.1f/s). App is DB-bound.", *phpSlow, dbName, *dbSlow)
			w.upsertFinding(ctx, tenantID, ec, "app_db_bottleneck", "warning",
				"Slow PHP requests correlate with database slow queries", msg,
				fmt.Sprintf(`{"php_p95_rt": %.1f, "db_slow_qps": %.1f}`, *phpSlow, *dbSlow),
				nil, &action, &dashLink)
		}
	}

	// ── Rule 3: db_disk_bottleneck — (mariadb hit < 95%) AND (disk read latency > 100ms) ──
	if hasAnySvc(svc, "mariadb") {
		bp := w.queryVM(ctx, tenantID,
			fmt.Sprintf(`mysql_global_status_innodb_buffer_pool_hit_ratio{host_id="%s"} < 0.95`, ec.machineID), timeSec)
		ioLat := w.queryVM(ctx, tenantID,
			fmt.Sprintf(`rate(system_disk_io_time_seconds_total{host_id="%s"}[5m]) / rate(system_disk_io_completed_total{host_id="%s"}[5m]) * 1000 > 100`, ec.machineID, ec.machineID), timeSec)
		if bp != nil && *bp < 0.95 && ioLat != nil && *ioLat > 100 {
			activeRules["db_disk_bottleneck"] = true
			action := "Increase innodb_buffer_pool_size or check disk health."
			dashLink := fmt.Sprintf("/mariadb/%s/dashboard", ec.machineID)
			msg := fmt.Sprintf("InnoDB hit ratio is %.1f%% and disk read latency is %.0fms. Working set exceeds buffer pool.", *bp*100, *ioLat)
			w.upsertFinding(ctx, tenantID, ec, "db_disk_bottleneck", "warning",
				"Buffer pool pressure correlates with disk I/O saturation", msg,
				fmt.Sprintf(`{"bp_hit_ratio": %.1f, "disk_io_ms": %.0f}`, *bp*100, *ioLat),
				nil, &action, &dashLink)
		}
	}

	// ── Rule 4: cache_miss_cascade — (redis hit < 90%) AND (DB queries > 100/s) ──
	if hasAnySvc(svc, "redis") && (hasAnySvc(svc, "mariadb") || hasAnySvc(svc, "postgres")) {
		cacheHit := w.queryVM(ctx, tenantID,
			fmt.Sprintf(`rate(redis_keyspace_hits_total{host_id="%s"}[5m]) / (rate(redis_keyspace_hits_total{host_id="%s"}[5m]) + rate(redis_keyspace_misses_total{host_id="%s"}[5m])) < 0.9`, ec.machineID, ec.machineID, ec.machineID), timeSec)
		var dbQPS *float64
		if hasAnySvc(svc, "mariadb") {
			dbQPS = w.queryVM(ctx, tenantID,
				fmt.Sprintf(`rate(mysql_global_status_queries{host_id="%s"}[5m])`, ec.machineID), timeSec)
		} else {
			dbQPS = w.queryVM(ctx, tenantID,
				fmt.Sprintf(`rate(pg_stat_database_tup_fetched{host_id="%s"}[5m])`, ec.machineID), timeSec)
		}
		if cacheHit != nil && *cacheHit < 0.9 && dbQPS != nil && *dbQPS > 100 {
			activeRules["cache_miss_cascade"] = true
			action := "Check Redis maxmemory setting and eviction policy."
			dashLink := fmt.Sprintf("/redis/%s/dashboard", ec.machineID)
			msg := fmt.Sprintf("Redis hit ratio is %.1f%% while DB query rate is high (%.0f/s). Cache may be undersized.", *cacheHit*100, *dbQPS)
			w.upsertFinding(ctx, tenantID, ec, "cache_miss_cascade", "warning",
				"Falling cache hit ratio correlates with rising DB load", msg,
				fmt.Sprintf(`{"cache_hit_pct": %.1f, "db_qps": %.0f}`, *cacheHit*100, *dbQPS),
				nil, &action, &dashLink)
		}
	}

	// ── Rule 5: host_exhaustion — (CPU > 90% OR mem > 90%) AND ≥2 services degraded ──
	if len(svc) >= 2 {
		cpuSat := w.queryVM(ctx, tenantID,
			fmt.Sprintf(`(1 - avg(system_cpu_utilization_ratio{host_id="%s",state="idle"}) > 0) * 100 > 90`, ec.machineID), timeSec)
		memSat := w.queryVM(ctx, tenantID,
			fmt.Sprintf(`sum(system_memory_usage_bytes{host_id="%s",state="used"}) / sum(system_memory_usage_bytes{host_id="%s",state=~"used|free|buffered|cached"}) * 100 > 90`, ec.machineID, ec.machineID), timeSec)
		if (cpuSat != nil && *cpuSat > 0) || (memSat != nil && *memSat > 0) {
			degradedCount := 0
			for _, s := range svc {
				if !s.MetricColl {
					continue
				}
				h := w.getServiceHealth(ctx, tenantID, tenantUUID, ec.machineID, s, timeSec, startNs, endNs)
				if h == "degraded" || h == "down" {
					degradedCount++
				}
			}
			if degradedCount >= 2 {
				activeRules["host_exhaustion"] = true
				action := "Investigate top CPU/MEM processes; consider scaling the host."
				msg := fmt.Sprintf("Resources saturated (CPU or mem >90%%) and %d services degraded. Host-level issue.", degradedCount)
				w.upsertFinding(ctx, tenantID, ec, "host_exhaustion", "critical",
					"Host resource exhaustion affecting multiple services", msg,
					fmt.Sprintf(`{"degraded_services": %d}`, degradedCount),
					nil, &action, nil)
			}
		}
	}

	// ── Rule 6: auth_brute_force — (401 rate > 10/s) AND (SSH failed > 10/5m) ──
	if hasAnySvc(svc, "nginx", "apache") {
		auth401 := w.queryVM(ctx, tenantID,
			fmt.Sprintf(`rate(nginx_http_requests_total{host_id="%s",status=~"401|403"}[5m])`, ec.machineID), timeSec)
		if auth401 == nil && hasAnySvc(svc, "apache") {
			auth401 = w.queryVM(ctx, tenantID,
				fmt.Sprintf(`rate(httpd_requests_total{host_id="%s",status=~"401|403"}[5m])`, ec.machineID), timeSec)
		}
		// SSH bruteforce from journald logs
		sshQ := fmt.Sprintf(`{service_name="journald", host_id=%q} | SYSLOG_IDENTIFIER="sshd" |~ "(?i)Failed password|authentication failure"`, ec.machineID)
		sshEntries, _ := w.lokiQueryLogs(ctx, tenantUUID, sshQ, startNs, endNs, 100)
		if auth401 != nil && *auth401 > 10 && len(sshEntries) > 10 {
			activeRules["auth_brute_force"] = true
			action := "Rate-limit or block offending IPs at the reverse-proxy level."
			dashLink := fmt.Sprintf("/nginx/%s/ip-investigation", ec.machineID)
			msg := fmt.Sprintf("Web 401/403 rate is %.0f/s and SSH failed attempts are %d/5m. Possible coordinated brute-force attack.", *auth401, len(sshEntries))
			w.upsertFinding(ctx, tenantID, ec, "auth_brute_force", "critical",
				"Coordinated brute-force attack across web and SSH", msg,
				fmt.Sprintf(`{"web_401_rate": %.0f, "ssh_failed": %d}`, *auth401, len(sshEntries)),
				nil, &action, &dashLink)
		}
	}

	// ── Rule 7: http_4xx_high — 4xx rate > 10% of total requests ──
	if hasAnySvc(svc, "nginx", "apache") {
		totalRate := w.queryVM(ctx, tenantID,
			fmt.Sprintf(`rate(nginx_http_requests_total{host_id="%s"}[5m])`, ec.machineID), timeSec)
		if totalRate == nil && hasAnySvc(svc, "apache") {
			totalRate = w.queryVM(ctx, tenantID,
				fmt.Sprintf(`rate(httpd_requests_total{host_id="%s"}[5m])`, ec.machineID), timeSec)
		}
		total4xx := w.queryVM(ctx, tenantID,
			fmt.Sprintf(`rate(nginx_http_requests_total{host_id="%s",status=~"4.."}[5m])`, ec.machineID), timeSec)
		if total4xx == nil && hasAnySvc(svc, "apache") {
			total4xx = w.queryVM(ctx, tenantID,
				fmt.Sprintf(`rate(httpd_requests_total{host_id="%s",status=~"4.."}[5m])`, ec.machineID), timeSec)
		}
		if totalRate != nil && *totalRate > 0 && total4xx != nil {
			pct := (*total4xx / *totalRate) * 100
			if pct > 10 {
				activeRules["http_4xx_high"] = true
				action := "Review recent deploys for broken paths; run config test."
				dashLink := fmt.Sprintf("/nginx/%s/explorer", ec.machineID)
				msg := fmt.Sprintf("HTTP 4xx rate is %.1f%% of total requests (%.0f/s out of %.0f/s). Client error surge.", pct, *total4xx, *totalRate)
				w.upsertFinding(ctx, tenantID, ec, "http_4xx_high", "warning",
					"Elevated HTTP 4xx client error rate", msg,
					fmt.Sprintf(`{"4xx_pct": %.1f, "total_rps": %.0f}`, pct, *totalRate),
					nil, &action, &dashLink)
			}
		}
	}

	// ── Rule 8: mariadb_deadlock_spike — deadlocks/min > 5 ──
	if hasAnySvc(svc, "mariadb") {
		deadlocks := w.queryVM(ctx, tenantID,
			fmt.Sprintf(`rate(mysql_global_status_innodb_deadlocks{host_id="%s"}[5m])`, ec.machineID), timeSec)
		if deadlocks != nil {
			perMin := *deadlocks * 60
			if perMin > 5 {
				activeRules["mariadb_deadlock_spike"] = true
				action := "Add covering indexes; reduce transaction scope."
				dashLink := fmt.Sprintf("/mariadb/%s/dashboard", ec.machineID)
				msg := fmt.Sprintf("InnoDB deadlock rate is %.1f/min (threshold: 5/min). Transactions are conflicting.", perMin)
				w.upsertFinding(ctx, tenantID, ec, "mariadb_deadlock_spike", "critical",
					"MariaDB deadlock spike detected", msg,
					fmt.Sprintf(`{"deadlocks_per_min": %.1f}`, perMin),
					nil, &action, &dashLink)
			}
		}
	}

	// ── Rule 9: postgres_replication_lag — replication lag > 10s ──
	if hasAnySvc(svc, "postgres") {
		lag := w.queryVM(ctx, tenantID,
			fmt.Sprintf(`pg_stat_replication_lag_seconds{host_id="%s"} > 10`, ec.machineID), timeSec)
		if lag != nil && *lag > 10 {
			activeRules["postgres_replication_lag"] = true
			action := "Check network latency between primary and replica."
			dashLink := fmt.Sprintf("/postgres/%s/dashboard", ec.machineID)
			msg := fmt.Sprintf("PostgreSQL replication lag is %.0fs (threshold: 10s). Replica may have stale data.", *lag)
			w.upsertFinding(ctx, tenantID, ec, "postgres_replication_lag", "warning",
				"PostgreSQL replication lag detected", msg,
				fmt.Sprintf(`{"lag_seconds": %.0f}`, *lag),
				nil, &action, &dashLink)
		}
	}

	// Resolve any findings for this host that are no longer active.
	for _, ruleID := range allCorrelationRuleIDs() {
		if !activeRules[ruleID] {
			if err := w.correlationFindingRepo.ResolveFinding(ctx, tenantID, ec.machineID, ruleID); err != nil {
				log.Printf("[correlation-worker] resolve %s for host %s: %v", ruleID, ec.machineID, err)
			}
		}
	}
}

func allCorrelationRuleIDs() []string {
	return []string{
		"web_app_exhaustion", "app_db_bottleneck", "db_disk_bottleneck",
		"cache_miss_cascade", "host_exhaustion", "auth_brute_force",
		"http_4xx_high", "mariadb_deadlock_spike", "postgres_replication_lag",
	}
}

func (w *CorrelationWorker) upsertFinding(
	ctx context.Context, tenantID uint, ec hostEvalContext,
	ruleID, severity, title, message, evidenceJSON string,
	serviceName, recommendedAction, dashboardLink *string,
) {
	now := time.Now()
	finding := &domain.CorrelationFinding{
		TenantID:          tenantID,
		MachineID:         ec.machineID,
		RuleID:            ruleID,
		ServiceName:       serviceName,
		Severity:          severity,
		Title:             title,
		Message:           message,
		Evidence:          evidenceJSON,
		RecommendedAction: recommendedAction,
		DashboardLink:     dashboardLink,
		Hostname:          ec.hostname,
		FiredAt:           now,
	}
	if err := w.correlationFindingRepo.UpsertFinding(ctx, finding); err != nil {
		log.Printf("[correlation-worker] upsert %s for host %s: %v", ruleID, ec.machineID, err)
	} else {
		log.Printf("[correlation-worker] UPSERTED rule=%s host=%s severity=%s", ruleID, ec.hostname, severity)
	}
}

// ─── Query helpers ────────────────────────────────────────────────────────────

func (w *CorrelationWorker) queryVM(ctx context.Context, tenantID uint, query string, timeSec int64) *float64 {
	upstream := fmt.Sprintf("http://%s:%s/select/%d/prometheus/api/v1/query",
		w.cfg.VMSelectHost, w.cfg.VMSelectPort, tenantID)
	params := url.Values{}
	params.Set("query", query)
	params.Set("time", strconv.FormatInt(timeSec, 10))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream+"?"+params.Encode(), nil)
	if err != nil {
		return nil
	}
	resp, err := w.httpClient.Do(req)
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

func (w *CorrelationWorker) queryLogErrorCount(ctx context.Context, tenantUUID, machineID, serviceName string, startNs, endNs int64) float64 {
	q := fmt.Sprintf(`count_over_time({service=%q, host_id=%q} |= "error" [5m])`, serviceName, machineID)
	rows, err := w.lokiQueryRange(ctx, tenantUUID, q, startNs, endNs, "5m", 1)
	if err != nil || len(rows) == 0 || len(rows[0].Values) == 0 {
		return 0
	}
	return rows[0].Values[len(rows[0].Values)-1][1]
}

func (w *CorrelationWorker) lokiQueryLogs(ctx context.Context, tenantUUID, query string, startNs, endNs int64, limit int) ([]LokiLogEntry, error) {
	upstream := fmt.Sprintf("http://%s:%s/loki/api/v1/query_range", w.cfg.LokiHost, w.cfg.LokiPort)
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
	resp, err := w.httpClient.Do(req)
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

func (w *CorrelationWorker) lokiQueryRange(ctx context.Context, tenantUUID, query string, startNs, endNs int64, step string, limit int) ([]LokiMatrixSeries, error) {
	upstream := fmt.Sprintf("http://%s:%s/loki/api/v1/query_range", w.cfg.LokiHost, w.cfg.LokiPort)
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
	resp, err := w.httpClient.Do(req)
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

func (w *CorrelationWorker) getServiceHealth(
	ctx context.Context, tenantID uint, tenantUUID, machineID string,
	svc serviceBrief, timeSec int64, startNs, endNs int64,
) string {
	if !svc.MetricColl {
		errCount := w.queryLogErrorCount(ctx, tenantUUID, machineID, svc.Name, startNs, endNs)
		if errCount > 10 {
			return "degraded"
		}
		return "unknown"
	}

	up := w.queryVM(ctx, tenantID, fmt.Sprintf(`%s_up{host_id="%s"}`, svc.Name, machineID), timeSec)
	if up != nil && *up == 0 {
		return "down"
	}

	scrapeErr := w.queryVM(ctx, tenantID, fmt.Sprintf(`%s_scrape_error{host_id="%s"}`, svc.Name, machineID), timeSec)
	if scrapeErr != nil && *scrapeErr > 0 {
		return "down"
	}

	switch svc.Name {
	case "phpfpm":
		sat := w.queryVM(ctx, tenantID,
			fmt.Sprintf(`phpfpm_active_workers{host_id="%s"} / phpfpm_total_workers{host_id="%s"} > 0.85`, machineID, machineID), timeSec)
		if sat != nil && *sat > 0.85 {
			return "degraded"
		}
	case "mariadb":
		bp := w.queryVM(ctx, tenantID,
			fmt.Sprintf(`mysql_global_status_innodb_buffer_pool_hit_ratio{host_id="%s"} < 0.95`, machineID), timeSec)
		if bp != nil && *bp < 0.95 {
			return "degraded"
		}
	case "postgres":
		conn := w.queryVM(ctx, tenantID,
			fmt.Sprintf(`pg_stat_activity_count{host_id="%s",datname!~"template.*|postgres"} > 80`, machineID), timeSec)
		if conn != nil && *conn > 80 {
			return "degraded"
		}
	case "redis":
		hit := w.queryVM(ctx, tenantID,
			fmt.Sprintf(`rate(redis_keyspace_hits_total{host_id="%s"}[5m]) / (rate(redis_keyspace_hits_total{host_id="%s"}[5m]) + rate(redis_keyspace_misses_total{host_id="%s"}[5m])) < 0.9`, machineID, machineID, machineID), timeSec)
		if hit != nil && *hit < 0.9 {
			return "degraded"
		}
	case "nginx", "apache":
		err5xx := w.queryVM(ctx, tenantID,
			fmt.Sprintf(`rate(%s_http_requests_total{host_id="%s",status=~"5.."}[5m]) / rate(%s_http_requests_total{host_id="%s"}[5m]) > 0.05`, svc.Name, machineID, svc.Name, machineID), timeSec)
		if err5xx != nil && *err5xx > 0.05 {
			return "degraded"
		}
	}

	errCount := w.queryLogErrorCount(ctx, tenantUUID, machineID, svc.Name, startNs, endNs)
	if errCount > 50 {
		return "degraded"
	}

	return "healthy"
}

func hasAnySvc(m map[string]serviceBrief, names ...string) bool {
	for _, n := range names {
		if _, ok := m[n]; ok {
			return true
		}
	}
	return false
}
