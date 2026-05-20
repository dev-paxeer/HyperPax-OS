#!/usr/bin/env bash
# price_submitter.sh — Validator price submission sidecar for x/paxoracle.
#
# Fetches prices from multiple public APIs, computes median, and submits
# to the 0x903 oracle precompile via EVM tx every INTERVAL seconds.
#
# Usage:
#   VALIDATOR_KEY=0x... RPC_URL=http://... ./scripts/price_submitter.sh
#
# Environment variables:
#   VALIDATOR_KEY  — Validator's ETH private key (hex, with 0x prefix). REQUIRED.
#   RPC_URL        — EVM JSON-RPC endpoint (default: http://127.0.0.1:8545)
#   INTERVAL       — Seconds between submissions (default: 20 = ~10 blocks at 2s)
#   GAS_LIMIT      — Gas limit per submitPrice tx (default: 200000)
#   CONFIDENCE     — Default confidence value in wei (default: 1e18 = full confidence)
#   LOG_FILE       — Log file path (default: /var/log/price_submitter.log)
#   DRY_RUN        — If "true", log prices but don't submit (default: false)
#
# Requires: cast (foundry), curl, jq, bc
set -euo pipefail

# ─── Configuration ────────────────────────────────────────────────────────────
VALIDATOR_KEY="${VALIDATOR_KEY:?ERROR: VALIDATOR_KEY not set}"
RPC_URL="${RPC_URL:-http://127.0.0.1:8545}"
INTERVAL="${INTERVAL:-20}"
GAS_LIMIT="${GAS_LIMIT:-200000}"
CONFIDENCE="${CONFIDENCE:-1000000000000000000}"
LOG_FILE="${LOG_FILE:-/var/log/price_submitter.log}"
DRY_RUN="${DRY_RUN:-false}"

ORACLE_PRECOMPILE="0x0000000000000000000000000000000000000903"

# ─── Custom API endpoints ─────────────────────────────────────────────────────
PAX_PRICE_API="https://radiant-harmony-production.up.railway.app/price"
SID_PRICE_API="https://feisty-caring-production-6368.up.railway.app/price"
CHAINFLOW_API="https://chainflowtrading.com/api/v1/quotes"

# ─── Market definitions ───────────────────────────────────────────────────────
# Format: "PAIR_NAME|BINANCE_SYMBOL|COINGECKO_ID|KRAKEN_PAIR|CHAINFLOW_SYMBOL|CUSTOM_API_URL"
# Empty field = source not available for this market.
# PAX and SID use custom Railway APIs exclusively.
# All other pairs use Binance + CoinGecko + Kraken + ChainFlow (backup).
MARKETS=(
  # ── USDC-quoted ─────────────────────────────────────────────────────────────
  "ETH/USDC|ETHUSDC|ethereum|ETHUSD|ethusd|"
  "BTC/USDC|BTCUSDC|bitcoin|XBTUSD|btcusd|"
  "PAX/USDC|||||${PAX_PRICE_API}"
  "BNB/USDC|BNBUSDC|binancecoin||bnbusd|"
  "SOL/USDC|SOLUSDC|solana|SOLUSD|solusd|"
  "DOGE/USDC|DOGEUSDC|dogecoin|XDGUSD|dogeusd|"
  "AVAX/USDC|AVAXUSDC|avalanche-2|AVAXUSD|avaxusd|"
  "LINK/USDC|LINKUSDC|chainlink|LINKUSD|linkusd|"
  "XRP/USDC|XRPUSDC|ripple|XRPUSD|xrpusd|"
  "DOT/USDC|DOTUSDC|polkadot|DOTUSD|dotusd|"
  "TON/USDC|TONUSDC|the-open-network||tonusd|"
  "LTC/USDC|LTCUSDC|litecoin|LTCUSD|ltcusd|"
  "ATOM/USDC|ATOMUSDC|cosmos|ATOMUSD|atomusd|"
  "SUI/USDC|SUIUSDC|sui||suiusd|"
  "APT/USDC|APTUSDC|aptos||aptusd|"
  "SID/USDC|||||${SID_PRICE_API}"
  "HYPE/USDC|HYPEUSDC|hyperliquid||hypeusd|"
  # ── USDT-quoted ─────────────────────────────────────────────────────────────
  "ETH/USDT|ETHUSDT|ethereum|ETHUSD|ethusd|"
  "BTC/USDT|BTCUSDT|bitcoin|XBTUSD|btcusd|"
  "PAX/USDT|||||${PAX_PRICE_API}"
)

# ─── Functions ────────────────────────────────────────────────────────────────
log() {
  local ts
  ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  echo "[$ts] $*" | tee -a "$LOG_FILE"
}

