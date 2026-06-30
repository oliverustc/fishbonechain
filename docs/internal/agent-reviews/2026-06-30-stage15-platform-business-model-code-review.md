# Stage 15 Code Review: Platform Business Model

- **Date**: 2026-06-30
- **Branch**: `stage/stage15-platform-business-model`
- **Plan reviewed**: `docs/internal/agent-plans/2026-06-29-stage15-platform-business-model.md`
- **Implementation commit reviewed**: `8c324ee docs(platform): define stage15 business model`
- **Reviewer**: Codex

## Scope Reviewed

Reviewed the Stage 15 implementation against the plan and plan-review resolution:

- `docs/architecture/platform-business-model.md`
- `scripts/platform-model/types.ts`
- `docs/README.md`
- `docs/implementation/data-trade-stage14-evidence-index.md`
- Latest Execution Record in `docs/internal/agent-plans/2026-06-29-stage15-platform-business-model.md`

## Findings

### High: Required type-draft validation is omitted and currently fails

- **File**: `docs/internal/agent-plans/2026-06-29-stage15-platform-business-model.md:127-133`, `docs/internal/agent-plans/2026-06-29-stage15-platform-business-model.md:285-290`, `scripts/platform-model/types.ts:8`
- **Issue**: The revised plan made `node --check scripts/platform-model/types.ts` a required validation command. The implementation did not record it as run successfully; it records the command as failed and treats that as an expected deviation. Re-running it confirms the failure:

```text
scripts/platform-model/types.ts:8
type HexHash = string;
     ^^^^^^^

SyntaxError: Unexpected identifier 'HexHash'
```

- **Impact**: The stage cannot be approved with a known failing required validation command. The plan's validation design was wrong for TypeScript-only syntax, but the implementation must either make the artifact satisfy the required check or update the plan in the same review-fix pass with a correct auditable validation command, then run that command.
- **Required fix**: Choose one scoped fix:
  - convert `scripts/platform-model/types.ts` into syntax that passes the plan's required validation while preserving the type draft intent; or
  - revise the plan's validation command to use an available TypeScript parser/checker without adding dependencies, or explicitly replace `node --check scripts/platform-model/types.ts` with an auditable command that matches the chosen artifact. Then update the Execution Record with the corrected command and result.

### Medium: Formal doc has a broken relative link to the type draft

- **File**: `docs/architecture/platform-business-model.md:378`
- **Issue**: The link `[Type draft](../scripts/platform-model/types.ts)` is wrong from `docs/architecture/`; it resolves to `docs/scripts/platform-model/types.ts`, which does not exist. The actual file is `scripts/platform-model/types.ts`, so the relative link should go up two levels.
- **Impact**: The formal architecture document points readers to a non-existent file, weakening the Stage 15 deliverable and docs index.
- **Required fix**: Change the link target to `../../scripts/platform-model/types.ts` and verify it with `test -f scripts/platform-model/types.ts` plus a direct inspection of the rendered/relative target.

### Medium: `on_chain_bound` description overclaims on-chain proof verification

- **File**: `docs/architecture/platform-business-model.md:175`
- **Issue**: The table says `on_chain_bound` means "Whether proof digest was verified on-chain". This conflicts with the same document's limitation section and project guardrails: the chain binds and checks proof digest/attestation metadata, but does not verify Groth16 proofs on-chain.
- **Impact**: This is a paper/security claim risk. It can be read as stronger verification than the prototype provides.
- **Required fix**: Reword the field as digest/metadata binding, for example "Whether the proof digest was bound into accepted chain state/events", and keep the "not on-chain Groth16 verification" limitation intact.

### Low: `chain_listing_id` type is inconsistent between doc and type draft

- **File**: `docs/architecture/platform-business-model.md:90`, `scripts/platform-model/types.ts:124`
- **Issue**: The architecture doc describes `DataAsset.chain_listing_id` as `ChainId | null`, but the type draft correctly uses `number | null`. A listing ID is not a chain ID.
- **Impact**: This can confuse later backend/event-indexer implementation.
- **Required fix**: Change the doc type to `number | null`.

## Required Changes

- Fix the required type-draft validation mismatch and update the Execution Record with successful, auditable validation.
- Fix the broken type-draft link in `docs/architecture/platform-business-model.md`.
- Reword `on_chain_bound` to avoid implying on-chain Groth16 verification.
- Correct `DataAsset.chain_listing_id` type in the formal doc.

## Accepted Risks

- This stage remains docs/schema design only; no backend, database, API, chain, proof, settlement, deployment, or experiment behavior is implemented.
- The type draft may remain dependency-free and backend-neutral, as the plan requires.
- No JSON Schema artifact is required for Stage 15.

## Verification Performed

```bash
git status --short --branch
git branch --show-current
git diff --stat main...HEAD
git diff --name-status main...HEAD
nl -ba docs/architecture/platform-business-model.md | sed -n '1,440p'
nl -ba scripts/platform-model/types.ts | sed -n '1,260p'
nl -ba docs/README.md | sed -n '1,60p'
nl -ba docs/implementation/data-trade-stage14-evidence-index.md | sed -n '130,165p'
test -f docs/architecture/platform-business-model.md
test -f scripts/platform-model/types.ts
rg -n "platform-business-model" docs/README.md
rg -n "proof_digest|business_input_hash|vk_hash|public_input_hash|on_chain_bound" docs/architecture/platform-business-model.md
rg -n "data-trade-security-model" docs/architecture/platform-business-model.md
rg -c '^\s*(import|require)\s' scripts/platform-model/types.ts || true
node --check scripts/platform-model/types.ts
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/data_trade_validation_summary.js
git diff --check main...HEAD
```

Observed:

- `node --check scripts/platform-model/types.ts` failed with `SyntaxError: Unexpected identifier 'HexHash'`.
- `node --check scripts/zk_real_data_trade_flow.js` passed.
- `node --check scripts/lib/data_trade_validation_summary.js` passed.
- `git diff --check main...HEAD` passed.
- Required files exist and `docs/README.md` indexes the new formal model.

## Branch And Commit Assessment

- Current branch is `stage/stage15-platform-business-model`.
- Stage branch contains plan, plan review, plan fix, and implementation commits on top of `main`.
- Implementation commit references the plan and validation, but omits the failing required type-draft validation command from the commit message.
- Working tree was clean before writing this review record.

## Decision

`approved-with-required-fixes`

Do not merge to `main` yet. The required fixes above are scoped and suitable for an opencode review-fix pass.
