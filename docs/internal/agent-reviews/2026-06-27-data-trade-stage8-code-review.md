# Data Trade Stage 8 Code Review

## Scope

Reviewed branch: `feat/data-trade-stage8-dynamic-requests`

Reviewed commits after Stage 8 plan review:

- `876aa92 feat(zk): add dynamic data trade dataset request schema`
- `fb000c4 feat(zk): build range witness from dynamic dataset request`
- `c10f871 feat(zk): add dynamic witness generation command`
- `980c6c2 test(zk): add dynamic data trade dataset request fixtures`
- `9f49fa2 docs: record dynamic data trade dataset request model`

## Decision

`changes requested`

The Stage 8 implementation works end-to-end and keeps the intended architecture boundary. Two small issues should be fixed before merge: one test does not actually exercise the intended overflow path, and one gap-matrix row was not updated per plan.

## Findings

### 1. Low: `mask_delta` overflow test passes for the wrong reason

File: `tools/data-trade-zk/internal/dynamic/schema_test.go`

Lines: `136-143`

`TestReadDatasetRejectsOverflowMaskDelta` builds JSON that appears to be malformed: the field object is not closed before the record array closes.

Current fragment:

```json
"mask_delta": 18446744073709551616}}]
```

Expected shape should close field, fields map, record object, records array:

```json
"mask_delta": 18446744073709551616}}}]
```

Because the JSON is malformed, the test can pass due to JSON syntax rejection rather than due to the intended uint64 overflow guard. This weakens the exact plan requirement added after Stage 8 plan review.

Required fix:

- Correct the JSON in `TestReadDatasetRejectsOverflowMaskDelta`.
- Ensure the test still fails when `mask_delta` is `18446744073709551616`.
- Prefer asserting the error mentions `mask_delta` or `uint64 range`, so syntax errors do not satisfy the test.

### 2. Low: `Custom constraint kind: range` gap-matrix row still says only one field/sample fixture

File: `docs/implementation/data-trade-paper-gap-matrix.md`

Line: `32`

The Stage 8 plan asked to update this row to mention dynamic multi-record/multi-field range witness generation. The `Circuit-level range business witness` row was updated correctly, but `Custom constraint kind: range` still says:

```text
Only one field/sample fixture
```

That is stale after Stage 8.

Required fix:

- Update the row to mention that range remains the only implemented constraint kind, but now supports dynamic dataset/request witness generation over multiple records/fields.
- Keep subset/substr marked `not-implemented`.

## Validation Run By Codex

```bash
go -C tools/data-trade-zk test ./internal/dynamic -run 'TestReadDatasetRejectsOverflow|TestReadDatasetUsesUint64Guard' -count=1 -v

rm -rf /tmp/fishbone-stage8-review
mkdir -p /tmp/fishbone-stage8-review
go -C tools/data-trade-zk run ./cmd/fishbone-zk make-witness \
  --dataset ../../scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request ../../scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --out /tmp/fishbone-stage8-review/factory_witness.json \
  --session-id 0 \
  --round-index 0
go -C tools/data-trade-zk run ./cmd/fishbone-zk business-fixture \
  --witness /tmp/fishbone-stage8-review/factory_witness.json \
  --out /tmp/fishbone-stage8-review/factory_proof \
  --session-id 0 \
  --round-index 0
go -C tools/data-trade-zk run ./cmd/fishbone-zk verify \
  --artifact /tmp/fishbone-stage8-review/factory_proof/artifact.json

go -C tools/data-trade-zk run ./cmd/fishbone-zk make-witness \
  --dataset ../../scripts/fixtures/data_trade_datasets/vehicle_telematics.json \
  --request ../../scripts/fixtures/data_trade_requests/vehicle_speed_range.json \
  --out /tmp/fishbone-stage8-review-vehicle/witness.json \
  --session-id 0 \
  --round-index 0
go -C tools/data-trade-zk run ./cmd/fishbone-zk business-fixture \
  --witness /tmp/fishbone-stage8-review-vehicle/witness.json \
  --out /tmp/fishbone-stage8-review-vehicle/proof \
  --session-id 0 \
  --round-index 0
go -C tools/data-trade-zk run ./cmd/fishbone-zk verify \
  --artifact /tmp/fishbone-stage8-review-vehicle/proof/artifact.json

go -C tools/data-trade-zk run ./cmd/fishbone-zk make-witness \
  --dataset ../../scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request ../../scripts/fixtures/data_trade_requests/factory_temperature_out_of_range.json \
  --out /tmp/fishbone-stage8-review/bad_witness.json \
  --session-id 0 \
  --round-index 0

go -C tools/data-trade-zk run ./cmd/fishbone-zk business-fixture \
  --witness ../../scripts/fixtures/data_trade_business_sample.json \
  --out /tmp/fishbone-stage8-review-compat \
  --session-id 0 \
  --round-index 0
go -C tools/data-trade-zk run ./cmd/fishbone-zk verify \
  --artifact /tmp/fishbone-stage8-review-compat/artifact.json

go -C tools/data-trade-zk test ./...
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js
```

Result:

- Go and JS checks passed.
- Factory dynamic request accepted.
- Vehicle dynamic request accepted with a different `business_input_hash`.
- Out-of-range dynamic request rejected before proof generation.
- Existing `business-fixture --witness scripts/fixtures/data_trade_business_sample.json` still works and verifies.

## Notes

- `make-witness -> business-fixture -> verify` pipeline is correctly separated.
- No Rust/runtime changes were introduced.
- No artifact schema, proof digest, JS digest, or attestation encoding changes were introduced.
- Existing proof path remains backward-compatible.
