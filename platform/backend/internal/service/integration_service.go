package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/catalog"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

var (
	ErrTenantNotFound            = errors.New("tenant not found")
	ErrIntegrationConfigRequired = errors.New("configure a log path or metrics endpoint before generating the integration")
	ErrMissingScriptParams       = errors.New("missing required query params")
)

type IntegrationScriptParams struct {
	ServiceID       string
	MachineID       string
	ServiceName     string
	APIKey          string
	LogPaths        string
	MetricsEndpoint string
	PlatformHost    string
	PlatformPort    string
	RequestHost     string
}

type IntegrationService struct {
	cfg           *config.Config
	tenantRepo    *repository.TenantRepository
	discoveryRepo *repository.DiscoveryRepository
}

func NewIntegrationService(
	cfg *config.Config,
	tenantRepo *repository.TenantRepository,
	discoveryRepo *repository.DiscoveryRepository,
) *IntegrationService {
	return &IntegrationService{
		cfg:           cfg,
		tenantRepo:    tenantRepo,
		discoveryRepo: discoveryRepo,
	}
}

func (s *IntegrationService) CreateInstallCommand(
	ctx context.Context,
	tenantID uint,
	serviceID uint,
	requestHost string,
) (*dto.IntegrationInstallResponse, error) {
	svc, err := s.discoveryRepo.FindServiceByIDTenantCtx(ctx, tenantID, serviceID)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, nil
	}

	tenant, err := s.tenantRepo.FindByIDCtx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, ErrTenantNotFound
	}

	detectedLogPaths, err := s.discoveryRepo.ListServiceLogPathsCtx(ctx, tenantID, svc.ID)
	if err != nil {
		return nil, err
	}
	var nginxVHosts []domain.NginxVirtualHost
	var apacheVHosts []domain.ApacheVirtualHost
	var logPathsJoined string

	if svc.LogCollectionOn {
		var err error
		nginxVHosts, err = s.listEnabledNginxVirtualHosts(ctx, tenantID, svc.ID)
		if err != nil {
			return nil, err
		}
		apacheVHosts, err = s.listEnabledApacheVirtualHosts(ctx, tenantID, svc.ID)
		if err != nil {
			return nil, err
		}

		if svc.ServiceName == "nginx" || svc.ServiceName == "apache" {
			var paths []string
			if svc.ServiceName == "nginx" {
				for _, vh := range nginxVHosts {
					if p := strings.TrimSpace(vh.LogPath); p != "" {
						paths = append(paths, p)
					}
				}
			} else {
				for _, vh := range apacheVHosts {
					if p := strings.TrimSpace(vh.LogPath); p != "" {
						paths = append(paths, p)
					}
				}
			}
			logPathsJoined = strings.Join(dedupeStrings(paths), ",")
		} else {
			logPaths := effectiveLogPathsForIntegration(svc, detectedLogPaths)
			logPathsJoined = strings.Join(logPaths, ",")
		}
	}

	var def *catalog.ServiceDef
	if d, ok := catalog.ResolveService(svc.ServiceName); ok {
		copy := d
		def = &copy
	}

	var metricsEndpoint string
	if svc.MetricCollectionOn {
		metricsEndpoint = metricsEndpointForIntegration(svc, def)
	}

	if logPathsJoined == "" && metricsEndpoint == "" && len(nginxVHosts) == 0 && len(apacheVHosts) == 0 {
		return nil, ErrIntegrationConfigRequired
	}

	scriptURL := fmt.Sprintf(
		"http://%s/integrations/generic.sh?service_id=%d&machine_id=%s&api_key=%s&service_name=%s&log_paths=%s&metrics_endpoint=%s&platform_host=%s&platform_port=%s",
		requestHost,
		serviceID,
		urlQueryEscape(svc.MachineID),
		urlQueryEscape(tenant.APIKey),
		urlQueryEscape(svc.ServiceName),
		urlQueryEscape(logPathsJoined),
		urlQueryEscape(metricsEndpoint),
		urlQueryEscape(requestHost),
		urlQueryEscape(s.cfg.ServerPort),
	)
	installCmd := fmt.Sprintf("curl -fsSL '%s' | sudo bash", scriptURL)

	if err := s.discoveryRepo.UpdateServiceCollectionCtx(
		ctx,
		tenantID,
		svc.ID,
		svc.LogCollectionOn,
		svc.MetricCollectionOn,
	); err != nil {
		return nil, err
	}

	return &dto.IntegrationInstallResponse{
		ServiceName:    svc.ServiceName,
		ScriptURL:      scriptURL,
		InstallCommand: installCmd,
	}, nil
}

