package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
)

// ─── Issue types ───────────────────────────────────────────────────────────────

type DiagnosedIssue struct {
	ID         string         `json:"id"`
	MachineID  string         `json:"machine_id"`
	Hostname   string         `json:"hostname"`
	Service    string         `json:"service"`
	Severity   string         `json:"severity"`
	Diagnosis  string         `json:"diagnosis"`
	What       string         `json:"what"`
	Why        string         `json:"why"`
	Impact     string         `json:"impact"`
	Action     string         `json:"action"`
	Evidence   []EvidenceItem `json:"evidence"`
	LogSnippet string         `json:"log_snippet,omitempty"`
	FiredAt    string         `json:"fired_at"`
	Duration   string         `json:"duration"`
}

type issueTemplate struct {
	Severity   string
	Diagnosis  string
	WhatTmpl   string
	WhyTmpl    string
	ImpactTmpl string
	ActionTmpl string
}

var issueTemplates = map[string]issueTemplate{
	// ── Nginx ──
	"nginx_5xx_spike": {
		Severity: "critical", Diagnosis: "High HTTP 5xx Error Rate on Nginx",
		WhatTmpl:   "Nginx is returning HTTP 5xx errors at %.1f%% (threshold: 5%%)",
		WhyTmpl:    "The upstream application pool has %d/%d workers active (%.0f%% saturation). Recent error log: %s",
		ImpactTmpl: "%.0f%% of recent requests are failing with HTTP 5xx errors",
		ActionTmpl: "Increase pm.max_children in php-fpm pool config or check upstream application health",
	},
	"nginx_4xx_high": {
		Severity: "warning", Diagnosis: "Elevated HTTP 4xx Rate on Nginx",
		WhatTmpl:   "Nginx 4xx rate is %.1f%% (threshold: 15%%)",
		WhyTmpl:    "High rate of client errors — most common status code is %s (%d responses)",
		ImpactTmpl: "%.0f%% of responses are client errors, potential misconfiguration or scanning activity",
		ActionTmpl: "Review nginx access logs for the most-hit 4xx paths; check recent configuration changes",
	},
	"nginx_4xx_auth": {
		Severity: "critical", Diagnosis: "Possible Brute-Force Attack on Nginx",
		WhatTmpl:   "Nginx reporting %.0f 401/403 responses per minute from %d unique IPs",
		WhyTmpl:    "Authentication failure rate is %.0f/min combined with known attack paths (wp-login, xmlrpc, admin)",
		ImpactTmpl: "Unauthorized access attempts against authentication endpoints at %.0f/min",
		ActionTmpl: "Enable fail2ban, rate-limit/block offending IPs, or restrict admin paths by IP whitelist",
	},
	"nginx_conn_saturation": {
		Severity: "warning", Diagnosis: "Nginx Connection Pool Near Saturation",
		WhatTmpl:   "Nginx active connections at %.0f (%.0f%% of worker_connections)",
		WhyTmpl:    "Connection pool is near saturation — reading: %.0f, writing: %.0f, waiting: %.0f",
		ImpactTmpl: "New connections may be queued or dropped if pool is exhausted",
		ActionTmpl: "Increase nginx worker_connections or add additional nginx workers",
	},
	"nginx_traffic_drop": {
		Severity: "warning", Diagnosis: "Sudden Traffic Drop Detected on Nginx",
		WhatTmpl:   "Current request rate (%.1f req/s) is %.0f%% below the 1-hour average (%.1f req/s)",
		WhyTmpl:    "Traffic dropped significantly — possible upstream DNS resolution failure, network issue, or CDN misconfiguration",
		ImpactTmpl: "Request volume decreased by %.0f%% compared to the 1-hour average",
		ActionTmpl: "Check DNS resolution, upstream network connectivity, and recent deploy changes",
	},
	"nginx_traffic_spike": {
		Severity: "info", Diagnosis: "Traffic Spike Detected on Nginx",
		WhatTmpl:   "Current request rate (%.1f req/s) is %.0f%% above the 1-hour average (%.1f req/s)",
		WhyTmpl:    "Traffic spike detected — this may be legitimate or due to a DDoS / load test",
		ImpactTmpl: "Request volume increased by %.0f%% compared to the 1-hour average; monitor resource usage",
		ActionTmpl: "Monitor CPU/memory/bandwidth; enable rate limiting if traffic is unexpected",
	},

	// ── Apache (mirrors nginx) ──
	"apache_5xx_spike": {
		Severity: "critical", Diagnosis: "High HTTP 5xx Error Rate on Apache",
		WhatTmpl:   "Apache is returning HTTP 5xx errors at %.1f%% (threshold: 5%%)",
		WhyTmpl:    "The upstream application pool has %d/%d workers active (%.0f%% saturation). Recent error log: %s",
		ImpactTmpl: "%.0f%% of recent requests are failing with HTTP 5xx errors",
		ActionTmpl: "Increase Apache MaxRequestWorkers or check upstream application health",
	},
	"apache_4xx_high": {
		Severity: "warning", Diagnosis: "Elevated HTTP 4xx Rate on Apache",
		WhatTmpl:   "Apache 4xx rate is %.1f%% (threshold: 15%%)",
		WhyTmpl:    "High rate of client errors — most common status code is %s (%d responses)",
		ImpactTmpl: "%.0f%% of responses are client errors, potential misconfiguration or scanning activity",
		ActionTmpl: "Review Apache access logs for the most-hit 4xx paths; check recent configuration changes",
	},
	"apache_4xx_auth": {
		Severity: "critical", Diagnosis: "Possible Brute-Force Attack on Apache",
		WhatTmpl:   "Apache reporting %.0f 401/403 responses per minute from %d unique IPs",
		WhyTmpl:    "Authentication failure rate is %.0f/min combined with known attack paths",
		ImpactTmpl: "Unauthorized access attempts against authentication endpoints at %.0f/min",
		ActionTmpl: "Enable fail2ban, rate-limit/block offending IPs, or restrict admin paths by IP whitelist",
	},

	// ── PHP-FPM ──
	"phpfpm_pool_exhaustion": {
		Severity: "critical", Diagnosis: "PHP-FPM Worker Pool Exhaustion",
		WhatTmpl:   "PHP-FPM worker pool is at %d/%d active workers (%.0f%% saturation, threshold: 85%%)",
		WhyTmpl:    "All %d workers are busy processing requests. %d requests are waiting in the listen queue (depth: %d). Pool cannot keep up with request volume",
		ImpactTmpl: "%.0f%% of PHP requests are queuing — response times will increase and 502 errors may appear on the web tier",
		ActionTmpl: "Increase pm.max_children in php-fpm pool config (currently %d) or optimize PHP script execution times",
	},
	"phpfpm_queue_buildup": {
		Severity: "warning", Diagnosis: "PHP-FPM Listen Queue Backlog",
		WhatTmpl:   "PHP-FPM listen queue depth is %d (max: %d, utilization: %.0f%%)",
		WhyTmpl:    "Requests are waiting in the listen queue because all workers are busy. Queue depth has been >0 for %s",
		ImpactTmpl: "%.0f requests are waiting to be processed — response times will be delayed",
		ActionTmpl: "Increase pm.max_children or pm.max_spare_servers; check if a script is blocking workers",
	},
	"phpfpm_slow_requests": {
		Severity: "warning", Diagnosis: "PHP-FPM Slow Request Rate Elevated",
		WhatTmpl:   "PHP-FPM slow request rate is %.1f/min (cumulative: %.0f slow requests in window)",
		WhyTmpl:    "Slow request log indicates scripts taking longer than pm.request_terminate_timeout to execute",
		ImpactTmpl: "%.0f slow requests in the last 5 minutes are blocking worker processes",
		ActionTmpl: "Check phpfpm slow log for the slowest scripts; optimize database queries and external API calls",
	},
	"phpfpm_opcache_low_hit": {
		Severity: "warning", Diagnosis: "PHP-FPM OpCache Hit Rate Low",
		WhatTmpl:   "OpCache hit rate is %.1f%% (threshold: 85%%)",
		WhyTmpl:    "Low opcache hit rate means PHP files are being recompiled frequently, wasting CPU cycles. opcache_memory_used: %.0f bytes, cached scripts: %.0f",
		ImpactTmpl: "PHP execution is %.0f%% slower due to unnecessary recompilation",
		ActionTmpl: "Increase opcache.memory_consumption or opcache.max_accelerated_files in php.ini",
	},
	"phpfpm_opcache_wasted": {
		Severity: "info", Diagnosis: "PHP-FPM OpCache Memory Fragmentation",
		WhatTmpl:   "OpCache wasted memory ratio is %.1f%% (threshold: 5%%)",
		WhyTmpl:    "OpCache memory is becoming fragmented — %.0f bytes wasted out of %.0f total. May trigger OOM restarts soon",
		ImpactTmpl: "OpCache efficiency is degrading due to memory fragmentation",
		ActionTmpl: "Monitor opcache.revalidate_freq setting; consider increasing opcache.memory_consumption",
	},
	"phpfpm_max_children_reached": {
		Severity: "critical", Diagnosis: "PHP-FPM max_children Reached",
		WhatTmpl:   "PHP-FPM has reached pm.max_children (%d) — the pool is completely saturated",
		WhyTmpl:    "pm.max_children setting (%d) was reached, meaning all available worker processes are in use. New requests will be queued or rejected",
		ImpactTmpl: "New PHP requests are being queued or rejected — web tier will return 502/503 errors",
		ActionTmpl: "Increase pm.max_children in php-fpm pool config (currently %d); review if a script is hanging",
	},

	// ── MariaDB ──
	"mariadb_buffer_pool_pressure": {
		Severity: "warning", Diagnosis: "MariaDB InnoDB Buffer Pool Pressure",
		WhatTmpl:   "InnoDB buffer pool hit ratio is %.1f%% (threshold: 95%%). Disk reads: %.0f/s",
		WhyTmpl:    "Working set exceeds the buffer pool size — database is reading from disk instead of memory. Current pool size may be insufficient",
		ImpactTmpl: "Query performance is degraded due to frequent disk reads (%.0f reads/s from disk)",
		ActionTmpl: "Increase innodb_buffer_pool_size (currently needs +%d MB) or optimize query access patterns",
	},
	"mariadb_slow_queries": {
		Severity: "warning", Diagnosis: "MariaDB Slow Query Rate Elevated",
		WhatTmpl:   "MariaDB slow query rate is %.1f/s (long_query_time threshold exceeded)",
		WhyTmpl:    "Queries exceeding long_query_time are running at %.1f/s. Poor indexing or large table scans may be the cause",
		ImpactTmpl: "%.0f slow queries per minute are degrading application response times",
		ActionTmpl: "Check slow query log (/var/log/mysql/mariadb-slow.log); add missing indexes; optimize SQL",
	},
	"mariadb_conn_saturation": {
		Severity: "warning", Diagnosis: "MariaDB Connection Pool Near Saturation",
		WhatTmpl:   "MariaDB connection pool at %.0f%% (%d/%d connections used)",
		WhyTmpl:    "Connection pool is near the max_connections limit — new connections may be rejected",
		ImpactTmpl: "Applications may fail to connect to the database if pool is exhausted",
		ActionTmpl: "Increase max_connections in my.cnf or investigate connection leaks in application code",
	},
	"mariadb_deadlocks": {
		Severity: "critical", Diagnosis: "MariaDB Deadlocks Detected",
		WhatTmpl:   "MariaDB deadlocks occurring at %.1f/min",
		WhyTmpl:    "Transactions are deadlocking — likely due to inconsistent lock ordering across concurrent transactions",
		ImpactTmpl: "Affected transactions are being rolled back, causing application errors",
		ActionTmpl: "Review application transaction logic; ensure consistent lock ordering; add covering indexes",
	},
	"mariadb_replication_lag": {
		Severity: "warning", Diagnosis: "MariaDB Replication Lag Detected",
		WhatTmpl:   "MariaDB replication lag is %.0f seconds (threshold: 30s). Slave IO: %s, Slave SQL: %s",
		WhyTmpl:    "Replica is falling behind the primary — check network throughput and long-running queries on primary",
		ImpactTmpl: "Read replicas have stale data; read-after-write consistency may be broken",
		ActionTmpl: "Check slave status (SHOW SLAVE STATUS); investigate if a长时间 running query is blocking",
	},

	// ── PostgreSQL ──
	"postgres_conn_saturation": {
		Severity: "warning", Diagnosis: "PostgreSQL Connection Pool Near Saturation",
		WhatTmpl:   "PostgreSQL connection pool at %.0f%% (%d/%d connections used)",
		WhyTmpl:    "Connection pool is near max_connections — new connections may be rejected",
		ImpactTmpl: "Applications may fail to connect if pool is exhausted",
		ActionTmpl: "Increase max_connections in postgresql.conf or use connection pooling (PgBouncer)",
	},
	"postgres_cache_hit_low": {
		Severity: "warning", Diagnosis: "PostgreSQL Cache Hit Ratio Low",
		WhatTmpl:   "PostgreSQL shared_buffers hit ratio is %.1f%% (threshold: 95%%)",
		WhyTmpl:    "Working set exceeds shared_buffers — database is reading from disk. Current: %d MB, effective_cache: %d MB",
		ImpactTmpl: "Query performance degraded due to frequent disk reads",
		ActionTmpl: "Increase shared_buffers in postgresql.conf; review effective_cache_size",
	},
	"postgres_deadlocks": {
		Severity: "critical", Diagnosis: "PostgreSQL Deadlocks Detected",
		WhatTmpl:   "PostgreSQL deadlock rate is %.1f/min",
		WhyTmpl:    "Concurrent transactions are deadlocking due to conflicting lock orders",
		ImpactTmpl: "Affected transactions are being rolled back, causing application errors",
		ActionTmpl: "Review transaction isolation levels and lock ordering; add indexes to reduce lock contention",
	},
	"postgres_replication_lag": {
		Severity: "warning", Diagnosis: "PostgreSQL Replication Lag Detected",
		WhatTmpl:   "PostgreSQL replication lag is %.0f seconds (threshold: 30s). WAL rate: %.0f/s",
		WhyTmpl:    "Replica is falling behind the primary — check network I/O and primary workload",
		ImpactTmpl: "Read replicas have stale data; read-after-write consistency may be broken",
		ActionTmpl: "Check pg_stat_replication; investigate network bandwidth and primary query load",
	},
	"postgres_long_tx": {
		Severity: "warning", Diagnosis: "Long-Running PostgreSQL Transactions",
		WhatTmpl:   "Longest transaction has been running for %.0f seconds (threshold: 300s). %d transactions exceed threshold",
		WhyTmpl:    "Long-running transactions hold locks and block autovacuum, causing table bloat and performance degradation",
		ImpactTmpl: "%d transactions exceeding 5 minutes are holding locks and blocking cleanup operations",
		ActionTmpl: "Check pg_stat_activity for idle-in-transaction queries; set statement_timeout in application",
	},

	// ── Redis ──
	"redis_memory_pressure": {
		Severity: "warning", Diagnosis: "Redis Memory Usage High",
		WhatTmpl:   "Redis memory usage is %.0f%% (%.0f MB / %.0f MB, threshold: 80%%)",
		WhyTmpl:    "Memory usage approaches maxmemory — eviction policy (%s) will start removing keys",
		ImpactTmpl: "Cache effectiveness will degrade as keys are evicted under memory pressure",
		ActionTmpl: "Increase Redis maxmemory or reduce TTL on cached keys; review eviction policy",
	},
	"redis_cache_miss_spike": {
		Severity: "warning", Diagnosis: "Redis Cache Hit Ratio Dropped",
		WhatTmpl:   "Redis cache hit ratio is %.1f%% (threshold: 90%%). Hits: %.0f/s, Misses: %.0f/s",
		WhyTmpl:    "Cache hit ratio dropped — cache keys may have expired simultaneously or the working set changed",
		ImpactTmpl: "%.0f%% of cache lookups miss and fall through to the database, increasing DB load",
		ActionTmpl: "Review key TTL values; check for cache stampede scenarios; consider warming the cache",
	},
	"redis_evictions": {
		Severity: "warning", Diagnosis: "Redis Keys Being Evicted",
		WhatTmpl:   "Redis eviction rate is %.1f/s (cumulative: %.0f evictions in window)",
		WhyTmpl:    "Keys are being evicted due to memory pressure under the %s eviction policy. maxmemory: %.0f MB, used: %.0f MB",
		ImpactTmpl: "%.0f keys evicted — frequently accessed data may be missing from cache",
		ActionTmpl: "Increase Redis maxmemory or optimize memory usage; review eviction policy (allkeys-lru recommended)",
	},
	"redis_oom": {
		Severity: "critical", Diagnosis: "Redis Out of Memory Errors",
		WhatTmpl:   "Redis reporting OOM errors — 'OOM command not allowed when used memory > maxmemory'",
		WhyTmpl:    "Redis has reached maxmemory with a noeviction policy — write commands are being rejected",
		ImpactTmpl: "Write operations to Redis are failing — application features relying on Redis writes are broken",
		ActionTmpl: "Set an eviction policy (e.g. allkeys-lru) or increase maxmemory in redis.conf",
	},

	// ── Host ──
	"host_cpu_saturation": {
		Severity: "warning", Diagnosis: "Host CPU Saturation",
		WhatTmpl:   "Host CPU usage is at %.0f%% (threshold: 90%%). Load average: %.1f (1m), %.1f (5m)",
		WhyTmpl:    "CPU is saturated — processes are competing for CPU time. Top processes may be CPU-bound or there may be a runaway process",
		ImpactTmpl: "All services on this host may experience slowdowns due to CPU starvation",
		ActionTmpl: "Identify top CPU processes with 'top' or 'htop'; consider vertical scaling or moving workloads",
	},
	"host_memory_pressure": {
		Severity: "warning", Diagnosis: "Host Memory Pressure",
		WhatTmpl:   "Host memory usage is at %.0f%% (threshold: 90%%). Swap: %.0f%% used (%.0f MB / %.0f MB)",
		WhyTmpl:    "System is running low on available memory — swap usage indicates memory pressure. OOM killer may become active",
		ImpactTmpl: "Services may be swapped out or killed by OOM killer under continued memory pressure",
		ActionTmpl: "Check top memory consumers with 'ps aux --sort=-%mem'; add RAM or reduce service footprint",
	},
	"host_disk_space": {
		Severity: "warning", Diagnosis: "Host Disk Space Low",
		WhatTmpl:   "Disk usage on / is at %.0f%% (threshold: 85%%). Total: %.0f GB, Used: %.0f GB, Free: %.0f GB",
		WhyTmpl:    "Disk is filling up — services may fail to write logs or data if space is exhausted",
		ImpactTmpl: "Log collection, database writes, and application temp files may fail",
		ActionTmpl: "Clean up old logs with 'logrotate'; remove unused Docker images; investigate disk usage with 'du -sh /*'",
	},
	"host_disk_io": {
		Severity: "warning", Diagnosis: "Host Disk I/O Latency Elevated",
		WhatTmpl:   "Average disk I/O latency is %.0f ms (threshold: 100ms). Queue depth: %.1f, I/O throughput: %.0f KB/s",
		WhyTmpl:    "Disk I/O latency is high — processes are waiting for disk operations to complete. This may affect database performance",
		ImpactTmpl: "Database query performance and file I/O operations are degraded due to slow disk",
		ActionTmpl: "Check iostat output; identify I/O-heavy processes; consider faster storage or move I/O workloads",
	},
	"host_oom_killer": {
		Severity: "critical", Diagnosis: "OOM Killer Active on Host",
		WhatTmpl:   "OOM killer invoked %d times in the last 5 minutes — processes are being killed due to memory exhaustion",
		WhyTmpl:    "System ran out of memory and the kernel OOM killer terminated processes to free memory. Killed process: %s",
		ImpactTmpl: "Critical services may have been terminated — verify all services are running",
		ActionTmpl: "Check 'dmesg | grep oom-kill' for killed processes; add swap or RAM; configure systemd OOMD",
	},
	"host_ssh_brute_force": {
		Severity: "critical", Diagnosis: "SSH Brute-Force Attack Detected",
		WhatTmpl:   "%d failed SSH authentication attempts in the last 5 minutes from %d unique source IPs",
		WhyTmpl:    "Multiple failed SSH login attempts detected — this is a brute-force or credential-stuffing attack",
		ImpactTmpl: "SSH service under active attack — 0.0f%% of recent auth attempts failed",
		ActionTmpl: "Enable fail2ban immediately: 'sudo fail2ban-client set sshd banip <IP>'; consider disabling password auth",
	},
	"host_service_restarts": {
		Severity: "warning", Diagnosis: "Systemd Service Failures Detected",
		WhatTmpl:   "%d systemd service restart/failure events in the last 5 minutes",
		WhyTmpl:    "Services are crashing or being restarted — affected units: %s. This may indicate application bugs or resource limits",
		ImpactTmpl: "%d service failures — affected services may be intermittently unavailable",
		ActionTmpl: "Check 'systemctl --failed' and 'journalctl -xe'; review service resource limits in unit files",
	},
	"host_swap_usage": {
		Severity: "info", Diagnosis: "Host Swap Usage Elevated",
		WhatTmpl:   "Host swap usage is at %.0f%% (%.0f MB / %.0f MB, threshold: 50%%)",
		WhyTmpl:    "Significant swap usage indicates the system is overcommitted on memory. Swapping degrades performance severely",
		ImpactTmpl: "System performance is degraded due to swap I/O — services may be slower than expected",
		ActionTmpl: "Check memory consumers; consider adding RAM or reducing service memory footprint",
	},
}

