# Stage 8 Data Trade Dynamic Dataset And Request Model Implementation Plan

> **Execution owner:** CodeWhale.
>
> **Codex role:** plan author, architecture reviewer, code reviewer, and final merge owner after review fixes pass.

## Goal

Make the data-trade ZK fixture path configurable by dataset and request, instead of relying on one fixed witness JSON.

After Stage 8, a script or CLI command should be able to take:

- a dataset JSON owned by DO,
- a request JSON created by DR,
- session metadata,

and produce a valid Stage 7 `RangeWitness` JSON for `fishbone-zk business-fixture`.

This stage is about **dynamic dataset/request to range witness generation**. It does not add new ZK constraint kinds yet.

## Current Baseline

Current working path:

- `fishbone-zk business-fixture --witness scripts/fixtures/data_trade_business_sample.json --out ...`
- `business.RangeWitness` contains one raw value, one range, one field, and one structured IMT fixture.
- Stage 7 structured IMT metadata is already present:
  - `dataset_id`
  - `field_name`
  - `record_id`
  - fixed structured IMT depths/indices
- `GenerateBusinessRangeFixture` already produces a verifiable artifact and uses `PrepareStructuredProof`.

Current limitation:

- The witness is manually authored.
- There is no dataset schema.
- There is no request schema.
- There is no reusable conversion from dataset/request to witness.
- Existing script defaults still point at one sample witness.

## Non-Goals

Do **not** implement any of the following in Stage 8:

- New constraint kinds such as subset/substr.
- Circuit redesign.
- Runtime, pallet, or chain storage changes.
- JS proof digest changes.
- Artifact schema changes.
- Attestation payload changes.
- Frontend UI.
- Rewriting `scripts/zk_real_data_trade_flow.js` or making it depend on dynamic fixtures by default.
- VM deployment or clean redeploy.
- Production data storage, encryption, ACL, wallet, or identity system.
- Production dynamic IMT service.

If any task appears to require one of these, stop and ask Codex/Owner.

## Design Decision

Introduce a Go package that converts a deterministic dataset/request pair into the existing `business.RangeWitness`.

Recommended package:

```text
tools/data-trade-zk/internal/dynamic
```

Keep `business.RangeWitness` as the canonical proof input for `business-fixture`.

Add a new CLI command:

```bash
fishbone-zk make-witness \
  --dataset <dataset.json> \
  --request <request.json> \
  --out <witness.json> \
  --session-id <id> \
  --round-index <idx>
```

Do not make `business-fixture` read dataset/request directly in Stage 8. This separation keeps the existing proof path stable:

```text
dataset + request -> make-witness -> RangeWitness -> business-fixture -> artifact -> verify
```

Do not wire this into the full chain E2E script yet. Stage 9 will handle end-to-end orchestration after this conversion layer is stable.

## Schema Contract

### Dataset JSON

Create deterministic demo dataset fixtures under `scripts/fixtures/data_trade_datasets/`.

Required minimal schema:

```json
{
  "version": 1,
  "dataset_id": "factory-sensors-demo",
  "description": "Factory sensor readings demo dataset",
  "schema_version": 1,
  "records": [
    {
      "record_id": "sensor-a-0001",
      "fields": {
        "temperature": {
          "type": "uint64",
          "value": 42,
          "salt_hex": "0x2222222222222222222222222222222222222222222222222222222222222222",
          "mask_delta": 1000
        },
        "humidity": {
          "type": "uint64",
          "value": 58,
          "salt_hex": "0x3333333333333333333333333333333333333333333333333333333333333333",
          "mask_delta": 2000
        }
      }
    }
  ]
}
```

Validation rules:

- `version == 1`
- `schema_version == 1`
- `dataset_id` non-empty ASCII
- at least 1 record
- each `record_id` non-empty ASCII and unique
- each field name non-empty ASCII and unique within the record
- Stage 8 supports only `type == "uint64"`
- `value` must fit `uint64`
- `salt_hex` must be 32-byte hex
- `mask_delta` must fit `uint64`

### Request JSON

Create deterministic request fixtures under `scripts/fixtures/data_trade_requests/`.

Required minimal schema:

```json
{
  "version": 1,
  "constraint_kind": "range",
  "request_hash": "0x6b9f5a1765adf428a2b7220c2fa6e11ef4f3d8235dc145d7c42b3e26fbd13a01",
  "dataset_id": "factory-sensors-demo",
  "record_id": "sensor-a-0001",
  "field_name": "temperature",
  "range": {
    "min_value": 18,
    "max_value": 65
  }
}
```

Validation rules:

- `version == 1`
- `constraint_kind == "range"` only in Stage 8
- `request_hash` must be 32-byte hex
- `dataset_id`, `record_id`, `field_name` non-empty ASCII
- `min_value <= max_value`

