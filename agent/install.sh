#!/bin/bash
# ============================================
# OBSERVABILITY PLATFORM - Agent Installer
# Usage future: curl -sSL https://platform/install.sh | API_KEY=xxx bash 
# local usage now : sudo API_KEY=obs_test123 PLATFORM_HOST=localhost bash install.sh
# Runs as root for full system access (logs, metrics, docker, traces)
# ============================================

set -e

# ---- Colors ----
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log()     { echo -e "${GREEN}[✓]${NC} $1"; }
warn()    { echo -e "${YELLOW}[!]${NC} $1"; }
error()   { echo -e "${RED}[✗]${NC} $1"; exit 1; }
section() { echo -e "\n${BLUE}==>${NC} $1"; }

# ============================================
# CONFIGURATION — injected by backend
# ============================================
API_KEY="${API_KEY:-}"
PLATFORM_HOST="${PLATFORM_HOST:-THE_INFRA_IP}"
OTEL_VERSION="0.119.0"
AGENT_VERSION="1.0.0"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/otelcol"

# ============================================
# CHECKS
# ============================================
section "Checking requirements"

[ -z "$API_KEY" ] && error "API_KEY is required. Usage: curl -sSL .../install.sh | API_KEY=your_key bash"
[ "$EUID" -ne 0 ] && error "Please run as root or with sudo"
[ "$(uname -s)" != "Linux" ] && error "This installer only supports Linux"

# Detect architecture
ARCH=$(uname -m)
case $ARCH in
  x86_64)  ARCH_NAME="amd64" ;;
  aarch64) ARCH_NAME="arm64" ;;
  armv7l)  ARCH_NAME="arm"   ;;
  *)       error "Unsupported architecture: $ARCH" ;;
esac

# Detect OS
if [ -f /etc/os-release ]; then
  . /etc/os-release
  OS_NAME=$NAME
  OS_ID=$ID
else
  OS_NAME="Unknown Linux"
  OS_ID="unknown"
fi

# Unique machine identity
MACHINE_ID=$(cat /etc/machine-id 2>/dev/null || cat /proc/sys/kernel/random/boot_id 2>/dev/null || echo "unknown")
HOSTNAME_FQDN=$(hostname -f 2>/dev/null || hostname)

log "OS: $OS_NAME"
log "Architecture: $ARCH_NAME"
log "Hostname: $HOSTNAME_FQDN"
log "Machine ID: ${MACHINE_ID:0:8}..."
log "API Key: ${API_KEY:0:8}... (truncated)"
log "Platform: $PLATFORM_HOST"
log "Agent version: $AGENT_VERSION"

# ============================================
# DOWNLOAD COLLECTOR
# ============================================
section "Downloading OpenTelemetry Collector v${OTEL_VERSION}"

DOWNLOAD_URL="https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v${OTEL_VERSION}/otelcol-contrib_${OTEL_VERSION}_linux_${ARCH_NAME}.tar.gz"

cd /tmp
curl -sSL -o otelcol.tar.gz "$DOWNLOAD_URL" || error "Failed to download collector"
tar -xzf otelcol.tar.gz otelcol-contrib
mv otelcol-contrib "$INSTALL_DIR/otelcol"
chmod +x "$INSTALL_DIR/otelcol"
rm -f otelcol.tar.gz

log "Collector installed at $INSTALL_DIR/otelcol"

# ============================================
# DETECT LOG SOURCES
# ============================================
section "Detecting log sources"

FILELOG_RECEIVERS=""
FILELOG_IN_PIPELINE=()

add_filelog() {
  local NAME=$1
  local SERVICE=$2
  local JOB=$3
  local PATHS=$4

  FILELOG_RECEIVERS="${FILELOG_RECEIVERS}
  filelog/${NAME}:
    include:
${PATHS}
    include_file_path: true
    start_at: end
    resource:
      service.name: ${SERVICE}
      job: ${JOB}"

  FILELOG_IN_PIPELINE+=("filelog/${NAME}")
  log "${SERVICE} logs detected"
}

# ---- SYSLOG (Ubuntu/Debian/CentOS/RHEL) ----
SYSLOG_PATHS=""
[ -f /var/log/syslog ]   && SYSLOG_PATHS="      - /var/log/syslog"
[ -f /var/log/messages ] && SYSLOG_PATHS="${SYSLOG_PATHS}
      - /var/log/messages"
[ -n "$SYSLOG_PATHS" ] && add_filelog "system" "system" "syslog" "$SYSLOG_PATHS"

