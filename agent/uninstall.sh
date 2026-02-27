#!/bin/bash
# ============================================
# OBSERVABILITY PLATFORM - Agent Uninstaller
# Usage: sudo bash uninstall.sh
# Or:    curl -sSL https://platform/uninstall.sh | bash
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
echo "    - otelcol systemd service"
echo "    - /usr/local/bin/otelcol binary"
echo "    - /etc/otelcol/ config directory"
echo "    - otelcol user (if leftover from old install)"
echo ""

# Ask confirmation unless --force
if [ "$1" != "--force" ]; then
  read -r -p "  Are you sure? (yes/no): " CONFIRM
  if [ "$CONFIRM" != "yes" ]; then
    echo ""
    warn "Uninstall cancelled."
    exit 0
  fi
fi

echo ""

# ============================================
# STOP AND DISABLE SERVICE
# ============================================
section "Stopping service"

if systemctl is-active --quiet otelcol 2>/dev/null; then
  systemctl stop otelcol
  log "Service stopped"
else
  warn "Service was not running"
fi

if systemctl is-enabled --quiet otelcol 2>/dev/null; then
  systemctl disable otelcol
  log "Service disabled"
else
  warn "Service was not enabled"
fi

# ============================================
# REMOVE SYSTEMD SERVICE FILE
# ============================================
section "Removing systemd service"

if [ -f /etc/systemd/system/otelcol.service ]; then
  rm -f /etc/systemd/system/otelcol.service
  systemctl daemon-reload
  systemctl reset-failed otelcol 2>/dev/null || true
  log "Service file removed"
else
  warn "Service file not found, skipping"
fi

# ============================================
# REMOVE BINARY
# ============================================
section "Removing binary"

if [ -f /usr/local/bin/otelcol ]; then
  rm -f /usr/local/bin/otelcol
  log "Binary removed: /usr/local/bin/otelcol"
else
  warn "Binary not found, skipping"
fi

# ============================================
# REMOVE CONFIG DIRECTORY
# ============================================
section "Removing configuration"

if [ -d /etc/otelcol ]; then
  rm -rf /etc/otelcol
  log "Config directory removed: /etc/otelcol"
else
  warn "Config directory not found, skipping"
fi

# ============================================
# REMOVE otelcol USER (leftover from old installs)
# Current version runs as root so no user is created,
# but old installs created this user — clean it up
# ============================================
section "Checking for leftover system user"

if id otelcol &>/dev/null; then
  userdel otelcol 2>/dev/null || true
  log "Removed leftover otelcol user"
else
  warn "No otelcol user found, skipping"
fi

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

# Clear journald logs for this service
journalctl --rotate 2>/dev/null || true
journalctl --vacuum-time=1s --unit=otelcol 2>/dev/null || true
log "Journald logs cleared for otelcol"

# ============================================
# VERIFY NOTHING IS LEFT
# ============================================
section "Verifying removal"

LEFTOVERS=0
[ -f /usr/local/bin/otelcol ]              && warn "Binary still exists!"        && LEFTOVERS=1
[ -d /etc/otelcol ]                        && warn "Config dir still exists!"     && LEFTOVERS=1
[ -f /etc/systemd/system/otelcol.service ] && warn "Service file still exists!"   && LEFTOVERS=1
id otelcol &>/dev/null                     && warn "User otelcol still exists!"   && LEFTOVERS=1

if [ $LEFTOVERS -eq 0 ]; then
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
echo ""
echo "  To reinstall:"
echo "    (future) : curl -sSL https://platform/install.sh | API_KEY=your_key bash"
echo "    (now) sudo API_KEY=obs_test123 PLATFORM_HOST=localhost bash install.sh"

echo ""