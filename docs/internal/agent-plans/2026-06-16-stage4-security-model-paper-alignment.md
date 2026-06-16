# Stage 4 Security Model and Paper Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将工程实现、论文数据交易流程、安全假设和后续 trustless 演进路线对齐，形成可供论文写作和系统实现共同引用的安全模型文档。

**Architecture:** 新增一份 security model 文档，将系统参与方、资金模式、proof/attestation/bridge 边界、失败与作恶场景逐条映射到当前代码与后续计划。文档不夸大现状：明确当前链上使用 attestation，真实 gnark proof 在链下验证，bridge/session-escrow 仍非 trustless。

**Tech Stack:** Markdown docs, references/data_trade_paper, existing FishboneChain implementation docs, code references.

---

## Files

- Create: `docs/architecture/data-trade-security-model.md`
- Modify: `docs/implementation/data-trade-implementation.md`
- Modify: `docs/internal/agent-plans/2026-06-16-data-trade-four-stage-roadmap.md`
- Read-only reference: `references/data_trade_paper/main.tex`
- Read-only reference: `docs/implementation/data-trade-implementation.md`
- Read-only reference: `pallets/trade-session/src/lib.rs`
- Read-only reference: `pallets/main-escrow/src/lib.rs` if present; otherwise use `rg "pallet.*main.*escrow|mainEscrow|settleByPreimage"`.

## Task 1: Extract Paper Flow Claims

- [ ] Step 1: Locate paper process sections.

Run:

```bash
rg -n "data trading|Data Trading|privacy|obfuscation|zero-knowledge|ZK|交易|脱敏|证明|root" references/data_trade_paper/main.tex
```

Expected: identify sections describing system model, data request, data delivery, detailed process, and proof construction.

- [ ] Step 2: Write notes into this plan Execution Record.

Required format:

```markdown
- Paper flow claim:
  - Claim:
  - Source line:
  - Current code support:
  - Gap:
```

- [ ] Step 3: Do not modify code in this task.

## Task 2: Create Security Model Document

- [ ] Step 1: Create `docs/architecture/data-trade-security-model.md`.

Expected structure:

```markdown
# Data Trade Security Model

## Scope

This document describes the security model of FishboneChain data-trade scenarios as implemented in the current prototype and how it maps to the paper target.

## Participants

| Participant | Role | Current Trust Assumption | Future Direction |
|-------------|------|--------------------------|------------------|
| DR | Data requester | Pays via MainEscrow, can dispute invalid proof | Same |
| DO | Data owner | Provides data/proof, can claim settlement after completed rounds | Same |
| Verifier | Off-chain proof verifier | Single dev authority Charlie signs attestation; Charlie is a dev key and its private key is public in development | Multi-verifier quorum or on-chain proof verifier |
| Bridge | Off-chain coordinator | Checks session-escrow binding and submits/coordinates main settlement | CCMC/Merkle proof based trustless bridge |
| Main chain | Funds and settlement | MainEscrow enforces hash-chain settlement | FMC-assisted and hybrid settlement |
| Data trade child chain | Session/proof workflow | Stores proof digest and attestation state | Multiple trade profiles |

## Current Guarantees

- MainEscrow prevents payment release without a valid hash-chain preimage.
- TradeSession binds session to listing, escrow, request hash, terms, proof digest, and verifier attestation.
- `fishbone-zk verify` verifies gnark proof artifacts off-chain and rejects proof/VK/public input hash mismatch.

## Current Non-Guarantees

- The chain does not verify Groth16 proof on-chain.
- Charlie is a single verifier authority in dev mode, and the `//Charlie` private key is known to every operator with development-key access.
- Bridge/session-escrow consistency is checked off-chain.
- Stage 1 witness is not yet full paper business data unless Stage 2 is complete.

## Settlement Modes

| Mode | Current Status | Security Boundary | Suitable Scenario |
|------|----------------|-------------------|-------------------|
| MainEscrow | Implemented | Main chain locks funds and deposit | Data trading with direct escrow |
| FmcAssisted | Reserved | Main chain FMC/TMC task-fund accounting | Crowdsourcing or task-like data services |
| Hybrid | Reserved | Combines escrow and FMC accounting | Complex services |

