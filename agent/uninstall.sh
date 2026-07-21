#!/bin/bash
# ============================================
# OBSERVABILITY PLATFORM — Agent Uninstaller
# ============================================
# Usage:
#   curl -sSL https://platform/uninstall.sh | sudo bash
#   sudo bash uninstall.sh
#   sudo bash uninstall.sh --auto-yes
#   sudo bash uninstall.sh --purge --auto-yes
# ============================================
# Flags:
#   --auto-yes / -y / --force  Skip all prompts
#   --purge                    Remove /var/lib/otelcol and agent rediscover log
#   --no-journal               Do not vacuum journald logs
# ============================================

set -euo pipefail

# ---- Colors ----
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

log()     { echo -e "${GREEN}[✓]${NC} $1"; }
warn()    { echo -e "${YELLOW}[!]${NC} $1"; }
error()   { echo -e "${RED}[✗]${NC} $1"; exit 1; }
section() { echo -e "\n${BOLD}${BLUE}━━━ $1 ━━━${NC}"; }
step()    { echo -e "\n${BOLD}${BLUE}◆ $1${NC}"; }

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
unit_known() { systemctl status "$1" >/dev/null 2>&1; }
unit_enabled() { systemctl is-enabled "$1" >/dev/null 2>&1; }
unit_active() { systemctl is-active "$1" >/dev/null 2>&1; }

stop_disable_unit() {
  local UNIT=$1
  systemctl_ok || return 0

  if unit_known "$UNIT" || unit_enabled "$UNIT" || unit_active "$UNIT"; then
    systemctl stop "$UNIT" 2>/dev/null || true
    systemctl disable "$UNIT" 2>/dev/null || true
    log "Stop/disable requested for $UNIT"
  else
    warn "$UNIT not found, skipping"
  fi
}

stop_disable_timer() {
  local TIMER=$1
  systemctl_ok || return 0

  if unit_known "$TIMER" || unit_enabled "$TIMER" || unit_active "$TIMER"; then
    systemctl stop "$TIMER" 2>/dev/null || true
    systemctl disable "$TIMER" 2>/dev/null || true
    log "Stop/disable requested for $TIMER"
  else
    warn "$TIMER not found, skipping"
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
section "Checking requirements"

[ "$EUID" -ne 0 ] && error "Please run as root or with sudo"

echo ""
echo -e "${BOLD}${RED}  ⚠  Agent Uninstaller${NC}"
echo -e "${BOLD}  This will remove the observability agent from this host${NC}"
echo ""
echo "  Components to remove:"
echo "    ${BOLD}•${NC} otelcol binary and systemd service"
echo "    ${BOLD}•${NC} All agent helper services: docker log tailer, metrics,"
echo "      event watcher, prometheus file server, heartbeat"
echo "    ${BOLD}•${NC} /etc/otelcol/ configuration and /usr/local/bin/agent-* scripts"
echo "    ${BOLD}•${NC} /etc/cron.d/observability-agent (daily rediscovery)"
echo ""
echo "  ${YELLOW}Does NOT delete your telemetry data in VictoriaMetrics,${NC}"
echo "  ${YELLOW}Loki, or Tempo — only the agent on this machine.${NC}"
echo ""
echo "  ${YELLOW}Optional (--purge):${NC} removes /var/lib/otelcol/ and logs${NC}"
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
stop_disable_unit agent-heartbeat.service
stop_disable_timer agent-docker-metrics.timer
stop_disable_timer agent-heartbeat.timer

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
remove_file /etc/systemd/system/agent-heartbeat.service
remove_file /etc/systemd/system/agent-heartbeat.timer

if systemctl_ok; then
  systemctl daemon-reload 2>/dev/null || true
  systemctl reset-failed otelcol 2>/dev/null || true
  systemctl reset-failed agent-docker-log-tailer 2>/dev/null || true
  systemctl reset-failed agent-docker-watcher 2>/dev/null || true
  systemctl reset-failed agent-prom-file-server 2>/dev/null || true
  systemctl reset-failed agent-docker-metrics 2>/dev/null || true
  systemctl reset-failed agent-heartbeat 2>/dev/null || true
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
remove_file /usr/local/bin/agent-heartbeat

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
  journalctl --vacuum-time=1s --unit=agent-heartbeat 2>/dev/null || true
  log "Journald logs cleared for agent services"
fi

# ============================================
# VERIFY NOTHING IS LEFT
# ============================================
section "Verifying removal"

LEFTOVERS=0
[ -f /usr/local/bin/otelcol ] && warn "Binary still exists!" && LEFTOVERS=1
[ -d /etc/otelcol ] && warn "Config dir still exists!" && LEFTOVERS=1
[ -n "$(pgrep -x otelcol 2>/dev/null || true)" ] && warn "otelcol process still running!" && LEFTOVERS=1
[ -f /etc/systemd/system/otelcol.service ] && warn "Service file still exists!" && LEFTOVERS=1
[ -f /etc/systemd/system/agent-docker-log-tailer.service ] && warn "Tailer service file still exists!" && LEFTOVERS=1
[ -f /etc/systemd/system/agent-docker-metrics.timer ] && warn "Metrics timer still exists!" && LEFTOVERS=1
[ -f /etc/systemd/system/agent-heartbeat.timer ] && warn "Heartbeat timer still exists!" && LEFTOVERS=1
[ -f /etc/cron.d/observability-agent ] && warn "Cron file still exists!" && LEFTOVERS=1

if [ "$LEFTOVERS" -eq 0 ]; then
  log "All components removed cleanly"
fi

# ============================================
# DONE
# ============================================
section "Uninstall Complete"
log "Agent removed successfully from ${HOSTNAME_FQDN:-$(hostname)}"
echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  ✓  Agent has been removed. Host is no longer monitored.${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  ${YELLOW}Note:${NC} Your telemetry data in VictoriaMetrics, Loki, and Tempo"
echo "  is preserved. Only the agent on this machine was removed."
echo ""
echo "  ${YELLOW}Note:${NC} The uninstaller does not revert service config changes"
echo "  made during install (PostgreSQL logging_collector, redis logfile, etc.)."
echo ""
echo "  ${BOLD}To reinstall:${NC}"
echo "    curl -sSL https://<platform>/install.sh | API_KEY=<your-key> sudo bash"
echo ""
