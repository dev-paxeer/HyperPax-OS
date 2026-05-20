#!/usr/bin/env bash
# Copyright PaxLabs Ltd.(Paxeer Network)
# Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)
#
# run_v20v21_localnet.sh — boot a single-node Paxeer testnet that exercises
# the v20-agent-foundations and v21-agent-payments hard-fork upgrades end-to-end.
#
# Flow:
#   1. Wipe any existing state at $HOMEDIR
#   2. Init chain (v18 base state), import dev keys, genesis tx
#   3. Start evmosd in the background, wait for first block
#   4. Submit + pass gov upgrade proposal: v20-agent-foundations
#   5. Wait for the upgrade height. Chain halts cleanly (Upgrade halt).
#   6. Restart same binary — upgrade handler runs, migrations apply, chain resumes
#   7. Submit + pass gov upgrade proposal: v21-agent-payments
#   8. Repeat halt/restart cycle
#   9. Print final EvmParams.ActivePrecompiles + module stores for verification
#
# Usage:
#   bash scripts/run_v20v21_localnet.sh
#
# Logs:
#   $HOMEDIR/node.log
#
# Stop:
#   pkill -f "evmosd start.*$HOMEDIR"

set -euo pipefail

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------
EVMOSD="${EVMOSD:-/root/go/bin/evmosd}"
HOMEDIR="${HOMEDIR:-$HOME/.paxeer-localnet}"
CHAINID="${CHAIN_ID:-pax_9000-1}"
MONIKER="paxeer-localnet"
KEYRING="test"
KEYALGO="eth_secp256k1"
LOGLEVEL="${LOGLEVEL:-info}"
BASEFEE="${BASEFEE:-1000000000}"
LOGFILE="$HOMEDIR/node.log"

# Voting period = 30s for fast gov flow (mainnet uses 172800s)
VOTING_PERIOD="${VOTING_PERIOD:-30s}"
DEPOSIT_PERIOD="${DEPOSIT_PERIOD:-30s}"

# Each upgrade runs at (currentHeight + UPGRADE_OFFSET) blocks ahead
UPGRADE_OFFSET="${UPGRADE_OFFSET:-15}"

# Path variables
CONFIG="$HOMEDIR/config/config.toml"
APP_TOML="$HOMEDIR/config/app.toml"
GENESIS="$HOMEDIR/config/genesis.json"
TMP_GENESIS="$HOMEDIR/config/tmp_genesis.json"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

log() { printf '\n\033[1;36m==>\033[0m %s\n' "$*" >&2; }
die() { printf '\n\033[1;31mERR:\033[0m %s\n' "$*" >&2; exit 1; }

cmd_d() { "$EVMOSD" --home "$HOMEDIR" "$@"; }

current_height() {
  cmd_d status 2>/dev/null | jq -r '.SyncInfo.latest_block_height // .sync_info.latest_block_height' 2>/dev/null || echo "0"
}

wait_for_block() {
  local target="$1"
  local timeout="${2:-120}"
  local elapsed=0
  while [[ $elapsed -lt $timeout ]]; do
    local h; h=$(current_height)
    if [[ "$h" =~ ^[0-9]+$ ]] && (( h >= target )); then
      return 0
    fi
    sleep 2; elapsed=$((elapsed + 2))
  done
  die "wait_for_block: timed out waiting for block $target (last height=$h)"
}

wait_for_node_alive() {
  local timeout=60
  local elapsed=0
  while [[ $elapsed -lt $timeout ]]; do
    if curl -s "http://localhost:26657/status" >/dev/null 2>&1; then
      local h; h=$(current_height)
      if [[ "$h" =~ ^[0-9]+$ ]] && (( h >= 1 )); then
        return 0
      fi
    fi
    sleep 1; elapsed=$((elapsed + 1))
  done
  die "wait_for_node_alive: node did not start within ${timeout}s"
}

start_node_background() {
  log "Starting evmosd in background -> $LOGFILE"
  nohup "$EVMOSD" start \
    --metrics \
    --log_level "$LOGLEVEL" \
    --minimum-gas-prices=0.0001ahpx \
    --json-rpc.api eth,txpool,personal,net,debug,web3 \
    --json-rpc.enable \
    --api.enable \
    --grpc.enable \
    --home "$HOMEDIR" \
    --chain-id "$CHAINID" \
    >>"$LOGFILE" 2>&1 &
  echo $! > "$HOMEDIR/node.pid"
  sleep 3
}