type IssueDetectionService struct {
	cfg        *config.Config
	httpClient *http.Client
}

func NewIssueDetectionService(cfg *config.Config) *IssueDetectionService {
	return &IssueDetectionService{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *IssueDetectionService) DetectHostIssues(
	ctx context.Context, tenantID uint, tenantUUID, machineID, hostname string,
	svcs []serviceBrief, timeSec int64, startNs, endNs int64,
) []DiagnosedIssue {
	if timeSec == 0 {
		timeSec = time.Now().Unix()
	}
	if startNs == 0 || endNs == 0 {
		now := time.Now()
		endNs = now.UnixNano()
		startNs = endNs - int64(5*time.Minute)
	}

	var issues []DiagnosedIssue
	nowStr := time.Now().UTC().Format(time.RFC3339)

	for _, svc := range svcs {
		if !supportedServices[svc.Name] {
			continue
		}

		switch svc.Name {
		case "nginx":
			issues = append(issues, s.detectNginxIssues(ctx, tenantID, tenantUUID, machineID, hostname, svc, timeSec, startNs, endNs, nowStr)...)
		case "apache":
			issues = append(issues, s.detectApacheIssues(ctx, tenantID, tenantUUID, machineID, hostname, svc, timeSec, startNs, endNs, nowStr)...)
		case "phpfpm":
			issues = append(issues, s.detectPHPFPMIssues(ctx, tenantID, tenantUUID, machineID, hostname, svc, timeSec, startNs, endNs, nowStr)...)
		case "mariadb":
			issues = append(issues, s.detectMariaDBIssues(ctx, tenantID, tenantUUID, machineID, hostname, svc, timeSec, startNs, endNs, nowStr)...)
		case "postgres":
			issues = append(issues, s.detectPostgresIssues(ctx, tenantID, tenantUUID, machineID, hostname, svc, timeSec, startNs, endNs, nowStr)...)
		case "redis":
			issues = append(issues, s.detectRedisIssues(ctx, tenantID, tenantUUID, machineID, hostname, svc, timeSec, startNs, endNs, nowStr)...)
		}
	}

	// Host-level issues
	hostIssues := s.detectHostIssues(ctx, tenantID, tenantUUID, machineID, hostname, timeSec, startNs, endNs, nowStr)
	issues = append(issues, hostIssues...)

	return issues
}

// ─── Query helpers (same pattern as OverviewService) ──────────────────────────

func (s *IssueDetectionService) queryVM(ctx context.Context, tenantID uint, query string, timeSec int64) *float64 {
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

func (s *IssueDetectionService) queryLogErrCount(ctx context.Context, tenantUUID, machineID, serviceName string, startNs, endNs int64) float64 {
	q := fmt.Sprintf(`count_over_time({service=%q, host_id=%q} |= "error" [5m])`, serviceName, machineID)
	upstream := fmt.Sprintf("http://%s:%s/loki/api/v1/query_range", s.cfg.LokiHost, s.cfg.LokiPort)
	params := url.Values{}
	params.Set("query", q)
	params.Set("start", strconv.FormatInt(startNs, 10))
	params.Set("end", strconv.FormatInt(endNs, 10))
	params.Set("step", "5m")
	params.Set("limit", "1")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream+"?"+params.Encode(), nil)
	if err != nil {
		return 0
	}
	req.Header.Set("X-Scope-OrgID", tenantUUID)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0
	}
	rows, err := parseLokiMatrixResponse(body)
	if err != nil || len(rows) == 0 || len(rows[0].Values) == 0 {
		return 0
	}
	return rows[0].Values[len(rows[0].Values)-1][1]
}

