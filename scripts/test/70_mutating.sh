#!/usr/bin/env bash
# 70_mutating.sh — full state-mutating end-to-end test of v20/v21 precompiles
# using `cast` directly against the live local chain.
#
# Why not forge script?
#   forge script runs Solidity inside Foundry's local revm sandbox. revm
#   only knows the standard EIP precompiles (0x01-0x09). It has NO knowledge
#   of Paxeer's custom precompiles at 0x901-0x908. When the script does
#   `IEIP712Helper(0x908).domainSeparator(...)`, Solidity's compiler-injected
#   `extcodesize > 0` guard checks the target before the call. revm sees
#   `eth_getCode(0x908) == 0x` (precompiles have no bytecode) and reverts
#   with "call to non-contract address" — BEFORE the call ever leaves revm.
#
#   This affects every high-level interface call to a custom precompile,
#   for every view AND broadcast surface inside `forge script`. There is no
#   workaround inside forge script — even low-level staticcall executes
#   inside revm, which still doesn't know about the precompile.
#
#   `cast call` and `cast send`, in contrast, hit the live RPC directly:
#     cast call  → eth_call          → evmosd's EVM dispatch → Go precompile
#     cast send  → eth_sendRawTransaction → ditto
#   Both reach the real precompile. This script uses cast exclusively.
#
# Pre-req: chain is up via local_node.sh with 0x905-0x908 in
# EvmParams.ActivePrecompiles (the genesis patch in local_node.sh handles
# this automatically).
#
# Steps:
#   1. Export the test validator's EVM private key from the evmosd keyring
#   2. RunEIP712     — verify 0x908 produces correct domainSeparator+digest
#   3. RunAttestor   — verify 0x907 rootCount + verify(empty)==false
#   4. RunScheduler  — deploy JobTarget, schedule poke() in 5 blocks, wait
#   5. RunStreams    — open 60s native stream @1 wei/s, verify accrued, settle

set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$DIR/lib.sh"

CONTRACTS_DIR="/paxeer-sdk/hyperpax-os-cronosRelease/contracts/paxspot"
JOB_TARGET_SRC="$CONTRACTS_DIR/scripts/test_v20v21/JobTarget.sol"

EIP712_ADDR=0x0000000000000000000000000000000000000908
ATTESTOR_ADDR=0x0000000000000000000000000000000000000907
SCHEDULER_ADDR=0x0000000000000000000000000000000000000905
STREAMS_ADDR=0x0000000000000000000000000000000000000906

PAYEE_ADDR="${PAYEE_ADDR:-0x000000000000000000000000000000000000bEEF}"

# Ensure cast/forge are in PATH
export PATH="/root/.foundry/bin:/root/go/bin:$PATH"

require_node_alive

# ---------------------------------------------------------------------------
# 1. Export the EVM private key for the test validator
# ---------------------------------------------------------------------------
log "[70.1] exporting EVM private key for '$VAL_KEY'"
set +o pipefail
PK_RAW=$(printf 'y\n' | "$EVMOSD" --home "$HOMEDIR" keys unsafe-export-eth-key \
            "$VAL_KEY" --keyring-backend "$KEYRING" 2>&1 | tail -n 1 | tr -d '[:space:]')
set -o pipefail
if [[ ! "$PK_RAW" =~ ^[0-9A-Fa-f]{64}$ ]]; then
  log "DIAG: full unsafe-export-eth-key output:"
  printf 'y\n' | "$EVMOSD" --home "$HOMEDIR" keys unsafe-export-eth-key \
      "$VAL_KEY" --keyring-backend "$KEYRING" 2>&1 | sed 's/^/  /' >&2
  fail "unexpected privkey output: '$PK_RAW'"
fi
export PRIVATE_KEY="0x$PK_RAW"
ok "PRIVATE_KEY ready (66 chars)"

SENDER=$(cast wallet address --private-key "$PRIVATE_KEY")
ok "sender address: $SENDER"

# Helpers
hex_to_dec() {
  local h="$1"
  # cast returns either 0x-prefixed hex, plain dec, or 'true'/'false'
  if [[ "$h" =~ ^0x[0-9a-fA-F]+$ ]]; then
    printf '%d' "$h"
  else
    printf '%s' "$h"
  fi
}

# Chain block timestamp via eth_getBlockByNumber. Use this instead of
# `date +%s` because the chain clock can drift from local time and the
# streams keeper validates (stopTime >= startTime + MinDuration) against
# ctx.BlockTime().
chain_time() {
  local hex
  hex=$(curl -s -X POST "$EVM_RPC" -H 'Content-Type: application/json' \
        -d '{"jsonrpc":"2.0","id":1,"method":"eth_getBlockByNumber","params":["latest",false]}' \
        | jq -r '.result.timestamp')
  [[ "$hex" =~ ^0x[0-9a-fA-F]+$ ]] || { echo "0"; return 1; }
  printf '%d' "$hex"
}

