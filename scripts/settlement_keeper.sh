#!/usr/bin/env bash
# settlement_keeper.sh — Validator keeper sidecar for epoch settlement.
#
# Calls SettlementEngine.settleEpoch() when the current epoch is ready.
# Checks blocksUntilEpoch() first to avoid wasting gas on reverts.
#
# Usage:
#   VALIDATOR_KEY=0x... RPC_URL=http://... ./scripts/settlement_keeper.sh
#
# Environment variables:
#   VALIDATOR_KEY  — Validator's ETH private key (hex, with 0x prefix). REQUIRED.
#   RPC_URL        — EVM JSON-RPC endpoint (default: http://127.0.0.1:8545)
#   INTERVAL       — Seconds between checks (default: 10 = ~5 blocks at 2s)
#   GAS_LIMIT      — Gas limit per settleEpoch tx (default: 500000)
#   LOG_FILE       — Log file path (default: /var/log/settlement_keeper.log)
#   DRY_RUN        — If "true", log but don't submit (default: false)
#
# Requires: cast (foundry), curl, jq
set -euo pipefail

# ─── Configuration ────────────────────────────────────────────────────────────
VALIDATOR_KEY="${VALIDATOR_KEY:?ERROR: VALIDATOR_KEY not set}"
RPC_URL="${RPC_URL:-http://127.0.0.1:8545}"
INTERVAL="${INTERVAL:-10}"
GAS_LIMIT="${GAS_LIMIT:-500000}"
LOG_FILE="${LOG_FILE:-/var/log/settlement_keeper.log}"
DRY_RUN="${DRY_RUN:-false}"

SETTLEMENT_ENGINE="0x2F8658Ea1668E81f0a47709FD06167017F190Afc"

# ─── Functions ────────────────────────────────────────────────────────────────
log() {
  local ts
  ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  echo "[$ts] $*" | tee -a "$LOG_FILE"
}

get_blocks_until_epoch() {
  cast call "$SETTLEMENT_ENGINE" "blocksUntilEpoch()(uint256)" \
    --rpc-url "$RPC_URL" 2>/dev/null | head -1 | tr -d ' '
}

get_pending_count() {
  cast call "$SETTLEMENT_ENGINE" "pendingSettlementCount()(uint256)" \
    --rpc-url "$RPC_URL" 2>/dev/null | head -1 | tr -d ' '
}

settle_epoch() {
  local result
  result=$(cast send "$SETTLEMENT_ENGINE" "settleEpoch()" \
    --private-key "$VALIDATOR_KEY" \
    --rpc-url "$RPC_URL" \
    --gas-limit "$GAS_LIMIT" \
    --legacy \
    --json 2>&1) || {
    log "SETTLE TX FAILED: $result"
    return 1
  }

  local status
  status=$(echo "$result" | jq -r '.status // "unknown"' 2>/dev/null)
  local tx_hash
  tx_hash=$(echo "$result" | jq -r '.transactionHash // "?"' 2>/dev/null)
  local gas_used
  gas_used=$(echo "$result" | jq -r '.gasUsed // "?"' 2>/dev/null)

  if [ "$status" = "0x1" ] || [ "$status" = "1" ]; then
    log "SETTLE SUCCESS tx=$tx_hash gas=$gas_used"
    return 0
  else
    log "SETTLE REVERTED tx=$tx_hash status=$status"
    return 1
  fi
}

# ─── Startup ──────────────────────────────────────────────────────────────────
KEEPER_ADDR=$(cast wallet address "$VALIDATOR_KEY" 2>/dev/null || echo "unknown")
log "Settlement keeper started"
log "  address=$KEEPER_ADDR"
log "  settlement_engine=$SETTLEMENT_ENGINE"
log "  rpc=$RPC_URL interval=${INTERVAL}s dry_run=$DRY_RUN"

# ─── Main Loop ────────────────────────────────────────────────────────────────
CONSECUTIVE_ERRORS=0
MAX_ERRORS=10

while true; do
  sleep "$INTERVAL"

  # Check if epoch is ready
  blocks_left=$(get_blocks_until_epoch 2>/dev/null || echo "error")
  if [ "$blocks_left" = "error" ] || [ -z "$blocks_left" ]; then
    CONSECUTIVE_ERRORS=$((CONSECUTIVE_ERRORS + 1))
    if [ "$CONSECUTIVE_ERRORS" -ge "$MAX_ERRORS" ]; then
      log "ERROR: $CONSECUTIVE_ERRORS consecutive RPC failures, backing off 60s"
      sleep 60
      CONSECUTIVE_ERRORS=0
    fi
    continue
  fi
  CONSECUTIVE_ERRORS=0

  if [ "$blocks_left" != "0" ]; then
    continue
  fi

  # Check if there are dirty users to settle
  pending=$(get_pending_count 2>/dev/null || echo "0")
  if [ "$pending" = "0" ] || [ -z "$pending" ]; then
    continue
  fi

  log "Epoch ready: pending_users=$pending"

  if [ "$DRY_RUN" = "true" ]; then
    log "DRY_RUN: would call settleEpoch()"
    continue
  fi

  settle_epoch || true
done