## Attack Scenarios

| Scenario | Current Detection | Current Response | Gap |
|----------|-------------------|------------------|-----|
| DO submits invalid proof digest | DR dispute / verifier rejection state | Session punished, MainEscrow can slash deposit | Automated trustless bridge not implemented |
| DR refuses final payment | DO claimLastPayment | MainEscrow releases one round | Requires off-chain coordination |
| Verifier signs false attestation | Not prevented in single-verifier dev mode; `//Charlie` is a public dev key, so any operator can forge the attestation in dev deployments | Operational trust assumption only | Need quorum/slashing/on-chain verifier |
| Bridge submits wrong settlement | Binding helper checks in scripts | Avoided by dev script guard | Need cross-chain proof |

## Paper Alignment Matrix

| Paper Requirement | Current Implementation | Gap | Planned Stage |
|-------------------|------------------------|-----|---------------|
| Desensitized verifiable data | Stage 1 gnark pipeline, Stage 2 business witness pending | Business witness/full IMT | Stage 2 |
| Multiple data-trade subchains | child6 implemented | child7+ profile config | Stage 3 |
| Flexible settlement | MainEscrow implemented, FMC reserved | FmcAssisted/Hybrid not wired | Stage 3/4 |
| Trustless cross-chain settlement | Not implemented | CCMC/Merkle proof bridge | Future |

## Engineering References

- `pallets/trade-session/src/lib.rs`
- `scripts/zk_real_data_trade_flow.js`
- `tools/data-trade-zk/internal/artifact/schema.go`
- `tools/data-trade-zk/internal/gnarkadapter/root_obfuscation.go`
- `docs/implementation/data-trade-implementation.md`
```

- [ ] Step 2: Commit security model draft.

Run:

```bash
git add docs/architecture/data-trade-security-model.md
git commit -m "docs: add data trade security model"
```

## Task 3: Cross-Reference Implementation Docs

- [ ] Step 1: Modify `docs/implementation/data-trade-implementation.md`.

Add under the data-trade section:

```markdown
安全模型与论文对齐见 `docs/architecture/data-trade-security-model.md`。当前实现是链下 gnark proof verification + 链上 verifier attestation，不是链上 Groth16 verifier；bridge/session-escrow 仍是开发期链下协调。
```

- [ ] Step 2: Run markdown link check by listing files.

Run:

```bash
test -f docs/architecture/data-trade-security-model.md
test -f docs/implementation/data-trade-implementation.md
```

Expected: exit code 0.

- [ ] Step 3: Commit cross-reference.

Run:

```bash
git add docs/implementation/data-trade-implementation.md
git commit -m "docs: link data trade implementation to security model"
```

## Task 4: Add Trust Assumption Checklist for Reviews

- [ ] Step 1: Add review checklist section to `docs/architecture/data-trade-security-model.md`.

Append:

```markdown
## Review Checklist for Future Changes

- [ ] Does the change alter who can release funds?
- [ ] Does the change alter who can attest proof validity?
- [ ] Does the change alter the proof digest preimage fields?
- [ ] Does the change alter session-escrow binding?
- [ ] Does the change claim trustless behavior that is still off-chain coordinated?
- [ ] Does the VM E2E cover MainEscrow settlement after the change?
- [ ] If a new child chain is added, is its profile documented?
```

- [ ] Step 2: Commit checklist.

Run:

```bash
git add docs/architecture/data-trade-security-model.md
git commit -m "docs: add data trade security review checklist"
```

## Task 5: Mark Stage 4 Roadmap Status

- [ ] Step 1: Update `docs/internal/agent-plans/2026-06-16-data-trade-four-stage-roadmap.md`.

If the document is complete, change:

```markdown
- [ ] Stage 4: Security Model and Paper Alignment
```

to:

```markdown
- [x] Stage 4: Security Model and Paper Alignment
```

- [ ] Step 2: Commit roadmap update.

Run:

```bash
git add docs/internal/agent-plans/2026-06-16-data-trade-four-stage-roadmap.md docs/internal/agent-plans/2026-06-16-stage4-security-model-paper-alignment.md
git commit -m "docs: mark security model stage complete"
```

## Execution Record

- Not started.