func (s *IssueDetectionService) lokiQueryLogs(ctx context.Context, tenantUUID, query string, startNs, endNs int64, limit int) ([]LokiLogEntry, error) {
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

// ─── Nginx issue detection ────────────────────────────────────────────────────

func (s *IssueDetectionService) detectNginxIssues(
	ctx context.Context, tenantID uint, tenantUUID, machineID, hostname string,
	svc serviceBrief, timeSec int64, startNs, endNs int64, nowStr string,
) []DiagnosedIssue {
	if !svc.MetricColl {
		return nil
	}
	var issues []DiagnosedIssue

	// Current req/s for baseline
	totalRate := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`rate(nginx_http_requests_total{host_id="%s"}[5m])`, machineID), timeSec)
	total5xx := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`rate(nginx_http_requests_total{host_id="%s",status=~"5.."}[5m])`, machineID), timeSec)
	total4xx := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`rate(nginx_http_requests_total{host_id="%s",status=~"4.."}[5m])`, machineID), timeSec)
	total401 := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`rate(nginx_http_requests_total{host_id="%s",status=~"401|403"}[5m])`, machineID), timeSec)

	// 5xx spike
	if total5xx != nil && totalRate != nil && *totalRate > 0 {
		pct := (*total5xx / *totalRate) * 100
		if pct > 5 {
			phpSat := s.queryVM(ctx, tenantID,
				fmt.Sprintf(`phpfpm_active_workers{host_id="%s"} / phpfpm_total_workers{host_id="%s"}`, machineID, machineID), timeSec)
			phpActive := s.queryVM(ctx, tenantID,
				fmt.Sprintf(`phpfpm_active_workers{host_id="%s"}`, machineID), timeSec)
			phpTotal := s.queryVM(ctx, tenantID,
				fmt.Sprintf(`phpfpm_total_workers{host_id="%s"}`, machineID), timeSec)

			// Try to get an error log snippet
			logSnippet := ""
			if svc.LogCollOn {
				entries, _ := s.lokiQueryLogs(ctx, tenantUUID,
					fmt.Sprintf(`{service="nginx", host_id=%q} |~ "5[0-9][0-9]"`, machineID), startNs, endNs, 1)
				if len(entries) > 0 {
					logSnippet = truncateString(entries[0].Line, 150)
				}
			}

			activeW := 0
			totalW := 0
			satPct := 0.0
			if phpSat != nil {
				satPct = *phpSat * 100
			}
			if phpActive != nil {
				activeW = int(*phpActive)
			}
			if phpTotal != nil {
				totalW = int(*phpTotal)
			}

			issues = append(issues, s.buildIssue("nginx_5xx_spike", "nginx", machineID, hostname, nowStr,
				pct, activeW, totalW, satPct, pct, logSnippet))
		}
	}

	// 4xx high
	if total4xx != nil && totalRate != nil && *totalRate > 0 {
		pct := (*total4xx / *totalRate) * 100
		if pct > 15 {
			issues = append(issues, s.buildIssue("nginx_4xx_high", "nginx", machineID, hostname, nowStr,
				pct, 0, 0, 0, pct, ""))
		}
	}

	// Auth brute force (401/403 + attack paths)
	if total401 != nil && *total401 > 10 {
		issues = append(issues, s.buildIssue("nginx_4xx_auth", "nginx", machineID, hostname, nowStr,
			*total401*60, 3, 0, 0, *total401*60, ""))
	}

	// Connections saturation
	connActive := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`nginx_connections_active{host_id="%s"}`, machineID), timeSec)
	connReading := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`nginx_connections_reading{host_id="%s"}`, machineID), timeSec)
	connWriting := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`nginx_connections_writing{host_id="%s"}`, machineID), timeSec)
	connWaiting := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`nginx_connections_waiting{host_id="%s"}`, machineID), timeSec)
	if connActive != nil && *connActive > 2000 {
		issues = append(issues, s.buildIssue("nginx_conn_saturation", "nginx", machineID, hostname, nowStr,
			*connActive, 1, 0, 0, *connActive,
			fmt.Sprintf("reading=%.0f writing=%.0f waiting=%.0f", *connReading, *connWriting, *connWaiting)))
	}

	return issues
}

