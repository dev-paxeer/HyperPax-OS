# Simple Arrangement forThree files below. Copy each block into the path shown above it.

---

### 1. `/paxeer-sdk/hyperpax-os-cronosRelease/CODE_OF_CONDUCT.md`

```markdown
# Code of Conduct

Paxeer Network is committed to a respectful, professional, and harassment-free experience for everyone who participates in this project — contributors, maintainers, reviewers, users, and observers — regardless of age, body size, visible or invisible disability, ethnicity, sex characteristics, gender identity and expression, level of experience, education, socio-economic status, nationality, personal appearance, race, religion, or sexual identity and orientation.

This Code of Conduct is adapted from the [Contributor Covenant v2.1](https://www.contributor-covenant.org/version/2/1/code_of_conduct/).

## Our standards

Behavior that helps build a healthy community:

- Showing empathy and kindness toward other people
- Being respectful of differing opinions, viewpoints, and experiences
- Giving and gracefully accepting constructive feedback
- Accepting responsibility, apologizing to those affected by mistakes, and learning from the experience
- Focusing on what is best for the project and community as a whole

Behavior that is not acceptable:

- The use of sexualized language or imagery, or sexual attention or advances of any kind
- Trolling, insulting or derogatory comments, and personal or political attacks
- Public or private harassment
- Publishing others' private information — physical or email address, employer, or anything else they haven't chosen to make public — without their explicit permission
- Sustained disruption of discussions or reviews
- Other conduct that would reasonably be considered inappropriate in a professional setting

## Scope

This Code of Conduct applies in all project spaces — this repository, related Paxeer-Network repositories, the issue tracker, pull requests, code review, the Paxeer Discord and forums, and at any event where someone is representing the project.

It also applies when an individual is officially representing the community in public spaces. Examples include using an official project email address, posting from an official social media account, or acting as an appointed representative at an online or offline event.

## Enforcement responsibilities

Project maintainers are responsible for clarifying and enforcing the standards in this document. They have the right and responsibility to remove, edit, or reject comments, commits, code, issues, pull requests, wiki edits, and other contributions that are not aligned with this Code of Conduct, and will communicate the reasons for moderation decisions when appropriate.

## Reporting

Instances of abusive, harassing, or otherwise unacceptable behavior may be reported to the project team at **[conduct@paxeer.app](mailto:conduct@paxeer.app)**. All complaints will be reviewed and investigated promptly and fairly. Reporters' identities are kept confidential.

When reporting, please include:

- A description of the incident
- Where it happened (repo, PR, channel, event)
- When it happened
- Any relevant links, screenshots, or message IDs
- Whether you've already taken any steps yourself

You will receive an acknowledgment within **72 hours** and a follow-up with the outcome of the investigation. If your report involves a maintainer, contact [legal@paxlabs.com](mailto:legal@paxlabs.com) instead — the message will be routed to a maintainer not involved in the situation.

## Enforcement guidelines

Maintainers will follow these guidelines when determining the consequences for any action they deem in violation of this Code of Conduct. Severity escalates with the impact of the behavior and any prior history.

### 1. Correction
**Impact**: Inappropriate language or other behavior deemed unprofessional or unwelcome.
**Consequence**: A private written warning from maintainers, with clarity about the violation and an explanation of why the behavior was inappropriate. A public apology may be requested.

### 2. Warning
**Impact**: A violation through a single incident or series of actions.
**Consequence**: A warning with consequences for continued behavior. No interaction with the people involved — including unsolicited interaction with those enforcing the Code of Conduct — for a specified period. This includes avoiding interactions in community spaces as well as external channels like social media. Violating these terms may lead to a temporary or permanent ban.

### 3. Temporary ban
**Impact**: A serious violation, including sustained inappropriate behavior.
**Consequence**: A temporary ban from any sort of interaction or public communication with the community for a specified period. No public or private interaction with the people involved during this period. Violating these terms may lead to a permanent ban.

### 4. Permanent ban
**Impact**: Demonstrating a pattern of violation, including sustained inappropriate behavior, harassment of an individual, or aggression toward or disparagement of classes of individuals.
**Consequence**: A permanent ban from any sort of public interaction within the community.

## Attribution

This Code of Conduct is adapted from the [Contributor Covenant](https://www.contributor-covenant.org), version 2.1, available at https://www.contributor-covenant.org/version/2/1/code_of_conduct.html.

The enforcement guidelines were inspired by [Mozilla's code of conduct enforcement ladder](https://github.com/mozilla/diversity).

For answers to common questions about this Code of Conduct, see the FAQ at https://www.contributor-covenant.org/faq. Translations are available at https://www.contributor-covenant.org/translations.
```

---

### 2. `/paxeer-sdk/hyperpax-os-cronosRelease/SAFU.md`

