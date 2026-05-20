#!/bin/bash
# =============================================================================
# railway-bootstrap.sh — Start command for Railway RPC services
#
# Env vars per Railway service:
#   NODE_NUM=1              (1-15, unique per service)
#   RESET_NODE=true         (wipe volume, fresh init + state sync)
#
# Volume mount: /data
# Image: ghcr.io/dev-paxeer/hyperpaxeer:railway
# =============================================================================
set -euo pipefail

CHAIN_ID="hyperpax_125-1"
MONIKER="hyperpax-railway-node-${NODE_NUM}"
DATA_DIR="/data"
GENESIS_URL="https://mainnet-beta.rpc.hyperpaxeer.com/genesis"
TARBALL_URL="${TARBALL_URL:-http://147.93.139.18:9999/hyperpax-resync-latest.tar.zst}"

# Reduced peer set — RPC nodes only need a few reliable peers for block sync.
# Fewer peers = fewer P2P connections = fewer pond goroutines = less memory.
ALL_PEERS="7833e281fad3098675352cc347cf2344292daf2a@147.93.139.18:26656,\
095d29800322acc0caf48e3eb415848b5ac5f6de@147.93.139.18:26706,\
5e33cba065fdc3a757b63fb1cdc68b8d7cfe1797@94.72.119.124:36656,\
b9d73e6fb0a3555ed57645cd852d40cc91899abb@161.97.117.111:36656,\
2ba171ceaf0541a2fb45b4d4838fb6aa7d9afdcc@161.97.116.202:36656,\
6c7321a13dc470e86365d1fb45b1f547aaeccb09@94.250.202.17:36656,\
b7d16e0aa5bf949172b0ada6095dc5b9d7c845b2@109.205.181.189:36656"

log() { echo "[$(date -u '+%H:%M:%S')] $*"; }

# =============================================================================
# RESET_NODE: wipe everything, fresh start
# Only triggers when RESET_NODE is EXPLICITLY set to "true".
# Unset, empty, or any other value = no wipe (safe default).
# =============================================================================
if [ "${RESET_NODE:-}" = "true" ]; then
    log "RESET_NODE=true — wiping volume for fresh tarball restore"
    rm -rf "$DATA_DIR/config" "$DATA_DIR/data"
else
    log "RESET_NODE not set to 'true' — preserving existing /data volume"
fi

# =============================================================================
# FIRST BOOT: Init + download tarball
# =============================================================================
if [ ! -f "$DATA_DIR/config/config.toml" ]; then
    log "=== FIRST BOOT: Initializing node $MONIKER ==="

    evmosd init "$MONIKER" --chain-id "$CHAIN_ID" --home "$DATA_DIR" 2>&1 | tail -1

    log "Fetching genesis..."
    curl -sf "$GENESIS_URL" | jq '.result.genesis' > "$DATA_DIR/config/genesis.json"
    GEN_CHAIN=$(jq -r '.chain_id' "$DATA_DIR/config/genesis.json")
    if [ "$GEN_CHAIN" != "$CHAIN_ID" ]; then
        log "FATAL: genesis chain_id=$GEN_CHAIN, expected $CHAIN_ID"
        exit 1
    fi
    log "Genesis OK: $(wc -c < "$DATA_DIR/config/genesis.json") bytes"

    log "Downloading chain data from $TARBALL_URL ..."
    cd /tmp
    for attempt in 1 2 3 4 5; do
        log "Download attempt $attempt/5..."
        if curl -f -L --progress-bar --retry 3 --retry-delay 5 -o resync.tar.zst "$TARBALL_URL"; then
            log "Download complete: $(ls -lh resync.tar.zst | awk '{print $5}')"
            break
        fi
        log "Download failed, retrying in 10s..."
        rm -f resync.tar.zst
        sleep 10
    done
    if [ ! -f /tmp/resync.tar.zst ] || [ ! -s /tmp/resync.tar.zst ]; then
        log "FATAL: Tarball download failed after 5 attempts"
        exit 1
    fi

    log "Extracting tarball..."
    mkdir -p "$DATA_DIR/data"
    cd "$DATA_DIR/data"
    rm -rf application.db blockstore.db memiavl.db state.db
    DLFILE="/tmp/resync.tar.zst"
    case "$TARBALL_URL" in
        *.tar.zst)
            zstd -dc "$DLFILE" | tar xf -
            ;;
        *.tar.gz|*.tgz)
            tar -xzf "$DLFILE"
            ;;
        *)
            # Auto-detect: try zstd first, fall back to gzip
            zstd -dc "$DLFILE" 2>/dev/null | tar xf - 2>/dev/null || tar -xzf "$DLFILE" || tar -xf "$DLFILE"
            ;;
    esac
    rm -f "$DLFILE"
    log "Data extracted: $(du -sh "$DATA_DIR/data" | awk '{print $1}')"

    echo '{"height":"0","round":0,"step":0}' > "$DATA_DIR/data/priv_validator_state.json"

    log "=== First boot init complete ==="