stop_node() {
  if [[ -f "$HOMEDIR/node.pid" ]]; then
    local pid; pid=$(cat "$HOMEDIR/node.pid" 2>/dev/null || true)
    if [[ -n "${pid:-}" ]] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      sleep 1
      kill -9 "$pid" 2>/dev/null || true
    fi
    rm -f "$HOMEDIR/node.pid"
  fi
  pkill -f "evmosd start.*$HOMEDIR" 2>/dev/null || true
  sleep 1
}

wait_for_halt() {
  local timeout="${1:-180}"
  local elapsed=0
  while [[ $elapsed -lt $timeout ]]; do
    local pid; pid=$(cat "$HOMEDIR/node.pid" 2>/dev/null || true)
    if [[ -z "${pid:-}" ]] || ! kill -0 "$pid" 2>/dev/null; then
      log "Node halted (process gone)."
      return 0
    fi
    # Also detect "UPGRADE \"...\" NEEDED at height" in the log — that's a
    # graceful halt signal.
    if grep -qE 'UPGRADE "[^"]+" NEEDED at height' "$LOGFILE" 2>/dev/null; then
      log "Upgrade halt detected in log."
      # Give it 3s to drain
      sleep 3
      # Kill if still up
      if [[ -n "${pid:-}" ]] && kill -0 "$pid" 2>/dev/null; then
        kill "$pid" 2>/dev/null || true
        sleep 2
      fi
      return 0
    fi
    sleep 2; elapsed=$((elapsed + 2))
  done
  die "wait_for_halt: chain didn't halt in ${timeout}s — upgrade may not have triggered"
}

submit_and_pass_upgrade() {
  local plan_name="$1"
  local info="$2"

  local cur; cur=$(current_height)
  if ! [[ "$cur" =~ ^[0-9]+$ ]]; then
    die "submit_and_pass_upgrade: cannot read current height"
  fi
  local upgrade_height=$((cur + UPGRADE_OFFSET))

  log "Submitting upgrade '$plan_name' targeting height $upgrade_height (current=$cur)"

  # Write the proposal JSON
  local prop_file="$HOMEDIR/upgrade_${plan_name}.json"
  local authority
  authority=$(cmd_d query auth module-account gov --output json 2>/dev/null \
              | jq -r '.account.value.address // .account.base_account.address' 2>/dev/null \
              || true)
  if [[ -z "$authority" || "$authority" == "null" ]]; then
    die "could not resolve gov module authority address from chain (auth module-account query returned empty)"
  fi

  cat >"$prop_file" <<EOF
{
  "messages": [
    {
      "@type": "/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade",
      "authority": "$authority",
      "plan": {
        "name": "$plan_name",
        "time": "0001-01-01T00:00:00Z",
        "height": "$upgrade_height",
        "info": $(jq -Rs . <<<"$info"),
        "upgraded_client_state": null
      }
    }
  ],
  "metadata": "ipfs://CID",
  "deposit": "20000000000000000000ahpx",
  "title": "Upgrade to $plan_name",
  "summary": "Activate $plan_name on local Paxeer testnet"
}
EOF

  # Submit
  local txhash
  txhash=$(cmd_d tx gov submit-proposal "$prop_file" \
            --from mykey \
            --keyring-backend "$KEYRING" \
            --chain-id "$CHAINID" \
            --gas auto --gas-adjustment 1.5 \
            --gas-prices "${BASEFEE}ahpx" \
            --yes --output json 2>&1 \
            | tail -n 1 | jq -r .txhash 2>/dev/null || true)
  log "submit-proposal txhash=$txhash"

  sleep 5

  # Find the latest proposal id
  local prop_id
  prop_id=$(cmd_d query gov proposals --output json 2>/dev/null \
            | jq -r '.proposals | sort_by(.id|tonumber) | last.id' 2>/dev/null \
            || true)
  if [[ -z "$prop_id" || "$prop_id" == "null" ]]; then
    die "submit_and_pass_upgrade: could not find proposal id (txhash=$txhash)"
  fi
  log "Latest proposal id = $prop_id"

  # Vote yes
  cmd_d tx gov vote "$prop_id" yes \
    --from mykey \
    --keyring-backend "$KEYRING" \
    --chain-id "$CHAINID" \
    --gas auto --gas-adjustment 1.5 \
    --gas-prices "${BASEFEE}ahpx" \
    --yes --output json >/dev/null 2>&1
  log "Voted yes on proposal $prop_id"

  # Wait for voting end + a few blocks for tally
  log "Sleeping $VOTING_PERIOD + 6s for tally"
  local vp_seconds
  vp_seconds=$(echo "$VOTING_PERIOD" | sed 's/s$//')
  sleep $((vp_seconds + 6))

  local status
  status=$(cmd_d query gov proposal "$prop_id" --output json 2>/dev/null | jq -r .status 2>/dev/null || true)
  log "Proposal $prop_id status: $status"
  if [[ "$status" != "PROPOSAL_STATUS_PASSED" && "$status" != "3" ]]; then
    die "Proposal $prop_id did not pass (status=$status). See $LOGFILE"
  fi

  echo "$upgrade_height"
}

