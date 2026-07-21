#!/bin/bash
# ============================================
# OBSERVABILITY PLATFORM — Agent Installer
# Usage:
#   curl -sSL http://<platform>:8080/install.sh | sudo API_KEY=xxx bash
#   sudo API_KEY=xxx PLATFORM_HOST=<host> bash install.sh
# Runs as root for full system access
# ============================================

set -uo pipefail

# ============================================
# BOOTSTRAP
# ============================================

# ---- Colors ----
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

log()     { echo -e "${GREEN}[✓]${NC} $1"; }
warn()    { echo -e "${YELLOW}[!]${NC} $1"; }
info()    { echo -e "${CYAN}[i]${NC} $1"; }
error()   { echo -e "${RED}[✗]${NC} $1"; exit 1; }
section() { echo -e "\n${BOLD}${BLUE}━━━ $1 ━━━${NC}"; }
step()    { echo -e "\n${BOLD}${BLUE}◆ $1${NC}"; }
fixed()   { echo -e "${GREEN}[fixed]${NC} $1"; }
skipped() { echo -e "${YELLOW}[skipped]${NC} $1"; }
manual()  { echo -e "${RED}[manual]${NC} $1"; }

# ---- Ask helper ----
ask() {
  local QUESTION=$1
  echo -en "${YELLOW}[?]${NC} $QUESTION ${BOLD}[y/N]${NC} "
  read -r REPLY </dev/tty
  [[ "$REPLY" =~ ^[Yy]$ ]]
}

# ---- Backup a file before modifying ----
backup_file() {
  local FILE=$1
  [ -f "$FILE" ] && cp "$FILE" "${FILE}.bak.$(date +%s)" && info "Backed up $FILE"
}

# ---- Startup banner ----
echo ""
echo -e "${BOLD}${BLUE}  ╔══════════════════════════════════════════════════════════╗${NC}"
printf "${BOLD}${BLUE}  ║${NC}  Observability Platform — Agent Installer%14s${BOLD}${BLUE}║${NC}\n" ""
printf "${BOLD}${BLUE}  ║${NC}  Version ${AGENT_VERSION:-1.0.0}%39s${BOLD}${BLUE}║${NC}\n" ""
echo -e "${BOLD}${BLUE}  ╚══════════════════════════════════════════════════════════╝${NC}"
echo ""

# ============================================
# CONFIGURATION
# ============================================
API_KEY="${API_KEY:-}"
PLATFORM_HOST="${PLATFORM_HOST:-THE_INFRA_IP}"
PLATFORM_PORT="${PLATFORM_PORT:-8080}"
OTEL_VERSION="0.119.0"
AGENT_VERSION="1.0.0"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/otelcol"
REPORT_FILE="/etc/otelcol/discovery-report.txt"

METRICS_ENDPOINT="http://${PLATFORM_HOST}:${PLATFORM_PORT}/ingest/metrics"
LOGS_ENDPOINT="http://${PLATFORM_HOST}:${PLATFORM_PORT}/ingest/logs"
TRACES_ENDPOINT="http://${PLATFORM_HOST}:${PLATFORM_PORT}/ingest/traces"
HOST_DISCOVERY_ENDPOINT="http://${PLATFORM_HOST}:${PLATFORM_PORT}/ingest/discovery/host-services"
CONTAINER_DISCOVERY_ENDPOINT="http://${PLATFORM_HOST}:${PLATFORM_PORT}/ingest/discovery/container-services"
HOST_REGISTER_ENDPOINT="http://${PLATFORM_HOST}:${PLATFORM_PORT}/ingest/hosts/register"
HOST_HEARTBEAT_ENDPOINT="http://${PLATFORM_HOST}:${PLATFORM_PORT}/ingest/hosts/heartbeat"

# ============================================
# DISCOVERY STATE
# ============================================
declare -A LOG_PATHS
declare -A SVC_PORT
declare -A SVC_VERSION
declare -A SVC_STATUS
declare -A SVC_NOTES
declare -a MANUAL_STEPS=()
declare -a ALL_SERVICES=()
declare -A LOG_FORMAT

# Phase 3: web-server virtual hosts support
declare -A WEBSERVER_VHOSTS_JSON

FILELOG_RECEIVERS_YAML=""
declare -a FILELOG_PIPELINE=()
declare -a METRIC_PIPELINE=("hostmetrics")

DOCKER_LOG_TAILER_UNIT_READY=false
DOCKER_METRICS_UNIT_READY=false
PROM_FILE_SERVER_UNIT_READY=false
DOCKER_WATCHER_UNIT_READY=false
HEARTBEAT_UNIT_READY=false

# ============================================
# CHECKS
# ============================================
section "Checking requirements"

[ -z "$API_KEY" ] && error "API_KEY is required. Usage: API_KEY=your_key bash install.sh"
[ "$EUID" -ne 0 ] && error "Please run as root or with sudo"
[ "$(uname -s)" != "Linux" ] && error "This installer only supports Linux"

ARCH=$(uname -m)
case $ARCH in
  x86_64)  ARCH_NAME="amd64" ;;
  aarch64) ARCH_NAME="arm64" ;;
  armv7l)  ARCH_NAME="arm"   ;;
  *)       error "Unsupported architecture: $ARCH" ;;
esac

if [ -f /etc/os-release ]; then
  . /etc/os-release
  OS_NAME=$NAME; OS_ID=$ID; OS_VERSION_ID=${VERSION_ID:-""}
else
  OS_NAME="Unknown Linux"; OS_ID="unknown"; OS_VERSION_ID=""
fi

MACHINE_ID=$(cat /etc/machine-id 2>/dev/null \
  || cat /proc/sys/kernel/random/boot_id 2>/dev/null \
  || echo "unknown")
HOSTNAME_FQDN=$(hostname -f 2>/dev/null || hostname)

log "OS: $OS_NAME $OS_VERSION_ID"
log "Architecture: $ARCH_NAME"
log "Hostname: $HOSTNAME_FQDN"
log "Machine ID: ${MACHINE_ID:0:8}..."
log "API Key: ${API_KEY:0:8}..."
log "Platform: $PLATFORM_HOST:$PLATFORM_PORT"

# ============================================
# UTILITY FUNCTIONS
# ============================================

get_unit_pids() {
  local UNIT=$1
  local MAIN
  MAIN=$(systemctl show -p MainPID "$UNIT" 2>/dev/null | cut -d= -f2)
  [ -n "$MAIN" ] && [ "$MAIN" != "0" ] && echo "$MAIN"
  local CG
  CG=$(systemctl show -p ControlGroup "$UNIT" 2>/dev/null | cut -d= -f2)
  if [ -n "$CG" ]; then
    local PROCS="/sys/fs/cgroup${CG}/cgroup.procs"
    [ -f "$PROCS" ] && cat "$PROCS" 2>/dev/null && return
    local PROCS1="/sys/fs/cgroup/systemd${CG}/cgroup.procs"
    [ -f "$PROCS1" ] && cat "$PROCS1" 2>/dev/null
  fi
}

get_proc_log_files() {
  local PID=$1
  [ -d "/proc/${PID}/fd" ] || return
  for FD in /proc/${PID}/fd/*; do
    local T
    T=$(readlink -f "$FD" 2>/dev/null) || continue
    [[ "$T" == /proc/* ]] || [[ "$T" == /dev/* ]] || \
    [[ "$T" == /sys/* ]]  || [[ "$T" == anon_inode* ]] || \
    [[ "$T" == pipe:* ]]  || [[ "$T" == socket:* ]] && continue
    [ -f "$T" ] || continue
    if [[ "$T" == *.log* ]] || [[ "$T" == */log/* ]] || [[ "$T" == */logs/* ]]; then
      echo "$T"
    fi
  done
}

collect_proc_logs() {
  local UNIT=$1
  local PATHS=""
  for PID in $(get_unit_pids "$UNIT" 2>/dev/null); do
    local FL
    FL=$(get_proc_log_files "$PID" 2>/dev/null || true)
    PATHS="${PATHS}"$'\n'"$FL"
  done
  echo "$PATHS" | sort -u | grep -v '^$'
}

path_exists() {
  local P=$1
  if [[ "$P" == *"*"* ]]; then
    compgen -G "$P" &>/dev/null
  else
    [ -f "$P" ]
  fi
}

filter_existing() {
  while IFS= read -r P; do
    [ -z "$P" ] && continue
    path_exists "$P" && echo "$P"
  done
}

to_yaml_list() {
  local INDENT="${1:-      }"
  while IFS= read -r P; do
    [ -n "$P" ] && echo "${INDENT}- ${P}"
  done
}

trim_one_line() {
  # Collapse whitespace/newlines to a single safe line.
  local S=${1-}
  S=${S//$'\r'/}
  S=${S//$'\n'/ }
  S=$(printf '%s' "$S" | tr -s '[:space:]' ' ' | sed 's/^ //;s/ $//') || true
  printf '%s' "$S"
}

yaml_quote() {
  # Double-quote and escape a YAML scalar.
  local S=${1-}
  S=${S//$'\r'/}
  S=${S//\\/\\\\}
  S=${S//\"/\\\"}
  S=${S//$'\n'/\\n}
  S=${S//$'\t'/\\t}
  printf '"%s"' "$S"
}

json_escape() {
  # Escape a string for safe JSON embedding (without surrounding quotes).
  local S=${1-}
  S=${S//\\/\\\\}
  S=${S//\"/\\\"}
  S=${S//$'\n'/\\n}
  S=${S//$'\r'/\\r}
  S=${S//$'\t'/\\t}
  printf '%s' "$S"
}

dedupe_array() {
  local -n ARR_REF=$1
  local -A SEEN=()
  local -a OUT=()
  local ITEM
  for ITEM in "${ARR_REF[@]}"; do
    [ -n "${SEEN[$ITEM]:-}" ] && continue
    SEEN["$ITEM"]=1
    OUT+=("$ITEM")
  done
  ARR_REF=("${OUT[@]}")
}

register_service() {
  local NAME=$1
  local PATHS=$2
  local STATUS=$3   # "discovered" | "logs_found" | "no_logs"
  local PORT=$4
  local NOTES=$5
  local VERSION=${6:-}

  ALL_SERVICES+=("$NAME")
  LOG_PATHS[$NAME]="$PATHS"
  SVC_PORT[$NAME]="$PORT"
  SVC_VERSION[$NAME]="$VERSION"
  SVC_NOTES[$NAME]="$NOTES"
  SVC_STATUS[$NAME]="$STATUS"
}

# ============================================
# PHASE 1 — SERVICE DISCOVERY (log path only)
# ============================================
section "Phase 1: Service discovery"

IGNORE_UNITS="systemd-|dbus|NetworkManager|network|firewalld|crond|cron|atd|\
accounts-daemon|polkit|rsyslog|syslog$|snapd|packagekit|udisks|\
ModemManager|avahi-daemon|cups|bluetooth|getty|serial-getty|\
ssh-agent|gpg-agent|user@|session-|auditd|irqbalance|\
multipathd|lvm2|dm-event|systemd-logind|systemd-resolved|\
systemd-timesyncd|systemd-udevd|systemd-journald|wpa_supplicant|\
rtkit-daemon|colord|thermald|fwupd|unattended-upgrades|haveged|\
 lightdm|smartmontools|pcscd|upower|containerd|docker$|docker.socket|\
otelcol$|agent-docker-|agent_|agent-prom-|agent-rediscover$"

RUNNING_UNITS=$(systemctl list-units --type=service --state=running \
  --no-legend --plain 2>/dev/null \
  | awk '{print $1}' | sed 's/\.service$//' || true)

# ---- Helper: detect if a unit logs to journald / syslog ----
unit_uses_journald() {
  local UNIT=$1
  # Units with StandardOutput=journal or no log file wired → they log to journald
  local STD_OUT
  STD_OUT=$(systemctl show -p StandardOutput "${UNIT}.service" 2>/dev/null | cut -d= -f2)
  [[ "$STD_OUT" == "journal" || "$STD_OUT" == "journal+console" || "$STD_OUT" == "kmsg" ]]
}

# ─────────────────────────────────────────────────────────────────────────────
# Nginx virtual host detection — pure bash, no python/perl/ruby required.
# Only uses: bash, grep, sed, find, tr, cut — all present on any Linux distro.
# ─────────────────────────────────────────────────────────────────────────────

# Internal: parses one server block's text and sets VHOST_RESULT to a JSON
# object, or "" if the block has no useful log info.
_parse_nginx_server_block() {
  local block="$1"
  VHOST_RESULT=""

  # ── server_name ────────────────────────────────────────────────────────
  local domain="(default)"
  local is_default="true"

  local sn_line
  sn_line=$(echo "$block" | grep -m1 -E '^\s*server_name\s+' \
    | sed 's/^\s*server_name\s*//' | tr -d ';' | xargs 2>/dev/null)

  if [ -n "$sn_line" ]; then
    # server_name line may have multiple names: take first non-wildcard one
    for token in $sn_line; do
      if [ "$token" != "_" ] && [ "$token" != "localhost" ] && [ -n "$token" ]; then
        domain="$token"
        is_default="false"
        break
      fi
    done
  fi

  # ── access_log ─────────────────────────────────────────────────────────
  local access_log=""
  local al_line
  al_line=$(echo "$block" | grep -m1 -E '^\s*access_log\s+' \
    | sed 's/^\s*access_log\s*//' | tr -d ';' | xargs 2>/dev/null)
  if [ -n "$al_line" ]; then
    # First token is path, second (optional) is log format name — ignore it
    local al_path
    al_path=$(echo "$al_line" | cut -d' ' -f1)
    [ "$al_path" != "off" ] && [ -n "$al_path" ] && access_log="$al_path"
  fi

  # ── error_log ──────────────────────────────────────────────────────────
  local error_log=""
  local el_line
  el_line=$(echo "$block" | grep -m1 -E '^\s*error_log\s+' \
    | sed 's/^\s*error_log\s*//' | tr -d ';' | xargs 2>/dev/null)
  if [ -n "$el_line" ]; then
    local el_path
    el_path=$(echo "$el_line" | cut -d' ' -f1)
    [ "$el_path" != "off" ] && [ -n "$el_path" ] && error_log="$el_path"
  fi

  # Skip blocks with no log info at all (usually catch-all or include stubs)
  if [ -z "$access_log" ] && [ -z "$error_log" ] && [ "$is_default" = "true" ]; then
    return 0
  fi

  # Minimal JSON escaping (backslash and double-quote in paths)
  access_log="${access_log//\\/\\\\}"; access_log="${access_log//\"/\\\"}"
  error_log="${error_log//\\/\\\\}";   error_log="${error_log//\"/\\\"}"
  domain="${domain//\"/\\\"}"

  VHOST_RESULT="{\"domain\":\"${domain}\",\"access_log\":\"${access_log}\",\"error_log\":\"${error_log}\",\"is_default\":${is_default}}"
}

# Public: scans all Nginx config files and returns a JSON array of virtual hosts.
# Output format:
#   [{"domain":"shop.example.com","access_log":"/var/log/nginx/shop.access.log","error_log":"/var/log/nginx/shop.error.log","is_default":false}, ...]
detect_nginx_vhosts() {
  local json_array="["
  local first=true
  local config_files=()

  # Scan standard Nginx config locations
  for dir in /etc/nginx/sites-enabled /etc/nginx/conf.d; do
    [ -d "$dir" ] || continue
    while IFS= read -r -d '' f; do
      config_files+=("$f")
    done < <(find "$dir" -maxdepth 1 \( -type f -o -type l \) -print0 2>/dev/null)
  done
  # Also include main config as fallback
  [ -f /etc/nginx/nginx.conf ] && config_files+=("/etc/nginx/nginx.conf")

  for cfg_file in "${config_files[@]}"; do
    [ -r "$cfg_file" ] || continue

    # Read file line by line, track brace depth to extract server { } blocks
    local in_server=false
    local depth=0
    local block=""

    while IFS= read -r line || [ -n "$line" ]; do
      # Strip inline comments before processing
      line="${line%%#*}"

      # Look for 'server {' at the top level (depth 0)
      if [ "$in_server" = false ] && [ "$depth" -eq 0 ]; then
        if echo "$line" | grep -qE '^\s*server\s*\{?\s*$|^\s*server\s+\{'; then
          in_server=true
          block=""
          local o c
          o=$(echo "$line" | tr -cd '{' | wc -c)
          c=$(echo "$line" | tr -cd '}' | wc -c)
          depth=$(( depth + o - c ))
          continue
        fi
      fi

      if [ "$in_server" = true ]; then
        block="${block}"$'\n'"${line}"

        local o c
        o=$(echo "$line" | tr -cd '{' | wc -c)
        c=$(echo "$line" | tr -cd '}' | wc -c)
        depth=$(( depth + o - c ))

        # When depth hits 0 we've closed the server block
        if [ "$depth" -le 0 ]; then
          _parse_nginx_server_block "$block"
          if [ -n "$VHOST_RESULT" ]; then
            $first || json_array+=","
            json_array+="$VHOST_RESULT"
            first=false
          fi
          in_server=false
          depth=0
          block=""
        fi
      fi
    done < "$cfg_file"
  done

  json_array+="]"
  echo "$json_array"
}

