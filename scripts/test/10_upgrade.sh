#!/usr/bin/env bash
# 10_upgrade.sh <plan-name>  — submit, vote, and pass a gov upgrade proposal.
#
# Usage:
#   bash 10_upgrade.sh v19_paxspot
#   bash 10_upgrade.sh v20-agent-foundations
#   bash 10_upgrade.sh v21-agent-payments
#
# After this exits the chain will halt at the target block. You must restart
# the node manually:
#   evmosd start --home ~/.tmp-evmosd --chain-id pax_9000-1 \
#     --minimum-gas-prices=0.0001ahpx \
#     --json-rpc.api eth,txpool,personal,net,debug,web3

set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$DIR/lib.sh"

PLAN_NAME="${1:?usage: $0 <plan-name>}"
UPGRADE_OFFSET="${UPGRADE_OFFSET:-30}"
VOTING_BUFFER="${VOTING_BUFFER:-50}"   # voting_period (30s) + safety buffer

# Wait until a broadcast tx is included in a block. Returns 0 on success.
wait_tx() {
  local hash="$1"; local timeout="${2:-30}"
  local elapsed=0
  while [[ $elapsed -lt $timeout ]]; do
    local out
    out=$(d query tx "$hash" --output json 2>/dev/null || true)
    if [[ -n "$out" ]]; then
      local code; code=$(echo "$out" | jq -r '.code // 0')
      if [[ "$code" == "0" ]]; then return 0; fi
      echo "$out" | jq -r '.raw_log // "tx failed"' >&2
      return 1
    fi
    sleep 1; elapsed=$((elapsed + 1))
  done
  return 1
}

require_node_alive
cur=$(current_height)
upgrade_h=$((cur + UPGRADE_OFFSET))
log "[10] Submitting upgrade '$PLAN_NAME' at height $upgrade_h (current=$cur)"

# Gov module authority (cosmos-sdk v0.47+)
authority=$(d query auth module-account gov --output json 2>/dev/null \
            | jq -r '.account.value.address // .account.base_account.address')
[[ -n "$authority" && "$authority" != "null" ]] || fail "could not resolve gov authority"
ok "gov authority: $authority"

prop_file="/tmp/prop_${PLAN_NAME}.json"
cat >"$prop_file" <<EOF
{
  "messages": [{
    "@type": "/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade",
    "authority": "$authority",
    "plan": {
      "name": "$PLAN_NAME",
      "time": "0001-01-01T00:00:00Z",
      "height": "$upgrade_h",
      "info": "(localnet test for $PLAN_NAME)",
      "upgraded_client_state": null
    }
  }],
  "metadata": "ipfs://CID",
  "deposit": "20000000000000000000${DENOM}",
  "title": "Upgrade $PLAN_NAME",
  "summary": "Local test upgrade for $PLAN_NAME"
}
EOF

log "Submitting proposal"
txout=$(d tx gov submit-proposal "$prop_file" \
    --from "$VAL_KEY" --keyring-backend "$KEYRING" --chain-id "$CHAINID" \
    --gas 500000 --gas-prices "${BASEFEE}${DENOM}" \
    --yes --output json 2>&1 | tail -n 1)
submit_hash=$(echo "$txout" | jq -r '.txhash // empty')
echo "$txout" | jq -r '"  txhash=\(.txhash) code=\(.code)"' >&2
[[ -n "$submit_hash" ]] || fail "no txhash from submit-proposal"
wait_tx "$submit_hash" 20 || fail "submit-proposal tx not included or failed"
ok "submit-proposal included"
sleep 2

prop_id=$(d query gov proposals --output json | jq -r '.proposals | sort_by(.id|tonumber) | last.id')
[[ -n "$prop_id" && "$prop_id" != "null" ]] || fail "could not find proposal id"
ok "proposal id: $prop_id"

# Verify proposal entered voting period (not stuck in deposit period)
pstatus=$(d query gov proposal "$prop_id" --output json | jq -r .status)
ok "proposal status before vote: $pstatus"
[[ "$pstatus" == "PROPOSAL_STATUS_VOTING_PERIOD" || "$pstatus" == "2" ]] \
    || fail "proposal $prop_id not in voting period (status=$pstatus). Check deposit amount."

log "Voting yes"
vout=$(d tx gov vote "$prop_id" yes \
    --from "$VAL_KEY" --keyring-backend "$KEYRING" --chain-id "$CHAINID" \
    --gas 300000 --gas-prices "${BASEFEE}${DENOM}" \
    --yes --output json 2>&1 | tail -n 1)
vote_hash=$(echo "$vout" | jq -r '.txhash // empty')
[[ -n "$vote_hash" ]] || fail "no txhash from vote"
wait_tx "$vote_hash" 20 || fail "vote tx not included or failed"
ok "vote tx included ($vote_hash)"

# Sanity: confirm the vote was actually recorded
voter_addr=$(d keys show "$VAL_KEY" -a --keyring-backend "$KEYRING")
vote_record=$(d query gov vote "$prop_id" "$voter_addr" --output json 2>&1 || echo '{}')
opt=$(echo "$vote_record" | jq -r '.option // .vote.option // empty')
[[ "$opt" == "VOTE_OPTION_YES" || "$opt" == "1" ]] \
    && ok "vote recorded as YES" \
    || note "vote record query returned: $vote_record"

log "Waiting ${VOTING_BUFFER}s for voting period end + tally..."
sleep "$VOTING_BUFFER"

status=$(d query gov proposal "$prop_id" --output json | jq -r .status)
ok "proposal $prop_id final status: $status"
if [[ "$status" != "PROPOSAL_STATUS_PASSED" && "$status" != "3" ]]; then
  log "DIAG: tally + votes"
  d query gov tally "$prop_id" --output json 2>&1 | jq . >&2 || true
  d query gov votes "$prop_id" --output json 2>&1 | jq . >&2 || true
  fail "proposal did NOT pass: $status"
fi

log "[10] Upgrade scheduled at height $upgrade_h"
echo
echo "  >>> Chain will HALT at block $upgrade_h. Restart with:"
echo "  evmosd start --home $HOMEDIR --chain-id $CHAINID \\"
echo "    --minimum-gas-prices=0.0001${DENOM} \\"
echo "    --json-rpc.api eth,txpool,personal,net,debug,web3"
echo