fi

# =============================================================================
# CONFIGURE (every boot — ensures config is always current)
# =============================================================================
log "Applying config for $MONIKER..."

CFG="$DATA_DIR/config/config.toml"
APP="$DATA_DIR/config/app.toml"

# ── config.toml: P2P (tuned for RPC node — minimize goroutine spawning) ──
sed -i "s|^persistent_peers = .*|persistent_peers = \"$ALL_PEERS\"|" "$CFG"
sed -i "s/^moniker = .*/moniker = \"$MONIKER\"/" "$CFG"
sed -i 's/^addr_book_strict = .*/addr_book_strict = false/' "$CFG"
sed -i 's/^pex = .*/pex = true/' "$CFG"
sed -i 's|^laddr = "tcp://127.0.0.1:26657"|laddr = "tcp://0.0.0.0:26657"|' "$CFG"
sed -i 's|^cors_allowed_origins = \[\]|cors_allowed_origins = ["*"]|' "$CFG"
sed -i 's/max_packet_msg_payload_size = .*/max_packet_msg_payload_size = 10240/' "$CFG"
# Lower P2P rates — RPC nodes don't gossip txs/blocks, just receive
sed -i 's/send_rate = .*/send_rate = 5120000/' "$CFG"
sed -i 's/recv_rate = .*/recv_rate = 5120000/' "$CFG"
sed -i 's/flush_throttle_timeout = .*/flush_throttle_timeout = "50ms"/' "$CFG"
# Cap connections to limit goroutine pool creation from pond
sed -i 's/max_num_inbound_peers = .*/max_num_inbound_peers = 10/' "$CFG"
sed -i 's/max_num_outbound_peers = .*/max_num_outbound_peers = 10/' "$CFG"
sed -i 's|^seeds = .*|seeds = ""|' "$CFG"
sed -i 's/^unconditional_peer_ids = .*/unconditional_peer_ids = "7833e281fad3098675352cc347cf2344292daf2a,fe0e3adc8b9c075b8f4612fb9823770cc0d5bba7,892741ed41b6ca2eb30a691c3daa1ac0f4a4e213,095d29800322acc0caf48e3eb415848b5ac5f6de"/' "$CFG"
sed -i 's/^max_txs_bytes = .*/max_txs_bytes = 268435456/' "$CFG"
sed -i 's/^ttl-duration = "0s"/ttl-duration = "300s"/' "$CFG"
sed -i 's/^ttl-num-blocks = 0$/ttl-num-blocks = 500/' "$CFG"

