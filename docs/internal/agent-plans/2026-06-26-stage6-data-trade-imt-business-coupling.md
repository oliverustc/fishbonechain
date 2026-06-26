# Stage 6 Data Trade IMT Business Coupling Implementation Plan

> **Execution owner:** CodeWhale.
>
> **Codex role:** plan author, architecture reviewer, and code reviewer. CodeWhale should execute this plan task by task and stop at the listed stop conditions instead of inventing a broader design.

## Goal

Upgrade the current range business proof path so the range business witness is linked to a deterministic IMT/root-obfuscation fixture.

Current Stage 2.2 proves:

- `raw_value in [min_value, max_value]`
- `masked_value = raw_value + mask_delta`
- `masked_value_hash = MiMC(masked_value, salt)`

But the current RO proof still uses a random Merkle tree unrelated to the business witness. Stage 6 must make the RO proof demonstrate that `masked_value_hash` is a leaf under a deterministic committed root in the proof artifact.

## Non-Goals

Do **not** implement any of the following in Stage 6:

- On-chain Groth16 verification.
- Runtime or pallet changes.
- Proof digest encoding changes.
- Attestation payload encoding changes.
- Trustless bridge / CCMC / Merkle proof settlement.
- Verifier quorum or verifier set management.
- Subset/substr constraints.
- Full production IMT with multiple datasets, attributes, dynamic records, or persistence.
- VM deployment or clean redeploy.

If any task appears to require one of these, stop and ask Codex/Owner.

## Design Decision

Implement a **minimal canonical IMT fixture** for the existing range path:

- `masked_value_hash` is the committed leaf.
- The fixture builds a deterministic binary Merkle tree of depth `10`.
- Leaf `0` is `masked_value_hash`.
- All other leaves are deterministic padding leaves derived from the fixture fields.
- The RO proof opens leaf `0`.
- The public root list has 4 roots:
  - `Root0`: actual deterministic IMT root.
  - `Root1..Root3`: deterministic decoy roots derived from fixture fields.
- `Index0=0`, `Index1=0`, so the circuit selects `Root0`.
- The artifact remains `constraint_kind = "range"` and `ro_depth = 10`.
- Existing `ProofArtifact` fields and digest encoding remain unchanged.

This is intentionally narrower than the full paper IMT. It is enough to upgrade the prototype claim from "range business witness is proven" to "range business witness is proven and linked to an endorsed deterministic root fixture."

## Files

Expected new files:

- `tools/data-trade-zk/internal/imt/schema.go`
- `tools/data-trade-zk/internal/imt/schema_test.go`

Expected modified files:

- `tools/data-trade-zk/internal/business/schema.go`
- `tools/data-trade-zk/internal/gnarkadapter/root_obfuscation.go`
- `tools/data-trade-zk/internal/gnarkadapter/business_range_obfuscation.go`
- `tools/data-trade-zk/internal/gnarkadapter/business_range_obfuscation_test.go`
- `tools/data-trade-zk/cmd/fishbone-zk/main.go`
- `tools/data-trade-zk/README.md`
- `scripts/fixtures/data_trade_business_sample.json`
- `docs/implementation/data-trade-implementation.md`
- `docs/implementation/data-trade-paper-gap-matrix.md`
- `docs/architecture/data-trade-security-model.md`
- This plan file, Execution Record section.

Do not modify Rust pallets or runtime files in this stage.

## Implementation Contract

### IMT Fixture Schema

Add package `internal/imt`.

Define a schema with these JSON fields:

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

Rules:

- `version` must be `1`.
- `depth` must be exactly `10` in Stage 6.
- `leaf_index` must be exactly `0` in Stage 6.
- `root_list_index` must be exactly `0` in Stage 6.
- `dataset_id` must be non-empty ASCII.
- `field_name` must be non-empty ASCII.

Add to `business.RangeWitness`:

```go
IMT imt.Fixture `json:"imt"`
```

Backward compatibility:

- If `imt` is omitted, `ReadRangeWitness` should fill the default fixture above.
- This keeps existing fixture JSON usable.

### Deterministic Hashing

