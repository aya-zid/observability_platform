#!/bin/bash
# ============================================
# OBSERVABILITY PLATFORM - Agent Uninstaller
# Usage:
#   sudo bash uninstall.sh
#   sudo bash uninstall.sh --auto-yes
#   sudo bash uninstall.sh --purge --auto-yes
# Flags:
#   --auto-yes / -y / --force  Skip prompts
#   --purge                  Also remove /var/lib/otelcol and agent rediscover log
#   --no-journal             Don't vacuum journald logs
# Or:    curl -sSL https://platform/uninstall.sh | bash
# ============================================

set -euo pipefail

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

# ---- Flags ----
AUTO_YES=false
PURGE_DATA=false
NO_JOURNAL=false

for ARG in "$@"; do
  case "$ARG" in
    --force|--auto-yes|-y) AUTO_YES=true ;;
    --purge)               PURGE_DATA=true ;;
    --no-journal)          NO_JOURNAL=true ;;
  esac
done

ask() {
  local QUESTION=$1
  if $AUTO_YES; then
    warn "$QUESTION → auto-yes"
    return 0
  fi
  read -r -p "  ${QUESTION} (yes/no): " CONFIRM </dev/tty || true
  [ "${CONFIRM:-}" = "yes" ]
}

systemctl_ok() { command -v systemctl &>/dev/null; }

stop_disable_unit() {
  local UNIT=$1
  systemctl_ok || return 0

  if systemctl list-unit-files --type=service --no-legend 2>/dev/null | awk '{print $1}' | grep -qx "$UNIT"; then
    systemctl stop "$UNIT" 2>/dev/null || true
    systemctl disable "$UNIT" 2>/dev/null || true
    log "Stopped/disabled $UNIT"
  else
    warn "$UNIT not installed, skipping"
  fi
}

stop_disable_timer() {
  local TIMER=$1
  systemctl_ok || return 0

  if systemctl list-unit-files --type=timer --no-legend 2>/dev/null | awk '{print $1}' | grep -qx "$TIMER"; then
    systemctl stop "$TIMER" 2>/dev/null || true
    systemctl disable "$TIMER" 2>/dev/null || true
    log "Stopped/disabled $TIMER"
  else
    warn "$TIMER not installed, skipping"
  fi
}

remove_file() {
  local P=$1
  if [ -e "$P" ]; then
    rm -f "$P"
    log "Removed $P"
  else
    warn "Not found: $P"
  fi
}

remove_dir() {
  local P=$1
  if [ -d "$P" ]; then
    rm -rf "$P"
    log "Removed $P/"
  else
    warn "Not found: $P/"
  fi
}

# ============================================
# CHECKS
# ============================================
if [ "$EUID" -ne 0 ]; then
  error "Please run as root or with sudo"
fi

echo ""
echo -e "${RED}============================================${NC}"
echo -e "${RED}  Observability Agent - Uninstaller${NC}"
echo -e "${RED}============================================${NC}"
echo ""
echo "  This will remove:"
echo "    - otelcol systemd service + config"
echo "    - /usr/local/bin/otelcol binary"
echo "    - helper services: docker log tailer/metrics/watcher, prom file server"
echo "    - helper scripts in /usr/local/bin/"
echo "    - cron: /etc/cron.d/observability-agent"
echo "    - /etc/otelcol/ config directory"
echo ""
echo "  Optional (--purge):"
echo "    - /var/lib/otelcol/ (docker-logs + docker-metrics.prom)"
echo "    - /var/log/agent-rediscover.log"
echo ""

if ! ask "Are you sure you want to uninstall the agent?"; then
  echo ""
  warn "Uninstall cancelled."
  exit 0
fi

if $PURGE_DATA; then
  if ! ask "Purge /var/lib/otelcol and agent rediscover logs?"; then
    PURGE_DATA=false
    warn "Purge disabled"
  fi
fi

echo ""

# ============================================
# STOP AND DISABLE SERVICE
# ============================================
section "Stopping service"

stop_disable_unit otelcol.service
stop_disable_unit agent-docker-log-tailer.service
stop_disable_unit agent-docker-watcher.service
stop_disable_unit agent-prom-file-server.service
stop_disable_unit agent-docker-metrics.service
stop_disable_timer agent-docker-metrics.timer

