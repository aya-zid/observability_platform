package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/catalog"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	custommiddleware "gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/middleware"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/validator"
)

type IntegrationHandler struct {
	cfg           *config.Config
	tenantRepo    *repository.TenantRepository
	discoveryRepo *repository.DiscoveryRepository
}

func NewIntegrationHandler(
	cfg *config.Config,
	tenantRepo *repository.TenantRepository,
	discoveryRepo *repository.DiscoveryRepository,
) *IntegrationHandler {
	return &IntegrationHandler{
		cfg:           cfg,
		tenantRepo:    tenantRepo,
		discoveryRepo: discoveryRepo,
	}
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
	metricsMode := strings.TrimSpace(svc.MetricsMode)
	userEndpoint := strings.TrimSpace(svc.UserMetricsEndpoint)
	defaultEndpoint := ""
	if def != nil {
		defaultEndpoint = strings.TrimSpace(def.DefaultMetricsEndpoint)
	}

	switch metricsMode {
	case "prometheus":
		return userEndpoint
	case "jmx":
		if svc.JMXExporterPort != nil {
			return fmt.Sprintf("http://127.0.0.1:%d/metrics", *svc.JMXExporterPort)
		}
		return ""
	case "none":
		return ""
	case "catalog", "":
		if userEndpoint != "" {
			return userEndpoint
		}
		return defaultEndpoint
	case "unknown":
		if userEndpoint != "" {
			return userEndpoint
		}
		return defaultEndpoint
	}
	return userEndpoint
}

func shellEscapeSingle(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
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
	default:
		return fmt.Sprintf("prometheus/%s_integration", serviceSlug)
	}
}

func (h *IntegrationHandler) CreateInstallCommand(c echo.Context) error {
	claimsAny := c.Get("claims")
	claims, ok := claimsAny.(*infrastructure.Claims)
	if !ok || claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	serviceID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid service id"})
	}

	svc, err := h.discoveryRepo.FindServiceByIDTenantCtx(c.Request().Context(), claims.TenantID, uint(serviceID))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load service"})
	}
	if svc == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "service not found"})
	}

	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load tenant"})
	}
	if tenant == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "tenant not found"})
	}

	detectedLogPaths, err := h.discoveryRepo.ListServiceLogPathsCtx(c.Request().Context(), claims.TenantID, svc.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load log paths"})
	}
	logPaths := effectiveLogPathsForIntegration(svc, detectedLogPaths)
	logPathsJoined := strings.Join(logPaths, ",")

	var def *catalog.ServiceDef
	if d, ok := catalog.Get(svc.ServiceName); ok {
		copy := d
		def = &copy
	}
	metricsEndpoint := metricsEndpointForIntegration(svc, def)
	if logPathsJoined == "" && metricsEndpoint == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "configure a log path or metrics endpoint before generating the integration"})
	}

	q := url.Values{}
	q.Set("service_id", strconv.FormatUint(serviceID, 10))
	q.Set("machine_id", svc.MachineID)
	q.Set("api_key", tenant.APIKey)
	q.Set("service_name", svc.ServiceName)
	q.Set("log_paths", logPathsJoined)
	q.Set("metrics_endpoint", metricsEndpoint)
	q.Set("platform_host", c.Request().Host)
	q.Set("platform_port", h.cfg.ServerPort)

	scriptURL := fmt.Sprintf("http://%s/integrations/generic.sh?%s", c.Request().Host, q.Encode())
	installCmd := fmt.Sprintf("curl -fsSL '%s' | sudo bash", scriptURL)

	if err := h.discoveryRepo.UpdateServiceCollectionCtx(
		c.Request().Context(),
		claims.TenantID,
		svc.ID,
		svc.LogCollectionOn,
		svc.MetricCollectionOn,
		domain.IntegrationStatusPending,
	); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to mark integration pending"})
	}

	return c.JSON(http.StatusOK, dto.IntegrationInstallResponse{
		ServiceName:    svc.ServiceName,
		ScriptURL:      scriptURL,
		InstallCommand: installCmd,
	})
}

func (h *IntegrationHandler) NginxScript(c echo.Context) error {
	return h.GenericScript(c)
}