Use the existing MiMC helper style already used by `BusinessRangeProof` and `RootObfuscationProof`.

Define deterministic padding leaves and decoy roots using domain-separated strings:

```text
FISHBONE:DATA_TRADE:IMT:PAD:v1
FISHBONE:DATA_TRADE:IMT:DECOY_ROOT:v1
```

CodeWhale may implement this by converting strings with `mimchash.Convert2Byte(...)` and hashing with `mimchash.MiMCHash(...)`, matching existing code style.

Required deterministic behavior:

- The same `RangeWitness` must produce the same IMT root, Merkle path, RO public assignment values, and `business_input_hash` across repeated runs.
- Do not require full `proof_digest` equality across repeated Groth16 fixture generation unless the existing proving path is already deterministic; proof bytes and setup/proving randomness may legitimately change the digest.
- Changing `raw_value`, `mask_delta`, `salt`, `masked_value_hash`, `dataset_id`, or `field_name` must change either `Root0`, the RO public assignment, or `business_input_hash`.

### RO Circuit Assignment

Do not redesign `RootObfuscationProof.Define`.

Add a deterministic assignment path, for example:

```go
func (rop *RootObfuscationProof) AssignFixture(curveName string, fixture imt.PreparedProof)
```

Where `imt.PreparedProof` can contain:

- `Leaf`
- `Path []`
- `Root0`
- `Root1`
- `Root2`
- `Root3`
- `Index0`
- `Index1`

`PreparedProof` must be defined in `internal/imt`, not in `internal/gnarkadapter`. Keep it independent of gnark frontend types where practical, so `internal/imt` remains self-contained and unit-testable.

Keep the old random `Assign(...)` for legacy Stage 1 fixture compatibility.

`GenerateBusinessRangeFixture` must use deterministic `AssignFixture`, not random `Assign`.

### Artifact Semantics

Do not add fields to `artifact.ProofArtifact` unless Codex explicitly approves.

Stage 6 should bind IMT through existing artifact fields:

- RO public witness changes because the deterministic root list changes.
- `public_input_hash` changes because RO public witness changes.
- `proof_digest` changes because `public_input_hash` changes.
- `business_input_hash` should also include IMT fixture metadata, so changing `dataset_id` or `field_name` changes digest even if proof files are not inspected.

Update business input hash construction in `GenerateBusinessRangeFixture` to include:

- existing values: `raw_value`, `min_value`, `max_value`, `mask_delta`, `salt`, `masked_value_hash`
- new values: `dataset_id`, `field_name`, `depth`, `leaf_index`, `root_list_index`

Use explicit byte encoding:

- strings: raw UTF-8 bytes prefixed by a 4-byte little-endian length
- integers: fixed-width little-endian

Add a unit-level test for the string encoding convention. At minimum, assert that encoding `"demo"` produces:

```text
04 00 00 00 64 65 6d 6f
```

Do not change Rust/JS proof digest encoding. Only the value of `business_input_hash` changes.

## Tasks

### Task 1: Add IMT Fixture Schema

- [ ] Create `tools/data-trade-zk/internal/imt/schema.go`.
- [ ] Create `tools/data-trade-zk/internal/imt/schema_test.go`.
- [ ] Implement validation and default fixture.
- [ ] Add tests:
  - default fixture validates.
  - `depth != 10` rejects.
  - `leaf_index != 0` rejects.
  - empty `dataset_id` rejects.
  - empty `field_name` rejects.

Validation:

```bash
go -C tools/data-trade-zk test ./internal/imt
```

Commit:

```bash
git add tools/data-trade-zk/internal/imt
git commit -m "feat(zk): add deterministic IMT fixture schema"
```

### Task 2: Extend Business Witness Schema

- [ ] Add `IMT imt.Fixture` to `business.RangeWitness`.
- [ ] Update `ReadRangeWitness` so omitted `imt` gets the default fixture.
- [ ] Update `Validate` to validate `IMT`.
- [ ] Update `scripts/fixtures/data_trade_business_sample.json` to include the default `imt`.
- [ ] Add/update tests in `tools/data-trade-zk/internal/business/schema_test.go`:
  - fixture without `imt` still reads and defaults.
  - invalid `imt.depth` rejects.
  - `imt.dataset_id` affects canonical metadata later in Task 4 or mark test pending until Task 4.

