# Data Trade CLI / API Boundary

**Status**: Stage 16 CLI/API boundary definition for future backend integration. This is NOT a backend/server implementation. No database schema, REST/RPC server, authentication, or UI is implied.

**Date**: 2026-06-30
**Stage**: Stage 16
**Plan**: `docs/internal/agent-plans/2026-06-30-stage16-data-trade-cli-api-boundary.md`

## 1. Overview

This document defines a stable command boundary for data-trade operations. The 10 commands below form the contract between future backend services, user-facing CLIs, and the existing chain/off-chain execution layer.

### Boundary Principles

1. **User-signed chain actions only.** Every chain-state-changing command requires user/dev signer keys. The CLI boundary does not make a backend trusted.
2. **Backend is orchestration, not trust replacement.** Commands produce evidence and return structured outputs for backend consumption. Database state is never protocol finality.
3. **Off-chain operations are explicitly labeled.** Proof generation, inspection, and request creation are off-chain operations that may be backend-scheduled but are not chain finality.
4. **Evidence-first.** Every command that produces results includes an `--evidence-out` path for structured evidence output.
5. **Stage 14 compatibility preserved.** The existing `scripts/run_data_trade_validation.sh` and `scripts/zk_real_data_trade_flow.js` must remain fully functional.

## 2. Command Reference

### 2.1 `publish-listing`

| Field | Value |
|-------|-------|
| **Actor** | DO (Data Owner, Bob in dev E2E) |
| **Classification** | Child-chain transaction |
| **Implementation Status** | `planned` — not independently executable in Stage 16 |
| **Signer Requirement** | DO's chain key (dev: `//Bob`) |

**Inputs**:
- `imt_root` (hex): IMT root hash
- `description` (string): Listing description
- `price_per_round` (uint): Price per round
- `max_rounds` (uint): Maximum trade rounds
- `deposit_hint` (uint): Suggested deposit amount
- `request_schema_hash` (hex): Hash of accepted request schema
- `proof_params_hash` (hex): Hash of proof parameters
- `--profile` (string): Trade profile ID

**Chain extrinsic**: `dataRegistry.publishData`

**Output**: `listing_id` (uint), chain events

**Stage 16 availability**: Published as a boundary definition with help output. Independent execution deferred to a later stage. Full-flow access via `run-flow`.

---

### 2.2 `create-request`

| Field | Value |
|-------|-------|
| **Actor** | DR (Data Requester) / Backend orchestration |
| **Classification** | Off-chain |
| **Implementation Status** | `planned` — represented by request JSON files |
| **Signer Requirement** | None (off-chain) |

**Inputs**:
- `--dataset` (path): Dataset JSON fixture
- `--request` (path): Request JSON fixture containing `request_hash`, `field_name`, `min`/`max` ranges

**Output**: Request hash, validated dataset/request binding

**Stage 16 availability**: Current flow uses pre-authored request JSON files under `scripts/fixtures/data_trade_requests/`. This command boundary is documented for future backend request creation. Full-flow access via `run-flow`.

---

### 2.3 `create-escrow`

| Field | Value |
|-------|-------|
| **Actor** | DR (Alice in dev E2E) and DO (Bob) |
| **Classification** | Main-chain transactions |
| **Implementation Status** | `planned` — not independently executable in Stage 16 |
| **Signer Requirement** | DR's chain key (dev: `//Alice`) and DO's chain key (dev: `//Bob`) |

**Inputs**:
- `max_rounds` (uint): Maximum trade rounds
- `price_per_round` (uint): Price per round
- `deposit_hint` (uint): Suggested deposit
- `hash_chain_anchor` (hex): Hash chain anchor
- `--profile` (string): Trade profile ID
- `--main` (wss URL): Main chain RPC

**Chain extrinsics**:
1. `mainEscrow.openEscrow(do_address, max_rounds, price_per_round, deposit_hint, hash_chain_anchor)`
2. `mainEscrow.lockFunds(escrow_id)` (DR)
3. `mainEscrow.lockDeposit(escrow_id)` (DO)

**Binding checks required**: `assertEscrowMatchesTradeTerms` for escrow ↔ trade terms consistency.

**Output**: `escrow_id` (uint), chain events

**Stage 16 availability**: Boundary definition only. Full-flow escrow creation via `run-flow`.

---

### 2.4 `open-session`

| Field | Value |
|-------|-------|
| **Actor** | DR (Alice) and DO (Bob) |
| **Classification** | Child-chain transactions |
| **Implementation Status** | `planned` — not independently executable in Stage 16 |
| **Signer Requirement** | DR's chain key (dev: `//Alice`) and DO's chain key (dev: `//Bob`) |

**Inputs**:
- `listing_id` (uint): Published listing ID
- `escrow_id` (uint): Opened escrow ID
- `request_hash` (hex): Request schema hash
- `--profile` (string): Trade profile ID
- `--child` (wss URL): Child chain RPC

**Chain extrinsics**:
1. `tradeSession.createSession(listing_id, escrow_id, do_address, request_hash, price, max_rounds, hash_chain_anchor, "MainEscrow")`
2. `tradeSession.acceptSession(session_id)` (DO)