# ============================================
# REMOVE SYSTEMD SERVICE FILE
# ============================================
section "Removing systemd service"

remove_file /etc/systemd/system/otelcol.service
remove_file /etc/systemd/system/agent-docker-log-tailer.service
remove_file /etc/systemd/system/agent-docker-watcher.service
remove_file /etc/systemd/system/agent-prom-file-server.service
remove_file /etc/systemd/system/agent-docker-metrics.service
remove_file /etc/systemd/system/agent-docker-metrics.timer

if systemctl_ok; then
  systemctl daemon-reload 2>/dev/null || true
  systemctl reset-failed otelcol 2>/dev/null || true
  systemctl reset-failed agent-docker-log-tailer 2>/dev/null || true
  systemctl reset-failed agent-docker-watcher 2>/dev/null || true
  systemctl reset-failed agent-prom-file-server 2>/dev/null || true
  systemctl reset-failed agent-docker-metrics 2>/dev/null || true
fi

# ============================================
# REMOVE BINARY
# ============================================
section "Removing binary"

remove_file /usr/local/bin/otelcol
remove_file /usr/local/bin/agent-docker-log-tailer
remove_file /usr/local/bin/agent-docker-metrics
remove_file /usr/local/bin/agent-prom-file-server
remove_file /usr/local/bin/agent-docker-update
remove_file /usr/local/bin/agent-rediscover

# ============================================
# REMOVE CONFIG DIRECTORY
# ============================================
section "Removing configuration"

remove_dir /etc/otelcol
remove_file /etc/cron.d/observability-agent

# ============================================
# CLEAN UP LOG FILES
# ============================================
section "Cleaning up logs"

if [ -d /var/log/otelcol ]; then
  rm -rf /var/log/otelcol
  log "Log directory removed: /var/log/otelcol"
else
  warn "No log directory found, skipping"
fi

if $PURGE_DATA; then
  remove_dir /var/lib/otelcol
  remove_file /var/log/agent-rediscover.log
fi

if ! $NO_JOURNAL && command -v journalctl &>/dev/null; then
  journalctl --rotate 2>/dev/null || true
  journalctl --vacuum-time=1s --unit=otelcol 2>/dev/null || true
  journalctl --vacuum-time=1s --unit=agent-docker-log-tailer 2>/dev/null || true
  journalctl --vacuum-time=1s --unit=agent-docker-metrics 2>/dev/null || true
  journalctl --vacuum-time=1s --unit=agent-docker-watcher 2>/dev/null || true
  journalctl --vacuum-time=1s --unit=agent-prom-file-server 2>/dev/null || true
  log "Journald logs cleared for agent services"
fi

# ============================================
# VERIFY NOTHING IS LEFT
# ============================================
section "Verifying removal"

LEFTOVERS=0
[ -f /usr/local/bin/otelcol ] && warn "Binary still exists!" && LEFTOVERS=1
[ -d /etc/otelcol ] && warn "Config dir still exists!" && LEFTOVERS=1
[ -f /etc/systemd/system/otelcol.service ] && warn "Service file still exists!" && LEFTOVERS=1
[ -f /etc/systemd/system/agent-docker-log-tailer.service ] && warn "Tailer service file still exists!" && LEFTOVERS=1
[ -f /etc/systemd/system/agent-docker-metrics.timer ] && warn "Metrics timer still exists!" && LEFTOVERS=1
[ -f /etc/cron.d/observability-agent ] && warn "Cron file still exists!" && LEFTOVERS=1

if [ "$LEFTOVERS" -eq 0 ]; then
  log "All components removed cleanly"
fi

# ============================================
# DONE
# ============================================
echo ""
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  Agent uninstalled successfully!${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""
echo "  Note: Your data in VictoriaMetrics, Loki, and Tempo"
echo "  is NOT deleted — only the agent on this machine."
echo "  Note: Uninstaller does not revert any service config"
echo "  changes made during install (e.g. PostgreSQL logging_collector, redis logfile)."
echo ""
echo "  To reinstall:"
echo "    (future) : curl -sSL https://platform/install.sh | API_KEY=your_key bash"
echo "    (now) sudo API_KEY=obs_test123 PLATFORM_HOST=localhost bash install.sh"

echo ""