Validation:

```bash
go -C tools/data-trade-zk test ./internal/business ./internal/imt
```

Commit:

```bash
git add tools/data-trade-zk/internal/business tools/data-trade-zk/internal/imt scripts/fixtures/data_trade_business_sample.json
git commit -m "feat(zk): attach IMT fixture to business witness"
```

### Task 3: Build Deterministic IMT Proof Helper

- [ ] In `internal/imt`, add a helper that prepares the deterministic depth-10 proof from:
  - curve name
  - `masked_value_hash`
  - validated fixture metadata
- [ ] The helper should return prepared values for `RootObfuscationProof.AssignFixture`.
- [ ] Use leaf `0 = masked_value_hash`.
- [ ] Generate deterministic padding leaves for remaining proof path.
- [ ] Generate deterministic decoy roots.
- [ ] Add tests:
  - same input produces same root/path.
  - changing `masked_value_hash` changes `Root0`.
  - changing `dataset_id` changes `Root0` or decoy roots.
  - proof path length is exactly `10`.

Implementation note:

- Do not depend on random `merkletree.RandConstruct`.
- If using `big.Int` or `frontend.Variable`, keep package boundaries simple. It is acceptable for `internal/imt` to return `[]byte`/`*big.Int` values and let `gnarkadapter` convert to `frontend.Variable`.

Validation:

```bash
go -C tools/data-trade-zk test ./internal/imt
```

Commit:

```bash
git add tools/data-trade-zk/internal/imt
git commit -m "feat(zk): derive deterministic IMT range proof inputs"
```

### Task 4: Use Deterministic IMT in Business Fixture

- [ ] Add `AssignFixture` to `RootObfuscationProof`.
- [ ] Update `GenerateBusinessRangeFixture` to:
  - compute/validate `masked_value_hash` as today.
  - prepare deterministic IMT proof using `w.IMT`.
  - call `roCircuit.AssignFixture(...)`.
  - stop using `roCircuit.Assign(curveName, 10)` in the business fixture path.
- [ ] Keep `GenerateRangeROFixture` unchanged for legacy random fixture.
- [ ] Update `business_input_hash` construction to include IMT fixture metadata.
- [ ] Add tests in `business_range_obfuscation_test.go`:
  - generated artifact verifies.
  - repeated generation with the same witness produces the same `business_input_hash`; if full proof bytes are not deterministic due to setup randomness, do not assert identical `proof_digest`.
  - changing `imt.dataset_id` changes `business_input_hash`.
  - string encoding is byte-exact; `"demo"` must encode as `04 00 00 00 64 65 6d 6f`.
  - changing `masked_value_hash` or a tampered fixture still rejects.

Validation:

```bash
go -C tools/data-trade-zk test ./internal/gnarkadapter ./internal/business ./internal/imt ./internal/artifact
```

Commit:

```bash
git add tools/data-trade-zk/internal/gnarkadapter tools/data-trade-zk/internal/business tools/data-trade-zk/internal/imt
git commit -m "feat(zk): bind business range proof to deterministic IMT root"
```

### Task 5: CLI and Artifact Smoke

- [ ] Run:

```bash
rm -rf /tmp/fishbone-stage6-business
go -C tools/data-trade-zk run ./cmd/fishbone-zk business-fixture \
  --witness ../../scripts/fixtures/data_trade_business_sample.json \
  --out /tmp/fishbone-stage6-business \
  --session-id 0 \
  --round-index 0
go -C tools/data-trade-zk run ./cmd/fishbone-zk verify \
  --artifact /tmp/fishbone-stage6-business/artifact.json
```

- [ ] Inspect `/tmp/fishbone-stage6-business/artifact.json`.
- [ ] Confirm:
  - `business_input_hash` exists.
  - `public_input_hash` exists.
  - `proof_digest` exists.
  - `files.ro_public_witness` exists.

No commit required unless code/docs changed during smoke fix.