# ─────────────────────────────────────────────────────────────────────────────
# Apache virtual host detection — mirrors the nginx detector for CustomLog and
# ErrorLog paths in <VirtualHost> blocks.
# ─────────────────────────────────────────────────────────────────────────────

_apache_expand_log_path() {
  local path="$1"
  path="${path//\$\{APACHE_LOG_DIR\}/\/var\/log\/apache2}"
  path="${path//\$\{SERVER_LOG_ROOT\}/\/var\/log\/httpd}"
  path="${path//\"/}"
  echo "$path"
}

_parse_apache_vhost_block() {
  local block="$1"
  VHOST_RESULT=""

  local domain="(default)"
  local is_default="true"
  local sn_line
  sn_line=$(echo "$block" | grep -m1 -E '^\s*ServerName\s+' \
    | sed 's/^\s*ServerName\s*//' | xargs 2>/dev/null)
  if [ -n "$sn_line" ]; then
    domain=$(echo "$sn_line" | cut -d' ' -f1)
    is_default="false"
  fi

  local access_log=""
  local al_line
  al_line=$(echo "$block" | grep -m1 -E '^\s*CustomLog\s+' \
    | sed 's/^\s*CustomLog\s*//' | xargs 2>/dev/null)
  if [ -n "$al_line" ]; then
    access_log=$(_apache_expand_log_path "$(echo "$al_line" | cut -d' ' -f1)")
  fi

  local error_log=""
  local el_line
  el_line=$(echo "$block" | grep -m1 -E '^\s*ErrorLog\s+' \
    | sed 's/^\s*ErrorLog\s*//' | xargs 2>/dev/null)
  if [ -n "$el_line" ]; then
    error_log=$(_apache_expand_log_path "$(echo "$el_line" | cut -d' ' -f1)")
  fi

  if [ -z "$access_log" ] && [ -z "$error_log" ] && [ "$is_default" = "true" ]; then
    return 0
  fi

  access_log="${access_log//\\/\\\\}"; access_log="${access_log//\"/\\\"}"
  error_log="${error_log//\\/\\\\}";   error_log="${error_log//\"/\\\"}"
  domain="${domain//\"/\\\"}"

  VHOST_RESULT="{\"domain\":\"${domain}\",\"access_log\":\"${access_log}\",\"error_log\":\"${error_log}\",\"is_default\":${is_default}}"
}

detect_apache_vhosts() {
  local json_array="["
  local first=true
  local config_files=()

  for dir in /etc/apache2/sites-enabled /etc/apache2/conf-enabled /etc/httpd/conf.d; do
    [ -d "$dir" ] || continue
    while IFS= read -r -d '' f; do
      config_files+=("$f")
    done < <(find "$dir" -maxdepth 1 \( -type f -o -type l \) -print0 2>/dev/null)
  done
  [ -f /etc/apache2/apache2.conf ] && config_files+=("/etc/apache2/apache2.conf")
  [ -f /etc/httpd/conf/httpd.conf ] && config_files+=("/etc/httpd/conf/httpd.conf")

  for cfg_file in "${config_files[@]}"; do
    [ -r "$cfg_file" ] || continue
    local in_vhost=false
    local block=""

    while IFS= read -r line || [ -n "$line" ]; do
      line="${line%%#*}"
      if [ "$in_vhost" = false ] && echo "$line" | grep -qiE '^\s*<VirtualHost\b'; then
        in_vhost=true
        block=""
        continue
      fi

      if [ "$in_vhost" = true ]; then
        if echo "$line" | grep -qiE '^\s*</VirtualHost>'; then
          _parse_apache_vhost_block "$block"
          if [ -n "$VHOST_RESULT" ]; then
            $first || json_array+=","
            json_array+="$VHOST_RESULT"
            first=false
          fi
          in_vhost=false
          block=""
        else
          block="${block}"$'\n'"${line}"
        fi
      fi
    done < "$cfg_file"
  done

  json_array+="]"
  echo "$json_array"
}

# ---------- NGINX ----------
discover_nginx() {
  local UNIT=$1
  info "  Discovering nginx..."
  local PORT; PORT=$(get_unit_port "${UNIT}.service" || echo "80")
  local PATHS=""

  if command -v nginx &>/dev/null; then
    local ACCESS ERROR
    ACCESS=$(nginx -T 2>/dev/null \
      | grep -oP '(?<=access_log\s)[^\s;]+' | grep -v 'off' | head -10 || true)
    ERROR=$(nginx -T 2>/dev/null \
      | grep -oP '(?<=error_log\s)[^\s;]+' | grep -v 'off' | head -10 || true)
    PATHS="${ACCESS}"$'\n'"${ERROR}"
  fi
  [ -d /var/log/nginx ] && PATHS="${PATHS}"$'\n'"/var/log/nginx/access.log"$'\n'"/var/log/nginx/error.log"
  PATHS=$(echo "$PATHS" | sort -u | filter_existing)

  LOG_FORMAT["nginx"]="nginx_combined"
  local VERSION=""
  if command -v nginx &>/dev/null; then
    VERSION=$(nginx -v 2>&1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || true)
  fi
  [ -z "$VERSION" ] && VERSION=$(get_unit_version "${UNIT}.service" || echo "")
  local NOTES="port=${PORT}"
  local STATUS="no_logs"
  [ -n "$PATHS" ] && STATUS="logs_found"
  unit_uses_journald "$UNIT" && NOTES="${NOTES} | also_logs_to=journald"
  
  # Phase 3: Parse Nginx virtual hosts for domain-aware config
  local VHOSTS_JSON
  VHOSTS_JSON=$(detect_nginx_vhosts)
  WEBSERVER_VHOSTS_JSON["nginx"]="$VHOSTS_JSON"
  
  register_service "nginx" "$PATHS" "$STATUS" "$PORT" "$NOTES" "$VERSION"
}

# ---------- APACHE ----------
discover_apache() {
  local UNIT=$1
  info "  Discovering apache..."
  local PORT; PORT=$(get_unit_port "${UNIT}.service" || echo "")
  if [ -z "$PORT" ]; then
    PORT=$(grep -oP '(?<=^Listen\s)\d+' /etc/apache2/ports.conf 2>/dev/null \
      | grep -v '443' | head -1 || echo "80")
  fi

  local PATHS=""
  if command -v apache2ctl &>/dev/null || command -v apachectl &>/dev/null; then
    local CMD; command -v apache2ctl &>/dev/null && CMD=apache2ctl || CMD=apachectl
    local AOUT; AOUT=$($CMD -S 2>/dev/null || true)
    local ELOGS ALOGS
    ELOGS=$(echo "$AOUT" | grep -oP '(?<=error log: ).*' || true)
    ALOGS=$(echo "$AOUT" | grep -oP '(?<=logfile: ).*' || true)
    PATHS="${ELOGS}"$'\n'"${ALOGS}"
  fi
  [ -d /var/log/apache2 ] && PATHS="${PATHS}"$'\n'"/var/log/apache2/access.log"$'\n'"/var/log/apache2/error.log"
  [ -d /var/log/httpd ]   && PATHS="${PATHS}"$'\n'"/var/log/httpd/access_log"$'\n'"/var/log/httpd/error_log"
  PATHS=$(echo "$PATHS" | sort -u | filter_existing)

  LOG_FORMAT["apache"]="apache_combined"
  local STATUS="no_logs"
  [ -n "$PATHS" ] && STATUS="logs_found"
  local VERSION=""
  if command -v apache2 &>/dev/null; then
    VERSION=$(apache2 -v 2>&1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || true)
  elif command -v httpd &>/dev/null; then
    VERSION=$(httpd -v 2>&1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || true)
  fi
  [ -z "$VERSION" ] && VERSION=$(get_unit_version "${UNIT}.service" || echo "")
  local NOTES="port=${PORT}"
  unit_uses_journald "$UNIT" && NOTES="${NOTES} | also_logs_to=journald"
  local VHOSTS_JSON
  VHOSTS_JSON=$(detect_apache_vhosts)
  WEBSERVER_VHOSTS_JSON["apache"]="$VHOSTS_JSON"
  register_service "apache" "$PATHS" "$STATUS" "$PORT" "$NOTES" "$VERSION"
}

