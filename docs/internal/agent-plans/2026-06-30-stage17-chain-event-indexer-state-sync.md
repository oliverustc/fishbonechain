# Stage 17 Plan: Chain Event Indexer And State Sync

Date: 2026-06-30
Stage: Stage 17
Roadmap: `docs/internal/agent-plans/2026-06-28-data-flow-platform-long-term-roadmap.md`
Branch: `stage/stage17-chain-event-indexer`

## Goal

Build a reusable, file-backed chain event indexer and state-sync foundation that can scan FishboneChain main/data-trade child chain events into normalized records, maintain resumable cursors, derive data-trade listing/session/escrow state snapshots, and correlate indexed events with existing evidence files without introducing a Web backend, database, protocol change, or new dependency.

## Background

Roadmap Stage 17 targets chain event indexing and state synchronization as future Web backend infrastructure. Required capabilities are multi-chain RPC configuration, event scanning, cursor saving, event normalization, listing/session/escrow state parsing, evidence-to-event correlation, and replay scanning. The roadmap explicitly allows this stage to begin as a script or lightweight service and does not require a complete Web backend.

Current repository facts verified while writing this plan:

- `docs/architecture/platform-business-model.md` defines `ChainEvent` as a normalized platform object with `chain_id`, `block_number`, `block_hash`, `extrinsic_index`, `event_index`, `pallet`, `variant`, `fields`, `cursor`, and `ingested_at`. It states chain events are a recoverable state source and backend records are indexing/orchestration metadata, not protocol finality.
- `scripts/platform-model/types.ts` contains dependency-free JSDoc type drafts for `ChainEvent`, `Evidence`, `EvidenceChainEventRef`, `DataAsset`, `WorkflowRun`, and related Stage 15 platform objects.
- `docs/implementation/data-trade-cli-api-boundary.md` defines the Stage 16 command boundary and evidence fields. It keeps independently chain-mutating subcommands planned/non-executable and preserves `run-flow` as the only full-flow transaction wrapper.
- `scripts/profiles/chains.json` defines `child6-data-trade` and `child7-business-trade` trade profiles with `main_ws`, `child_ws`, settlement mode, verifier mode, verifier authority, and proof config.
- `scripts/lib/trade_profile.js` already loads and validates trade profiles from `scripts/profiles/chains.json`.
- `scripts/lib/data_trade_events.js` only extracts events from transaction results; it is not a block scanner or durable indexer.
- `scripts/bridges/data_trade.js` can subscribe to live child-chain `tradeSession` events and optionally coordinate main-chain actions in dev mode, but it does not persist normalized events, cursors, replay windows, or derived platform state.
- `package.json` already uses ES modules and has `@polkadot/api` as a dev dependency. Stage 17 should not add npm dependencies.
- Runtime event sources verified in pallets:
  - `pallet-data-registry`: `DataPublished`, `ImtRootUpdated`, `ListingStatusChanged`.
  - `pallet-trade-session`: `SessionCreated`, `SessionAccepted`, `RoundOpened`, `PaymentProofSubmitted`, `DataProofSubmitted`, `DataProofAttested`, `ProofSignatureSubmitted`, `DataDelivered`, `PaymentPreimageSubmitted`, `RoundCompleted`, `SettlementClaimed`, `SessionPunished`, `LastPaymentClaimed`.
  - `pallet-main-escrow`: `EscrowOpened`, `FundsLocked`, `DepositLocked`, `EscrowSettled`, `EscrowPunished`.
- Existing Stage 14 validation summary records data-trade evidence metadata, including `listing_id`, `escrow_id`, `session_id`, `scenario_outcome.events`, per-scenario `log_path`, and `evidence_path`, but it does not index chain blocks.

## Scope

Allowed changes:

- Add a new script entrypoint, preferably `scripts/chain_event_indexer.js`, with explicit subcommands for:
  - `scan`: scan one or both chains over a bounded block range and write normalized chain events.
  - `replay`: rebuild normalized events and derived state from a stored raw/normalized event file.
  - `state`: derive or print data-trade state snapshots from indexed events.
  - `correlate-evidence`: correlate evidence JSON/summary files with indexed events by `listing_id`, `session_id`, `escrow_id`, and event names where available.
  - `inspect`: inspect cursor, event count, state summary, or supported event mappings without RPC.
