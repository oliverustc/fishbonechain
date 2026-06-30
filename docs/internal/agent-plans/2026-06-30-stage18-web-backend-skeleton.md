# Stage 18 Plan: Web Backend Minimal Skeleton

## Goal

Create a minimal, dependency-free platform Web backend skeleton that exposes basic HTTP APIs for users, login sessions, chain-account binding records, business tasks, chain events, evidence, and offchain jobs, while preserving the rule that chain state and evidence remain the recoverable source of truth.

## Background

The long-term roadmap defines Stage 18 as "Web 后端最小骨架" with these core functions:

- user registration/login;
- chain account binding;
- business task table;
- chain event table;
- evidence table;
- offchain job table;
- basic API skeleton.

The same roadmap states that the backend records business tasks, evidence metadata, chain event indexes, frontend query data, and offchain job orchestration metadata. It must not hold user private keys, sign without user authorization, build a complex frontend, or connect all business modules at once.

Current repository facts verified before writing this plan:

- `docs/architecture/platform-business-model.md` is the Stage 15 design baseline for `User`, `ChainAccount`, `Dataset`, `DataAsset`, `BusinessTask`, `WorkflowRun`, `Evidence`, `ChainEvent`, and `OffchainJob`. It explicitly says backend records are orchestration and audit metadata, not chain finality.
- `scripts/platform-model/types.ts` is a dependency-free JSDoc type draft for the same core objects and passes normal Node syntax checks by design.
- `docs/implementation/data-trade-cli-api-boundary.md` is the Stage 16 boundary. It states that chain-changing data-trade operations require user/dev signer keys and that backend orchestration is not a trust replacement.
- `docs/implementation/chain-event-indexer-state-sync.md` is the Stage 17 implementation record for the file-backed event indexer. `scripts/chain_event_indexer.js` can write normalized `events.jsonl`, `cursor.json`, derived `state.json`, and evidence correlations.
- Root `package.json` is `"type": "module"` and only declares `@polkadot/api` as a dev dependency. `monitor/` is a separate TypeScript service with its own package and dependencies; Stage 18 should not silently couple the platform backend skeleton to the monitor package.
- `.gitignore` already ignores `.agents/`, `node_modules/`, logs, build output, and generated experiment data. It does not yet define a backend data directory.
- The repository was clean on `main` before this plan branch was created.
- Stage branch created for this plan: `stage/stage18-web-backend-skeleton`.

## Scope

Implementation may add or update:

- a new dependency-free backend skeleton under `scripts/platform-backend/`;
- small reusable modules under `scripts/platform-backend/lib/`;
- focused Node test files under `scripts/platform-backend/test/`;
- a formal implementation document, preferably `docs/implementation/platform-backend-skeleton.md`;
- `docs/README.md` index entry for the new formal backend document;
- `.gitignore` entries for generated backend runtime state, if the backend writes repo-local development data;
- this plan's Execution Record only during implementation.

Recommended file layout:

```text
scripts/platform-backend/
  server.js
  lib/
    ids.js
    json_store.js
    schema.js
    auth.js
    routes.js
    http.js
    importers.js
  test/
    backend_store.test.js
    backend_api.test.js
docs/implementation/platform-backend-skeleton.md
```

The implementation does not need to use exactly these module names if a smaller layout is clearer, but it must keep backend code isolated from existing experiment scripts and avoid a framework.

## Non-Goals

- Do not build a frontend.
- Do not add Express, Fastify, SQLite, Prisma, PostgreSQL, JWT libraries, password libraries, or any other dependency unless the owner explicitly approves the dependency change first.
- Do not add production authentication claims. This stage may use a development-grade session token skeleton and must document its limitations.
- Do not store private keys, mnemonics, dev seed phrases, or chain signing material.
- Do not send chain transactions from the backend.
- Do not make database state protocol finality.
- Do not change pallets, runtime, proof digest fields, settlement rules, verifier assumptions, chain specs, deployment topology, experiment metrics, or paper-facing measured data.
- Do not replace the Stage 17 indexer; import or replay its JSON outputs instead.
- Do not implement all data-trade business APIs from Stage 20. Only generic platform resources and a minimal data-trade-shaped sample task/evidence flow are in scope.

## Current Facts

