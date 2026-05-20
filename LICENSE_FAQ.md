# HyperPax-OS-Protocol License — FAQ

This document is a plain-language guide to the **HyperPax-OS-Protocol License** (the "License") that governs this repository. It is **not** a substitute for the License itself. If anything here conflicts with [`LICENSE`](./LICENSE), the License controls.

> Contact: [license@paxlabs.com](mailto:license@paxlabs.com) · [legal@paxlabs.com](mailto:legal@paxlabs.com)

---

## TL;DR

- You can **read, run, integrate, and call** HyperPax-OS as much as you like.
- You can **fork and modify** it, but if you ship the modified version you must release your changes under this same license.
- You only need a **paid Commercial License** if you cross one of the dollar-denominated triggers in §5.2 of the License.
- Routing flow, market-making, and arbitrage through the chain currently sit under an **enforcement waiver** (§5.3) — revocable at any time, with a 10-day cure window.

If your situation is unusual, email [license@paxlabs.com](mailto:license@paxlabs.com) before you ship.

---

## What's covered by this license?

> **What is the "Licensed Work"?**

Everything in this repository that PaxLabs publishes under this License: the HyperPax-OS execution engine, all custom Cosmos modules under `x/`, the precompiles under `precompiles/`, the Solidity interfaces under `contracts/paxspot/`, SDK stubs, schemas, configs, build/deploy scripts, tests, tooling, and the documentation that ships with each release. Future updates and patches PaxLabs publishes under this License are also Licensed Work.

> **Are third-party components covered too?**

No. Files marked with upstream copyright headers (Cosmos SDK, CometBFT, go-ethereum, Evmos, IAVL, etc.) keep their original licenses. The License explicitly preserves third-party terms — see §11.2 and any per-file headers.

---

## Free use

> **Can I run a node, query the chain, or build a dApp on Paxeer without a paid license?**

Yes. Operating a node, calling precompiles via JSON-RPC or gRPC, deploying contracts that call the Licensed Work through published ABIs, and building anything that interacts with Paxeer through the standard interfaces is **Pure Caller Use** (License §2.2). It is permitted for free.

> **Can I experiment, fork, or build a prototype?**

Yes. Non-commercial use — research, prototyping, hackathons, audits, community pilots — is free under §4. If you publish modifications, the copyleft in §3 applies. If you start charging fees that hit a Commercial Trigger (§5.2), you need a Commercial License.

> **Are security researchers protected?**

Yes. §2.3 grants an explicit safe harbor for good-faith security research and audits. We do still expect coordinated disclosure — see [`SECURITY.md`](./SECURITY.md).

---

## Modifying the code

> **Can I fork HyperPax-OS?**

Yes. The License is copyleft, not source-available. You can fork.

> **What changes when I modify it?**

Three things, per §3.1:

1. You must publish your modifications under the **same license** (`LicenseRef-Paxlabs-HyperPax-OS-Protocol`) and at no charge.
2. You must preserve all existing copyright, license, and third-party notices, mark your changes clearly with dates, and provide enough build/deploy instructions for someone else to reproduce your work.
3. You must add a prominent attribution: **"Powered by HyperPax-OS-Protocol — © Paxlabs Inc 2026"** in your repo README and in the user-facing UI where applicable.

> **What counts as a "Modification"?**

Forking the source, translating it, extending it, creating plug-ins or modules that share the same runtime or EVM address space (delegatecall / proxy patterns count), or bundling the Licensed Work with your additions as one product. Calling the chain through public ABIs is **not** modification.

> **What about code I write that just calls the chain?**

Independent code that calls or interfaces with the Licensed Work is **not** subject to the §3.1 copyleft. §3.3 makes this explicit. Your client-side dApp, your indexer, your trading bot — none of those need to be relicensed just because they talk to Paxeer.

---

## Commercial use

> **When do I need a paid Commercial License?**

When you cross any of these (§5.2):

| Trigger | Threshold |
| --- | --- |
| **Aggregated Charged Fees** in any rolling 12-month period | More than **USD $100,000** |
| **Liquidity Under Control (LUC)** at any point in time | More than **USD $10,000,000** |
| **Operator / direct LP** that bypasses Paxeer Network's permitted interfaces and hits triggers A or B above | Same thresholds |

You aggregate Affiliates and entities under common control. No splitting, white-labeling, or routing through subsidiaries to avoid a trigger — §5.2 closes that loophole explicitly with the **Control or Benefit Principle** (§1.4).

> **What counts as "Charged Fees"?**