# ---------- POSTGRESQL ----------
discover_postgresql() {
  local UNIT=$1
  info "  Discovering postgresql ($UNIT)..."

  local VERSION; VERSION=$(echo "$UNIT" | grep -oP '(?<=@)\d+' || echo "")
  local CLUSTER; CLUSTER=$(echo "$UNIT" | grep -oP '(?<=@\d{1,2}-).*' || echo "main")
  local SAFE_NAME="postgresql"
  [ -n "$VERSION" ] && SAFE_NAME="postgresql${VERSION}"

  local PORT; PORT=$(get_unit_port "${UNIT}.service" || echo "5432")

  local SERVER_VERSION
  SERVER_VERSION=$(sudo -u postgres psql -p "${PORT}" -tAc "SHOW server_version;" 2>/dev/null | tr -d ' ' || true)
  [ -n "$SERVER_VERSION" ] && VERSION="$SERVER_VERSION"

  local LOG_DIR DATA_DIR LOG_COLLECTOR
  LOG_DIR=$(sudo -u postgres psql -p "${PORT}" -tAc "SHOW log_directory;" 2>/dev/null \
    | tr -d ' ') || true
  DATA_DIR=$(sudo -u postgres psql -p "${PORT}" -tAc "SHOW data_directory;" 2>/dev/null \
    | tr -d ' ') || true
  LOG_COLLECTOR=$(sudo -u postgres psql -p "${PORT}" -tAc "SHOW logging_collector;" 2>/dev/null \
    | tr -d ' ') || true

  local PATHS=""
  if [ "$LOG_COLLECTOR" = "on" ] && [ -n "$LOG_DIR" ] && [ -n "$DATA_DIR" ]; then
    local ABS_LOG_DIR
    if [[ "$LOG_DIR" == /* ]]; then
      ABS_LOG_DIR="$LOG_DIR"
    else
      ABS_LOG_DIR="${DATA_DIR}/${LOG_DIR}"
    fi
    PATHS="${ABS_LOG_DIR}/*.log"
    log "    logging_collector=on, logs at ${ABS_LOG_DIR}"
  fi
  [ -d /var/log/postgresql ] && PATHS="${PATHS}"$'\n'"/var/log/postgresql/*.log"

  LOG_FORMAT["$SAFE_NAME"]="pg_csv"
  local STATUS="no_logs"
  [ -n "$PATHS" ] && STATUS="logs_found"
  local NOTES="port=${PORT} version=${VERSION} cluster=${CLUSTER} logging_collector=${LOG_COLLECTOR:-unknown}"
  [ "$LOG_COLLECTOR" = "off" ] && NOTES="${NOTES} | logs_via=journald"
  register_service "$SAFE_NAME" "$PATHS" "$STATUS" "$PORT" "$NOTES" "$VERSION"
  SVC_PORT[$SAFE_NAME]="$PORT"
}

# ---------- MYSQL / MARIADB ----------
discover_mysql() {
  local UNIT=$1
  local SVC_NAME=${2:-mysql}
  info "  Discovering ${SVC_NAME} ($UNIT)..."
  local PORT; PORT=$(get_unit_port "${UNIT}.service" || echo "3306")
  local PATHS=""
  local VERSION=""

  if command -v mysql &>/dev/null; then
    local LOG_FILE ERR_LOG SLOW_LOG
    LOG_FILE=$(mysql -u root --batch --skip-column-names \
      -e "SELECT @@general_log_file;" 2>/dev/null | tr -d ' ') || true
    ERR_LOG=$(mysql -u root --batch --skip-column-names \
      -e "SELECT @@log_error;" 2>/dev/null | tr -d ' ') || true
    SLOW_LOG=$(mysql -u root --batch --skip-column-names \
      -e "SELECT @@slow_query_log_file;" 2>/dev/null | tr -d ' ') || true
    VERSION=$(mysql -u root --batch --skip-column-names \
      -e "SELECT @@version;" 2>/dev/null | tr -d ' ') || true
    [ -f "$LOG_FILE" ] && PATHS="$LOG_FILE"
    [ -f "$ERR_LOG"  ] && PATHS="${PATHS}"$'\n'"$ERR_LOG"
    [ -f "$SLOW_LOG" ] && PATHS="${PATHS}"$'\n'"$SLOW_LOG"
  fi
  [ -f /var/log/mysql/error.log ]     && PATHS="${PATHS}"$'\n'"/var/log/mysql/error.log"
  [ -f /var/log/mysql/mysql.log ]     && PATHS="${PATHS}"$'\n'"/var/log/mysql/mysql.log"
  [ -f /var/log/mariadb/mariadb.log ] && PATHS="${PATHS}"$'\n'"/var/log/mariadb/mariadb.log"
  [ -f /var/log/mysqld.log ]          && PATHS="${PATHS}"$'\n'"/var/log/mysqld.log"
  PATHS=$(echo "$PATHS" | sort -u | filter_existing)

  LOG_FORMAT["$SVC_NAME"]="mysql_log"
  local STATUS="no_logs"
  [ -n "$PATHS" ] && STATUS="logs_found"
  [ -z "$VERSION" ] && VERSION=$(get_unit_version "${UNIT}.service" || echo "")
  local NOTES="port=${PORT}"
  unit_uses_journald "$UNIT" && NOTES="${NOTES} | also_logs_to=journald"
  register_service "$SVC_NAME" "$PATHS" "$STATUS" "$PORT" "$NOTES" "$VERSION"
}

# ---------- MONGODB ----------
discover_mongodb() {
  local UNIT=$1
  info "  Discovering mongodb ($UNIT)..."
  local PORT; PORT=$(get_unit_port "${UNIT}.service" || echo "27017")
  local PATHS=""

  for CONF in /etc/mongod.conf /etc/mongodb.conf; do
    if [ -f "$CONF" ]; then
      local LP
      LP=$(grep -oP '(?<=path:\s).*' "$CONF" 2>/dev/null | head -1 | tr -d ' ') || true
      [ -f "$LP" ] && PATHS="$LP"
    fi
  done
  [ -f /var/log/mongodb/mongod.log ] && PATHS="${PATHS}"$'\n'"/var/log/mongodb/mongod.log"
  [ -f /var/log/mongod.log ]         && PATHS="${PATHS}"$'\n'"/var/log/mongod.log"
  PATHS=$(echo "$PATHS" | sort -u | filter_existing)

  LOG_FORMAT["mongodb"]="json"
  local STATUS="no_logs"
  [ -n "$PATHS" ] && STATUS="logs_found"
  local VERSION; VERSION=$(get_unit_version "${UNIT}.service" || echo "")
  local NOTES="port=${PORT}"
  unit_uses_journald "$UNIT" && NOTES="${NOTES} | also_logs_to=journald"
  register_service "mongodb" "$PATHS" "$STATUS" "$PORT" "$NOTES" "$VERSION"
}

# ---------- REDIS ----------
discover_redis() {
  local UNIT=$1
  info "  Discovering redis ($UNIT)..."
  local PORT; PORT=$(get_unit_port "${UNIT}.service" || echo "6379")
  local PATHS=""

  if command -v redis-cli &>/dev/null; then
    local LF
    LF=$(redis-cli -p "${PORT}" CONFIG GET logfile 2>/dev/null | tail -1 | tr -d ' ') || true
    [ -n "$LF" ] && [ "$LF" != '""' ] && [ -f "$LF" ] && PATHS="$LF"
  fi
  for CONF in /etc/redis/redis.conf /etc/redis.conf; do
    if [ -f "$CONF" ]; then
      local LP
      LP=$(grep -oP '(?<=^logfile\s).*' "$CONF" 2>/dev/null | tr -d '"' | tr -d ' ') || true
      [ -f "$LP" ] && PATHS="${PATHS}"$'\n'"$LP"
    fi
  done
  [ -f /var/log/redis/redis-server.log ] && PATHS="${PATHS}"$'\n'"/var/log/redis/redis-server.log"
  [ -f /var/log/redis.log ]              && PATHS="${PATHS}"$'\n'"/var/log/redis.log"
  PATHS=$(echo "$PATHS" | sort -u | filter_existing)

  LOG_FORMAT["redis"]="redis_log"
  local STATUS="no_logs"
  [ -n "$PATHS" ] && STATUS="logs_found"
  local VERSION; VERSION=$(get_unit_version "${UNIT}.service" || echo "")
  local NOTES="port=${PORT}"
  unit_uses_journald "$UNIT" && NOTES="${NOTES} | also_logs_to=journald"
  register_service "redis" "$PATHS" "$STATUS" "$PORT" "$NOTES" "$VERSION"
}

# ---------- PHP-FPM ----------
discover_phpfpm() {
  local UNIT=$1
  info "  Discovering php-fpm ($UNIT)..."
  local PATHS=""

  # Read error log from main config (one per FPM master, not per pool)
  for CONF in /etc/php/*/fpm/php-fpm.conf /etc/php-fpm.conf; do
    [ -f "$CONF" ] || continue
    local E
    E=$(grep -oP '(?<=^error_log\s*=\s*).*' "$CONF" 2>/dev/null | tr -d ' ') || true
    [ -f "$E" ] && PATHS="${PATHS}"$'\n'"$E"
  done

  # Read per-pool log files from each pool config
  # Resolve $pool variable to the actual pool name (filename without .conf)
  local LISTEN_ALL=""
  local POOL_COUNT=0
  for POOL in /etc/php/*/fpm/pool.d/*.conf /etc/php-fpm.d/*.conf; do
    [ -f "$POOL" ] || continue
    local POOL_NAME
    POOL_NAME=$(basename "$POOL" .conf)
    local S A L
    S=$(grep -oP '(?<=^slowlog\s*=\s*).*'    "$POOL" 2>/dev/null | tr -d ' ') || true
    A=$(grep -oP '(?<=^access\.log\s*=\s*).*' "$POOL" 2>/dev/null | tr -d ' ') || true
    L=$(grep -oP '(?<=^listen\s*=\s*).*'     "$POOL" 2>/dev/null | head -1 | tr -d ' ') || true

    # Resolve $pool variable to actual pool name
    S="${S//\$pool/$POOL_NAME}"
    A="${A//\$pool/$POOL_NAME}"

    [ -f "$S" ] && PATHS="${PATHS}"$'\n'"$S"
    [ -f "$A" ] && PATHS="${PATHS}"$'\n'"$A"
    if [ -n "$L" ]; then
      [ -n "$LISTEN_ALL" ] && LISTEN_ALL="${LISTEN_ALL}, "
      LISTEN_ALL="${LISTEN_ALL}${L}"
      POOL_COUNT=$((POOL_COUNT + 1))
    fi
  done

  # Fallback: catch any log files in the common log directory
  [ -d /var/log/php-fpm ] && PATHS="${PATHS}"$'\n'"/var/log/php-fpm/*.log"
  PATHS=$(echo "$PATHS" | sort -u | filter_existing)

  LOG_FORMAT["phpfpm"]="phpfpm_log"
  local STATUS="no_logs"
  [ -n "$PATHS" ] && STATUS="logs_found"
  local VERSION; VERSION=$(get_unit_version "${UNIT}.service" || echo "")
  local NOTES="php-fpm"
  if [ -n "$LISTEN_ALL" ]; then
    NOTES="${NOTES} | listen=${LISTEN_ALL}"
    [ "$POOL_COUNT" -gt 1 ] && NOTES="${NOTES} | pools=${POOL_COUNT}"
  fi
  unit_uses_journald "$UNIT" && NOTES="${NOTES} | also_logs_to=journald"
  register_service "phpfpm" "$PATHS" "$STATUS" "" "$NOTES" "$VERSION"
}

# ---------- ELASTICSEARCH ----------
discover_elasticsearch() {
  local UNIT=$1
  info "  Discovering elasticsearch..."
  local PORT; PORT=$(get_unit_port "${UNIT}.service" || echo "9200")
  local PATHS=""

  local CONF="/etc/elasticsearch/elasticsearch.yml"
  if [ -f "$CONF" ]; then
    local LD
    LD=$(grep -oP '(?<=^path\.logs:\s).*' "$CONF" 2>/dev/null | tr -d ' ') || true
    [ -d "$LD" ] && PATHS="${LD}/*.log"
  fi
  [ -d /var/log/elasticsearch ] && PATHS="${PATHS}"$'\n'"/var/log/elasticsearch/*.log"
  PATHS=$(echo "$PATHS" | sort -u | filter_existing)

  local STATUS="no_logs"
  [ -n "$PATHS" ] && STATUS="logs_found"
  local VERSION; VERSION=$(get_unit_version "${UNIT}.service" || echo "")
  local NOTES="port=${PORT}"
  unit_uses_journald "$UNIT" && NOTES="${NOTES} | also_logs_to=journald"
  register_service "elasticsearch" "$PATHS" "$STATUS" "$PORT" "$NOTES" "$VERSION"
}

# ---------- RABBITMQ ----------
discover_rabbitmq() {
  local UNIT=$1
  info "  Discovering rabbitmq..."
  local PATHS=""

  [ -d /var/log/rabbitmq ] && PATHS="/var/log/rabbitmq/*.log"
  if command -v rabbitmq-diagnostics &>/dev/null; then
    local LP
    LP=$(rabbitmq-diagnostics log_location 2>/dev/null \
      | grep -oP '(?<=Log file: ).*' | head -1) || true
    [ -f "$LP" ] && PATHS="${PATHS}"$'\n'"$LP"
  fi
  PATHS=$(echo "$PATHS" | sort -u | filter_existing)

  local STATUS="no_logs"
  [ -n "$PATHS" ] && STATUS="logs_found"
  local VERSION; VERSION=$(get_unit_version "${UNIT}.service" || echo "")
  local NOTES="management_port=$(get_unit_port "${UNIT}.service" || echo "15672")"
  unit_uses_journald "$UNIT" && NOTES="${NOTES} | also_logs_to=journald"
  register_service "rabbitmq" "$PATHS" "$STATUS" "" "$NOTES" "$VERSION"
}

# ---------- KAFKA ----------
discover_kafka() {
  local UNIT=$1
  info "  Discovering kafka..."
  local PATHS=""

  for D in /var/log/kafka /opt/kafka/logs /usr/local/kafka/logs /kafka/logs; do
    [ -d "$D" ] && PATHS="${PATHS}"$'\n'"${D}/*.log"
  done
  local KAFKA_PID
  KAFKA_PID=$(pgrep -f "kafka.Kafka" 2>/dev/null | head -1) || true
  if [ -n "$KAFKA_PID" ]; then
    local KLD
    KLD=$(cat /proc/${KAFKA_PID}/environ 2>/dev/null \
      | tr '\0' '\n' | grep 'LOG_DIR=' | cut -d= -f2) || true
    [ -d "$KLD" ] && PATHS="${PATHS}"$'\n'"${KLD}/*.log"
  fi
  PATHS=$(echo "$PATHS" | sort -u | filter_existing)

  local STATUS="no_logs"
  [ -n "$PATHS" ] && STATUS="logs_found"
  local VERSION; VERSION=$(get_unit_version "${UNIT}.service" || echo "")
  local NOTES=""
  unit_uses_journald "$UNIT" && NOTES="logs_to=journald"
  register_service "kafka" "$PATHS" "$STATUS" "" "$NOTES" "$VERSION"
}

# ---------- HAPROXY ----------
discover_haproxy() {
  local UNIT=$1
  info "  Discovering haproxy..."
  local PORT; PORT=$(get_unit_port "${UNIT}.service" || echo "")
  local PATHS=""

  for CONF in /etc/haproxy/haproxy.cfg; do
    [ -f "$CONF" ] || continue
    local LP
    LP=$(grep -oP '(?<=log\s)/[^\s]+' "$CONF" 2>/dev/null | head -3) || true
    PATHS="$LP"
  done
  [ -f /var/log/haproxy.log ] && PATHS="${PATHS}"$'\n'"/var/log/haproxy.log"
  PATHS=$(echo "$PATHS" | sort -u | filter_existing)

  local STATUS="no_logs"
  [ -n "$PATHS" ] && STATUS="logs_found"
  local VERSION; VERSION=$(get_unit_version "${UNIT}.service" || echo "")
  local NOTES="port=${PORT}"
  unit_uses_journald "$UNIT" && NOTES="${NOTES} | also_logs_to=journald"
  register_service "haproxy" "$PATHS" "$STATUS" "$PORT" "$NOTES" "$VERSION"
}

# ---------- TRAEFIK ----------
discover_traefik() {
  local UNIT=$1
  info "  Discovering traefik..."
  local PORT; PORT=$(get_unit_port "${UNIT}.service" || echo "")
  local PATHS=""

  for CONF in /etc/traefik/traefik.yml /etc/traefik/traefik.toml; do
    [ -f "$CONF" ] || continue
    local LP
    LP=$(grep -oP '(?<=filePath:\s).*' "$CONF" 2>/dev/null | head -2 | tr -d '"') || true
    PATHS="${PATHS}"$'\n'"$LP"
  done
  [ -f /var/log/traefik/traefik.log ] && PATHS="${PATHS}"$'\n'"/var/log/traefik/traefik.log"
  [ -f /var/log/traefik/access.log ]  && PATHS="${PATHS}"$'\n'"/var/log/traefik/access.log"
  PATHS=$(echo "$PATHS" | sort -u | filter_existing)

  local STATUS="no_logs"
  [ -n "$PATHS" ] && STATUS="logs_found"
  local VERSION; VERSION=$(get_unit_version "${UNIT}.service" || echo "")
  local NOTES="port=${PORT}"
  unit_uses_journald "$UNIT" && NOTES="${NOTES} | also_logs_to=journald"
  register_service "traefik" "$PATHS" "$STATUS" "$PORT" "$NOTES" "$VERSION"
}

# ---------- VAULT ----------
discover_vault() {
  local UNIT=$1
  info "  Discovering vault..."
  local PATHS=""

  for CONF in /etc/vault.d/vault.hcl /etc/vault/vault.hcl; do
    [ -f "$CONF" ] || continue
    local LP
    LP=$(grep -oP '(?<=file_path\s*=\s*")[^"]+' "$CONF" 2>/dev/null | head -1) || true
    [ -f "$LP" ] && PATHS="$LP"
  done
  [ -f /var/log/vault/vault.log ] && PATHS="${PATHS}"$'\n'"/var/log/vault/vault.log"
  PATHS=$(echo "$PATHS" | sort -u | filter_existing)

  local STATUS="no_logs"
  [ -n "$PATHS" ] && STATUS="logs_found"
  local VERSION; VERSION=$(get_unit_version "${UNIT}.service" || echo "")
  local NOTES=""
  unit_uses_journald "$UNIT" && NOTES="logs_to=journald"
  register_service "vault" "$PATHS" "$STATUS" "" "$NOTES" "$VERSION"
}

# ---------- CONSUL ----------
discover_consul() {
  local UNIT=$1
  info "  Discovering consul..."
  local PATHS=""

  for CONF in /etc/consul.d/consul.hcl /etc/consul/consul.hcl; do
    [ -f "$CONF" ] || continue
    local LP
    LP=$(grep -oP '(?<=log_file\s*=\s*")[^"]+' "$CONF" 2>/dev/null | head -1) || true
    [ -f "$LP" ] && PATHS="$LP"
  done
  [ -f /var/log/consul/consul.log ] && PATHS="${PATHS}"$'\n'"/var/log/consul/consul.log"
  PATHS=$(echo "$PATHS" | sort -u | filter_existing)

  local STATUS="no_logs"
  [ -n "$PATHS" ] && STATUS="logs_found"
  local VERSION; VERSION=$(get_unit_version "${UNIT}.service" || echo "")
  local NOTES=""
  unit_uses_journald "$UNIT" && NOTES="logs_to=journald"
  register_service "consul" "$PATHS" "$STATUS" "" "$NOTES" "$VERSION"
}

# ---------- MEMCACHED ----------
discover_memcached() {
  local UNIT=$1
  info "  Discovering memcached..."
  local PORT; PORT=$(get_unit_port "${UNIT}.service" || echo "11211")
  local PATHS=""
  PATHS=$(collect_proc_logs "${UNIT}.service")
  local STATUS="no_logs"
  [ -n "$PATHS" ] && STATUS="logs_found"
  local VERSION; VERSION=$(get_unit_version "${UNIT}.service" || echo "")
  local NOTES="port=${PORT}"
  unit_uses_journald "$UNIT" && NOTES="${NOTES} | also_logs_to=journald"
  register_service "memcached" "$PATHS" "$STATUS" "$PORT" "$NOTES" "$VERSION"
}

# ---------- CADDY ----------
discover_caddy() {
  local UNIT=$1
  info "  Discovering caddy..."
  local PORT; PORT=$(get_unit_port "${UNIT}.service" || echo "")
  local PATHS=""

  for CONF in /etc/caddy/Caddyfile; do
    [ -f "$CONF" ] || continue
    local LP
    LP=$(grep -oP '(?<=output file\s).*' "$CONF" 2>/dev/null | head -2 | tr -d '"') || true
    PATHS="${PATHS}"$'\n'"$LP"
  done
  [ -d /var/log/caddy ] && PATHS="${PATHS}"$'\n'"/var/log/caddy/*.log"
  PATHS=$(echo "$PATHS" | sort -u | filter_existing)

  local STATUS="no_logs"
  [ -n "$PATHS" ] && STATUS="logs_found"
  local VERSION; VERSION=$(get_unit_version "${UNIT}.service" || echo "")
  local NOTES="port=${PORT}"
  unit_uses_journald "$UNIT" && NOTES="${NOTES} | also_logs_to=journald"
  register_service "caddy" "$PATHS" "$STATUS" "$PORT" "$NOTES" "$VERSION"
}

# ---------- CLICKHOUSE ----------
discover_clickhouse() {
  local UNIT=$1
  info "  Discovering clickhouse..."
  local PATHS=""

  local CONF="/etc/clickhouse-server/config.xml"
  if [ -f "$CONF" ]; then
    local LP
    LP=$(grep -oP '(?<=<log>)[^<]+' "$CONF" 2>/dev/null | head -1) || true
    [ -f "$LP" ] && PATHS="$LP"
  fi
  [ -d /var/log/clickhouse-server ] && PATHS="${PATHS}"$'\n'"/var/log/clickhouse-server/*.log"
  PATHS=$(echo "$PATHS" | sort -u | filter_existing)

  local STATUS="no_logs"
  [ -n "$PATHS" ] && STATUS="logs_found"
  local VERSION; VERSION=$(get_unit_version "${UNIT}.service" || echo "")
  local NOTES="port=8123"
  unit_uses_journald "$UNIT" && NOTES="${NOTES} | also_logs_to=journald"
  register_service "clickhouse" "$PATHS" "$STATUS" "8123" "$NOTES" "$VERSION"
}

# ---------- CASSANDRA ----------
discover_cassandra() {
  local UNIT=$1
  info "  Discovering cassandra..."
  local PATHS=""
  [ -d /var/log/cassandra ] && PATHS="/var/log/cassandra/*.log"
  PATHS=$(echo "$PATHS" | sort -u | filter_existing)
  local STATUS="no_logs"
  [ -n "$PATHS" ] && STATUS="logs_found"
  local VERSION; VERSION=$(get_unit_version "${UNIT}.service" || echo "")
  local NOTES=""
  unit_uses_journald "$UNIT" && NOTES="logs_to=journald"
  register_service "cassandra" "$PATHS" "$STATUS" "" "$NOTES" "$VERSION"
}

# ---- Unit-to-discoverer map ----
unit_canonical() {
  local UNIT=$1
  case "$UNIT" in
    nginx)                          echo "nginx" ;;
    apache2|httpd)                  echo "apache" ;;
    postgresql@*|postgresql)        echo "postgresql" ;;
    mysql|mysqld)                   echo "mysql" ;;
    mariadb)                        echo "mariadb" ;;
    mongod|mongodb)                 echo "mongodb" ;;
    redis|redis-server|redis@*)     echo "redis" ;;
    php*fpm*|php-fpm)               echo "phpfpm" ;;
    elasticsearch)                  echo "elasticsearch" ;;
    rabbitmq-server)                echo "rabbitmq" ;;
    kafka|kafka@*)                  echo "kafka" ;;
    haproxy)                        echo "haproxy" ;;
    traefik)                        echo "traefik" ;;
    vault)                          echo "vault" ;;
    consul)                         echo "consul" ;;
    memcached)                      echo "memcached" ;;
    caddy)                          echo "caddy" ;;
    clickhouse-server)              echo "clickhouse" ;;
    cassandra)                      echo "cassandra" ;;
    *)                              echo "" ;;
  esac
}

get_unit_port() {
  local UNIT=$1
  for PID in $(get_unit_pids "$UNIT" 2>/dev/null); do
    local PORT
    PORT=$(ss -tlnp 2>/dev/null \
      | grep "pid=${PID}," \
      | grep -oP '(?<=:)\d+(?=\s)' \
      | head -1)
    [ -n "$PORT" ] && echo "$PORT" && return
  done
}

get_unit_version() {
  local UNIT=$1
  local PID EXE OUT VERSION
  for PID in $(get_unit_pids "$UNIT" 2>/dev/null); do
    EXE=$(readlink -f "/proc/${PID}/exe" 2>/dev/null) || continue
    [ -x "$EXE" ] || continue
    OUT=$("$EXE" --version 2>&1 | head -1) || true
    [ -z "$OUT" ] && OUT=$("$EXE" -v 2>&1 | head -1) || true
    if [ -z "$OUT" ] && [[ "$EXE" == *nginx ]]; then
      OUT=$(nginx -v 2>&1 | head -1) || true
    fi
    VERSION=$(printf '%s' "$OUT" | grep -oE '[0-9]+([.][0-9]+)+' | head -1) || true
    [ -n "$VERSION" ] && echo "$VERSION" && return
  done
}

declare -A PROCESSED_CANONICALS=()

while IFS= read -r UNIT; do
  [ -z "$UNIT" ] && continue
  echo "$UNIT" | grep -qE "$IGNORE_UNITS" && continue
  CANONICAL=$(unit_canonical "$UNIT")
  if [ -n "$CANONICAL" ]; then
    case "$CANONICAL" in
      postgresql)
        discover_postgresql "$UNIT"
        ;;
      mysql|mariadb)
        [ "${PROCESSED_CANONICALS[$CANONICAL]+_}" ] && continue
        PROCESSED_CANONICALS[$CANONICAL]=1
        discover_mysql "$UNIT" "$CANONICAL"
        ;;
      *)
        [ "${PROCESSED_CANONICALS[$CANONICAL]+_}" ] && continue
        PROCESSED_CANONICALS[$CANONICAL]=1
        "discover_${CANONICAL}" "$UNIT"
        ;;
    esac
  else
    # Unknown service — try to find log files via open fd inspection
    PATHS=$(collect_proc_logs "${UNIT}.service" 2>/dev/null || true)
    SAFE=$(echo "$UNIT" | tr '[:upper:]' '[:lower:]' | tr -cs 'a-z0-9' '_' | sed 's/_$//')
    UNKNOWN_PORT=$(get_unit_port "${UNIT}.service" 2>/dev/null || echo "")
    UNKNOWN_VERSION=$(get_unit_version "${UNIT}.service" 2>/dev/null || echo "")
    local_status="no_logs"
    [ -n "$PATHS" ] && local_status="logs_found"
    local_notes="unknown service — auto-discovered port=${UNKNOWN_PORT:-?}"
    unit_uses_journald "$UNIT" && local_notes="${local_notes} | also_logs_to=journald"
    if [ -n "$PATHS" ] || unit_uses_journald "$UNIT"; then
      register_service "$SAFE" "$PATHS" "$local_status" "${UNKNOWN_PORT:-}" "$local_notes" "${UNKNOWN_VERSION:-}"
      LOG_FORMAT["$SAFE"]="unknown"
    fi
  fi
done <<< "$RUNNING_UNITS"

# ============================================
# PHASE 1b — DOCKER CONTAINER DISCOVERY
# ============================================
section "Phase 1b: Docker container discovery"

# ---- Helper: identify what service a container is running ----
# Strategy: prefer Docker/Compose labels, then image name.
# Avoid docker exec here: some minimal/distroless images don't include `ps`/`cat`,
# and Docker error output can corrupt generated YAML if accidentally captured.
identify_container_service() {
  local CID=$1
  local IMAGE=$2

  # 1. Check image name (fastest, most reliable for known infrastructure images)
  local IMG_BASE
  IMG_BASE=$(echo "$IMAGE" | sed 's|.*/||' | cut -d: -f1 | tr '[:upper:]' '[:lower:]')
  case "$IMG_BASE" in
    nginx)                                 echo "nginx" ; return ;;
    apache*|httpd)                         echo "apache" ; return ;;
    mysql)                                echo "mysql" ; return ;;
    mariadb)                              echo "mariadb" ; return ;;
    postgres*|postgresql*)                 echo "postgresql" ; return ;;
    redis*)                                echo "redis" ; return ;;
    mongo*)                                echo "mongodb" ; return ;;
    rabbitmq*)                             echo "rabbitmq" ; return ;;
    kafka*|confluentinc*)                  echo "kafka" ; return ;;
    elasticsearch*|elastic*)               echo "elasticsearch" ; return ;;
    clickhouse*)                           echo "clickhouse" ; return ;;
    vault*)                                echo "vault" ; return ;;
    consul*)                               echo "consul" ; return ;;
    memcached*)                            echo "memcached" ; return ;;
    traefik*)                              echo "traefik" ; return ;;
    haproxy*)                              echo "haproxy" ; return ;;
    caddy*)                                echo "caddy" ; return ;;
    cassandra*)                            echo "cassandra" ; return ;;
    node|node-*)                           echo "nodejs" ; return ;;
    python*)                               echo "python" ; return ;;
    ruby*|rails*)                          echo "ruby" ; return ;;
    php*)                                  echo "php" ; return ;;
    java*|openjdk*|eclipse-temurin*)       echo "java" ; return ;;
  esac

  # 2. Inspect configured command (helps for custom images, e.g. a client image that runs nginx)
  local CMDLINE CMDLINE_LC
  CMDLINE=$(docker inspect --format '{{.Path}} {{range .Args}}{{.}} {{end}}' "$CID" 2>/dev/null || true)
  CMDLINE=$(trim_one_line "$CMDLINE")
  CMDLINE_LC=$(echo "$CMDLINE" | tr '[:upper:]' '[:lower:]')
  case "$CMDLINE_LC" in
    *nginx*)    echo "nginx" ; return ;;
    *httpd*|*apache*) echo "apache" ; return ;;
    *postgres*) echo "postgresql" ; return ;;
    *redis-server*|*redis*) echo "redis" ; return ;;
    *mongod*)   echo "mongodb" ; return ;;
    *mysqld*)   echo "mysql" ; return ;;
    *mariadbd*) echo "mariadb" ; return ;;
    *loki*)     echo "loki" ; return ;;
    *tempo*)    echo "tempo" ; return ;;
    *vmselect*) echo "vmselect" ; return ;;
    *vminsert*) echo "vminsert" ; return ;;
    *vmstorage*) echo "vmstorage" ; return ;;
    *node*)     echo "nodejs" ; return ;;
    *python*|*gunicorn*|*uvicorn*) echo "python" ; return ;;
    *java*)     echo "java" ; return ;;
    *ruby*)     echo "ruby" ; return ;;
    *php*)      echo "php" ; return ;;
  esac

  # 3. docker-compose label (useful generic fallback, e.g. "obs-backend")
  local COMPOSE_SVC
  COMPOSE_SVC=$(docker inspect --format '{{ index .Config.Labels "com.docker.compose.service" }}' "$CID" 2>/dev/null || true)
  COMPOSE_SVC=$(trim_one_line "$COMPOSE_SVC")
  if [ -n "$COMPOSE_SVC" ] && [ "$COMPOSE_SVC" != "<no value>" ]; then
    printf '%s\n' "$COMPOSE_SVC"
    return
  fi

  echo "unknown"
}