# ---------------------------------------------------------------------------
# 2. EIP712 — stateless precompile. Compare on-chain output vs cast-computed
#    Solidity reference. They MUST match byte-for-byte.
# ---------------------------------------------------------------------------
log "[70.3] RunEIP712 (0x908 — domainSeparator + hashTypedData vs reference)"

NAME="Paxeer"
VERSION="1"
CHAIN_ID=9000
VERIFYING_CONTRACT=0xdEad000000000000000000000000000000000000

# Reference domainSeparator computation in bash via cast:
#   keccak256(abi.encode(DOMAIN_TYPEHASH, keccak(name), keccak(version), chainId, addr))
DOMAIN_TH=$(cast keccak "EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)")
NAME_HASH=$(cast keccak "$NAME")
VER_HASH=$(cast keccak "$VERSION")
EXPECTED_DOMAIN=$(cast keccak \
  "$(cast abi-encode 'f(bytes32,bytes32,bytes32,uint256,address)' \
       "$DOMAIN_TH" "$NAME_HASH" "$VER_HASH" "$CHAIN_ID" "$VERIFYING_CONTRACT")")

ACTUAL_DOMAIN=$(cast call --rpc-url "$EVM_RPC" "$EIP712_ADDR" \
    "domainSeparator(string,string,uint256,address)(bytes32)" \
    "$NAME" "$VERSION" "$CHAIN_ID" "$VERIFYING_CONTRACT")

note "  expected domain = $EXPECTED_DOMAIN"
note "  actual   domain = $ACTUAL_DOMAIN"
[[ "$ACTUAL_DOMAIN" == "$EXPECTED_DOMAIN" ]] || fail "domainSeparator mismatch"
ok "domainSeparator matches reference"

# hashTypedData reference: keccak256("\x19\x01" || domain || structHash)
STRUCT_HASH=$(cast keccak "hello-paxeer")
EXPECTED_DIGEST=$(cast keccak "0x1901${ACTUAL_DOMAIN:2}${STRUCT_HASH:2}")
ACTUAL_DIGEST=$(cast call --rpc-url "$EVM_RPC" "$EIP712_ADDR" \
    "hashTypedData(bytes32,bytes32)(bytes32)" "$ACTUAL_DOMAIN" "$STRUCT_HASH")

note "  expected digest = $EXPECTED_DIGEST"
note "  actual   digest = $ACTUAL_DIGEST"
[[ "$ACTUAL_DIGEST" == "$EXPECTED_DIGEST" ]] || fail "hashTypedData mismatch"
ok "hashTypedData matches reference"

# ---------------------------------------------------------------------------
# 3. TEEAttestor — read-only. rootCount returns 0 for every family until
#    a gov MsgUpdateTEERoots loads roots; verify(empty) returns false.
# ---------------------------------------------------------------------------
log "[70.4] RunAttestor (0x907 — rootCount + verify(empty))"

for t in 0 1 2 3; do
  n_raw=$(cast call --rpc-url "$EVM_RPC" "$ATTESTOR_ADDR" \
              "rootCount(uint8)(uint256)" "$t")
  n=$(hex_to_dec "$n_raw")
  note "  teeType=$t rootCount=$n"
done

# verify(family=0, empty envelope). On a chain with no TEE roots loaded
# (default genesis state — roots come via gov MsgUpdateTEERoots), this
# either returns false OR reverts with "no trusted roots loaded". Both
# outcomes prove the precompile is reachable and behaving deterministically.
# Once governance loads real roots, this test would assert
# `verify(real_quote) == true` instead.
set +e
VERIFY_OUT=$(cast call --rpc-url "$EVM_RPC" "$ATTESTOR_ADDR" \
             "verify(uint8,bytes)(bool)" 0 "0x" 2>&1)
VERIFY_RC=$?
set -e
note "  verify(0, '0x') exit=$VERIFY_RC out='$VERIFY_OUT'"
if [[ "$VERIFY_RC" -eq 0 && "$VERIFY_OUT" == "false" ]]; then
  ok "TEEAttestor verify(empty) returned false (no roots loaded path)"
elif [[ "$VERIFY_RC" -ne 0 && "$VERIFY_OUT" == *"no trusted roots loaded"* ]]; then
  ok "TEEAttestor verify(empty) reverted with 'no trusted roots loaded' (expected pre-gov-root-load)"
else
  fail "verify(empty) unexpected: exit=$VERIFY_RC out='$VERIFY_OUT'"
fi

