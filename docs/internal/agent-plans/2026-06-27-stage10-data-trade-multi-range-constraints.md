# Stage 10 Data Trade Multi-Range Constraint Implementation Plan

> **Execution owner:** CodeWhale.
>
> **Codex role:** plan author, architecture reviewer, code reviewer, and final merge owner after review fixes pass.

## Goal

Extend the dynamic data-trade request model from a single range constraint to a small, practical conjunction of range constraints.

Stage 9 can already run:

```text
dataset + single range request
  -> make-witness
  -> business-fixture
  -> verify
  -> scripted chain E2E / dry-run evidence
```

Stage 10 should support a request like:

```text
factory sensor record A must satisfy:
  temperature in [35, 50]
  pressure in [900, 1100]
```

and produce a verifiable scripted proof flow for each condition, with one evidence file describing the combined request.

This stage is still **range-only at the circuit level**. It improves request flexibility by allowing multiple range conditions in one logical request. It must not implement subset/substr circuits in Stage 10.

## Why Multi-Range Instead Of Subset/Substr Now

The current codebase has:

- `BusinessRangeProof` as the only real business circuit.
- `business.RangeWitness` as the only business witness schema.
- `fishbone-zk business-fixture` hardwired to `GenerateBusinessRangeFixture`.
- `artifact.ProofArtifact` already reserving `range/subset/substr`, but only range artifacts are actually generated and verified.
- `trade-session` and JS proof digest logic already stable for per-round range artifacts.

Adding subset/substr now would require new witness schemas, new circuits, new artifact generation/verification branches, new tests, and careful proof digest semantics. That is a larger architecture step.

Multi-range conjunction gives the thesis demo a better "flexible request" story while staying inside the proven range circuit and current on-chain boundary.

## Non-Goals

Do **not** implement any of the following in Stage 10:

- subset circuit.
- substr circuit.
- new gnark circuit unless Codex explicitly approves after a stop condition.
- runtime/pallet/storage changes.
- `artifact.ProofArtifact` schema changes.
- JS proof digest encoding changes.
- verifier attestation payload/digest changes.
- on-chain Groth16 verification.
- trustless bridge / CCMC / Merkle proof settlement.
- frontend.
- production request-language parser.
- external dependencies.

If implementation appears to require any item above, stop and ask Codex/Owner.

## Design Decision

Add a backward-compatible request schema extension:

### Existing Single-Constraint Request

Keep supporting the current Stage 8/9 shape:

```json
{
  "version": 1,
  "constraint_kind": "range",
  "request_hash": "0x...",
  "dataset_id": "factory-sensors",
  "record_id": "sensor-a-0001",
  "field_name": "temperature",
  "range": {
    "min_value": 35,
    "max_value": 50
  }
}
```

### New Multi-Range Request

Add support for:

```json
{
  "version": 1,
  "constraint_kind": "multi_range",
  "request_hash": "0x...",
  "dataset_id": "factory-sensors",
  "record_id": "sensor-a-0001",
  "constraints": [
    {
      "field_name": "temperature",
      "range": {
        "min_value": 35,
        "max_value": 50
      }
    },
    {
      "field_name": "pressure",
      "range": {
        "min_value": 900,
        "max_value": 1100
      }
    }
  ]
}
```

Semantics:

- `multi_range` means logical **AND** across all constraints.
- All constraints must target the same `dataset_id` and `record_id` in Stage 10.
- Each constraint targets one `uint64` field.
- Each constraint produces one `business.RangeWitness`.
- Each witness uses the same `request_hash`, `session_id`, and `round_index`, but different field metadata and field value.
- The script must verify every generated artifact before treating the combined request as accepted.
- Evidence must preserve per-constraint artifact details.

Important boundary:

- On-chain `TradeSession` still sees one `request_hash` and one submitted proof digest/attestation per round.
- Stage 10 does **not** add a new on-chain "aggregate proof" object.
- For live E2E compatibility, use the first verified constraint artifact as the on-chain proof metadata for that round, and record all verified constraint artifacts in off-chain evidence.
- Documentation must explicitly state this limitation: multi-range is a scripted/off-chain conjunction prototype; the chain currently binds one proof digest per round.

This keeps Stage 10 implementable without runtime or digest schema changes.

## Implementation Tasks

### Task 1: Extend Dynamic Request Schema

Files:

- `tools/data-trade-zk/internal/dynamic/schema.go`
- `tools/data-trade-zk/internal/dynamic/schema_test.go`

Add:

- `ConstraintKind` may be `"range"` or `"multi_range"`.
- New request field:

```go
Constraints []RangeFieldConstraint `json:"constraints,omitempty"`
```

- Suggested type:

```go
type RangeFieldConstraint struct {
    FieldName string          `json:"field_name"`
    Range     RangeConstraint `json:"range"`
}
```

Validation rules:

- For `constraint_kind == "range"`:
  - Preserve existing required fields: `field_name` and `range`.
  - `constraints` should be empty or ignored only if empty. Prefer rejecting non-empty `constraints` in single range mode to avoid ambiguity.
- For `constraint_kind == "multi_range"`:
  - `field_name` must be empty.
  - Top-level `range` must be omitted or left as the JSON zero value (`min_value: 0`, `max_value: 0`). Do not read top-level `range` in multi-range mode.
  - `constraints` length must be at least 2.
  - Put a practical cap, e.g. max 4 constraints, to keep proof generation time reasonable.
  - Each constraint must have non-empty ASCII `field_name`.
  - Each `range.min_value <= range.max_value`.
  - Duplicate `field_name` in one request should reject.
  - Keep `request_hash`, `dataset_id`, and `record_id` validation exactly as strict as today.

Backward compatibility:

- Existing Stage 8/9 request fixtures must still pass unchanged.

Tests:

- Existing single range request still validates.
- Valid multi-range request validates.
- Empty constraints reject.
- One constraint in `multi_range` rejects.
- Duplicate fields reject.
- Non-ASCII field rejects.
- `min_value > max_value` rejects.
- Non-range/non-multi-range `constraint_kind` rejects.

Commit:

```bash
git add tools/data-trade-zk/internal/dynamic/schema.go tools/data-trade-zk/internal/dynamic/schema_test.go
git commit -m "feat(dynamic): support multi range request schema"
```

### Task 2: Build Multiple Range Witnesses

Files:

- `tools/data-trade-zk/internal/dynamic/witness.go`
- `tools/data-trade-zk/internal/dynamic/witness_test.go`

Add a function that returns all per-constraint witnesses:

```go
func BuildRangeWitnesses(ds Dataset, req Request, sessionID, roundIndex uint32) ([]business.RangeWitness, error)
```

Behavior:

- For `constraint_kind == "range"`:
  - Return a one-element slice.
  - Reuse current `BuildRangeWitness` behavior.
- For `constraint_kind == "multi_range"`:
  - Validate dataset/request alignment.
  - Find the requested record once.
  - For each constraint:
    - find field;
    - require field type `uint64`;
    - enforce field value in requested range;
    - build a `business.RangeWitness` using:
      - same `RequestHash`
      - same `SessionID`
      - same `RoundIndex`
      - per-field `RawValue`, `MinValue`, `MaxValue`, `MaskDelta`, `SaltHex`
      - IMT fixture metadata using same dataset/record/schema and per-field `FieldName`
  - Preserve constraint order from request JSON.

Keep the existing `BuildRangeWitness` function as a compatibility wrapper:

```go
func BuildRangeWitness(ds Dataset, req Request, sessionID, roundIndex uint32) (business.RangeWitness, error) {
    witnesses, err := BuildRangeWitnesses(ds, req, sessionID, roundIndex)
    if err != nil { ... }
    if len(witnesses) != 1 { return business.RangeWitness{}, fmt.Errorf("...") }
    return witnesses[0], nil
}
```

Do not remove `BuildRangeWitness`; existing call sites and tests depend on it.

Tests:

- Single range returns one witness identical to prior behavior.
- Multi-range returns witnesses in request order.
- Each witness has the correct per-field raw value/range/mask/salt/IMT field name.
- Missing second field rejects.
- Out-of-range second field rejects.
- Dataset ID mismatch still rejects.
- Missing record still rejects.

Commit:

```bash
git add tools/data-trade-zk/internal/dynamic/witness.go tools/data-trade-zk/internal/dynamic/witness_test.go
git commit -m "feat(dynamic): build witnesses for multi range requests"
```

### Task 3: Add CLI Support For Multi-Range Witness Output

File:

- `tools/data-trade-zk/cmd/fishbone-zk/main.go`

Current `make-witness` writes one witness JSON to `--out`.

Extend `make-witness` so it supports both:

#### Single Range

