# HyperPaxeer v19-paxspot — Validator Node Image
# Includes: evmosd binary, price_submitter sidecar, cast (Foundry)
#
# Build:
#   docker build -t hyperpaxeer:v19-paxspot -f Dockerfile .
#
# The build context must contain:
#   ./evmosd                — Pre-built HyperPaxeer binary (from build/evmosd)
#   ./price_submitter.sh    — Validator price submission script
FROM ubuntu:24.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    jq \
    bash \
    curl \
    bc \
    lz4 \
    && rm -rf /var/lib/apt/lists/*

# Copy pre-built binary
COPY evmosd /usr/local/bin/evmosd
RUN chmod +x /usr/local/bin/evmosd && evmosd version

EXPOSE 26656 26657 1317 9090 8545 8546

ENTRYPOINT ["evmosd"]