# ---------------------------------------------------------------------------
# 4. Scheduler — deploy JobTarget, schedule poke() ~5 blocks ahead, wait
# ---------------------------------------------------------------------------
log "[70.5] RunScheduler (deploy JobTarget + schedule poke() via 0x905)"

# Build & deploy JobTarget via `forge create`. JobTarget's constructor calls
# NO precompiles, so revm-side execution is fine — it deploys a normal
# contract on the live chain.
log "  building forge artifacts"
cd "$CONTRACTS_DIR"
forge build --skip test 2>&1 | tail -3

log "  forge create JobTarget"
DEPLOY_OUT=$(forge create \
    --rpc-url "$EVM_RPC" \
    --private-key "$PRIVATE_KEY" \
    --broadcast \
    "scripts/test_v20v21/JobTarget.sol:JobTarget" 2>&1)
echo "$DEPLOY_OUT" | tail -5 >&2
JOB_TARGET=$(echo "$DEPLOY_OUT" | grep -oE 'Deployed to: 0x[a-fA-F0-9]{40}' \
                 | awk '{print $3}')
[[ -n "$JOB_TARGET" ]] || fail "JobTarget deploy failed (no 'Deployed to:' line)"
ok "JobTarget deployed at: $JOB_TARGET"

# Compute schedule args
CUR_BLOCK_HEX=$(eth_block_number)
CUR_BLOCK=$(hex_to_dec "$CUR_BLOCK_HEX")
FIRE_AT_BLOCK=$((CUR_BLOCK + 5))
POKE_CALLDATA=$(cast calldata "poke()")
GAS_LIMIT=200000
DEPOSIT_WEI=1000000000000000000   # 1 ether

note "  current block = $CUR_BLOCK"
note "  fire at block = $FIRE_AT_BLOCK"
note "  deposit wei   = $DEPOSIT_WEI"

# Preview jobId via eth_call (simulates without committing → returns the
# next-jobId counter without bumping it).
JOB_ID_HEX=$(cast call --rpc-url "$EVM_RPC" --from "$SENDER" \
    --value "$DEPOSIT_WEI" \
    "$SCHEDULER_ADDR" \
    "schedule(address,bytes,uint64,uint64)(uint256)" \
    "$JOB_TARGET" "$POKE_CALLDATA" "$FIRE_AT_BLOCK" "$GAS_LIMIT")
JOB_ID=$(hex_to_dec "$JOB_ID_HEX")
ok "preview jobId = $JOB_ID"

# Actually broadcast the schedule tx with the same args
log "  cast send schedule(...)"
cast send --rpc-url "$EVM_RPC" --private-key "$PRIVATE_KEY" \
    --value "$DEPOSIT_WEI" \
    "$SCHEDULER_ADDR" \
    "schedule(address,bytes,uint64,uint64)" \
    "$JOB_TARGET" "$POKE_CALLDATA" "$FIRE_AT_BLOCK" "$GAS_LIMIT" 2>&1 | tail -3

# Verify the job is in pending() for our sender
PENDING_RAW=$(cast call --rpc-url "$EVM_RPC" "$SCHEDULER_ADDR" \
                "pending(address)(uint256[])" "$SENDER")
note "  pending() = $PENDING_RAW"
echo "$PENDING_RAW" | grep -qE "(^|[^0-9])${JOB_ID}([^0-9]|$)" \
  || fail "jobId $JOB_ID not in pending() for $SENDER"
ok "jobId $JOB_ID confirmed pending"

# Poll until JobTarget.count() ticks
log "[70.5] polling JobTarget($JOB_TARGET).count() until job fires (fireAt=$FIRE_AT_BLOCK)"
deadline=$((FIRE_AT_BLOCK + 20))
fired=0
while :; do
  cur_hex=$(eth_block_number); cur_dec=$(hex_to_dec "$cur_hex")
  count_raw=$(cast call --rpc-url "$EVM_RPC" "$JOB_TARGET" \
                "count()(uint256)" 2>/dev/null || echo "0")
  count=$(hex_to_dec "$count_raw")
  note "  block=$cur_dec count=$count"
  if (( count >= 1 )); then
    ok "JobTarget.count() == $count (job fired at block $cur_dec)"
    fired=1
    break
  fi
  if (( cur_dec > deadline )); then
    job_state=$(cast call --rpc-url "$EVM_RPC" "$SCHEDULER_ADDR" \
        "getJob(uint256)" "$JOB_ID" 2>&1 || echo "(getJob call failed)")
    log "DIAG: getJob($JOB_ID) = $job_state"
    fail "job did not fire within ${deadline} blocks"
  fi
  sleep 2
done
(( fired == 1 )) || fail "scheduler poll exited without firing"