- Add focused helper modules under `scripts/lib/` when useful, such as:
  - `chain_event_indexer.js` or `chain_event_normalizer.js` for normalization and field serialization.
  - `data_trade_state_sync.js` for deriving listing/session/escrow status from normalized events.
  - `data_trade_evidence_correlation.js` for evidence-to-event matching.
- Use file-backed outputs under a user-provided `--out <dir>`:
  - `events.jsonl` for normalized `ChainEvent` records.
  - `cursor.json` for resumable scan position by chain.
  - `state.json` for derived listing/session/escrow state.
  - `correlations.json` for evidence-event links.
  - `summary.json` and `summary.md` for human-readable validation.
- Support profile-based RPC configuration using `--profile child6-data-trade` and explicit overrides `--main`, `--child`.
- Support no-RPC fixture/replay validation using committed small fixture files or generated local fixtures under `.agents/fwf/runs/stage17/...`.
- Add a formal documentation page, preferably `docs/implementation/chain-event-indexer-state-sync.md`, describing indexer scope, normalized schema, cursor semantics, event mappings, state derivation rules, evidence correlation, and current limitations.
- Update `docs/README.md`, `docs/architecture/platform-business-model.md`, and data-trade implementation docs with forward references only where they describe current Stage 17 behavior.

## Non-Goals

- No Web backend, HTTP server, REST/RPC API, database schema, migrations, authentication, frontend, or long-running production daemon.
- No pallet/runtime/extrinsic/event changes.
- No protocol changes, proof digest changes, verifier changes, settlement changes, bridge trust changes, deployment topology changes, or chain reset.
- No private-key handling, signing, transaction submission, or backend custody.
- No new npm, Rust, Go, Python, or system dependencies.
- No paper-facing metric or conclusion changes.
- No claim that event indexing makes cross-chain settlement trustless. The current bridge/session-escrow binding remains off-chain coordination.
- No requirement that live RPC be available as the only validation path.

## Current Facts

- The platform model already treats `ChainEvent` as infrastructure for reconstruction and correlation, but no implementation writes durable `ChainEvent` records yet.
- Data-trade chain state spans at least two chains in current profiles:
  - Child data-trade chain: `dataRegistry` and `tradeSession` events.
  - Main chain: `mainEscrow` events.
- `@polkadot/api` can read historical block hashes and events with `api.rpc.chain.getBlockHash(blockNumber)` and `api.query.system.events.at(blockHash)`. This is the expected scan mechanism.
- Event field names are available from metadata at runtime when decoding events. The indexer should prefer named fields when present and fall back to positional names only when metadata does not expose field names.
- The repository already has Stage 14 evidence and Stage 16 CLI boundary outputs; Stage 17 should correlate with those artifacts rather than rewriting them.
- Because scan outputs can be large and environment-specific, generated events, cursors, state snapshots, and correlations belong under `.agents/fwf/runs/stage17/...` during validation and must not be committed unless a deliberately tiny fixture is added for no-RPC tests.

## Event Mapping Baseline

The initial indexer must normalize at least these data-trade events:

### Child Chain

- `dataRegistry.DataPublished`
- `dataRegistry.ImtRootUpdated`
- `dataRegistry.ListingStatusChanged`
- `tradeSession.SessionCreated`
- `tradeSession.SessionAccepted`
- `tradeSession.RoundOpened`
- `tradeSession.PaymentProofSubmitted`
- `tradeSession.DataProofSubmitted`
- `tradeSession.DataProofAttested`
- `tradeSession.ProofSignatureSubmitted`
- `tradeSession.DataDelivered`
- `tradeSession.PaymentPreimageSubmitted`
- `tradeSession.RoundCompleted`
- `tradeSession.SettlementClaimed`
- `tradeSession.SessionPunished`
- `tradeSession.LastPaymentClaimed`

### Main Chain

- `mainEscrow.EscrowOpened`
- `mainEscrow.FundsLocked`
- `mainEscrow.DepositLocked`
- `mainEscrow.EscrowSettled`
- `mainEscrow.EscrowPunished`

Other events may be recorded when `--include-all` is set, but the state derivation and summaries should focus on the baseline mapping above.

## State Derivation Baseline

Implement state derivation as replay over normalized events, not as hidden DB mutation.

Required derived records:

- `listings[listing_id]`
  - `owner`, `price_per_round`, `max_rounds`, `status`, `last_event_id`.
  - `DataPublished` creates/activates a listing.
  - `ListingStatusChanged` updates listing status.
  - `ImtRootUpdated` records latest IMT root.
