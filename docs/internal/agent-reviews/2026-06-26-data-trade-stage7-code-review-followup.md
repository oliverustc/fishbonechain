# Data Trade Stage 7 Code Review Follow-up

## Scope

Reviewed commit after requested fixes:

- `0b4a2bd fix: address Stage 7 code review findings`

Previous review:

- `docs/internal/agent-reviews/2026-06-26-data-trade-stage7-code-review.md`
- Previous decision: `changes requested`

## Decision

`approved`

The requested Stage 7 fixes have been applied. Stage 7 is ready to merge to `main`.

## Verification

### Finding 1: Deprecated alias defaulting

Status: fixed.

- `defaultFromRaw` now tracks whether `published_depth` and `published_leaf_index` were populated from deprecated aliases.
- Alias-derived values are no longer overwritten by defaults.
- Added tests:
  - `depth: 8` with omitted `published_depth` rejects.
  - `leaf_index: 1` with omitted `published_leaf_index` rejects.
  - `depth: 10` with omitted `published_depth` accepts.
  - `leaf_index: 0` with omitted `published_leaf_index` accepts.

### Finding 2: Stale Stage 6 candidate wording

Status: fixed.

- `docs/implementation/data-trade-paper-gap-matrix.md` no longer uses stale `Stage 6 candidate` or `Stage 6/7 candidate` wording for post-IMT items.

## Validation Run By Codex

```bash
go -C tools/data-trade-zk test ./...
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js
rg -n "Stage 6 candidate|Stage 6/7 candidate" docs/implementation/data-trade-paper-gap-matrix.md docs/implementation/data-trade-implementation.md docs/architecture/data-trade-security-model.md tools/data-trade-zk/README.md
```

Result:

- Go tests passed.
- JS syntax checks passed.
- `rg` found no stale Stage 6 candidate wording.

## Remaining Notes

- Stage 7 remains structured IMT membership lite, not production dynamic IMT.
- No Rust/runtime changes were introduced.
- No artifact schema or proof digest encoding changes were introduced.