Pretty much any value you receive in connection with operating the Licensed Work: trading fees, spreads, mark-ups, MEV / builder payments, order-flow payments, rebates, subscription fees, performance fees, token grants, airdrops tied to your operations, etc. The License lists the categories in §1.2. Measured at fair-market USD value when received or accrued.

> **What about market makers, arbitrage bots, and aggregators?**

§5.3 currently waives enforcement of the Commercial Triggers for **Volume Activities** — routing, aggregation, arbitrage, and market-making — even when you charge fees, capture spreads, or trade your own capital. Important caveats:

- **The waiver is not a license.** It creates no reliance rights.
- **It can be revoked any time.** PaxLabs publishes notice in this repo or contacts you directly.
- **You get 10 days** after revocation to either stop the activity or sign a Commercial License.
- **Past breaches unrelated to Volume Activities are not excused.**

If your business depends on this waiver, watch the repo and plan accordingly.

> **How do I get a Commercial License?**

Email [license@paxlabs.com](mailto:license@paxlabs.com). Per §5.4 you have 15 days from crossing a Trigger to make contact. Terms are negotiated and confidential.

---

## Audit and compliance

> **Can PaxLabs audit my fees / LUC?**

Yes — once a year under NDA, at PaxLabs's expense (§6). If under-reporting exceeds 5%, you reimburse the audit cost and remain liable for any other remedies. PaxLabs may also request "for cause" attestations if there are objective indications of a Trigger. You're expected to cooperate and provide accurate records.

> **What if I disagree about whether a Trigger fired?**

Talk to us first ([license@paxlabs.com](mailto:license@paxlabs.com)). The License covers governing law (New York) and venue (SDNY) in §10 if it gets that far, but the practical answer is: most edge cases are easier to resolve by emailing the legal address before you ship.

---

## IP, patents, and trademarks

> **Do I get any patent rights?**

A narrow one (§7.1): PaxLabs grants a patent license sufficient to exercise the rights you actually have under this License. The patent license terminates if you stop using the Licensed Work or assert any patent claim against PaxLabs or compliant users. There is no implied broader patent grant.

> **Can I use the "Paxeer" or "HyperPax-OS" names and logos?**

Only to make truthful statements like "compatible with HyperPax-OS" or "integrated with Paxeer Network". §7.2 does not grant trademark rights. Anything beyond factual compatibility statements needs separate written permission and must follow PaxLabs's brand guidelines.

> **Can I claim PaxLabs endorses my product?**

Not without a written agreement. §7.4 prohibits implying endorsement or certification.

---

## Termination

> **What ends my license?**

Material breach of any of §§2–9, not cured within 15 days of notice (§9). Distributions you made while compliant survive. The attribution, copyleft, warranty disclaimer, indemnity, and assignment clauses (§§2.4, 3, 8, 10, 11.3) survive termination.

---

## Common scenarios

> **I want to deploy a Solidity contract on Paxeer mainnet.**

Free. That's Pure Caller Use.

> **I want to fork the chain and run my own L1 with custom precompiles.**

Allowed under copyleft (§3). Your fork must be open under the same license, you must preserve notices, and you must publish "Powered by HyperPax-OS-Protocol — © Paxlabs Inc 2026" in your repo README and UI.

> **I want to embed HyperPax-OS in a closed-source product I sell.**

You need a Commercial License. The §3 copyleft prevents closed-source distribution of the Licensed Work or any Modification of it.

> **I'm running a market-making firm trading on Paxeer.**

Currently covered by the §5.3 waiver. Watch this repo and the License page for revocation notices. Plan for the 10-day cure window.

> **I'm building an SDK or framework that wraps Paxeer and charging subscriptions.**

Subscription revenue counts as Charged Fees. If you cross USD $100k in any rolling 12-month period, you need a Commercial License. Below that, free.

> **I want to write an article that quotes parts of the source code.**

Fine. §2.4 requires you to preserve copyright/license notices and include the attribution "HyperPaxeer — © Paxlabs Inc 2026" alongside any analysis or output you publish.

---

## Versioning

The License is versioned. Each release of the Licensed Work is governed by the License version included in that release tag. PaxLabs may publish new versions or re-release portions under different terms in the future (§12). Pinning to a specific commit or release locks you to that commit's License.

---

## Still unsure?

Email [license@paxlabs.com](mailto:license@paxlabs.com) **before** you ship. We'd rather have a 10-minute email exchange now than a 10-day cure window later.

For legal notices unrelated to the License, use [legal@paxlabs.com](mailto:legal@paxlabs.com) with subject `HyperPaxeer Notice` (per §11.1).