- `sessions[session_id]`
  - `requester`, `data_owner`, `listing_id`, `escrow_id`, `status`, `rounds`, `last_event_id`.
  - `SessionCreated` creates pending/created session state.
  - `SessionAccepted` marks active.
  - Round events update per-round status in event order.
  - `SettlementClaimed` marks settlement claimed on child chain.
  - `SessionPunished` marks punished.
  - `LastPaymentClaimed` records last-payment path.
- `escrows[escrow_id]`
  - `requester`, `data_owner`, `status`, `funds_locked`, `deposit_locked`, `paid_rounds`, `refunded`, `slashed_deposit`, `last_event_id`.
  - `EscrowOpened` creates opened state.
  - `FundsLocked` marks funded amount.
  - `DepositLocked` marks ready.
  - `EscrowSettled` marks settled.
  - `EscrowPunished` marks punished.

The state snapshot must keep source event references so later backend code can audit how each derived status was reached.

## Risks

- Security risk: indexer output can look authoritative. Documentation and output fields must state that indexed state is a replay/cache of chain events and not a substitute for chain finality.
- Data integrity risk: cross-chain event order is not globally ordered. The plan must avoid deriving main/child causality from wall-clock ordering alone; correlation should use explicit IDs (`session_id`, `escrow_id`, `listing_id`) and chain IDs.
- Compatibility risk: event metadata field names can differ across runtime versions. Normalization should include raw `event.toHuman()` or JSON-compatible raw fields for audit, plus a stable normalized field map.
- Deployment risk: live RPC may be unavailable. Required validation must include no-RPC fixture/replay tests and optional bounded live scans only when endpoints are reachable.
- Performance risk: full-chain scans can be slow. Required CLI must support bounded `--from`/`--to`, `--max-blocks`, and clear refusal or warning for unbounded live scans.
- Evidence-correlation risk: dry-run evidence has null chain IDs; correlation should mark no-chain evidence as `not_applicable` rather than inventing event links.
- Paper-facing risk: do not change experiment metrics or claim stronger guarantees from indexing.

## Stop Conditions

Implementation agent must stop and ask Codex/Owner before:

- changing pallet events, runtime metadata, extrinsics, storage layouts, proof digest fields, verifier assumptions, settlement rules, or bridge trust assumptions;
- adding a database, backend server, REST/RPC API, frontend, auth system, or new dependency;
- introducing transaction submission or signer/private-key handling in the indexer;
- changing Stage 14 evidence schema, scenario IDs, expected result strings, experiment metrics, or paper-facing conclusions;
- making live RPC mutation or chain reset part of validation;
- deleting or rewriting existing validation evidence, generated experiment data, or deployment artifacts;
- claiming trustless cross-chain event ordering or finality beyond what the current chains provide.

## Branch And Commit Plan

- Continue on branch `stage/stage17-chain-event-indexer`.
- Commit only after:
  - the indexer script and helper modules pass syntax checks;
  - fixture/replay validation writes normalized events, cursor, derived state, and summary under `.agents/fwf/runs/stage17/...`;
  - evidence correlation is validated against at least one no-chain/dry-run evidence file and correctly reports non-applicable chain links when IDs are null;
  - formal docs are updated and indexed;
  - this plan's Execution Record is updated with exact commands and results.
- Recommended implementation commit message:

```text
feat(indexer): add chain event state sync foundation

Plan: docs/internal/agent-plans/2026-06-30-stage17-chain-event-indexer-state-sync.md
Validation:
- <commands run>
```

## Task List

- [ ] Re-read required inputs: `agent.md`, `docs/internal/agent-collaboration.md`, this plan, roadmap Stage 17, `docs/architecture/platform-business-model.md`, `docs/implementation/data-trade-cli-api-boundary.md`, `scripts/platform-model/types.ts`, `scripts/profiles/chains.json`, `scripts/lib/trade_profile.js`, `scripts/bridges/data_trade.js`, and relevant pallet event definitions.
- [ ] Add `scripts/chain_event_indexer.js` with discoverable `--help` and subcommand help for `scan`, `replay`, `state`, `correlate-evidence`, and `inspect`.
- [ ] Implement profile/RPC option parsing:
  - `--profile <id>` loads `main_ws` and `child_ws` from `scripts/profiles/chains.json`;
  - `--main <ws>` and `--child <ws>` override profile endpoints;
  - `--chain main|child|both` selects scan target;
  - `--from <block>`, `--to <block>`, and `--max-blocks <n>` bound live scans.
