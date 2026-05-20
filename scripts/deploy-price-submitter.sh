#!/usr/bin/env bash
# deploy-price-submitter.sh — Deploy oracle price submitter to a validator server.
#
# Auto-detects the server by IP address and configures the correct validators.
# Installs Foundry, exports ETH keys, creates systemd services.
#
# Usage:
#   ./deploy-price-submitter.sh              # auto-detect server
#   ./deploy-price-submitter.sh --dry-run    # show what would happen
#   ./deploy-price-submitter.sh --server primary  # force server identity
#
# Requires: docker, curl, jq (Foundry installed automatically)
set -euo pipefail

# ─── Globals ─────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="/root/hyperpax-price-submitter"
ENV_DIR="/etc/price-submitter"
DRY_RUN=false
FORCE_SERVER=""

# ─── Parse args ──────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)  DRY_RUN=true; shift ;;
    --server)   FORCE_SERVER="$2"; shift 2 ;;
    *)          echo "Unknown arg: $1"; exit 1 ;;
  esac
done

# ─── Server → Validator configs ──────────────────────────────────────────────
# Format: "INSTANCE_NAME|CONTAINER_NAME|KEY_NAME|EVM_PORT"
#
# INSTANCE_NAME = systemd instance suffix (price-submitter@INSTANCE_NAME)
# CONTAINER_NAME = docker container to export key from
# KEY_NAME = key name in the container's keyring (--keyring-backend test)
# EVM_PORT = localhost EVM JSON-RPC port
# ─────────────────────────────────────────────────────────────────────────────

declare -A SERVER_CONFIGS

# Primary VPS (147.93.139.18) — 4 validators
SERVER_CONFIGS["primary"]="
  primary-v1|evmos-validator1|validator1|8545
  primary-v2|evmos-validator2|validator2-new|8555
  primary-v3|evmos-validator3|validator3|8565
  primary-v4|evmos-validator4|validator4|8575
"

# US-East Ext (94.72.119.124) — 4 validators
SERVER_CONFIGS["us-east"]="
  us-east-v1|hyperpax-ext-validator1|ext-validator1|18545
  us-east-v2|hyperpax-ext-validator2|ext-validator2|18555
  us-east-v3|hyperpax-ext-validator3|ext-validator3|18565
  us-east-v4|hyperpax-ext-validator4|ext-validator4|18575
"

# EU-East VPS A (161.97.117.111) — 4 validators
SERVER_CONFIGS["eu-east"]="
  eu-east-v1|hyperpax-vpsA-validator1|vpsA-validator1|18545
  eu-east-v2|hyperpax-vpsA-validator2|vpsA-validator2|18555
  eu-east-v3|hyperpax-vpsA-validator3|vpsA-validator3|18565
  eu-east-v4|hyperpax-vpsA-validator4|vpsA-validator4|18575
"

# UK-East VPS B (161.97.116.202) — 4 validators
SERVER_CONFIGS["uk-east"]="
  uk-east-v1|hyperpax-vpsB-validator1|vpsB-validator1|18545
  uk-east-v2|hyperpax-vpsB-validator2|vpsB-validator2|18555
  uk-east-v3|hyperpax-vpsB-validator3|vpsB-validator3|18565
  uk-east-v4|hyperpax-vpsB-validator4|vpsB-validator4|18575
"

# VPS C (185.205.246.214) — 4 validators
SERVER_CONFIGS["vpsc"]="
  vpsc-v1|hyperpax-ext-validator1|ext-validator1|18545
  vpsc-v2|hyperpax-ext-validator2|ext-validator2|18555
  vpsc-v3|hyperpax-ext-validator3|ext-validator3|18565
  vpsc-v4|hyperpax-ext-validator4|ext-validator4|18575
"

