# Stage 9 Data Trade Dynamic Scripted E2E Implementation Plan

> **Execution owner:** CodeWhale.
>
> **Codex role:** plan author, architecture reviewer, code reviewer, and final merge owner after review fixes pass.

## Goal

Extend the existing real ZK data-trade E2E script so one command can run a full on-chain + off-chain data trade using Stage 8 dynamic dataset/request inputs.

Stage 8 added:

```text
dataset + request -> make-witness -> RangeWitness -> business-fixture -> verify
```

Stage 9 must compose that into the existing chain flow:

```text
DO publish listing
DR open escrow/session
make-witness
business-fixture
verify
submit proof digest + verifier attestation
delivery/signature/payment preimage
claim settlement
main escrow settlement
write evidence summary
```

This stage is still **range-only** and **script-only**. It must not implement frontend, subset/substr, on-chain verification, trustless bridge, or verifier quorum.

## Current Baseline

Existing script:

- `scripts/zk_real_data_trade_flow.js`
- Already runs a real gnark proof path using `fishbone-zk business-fixture`.
- Already submits proof metadata and Charlie verifier attestation on child chain.
- Already settles through `MainEscrow`.
- Currently uses:
  - `scripts/lib/data_trade_sample.js` for listing/request hashes.
  - `--business-witness` or profile `business_witness` for proof input.

New Stage 8 inputs:

- `fishbone-zk make-witness`
- `scripts/fixtures/data_trade_datasets/factory_sensors.json`
- `scripts/fixtures/data_trade_datasets/vehicle_telematics.json`
- `scripts/fixtures/data_trade_requests/factory_temperature_range.json`
- `scripts/fixtures/data_trade_requests/vehicle_speed_range.json`

## Non-Goals

Do **not** implement any of the following in Stage 9:

- Frontend UI.
- New ZK circuits.
- subset/substr constraints.
- Runtime, pallet, or chain storage changes.
- JS proof digest encoding changes.
- Artifact schema changes.
- Attestation payload changes.
- Trustless bridge / CCMC / Merkle proof settlement.
- Verifier quorum or verifier registry.
- VM clean redeploy automation.
- Production deployment hardening.
- External dependencies.

If any task appears to require one of these, stop and ask Codex/Owner.

## Design Decision

Extend `scripts/zk_real_data_trade_flow.js` with an optional dynamic mode.

Do not create a second full E2E script unless CodeWhale finds the existing file impossible to extend cleanly. The existing script already contains the correct chain/session/escrow/attestation/settlement behavior, so duplicating it would create two paths to maintain.

Dynamic mode is enabled when both are provided:

```bash
--dataset <dataset.json>
--request <request.json>
```

Legacy mode remains unchanged:

```bash
--business-witness <witness.json>
```

Mode selection:

1. Explicit `--dataset` + `--request` runs dynamic mode.
2. Explicit `--business-witness` with no dataset/request runs legacy mode.
3. Explicit `--business-witness` together with `--dataset` or `--request` must reject as ambiguous.
4. Profile/default `business_witness` is used only in legacy mode. It must not prevent dynamic mode when `--dataset` + `--request` are provided.
5. Profile defaults may provide dataset/request later, but Stage 9 does not need to change `scripts/profiles/chains.json` unless explicitly required by tests.
6. If neither dynamic inputs nor explicit `--business-witness` are provided, keep the existing default witness behavior.

## Dynamic Mode Contract

When `--dataset` and `--request` are provided:

1. Read request JSON before publishing data.
2. Use `request.request_hash` as the on-chain request hash for:
   - data registry listing
   - trade session creation
   - `business-fixture --request-hash`
3. Generate a per-round witness before `business-fixture`:

```bash
fishbone-zk make-witness \
  --dataset <dataset> \
  --request <request> \
  --out <outDir>/witness.json \
  --session-id <sessionId> \
  --round-index <round>
```

4. Run `business-fixture` using the generated witness:

```bash
fishbone-zk business-fixture \
  --witness <outDir>/witness.json \
  --out <outDir> \
  --request-hash <request_hash> \
  --session-id <sessionId> \
  --round-index <round>
```

5. Verify artifact as today.
6. Submit proof metadata and attestation as today.
7. Write an evidence summary file.

## Evidence Contract

Add a JSON evidence summary for each run, for example:

```text
target/data-trade-zk/session-<sessionId>-evidence.json
```

Minimum fields:

```json
{
  "version": 1,
  "mode": "dynamic",
  "profile": "child6-data-trade",
  "main_ws": "...",
  "child_ws": "...",
  "dataset_path": "...",
  "request_path": "...",
  "request_hash": "0x...",
  "listing_id": 0,
  "escrow_id": 0,
  "session_id": 0,
  "rounds": [
    {
      "round_index": 0,
      "witness_path": "...",
      "artifact_path": "...",
      "proof_digest": "0x...",
      "business_input_hash": "0x...",
      "public_input_hash": "0x..."
    }
  ],
  "settlement": {
    "completed_rounds": 2,
    "remaining_rounds": 1
  },
  "result": "accepted"
}
```

Legacy mode may also write evidence with `"mode": "legacy-witness"` if simple. Do not block Stage 9 on perfect legacy evidence; dynamic mode evidence is required.

## Implementation Tasks

### Task 1: Refactor E2E Script Input Handling

- [ ] Update `scripts/zk_real_data_trade_flow.js`.
- [ ] Add args:
  - `--dataset`
  - `--request`
  - `--evidence-out`
- [ ] Parse dynamic mode only when both `--dataset` and `--request` are present.
- [ ] Reject if only one of `--dataset` or `--request` is present.
- [ ] Reject if explicit `--business-witness` is combined with `--dataset` or `--request`.
- [ ] Do not let profile/default `business_witness` disable dynamic mode.
- [ ] Preserve existing `--business-witness` behavior.
- [ ] Read request JSON in dynamic mode and extract `request_hash`.
- [ ] Use dynamic `request_hash` instead of `sample.requestHash` for listing/session/proof generation.
- [ ] Keep `sample.imtRoot`, description, price, rounds, and proof params as current dev listing values unless a later task explicitly changes them.

Tests/checks:

```bash
node --check scripts/zk_real_data_trade_flow.js
```

Commit:

```bash
git add scripts/zk_real_data_trade_flow.js
git commit -m "feat(script): add dynamic inputs to real data trade e2e"
```

### Task 2: Add Dynamic Witness Generation To Each Round

- [ ] In dynamic mode, run `fishbone-zk make-witness` before `business-fixture`.
- [ ] Write witness to the round output directory:
  - `target/data-trade-zk/session-<sessionId>-round-<round>/witness.json`
- [ ] Use generated witness for `business-fixture`.
- [ ] Keep legacy mode using `BUSINESS_WITNESS` directly.
- [ ] Ensure spawned command failures include readable stderr/stdout context.
- [ ] Do not change verifier attestation digest logic.

Suggested helper:

```js
function runCli(args, label, opts = {}) { ... }
```

Only add a helper if it keeps the file simpler. Do not refactor unrelated transaction logic.

Tests/checks:

```bash
node --check scripts/zk_real_data_trade_flow.js
```

Commit:

```bash
git add scripts/zk_real_data_trade_flow.js
git commit -m "feat(script): generate dynamic witness during zk e2e"
```

### Task 3: Add Evidence Summary Output

- [ ] Accumulate run evidence in memory.
- [ ] Record:
  - mode
  - dataset/request paths
  - request hash
  - listing id
  - escrow id
  - session id
  - each round's witness/artifact/proof digest/business hash/public input hash
  - settlement completed/remaining rounds
  - result
- [ ] Write evidence JSON at:
  - `--evidence-out <path>` if provided
  - otherwise `target/data-trade-zk/session-<sessionId>-evidence.json`
- [ ] Ensure evidence directory is created if needed.
- [ ] If the script fails after session creation, best effort evidence with `"result": "failed"` is acceptable, but do not swallow the original error.

Tests/checks:

```bash
node --check scripts/zk_real_data_trade_flow.js
```

Commit:

```bash
git add scripts/zk_real_data_trade_flow.js
git commit -m "feat(script): write dynamic data trade e2e evidence"
```

### Task 4: Add Dry-Run / CLI Smoke For Dynamic Mode

Because a live chain may not be available in every agent session, add a non-chain smoke mode:

```bash
node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --dry-run-dynamic
```

Dry run must:

- validate args,
- read request JSON,
- run `make-witness`,
- run `business-fixture`,
- run `verify`,
- write evidence with `"result": "dry-run-accepted"`,
- not connect to chains,
- not submit transactions.

