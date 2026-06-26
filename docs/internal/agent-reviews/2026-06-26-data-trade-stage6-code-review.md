# Data Trade Stage 6 Code Review

## Scope

Reviewed branch: `feat/data-trade-stage6-imt-coupling`

Reviewed commits after Stage 6 plan review:

- `503ec9d feat(zk): add deterministic IMT fixture schema`
- `9ac5ecc feat(zk): attach IMT fixture to business witness`
- `bc10266 feat(zk): derive deterministic IMT range proof inputs`
- `8424384 feat(zk): bind business range proof to deterministic IMT root`
- `4e1982c docs: record deterministic IMT business coupling`
- `3f4a10d docs: record Stage 6 execution complete`

## Decision

`changes requested`

The implementation is directionally correct and the core smoke path passes, but two plan-level requirements are not fully satisfied. Fix these before merging Stage 6 to `main`.

## Findings

### 1. Medium: Selected IMT root does not bind fixture metadata

File: `tools/data-trade-zk/internal/imt/proof.go`

Lines: `31`, `84-93`, `119-127`

The Stage 6 plan says padding leaves must be "derived from the fixture fields." Current implementation derives padding leaves only from:

```text
FISHBONE:DATA_TRADE:IMT:PAD:v1 || index
```

`dataset_id` and `field_name` are used only for `Root1..Root3` decoy roots. Because `Index0=0` and `Index1=0`, the circuit-selected root is always `Root0`; therefore changing fixture metadata does not change the selected committed root. It changes the RO public witness through decoy roots and changes `business_input_hash`, but the actual selected IMT root remains independent of dataset/field identity.

Required fix:

- Include `dataset_id` and `field_name` in padding leaf derivation, or otherwise include fixture metadata in the selected `Root0` derivation.
- Add a test that changing `dataset_id` changes `Root0`, not only decoy roots.
- Keep deterministic behavior and do not add external dependencies.

### 2. Low: Required byte-exact string encoding test is missing

File: `tools/data-trade-zk/internal/gnarkadapter/business_range_obfuscation.go`

Lines: `145-153`

File: `tools/data-trade-zk/internal/gnarkadapter/business_range_obfuscation_test.go`

Lines: `147-183`

The plan review required a unit-level byte-exact test:

```text
"demo" => 04 00 00 00 64 65 6d 6f
```

Current test only proves that `"A"` and `"AB"` produce different `business_input_hash` values. That does not verify the exact 4-byte little-endian length-prefix convention and would not catch several encoding bugs.

Required fix:

- Move the string encoding helper out of the local closure into a small package-level helper, or otherwise make it directly testable.
- Add a byte-exact test for `"demo"`.

### 3. Low: Documentation says there are 9 padding leaves for a depth-10 tree

File: `docs/implementation/data-trade-implementation.md`

Line: `136`

File: `tools/data-trade-zk/README.md`

Lines: `21-25`

A depth-10 binary tree has `2^10 = 1024` leaves. With leaf `0 = masked_value_hash`, there are `1023` padding leaves, not `9`. The current wording is likely confusing "depth 10" with "9 other leaves."

Required fix:

- Replace `9 padding leaves` with `1023 padding leaves` or a clearer phrase such as `all remaining depth-10 leaves are deterministic padding leaves`.

## Validation Run By Codex

```bash
go -C tools/data-trade-zk test ./internal/imt ./internal/business ./internal/gnarkadapter ./internal/artifact
go -C tools/data-trade-zk test ./...
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js
```

Result: passed.

Rust validation was not rerun by Codex during this review because Stage 6 did not touch Rust and CodeWhale recorded `SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session` as passed.

## Notes

- The artifact schema was not expanded.
- Rust/runtime files were not modified.
- JS digest encoding was not modified.
- `business-fixture` now uses deterministic `AssignFixture` rather than random RO assignment.
- `ProofArtifact.ComputeProofDigest` was not changed.