# ---- NGINX ----
if [ -d /var/log/nginx ]; then
  add_filelog "nginx" "nginx" "nginx" \
    "      - /var/log/nginx/access.log
      - /var/log/nginx/error.log"
fi

# ---- APACHE ----
APACHE_PATHS=""
[ -d /var/log/apache2 ] && APACHE_PATHS="      - /var/log/apache2/access.log
      - /var/log/apache2/error.log"
[ -d /var/log/httpd ]   && APACHE_PATHS="${APACHE_PATHS}
      - /var/log/httpd/access_log
      - /var/log/httpd/error_log"
[ -n "$APACHE_PATHS" ] && add_filelog "apache" "apache" "apache" "$APACHE_PATHS"

# ---- POSTGRESQL ----
POSTGRES_PATHS=""
[ -d /var/log/postgresql ]         && POSTGRES_PATHS="      - /var/log/postgresql/*.log"
[ -d /var/lib/pgsql/data/pg_log ]  && POSTGRES_PATHS="${POSTGRES_PATHS}
      - /var/lib/pgsql/data/pg_log/*.log"
[ -d /var/lib/pgsql/14/data/log ]  && POSTGRES_PATHS="${POSTGRES_PATHS}
      - /var/lib/pgsql/14/data/log/*.log"
[ -d /var/lib/pgsql/15/data/log ]  && POSTGRES_PATHS="${POSTGRES_PATHS}
      - /var/lib/pgsql/15/data/log/*.log"
[ -d /var/lib/pgsql/16/data/log ]  && POSTGRES_PATHS="${POSTGRES_PATHS}
      - /var/lib/pgsql/16/data/log/*.log"
[ -n "$POSTGRES_PATHS" ] && add_filelog "postgresql" "postgresql" "postgresql" "$POSTGRES_PATHS"

# ---- MYSQL / MARIADB ----
MYSQL_PATHS=""
[ -f /var/log/mysql/error.log ]     && MYSQL_PATHS="      - /var/log/mysql/error.log"
[ -f /var/log/mysql/mysql.log ]     && MYSQL_PATHS="${MYSQL_PATHS}
      - /var/log/mysql/mysql.log"
[ -f /var/log/mariadb/mariadb.log ] && MYSQL_PATHS="${MYSQL_PATHS}
      - /var/log/mariadb/mariadb.log"
[ -f /var/log/mysqld.log ]          && MYSQL_PATHS="${MYSQL_PATHS}
      - /var/log/mysqld.log"
[ -n "$MYSQL_PATHS" ] && add_filelog "mysql" "mysql" "mysql" "$MYSQL_PATHS"

# ---- MONGODB ----
MONGO_PATHS=""
[ -f /var/log/mongodb/mongod.log ] && MONGO_PATHS="      - /var/log/mongodb/mongod.log"
[ -f /var/log/mongod.log ]         && MONGO_PATHS="${MONGO_PATHS}
      - /var/log/mongod.log"
[ -n "$MONGO_PATHS" ] && add_filelog "mongodb" "mongodb" "mongodb" "$MONGO_PATHS"

# ---- REDIS ----
REDIS_PATHS=""
[ -f /var/log/redis/redis-server.log ] && REDIS_PATHS="      - /var/log/redis/redis-server.log"
[ -f /var/log/redis.log ]              && REDIS_PATHS="${REDIS_PATHS}
      - /var/log/redis.log"
[ -n "$REDIS_PATHS" ] && add_filelog "redis" "redis" "redis" "$REDIS_PATHS"

# ---- KAFKA ----
KAFKA_PATHS=""
[ -d /var/log/kafka ]        && KAFKA_PATHS="      - /var/log/kafka/*.log"
[ -d /opt/kafka/logs ]       && KAFKA_PATHS="${KAFKA_PATHS}
      - /opt/kafka/logs/*.log"
[ -d /usr/local/kafka/logs ] && KAFKA_PATHS="${KAFKA_PATHS}
      - /usr/local/kafka/logs/*.log"
[ -n "$KAFKA_PATHS" ] && add_filelog "kafka" "kafka" "kafka" "$KAFKA_PATHS"

# ---- ELASTICSEARCH ----
ELASTIC_PATHS=""
[ -d /var/log/elasticsearch ] && ELASTIC_PATHS="      - /var/log/elasticsearch/*.log"
[ -n "$ELASTIC_PATHS" ] && add_filelog "elasticsearch" "elasticsearch" "elasticsearch" "$ELASTIC_PATHS"

# ---- RABBITMQ ----
RABBIT_PATHS=""
[ -d /var/log/rabbitmq ] && RABBIT_PATHS="      - /var/log/rabbitmq/*.log"
[ -n "$RABBIT_PATHS" ] && add_filelog "rabbitmq" "rabbitmq" "rabbitmq" "$RABBIT_PATHS"

# ---- HAPROXY ----
HAPROXY_PATHS=""
[ -f /var/log/haproxy.log ] && HAPROXY_PATHS="      - /var/log/haproxy.log"
[ -n "$HAPROXY_PATHS" ] && add_filelog "haproxy" "haproxy" "haproxy" "$HAPROXY_PATHS"

# ---- SSH / AUTH ----
SSH_PATHS=""
[ -f /var/log/auth.log ] && SSH_PATHS="      - /var/log/auth.log"
[ -f /var/log/secure ]   && SSH_PATHS="${SSH_PATHS}
      - /var/log/secure"
[ -n "$SSH_PATHS" ] && add_filelog "ssh" "ssh" "ssh" "$SSH_PATHS"

# ---- DOCKER CONTAINER LOGS ----
if [ -S /var/run/docker.sock ]; then
  FILELOG_RECEIVERS="${FILELOG_RECEIVERS}
  filelog/docker:
    include:
      - /var/lib/docker/containers/*/*.log
    include_file_path: true
    start_at: end
    operators:
      - type: json_parser
        timestamp:
          parse_from: attributes.time
          layout: '%Y-%m-%dT%H:%M:%S.%LZ'
      - type: move
        from: attributes.log
        to: body
      - type: move
        from: attributes.stream
        to: attributes[\"docker.stream\"]
    resource:
      service.name: docker
      job: docker"
  FILELOG_IN_PIPELINE+=("filelog/docker")
  log "Docker detected"
fi

# ---- Build pipeline receiver list ----
PIPELINE_RECEIVERS="journald"
if [ ${#FILELOG_IN_PIPELINE[@]} -gt 0 ]; then
  for r in "${FILELOG_IN_PIPELINE[@]}"; do
    PIPELINE_RECEIVERS="${PIPELINE_RECEIVERS}, ${r}"
  done
fi

# ============================================
# WRITE CONFIG FILE
# ============================================
section "Generating agent configuration"

AUTH_B64=$(echo -n "${API_KEY}:${API_KEY}" | base64 -w 0)
mkdir -p "$CONFIG_DIR"

cat > "$CONFIG_DIR/config.yaml" <<YAML
# ============================================
# OTEL COLLECTOR - Auto-generated by installer
# Agent Version : ${AGENT_VERSION}
# Tenant        : ${API_KEY:0:8}...
# Host          : ${HOSTNAME_FQDN}
# Machine ID    : ${MACHINE_ID:0:8}...
# Generated     : $(date -u +"%Y-%m-%dT%H:%M:%SZ")
# ============================================

extensions:
  health_check:
    endpoint: 0.0.0.0:13133

# ============================================
# RECEIVERS
# ============================================
receivers:

  # Host metrics — CPU, memory, disk, network, processes
  hostmetrics:
    collection_interval: 30s
    scrapers:
      cpu:
        metrics:
          system.cpu.utilization:
            enabled: true
      memory:
        metrics:
          system.memory.utilization:
            enabled: true
      disk: {}
      filesystem:
        metrics:
          system.filesystem.utilization:
            enabled: true
      network: {}
      load: {}
      processes: {}
      process:
        mute_process_name_error: true
        mute_process_exe_error: true
        mute_process_io_error: true

  # Systemd journal logs
  journald:
    start_at: end
    resource:
      service.name: journald
      job: journald

  # OTLP — receives traces from instrumented apps
  # Apps on this host     → localhost:4317 or localhost:4318
  # Apps inside Docker    → host.docker.internal:4317
  # Apps on other servers → this_machine_ip:4317
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
${FILELOG_RECEIVERS}

# ============================================
# PROCESSORS
# ============================================
processors:

  batch:
    timeout: 5s
    send_batch_size: 1024

  resource:
    attributes:
      - key: tenant.id
        value: "${API_KEY}"
        action: upsert
      - key: host.name
        value: "${HOSTNAME_FQDN}"
        action: upsert
      - key: host.id
        value: "${MACHINE_ID}"
        action: upsert
      - key: agent.version
        value: "${AGENT_VERSION}"
        action: upsert
      - key: os.type
        value: "${OS_ID}"
        action: upsert

  resourcedetection:
    detectors: [env, system, docker]
    timeout: 5s
    override: false

# ============================================
# EXPORTERS
# ============================================
exporters:

  # Metrics → VictoriaMetrics via vmauth
  prometheusremotewrite:
    endpoint: "http://${PLATFORM_HOST}:8428/api/v1/write"
    tls:
      insecure: true
    headers:
      Authorization: "Basic ${AUTH_B64}"

  # Logs → Loki
  otlphttp/loki:
    endpoint: "http://${PLATFORM_HOST}:3100/otlp"
    tls:
      insecure: true
    headers:
      X-Scope-OrgID: "${API_KEY}"

  # Traces → Tempo (HTTP, not gRPC — Tempo exposes HTTP on port 3200)
  otlphttp/tempo:
    endpoint: "http://${PLATFORM_HOST}:4321"
    tls:
      insecure: true
    headers:
      X-Scope-OrgID: "${API_KEY}"

# ============================================
# PIPELINES
# ============================================
service:
  extensions: [health_check]
  pipelines:

    metrics:
      receivers: [hostmetrics]
      processors: [resourcedetection, resource, batch]
      exporters: [prometheusremotewrite]

    logs:
      receivers: [${PIPELINE_RECEIVERS}]
      processors: [resourcedetection, resource, batch]
      exporters: [otlphttp/loki]

    # Traces from instrumented apps (Node.js, Python, Go, etc.)
    # Note: only batch processor — apps set their own service.name and tenant.id
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp/tempo]

  telemetry:
    logs:
      level: warn
    metrics:
      level: none
YAML

# Config contains API key — root only
chown root:root "$CONFIG_DIR/config.yaml"
chmod 600 "$CONFIG_DIR/config.yaml"
log "Config written to $CONFIG_DIR/config.yaml (permissions: 600, owner: root)"

# ============================================
# REMOVE OLD otelcol USER IF EXISTS
# ============================================
if id otelcol &>/dev/null; then
  warn "Removing old otelcol user (agent now runs as root)"
  chown root:root -R "$CONFIG_DIR" 2>/dev/null || true
  userdel otelcol 2>/dev/null || true
fi

# ============================================
# SYSTEMD SERVICE — runs as root
# ============================================
section "Installing systemd service"

cat > /etc/systemd/system/otelcol.service <<EOF
[Unit]
Description=OpenTelemetry Collector Agent v${AGENT_VERSION}
Documentation=https://opentelemetry.io
After=network.target
Wants=network.target

[Service]
Type=simple
User=root
Group=root
ExecStart=/usr/local/bin/otelcol --config /etc/otelcol/config.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=otelcol
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable otelcol
systemctl restart otelcol

sleep 3
if systemctl is-active --quiet otelcol; then
  log "Service started successfully"
else
  error "Service failed to start. Run: journalctl -u otelcol -n 50"
fi

# ============================================
# SUMMARY
# ============================================
echo ""
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  Agent installed successfully!${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""
echo "  Agent version : $AGENT_VERSION"
echo "  Collector     : otelcol-contrib v$OTEL_VERSION"
echo "  Host          : $HOSTNAME_FQDN"
echo "  Machine ID    : ${MACHINE_ID:0:8}..."
echo "  Tenant        : ${API_KEY:0:8}..."
echo "  Platform      : $PLATFORM_HOST"
echo ""
echo "  Collecting:"
echo "    ✓ Host metrics (CPU, memory, disk, network, processes)"
echo "    ✓ System logs (journald)"
for r in "${FILELOG_IN_PIPELINE[@]}"; do
  NAME=$(echo "$r" | sed 's/filelog\///')
  echo "    ✓ ${NAME} logs"
done
echo "    ✓ Traces from apps (OTLP gRPC:4317 / HTTP:4318)"
echo ""
echo "  Commands:"
echo "    Status  : sudo systemctl status otelcol"
echo "    Logs    : sudo journalctl -u otelcol -f"
echo "    Restart : sudo systemctl restart otelcol"
echo "    Stop    : sudo systemctl stop otelcol"
echo ""
echo "  To instrument an app on this host:"
echo "    OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317"
echo ""
echo "  To instrument an app in Docker on this host:"
echo "    OTEL_EXPORTER_OTLP_ENDPOINT=http://host.docker.internal:4317"
echo ""