Dry-run metadata:

- Use synthetic `session_id = 0`.
- Use `round_index = 0`.
- Use one round only.
- Evidence should set `mode = "dynamic-dry-run"` and may omit `listing_id`, `escrow_id`, and live settlement fields or set them to `null`.

This is the required automated validation for Stage 9. Live chain E2E is useful but optional depending on environment availability.

Required checks:

```bash
rm -rf /tmp/fishbone-stage9-dry-run
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --evidence-out /tmp/fishbone-stage9-dry-run/evidence.json \
  --dry-run-dynamic
test -f /tmp/fishbone-stage9-dry-run/evidence.json
```

If `target/tools/fishbone-zk` does not exist, build it first:

```bash
go -C tools/data-trade-zk build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
```

Commit:

```bash
git add scripts/zk_real_data_trade_flow.js
git commit -m "test(script): add dynamic zk e2e dry run"
```

### Task 5: Documentation Updates

Update:

- `docs/implementation/data-trade-implementation.md`
- `docs/implementation/data-trade-paper-gap-matrix.md`
- `docs/implementation/data-trade-evidence.md`
- `docs/architecture/data-trade-security-model.md`
- `docs/internal/agent-plans/2026-06-26-data-trade-long-term-scripted-e2e-roadmap.md`
- This plan's Execution Record

Required wording:

- Stage 9 composes dynamic dataset/request witness generation into the real ZK E2E script.
- Stage 9 supports dry-run dynamic validation without chain RPC.
- Live chain E2E should be recorded only if actually run.
- Still range-only.
- Still off-chain verifier + on-chain attestation.
- No frontend, runtime, artifact schema, JS digest, or attestation encoding changes.

Suggested gap matrix updates:

- `Multi-round delivery`: mention dynamic ZK E2E script can now use dataset/request-driven witness in the proof path.
- `ZK proof artifact generation`: mention dynamic E2E evidence includes witness/artifact/proof digest/business hash/public input hash.
- Do not mark trustless bridge or on-chain verification as implemented.

Commit:

```bash
git add docs/implementation/data-trade-implementation.md docs/implementation/data-trade-paper-gap-matrix.md docs/implementation/data-trade-evidence.md docs/architecture/data-trade-security-model.md docs/internal/agent-plans/2026-06-26-data-trade-long-term-scripted-e2e-roadmap.md docs/internal/agent-plans/2026-06-27-stage9-data-trade-dynamic-scripted-e2e.md
git commit -m "docs: record dynamic data trade scripted e2e"
```

### Task 6: Final Validation

Required:

```bash
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js
go -C tools/data-trade-zk test ./...
```

Required dry-run:

```bash
go -C tools/data-trade-zk build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
rm -rf /tmp/fishbone-stage9-dry-run
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --evidence-out /tmp/fishbone-stage9-dry-run/evidence.json \
  --dry-run-dynamic
test -f /tmp/fishbone-stage9-dry-run/evidence.json
```

Optional live chain validation:

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --evidence-out /tmp/fishbone-stage9-live/evidence.json
```

Only record live chain validation as passed if it actually ran against reachable RPC endpoints. If RPC is unavailable, record it as not run due to environment.

## Stop Conditions

CodeWhale must stop and ask Codex before:

- Changing Rust/runtime/pallet code.
- Changing `artifact.ProofArtifact`.
- Changing JS proof digest calculation.
- Changing verifier attestation digest or payload encoding.
- Adding subset/substr or a new circuit.
- Replacing the existing E2E script with a full rewrite.
- Removing or breaking `--business-witness`.
- Claiming trustless bridge or on-chain verifier behavior.
- Running VM clean redeploy.
- Adding external dependencies.

## Acceptance Criteria

- Existing legacy command path still works:

```bash
node --check scripts/zk_real_data_trade_flow.js
```

- Dynamic dry-run command generates witness, artifact, verified proof, and evidence JSON.
- Dynamic mode uses `request.request_hash` for listing/session/proof request hash.
- Evidence JSON records session/proof fields in live mode and proof fields in dry-run mode.
- Existing `--business-witness` path remains available.
- No runtime/Rust/artifact schema/JS digest/attestation changes.
- Documentation clearly says Stage 9 is script orchestration, still range-only and not frontend.

## Execution Record

Not started.