**Binding checks required**: `assertSessionMatchesListingAndEscrow` for session ↔ listing ↔ escrow consistency.

**Output**: `session_id` (uint), chain events

**Stage 16 availability**: Boundary definition only. Full-flow session creation via `run-flow`.

---

### 2.5 `generate-proof`

| Field | Value |
|-------|-------|
| **Actor** | DO / Backend off-chain worker |
| **Classification** | Off-chain ZK pipeline |
| **Implementation Status** | `implemented` — no-live-chain, delegates to existing ZK pipeline |
| **Signer Requirement** | None (off-chain) |

**Inputs**:
- `--profile` (string): Trade profile ID (provides ZK_CMD path, proof params)
- `--dataset` (path): Dataset JSON (e.g., `scripts/fixtures/data_trade_datasets/factory_sensors.json`)
- `--request` (path): Request JSON (e.g., `scripts/fixtures/data_trade_requests/factory_temperature_range.json`)
- `--evidence-out` (path): Evidence output JSON path

**Pipeline**:
1. `fishbone-zk make-witness --dataset <path> --request <path> --out <outDir>/witness.json`
2. `fishbone-zk business-fixture --witness <witness> --out <outDir> --request-hash <hash> --session-id 0 --round-index 0`
3. `fishbone-zk verify --artifact <outDir>/artifact.json`

**Output**:
- Evidence JSON with `proof_digest`, `business_input_hash`, `public_input_hash`, `constraints[]`, `mode: "dynamic-dry-run"`, `result: "dry-run-accepted"`
- Written to `--evidence-out` path

**Environment**: Requires `ZK_VERIFIER_CMD` or profile's `zk_verifier_cmd` to point to a working `fishbone-zk` binary.

**Stage 16 availability**: Fully implemented as no-live-chain command.

---

### 2.6 `submit-delivery`

| Field | Value |
|-------|-------|
| **Actor** | DR and DO and Verifier |
| **Classification** | Mixed child-chain round operations |
| **Implementation Status** | `planned` — not independently executable in Stage 16 |
| **Signer Requirement** | DR, DO, and Verifier chain keys |

**Operations per round**:
1. DR opens round (`tradeSession.openRound`)
2. DR submits payment proof (`tradeSession.submitPaymentProof`)
3. DO submits data proof (`tradeSession.submitDataProof`) — 10 params including proof digest
4. Verifier attests (`tradeSession.attestDataProof`)
5. DR signs (`tradeSession.submitProofSignature`)
6. DO delivers data hash (`tradeSession.submitDataDeliveryHash`)
7. DR submits payment preimage (`tradeSession.submitPaymentPreimage`)

**Binding checks required**: `computeZkAttestationDigest` for attestation payload validation.

**Stage 16 availability**: Boundary definition only. Full-flow round delivery via `run-flow`.

---

### 2.7 `settle`

| Field | Value |
|-------|-------|
| **Actor** | DO (Bob) |
| **Classification** | Child-chain + main-chain settlement transactions |
| **Implementation Status** | `planned` — not independently executable in Stage 16 |
| **Signer Requirement** | DO's chain key (dev: `//Bob`) |

**Paths**:
- **Happy path**: `tradeSession.claimSettlement(session_id, preimage, remaining_rounds)` → `mainEscrow.settleByPreimage(escrow_id, preimage, remaining_rounds)`
- **DR refuses payment**: `tradeSession.claimLastPayment(session_id, round)` → `mainEscrow.claimLastPayment(escrow_id, round)`

**Output**: Settlement events, DO receives payment, DR receives refund, DO deposit released.

**Stage 16 availability**: Boundary definition only. Full-flow settlement via `run-flow`.

---

### 2.8 `dispute`

| Field | Value |
|-------|-------|
| **Actor** | DR (Alice) |
| **Classification** | Child-chain dispute + main-chain punishment |
| **Implementation Status** | `planned` — not independently executable in Stage 16 |
| **Signer Requirement** | DR's chain key (dev: `//Alice`) |

**Paths**:
- **Invalid proof**: `tradeSession.disputeInvalidProof(session_id, round, bad_digest)` → `mainEscrow.punishDataOwner(escrow_id)`
- **Invalid plaintext**: `tradeSession.disputeInvalidPlaintext(session_id, round, delivered_hash, expected_hash)` → `mainEscrow.punishDataOwner(escrow_id)`

**Output**: Session punished events, escrow punished events.

**Stage 16 availability**: Boundary definition only. Full-flow dispute via `run-flow`.

---

### 2.9 `inspect`

| Field | Value |
|-------|-------|
| **Actor** | Any (query-only) |
| **Classification** | Off-chain inspection |
| **Implementation Status** | `implemented` — no-live-chain safe |
| **Signer Requirement** | None (query-only, no chain connection) |

**Subcommands**:

#### `inspect profile`
- `--profile` (string): Trade profile ID
- `--out` (path): Output JSON path
- Reads profile from `scripts/profiles/chains.json` and writes structured profile data.