# ---------------------------------------------------------------------------
# Phase 1: clean + init
# ---------------------------------------------------------------------------

[[ -x "$EVMOSD" ]] || die "evmosd binary not found at $EVMOSD"

log "Stopping any pre-existing node"
stop_node || true

log "Wiping $HOMEDIR"
rm -rf "$HOMEDIR"
mkdir -p "$HOMEDIR"

log "Initializing chain '$CHAINID'"
cmd_d config keyring-backend "$KEYRING"
cmd_d config chain-id "$CHAINID"
cmd_d init "$MONIKER" -o --chain-id "$CHAINID"

# Validator key
VAL_KEY="mykey"
VAL_MNEMONIC="gesture inject test cycle original hollow east ridge hen combine junk child bacon zero hope comfort vacuum milk pitch cage oppose unhappy lunar seat"
USER1_KEY="dev0"
USER1_MNEMONIC="copper push brief egg scan entry inform record adjust fossil boss egg comic alien upon aspect dry avoid interest fury window hint race symptom"

log "Importing keys"
echo "$VAL_MNEMONIC" | cmd_d keys add "$VAL_KEY" --recover --keyring-backend "$KEYRING" --algo "$KEYALGO" >/dev/null
echo "$USER1_MNEMONIC" | cmd_d keys add "$USER1_KEY" --recover --keyring-backend "$KEYRING" --algo "$KEYALGO" >/dev/null
NODE_ADDRESS=$(cmd_d keys show -a "$VAL_KEY" --keyring-backend "$KEYRING")
USER1_ADDRESS=$(cmd_d keys show -a "$USER1_KEY" --keyring-backend "$KEYRING")
log "Validator: $NODE_ADDRESS"
log "Dev0:      $USER1_ADDRESS"

# Genesis tweaks
log "Patching genesis (denom=ahpx, gov fast-track, basefee)"
jq '.app_state["staking"]["params"]["bond_denom"]="ahpx"
   |.app_state["crisis"]["constant_fee"]["denom"]="ahpx"
   |.app_state["gov"]["deposit_params"]["min_deposit"][0]["denom"]="ahpx"
   |.app_state["gov"]["deposit_params"]["min_deposit"][0]["amount"]="1000000000000000000"
   |.app_state["gov"]["params"]["min_deposit"][0]["denom"]="ahpx"
   |.app_state["gov"]["params"]["min_deposit"][0]["amount"]="1000000000000000000"
   |.app_state["gov"]["params"]["voting_period"]="'"$VOTING_PERIOD"'"
   |.app_state["gov"]["params"]["max_deposit_period"]="'"$DEPOSIT_PERIOD"'"
   |.app_state["gov"]["voting_params"]["voting_period"]="'"$VOTING_PERIOD"'"
   |.app_state["gov"]["deposit_params"]["max_deposit_period"]="'"$DEPOSIT_PERIOD"'"
   |.app_state["evm"]["params"]["evm_denom"]="ahpx"
   |.app_state["inflation"]["params"]["mint_denom"]="ahpx"
   |.app_state["feemarket"]["params"]["base_fee"]="'"$BASEFEE"'"
   |.consensus_params["block"]["max_gas"]="10000000"' \
   "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

# Claims module — same defaults as local_node.sh
current_date=$(date -u +"%Y-%m-%dT%TZ")
jq -r --arg current_date "$current_date" '
    .app_state["claims"]["params"]["airdrop_start_time"]=$current_date
   |.app_state["claims"]["params"]["duration_of_decay"]="1000000s"
   |.app_state["claims"]["params"]["duration_until_decay"]="100000s"
   |.app_state["claims"]["claims_records"]=[]
' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

# Balance the claims module's bank balance with claim records.
# Since claims_records is empty, the claims module account starts at zero — no bank balance entry required.

log "Adding genesis accounts"
cmd_d add-genesis-account "$NODE_ADDRESS" 100000000000000000000000000ahpx --keyring-backend "$KEYRING" >/dev/null
cmd_d add-genesis-account "$USER1_ADDRESS" 1000000000000000000000000ahpx --keyring-backend "$KEYRING" >/dev/null

# Recompute total supply
total_supply="101000000000000000000000000"
jq -r --arg total_supply "$total_supply" '.app_state["bank"]["supply"][0]["amount"]=$total_supply' \
   "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