// ─── Apache issue detection ───────────────────────────────────────────────────

func (s *IssueDetectionService) detectApacheIssues(
	ctx context.Context, tenantID uint, tenantUUID, machineID, hostname string,
	svc serviceBrief, timeSec int64, startNs, endNs int64, nowStr string,
) []DiagnosedIssue {
	if !svc.MetricColl {
		return nil
	}
	var issues []DiagnosedIssue

	totalRate := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`rate(httpd_requests_total{host_id="%s"}[5m])`, machineID), timeSec)
	total5xx := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`rate(httpd_requests_total{host_id="%s",status=~"5.."}[5m])`, machineID), timeSec)
	total4xx := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`rate(httpd_requests_total{host_id="%s",status=~"4.."}[5m])`, machineID), timeSec)
	total401 := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`rate(httpd_requests_total{host_id="%s",status=~"401|403"}[5m])`, machineID), timeSec)

	if total5xx != nil && totalRate != nil && *totalRate > 0 {
		pct := (*total5xx / *totalRate) * 100
		if pct > 5 {
			issues = append(issues, s.buildIssue("apache_5xx_spike", "apache", machineID, hostname, nowStr, pct, 0, 0, 0, pct, ""))
		}
	}
	if total4xx != nil && totalRate != nil && *totalRate > 0 {
		pct := (*total4xx / *totalRate) * 100
		if pct > 15 {
			issues = append(issues, s.buildIssue("apache_4xx_high", "apache", machineID, hostname, nowStr, pct, 0, 0, 0, pct, ""))
		}
	}
	if total401 != nil && *total401 > 10 {
		issues = append(issues, s.buildIssue("apache_4xx_auth", "apache", machineID, hostname, nowStr, *total401*60, 3, 0, 0, *total401*60, ""))
	}

	return issues
}