# ── config.toml: consensus ──
sed -i 's/^timeout_propose = .*/timeout_propose = "300ms"/' "$CFG"
sed -i 's/^timeout_propose_delta = .*/timeout_propose_delta = "100ms"/' "$CFG"
sed -i 's/^timeout_prevote = .*/timeout_prevote = "200ms"/' "$CFG"
sed -i 's/^timeout_prevote_delta = .*/timeout_prevote_delta = "100ms"/' "$CFG"
sed -i 's/^timeout_precommit = .*/timeout_precommit = "200ms"/' "$CFG"
sed -i 's/^timeout_precommit_delta = .*/timeout_precommit_delta = "100ms"/' "$CFG"
sed -i 's/^timeout_commit = .*/timeout_commit = "0ms"/' "$CFG"
sed -i 's/^skip_timeout_commit = .*/skip_timeout_commit = true/' "$CFG"

# ── config.toml: disable state sync (using tarball restore instead) ──
sed -i '/^\[statesync\]/,/^\[/{
    s/^enable = .*/enable = false/
}' "$CFG"

# ── app.toml: memiavl with aggressive FD mitigation ──
sed -i 's/^minimum-gas-prices = .*/minimum-gas-prices = "7hpx"/' "$APP"
sed -i 's/^pruning = .*/pruning = "default"/' "$APP"
sed -i 's/^iavl-cache-size = .*/iavl-cache-size = 0/' "$APP"

sed -i '/^\[memiavl\]/,/^\[/{
    s/^enable = .*/enable = true/
    s/^async-commit-buffer = .*/async-commit-buffer = 0/
    s/^snapshot-keep-recent = .*/snapshot-keep-recent = 1/
    s/^snapshot-interval = .*/snapshot-interval = 2000/
    s/^cache-size = .*/cache-size = 0/
    s/^zero-copy = .*/zero-copy = false/
}' "$APP"

sed -i '/^\[state-sync\]/,/^\[/{
    s/^snapshot-interval = .*/snapshot-interval = 5000/
    s/^snapshot-keep-recent = .*/snapshot-keep-recent = 1/
}' "$APP"

# ── app.toml: RPC bindings (0.0.0.0 for Railway networking) ──
sed -i 's|^address = "tcp://localhost:1317"|address = "tcp://0.0.0.0:1317"|' "$APP"
sed -i 's|^address = "localhost:9090"|address = "0.0.0.0:9090"|' "$APP"
sed -i 's|^address = "localhost:9091"|address = "0.0.0.0:9091"|' "$APP"
sed -i '/^\[api\]/,/^\[/{s/^enable = false/enable = true/}' "$APP"
sed -i 's/^enabled-unsafe-cors = false/enabled-unsafe-cors = true/' "$APP"

# ── app.toml: JSON-RPC (public RPC tuning) ──
sed -i '/^\[json-rpc\]/,/^\[/{
    s/^api = .*/api = "eth,net,web3,debug,txpool,personal"/
    s/^gas-cap = .*/gas-cap = 25000000/
    s/^evm-timeout = .*/evm-timeout = "30s"/
    s/^logs-cap = .*/logs-cap = 10000/
    s/^block-range-cap = .*/block-range-cap = 10000/
    s/^http-timeout = .*/http-timeout = "30s"/
    s/^http-idle-timeout = .*/http-idle-timeout = "2m0s"/
    s/^max-open-connections = .*/max-open-connections = 200/
    s/^allow-insecure-unlock = .*/allow-insecure-unlock = true/
    s/^enable-indexer = .*/enable-indexer = true/
}' "$APP"

log "Config applied."

