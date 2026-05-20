# Changelog

All notable changes to HyperPax-OS are documented here. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project follows [Semantic Versioning](https://semver.org/) for non-upgrade releases. Coordinated hard-fork upgrades use the `v<NN>-<name>` convention (e.g. `v21-agent-payments`).

The `[Unreleased]` section accumulates work-in-progress until a tag cuts.

---

## [Unreleased]

### Added
- Repository documentation suite (`README.md`, `CONTRIBUTING.md`, `SECURITY.md`, `LICENSE_FAQ.md`, `CODE_OF_CONDUCT.md`) aligned with HyperPax-OS-Protocol License.

### Changed
- CLI environment-variable prefix renamed from `EVMOS_*` to `PAXEER_*` (`cmd/evmosd/root.go`).
- Default chain-id prefix changed from `evmos_*` to `pax_*` for `evmosd init` and `evmosd testnet` when no chain-id is supplied.
- Default minimum gas price denomination in testnet bootstrap switched from `aevmos` to `ahpx` to match the active chain denom.
- Hard-coded upstream seed nodes removed from `evmosd init`. Operators configure peers via `--p2p.seeds` or `config.toml`.
- `local_node.sh` and `scripts/run_v20v21_localnet.sh` rebranded throughout (denom, chain-id, comments, log strings).

---

## v21-agent-payments — Coordinated upgrade (in progress)

Adds the agent-payments primitives. Activated by gov proposal post-deploy.

### Added
- **PaymentStreams precompile** at `0x0906` and backing `x/streams` module. Rate-based payment streams with O(1) settlement, optional caps, and PoFQ-shaped fill events on settle/close.
- **TEE Attestation precompile** at `0x0907` and backing `x/attestor` module. Verifies Intel TDX, AMD SEV-SNP, and NVIDIA H100 attestation quotes against a gov-managed root certificate registry. Verification cost is roughly 30k gas plus a per-family component, against ~1–10M gas for Solidity equivalents.
- **EIP-712 helper precompile** at `0x0908`. Native `hashTypedData`, `domainSeparator`, and `recoverTypedSigner`. Saves roughly 5k gas per signed-message verification.
- New gov-tunable params for the v21 precompile gas costs (`x/streams/types/params.go`, `x/attestor/types/params.go`).

### Changed
- `EVMParams.ActivePrecompiles` extended via the `v21-agent-payments` upgrade handler. Precompiles are dormant until activation.

---

## v20-agent-foundations — Coordinated upgrade (in progress)

Adds the substrate that the agent-payments upgrade builds on.

### Added
- **EIP-7702 in-protocol delegation.** New transaction type `0x04` (`SetCodeTx`) with authorization tuples. Validation in `app/ante/eth/`, application in `x/evm/keeper/state_transition.go`. Activation gated by `EIP7702BlockNumber` EVM param.
- **Native Scheduler precompile** at `0x0905` and backing `x/scheduler` module. Schedule arbitrary EVM calls at future block heights with a deposit that pays for execution. Replaces external keeper networks.
- **Agent fee lane** in `x/feemarket`. Flat-rate gas pricing for transactions where the caller is a registered AgentWallet or the target is a registered HPS service. Default disabled; activated by gov param.

### Changed
- **Precompile gas reductions** on `0x901`–`0x904`. Native execution costs dropped 5–10× across OROB, BatchClearing, OracleAggregator, and PoFQ. No interface changes; existing consumers benefit transparently.
  - OROB `resolveOffset`: 50 → **5**
  - OROB `resolveOffsetBatch` per item: 30 → **3**
  - BatchClearing base: 200 → **50**, per order: 30 → **3**
  - OracleAggregator `aggregate`: 100+50/feed → **20+5/feed**
  - PoFQ `scoreFill`: 50 → **5**, `scoreBatch` per fill: 40 → **3**
  - Full table in `Paxeer_Chain_Upgrades.md` §2.2.

---

## v19-paxspot — Coordinated upgrade

The PaxSpot release. Establishes the exchange primitives that the rest of the stack builds on.

### Added
- **OROB precompile** at `0x0901`. Oracle-Relative Order Book — orders priced as basis-point offsets from a live oracle.
- **BatchClearing precompile** at `0x0902`. Sealed-bid batch clearing for the volatility-mode execution path.
- **OracleAggregator precompile** at `0x0903`. Validator price aggregation backed by `x/paxoracle`.
- **PoFQ precompile** at `0x0904`. Proof-of-Fill-Quality scoring against the oracle reference price.
- `x/paxoracle` module — validator price-attestation pipeline with median quorum failover.
- `x/paxspot` module — exchange state (orders, fills, vaults, fill-quality scores).
- Solidity interfaces under `contracts/paxspot/src/interfaces/precompiles/`.

### Changed
- `EVMParams.ActivePrecompiles` extended for the PaxSpot precompile address range.

---

## Pre-PaxSpot upgrade history

The chain inherits Evmos v18's upgrade ladder. The following upgrade handlers remain in `app/upgrades/` to support state migration from older nodes:

- `v18.1.0` / `v18.0.0` — base release that PaxSpot forks from.
- `v17.0.0` — module manager and IBC client refactor.
- `v16.0.0` — staking precompile rework.
- `v15.0.0` — vesting cleanup and ICA/ICQ updates.
- `v14.0.0` — slashing parameter migration.
- `v13.0.2` — incentive module deprecation.
- `v12.1.0` / v9–v12 series — early Cosmos SDK and EVM module migrations.
- `v11.0.0` / `v10.0.0` — IBC and bank changes.
- `v9.1.0` / `v9.0.0` — initial coordinated forks of the upstream chain.
- `v8.x` series — earliest captured upgrade history.

These handlers are preserved verbatim. Bug fixes that affect them are not backported — operators running pre-`v18` nodes are expected to upgrade through the ladder before joining the current network.

---

## v1.0.0 — HyperPaxeer Network Genesis

### Features

- Full EVM compatibility on Cosmos SDK + CometBFT
- ERC-20 module with Single Token Representation v2
- EIP-1559 fee market mechanism
- JSON-RPC server with full Ethereum API support
- EIP-712 structured-data signing
- IBC interoperability
- Precompiles for staking, distribution, bank, and governance
- Ledger hardware wallet support

[Unreleased]: https://github.com/Paxeer-Network/hyperpax-os-cronosRelease/compare/v1.0.0...HEAD
