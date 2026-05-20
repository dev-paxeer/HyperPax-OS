#!/usr/bin/env bash
# 00_alive.sh — basic chain liveness check
#
# Confirms:
#   - Tendermint RPC reachable, producing blocks
#   - EVM JSON-RPC reachable, returns chain ID
#   - x/evm params query works, prints ActivePrecompiles list
#   - Validator set has at least 1 active validator

set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$DIR/lib.sh"

log "[00] Chain liveness"
require_node_alive

H0=$(current_height); sleep 3; H1=$(current_height)
if (( H1 > H0 )); then ok "producing blocks ($H0 -> $H1)"; else fail "chain stalled at $H0"; fi

cid=$(eth_chain_id); [[ -n "$cid" ]] && ok "eth_chainId = $cid" || fail "EVM RPC unreachable"
bn=$(eth_block_number);  ok "eth_blockNumber = $bn (decimal: $((bn)))"

log "Validator set"
nval=$(dq staking validators | jq '.validators | length')
ok "active validators: $nval"
(( nval >= 1 )) || fail "no active validators"

log "EVM ActivePrecompiles"
dq evm params | jq -r '.params.active_precompiles[]' | sed 's/^/  /'

log "Module store presence (paxoracle/scheduler/streams/attestor)"
for mod in paxoracle scheduler streams attestor; do
  if dq "$mod" params 2>/dev/null | jq -e .params >/dev/null 2>&1; then
    ok "$mod params reachable"
  else
    note "$mod params not queryable (module may not be active yet)"
  fi
done

log "[00] PASS"