// ─── PHP-FPM issue detection ──────────────────────────────────────────────────

func (s *IssueDetectionService) detectPHPFPMIssues(
	ctx context.Context, tenantID uint, tenantUUID, machineID, hostname string,
	svc serviceBrief, timeSec int64, startNs, endNs int64, nowStr string,
) []DiagnosedIssue {
	if !svc.MetricColl {
		return nil
	}
	var issues []DiagnosedIssue

	active := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`phpfpm_active_workers{host_id="%s"}`, machineID), timeSec)
	total := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`phpfpm_total_workers{host_id="%s"}`, machineID), timeSec)
	queue := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`phpfpm_listen_queue{host_id="%s"}`, machineID), timeSec)
	maxQueue := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`phpfpm_max_listen_queue{host_id="%s"}`, machineID), timeSec)
	slowReq := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`rate(phpfpm_slow_requests_total{host_id="%s"}[5m])`, machineID), timeSec)
	maxChildren := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`phpfpm_max_children_reached{host_id="%s"}`, machineID), timeSec)
	opcacheHit := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`phpfpm_opcache_hit_rate{host_id="%s"}`, machineID), timeSec)
	opcacheWasted := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`phpfpm_opcache_wasted_ratio{host_id="%s"}`, machineID), timeSec)

	activeW := int(0)
	totalW := int(1)
	if active != nil {
		activeW = int(*active)
	}
	if total != nil {
		totalW = int(*total)
	}
	satPct := float64(activeW) / float64(totalW) * 100

	// Pool exhaustion
	if satPct > 85 {
		queueDepth := 0
		if queue != nil {
			queueDepth = int(*queue)
		}
		issues = append(issues, s.buildIssue("phpfpm_pool_exhaustion", "phpfpm", machineID, hostname, nowStr,
			activeW, totalW, satPct, satPct, float64(queueDepth), totalW))
	}

	// Queue buildup
	if queue != nil && *queue > 0 && maxQueue != nil && *maxQueue > 0 {
		util := (*queue / *maxQueue) * 100
		if util > 50 {
			issues = append(issues, s.buildIssue("phpfpm_queue_buildup", "phpfpm", machineID, hostname, nowStr,
				int(*queue), int(*maxQueue), util, util, 0, ""))
		}
	}

	// Slow requests
	if slowReq != nil && *slowReq > 0.5 {
		slowPerMin := *slowReq * 60
		issues = append(issues, s.buildIssue("phpfpm_slow_requests", "phpfpm", machineID, hostname, nowStr,
			slowPerMin, 0, 0, 0, slowPerMin, ""))
	}

	// Opcache low hit (metric is a ratio, multiply by 100 for percentage)
	if opcacheHit != nil && *opcacheHit*100 < 85 {
		hitPct := *opcacheHit * 100
		issues = append(issues, s.buildIssue("phpfpm_opcache_low_hit", "phpfpm", machineID, hostname, nowStr,
			hitPct, 0, 0, 0, hitPct, ""))
	}

	// Opcache wasted (metric is a ratio, multiply by 100 for percentage)
	if opcacheWasted != nil && *opcacheWasted*100 > 5 {
		wastedPct := *opcacheWasted * 100
		issues = append(issues, s.buildIssue("phpfpm_opcache_wasted", "phpfpm", machineID, hostname, nowStr,
			wastedPct, 0, 0, 0, wastedPct, ""))
	}

	// max_children reached
	if maxChildren != nil && *maxChildren > 0 {
		issues = append(issues, s.buildIssue("phpfpm_max_children_reached", "phpfpm", machineID, hostname, nowStr,
			totalW, totalW, 0, 0, 0, totalW))
	}

	return issues
}

