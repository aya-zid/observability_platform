package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
)

type QueryType string

const (
	QueryTypePromQL  QueryType = "promql"
	QueryTypeLogQL   QueryType = "logql"
	QueryTypeStatus  QueryType = "status"
)

type BuiltQuery struct {
	Query     string
	QueryType QueryType
}

type vmInstantResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string          `json:"resultType"`
		Result     []vmInstantItem `json:"result"`
	} `json:"data"`
}

type vmInstantItem struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

type lokiInstantResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string            `json:"resultType"`
		Result     []lokiInstantItem `json:"result"`
	} `json:"data"`
}

type lokiInstantItem struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

type AlertQueryBuilder struct {
	cfg        *config.Config
	httpClient *http.Client
}

func NewAlertQueryBuilder(cfg *config.Config) *AlertQueryBuilder {
	return &AlertQueryBuilder{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (b *AlertQueryBuilder) BuildQuery(templateID, hostMachineID, serviceName, domain, pool, containerName, forDuration string) (*BuiltQuery, error) {
	if forDuration == "" {
		forDuration = "5m"
	}

	switch templateID {
	// ── Host templates (PromQL) ────────────────────────────────
	case "host_cpu":
		return b.promQL(fmt.Sprintf(
			`(1 - avg(system_cpu_utilization_ratio{host_id=%q,state="idle"})) * 100`,
			hostMachineID,
		)), nil

	case "host_memory":
		return b.promQL(fmt.Sprintf(
			`sum(system_memory_usage_bytes{host_id=%q,state="used"}) / clamp_min(sum(system_memory_usage_bytes{host_id=%q,state=~"used|free|buffered|cached"}), 1) * 100`,
			hostMachineID, hostMachineID,
		)), nil

	case "host_disk":
		return b.promQL(fmt.Sprintf(
			`max(system_filesystem_utilization_ratio{host_id=%q,mountpoint!~"/boot.*|/run.*|/dev.*"}) * 100`,
			hostMachineID,
		)), nil

	case "host_swap":
		return b.promQL(fmt.Sprintf(
			`sum(system_paging_usage_bytes{host_id=%q,state="used"}) / clamp_min(sum(system_paging_usage_bytes{host_id=%q,state=~"used|free"}), 1) * 100`,
			hostMachineID, hostMachineID,
		)), nil

	case "host_load1":
		return b.promQL(fmt.Sprintf(
			`system_cpu_load_average_1m{host_id=%q}`,
			hostMachineID,
		)), nil

	case "host_disk_io_latency":
		return b.promQL(fmt.Sprintf(
			`rate(system_disk_io_time_seconds_total{host_id=%q}[%s]) / clamp_min(rate(system_disk_io_completed_total{host_id=%q}[%s]), 1) * 1000`,
			hostMachineID, forDuration, hostMachineID, forDuration,
		)), nil

	case "host_conntrack":
		return b.promQL(fmt.Sprintf(
			`system_network_conntrack_count{host_id=%q} / clamp_min(system_network_conntrack_max{host_id=%q}, 1) * 100`,
			hostMachineID, hostMachineID,
		)), nil

	case "host_network_errors":
		return b.promQL(fmt.Sprintf(
			`rate(system_network_errors_total{host_id=%q}[%s])`,
			hostMachineID, forDuration,
		)), nil

	// ── Host templates (LogQL from journald) ──────────────────
	case "host_ssh_brute_force":
		return b.logQL(fmt.Sprintf(
			`count_over_time({service_name="journald"} | host_id=%q | SYSLOG_IDENTIFIER="sshd" |~ "(?i)Failed password|authentication failure" [%s])`,
			hostMachineID, forDuration,
		)), nil

	case "host_oom_kill":
		return b.logQL(fmt.Sprintf(
			`count_over_time({service_name="journald"} | host_id=%q | SYSLOG_IDENTIFIER="kernel" |~ "(?i)Out of memory: Killed process" [%s])`,
			hostMachineID, forDuration,
		)), nil

	case "host_service_crash_loop":
		return b.logQL(fmt.Sprintf(
			`count_over_time({service_name="journald"} | host_id=%q |~ "(?i)Failed with result|Main process exited" [%s])`,
			hostMachineID, forDuration,
		)), nil

	case "host_disk_error":
		return b.logQL(fmt.Sprintf(
			`count_over_time({service_name="journald"} | host_id=%q |~ "(?i)I/O error|disk error|Buffer I/O error" [%s])`,
			hostMachineID, forDuration,
		)), nil

	// ── Nginx templates (LogQL + PromQL) ──────────────────────
	case "nginx_5xx":
		return b.buildNginx5xx(hostMachineID, serviceName, domain, forDuration), nil

	case "nginx_p99":
		return b.buildNginxP99(hostMachineID, serviceName, domain, forDuration), nil

	case "nginx_4xx":
		return b.buildNginx4xx(hostMachineID, serviceName, domain, forDuration), nil

	case "nginx_conn_saturation":
		return b.promQL(fmt.Sprintf(
			`nginx_connections_current{state="active",host_id=%q}`,
			hostMachineID,
		)), nil

	case "nginx_traffic_drop":
		return b.buildNginxTrafficDrop(hostMachineID, serviceName, domain, forDuration), nil

	case "nginx_auth_brute_force":
		return b.buildNginxAuthBruteForce(hostMachineID, serviceName, domain, forDuration), nil

	// ── Apache templates (LogQL + PromQL) ────────────────────
	case "apache_auth_brute_force":
		return b.buildApacheAuthBruteForce(hostMachineID, serviceName, domain, forDuration), nil

	case "apache_4xx":
		return b.buildApache4xx(hostMachineID, serviceName, domain, forDuration), nil

	case "apache_5xx":
		return b.buildApache5xx(hostMachineID, serviceName, domain, forDuration), nil

	case "apache_p99":
		return b.buildApacheP99(hostMachineID, serviceName, domain, forDuration), nil

	case "apache_workers_saturation":
		return b.promQL(fmt.Sprintf(
			`apache_workers{state="busy",host_id=%q} / clamp_min(sum(apache_workers{host_id=%q}), 1) * 100`,
			hostMachineID, hostMachineID,
		)), nil

	case "apache_traffic_drop":
		return b.buildApacheTrafficDrop(hostMachineID, serviceName, domain, forDuration), nil

	// ── PostgreSQL templates (PromQL + LogQL) ─────────────────
	case "postgres_connections":
		return b.promQL(fmt.Sprintf(
			`100 * sum(pg_stat_database_numbackends{host_id=%q}) / clamp_min(max(pg_settings_max_connections{host_id=%q}), 1)`,
			hostMachineID, hostMachineID,
		)), nil

	case "postgres_deadlocks":
		return b.promQL(fmt.Sprintf(
			`sum(rate(pg_stat_database_deadlocks_total{host_id=%q}[%s])) * 60`,
			hostMachineID, forDuration,
		)), nil

	case "postgres_cache_hit":
		return b.promQL(fmt.Sprintf(
			`100 * sum(rate(pg_stat_database_blks_hit_total{host_id=%q}[%s])) / clamp_min(sum(rate(pg_stat_database_blks_hit_total{host_id=%q}[%s]) + rate(pg_stat_database_blks_read_total{host_id=%q}[%s])), 1)`,
			hostMachineID, forDuration, hostMachineID, forDuration, hostMachineID, forDuration,
		)), nil

	case "postgres_replication_lag":
		return b.promQL(fmt.Sprintf(
			`pg_replication_lag_seconds{host_id=%q}`,
			hostMachineID,
		)), nil

	case "postgres_long_transaction":
		return b.promQL(fmt.Sprintf(
			`max(pg_stat_activity_max_tx_duration{host_id=%q,state="active"})`,
			hostMachineID,
		)), nil

	case "postgres_temp_files":
		return b.promQL(fmt.Sprintf(
			`sum(rate(pg_stat_database_temp_files_total{host_id=%q}[%s]))`,
			hostMachineID, forDuration,
		)), nil

	case "postgres_archive_failures":
		return b.promQL(fmt.Sprintf(
			`sum(rate(pg_stat_archiver_failed_count_total{host_id=%q}[%s]))`,
			hostMachineID, forDuration,
		)), nil

	case "postgres_conflicts":
		return b.promQL(fmt.Sprintf(
			`sum(rate(pg_stat_database_conflicts_total{host_id=%q}[%s]))`,
			hostMachineID, forDuration,
		)), nil

	case "postgres_auth_failures":
		return b.logQL(fmt.Sprintf(
			`count_over_time({service_name="journald"} | host_id=%q |~ "(?i)password authentication failed" [%s])`,
			hostMachineID, forDuration,
		)), nil

	case "postgres_lock_waits":
		return b.logQL(fmt.Sprintf(
			`count_over_time({service_name="journald"} | host_id=%q |~ "(?i)still waiting for" [%s])`,
			hostMachineID, forDuration,
		)), nil

	// ── MariaDB templates (PromQL + LogQL) ───────────────────
	case "mariadb_slow_queries":
		return b.promQL(fmt.Sprintf(
			`rate(mysql_global_status_slow_queries{host_id=%q}[%s]) * 60`,
			hostMachineID, forDuration,
		)), nil

	case "mariadb_connections":
		return b.promQL(fmt.Sprintf(
			`100 * mysql_global_status_threads_connected{host_id=%q} / clamp_min(mysql_global_variables_max_connections{host_id=%q}, 1)`,
			hostMachineID, hostMachineID,
		)), nil

	case "mariadb_buffer_pool_hit":
		return b.promQL(fmt.Sprintf(
			`100 * (1 - (rate(mysql_global_status_innodb_buffer_pool_reads{host_id=%q}[%s]) / clamp_min(rate(mysql_global_status_innodb_buffer_pool_read_requests{host_id=%q}[%s]), 1)))`,
			hostMachineID, forDuration, hostMachineID, forDuration,
		)), nil

	case "mariadb_deadlocks":
		return b.promQL(fmt.Sprintf(
			`rate(mysql_global_status_innodb_deadlocks{host_id=%q}[%s]) * 60`,
			hostMachineID, forDuration,
		)), nil

	case "mariadb_replication_lag":
		return b.promQL(fmt.Sprintf(
			`mysql_slave_lag_seconds{host_id=%q} or mysql_slave_status_seconds_behind_master{host_id=%q}`,
			hostMachineID, hostMachineID,
		)), nil

	case "mariadb_aborted_connects":
		return b.promQL(fmt.Sprintf(
			`rate(mysql_global_status_aborted_connects{host_id=%q}[%s]) * 60`,
			hostMachineID, forDuration,
		)), nil

	case "mariadb_tmp_disk_tables":
		return b.promQL(fmt.Sprintf(
			`rate(mysql_global_status_created_tmp_disk_tables{host_id=%q}[%s])`,
			hostMachineID, forDuration,
		)), nil

	case "mariadb_row_lock_wait":
		return b.promQL(fmt.Sprintf(
			`rate(mysql_global_status_innodb_row_lock_waits{host_id=%q}[%s]) * 60`,
			hostMachineID, forDuration,
		)), nil

	case "mariadb_auth_failures":
		return b.logQL(fmt.Sprintf(
			`count_over_time({service_name="journald"} | host_id=%q |~ "(?i)Access denied for user" [%s])`,
			hostMachineID, forDuration,
		)), nil

	case "mariadb_history_list_length":
		return b.promQL(fmt.Sprintf(
			`mysql_global_status_innodb_history_list_length{host_id=%q}`,
			hostMachineID,
		)), nil

	// ── Redis templates (PromQL + LogQL) ─────────────────────
	case "redis_memory":
		return b.promQL(fmt.Sprintf(
			`redis_memory_used_bytes{host_id=%q} / clamp_min(redis_memory_max_bytes{host_id=%q}, 1) * 100`,
			hostMachineID, hostMachineID,
		)), nil

	case "redis_evictions":
		return b.promQL(fmt.Sprintf(
			`rate(redis_evicted_keys_total{host_id=%q}[%s])`,
			hostMachineID, forDuration,
		)), nil

	case "redis_cache_hit":
		return b.promQL(fmt.Sprintf(
			`100 * rate(redis_keyspace_hits_total{host_id=%q}[%s]) / clamp_min(rate(redis_keyspace_hits_total{host_id=%q}[%s]) + rate(redis_keyspace_misses_total{host_id=%q}[%s]), 1)`,
			hostMachineID, forDuration, hostMachineID, forDuration, hostMachineID, forDuration,
		)), nil

	case "redis_blocked_clients":
		return b.promQL(fmt.Sprintf(
			`redis_blocked_clients{host_id=%q}`,
			hostMachineID,
		)), nil

	case "redis_rejected_connections":
		return b.promQL(fmt.Sprintf(
			`rate(redis_rejected_connections_total{host_id=%q}[%s])`,
			hostMachineID, forDuration,
		)), nil

	case "redis_error_replies":
		return b.promQL(fmt.Sprintf(
			`rate(redis_error_replies_total{host_id=%q}[%s])`,
			hostMachineID, forDuration,
		)), nil

	case "redis_fragmentation":
		return b.promQL(fmt.Sprintf(
			`redis_mem_fragmentation_ratio{host_id=%q}`,
			hostMachineID,
		)), nil

	case "redis_slave_disconnect":
		return b.promQL(fmt.Sprintf(
			`redis_connected_slaves{host_id=%q}`,
			hostMachineID,
		)), nil

	case "redis_auth_or_security":
		return b.logQL(fmt.Sprintf(
			`count_over_time({service_name="journald"} | host_id=%q |~ "(?i)AUTH failed|SECURITY ATTACK" [%s])`,
			hostMachineID, forDuration,
		)), nil

	// ── PHP-FPM templates (PromQL) ───────────────────────────
	case "phpfpm_processes":
		return b.buildPHPFPMProcesses(hostMachineID, pool), nil

	case "phpfpm_queue_saturation":
		return b.buildPHPFPMQueueSaturation(hostMachineID, pool), nil

	case "phpfpm_slow_requests":
		return b.buildPHPFPMSlowRequests(hostMachineID, pool, forDuration), nil

	case "phpfpm_max_children_reached":
		return b.buildPHPFPMMaxChildrenReached(hostMachineID, pool, forDuration), nil

	case "phpfpm_opcache_hit_low":
		return b.buildPHPFPMOpcacheHitLow(hostMachineID), nil

	case "phpfpm_opcache_wasted":
		return b.buildPHPFPMOpcacheWasted(hostMachineID), nil

	case "phpfpm_error_rate":
		return b.buildPHPFPMErrorRate(hostMachineID, serviceName, forDuration), nil

	// ── Status-based templates (DB queries, not PromQL/LogQL) ───
	case "container_stopped", "host_service_stopped":
		return &BuiltQuery{QueryType: QueryTypeStatus}, nil

	// ── Container templates (PromQL) ────────────────────────────
	case "container_cpu":
		return b.buildContainerCPU(containerName), nil

	case "container_memory":
		return b.buildContainerMemory(containerName), nil

	default:
		return nil, fmt.Errorf("alert_query_builder: unknown template_id %q", templateID)
	}
}

func (b *AlertQueryBuilder) promQL(query string) *BuiltQuery {
	return &BuiltQuery{Query: query, QueryType: QueryTypePromQL}
}

func (b *AlertQueryBuilder) logQL(query string) *BuiltQuery {
	return &BuiltQuery{Query: query, QueryType: QueryTypeLogQL}
}

// ── Nginx/ Apache LogQL builders ──────────────────────────────────

const nginxParseExpr = `| regexp ` + "`" + `^(?P<ip>[^ ]+) [^ ]+ [^ ]+ \[[^\]]+\] "(?P<method>[A-Z]+) (?P<path>[^ ]+) [^"]*" (?P<status>\d{3}) (?P<bytes>[\d-]+)` + "`"

const nginxRTParser = `| regexp ` + "`" + `rt=(?P<rt>[0-9.]+)$` + "`"

func lokiWebSelector(serviceName, domain, logKind, hostID string) string {
	selector := fmt.Sprintf(`{service_name=%q}`, lokiLabelValue(serviceName))
	selector += fmt.Sprintf(` | log_kind=%q`, lokiLabelValue(logKind))
	if hostID != "" {
		selector += fmt.Sprintf(` | host_id=%q`, lokiLabelValue(hostID))
	}
	if domain = strings.TrimSpace(domain); domain != "" {
		selector += fmt.Sprintf(` | vhost=%q`, lokiLabelValue(domain))
	}
	return selector
}

func (b *AlertQueryBuilder) buildNginx5xx(hostID, serviceName, domain, forDuration string) *BuiltQuery {
	base := lokiWebSelector(serviceName, domain, "access", hostID)
	query := fmt.Sprintf(
		`sum(rate(%s %s | status =~ "5.." [%s])) / sum(rate(%s [%s])) * 100`,
		base, nginxParseExpr, forDuration, base, forDuration,
	)
	return b.logQL(query)
}

func (b *AlertQueryBuilder) buildNginx4xx(hostID, serviceName, domain, forDuration string) *BuiltQuery {
	base := lokiWebSelector(serviceName, domain, "access", hostID)
	query := fmt.Sprintf(
		`sum(rate(%s %s | status =~ "4.." [%s])) / sum(rate(%s [%s])) * 100`,
		base, nginxParseExpr, forDuration, base, forDuration,
	)
	return b.logQL(query)
}

func (b *AlertQueryBuilder) buildNginxP99(hostID, serviceName, domain, forDuration string) *BuiltQuery {
	base := lokiWebSelector(serviceName, domain, "access", hostID)
	query := fmt.Sprintf(
		`quantile_over_time(0.99, %s %s | unwrap rt [%s]) by ()`,
		base, nginxRTParser, forDuration,
	)
	return b.logQL(query)
}

func (b *AlertQueryBuilder) buildApache5xx(hostID, serviceName, domain, forDuration string) *BuiltQuery {
	return b.buildNginx5xx(hostID, serviceName, domain, forDuration)
}

func (b *AlertQueryBuilder) buildApacheP99(hostID, serviceName, domain, forDuration string) *BuiltQuery {
	return b.buildNginxP99(hostID, serviceName, domain, forDuration)
}

func (b *AlertQueryBuilder) buildApache4xx(hostID, serviceName, domain, forDuration string) *BuiltQuery {
	return b.buildNginx4xx(hostID, serviceName, domain, forDuration)
}

func (b *AlertQueryBuilder) buildApacheAuthBruteForce(hostID, serviceName, domain, forDuration string) *BuiltQuery {
	return b.buildNginxAuthBruteForce(hostID, serviceName, domain, forDuration)
}

func (b *AlertQueryBuilder) buildApacheTrafficDrop(hostID, serviceName, domain, forDuration string) *BuiltQuery {
	return b.buildNginxTrafficDrop(hostID, serviceName, domain, forDuration)
}

// buildNginxTrafficDrop compares current request rate (5m) against historical rate (1h).
// A negative percentage means traffic dropped — fires when drop exceeds threshold (e.g. -50%).
func (b *AlertQueryBuilder) buildNginxTrafficDrop(hostID, serviceName, domain, forDuration string) *BuiltQuery {
	const longWindow = "1h"
	base := lokiWebSelector(serviceName, domain, "access", hostID)
	shortRate := fmt.Sprintf(`sum(rate(%s %s [%s]))`, base, parseExpr, forDuration)
	longRate := fmt.Sprintf(`sum(rate(%s %s [%s]))`, base, parseExpr, longWindow)
	query := fmt.Sprintf(
		`(clamp_min(%s, 0) - %s) / clamp_min(%s, 1) * 100`,
		shortRate, longRate, longRate,
	)
	return b.logQL(query)
}

// buildNginxAuthBruteForce counts 401/403 responses — any elevated 401/403 rate
// indicates potential brute-force or credential-stuffing attempts.
func (b *AlertQueryBuilder) buildNginxAuthBruteForce(hostID, serviceName, domain, forDuration string) *BuiltQuery {
	base := lokiWebSelector(serviceName, domain, "access", hostID)
	query := fmt.Sprintf(
		`sum(count_over_time(%s %s | status =~ "401|403" [%s]))`,
		base, parseExpr, forDuration,
	)
	return b.logQL(query)
}

// ── PHP-FPM PromQL builders ──────────────────────────────────────

func poolFilter(hostMachineID, pool string) string {
	if pool == "" {
		return fmt.Sprintf(`{host_id=%q}`, hostMachineID)
	}
	return fmt.Sprintf(`{host_id=%q,pool=%q}`, hostMachineID, pool)
}

func (b *AlertQueryBuilder) buildPHPFPMProcesses(hostMachineID, pool string) *BuiltQuery {
	pf := poolFilter(hostMachineID, pool)
	return b.promQL(fmt.Sprintf(
		`phpfpm_active_processes%s / clamp_min(phpfpm_total_processes%s, 1) * 100`,
		pf, pf,
	))
}

func (b *AlertQueryBuilder) buildPHPFPMQueueSaturation(hostMachineID, pool string) *BuiltQuery {
	pf := poolFilter(hostMachineID, pool)
	return b.promQL(fmt.Sprintf(
		`phpfpm_listen_queue%s / clamp_min(phpfpm_max_listen_queue_total%s, 1) * 100`,
		pf, pf,
	))
}

func (b *AlertQueryBuilder) buildPHPFPMSlowRequests(hostMachineID, pool, forDuration string) *BuiltQuery {
	pf := poolFilter(hostMachineID, pool)
	return b.promQL(fmt.Sprintf(
		`rate(phpfpm_slow_requests_total%s[%s]) * 60`,
		pf, forDuration,
	))
}

func (b *AlertQueryBuilder) buildPHPFPMMaxChildrenReached(hostMachineID, pool, forDuration string) *BuiltQuery {
	pf := poolFilter(hostMachineID, pool)
	return b.promQL(fmt.Sprintf(
		`rate(phpfpm_max_children_reached_total%s[%s])`,
		pf, forDuration,
	))
}

func (b *AlertQueryBuilder) buildPHPFPMOpcacheHitLow(hostMachineID string) *BuiltQuery {
	return b.promQL(fmt.Sprintf(
		`phpfpm_opcache_hit_rate{host_id=%q} * 100`,
		hostMachineID,
	))
}

func (b *AlertQueryBuilder) buildPHPFPMOpcacheWasted(hostMachineID string) *BuiltQuery {
	return b.promQL(fmt.Sprintf(
		`phpfpm_opcache_wasted_ratio{host_id=%q} * 100`,
		hostMachineID,
	))
}

func (b *AlertQueryBuilder) buildPHPFPMErrorRate(hostMachineID, serviceName, forDuration string) *BuiltQuery {
	base := fmt.Sprintf(`{service_name=%q}`, lokiLabelValue(serviceName))
	if hostMachineID != "" {
		base += fmt.Sprintf(` | host_id=%q`, lokiLabelValue(hostMachineID))
	}
	query := fmt.Sprintf(
		`count_over_time(%s | log_file_name="error.log" |~ "(?i)ERROR|ALERT|CRITICAL" [%s])`,
		base, forDuration,
	)
	return b.logQL(query)
}

// DiscoverPHPFPMPools queries VictoriaMetrics for distinct PHP-FPM pool label values.
func (b *AlertQueryBuilder) DiscoverPHPFPMPools(ctx context.Context, tenantID uint, hostID string) ([]string, error) {
	upstream := fmt.Sprintf("http://%s:%s/select/%d/prometheus/api/v1/query",
		b.cfg.VMSelectHost, b.cfg.VMSelectPort, tenantID)

	query := fmt.Sprintf(`phpfpm_total_processes{host_id=%q}`, hostID)
	params := url.Values{}
	params.Set("query", query)
	params.Set("time", strconv.FormatInt(time.Now().Unix(), 10))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("discover pools req: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discover pools do: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("discover pools read: %w", err)
	}

	var result vmInstantResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("discover pools parse: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("discover pools status not success: %s", string(body))
	}

	seen := map[string]struct{}{}
	for _, item := range result.Data.Result {
		if pool, ok := item.Metric["pool"]; ok && pool != "" {
			seen[pool] = struct{}{}
		}
	}
	pools := make([]string, 0, len(seen))
	for p := range seen {
		pools = append(pools, p)
	}
	if len(pools) == 0 {
		return nil, nil
	}
	return pools, nil
}

// ── Container PromQL builders ────────────────────────────────────

func (b *AlertQueryBuilder) buildContainerCPU(containerName string) *BuiltQuery {
	return b.promQL(fmt.Sprintf(`%s_cpu_percent`, containerSafeName(containerName)))
}

func (b *AlertQueryBuilder) buildContainerMemory(containerName string) *BuiltQuery {
	return b.promQL(fmt.Sprintf(`%s_mem_percent`, containerSafeName(containerName)))
}

// ── Query execution ──────────────────────────────────────────────

func (b *AlertQueryBuilder) ExecutePromQL(ctx context.Context, tenantID uint, query string) (*float64, error) {
	upstream := fmt.Sprintf("http://%s:%s/select/%d/prometheus/api/v1/query",
		b.cfg.VMSelectHost, b.cfg.VMSelectPort, tenantID)

	params := url.Values{}
	params.Set("query", query)
	params.Set("time", strconv.FormatInt(time.Now().Unix(), 10))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("promql req: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("promql do: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("promql read: %w", err)
	}

	var result vmInstantResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("promql parse: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("promql status not success: %s", string(body))
	}

	if len(result.Data.Result) == 0 {
		return nil, nil
	}

	val, err := extractValue(result.Data.Result[0].Value)
	if err != nil {
		return nil, fmt.Errorf("promql extract: %w", err)
	}
	return &val, nil
}

func (b *AlertQueryBuilder) ExecuteLogQL(ctx context.Context, tenantUUID, query string) (*float64, error) {
	upstream := fmt.Sprintf("http://%s:%s/loki/api/v1/query",
		b.cfg.LokiHost, b.cfg.LokiPort)

	params := url.Values{}
	params.Set("query", query)
	params.Set("time", strconv.FormatInt(time.Now().Unix(), 10))
	params.Set("limit", "1")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("logql req: %w", err)
	}
	req.Header.Set("X-Scope-OrgID", tenantUUID)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("logql do: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("logql read: %w", err)
	}

	var result lokiInstantResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("logql parse: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("logql status not success: %s", string(body))
	}

	if len(result.Data.Result) == 0 {
		return nil, nil
	}

	val, err := extractValue(result.Data.Result[0].Value)
	if err != nil {
		return nil, fmt.Errorf("logql extract: %w", err)
	}
	return &val, nil
}

// extractValue parses the value from a VM or Loki instant result.
// The value is [unixTimestamp, "valueString"].
func extractValue(val []interface{}) (float64, error) {
	if len(val) < 2 {
		return 0, fmt.Errorf("unexpected value length: %d", len(val))
	}
	str, ok := val[1].(string)
	if !ok {
		return 0, fmt.Errorf("value is not string: %v", val[1])
	}
	f, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0, fmt.Errorf("parse float: %w", err)
	}
	return f, nil
}