declare -A CONTAINER_ID_TO_NAME=()

if [ -S /var/run/docker.sock ] && command -v docker &>/dev/null; then
  CONTAINERS=$(docker ps --format $'{{.ID}}\t{{.Names}}\t{{.Image}}' 2>/dev/null || true)

  while IFS=$'\t' read -r CID_SHORT CNAME CIMAGE; do
    [ -z "$CID_SHORT" ] && continue

    FULL_CID=$(docker inspect --format '{{.Id}}' "$CID_SHORT" 2>/dev/null || echo "")
    if [ -z "$FULL_CID" ]; then
      warn "  Could not resolve full ID for container ${CNAME} — skipping"
      continue
    fi
    CID="$FULL_CID"

    info "  Container: ${CNAME} (${CIMAGE})"
    CONTAINER_ID_TO_NAME[$CID]="$CNAME"

    # ---- Identify what service this container is running ----
    CONTAINER_SERVICE=$(identify_container_service "$CID_SHORT" "$CIMAGE")
    CONTAINER_SERVICE=$(trim_one_line "$CONTAINER_SERVICE")
    info "    Detected service: ${CONTAINER_SERVICE}"

    SAFE_CNAME=$(echo "${CNAME}" | tr '[:upper:]' '[:lower:]' | tr -cs 'a-z0-9' '_' | sed 's/_$//')

    # ----------------------------------------------------------------
    # CONTAINER LOGS — priority order:
    #   1. Bind-mounted volume log paths on the host (highest quality)
    #   2. Real-time stdout/stderr via docker logs (fifo/pipe approach)
    # ----------------------------------------------------------------

    # -- Source 1: Check for bind-mounted log volumes (preferred) --
    VOLUME_LOG_PATHS=$(docker inspect --format \
      '{{range .Mounts}}{{if eq .Type "bind"}}{{.Source}}:{{.Destination}}{{"\n"}}{{end}}{{end}}' \
      "$CID" 2>/dev/null | while IFS=':' read -r SRC DST; do
        [[ "$DST" == */log* ]] || [[ "$DST" == *.log ]] || \
        [[ "$DST" == */logs* ]] || [[ "$DST" == */logging* ]] || continue
        if [ -d "$SRC" ]; then
          echo "${SRC}/*.log"
        elif [ -f "$SRC" ]; then
          echo "$SRC"
        fi
      done 2>/dev/null) || true

    HAS_VOLUME_LOGS=false
    if [ -n "$VOLUME_LOG_PATHS" ]; then
      EXISTING_VOLUME_PATHS=$(echo "$VOLUME_LOG_PATHS" | filter_existing)
      if [ -n "$EXISTING_VOLUME_PATHS" ]; then
        HAS_VOLUME_LOGS=true
        VOLUME_PATHS_YAML=$(echo "$EXISTING_VOLUME_PATHS" | while IFS= read -r P; do
          [ -n "$P" ] && echo "      - $P"
        done)
        VOLUME_RECV="filelog/docker_volume_${SAFE_CNAME}"
        FILELOG_RECEIVERS_YAML="${FILELOG_RECEIVERS_YAML}
  ${VOLUME_RECV}:
    include:
${VOLUME_PATHS_YAML}
    include_file_path: true
    start_at: end
    operators:
      - type: json_parser
        on_error: send_quiet
      - type: copy
        from: attributes.s
        to: attributes.log_level
        if: 'attributes.s != nil'
        on_error: send_quiet
      - type: copy
        from: attributes.level
        to: attributes.log_level
        if: 'attributes.level != nil'
        on_error: send_quiet
      - type: copy
        from: attributes.severity
        to: attributes.log_level
        if: 'attributes.severity != nil'
        on_error: send_quiet
      - type: severity_parser
        parse_from: body
        on_error: send_quiet
    resource:
      service.name: $(yaml_quote "container_${CNAME}")
      container.id: $(yaml_quote "$CID")
      container.image: $(yaml_quote "$CIMAGE")
      container.service: $(yaml_quote "$CONTAINER_SERVICE")
      log.source: volume
      job: docker
"
        FILELOG_PIPELINE+=("${VOLUME_RECV}")
        log "    ${CNAME}: volume log receiver → ${VOLUME_RECV}"
      fi
    fi

    # -- Source 2: Real-time stdout/stderr via docker logs --
    # Set up for ALL containers regardless of volume logs.
    # Containers with volume logs get both receivers (volume=structured file, stdout=app output).
    # Containers without volume logs get stdout only.
    FIFO_PATH="/var/lib/otelcol/docker-logs/${SAFE_CNAME}.log"
    STDOUT_RECV="filelog/docker_stdout_${SAFE_CNAME}"

    FILELOG_RECEIVERS_YAML="${FILELOG_RECEIVERS_YAML}
  ${STDOUT_RECV}:
    include:
      - ${FIFO_PATH}
    include_file_path: true
    start_at: beginning
    operators:
      # Best-effort JSON parse for apps that log structured JSON (Loki, Tempo, etc.)
      # on_error: send_quiet ensures plain-text apps pass through unchanged with body intact
      - type: json_parser
        on_error: send_quiet
      - type: copy
        from: attributes.level
        to: attributes.log_level
        if: 'attributes.level != nil'
        on_error: send_quiet
      - type: copy
        from: attributes.severity
        to: attributes.log_level
        if: 'attributes.severity != nil'
        on_error: send_quiet
      - type: copy
        from: attributes.msg
        to: body
        if: 'attributes.msg != nil'
        on_error: send_quiet
    resource:
      service.name: $(yaml_quote "container_${CNAME}")
      container.id: $(yaml_quote "$CID")
      container.image: $(yaml_quote "$CIMAGE")
      container.service: $(yaml_quote "$CONTAINER_SERVICE")
      log.source: stdout
      job: docker
"
    FILELOG_PIPELINE+=("${STDOUT_RECV}")
    log "    ${CNAME}: real-time stdout receiver → ${STDOUT_RECV}"

    # -- Register service entry for report --
    if $HAS_VOLUME_LOGS; then
      ALL_SERVICES+=("docker_${SAFE_CNAME}")
      LOG_PATHS["docker_${SAFE_CNAME}"]="${EXISTING_VOLUME_PATHS}"
      SVC_PORT["docker_${SAFE_CNAME}"]=""
      SVC_STATUS["docker_${SAFE_CNAME}"]="logs_found"
      SVC_NOTES["docker_${SAFE_CNAME}"]="image=${CIMAGE} service=${CONTAINER_SERVICE} log_source=volume"
    else
      ALL_SERVICES+=("docker_${SAFE_CNAME}")
      LOG_PATHS["docker_${SAFE_CNAME}"]="${FIFO_PATH}"
      SVC_PORT["docker_${SAFE_CNAME}"]=""
      SVC_STATUS["docker_${SAFE_CNAME}"]="logs_found"
      SVC_NOTES["docker_${SAFE_CNAME}"]="image=${CIMAGE} service=${CONTAINER_SERVICE} log_source=stdout"
    fi

  done <<< "$CONTAINERS"

  # ---- Docker log tailer service ----
  # Creates a persistent background script per container that tails
  # docker logs --follow and writes to named files under /var/lib/otelcol/docker-logs/
  mkdir -p /var/lib/otelcol/docker-logs

  cat > /usr/local/bin/agent-docker-log-tailer <<'DOCKERTAILER'
#!/bin/bash
# agent-docker-log-tailer — tail stdout/stderr of ALL running containers
# Writes plain-text output to /var/lib/otelcol/docker-logs/<n>.log
# Uses per-container state files to resume from last position on restart,
# preventing log replay on service restarts.

LOG_DIR="/var/lib/otelcol/docker-logs"
STATE_DIR="/var/lib/otelcol/docker-log-state"
mkdir -p "$LOG_DIR" "$STATE_DIR"
declare -A TAIL_PIDS=()

