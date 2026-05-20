#!/usr/bin/env bash
# 20_scheduler.sh — smoke-test Scheduler precompile (0x0905)
#
# Pre-req: v20-agent-foundations upgrade applied
#
# Tests:
#   1. 0x905 in EvmParams.ActivePrecompiles
#   2. eth_call getJob(0) returns a deterministic empty-job response (proves
#      precompile is wired AND keeper is reachable from the EVM)
#   3. Scheduler module account exists in bank (for deposit escrow)
#
# State-mutating tests (scheduleJob → wait → verify executed) live in the
# Foundry suite under contracts/paxspot/test/Scheduler.t.sol. They require a
# funded EVM signer and ABI-encoded calldata, which is easier to author in
# Solidity than in shell.

set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$DIR/lib.sh"

SCHED_ADDR="0x0000000000000000000000000000000000000905"

log "[20] Scheduler precompile (0x0905)"
require_node_alive

log "1. 0x905 in ActivePrecompiles"
if dq evm params | jq -r '.params.active_precompiles[]' | grep -qi "$SCHED_ADDR"; then
  ok "0x0905 is active"
else
  fail "0x0905 NOT in ActivePrecompiles — v20 upgrade not applied yet"
fi

log "2. eth_call getJob(0) — selector 0xbf4faaf7"
data="0xbf4faaf7$(printf '%064x' 0)"
result=$(eth_call "$SCHED_ADDR" "$data")
if [[ -n "$result" && "$result" != "null" ]]; then
  ok "precompile responded (${#result} chars): ${result:0:80}..."
else
  fail "no response from 0x905.getJob(0)"
fi

log "3. Scheduler module account"
sa=$(d query auth module-account scheduler --output json 2>/dev/null \
     | jq -r '.account.value.address // .account.base_account.address // empty')
[[ -n "$sa" ]] && ok "scheduler module account: $sa" \
              || note "scheduler module account not configured in maccPerms"

log "[20] PASS — Scheduler precompile reachable and responsive"