#### `inspect evidence`
- `--evidence` (path): Evidence JSON file path
- `--out` (path): Output JSON path
- Reads evidence JSON, validates structure, writes summary with `proof_digest`, `business_input_hash`, `constraints[]`, `result`.

**Output**: Structured JSON summary at `--out` path.

**Stage 16 availability**: Fully implemented as no-live-chain command.

---

### 2.10 `run-flow`

| Field | Value |
|-------|-------|
| **Actor** | Full flow (all roles) |
| **Classification** | Compatibility wrapper for existing E2E flow |
| **Implementation Status** | `implemented` — delegates to `scripts/zk_real_data_trade_flow.js` |
| **Signer Requirement** | As per `zk_real_data_trade_flow.js` (requires dev keys for live-chain) |

**Inputs (preserved flag surface)**:
- `--profile` (string): Trade profile ID
- `--main` (wss URL): Main chain RPC (optional, defaults to profile)
- `--child` (wss URL): Child chain RPC (optional, defaults to profile)
- `--business-witness` (path): Witness JSON to use
- `--dataset` (path): Dataset JSON for dynamic mode
- `--request` (path): Request JSON for dynamic mode
- `--scenario` (string): `happy`, `invalid-proof-dispute`, `invalid-plaintext-dispute`, `requester-refuses-payment`
- `--evidence-out` (path): Evidence output JSON
- `--dry-run-dynamic` (flag): Run proof pipeline without chain connection
- `--verbose` (flag): Verbose output

**Environment**: `ZK_VERIFIER_CMD` for ZK CLI path.

**Stage 16 availability**: Fully implemented as compatibility wrapper.

## 3. Role Summary

| Role | Dev Key | Responsibilities |
|------|---------|-----------------|
| DO (Data Owner) | `//Bob` | Publish listing, accept session, submit data proof, deliver data hash, claim settlement/last payment |
| DR (Data Requester) | `//Alice` | Create escrow/session, lock funds, open rounds, submit payment proof, sign, submit payment preimage, dispute |
| Verifier | `//Charlie` | Submit proof attestation |

## 4. Chain / Off-Chain Classification

```
publish-listing    → child-chain transaction
create-request     → off-chain
create-escrow      → main-chain transactions
open-session       → child-chain transactions
generate-proof     → off-chain (ZK pipeline)
submit-delivery    → child-chain transactions (mixed)
settle             → child-chain + main-chain transactions
dispute            → child-chain + main-chain transactions
inspect            → off-chain (query only)
run-flow           → all (full flow)
```

## 5. Evidence Fields

All commands that produce evidence write JSON with the following fields:

| Field | Type | Description |
|-------|------|-------------|
| `version` | `number` | Evidence format version (`1`) |
| `scenario` | `string` | Business scenario name |
| `mode` | `string` | `"dynamic-dry-run"` for no-chain, `"dynamic"` for live-chain |
| `profile` | `string` | Trade profile ID |
| `main_ws` | `string` | Main chain RPC URL |
| `child_ws` | `string` | Child chain RPC URL |
| `dataset_path` | `string` | Dataset fixture path |
| `request_path` | `string` | Request fixture path |
| `request_hash` | `string` | Request hash |
| `listing_id` | `number` | Chain listing ID (null for dry-run) |
| `escrow_id` | `number` | Chain escrow ID (null for dry-run) |
| `session_id` | `number` | Chain session ID |
| `rounds` | `array` | Per-round evidence with `constraints[]` |
| `settlement` | `object` | Settlement outcome (null for non-happy paths) |
| `result` | `string` | Evidence result (`"dry-run-accepted"`, `"accepted"`, etc.) |
| `scenario_outcome` | `object` | Dispute/exception result (null for happy path) |

Per-constraint binding:

| Field | Description |
|-------|-------------|
| `index` | Constraint index within round |
| `field_name` | Data field name |
| `witness_path` | Path to witness file |
| `artifact_path` | Path to proof artifact |
| `proof_digest` | ZK proof digest (hex) |
| `business_input_hash` | Business witness input hash (hex) |
| `public_input_hash` | Public input hash (hex) |
| `on_chain_bound` | Whether digest was bound on-chain |

## 6. Current Security Boundaries (unchanged)

- Off-chain gnark proof verification (Groth16 BN254) plus on-chain digest/attestation binding
- Single dev verifier authority: Charlie (`//Charlie`)
- Off-chain bridge/session-escrow coordination
- `MainEscrow` as the only implemented settlement mode
- `AlwaysPassVerifier` for on-chain verifier (no on-chain Groth16 verification)

## 7. References

- [Platform Business Model](../architecture/platform-business-model.md)
- [Data Trade Implementation](data-trade-implementation.md)
- [Data Trade Validation Experiment](../experiments/data-trade-validation.md)
- [Stage 14 Evidence Index](data-trade-stage14-evidence-index.md)
- [Stage 16 Plan](../internal/agent-plans/2026-06-30-stage16-data-trade-cli-api-boundary.md)
