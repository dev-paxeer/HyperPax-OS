#!/usr/bin/env bash
# run_all.sh — interactive test runner. Goes through every test in order.
# Pauses between upgrade halts so you can restart the node manually.

set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$DIR/lib.sh"

pause_for_restart() {
  local plan="$1"
  echo
  echo "==> Chain has halted after $plan upgrade."
  echo "==> Restart the node in another terminal:"
  echo "     evmosd start --home $HOMEDIR --chain-id $CHAINID \\"
  echo "        --minimum-gas-prices=0.0001${DENOM} \\"
  echo "        --json-rpc.api eth,txpool,personal,net,debug,web3"
  echo
  read -rp "Press <enter> once the node is back online and producing blocks... " _
  require_node_alive
}

bash "$DIR/00_alive.sh"

bash "$DIR/10_upgrade.sh" v19_paxspot
pause_for_restart "v19_paxspot"

bash "$DIR/10_upgrade.sh" v20-agent-foundations
pause_for_restart "v20-agent-foundations"

bash "$DIR/20_scheduler.sh"
bash "$DIR/60_eip7702.sh"

bash "$DIR/10_upgrade.sh" v21-agent-payments
pause_for_restart "v21-agent-payments"

bash "$DIR/30_streams.sh"
bash "$DIR/40_attestor.sh"
bash "$DIR/50_eip712.sh"

log "ALL TESTS PASSED"