start_tailer() {
  local CID_SHORT=$1
  local CNAME=$2
  local SAFE
  SAFE=$(echo "$CNAME" | tr '[:upper:]' '[:lower:]' | tr -cs 'a-z0-9' '_' | sed 's/_$//')
  local OUT="${LOG_DIR}/${SAFE}.log"
  local STATE_FILE="${STATE_DIR}/${SAFE}.since"

  # Already tailing this container and process still alive? Skip.
  if [ -n "${TAIL_PIDS[$CID_SHORT]:-}" ] && kill -0 "${TAIL_PIDS[$CID_SHORT]}" 2>/dev/null; then
    return
  fi

  # Build --since arg: resume from last saved timestamp to avoid replaying all logs
  local SINCE_ARG=""
  if [ -f "$STATE_FILE" ]; then
    local LAST_TS
    LAST_TS=$(cat "$STATE_FILE" 2>/dev/null | tr -d '[:space:]')
    [ -n "$LAST_TS" ] && SINCE_ARG="--since=${LAST_TS}"
  fi

  # Tail plain text — NO --timestamps flag; lines are raw app output, not JSON envelopes.
  # Update state file every 50 lines for cheap resume-point tracking.
  (
    LINE_COUNT=0
    docker logs --follow $SINCE_ARG "$CID_SHORT" 2>&1 | \
    while IFS= read -r LINE; do
      printf '%s\n' "$LINE" >> "$OUT"
      LINE_COUNT=$((LINE_COUNT + 1))
      if [ $((LINE_COUNT % 50)) -eq 0 ]; then
        printf '%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "${STATE_FILE}.tmp" && \
          mv "${STATE_FILE}.tmp" "$STATE_FILE" 2>/dev/null || true
        LINE_COUNT=0
      fi
    done
    # Save final timestamp when docker logs exits (container stopped)
    printf '%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "${STATE_FILE}.tmp" && \
      mv "${STATE_FILE}.tmp" "$STATE_FILE" 2>/dev/null || true
  ) &
  TAIL_PIDS[$CID_SHORT]=$!
  logger -t agent-docker-log-tailer "Started tailer for ${CNAME} (${CID_SHORT}) -> ${OUT}${SINCE_ARG:+ (resuming from $SINCE_ARG)}"
}

stop_tailer() {
  local CID_SHORT=$1
  if [ -n "${TAIL_PIDS[$CID_SHORT]:-}" ]; then
    kill "${TAIL_PIDS[$CID_SHORT]}" 2>/dev/null || true
    unset TAIL_PIDS[$CID_SHORT]
  fi
}

# Initial start for all currently running containers
while IFS=$'\t' read -r CID CNAME _REST; do
  [ -z "$CID" ] && continue
  start_tailer "$CID" "$CNAME"
done < <(docker ps --format $'{{.ID}}\t{{.Names}}\t{{.Image}}' 2>/dev/null || true)

# Watch docker events for container lifecycle changes
docker events --filter "event=start" --filter "event=die" --filter "event=stop" \
  --format '{{.Action}} {{.Actor.ID}} {{index .Actor.Attributes "name"}}' 2>/dev/null \
  | while read -r ACTION CID CNAME; do
    case "$ACTION" in
      start)    start_tailer "$CID" "$CNAME" ;;
      die|stop) stop_tailer  "$CID" ;;
    esac
  done
DOCKERTAILER
  chmod +x /usr/local/bin/agent-docker-log-tailer

  cat > /etc/systemd/system/agent-docker-log-tailer.service <<'DOCKERTAILERSVC'
[Unit]
Description=Observability Agent — Docker real-time log tailer
After=docker.service otelcol.service
Requires=docker.service
[Service]
Type=simple
ExecStart=/usr/local/bin/agent-docker-log-tailer
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=agent-docker-log-tailer
[Install]
WantedBy=multi-user.target
DOCKERTAILERSVC

  DOCKER_LOG_TAILER_UNIT_READY=true
  log "  Docker real-time log tailer unit installed"

else
  info "Docker not available — skipping container discovery"
fi

# ============================================
# DOCKER CONTAINER METRICS
# ============================================
# Per-container metrics: cpu, mem, net, block I/O, pids
# Collected every 30s via docker stats; served as Prometheus text via file server
# ============================================
step "Docker container metrics collector"

if [ -S /var/run/docker.sock ] && command -v docker &>/dev/null; then
  cat > /usr/local/bin/agent-docker-metrics <<'DOCKERMETRICS'
#!/bin/bash
# agent-docker-metrics — collect docker stats every 30s

OUT_DIR="/var/lib/otelcol"
OUT_FILE="${OUT_DIR}/docker-metrics.prom"
TMP_FILE="${OUT_FILE}.tmp"
mkdir -p "$OUT_DIR"

TIMESTAMP=$(date +%s%3N)

parse_bytes() {
  local RAW="$1"
  local NUM UNIT
  NUM=$(echo "$RAW" | grep -oP '[\d.]+' | head -1)
  UNIT=$(echo "$RAW" | grep -oP '[kKmMgGtT]i?[bB]' | head -1)
  case "${UNIT,,}" in
    kib|kb) awk -v n="$NUM" 'BEGIN{printf "%.0f", n*1024}' ;;
    mib|mb) awk -v n="$NUM" 'BEGIN{printf "%.0f", n*1024*1024}' ;;
    gib|gb) awk -v n="$NUM" 'BEGIN{printf "%.0f", n*1024*1024*1024}' ;;
    tib|tb) awk -v n="$NUM" 'BEGIN{printf "%.0f", n*1024*1024*1024*1024}' ;;
    *)      awk -v n="$NUM" 'BEGIN{printf "%.0f", n}' ;;
  esac
}

{
  echo "# Metrics naming: container_<container_name>_<metric>"

  docker stats --no-stream --format \
    '{{.Name}}|{{.CPUPerc}}|{{.MemUsage}}|{{.MemPerc}}|{{.NetIO}}|{{.BlockIO}}|{{.PIDs}}' \
    2>/dev/null | while IFS='|' read -r NAME CPU MEM_USAGE MEM_PERC NET_IO BLOCK_IO PIDS; do
    [ -z "$NAME" ] && continue

    SAFE=$(echo "$NAME" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9_' '_' | sed 's/__*/_/g;s/^_//;s/_$//')
    [ -z "$SAFE" ] && continue

    CPU_VAL=$(echo "$CPU"          | tr -d '%' | tr -d ' ')
    MEM_PERC_VAL=$(echo "$MEM_PERC" | tr -d '%' | tr -d ' ')

    MEM_PARTS=$(echo "$MEM_USAGE" | tr '/' '\n')
    MEM_USED_BYTES=$(parse_bytes "$(echo "$MEM_PARTS" | head -1 | tr -d ' ')")
    MEM_LIMIT_BYTES=$(parse_bytes "$(echo "$MEM_PARTS" | tail -1 | tr -d ' ')")

    NET_PARTS=$(echo "$NET_IO" | tr '/' '\n')
    NET_RX_BYTES=$(parse_bytes "$(echo "$NET_PARTS" | head -1 | tr -d ' ')")
    NET_TX_BYTES=$(parse_bytes "$(echo "$NET_PARTS" | tail -1 | tr -d ' ')")

    BLOCK_PARTS=$(echo "$BLOCK_IO" | tr '/' '\n')
    BLOCK_R_BYTES=$(parse_bytes "$(echo "$BLOCK_PARTS" | head -1 | tr -d ' ')")
    BLOCK_W_BYTES=$(parse_bytes "$(echo "$BLOCK_PARTS" | tail -1 | tr -d ' ')")

    PIDS_VAL=$(echo "$PIDS" | tr -d ' ')

    echo "# HELP container_${SAFE}_cpu_percent CPU usage percent"
    echo "# TYPE container_${SAFE}_cpu_percent gauge"
    echo "container_${SAFE}_cpu_percent ${CPU_VAL:-0} ${TIMESTAMP}"

    echo "# HELP container_${SAFE}_mem_usage_bytes Memory usage in bytes"
    echo "# TYPE container_${SAFE}_mem_usage_bytes gauge"
    echo "container_${SAFE}_mem_usage_bytes ${MEM_USED_BYTES:-0} ${TIMESTAMP}"

    echo "# HELP container_${SAFE}_mem_limit_bytes Memory limit in bytes"
    echo "# TYPE container_${SAFE}_mem_limit_bytes gauge"
    echo "container_${SAFE}_mem_limit_bytes ${MEM_LIMIT_BYTES:-0} ${TIMESTAMP}"

    echo "# HELP container_${SAFE}_mem_percent Memory usage percent"
    echo "# TYPE container_${SAFE}_mem_percent gauge"
    echo "container_${SAFE}_mem_percent ${MEM_PERC_VAL:-0} ${TIMESTAMP}"

    echo "# HELP container_${SAFE}_net_rx_bytes Network bytes received"
    echo "# TYPE container_${SAFE}_net_rx_bytes counter"
    echo "container_${SAFE}_net_rx_bytes ${NET_RX_BYTES:-0} ${TIMESTAMP}"

    echo "# HELP container_${SAFE}_net_tx_bytes Network bytes transmitted"
    echo "# TYPE container_${SAFE}_net_tx_bytes counter"
    echo "container_${SAFE}_net_tx_bytes ${NET_TX_BYTES:-0} ${TIMESTAMP}"

    echo "# HELP container_${SAFE}_block_read_bytes Block I/O read bytes"
    echo "# TYPE container_${SAFE}_block_read_bytes counter"
    echo "container_${SAFE}_block_read_bytes ${BLOCK_R_BYTES:-0} ${TIMESTAMP}"

    echo "# HELP container_${SAFE}_block_write_bytes Block I/O write bytes"
    echo "# TYPE container_${SAFE}_block_write_bytes counter"
    echo "container_${SAFE}_block_write_bytes ${BLOCK_W_BYTES:-0} ${TIMESTAMP}"

    echo "# HELP container_${SAFE}_pids Process count"
    echo "# TYPE container_${SAFE}_pids gauge"
    echo "container_${SAFE}_pids ${PIDS_VAL:-0} ${TIMESTAMP}"

  done
} > "$TMP_FILE" 2>/dev/null && mv "$TMP_FILE" "$OUT_FILE" || true
DOCKERMETRICS
  chmod +x /usr/local/bin/agent-docker-metrics

  cat > /etc/systemd/system/agent-docker-metrics.service <<'DOCKERMETRICSVC'
[Unit]
Description=Observability Agent — Docker container metrics collector
After=docker.service otelcol.service
Requires=docker.service
[Service]
Type=oneshot
ExecStart=/usr/local/bin/agent-docker-metrics
StandardOutput=journal
StandardError=journal
SyslogIdentifier=agent-docker-metrics
DOCKERMETRICSVC

  cat > /etc/systemd/system/agent-docker-metrics.timer <<'DOCKERMETRICTIMER'
[Unit]
Description=Run docker container metrics collector every 30 seconds
[Timer]
OnBootSec=20s
OnUnitActiveSec=30s
AccuracySec=5s
[Install]
WantedBy=timers.target
DOCKERMETRICTIMER

  DOCKER_METRICS_UNIT_READY=true
  DOCKER_METRICS_OK=true
  METRIC_PIPELINE+=("prometheus/docker_containers")
  log "  Docker container metrics collector unit installed"
else
  info "  Docker not available — skipping container metrics collector"
fi

# ============================================
# PROM FILE SERVER (always installed, serves docker + mariadb)
# ============================================
if command -v python3 &>/dev/null; then
  cat > /usr/local/bin/agent-prom-file-server <<'PROMSERVER'
#!/usr/bin/env python3
"""Serve Prometheus text files on port 19101."""
import http.server, os, socketserver

PORT = 19101

ROUTES = {
    "/docker":  "/var/lib/otelcol/docker-metrics.prom",
    "/metrics": "/var/lib/otelcol/docker-metrics.prom",
    "/mariadb": "/var/lib/agent-prom/mariadb_query_digests.prom",
    "/phpfpm":  "/var/lib/agent-prom/phpfpm_opcache.prom",
}

class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *args): pass
    def do_GET(self):
        fpath = ROUTES.get(self.path)
        if fpath is None:
            self.send_response(404); self.end_headers(); return
        self.send_response(200)
        self.send_header("Content-Type", "text/plain; version=0.0.4")
        self.end_headers()
        if os.path.exists(fpath):
            with open(fpath, "rb") as f:
                self.wfile.write(f.read())

with socketserver.TCPServer(("127.0.0.1", PORT), Handler) as httpd:
    httpd.serve_forever()
PROMSERVER
  chmod +x /usr/local/bin/agent-prom-file-server

  cat > /etc/systemd/system/agent-prom-file-server.service <<'PROMSERVERSVC'
[Unit]
Description=Observability Agent — Prometheus metrics file server (port 19101)
After=network.target
[Service]
Type=simple
ExecStart=/usr/local/bin/agent-prom-file-server
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=agent-prom-file-server
[Install]
WantedBy=multi-user.target
PROMSERVERSVC

  PROM_FILE_SERVER_UNIT_READY=true
  log "Prometheus file server unit installed"
else
  warn "python3 not found — Prometheus file server not installed"
  MANUAL_STEPS+=("Install python3 and then: systemctl enable --now agent-prom-file-server")
fi

# ============================================
# PHASE 2 — INTERACTIVE FIX LOOP (log paths only)
# ============================================
section "Phase 2: Fixing discovered issues"

# ---- 2a: PostgreSQL logging_collector ----
for SVC in "${ALL_SERVICES[@]}"; do
  [[ "$SVC" == postgresql* ]] || continue
  PORT="${SVC_PORT[$SVC]:-5432}"
  COLLECTOR=$(sudo -u postgres psql -p "$PORT" -tAc "SHOW logging_collector;" 2>/dev/null \
    | tr -d ' ') || true
  if [ "$COLLECTOR" = "off" ]; then
    warn "  PostgreSQL (port ${PORT}): logging_collector is OFF — logs going to journald only"
    if ask "  Enable logging_collector for PostgreSQL on port ${PORT}?"; then
      sudo -u postgres psql -p "$PORT" -c "ALTER SYSTEM SET logging_collector = on;" 2>/dev/null
      sudo -u postgres psql -p "$PORT" -c "ALTER SYSTEM SET log_directory = 'log';" 2>/dev/null
      sudo -u postgres psql -p "$PORT" -c "ALTER SYSTEM SET log_filename = 'postgresql-%Y-%m-%d.log';" 2>/dev/null
      sudo -u postgres psql -p "$PORT" -c "ALTER SYSTEM SET log_min_messages = 'info';" 2>/dev/null
      PG_UNIT=$(systemctl list-units --type=service --state=running --no-legend --plain \
        | grep "postgresql" | awk '{print $1}' | while read U; do
          UP=$(systemctl show -p MainPID "$U" 2>/dev/null | cut -d= -f2)
          ACTUAL_PORT=$(ss -tlnp | grep "pid=${UP}," | grep -oP '(?<=:)\d+(?=\s)' | head -1)
          [ "$ACTUAL_PORT" = "$PORT" ] && echo "$U" && break
        done)
      [ -n "$PG_UNIT" ] && sudo systemctl restart "$PG_UNIT" && \
        fixed "  PostgreSQL ${PORT}: logging_collector enabled"
      DATA_DIR=$(sudo -u postgres psql -p "$PORT" -tAc "SHOW data_directory;" 2>/dev/null | tr -d ' ') || true
      [ -n "$DATA_DIR" ] && LOG_PATHS[$SVC]="${DATA_DIR}/log/*.log"
      SVC_NOTES[$SVC]="${SVC_NOTES[$SVC]/logging_collector=off/logging_collector=on}"
    else
      skipped "  PostgreSQL ${PORT}: logging_collector stays off"
      SVC_NOTES[$SVC]="${SVC_NOTES[$SVC]:-} | logs_via=journald"
    fi
  fi
done

# ---- 2b: Redis log file ----
if [ "${PROCESSED_CANONICALS[redis]+_}" ] && [ -z "${LOG_PATHS[redis]:-}" ]; then
  warn "  redis: no log file configured"
  if ask "  Configure redis log file at /var/log/redis/redis-server.log?"; then
    mkdir -p /var/log/redis
    chown redis:adm /var/log/redis 2>/dev/null || chown redis:redis /var/log/redis 2>/dev/null || true
    for RCONF in /etc/redis/redis.conf /etc/redis.conf; do
      if [ -f "$RCONF" ]; then
        backup_file "$RCONF"
        sed -i 's|^logfile .*|logfile /var/log/redis/redis-server.log|' "$RCONF"
        systemctl restart redis-server 2>/dev/null || systemctl restart redis 2>/dev/null || true
        sleep 1
        [ -f /var/log/redis/redis-server.log ] && {
          LOG_PATHS["redis"]="/var/log/redis/redis-server.log"
          fixed "  redis log file configured"
        }
        break
      fi
    done
  else
    skipped "  redis log file not configured"
  fi
fi

