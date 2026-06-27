# Stage 12 Data Trade Paper Experiment Freeze Plan

> **Execution owner:** CodeWhale.
>
> **Codex role:** plan author, plan-review resolver, code reviewer, and final merge owner after review fixes pass.

## Goal

Turn the Stage 8-11 data-trade prototype into paper-facing, reproducible experiment material.

Stage 12 is a **freeze/documentation/evidence packaging stage**, not a new capability stage. The output should let a new agent or human start from the docs, run the supported commands, understand which evidence is dry-run vs live-chain, and accurately describe the implementation in the paper without overstating trust assumptions.

Current baseline after Stage 11:

- Dynamic dataset/request model exists.
- `range` and `multi_range` requests are supported by the off-chain witness/proof pipeline.
- `zk_real_data_trade_flow.js` supports:
  - dynamic dataset/request happy path;
  - `--dry-run-dynamic`;
  - evidence output;
  - Stage 11 scenarios:
    - `invalid-proof-dispute`
    - `invalid-plaintext-dispute`
    - `requester-refuses-payment`
- Stage 11 code review is approved and merged into `main`.
- Live-chain Stage 11 scenario evidence has **not** been produced yet because RPC was unavailable during review.

## Non-Goals

Do **not** implement any of the following in Stage 12:

- new pallets, runtime logic, or storage migrations;
- new ZK circuits;
- subset/substr constraints;
- artifact schema changes;
- proof digest or attestation digest encoding changes;
- verifier quorum/slashing;
- on-chain Groth16 verification;
- trustless bridge / CCMC / Merkle proof settlement;
- frontend UI;
- production timeout/challenge-period logic;
- destructive VM redeploy or clean-chain reset unless the owner explicitly approves it for this stage.

If a task appears to require one of these, stop and ask Codex/Owner.

## Deliverables

Stage 12 should produce three concrete deliverables:

1. A paper demo guide with copy-pasteable commands.
2. A stable evidence index that distinguishes dry-run evidence, local generated artifact evidence, and live-chain evidence.
3. Updated implementation/gap/security docs that make the final Stage 12 boundary clear.

Recommended new files:

```text
docs/implementation/data-trade-demo-guide.md
docs/implementation/data-trade-stage12-evidence-index.md
```

Existing files that likely need updates:

```text
docs/implementation/data-trade-evidence.md
docs/implementation/data-trade-implementation.md
docs/implementation/data-trade-paper-gap-matrix.md
docs/architecture/data-trade-security-model.md
docs/internal/agent-plans/2026-06-26-data-trade-long-term-scripted-e2e-roadmap.md
```

Only update `README.md` if there is already a suitable data-trade/demo section. If adding a README link would create churn or duplicate detailed docs, skip it and document the reason in the Stage 12 execution notes.

## Required Demo Matrix

Document and, where safe in the current environment, validate the following command categories.

### 1. Dynamic proof-pipeline dry-runs

These do not require live chain RPC and should be runnable in normal development environments.

Happy single range:

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --evidence-out target/data-trade-stage12/factory-temperature-dry-run/evidence.json \
  --dry-run-dynamic
```

Happy multi-range:

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --evidence-out target/data-trade-stage12/factory-multi-range-dry-run/evidence.json \
  --dry-run-dynamic
```

Different dataset/request:

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/vehicle_telematics.json \
  --request scripts/fixtures/data_trade_requests/vehicle_speed_range.json \
  --evidence-out target/data-trade-stage12/vehicle-speed-dry-run/evidence.json \
  --dry-run-dynamic
```

Expected result:

- command exits 0;
- evidence `mode` is `dynamic-dry-run`;
- `listing_id`, `escrow_id`, `session_id`, and `settlement` are `null`;
- evidence records request/dataset metadata and generated constraints;
- no live-chain claims are made from dry-run output.

### 2. Negative dynamic validation commands

These should reject before chain interaction.

Out-of-range single range:

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_out_of_range.json \
  --dry-run-dynamic
```

Out-of-range multi-range:

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range_out_of_range.json \
  --dry-run-dynamic
```

Expected result:

- command exits non-zero;
- failure happens before chain interaction;
- docs should record that this validates request/dataset consistency and witness construction boundaries, not on-chain dispute behavior.

### 3. Live-chain happy path command

Document but only run if the configured RPC endpoints are reachable and the owner has not forbidden live execution.

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --evidence-out target/data-trade-stage12/live-happy-multi-range/evidence.json
```