- [ ] Implement normalized event record generation with stable fields:
  - `event_id`, `chain_id`, `chain_role`, `profile`, `block_number`, `block_hash`, `extrinsic_index`, `event_index`, `pallet`, `variant`, `fields`, `cursor`, `ingested_at`, and optional `raw`.
- [ ] Ensure event field serialization is JSON-compatible for `BN`, `u128`, account IDs, hashes, bools, enums, vectors, and nulls.
- [ ] Implement `scan` to read historical events with `@polkadot/api`, filter to the baseline data-trade event mapping by default, append/write `events.jsonl`, and update `cursor.json`.
- [ ] Implement `replay --events <events.jsonl> --out <dir>` to rebuild `state.json`, `summary.json`, and `summary.md` without RPC.
- [ ] Implement `state --events <events.jsonl> --out <dir>` or equivalent to derive listing/session/escrow snapshots from normalized events.
- [ ] Implement `inspect` modes that do not require RPC:
  - list supported event mappings;
  - print cursor summary from `cursor.json`;
  - print event/state counts from generated files.
- [ ] Implement `correlate-evidence --events <events.jsonl> --evidence <path-or-summary> --out <dir>`:
  - match live-chain evidence by `listing_id`, `session_id`, `escrow_id`, and expected event names where present;
  - mark dry-run/no-chain evidence as `not_applicable` when IDs are null;
  - write `correlations.json` and summary.
- [ ] Add no-RPC fixture support. Prefer a small committed fixture under `scripts/fixtures/chain_events/` if it is stable and minimal; otherwise generate fixture files under `.agents/fwf/runs/stage17/fixtures/` during validation and do not commit generated outputs.
- [ ] Add formal documentation `docs/implementation/chain-event-indexer-state-sync.md` covering:
  - commands and examples;
  - normalized `ChainEvent` schema;
  - cursor semantics;
  - event mapping baseline;
  - state derivation rules;
  - evidence correlation behavior;
  - live scan limitations and replay validation flow;
  - explicit non-goals and trust boundaries.
- [ ] Update `docs/README.md` to index the new formal documentation.
- [ ] Add a forward reference from `docs/architecture/platform-business-model.md` to the Stage 17 indexer document in the `ChainEvent` section.
- [ ] Add a short forward reference from `docs/implementation/data-trade-implementation.md` or `docs/implementation/data-trade-cli-api-boundary.md` if needed to point data-trade operators to the indexer.
- [ ] Do not alter Stage 14 validation scripts or Stage 16 CLI behavior except to read their outputs for correlation.
- [ ] Update this plan's Execution Record with commits, files changed, exact validation commands, skipped validations, deviations, and remaining risks.

## Acceptance Criteria

- `scripts/chain_event_indexer.js` exists and exposes help for all Stage 17 subcommands.
- The indexer can produce normalized `ChainEvent` JSONL records from a fixture or bounded live scan.
- Cursor output records per-chain scan progress and can be inspected without RPC.
- Replay/state derivation from normalized events produces listing/session/escrow snapshots with source event references.
- Evidence correlation handles both:
  - no-chain/dry-run evidence by reporting `not_applicable`;
  - fixture or live events by linking matching IDs/events when available.
- Formal docs describe current behavior and limitations, and `docs/README.md` indexes the new document.
- No backend/server/database/auth/frontend/dependency/protocol/runtime/settlement/proof changes are made.
- Validation evidence is stored under `.agents/fwf/runs/stage17/...` and not committed.

## Validation Commands

Run these at minimum:

```bash
git status --short --branch
test -f scripts/chain_event_indexer.js
test -f docs/implementation/chain-event-indexer-state-sync.md
rg -n "chain-event-indexer-state-sync" docs/README.md docs/architecture/platform-business-model.md
node --check scripts/chain_event_indexer.js
node --check scripts/lib/chain_event_normalizer.js
node --check scripts/lib/data_trade_state_sync.js
node --check scripts/lib/data_trade_evidence_correlation.js
node scripts/chain_event_indexer.js --help
node scripts/chain_event_indexer.js scan --help
node scripts/chain_event_indexer.js replay --help
node scripts/chain_event_indexer.js state --help
node scripts/chain_event_indexer.js correlate-evidence --help
node scripts/chain_event_indexer.js inspect --help
```

If a helper module listed above is not created because the implementation keeps the logic in fewer files, replace its `node --check` command with the actual helper file(s) created and record the deviation in the Execution Record.

