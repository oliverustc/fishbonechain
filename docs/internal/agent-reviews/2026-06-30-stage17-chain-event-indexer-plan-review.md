# Stage 17 Plan Review: Chain Event Indexer And State Sync

Date: 2026-06-30
Reviewer: opencode
Plan: `docs/internal/agent-plans/2026-06-30-stage17-chain-event-indexer-state-sync.md`
Decision: **approved-with-required-fixes**

## Scope Reviewed

Entire plan: goal, scope, non-goals, current facts, event mapping baseline, state derivation baseline, risks, stop conditions, task list, acceptance criteria, validation commands, and documentation updates.

## Inputs Read

- `agent.md`
- `docs/internal/agent-collaboration.md`
- `docs/internal/agent-plans/2026-06-30-stage17-chain-event-indexer-state-sync.md`
- `docs/internal/agent-plans/2026-06-28-data-flow-platform-long-term-roadmap.md`
- `docs/architecture/platform-business-model.md`
- `docs/implementation/data-trade-cli-api-boundary.md`
- `scripts/platform-model/types.ts`
- `scripts/profiles/chains.json`
- `scripts/lib/trade_profile.js`
- `scripts/lib/data_trade_events.js`
- `scripts/bridges/data_trade.js`
- `package.json`
- `pallets/data-registry/src/lib.rs` (events: DataPublished, ImtRootUpdated, ListingStatusChanged)
- `pallets/trade-session/src/lib.rs` (events: all 13 variants claimed)
- `pallets/main-escrow/src/lib.rs` (events: all 5 variants claimed)
- `docs/implementation/data-trade-implementation.md`
- `docs/implementation/data-trade-stage14-evidence-index.md`
- `git status --short --branch` (branch: `stage/stage17-chain-event-indexer`, clean)

## Findings

### 1. Verified Current Facts

All plan factual claims are backed by repository evidence:

- `ChainEvent` schema in `docs/architecture/platform-business-model.md:185-202` matches the plan's normalized event schema (event_id, chain_id, block_number, block_hash, extrinsic_index, event_index, pallet, variant, fields, cursor, ingested_at) ✓
- `scripts/platform-model/types.ts:220-232` contains JSDoc type draft for ChainEvent with matching fields ✓
- `docs/implementation/data-trade-cli-api-boundary.md` exists as Stage 16 output ✓
- `scripts/profiles/chains.json` defines `child6-data-trade` and `child7-business-trade` with `main_ws`, `child_ws`, `settlement_mode`, `verifier_mode`, `verifier_authority`, `proof` config ✓
- `scripts/lib/trade_profile.js` loads and validates trade profiles from `scripts/profiles/chains.json` ✓
- `scripts/lib/data_trade_events.js` extracts events from tx results only; not a block scanner or durable indexer ✓
- `scripts/bridges/data_trade.js` subscribes to live child-chain events but does not persist normalized events, cursors, or derived state ✓
- `package.json` uses ES modules (`"type": "module"`) and has `@polkadot/api` as dev dependency ✓
- Pallet events verified in source code match plan's event mapping baseline exactly:
  - `pallet-data-registry`: `DataPublished`, `ImtRootUpdated`, `ListingStatusChanged` ✓
  - `pallet-trade-session`: `SessionCreated`, `SessionAccepted`, `RoundOpened`, `PaymentProofSubmitted`, `DataProofSubmitted`, `DataProofAttested`, `ProofSignatureSubmitted`, `DataDelivered`, `PaymentPreimageSubmitted`, `RoundCompleted`, `SettlementClaimed`, `SessionPunished`, `LastPaymentClaimed` ✓
  - `pallet-main-escrow`: `EscrowOpened`, `FundsLocked`, `DepositLocked`, `EscrowSettled`, `EscrowPunished` ✓
- Stage 14 evidence schema (`docs/implementation/data-trade-stage14-evidence-index.md:119-137`) confirms `listing_id`, `escrow_id`, `session_id` are nullable integers, matching the plan's correlation risk note about null IDs for dry-run ✓
- Branch `stage/stage17-chain-event-indexer` exists, working tree clean ✓

### 2. Strengths

