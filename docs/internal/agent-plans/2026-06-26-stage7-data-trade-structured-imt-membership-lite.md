# Stage 7 Data Trade Structured IMT Membership Lite Implementation Plan

> **Execution owner:** CodeWhale.
>
> **Codex role:** plan author, architecture reviewer, code reviewer, and final merge owner after review fixes pass.

## Goal

Upgrade Stage 6's single-leaf deterministic IMT fixture into a structured paper-style IMT membership prototype for the existing range path.

Stage 6 already proves that `masked_value_hash` is a leaf in a deterministic depth-10 Merkle tree selected by the RO proof. Stage 7 must make that leaf part of a canonical structured model:

- Entry leaf: represents one committed field value.
- Dataset root: commits to entries for one dataset.
- Aggregate root: commits to one or more dataset roots.
- Published IMT root: commits to the aggregate root plus deterministic padding.

This remains a **lite prototype**, not a production dynamic IMT. The purpose is to close the paper gap from "fixture coupling" toward "paper-style IMT membership prototype" without destabilizing runtime, artifact, or frontend work.

## Non-Goals

Do **not** implement any of the following in Stage 7:

- Runtime, pallet, or chain storage changes.
- On-chain Groth16 verification.
- Trustless bridge / CCMC / Merkle proof settlement.
- Verifier quorum, verifier registry, slashing, or key management.
- New constraint kinds such as subset/substr.
- Frontend UI.
- VM deployment, clean redeploy, or long-running benchmark.
- New artifact fields in `artifact.ProofArtifact`.
- Changes to `ProofDigestDomain`, `ComputeProofDigest`, JS digest calculation, or attestation payload encoding.
- A production dynamic IMT with persistence, append/delete/update operations, or multi-user indexing.

If any task appears to require one of these, stop and ask Codex/Owner.

## Design Decision

Build a deterministic structured IMT model inside `tools/data-trade-zk/internal/imt` and feed its published root/path into the existing `RootObfuscationProof`.

Keep the existing RO circuit unchanged:

- `RootObfuscationProof.Define` stays as-is.
- The RO proof still opens one leaf path against one selected public root.
- The selected public root remains `Root0`.
- `constraint_kind` remains `range`.
- `ro_depth` remains `10`.

The structural IMT model is represented in Go fixture preparation and business witness metadata. It is not encoded as new artifact fields.

## Canonical Structure

Stage 7 must define these conceptual layers:

```text
Entry leaf
  -> Dataset root
  -> Aggregate root
  -> Published root
  -> RO root list Root0..Root3
```

Required deterministic conventions:

- Entry leaf commits to:
  - `dataset_id`
  - `field_name`
  - `record_id`
  - `schema_version`
  - `masked_value_hash`
- Dataset root commits to:
  - the selected entry leaf
  - deterministic sibling entry leaves
  - `dataset_id`
  - `schema_version`
- Aggregate root commits to:
  - the dataset root
  - deterministic sibling dataset roots
- Published root commits to:
  - the aggregate root
  - deterministic padding leaves to reach depth `10`
- RO proof opens the published tree leaf containing the aggregate root.

Depths must be fixed in Stage 7:

- `entry_depth = 4`
- `dataset_depth = 4`
- `aggregate_depth = 2`
- `published_depth = 10`

All selected indices must default to `0`:

- `entry_index = 0`
- `dataset_index = 0`
- `aggregate_index = 0`
- `published_leaf_index = 0`

CodeWhale may keep Stage 6 compatibility by treating old `imt` JSON as default structured metadata with these indices.

## Files

Expected new files:

- `tools/data-trade-zk/internal/imt/structured.go`
- `tools/data-trade-zk/internal/imt/structured_test.go`

Expected modified files:

- `tools/data-trade-zk/internal/imt/schema.go`
- `tools/data-trade-zk/internal/imt/schema_test.go`
- `tools/data-trade-zk/internal/imt/proof.go`
- `tools/data-trade-zk/internal/imt/proof_test.go`
- `tools/data-trade-zk/internal/business/schema.go`
- `tools/data-trade-zk/internal/business/schema_test.go`
- `tools/data-trade-zk/internal/gnarkadapter/business_range_obfuscation.go`
- `tools/data-trade-zk/internal/gnarkadapter/business_range_obfuscation_test.go`
- `scripts/fixtures/data_trade_business_sample.json`
- `tools/data-trade-zk/README.md`
- `docs/implementation/data-trade-implementation.md`
- `docs/implementation/data-trade-paper-gap-matrix.md`
- `docs/architecture/data-trade-security-model.md`
- This plan file, Execution Record section.