func (h *IntegrationHandler) GenericScript(c echo.Context) error {
	serviceID := c.QueryParam("service_id")
	machineID := c.QueryParam("machine_id")
	serviceName := c.QueryParam("service_name")
	apiKey := c.QueryParam("api_key")
	logPaths := c.QueryParam("log_paths")
	metricsEndpoint := c.QueryParam("metrics_endpoint")
	platformHost := c.QueryParam("platform_host")
	platformPort := c.QueryParam("platform_port")

	if serviceID == "" || machineID == "" || apiKey == "" || serviceName == "" {
		return c.String(http.StatusBadRequest, "missing required query params")
	}
	if platformHost == "" {
		platformHost = c.Request().Host
	}
	if platformPort == "" {
		platformPort = h.cfg.ServerPort
	}

	var paths []string
	for _, p := range strings.Split(logPaths, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			paths = append(paths, p)
		}
	}
	logPathsSpace := strings.Join(paths, " ")
	serviceSlug := sanitizeServiceSlug(serviceName)

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
  local STATUS="$1"
  local LOG_ON="$2"
  local METRICS_ON="$3"
  PAYLOAD=$(cat <<JSON
{
  "machine_id": "${MACHINE_ID}",
  "service_name": "${SERVICE_NAME}",
  "log_collection_on": ${LOG_ON},
  "metric_collection_on": ${METRICS_ON},
  "integration_status": "${STATUS}"
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

export CONFIG_FILE SERVICE_NAME SERVICE_SLUG LOG_PATHS METRICS_ENDPOINT
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

if log_paths and f"  {filelog_key}:\n" not in text:
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
    if metrics_key.startswith("nginx/"):
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
        receiver = metrics_key if current == "metrics" else filelog_key
        if (current == "logs" and log_paths) or (current == "metrics" and metrics_endpoint):
            if receiver not in arr:
                arr.append(receiver)
        lines[i] = line[: line.index("receivers:")] + "receivers: [" + ", ".join(arr) + "]"

with open(cfg_path, "w", encoding="utf-8") as f:
    f.write("\n".join(lines) + "\n")
PY
then
  echo "[x] Failed to patch collector config"
  cp "$BACKUP_FILE" "$CONFIG_FILE" || true
  notify_status "error" false false
  exit 1
fi

if ! /usr/local/bin/otelcol validate --config "file:${CONFIG_FILE}" >/dev/null 2>&1; then
  echo "[x] Collector config validation failed, rolling back"
  cp "$BACKUP_FILE" "$CONFIG_FILE" || true
  notify_status "error" false false
  exit 1
fi

if systemctl reload otelcol >/dev/null 2>&1; then
  :
else
  systemctl restart otelcol >/dev/null 2>&1 || {
    echo "[x] Failed to reload/restart otelcol"
    cp "$BACKUP_FILE" "$CONFIG_FILE" || true
    systemctl restart otelcol >/dev/null 2>&1 || true
    notify_status "error" false false
    exit 1
  }
fi

LOG_ON=false
METRICS_ON=false
[ -n "$LOG_PATHS" ] && LOG_ON=true
[ -n "$METRICS_ENDPOINT" ] && METRICS_ON=true

if systemctl is-active --quiet otelcol; then
  notify_status "active" ${LOG_ON} ${METRICS_ON}
  echo "[✓] Integration applied for ${SERVICE_NAME}"
else
  cp "$BACKUP_FILE" "$CONFIG_FILE" || true
  systemctl restart otelcol >/dev/null 2>&1 || true
  notify_status "error" false false
  echo "[x] otelcol is not active after update"
  exit 1
fi
`, shellEscapeSingle(machineID), shellEscapeSingle(apiKey), shellEscapeSingle(serviceName), shellEscapeSingle(serviceSlug), shellEscapeSingle(logPathsSpace), shellEscapeSingle(metricsEndpoint), shellEscapeSingle(platformHost), shellEscapeSingle(platformPort))

	metricsReceiverKey := metricsReceiverKeyForIntegration(serviceSlug)
	script = strings.Replace(script, "export CONFIG_FILE SERVICE_NAME SERVICE_SLUG LOG_PATHS METRICS_ENDPOINT\n", fmt.Sprintf("export CONFIG_FILE SERVICE_NAME SERVICE_SLUG LOG_PATHS METRICS_ENDPOINT METRICS_RECEIVER_KEY\nMETRICS_RECEIVER_KEY=%s\n", shellEscapeSingle(metricsReceiverKey)), 1)

	c.Response().Header().Set(echo.HeaderContentType, "text/x-shellscript; charset=utf-8")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	c.Response().Header().Set("X-Content-Type-Options", "nosniff")
	return c.String(http.StatusOK, script)
}

func (h *IntegrationHandler) IngestStatus(c echo.Context) error {
	tenant := custommiddleware.GetTenantFromContext(c)
	if tenant == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	var req dto.IntegrationStatusIngestRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	status := domain.IntegrationStatusActive
	if req.IntegrationStatus != "" {
		status = domain.IntegrationStatus(req.IntegrationStatus)
	}

	err := h.discoveryRepo.UpdateServiceCollectionByMachineNameCtx(
		c.Request().Context(),
		tenant.ID,
		req.MachineID,
		req.ServiceName,
		req.LogCollectionOn,
		req.MetricCollectionOn,
		status,
	)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "service not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update integration status"})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func scriptPathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