log "Creating validator gentx"
cmd_d gentx "$VAL_KEY" 1000000000000000000000ahpx --gas-prices "${BASEFEE}ahpx" --keyring-backend "$KEYRING" --chain-id "$CHAINID" >/dev/null

log "Collecting gentxs + validating genesis"
cmd_d collect-gentxs >/dev/null
cmd_d validate-genesis

# Enable RPC, API in app.toml; allow non-localhost queries.
sed -i 's/^enable = false/enable = true/g' "$APP_TOML"
sed -i 's/^enabled = false/enabled = true/g' "$APP_TOML"
# Shorter pruning for dev
sed -i 's/^pruning = "default"/pruning = "custom"/' "$APP_TOML"
sed -i 's/^pruning-keep-recent = "0"/pruning-keep-recent = "100"/' "$APP_TOML"
sed -i 's/^pruning-interval = "0"/pruning-interval = "10"/' "$APP_TOML"

# ---------------------------------------------------------------------------
# Phase 2: first boot (v18 base)
# ---------------------------------------------------------------------------

: >"$LOGFILE"
start_node_background
wait_for_node_alive
log "Chain alive at height $(current_height) — base (v18) state"

# Confirm v19_paxspot upgrade is not actually applied here (no v19 prop ran on this fresh chain).
# Note: the upgrade handlers for v19_paxspot, v20agent, v21agent are *registered* in app.go, but
# none of them run unless a MsgSoftwareUpgrade plan with the matching name passes gov AND the
# chain reaches the planned height.

# ---------------------------------------------------------------------------
# Phase 3: v20-agent-foundations upgrade
# ---------------------------------------------------------------------------

# We need v19_paxspot to land first if the chain's store-upgrade switch hard-orders v19 -> v20.
# Inspect app.go: each upgrade adds its own store keys (v19 adds paxoracle, v20 adds scheduler).
# Since these are independent module store additions, we can run v20 directly without v19 —
# v19's paxoracle module is registered at genesis already (Phase 1 init ran it), so v19 just
# activates 0x901-0x904 precompiles. We need v19's precompile activation for the agent fee lane
# to be testable, but it isn't strictly required for the scheduler + EIP-7702 paths.
#
# For the safest test, we run v19 first, then v20, then v21.

V19_HEIGHT=$(submit_and_pass_upgrade "v19_paxspot" "(local test)")
log "Waiting for v19_paxspot upgrade halt at height $V19_HEIGHT"
wait_for_block "$V19_HEIGHT" 120 || true
wait_for_halt 120
log "Chain halted at v19_paxspot upgrade. Restarting binary..."
start_node_background
wait_for_node_alive
log "v19_paxspot applied. Chain back online at height $(current_height)"

V20_HEIGHT=$(submit_and_pass_upgrade "v20-agent-foundations" "(local test)")
log "Waiting for v20-agent-foundations upgrade halt at height $V20_HEIGHT"
wait_for_block "$V20_HEIGHT" 120 || true
wait_for_halt 120
log "Chain halted at v20 upgrade. Restarting binary..."
start_node_background
wait_for_node_alive
log "v20-agent-foundations applied. Chain back online at height $(current_height)"

# ---------------------------------------------------------------------------
# Phase 4: v21-agent-payments upgrade
# ---------------------------------------------------------------------------

V21_HEIGHT=$(submit_and_pass_upgrade "v21-agent-payments" "(local test)")
log "Waiting for v21-agent-payments upgrade halt at height $V21_HEIGHT"
wait_for_block "$V21_HEIGHT" 120 || true
wait_for_halt 120
log "Chain halted at v21 upgrade. Restarting binary..."
start_node_background
wait_for_node_alive
log "v21-agent-payments applied. Chain back online at height $(current_height)"

# ---------------------------------------------------------------------------
# Phase 5: verification
# ---------------------------------------------------------------------------

log "Verifying ActivePrecompiles"
cmd_d query evm params --output json 2>/dev/null | jq '.params.active_precompiles' || true

log "Verifying module accounts"
for mod in scheduler streams; do
  cmd_d query auth module-account "$mod" --output json 2>/dev/null \
    | jq -r '.account.value.address // .account.base_account.address // "MISSING"' \
    | xargs -I{} printf '  %s -> %s\n' "$mod" {} || true
done

log "Done. Chain home: $HOMEDIR"
log "Logs:             $LOGFILE"
log "RPC (Tendermint): http://localhost:26657"
log "JSON-RPC (EVM):   http://localhost:8545"
log "Stop node:        pkill -f 'evmosd start.*$HOMEDIR'"