Dataset/request consistency rules:

- `request.dataset_id` must equal dataset `dataset_id`
- requested `record_id` must exist
- requested `field_name` must exist on that record
- requested field must be `uint64`
- field value must be inside `[min_value, max_value]`
- if field value is outside range, witness generation must reject

## Witness Generation Contract

`make-witness` must generate a `business.RangeWitness` equivalent to a manually-authored Stage 7 witness:

- `request_hash` from request
- `session_id` from CLI flag
- `round_index` from CLI flag
- `raw_value` from dataset selected field
- `min_value` / `max_value` from request
- `mask_delta` from dataset selected field
- `salt_hex` from dataset selected field
- `masked_value_hash` should be `""` by default, so `business-fixture` computes and validates it
- `imt.dataset_id` from dataset
- `imt.field_name` from request
- `imt.record_id` from request
- `imt.schema_version` from dataset
- all Stage 7 fixed depth/index metadata from `imt.DefaultFixture()`

Do not duplicate MiMC masked hash computation in the dynamic package unless it is already available through a clean existing helper. The proof path remains responsible for computing `masked_value_hash`.

## Deterministic Fixtures

Add at least two dataset fixtures:

- `scripts/fixtures/data_trade_datasets/factory_sensors.json`
- `scripts/fixtures/data_trade_datasets/vehicle_telematics.json`

Each dataset must include:

- at least 2 records
- at least 2 `uint64` fields per record
- deterministic salt/mask values

Add at least three request fixtures:

- one valid request for `factory_sensors.temperature`
- one valid request for a different field or record
- one invalid out-of-range request used by tests

## Tasks

### Task 1: Add Dataset/Request Schema Package

- [ ] Create `tools/data-trade-zk/internal/dynamic/schema.go`.
- [ ] Create `tools/data-trade-zk/internal/dynamic/schema_test.go`.
- [ ] Implement `Dataset`, `Record`, `Field`, `Request`, and `RangeConstraint` structs.
- [ ] Implement read/validate helpers:
  - `ReadDataset(path string) (Dataset, error)`
  - `ReadRequest(path string) (Request, error)`
  - `ValidateDataset(Dataset) error`
  - `ValidateRequest(Request) error`
- [ ] Add tests:
  - valid dataset accepts.
  - duplicate record id rejects.
  - unsupported field type rejects.
  - invalid salt rejects.
  - valid request accepts.
  - non-range `constraint_kind` rejects.
  - `min_value > max_value` rejects.

Validation:

```bash
go -C tools/data-trade-zk test ./internal/dynamic
```

Commit:

```bash
git add tools/data-trade-zk/internal/dynamic
git commit -m "feat(zk): add dynamic data trade dataset request schema"
```

### Task 2: Implement Dataset/Request To Witness Conversion

- [ ] Add conversion helper in `internal/dynamic`, for example:

```go
func BuildRangeWitness(dataset Dataset, request Request, sessionID uint32, roundIndex uint32) (business.RangeWitness, error)
```

- [ ] Enforce dataset/request consistency rules.
- [ ] Fill Stage 7 IMT metadata from dataset/request.
- [ ] Use `imt.DefaultFixture()` for fixed depth/index values.
- [ ] Keep `masked_value_hash` empty in the generated witness.
- [ ] Add tests:
  - valid dataset/request produces expected `RangeWitness` fields.
  - changing requested field changes `RawValue`, `MaskDelta`, `SaltHex`, and IMT `FieldName`.
  - changing requested record changes IMT `RecordID`.
  - out-of-range selected value rejects.
  - dataset id mismatch rejects.
  - missing record rejects.
  - missing field rejects.

Validation:

```bash
go -C tools/data-trade-zk test ./internal/dynamic ./internal/business ./internal/imt
```

Commit:

```bash
git add tools/data-trade-zk/internal/dynamic
git commit -m "feat(zk): build range witness from dynamic dataset request"
```

### Task 3: Add `make-witness` CLI Command

- [ ] Update `tools/data-trade-zk/cmd/fishbone-zk/main.go`.
- [ ] Add command:

```bash
fishbone-zk make-witness --dataset <dataset.json> --request <request.json> --out <witness.json> --session-id <id> --round-index <idx>
```

- [ ] Output pretty JSON with stable key names.
- [ ] Print at least:
  - `witness=<path>`
  - `dataset_id=<id>`
  - `record_id=<id>`
  - `field_name=<name>`
- [ ] Keep existing `business-fixture` behavior unchanged.
- [ ] Add command-level tests only if the repo already has a CLI test pattern; otherwise cover through package tests and CLI smoke.

Validation:

```bash
go -C tools/data-trade-zk test ./...
```

Commit:

```bash
git add tools/data-trade-zk/cmd/fishbone-zk tools/data-trade-zk/internal/dynamic
git commit -m "feat(zk): add dynamic witness generation command"
```

### Task 4: Add Fixtures And End-To-End Smoke

- [ ] Add dataset fixtures under `scripts/fixtures/data_trade_datasets/`.
- [ ] Add request fixtures under `scripts/fixtures/data_trade_requests/`.
- [ ] Keep `scripts/fixtures/data_trade_business_sample.json` for backward compatibility.
- [ ] Run smoke:

```bash
rm -rf /tmp/fishbone-stage8-dynamic
mkdir -p /tmp/fishbone-stage8-dynamic
go -C tools/data-trade-zk run ./cmd/fishbone-zk make-witness \
  --dataset ../../scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request ../../scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --out /tmp/fishbone-stage8-dynamic/witness.json \
  --session-id 0 \
  --round-index 0
go -C tools/data-trade-zk run ./cmd/fishbone-zk business-fixture \
  --witness /tmp/fishbone-stage8-dynamic/witness.json \
  --out /tmp/fishbone-stage8-dynamic/proof \
  --session-id 0 \
  --round-index 0
go -C tools/data-trade-zk run ./cmd/fishbone-zk verify \
  --artifact /tmp/fishbone-stage8-dynamic/proof/artifact.json
```

- [ ] Run a second valid request and confirm it produces a different `business_input_hash`.
- [ ] Run invalid out-of-range request and confirm `make-witness` rejects.

Commit:

```bash
git add scripts/fixtures/data_trade_datasets scripts/fixtures/data_trade_requests
git commit -m "test(zk): add dynamic data trade dataset request fixtures"
```

### Task 5: Documentation Updates

Update:

- `tools/data-trade-zk/README.md`
- `docs/implementation/data-trade-implementation.md`
- `docs/implementation/data-trade-paper-gap-matrix.md`
- `docs/architecture/data-trade-security-model.md`
- `docs/internal/agent-plans/2026-06-26-data-trade-long-term-scripted-e2e-roadmap.md`
- This plan's Execution Record

Required wording:

- Stage 8 introduces dynamic dataset/request to witness generation.
- Stage 8 remains range-only.
- Stage 8 does not implement subset/substr.
- Stage 8 does not implement frontend UI.
- Stage 8 does not change runtime, artifact schema, or attestation payloads.
- Existing `business-fixture --witness` remains supported.

Suggested gap matrix updates:

- `Circuit-level range business witness`: gap should no longer say fixture data is static and single-field; update to "dynamic dataset/request prototype for range only."
- `Custom constraint kind: range`: update current implementation to mention dynamic multi-record/multi-field range witness generation.
- `Custom constraint kinds: subset/substr`: still `not-implemented`.

Commit:

```bash
git add tools/data-trade-zk/README.md docs/implementation/data-trade-implementation.md docs/implementation/data-trade-paper-gap-matrix.md docs/architecture/data-trade-security-model.md docs/internal/agent-plans/2026-06-26-data-trade-long-term-scripted-e2e-roadmap.md docs/internal/agent-plans/2026-06-27-stage8-data-trade-dynamic-dataset-request-model.md
git commit -m "docs: record dynamic data trade dataset request model"
```

### Task 6: Final Validation

Run:

```bash
go -C tools/data-trade-zk test ./...
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js
```

Rust is not required unless Stage 8 accidentally touches Rust. If Rust files change, stop and ask Codex before proceeding.

## Stop Conditions

CodeWhale must stop and ask Codex before:

- Changing gnark circuits.
- Adding subset/substr or any non-range constraint.
- Changing `artifact.ProofArtifact`.
- Changing `ProofDigestDomain` or `ComputeProofDigest`.
- Changing JS digest calculation.
- Changing Rust pallets/runtime.
- Changing attestation payload encoding.
- Making `business-fixture` incompatible with existing witness JSON.
- Adding frontend UI.
- Running VM deployment or clean redeploy.
- Adding external dependencies.

## Acceptance Criteria

- Existing `business-fixture --witness scripts/fixtures/data_trade_business_sample.json` still works.
- `make-witness` can generate a valid `RangeWitness` from dataset/request JSON.
- At least two valid dynamic requests generate valid proof artifacts accepted by `fishbone-zk verify`.
- Different dataset/record/field requests change `business_input_hash`.
- Invalid out-of-range request rejects before proof generation.
- No runtime/Rust/artifact schema/JS digest/attestation changes.
- Documentation clearly states Stage 8 is dynamic range witness generation, not subset/substr or frontend.

## Execution Record

Not started.