func (s *IntegrationService) RenderGenericScript(params IntegrationScriptParams) (string, error) {
	if params.ServiceID == "" || params.MachineID == "" || params.APIKey == "" || params.ServiceName == "" {
		return "", ErrMissingScriptParams
	}
	if params.PlatformHost == "" {
		params.PlatformHost = params.RequestHost
	}
	if params.PlatformPort == "" {
		params.PlatformPort = s.cfg.ServerPort
	}

	if params.ServiceName == "nginx" {
		vhosts, err := s.loadNginxVirtualHostsForScript(params)
		if err != nil {
			return "", err
		}
		if len(vhosts) > 0 {
			return s.renderNginxScript(params, vhosts)
		}
	}

	if params.ServiceName == "apache" {
		vhosts, err := s.loadApacheVirtualHostsForScript(params)
		if err != nil {
			return "", err
		}
		if len(vhosts) > 0 {
			return s.renderApacheScript(params, vhosts)
		}
	}

	var paths []string
	for _, p := range strings.Split(params.LogPaths, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			paths = append(paths, p)
		}
	}

	logPathsSpace := strings.Join(paths, " ")
	serviceSlug := sanitizeServiceSlug(params.ServiceName)
	script := fmt.Sprintf(`#!/bin/bash
set -uo pipefail

MACHINE_ID=%s
API_KEY=%s
SERVICE_NAME=%s
SERVICE_SLUG=%s
LOG_PATHS=%s
METRICS_ENDPOINT=%s
PLATFORM_HOST=%s
PLATFORM_PORT=%s

CURRENT_MACHINE_ID=$(cat /etc/machine-id 2>/dev/null || cat /proc/sys/kernel/random/boot_id 2>/dev/null || echo "")
if [ "$CURRENT_MACHINE_ID" != "$MACHINE_ID" ]; then
  echo "[x] Wrong host for this integration script"
  exit 1
fi

[ "$EUID" -ne 0 ] && { echo "[x] Run as root (sudo)"; exit 1; }
[ ! -f /etc/otelcol/config.yaml ] && { echo "[x] Agent config not found: /etc/otelcol/config.yaml"; exit 1; }
[ ! -x /usr/local/bin/otelcol ] && { echo "[x] otelcol binary not found at /usr/local/bin/otelcol"; exit 1; }

AUTH_B64=$(echo -n "${API_KEY}:${API_KEY}" | base64 -w 0)
STATUS_ENDPOINT="http://${PLATFORM_HOST}:${PLATFORM_PORT}/ingest/integrations/status"

notify_status() {
  local LOG_ON="$1"
  local METRICS_ON="$2"
  PAYLOAD=$(cat <<JSON
{
  "machine_id": "${MACHINE_ID}",
  "service_name": "${SERVICE_NAME}",
  "log_collection_on": ${LOG_ON},
  "metric_collection_on": ${METRICS_ON}
}
JSON
)
  curl -sf -X POST \
    -H "Content-Type: application/json" \
    -H "Authorization: Basic ${AUTH_B64}" \
    -d "${PAYLOAD}" \
    "${STATUS_ENDPOINT}" >/dev/null 2>&1 || true
}

CONFIG_FILE="/etc/otelcol/config.yaml"
BACKUP_FILE="/etc/otelcol/config.yaml.integration.bak.$(date +%%s)"
cp "$CONFIG_FILE" "$BACKUP_FILE"
echo "[i] Backup created: $BACKUP_FILE"

export CONFIG_FILE SERVICE_NAME SERVICE_SLUG LOG_PATHS METRICS_ENDPOINT METRICS_RECEIVER_KEY
METRICS_RECEIVER_KEY=%s
if ! python3 - <<'PY'
import os
from urllib.parse import urlparse

cfg_path = os.environ["CONFIG_FILE"]
service_name = os.environ["SERVICE_NAME"]
service_slug = os.environ["SERVICE_SLUG"]
log_paths = [p for p in os.environ.get("LOG_PATHS", "").split() if p]
metrics_endpoint = os.environ.get("METRICS_ENDPOINT", "").strip()
filelog_key = f"filelog/{service_slug}_integration"
metrics_key = os.environ.get("METRICS_RECEIVER_KEY", "").strip()
if not metrics_key:
    metrics_key = f"prometheus/{service_slug}_integration"

with open(cfg_path, "r", encoding="utf-8") as f:
    text = f.read()

if log_paths:
    is_mysql = service_name.lower() in ("mysql", "mariadb", "mysqld", "mariadbd")
    is_postgres = service_name.lower().startswith("postgres")
    err_paths = [p for p in log_paths if "error" in p.lower()]
    slow_paths = [p for p in log_paths if "error" not in p.lower()]
    err_key = f"filelog/{service_slug}_error_integration"
    slow_key = f"filelog/{service_slug}_slow_integration"
    already_integrated = (
        (err_paths and f"  {err_key}:\n" in text)
        or (slow_paths and f"  {slow_key}:\n" in text)
    )
    if is_mysql and not already_integrated:
        inserted_block = []

        if err_paths:
            err_lines = "\n".join([f"      - {p}" for p in err_paths])
            inserted_block.append(
                f"  {err_key}:\n"
                "    include:\n"
                f"{err_lines}\n"
                "    include_file_path: true\n"
                "    start_at: end\n"
                "    operators:\n"
                "      - type: regex_parser\n"
                "        regex: '^\\\\d{4}-\\\\d{2}-\\\\d{2}T\\\\d{2}:\\\\d{2}:\\\\d{2}[.\\\\dZ+:\\\\-]*\\\\s+\\\\d+\\\\s+\\\\(?P<log_level>System|Warning|ERROR|Note)\\\\](?:\\\\s+\\\\(?P<mysql_error_code>MY-\\\\d+)\\\\])?\\\\s+\\\\(?P<mysql_subsystem>\\\\w+)\\\\]\\\\s+(?P<mysql_message>.{0,400})'\n"
                "        on_error: send_quiet\n"
                "    resource:\n"
                f"      service.name: {service_name}\n"
                f"      job: {service_name}\n"
                "      log_kind: error\n"
            )

        if slow_paths:
            slow_lines = "\n".join([f"      - {p}" for p in slow_paths])
            inserted_block.append(
                f"  {slow_key}:\n"
                "    include:\n"
                f"{slow_lines}\n"
                "    include_file_path: true\n"
                "    start_at: end\n"
                "    operators:\n"
                "      # Drop lines containing \"# Time:\" — these are standalone metadata\n"
                "      # lines that appear between queries and carry no query data.\n"
                "      # They can appear as \"# Time: ...\" or with a timestamp prefix.\n"
                "      - type: filter\n"
                "        expr: 'body matches \"#\\\\s*Time:\\\\s*\\\\d+\"'\n"
                "        on_error: send_quiet\n"
                "      # Join every physical line of one slow-query entry into a single\n"
                "      # record, using \"# User@Host:\" as the boundary — it appears exactly\n"
                "      # once per query, unlike \"# Time:\" which only appears when the\n"
                "      # second rolls over. This also prevents Loki from reordering lines\n"
                "      # that share an identical timestamp.\n"
                "      - type: recombine\n"
                "        is_first_entry: 'body matches \"^# User@Host:\"'\n"
                "        combine_field: body\n"
                "        combine_with: \"\\n\"\n"
                "        source_identifier: attributes[\"log.file.path\"]\n"
                "        max_batch_size: 200\n"
"        force_flush_period: 5s\n"
"        max_log_size: 65536\n"
                "        on_error: send_quiet\n"
                "      # Drop the monitoring agent's own queries entirely, now that each\n"
                "      # entry is a complete record. \"monitor\" is the DB user created for\n"
                "      # the observability agent's connection, not real app traffic.\n"
                "      - type: filter\n"
                "        expr: 'body matches \"User@Host:\\\\s*monitor\\\\[monitor\\\\]\"'\n"
                "        on_error: send_quiet\n"
                "    resource:\n"
                f"      service.name: {service_name}\n"
                f"      job: {service_name}\n"
                "      log_kind: slow\n"
            )

        block = "\n".join(inserted_block)
        marker = "\nprocessors:\n"
        if marker in text:
            text = text.replace(marker, "\n" + block + "\nprocessors:\n", 1)
        else:
            text += "\n" + block + "\n"
    elif is_postgres and f"  {filelog_key}:\n" not in text:
        include_lines = "\n".join([f"      - {p}" for p in log_paths])
        filelog_block = (
            f"  {filelog_key}:\n"
            "    include:\n"
            f"{include_lines}\n"
            "    include_file_path: true\n"
            "    start_at: end\n"
            "    operators:\n"
            "      # Join continuation lines (no leading timestamp) into the\n"
            "      # preceding entry that started with one.\n"
            "      - type: recombine\n"
            "        is_first_entry: 'body matches \"^\\\\d{4}-\\\\d{2}-\\\\d{2}[T ]\\\\d{2}:\\\\d{2}:\\\\d{2}\"'\n"
            "        combine_field: body\n"
            "        combine_with: \"\\n\"\n"
            "        source_identifier: attributes[\"log.file.path\"]\n"
            "        max_batch_size: 200\n"
            "        force_flush_period: 5s\n"
            "        max_log_size: 65536\n"
            "        on_error: send_quiet\n"
            "      - type: regex_parser\n"
            "        regex: '^\\d{4}-\\d{2}-\\d{2}[T ]\\d{2}:\\d{2}:\\d{2}[^\\[]*\\[\\d+\\]\\s*(?:(?P<pg_user>\\S+)@(?P<pg_database>\\S+)\\s+)?\\S+\\(\\d+\\)\\s+\\S+\\s+(?P<log_level>LOG|ERROR|FATAL|PANIC|WARNING|NOTICE|INFO|DETAIL|HINT|DEBUG\\d*):\\s+(?P<pg_message>.{0,600})'\n"
            "        on_error: send_quiet\n"
            "      # Drop \"connection received\" lines — pre-auth, no user info.\n"
            "      - type: filter\n"
            "        expr: 'body matches \"connection received:\"'\n"
            "        on_error: send_quiet\n"
            "      # Drop the monitoring agent's own queries — not real app traffic.\n"
            "      - type: filter\n"
            "        expr: 'attributes.pg_user == \"monitoring\" or attributes.pg_user == \"postgres_exporter\" or attributes.pg_user == \"pgexporter\"'\n"
            "        on_error: send_quiet\n"
            "    resource:\n"
            f"      service.name: {service_name}\n"
            f"      job: {service_name}\n"
            "      log.format: pg_csv\n"
        )
        marker = "\nprocessors:\n"
        if marker in text:
            text = text.replace(marker, "\n" + filelog_block + "\nprocessors:\n", 1)
        else:
            text += "\n" + filelog_block + "\n"
    elif not is_mysql and not is_postgres and f"  {filelog_key}:\n" not in text:
        include_lines = "\n".join([f"      - {p}" for p in log_paths])
        filelog_block = (
            f"  {filelog_key}:\n"
            "    include:\n"
            f"{include_lines}\n"
            "    include_file_path: true\n"
            "    start_at: end\n"
            "    resource:\n"
            f"      service.name: {service_name}\n"
            f"      job: {service_name}\n"
        )
        marker = "\nprocessors:\n"
        if marker in text:
            text = text.replace(marker, "\n" + filelog_block + "\nprocessors:\n", 1)
        else:
            text += "\n" + filelog_block + "\n"

if metrics_endpoint and f"  {metrics_key}:\n" not in text:
    parsed = urlparse(metrics_endpoint)
    host_port = parsed.netloc
    path = parsed.path or "/metrics"
    if metrics_key.startswith("nginx/") or metrics_key.startswith("apache/"):
        prom_block = (
            f"  {metrics_key}:\n"
            f"    endpoint: {metrics_endpoint}\n"
            "    collection_interval: 30s\n"
        )
    else:
        prom_block = (
            f"  {metrics_key}:\n"
            "    config:\n"
            "      scrape_configs:\n"
            f"        - job_name: {service_slug}\n"
            "          static_configs:\n"
            f"            - targets: ['{host_port}']\n"
            f"          metrics_path: {path}\n"
            "          scrape_interval: 30s\n"
        )
    marker = "\nprocessors:\n"
    if marker in text:
        text = text.replace(marker, "\n" + prom_block + "\nprocessors:\n", 1)
    else:
        text += "\n" + prom_block + "\n"

lines = text.splitlines()
current = ""
for i, line in enumerate(lines):
    stripped = line.strip()
    if stripped == "metrics:" and line.startswith("    "):
        current = "metrics"
    elif stripped == "logs:" and line.startswith("    "):
        current = "logs"
    elif stripped.endswith(":") and line.startswith("    ") and stripped not in ("metrics:", "logs:"):
        current = ""

    if current in ("metrics", "logs") and stripped.startswith("receivers: ["):
        start = stripped.find("[")
        end = stripped.rfind("]")
        payload = stripped[start + 1 : end].strip()
        arr = [x.strip() for x in payload.split(",") if x.strip()]
        if current == "logs" and log_paths:
            if is_mysql:
                receiver_keys = []
                if err_paths:
                    receiver_keys.append(f"filelog/{service_slug}_error_integration")
                if slow_paths:
                    receiver_keys.append(f"filelog/{service_slug}_slow_integration")
                for rk in receiver_keys:
                    if rk not in arr:
                        arr.append(rk)
            elif filelog_key not in arr:
                arr.append(filelog_key)
        elif current == "metrics" and metrics_endpoint:
            if metrics_key not in arr:
                arr.append(metrics_key)
        lines[i] = line[: line.index("receivers:")] + "receivers: [" + ", ".join(arr) + "]"

# Inject filter/drop_phpfpm_pollution processor for log pollution suppression
pollution_filter = (
    "  filter/drop_phpfpm_pollution:\n"
    "    error_mode: ignore\n"
    "    logs:\n"
    "      log_record:\n"
    "        - 'IsMatch(body, \"GET /status\")'\n"
    "        - 'IsMatch(body, \"/internal/opcache-collector\")'\n"
)
if "filter/drop_phpfpm_pollution:" not in text:
    marker = "\nprocessors:\n"
    if marker in text:
        text = text.replace(marker, "\n" + pollution_filter + "processors:\n", 1)
    else:
        text += "\n" + pollution_filter + "\n"
    lines = text.splitlines()

# Add filter/drop_phpfpm_pollution to logs pipeline processors list
for i, line in enumerate(lines):
    stripped = line.strip()
    if stripped.startswith("processors: [") and i > 0 and lines[i-1].strip() == "logs:":
        if "filter/drop_phpfpm_pollution" not in stripped:
            start = stripped.find("[")
            end = stripped.rfind("]")
            payload = stripped[start + 1 : end].strip()
            arr_proc = [x.strip() for x in payload.split(",") if x.strip()]
            if "filter/drop_phpfpm_pollution" not in arr_proc:
                arr_proc.append("filter/drop_phpfpm_pollution")
            lines[i] = line[: line.index("processors:")] + "processors: [" + ", ".join(arr_proc) + "]"

# Inject memory_limiter processor as defense-in-depth against unbounded memory
memory_limiter_def = (
    "  memory_limiter:\n"
    "    check_interval: 1s\n"
    "    limit_mib: 512\n"
    "    spike_limit_mib: 128\n"
)
if "memory_limiter:" not in text:
    marker = "\nprocessors:\n"
    if marker in text:
        text = text.replace(marker, "\n" + memory_limiter_def + "processors:\n", 1)
    else:
        text += "\n" + memory_limiter_def + "\n"
    lines = text.splitlines()

# Add memory_limiter as first processor in every pipeline
for i, line in enumerate(lines):
    stripped = line.strip()
    if stripped.startswith("processors: [") and i > 0:
        pipeline_name = lines[i-1].strip().rstrip(":")
        if pipeline_name in ("logs", "metrics", "traces") and "memory_limiter" not in stripped:
            start = stripped.find("[")
            end = stripped.rfind("]")
            cur = [x.strip() for x in stripped[start + 1 : end].split(",") if x.strip()]
            if "memory_limiter" not in cur:
                cur.insert(0, "memory_limiter")
            lines[i] = line[: line.index("processors:")] + "processors: [" + ", ".join(cur) + "]"

with open(cfg_path, "w", encoding="utf-8") as f:
    f.write("\n".join(lines) + "\n")
PY
then
  echo "[x] Failed to patch collector config"
  cp "$BACKUP_FILE" "$CONFIG_FILE" || true
  notify_status ${LOG_ON} ${METRICS_ON}
  exit 1
fi

if ! /usr/local/bin/otelcol validate --config "file:${CONFIG_FILE}" >/dev/null 2>&1; then
  echo "[x] Collector config validation failed, rolling back"
  cp "$BACKUP_FILE" "$CONFIG_FILE" || true
  notify_status ${LOG_ON} ${METRICS_ON}
  exit 1
fi

if systemctl reload otelcol >/dev/null 2>&1; then
  :
else
  systemctl restart otelcol >/dev/null 2>&1 || {
    echo "[x] Failed to reload/restart otelcol"
    cp "$BACKUP_FILE" "$CONFIG_FILE" || true
    systemctl restart otelcol >/dev/null 2>&1 || true
    notify_status ${LOG_ON} ${METRICS_ON}
    exit 1
  }
fi

LOG_ON=false
METRICS_ON=false
[ -n "$LOG_PATHS" ] && LOG_ON=true
[ -n "$METRICS_ENDPOINT" ] && METRICS_ON=true

if systemctl is-active --quiet otelcol; then
  notify_status ${LOG_ON} ${METRICS_ON}
  echo "[✓] Integration applied for ${SERVICE_NAME}"
else
  cp "$BACKUP_FILE" "$CONFIG_FILE" || true
  systemctl restart otelcol >/dev/null 2>&1 || true
  notify_status ${LOG_ON} ${METRICS_ON}
  echo "[x] otelcol is not active after update"
  exit 1
fi
`,
		shellEscapeSingle(params.MachineID),
		shellEscapeSingle(params.APIKey),
		shellEscapeSingle(params.ServiceName),
		shellEscapeSingle(serviceSlug),
		shellEscapeSingle(logPathsSpace),
		shellEscapeSingle(params.MetricsEndpoint),
		shellEscapeSingle(params.PlatformHost),
		shellEscapeSingle(params.PlatformPort),
		shellEscapeSingle(metricsReceiverKeyForIntegration(serviceSlug)),
	)

	return script, nil
}

