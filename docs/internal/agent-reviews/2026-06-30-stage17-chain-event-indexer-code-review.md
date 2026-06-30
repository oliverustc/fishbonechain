# Stage 17 Code Review: Chain Event Indexer And State Sync

Date: 2026-06-30
Reviewer: Codex
Plan: `docs/internal/agent-plans/2026-06-30-stage17-chain-event-indexer-state-sync.md`
Branch: `stage/stage17-chain-event-indexer`
Reviewed head: `dc08a4f`

## Scope Reviewed

- `scripts/chain_event_indexer.js`
- `scripts/lib/chain_event_normalizer.js`
- `scripts/lib/data_trade_state_sync.js`
- `scripts/lib/data_trade_evidence_correlation.js`
- `scripts/fixtures/chain_events/data_trade_sample_events.jsonl`
- `docs/implementation/chain-event-indexer-state-sync.md`
- Formal docs updated by Stage 17
- Stage 17 plan Execution Record

## Inputs Read

- `agent.md`
- `docs/internal/agent-collaboration.md`
- `.agents/skills/fwf/references/workflow-common.md`
- `.agents/skills/fwf/references/code-review-prompt.md`
- `docs/internal/agent-plans/2026-06-30-stage17-chain-event-indexer-state-sync.md`
- `docs/implementation/chain-event-indexer-state-sync.md`
- `git status --short --branch`
- `git diff --stat main...HEAD`
- `git diff --name-only main...HEAD`
- Relevant scripts and helper modules listed above

## Findings

### Required: Live-scanned events do not derive state or correlate with evidence because field names are not canonicalized

`scripts/lib/chain_event_normalizer.js:94` preserves Polkadot.js metadata field names as-is. Existing repository code reads the live data-trade event fields as camelCase, for example `listingId`, `escrowId`, `sessionId`, `roundIndex`, and `remainingRounds` in `scripts/zk_real_data_trade_flow.js:197`, `scripts/zk_real_data_trade_flow.js:203`, `scripts/zk_real_data_trade_flow.js:218`, and `scripts/bridges/data_trade.js:126`.

The new state and correlation helpers only read snake_case field names: `listing_id`, `session_id`, `escrow_id`, and `round_index` in `scripts/lib/data_trade_state_sync.js:35`, `scripts/lib/data_trade_state_sync.js:39`, `scripts/lib/data_trade_state_sync.js:43`, and `scripts/lib/data_trade_evidence_correlation.js:94`.

The committed fixture uses snake_case fields, so fixture validation passes, but real `scan` output from Polkadot.js will not populate listings, sessions, or escrows and will not match live evidence by IDs. I reproduced this with camelCase normalized events: `deriveState()` returned empty `listings`, `sessions`, and `escrows`, and `correlate()` returned `partial` for evidence containing matching IDs.

Required fix: canonicalize baseline event field names during normalization, or make state/correlation helpers accept both Polkadot.js camelCase and fixture/evidence snake_case names. Then add a no-RPC validation case using camelCase event fields so fixture validation covers live-scan-shaped records.

### Required: `scan --chain` accepts invalid values and exits successfully without scanning

`scripts/chain_event_indexer.js:220` reads `--chain` but never validates it against `main`, `child`, or `both`. The conditionals at `scripts/chain_event_indexer.js:251` and `scripts/chain_event_indexer.js:286` only run for recognized values, so an invalid value such as `--chain nope` connects no APIs, writes an empty `cursor.json`, reports success, and exits `0`.

This violates the planned CLI contract (`--chain main|child|both`) and can create false validation evidence because a typo looks like a successful scan.

Required fix: reject any `--chain` value outside `main`, `child`, and `both` with a nonzero exit before creating output files. Add a validation command such as `node scripts/chain_event_indexer.js scan --chain nope ...; test "$?" -ne 0`.

## Required Fixes

- Canonicalize or dual-read live Polkadot.js event field names so scanned events can drive state derivation and evidence correlation.
- Add no-RPC regression coverage for camelCase live-shaped event fields.
- Validate `--chain` values and fail closed on invalid values.

## Accepted Risks

- Live RPC scan validation was not run; RPC endpoints were unavailable in this environment. This remains acceptable only after the field-name fix is covered by no-RPC live-shaped fixtures.
- The indexer remains file-backed and not a backend/database service, per plan.

## Verification Performed

- `node --check scripts/chain_event_indexer.js`
- `node --check scripts/lib/chain_event_normalizer.js`
- `node --check scripts/lib/data_trade_state_sync.js`
- `node --check scripts/lib/data_trade_evidence_correlation.js`
- `node --check scripts/data_trade_cli.js`
- `node --check scripts/zk_real_data_trade_flow.js`
- `bash -n scripts/run_data_trade_validation.sh`
- `node scripts/chain_event_indexer.js --help`
- `node scripts/chain_event_indexer.js scan --help`
- `node scripts/chain_event_indexer.js replay --help`
- `node scripts/chain_event_indexer.js state --help`
- `node scripts/chain_event_indexer.js correlate-evidence --help`
- `node scripts/chain_event_indexer.js inspect --help`
- `node scripts/data_trade_cli.js --help`
- `git diff --check main...HEAD`
- `rg -n "chain-event-indexer-state-sync" docs/README.md docs/architecture/platform-business-model.md docs/implementation/data-trade-cli-api-boundary.md docs/implementation/data-trade-implementation.md`
- `node scripts/chain_event_indexer.js inspect mappings --out .agents/fwf/runs/stage17/review/inspect-mappings.json`
- `node scripts/chain_event_indexer.js replay --events scripts/fixtures/chain_events/data_trade_sample_events.jsonl --out .agents/fwf/runs/stage17/review/replay-fixture`
- `node scripts/chain_event_indexer.js state --events scripts/fixtures/chain_events/data_trade_sample_events.jsonl --out .agents/fwf/runs/stage17/review/state-fixture`
- `node scripts/chain_event_indexer.js inspect sample-evidence --kind dry-run --out .agents/fwf/runs/stage17/review/sample-dry-run-evidence.json`
- `node scripts/chain_event_indexer.js correlate-evidence --events scripts/fixtures/chain_events/data_trade_sample_events.jsonl --evidence .agents/fwf/runs/stage17/review/sample-dry-run-evidence.json --out .agents/fwf/runs/stage17/review/correlate-dry-run`
- `node scripts/chain_event_indexer.js inspect counts --events scripts/fixtures/chain_events/data_trade_sample_events.jsonl`
- Manual no-RPC reproduction of camelCase event fields against `deriveState()` and `correlate()`
- Manual check that `node scripts/chain_event_indexer.js scan --chain nope --from 1 --to 1 --out .agents/fwf/runs/stage17/review/invalid-chain` exits `0` and writes an empty cursor

## Branch And Commit Assessment

- Branch is correctly named `stage/stage17-chain-event-indexer`.
- Implementation commit `dc08a4f` is scoped to the Stage 17 indexer and documentation.
- Plan Execution Record was updated with validation and skipped live-RPC rationale.
- Generated review validation artifacts are under `.agents/fwf/runs/stage17/review/` and are not committed.

## Decision

`approved-with-required-fixes`

