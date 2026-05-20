#!/usr/bin/env bash
# deploy-price-submitters-all.sh — Push price submitter to all validator servers.
#
# Run from the primary VPS (147.93.139.18). Copies scripts via SCP, then
# executes deploy-price-submitter.sh on each remote server via SSH.
#
# Usage:
#   ./deploy-price-submitters-all.sh              # deploy everywhere
#   ./deploy-price-submitters-all.sh --dry-run    # preview only
#   ./deploy-price-submitters-all.sh primary      # deploy to one server only
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DRY_RUN=false
TARGET_SERVER=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=true; shift ;;
    *)         TARGET_SERVER="$1"; shift ;;
  esac
done

# Files to copy to each server
FILES=(
  "${SCRIPT_DIR}/price_submitter.sh"
  "${SCRIPT_DIR}/deploy-price-submitter.sh"
)

# Server definitions: "NAME|IP|SSH_USER"
SERVERS=(
  "primary|127.0.0.1|root"
  "us-east|94.72.119.124|root"
  "eu-east|161.97.117.111|root"
  "uk-east|161.97.116.202|root"
  "vpsc|185.205.246.214|root"
)

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
log()      { echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $*"; }
log_ok()   { echo -e "${GREEN}[$(date '+%H:%M:%S')] OK${NC} $*"; }
log_warn() { echo -e "${YELLOW}[$(date '+%H:%M:%S')] WARN${NC} $*"; }
log_err()  { echo -e "${RED}[$(date '+%H:%M:%S')] ERR${NC} $*"; }

REMOTE_DIR="/root/hyperpax-price-submitter"
SUCCESS=0
FAIL=0

for server_def in "${SERVERS[@]}"; do
  IFS='|' read -r name ip user <<< "$server_def"

  if [ -n "$TARGET_SERVER" ] && [ "$TARGET_SERVER" != "$name" ]; then
    continue
  fi

  echo ""
  log "════════════════════════════════════════════"
  log "  Deploying to: $name ($ip)"
  log "════════════════════════════════════════════"

  DRY_FLAG=""
  if [ "$DRY_RUN" = true ]; then
    DRY_FLAG="--dry-run"
  fi

  if [ "$ip" = "127.0.0.1" ]; then
    # Local deployment (primary server)
    log "Local deployment..."
    mkdir -p "$REMOTE_DIR"
    for f in "${FILES[@]}"; do
      cp "$f" "$REMOTE_DIR/"
    done
    chmod +x "$REMOTE_DIR"/*.sh

    if bash "$REMOTE_DIR/deploy-price-submitter.sh" --server "$name" $DRY_FLAG; then
      log_ok "$name: deployed"
      SUCCESS=$((SUCCESS + 1))
    else
      log_err "$name: deployment failed"
      FAIL=$((FAIL + 1))
    fi
  else
    # Remote deployment via SSH
    log "Copying files to $user@$ip..."
    if ! ssh -o ConnectTimeout=10 -o BatchMode=yes "$user@$ip" "mkdir -p $REMOTE_DIR" 2>/dev/null; then
      log_err "$name: SSH connection failed"
      FAIL=$((FAIL + 1))
      continue
    fi

    for f in "${FILES[@]}"; do
      scp -o ConnectTimeout=10 "$f" "$user@$ip:$REMOTE_DIR/" 2>/dev/null || {
        log_err "$name: SCP failed for $(basename "$f")"
        FAIL=$((FAIL + 1))
        continue 2
      }
    done

    ssh "$user@$ip" "chmod +x $REMOTE_DIR/*.sh"

    log "Running deploy script on $name..."
    if ssh -o ConnectTimeout=10 "$user@$ip" \
      "bash $REMOTE_DIR/deploy-price-submitter.sh --server $name $DRY_FLAG" 2>&1; then
      log_ok "$name: deployed"
      SUCCESS=$((SUCCESS + 1))
    else
      log_err "$name: deployment failed"
      FAIL=$((FAIL + 1))
    fi
  fi
done

echo ""
log "════════════════════════════════════════════"
log "  DONE: $SUCCESS succeeded, $FAIL failed"
log "════════════════════════════════════════════"