# ---------------------------------------------------------------------------
# 5. Streams — open + wait + verify accrued > 0 + settle moves funds
# ---------------------------------------------------------------------------
log "[70.6] RunStreams (open native stream via 0x906 + verify accrued + settle)"

# Use chain block timestamp (NOT local clock — they can drift). The streams
# keeper validates stopTime - startTime >= params.MinDuration (default 60s)
# against ctx.BlockTime(). We pass startTime=0 so the chain sets it to its
# own block time, then stopTime is chain-time + 120s for a safe margin.
NOW_CHAIN=$(chain_time)
[[ "$NOW_CHAIN" -gt 0 ]] || fail "chain_time returned 0 — RPC unreachable?"
START_TIME=0                              # 0 → chain uses ctx.BlockTime()
STOP_TIME=$((NOW_CHAIN + 120))            # generous: 120s window
RATE_PER_SECOND=1
CAP=100

note "  payee = $PAYEE_ADDR"
note "  rate  = $RATE_PER_SECOND wei/s"
note "  start = $START_TIME"
note "  stop  = $STOP_TIME"
note "  cap   = $CAP wei"

# Preview streamId via eth_call (same trick as scheduler)
STREAM_ID_HEX=$(cast call --rpc-url "$EVM_RPC" --from "$SENDER" \
    "$STREAMS_ADDR" \
    "open(address,address,uint256,uint64,uint64,uint256)(uint256)" \
    "$PAYEE_ADDR" "0x0000000000000000000000000000000000000000" \
    "$RATE_PER_SECOND" "$START_TIME" "$STOP_TIME" "$CAP")
STREAM_ID=$(hex_to_dec "$STREAM_ID_HEX")
ok "preview streamId = $STREAM_ID"

# Broadcast the open
log "  cast send open(...)"
cast send --rpc-url "$EVM_RPC" --private-key "$PRIVATE_KEY" \
    "$STREAMS_ADDR" \
    "open(address,address,uint256,uint64,uint64,uint256)" \
    "$PAYEE_ADDR" "0x0000000000000000000000000000000000000000" \
    "$RATE_PER_SECOND" "$START_TIME" "$STOP_TIME" "$CAP" 2>&1 | tail -3

# Confirm via getStream — the tuple's last field is the `active` bool.
GS_OUT=$(cast call --rpc-url "$EVM_RPC" "$STREAMS_ADDR" \
            "getStream(uint256)((uint256,address,address,address,uint256,uint256,uint64,uint64,uint256,bool))" \
            "$STREAM_ID")
note "  getStream($STREAM_ID) = $GS_OUT"
# cast returns the tuple as space- or newline-separated values; last is `active`
GS_LAST=$(printf '%s\n' "$GS_OUT" | tr -d '()' | tr ',' ' ' | awk '{print $NF}')
[[ "$GS_LAST" == "true" ]] || fail "stream $STREAM_ID not active after open (active='$GS_LAST')"
ok "stream $STREAM_ID active"

# Wait ~6 seconds and verify accrued grew
log "  sleeping 6s then reading accrued($STREAM_ID)"
sleep 6
ACCRUED_RAW=$(cast call --rpc-url "$EVM_RPC" "$STREAMS_ADDR" \
                 "accrued(uint256)(uint256)" "$STREAM_ID")
ACCRUED=$(hex_to_dec "$ACCRUED_RAW")
ok "accrued($STREAM_ID) = $ACCRUED (expected >= 5 after 6s @ 1 wei/s)"
(( ACCRUED >= 5 )) || fail "accrued did not grow: $ACCRUED"

# Capture payee balance before settle
BAL_BEFORE_HEX=$(cast balance --rpc-url "$EVM_RPC" "$PAYEE_ADDR")
BAL_BEFORE=$(hex_to_dec "$BAL_BEFORE_HEX")
note "  $PAYEE_ADDR balance before settle = $BAL_BEFORE wei"

# settle
log "  cast send settle($STREAM_ID)"
cast send --rpc-url "$EVM_RPC" --private-key "$PRIVATE_KEY" \
    "$STREAMS_ADDR" "settle(uint256)" "$STREAM_ID" 2>&1 | tail -3
sleep 2

BAL_AFTER_HEX=$(cast balance --rpc-url "$EVM_RPC" "$PAYEE_ADDR")
BAL_AFTER=$(hex_to_dec "$BAL_AFTER_HEX")
note "  $PAYEE_ADDR balance after settle  = $BAL_AFTER wei"

if (( BAL_AFTER > BAL_BEFORE )); then
  ok "settle: payee balance grew from $BAL_BEFORE to $BAL_AFTER wei"
else
  fail "settle did NOT move funds (before=$BAL_BEFORE after=$BAL_AFTER)"
fi

log "[70] ALL STATE-MUTATING TESTS PASS"
