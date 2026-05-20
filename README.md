<div align="center">
  <img src="https://raw.githubusercontent.com/Paxeer-Network/.github/refs/heads/main/7684009b-8f63-4bc2-9904-25068ea7f8a5.png" alt="Paxeer Network" width="100%" />
</div>

<h1 align="center">HyperPax-OS — Cronos Release</h1>

<p align="center">
  <strong>The reference node implementation of Paxeer Network: a sovereign, EVM-compatible Cosmos chain built for the machine economy.</strong>
</p>

<p align="center">
  <a href="https://github.com/Paxeer-Network/hyperpax-os-cronosRelease/actions/workflows/build.yml"><img src="https://img.shields.io/github/actions/workflow/status/Paxeer-Network/hyperpax-os-cronosRelease/build.yml?branch=main&label=build&logo=github" alt="Build" /></a>
  <a href="https://github.com/Paxeer-Network/hyperpax-os-cronosRelease/actions/workflows/test.yml"><img src="https://img.shields.io/github/actions/workflow/status/Paxeer-Network/hyperpax-os-cronosRelease/test.yml?branch=main&label=tests&logo=go" alt="Tests" /></a>
  <a href="https://github.com/Paxeer-Network/hyperpax-os-cronosRelease/actions/workflows/lint.yml"><img src="https://img.shields.io/github/actions/workflow/status/Paxeer-Network/hyperpax-os-cronosRelease/lint.yml?branch=main&label=lint" alt="Lint" /></a>
  <img src="https://img.shields.io/badge/go-1.20%2B-00ADD8?logo=go" alt="Go" />
  <img src="https://img.shields.io/badge/EVM-chain%20id%20125-7C3AED" alt="EVM Chain ID 125" />
  <img src="https://img.shields.io/badge/Cosmos-hyperpax__125--1-2E3148?logo=cosmos" alt="Cosmos hyperpax_125-1" />
  <a href="https://docs.paxeer.app/"><img src="https://img.shields.io/badge/docs-paxeer.app-1f6feb" alt="Docs" /></a>
  <img src="https://img.shields.io/badge/license-HyperPax--OS--Protocol-orange" alt="License" />
</p>

<p align="center">
  <a href="https://docs.paxeer.app/">Documentation</a> ·
  <a href="https://docs.hyperpaxeer.com/performance">Network performance</a> ·
  <a href="https://paxscan.paxeer.app">PaxScan explorer</a> ·
  <a href="https://x.com/paxeernetwork">X / Twitter</a>
</p>

---

## What this is

HyperPax-OS is the node software that powers Paxeer Network. It is a Cosmos SDK chain with full EVM compatibility, sub-second deterministic finality from CometBFT, and a set of native precompiles and modules that exist specifically to make autonomous software — agents, compute providers, oracle networks, machine-operated marketplaces — first-class economic actors on-chain.

The chain stays familiar where it should. EVM tooling works without modification: Foundry, Hardhat, Remix, viem, ethers, wagmi all connect the same way they would to Ethereum. The Cosmos side gives Paxeer sovereign blockspace, IBC, and the freedom to extend consensus and execution at the layer where it actually matters.

The chain stops being familiar where the machine economy needs more than what generic EVMs offer. Native precompiles for scheduling, payment streams, TEE attestation, EIP-712 helpers, and oracle-relative pricing run as Go code at validator level — orders of magnitude cheaper than Solidity equivalents. EIP-7702 in-protocol delegation removes the bundler, factory, and init-code overhead that normally sits between an agent and its wallet. Argus VM enforces capital policy as a separate, parallel execution environment that talks to the EVM through clean read/write contract boundaries.

If you want the why behind all of this, read the [Machine Economy whitepaper](../Paxeer_Machine_Economy_Whitepaper_Final.md). If you want to run a node, this README is enough.

---

## Network at a glance

