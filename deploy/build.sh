#!/usr/bin/env bash
# build.sh — Build the v19-paxspot Docker image for distribution.
#
# Usage:
#   ./build.sh                    # Build from pre-built binary in ../build/evmosd
#   ./build.sh --from-source      # Build binary from source inside Docker (slow)
#
# Output: Docker image tagged "hyperpaxeer:v19-paxspot"
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
IMAGE_TAG="hyperpaxeer:v19-paxspot"
BUILD_DIR=$(mktemp -d)

trap 'rm -rf "$BUILD_DIR"' EXIT

echo "=== Building $IMAGE_TAG ==="

if [[ "${1:-}" == "--from-source" ]]; then
    echo "Building binary from source (this takes 5-10 minutes)..."
    cd "$REPO_ROOT"
    go build -o "$BUILD_DIR/evmosd" ./cmd/evmosd
else
    BINARY="$REPO_ROOT/build/evmosd"
    if [[ ! -f "$BINARY" ]]; then
        echo "ERROR: Binary not found at $BINARY"
        echo "Either run 'go build -o build/evmosd ./cmd/evmosd' first,"
        echo "or use './build.sh --from-source'"
        exit 1
    fi
    cp "$BINARY" "$BUILD_DIR/evmosd"
fi

# Copy scripts and Dockerfile into build context
cp "$REPO_ROOT/scripts/price_submitter.sh" "$BUILD_DIR/price_submitter.sh"
cp "$SCRIPT_DIR/Dockerfile" "$BUILD_DIR/Dockerfile"

# Verify binary
echo "Binary: $(file "$BUILD_DIR/evmosd")"
echo "Size:   $(du -h "$BUILD_DIR/evmosd" | cut -f1)"

# Build Docker image
echo ""
echo "Building Docker image..."
docker build -t "$IMAGE_TAG" -f "$BUILD_DIR/Dockerfile" "$BUILD_DIR"

echo ""
echo "=== Build complete ==="
echo "Image: $IMAGE_TAG"
docker images "$IMAGE_TAG" --format "Size: {{.Size}}"
echo ""
echo "To save as tarball for distribution:"
echo "  docker save $IMAGE_TAG | gzip > hyperpaxeer-v19-paxspot.tar.gz"
echo ""
echo "To load on validator nodes:"
echo "  docker load < hyperpaxeer-v19-paxspot.tar.gz"
