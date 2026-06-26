# Data Trade Stage 8 Code Review Follow-up

## Scope

Reviewed commit after requested fixes:

- `4bd2e13 fix: address Stage 8 code review findings`

Previous review:

- `docs/internal/agent-reviews/2026-06-27-data-trade-stage8-code-review.md`
- Previous decision: `changes requested`

## Decision

`approved`

The requested Stage 8 fixes have been applied. Stage 8 is ready to merge to `main`.

## Verification

### Finding 1: `mask_delta` overflow test

Status: fixed.

- `TestReadDatasetRejectsOverflowMaskDelta` now uses valid JSON.
- The overflow test passes through the intended dataset read path and rejects the out-of-range value.

### Finding 2: Gap matrix range row

Status: fixed.

- `Custom constraint kind: range` now documents Stage 8 dynamic multi-record/multi-field witness generation.
- The row also keeps subset/substr explicitly outside the implemented range constraint.

## Validation Run By Codex

```bash
go -C tools/data-trade-zk test ./internal/dynamic -run 'TestReadDatasetRejectsOverflowMaskDelta|TestReadDatasetRejectsOverflowValue|TestReadDatasetUsesUint64Guard' -count=1 -v
go -C tools/data-trade-zk test ./...
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js

rm -rf /tmp/fishbone-stage8-followup
mkdir -p /tmp/fishbone-stage8-followup
go -C tools/data-trade-zk run ./cmd/fishbone-zk make-witness \
  --dataset ../../scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request ../../scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --out /tmp/fishbone-stage8-followup/witness.json \
  --session-id 0 \
  --round-index 0
go -C tools/data-trade-zk run ./cmd/fishbone-zk business-fixture \
  --witness /tmp/fishbone-stage8-followup/witness.json \
  --out /tmp/fishbone-stage8-followup/proof \
  --session-id 0 \
  --round-index 0
go -C tools/data-trade-zk run ./cmd/fishbone-zk verify \
  --artifact /tmp/fishbone-stage8-followup/proof/artifact.json
```

Result:

- Targeted overflow tests passed.
- Full Go test suite passed.
- JS syntax checks passed.
- Dynamic `make-witness -> business-fixture -> verify` smoke passed.

## Remaining Notes

- Stage 8 remains range-only.
- Full scripted chain E2E orchestration is still deferred to Stage 9.
- No Rust/runtime/artifact schema/proof digest/attestation changes were introduced.
