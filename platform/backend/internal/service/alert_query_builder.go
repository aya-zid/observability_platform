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
	QueryTypePromQL QueryType = "promql"
	QueryTypeLogQL  QueryType = "logql"
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

func (b *AlertQueryBuilder) BuildQuery(templateID, hostMachineID, serviceName, domain, forDuration string) (*BuiltQuery, error) {
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

	// ── Nginx templates (LogQL) ───────────────────────────────
	case "nginx_5xx":
		return b.buildNginx5xx(hostMachineID, serviceName, domain, forDuration), nil

	case "nginx_p99":
		return b.buildNginxP99(hostMachineID, serviceName, domain, forDuration), nil

	// ── Apache templates (LogQL) ─────────────────────────────
	case "apache_5xx":
		return b.buildApache5xx(hostMachineID, serviceName, domain, forDuration), nil

	case "apache_p99":
		return b.buildApacheP99(hostMachineID, serviceName, domain, forDuration), nil

	// ── PostgreSQL templates (PromQL) ─────────────────────────
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

	// ── MariaDB templates (PromQL) ───────────────────────────
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

	// ── Redis templates (PromQL) ─────────────────────────────
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

	// ── PHP-FPM templates (PromQL) ───────────────────────────
	case "phpfpm_processes":
		return b.promQL(fmt.Sprintf(
			`phpfpm_active_processes{host_id=%q} / clamp_min(phpfpm_total_processes{host_id=%q}, 1) * 100`,
			hostMachineID, hostMachineID,
		)), nil

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
		`sum(rate(%s %s | status =~ "5.." [%s])) / clamp_min(sum(rate(%s [%s])), 1) * 100`,
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
	params.Set("time", strconv.FormatInt(time.Now().UnixNano(), 10))
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