- Goal is concrete ("build a reusable, file-backed chain event indexer") with clear deliverables ✓
- Scope is precisely bounded with explicit allowed changes and a strong non-goals list (no backend, no DB, no server, no protocol changes, no new deps) ✓
- Stop conditions cover protocol, proof, settlement, metrics, deployment, and trust boundaries comprehensively ✓
- Risks are well-categorized (security, data integrity, compatibility, deployment, performance, evidence-correlation, paper-facing) with explicit mitigations ✓
- Task list is ordered and executable — 19 tasks with enough detail for implementation without architecture invention ✓
- Event mapping baseline is verified against pallet source code ✓
- State derivation baseline is concrete with required fields per derived record type and source event hints ✓
- No-RPC fixture/replay validation pathway exists for when live chains are unavailable ✓
- Evidence correlation correctly handles null IDs for dry-run/no-chain evidence ✓

### 3. Required Fixes

**F1. Fixture file ambiguity** — The validation commands reference `scripts/fixtures/chain_events/data_trade_sample_events.jsonl` as an input fixture. The task list says "Prefer a small committed fixture under `scripts/fixtures/chain_events/` if it is stable and minimal; otherwise generate fixture files under `.agents/fwf/runs/stage17/fixtures/` during validation." The validation commands implicitly require the committed fixture to exist before validation can run, but no task explicitly creates it. Fix: add a task to create and commit the fixture file before it is referenced in validation, or change the validation commands to use a generated fixture path.

**F2. `scan` vs `state.json` inconsistency** — The optional live scan validation block (line 289-298) checks for `state.json` at the scan output. The `scan` task (line 194) only says "append/write `events.jsonl`, and update `cursor.json`." State derivation is described as `replay` or `state` subcommand territory (lines 195-196). These are inconsistent. Fix: either add `state.json` production to the `scan` task description, or remove `state.json` from the live scan validation check (and run `state` as a follow-up step).

**F3. `inspect mappings` subcommand unspecified** — The failidstion commands include `inspect mappings --out ...` but the task list describes `inspect` in general terms ("list supported event mappings", "print cursor summary", "print event/state counts"). No task explicitly defines the `mappings` sub-mode or specifies its output format. Fix: add an explicit task for the `inspect mappings` mode describing what the output should contain.

**F4. Missing task for `sample-dry-run-evidence.json`** — The correlation validation references `.agents/fwf/runs/stage17/sample-dry-run-evidence.json` as input. This file does not exist and must be created before the correlation test can run. The plan says "If it generates `sample-dry-run-evidence.json` during validation, the command that creates it must be recorded." But generating it is not a listed task. Fix: add an explicit task to create a minimal dry-run evidence sample file before the correlation validation step.

**F5. `chain_role` field is underspecified** — Task line 192 defines normalized event fields including `chain_role`, but the plan never explains what values it takes, how it differs from `chain_id`, or where it comes from. The `ChainEvent` schema in `docs/architecture/platform-business-model.md:185-202` does not include `chain_role`. Fix: define `chain_role` semantics and values, or drop it from the normalized schema.

### 4. Suggested Improvements

- Consider adding a task to verify that `@polkadot/api`'s `api.query.system.events.at(blockHash)` returns events with metadata containing named fields (Polkadot.js v16+ should have this, but it would be good to confirm during implementation).
- The `replay` subcommand is defined as reproducing `state.json`, `summary.json`, and `summary.md` from events, but the `state` subcommand also produces `state.json`. Consider whether these should be distinct or whether `replay` should delegate to `state` for the state derivation step. The task list treats them independently (line 195-197), which is fine, but the overlap could be clarified.
- Consider whether `--include-all` (line 110) should be a subcommand flag or a separate concerns. The current text mentions it only for `scan`, but it could be relevant for `replay` filtering as well.
- The plan could benefit from a small note about the expected JSONL format: whether each line is a JSON object with no trailing comma, and whether the file should end with a newline. Minor but avoids format guesswork.

### 5. Risks If Unchanged

- F1/F2/F3/F4 result in validation commands that cannot pass as written because referenced files or subcommands don't match tasks. The implementation agent would need to deviate from the plan during validation.
- F5 could cause the implementation agent to make up `chain_role` semantics, potentially diverging from future platform intent.
- Even with fixes, the plan has enough detail that a competent implementation agent should succeed. No protocol, security, or data integrity risks arise from the plan itself.

### 6. Questions for Codex/Owner

- Should `scripts/fixtures/chain_events/data_trade_sample_events.jsonl` be created from real chain data (requiring live RPC) or as a hand-crafted fixture with synthetic events?
- Is `chain_role` intended to be something like `"main"` / `"child"` derived from the chain profile, or a different concept? The `ChainEvent` in the platform model doesn't include it.