# ============================================
# PHASE 3 — BUILD OTELCOL CONFIG
# ============================================
section "Phase 3: Generating otelcol configuration"

AUTH_B64=$(echo -n "${API_KEY}:${API_KEY}" | base64 -w 0)

# ---- Non-docker services: report-only, no log collection ----
# System services (nginx, apache, postgresql, etc.) are discovered and reported
# but NOT collected — only container logs and system logs are ingested.
# The discovery loop below is intentionally skipped for non-docker services.
for SVC in "${ALL_SERVICES[@]}"; do
  [[ "$SVC" == docker_* ]] && continue
  # Report only — no filelog receiver generated
  continue

  # (dead code kept as reference for future per-service collection)
  PATHS="${LOG_PATHS[$SVC]:-}"
  [ -z "$PATHS" ] && continue

  M_TYPE_VAL="${SVC_STATUS[$SVC]:-}"

  PATHS_YAML=$(echo "$PATHS" | while IFS= read -r P; do [ -n "$P" ] && echo "      - $P"; done)
  [ -z "$PATHS_YAML" ] && continue

  FMT="${LOG_FORMAT[$SVC]:-unknown}"
  RECV_TMP=$(mktemp /tmp/otelcol_recv_XXXXXX)

  case "$FMT" in
    nginx_combined|apache_combined)
      cat >> "$RECV_TMP" << RECV
  filelog/${SVC}:
    include:
${PATHS_YAML}
    include_file_path: true
    start_at: end
    operators:
      - type: regex_parser
        regex: '^(?P<client_ip>[:\w.]+(?:::ffff:[\d.]+)?)\s+\S+\s+\S+\s+\[(?P<log_day>\d{2})/\w+/(?P<log_year>\d{4}):(?P<log_hour>\d{2}):\d{2}:\d{2}\s[^\]]+\]\s+"(?P<http_method>[A-Z]+)\s+(?P<request_path>[^\s?]+)(?:[^\s]*)\s+HTTP/[^"]+"\s+(?P<http_status>\d{3})\s+(?P<response_bytes>\d+|-)(?:\s+"(?P<http_referer>[^"]*)"\s+"(?P<user_agent>[^"]*)")?(?:.*?rt=(?P<request_time_s>[\d.]+))?'
        on_error: send_quiet
      - type: add
        field: attributes.log_format
        value: combined
        on_error: send_quiet
    resource:
      service.name: ${SVC}
      job: ${SVC}
      log.format: ${FMT}
RECV
      ;;
    json)
      cat >> "$RECV_TMP" << RECV
  filelog/${SVC}:
    include:
${PATHS_YAML}
    include_file_path: true
    start_at: end
    operators:
      - type: json_parser
        on_error: send_quiet
      - type: copy
        from: attributes.s
        to: attributes.log_level
        if: 'attributes.s != nil'
        on_error: send_quiet
      - type: copy
        from: attributes.level
        to: attributes.log_level
        if: 'attributes.level != nil'
        on_error: send_quiet
      - type: move
        from: attributes.msg
        to: body
        if: 'attributes.msg != nil'
        on_error: send_quiet
      - type: add
        field: attributes.log_format
        value: json
        on_error: send_quiet
    resource:
      service.name: ${SVC}
      job: ${SVC}
      log.format: json
RECV
      ;;
    pg_csv)
      cat >> "$RECV_TMP" << RECV
  filelog/${SVC}:
    include:
${PATHS_YAML}
    include_file_path: true
    start_at: end
    operators:
      # Combine continuation lines (e.g. bare DETAIL/HINT/CONTEXT/LOCATION
      # lines without a full log header) into the preceding primary entry.
      - type: recombine
        is_first_entry: 'body matches "^\\\\d{4}-\\\\d{2}-\\\\d{2}[T ]\\\\d{2}:\\\\d{2}:\\\\d{2}"'
        combine_field: body
        combine_with: "\n"
        source_identifier: attributes["log.file.path"]
        max_batch_size: 200
        force_flush_period: 5s
        max_log_size: 65536
        on_error: send_quiet
      - type: regex_parser
        regex: '^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}[^\[]*\[\d+\]\s*(?:(?P<pg_user>\S+)@(?P<pg_database>\S+)\s+)?\S+\(\d+\)\s+\S+\s+(?P<log_level>LOG|ERROR|FATAL|PANIC|WARNING|NOTICE|INFO|DETAIL|HINT|DEBUG\d*):\s+(?P<pg_message>.{0,600})'
        on_error: send_quiet
      - type: regex_parser
        parse_from: attributes.pg_message
        regex: '^duration:\s+(?P<duration_ms>[\d.]+)\s+ms'
        on_error: send_quiet
      # Drop "connection received" lines — they carry no user info and are not
      # useful in isolation. The meaningful data is in the authenticated/
      # authorized/disconnection lines which have the actual user.
      - type: filter
        expr: 'body matches "connection received:"'
        on_error: send_quiet
      # Drop monitoring user logs — the agent's own connections, not real app traffic.
      - type: filter
        expr: 'attributes.pg_user == "monitoring" or attributes.pg_user == "postgres_exporter" or attributes.pg_user == "pgexporter"'
        on_error: send_quiet
      - type: add
        field: attributes.log_format
        value: pg_csv
        on_error: send_quiet
    resource:
      service.name: ${SVC}
      job: ${SVC}
      log.format: pg_csv
RECV
      ;;
    redis_log)
      cat >> "$RECV_TMP" << RECV
  filelog/${SVC}:
    include:
${PATHS_YAML}
    include_file_path: true
    start_at: end
    operators:
      - type: regex_parser
        regex: '^\d+:[A-Z]\s+\d+\s+\w+\s+\d{4}\s+[\d:.]+\s+(?P<redis_level_sym>[*#\-.])\s+(?P<redis_message>.+)'
        on_error: send_quiet
      - type: add
        field: attributes.log_format
        value: redis_log
        on_error: send_quiet
    resource:
      service.name: ${SVC}
      job: ${SVC}
      log.format: redis_log
RECV
      ;;
    mysql_log)
      # Split paths: error.log → log_kind="error", slow.log/general log → log_kind="slow"
      ERR_PATHS=$(echo "$PATHS" | grep -iE "error\." || true)
      SLOW_PATHS=$(echo "$PATHS" | grep -ivE "error\." || true)
      ERR_YAML=$(echo "$ERR_PATHS" | while IFS= read -r P; do [ -n "$P" ] && echo "      - $P"; done)
      SLOW_YAML=$(echo "$SLOW_PATHS" | while IFS= read -r P; do [ -n "$P" ] && echo "      - $P"; done)

      if [ -n "$ERR_YAML" ]; then
        cat >> "$RECV_TMP" << RECV
  filelog/${SVC}_error:
    include:
${ERR_YAML}
    include_file_path: true
    start_at: end
    operators:
      - type: regex_parser
        regex: '^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[.\dZ+:\-]*\s+\d+\s+\[(?P<log_level>System|Warning|ERROR|Note)\](?:\s+\[(?P<mysql_error_code>MY-\d+)\])?\s+\[(?P<mysql_subsystem>\w+)\]\s+(?P<mysql_message>.{0,400})'
        on_error: send_quiet
      - type: add
        field: attributes.log_format
        value: mysql_log
        on_error: send_quiet
    resource:
      service.name: ${SVC}
      job: ${SVC}
      log.format: mysql_log
      log_kind: error
RECV
      fi

      if [ -n "$SLOW_YAML" ]; then
        cat >> "$RECV_TMP" << RECV
  filelog/${SVC}_slow:
    include:
${SLOW_YAML}
    include_file_path: true
    start_at: end
    operators:
      # Drop the useless "# Time:" lines — they appear at second boundaries
      # but carry no query data and break the recombine boundary detection.
      # Also drops timestamp-prefixed variant: "2026-07-08 19:12:35.005 # Time: ..."
      - type: filter
        expr: 'body matches "#\\s*Time:\\s*\\d+"'
        on_error: send_quiet
      # Join every physical line of one slow-query entry into a single record,
      # using the "# User@Host:" line as the boundary (appears once per query).
      - type: recombine
        is_first_entry: 'body matches "^# User@Host:"'
        combine_field: body
        combine_with: "\n"
        source_identifier: attributes["log.file.path"]
        max_batch_size: 200
        force_flush_period: 5s
        max_log_size: 65536
        on_error: send_quiet
      # Drop the monitoring agent's own queries entirely, now that each entry
      # is a complete record. The "monitor" user is created for the agent's DB
      # connection and is not real application traffic.
      - type: filter
        expr: 'body matches "User@Host:\\\\s*monitor\\\\[monitor\\\\]"'
        on_error: send_quiet
      - type: regex_parser
        regex: '^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[.\dZ+:\-]*\s+\d+\s+\[(?P<log_level>System|Warning|ERROR|Note)\](?:\s+\[(?P<mysql_error_code>MY-\d+)\])?\s+\[(?P<mysql_subsystem>\w+)\]\s+(?P<mysql_message>.{0,400})'
        on_error: send_quiet
      - type: add
        field: attributes.log_format
        value: mysql_log
        on_error: send_quiet
    resource:
      service.name: ${SVC}
      job: ${SVC}
      log.format: mysql_log
      log_kind: slow
RECV
      fi
      ;;
    phpfpm_log)
      cat >> "$RECV_TMP" << RECV
  filelog/${SVC}:
    include:
${PATHS_YAML}
    include_file_path: true
    start_at: end
    operators:
      - type: regex_parser
        regex: '^\[[\d\- :]+\]\s+(?:(?P<fpm_pool>pool \S+):\s+(?P<fpm_message>.{0,300})|(?P<log_level>WARNING|ERROR|NOTICE|ALERT):\s+(?P<fpm_message_plain>.{0,300}))'
        on_error: send_quiet
      - type: add
        field: attributes.log_format
        value: phpfpm_log
        on_error: send_quiet
    resource:
      service.name: ${SVC}
      job: ${SVC}
      log.format: phpfpm_log
RECV
      ;;
    syslog)
      cat >> "$RECV_TMP" << RECV
  filelog/${SVC}:
    include:
${PATHS_YAML}
    include_file_path: true
    start_at: end
    operators:
      - type: regex_parser
        regex: '^\w+\s+\d+\s+[\d:]+\s+\S+\s+(?P<syslog_process>[^\[:]+)(?:\[(?P<syslog_pid>\d+)\])?:\s+(?P<syslog_message>.+)'
        on_error: send_quiet
      - type: severity_parser
        parse_from: body
        on_error: send_quiet
    resource:
      service.name: ${SVC}
      job: ${SVC}
      log.format: syslog
RECV
      ;;
    unknown|*)
      cat >> "$RECV_TMP" << RECV
  filelog/${SVC}:
    include:
${PATHS_YAML}
    include_file_path: true
    start_at: end
    operators:
      - type: json_parser
        on_error: send_quiet
      - type: copy
        from: attributes.level
        to: attributes.log_level
        if: 'attributes.level != nil'
        on_error: send_quiet
      - type: severity_parser
        parse_from: body
        on_error: send_quiet
    resource:
      service.name: ${SVC}
      job: ${SVC}
      log.format: unknown
RECV
      ;;
  esac

  FILELOG_RECEIVERS_YAML="${FILELOG_RECEIVERS_YAML}$(cat "$RECV_TMP")"$'\n'
  rm -f "$RECV_TMP"
  FILELOG_PIPELINE+=("filelog/${SVC}")
done

# Add syslog fallback log receiver
SYSLOG_YAML=""
[ -f /var/log/syslog ]   && SYSLOG_YAML="      - /var/log/syslog"
[ -f /var/log/messages ] && SYSLOG_YAML="${SYSLOG_YAML}"$'\n'"      - /var/log/messages"
if [ -n "$SYSLOG_YAML" ]; then
  FILELOG_RECEIVERS_YAML="${FILELOG_RECEIVERS_YAML}
  filelog/syslog:
    include:
${SYSLOG_YAML}
    include_file_path: true
    start_at: end
    resource:
      service.name: system
      job: syslog
"
  FILELOG_PIPELINE+=("filelog/syslog")
fi

# ---- Docker container metrics receiver (if file server is up) ----
DOCKER_STATS_YAML=""
if [ "${DOCKER_METRICS_OK:-false}" = "true" ]; then
  DOCKER_STATS_YAML="
  prometheus/docker_containers:
    config:
      scrape_configs:
        - job_name: docker_container_stats
          static_configs:
            - targets: ['localhost:19101']
          metrics_path: /docker
          scrape_interval: 35s
"
fi

# ---- MariaDB query digest receiver (if .prom file exists) ----
MARIADB_DIGEST_YAML=""
if [ -f /var/lib/agent-prom/mariadb_query_digests.prom ]; then
  MARIADB_DIGEST_YAML="
  prometheus/mariadb_digests:
    config:
      scrape_configs:
        - job_name: mariadb_query_digests
          static_configs:
            - targets: ['localhost:19101']
          metrics_path: /mariadb
          scrape_interval: 60s
"
  METRIC_PIPELINE+=("prometheus/mariadb_digests")
fi

# ---- PHP-FPM OPcache receiver (if .prom file exists) ----
PHPFPM_OPCACHE_YAML=""
if [ -f /var/lib/agent-prom/phpfpm_opcache.prom ]; then
  PHPFPM_OPCACHE_YAML="
  prometheus/phpfpm_opcache:
    config:
      scrape_configs:
        - job_name: phpfpm_opcache
          static_configs:
            - targets: ['localhost:19101']
          metrics_path: /phpfpm
          scrape_interval: 60s
"
  METRIC_PIPELINE+=("prometheus/phpfpm_opcache")
fi

# ---- Assemble pipeline receiver lists ----
METRIC_PIPELINE+=("statsd")
dedupe_array METRIC_PIPELINE
dedupe_array FILELOG_PIPELINE

METRIC_RECEIVERS_STR=$(printf '%s, ' "${METRIC_PIPELINE[@]}" | sed 's/, $//')

LOG_RECEIVERS_LIST="journald"
for R in "${FILELOG_PIPELINE[@]}"; do
  LOG_RECEIVERS_LIST="${LOG_RECEIVERS_LIST}, ${R}"
done

host_log_unit_match_expr() {
  local UNIT=$1
  printf '(attributes["_SYSTEMD_UNIT"] == "%s" or attributes["systemd.unit"] == "%s" or attributes["systemd_unit"] == "%s" or resource.attributes["systemd_unit"] == "%s")' "$UNIT" "$UNIT" "$UNIT" "$UNIT"
}

host_log_identifier_match_expr() {
  local IDENTIFIER=$1
  printf '(attributes["SYSLOG_IDENTIFIER"] == "%s" or attributes["syslog.identifier"] == "%s" or attributes["syslog_identifier"] == "%s")' "$IDENTIFIER" "$IDENTIFIER" "$IDENTIFIER"
}

host_log_or_expr() {
  local EXPR=""
  local PART
  for PART in "$@"; do
    [ -z "$PART" ] && continue
    if [ -z "$EXPR" ]; then
      EXPR="$PART"
    else
      EXPR="${EXPR} or ${PART}"
    fi
  done
  printf '%s' "$EXPR"
}

declare -a HOST_LOG_KEEP_EXPR_PARTS=()
HOST_LOG_KEEP_EXPR_PARTS+=("$(host_log_identifier_match_expr sshd)")
HOST_LOG_KEEP_EXPR_PARTS+=("$(host_log_identifier_match_expr sudo)")
HOST_LOG_KEEP_EXPR_PARTS+=("$(host_log_identifier_match_expr otelcol)")
HOST_LOG_KEEP_EXPR_PARTS+=("$(host_log_unit_match_expr ssh.service)")
HOST_LOG_KEEP_EXPR_PARTS+=("$(host_log_unit_match_expr sshd.service)")
HOST_LOG_KEEP_EXPR_PARTS+=("$(host_log_unit_match_expr sudo.service)")
HOST_LOG_KEEP_EXPR_PARTS+=("$(host_log_unit_match_expr otelcol.service)")

for SVC in "${ALL_SERVICES[@]}"; do
  [ -z "$SVC" ] && continue
  HOST_LOG_KEEP_EXPR_PARTS+=("$(host_log_unit_match_expr "${SVC}.service")")
  HOST_LOG_KEEP_EXPR_PARTS+=("$(host_log_identifier_match_expr "$SVC")")
done

HOST_LOG_KEEP_EXPR=$(host_log_or_expr "${HOST_LOG_KEEP_EXPR_PARTS[@]}")
[ -z "$HOST_LOG_KEEP_EXPR" ] && HOST_LOG_KEEP_EXPR="false"