Do not modify Rust, runtime, pallets, JS artifact/digest code, or frontend files in Stage 7.

## Schema Contract

Extend the IMT fixture schema conservatively.

Current Stage 6 fixture:

```json
{
  "version": 1,
  "depth": 10,
  "leaf_index": 0,
  "root_list_index": 0,
  "dataset_id": "demo-range-dataset",
  "field_name": "sensor_value"
}
```

Stage 7 should accept this shape and fill defaults for new fields.

Add fields:

```json
{
  "record_id": "demo-record-0",
  "schema_version": 1,
  "entry_depth": 4,
  "dataset_depth": 4,
  "aggregate_depth": 2,
  "published_depth": 10,
  "entry_index": 0,
  "dataset_index": 0,
  "aggregate_index": 0,
  "published_leaf_index": 0
}
```

Validation rules:

- `version == 1`
- `depth == 10` remains accepted as backward-compatible alias for `published_depth`
- Treat `depth` as deprecated alias: if `depth` is present and `published_depth` is omitted, copy `depth` into `published_depth`; if both are present and disagree, reject.
- `published_depth == 10`
- `entry_depth == 4`
- `dataset_depth == 4`
- `aggregate_depth == 2`
- `leaf_index == 0` remains accepted as backward-compatible alias for `published_leaf_index`
- Treat `leaf_index` as deprecated alias: if `leaf_index` is present and `published_leaf_index` is omitted, copy `leaf_index` into `published_leaf_index`; if both are present and disagree, reject.
- `published_leaf_index == 0`
- `root_list_index == 0`
- `entry_index == 0`
- `dataset_index == 0`
- `aggregate_index == 0`
- `schema_version == 1`
- `dataset_id`, `field_name`, and `record_id` must be non-empty ASCII

Backward compatibility requirement:

- Old witness JSON with no `imt` still defaults and validates.
- Stage 6 witness JSON with only the old IMT fields still defaults new fields and validates.
- Partial IMT JSON that explicitly sets invalid values must reject; do not silently repair explicit invalid values.

Implementation warning:

- Plain Go `int` fields cannot distinguish omitted JSON fields from explicit `0` after normal unmarshalling. CodeWhale must use a raw JSON/defaulting helper, pointer-backed decode struct, or equivalent approach so omitted Stage 7 fields default while explicitly provided invalid `0` values reject.

## Hashing Contract

Use existing MiMC helper style from `internal/imt/proof.go`.

Add domain-separated labels:

```text
FISHBONE:DATA_TRADE:IMT:ENTRY:v1
FISHBONE:DATA_TRADE:IMT:ENTRY_PAD:v1
FISHBONE:DATA_TRADE:IMT:DATASET:v1
FISHBONE:DATA_TRADE:IMT:DATASET_PAD:v1
FISHBONE:DATA_TRADE:IMT:AGGREGATE:v1
FISHBONE:DATA_TRADE:IMT:AGGREGATE_PAD:v1
FISHBONE:DATA_TRADE:IMT:PUBLISHED_PAD:v1
FISHBONE:DATA_TRADE:IMT:DECOY_ROOT:v1
```

Required properties:

- Same input fixture and `masked_value_hash` produce same structured roots and RO assignment.
- Changing `masked_value_hash` changes entry root, dataset root, aggregate root, published `Root0`, and `business_input_hash`.
- Changing `record_id` changes entry root, dataset root, aggregate root, published `Root0`, and `business_input_hash`.
- Changing `dataset_id` changes dataset root, aggregate root, published `Root0`, decoy roots, and `business_input_hash`.
- Changing `field_name` changes entry root and downstream roots.
- Decoy roots remain deterministic and metadata-derived.

Do not use random tree construction.

## Prepared Proof Contract

Keep `imt.PreparedProof` owned by `internal/imt`.

Add a new function for Stage 7:

```go
func PrepareStructuredProof(curveName string, maskedValueHash []byte, f Fixture) (PreparedProof, error)
```

Keep the existing Stage 6 `PrepareProof(...)` function available and signature-stable for legacy tests and compatibility. Do not replace it or change its meaning. `GenerateBusinessRangeFixture` must switch to `PrepareStructuredProof(...)`.

It may be extended with structured metadata for tests and documentation, for example:

```go
type PreparedProof struct {
    Leaf []byte
    Path [][]byte
    Root0 []byte
    Root1 []byte
    Root2 []byte
    Root3 []byte
    Index0 int
    Index1 int

    EntryRoot []byte
    DatasetRoot []byte
    AggregateRoot []byte
    PublishedRoot []byte
}
```

Do not introduce gnark frontend types into `internal/imt`.

`RootObfuscationProof.AssignFixture` should continue to consume `PreparedProof` without circuit redesign.

## Business Hash Contract

Update `business_input_hash` construction to include the new IMT metadata:

- existing Stage 6 business fields
- `record_id`
- `schema_version`
- `entry_depth`
- `dataset_depth`
- `aggregate_depth`
- `published_depth`
- `entry_index`
- `dataset_index`
- `aggregate_index`
- `published_leaf_index`

Keep the existing string encoding helper:

- strings: 4-byte little-endian length prefix + raw UTF-8 bytes
- integers: fixed-width little-endian

Do not change JS/Rust proof digest encoding. Only the value of `business_input_hash` may change because the hash input changes.

## Tasks

### Task 1: Extend IMT Fixture Schema

- [ ] Extend `imt.Fixture` with Stage 7 fields.
- [ ] Update `DefaultFixture()`.
- [ ] Implement defaulting for omitted Stage 7 fields while preserving explicit invalid-value rejection.
- [ ] Add a decode/default helper that can distinguish omitted fields from explicit invalid zero values.
- [ ] Add tests:
  - old Stage 6 fixture validates after defaulting.
  - omitted `imt` still defaults.
  - `record_id` empty rejects.
  - `schema_version != 1` rejects.
  - wrong fixed depths reject.
  - wrong fixed indices reject.

Validation:

```bash
go -C tools/data-trade-zk test ./internal/imt ./internal/business
```

Commit:

```bash
git add tools/data-trade-zk/internal/imt tools/data-trade-zk/internal/business scripts/fixtures/data_trade_business_sample.json
git commit -m "feat(zk): extend IMT fixture schema for structured membership"
```

### Task 2: Add Structured IMT Builder

- [ ] Create `internal/imt/structured.go`.
- [ ] Implement deterministic builders for:
  - entry leaf/root
  - dataset root
  - aggregate root
  - published root
- [ ] Reuse a small generic deterministic binary tree helper if it reduces duplication.
- [ ] Add tests:
  - same fixture produces same roots/path.
  - changing `masked_value_hash` changes all downstream roots.
  - changing `record_id` changes all downstream roots.
  - changing `dataset_id` changes dataset/aggregate/published roots.
  - changing `field_name` changes entry/downstream roots.
  - published RO path length is exactly `10`.

Validation:

```bash
go -C tools/data-trade-zk test ./internal/imt
```

Commit:

```bash
git add tools/data-trade-zk/internal/imt
git commit -m "feat(zk): build structured IMT membership fixture"
```

### Task 3: Wire Structured IMT Into Business Fixture

- [ ] Add `PrepareStructuredProof` and keep existing `PrepareProof` unchanged.
- [ ] Update `GenerateBusinessRangeFixture` to call `PrepareStructuredProof`.
- [ ] Keep `RootObfuscationProof.AssignFixture` signature stable if practical.
- [ ] Update `GenerateBusinessRangeFixture` to include new IMT metadata in `business_input_hash`.
- [ ] Add tests:
  - generated artifact verifies.
  - `business_input_hash` changes when `record_id` changes.
  - `business_input_hash` changes when `schema_version` or fixed-depth metadata changes, if those values can be varied before validation; otherwise test rejection.
  - `public_input_hash` changes when `record_id` changes because `Root0` changes.
  - do not assert full `proof_digest` determinism across runs.

Validation:

```bash
go -C tools/data-trade-zk test ./internal/gnarkadapter ./internal/imt ./internal/business ./internal/artifact
```

Commit:

```bash
git add tools/data-trade-zk/internal/gnarkadapter tools/data-trade-zk/internal/imt tools/data-trade-zk/internal/business
git commit -m "feat(zk): bind range proof to structured IMT membership"
```

### Task 4: CLI Smoke

Run:

```bash
rm -rf /tmp/fishbone-stage7-business
go -C tools/data-trade-zk run ./cmd/fishbone-zk business-fixture \
  --witness ../../scripts/fixtures/data_trade_business_sample.json \
  --out /tmp/fishbone-stage7-business \
  --session-id 0 \
  --round-index 0
go -C tools/data-trade-zk run ./cmd/fishbone-zk verify \
  --artifact /tmp/fishbone-stage7-business/artifact.json
```

