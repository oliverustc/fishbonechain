# Stage 11 Data Trade Failure/Dispute Scenario Implementation Plan

> **Execution owner:** CodeWhale.
>
> **Codex role:** plan author, architecture reviewer, code reviewer, and final merge owner after review fixes pass.

## Goal

Extend the Stage 9/10 real ZK data-trade script so it can demonstrate important failure and dispute paths, not only the happy path.

Current state:

- `scripts/data_trade_flow.js` already has dev-attested base scenarios:
  - `happy`
  - `invalid-proof`
  - `requester-refuses-payment`
- `scripts/zk_real_data_trade_flow.js` now supports dynamic dataset/request, single range, `multi_range`, real gnark artifacts, dry-run evidence, and full successful on-chain settlement.

Stage 11 should add scenario selection to `zk_real_data_trade_flow.js` and implement at least three failure/dispute paths using the real ZK/dynamic script surface where practical:

1. `invalid-proof-dispute`
2. `invalid-plaintext-dispute`
3. `requester-refuses-payment`

Optional if simple:

4. `verifier-rejection`

This stage is still **script-only**. It must not add new pallets, runtime logic, new circuits, verifier quorum, or trustless bridge behavior.

## Non-Goals

Do **not** implement any of the following in Stage 11:

- runtime/pallet/storage changes;
- new dispute pallet behavior;
- new ZK circuits;
- artifact schema changes;
- JS proof digest encoding changes;
- verifier attestation digest/payload changes;
- on-chain Groth16 verification;
- verifier quorum/slashing;
- trustless bridge or CCMC/Merkle proof settlement;
- frontend UI;
- production timeout/challenge-period logic;
- VM clean redeploy automation.

If any task appears to require one of these, stop and ask Codex/Owner.

## Design Decision

Extend `scripts/zk_real_data_trade_flow.js` rather than creating a separate failure script.

Reasons:

- Stage 9/10 dynamic request handling and evidence output already live there.
- Failure scenarios should share the same setup, dynamic witness generation, real proof artifact generation, verifier attestation digest logic, and evidence format.
- A separate script would duplicate the chain setup and proof path.

Add a new argument:

```bash
--scenario <name>
```

Supported values:

```text
happy
invalid-proof-dispute
invalid-plaintext-dispute
requester-refuses-payment
verifier-rejection       # optional if simple
```

Default:

- `happy`

Backward compatibility:

- Existing commands without `--scenario` must keep their current behavior.
- Existing `--dry-run-dynamic` should remain a proof-pipeline dry-run and should not require chain scenarios.
- Existing legacy `--business-witness` mode must keep working for `happy`.

## Scenario Semantics

### Scenario: `happy`

Keep current successful behavior:

```text
setup -> two completed rounds -> claimSettlement -> settleByPreimage
```

Evidence result:

```json
"scenario": "happy",
"result": "accepted"
```

### Scenario: `invalid-proof-dispute`

Goal:

Show DR can dispute an invalid proof and then punish DO deposit on main escrow.

Recommended path:

```text
setup
open round 0
submit payment proof
generate a valid real artifact off-chain for evidence
submit intentionally wrong proof metadata or wrong proof digest on-chain
DR calls tradeSession.disputeInvalidProof(sessionId, 0, badDigest)
DR calls mainEscrow.punishDataOwner(escrowId)
record SessionPunished + EscrowPunished evidence
stop
```

Implementation guidance:

- It is acceptable to generate a valid artifact first, then derive a bad digest by flipping one byte/hex nibble for the on-chain `submitDataProof` call.
- Do **not** change artifact schema or verifier code.
- Do **not** call verifier attestation for this scenario unless the current pallet flow requires it. Existing `dispute_invalid_proof` only requires session `InDelivery`, so the scenario can dispute after `submitDataProof`.
- Evidence must record both:
  - `valid_artifact.proof_digest`
  - `submitted_bad_proof_digest`

Expected terminal state:

- child chain emits `tradeSession.SessionPunished`;
- main chain emits `mainEscrow.EscrowPunished`;
- evidence result should be:

```json
"result": "expected-dispute-accepted"
```

### Scenario: `invalid-plaintext-dispute`

Goal:

Show DR can dispute delivered plaintext/hash mismatch after proof signature and delivery.