| Field | Value |
| --- | --- |
| **EVM chain ID** | `125` |
| **Cosmos chain ID** | `hyperpax_125-1` |
| **Bech32 prefix** | `pax` |
| **Native asset** | PAX (Cosmos-side denom: `ahpx`, atto-hpx) |
| **Display denom** | `hpx` (1 hpx = 10¹⁸ ahpx) |
| **Consensus** | CometBFT (PoS) |
| **Block time** | ~277 ms average, ~358 ms p99 |
| **Finality** | Instant deterministic (single-slot) |
| **Validators** | 10 active |
| **Public RPC nodes** | 48 Cosmos / 47 EVM JSON-RPC |
| **Explorer** | [PaxScan](https://paxscan.paxeer.app) |

The current public network endpoints, faucet, and validator set are tracked at [docs.paxeer.app](https://docs.paxeer.app/).

---

## Quick start

Build the daemon and run a single-node local chain. Tested on Linux and macOS.

### Prerequisites

- Go **1.20+** (`go version`)
- `make`, `git`, `jq`
- A C toolchain (`gcc` / Xcode CLT) for cgo

### Build

```bash
git clone https://github.com/Paxeer-Network/hyperpax-os-cronosRelease.git
cd hyperpax-os-cronosRelease
make install   # builds and installs `evmosd` to $GOPATH/bin
evmosd version
```

The binary is named `evmosd` for upstream compatibility — every Cosmos and EVM tool that already speaks to an Evmos-family daemon will work without changes.

### Run a local devnet

```bash
./local_node.sh
```

This wipes `~/.tmp-evmosd`, generates dev keys with funded balances, writes a genesis with `ahpx` denom and the `pax_9000-1` chain ID, enables JSON-RPC on `:8545`, and starts the node. Tail logs with `tail -f /tmp/evmosd.log`.

To exercise the v20-agent-foundations and v21-agent-payments hard-fork upgrades end-to-end on a local chain:

```bash
bash scripts/run_v20v21_localnet.sh
```

### Connect to public Paxeer

```bash
evmosd init my-node --chain-id hyperpax_125-1
# Drop the latest genesis into ~/.evmosd/config/genesis.json
# Configure --p2p.seeds and --p2p.persistent_peers from docs.paxeer.app
evmosd start --metrics --json-rpc.enable --api.enable --grpc.enable
```

EVM clients connect over JSON-RPC at the standard ports (`8545` HTTP, `8546` WS).

---

## Architecture

HyperPax-OS runs **two execution environments** in the same binary:

- **EVM OS Layer** (Evmos v18 fork) — handles everything users and dApps interact with directly. EVM transactions, the spot exchange contracts, the PaxSpot precompiles at `0x0901`–`0x0904`, the agent-tier precompiles at `0x0905`–`0x0908`, and module-level features like staking, gov, IBC, and feemarket.
- **Argus VM** — an external runtime for capital orchestration that reads PoFQ scores, realized PnL, and open positions through the on-chain `IHyperpax-osReader` interface, then sets agent allowances and policy through `IAllowanceProvider`. The two VMs evolve independently. The matching engine doesn't know how Argus allocates capital. Argus doesn't know how matching works.

The chain ships with eight on-chain primitives that don't exist on stock EVM stacks:

- **Oracle-Relative Order Book (OROB)** — orders priced as basis-point offsets from a live oracle, not absolute amounts. Quotes track the market without active management.
- **Adaptive Dual-Mode Execution** — continuous matching during calm markets, sealed-bid batch auctions during volatility spikes. Mode switches per block, per market, at consensus level.
- **Programmable Liquidity Vaults (PLVs)** — composable LP curves built from interchangeable primitives (constant-product, concentrated, sigmoid, etc.) plus modifiers (volatility scaling, inventory skew, time decay).
- **Proof-of-Fill-Quality (PoFQ)** — every fill scored against the oracle price. Scores feed fee shares, routing priority, and Argus capital allocation.
- **Lazy Net Settlement** — virtual balances update per fill, real token transfers batched every ~10 seconds. 5–10× cheaper settlement.
- **Native Scheduler** (`0x0905`) — schedule arbitrary EVM calls at a future block height with a deposit. Replaces external keeper networks.
- **PaymentStreams** (`0x0906`) — rate-based payment streams with O(1) settlement and PoFQ integration.
- **TEE Attestation** (`0x0907`) — verify Intel TDX, AMD SEV-SNP, and NVIDIA H100 attestation quotes on-chain in milliseconds instead of millions of gas.

Plus **EIP-7702** in-protocol delegation, **EIP-712 helper precompile** (`0x0908`), and an **agent fee lane** in `x/feemarket` for registered AgentWallets and HPS services.

The full design rationale and gas tables live in [`Paxeer_Chain_Upgrades.md`](../Paxeer_Chain_Upgrades.md).

---

## Repository layout

```
.
├── app/                     # ABCI app, ante handlers, upgrade handlers (v9 → v21)
│   ├── ante/                # transaction validation pipeline (cosmos + eth)
│   └── upgrades/            # one subdir per coordinated network upgrade
├── cmd/
│   ├── evmosd/              # daemon entry point (binary: `evmosd`)
│   └── config/              # bech32 prefixes, denom registration, BIP44
├── client/                  # CLI helpers (keys, testnet bootstrap, debug)
├── contracts/paxspot/       # Solidity contracts that sit on top of the precompiles
├── ibc/                     # IBC middleware (transfer, callbacks, claims)
├── precompiles/
│   ├── paxspot/             # OROB, Clearing, Oracle, PoFQ (0x901–0x904)
│   ├── scheduler/           # native cron precompile (0x905, v20+)
│   ├── streams/             # payment streams (0x906, v21+)
│   ├── teeattestor/         # TEE quote verification (0x907, v21+)
│   ├── eip712/              # typed-data helper (0x908, v21+)
│   └── ...                  # staking, distribution, ics20, etc.
├── proto/                   # protobuf definitions for all custom modules
├── rpc/                     # EVM JSON-RPC namespaces, websocket, eth filters
├── scripts/                 # operational scripts (localnet, integration tests)
├── server/                  # JSON-RPC server config, indexer, telemetry
├── tests/
│   ├── e2e/                 # Docker-based upgrade tests across released versions
│   └── nix_tests/           # nix-shell integration tests
├── third_party/iavl/        # vendored cosmos/iavl tree
├── types/                   # core chain types (denom constants, errors)
├── x/                       # Cosmos modules
│   ├── claims/              # airdrop / claims
│   ├── erc20/               # ERC-20 ↔ Cosmos coin bridge
│   ├── evm/                 # EVM module (custom precompile registry)
│   ├── feemarket/           # EIP-1559 + agent fee lane
│   ├── inflation/           # native inflation schedule
│   ├── paxoracle/           # validator price feed aggregation
│   ├── paxspot/             # exchange state (orders, fills, vaults)
│   ├── scheduler/           # native cron module (v20+)
│   ├── streams/             # payment streams module (v21+)
│   └── attestor/            # TEE root certificate registry (v21+)
├── local_node.sh            # one-shot local devnet bootstrap
├── Makefile                 # build, test, lint, abis-export
└── go.mod
```

---

## Development

### Common make targets

```bash
make build              # local build → ./build/evmosd
make install            # install to $GOPATH/bin
make test-unit          # unit tests
make test-race          # race detector
make test-e2e           # docker-based upgrade e2e (slow)
make lint               # golangci-lint + solhint
make lint-fix           # auto-fix lint
make format             # gofumpt
make vulncheck          # govulncheck against ./...
make abis-export        # export precompile ABIs to ../paxeer-sdk/abis
```

### Working on a precompile

The reference template is `precompiles/paxspot/oracle/` paired with `x/paxoracle/`. Every new precompile follows the same shape:

1. `precompiles/<name>/<name>.go` — implements `vm.PrecompiledContract` (`Address`, `RequiredGas`, `Run`).
2. `precompiles/<name>/abi.json` — go-ethereum ABI.
3. `precompiles/<name>/<Name>.sol` — Solidity interface for consumers.
4. `x/<name>/` — Cosmos module (keeper, types, genesis, abci hooks).
5. Wire in `x/evm/keeper/precompiles.go`, `app/keys.go`, `app/app.go`.
6. Activate via param flag in the relevant upgrade handler under `app/upgrades/`.



### Working on a chain upgrade

Each upgrade lives in `app/upgrades/<version>/` with three files: `constants.go` (upgrade name, store-add list), `upgrades.go` (`CreateUpgradeHandler`), and an optional `agent/` package for the new module's bootstrap state. Register the upgrade in `app/upgrades.go`. Activation height is set by gov proposal post-deploy.

The current active branch is `v21-agent-payments`. The upgrade ladder is documented in `app/upgrades/HANDOFF_v20v21_implementation_pass2.md`.

### Running tests against a real chain

```bash
make test-rpc           # JSON-RPC integration tests against a fresh local node
make test-solidity      # solidity contract tests against a fresh local node
```

E2E upgrade tests pull released `tharsishq/evmos` and `paxeernetwork/hyperpax-os` Docker images and replay genesis through every upgrade handler in order. They are slow but they catch state-migration bugs that unit tests cannot.

---

## SDK

The companion repository — [`paxeer-sdk`](https://github.com/Paxeer-Network/paxeer-sdk) — generates idiomatic TypeScript and Python wrappers for every precompile ABI. The chain exports them via:

```bash
make abis-export DEST=../paxeer-sdk/abis
```

Use the SDK from agent code; use the precompiles directly only when you need fine control or are writing a Solidity integration.

---

## Security

Smart contract and consensus-layer security is taken seriously. Reporting paths:

- **Critical** (consensus halt, fund loss, signature bypass): contact `security@paxeer.app` via PGP — see `SECURITY.md` once restored.
- **High / medium**: open a [GitHub security advisory](https://github.com/Paxeer-Network/hyperpax-os-cronosRelease/security/advisories/new) on this repo.
- **Audits**: completed audit reports are linked from the docs site as they ship.

The chain ships with `make vulncheck`, semgrep, slither, codeql, and a custom consensuswarn workflow on every PR. None of those replace a real audit; they catch the obvious cases.

---

## Contributing

Pull requests are welcome. Before opening one:

1. Run `make lint test-unit` locally.
2. Stick to the existing module/precompile pattern. New consensus-layer code without an existing template is reviewed by the core team before merge.
3. Sign your commits if you are contributing externally.
4. Reference the upgrade name (`v20-agent-foundations`, `v21-agent-payments`, etc.) in your PR title if the change has to land in a coordinated fork.

For larger changes, open an issue first and label it `proposal`. Discussion happens there before any code review.

---

## License

Source is released under the **HyperPax-OS-Protocol License** (PaxLabs Inc., 2026). See [`LICENSE`](./LICENSE) for the full text and [`LICENSE_FAQ.md`](./LICENSE_FAQ.md) for plain-language guidance on what is and isn't permitted.

Some files retain upstream copyright headers (Cosmos SDK, Evmos, go-ethereum, IAVL); those keep their original licenses. Dual-licensed files are marked at the top of each file.

---

## Acknowledgements

HyperPax-OS builds on years of work by the Cosmos SDK, CometBFT, go-ethereum, and Evmos teams. The chain wouldn't exist without their open-source foundations.

<p align="center">
  <sub>Built by <a href="https://paxeer.app">Paxeer Network</a>. Settlement, coordination, and reputation infrastructure for the machine economy.</sub>
</p>