- The platform business model requires user-signed chain actions only. `ChainAccount.verified_at` records whether a chain-signature challenge was confirmed, but backend user roles do not authorize chain extrinsics.
- `ChainEvent` in Stage 15 does not include `chain_role`. Stage 17 `events.jsonl` includes `chain_role` as script-local metadata only. Backend persistence must not require `chain_role` in the platform `ChainEvent` table/model.
- Stage 17 normalized events are newline-delimited JSON records and are replayable from `events.jsonl`. Backend import should treat imported chain events as cached indexed records, not proof of finality beyond the indexed source.
- Data-trade proof generation is an offchain operation under Stage 16. Backend may create an `OffchainJob` metadata record for `proof_generation`, but must not run the ZK pipeline in this stage unless the implementation explicitly keeps it as a queued metadata stub.
- `monitor/` already has an HTTP-like operational service, but it is for monitoring and should not become the platform business backend in this stage.

## Design Constraints

Use Node built-in modules only for the first skeleton:

- `node:http` for the HTTP server;
- `node:fs` and `node:path` for file-backed storage;
- `node:crypto` for password hashing and opaque development session tokens;
- `node:test` and `node:assert` for validation.

Minimum runtime for validation: Node.js 20 or newer is preferred because
`node:test` is stable there. If the local environment is Node.js 18, the
implementation agent must record that `node:test` is experimental in that
runtime and still run the listed validation commands unless the command is
unavailable.

Use a repo-local JSON store by default, for example `.agents/fwf/runs/stage18/backend-data/` in tests and `var/platform-backend/` or `.agents/fwf/runs/stage18/dev-server/` for manual development. If a persistent runtime directory is added under the repo, add it to `.gitignore`.

The backend API should return JSON only. It should expose a small health endpoint and versioned API paths.

Server CLI semantics:

- `--host <addr>` selects the bind address.
- `--port <n>` selects the port. `--port 0` means ask the OS for an ephemeral port, print the resolved listening URL to stdout after the server starts, and keep running until terminated.
- `--data-dir <path>` selects the file-backed store directory.
- `--help` prints usage and exits without starting the server.

Recommended endpoint surface:

```text
GET  /health
POST /api/users/register
POST /api/users/login
GET  /api/users/me
POST /api/chain-accounts
GET  /api/chain-accounts
POST /api/business-tasks
GET  /api/business-tasks
GET  /api/business-tasks/:id
POST /api/workflow-runs
GET  /api/workflow-runs
POST /api/evidence
GET  /api/evidence
POST /api/chain-events/import
GET  /api/chain-events
POST /api/offchain-jobs
GET  /api/offchain-jobs
```

Minimal behavior expected:

- Registration creates a `User` record with a password hash, not a plaintext password.
- Login returns an opaque session token stored in the file-backed development store.
- Authenticated endpoints require the session token, for example `Authorization: Bearer <token>`.
- Chain account binding records include `user_id`, `chain_id`, `address`, `scene_kind`, and `verified_at`. If full SS58 signature verification is not implemented without adding dependencies, the endpoint must create an unverified binding with `verified_at: null` and document that cryptographic challenge verification is deferred.
- Business task, workflow run, evidence, chain event, and offchain job records should align with Stage 15 field names where practical.
- Chain event import should accept either an array of normalized events or a Stage 17 `events.jsonl` file path under a controlled repo-local path. It must reject absolute paths outside the repository unless the plan is explicitly amended.
- Records should use stable generated IDs, timestamps, and basic validation errors.
- API errors should be structured JSON with a status code and message.

## Risks

- Security risk: a login skeleton can be mistaken for production auth. The implementation must call it a development skeleton and avoid claims about production readiness.
- Key custody risk: account binding can be mistaken for backend signing. The backend must not accept or store private keys and must not call chain extrinsics.
- Data integrity risk: file-backed JSON writes can corrupt data if writes are concurrent. The implementation should keep the skeleton single-process and document the limitation; tests should verify deterministic writes for sequential requests.
- Trust-boundary risk: imported chain events and evidence can be displayed as if final. Docs and response field names must distinguish cached/indexed records from chain finality.
- Scope risk: Stage 18 can expand into Stage 20 data-trade APIs or Stage 19 job execution. Keep data-trade-specific behavior to sample references and generic records.
- Compatibility risk: backend schema may drift from Stage 15 objects. Add tests or schema checks for required field names on core records.
- Repository hygiene risk: generated backend data can be committed by accident. Use ignored directories for runtime data and only commit fixtures/tests/docs.

## Stop Conditions

The implementation agent must stop and ask Codex or the owner before:

- adding or updating package dependencies or lockfiles;
- implementing production auth, OAuth, JWT, database migrations, or a real database server;
- adding private-key handling, seed phrase handling, or backend chain signing;
- changing pallet/runtime/proof/settlement behavior;
- changing Stage 17 event normalization semantics or cursor semantics;
- claiming account ownership verification without actual cryptographic signature verification evidence;
- importing arbitrary filesystem paths from API input;
- changing experiment metrics, figures, denominators, or paper-facing claims;
- broadening into Stage 19 job execution or Stage 20 data-trade module APIs.

## Branch and Commit Plan

- Branch: `stage/stage18-web-backend-skeleton`.
- Implementation should stay on this branch.
- Commit after a coherent implementation pass only when:
  - backend code, tests, docs, and this plan's Execution Record are updated;
  - validation commands below have run or skipped commands are explicitly justified;
  - `git status --short` contains only intended Stage 18 changes and ignored local runtime data.

Recommended implementation commit message:

```text
feat(backend): add minimal platform backend skeleton

Plan: docs/internal/agent-plans/2026-06-30-stage18-web-backend-skeleton.md
Validation:
- node --check scripts/platform-backend/server.js
- node --test scripts/platform-backend/test/*.test.js
```

## Task List

- [ ] Re-read `agent.md`, `docs/internal/agent-collaboration.md`, this plan, `docs/architecture/platform-business-model.md`, `docs/implementation/data-trade-cli-api-boundary.md`, and `docs/implementation/chain-event-indexer-state-sync.md`.
- [ ] Confirm the branch is `stage/stage18-web-backend-skeleton` and record `git status --short`.
- [ ] Create the backend skeleton directory under `scripts/platform-backend/` using Node built-in modules only.
- [ ] Implement a small JSON store with explicit collections for `users`, `sessions`, `chain_accounts`, `business_tasks`, `workflow_runs`, `evidence`, `chain_events`, and `offchain_jobs`.
- [ ] Implement ID/timestamp helpers and basic schema validation for the Stage 15 core object fields used by each collection.
- [ ] Implement development auth: registration with password hashing, login with opaque token creation, token lookup, and authenticated `me`.
- [ ] Implement generic CRUD-style create/list/get endpoints only where listed in this plan. Keep updates/deletes out unless needed for login/session mechanics.
- [ ] Implement chain-account binding as a metadata record. If cryptographic signature verification is not implemented, keep `verified_at: null` and return a response field explaining `verification_status: "unverified"`.
- [ ] Implement chain-event import from Stage 17-compatible records. If file-path import is implemented, constrain paths to the repository and test rejection of unsafe paths.
- [ ] Implement evidence and offchain-job record creation as metadata only. Do not run proof generation or worker processes in this stage.
- [ ] Add `--help`, `--host`, `--port`, and `--data-dir` options to the server entrypoint.
- [ ] Add Node tests for store behavior, auth flow, protected route rejection, task/evidence/job creation, Stage 15 field-name alignment, chain event import, and path safety if file import exists.
- [ ] Verify backend code does not import from or depend on `monitor/`.
- [ ] Add or update `.gitignore` for generated backend runtime data if a non-ignored runtime directory is introduced.
- [ ] Write `docs/implementation/platform-backend-skeleton.md` with architecture, API list, data storage, trust boundaries, validation commands, and known limitations.
- [ ] Update `docs/README.md` with the new backend skeleton document.
- [ ] Update this plan's Execution Record with files changed, tests run, deviations, and remaining risks.
- [ ] Run the validation commands and inspect generated output files before committing.

## Acceptance Criteria

- A minimal backend server can start with a repo-local data directory and answer `GET /health`.
- User registration and login work in tests without storing plaintext passwords.
- Authenticated endpoints reject missing/invalid tokens.
- Chain-account binding creates records without private-key fields and without false verification claims.
- Business task, workflow run, evidence, chain event, and offchain job metadata can be created and listed through JSON APIs.
- Chain event import accepts Stage 17-compatible normalized records and preserves platform `ChainEvent` fields without requiring `chain_role`.
- Runtime data generated by tests or manual runs is not committed.
- Formal docs describe how to run the backend skeleton and state that it is not production auth, not a database finality source, and not a chain signer.
- `docs/README.md` includes an index entry for `docs/implementation/platform-backend-skeleton.md`.
- No new dependencies or lockfile changes are introduced unless separately approved.

## Validation Commands

Run these from the repository root:

```bash
git branch --show-current
git status --short
node --check scripts/platform-backend/server.js
find scripts/platform-backend/lib -name '*.js' -print -exec node --check {} \;
node --test scripts/platform-backend/test/*.test.js
node scripts/platform-backend/server.js --help
test -f docs/implementation/platform-backend-skeleton.md
grep -q platform-backend-skeleton docs/README.md
! rg -q 'monitor/' scripts/platform-backend/
git status --short
```

The `node --test` suite must include explicit coverage for:

- Stage 15 field-name alignment for core records, including `user_id`, `account_id`, `task_id`, `run_id`, `evidence_id`, `event_id`, `job_id`, and `chain_id`;
- chain-event import accepting Stage 17-compatible records while not requiring platform `ChainEvent` records to contain `chain_role`;
- unsafe file path rejection for chain-event import if file-path import is implemented.

If a project runtime directory is introduced, for example `var/platform-backend/`,
add a `.gitignore` entry and validate it:

```bash
git check-ignore var/platform-backend/
```

After tests and any smoke run, `git status --short` must not show generated
runtime data. If generated data appears, fix the runtime output path or
`.gitignore` before committing.

If the implementation adds a manual smoke script or uses a long-running server smoke test, write outputs under `.agents/fwf/runs/stage18/` and record the exact command in the Execution Record.

Optional integration smoke, only if the implementation supports the described CLI and the agent can manage the server process cleanly:

```bash
mkdir -p .agents/fwf/runs/stage18/smoke
node scripts/platform-backend/server.js --host 127.0.0.1 --port 0 --data-dir .agents/fwf/runs/stage18/smoke/data
```

For automated smoke validation, prefer tests over a manual long-running server.
If a real server process is used, run it with a bounded harness that starts the
server, calls `/health`, and terminates the process cleanly; do not leave a
server running after validation.

## Validation Output Paths

Use repo-local ignored output paths:

```text
.agents/fwf/runs/stage18/backend-test/
.agents/fwf/runs/stage18/smoke/
.agents/fwf/runs/stage18/import-fixture/
```

Do not write validation summaries to `/tmp` unless a tool has no usable repo-local output option. Do not commit `.agents/fwf/runs/`.

## Documentation Updates

Required:

- Add `docs/implementation/platform-backend-skeleton.md`.
- Update `docs/README.md` implementation/development index.

The formal backend document must include:

- status and scope for Stage 18;
- server start command and data-dir behavior;
- API endpoint table;
- core collection names and relationship to Stage 15 objects;
- Stage 17 chain-event import behavior;
- trust boundaries and non-goals;
- validation commands and current known limitations.

Do not update experiment reports, paper gap matrices, deployment runbooks, or chain architecture claims unless implementation actually changes those areas. This stage should not change them.

## Execution Record

### 2026-06-30 opencode Pass 1

- Branch: `stage/stage18-web-backend-skeleton`
- Base commit: `bd008fe` (plan re-review approval)
- Tasks completed: All 18 plan tasks.
- Files changed (10 new, 3 modified):
  - `scripts/platform-backend/server.js` — entrypoint with CLI
  - `scripts/platform-backend/lib/ids.js` — UUID generation and ISO timestamps
  - `scripts/platform-backend/lib/json_store.js` — file-backed JSON store with atomic writes
  - `scripts/platform-backend/lib/schema.js` — Stage 15 field-name validation
  - `scripts/platform-backend/lib/auth.js` — password hashing (SHA-256+salt), session tokens, AuthService
  - `scripts/platform-backend/lib/routes.js` — HTTP route definitions for all 17 endpoints
  - `scripts/platform-backend/lib/http.js` — JSON response helpers, body parsing, token extraction
  - `scripts/platform-backend/lib/importers.js` — JSONL chain event import, path safety validation
  - `scripts/platform-backend/test/backend_store.test.js` — JsonStore, Schema, Auth, Importers tests (33 tests)
  - `scripts/platform-backend/test/backend_api.test.js` — Full HTTP API integration tests (23 tests)
  - `.gitignore` — added `/var/platform-backend/`
  - `docs/implementation/platform-backend-skeleton.md` — formal implementation document
  - `docs/README.md` — added platform-backend-skeleton index entry