HOST_LOG_PRIORITY_AT_LEAST_WARNING='((attributes["PRIORITY"] == nil and attributes["priority"] == nil) or (attributes["PRIORITY"] != nil and Int(attributes["PRIORITY"]) >= 4) or (attributes["priority"] != nil and Int(attributes["priority"]) >= 4))'
HOST_LOG_PRIORITY_INFO_OR_DEBUG='((attributes["PRIORITY"] != nil and Int(attributes["PRIORITY"]) >= 6) or (attributes["priority"] != nil and Int(attributes["priority"]) >= 6))'
HOST_LOG_NOISY_ERROR_PLUS_ONLY='(attributes["_SYSTEMD_UNIT"] == "systemd-udevd.service" or attributes["systemd.unit"] == "systemd-udevd.service" or attributes["systemd_unit"] == "systemd-udevd.service" or resource.attributes["systemd_unit"] == "systemd-udevd.service" or attributes["SYSLOG_IDENTIFIER"] == "kernel" or attributes["syslog.identifier"] == "kernel" or attributes["syslog_identifier"] == "kernel" or attributes["_TRANSPORT"] == "kernel" or attributes["systemd.transport"] == "kernel" or attributes["transport"] == "kernel")'
HOST_LOG_NOISY_INFO_DEBUG='(attributes["_SYSTEMD_UNIT"] == "systemd-resolved.service" or attributes["systemd.unit"] == "systemd-resolved.service" or attributes["systemd_unit"] == "systemd-resolved.service" or resource.attributes["systemd_unit"] == "systemd-resolved.service" or attributes["_SYSTEMD_UNIT"] == "systemd-timesyncd.service" or attributes["systemd.unit"] == "systemd-timesyncd.service" or attributes["systemd_unit"] == "systemd-timesyncd.service" or resource.attributes["systemd_unit"] == "systemd-timesyncd.service" or attributes["_SYSTEMD_UNIT"] == "NetworkManager.service" or attributes["systemd.unit"] == "NetworkManager.service" or attributes["systemd_unit"] == "NetworkManager.service" or resource.attributes["systemd_unit"] == "NetworkManager.service" or attributes["SYSLOG_IDENTIFIER"] == "NetworkManager" or attributes["syslog.identifier"] == "NetworkManager" or attributes["syslog_identifier"] == "NetworkManager")'
HOST_LOG_OTELCOL_STACK_FRAME='((attributes["_SYSTEMD_UNIT"] == "otelcol.service" or attributes["systemd.unit"] == "otelcol.service" or attributes["systemd_unit"] == "otelcol.service" or attributes["SYSLOG_IDENTIFIER"] == "otelcol" or attributes["syslog.identifier"] == "otelcol" or attributes["syslog_identifier"] == "otelcol") and IsMatch(body, "^github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza"))'

# ============================================
# WRITE CONFIG FILE
# ============================================
mkdir -p "$CONFIG_DIR"

cat > "$CONFIG_DIR/config.yaml" <<YAML
# ============================================
# OTEL COLLECTOR — Auto-generated by installer
# Agent Version : ${AGENT_VERSION}
# Tenant        : ${API_KEY:0:8}...
# Host          : ${HOSTNAME_FQDN}
# Machine ID    : ${MACHINE_ID:0:8}...
# Generated     : $(date -u +"%Y-%m-%dT%H:%M:%SZ")
# ============================================
# METRIC NAMING CONVENTION
# ============================================
# System    : system_cpu_*, system_memory_*, system_disk_*, system_network_*
# Containers: container_<n>_cpu_percent, container_<n>_mem_usage_bytes,
#             container_<n>_mem_limit_bytes, container_<n>_mem_percent,
#             container_<n>_net_rx_bytes, container_<n>_net_tx_bytes,
#             container_<n>_block_read_bytes, container_<n>_block_write_bytes,
#             container_<n>_pids
# Custom    : any StatsD metric sent to localhost:8125
# ============================================

extensions:
  health_check:
    endpoint: 0.0.0.0:13133

receivers:

  hostmetrics:
    collection_interval: 30s
    scrapers:
      cpu:
        metrics:
          system.cpu.time:
            enabled: true
          system.cpu.utilization:
            enabled: true
          system.cpu.logical.count:
            enabled: true
          system.cpu.frequency:
            enabled: true
      memory:
        metrics:
          system.memory.usage:
            enabled: true
          system.memory.utilization:
            enabled: true
      paging:
        metrics:
          system.paging.usage:
            enabled: true
          system.paging.operations:
            enabled: true
          system.paging.faults:
            enabled: true
      disk:
        metrics:
          system.disk.io:
            enabled: true
          system.disk.io_time:
            enabled: true
          system.disk.operation_time:
            enabled: true
          system.disk.operations:
            enabled: true
          system.disk.pending_operations:
            enabled: true
          system.disk.weighted_io_time:
            enabled: true
      filesystem:
        metrics:
          system.filesystem.usage:
            enabled: true
          system.filesystem.utilization:
            enabled: true
          system.filesystem.inodes.usage:
            enabled: true
      network:
        metrics:
          system.network.io:
            enabled: true
          system.network.packets:
            enabled: true
          system.network.errors:
            enabled: true
          system.network.dropped:
            enabled: true
          system.network.connections:
            enabled: true
          system.network.conntrack.count:
            enabled: true
          system.network.conntrack.max:
            enabled: true
      load:
        metrics:
          system.cpu.load_average.1m:
            enabled: true
          system.cpu.load_average.5m:
            enabled: true
          system.cpu.load_average.15m:
            enabled: true
      processes:
        metrics:
          system.processes.count:
            enabled: true
          system.processes.created:
            enabled: true
  journald:
    start_at: end
    resource:
      service.name: journald
      job: journald

  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

  statsd:
    endpoint: 0.0.0.0:8125
    aggregation_interval: 30s
    enable_metric_type: true

${DOCKER_STATS_YAML}
${MARIADB_DIGEST_YAML}
${PHPFPM_OPCACHE_YAML}
${FILELOG_RECEIVERS_YAML}

processors:

  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128

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

  filter/drop_scrape_meta:
    error_mode: ignore
    metrics:
      metric:
        - 'name == "up"'
        - 'name == "target_info"'
        - 'IsMatch(name, "^scrape_")'

  filter/drop_process:
    error_mode: ignore
    metrics:
      metric:
        - 'IsMatch(name, "^process_")'

  filter/drop_phpfpm_pollution:
    error_mode: ignore
    logs:
      log_record:
        - 'IsMatch(body, "GET /status")'
        - 'IsMatch(body, "/internal/opcache-collector")'

  filter/drop_host_log_noise:
    error_mode: ignore
    logs:
      log_record:
        - '${HOST_LOG_OTELCOL_STACK_FRAME}'
        - '(${HOST_LOG_NOISY_ERROR_PLUS_ONLY}) and (${HOST_LOG_PRIORITY_AT_LEAST_WARNING})'
        - '(${HOST_LOG_NOISY_INFO_DEBUG}) and (${HOST_LOG_PRIORITY_INFO_OR_DEBUG})'
        - '(${HOST_LOG_PRIORITY_INFO_OR_DEBUG}) and not (${HOST_LOG_KEEP_EXPR})'

  filter/drop_container_log_noise:
    error_mode: ignore
    logs:
      log_record:
        # Only apply to container logs (job: docker)
        - 'resource.attributes["job"] == "docker" and body == nil'
        - 'resource.attributes["job"] == "docker" and body == ""'
        - 'resource.attributes["job"] == "docker" and IsMatch(body, "^[[:space:]]*$")'
        - 'resource.attributes["job"] == "docker" and IsMatch(body, "(?i)(GET|POST|HEAD|PUT|DELETE|OPTIONS|PATCH) /(health|healthz|readyz|livez|ping|status|metrics|heartbeat)")'
        - 'resource.attributes["job"] == "docker" and IsMatch(body, "(?i)\"GET /(health|healthz|readyz|livez|ping|status|metrics|heartbeat)\"")'
        - 'resource.attributes["job"] == "docker" and IsMatch(body, "(?i)health check|health status|container (is )?(healthy|unhealthy|starting)")'
        - 'resource.attributes["job"] == "docker" and IsMatch(body, "(?i)heartbeat|keepalive|alive check")'
        - 'resource.attributes["job"] == "docker" and IsMatch(body, "^-+$")'
        # Drop structured info-level logs (common formats: level=info, "level":"info")
        # unless the line also contains error/warn/fatal/etc keywords
        - 'resource.attributes["job"] == "docker" and IsMatch(body, "(?i)level=info($|[^a-z0-9])") and not IsMatch(body, "(?i)(error|warn|fatal|panic|exception|critical)")'
        - 'resource.attributes["job"] == "docker" and IsMatch(body, "(?i)\\"level\\"[[:space:]]*:[[:space:]]*\\"info\\"") and not IsMatch(body, "(?i)(error|warn|fatal|panic|exception|critical|\\"alert\\"|\\"emergency\\")")'

exporters:

  prometheusremotewrite:
    endpoint: "${METRICS_ENDPOINT}"
    tls:
      insecure: true
    headers:
      Authorization: "Basic ${AUTH_B64}"
    resource_to_telemetry_conversion:
      enabled: true

  otlphttp/logs:
    endpoint: "${LOGS_ENDPOINT}"
    tls:
      insecure: true
    headers:
      Authorization: "Basic ${AUTH_B64}"

  otlphttp/traces:
    endpoint: "${TRACES_ENDPOINT}"
    tls:
      insecure: true
    headers:
      Authorization: "Basic ${AUTH_B64}"

service:
  extensions: [health_check]
  pipelines:

    metrics:
      receivers: [${METRIC_RECEIVERS_STR}]
      processors: [memory_limiter, resourcedetection, resource, filter/drop_scrape_meta, filter/drop_process, batch]
      exporters: [prometheusremotewrite]

    logs:
      receivers: [${LOG_RECEIVERS_LIST}]
      processors: [memory_limiter, resourcedetection, resource, filter/drop_phpfpm_pollution, filter/drop_host_log_noise, filter/drop_container_log_noise, batch]
      exporters: [otlphttp/logs]

    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlphttp/traces]

  telemetry:
    logs:
      level: warn
    metrics:
      level: none
YAML

chown root:root "$CONFIG_DIR/config.yaml"
chmod 600 "$CONFIG_DIR/config.yaml"
log "Config written to $CONFIG_DIR/config.yaml"

# ============================================
# PHASE 4 — DOWNLOAD & INSTALL OTELCOL
# ============================================
section "Phase 4: Installing OpenTelemetry Collector v${OTEL_VERSION}"

DOWNLOAD_URL="https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v${OTEL_VERSION}/otelcol-contrib_${OTEL_VERSION}_linux_${ARCH_NAME}.tar.gz"

cd /tmp
curl -sSL -o otelcol.tar.gz "$DOWNLOAD_URL" || error "Failed to download collector"
tar -xzf otelcol.tar.gz otelcol-contrib
mv otelcol-contrib "$INSTALL_DIR/otelcol"
chmod +x "$INSTALL_DIR/otelcol"
rm -f otelcol.tar.gz
log "Collector installed at $INSTALL_DIR/otelcol"

if ! "$INSTALL_DIR/otelcol" validate --config "file:${CONFIG_DIR}/config.yaml" >/dev/null 2>&1; then
  "$INSTALL_DIR/otelcol" validate --config "file:${CONFIG_DIR}/config.yaml" || true
  error "Generated otelcol config is invalid — fix ${CONFIG_DIR}/config.yaml then: systemctl restart otelcol"
fi

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
ExecStart=/usr/local/bin/otelcol --config=file:/etc/otelcol/config.yaml
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
  log "otelcol service started"
else
  error "otelcol failed to start — run: journalctl -u otelcol -n 50"
fi

if [ "$PROM_FILE_SERVER_UNIT_READY" = "true" ]; then
  systemctl enable agent-prom-file-server
  if systemctl restart agent-prom-file-server && sleep 1 && systemctl is-active --quiet agent-prom-file-server; then
    log "Prometheus file server started on 127.0.0.1:19101 (routes: /docker, /mariadb)"
  else
    warn "Prometheus file server failed to start"
    MANUAL_STEPS+=("Start the metrics file server: systemctl restart agent-prom-file-server")
  fi
fi

if [ "$DOCKER_LOG_TAILER_UNIT_READY" = "true" ]; then
  systemctl enable agent-docker-log-tailer
  if systemctl restart agent-docker-log-tailer && sleep 2 && systemctl is-active --quiet agent-docker-log-tailer; then
    log "  Docker real-time log tailer started"
  else
    warn "  Docker log tailer failed to start"
    MANUAL_STEPS+=("Docker log tailer: systemctl restart agent-docker-log-tailer")
  fi
fi

if [ "$DOCKER_METRICS_UNIT_READY" = "true" ]; then
  systemctl enable agent-docker-metrics.timer
  if systemctl restart agent-docker-metrics.timer && systemctl start agent-docker-metrics; then
    log "  Docker container metrics collector started (timer: every 30s)"
  else
    warn "  Docker metrics collector failed to start"
    MANUAL_STEPS+=("Docker container metrics: systemctl restart agent-docker-metrics.timer")
  fi
fi

# ============================================
# PHASE 5 — DOCKER EVENT WATCHER
# ============================================
section "Phase 5: Docker event watcher"

if [ -S /var/run/docker.sock ] && command -v docker &>/dev/null; then
  cat > /usr/local/bin/agent-docker-update <<'DOCKERUPDATE'
#!/bin/bash
# Updates container name mapping and signals otelcol to reload
MAP_FILE="/etc/otelcol/docker-names.map"
LOCK_FILE="/tmp/agent-docker-update.lock"
exec 9>"$LOCK_FILE"
flock -n 9 || exit 0
docker ps --format '{{.ID}}={{.Names}}' > "${MAP_FILE}.tmp" 2>/dev/null && mv "${MAP_FILE}.tmp" "$MAP_FILE"
pkill -HUP -x otelcol 2>/dev/null || true
logger -t agent-docker-update "Container map updated: $(wc -l < $MAP_FILE) containers"
DOCKERUPDATE
  chmod +x /usr/local/bin/agent-docker-update

  cat > /etc/systemd/system/agent-docker-watcher.service <<'DOCKERSVC'
[Unit]
Description=Observability Agent — Docker Event Watcher
After=docker.service otelcol.service
Requires=docker.service
[Service]
Type=simple
ExecStart=/bin/bash -c 'docker events --filter "event=start" --filter "event=die" --filter "event=rename" --format "{{.Type}} {{.Action}} {{.Actor.ID}}" | while read -r LINE; do /usr/local/bin/agent-docker-update; done'
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=agent-docker-watcher
[Install]
WantedBy=multi-user.target
DOCKERSVC

  DOCKER_WATCHER_UNIT_READY=true
  log "  Docker event watcher unit installed"
else
  info "  Docker not available — skipping docker event watcher"
fi

# ============================================
# PHASE 6 — DAILY REDISCOVERY CRON
# ============================================
section "Phase 6: Setting up daily service rediscovery"

cat > /usr/local/bin/agent-rediscover <<REDISCOVER
#!/bin/bash
KNOWN_FILE="/etc/otelcol/known-services.txt"
PLATFORM_HOST="${PLATFORM_HOST}"
PLATFORM_PORT="${PLATFORM_PORT}"
API_KEY="${API_KEY}"
AUTH_B64="${AUTH_B64}"

CURRENT=\$(systemctl list-units --type=service --state=running --no-legend --plain 2>/dev/null | awk '{print \$1}' | sort)
KNOWN=\$(cat "\$KNOWN_FILE" 2>/dev/null | sort)
NEW_SVCS=\$(comm -23 <(echo "\$CURRENT") <(echo "\$KNOWN") 2>/dev/null | grep -v '^\$' || true)

if [ -n "\$NEW_SVCS" ]; then
  logger -t agent-rediscover "New services detected: \$(echo \$NEW_SVCS | tr '\n' ' ')"
  NEW_JSON=\$(echo "\$NEW_SVCS" | while read SVC; do
    echo -n "{\"service\":\"\${SVC}\",\"host\":\"\$(hostname -f)\"},"
  done | sed 's/,\$//')
  curl -sf -X POST \
    -H "Content-Type: application/json" \
    -H "Authorization: Basic \${AUTH_B64}" \
    -d "{\"api_key\":\"\${API_KEY}\",\"host\":\"\$(hostname -f)\",\"new_services\":[\${NEW_JSON}]}" \
    "http://\${PLATFORM_HOST}:\${PLATFORM_PORT}/api/agent/new-services" &>/dev/null || true
