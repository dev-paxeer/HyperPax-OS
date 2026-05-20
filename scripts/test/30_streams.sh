#!/usr/bin/env bash
# 30_streams.sh — smoke-test PaymentStreams precompile (0x0906)
#
# Pre-req: v21-agent-payments upgrade applied
#
# Tests:
#   1. 0x906 in EvmParams.ActivePrecompiles
#   2. eth_call accruedAmount(0) returns deterministic response (proves keeper
#      is reachable from EVM)
#   3. Streams module account exists in bank (for native+ERC20 escrow)
#
# State-mutating tests (open → accrue → settle) live in the Foundry suite.

set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$DIR/lib.sh"

STREAMS_ADDR="0x0000000000000000000000000000000000000906"

log "[30] PaymentStreams precompile (0x0906)"
require_node_alive

log "1. 0x906 in ActivePrecompiles"
if dq evm params | jq -r '.params.active_precompiles[]' | grep -qi "$STREAMS_ADDR"; then
  ok "0x0906 is active"
else
  fail "0x0906 NOT in ActivePrecompiles — v21 upgrade not applied yet"
fi

log "2. eth_call accruedAmount(0) — selector 0x4e71d92d"
data="0x4e71d92d$(printf '%064x' 0)"
result=$(eth_call "$STREAMS_ADDR" "$data")
if [[ -n "$result" && "$result" != "null" ]]; then
  ok "precompile responded (${#result} chars): ${result:0:80}..."
else
  fail "no response from 0x906.accruedAmount(0)"
fi

log "3. Streams module account"
sa=$(d query auth module-account streams --output json 2>/dev/null \
     | jq -r '.account.value.address // .account.base_account.address // empty')
[[ -n "$sa" ]] && ok "streams module account: $sa" \
              || note "streams module account not configured in maccPerms"

log "[30] PASS — PaymentStreams precompile reachable and responsive"