- Tests run: `node --test scripts/platform-backend/test/*.test.js` — 57 tests, 57 pass, 0 fail
- Tests not run: None. All validation commands executed.
- Validation output paths: `.agents/fwf/runs/stage18/backend-test/` (test runtime data, git-ignored)
- Deviations from plan:
  - None. Followed plan file layout exactly with the recommended module names.
  - Chain-event import supports both JSON array and text/plain file path modes (plan allows either).
  - No `workflow_runs/:id` endpoint (plan only lists list/create for workflow runs).
- Questions for Codex/Owner: None.
- Remaining risks:
  - Single-process only — concurrent writes unhandled (documented limitation).
  - No session expiration (documented as development-grade).
  - Password hashing uses SHA-256+salt, not bcrypt/argon2 (documented as development skeleton).
  - Chain account `verified_at` is always `null` (documented as `verification_status: "unverified"`).

### 2026-06-30 opencode Review-Fix Pass 2

- Branch: `stage/stage18-web-backend-skeleton`
- Base commit: `2771161` (Stage 18 implementation)
- Head commit: `b7378c7` (`fix(backend): validate password input and harden import path against symlinks`)
- Commits:
  - `b7378c7` (`fix(backend): validate password input and harden import path against symlinks`)
- Tasks completed:
  - Fix 1: Validate registration password input (reject missing, empty, non-string).
  - Fix 2: Harden chain-event file import path confinement against symlink escape.
  - Added 6 regression tests (3 API-level password tests, 2 store-level password tests, 1 symlink escape test).
- Files changed:
  - `scripts/platform-backend/lib/auth.js` — added password non-empty string validation in `register()`.
  - `scripts/platform-backend/lib/importers.js` — added `fs.realpathSync()` to `resolveImportPath()`.
  - `scripts/platform-backend/test/backend_store.test.js` — added 2 password validation tests + 1 symlink escape test.
  - `scripts/platform-backend/test/backend_api.test.js` — added 3 password validation tests.
  - `docs/internal/agent-plans/2026-06-30-stage18-web-backend-skeleton.md` — this Execution Record update.
- Tests run: `node --test scripts/platform-backend/test/*.test.js` — 63 tests, 63 pass, 0 fail
- Tests not run: None.
- Validation output paths: (none, test evidence from Pass 1 still at `.agents/fwf/runs/stage18/backend-test/`)
- Deviations from plan: None. Both fixes scoped to required findings only.
- Questions for Codex/Owner: None.
- Remaining risks:
  - Same as Pass 1 (single-process, no session expiry, SHA-256+salt, `verified_at: null`).
  - Symlink escape vector is now blocked by `fs.realpathSync()` validation.

## Plan-Review Focus

opencode should review:

- whether a dependency-free file-backed backend skeleton is sufficient for Stage 18 or whether the plan should explicitly authorize a real web/database dependency;
- whether the chain-account binding scope is honest enough when cryptographic signature verification may be deferred;
- whether the API surface is minimal and generic enough to avoid drifting into Stage 20 data-trade APIs;
- whether the Stage 17 chain-event import constraints are safe and concrete;
- whether validation commands are enough for a docs/code skeleton with no compiler gate;
- whether any formal docs beyond `docs/implementation/platform-backend-skeleton.md` and `docs/README.md` must be updated.

## Plan Review Resolution

Plan review:
`docs/internal/agent-reviews/2026-06-30-stage18-web-backend-skeleton-plan-review.md`

Decision: `approved-with-required-fixes`.

Required fixes applied:

- F1: Added `test -f docs/implementation/platform-backend-skeleton.md` to validation commands.
- F2: Added `grep -q platform-backend-skeleton docs/README.md` to validation commands and added an acceptance criterion for the docs index entry.
- F3: Added runtime-data hygiene validation using a final `git status --short`, with `git check-ignore var/platform-backend/` required if a project runtime directory is introduced.

Suggested improvements accepted:

- Defined `--port 0` as ephemeral port assignment with the resolved listening URL printed to stdout.
- Added Node.js runtime guidance for `node:test`.
- Required schema field-name alignment coverage in the Node test suite.
- Added a monitor-decoupling validation command.
- Made chain-event file path safety testing explicit when file-path import exists.
- Clarified that any real server smoke validation must be bounded and must terminate the server cleanly.

Suggestions or required fixes rejected:

- None. No minimum test count was added because the plan already requires functional coverage by behavior; implementation should add as many tests as needed to cover the required cases.

Ready for opencode re-review: yes. Implementation should not proceed until the revised plan is re-reviewed and approved with no required fixes, unless the owner explicitly overrides that gate.