```markdown
# Simple Arrangement for Funding Upload (SAFU)

This Simple Arrangement for Funding Upload (the "**SAFU**" or "**Arrangement**") is the post-exploit policy for whitehat actors who secure vulnerable funds on Paxeer Network. It is based on the SAFU framework published by [Jump Crypto](https://jumpcrypto.com/safu-creating-a-standard-for-whitehats/).

The Arrangement exists to remove three forms of friction that typically surround live exploits:

- **Legal uncertainty.** No clear grace period for hackers to declare themselves whitehats. No formal commitment from the project team about whether legal action will be taken.
- **Procedural ambiguity.** No clear destination for secured funds. No clear answer on whether — or how much — compensation will be paid for securing them.
- **Execution risk.** Conflicting proposals and stressed negotiation during an active exploit make outcomes worse for everyone.

By following this Arrangement you have a defined path to act, deposit, and claim a reward, with predictable consequences at each step.

---

## Concepts

- **Dropbox Address ("Dropbox")** — the on-chain destination where secured funds must be deposited. The Paxeer Dropbox is a `ModuleAccount`, which means it is not controlled by any individual or by the team.
- **Deposit Interval** — the grace period during which a sender must deposit funds in the Dropbox after removing them from a vulnerable protocol.
- **Claim Delay** — the minimum waiting period before a sender may claim their reward. This window lets the team and community assess the full scope of the exploit.
- **Sender Claim Interval** — the maximum waiting period after which the protocol may reclaim an unclaimed reward, so funds are not stranded indefinitely.
- **Bounty Percent** — the pro-rata share of secured funds claimable by the whitehat.
- **Bounty Cap** — the absolute maximum a single whitehat may claim, regardless of how much was secured.

---

## Statement for whitehats

PaxLabs Ltd. (the "**Team**") commits to **not pursue legal action** against whitehats who act in accordance with this Arrangement for vulnerabilities found in the Paxeer Network blockchain (EVM chain ID `125`).

### Timeline

- The Team grants **48 hours** ("**Grace Period**") from the moment a hacker obtains tokens via an exploited vulnerability to deposit those tokens to the Dropbox. After the Grace Period elapses without a deposit, the Team will treat the actor as malicious and acting outside this Arrangement.
- The Team commits that the **claim process will begin no later than 30 days** after the deposit, or during the next chain upgrade — whichever comes first. Claim execution may require an upgrade today; we expect this to become automatic once a dedicated trustless Cosmos module is in place.
- If the whitehat does **not claim** their reward within **30 days** of the deposit ("**Sender Claim Interval**"), the unclaimed tokens will be reclaimed and transferred to the Paxeer community pool.

### Reward policy

Whitehats who secure vulnerable funds may claim **5% of the total funds secured** ("**Bounty Percent**"), capped at a total of **250,000 PAX** ("**Bounty Cap**").

There is no minimum to the amount that can be secured. The reward floor is **1 atto-PAX** (`1 × 10⁻¹⁸` PAX, equivalent to 1 wei on Ethereum).

We strongly encourage whitehats to report **undisclosed** vulnerabilities through the standard channel: [security@paxeer.app](mailto:security@paxeer.app). See [[SECURITY.md](cci:7://file:///paxeer-sdk/.tmp/.md/SECURITY.md:0:0-0:0)](./SECURITY.md) for the full disclosure process.

---

## Dropbox for protocol funds

The Dropbox below is available on the Paxeer Network blockchain for transferring secured funds:

|         | Bech32 format                                  | Hex format                                     |
| ------- | ---------------------------------------------- | ---------------------------------------------- |
| Dropbox | `pax1c6jdy4gy86s69auueqwfjs86vse7kz3grxm9h2`   | `0xc6A4d255043ea1A2F79CC81c9940FA6433eb0A28`   |

While the original purpose of the Dropbox is to help secure vulnerable PAX tokens, it can also be used as a general-purpose escrow account for ERC-20 or other native tokens that have been exploited due to a vulnerability — provided the deposit is consistent with the spirit of this Arrangement.

The Team will serve as a mediator between the affected protocol and any whitehat who deposits secured funds. The Team is **not responsible or liable** for the outcome of any negotiation between those parties.

### Address derivation

The Dropbox address corresponds to a `ModuleAccount` whose address is derived deterministically from the first 20 bytes of the SHA256 hash of the literal string `safu`:

```bash
address = sha256([]byte("safu"))[:20]
```

The address cannot be controlled by any individual, the Team, or any subset of validators. It is only writable through governance or through the SAFU module's defined claim flow.

### Conditions for claiming

#### KYC / KYB

Claims with a **value above USD $1,000** require KYC (for individuals) or KYB (for entities) by our independent service provider, Provenance ([provenancecompliance.com](https://provenancecompliance.com)). The provider verifies your identity and submits a binary accept/reject result to the Paxeer team — the team does not see the underlying documentation.

The information submitted to the service provider typically includes:

- Email address
- Physical address
- Proof of address — a utility bill (mobile phone bills excluded) or bank statement no older than 3 months
- Government-issued passport or national ID, plus a selfie
- For entities: directors, beneficial owners, and corporate documents
- The on-chain receiving address for the bounty

The service provider may request documents in English or certified translations. Allow a few business days for the review.

#### Patch landing

Bounty payouts are released only after the underlying vulnerability is **patched in production** (mainnet) and the Team has confirmed the fix is live and stable.

---

## References

- [Jump Crypto — SAFU: Creating a Standard for Whitehats](https://jumpcrypto.com/safu-creating-a-standard-for-whitehats/)
- [[SECURITY.md](cci:7://file:///paxeer-sdk/.tmp/.md/SECURITY.md:0:0-0:0)](./SECURITY.md) — Paxeer's full vulnerability disclosure policy
```

