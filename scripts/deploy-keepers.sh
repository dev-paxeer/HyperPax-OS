#!/usr/bin/env bash
# deploy-keepers.sh — Deploy settlement + batch clearing keeper sidecars.
#
# Installs keeper scripts and systemd services on the current server.
# Reuses the existing /etc/price-submitter/*.env files (same VALIDATOR_KEY + RPC_URL).
# Only needs to run ONCE per validator instance — one settlement keeper and one
# batch clearing keeper per validator (they share the same key).
#
# Usage:
#   ./deploy-keepers.sh              # auto-detect server, deploy all
#   ./deploy-keepers.sh --dry-run    # preview only
#   ./deploy-keepers.sh --server primary  # force server identity
#
# Prerequisites:
#   - price_submitter already deployed (env files at /etc/price-submitter/*.env)
#   - Foundry installed (cast available)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="/root/hyperpax-price-submitter"
ENV_DIR="/etc/price-submitter"
DRY_RUN=false
FORCE_SERVER=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)  DRY_RUN=true; shift ;;
    --server)   FORCE_SERVER="$2"; shift 2 ;;
    *)          echo "Unknown arg: $1"; exit 1 ;;
  esac
done

# ─── Server → instance mapping (same as deploy-price-submitter.sh) ───────────
declare -A SERVER_INSTANCES
SERVER_INSTANCES["primary"]="primary-v1 primary-v2 primary-v3 primary-v4"
SERVER_INSTANCES["us-east"]="us-east-v1 us-east-v2 us-east-v3 us-east-v4"
SERVER_INSTANCES["eu-east"]="eu-east-v1 eu-east-v2 eu-east-v3 eu-east-v4"
SERVER_INSTANCES["uk-east"]="uk-east-v1 uk-east-v2 uk-east-v3 uk-east-v4"
SERVER_INSTANCES["vpsc"]="vpsc-v1 vpsc-v2 vpsc-v3 vpsc-v4"
SERVER_INSTANCES["vpsh"]="vpsh-v1 vpsh-v2 vpsh-v3 vpsh-v4"
SERVER_INSTANCES["vpsi"]="vpsi-v1 vpsi-v2 vpsi-v3 vpsi-v4"