Required no-RPC fixture/replay validation:

```bash
node scripts/chain_event_indexer.js inspect mappings \
  --out .agents/fwf/runs/stage17/inspect-mappings.json
test -f .agents/fwf/runs/stage17/inspect-mappings.json

node scripts/chain_event_indexer.js replay \
  --events scripts/fixtures/chain_events/data_trade_sample_events.jsonl \
  --out .agents/fwf/runs/stage17/replay-fixture
test -f .agents/fwf/runs/stage17/replay-fixture/events.jsonl
test -f .agents/fwf/runs/stage17/replay-fixture/state.json
test -f .agents/fwf/runs/stage17/replay-fixture/summary.json
test -f .agents/fwf/runs/stage17/replay-fixture/summary.md

node scripts/chain_event_indexer.js state \
  --events .agents/fwf/runs/stage17/replay-fixture/events.jsonl \
  --out .agents/fwf/runs/stage17/state-fixture
test -f .agents/fwf/runs/stage17/state-fixture/state.json

node scripts/chain_event_indexer.js correlate-evidence \
  --events .agents/fwf/runs/stage17/replay-fixture/events.jsonl \
  --evidence .agents/fwf/runs/stage17/sample-dry-run-evidence.json \
  --out .agents/fwf/runs/stage17/correlate-dry-run
test -f .agents/fwf/runs/stage17/correlate-dry-run/correlations.json
```

The implementation may choose a different fixture file path, but it must be repo-local and committed if referenced by required validation. If it generates `sample-dry-run-evidence.json` during validation, the command that creates it must be recorded and the file must remain uncommitted.

Optional bounded live scan validation, only if RPC readiness is available:

```bash
node scripts/chain_event_indexer.js scan \
  --profile child6-data-trade \
  --chain both \
  --from <known-safe-start-block> \
  --to <known-safe-end-block> \
  --out .agents/fwf/runs/stage17/live-scan
test -f .agents/fwf/runs/stage17/live-scan/events.jsonl
test -f .agents/fwf/runs/stage17/live-scan/cursor.json
test -f .agents/fwf/runs/stage17/live-scan/state.json
```

If live RPC is unavailable, record the exact reason and do not claim live indexing validation.

Regression/compatibility checks:

```bash
node --check scripts/data_trade_cli.js
node --check scripts/zk_real_data_trade_flow.js
bash -n scripts/run_data_trade_validation.sh
node scripts/data_trade_cli.js --help
```

## Validation Output Paths

Use repo-local ignored output paths:

```text
.agents/fwf/runs/stage17/
```

Do not write validation output to `/tmp` or commit generated outputs under `.agents/fwf/runs/stage17/`.

## Documentation Updates

Required:

- `docs/implementation/chain-event-indexer-state-sync.md`
- `docs/README.md`
- `docs/architecture/platform-business-model.md` forward reference in or near the `ChainEvent` section

Expected forward reference if useful:

- `docs/implementation/data-trade-implementation.md`
- `docs/implementation/data-trade-cli-api-boundary.md`

Do not alter experiment conclusions, measured results, security claims, or paper-facing statements except to add an indexer pointer and clarify that indexed state is a replay/cache of chain events.

## Plan-Review Focus

Ask opencode plan review to focus on:

- whether the Stage 17 scope is narrow enough to avoid accidentally building a backend/database/server;
- whether the normalized `ChainEvent` schema and cursor semantics are concrete enough for implementation;
- whether the state derivation rules cover the current data-trade listing/session/escrow events without inventing cross-chain finality;
- whether fixture/replay validation is sufficient when live RPC is unavailable;
- whether evidence correlation requirements avoid false links for dry-run/no-chain evidence;
- whether the plan contains enough guardrails against changing protocol, proof, settlement, metrics, or deployment topology.

## Execution Record

### 2026-06-30 Codex Plan Authoring

- Branch: `stage/stage17-chain-event-indexer`
- Commits:
  - Pending.
- Tasks completed:
  - Created Stage 17 branch from local `main`.
  - Authored initial Stage 17 plan from the long-term roadmap.
- Tests run:
  - `git status --short --branch`
  - `git branch --show-current`
- Tests not run:
  - Implementation validation commands are for the implementation pass and were not run during plan authoring.
- Deviations from plan:
  - None.
- Questions for Codex/Owner:
  - None.
- Remaining risks:
  - Live RPC availability for optional bounded scan validation is unknown.
