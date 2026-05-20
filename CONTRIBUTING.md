# Contributing to HyperPax-OS

Thanks for your interest in contributing to the Paxeer Network node software. This guide covers what we expect from pull requests, how the codebase is organized, and how chain upgrades are coordinated.

If you've never touched a Cosmos SDK or Evmos-family codebase before, the [README](./README.md) is the right place to start. This document assumes you've already built and run a local node.

---

## Where to start

- **Bug reports** — open a [GitHub issue](https://github.com/Paxeer-Network/hyperpax-os-cronosRelease/issues) using the bug report template.
- **Feature ideas** — open an issue with the `proposal` label first. Discussion happens before code review.
- **Security findings** — do **not** open a public issue. Follow [`SECURITY.md`](./SECURITY.md).
- **Documentation gaps** — small fixes can go straight to a PR. Larger restructuring should start as an issue.
- **First-time contributor?** — look for issues tagged [`good first issue`](https://github.com/Paxeer-Network/hyperpax-os-cronosRelease/issues?q=label%3A%22good+first+issue%22).

---

## Repository conventions

### License headers

Every new Go file in this repo starts with:

```go
// Copyright PaxLabs Ltd.(Paxeer Network)
// HyperPax-OS-Protocol License (https://github.com/Paxeer-Network/hyperpax-os-cronosRelease/blob/main/LICENSE_FAQ.md)


package <pkg>
```

The blank line before `package` is intentional — it matches the rest of the repo. By contributing code you confirm it's licensed under the [HyperPax-OS-Protocol License](./LICENSE).

### Branch model

- `main` is the released branch. It must always build, lint, and pass the unit test suite.
- Feature work happens on topic branches off `main`, named `<initial>/<short-slug>` (e.g. `aw/scheduler-cancel-fix`).
- Hard-fork upgrade work happens on `release/v<NN>-<name>` branches (e.g. `release/v21-agent-payments`). These get rebased onto `main` once the upgrade ships.
- Force-pushing to `main` or any `release/*` branch is forbidden outside of release-manager intervention.

Open PRs against `main` unless the change is specifically scoped to an active upgrade branch.

### Commit messages

We follow [Conventional Commits](https://www.conventionalcommits.org/). Common types in this repo:

- `feat(<scope>):` — new functionality
- `fix(<scope>):` — bug fix
- `refactor(<scope>):` — non-behavioral code changes
- `chore(<scope>):` — tooling, CI, deps
- `docs:` — documentation only
- `test:` — test-only changes
- `upgrade(v21):` — code that lands as part of a specific hard-fork upgrade

Examples:
- `feat(scheduler): allow rescheduling within same block`
- `fix(streams): correct accrual rounding on close`
- `upgrade(v21): activate 0x907 and 0x908 in EVM params`

Keep messages short, descriptive, and in the imperative ("add", not "added").

### Code style

- **Go**: run `make format` (gofumpt) and `make lint-fix` before pushing. CI runs `golangci-lint` on every PR.
- **Solidity**: run `cd contracts/paxspot && forge fmt`. CI runs `solhint` and `slither`.
- **Imports**: stdlib → third-party → `github.com/evmos/evmos/v18/...`, separated by blank lines.
- **Errors**: register sentinels via `errorsmod.Register(ModuleName, code, msg)` in `x/<name>/types/errors.go`. Reserve code `1` for "internal error". Wrap with `err.Wrap(...)` / `err.Wrapf(...)`.
- **Comments**: explain *why*, not *what*. Public API needs godoc.

### Determinism

This is a consensus-layer codebase. Anything that runs inside a precompile's `Run()`, an ABCI hook, or a keeper called during block processing **must be deterministic across validators**:

- Use `ctx.BlockTime()` and `ctx.BlockHeight()`. Never `time.Now()`.
- Never read environment variables, system entropy, or wall-clock time inside consensus paths.
- Map iteration order is not deterministic — sort keys before iterating if the result feeds state.
- All randomness must come from a deterministic source (e.g. block-hash mixing, future VRF precompile).

Reviewers will block PRs that introduce nondeterministic behavior. If you're unsure whether a code path is consensus-critical, ask in the PR.

---

## Repository layout

A short tour of where things live. The full layout is in the [README](./README.md#repository-layout).

| What | Where | Reference template |
| --- | --- | --- |
| Cosmos modules | `x/<name>/` | `x/paxoracle/` is the canonical pattern |
| Precompiles (stateful) | `precompiles/<name>/` | `precompiles/paxspot/oracle/` |
| Precompiles (stateless) | `precompiles/<name>/` | `precompiles/paxspot/orob/` |
| Solidity interfaces | `contracts/paxspot/src/interfaces/precompiles/` | `IOracleAggregator.sol` |
| Upgrade handlers | `app/upgrades/<version>/` | `app/upgrades/v19_paxspot/` |
| App wiring | `app/app.go`, `app/keys.go` | follow existing imports/initialization order |
| EVM precompile registry | `x/evm/keeper/precompiles.go` | add new constructors here |

When adding a new module or precompile, **mirror an existing one byte-for-byte** before deviating. Reviewers will compare against the reference template.

---

## Pull request process

### Before opening a PR

1. **Run the inner loop.** `make build` then the relevant module/precompile tests:
   ```bash
   go build ./...
   go test ./x/<module>/... -count=1
   go test ./precompiles/<name>/... -count=1
   ```
2. **Run the wider checks.** `make lint test-unit` should pass cleanly. For consensus-touching changes also run `make test-race`.
3. **Add tests.** New behavior needs unit tests at minimum. Anything that touches state, gas, or precompile logic needs an integration test under `precompiles/<name>/integration_test.go` or `x/<name>/keeper/keeper_test.go`.
4. **Add a CHANGELOG entry.** Append a one-line entry under the `[Unreleased]` section in [`CHANGELOG.md`](./CHANGELOG.md). Categorize under *Added*, *Changed*, *Fixed*, or *Removed*.
5. **Update docs.** If your PR changes a public interface, update the relevant Solidity interface, README section, or `app/upgrades/<version>/README.md`.

### Opening the PR

- Use a Conventional Commits style title.
- Open in **Draft** if you want early feedback. Mark **Ready for review** when you believe it can merge.
- Reference the issue in the description (`Closes #123`).
- Fill in the PR template — the checklist exists to catch the things reviewers check anyway.

### Review process

- All non-trivial PRs require **two approvals** before merge.
- Consensus-layer or precompile changes require at least one approval from a core maintainer.
- Reviewers leave one of three signals:
  - **`LGTM` (comment only)** — surface review, didn't run the code.
  - **Approval (GitHub UI)** — read the diff, ran the relevant tests locally, willing to vouch for the change.
  - **Request changes** — blocking concern. Address before merge.
- Squash-merge is the default. Use a merge commit only when the individual commits are independently reviewable and worth preserving.

### After merge

- The contributor's last commit message becomes the squash-merge title — make sure it makes sense as a standalone changelog line.
- The CHANGELOG entry stays under `[Unreleased]` until the next release cuts.

---

## Chain upgrades

Coordinated hard-fork upgrades go through a stricter flow. The current upgrade ladder is:

| Version | Status | What it adds |
| --- | --- | --- |
| `v19-paxspot` | shipped | OROB, BatchClearing, Oracle, PoFQ precompiles (`0x901`–`0x904`) |
| `v20-agent-foundations` | active | EIP-7702, gas reductions, agent fee lane, Scheduler (`0x905`) |
| `v21-agent-payments` | active | PaymentStreams (`0x906`), TEEAttestor (`0x907`), EIP-712 helper (`0x908`) |

If your change has to land in a coordinated fork:

1. Open the PR against the `release/<version>` branch, not `main`.
2. Title the PR `upgrade(<version>): ...`.
3. Update the `app/upgrades/<version>/upgrades.go` handler if your change requires migration logic, store-key adds, or EVM param changes.
4. Add or update the per-upgrade `README.md` under `app/upgrades/<version>/`.
5. Coordinate with a core maintainer before merging — upgrade branches are protected.

The full design rationale and gas tables for `v20`/`v21` live in [`Paxeer_Chain_Upgrades.md`](../Paxeer_Chain_Upgrades.md).

---

## Testing

- **Unit tests** — `make test-unit`. Should be fast (single-digit seconds per package).
- **Race detector** — `make test-race`. Catches goroutine bugs in scheduler, streams, and abci hooks.
- **Integration tests** — `go test ./precompiles/<name>/integration_test.go`. Spins up an in-process node and exercises the precompile through Solidity consumers.
- **JSON-RPC integration** — `make test-rpc`. Spins up a fresh local node and runs the EVM RPC test suite.
- **Solidity tests** — `make test-solidity`. Runs Foundry tests against a fresh local node.
- **End-to-end upgrade tests** — `make test-e2e`. Slow. Pulls released Docker images and replays the upgrade ladder. Run before tagging a release; not required on every PR.

PRs that drop test coverage on touched packages will be sent back.

---

## Documentation

- Public-facing docs live at [docs.paxeer.app](https://docs.paxeer.app/) — that's a separate repo.
- Repo-internal docs (architecture notes, per-upgrade READMEs, design rationales) live alongside the code they describe.
- The top-level [`README.md`](./README.md) is the entry point. Keep it current.
- API surface (Solidity interfaces, gRPC queries) should be documented in the file that defines it.

---

## Dependencies

- We use Go modules. `go.mod` is the source of truth.
- Don't add a dependency for something the standard library already does.
- Cosmos SDK and CometBFT versions are pinned. Bumping them is its own coordinated PR with a full upgrade-test pass.
- Solidity contracts use Foundry; dependencies are vendored under `contracts/paxspot/lib/`.

If a dependency is broken upstream, prefer `go mod tidy` and a focused workaround over a fork. If a fork is genuinely necessary, document the rationale in the PR.

---

## Code of conduct

Contributing to this repo means agreeing to the [Code of Conduct](./CODE_OF_CONDUCT.md). Treat reviewers and maintainers the way you'd want to be treated. Disagreements are fine; personal attacks are not.

---

## License

By submitting a contribution you agree it will be licensed under the [HyperPax-OS-Protocol License](./LICENSE). Plain-language explanation in [`LICENSE_FAQ.md`](./LICENSE_FAQ.md).

---

## Questions?

- Bugs / feature requests: [GitHub Issues](https://github.com/Paxeer-Network/hyperpax-os-cronosRelease/issues)
- Security: [security@paxeer.app](mailto:security@paxeer.app)
- Licensing: [license@paxlabs.com](mailto:license@paxlabs.com)
- General: [docs.paxeer.app](https://docs.paxeer.app/)