# Fetch price from Binance (returns USD price as float, or empty on failure)
fetch_binance() {
  local symbol="$1"
  curl -sf --max-time 5 "https://api.binance.com/api/v3/ticker/price?symbol=${symbol}" 2>/dev/null \
    | jq -r '.price // empty' 2>/dev/null || true
}

# Fetch price from CoinGecko (returns USD price as float, or empty on failure)
fetch_coingecko() {
  local id="$1"
  curl -sf --max-time 5 "https://api.coingecko.com/api/v3/simple/price?ids=${id}&vs_currencies=usd" 2>/dev/null \
    | jq -r ".[\"${id}\"].usd // empty" 2>/dev/null || true
}

# Fetch price from Kraken (returns USD price as float, or empty on failure)
fetch_kraken() {
  local pair="$1"
  curl -sf --max-time 5 "https://api.kraken.com/0/public/Ticker?pair=${pair}" 2>/dev/null \
    | jq -r ".result | to_entries[0].value.c[0] // empty" 2>/dev/null || true
}

# Fetch price from a custom Railway API (PAX/SID)
# Expects JSON: {"price": <float>, ...}
fetch_custom_api() {
  local url="$1"
  curl -sf --max-time 5 "$url" 2>/dev/null \
    | jq -r '.price // empty' 2>/dev/null || true
}

# Check if a value is a valid positive price
is_valid_price() {
  local p="$1"
  [ -n "$p" ] && [ "$p" != "null" ] && [ "$(echo "$p > 0" | bc -l 2>/dev/null || echo 0)" -eq 1 ]
}

# Compute median of a list of prices (newline-separated)
compute_median() {
  local prices="$1"
  local count
  count=$(echo "$prices" | wc -l)

  if [ "$count" -eq 0 ]; then
    echo ""
    return
  fi

  local sorted
  sorted=$(echo "$prices" | sort -n)

  if [ "$((count % 2))" -eq 1 ]; then
    # Odd count: pick middle
    echo "$sorted" | sed -n "$(( (count + 1) / 2 ))p"
  else
    # Even count: average of two middle values
    local a b
    a=$(echo "$sorted" | sed -n "$(( count / 2 ))p")
    b=$(echo "$sorted" | sed -n "$(( count / 2 + 1 ))p")
    echo "scale=18; ($a + $b) / 2" | bc -l
  fi
}

# Convert a USD float price to 18-decimal wei integer string
# e.g., "3500.25" -> "3500250000000000000000"
price_to_wei() {
  local price="$1"
  # Multiply by 1e18 using bc, then truncate to integer
  echo "scale=0; ($price * 1000000000000000000) / 1" | bc -l
}

# Compute confidence based on number of sources that agreed
# 3 sources: 1e18, 2 sources: 8e17, 1 source: 5e17
compute_confidence() {
  local source_count="$1"
  case "$source_count" in
    3|4|5) echo "1000000000000000000" ;;  # 1e18
    2)     echo "800000000000000000" ;;    # 8e17
    1)     echo "500000000000000000" ;;    # 5e17
    *)     echo "0" ;;
  esac
}

# ─── Preflight checks ────────────────────────────────────────────────────────
for cmd in cast curl jq bc; do
  if ! command -v "$cmd" &>/dev/null; then
    log "ERROR: $cmd not found in PATH"
    exit 1
  fi
done

VALIDATOR_ADDR=$(cast wallet address "$VALIDATOR_KEY" 2>/dev/null)
log "Price submitter starting"
log "  Validator: $VALIDATOR_ADDR"
log "  RPC:       $RPC_URL"
log "  Interval:  ${INTERVAL}s"
log "  Markets:   ${#MARKETS[@]}"
log "  Dry run:   $DRY_RUN"

# ─── Precompute market IDs ────────────────────────────────────────────────────
if [ "${BASH_VERSINFO[0]}" -lt 4 ]; then
  log "ERROR: bash 4+ required for associative arrays (have ${BASH_VERSION})"
  exit 1
fi

declare -A MARKET_IDS
for market_def in "${MARKETS[@]}"; do
  IFS='|' read -r PAIR_NAME _ _ _ _ _ <<< "$market_def"
  MARKET_IDS["$PAIR_NAME"]=$(cast keccak "$PAIR_NAME" 2>/dev/null)
  log "  ${PAIR_NAME} => ${MARKET_IDS[$PAIR_NAME]}"
done

# ─── Build deduplicated ChainFlow symbols list ───────────────────────────────
CHAINFLOW_SYMBOLS=""
for market_def in "${MARKETS[@]}"; do
  IFS='|' read -r _ _ _ _ CF_SYM _ <<< "$market_def"
  if [ -n "$CF_SYM" ] && [[ ! ",${CHAINFLOW_SYMBOLS}," == *",${CF_SYM},"* ]]; then
    CHAINFLOW_SYMBOLS="${CHAINFLOW_SYMBOLS:+${CHAINFLOW_SYMBOLS},}${CF_SYM}"
  fi