// ─── MariaDB issue detection ──────────────────────────────────────────────────

func (s *IssueDetectionService) detectMariaDBIssues(
	ctx context.Context, tenantID uint, tenantUUID, machineID, hostname string,
	svc serviceBrief, timeSec int64, startNs, endNs int64, nowStr string,
) []DiagnosedIssue {
	if !svc.MetricColl {
		return nil
	}
	var issues []DiagnosedIssue

	hitRatio := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`mysql_global_status_innodb_buffer_pool_hit_ratio{host_id="%s"}`, machineID), timeSec)
	slowQ := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`rate(mysql_global_status_slow_queries{host_id="%s"}[5m])`, machineID), timeSec)
	threads := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`mysql_global_status_threads_connected{host_id="%s"}`, machineID), timeSec)
	maxConn := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`mysql_global_variables_max_connections{host_id="%s"}`, machineID), timeSec)
	deadlocks := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`rate(mysql_global_status_innodb_deadlocks{host_id="%s"}[5m])`, machineID), timeSec)

	// Buffer pool pressure
	if hitRatio != nil && *hitRatio < 0.95 {
		diskReads := s.queryVM(ctx, tenantID,
			fmt.Sprintf(`rate(innodb_buffer_pool_reads{host_id="%s"}[5m])`, machineID), timeSec)
		diskReadV := 0.0
		if diskReads != nil {
			diskReadV = *diskReads
		}
		issues = append(issues, s.buildIssue("mariadb_buffer_pool_pressure", "mariadb", machineID, hostname, nowStr,
			*hitRatio*100, diskReadV, 0, 0, *hitRatio*100, 0))
	}

	// Slow queries
	if slowQ != nil && *slowQ > 0.5 {
		issues = append(issues, s.buildIssue("mariadb_slow_queries", "mariadb", machineID, hostname, nowStr,
			*slowQ, 0, 0, 0, *slowQ*60, 0))
	}

	// Connection saturation
	if threads != nil && maxConn != nil && *maxConn > 0 {
		pct := (*threads / *maxConn) * 100
		if pct > 80 {
			issues = append(issues, s.buildIssue("mariadb_conn_saturation", "mariadb", machineID, hostname, nowStr,
				pct, int(*threads), int(*maxConn), pct, pct, 0))
		}
	}

	// Deadlocks
	if deadlocks != nil && *deadlocks > 0 {
		perMin := *deadlocks * 60
		issues = append(issues, s.buildIssue("mariadb_deadlocks", "mariadb", machineID, hostname, nowStr,
			perMin, 0, 0, 0, perMin, 0))
	}

	return issues
}

// ─── PostgreSQL issue detection ────────────────────────────────────────────────

func (s *IssueDetectionService) detectPostgresIssues(
	ctx context.Context, tenantID uint, tenantUUID, machineID, hostname string,
	svc serviceBrief, timeSec int64, startNs, endNs int64, nowStr string,
) []DiagnosedIssue {
	if !svc.MetricColl {
		return nil
	}
	var issues []DiagnosedIssue

	backends := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`sum(pg_stat_database_numbackends{host_id="%s"})`, machineID), timeSec)
	maxConns := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`pg_settings_max_connections{host_id="%s"}`, machineID), timeSec)
	hitRatio := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`avg(pg_stat_database_blks_hit{host_id="%s"}) / (avg(pg_stat_database_blks_hit{host_id="%s"}) + avg(pg_stat_database_blks_read{host_id="%s"})) * 100`, machineID, machineID, machineID), timeSec)
	deadlocks := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`rate(pg_stat_database_deadlocks_total{host_id="%s"}[5m])`, machineID), timeSec)

	// Connection saturation
	if backends != nil && maxConns != nil && *maxConns > 0 {
		pct := (*backends / *maxConns) * 100
		if pct > 80 {
			issues = append(issues, s.buildIssue("postgres_conn_saturation", "postgres", machineID, hostname, nowStr,
				pct, int(*backends), int(*maxConns), pct, pct, 0))
		}
	}

	// Cache hit low
	if hitRatio != nil && *hitRatio < 95 {
		issues = append(issues, s.buildIssue("postgres_cache_hit_low", "postgres", machineID, hostname, nowStr,
			*hitRatio, 0, 0, 0, *hitRatio, 0))
	}

	// Deadlocks
	if deadlocks != nil && *deadlocks > 0 {
		perMin := *deadlocks * 60
		issues = append(issues, s.buildIssue("postgres_deadlocks", "postgres", machineID, hostname, nowStr,
			perMin, 0, 0, 0, perMin, 0))
	}

	return issues
}

// ─── Redis issue detection ────────────────────────────────────────────────────

func (s *IssueDetectionService) detectRedisIssues(
	ctx context.Context, tenantID uint, tenantUUID, machineID, hostname string,
	svc serviceBrief, timeSec int64, startNs, endNs int64, nowStr string,
) []DiagnosedIssue {
	if !svc.MetricColl {
		return nil
	}
	var issues []DiagnosedIssue

	memUsed := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`redis_memory_used_bytes{host_id="%s"}`, machineID), timeSec)
	memMax := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`redis_memory_max_bytes{host_id="%s"}`, machineID), timeSec)
	hits := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`rate(redis_keyspace_hits_total{host_id="%s"}[5m])`, machineID), timeSec)
	misses := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`rate(redis_keyspace_misses_total{host_id="%s"}[5m])`, machineID), timeSec)
	evictions := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`rate(redis_evicted_keys_total{host_id="%s"}[5m])`, machineID), timeSec)

	// Memory pressure
	if memUsed != nil && memMax != nil && *memMax > 0 {
		pct := (*memUsed / *memMax) * 100
		if pct > 80 {
			issues = append(issues, s.buildIssue("redis_memory_pressure", "redis", machineID, hostname, nowStr,
				pct, *memUsed/1024/1024, *memMax/1024/1024, pct, pct, ""))
		}
	}

	// Cache miss spike
	if hits != nil && misses != nil && (*hits+*misses) > 0 {
		hitPct := (*hits / (*hits + *misses)) * 100
		if hitPct < 90 {
			issues = append(issues, s.buildIssue("redis_cache_miss_spike", "redis", machineID, hostname, nowStr,
				hitPct, *hits, *misses, hitPct, hitPct, ""))
		}
	}

	// Evictions
	if evictions != nil && *evictions > 1 {
		issues = append(issues, s.buildIssue("redis_evictions", "redis", machineID, hostname, nowStr,
			*evictions, 0, 0, 0, *evictions, ""))
	}

	// OOM from logs
	if svc.LogCollOn {
		entries, _ := s.lokiQueryLogs(ctx, tenantUUID,
			fmt.Sprintf(`{service="redis", host_id=%q} |= "OOM command not allowed"`, machineID), startNs, endNs, 1)
		if len(entries) > 0 {
			snippet := truncateString(entries[0].Line, 150)
			issues = append(issues, s.buildIssue("redis_oom", "redis", machineID, hostname, nowStr,
				0, 0, 0, 0, 0, snippet))
		}
	}

	return issues
}

