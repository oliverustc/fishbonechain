# Platform Backend Skeleton

**Status**: Stage 18 implementation. This is a minimal, dependency-free file-backed development backend skeleton. It is NOT a production backend, NOT a production auth system, NOT a database finality source, and NOT a chain signer.

**Date**: 2026-06-30
**Stage**: Stage 18
**Plan**: `docs/internal/agent-plans/2026-06-30-stage18-web-backend-skeleton.md`

## 1. Overview

The platform backend skeleton provides a minimal HTTP API server for platform business objects defined in Stage 15. It uses Node.js built-in modules only (no Express, Fastify, SQLite, Prisma, PostgreSQL, JWT, or password libraries). All data is stored in JSON files on disk.

Key design points:

- **Backend is orchestration and indexing, not trust replacement.** Backend records are metadata and cache. Chain state and evidence digests are the recoverable source of truth.
- **No private keys.** The backend never stores private keys, mnemonics, or chain signing material. It never sends chain transactions.
- **Development-grade auth only.** Passwords are hashed with SHA-256 + salt. Session tokens are opaque random bytes. This is a skeleton — do not deploy to production.
- **No database finality.** JSON file storage is single-process by design. Records are orchestration metadata, not protocol finality.

## 2. Server Start

```bash
# Start with a specific port and data directory
node scripts/platform-backend/server.js --host 127.0.0.1 --port 3000 --data-dir var/platform-backend/

# Start with an ephemeral port (OS assigns, prints the listening URL)
node scripts/platform-backend/server.js --data-dir var/platform-backend/ --port 0
```

Options:

| Option | Description |
|--------|-------------|
| `--host <addr>` | Bind address (default: `127.0.0.1`) |
| `--port <n>` | Port number; `0` for ephemeral (default: `3000`) |
| `--data-dir <p>` | File-backed store directory (required) |
| `--help` | Print usage and exit |

The server prints the listening URL to stdout. With `--port 0`, the ephemeral port URL is printed on its own line for script consumption.

## 3. API Endpoints

