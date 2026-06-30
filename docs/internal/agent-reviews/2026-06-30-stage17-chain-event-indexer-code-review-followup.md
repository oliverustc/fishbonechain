# Stage 17 Code Review Follow-up: Chain Event Indexer

Date: 2026-06-30
Reviewer: Codex
Plan: `docs/internal/agent-plans/2026-06-30-stage17-chain-event-indexer-state-sync.md`
Prior code review: `docs/internal/agent-reviews/2026-06-30-stage17-chain-event-indexer-code-review.md`
Branch: `stage/stage17-chain-event-indexer`
Reviewed head: `1b636d6`

## Scope Reviewed

- Review-fix commit `9a4e87b`
- Formatting cleanup commit `1b636d6`
- `scripts/chain_event_indexer.js`
- `scripts/lib/chain_event_normalizer.js`
- `scripts/lib/data_trade_state_sync.js`
- `scripts/lib/data_trade_evidence_correlation.js`
- Stage 17 plan Execution Record

## Inputs Read

- `agent.md`
- `docs/internal/agent-collaboration.md`
- `.agents/skills/fwf/references/workflow-common.md`
- `.agents/skills/fwf/references/code-review-prompt.md`
- `docs/internal/agent-plans/2026-06-30-stage17-chain-event-indexer-state-sync.md`
- `docs/internal/agent-reviews/2026-06-30-stage17-chain-event-indexer-code-review.md`
- Relevant implementation files listed above
- `git status --short --branch`
- `git diff --stat main...HEAD`
- `git diff --check main...HEAD`

## Findings

No required issues remain.

The prior required fixes were addressed:

- Live-shaped camelCase event fields now derive state and correlate evidence. `chain_event_normalizer.js` canonicalizes known Polkadot.js field names, while `data_trade_state_sync.js` and `data_trade_evidence_correlation.js` also dual-read snake_case and camelCase fields.
- `scan --chain` now rejects invalid values before creating scan outputs.

## Required Fixes

None.

## Accepted Risks

- Live RPC scan validation was not run because endpoints were unavailable in this environment. No live indexing claim is made. The review covered live-shaped field-name behavior with no-RPC camelCase events.
- The canonicalization map may need updates if future pallet events introduce additional field names.

## Verification Performed

- `node --check scripts/chain_event_indexer.js`
- `node --check scripts/lib/chain_event_normalizer.js`
- `node --check scripts/lib/data_trade_state_sync.js`
- `node --check scripts/lib/data_trade_evidence_correlation.js`
- `node --check scripts/data_trade_cli.js`
- `node --check scripts/zk_real_data_trade_flow.js`
- `bash -n scripts/run_data_trade_validation.sh`
- `node scripts/chain_event_indexer.js inspect mappings --out .agents/fwf/runs/stage17/review2/inspect-mappings.json`
- `node scripts/chain_event_indexer.js replay --events scripts/fixtures/chain_events/data_trade_sample_events.jsonl --out .agents/fwf/runs/stage17/review2/replay-fixture`
- `node scripts/chain_event_indexer.js state --events scripts/fixtures/chain_events/data_trade_sample_events.jsonl --out .agents/fwf/runs/stage17/review2/state-fixture`
- `node scripts/chain_event_indexer.js inspect sample-evidence --kind dry-run --out .agents/fwf/runs/stage17/review2/sample-dry-run-evidence.json`
- `node scripts/chain_event_indexer.js correlate-evidence --events scripts/fixtures/chain_events/data_trade_sample_events.jsonl --evidence .agents/fwf/runs/stage17/review2/sample-dry-run-evidence.json --out .agents/fwf/runs/stage17/review2/correlate-dry-run`
- `node scripts/chain_event_indexer.js scan --chain nope --from 1 --to 1 --out .agents/fwf/runs/stage17/review2/invalid-chain` returned nonzero as expected.
- No-RPC camelCase event regression: derived `listing=active`, `session=settled`, `escrow=settled`, and live evidence correlation returned `matched` with 7 events.
- `git diff --check main...HEAD`
- `rg -n "chain-event-indexer-state-sync" docs/README.md docs/architecture/platform-business-model.md docs/implementation/data-trade-cli-api-boundary.md docs/implementation/data-trade-implementation.md`

## Branch And Commit Assessment

- Branch is correctly named `stage/stage17-chain-event-indexer`.
- Implementation commit `dc08a4f` and review-fix commit `9a4e87b` are scoped to Stage 17.
- Process commits reference the plan and review records.
- Working tree is clean except ignored local validation artifacts.

## Decision

`approved`