// ─── Host-level issue detection ───────────────────────────────────────────────

func (s *IssueDetectionService) detectHostIssues(
	ctx context.Context, tenantID uint, tenantUUID, machineID, hostname string,
	timeSec int64, startNs, endNs int64, nowStr string,
) []DiagnosedIssue {
	var issues []DiagnosedIssue

	cpu := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`(1 - avg(system_cpu_utilization_ratio{host_id="%s",state="idle"} > 0)) * 100`, machineID), timeSec)
	mem := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`sum(system_memory_usage_bytes{host_id="%s",state="used"}) / sum(system_memory_usage_bytes{host_id="%s",state=~"used|free|buffered|cached"}) * 100`, machineID, machineID), timeSec)
	disk := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`system_filesystem_utilization_ratio{host_id="%s",mountpoint="/"} * 100`, machineID), timeSec)
	swap := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`sum(system_paging_usage_bytes{host_id="%s",state="used"}) / clamp_min(sum(system_paging_usage_bytes{host_id="%s",state=~"used|free"}), 1) * 100`, machineID, machineID), timeSec)

	// CPU saturation
	if cpu != nil && *cpu > 90 {
		load1 := s.queryVM(ctx, tenantID, fmt.Sprintf(`system_cpu_load_average_1m{host_id="%s"}`, machineID), timeSec)
		load5 := s.queryVM(ctx, tenantID, fmt.Sprintf(`system_cpu_load_average_5m{host_id="%s"}`, machineID), timeSec)
		l1, l5 := 0.0, 0.0
		if load1 != nil {
			l1 = *load1
		}
		if load5 != nil {
			l5 = *load5
		}
		issues = append(issues, s.buildIssue("host_cpu_saturation", "host", machineID, hostname, nowStr,
			*cpu, l1, l5, 0, 0, ""))
	}

	// Memory pressure
	if mem != nil && *mem > 90 {
		swapUse := 0.0
		if swap != nil {
			swapUse = *swap
		}
		issues = append(issues, s.buildIssue("host_memory_pressure", "host", machineID, hostname, nowStr,
			*mem, swapUse, 0, 0, 0, ""))
	}

	// Disk space
	if disk != nil && *disk > 85 {
		issues = append(issues, s.buildIssue("host_disk_space", "host", machineID, hostname, nowStr,
			*disk, 0, 0, 0, *disk, ""))
	}

	// Disk I/O latency
	ioLatency := s.queryVM(ctx, tenantID,
		fmt.Sprintf(`rate(system_disk_io_time_seconds_total{host_id="%s"}[5m]) / rate(system_disk_io_completed_total{host_id="%s"}[5m]) * 1000`, machineID, machineID), timeSec)
	if ioLatency != nil && *ioLatency > 100 {
		issues = append(issues, s.buildIssue("host_disk_io", "host", machineID, hostname, nowStr,
			*ioLatency, 0, 0, 0, *ioLatency, ""))
	}

	// Swap usage
	if swap != nil && *swap > 50 {
		issues = append(issues, s.buildIssue("host_swap_usage", "host", machineID, hostname, nowStr,
			*swap, 0, 0, 0, *swap, ""))
	}

	// Log-based issues (OOM, SSH brute force, service restarts)
	selector := fmt.Sprintf(`{service_name="journald"} | host_id=%q`, machineID)

	// OOM killer
	oomQ := fmt.Sprintf(`%s | SYSLOG_IDENTIFIER="kernel" |~ "(?i)Out of memory: Killed process"`, selector)
	oomEntries, _ := s.lokiQueryLogs(ctx, tenantUUID, oomQ, startNs, endNs, 5)
	if len(oomEntries) > 0 {
		killed := truncateString(oomEntries[0].Line, 100)
		issues = append(issues, s.buildIssue("host_oom_killer", "host", machineID, hostname, nowStr,
			len(oomEntries), killed, 0, 0, 0, killed))
	}

	// SSH brute force
	sshQ := fmt.Sprintf(`%s | SYSLOG_IDENTIFIER="sshd" |~ "(?i)Failed password|authentication failure"`, selector)
	sshEntries, _ := s.lokiQueryLogs(ctx, tenantUUID, sshQ, startNs, endNs, 100)
	if len(sshEntries) > 0 {
		uniqueIPs := countUniqueSSHIPs(sshEntries)
		issues = append(issues, s.buildIssue("host_ssh_brute_force", "host", machineID, hostname, nowStr,
			len(sshEntries), uniqueIPs, 0, 0, float64(len(sshEntries)), ""))
	}

	// Service restarts
	restartQ := fmt.Sprintf(`%s |~ "(?i)Failed with result|Main process exited"`, selector)
	restartEntries, _ := s.lokiQueryLogs(ctx, tenantUUID, restartQ, startNs, endNs, 20)
	if len(restartEntries) > 0 {
		units := extractFailedUnits(restartEntries)
		issues = append(issues, s.buildIssue("host_service_restarts", "host", machineID, hostname, nowStr,
			len(restartEntries), strings.Join(units, ", "), 0, 0, 0, ""))
	}

	return issues
}

// ─── Issue builder ─────────────────────────────────────────────────────────────

