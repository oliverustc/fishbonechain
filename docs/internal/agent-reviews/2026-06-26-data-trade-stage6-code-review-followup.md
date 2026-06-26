# Data Trade Stage 6 Code Review Follow-up

## Scope

Reviewed commit after requested fixes:

- `c6b75eb fix: address Stage 6 code review findings`

Previous review:

- `docs/internal/agent-reviews/2026-06-26-data-trade-stage6-code-review.md`
- Previous decision: `changes requested`

## Decision

`approved`

The requested Stage 6 fixes have been applied. Stage 6 is ready to merge to `main` from the code-review perspective.

## Verification

### Finding 1: Selected IMT root binds fixture metadata

Status: fixed.

- `tools/data-trade-zk/internal/imt/proof.go` now derives padding leaves with `fixtureString(domainPad, f.DatasetID, f.FieldName, i)`.
- `TestPrepareProofChangingDatasetIDChangesRoot` now asserts that changing `dataset_id` changes `Root0`, not only decoy roots.

### Finding 2: Byte-exact string encoding test

Status: fixed.

- `strLE` is now a package-level helper.
- `TestStrLEEncodesExactly` verifies:
  - `"demo"` -> `04 00 00 00 64 65 6d 6f`
  - `""` -> `00 00 00 00`

### Finding 3: Padding leaf count documentation

Status: fixed.

- Documentation now says depth 10 has `1023` padding leaves.
- Updated files:
  - `docs/implementation/data-trade-implementation.md`
  - `tools/data-trade-zk/README.md`

## Validation Run By Codex

```bash
go -C tools/data-trade-zk test ./...
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js
```

Result: passed.

## Remaining Notes

- Stage 6 remains fixture-level deterministic IMT coupling, not production IMT.
- No Rust/runtime changes were introduced.
- No artifact schema or proof digest encoding changes were introduced.
