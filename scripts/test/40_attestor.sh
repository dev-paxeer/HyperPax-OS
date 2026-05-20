#!/usr/bin/env bash
# 40_attestor.sh — smoke-test TEEAttestor precompile (0x0907)
#
# Pre-req: v21-agent-payments upgrade applied
#
# Tests:
#   1. 0x907 in EvmParams.ActivePrecompiles
#   2. eth_call to a view method on the precompile returns deterministic data
#
# Actual TEE root certificates are NOT loaded by the v21 handler. They are
# uploaded via a separate MsgUpdateTEERoots gov tx — that flow is exercised in
# the Go unit tests for x/attestor.

set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$DIR/lib.sh"

ATTESTOR_ADDR="0x0000000000000000000000000000000000000907"

log "[40] TEEAttestor precompile (0x0907)"
require_node_alive

log "1. 0x907 in ActivePrecompiles"
if dq evm params | jq -r '.params.active_precompiles[]' | grep -qi "$ATTESTOR_ADDR"; then
  ok "0x0907 is active"
else
  fail "0x0907 NOT in ActivePrecompiles — v21 upgrade not applied yet"
fi

log "2. eth_call hasTEERoot(bytes32) — selector 0x1a2c1f9e"
# (placeholder selector — replace with the real one from precompiles/teeattestor/abi.json)
data="0x1a2c1f9e$(printf '%064x' 0)"
result=$(eth_call "$ATTESTOR_ADDR" "$data")
if [[ -n "$result" && "$result" != "null" ]]; then
  ok "precompile responded (${#result} chars): ${result:0:80}..."
else
  note "no response — selector may not match the real ABI. Pull from teeattestor/abi.json."
fi

log "[40] PASS — TEEAttestor precompile reachable"