func (s *IntegrationService) listEnabledNginxVirtualHosts(ctx context.Context, tenantID uint, serviceID uint) ([]domain.NginxVirtualHost, error) {
	rows, err := s.discoveryRepo.ListVirtualHostsByServiceCtx(ctx, tenantID, serviceID)
	if err != nil {
		return nil, err
	}

	filtered := make([]domain.NginxVirtualHost, 0, len(rows))
	for _, row := range rows {
		if !row.Enabled || strings.TrimSpace(row.LogPath) == "" {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered, nil
}

func (s *IntegrationService) loadNginxVirtualHostsForScript(params IntegrationScriptParams) ([]domain.NginxVirtualHost, error) {
	serviceID, err := strconv.ParseUint(strings.TrimSpace(params.ServiceID), 10, 64)
	if err != nil {
		return nil, ErrMissingScriptParams
	}

	tenant, err := s.tenantRepo.FindByAPIKey(context.Background(), params.APIKey)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, ErrTenantNotFound
	}

	svc, err := s.discoveryRepo.FindServiceByIDTenantCtx(context.Background(), tenant.ID, uint(serviceID))
	if err != nil {
		return nil, err
	}
	if svc == nil || svc.ServiceName != "nginx" {
		return nil, nil
	}
	if !svc.LogCollectionOn {
		return nil, nil
	}

	return s.listEnabledNginxVirtualHosts(context.Background(), tenant.ID, svc.ID)
}

func (s *IntegrationService) listEnabledApacheVirtualHosts(ctx context.Context, tenantID uint, serviceID uint) ([]domain.ApacheVirtualHost, error) {
	rows, err := s.discoveryRepo.ListApacheVirtualHostsByServiceCtx(ctx, tenantID, serviceID)
	if err != nil {
		return nil, err
	}

	filtered := make([]domain.ApacheVirtualHost, 0, len(rows))
	for _, row := range rows {
		if !row.Enabled || strings.TrimSpace(row.LogPath) == "" {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered, nil
}

func (s *IntegrationService) loadApacheVirtualHostsForScript(params IntegrationScriptParams) ([]domain.ApacheVirtualHost, error) {
	serviceID, err := strconv.ParseUint(strings.TrimSpace(params.ServiceID), 10, 64)
	if err != nil {
		return nil, ErrMissingScriptParams
	}

	tenant, err := s.tenantRepo.FindByAPIKey(context.Background(), params.APIKey)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, ErrTenantNotFound
	}

	svc, err := s.discoveryRepo.FindServiceByIDTenantCtx(context.Background(), tenant.ID, uint(serviceID))
	if err != nil {
		return nil, err
	}
	if svc == nil || svc.ServiceName != "apache" {
		return nil, nil
	}
	if !svc.LogCollectionOn {
		return nil, nil
	}

	return s.listEnabledApacheVirtualHosts(context.Background(), tenant.ID, svc.ID)
}

func (s *IntegrationService) renderNginxScript(
	params IntegrationScriptParams,
	vhosts []domain.NginxVirtualHost,
) (string, error) {
	// vhostConfig represents one filelog receiver block on the agent side.
	// Each domain produces up to two receivers: one for access logs (log_kind=access)
	// and one for error logs (log_kind=error). This lets Loki queries filter by
	// log_kind as a structured metadata label rather than guessing from file name.
	type vhostConfig struct {
		Receiver string   `json:"receiver"`
		Domain   string   `json:"domain"`
		LogKind  string   `json:"log_kind"` // "access" | "error"
		Paths    []string `json:"paths"`
	}

	type groupedVHost struct {
		domain      string
		accessPaths []string
		errorPaths  []string
	}

	grouped := make(map[string]*groupedVHost)
	order := make([]string, 0)
	for _, vh := range vhosts {
		domainName := strings.TrimSpace(vh.Domain)
		if domainName == "" {
			domainName = "(default)"
		}
		entry, ok := grouped[domainName]
		if !ok {
			entry = &groupedVHost{domain: domainName}
			grouped[domainName] = entry
			order = append(order, domainName)
		}
		logPath := strings.TrimSpace(vh.LogPath)
		// Use LogKind from the database row (set when the user configured the
		// virtual host). Fall back to filename heuristic if somehow empty.
		logKind := strings.TrimSpace(vh.LogKind)
		if logKind == "" {
			if strings.Contains(logPath, "error") {
				logKind = "error"
			} else {
				logKind = "access"
			}
		}
		if logKind == "error" {
			entry.errorPaths = append(entry.errorPaths, logPath)
		} else {
			entry.accessPaths = append(entry.accessPaths, logPath)
		}
	}

	usedReceivers := make(map[string]int)
	payload := make([]vhostConfig, 0, len(order)*2)
	for _, domainName := range order {
		entry := grouped[domainName]
		slug := sanitizeServiceSlug(domainName)

		// Access log receiver
		accessPaths := dedupeStrings(entry.accessPaths)
		if len(accessPaths) > 0 {
			baseKey := "filelog/nginx_" + slug + "_access_integration"
			receiverKey := baseKey
			if n := usedReceivers[baseKey]; n > 0 {
				receiverKey = fmt.Sprintf("%s_%d", baseKey, n+1)
			}
			usedReceivers[baseKey]++
			payload = append(payload, vhostConfig{
				Receiver: receiverKey,
				Domain:   domainName,
				LogKind:  "access",
				Paths:    accessPaths,
			})
		}

		// Error log receiver
		errorPaths := dedupeStrings(entry.errorPaths)
		if len(errorPaths) > 0 {
			baseKey := "filelog/nginx_" + slug + "_error_integration"
			receiverKey := baseKey
			if n := usedReceivers[baseKey]; n > 0 {
				receiverKey = fmt.Sprintf("%s_%d", baseKey, n+1)
			}
			usedReceivers[baseKey]++
			payload = append(payload, vhostConfig{
				Receiver: receiverKey,
				Domain:   domainName,
				LogKind:  "error",
				Paths:    errorPaths,
			})
		}
	}

	if len(payload) == 0 {
		return "", ErrIntegrationConfigRequired
	}

	vhostsJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	serviceSlug := sanitizeServiceSlug(params.ServiceName)
	script := fmt.Sprintf(`#!/bin/bash
set -uo pipefail

MACHINE_ID=%s
API_KEY=%s
SERVICE_NAME=%s
SERVICE_SLUG=%s
METRICS_ENDPOINT=%s
PLATFORM_HOST=%s
PLATFORM_PORT=%s
NGINX_VHOSTS_JSON=%s

CURRENT_MACHINE_ID=$(cat /etc/machine-id 2>/dev/null || cat /proc/sys/kernel/random/boot_id 2>/dev/null || echo "")
if [ "$CURRENT_MACHINE_ID" != "$MACHINE_ID" ]; then
  echo "[x] Wrong host for this integration script"
  exit 1
fi

[ "$EUID" -ne 0 ] && { echo "[x] Run as root (sudo)"; exit 1; }
[ ! -f /etc/otelcol/config.yaml ] && { echo "[x] Agent config not found: /etc/otelcol/config.yaml"; exit 1; }
[ ! -x /usr/local/bin/otelcol ] && { echo "[x] otelcol binary not found at /usr/local/bin/otelcol"; exit 1; }

AUTH_B64=$(echo -n "${API_KEY}:${API_KEY}" | base64 -w 0)
STATUS_ENDPOINT="http://${PLATFORM_HOST}:${PLATFORM_PORT}/ingest/integrations/status"

notify_status() {
  local LOG_ON="$1"
  local METRICS_ON="$2"
  PAYLOAD=$(cat <<JSON
{
  "machine_id": "${MACHINE_ID}",
  "service_name": "${SERVICE_NAME}",
  "log_collection_on": ${LOG_ON},
  "metric_collection_on": ${METRICS_ON}
}
JSON
)
  curl -sf -X POST \
    -H "Content-Type: application/json" \
    -H "Authorization: Basic ${AUTH_B64}" \
    -d "${PAYLOAD}" \
    "${STATUS_ENDPOINT}" >/dev/null 2>&1 || true
}

CONFIG_FILE="/etc/otelcol/config.yaml"
BACKUP_FILE="/etc/otelcol/config.yaml.integration.bak.$(date +%%s)"
cp "$CONFIG_FILE" "$BACKUP_FILE"
echo "[i] Backup created: $BACKUP_FILE"

export CONFIG_FILE SERVICE_NAME SERVICE_SLUG METRICS_ENDPOINT NGINX_VHOSTS_JSON METRICS_RECEIVER_KEY
METRICS_RECEIVER_KEY=%s
if ! python3 - <<'PY'
import json
import os
from urllib.parse import urlparse

cfg_path = os.environ["CONFIG_FILE"]
service_name = os.environ["SERVICE_NAME"]
service_slug = os.environ["SERVICE_SLUG"]
metrics_endpoint = os.environ.get("METRICS_ENDPOINT", "").strip()
metrics_key = os.environ.get("METRICS_RECEIVER_KEY", "").strip()
vhosts = json.loads(os.environ.get("NGINX_VHOSTS_JSON", "[]"))
if not metrics_key:
    metrics_key = f"prometheus/{service_slug}_integration"

with open(cfg_path, "r", encoding="utf-8") as f:
    text = f.read()

receiver_keys = []      # access log receivers
error_receiver_keys = []  # error log receivers
receiver_blocks = []

for item in vhosts:
    receiver = str(item.get("receiver", "")).strip()
    domain = str(item.get("domain", "")).strip() or "(default)"
    paths = [str(p).strip() for p in item.get("paths", []) if str(p).strip()]
    if not receiver or not paths:
        continue
    log_kind = str(item.get("log_kind", "access")).strip() or "access"

    if log_kind == "access":
        receiver_keys.append(receiver)
    else:
        error_receiver_keys.append(receiver)

    if f"  {receiver}:\n" in text:
        continue

    include_lines = "\n".join([f"      - {p}" for p in paths])

    if log_kind == "access":
        # Access logs: parse nginx combined log line with regex_parser.
        # normalized_path is computed by the transform/normalize_nginx_paths
        # PROCESSOR downstream — stanza operators do not support replace_pattern().
        receiver_blocks.append(
            f"  {receiver}:\n"
            "    include:\n"
            f"{include_lines}\n"
            "    include_file_path: true\n"
            "    start_at: end\n"
            "    operators:\n"
            "      - type: regex_parser\n"
            "        regex: '^(?P<client_ip>[^ ]+) [^ ]+ [^ ]+ \\[[^\\]]+\\] \"(?P<http_method>[A-Z]+) (?P<request_path>[^ ]+) [^\"]*\" (?P<http_status>\\d{3}) (?P<response_bytes>[\\d-]+)(?:\\s+\"(?P<http_referer>[^\"]*)\"\\s+\"(?P<user_agent>[^\"]*)\")?(?:.*?rt=(?P<request_time_s>[\\d.]+))?'\n"
            "        on_error: send_quiet\n"
            "    resource:\n"
            f"      service.name: {service_name}\n"
            f"      job: {service_name}\n"
            f"      vhost: {json.dumps(domain)}\n"
            f"      log_kind: {json.dumps(log_kind)}\n"
        )
    else:
        # Error logs: ship as-is.
        receiver_blocks.append(
            f"  {receiver}:\n"
            "    include:\n"
            f"{include_lines}\n"
            "    include_file_path: true\n"
            "    start_at: end\n"
            "    resource:\n"
            f"      service.name: {service_name}\n"
            f"      job: {service_name}\n"
            f"      vhost: {json.dumps(domain)}\n"
            f"      log_kind: {json.dumps(log_kind)}\n"
        )

all_nginx_receivers = receiver_keys + error_receiver_keys

# Inject filelog receiver blocks before the processors: section.
if receiver_blocks:
    block = "\n".join(receiver_blocks)
    marker = "\nprocessors:\n"
    if marker in text:
        text = text.replace(marker, "\n" + block + "\nprocessors:\n", 1)
    else:
        text += "\n" + block + "\n"

# Inject the transform/normalize_nginx_paths PROCESSOR.
# OTTL replace_pattern() is only valid in processors, not in filelog stanza operators.
normalize_proc_key = "transform/normalize_nginx_paths"
if all_nginx_receivers and f"  {normalize_proc_key}:\n" not in text:
    normalize_proc = (
        f"  {normalize_proc_key}:\n"
        "    error_mode: ignore\n"
        "    log_statements:\n"
        "      - context: log\n"
        "        statements:\n"
        "          - set(attributes[\"normalized_path\"], attributes[\"request_path\"])\n"
        "          - replace_pattern(attributes[\"normalized_path\"], \"\\\\?.*$\", \"\")\n"
        "          - replace_pattern(attributes[\"normalized_path\"], \"/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\", \"/:id\")\n"
        "          - replace_pattern(attributes[\"normalized_path\"], \"/[0-9a-fA-F]{6,35}\", \"/:id\")\n"
        "          - replace_pattern(attributes[\"normalized_path\"], \"/\\\\d{3,}\", \"/:id\")\n"
    )
    if "\nprocessors:\n" in text:
        text = text.replace("\nprocessors:\n", "\nprocessors:\n" + normalize_proc, 1)
    else:
        text += "\nprocessors:\n" + normalize_proc

# Inject metrics receiver block if needed.
if metrics_endpoint and f"  {metrics_key}:\n" not in text:
    parsed = urlparse(metrics_endpoint)
    host_port = parsed.netloc
    path = parsed.path or "/metrics"
    if metrics_key.startswith("nginx/") or metrics_key.startswith("apache/"):
        prom_block = (
            f"  {metrics_key}:\n"
            f"    endpoint: {metrics_endpoint}\n"
            "    collection_interval: 30s\n"
        )
    else:
        prom_block = (
            f"  {metrics_key}:\n"
            "    config:\n"
            "      scrape_configs:\n"
            f"        - job_name: {service_slug}\n"
            "          static_configs:\n"
            f"            - targets: ['{host_port}']\n"
            f"          metrics_path: {path}\n"
            "          scrape_interval: 30s\n"
        )
    marker = "\nprocessors:\n"
    if marker in text:
        text = text.replace(marker, "\n" + prom_block + "\nprocessors:\n", 1)
    else:
        text += "\n" + prom_block + "\n"

# ── Pipeline patching ──────────────────────────────────────────────────────
# nginx receivers go into a DEDICATED pipeline (logs/nginx) with the normalize
# processor. A receiver can only belong to ONE pipeline in otelcol — they must
# NOT be added to the main logs pipeline.
# metrics_key is added to the main metrics pipeline as before.

lines = text.splitlines()
current = ""
for i, line in enumerate(lines):
    stripped = line.strip()
    if stripped == "metrics:" and line.startswith("    "):
        current = "metrics"
    elif stripped == "logs:" and line.startswith("    "):
        current = "logs"
    elif stripped.endswith(":") and line.startswith("    ") and stripped not in ("metrics:", "logs:"):
        current = ""

    if current == "metrics" and stripped.startswith("receivers: [") and metrics_endpoint:
        start = stripped.find("[")
        end = stripped.rfind("]")
        payload = stripped[start + 1 : end].strip()
        arr = [x.strip() for x in payload.split(",") if x.strip()]
        if metrics_key not in arr:
            arr.append(metrics_key)
        lines[i] = line[: line.index("receivers:")] + "receivers: [" + ", ".join(arr) + "]"

text = "\n".join(lines) + "\n"

# Add the logs/nginx pipeline if it does not exist yet.
nginx_pipeline_key = "logs/nginx"
if all_nginx_receivers and f"    {nginx_pipeline_key}:\n" not in text:
    recv_list = ", ".join(all_nginx_receivers)
    nginx_pipeline = (
        f"    {nginx_pipeline_key}:\n"
        f"      receivers: [{recv_list}]\n"
        "      processors: [resourcedetection, resource, transform/normalize_nginx_paths, batch]\n"
        "      exporters: [otlphttp/logs]\n"
    )
    if "    traces:\n" in text:
        text = text.replace("    traces:\n", nginx_pipeline + "    traces:\n", 1)
    else:
        text = text.rstrip("\n") + "\n" + nginx_pipeline + "\n"

with open(cfg_path, "w", encoding="utf-8") as f:
    f.write(text)
PY
then
  echo "[x] Failed to patch collector config"
  cp "$BACKUP_FILE" "$CONFIG_FILE" || true
  notify_status ${LOG_ON} ${METRICS_ON}
  exit 1
fi

if ! /usr/local/bin/otelcol validate --config "file:${CONFIG_FILE}" >/dev/null 2>&1; then
  echo "[x] Collector config validation failed, rolling back"
  cp "$BACKUP_FILE" "$CONFIG_FILE" || true
  notify_status ${LOG_ON} ${METRICS_ON}
  exit 1
fi

if systemctl reload otelcol >/dev/null 2>&1; then
  :
else
  systemctl restart otelcol >/dev/null 2>&1 || {
    echo "[x] Failed to reload/restart otelcol"
    cp "$BACKUP_FILE" "$CONFIG_FILE" || true
    systemctl restart otelcol >/dev/null 2>&1 || true
    notify_status ${LOG_ON} ${METRICS_ON}
    exit 1
  }
fi

LOG_ON=false
[ -n "$NGINX_VHOSTS_JSON" ] && [ "$NGINX_VHOSTS_JSON" != "[]" ] && LOG_ON=true
METRICS_ON=false
[ -n "$METRICS_ENDPOINT" ] && METRICS_ON=true

if systemctl is-active --quiet otelcol; then
  notify_status ${LOG_ON} ${METRICS_ON}
  echo "[✓] Integration applied for ${SERVICE_NAME}"
else
  cp "$BACKUP_FILE" "$CONFIG_FILE" || true
  systemctl restart otelcol >/dev/null 2>&1 || true
  notify_status ${LOG_ON} ${METRICS_ON}
  echo "[x] otelcol is not active after update"
  exit 1
fi
`,
		shellEscapeSingle(params.MachineID),
		shellEscapeSingle(params.APIKey),
		shellEscapeSingle(params.ServiceName),
		shellEscapeSingle(serviceSlug),
		shellEscapeSingle(params.MetricsEndpoint),
		shellEscapeSingle(params.PlatformHost),
		shellEscapeSingle(params.PlatformPort),
		shellEscapeSingle(string(vhostsJSON)),
		shellEscapeSingle(metricsReceiverKeyForIntegration(serviceSlug)),
	)

	return script, nil
}

func (s *IntegrationService) renderApacheScript(
	params IntegrationScriptParams,
	vhosts []domain.ApacheVirtualHost,
) (string, error) {
	type vhostConfig struct {
		Receiver string   `json:"receiver"`
		Domain   string   `json:"domain"`
		LogKind  string   `json:"log_kind"`
		Paths    []string `json:"paths"`
	}

	type groupedVHost struct {
		domain      string
		accessPaths []string
		errorPaths  []string
	}

	grouped := make(map[string]*groupedVHost)
	order := make([]string, 0)
	for _, vh := range vhosts {
		domainName := strings.TrimSpace(vh.Domain)
		if domainName == "" {
			domainName = "(default)"
		}
		entry, ok := grouped[domainName]
		if !ok {
			entry = &groupedVHost{domain: domainName}
			grouped[domainName] = entry
			order = append(order, domainName)
		}
		logPath := strings.TrimSpace(vh.LogPath)
		logKind := strings.TrimSpace(vh.LogKind)
		if logKind == "" {
			if strings.Contains(logPath, "error") {
				logKind = "error"
			} else {
				logKind = "access"
			}
		}
		if logKind == "error" {
			entry.errorPaths = append(entry.errorPaths, logPath)
		} else {
			entry.accessPaths = append(entry.accessPaths, logPath)
		}
	}

	usedReceivers := make(map[string]int)
	payload := make([]vhostConfig, 0, len(order)*2)
	for _, domainName := range order {
		entry := grouped[domainName]
		slug := sanitizeServiceSlug(domainName)

		accessPaths := dedupeStrings(entry.accessPaths)
		if len(accessPaths) > 0 {
			baseKey := "filelog/apache_" + slug + "_access_integration"
			receiverKey := baseKey
			if n := usedReceivers[baseKey]; n > 0 {
				receiverKey = fmt.Sprintf("%s_%d", baseKey, n+1)
			}
			usedReceivers[baseKey]++
			payload = append(payload, vhostConfig{
				Receiver: receiverKey,
				Domain:   domainName,
				LogKind:  "access",
				Paths:    accessPaths,
			})
		}

		errorPaths := dedupeStrings(entry.errorPaths)
		if len(errorPaths) > 0 {
			baseKey := "filelog/apache_" + slug + "_error_integration"
			receiverKey := baseKey
			if n := usedReceivers[baseKey]; n > 0 {
				receiverKey = fmt.Sprintf("%s_%d", baseKey, n+1)
			}
			usedReceivers[baseKey]++
			payload = append(payload, vhostConfig{
				Receiver: receiverKey,
				Domain:   domainName,
				LogKind:  "error",
				Paths:    errorPaths,
			})
		}
	}

	if len(payload) == 0 {
		return "", ErrIntegrationConfigRequired
	}

	vhostsJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	serviceSlug := sanitizeServiceSlug(params.ServiceName)
	script := fmt.Sprintf(`#!/bin/bash
set -uo pipefail

MACHINE_ID=%s
API_KEY=%s
SERVICE_NAME=%s
SERVICE_SLUG=%s
METRICS_ENDPOINT=%s
PLATFORM_HOST=%s
PLATFORM_PORT=%s
APACHE_VHOSTS_JSON=%s

CURRENT_MACHINE_ID=$(cat /etc/machine-id 2>/dev/null || cat /proc/sys/kernel/random/boot_id 2>/dev/null || echo "")
if [ "$CURRENT_MACHINE_ID" != "$MACHINE_ID" ]; then
  echo "[x] Wrong host for this integration script"
  exit 1
fi

[ "$EUID" -ne 0 ] && { echo "[x] Run as root (sudo)"; exit 1; }
[ ! -f /etc/otelcol/config.yaml ] && { echo "[x] Agent config not found: /etc/otelcol/config.yaml"; exit 1; }
[ ! -x /usr/local/bin/otelcol ] && { echo "[x] otelcol binary not found at /usr/local/bin/otelcol"; exit 1; }

AUTH_B64=$(echo -n "${API_KEY}:${API_KEY}" | base64 -w 0)
STATUS_ENDPOINT="http://${PLATFORM_HOST}:${PLATFORM_PORT}/ingest/integrations/status"

notify_status() {
  local LOG_ON="$1"
  local METRICS_ON="$2"
  PAYLOAD=$(cat <<JSON
{
  "machine_id": "${MACHINE_ID}",
  "service_name": "${SERVICE_NAME}",
  "log_collection_on": ${LOG_ON},
  "metric_collection_on": ${METRICS_ON}
}
JSON
)
  curl -sf -X POST \
    -H "Content-Type: application/json" \
    -H "Authorization: Basic ${AUTH_B64}" \
    -d "${PAYLOAD}" \
    "${STATUS_ENDPOINT}" >/dev/null 2>&1 || true
}

CONFIG_FILE="/etc/otelcol/config.yaml"
BACKUP_FILE="/etc/otelcol/config.yaml.integration.bak.$(date +%%s)"
cp "$CONFIG_FILE" "$BACKUP_FILE"
echo "[i] Backup created: $BACKUP_FILE"

export CONFIG_FILE SERVICE_NAME SERVICE_SLUG METRICS_ENDPOINT APACHE_VHOSTS_JSON METRICS_RECEIVER_KEY
METRICS_RECEIVER_KEY=%s
if ! python3 - <<'PY'
import json
import os
from urllib.parse import urlparse

cfg_path = os.environ["CONFIG_FILE"]
service_name = os.environ["SERVICE_NAME"]
service_slug = os.environ["SERVICE_SLUG"]
metrics_endpoint = os.environ.get("METRICS_ENDPOINT", "").strip()
metrics_key = os.environ.get("METRICS_RECEIVER_KEY", "").strip()
vhosts = json.loads(os.environ.get("APACHE_VHOSTS_JSON", "[]"))
if not metrics_key:
    metrics_key = f"prometheus/{service_slug}_integration"

with open(cfg_path, "r", encoding="utf-8") as f:
    text = f.read()

receiver_keys = []
error_receiver_keys = []
receiver_blocks = []

for item in vhosts:
    receiver = str(item.get("receiver", "")).strip()
    domain = str(item.get("domain", "")).strip() or "(default)"
    paths = [str(p).strip() for p in item.get("paths", []) if str(p).strip()]
    if not receiver or not paths:
        continue
    log_kind = str(item.get("log_kind", "access")).strip() or "access"

    if log_kind == "access":
        receiver_keys.append(receiver)
    else:
        error_receiver_keys.append(receiver)

    if f"  {receiver}:\n" in text:
        continue

    include_lines = "\n".join([f"      - {p}" for p in paths])

    if log_kind == "access":
        receiver_blocks.append(
            f"  {receiver}:\n"
            "    include:\n"
            f"{include_lines}\n"
            "    include_file_path: true\n"
            "    start_at: end\n"
            "    operators:\n"
            "      - type: regex_parser\n"
            "        regex: '^(?P<client_ip>[^ ]+) [^ ]+ [^ ]+ \\[[^\\]]+\\] \"(?P<http_method>[A-Z]+) (?P<request_path>[^ ]+) [^\"]*\" (?P<http_status>\\d{3}) (?P<response_bytes>[\\d-]+)(?:\\s+\"(?P<http_referer>[^\"]*)\"\\s+\"(?P<user_agent>[^\"]*)\")?(?:.*?rt=(?P<request_time_s>[\\d.]+))?'\n"
            "        on_error: send_quiet\n"
            "    resource:\n"
            f"      service.name: {service_name}\n"
            f"      job: {service_name}\n"
            f"      vhost: {json.dumps(domain)}\n"
            f"      log_kind: {json.dumps(log_kind)}\n"
        )
    else:
        receiver_blocks.append(
            f"  {receiver}:\n"
            "    include:\n"
            f"{include_lines}\n"
            "    include_file_path: true\n"
            "    start_at: end\n"
            "    resource:\n"
            f"      service.name: {service_name}\n"
            f"      job: {service_name}\n"
            f"      vhost: {json.dumps(domain)}\n"
            f"      log_kind: {json.dumps(log_kind)}\n"
        )

all_apache_receivers = receiver_keys + error_receiver_keys

if receiver_blocks:
    block = "\n".join(receiver_blocks)
    marker = "\nprocessors:\n"
    if marker in text:
        text = text.replace(marker, "\n" + block + "\nprocessors:\n", 1)
    else:
        text += "\n" + block + "\n"

normalize_proc_key = "transform/normalize_apache_paths"
if all_apache_receivers and f"  {normalize_proc_key}:\n" not in text:
    normalize_proc = (
        f"  {normalize_proc_key}:\n"
        "    error_mode: ignore\n"
        "    log_statements:\n"
        "      - context: log\n"
        "        statements:\n"
        "          - set(attributes[\"normalized_path\"], attributes[\"request_path\"])\n"
        "          - replace_pattern(attributes[\"normalized_path\"], \"\\\\?.*$\", \"\")\n"
        "          - replace_pattern(attributes[\"normalized_path\"], \"/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\", \"/:id\")\n"
        "          - replace_pattern(attributes[\"normalized_path\"], \"/[0-9a-fA-F]{6,35}\", \"/:id\")\n"
        "          - replace_pattern(attributes[\"normalized_path\"], \"/\\\\d{3,}\", \"/:id\")\n"
    )
    if "\nprocessors:\n" in text:
        text = text.replace("\nprocessors:\n", "\nprocessors:\n" + normalize_proc, 1)
    else:
        text += "\nprocessors:\n" + normalize_proc

if metrics_endpoint and f"  {metrics_key}:\n" not in text:
    parsed = urlparse(metrics_endpoint)
    host_port = parsed.netloc
    path = parsed.path or "/metrics"
    if metrics_key.startswith("apache/"):
        prom_block = (
            f"  {metrics_key}:\n"
            f"    endpoint: {metrics_endpoint}\n"
            "    collection_interval: 30s\n"
        )
    else:
        prom_block = (
            f"  {metrics_key}:\n"
            "    config:\n"
            "      scrape_configs:\n"
            f"        - job_name: {service_slug}\n"
            "          static_configs:\n"
            f"            - targets: ['{host_port}']\n"
            f"          metrics_path: {path}\n"
            "          scrape_interval: 30s\n"
        )
    marker = "\nprocessors:\n"
    if marker in text:
        text = text.replace(marker, "\n" + prom_block + "\nprocessors:\n", 1)
    else:
        text += "\n" + prom_block + "\n"

lines = text.splitlines()
current = ""
for i, line in enumerate(lines):
    stripped = line.strip()
    if stripped == "metrics:" and line.startswith("    "):
        current = "metrics"
    elif stripped == "logs:" and line.startswith("    "):
        current = "logs"
    elif stripped.endswith(":") and line.startswith("    ") and stripped not in ("metrics:", "logs:"):
        current = ""

    if current == "metrics" and stripped.startswith("receivers: [") and metrics_endpoint:
        start = stripped.find("[")
        end = stripped.rfind("]")
        payload = stripped[start + 1 : end].strip()
        arr = [x.strip() for x in payload.split(",") if x.strip()]
        if metrics_key not in arr:
            arr.append(metrics_key)
        lines[i] = line[: line.index("receivers:")] + "receivers: [" + ", ".join(arr) + "]"

text = "\n".join(lines) + "\n"

apache_pipeline_key = "logs/apache"
if all_apache_receivers and f"    {apache_pipeline_key}:\n" not in text:
    recv_list = ", ".join(all_apache_receivers)
    apache_pipeline = (
        f"    {apache_pipeline_key}:\n"
        f"      receivers: [{recv_list}]\n"
        "      processors: [resourcedetection, resource, transform/normalize_apache_paths, batch]\n"
        "      exporters: [otlphttp/logs]\n"
    )
    if "    traces:\n" in text:
        text = text.replace("    traces:\n", apache_pipeline + "    traces:\n", 1)
    else:
        text = text.rstrip("\n") + "\n" + apache_pipeline + "\n"

with open(cfg_path, "w", encoding="utf-8") as f:
    f.write(text)
PY
then
  echo "[x] Failed to patch collector config"
  cp "$BACKUP_FILE" "$CONFIG_FILE" || true
  notify_status ${LOG_ON} ${METRICS_ON}
  exit 1
fi

if ! /usr/local/bin/otelcol validate --config "file:${CONFIG_FILE}" >/dev/null 2>&1; then
  echo "[x] Collector config validation failed, rolling back"
  cp "$BACKUP_FILE" "$CONFIG_FILE" || true
  notify_status ${LOG_ON} ${METRICS_ON}
  exit 1
fi

if systemctl reload otelcol >/dev/null 2>&1; then
  :
else
  systemctl restart otelcol >/dev/null 2>&1 || {
    echo "[x] Failed to reload/restart otelcol"
    cp "$BACKUP_FILE" "$CONFIG_FILE" || true
    systemctl restart otelcol >/dev/null 2>&1 || true
    notify_status ${LOG_ON} ${METRICS_ON}
    exit 1
  }
fi

LOG_ON=false
[ -n "$APACHE_VHOSTS_JSON" ] && [ "$APACHE_VHOSTS_JSON" != "[]" ] && LOG_ON=true
METRICS_ON=false
[ -n "$METRICS_ENDPOINT" ] && METRICS_ON=true

if systemctl is-active --quiet otelcol; then
  notify_status ${LOG_ON} ${METRICS_ON}
  echo "[✓] Integration applied for ${SERVICE_NAME}"
else
  cp "$BACKUP_FILE" "$CONFIG_FILE" || true
  systemctl restart otelcol >/dev/null 2>&1 || true
  notify_status ${LOG_ON} ${METRICS_ON}
  echo "[x] otelcol is not active after update"
  exit 1
fi
`,
		shellEscapeSingle(params.MachineID),
		shellEscapeSingle(params.APIKey),
		shellEscapeSingle(params.ServiceName),
		shellEscapeSingle(serviceSlug),
		shellEscapeSingle(params.MetricsEndpoint),
		shellEscapeSingle(params.PlatformHost),
		shellEscapeSingle(params.PlatformPort),
		shellEscapeSingle(string(vhostsJSON)),
		shellEscapeSingle(metricsReceiverKeyForIntegration(serviceSlug)),
	)

	return script, nil
}

func (s *IntegrationService) IngestStatus(ctx context.Context, tenantID uint, req dto.IntegrationStatusIngestRequest) error {
	return s.discoveryRepo.UpdateServiceCollectionByMachineNameCtx(
		ctx,
		tenantID,
		req.MachineID,
		req.ServiceName,
		req.LogCollectionOn,
		req.MetricCollectionOn,
	)
}

func effectiveLogPathsForIntegration(svc *domain.DiscoveredService, detected []string) []string {
	if svc == nil {
		return nil
	}
	if strings.TrimSpace(svc.ConfirmedLogPath) != "" {
		return []string{strings.TrimSpace(svc.ConfirmedLogPath)}
	}
	if len(detected) > 0 {
		return detected
	}
	out := make([]string, 0, 2)
	if svc.AccessLogPath != "" {
		out = append(out, svc.AccessLogPath)
	}
	if svc.ErrorLogPath != "" && svc.ErrorLogPath != svc.AccessLogPath {
		out = append(out, svc.ErrorLogPath)
	}
	return out
}

func metricsEndpointForIntegration(svc *domain.DiscoveredService, def *catalog.ServiceDef) string {
	if svc == nil {
		return ""
	}
	userEndpoint := strings.TrimSpace(svc.UserMetricsEndpoint)
	if userEndpoint != "" {
		return userEndpoint
	}
	if def != nil {
		return strings.TrimSpace(def.DefaultMetricsEndpoint)
	}
	return ""
}

func shellEscapeSingle(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func dedupeStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func sanitizeServiceSlug(serviceName string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	slug := strings.ToLower(strings.TrimSpace(serviceName))
	slug = re.ReplaceAllString(slug, "_")
	slug = strings.Trim(slug, "_")
	if slug == "" {
		return "service"
	}
	return slug
}

func metricsReceiverKeyForIntegration(serviceSlug string) string {
	switch serviceSlug {
	case "nginx":
		return fmt.Sprintf("nginx/%s_integration", serviceSlug)
	case "apache":
		return fmt.Sprintf("apache/%s_integration", serviceSlug)
	default:
		return fmt.Sprintf("prometheus/%s_integration", serviceSlug)
	}
}

func urlQueryEscape(v string) string {
	return url.QueryEscape(v)
}