fi
echo "\$CURRENT" > "\$KNOWN_FILE"
REDISCOVER
chmod +x /usr/local/bin/agent-rediscover

systemctl list-units --type=service --state=running --no-legend --plain 2>/dev/null \
  | awk '{print $1}' | sort > /etc/otelcol/known-services.txt

cat > /etc/cron.d/observability-agent <<'CRONEOF'
0 12 * * * root /usr/local/bin/agent-rediscover >> /var/log/agent-rediscover.log 2>&1
CRONEOF

log "  Rediscovery cron installed — runs daily at 06:00"

# ============================================
# PHASE 6.5 — HOST REGISTRATION + HEARTBEAT
# ============================================
section "Phase 6.5: Registering host and setting heartbeat"

HOST_REGISTER_PAYLOAD=$(cat <<EOF
{
  "machine_id": "${MACHINE_ID}",
  "hostname": "${HOSTNAME_FQDN}",
  "os_name": "${OS_NAME}",
  "os_version": "${OS_VERSION_ID}",
  "arch": "${ARCH_NAME}",
  "agent_version": "${AGENT_VERSION}"
}
EOF
)

if curl -sf -X POST \
    -H "Content-Type: application/json" \
    -H "Authorization: Basic ${AUTH_B64}" \
    -d "${HOST_REGISTER_PAYLOAD}" \
    "${HOST_REGISTER_ENDPOINT}" &>/dev/null; then
  log "Host registered in platform backend"
else
  warn "Could not register host now (${HOST_REGISTER_ENDPOINT})"
fi

cat > /usr/local/bin/agent-heartbeat <<HEARTBEAT
#!/bin/bash
set -uo pipefail
API_KEY="${API_KEY}"
HOST_HEARTBEAT_ENDPOINT="${HOST_HEARTBEAT_ENDPOINT}"
MACHINE_ID="${MACHINE_ID}"
HOSTNAME_FQDN="${HOSTNAME_FQDN}"
AGENT_VERSION="${AGENT_VERSION}"
AUTH_B64=\$(echo -n "\${API_KEY}:\${API_KEY}" | base64 -w 0)

PAYLOAD=\$(cat <<JSON
{
  "machine_id": "\${MACHINE_ID}",
  "hostname": "\${HOSTNAME_FQDN}",
  "agent_version": "\${AGENT_VERSION}"
}
JSON
)

MAX_ATTEMPTS=3
for i in \$(seq 1 \$MAX_ATTEMPTS); do
  if curl -sf -X POST \
    -H "Content-Type: application/json" \
    -H "Authorization: Basic \${AUTH_B64}" \
    -d "\${PAYLOAD}" \
    "\${HOST_HEARTBEAT_ENDPOINT}" >/dev/null 2>&1; then
    exit 0
  fi
  if [ \$i -lt \$MAX_ATTEMPTS ]; then
    sleep \$((i * 5))
  fi
done
logger -t agent-heartbeat "Failed to send heartbeat after \$MAX_ATTEMPTS attempts"
HEARTBEAT
chmod +x /usr/local/bin/agent-heartbeat

cat > /etc/systemd/system/agent-heartbeat.service <<'HEARTBEAT_SVC'
[Unit]
Description=Observability Agent — Host heartbeat
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/agent-heartbeat
HEARTBEAT_SVC

cat > /etc/systemd/system/agent-heartbeat.timer <<'HEARTBEAT_TIMER'
[Unit]
Description=Observability Agent — Host heartbeat every 5 minutes

[Timer]
OnBootSec=2min
OnUnitActiveSec=5min
AccuracySec=30s
Persistent=true

[Install]
WantedBy=timers.target
HEARTBEAT_TIMER

HEARTBEAT_UNIT_READY=true

if [ "$DOCKER_WATCHER_UNIT_READY" = "true" ] || [ "$HEARTBEAT_UNIT_READY" = "true" ]; then
  systemctl daemon-reload
fi

if [ "$DOCKER_WATCHER_UNIT_READY" = "true" ]; then
  systemctl enable agent-docker-watcher
  if systemctl restart agent-docker-watcher && sleep 2 && systemctl is-active --quiet agent-docker-watcher; then
    log "  Docker event watcher started"
  else
    warn "  Docker event watcher failed to start"
    MANUAL_STEPS+=("Docker watcher: systemctl restart agent-docker-watcher")
  fi
fi

if [ "$HEARTBEAT_UNIT_READY" = "true" ]; then
  systemctl enable --now agent-heartbeat.timer
  if systemctl is-active --quiet agent-heartbeat.timer; then
    log "Heartbeat timer enabled (every 5 minutes)"
  else
    warn "Failed to start heartbeat timer"
  fi
fi

# ============================================
# PHASE 7 — DISCOVERY REPORT
# ============================================
section "Phase 7: Discovery report"

REPORT_LINES=()
REPORT_LINES+=("OBSERVABILITY AGENT — INSTALLATION REPORT")
REPORT_LINES+=("Generated : $(date -u +"%Y-%m-%dT%H:%M:%SZ")")
REPORT_LINES+=("Host      : ${HOSTNAME_FQDN}")
REPORT_LINES+=("Machine   : ${MACHINE_ID:0:8}...")
REPORT_LINES+=("Tenant    : ${API_KEY:0:8}...")
REPORT_LINES+=("Platform  : ${PLATFORM_HOST}:${PLATFORM_PORT}")
REPORT_LINES+=("")
REPORT_LINES+=("━━━ DISCOVERED SERVICES ━━━")
REPORT_LINES+=("(log paths only — metrics are host-level and container-level only)")
REPORT_LINES+=("")

for SVC in "${ALL_SERVICES[@]}"; do
  STATUS="${SVC_STATUS[$SVC]:-skipped}"
  PATHS="${LOG_PATHS[$SVC]:-none}"
  NOTES="${SVC_NOTES[$SVC]:-}"
  case $STATUS in
    logs_found)   ICON="✓" ;;
    no_logs)      ICON="~" ;;
    *)            ICON="✗" ;;
  esac
  REPORT_LINES+=("  ${ICON} ${SVC} (${STATUS})")
  REPORT_LINES+=("    logs  : $(echo "$PATHS" | head -3 | tr '\n' ' ')")
  [ -n "$NOTES" ] && REPORT_LINES+=("    notes : ${NOTES}")
  REPORT_LINES+=("")
done

if [ ${#MANUAL_STEPS[@]} -gt 0 ]; then
  REPORT_LINES+=("")
  REPORT_LINES+=("━━━ MANUAL STEPS REQUIRED ━━━")
  for STEP in "${MANUAL_STEPS[@]}"; do
    REPORT_LINES+=("  ! ${STEP}")
  done
fi

REPORT_LINES+=("")
REPORT_LINES+=("━━━ METRIC NAMING ━━━")
REPORT_LINES+=("  System     : system_cpu_*, system_memory_*, system_disk_*, system_network_*")
REPORT_LINES+=("  Containers : container_<name>_cpu_percent, container_<name>_mem_usage_bytes, ...")
REPORT_LINES+=("               container_<name>_net_rx_bytes, container_<name>_block_read_bytes")
REPORT_LINES+=("               container_<name>_pids")
REPORT_LINES+=("")
REPORT_LINES+=("━━━ AGENT COMPONENTS ━━━")
REPORT_LINES+=("  otelcol              : $(systemctl is-active otelcol 2>/dev/null || echo unknown)")
REPORT_LINES+=("  docker-watcher       : $(systemctl is-active agent-docker-watcher 2>/dev/null || echo not installed)")
REPORT_LINES+=("  docker-metrics       : $(systemctl is-active agent-docker-metrics.timer 2>/dev/null || echo not installed)")
REPORT_LINES+=("  docker-log-tailer    : $(systemctl is-active agent-docker-log-tailer 2>/dev/null || echo not installed)")
REPORT_LINES+=("  prom-file-server     : $(systemctl is-active agent-prom-file-server 2>/dev/null || echo not installed)")
REPORT_LINES+=("")
REPORT_LINES+=("━━━ OTELCOL COMMANDS ━━━")
REPORT_LINES+=("  Status  : sudo systemctl status otelcol")
REPORT_LINES+=("  Logs    : sudo journalctl -u otelcol -f")
REPORT_LINES+=("  Config  : sudo cat ${CONFIG_DIR}/config.yaml")
REPORT_LINES+=("  Restart : sudo systemctl restart otelcol")
REPORT_LINES+=("")
REPORT_LINES+=("━━━ CUSTOM APP METRICS (StatsD) ━━━")
REPORT_LINES+=("  Send to localhost:8125 (UDP)")
REPORT_LINES+=("  Node.js : npm install node-statsd")
REPORT_LINES+=("  Python  : pip install statsd")
REPORT_LINES+=("")
REPORT_LINES+=("━━━ APP TRACES / SDK METRICS (OTLP) ━━━")
REPORT_LINES+=("  On this host   : OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317")
REPORT_LINES+=("  Inside Docker  : OTEL_EXPORTER_OTLP_ENDPOINT=http://host.docker.internal:4317")

echo ""
for LINE in "${REPORT_LINES[@]}"; do
  echo "$LINE"
done

mkdir -p "$CONFIG_DIR"
printf '%s\n' "${REPORT_LINES[@]}" > "$REPORT_FILE"
log "Report saved to $REPORT_FILE"

JSON_HOST_SERVICES=""
JSON_CONTAINER_SERVICES=""
for SVC in "${ALL_SERVICES[@]}"; do
  ESC_NAME=$(json_escape "$SVC")
  ESC_STATUS=$(json_escape "${SVC_STATUS[$SVC]:-skipped}")
  ESC_LOG_PATHS=$(json_escape "${LOG_PATHS[$SVC]:-none}")
  ESC_NOTES=$(json_escape "${SVC_NOTES[$SVC]:-}")
  ESC_PORT=$(json_escape "${SVC_PORT[$SVC]:-}")
  ESC_VERSION=$(json_escape "${SVC_VERSION[$SVC]:-}")
  
  # Phase 3: Add virtual_hosts for web servers if available
  VHOSTS_FIELD=""
  if [[ "$SVC" == "nginx" || "$SVC" == "apache" ]] && [ -n "${WEBSERVER_VHOSTS_JSON[$SVC]:-}" ]; then
    VHOSTS_FIELD=",\"virtual_hosts\":${WEBSERVER_VHOSTS_JSON[$SVC]}"
  fi
  
  ITEM="{\"name\":\"${ESC_NAME}\",\"status\":\"${ESC_STATUS}\",\"log_paths\":\"${ESC_LOG_PATHS}\",\"notes\":\"${ESC_NOTES}\",\"port\":$( [ -n "${SVC_PORT[$SVC]:-}" ] && printf '%s' "${SVC_PORT[$SVC]}" || printf 'null' ),\"version\":\"${ESC_VERSION}\"${VHOSTS_FIELD}}"
  if [[ "$SVC" == docker_* ]]; then
    CNAME="${SVC#docker_}"
    ESC_CNAME=$(json_escape "$CNAME")
    ITEM="{\"name\":\"${ESC_CNAME}\",\"status\":\"${ESC_STATUS}\",\"log_paths\":\"${ESC_LOG_PATHS}\",\"notes\":\"${ESC_NOTES}\",\"port\":$( [ -n "${SVC_PORT[$SVC]:-}" ] && printf '%s' "${SVC_PORT[$SVC]}" || printf 'null' ),\"version\":\"${ESC_VERSION}\"${VHOSTS_FIELD}}"
    [ -n "$JSON_CONTAINER_SERVICES" ] && JSON_CONTAINER_SERVICES="${JSON_CONTAINER_SERVICES},"
    JSON_CONTAINER_SERVICES="${JSON_CONTAINER_SERVICES}${ITEM}"
  else
    [ -n "$JSON_HOST_SERVICES" ] && JSON_HOST_SERVICES="${JSON_HOST_SERVICES},"
    JSON_HOST_SERVICES="${JSON_HOST_SERVICES}${ITEM}"
  fi
done

JSON_MANUAL=""
for STEP in "${MANUAL_STEPS[@]}"; do
  [ -n "$JSON_MANUAL" ] && JSON_MANUAL="${JSON_MANUAL},"
  JSON_MANUAL="${JSON_MANUAL}\"$(json_escape "$STEP")\""
done

ESC_API_KEY=$(json_escape "$API_KEY")
ESC_HOSTNAME=$(json_escape "$HOSTNAME_FQDN")
ESC_MACHINE_ID=$(json_escape "$MACHINE_ID")
ESC_OS_NAME=$(json_escape "$OS_NAME")
ESC_ARCH_NAME=$(json_escape "$ARCH_NAME")
ESC_AGENT_VERSION=$(json_escape "$AGENT_VERSION")

HOST_DISCOVERY_PAYLOAD=$(cat <<EOF
{
  "api_key": "${ESC_API_KEY}",
  "host": "${ESC_HOSTNAME}",
  "machine_id": "${ESC_MACHINE_ID}",
  "os": "${ESC_OS_NAME}",
  "arch": "${ESC_ARCH_NAME}",
  "agent_version": "${ESC_AGENT_VERSION}",
  "generated_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "services": [${JSON_HOST_SERVICES}],
  "manual_steps": [${JSON_MANUAL}]
}
EOF
)

CONTAINER_DISCOVERY_PAYLOAD=$(cat <<EOF
{
  "api_key": "${ESC_API_KEY}",
  "host": "${ESC_HOSTNAME}",
  "machine_id": "${ESC_MACHINE_ID}",
  "os": "${ESC_OS_NAME}",
  "arch": "${ESC_ARCH_NAME}",
  "agent_version": "${ESC_AGENT_VERSION}",
  "generated_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "services": [${JSON_CONTAINER_SERVICES}],
  "manual_steps": [${JSON_MANUAL}]
}
EOF
)

HOST_SENT=false
if curl -sf -X POST \
    -H "Content-Type: application/json" \
    -H "Authorization: Basic ${AUTH_B64}" \
    -d "$HOST_DISCOVERY_PAYLOAD" \
    "${HOST_DISCOVERY_ENDPOINT}" &>/dev/null; then
  HOST_SENT=true
fi

CONTAINERS_SENT=false
if curl -sf -X POST \
    -H "Content-Type: application/json" \
    -H "Authorization: Basic ${AUTH_B64}" \
    -d "$CONTAINER_DISCOVERY_PAYLOAD" \
    "${CONTAINER_DISCOVERY_ENDPOINT}" &>/dev/null; then
  CONTAINERS_SENT=true
fi

if [ "$HOST_SENT" = true ]; then
  log "Host discovered services sent to platform backend"
else
  warn "Could not reach ${HOST_DISCOVERY_ENDPOINT} — host discovery kept local"
fi

if [ "$CONTAINERS_SENT" = true ]; then
  log "Container discovered services sent to platform backend"
else
  warn "Could not reach ${CONTAINER_DISCOVERY_ENDPOINT} — container discovery kept local"
fi

# ============================================
# INSTALLATION COMPLETE
# ============================================
echo ""
_BANNER_LN() { local T="$1"; local P=$((54 - 2 - ${#T})); printf "${GREEN}  ║${NC}  %s%${P}s${GREEN}║${NC}\n" "$T" ""; }
echo -e "${GREEN}  ╔══════════════════════════════════════════════════════════╗${NC}"
_BANNER_LN "✓  Installation Complete"
_BANNER_LN "Agent v${AGENT_VERSION} — ${HOSTNAME_FQDN}"
echo -e "${GREEN}  ╚══════════════════════════════════════════════════════════╝${NC}"
unset -f _BANNER_LN
echo ""
log "Agent v${AGENT_VERSION} installed successfully on ${HOSTNAME_FQDN}"
log "Platform: ${PLATFORM_HOST}:${PLATFORM_PORT}"
echo ""
echo -e "  ${BOLD}Quick Start:${NC}"
info "  View agent status  → sudo systemctl status otelcol"
info "  View agent logs    → sudo journalctl -u otelcol -f"
info ""
echo -e "  ${BOLD}Uninstall:${NC}"
info "  curl -sSL http://${PLATFORM_HOST}:${PLATFORM_PORT}/uninstall.sh | sudo bash"
echo ""
log "System monitoring is now active"