declare -A IP_TO_SERVER
IP_TO_SERVER["147.93.139.18"]="primary"
IP_TO_SERVER["94.72.119.124"]="us-east"
IP_TO_SERVER["161.97.117.111"]="eu-east"
IP_TO_SERVER["161.97.116.202"]="uk-east"
IP_TO_SERVER["185.205.246.214"]="vpsc"
IP_TO_SERVER["94.250.202.17"]="vpsh"
IP_TO_SERVER["109.205.181.189"]="vpsi"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
log()      { echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $*"; }
log_ok()   { echo -e "${GREEN}[$(date '+%H:%M:%S')] OK${NC} $*"; }
log_warn() { echo -e "${YELLOW}[$(date '+%H:%M:%S')] WARN${NC} $*"; }
log_err()  { echo -e "${RED}[$(date '+%H:%M:%S')] ERR${NC} $*"; }
die()      { log_err "$*"; exit 1; }

detect_server() {
  if [ -n "$FORCE_SERVER" ]; then
    if [ -z "${SERVER_INSTANCES[$FORCE_SERVER]+x}" ]; then
      die "Unknown server: $FORCE_SERVER (known: ${!SERVER_INSTANCES[*]})"
    fi
    echo "$FORCE_SERVER"
    return
  fi
  local my_ips
  my_ips=$(hostname -I 2>/dev/null || ip -4 addr show | grep -oP '(?<=inet\s)\d+\.\d+\.\d+\.\d+' || true)
  for ip in $my_ips; do
    if [ -n "${IP_TO_SERVER[$ip]+x}" ]; then
      echo "${IP_TO_SERVER[$ip]}"
      return
    fi
  done
  die "Cannot auto-detect server. Use --server <name>"
}

# ─── Phase 1: Install scripts ────────────────────────────────────────────────
install_scripts() {
  log "Installing keeper scripts to $INSTALL_DIR"
  if [ "$DRY_RUN" = true ]; then
    log_warn "DRY_RUN: would copy scripts"
    return
  fi
  mkdir -p "$INSTALL_DIR"

  for script in settlement_keeper.sh batch_clearing_keeper.sh; do
    local src="${SCRIPT_DIR}/${script}"
    local dst="${INSTALL_DIR}/${script}"
    if [ ! -f "$src" ]; then
      die "$script not found at $src"
    fi
    # Avoid cp error when src and dst are the same file
    if [ ! "$src" -ef "$dst" ]; then
      cp "$src" "$dst"
    fi
    chmod +x "$dst"
    cp "$dst" "/usr/local/bin/${script}"
    chmod +x "/usr/local/bin/${script}"
  done
  log_ok "Scripts installed"
}

# ─── Phase 2: Install systemd templates ──────────────────────────────────────
install_systemd_templates() {
  log "Installing systemd service templates"
  if [ "$DRY_RUN" = true ]; then
    log_warn "DRY_RUN: would install systemd templates"
    return
  fi

  cat > /etc/systemd/system/settlement-keeper@.service << 'UNIT'
[Unit]
Description=PaxSpot Settlement Keeper — %i
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/price-submitter/%i.env
ExecStart=/usr/local/bin/settlement_keeper.sh
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=settlement-keeper-%i
Environment=PATH=/root/.foundry/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

[Install]
WantedBy=multi-user.target
UNIT

  cat > /etc/systemd/system/batch-clearing-keeper@.service << 'UNIT'
[Unit]
Description=PaxSpot Batch Clearing Keeper — %i
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/price-submitter/%i.env
ExecStart=/usr/local/bin/batch_clearing_keeper.sh
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=batch-clearing-keeper-%i
Environment=PATH=/root/.foundry/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

[Install]
WantedBy=multi-user.target
UNIT

  systemctl daemon-reload
  log_ok "Systemd templates installed"
}

# ─── Phase 3: Enable and start keepers for each validator instance ───────────
start_keepers() {
  local server_name="$1"
  local instances="${SERVER_INSTANCES[$server_name]}"
  local started=0
  local skipped=0

  for instance in $instances; do
    local env_file="${ENV_DIR}/${instance}.env"

    if [ ! -f "$env_file" ]; then
      log_warn "$instance: no env file at $env_file — skipping (run deploy-price-submitter.sh first)"
      skipped=$((skipped + 1))
      continue
    fi

    log "─── $instance ───"

    if [ "$DRY_RUN" = true ]; then
      log_warn "  DRY_RUN: would start settlement-keeper@${instance} + batch-clearing-keeper@${instance}"
      started=$((started + 1))
      continue
    fi

    # Settlement keeper
    systemctl enable "settlement-keeper@${instance}" 2>/dev/null || true
    systemctl restart "settlement-keeper@${instance}"
    log_ok "  settlement-keeper@${instance} started"

    # Batch clearing keeper
    systemctl enable "batch-clearing-keeper@${instance}" 2>/dev/null || true
    systemctl restart "batch-clearing-keeper@${instance}"
    log_ok "  batch-clearing-keeper@${instance} started"

    started=$((started + 1))
  done

  echo ""
  log "════════════════════════════════════════════════════════════════"
  log "  Keepers deployed: $started instances, $skipped skipped"
  log "  Services per instance: settlement-keeper + batch-clearing-keeper"
  log "════════════════════════════════════════════════════════════════"

  if [ "$started" -gt 0 ] && [ "$DRY_RUN" != true ]; then
    echo ""
    log "Check status:"
    echo "  systemctl status 'settlement-keeper@*' 'batch-clearing-keeper@*'"
    echo ""
    log "View logs:"
    echo "  journalctl -u 'settlement-keeper@*' -u 'batch-clearing-keeper@*' -f"
  fi
}

# ─── Main ─────────────────────────────────────────────────────────────────────
main() {
  echo ""
  echo "╔══════════════════════════════════════════════════════╗"
  echo "║   PaxSpot Keeper Sidecars — Deployment Tool          ║"
  echo "║   Settlement Keeper + Batch Clearing Keeper          ║"
  echo "╚══════════════════════════════════════════════════════╝"
  echo ""

  if [ "$DRY_RUN" = true ]; then
    log_warn "DRY RUN MODE — no changes will be made"
    echo ""
  fi

  local server
  server=$(detect_server)
  log "Detected server: $server"
  log "Instances: ${SERVER_INSTANCES[$server]}"
  echo ""

  install_scripts
  install_systemd_templates
  start_keepers "$server"
}

main