# IP → server name mapping
declare -A IP_TO_SERVER
IP_TO_SERVER["147.93.139.18"]="primary"
IP_TO_SERVER["94.72.119.124"]="us-east"
IP_TO_SERVER["161.97.117.111"]="eu-east"
IP_TO_SERVER["161.97.116.202"]="uk-east"
IP_TO_SERVER["185.205.246.214"]="vpsc"

# ─── Helpers ─────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
log()      { echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $*"; }
log_ok()   { echo -e "${GREEN}[$(date '+%H:%M:%S')] OK${NC} $*"; }
log_warn() { echo -e "${YELLOW}[$(date '+%H:%M:%S')] WARN${NC} $*"; }
log_err()  { echo -e "${RED}[$(date '+%H:%M:%S')] ERR${NC} $*"; }
die()      { log_err "$*"; exit 1; }

# ─── Detect server identity ─────────────────────────────────────────────────
detect_server() {
  if [ -n "$FORCE_SERVER" ]; then
    if [ -z "${SERVER_CONFIGS[$FORCE_SERVER]+x}" ]; then
      die "Unknown server: $FORCE_SERVER (known: ${!SERVER_CONFIGS[*]})"
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

  die "Cannot auto-detect server. Known IPs: ${!IP_TO_SERVER[*]}. Use --server <name>"
}

# ─── Phase 1: Install Foundry ───────────────────────────────────────────────
install_foundry() {
  if command -v cast &>/dev/null; then
    log_ok "Foundry already installed: $(cast --version 2>/dev/null | head -1)"
    return
  fi

  # Check if foundryup exists but cast not in PATH
  if [ -f "$HOME/.foundry/bin/cast" ]; then
    export PATH="$HOME/.foundry/bin:$PATH"
    log_ok "Foundry found at $HOME/.foundry/bin"
    return
  fi

  log "Installing Foundry..."
  if [ "$DRY_RUN" = true ]; then
    log_warn "DRY_RUN: would install Foundry"
    return
  fi

  curl -L https://foundry.paradigm.xyz | bash
  export PATH="$HOME/.foundry/bin:$PATH"
  "$HOME/.foundry/bin/foundryup"
  log_ok "Foundry installed: $(cast --version 2>/dev/null | head -1)"
}

# ─── Phase 2: Copy price_submitter.sh ───────────────────────────────────────
install_script() {
  local src="${SCRIPT_DIR}/price_submitter.sh"
  local dst="${INSTALL_DIR}/price_submitter.sh"

  if [ ! -f "$src" ]; then
    die "price_submitter.sh not found at $src"
  fi

  log "Installing price_submitter.sh to $INSTALL_DIR"
  if [ "$DRY_RUN" = true ]; then
    log_warn "DRY_RUN: would copy $src → $dst"
    return
  fi

  mkdir -p "$INSTALL_DIR"
  if [ "$src" -ef "$dst" ]; then
    log_ok "Script already in place at $dst"
  else
    cp "$src" "$dst"
    log_ok "Script installed"
  fi
  chmod +x "$dst"
}

# ─── Phase 3: Install systemd template ──────────────────────────────────────
install_systemd_template() {
  local src="${SCRIPT_DIR}/price-submitter@.service"
  local dst="/etc/systemd/system/price-submitter@.service"

  log "Installing systemd template"
  if [ "$DRY_RUN" = true ]; then
    log_warn "DRY_RUN: would install $dst"
    return
  fi

  # Write the template unit (self-contained, no external file needed)
  cat > "$dst" << 'UNIT'
[Unit]
Description=HyperPax Price Submitter — %i
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/price-submitter/%i.env
ExecStart=/root/hyperpax-price-submitter/price_submitter.sh
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=price-submitter-%i
Environment=PATH=/root/.foundry/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

[Install]
WantedBy=multi-user.target
UNIT

  systemctl daemon-reload
  log_ok "Systemd template installed"
}

# ─── Phase 4: Export keys and create env files ───────────────────────────────
setup_validators() {
  local server_name="$1"
  local config="${SERVER_CONFIGS[$server_name]}"

  mkdir -p "$ENV_DIR"

  local count=0
  local failed=0

  while IFS='|' read -r instance container key_name evm_port; do
    # Skip empty lines
    instance=$(echo "$instance" | xargs)
    [ -z "$instance" ] && continue

    container=$(echo "$container" | xargs)
    key_name=$(echo "$key_name" | xargs)
    evm_port=$(echo "$evm_port" | xargs)

    log "─── $instance ───"
    log "  Container: $container | Key: $key_name | EVM: $evm_port"

    # Check container is running
    if ! docker inspect "$container" &>/dev/null; then
      log_warn "  Container $container not found — skipping"
      failed=$((failed + 1))
      continue
    fi

    if [ "$(docker inspect -f '{{.State.Running}}' "$container" 2>/dev/null)" != "true" ]; then
      log_warn "  Container $container not running — skipping"
      failed=$((failed + 1))
      continue
    fi

    # Export ETH private key
    log "  Exporting ETH key..."
    if [ "$DRY_RUN" = true ]; then
      log_warn "  DRY_RUN: would export key $key_name from $container"
      count=$((count + 1))
      continue
    fi

    local eth_key
    eth_key=$(docker exec "$container" evmosd keys unsafe-export-eth-key "$key_name" \
      --keyring-backend test --home /root/.evmosd 2>/dev/null) || {
      log_err "  Failed to export key $key_name from $container"
      failed=$((failed + 1))
      continue
    }

    if [ -z "$eth_key" ]; then
      log_err "  Empty key returned for $key_name"
      failed=$((failed + 1))
      continue
    fi

    # Create env file (permissions: root-only read)
    local env_file="${ENV_DIR}/${instance}.env"
    cat > "$env_file" << EOF
VALIDATOR_KEY=0x${eth_key}
RPC_URL=http://127.0.0.1:${evm_port}
INTERVAL=20
DRY_RUN=false
LOG_FILE=/var/log/price-submitter-${instance}.log
EOF
    chmod 600 "$env_file"

    # Verify the key resolves to an address
    local addr
    addr=$(cast wallet address "0x${eth_key}" 2>/dev/null) || addr="unknown"
    log_ok "  Key exported: $addr"
    log_ok "  Env file: $env_file"

    # Enable and start the service
    systemctl enable "price-submitter@${instance}" 2>/dev/null || true
    systemctl restart "price-submitter@${instance}"
    log_ok "  Service started: price-submitter@${instance}"

    count=$((count + 1))
  done <<< "$config"

  echo ""
  log "════════════════════════════════════════════════════════════════"
  log "  Deployed: $count validators, $failed skipped"
  log "════════════════════════════════════════════════════════════════"

  if [ "$count" -gt 0 ] && [ "$DRY_RUN" != true ]; then
    echo ""
    log "Check status with:"
    for instance in $(echo "$config" | awk -F'|' '{print $1}' | xargs); do
      [ -z "$instance" ] && continue
      echo "  systemctl status price-submitter@${instance}"
    done
    echo ""
    log "View logs with:"
    echo "  journalctl -u 'price-submitter@*' -f"
  fi
}

# ─── Main ────────────────────────────────────────────────────────────────────
main() {
  echo ""
  echo "╔══════════════════════════════════════════════════╗"
  echo "║   HyperPax Price Submitter — Deployment Tool    ║"
  echo "╚══════════════════════════════════════════════════╝"
  echo ""

  if [ "$DRY_RUN" = true ]; then
    log_warn "DRY RUN MODE — no changes will be made"
    echo ""
  fi

  local server
  server=$(detect_server)
  log "Detected server: $server"
  log "Validators: $(echo "${SERVER_CONFIGS[$server]}" | grep -c '|' || echo 0)"
  echo ""

  install_foundry
  install_script
  install_systemd_template
  setup_validators "$server"
}

main
