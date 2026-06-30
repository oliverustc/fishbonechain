# Chain Event Indexer and State Sync

**Status**: Stage 17 implementation. This is a file-backed chain event indexer and state-sync foundation, not a backend or database server.

**Date**: 2026-06-30
**Stage**: Stage 17
**Plan**: `docs/internal/agent-plans/2026-06-30-stage17-chain-event-indexer-state-sync.md`

## 1. Overview

The chain event indexer provides reusable infrastructure for scanning FishboneChain events, normalizing them into platform-compatible records, maintaining resumable cursors, and deriving data-trade state snapshots. It is designed as a CLI script (`scripts/chain_event_indexer.js`) without introducing a Web backend, database, protocol change, or new dependency.

Key capabilities:

- Multi-chain RPC event scanning with bounded block ranges
- Cursor-based resumable scans
- Event normalization into `ChainEvent` JSONL records
- State derivation for data-trade listings, sessions, and escrows
- Evidence-to-event correlation
- No-RPC fixture/replay validation

## 2. Commands

### 2.1 `scan`

Scan one or both chains over a bounded block range and write normalized chain events.

```bash
node scripts/chain_event_indexer.js scan \
  --profile child6-data-trade \
  --chain both \
  --from 1 --to 100 \
  --out .agents/fwf/runs/stage17/live-scan
```

Options:
| Option | Description |
|--------|-------------|
| `--profile <id>` | Load `main_ws` and `child_ws` from `scripts/profiles/chains.json` |
| `--main <ws>` | Override main chain RPC endpoint |
| `--child <ws>` | Override child chain RPC endpoint |
| `--chain <role>` | `main`, `child`, or `both` (default: `both`) |
| `--from <block>` | Starting block number (required) |
| `--to <block>` | Ending block number (required) |
| `--max-blocks <n>` | Maximum blocks to scan (reduces effective range) |
| `--include-all` | Record all events, not just baseline data-trade events |
| `--out <dir>` | Output directory (required) |

Outputs: `events.jsonl`, `cursor.json`.

### 2.2 `replay`

Rebuild derived state, summary, and event copy from stored `events.jsonl` without RPC.

```bash
node scripts/chain_event_indexer.js replay \
  --events scripts/fixtures/chain_events/data_trade_sample_events.jsonl \
  --out .agents/fwf/runs/stage17/replay-fixture
```

Options:
| Option | Description |
|--------|-------------|
| `--events <path>` | Input `events.jsonl` file (required) |
| `--out <dir>` | Output directory (required) |

Outputs: `events.jsonl`, `state.json`, `summary.json`, `summary.md`.

### 2.3 `state`

Derive data-trade state snapshots (listings, sessions, escrows) from normalized events.

```bash
node scripts/chain_event_indexer.js state \
  --events events.jsonl \
  --out <dir>
```

| Option | Description |
|--------|-------------|
| `--events <path>` | Input `events.jsonl` file (required) |
| `--out <dir>` | Output directory (required) |

Outputs: `state.json`, `summary.json`.

### 2.4 `correlate-evidence`

Correlate evidence JSON files with indexed chain events.

```bash
node scripts/chain_event_indexer.js correlate-evidence \
  --events events.jsonl \
  --evidence summary.json \
  --out <dir>
```

| Option | Description |
|--------|-------------|
| `--events <path>` | Input `events.jsonl` file (required) |
| `--evidence <path>` | Evidence JSON file (required) |
| `--out <dir>` | Output directory (required) |

Outputs: `correlations.json`.

Correlation matches evidence entries to events by:
- `listing_id` → `dataRegistry` events
- `session_id` → `tradeSession` events
- `escrow_id` → `mainEscrow` events
- Event name matching against expected event lists

Dry-run/no-chain evidence (null IDs) is marked `not_applicable` — the indexer does not invent false event links.

### 2.5 `inspect`

Inspect indexer data without RPC.

```bash
# Write supported event mappings
node scripts/chain_event_indexer.js inspect mappings --out mappings.json

# Print cursor summary
node scripts/chain_event_indexer.js inspect cursor --cursor cursor.json

# Print event/state counts
node scripts/chain_event_indexer.js inspect counts --events events.jsonl

# Create sample dry-run evidence for correlation testing
node scripts/chain_event_indexer.js inspect sample-evidence --kind dry-run --out sample.json
```

## 3. Normalized ChainEvent Schema

Each normalized event record in `events.jsonl` is a JSON object with these fields:

| Field | Type | Description |
|-------|------|-------------|
| `event_id` | `string` | Unique event identifier |
| `chain_id` | `string` | Source chain identifier (e.g., `"main"`, `"child6"`) |
| `chain_role` | `string` | `"main"` or `"child"` — Stage 17 script-local metadata |
| `profile` | `string` | Trade profile name |
| `block_number` | `number` | Block number |
| `block_hash` | `string` | Block hash (hex) |
| `extrinsic_index` | `number \| null` | Extrinsic index within the block |
| `event_index` | `number` | Event index within the extrinsic |
| `pallet` | `string` | Pallet name |
| `variant` | `string` | Event variant |
| `fields` | `object` | Event field values (JSON-serializable) |
| `cursor` | `string` | Indexer cursor for resumption |
| `ingested_at` | `string` | ISO 8601 ingestion timestamp |

`chain_role` is Stage 17 indexer metadata only. It is not part of the Stage 15 platform `ChainEvent` object and must not be required by future backend schemas. The platform source-chain identifier is `chain_id`.

### JSONL Format

`events.jsonl` is newline-delimited JSON: one complete JSON object per line, no array wrapper, no trailing commas. The file ends with a newline. This format allows future scanners to append safely without rewriting the whole file.

