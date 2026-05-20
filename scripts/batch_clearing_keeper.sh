#!/usr/bin/env bash
# batch_clearing_keeper.sh — Validator keeper sidecar for batch auction clearing.
#
# For each market in BATCH mode, calls MatchingEngine.clearBatch(marketId)
# when there are queued orders from a previous block.
#
# Usage:
#   VALIDATOR_KEY=0x... RPC_URL=http://... ./scripts/batch_clearing_keeper.sh
#
# Environment variables:
#   VALIDATOR_KEY  — Validator's ETH private key (hex, with 0x prefix). REQUIRED.
#   RPC_URL        — EVM JSON-RPC endpoint (default: http://127.0.0.1:8545)
#   INTERVAL       — Seconds between checks (default: 4 = ~2 blocks at 2s)
#   GAS_LIMIT      — Gas limit per clearBatch tx (default: 1000000)
#   LOG_FILE       — Log file path (default: /var/log/batch_clearing_keeper.log)
#   DRY_RUN        — If "true", log but don't submit (default: false)
#
# Requires: cast (foundry), jq
set -euo pipefail

# ─── Configuration ────────────────────────────────────────────────────────────
VALIDATOR_KEY="${VALIDATOR_KEY:?ERROR: VALIDATOR_KEY not set}"
RPC_URL="${RPC_URL:-http://127.0.0.1:8545}"
INTERVAL="${INTERVAL:-4}"
GAS_LIMIT="${GAS_LIMIT:-1000000}"
LOG_FILE="${LOG_FILE:-/var/log/batch_clearing_keeper.log}"
DRY_RUN="${DRY_RUN:-false}"

MATCHING_ENGINE="0x9e98f0CE6BEEc08206Be9D9dd13C493E9Ef25541"

# ─── Market IDs (keccak256 of pair names) ─────────────────────────────────────
# Precomputed at startup to avoid repeated hashing.
declare -A MARKET_IDS

PAIRS=(
  "ETH/USDC" "BTC/USDC" "PAX/USDC" "BNB/USDC" "SOL/USDC"
  "DOGE/USDC" "AVAX/USDC" "LINK/USDC" "XRP/USDC" "DOT/USDC"
  "TON/USDC" "LTC/USDC" "ATOM/USDC" "SUI/USDC" "APT/USDC"
  "SID/USDC" "HYPE/USDC" "ETH/USDT" "BTC/USDT" "PAX/USDT"
)

# ─── Functions ────────────────────────────────────────────────────────────────
log() {
  local ts
  ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  echo "[$ts] $*" | tee -a "$LOG_FILE"
}

compute_market_ids() {
  for pair in "${PAIRS[@]}"; do
    local mid
    mid=$(cast keccak "$pair" 2>/dev/null) || {
      log "WARN: failed to hash $pair"
      continue
    }
    MARKET_IDS["$pair"]="$mid"
  done
  log "Computed ${#MARKET_IDS[@]} market IDs"
}

get_batch_queue_size() {
  local market_id="$1"
  local result
  result=$(cast call "$MATCHING_ENGINE" \
    "batchQueueSize(bytes32)(uint256,uint256)" \
    "$market_id" \
    --rpc-url "$RPC_URL" 2>/dev/null) || {
    echo "0 0"
    return
  }
  local buys sells
  buys=$(echo "$result" | sed -n '1p' | tr -d ' ')
  sells=$(echo "$result" | sed -n '2p' | tr -d ' ')
  echo "${buys:-0} ${sells:-0}"
}

clear_batch() {
  local pair="$1"
  local market_id="$2"
  local result
  result=$(cast send "$MATCHING_ENGINE" \
    "clearBatch(bytes32)" \
    "$market_id" \
    --private-key "$VALIDATOR_KEY" \
    --rpc-url "$RPC_URL" \
    --gas-limit "$GAS_LIMIT" \
    --legacy \
    --json 2>&1) || {
    log "CLEAR TX FAILED [$pair]: $result"
    return 1
  }

  local status
  status=$(echo "$result" | jq -r '.status // "unknown"' 2>/dev/null)
  local tx_hash
  tx_hash=$(echo "$result" | jq -r '.transactionHash // "?"' 2>/dev/null)
  local gas_used
  gas_used=$(echo "$result" | jq -r '.gasUsed // "?"' 2>/dev/null)

  if [ "$status" = "0x1" ] || [ "$status" = "1" ]; then
    log "CLEAR SUCCESS [$pair] tx=$tx_hash gas=$gas_used"
    return 0
  else
    log "CLEAR REVERTED [$pair] tx=$tx_hash status=$status"
    return 1
  fi
}

# ─── Startup ──────────────────────────────────────────────────────────────────
KEEPER_ADDR=$(cast wallet address "$VALIDATOR_KEY" 2>/dev/null || echo "unknown")
log "Batch clearing keeper started"
log "  address=$KEEPER_ADDR"
log "  matching_engine=$MATCHING_ENGINE"
log "  rpc=$RPC_URL interval=${INTERVAL}s dry_run=$DRY_RUN"

compute_market_ids

# ─── Main Loop ────────────────────────────────────────────────────────────────
CONSECUTIVE_ERRORS=0
MAX_ERRORS=10

while true; do
  sleep "$INTERVAL"

  for pair in "${PAIRS[@]}"; do
    mid="${MARKET_IDS[$pair]:-}"
    if [ -z "$mid" ]; then continue; fi

    # Check batch queue
    read -r buys sells <<< "$(get_batch_queue_size "$mid")"

    if [ "$buys" = "0" ] || [ "$sells" = "0" ]; then
      continue
    fi

    log "Batch ready [$pair]: buys=$buys sells=$sells"

    if [ "$DRY_RUN" = "true" ]; then
      log "DRY_RUN: would call clearBatch($pair)"
      continue
    fi

    clear_batch "$pair" "$mid" || true
  done
done