# =============================================================================
# VOLUME CLEANUP (runs every 24h in background)
# Removes stale memiavl snapshots, old WAL files, addrbook bloat, temp files
# Keeps volume lean for Railway's limited disk
# =============================================================================
cleanup_volume() {
    while true; do
        sleep 86400  # 24 hours
        log "=== Volume cleanup starting ==="
        local before=$(du -sm "$DATA_DIR" 2>/dev/null | awk '{print $1}')

        # Remove old memiavl snapshots beyond the 1 we keep
        if [ -d "$DATA_DIR/data/memiavl.db" ]; then
            local snap_dir="$DATA_DIR/data/memiavl.db"
            local snap_count=$(find "$snap_dir" -maxdepth 1 -name "snapshot-*" -type d 2>/dev/null | wc -l)
            if [ "$snap_count" -gt 1 ]; then
                find "$snap_dir" -maxdepth 1 -name "snapshot-*" -type d 2>/dev/null | sort | head -n -1 | while read d; do
                    rm -rf "$d"
                    log "  removed old memiavl snapshot: $(basename "$d")"
                done
            fi
        fi

        # Remove old ABCI state sync snapshots (keep latest 1)
        if [ -d "$DATA_DIR/data/snapshots" ]; then
            local scount=$(ls -d "$DATA_DIR/data/snapshots"/[0-9]* 2>/dev/null | wc -l)
            if [ "$scount" -gt 1 ]; then
                ls -d "$DATA_DIR/data/snapshots"/[0-9]* 2>/dev/null | sort -n | head -n -1 | while read d; do
                    rm -rf "$d"
                    log "  removed old ABCI snapshot: $(basename "$d")"
                done
            fi
        fi

        # Compact WAL files
        if [ -d "$DATA_DIR/data/cs.wal" ]; then
            find "$DATA_DIR/data/cs.wal" -name "wal" -size +100M -exec truncate -s 0 {} \; 2>/dev/null
        fi

        # Remove stale addrbook (force peer rediscovery)
        if [ -f "$DATA_DIR/config/addrbook.json" ]; then
            local asize=$(stat -c%s "$DATA_DIR/config/addrbook.json" 2>/dev/null || echo 0)
            if [ "$asize" -gt 5242880 ]; then  # >5MB
                rm -f "$DATA_DIR/config/addrbook.json"
                log "  removed bloated addrbook (${asize} bytes)"
            fi
        fi

        # Remove any tmp/crash files
        find "$DATA_DIR" -name "*.tmp" -o -name "*.bak" -o -name "core.*" 2>/dev/null | xargs rm -f 2>/dev/null

        local after=$(du -sm "$DATA_DIR" 2>/dev/null | awk '{print $1}')
        log "=== Volume cleanup done: ${before}MB -> ${after}MB (freed $((before - after))MB) ==="
    done
}
cleanup_volume &
CLEANUP_PID=$!
log "Volume cleanup daemon started (PID=$CLEANUP_PID, runs every 24h)"

# =============================================================================
# START
# =============================================================================
log "=========================================="
log "Starting HyperPax Railway Node: $MONIKER"
log "=========================================="
log "Home:       $DATA_DIR"
log "Node num:   $NODE_NUM"
log "DB backend: memiavl (FD-mitigated)"
log "Cleanup:    24h volume janitor active"

ulimit -n 1048576 2>/dev/null || true

# Go runtime memory controls — critical for preventing OOM
# GOMEMLIMIT: soft limit tells GC to work harder before hitting Railway's 24GB hard limit
# GOGC=50: GC at 50% heap growth (default 100%) — more frequent GC, less peak memory
export GOMEMLIMIT=${GOMEMLIMIT:-18GiB}
export GOGC=${GOGC:-50}
export GOMAXPROCS=${GOMAXPROCS:-6}
log "Go runtime: GOMEMLIMIT=$GOMEMLIMIT GOGC=$GOGC GOMAXPROCS=$GOMAXPROCS"

exec evmosd start \
    --home "$DATA_DIR" \
    --chain-id "$CHAIN_ID" \
    --json-rpc.api eth,txpool,personal,net,debug,web3 \
    --api.enable \
    --json-rpc.enable \
    --json-rpc.address 0.0.0.0:8545 \
    --json-rpc.ws-address 0.0.0.0:8546 \
    --json-rpc.max-open-connections 200 \
    --json-rpc.enable-indexer \
    --json-rpc.gas-cap 25000000 \
    --json-rpc.evm-timeout "30s" \
    --json-rpc.block-range-cap 10000 \
    --json-rpc.logs-cap 10000 \
    --grpc.enable \
    --grpc.address 0.0.0.0:9090 \
    --pruning default \
    --minimum-gas-prices "7hpx" \
    --log_level warn
