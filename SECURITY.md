# Security Policy

Reporting vulnerabilities responsibly is a critical part of keeping Paxeer Network safe. This document explains how to report, what to expect from us, and what's in scope.

> **Report security issues to [security@paxeer.app](mailto:security@paxeer.app). Do not open public GitHub issues for vulnerabilities.**

---

## Reporting a vulnerability

Use [security@paxeer.app](mailto:security@paxeer.app) for all reports. PGP details are published at [docs.paxeer.app/security](https://docs.paxeer.app/security).

We ask that researchers:

- Disclose only through email — not GitHub, Discord, Telegram, X, or any other public channel — until the issue is patched and we've agreed on disclosure timing.
- Avoid violating user privacy, degrading service, or destroying data while reproducing the issue.
- Keep details confidential between you and the engineering team until disclosure.
- Avoid posting any personally identifiable information, public or private.

In return we commit to:

- Acknowledge your report within the response window below.
- Work with you to understand, reproduce, fix, and disclose the issue.
- Not pursue legal action against good-faith research that follows this policy.

---

## Disclosure process

1. **Acknowledge.** A report received at [security@paxeer.app](mailto:security@paxeer.app) is reviewed by the security lead and at least one engineer from the affected component. We score severity using [CVSS v4](https://nvd.nist.gov/vuln-metrics/cvss). First response targets:

   | Severity | First response |
   | --- | --- |
   | Critical | 48 hours |
   | High | 96 hours |
   | Medium / Low / Informational | 96 hours |

2. **Triage.** Confirmed `Informational` and `Low` reports are tracked as public issues once we've agreed they don't expose anything live. `Medium` reports are tracked internally and patched in the next release. `High` and `Critical` reports become a [GitHub Security Advisory](https://docs.github.com/en/code-security/repository-security-advisories/creating-a-repository-security-advisory).

3. **Fix.** For `High` / `Critical`:
   - Patches are developed in a [private fork](https://docs.github.com/en/code-security/repository-security-advisories/collaborating-in-a-temporary-private-fork-to-resolve-a-repository-security-vulnerability) of this repo.
   - Validators, the core team, and any directly-affected operators are notified privately first.
   - Patched releases are made public 24 hours after that notification.
   - One week after the patched release ships, we publish that the release contained a security fix (without exploit details).
   - Two weeks after that, we publish the full advisory and a CVE.

4. **Fix.** For `Medium` and below:
   - Tracked publicly, fixed in the next regular release.
   - A short post-mortem note is published one week after the release.

We try to handle every report in a timely fashion, but the process exists to make sure disclosures are consistent and downstream operators (validators, custodians, integrators, dApp developers) get the time they need to upgrade.

---

## Scope

We're interested in bugs with **demonstrable security impact** — anywhere from a unit-test reproduction to a complex multi-transaction exploit on a mainnet fork.

### In scope

The contents of this repository, in particular:

#### Consensus, EVM, and modules

- Memory allocation bugs, race conditions, timing attacks
- Information leaks, authentication bypasses
- Application or protocol-layer denial-of-service
- Lost-write bugs, double-spend, unauthorized account or capability access
- Loss or theft of funds, token inflation
- Payloads or transactions that cause panics
- Non-deterministic logic in consensus-critical code paths

#### JSON-RPC

- Write access beyond standard transaction submission
- Authentication bypasses
- Denial-of-service
- Leakage of secrets (keys, mnemonics, internal state)

#### P2P / RPC layer

- Amplification attacks
- Resource abuse
- Deadlocks and race conditions

#### Precompiles (`0x901`–`0x908`)

- State override via misuse of `DELEGATECALL`, `STATICCALL`, or `CALLCODE`
- Unauthorized state mutations (e.g. ERC-20 approvals or token transfers initiated through a precompile)
- Gas accounting bugs that allow underpriced or free execution
- Reentrancy paths that bypass Checks-Effects-Interactions

#### EVM module

- Memory allocation bugs
- Payloads that cause panics
- Authorization of invalid transactions
- EIP-7702 delegation flaws (improper authorization, signature replay, gas accounting)

#### Fee market / EIP-1559

- Memory allocation bugs
- Improper or unpenalized manipulation of `BaseFee`
- Agent fee-lane bypasses

### Out of scope

- Third-party services that integrate with Paxeer (block explorers, custodial wallets, exchanges) — report those to the operators
- Findings from social engineering (phishing, impersonation)
- Denial-of-service that requires unrealistic resources or only affects a single node
- Issues already publicly disclosed
- Bugs in unsupported releases (anything more than two minor versions behind the latest)
- Theoretical attacks without a working proof-of-concept

---

## Bounty payments

For `Critical` and `High` severity reports, payouts are made under [`SAFU.md`](./SAFU.md) — the Simple Arrangement for Funding Upload that governs whitehat rewards on the Paxeer chain.

Payouts require:

- The whitehat completes KYC/KYB through our independent service provider (see SAFU §"KYC/KYB Process"). The KYC provider keeps your information; the Paxeer team only sees the accept/reject result.
- The vulnerability is patched in production (mainnet).

Bounty math is in `SAFU.md` §"Reward Policy" — currently 5% of secured value, capped at 250,000 PAX.

---

## Supported releases

We commit to security patches for the **latest minor release** of HyperPax-OS. Operators running older versions are encouraged to upgrade promptly. While we won't actively backport patches further than that, downstream forks are welcome to do so for their own use.

If you operate a node, subscribe to release notifications on this repo so you don't miss security releases.

---

## Contact

- **Vulnerabilities**: [security@paxeer.app](mailto:security@paxeer.app)
- **General questions**: [docs.paxeer.app/security](https://docs.paxeer.app/security)
- **Legal / disclosure coordination**: [legal@paxlabs.com](mailto:legal@paxlabs.com) — subject `Security Coordination`

The security inbox is monitored continuously. If you believe you've found a Critical issue with active exploitation potential, mark the email subject `URGENT: SECURITY` and we'll page the on-call engineer.