Recommended path:

```text
setup
open round 0
submit payment proof
generate and verify real artifact
submit data proof
verifier attests accepted
DR submits proof signature
DO submits data delivery hash
DR calls tradeSession.disputeInvalidPlaintext(sessionId, 0, dataHash, expectedHash) with mismatched hashes
DR calls mainEscrow.punishDataOwner(escrowId)
record SessionPunished + EscrowPunished evidence
stop
```

Implementation details:

- Use two deterministic but different 32-byte hashes. Example:

```js
const deliveredHash = hashNTimes("delivered-bad", 1);
const expectedHash = hashNTimes("expected-good", 1);
```

- `disputeInvalidPlaintext` requires the hashes to mismatch. Do not pass equal hashes.
- The current `submitDataDeliveryHash` call uses a hash-like value. For this scenario, submit `deliveredHash`, then dispute with `deliveredHash` vs `expectedHash`.

Expected terminal state:

- child chain emits `tradeSession.SessionPunished`;
- main chain emits `mainEscrow.EscrowPunished`;
- evidence result should be:

```json
"result": "expected-plaintext-dispute-accepted"
```

### Scenario: `requester-refuses-payment`

Goal:

Show DO can claim last payment when DR refuses to submit payment preimage after data delivery.

Recommended path:

```text
setup
open round 0
submit payment proof
generate and verify real artifact
submit data proof
verifier attests accepted
DR submits proof signature
DO submits data delivery hash
DR intentionally does NOT submit payment preimage
DO calls tradeSession.claimLastPayment(sessionId, 0)
DO calls mainEscrow.claimLastPayment(escrowId, 0)
record LastPaymentClaimed + main escrow final state evidence
stop
```

Expected terminal state:

- child chain emits `tradeSession.LastPaymentClaimed`;
- main escrow status becomes settled through `claimLastPayment`;
- evidence result should be:

```json
"result": "expected-last-payment-claimed"
```

Note:

- This is a scripted immediate claim, not a production timeout/challenge-period implementation.
- Documentation must state that production timing rules are future work.

### Optional Scenario: `verifier-rejection`

Goal:

Show a rejected verifier attestation blocks DR proof signature.

Suggested path:

```text
setup
open round 0
submit payment proof
generate and verify real artifact
submit data proof
Charlie attests accepted=false with correct attestation digest
attempt DR submitProofSignature and expect dispatch error RoundStepsOutOfOrder
record DataProofAttested(accepted=false) and expected rejected extrinsic
stop
```

Rules:

- If this is not simple with the current `submitTx()` helper, skip it in Stage 11. Do not overcomplicate the script.
- If implemented, add a helper for expected dispatch failure rather than weakening `submitTx()`.

Evidence result:

```json
"result": "expected-verifier-rejection-blocked-signature"
```

## Implementation Tasks

### Task 1: Add Scenario Argument And Validation

File:

- `scripts/zk_real_data_trade_flow.js`

Add:

```js
const SCENARIO = parseArg("--scenario") || "happy";
```

Allowed scenario set:

```js
const ALLOWED_SCENARIOS = new Set([
  "happy",
  "invalid-proof-dispute",
  "invalid-plaintext-dispute",
  "requester-refuses-payment",
  "verifier-rejection", // only include if implemented
]);
```

Rules:

- Unknown scenario exits with code `2`.
- `--dry-run-dynamic` ignores chain scenario selection or rejects non-`happy` scenario with a clear message. Prefer rejecting non-`happy` scenario in dry-run:

```text
--dry-run-dynamic only supports proof-pipeline validation; omit --scenario or use --scenario happy
```

- Add `"scenario": SCENARIO` to evidence.

Commit:

```bash
git add scripts/zk_real_data_trade_flow.js
git commit -m "feat(script): add zk data trade scenario selection"
```

### Task 2: Refactor Shared Setup And Round Helpers

File:

- `scripts/zk_real_data_trade_flow.js`

Refactor carefully. Do not rewrite the whole script.

Suggested helpers:

```js
async function setupTrade({ mainApi, childApi, alice, bob, sample, requestHash, maxRounds, pricePerRound, depositHint, hashChainAnchor }) {
  // publishData, openEscrow, lockFunds, lockDeposit, assert escrow,
  // createSession, assert session, acceptSession
  // return { listingId, escrowId, sessionId }
}

function makeRoundEvidence(round, multiRange) { ... }

function generateAndVerifyRoundArtifacts({ outDir, sessionId, round, requestHash }) {
  // uses Stage 10 dynamic/legacy witness bundle
  // runs business-fixture + verify
  // returns { roundEvidence, chainArt }
}

async function submitRoundProofAccepted({ childApi, alice, bob, charlie, sessionId, round, requestHash, chainArt, paymentCommitment }) {
  // openRound, submitPaymentProof, submitDataProof, attest accepted=true
}

async function completeDeliveryAndPayment({ childApi, alice, bob, sessionId, round, paymentCommitment }) {
  // submitProofSignature, submitDataDeliveryHash, submitPaymentPreimage
}
```

Important:

- Keep behavior of `happy` unchanged.
- Keep Stage 10 dynamic `rounds[].constraints[]` evidence shape unchanged.
- For legacy non-dynamic witness mode, preserve existing behavior enough for `happy`.
- Avoid broad style-only refactors.

Commit:

```bash
git add scripts/zk_real_data_trade_flow.js
git commit -m "refactor(script): share zk data trade round helpers"
```

### Task 3: Implement Failure/Dispute Scenarios

File:

- `scripts/zk_real_data_trade_flow.js`

Add scenario functions:

```js
async function runHappyScenario(ctx) { ... }
async function runInvalidProofDisputeScenario(ctx) { ... }
async function runInvalidPlaintextDisputeScenario(ctx) { ... }
async function runRequesterRefusesPaymentScenario(ctx) { ... }
```

Optional:

```js
async function runVerifierRejectionScenario(ctx) { ... }
```

Evidence requirements:

Top-level evidence should include:

```json
{
  "scenario": "invalid-proof-dispute",
  "expected_failure": true,
  "result": "expected-dispute-accepted",
  "listing_id": 0,
  "escrow_id": 0,
  "session_id": 0,
  "rounds": [ ... ],
  "dispute": {
    "type": "invalid-proof",
    "child_event": "tradeSession.SessionPunished",
    "main_event": "mainEscrow.EscrowPunished"
  }
}
```

For `requester-refuses-payment`, use:

```json
"dispute": {
  "type": "requester-refuses-payment",
  "child_event": "tradeSession.LastPaymentClaimed",
  "main_event": "mainEscrow.EscrowSettled"
}
```

The exact field may be named `scenario_outcome` instead of `dispute` if that reads better, but keep it structured and documented.

Required event checks:

- Use `findEvent()` to assert expected child/main events after the relevant extrinsic.
- Record the expected event names in evidence.
- Do not rely only on log output.

Expected dispatch failure helper for optional verifier rejection:

```js
async function expectTxFailure(signer, tx, label, expectedErrorName) { ... }
```

Only add this helper if implementing `verifier-rejection`.

Commit:

```bash
git add scripts/zk_real_data_trade_flow.js
git commit -m "feat(script): add zk data trade dispute scenarios"
```

### Task 4: Add Chainless Scenario Smoke Mode If Needed

Stage 11 is mostly chain-state behavior, so full validation needs reachable main/child RPC. However, CodeWhale may not have live RPC available.

Add one of the following, whichever is smaller and cleaner:

Option A: no new dry-run mode, but document that chain scenario validation is not run when RPC is unavailable.

Option B: add `--dry-run-scenario-plan` that only validates argument parsing and prints the planned scenario steps as JSON.

Preferred: **Option A**. Do not add artificial dry-run behavior unless implementation/testing becomes too hard.

Minimum local checks without RPC:

```bash
node --check scripts/zk_real_data_trade_flow.js
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --dry-run-dynamic
```

Commit only if docs or script changed for this task.

### Task 5: Documentation Updates

Update:

- `docs/implementation/data-trade-implementation.md`
- `docs/implementation/data-trade-evidence.md`
- `docs/implementation/data-trade-paper-gap-matrix.md`
- `docs/architecture/data-trade-security-model.md`
- `docs/internal/agent-plans/2026-06-26-data-trade-long-term-scripted-e2e-roadmap.md`
- this plan's Execution Record

Required wording:

- Stage 11 scripts key failure/dispute paths for the ZK/dynamic data-trade flow.
- Scenarios are script-level demonstrations over existing pallet behavior.
- No new dispute mechanism, no production timeout, no on-chain ZK verification, no trustless bridge, no verifier quorum.
- If live chain scenario validation was not run, say so explicitly. Do not claim live evidence unless commands actually completed.
- Evidence files should include scenario name, expected result, expected events, IDs, and final result.

Commit:

```bash
git add docs/implementation/data-trade-implementation.md docs/implementation/data-trade-evidence.md docs/implementation/data-trade-paper-gap-matrix.md docs/architecture/data-trade-security-model.md docs/internal/agent-plans/2026-06-26-data-trade-long-term-scripted-e2e-roadmap.md docs/internal/agent-plans/2026-06-27-stage11-data-trade-failure-dispute-scenarios.md
git commit -m "docs: record data trade failure scenario scripts"
```

### Task 6: Final Validation

Required local checks:

```bash
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js
go -C tools/data-trade-zk test ./...
go -C tools/data-trade-zk build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
```

Required dynamic dry-run regression:

```bash
rm -rf target/data-trade-zk/session-0-round-0 /tmp/fishbone-stage11-dry-run
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --evidence-out /tmp/fishbone-stage11-dry-run/evidence.json \
  --dry-run-dynamic
test -f /tmp/fishbone-stage11-dry-run/evidence.json
```

Optional live chain validation, only if RPC endpoints are reachable:

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --scenario invalid-proof-dispute \
  --evidence-out /tmp/fishbone-stage11-invalid-proof/evidence.json

ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --scenario invalid-plaintext-dispute \
  --evidence-out /tmp/fishbone-stage11-invalid-plaintext/evidence.json

ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --scenario requester-refuses-payment \
  --evidence-out /tmp/fishbone-stage11-refuses-payment/evidence.json
```

Only record these as passed if they actually complete. If the environment lacks live RPC, record them as not run due to unavailable RPC.

## Stop Conditions

CodeWhale must stop and ask Codex before:

- Changing Rust/runtime/pallet code.
- Changing `artifact.ProofArtifact`.
- Changing JS proof digest calculation.
- Changing verifier attestation digest or payload encoding.
- Adding new ZK circuits or verifier behavior.
- Adding timeout/challenge-period semantics.
- Changing escrow economics or balances.
- Rewriting `zk_real_data_trade_flow.js` from scratch.
- Removing or breaking `happy`, `--dry-run-dynamic`, dynamic `range`, dynamic `multi_range`, or legacy `--business-witness` paths.
- Running VM clean redeploy.
- Claiming live chain validation without actually running it.
- Adding external dependencies.

## Acceptance Criteria

- `--scenario happy` remains equivalent to current behavior.
- `--scenario invalid-proof-dispute` is implemented and records expected `SessionPunished` + `EscrowPunished` evidence.
- `--scenario invalid-plaintext-dispute` is implemented and records expected `SessionPunished` + `EscrowPunished` evidence.
- `--scenario requester-refuses-payment` is implemented and records expected `LastPaymentClaimed` + main escrow claim evidence.
- At least three failure/dispute scenarios are available via `--scenario`.
- Evidence includes scenario name, expected result, relevant IDs, expected events, and final result.
- Dynamic dry-run proof validation still works.
- No runtime, artifact schema, digest encoding, attestation encoding, new circuit, trustless bridge, or verifier quorum changes.
- Documentation states Stage 11 is scripted demonstration of existing behavior, not new production dispute machinery.

## Plan Review Checklist For CodeWhale

Before implementation, review this plan and specifically check:

- Whether `zk_real_data_trade_flow.js` can be refactored safely without a large rewrite.
- Whether `invalid-proof-dispute` should dispute before or after verifier attestation. The plan recommends before attestation because current pallet allows it.
- Whether `invalid-plaintext-dispute` should reuse the real artifact proof path or the existing hash values. The plan recommends real artifact path plus mismatched delivery/expected hashes.
- Whether `verifier-rejection` should be included or deferred. It is optional and should be skipped if it complicates the helper flow.
- Whether evidence shape needs a stable `scenario_outcome` object name instead of `dispute`.

## Execution Record

Not started.
