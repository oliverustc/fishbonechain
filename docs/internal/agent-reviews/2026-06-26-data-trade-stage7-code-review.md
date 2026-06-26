# Data Trade Stage 7 Code Review

## Scope

Reviewed branch: `feat/data-trade-stage7-structured-imt`

Reviewed commits after Stage 7 plan review:

- `ca2f304 feat(zk): extend IMT fixture schema for structured membership`
- `373550d feat(zk): build structured IMT membership fixture`
- `1d40c87 feat(zk): bind range proof to structured IMT membership`
- `9cdafaf docs: record structured IMT membership lite`

## Decision

`changes requested`

The structured IMT implementation is directionally correct and tests pass, but one schema defaulting bug violates the Stage 7 plan's explicit alias/defaulting contract. There is also one documentation cleanup item.

## Findings

### 1. Medium: Deprecated alias defaulting can silently accept invalid explicit values

File: `tools/data-trade-zk/internal/imt/schema.go`

Lines: `136-174`

The Stage 7 plan requires:

- If `depth` is present and `published_depth` is omitted, copy `depth` into `published_depth`.
- If both are present and disagree, reject.
- Explicit invalid values must reject rather than being silently repaired.

Current `defaultFromRaw` copies `depth` into `PublishedDepth`, but then later defaults `PublishedDepth` again when `published_depth` is omitted:

```go
if has("depth") && !has("published_depth") {
    f.PublishedDepth = f.Depth
}
...
if !has("published_depth") {
    f.PublishedDepth = def.PublishedDepth
}
```

As a result, JSON like this is incorrectly accepted:

```json
{
  "version": 1,
  "depth": 8,
  "root_list_index": 0,
  "dataset_id": "test",
  "field_name": "f"
}
```

The same issue exists for `leaf_index` / `published_leaf_index`: an explicit invalid old alias can be overwritten by the default canonical field.

Required fix:

- If old alias is present and new field is omitted, copy the alias and do not later overwrite the canonical field with defaults.
- Add tests for:
  - `depth: 8` with omitted `published_depth` rejects.
  - `leaf_index: 1` with omitted `published_leaf_index` rejects.
  - `depth: 10` with omitted `published_depth` still accepts.
  - `leaf_index: 0` with omitted `published_leaf_index` still accepts.

### 2. Low: Gap matrix still contains stale Stage 6 candidate wording

File: `docs/implementation/data-trade-paper-gap-matrix.md`

Lines: `33-37`

The Stage 7 documentation task explicitly says to replace stale "Stage 6 candidate" next-step wording. Several rows still say:

- `Stage 6/7 candidate after range+IMT`
- `Stage 6 candidate, high risk`
- `Stage 6 candidate: threshold attestation`
- `Stage 6 candidate: CCMC/Merkle proof settlement`

Required fix:

- Replace these with Stage 8+ or future-work wording, for example:
  - `Stage 8+ candidate`
  - `Future hardening`
  - `Post-IMT security hardening`

## Validation Run By Codex

```bash
go -C tools/data-trade-zk test ./...
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js
```

Result: passed.

## Notes

- `PrepareStructuredProof(...)` was added while keeping `PrepareProof(...)` available.
- `GenerateBusinessRangeFixture` now calls `PrepareStructuredProof(...)`.
- The four-layer Entry/Dataset/Aggregate/Published-root model is implemented in `internal/imt/structured.go`.
- No Rust/runtime changes were introduced.
- No artifact schema or proof digest encoding changes were introduced.
