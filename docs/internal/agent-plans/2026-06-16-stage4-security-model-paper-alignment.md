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

> **Note:** All task checkboxes below are historical. Completion is recorded in the Execution Record (see `### 2026-06-26 CodeWhale Stage 4 Execution Complete`).

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

Add a standalone `### 安全模型与论文对齐` subsection immediately after `### 边界与限制` and before `## 测试状态`.

Required text:

```markdown
### 安全模型与论文对齐

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

### 2026-06-26 CodeWhale Pre-Execution Questions

- Branch: (not yet created)
- Tasks completed: none
- Tests run: none
- Tests not run: none
- Deviations from plan: none yet
- **Questions for Codex/Owner:**
  - **Task 3 插入位置**：Plan 要求 "Add under the data-trade section" 将安全模型交叉引用加入 `data-trade-implementation.md`。当前文档已有"边界与限制"小节。待确认插入位置：追加到"边界与限制"末尾（方案 A）；或作为独立小节放"后续方向"之前（方案 B）。
- Remaining risks: none new

### 2026-06-26 Task 1 Complete — Paper Flow Claims Extracted

Source: `references/data_trade_paper/main.tex` (580 lines, IEEEtran, English)

- **Paper flow claim: System Model (Section 3)**
  - Claim: Three roles — Data Owner (DO) collects data, applies masking rules, delivers via off-chain channel. Data Requester (DR) submits requests with constraints and masking rules, interacts multi-round. Smart contracts: DC stores data digests/metadata, VC manages funds/deposits and verifies zk-SNARK proofs for dispute resolution.
  - Source line: L106-131
  - Current code support: DO = `pallet-data-registry` listing owner + `pallet-trade-session` proof submitter. DR = session creator, payment submitter, dispute caller. DC ≈ `pallet-data-registry` (stores listing, IMT root, proof params). VC ≈ `pallet-trade-session` (state machine, proof digest binding, verifier attestation).
  - Gap: Paper DC/VC are separate contracts on the same chain; FishboneChain splits across main (MainEscrow) and child6 (DataRegistry+TradeSession). Paper VC verifies zk-SNARK proofs on-chain; FishboneChain uses off-chain `fishbone-zk verify` + on-chain verifier attestation.

- **Paper flow claim: Threat Model (Section 3.2)**
  - Claim: Both DO and DR are modeled as potentially malicious. Malicious DO may provide fabricated/tampered/low-quality data. Malicious DR may refuse payment after receiving data.
  - Source line: L132-140
  - Current code support: `dispute_invalid_proof` (DR disputes bad proof), `dispute_invalid_plaintext` (DR disputes hash mismatch), `claim_last_payment` (DO remedies DR non-payment), `punish_data_owner` (MainEscrow slashes DO deposit).
  - Gap: Paper assumes on-chain zk-SNARK verification for disputes; current implementation relies on verifier attestation + off-chain proof verification.

- **Paper flow claim: Integrity Merkle Tree — IMT (Section 4.1)**
  - Claim: Four-layer Merkle tree (Entry → Dataset → Aggregate → Padding). Each entry field has leaf hash with salt. Dataset layer has attribute subtree + entry subtree. Aggregate layer combines multiple datasets. Padding layer enforces uniform depth for privacy.
  - Source line: L189-209
  - Current code support: `pallet-data-registry` stores `imt_root` in listing. `pallet-trade-session` stores `ro_proof_hash` in round state. Stage 2.2 `BusinessRangeProof` uses MiMC for hash constraints but does not implement full IMT membership; RO circuit uses gnark Merkle proof with depth=10.
  - Gap: Current RO circuit uses a simplified root obfuscation (4-root padding), not the full four-layer IMT with entry-level field hashing and salts. Full IMT with per-field salt hashing is future work.

- **Paper flow claim: Customizable Data Request (Section 4.2)**
  - Claim: DR specifies constraints (e.g., `age ∈ [20,40]`) and masking rules (e.g., `age → age ⊕ δ`, `name → hash(name)`). Masking is applied before delivery; constraints are verified via zk-SNARK.
  - Source line: L211-260
  - Current code support: Stage 2.2 `BusinessRangeProof` implements `raw_value ∈ [min, max]` + `masked_value = raw_value + delta` + `masked_value_hash = MiMC(masked_value, salt)`. `constraint_kind` = `Range` is stored in `RoundState`.
  - Gap: Only `Range` constraint kind is implemented. Subset, substring, and multi-field constraints are not implemented. Paper's masking rules are richer (additive, hash, etc.); current implementation uses additive masking only.

- **Paper flow claim: Multi-Round Delivery (Section 4.3)**
  - Claim: Five steps per round — (1) DR → DO: π_pc (payment commitment proof), (2) DO → DR: π (CH+RO proof of tailored dataset), (3) DR → DO: σ (signature of π), (4) DO → DR: DS_{T,i} (tailored dataset), (5) DR → DO: payment commitment. n rounds total.
  - Source line: L278-310
  - Current code support: `pallet-trade-session` implements the full multi-round state machine: `open_round` → `submit_payment_proof` → `submit_data_proof` → `attest_data_proof` → `submit_proof_signature` → `submit_data_delivery_hash` → `submit_payment_preimage`. `claim_settlement` after completed rounds.
  - Gap: Paper step (1) uses a zk-SNARK payment commitment proof (π_pc); current implementation uses hash-chain commitments in MainEscrow, not zk-SNARK for payment. Paper step (3) is DR's direct signature on proof; current implementation adds a verifier attestation step (Charlie) between DO's proof submission and DR's signature. Paper step (5) is a plaintext payment commitment; current uses hash-chain preimage.

- **Paper flow claim: Dispute Resolution (Section 4.4)**
  - Claim: DR can invoke VC to verify zk-SNARK proof on-chain and deduct DO deposit if proof is invalid or data does not match constraints. DO can claim payment if DR refuses.
  - Source line: L340-380 (approximate)
  - Current code support: `dispute_invalid_proof` (DR disputes, session marked Punished), `dispute_invalid_plaintext` (DR disputes hash mismatch), `punish_data_owner` (MainEscrow slashes DO deposit), `claim_last_payment` (DO remedies).
  - Gap: Paper dispute verification is on-chain zk-SNARK; current dispute relies on verifier attestation state (DR can dispute even after verifier accepted). Trustless on-chain proof verification not implemented.

### 2026-06-26 Codex Plan Clarification

- Decision: use a standalone subsection, not a bullet appended to `### 边界与限制`.
- Exact location: insert `### 安全模型与论文对齐` immediately after `### 边界与限制` and before `## 测试状态` in `docs/implementation/data-trade-implementation.md`.
- Rationale: the security model link is a navigation and truth-boundary pointer. Keeping it as a standalone subsection makes it visible without mixing it into the limitation bullets.
- Plan updated: Task 3 now includes the exact insertion location and required heading/text.

### 2026-06-26 CodeWhale Stage 4 Execution Complete

- Branch: `docs/data-trade-stage4-security-model`
- Tasks completed: Task 1, 2, 3, 4, 5 (all)
- Tests run:
  - `test -f docs/architecture/data-trade-security-model.md` ✅
  - `test -f docs/implementation/data-trade-implementation.md` ✅
  - `test -f docs/internal/agent-plans/2026-06-16-data-trade-four-stage-roadmap.md` ✅
- Tests not run:
  - Plan requires `cargo test` / `go test` / `node --check` — not applicable (docs-only change, no code modified)
  - Plan requires VM E2E — not applicable (docs-only change, no deployment or runtime changes)
- Deviations from plan:
  - Task 2 template updated per current progress: Stage 2.2 circuit-level business witness is complete, Stage 3 multi-subchain profiles are complete — template facts updated from "pending" to "complete"
- Questions for Codex/Owner: none remaining
- Remaining risks: none — this is a documentation-only stage; all security claims reflect current prototype limitations truthfully