---

### 3. `/paxeer-sdk/hyperpax-os-cronosRelease/.github/PULL_REQUEST_TEMPLATE.md`

```markdown
## Description

<!--
  What does this PR do, and why? Mention the most critical files to review.
  If this PR is part of a coordinated upgrade, prefix the title with `upgrade(v21):` etc.
-->

Closes #

## Type of change

<!-- Check at least one. -->

- [ ] `feat` — new feature
- [ ] `fix` — bug fix
- [ ] `refactor` — non-behavioral code change
- [ ] `perf` — performance improvement
- [ ] `chore` — tooling, CI, deps
- [ ] [docs](cci:9://file:///paxeer-sdk/hyperpax-os-cronosRelease/client/docs:0:0-0:0) — documentation only
- [ ] `test` — test-only change
- [ ] `upgrade(<version>)` — change that has to land in a coordinated hard-fork

## Affected components

<!-- Tick everything this PR touches. -->

- [ ] `app/` (ABCI app, ante, upgrades)
- [ ] [cmd/evmosd/](cci:9://file:///paxeer-sdk/hyperpax-os-cronosRelease/cmd/evmosd:0:0-0:0) (daemon entry, init, root, testnet)
- [ ] `client/` (CLI helpers)
- [ ] `precompiles/paxspot/` (`0x901`–`0x904`)
- [ ] `precompiles/scheduler/` (`0x905`)
- [ ] `precompiles/streams/` (`0x906`)
- [ ] `precompiles/teeattestor/` (`0x907`)
- [ ] `precompiles/eip712/` (`0x908`)
- [ ] `x/<module>/` — which: <!-- e.g. x/scheduler -->
- [ ] `contracts/paxspot/` (Solidity)
- [ ] RPC / JSON-RPC layer
- [ ] Genesis or state migration
- [ ] CI / build / tooling
- [ ] Documentation

## Author checklist

- [ ] Title uses [Conventional Commits](https://www.conventionalcommits.org/) prefix.
- [ ] Targeted the correct branch ([main](cci:1://file:///paxeer-sdk/hyperpax-os-cronosRelease/cmd/evmosd/main.go:17:0-32:1) for normal work, `release/v<NN>-<name>` for upgrade-bound changes).
- [ ] Added a `[Unreleased]` entry in [[CHANGELOG.md](cci:7://file:///paxeer-sdk/hyperpax-os-cronosRelease/CHANGELOG.md:0:0-0:0)](../CHANGELOG.md).
- [ ] Added or updated unit tests under the affected package.
- [ ] Added or updated integration tests if the change touches state, gas, or precompile logic.
- [ ] Ran `make lint test-unit` locally and they pass.
- [ ] If the change is consensus-critical, also ran `make test-race`.
- [ ] Updated relevant documentation (README, per-upgrade `app/upgrades/<version>/README.md`, Solidity interface comments).
- [ ] Reviewed my own diff in "Files changed" — left comments where reviewers will need extra context.
- [ ] No `time.Now()`, environment reads, or other non-deterministic calls in consensus paths.
- [ ] License header present on all new Go files.

## Upgrade-specific (only if `upgrade(<version>)`)

- [ ] Updated `app/upgrades/<version>/upgrades.go` handler with any required migration / store-key adds / EVM param changes.
- [ ] Updated `app/upgrades/<version>/README.md` with the rationale and activation notes.
- [ ] Confirmed activation is gated by the upgrade handler — no precompile or feature is reachable before the upgrade height.
- [ ] Coordinated with a core maintainer before merging.

## Testing

<!--
  How did you verify the change? Paste relevant commands and output.
  For consensus-critical changes, include a `make test-race` snippet.
-->

```

---

## Reviewers checklist

- [ ] Title prefix matches the actual change scope.
- [ ] All author checklist items are addressed (or clearly N/A with a note).
- [ ] Diff doesn't introduce non-determinism in consensus code paths.
- [ ] Test coverage on touched packages did not regress.
- [ ] CHANGELOG entry accurately describes the user-facing impact.
- [ ] For upgrade PRs: activation is properly gated and migration is reversible/safe.
```

---

That's all 3. Once you've pasted them in, let me know and I'll:
1. Run `go build ./...` to confirm nothing is broken (the .md files won't affect Go compilation but it's a free smoke test).
2. Delete the corresponding `.tmp/.md/` originals so the staging area only holds what's intentionally not being restored (agent docs, dev plans, third_party iavl docs).
```