### Task 6: JS Flow Compatibility

Because artifact schema should not change, JS should continue to work without code changes.

- [ ] Run:

```bash
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js
```

- [ ] If JS code needs changes, stop and ask Codex before changing artifact encoding.

### Task 7: Documentation Updates

Update:

- `tools/data-trade-zk/README.md`
- `docs/implementation/data-trade-implementation.md`
- `docs/implementation/data-trade-paper-gap-matrix.md`
- `docs/architecture/data-trade-security-model.md`
- This plan's Execution Record

Required wording:

- Stage 6 implements a **deterministic IMT fixture coupling** for the range path.
- It does **not** implement full production IMT.
- It does **not** implement on-chain proof verification.
- It does **not** implement trustless bridge settlement.
- It does **not** implement subset/substr.

Commit:

```bash
git add tools/data-trade-zk/README.md docs/implementation/data-trade-implementation.md docs/implementation/data-trade-paper-gap-matrix.md docs/architecture/data-trade-security-model.md docs/internal/agent-plans/2026-06-26-stage6-data-trade-imt-business-coupling.md
git commit -m "docs: record deterministic IMT business coupling"
```

### Task 8: Final Validation

Run all:

```bash
go -C tools/data-trade-zk test ./...
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session
```

Rust should still pass because Stage 6 must not change Rust. If Rust fails, stop and ask Codex unless the failure is clearly unrelated environment flakiness.

## Stop Conditions

CodeWhale must stop and ask Codex before:

- Adding fields to `artifact.ProofArtifact`.
- Changing `ProofDigestDomain` or `ComputeProofDigest`.
- Changing JS digest calculation.
- Changing Rust pallets/runtime.
- Changing `constraint_kind` away from `range`.
- Supporting `ro_depth` other than `10`.
- Making the IMT fixture random.
- Claiming full production IMT.
- Running destructive VM deployment or clean redeploy.
- Adding external dependencies.

## Acceptance Criteria

- Existing `business-fixture` still works with old witness JSON that omits `imt`.
- Updated default fixture JSON includes `imt`.
- Business fixture RO proof is deterministic with respect to the witness fixture, not random.
- Artifact verification passes with `fishbone-zk verify`.
- `business_input_hash` changes when IMT metadata changes.
- Documentation clearly states Stage 6 is deterministic fixture coupling, not full production IMT.
- No Rust/runtime/proof-digest encoding changes.

## Execution Record

### 2026-06-26 CodeWhale Stage 6 Execution Complete

- Branch: `feat/data-trade-stage6-imt-coupling`
- Commits:
  - `503ec9d feat(zk): add deterministic IMT fixture schema`
  - `9ac5ecc feat(zk): attach IMT fixture to business witness`
  - `bc10266 feat(zk): derive deterministic IMT range proof inputs`
  - `8424384 feat(zk): bind business range proof to deterministic IMT root`
  - `4e1982c docs: record deterministic IMT business coupling`
- Tasks completed: Task 1-8 (all)
- Tests run:
  - Go: `go -C tools/data-trade-zk test ./...` — `internal/imt` (14 tests), `internal/business` (7 tests), `internal/gnarkadapter` (9 tests, incl. 3 new), `internal/artifact` (6 tests) — all passed
  - JS: `node --check` on `zk_real_data_trade_flow.js`, `zk_artifact.js`, `zk_verifier_client.js`, `zk_attestation.js` — all passed
  - Rust: `SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session` — 19 passed
- CLI smoke: `fishbone-zk business-fixture` + `fishbone-zk verify` — `accepted`
- Deviations from plan:
  - Decoy roots derivation now uses `dataset_id` + `field_name` in domain string (plan review finding #1 — ensures fixture metadata changes observable)
  - `Validate()` auto-fills default IMT for backward compatibility; `ReadRangeWitness` detects missing `"imt"` JSON key via `json.RawMessage`
  - `PreparedProof` type placed in `internal/imt` per plan review recommendation
- Questions for Codex/Owner: none
- Remaining risks: none — Stage 6 is Go-only, no Rust/JS/artifact encoding changes
