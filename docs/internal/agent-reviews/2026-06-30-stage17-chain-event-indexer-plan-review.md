# Stage 17 Plan Review: Chain Event Indexer And State Sync

Date: 2026-06-30
Reviewer: opencode (re-review after plan revision)
Plan: `docs/internal/agent-plans/2026-06-30-stage17-chain-event-indexer-state-sync.md`
Decision: **approved**

## Scope Reviewed

Complete revised plan including the new Normalized Event Format section, Plan Review Resolution section, and all updated task and validation command blocks. Compared against commit `16bb21f` which resolves all five required fixes from the initial plan review (`b964467`).

## Inputs Read

- `agent.md`
- `docs/internal/agent-collaboration.md`
- `docs/internal/agent-plans/2026-06-30-stage17-chain-event-indexer-state-sync.md` (revised)
- `docs/internal/agent-plans/2026-06-28-data-flow-platform-long-term-roadmap.md`
- `docs/architecture/platform-business-model.md`
- `docs/implementation/data-trade-cli-api-boundary.md`
- `docs/implementation/data-trade-stage14-evidence-index.md`
- `docs/implementation/data-trade-implementation.md`
- `scripts/platform-model/types.ts`
- `scripts/profiles/chains.json`
- `scripts/lib/trade_profile.js`
- `scripts/lib/data_trade_events.js`
- `scripts/bridges/data_trade.js`
- `package.json`
- `pallets/data-registry/src/lib.rs` (events: DataPublished, ImtRootUpdated, ListingStatusChanged)
- `pallets/trade-session/src/lib.rs` (all 13 variants verified)
- `pallets/main-escrow/src/lib.rs` (all 5 variants verified)
- `git diff b964467..16bb21f` (plan revision delta)

## Findings

### 1. Resolution of Previous Required Fixes

| Fix | Status | Evidence |
|-----|--------|----------|
| F1 — fixture file ambiguity | Resolved | Line 54 adds committed fixture to scope; line 238 adds explicit task; validation path fixed at `scripts/fixtures/chain_events/data_trade_sample_events.jsonl` |
| F2 — scan vs state.json inconsistency | Resolved | Line 227: "scan does not have to write state.json"; live scan validation (lines 337-340) runs separate `state` command |
| F3 — inspect mappings unspecified | Resolved | Line 231: explicit schema with `version`, `generated_at`, `supported_events.child`, `supported_events.main`, `state_derivation_events` |
| F4 — missing sample-dry-run-evidence task | Resolved | Line 239 adds explicit task; lines 311-314 add `inspect sample-evidence --kind dry-run` validation step |
| F5 — chain_role underspecified | Resolved | Lines 79-80 define as `main`/`child` script-local metadata; lines 142, 225 reiterate it is not a platform `ChainEvent` field |

### 2. Improvements Accepted

- JSONL format specification added (lines 114-142): newline-delimited, no array wrapper, no trailing commas, EOF newline ✓
- Replay/state delegation clarified (line 228): "replay may delegate the state derivation logic to the same helper used by `state`" ✓

### 3. Plan Review Resolution Section

The new section (lines 390-413) accurately records the review feedback cycle with required fixes, accepted suggestions, and rejected suggestions. It correctly states that implementation should proceed after re-review approval.

### 4. Current Facts

All current-fact claims from the original plan remain verifiable. The new `chain_role` fact (line 79-80) is internally consistent — it positions `chain_role` as a Stage 17 indexer convenience field, not an extension of the Stage 15 platform model. This is the correct boundary.

### 5. Scope, Non-Goals, Stop Conditions, Risks

Unchanged from the original plan. All remain sufficient and well-grounded in repository evidence.

### 6. Task List

21 tasks, all executable without architecture invention:
- New tasks for fixture creation (line 238) and evidence sample generation (line 239) are properly scoped
- Inspect subcommands are concrete with output specifications
- JSONL format constraint is explicit

### 7. Validation Commands

All commands are now internally consistent with the task list:
- Fixture path is unambiguous (line 323 refers to the committed path)
- Scan validation no longer checks for state.json at scan output
- Evidence sample is generated before correlation
- Live scan state is validated via a separate `state` command

### 8. Minor Observation (not a blocker)

The validation command `node scripts/chain_event_indexer.js inspect sample-evidence --kind dry-run` (line 311) names a subcommand `sample-evidence` under `inspect`. The inspect task (lines 230-233) does not explicitly list `sample-evidence` alongside `mappings`, cursor summary, and counts. The implementation agent can resolve this from the validation command as specification — it does not block execution.

### 9. Acceptance Criteria

All seven criteria remain measurable and verifiable. No backend/server/database/protocol changes are permitted.

## Verification Performed

- `git status --short` — clean working tree
- `git branch --show-current` — `stage/stage17-chain-event-indexer`
- `git diff b964467..16bb21f` — verified all F1-F5 fixes are applied
- Manual inspection of all 435 lines of the revised plan against repository evidence