## 4. Cursor Semantics

A cursor is a string of the form `<chain_role>:<next_block>`, e.g., `main:42` or `child:100`. After scanning block `N`, the cursor is updated to `N+1`.

`cursor.json` maps chain roles to their cursors:

```json
{
  "main": "main:42",
  "child": "child:100"
}
```

Resuming a scan starts from the recorded `next_block` for each chain role. The `parseCursor()` function extracts `{ chainRole, block }` from a cursor string.

## 5. Event Mapping Baseline

The indexer tracks these data-trade events by default:

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

Other events may be recorded when `--include-all` is set on the `scan` command.

## 6. State Derivation Rules

State is derived by replaying normalized events in order. It is not stored in a database — each derivation pass reads from `events.jsonl` and produces deterministic output.

### Listings

| Event | Effect |
|-------|--------|
| `DataPublished` | Creates listing with owner, price_per_round, max_rounds; status = `active` |
| `ListingStatusChanged` | Updates listing `status` |
| `ImtRootUpdated` | Records latest IMT root |

Listing state: `{ listing_id, owner, price_per_round, max_rounds, status, imt_root, source_events[], last_event_id }`

### Sessions

| Event | Effect |
|-------|--------|
| `SessionCreated` | Creates session with requester, data_owner, listing_id, escrow_id |
| `SessionAccepted` | Status → `active` |
| `SettlementClaimed` | Status → `settled` |
| `SessionPunished` | Status → `punished` |
| `LastPaymentClaimed` | Status → `last_payment_claimed` |
| Round events | Track per-round status in `rounds[round_index]` |

Session state: `{ session_id, requester, data_owner, listing_id, escrow_id, status, rounds, source_events[], last_event_id }`

### Escrows

| Event | Effect |
|-------|--------|
| `EscrowOpened` | Creates escrow with requester, data_owner; status = `opened` |
| `FundsLocked` | Records `funds_locked`; status → `funded` |
| `DepositLocked` | Records `deposit_locked`; status → `ready` |
| `EscrowSettled` | Records `paid_rounds`, `refunded`; status → `settled` |
| `EscrowPunished` | Records `slashed_deposit`; status → `punished` |

Escrow state: `{ escrow_id, requester, data_owner, status, funds_locked, deposit_locked, paid_rounds, refunded, slashed_deposit, source_events[], last_event_id }`

## 7. Evidence Correlation

The correlation engine matches evidence entries to chain events by:

1. ID matching: `listing_id`, `session_id`, `escrow_id` compared numerically against event `fields`
2. Event name matching: expected event names (`pallet.variant`) compared against indexed events

Correlation results classify each evidence entry as:
- `matched`: at least one matching event found
- `not_applicable`: no chain IDs present (dry-run/no-chain evidence)
- `partial`: chain IDs present but no matching events found

The correlation does not invent false links and does not claim cross-chain finality.

## 8. Live Scan Limitations

- Live scans require reachable RPC endpoints. The indexer exists gracefully when endpoints are unavailable.
- Scanning large block ranges is slow. Use `--from`/`--to` with `--max-blocks` to bound scans.
- Unbounded scans (without `--to`) are not supported — the indexer requires explicit block range boundaries.
- Event ordering across chains is not globally ordered. Correlation uses explicit IDs, not wall-clock block ordering.

## 9. Validation Flow

### No-RPC Fixture Validation

```bash
# Inspect event mappings
node scripts/chain_event_indexer.js inspect mappings --out inspect-mappings.json

# Replay from committed fixture
node scripts/chain_event_indexer.js replay \
  --events scripts/fixtures/chain_events/data_trade_sample_events.jsonl \
  --out replay-fixture

# Derive state independently
node scripts/chain_event_indexer.js state \
  --events replay-fixture/events.jsonl \
  --out state-fixture

# Create sample dry-run evidence
node scripts/chain_event_indexer.js inspect sample-evidence \
  --kind dry-run --out sample-dry-run-evidence.json

# Correlate dry-run evidence (should report not_applicable)
node scripts/chain_event_indexer.js correlate-evidence \
  --events replay-fixture/events.jsonl \
  --evidence sample-dry-run-evidence.json \
  --out correlate-dry-run
```

### Optional Bounded Live Scan

Only when RPC endpoints are reachable:

```bash
node scripts/chain_event_indexer.js scan \
  --profile child6-data-trade \
  --chain both \
  --from <start> --to <end> \
  --out live-scan

# Derive state from live scan events
node scripts/chain_event_indexer.js state \
  --events live-scan/events.jsonl \
  --out live-scan-state
```

## 10. Non-Goals and Trust Boundaries

This indexer is **not**:

- A Web backend, HTTP server, REST/RPC API, or database
- A production daemon or long-running service
- A cross-chain settlement verification tool
- A replacement for chain finality

**Trust boundaries**:

- Indexed state is a replay/cache of chain events. It is not a substitute for on-chain state queries.
- Cross-chain event order is not globally ordered. Do not derive causality from event ordering alone.
- Event field names may differ across runtime versions. The indexer normalizes fields but does not guarantee compatibility across breaking metadata changes.
- Evidence correlation does not claim to prove cross-chain event causality.

## 11. References

- [Platform Business Model](../architecture/platform-business-model.md) — Stage 15 platform object model including `ChainEvent`
- [Data Trade CLI/API Boundary](data-trade-cli-api-boundary.md) — Stage 16 command boundary
- [Data Trade Implementation](data-trade-implementation.md)
- [Stage 14 Evidence Index](data-trade-stage14-evidence-index.md)
- [Long-term Roadmap](../internal/agent-plans/2026-06-28-data-flow-platform-long-term-roadmap.md) — Stage 17 definition
