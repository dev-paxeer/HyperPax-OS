#!/usr/bin/env bash
# Shared helpers for the v20/v21 live-test scripts. Source this file at the
# top of every test script.

set -euo pipefail

# ---------------------------------------------------------------------------
# Config (env-overridable)
# ---------------------------------------------------------------------------
export EVMOSD="${EVMOSD:-/root/go/bin/evmosd}"
export HOMEDIR="${HOMEDIR:-$HOME/.tmp-evmosd}"
export CHAINID="${CHAIN_ID:-pax_9000-1}"
export KEYRING="${KEYRING:-test}"
export DENOM="${DENOM:-ahpx}"
export BASEFEE="${BASEFEE:-1000000000}"
export TM_RPC="${TM_RPC:-http://localhost:26657}"
export EVM_RPC="${EVM_RPC:-http://localhost:8545}"

# Test key (mykey from local_node.sh — known mnemonic, deterministic addr)
export VAL_KEY="${VAL_KEY:-mykey}"

# ---------------------------------------------------------------------------
# Output
# ---------------------------------------------------------------------------
log()   { printf '\n\033[1;36m==>\033[0m %s\n' "$*" >&2; }
ok()    { printf '\033[1;32m  ok\033[0m %s\n' "$*" >&2; }
fail()  { printf '\033[1;31mFAIL\033[0m %s\n' "$*" >&2; exit 1; }
note()  { printf '\033[1;33m  ..\033[0m %s\n' "$*" >&2; }

# ---------------------------------------------------------------------------
# Wrappers
# ---------------------------------------------------------------------------
d() { "$EVMOSD" --home "$HOMEDIR" "$@"; }

dq() {  # silent query helper, returns JSON
  "$EVMOSD" --home "$HOMEDIR" query "$@" --output json 2>/dev/null
}

current_height() {
  curl -s "$TM_RPC/status" | jq -r '.result.sync_info.latest_block_height // empty'
}

require_node_alive() {
  local h; h=$(current_height || echo "")
  [[ -n "$h" && "$h" =~ ^[0-9]+$ ]] || fail "node not reachable at $TM_RPC"
  ok "node alive at height $h"
}

# ---------------------------------------------------------------------------
# EVM JSON-RPC
# ---------------------------------------------------------------------------
eth_call() {
  # eth_call TO DATA [BLOCK]
  local to="$1"; local data="$2"; local blk="${3:-latest}"
  curl -s -X POST "$EVM_RPC" \
    -H 'Content-Type: application/json' \
    -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"eth_call\",\"params\":[{\"to\":\"$to\",\"data\":\"$data\"},\"$blk\"]}" \
    | jq -r '.result // .error.message // empty'
}

eth_chain_id() {
  curl -s -X POST "$EVM_RPC" -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}' \
    | jq -r '.result // empty'
}

eth_block_number() {
  curl -s -X POST "$EVM_RPC" -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}' \
    | jq -r '.result // empty'
}

# ABI selectors (first 4 bytes of keccak256(signature))
# computed offline; documented next to usage in each test script.
selector() { printf '%s' "$1"; }