Expected result if run:

- two completed rounds;
- real gnark artifacts generated and verified off-chain;
- chain records proof digest + verifier attestation;
- MainEscrow settlement completes;
- evidence `result` is `"accepted"`.

### 4. Live-chain failure/dispute scenario commands

Document but only run if RPC endpoints are reachable and live execution is appropriate.

Invalid proof dispute:

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --scenario invalid-proof-dispute \
  --evidence-out target/data-trade-stage12/live-invalid-proof/evidence.json
```

Invalid plaintext dispute:

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --scenario invalid-plaintext-dispute \
  --evidence-out target/data-trade-stage12/live-invalid-plaintext/evidence.json
```

Requester refuses payment:

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --scenario requester-refuses-payment \
  --evidence-out target/data-trade-stage12/live-requester-refuses-payment/evidence.json
```

Expected result if run:

| Scenario | Expected `result` | Expected asserted events |
|----------|-------------------|--------------------------|
| `invalid-proof-dispute` | `"expected-dispute-accepted"` | `tradeSession.SessionPunished`, `mainEscrow.EscrowPunished` |
| `invalid-plaintext-dispute` | `"expected-plaintext-dispute-accepted"` | `tradeSession.SessionPunished`, `mainEscrow.EscrowPunished` |
| `requester-refuses-payment` | `"expected-last-payment-claimed"` | `tradeSession.LastPaymentClaimed`, `mainEscrow.EscrowSettled` |

## Implementation Tasks

### Task 1: Create the demo guide

Create `docs/implementation/data-trade-demo-guide.md`.

It should include:

- prerequisite build commands:

```bash
go -C tools/data-trade-zk build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
node --check scripts/zk_real_data_trade_flow.js
```

- the required demo matrix commands above;
- a short explanation of what each command proves;
- a clear separation between:
  - no-chain dry-run commands;
  - negative validation commands;
  - live-chain commands;
- the statement that live-chain commands require configured RPC endpoints from `scripts/profiles/chains.json`;
- the statement that clean redeploy is out of scope unless explicitly approved.

Keep this guide operational. Do not turn it into a paper narrative.

### Task 2: Create the Stage 12 evidence index

Create `docs/implementation/data-trade-stage12-evidence-index.md`.

It should define a stable evidence layout under:

```text
target/data-trade-stage12/
```

Document expected evidence paths for every command in the demo matrix.

For each evidence item, include:

- command category (`dry-run`, `negative-validation`, `live-chain-happy`, `live-chain-dispute`);
- dataset fixture;
- request fixture;
- scenario;
- expected `result` or expected rejection;
- whether chain state is expected;
- whether proof artifacts are generated;
- paper usage note.

Important:

- Do not commit generated `target/` evidence unless the repository already tracks such generated evidence. Prefer documenting paths and summary format.
- If evidence is generated during validation, summarize it in docs but keep generated files out of git unless Codex/Owner explicitly asks to track them.

### Task 3: Run safe validation

Run validation that is safe in the current environment.

Required safe checks:

```bash
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js
go -C tools/data-trade-zk test ./...
go -C tools/data-trade-zk build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
```

Then run the three positive dry-run commands from the demo matrix if the Go build succeeds.

Run the two negative validation commands and confirm they reject before chain interaction.

Optional live-chain checks:

- First probe RPC readiness using the existing wait helper if appropriate:

```bash
node scripts/lib/wait_for_ws_chain.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --min-blocks 1 --timeout-ms 30000
```

- If RPC is not reachable, do not run live-chain scenario commands. Record "not run: RPC unavailable" in docs.
- If RPC is reachable, live-chain commands may be run only if this will not disrupt other work. Do not clean redeploy.

### Task 4: Update existing paper-facing docs

Update `docs/implementation/data-trade-evidence.md`:

- add a Stage 12 section linking to the demo guide and evidence index;
- summarize which commands were actually validated in Stage 12;
- distinguish generated dry-run evidence from live-chain evidence;
- if live-chain scenarios were not run, state that explicitly.

Update `docs/implementation/data-trade-implementation.md`:

- add a Stage 12 boundary note;
- mention the demo guide and evidence index;
- avoid claiming new implementation functionality.

Update `docs/implementation/data-trade-paper-gap-matrix.md`:

- add or revise rows only if needed to reflect Stage 12 documentation/evidence packaging;
- do not change statuses to stronger claims based only on dry-run evidence;
- preserve limitations for:
  - on-chain ZK verification;
  - trustless cross-chain settlement;
  - verifier quorum;
  - subset/substr constraints;
  - production dynamic IMT.

Update `docs/architecture/data-trade-security-model.md`:

- add a short Stage 12 note that this is evidence packaging/freeze;
- do not imply any security mechanism was added.

Update long-term roadmap:

```text
docs/internal/agent-plans/2026-06-26-data-trade-long-term-scripted-e2e-roadmap.md
```

- mark Stage 12 as complete only after CodeWhale has run the safe checks and updated docs;
- record whether live-chain commands were run or skipped.

### Task 5: Add execution notes

Add an execution notes section to the Stage 12 plan file itself after implementation.

It should record:

- branch name;
- commands actually run;
- pass/fail status;
- generated evidence paths, if any;
- live-chain status:
  - `run`;
  - `skipped: RPC unavailable`;
  - or `skipped: not approved / environment risk`;
- any deviations from this plan.

## Acceptance Criteria

Stage 12 is complete when:

- `docs/implementation/data-trade-demo-guide.md` exists and contains copy-pasteable demo commands.
- `docs/implementation/data-trade-stage12-evidence-index.md` exists and maps all required demo commands to expected evidence/results.
- Existing implementation/evidence/gap/security docs link or refer to the Stage 12 demo/evidence docs.
- Safe validation commands were run and recorded.
- Positive dry-run commands produce expected dynamic evidence if the ZK CLI builds.
- Negative validation commands reject as expected.
- Live-chain command status is explicitly recorded as run or skipped; there is no ambiguous "validated" claim.
- No generated `target/` evidence is committed unless explicitly approved.
- No new runtime/pallet/circuit/artifact schema behavior is introduced.

## Stop Conditions

Stop and ask Codex/Owner if:

- `go test` or `fishbone-zk` build fails for reasons unrelated to Stage 12 docs.
- Dry-run commands fail unexpectedly.
- A live-chain command requires clean redeploy or chain reset.
- RPC is reachable but chain state appears stale/incompatible with current metadata.
- You feel tempted to "fix" Stage 12 by changing pallets, runtime, circuits, proof digest encoding, or artifact schema.
- You need to commit generated evidence files under `target/`.

## Plan Review Checklist for CodeWhale

Before implementation, review this plan and comment on:

- whether the required demo matrix is complete enough for paper use;
- whether any command is unsafe or unrealistic in the current environment;
- whether the evidence index format needs more fields;
- whether any existing docs besides the listed files should be updated;
- whether the live-chain policy is clear enough to prevent accidental destructive operations.

Do not start implementation until Codex has resolved plan-review feedback.

## Codex Review Focus

When reviewing Stage 12 implementation, Codex should check:

- docs do not overclaim live-chain, trustless bridge, on-chain ZK, or production verifier guarantees;
- demo commands match actual script flags and fixture names;
- evidence paths are stable and not accidentally committed under `target/`;
- safe validation results are accurately recorded;
- roadmap and paper gap matrix remain consistent with the current implementation boundary;
- no unrelated code or generated artifacts are included.

## Execution Record

### 2026-06-27 CodeWhale Stage 12 Execution

- Branch: `feat/data-trade-stage12-paper-freeze`
- Validation: JS syntax ✅, Go test ✅, Go build ✅
- Dry-run results:
  - `factory-temperature-dry-run`: `result: dry-run-accepted` ✅
  - `factory-multi-range-dry-run`: `result: dry-run-accepted` ✅
  - `vehicle-speed-dry-run`: `result: dry-run-accepted` ✅
  - `factory-temperature-out-of-range`: correctly rejected ✅
  - `factory-multi-range-out-of-range`: correctly rejected ✅
- Live-chain: skipped (RPC unavailable)
- Deliverables:
  - `docs/implementation/data-trade-demo-guide.md` (7 commands, copy-pasteable)
  - `docs/implementation/data-trade-stage12-evidence-index.md` (evidence layout + paper usage)
  - Updated: evidence, implementation, gap-matrix, security-model, roadmap docs