done
log "  ChainFlow symbols: ${CHAINFLOW_SYMBOLS:-none}"

# ─── Main loop ────────────────────────────────────────────────────────────────
while true; do
  # Batch-fetch ChainFlow prices (single HTTP call for all standard markets)
  CF_JSON=""
  if [ -n "$CHAINFLOW_SYMBOLS" ]; then
    CF_JSON=$(curl -sf --max-time 10 "${CHAINFLOW_API}?symbols=${CHAINFLOW_SYMBOLS}" 2>/dev/null) || {
      log "WARN: ChainFlow batch fetch failed"
      CF_JSON=""
    }
  fi

  for market_def in "${MARKETS[@]}"; do
    IFS='|' read -r PAIR_NAME BINANCE_SYM COINGECKO_ID KRAKEN_PAIR CF_SYM CUSTOM_URL <<< "$market_def"
    MARKET_ID="${MARKET_IDS[$PAIR_NAME]}"

    prices=""
    source_count=0
    source_log=""

    # Source 1: Custom API (Railway — PAX/SID only)
    if [ -n "$CUSTOM_URL" ]; then
      p=$(fetch_custom_api "$CUSTOM_URL")
      if is_valid_price "$p"; then
        prices="$p"
        source_count=$((source_count + 1))
        source_log="Custom=$p"
      fi
    fi

    # Source 2: Binance
    if [ -n "$BINANCE_SYM" ]; then
      p=$(fetch_binance "$BINANCE_SYM")
      if is_valid_price "$p"; then
        [ -n "$prices" ] && prices="${prices}"$'\n'"${p}" || prices="$p"
        source_count=$((source_count + 1))
        source_log="${source_log:+${source_log} }Binance=$p"
      fi
    fi

    # Source 3: CoinGecko
    if [ -n "$COINGECKO_ID" ]; then
      p=$(fetch_coingecko "$COINGECKO_ID")
      if is_valid_price "$p"; then
        [ -n "$prices" ] && prices="${prices}"$'\n'"${p}" || prices="$p"
        source_count=$((source_count + 1))
        source_log="${source_log:+${source_log} }CoinGecko=$p"
      fi
    fi

    # Source 4: Kraken
    if [ -n "$KRAKEN_PAIR" ]; then
      p=$(fetch_kraken "$KRAKEN_PAIR")
      if is_valid_price "$p"; then
        [ -n "$prices" ] && prices="${prices}"$'\n'"${p}" || prices="$p"
        source_count=$((source_count + 1))
        source_log="${source_log:+${source_log} }Kraken=$p"
      fi
    fi

    # Source 5: ChainFlow (from cached batch response)
    if [ -n "$CF_SYM" ] && [ -n "$CF_JSON" ]; then
      p=$(echo "$CF_JSON" | jq -r ".\"${CF_SYM}\".last // empty" 2>/dev/null) || true
      if is_valid_price "$p"; then
        [ -n "$prices" ] && prices="${prices}"$'\n'"${p}" || prices="$p"
        source_count=$((source_count + 1))
        source_log="${source_log:+${source_log} }ChainFlow=$p"
      fi
    fi

    if [ "$source_count" -eq 0 ]; then
      log "WARN: No valid prices for $PAIR_NAME — skipping"
      continue
    fi

    median_price=$(compute_median "$prices")
    confidence=$(compute_confidence "$source_count")
    price_wei=$(price_to_wei "$median_price")

    log "$PAIR_NAME: median=\$${median_price} sources=${source_count} conf=${confidence}"
    log "  ${source_log}"

    if [ "$DRY_RUN" = "true" ]; then
      log "  DRY_RUN: would submit wei=$price_wei conf=$confidence to $MARKET_ID"
      continue
    fi

    tx_output=$(cast send "$ORACLE_PRECOMPILE" \
      "submitPrice(bytes32,int256,uint256)" \
      "$MARKET_ID" "$price_wei" "$confidence" \
      --private-key "$VALIDATOR_KEY" \
      --rpc-url "$RPC_URL" \
      --legacy \
      --gas-limit "$GAS_LIMIT" \
      2>&1) || true

    tx_status=$(echo "$tx_output" | grep -oP 'status\s+\K\d+' || echo "unknown")

    if [ "$tx_status" = "1" ]; then
      log "  TX SUCCESS"
    else
      log "  TX FAILED (status=$tx_status)"
      log "  Output: $(echo "$tx_output" | tail -3)"
    fi
  done

  sleep "$INTERVAL"
done