func (s *IssueDetectionService) buildIssue(ruleID, service, machineID, hostname, nowStr string, args ...interface{}) DiagnosedIssue {
	tmpl, ok := issueTemplates[ruleID]
	if !ok {
		return DiagnosedIssue{
			ID: machineID + ":" + ruleID, MachineID: machineID, Hostname: hostname,
			Service: service, Severity: "info", Diagnosis: ruleID, FiredAt: nowStr, Duration: "just now",
		}
	}

	what := safeSprintf(tmpl.WhatTmpl, extractArgs(args, 0, 5)...)
	why := safeSprintf(tmpl.WhyTmpl, extractArgs(args, 0, 6)...)
	impact := safeSprintf(tmpl.ImpactTmpl, extractArgs(args, 0, 3)...)
	action := safeSprintf(tmpl.ActionTmpl, extractArgs(args, 0, 3)...)

	var evidence []EvidenceItem
	v := s.getEvidenceValue(args, 0)

	switch ruleID {
	case "nginx_5xx_spike":
		evidence = []EvidenceItem{
			{Metric: "5xx rate", Value: fmt.Sprintf("%.1f%%", v), Status: "critical"},
			{Metric: "PHP-FPM saturation", Value: safeSprintf("%.0f%%", extractArgs(args, 3, 1)...), Status: "warning"},
		}
	case "nginx_conn_saturation":
		evidence = []EvidenceItem{
			{Metric: "Active connections", Value: fmt.Sprintf("%.0f", v), Status: "warning"},
		}
	case "phpfpm_pool_exhaustion":
		evidence = []EvidenceItem{
			{Metric: "Active workers", Value: safeSprintf("%d/%d", extractArgs(args, 0, 2)...), Status: "critical"},
			{Metric: "Saturation", Value: fmt.Sprintf("%.0f%%", v), Status: "critical"},
		}
	case "phpfpm_queue_buildup":
		evidence = []EvidenceItem{
			{Metric: "Queue depth", Value: fmt.Sprintf("%.0f", v), Status: "warning"},
		}
	case "phpfpm_slow_requests":
		evidence = []EvidenceItem{
			{Metric: "Slow requests/min", Value: fmt.Sprintf("%.1f", v), Status: "warning"},
		}
	case "mariadb_buffer_pool_pressure":
		evidence = []EvidenceItem{
			{Metric: "Hit ratio", Value: fmt.Sprintf("%.1f%%", v), Status: "warning"},
			{Metric: "Disk reads", Value: safeSprintf("%.0f/s", extractArgs(args, 1, 1)...), Status: "info"},
		}
	case "mariadb_slow_queries":
		evidence = []EvidenceItem{
			{Metric: "Slow queries/s", Value: fmt.Sprintf("%.1f", v), Status: "warning"},
		}
	case "mariadb_deadlocks", "postgres_deadlocks":
		evidence = []EvidenceItem{
			{Metric: "Deadlocks/min", Value: fmt.Sprintf("%.1f", v), Status: "critical"},
		}
	case "redis_memory_pressure":
		evidence = []EvidenceItem{
			{Metric: "Memory usage", Value: fmt.Sprintf("%.0f%%", v), Status: "warning"},
		}
	case "redis_cache_miss_spike":
		evidence = []EvidenceItem{
			{Metric: "Hit ratio", Value: fmt.Sprintf("%.1f%%", v), Status: "warning"},
		}
	case "phpfpm_opcache_low_hit":
		evidence = []EvidenceItem{
			{Metric: "Hit rate", Value: fmt.Sprintf("%.1f%%", v), Status: "warning"},
		}
	case "phpfpm_opcache_wasted":
		evidence = []EvidenceItem{
			{Metric: "Wasted ratio", Value: fmt.Sprintf("%.1f%%", v), Status: "warning"},
		}
	case "redis_evictions":
		evidence = []EvidenceItem{
			{Metric: "Evictions/s", Value: fmt.Sprintf("%.1f", v), Status: "warning"},
		}
	case "host_cpu_saturation":
		evidence = []EvidenceItem{
			{Metric: "CPU usage", Value: fmt.Sprintf("%.0f%%", v), Status: "warning"},
			{Metric: "Load 1m", Value: safeSprintf("%.1f", extractArgs(args, 1, 1)...), Status: "info"},
		}
	case "host_memory_pressure":
		evidence = []EvidenceItem{
			{Metric: "Memory", Value: fmt.Sprintf("%.0f%%", v), Status: "warning"},
			{Metric: "Swap", Value: safeSprintf("%.0f%%", extractArgs(args, 1, 1)...), Status: "info"},
		}
	case "host_disk_space":
		evidence = []EvidenceItem{
			{Metric: "Disk usage", Value: fmt.Sprintf("%.0f%%", v), Status: "warning"},
		}
	case "host_oom_killer":
		evidence = []EvidenceItem{
			{Metric: "OOM events", Value: fmt.Sprintf("%.0f", v), Status: "critical"},
		}
	case "host_ssh_brute_force":
		evidence = []EvidenceItem{
			{Metric: "Failed attempts", Value: fmt.Sprintf("%.0f", v), Status: "critical"},
			{Metric: "Unique IPs", Value: safeSprintf("%.0f", extractArgs(args, 1, 1)...), Status: "warning"},
		}
	case "host_service_restarts":
		evidence = []EvidenceItem{
			{Metric: "Service failures", Value: fmt.Sprintf("%.0f", v), Status: "warning"},
		}
	default:
		evidence = []EvidenceItem{
			{Metric: "Value", Value: fmt.Sprintf("%.1f", v), Status: "warning"},
		}
	}

	return DiagnosedIssue{
		ID:         fmt.Sprintf("%s:%s:%s", machineID, service, ruleID),
		MachineID:  machineID,
		Hostname:   hostname,
		Service:    service,
		Severity:   tmpl.Severity,
		Diagnosis:  tmpl.Diagnosis,
		What:       what,
		Why:        why,
		Impact:     impact,
		Action:     action,
		Evidence:   evidence,
		LogSnippet: s.getLogSnippet(args),
		FiredAt:    nowStr,
		Duration:   "just now",
	}
}

func (s *IssueDetectionService) getEvidenceValue(args []interface{}, idx int) float64 {
	if idx >= len(args) {
		return 0
	}
	switch v := args[idx].(type) {
	case int:
		return float64(v)
	case float64:
		return v
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	return 0
}

func (s *IssueDetectionService) getLogSnippet(args []interface{}) string {
	for _, a := range args {
		if str, ok := a.(string); ok && len(str) > 10 {
			return str
		}
	}
	return ""
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func safeSprintf(format string, args ...interface{}) string {
	if len(args) == 0 {
		return format
	}
	r := strings.NewReplacer("%d", "%v", "%s", "%v", "%f", "%v")
	safeFmt := r.Replace(format)
	n := countFormatSpecifiers(safeFmt)
	if n < len(args) {
		args = args[:n]
	}
	return fmt.Sprintf(safeFmt, args...)
}

func countFormatSpecifiers(fmt string) int {
	n := 0
	for i := 0; i < len(fmt); i++ {
		if fmt[i] == '%' {
			if i+1 < len(fmt) && fmt[i+1] == '%' {
				i++
				continue
			}
			n++
		}
	}
	return n
}

func extractArgs(args []interface{}, start, count int) []interface{} {
	if start >= len(args) {
		return []interface{}{}
	}
	end := start + count
	if end > len(args) {
		end = len(args)
	}
	return args[start:end]
}

func countUniqueSSHIPs(entries []LokiLogEntry) int {
	seen := map[string]bool{}
	for _, e := range entries {
		// Extract IP from log line: "Failed password for root from 10.0.0.1"
		parts := strings.Fields(e.Line)
		for i, p := range parts {
			if p == "from" && i+1 < len(parts) {
				seen[parts[i+1]] = true
			}
		}
	}
	return len(seen)
}

func extractFailedUnits(entries []LokiLogEntry) []string {
	seen := map[string]bool{}
	var units []string
	for _, e := range entries {
		for _, part := range strings.Fields(e.Line) {
			if strings.Contains(part, ".service") && !seen[part] {
				seen[part] = true
				units = append(units, part)
			}
		}
	}
	return units
}