Keep current behavior unchanged:

```bash
fishbone-zk make-witness \
  --dataset <dataset.json> \
  --request <single-range-request.json> \
  --out <witness.json>
```

#### Multi-Range

When request is `multi_range`, use a new `--out-dir` flag for the output directory:

```bash
fishbone-zk make-witness \
  --dataset <dataset.json> \
  --request <multi-range-request.json> \
  --out-dir <out-dir>
```

Expected output:

```text
<out-dir>/witness-0.json
<out-dir>/witness-1.json
<out-dir>/manifest.json
```

Suggested `manifest.json`:

```json
{
  "version": 1,
  "constraint_kind": "multi_range",
  "request_hash": "0x...",
  "dataset_id": "factory-sensors",
  "record_id": "sensor-a-0001",
  "witnesses": [
    {
      "index": 0,
      "field_name": "temperature",
      "witness_path": "witness-0.json"
    },
    {
      "index": 1,
      "field_name": "pressure",
      "witness_path": "witness-1.json"
    }
  ]
}
```

Compatibility rules:

- Single range must still write exactly the JSON witness to `--out`.
- Multi-range must use `--out-dir`, not `--out`.
- Reject `--out-dir` for single-range requests.
- Reject `--out` for multi-range requests.
- Reject calls that provide both `--out` and `--out-dir`.
- Do not silently overwrite an existing regular file when multi-range expects a directory. If `--out-dir` exists and is a file, exit with a clear error.
- Create output directory when needed.
- Print readable output lines, for example:

```text
witness_manifest=<out-dir>/manifest.json
witness[0]=<out-dir>/witness-0.json field=temperature
witness[1]=<out-dir>/witness-1.json field=pressure
```

Tests/checks:

```bash
go -C tools/data-trade-zk test ./...
go -C tools/data-trade-zk build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
```

Manual smoke:

```bash
rm -rf /tmp/fishbone-stage10-mw
target/tools/fishbone-zk make-witness \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --out-dir /tmp/fishbone-stage10-mw \
  --session-id 0 \
  --round-index 0
test -f /tmp/fishbone-stage10-mw/manifest.json
test -f /tmp/fishbone-stage10-mw/witness-0.json
test -f /tmp/fishbone-stage10-mw/witness-1.json
```

Commit:

```bash
git add tools/data-trade-zk/cmd/fishbone-zk/main.go
git commit -m "feat(cli): emit multi range witness bundles"
```

### Task 4: Add Multi-Range Fixtures

Files:

- `scripts/fixtures/data_trade_requests/factory_multi_range.json`
- `scripts/fixtures/data_trade_requests/factory_multi_range_out_of_range.json`
- optional: `scripts/fixtures/data_trade_requests/vehicle_multi_range.json`

Required fixture:

- `factory_multi_range.json`:
  - `dataset_id`: `factory-sensors`
  - `record_id`: an existing record in `factory_sensors.json`
  - constraints:
    - existing numeric field `temperature`, valid range
    - existing numeric field `pressure` or another available numeric field, valid range

Negative fixture:

- `factory_multi_range_out_of_range.json`:
  - same dataset/record
  - one valid constraint
  - one invalid out-of-range constraint

Important:

- `request_hash` can be a fixed 32-byte hex fixture value. It does not need to be cryptographically derived in Stage 10, but it must differ from existing single-range request hashes.
- Use fields that actually exist in the selected dataset. Check fixture field names before writing requests.

Commit:

```bash
git add scripts/fixtures/data_trade_requests/factory_multi_range*.json scripts/fixtures/data_trade_requests/vehicle_multi_range.json
git commit -m "test(fixtures): add multi range data trade requests"
```

If the optional vehicle fixture is not added, omit it from `git add`.

### Task 5: Extend ZK Dynamic Dry-Run Script Path

File:

- `scripts/zk_real_data_trade_flow.js`

Current dynamic mode assumes `make-witness --out <file>` and then runs `business-fixture` once per round.

Add support for `multi_range` requests while preserving single range behavior.

Implementation guidance:

1. Read dynamic request JSON as today.
2. Detect:
   - `constraint_kind === "range"`: existing behavior.
   - `constraint_kind === "multi_range"`: new behavior.
3. Add helper logic to normalize witnesses for one round:

```js
function generateDynamicWitnessBundle({ outDir, sessionId, roundIndex }) {
  // range: writes outDir/witness.json and returns [{ index: 0, fieldName, witnessPath }]
  // multi_range: runs make-witness with --out-dir outDir/witnesses,
  //              reads manifest.json,
  //              returns manifest witnesses resolved to absolute/relative paths
}
```

4. For each witness returned:
   - run `business-fixture` into a per-constraint artifact directory, e.g.

```text
target/data-trade-zk/session-<id>-round-<round>/constraint-0/
target/data-trade-zk/session-<id>-round-<round>/constraint-1/
```

   - run `verify` for each artifact.
   - record each proof digest/business hash/public input hash in evidence.

5. For live chain submission:
   - submit only the first verified constraint artifact to `tradeSession.submitDataProof` and attestation for compatibility with current pallet.
   - evidence must mark the first artifact as `"on_chain_bound": true` and the remaining verified artifacts as `"on_chain_bound": false`.
   - Add a short evidence-level field, for example:

```json
"chain_binding_mode": "first_constraint_digest"
```

6. For `--dry-run-dynamic`:
   - run and verify all constraints;
   - write evidence with all constraints;
   - no chain RPC.

7. For live preflight:
   - single range: existing preflight behavior is acceptable.
   - multi-range: preflight must validate all constraints by invoking `make-witness` into a preflight directory before chain connection.

Stop condition:

- If CodeWhale believes live-chain multi-range cannot be represented honestly without runtime or proof digest schema changes, stop and ask Codex. Do not fake aggregate on-chain proof semantics.

Suggested evidence shape for a round:

```json
{
  "round_index": 0,
  "constraint_kind": "multi_range",
  "chain_binding_mode": "first_constraint_digest",
  "constraints": [
    {
      "index": 0,
      "field_name": "temperature",
      "witness_path": ".../witness-0.json",
      "artifact_path": ".../constraint-0/artifact.json",
      "proof_digest": "0x...",
      "business_input_hash": "0x...",
      "public_input_hash": "0x...",
      "on_chain_bound": true
    },
    {
      "index": 1,
      "field_name": "pressure",
      "witness_path": ".../witness-1.json",
      "artifact_path": ".../constraint-1/artifact.json",
      "proof_digest": "0x...",
      "business_input_hash": "0x...",
      "public_input_hash": "0x...",
      "on_chain_bound": false
    }
  ]
}
```

Single-range dynamic evidence must use the same round shape with a one-element `constraints` array:

```json
{
  "round_index": 0,
  "constraint_kind": "range",
  "chain_binding_mode": "single_constraint_digest",
  "constraints": [
    {
      "index": 0,
      "field_name": "temperature",
      "witness_path": ".../witness.json",
      "artifact_path": ".../constraint-0/artifact.json",
      "proof_digest": "0x...",
      "business_input_hash": "0x...",
      "public_input_hash": "0x...",
      "on_chain_bound": true
    }
  ]
}
```

Do not write separate top-level `witness_path`, `artifact_path`, `proof_digest`, `business_input_hash`, or `public_input_hash` fields for dynamic-mode rounds. Put these fields only inside `rounds[].constraints[]`. Legacy `--business-witness` mode may keep its existing evidence shape if changing it would expand scope.

Backward compatibility:

- Existing single range dry-run must keep working.
- Existing single range dynamic evidence must be normalized to the same `constraints` array shape as multi-range evidence.
- Documentation must call out this Stage 10 evidence shape change from Stage 9 flat dynamic round fields.
- Existing legacy `--business-witness` path must keep working.

Commit:

```bash
git add scripts/zk_real_data_trade_flow.js
git commit -m "feat(script): support multi range dynamic zk flow"
```

### Task 6: Documentation Updates

Update:

- `docs/implementation/data-trade-implementation.md`
- `docs/implementation/data-trade-paper-gap-matrix.md`
- `docs/implementation/data-trade-evidence.md`
- `docs/architecture/data-trade-security-model.md`
- `docs/internal/agent-plans/2026-06-26-data-trade-long-term-scripted-e2e-roadmap.md`
- this plan's Execution Record

Required wording:

- Stage 10 supports `multi_range` request fixtures as an off-chain/scripted conjunction of multiple range proofs.
- The real circuit is still `BusinessRangeProof`; Stage 10 does not implement subset/substr.
- In live chain mode, current runtime binds one proof digest per round. Stage 10 records the full multi-range proof set in evidence and binds the first verified constraint artifact on-chain for compatibility.
- Dynamic evidence now uses a normalized `rounds[].constraints[]` shape for both single range and multi-range requests.
- This is a prototype representation of flexible request constraints, not a production aggregate proof.
- Do not claim on-chain verification, trustless bridge settlement, subset/substr, verifier quorum, or frontend support.