Confirm:

- `artifact.json` validates.
- `business_input_hash`, `public_input_hash`, and `proof_digest` exist.
- `files.ro_public_witness` exists.

No commit required unless a fix is needed.

### Task 5: Documentation Updates

Update:

- `tools/data-trade-zk/README.md`
- `docs/implementation/data-trade-implementation.md`
- `docs/implementation/data-trade-paper-gap-matrix.md`
- `docs/architecture/data-trade-security-model.md`
- This plan's Execution Record

Required wording:

- Stage 7 implements **structured IMT membership lite** for the range path.
- It introduces Entry/Dataset/Aggregate/Published-root layers in deterministic fixture generation.
- It is still not production dynamic IMT.
- It does not implement subset/substr.
- It does not implement on-chain verification.
- It does not implement trustless bridge settlement.

Suggested gap matrix updates:

- `Root obfuscation proof`: move from `partially-supported` to `prototype-supported` if tests verify structured membership root coupling.
- `Full IMT membership`: move from `not-implemented` to `partially-supported`, with the gap "lite deterministic structured prototype, not dynamic production IMT."
- Replace stale "Stage 6 candidate" next-step wording with Stage 7+ or future-work wording.

Commit:

```bash
git add tools/data-trade-zk/README.md docs/implementation/data-trade-implementation.md docs/implementation/data-trade-paper-gap-matrix.md docs/architecture/data-trade-security-model.md docs/internal/agent-plans/2026-06-26-stage7-data-trade-structured-imt-membership-lite.md
git commit -m "docs: record structured IMT membership lite"
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

Rust is not required unless Stage 7 accidentally touches Rust. If Rust files change, stop and ask Codex before proceeding.

## Stop Conditions

CodeWhale must stop and ask Codex before:

- Adding fields to `artifact.ProofArtifact`.
- Changing `ProofDigestDomain` or `ComputeProofDigest`.
- Changing JS digest calculation.
- Changing Rust pallets/runtime.
- Changing `constraint_kind` away from `range`.
- Supporting dynamic depth/index values beyond the fixed Stage 7 constants.
- Adding subset/substr constraints.
- Adding frontend UI.
- Running VM deployment or clean redeploy.
- Adding external dependencies.
- Claiming production dynamic IMT.

## Acceptance Criteria

- Stage 6 witness JSON remains backward-compatible.
- Default fixture JSON includes Stage 7 structured metadata.
- Business fixture uses structured IMT roots by default.
- `Root0` changes when `record_id`, `dataset_id`, `field_name`, or `masked_value_hash` changes.
- `business_input_hash` changes when Stage 7 IMT metadata changes.
- `fishbone-zk verify` accepts the generated artifact.
- No Rust/runtime/artifact schema/proof digest encoding changes.
- Documentation clearly states Stage 7 is structured IMT membership lite, not production IMT.

## Execution Record

### 2026-06-26 CodeWhale Stage 7 Execution Complete

- Branch: `feat/data-trade-stage7-structured-imt`
- Commits:
  - `ca2f304 feat(zk): extend IMT fixture schema for structured membership`
  - `373550d feat(zk): build structured IMT membership fixture`
  - `1d40c87 feat(zk): bind range proof to structured IMT membership`
- Tasks completed: Task 1-6 (all)
- Tests run:
  - Go: `go test ./...` — `internal/imt` (33 tests incl. 7 new structured), `internal/business` (7), `internal/gnarkadapter` (10), `internal/artifact` (6) — all passed
  - JS: `node --check` on `zk_real_data_trade_flow.js`, `zk_artifact.js`, `zk_verifier_client.js`, `zk_attestation.js` — all passed
- CLI smoke: `fishbone-zk business-fixture` + `fishbone-zk verify` — `accepted`
- Deviations from plan:
  - `buildDeterministicTree` kept for intermediate layers; `buildPublishedRoot` does in-place path extraction (needed because tree nodes are overwritten during compression)
  - `Leaf` field in `PreparedProof` carries aggregate root (published tree leaf), not masked_value_hash
  - `AssignFixture` uses keccak-mapped `Convert2Byte` for all path/root values — this is the proven Stage 6 path that correctly matches the gnark circuit's field-element representation
  - `depth` and `leaf_index` deprecated aliases handled via `defaultFromRaw` with conflict rejection
- Questions for Codex/Owner: none
- Remaining risks: none — Stage 7 is Go-only, no Rust/JS/artifact schema changes
