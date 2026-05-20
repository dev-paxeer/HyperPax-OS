#!/usr/bin/env bash
# local_node.sh — Bootstrap a single-validator HyperPaxeer local chain for testing.
# Usage: ./scripts/local_node.sh [--reset]
#   --reset  Wipe existing data and re-initialize
#
# Requires: evmosd binary in build/ or PATH
set -euo pipefail

CHAIN_ID="hyperpax_125-1"
MONIKER="local-test"
HOME_DIR="${HOME}/.hyperpaxeer-local"
BINARY="${BINARY:-$(dirname "$0")/../build/evmosd}"
DENOM="ahpx"
DISPLAY_DENOM="hpx"
KEYRING="test"
KEY_NAME="deployer"
KEY_NAME_VAL="validator"

# Deterministic test mnemonic (DO NOT use in production)
DEPLOYER_MNEMONIC="gesture inject test cycle original hollow east ridge hen combine junk child bacon zero hope comfort vacuum milk pitch cage oppose unhappy lunar seat"

# Handle --reset flag
if [[ "${1:-}" == "--reset" ]] || [[ ! -d "$HOME_DIR" ]]; then
    echo "==> Resetting local chain data..."
    rm -rf "$HOME_DIR"
fi

if [[ ! -f "$BINARY" ]]; then
    echo "ERROR: Binary not found at $BINARY"
    echo "Run 'make build' first or set BINARY env var."
    exit 1
fi

# ─── Init if needed ──────────────────────────────────────────────────
if [[ ! -d "$HOME_DIR/config" ]]; then
    echo "==> Initializing chain ($CHAIN_ID)..."
    $BINARY init "$MONIKER" --chain-id "$CHAIN_ID" --home "$HOME_DIR" 2>/dev/null

    # Patch genesis: use ahpx denomination, short block time
    GENESIS="$HOME_DIR/config/genesis.json"

    # Replace all denom references with ahpx
    sed -i 's/"aevmos"/"ahpx"/g' "$GENESIS"
    sed -i 's/"aphoton"/"ahpx"/g' "$GENESIS"
    sed -i 's/"stake"/"ahpx"/g' "$GENESIS"
    # Set minimum gas price
    sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0ahpx"/' "$HOME_DIR/config/app.toml"
    # Enable API and JSON-RPC
    sed -i 's/enable = false/enable = true/' "$HOME_DIR/config/app.toml"
    sed -i 's/address = "127.0.0.1:8545"/address = "0.0.0.0:8545"/' "$HOME_DIR/config/app.toml"
    sed -i 's/ws-address = "127.0.0.1:8546"/ws-address = "0.0.0.0:8546"/' "$HOME_DIR/config/app.toml"
    # Fast block time for testing
    sed -i 's/timeout_commit = ".*"/timeout_commit = "1s"/' "$HOME_DIR/config/config.toml"

    # ─── Create accounts ─────────────────────────────────────────────
    echo "==> Creating validator key..."
    $BINARY keys add "$KEY_NAME_VAL" --keyring-backend "$KEYRING" --home "$HOME_DIR" 2>/dev/null

    echo "==> Importing deployer key from mnemonic..."
    echo "$DEPLOYER_MNEMONIC" | $BINARY keys add "$KEY_NAME" --keyring-backend "$KEYRING" --home "$HOME_DIR" --recover 2>/dev/null

    VAL_ADDR=$($BINARY keys show "$KEY_NAME_VAL" -a --keyring-backend "$KEYRING" --home "$HOME_DIR")
    DEPLOYER_ADDR=$($BINARY keys show "$KEY_NAME" -a --keyring-backend "$KEYRING" --home "$HOME_DIR")

    echo "  Validator: $VAL_ADDR"
    echo "  Deployer:  $DEPLOYER_ADDR"

    # Fund accounts in genesis
    # 1B HPX = 1e27 ahpx for validator, 100M HPX for deployer
    $BINARY add-genesis-account "$VAL_ADDR" "1000000000000000000000000000${DENOM}" --home "$HOME_DIR"
    $BINARY add-genesis-account "$DEPLOYER_ADDR" "100000000000000000000000000${DENOM}" --home "$HOME_DIR"

    # Create validator gentx
    $BINARY gentx "$KEY_NAME_VAL" \
        "500000000000000000000000000${DENOM}" \
        --chain-id "$CHAIN_ID" \
        --keyring-backend "$KEYRING" \
        --home "$HOME_DIR" \
        --moniker "$MONIKER" \
        --commission-rate "0.1" \
        --commission-max-rate "0.2" \
        --commission-max-change-rate "0.01" \
        --min-self-delegation "1" \
        2>/dev/null

    $BINARY collect-gentxs --home "$HOME_DIR" 2>/dev/null
    $BINARY validate-genesis --home "$HOME_DIR"

    echo "==> Genesis initialized."
fi

# ─── Print deployer info ─────────────────────────────────────────────
DEPLOYER_ADDR=$($BINARY keys show "$KEY_NAME" -a --keyring-backend "$KEYRING" --home "$HOME_DIR")
DEPLOYER_ETH=$($BINARY keys show "$KEY_NAME" -a --keyring-backend "$KEYRING" --home "$HOME_DIR" --bech eth 2>/dev/null || echo "(run 'evmosd debug addr $DEPLOYER_ADDR' to get hex)")

echo ""
echo "============================================"
echo "  Local HyperPaxeer Chain"
echo "  Chain ID:  $CHAIN_ID"
echo "  RPC:       http://127.0.0.1:26657"
echo "  EVM RPC:   http://127.0.0.1:8545"
echo "  Deployer:  $DEPLOYER_ADDR"
echo "  Deployer (ETH): $DEPLOYER_ETH"
echo "============================================"
echo ""

# ─── Start node ──────────────────────────────────────────────────────
echo "==> Starting node..."
exec $BINARY start \
    --home "$HOME_DIR" \
    --json-rpc.api eth,txpool,personal,net,debug,web3 \
    --json-rpc.enable \
    --api.enable \
    --minimum-gas-prices "0${DENOM}"