Commit:

```bash
git add docs/implementation/data-trade-implementation.md docs/implementation/data-trade-paper-gap-matrix.md docs/implementation/data-trade-evidence.md docs/architecture/data-trade-security-model.md docs/internal/agent-plans/2026-06-26-data-trade-long-term-scripted-e2e-roadmap.md docs/internal/agent-plans/2026-06-27-stage10-data-trade-multi-range-constraints.md
git commit -m "docs: record multi range data trade constraints"
```

### Task 7: Final Validation

Required:

```bash
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js
go -C tools/data-trade-zk test ./...
go -C tools/data-trade-zk build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
```

Required single-range regression:

```bash
rm -rf /tmp/fishbone-stage10-single
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --evidence-out /tmp/fishbone-stage10-single/evidence.json \
  --dry-run-dynamic
test -f /tmp/fishbone-stage10-single/evidence.json
```

Required multi-range dry-run:

```bash
rm -rf /tmp/fishbone-stage10-multi
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --evidence-out /tmp/fishbone-stage10-multi/evidence.json \
  --dry-run-dynamic
test -f /tmp/fishbone-stage10-multi/evidence.json
```

Required negative multi-range dry-run:

```bash
rm -rf /tmp/fishbone-stage10-multi-bad
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range_out_of_range.json \
  --evidence-out /tmp/fishbone-stage10-multi-bad/evidence.json \
  --dry-run-dynamic
```

Expected: exits non-zero before proof submission/acceptance.

Optional live chain validation:

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --evidence-out /tmp/fishbone-stage10-live/evidence.json
```

Only record live chain validation as passed if it actually runs against reachable RPC endpoints. If RPC is unavailable, record it as not run due to environment.

## Stop Conditions

CodeWhale must stop and ask Codex before:

- Changing Rust/runtime/pallet code.
- Changing `artifact.ProofArtifact`.
- Changing JS proof digest calculation.
- Changing verifier attestation digest or payload encoding.
- Adding subset/substr or any non-range circuit.
- Replacing `BusinessRangeProof`.
- Claiming the chain verifies all multi-range constraints on-chain.
- Claiming an aggregate proof was implemented.
- Removing or breaking single range dynamic mode.
- Removing or breaking `--business-witness`.
- Running VM clean redeploy.
- Adding external dependencies.

## Acceptance Criteria

- Existing single range request fixtures still pass.
- `fishbone-zk make-witness` supports both:
  - single range file output via `--out <witness.json>`;
  - multi-range directory output via `--out-dir <dir>` with `manifest.json` and per-constraint witnesses.
- `make-witness` rejects ambiguous output flags:
  - `--out-dir` with single range;
  - `--out` with multi-range;
  - both `--out` and `--out-dir` together.
- Multi-range request can generate and verify at least two range artifacts in one dynamic dry-run.
- Multi-range out-of-range fixture fails.
- `zk_real_data_trade_flow.js --dry-run-dynamic` writes normalized `rounds[].constraints[]` evidence listing every constraint artifact for both single and multi-range dynamic requests.
- Live chain path, if run, honestly records that only the first constraint digest is bound on-chain.
- No runtime, artifact schema, JS digest, or attestation encoding changes.
- Documentation accurately describes Stage 10 as a scripted/off-chain multi-range conjunction prototype.

## Plan Review Resolution

CodeWhale reviewed the initial Stage 10 plan and raised two medium findings. Both are resolved in this plan revision:

- `make-witness` now uses `--out <file>` only for single range and `--out-dir <dir>` only for multi-range.
- Dynamic evidence now uses one normalized `rounds[].constraints[]` shape for both single range and multi-range.

Remaining accepted design decisions:

- The live-chain "first constraint digest" compatibility mode is intentionally documented and must not be overstated.
- Multi-range `request_hash` may remain fixture-fixed in Stage 10. Later stages may add deterministic request hashing.

## Execution Record

- 2026-06-27: Initial Codex plan committed.
- 2026-06-27: CodeWhale plan review completed. Plan updated to resolve `--out`/`--out-dir` ambiguity and require normalized dynamic evidence shape.