All responses are JSON. Authenticated endpoints require `Authorization: Bearer <token>`.

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/health` | No | Health check, returns `{status, timestamp}` |
| `POST` | `/api/users/register` | No | Register a user: `{display_name, role, password}` |
| `POST` | `/api/users/login` | No | Login: `{display_name, password}` → `{user, token}` |
| `GET` | `/api/users/me` | Yes | Return authenticated user profile |
| `POST` | `/api/chain-accounts` | Yes | Bind a chain account: `{chain_id, address, scene_kind}` |
| `GET` | `/api/chain-accounts` | Yes | List authenticated user's chain accounts |
| `POST` | `/api/business-tasks` | Yes | Create a business task |
| `GET` | `/api/business-tasks` | Yes | List all business tasks |
| `GET` | `/api/business-tasks/:id` | Yes | Get a business task by ID |
| `POST` | `/api/workflow-runs` | Yes | Create a workflow run |
| `GET` | `/api/workflow-runs` | Yes | List all workflow runs |
| `POST` | `/api/evidence` | Yes | Create an evidence record |
| `GET` | `/api/evidence` | Yes | List all evidence records |
| `POST` | `/api/chain-events/import` | Yes | Import chain events (JSON array or text/plain file path) |
| `GET` | `/api/chain-events` | Yes | List cached chain events |
| `POST` | `/api/offchain-jobs` | Yes | Create an offchain job (metadata only) |
| `GET` | `/api/offchain-jobs` | Yes | List all offchain jobs |

### Chain Event Import

Two modes are supported for `POST /api/chain-events/import`:

**JSON array** (`Content-Type: application/json`):
```json
[{"event_id":"e1","chain_id":"main","block_number":1,"block_hash":"0x01","pallet":"t","variant":"T"}]
```

**File path** (`Content-Type: text/plain`):
```
path/to/events.jsonl
```

File path import reads Stage 17 `events.jsonl` format (newline-delimited JSON). Paths are validated: absolute paths outside the repository are rejected with a 400 error. Only repo-local paths are accepted.

## 4. Data Storage

The JSON store maintains eight collections matching Stage 15 platform objects:

| Collection | File | Stage 15 Object |
|------------|------|-----------------|
| `users` | `users.json` | `User` |
| `sessions` | `sessions.json` | (internal) |
| `chain_accounts` | `chain_accounts.json` | `ChainAccount` |
| `business_tasks` | `business_tasks.json` | `BusinessTask` |
| `workflow_runs` | `workflow_runs.json` | `WorkflowRun` |
| `evidence` | `evidence.json` | `Evidence` |
| `chain_events` | `chain_events.json` | `ChainEvent` |
| `offchain_jobs` | `offchain_jobs.json` | `OffchainJob` |

Each collection file is a JSON array. Writes use atomic rename (write to `.tmp`, then rename).

Record field names align with Stage 15 `docs/architecture/platform-business-model.md` and `scripts/platform-model/types.ts`. Schema validation checks for required fields on creation.

### `chain_role` Handling

Stage 17 `events.jsonl` records may include a `chain_role` field (`"main"` or `"child"`). This field is Stage 17 indexer script-local metadata and is NOT part of the Stage 15 platform `ChainEvent` model. The backend import discards `chain_role` and does not require it in stored `ChainEvent` records. The platform source-chain identifier is `chain_id`.

## 5. Trust Boundaries

- **Backend records are NOT chain finality.** Chain state is the recoverable source of truth. Backend records are cached indexed metadata.
- **Chain account binding is metadata only.** `verified_at` is `null` until cryptographic signature challenge verification is implemented. `verification_status: "unverified"` is returned in responses.
- **Offchain jobs are metadata stubs.** No proof generation, worker execution, or ZK pipeline runs in this stage. Jobs are created as `{queued}` metadata records.
- **Auth is development-grade.** SHA-256 + salt password hashing and opaque session tokens. This skeleton must NOT be deployed to production.
- **No private keys.** The backend never stores or handles private keys, mnemonics, or chain signing material. Backend user roles do not authorize chain extrinsics.

## 6. Non-Goals

- No frontend.
- No Express, Fastify, SQLite, Prisma, PostgreSQL, JWT, or password libraries.
- No production authentication claims.
- No chain transaction submission.
- No pallet/runtime/proof/settlement changes.
- No Stage 19 job execution or Stage 20 data-trade module APIs.
- No replacement of the Stage 17 indexer.

## 7. Code Layout

```text
scripts/platform-backend/
  server.js           Entrypoint with CLI (--help, --host, --port, --data-dir)
  lib/
    ids.js            UUID generation and ISO timestamps
    json_store.js     File-backed JSON store with atomic writes
    schema.js         Stage 15 field-name validation
    auth.js           Password hashing, session tokens, AuthService
    routes.js         HTTP route definitions
    http.js           JSON response helpers, body parsing, token extraction
    importers.js      Chain event JSONL import, path safety validation
  test/
    backend_store.test.js   Store, schema, auth, import tests
    backend_api.test.js     Full HTTP API integration tests
```

All modules use Node built-in modules only: `node:http`, `node:fs`, `node:path`, `node:crypto`, `node:test`, `node:assert`.

## 8. Validation

```bash
# Syntax checks
node --check scripts/platform-backend/server.js
find scripts/platform-backend/lib -name '*.js' -print -exec node --check {} \;

# Run all tests
node --test scripts/platform-backend/test/*.test.js

# CLI help
node scripts/platform-backend/server.js --help

# Verify no monitor coupling
! rg -q 'monitor/' scripts/platform-backend/

# Verify gitignore for runtime data
git check-ignore var/platform-backend/
```

Requires Node.js 20 or newer for stable `node:test`. Node.js 18 users should note that `node:test` is experimental.

## 9. Known Limitations

- Single-process only. Concurrent writes to the same JSON file can cause data loss.
- No updates or deletes for most resources. Only create, list, and get-by-id are implemented.
- Chain account binding does not cryptographically verify address ownership. `verified_at` is always `null`.
- Password hashing uses SHA-256 with salt, not bcrypt/argon2. This is a development skeleton.
- Session tokens are opaque random bytes with no expiration.
- No pagination, filtering, or search on list endpoints.
- File-backed storage is not suitable for concurrent access or production workloads.

## 10. References

- [Platform Business Model](../architecture/platform-business-model.md) — Stage 15 object model
- [Data Trade CLI/API Boundary](data-trade-cli-api-boundary.md) — Stage 16
- [Chain Event Indexer and State Sync](chain-event-indexer-state-sync.md) — Stage 17 indexer
- [Long-term Roadmap](../internal/agent-plans/2026-06-28-data-flow-platform-long-term-roadmap.md) — Stage 18 definition
