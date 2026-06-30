# Stage 18 Plan Review: Web Backend Minimal Skeleton

Date: 2026-06-30
Reviewer: opencode
Plan: `docs/internal/agent-plans/2026-06-30-stage18-web-backend-skeleton.md`
Review type: plan review

## Scope Reviewed

- Stage 18 plan in its entirety.
- All current facts, scope, non-goals, risks, stop conditions, task list, acceptance criteria, validation commands, and documentation updates.

## Inputs Read

1. `agent.md` — current project state, conventions, Stage 12 lessons learned.
2. `docs/internal/agent-collaboration.md` — agent roles, plan/review/execution standards, stop conditions.
3. `docs/internal/agent-plans/2026-06-30-stage18-web-backend-skeleton.md` — the plan under review.
4. `docs/internal/agent-plans/2026-06-28-data-flow-platform-long-term-roadmap.md` — Stage 18 definition (lines 250-279).
5. `docs/architecture/platform-business-model.md` — Stage 15 baseline, 9 core objects, field definitions.
6. `docs/implementation/data-trade-cli-api-boundary.md` — Stage 16 boundary, security model, chain/off-chain classification.
7. `docs/implementation/chain-event-indexer-state-sync.md` — Stage 17 indexer, normalized event schema, cursor semantics, `chain_role` metadata.
8. `scripts/platform-model/types.ts` — JSDoc type draft for core objects (248 lines).
9. `package.json` — `"type": "module"`, only `@polkadot/api` as devDependency.
10. `.gitignore` — no backend runtime data entries yet.
11. `monitor/package.json` — separate TypeScript service, distinct from platform backend.
12. `docs/README.md` — no `platform-backend-skeleton` entry yet (correct for planning stage).
13. `docs/implementation/` — `platform-backend-skeleton.md` does not exist (correct).
14. `scripts/platform-backend/` — directory does not exist (correct).
15. `scripts/chain_event_indexer.js` — exists, Stage 17 indexer.
16. `git log --oneline -5` — branch `stage/stage18-web-backend-skeleton` has one commit (plan only, `e9ce4dd`).
17. `git status --short` — clean working tree.

## Current Facts Verification

### Verified (backed by repository evidence)

| Claim | Evidence |
|-------|----------|
| `docs/architecture/platform-business-model.md` is the Stage 15 design baseline | File exists (380 lines), defines all 9 core objects |
| `scripts/platform-model/types.ts` is dependency-free JSDoc type draft | File exists (248 lines), passes `node --check` |
| `docs/implementation/data-trade-cli-api-boundary.md` is Stage 16 boundary | File exists (345 lines), defines 10 commands |
| `docs/implementation/chain-event-indexer-state-sync.md` is Stage 17 impl record | File exists (333 lines), defines scan/replay/state/correlate |
| `scripts/chain_event_indexer.js` exists | File confirmed, Stage 17 deliverable |
| Root `package.json` is `"type": "module"`, only `@polkadot/api` as devDependency | Verified: 6 lines, no other deps |
| `monitor/` is a separate TypeScript service | `monitor/package.json` exists with own deps |
| `.gitignore` does not yet define backend data directory | Verified: 53 lines, no `platform-backend` or `var/` entries |
| Stage branch is `stage/stage18-web-backend-skeleton` | `git branch --show-current` confirms |
| Working tree is clean | `git status --short` shows no output |
| `scripts/platform-backend/` does not exist | Glob returns no files |
| `docs/implementation/platform-backend-skeleton.md` does not exist | Confirmed by listing `docs/implementation/` |
| `docs/README.md` has no backend-skeleton entry | Verified: no mention of `platform-backend-skeleton` |

### Additional facts confirmed

- **`chain_role` status**: Plan correctly states `chain_role` is Stage 17 script-local metadata only, not a platform `ChainEvent` field. Verified in Stage 17 formal doc (lines 151, 153).
- **Stage 15 `ChainEvent` model**: As claimed, does not include `chain_role`. Confirmed in `platform-business-model.md` lines 185-197 and `types.ts` lines 220-231.
- **Roadmap alignment**: Stage 18 definition (roadmap lines 250-279) matches plan scope: user registration/login, chain account binding, business task table, chain event table, evidence table, offchain job table, basic API skeleton.

## Findings

### Finding 1 — severity: medium — Formal doc creation not in validation commands

**Location**: Validation Commands section.

**Issue**: The task list requires writing `docs/implementation/platform-backend-skeleton.md` and the acceptance criteria state "Formal docs describe how to run the backend skeleton." However, the validation commands do not include `test -f docs/implementation/platform-backend-skeleton.md`. Per Stage 12 lessons learned (agent.md lines 112-128), every Execution Record file claim must be verified by `test -f`. For a docs-containing skeleton stage with no compiler gate, this verification is essential.

**Required fix**: Add `test -f docs/implementation/platform-backend-skeleton.md` to the validation commands section.

### Finding 2 — severity: medium — `docs/README.md` update verification missing

**Location**: Validation Commands section.

**Issue**: The plan requires updating `docs/README.md` with the new backend skeleton document. The acceptance criteria do not explicitly list this, and the validation commands do not include a check that the index was updated (e.g., `grep -q platform-backend-skeleton docs/README.md`). Per the documentation lifecycle rules (agent.md lines 85-87), formal docs updates must be synchronized with index.

**Required fix**: Add `grep -q platform-backend-skeleton docs/README.md` to validation commands, or equivalently add an explicit acceptance criterion that `docs/README.md` includes the new document entry.

### Finding 3 — severity: medium — `.gitignore` update not validated

**Location**: Validation Commands section.

**Issue**: The plan says to "Add or update `.gitignore` for generated backend runtime data if a non-ignored runtime directory is introduced." The validation commands include `git status --short` but this only shows tracked changes, not whether ignored patterns are correct. There's no verification that generated runtime data from tests is actually ignored. Per the repository hygiene risk (plan line 139), committed runtime data is a risk.

**Required fix**: Add a validation step: after running tests, verify no generated runtime data appears in `git status --short`. If a project runtime directory is introduced (e.g., `var/platform-backend/`), add `git check-ignore var/platform-backend/` to confirm the gitignore entry is effective.

### Finding 4 — severity: low — `--port 0` behavior undefined

**Location**: Validation Commands section, optional smoke test.

**Issue**: The optional smoke test uses `--port 0`, but the plan never defines what this value means. Does the server auto-assign an ephemeral port and print it? Does it bind to port 0 (which is invalid)? Does it exit with an error? The command as written (`node scripts/platform-backend/server.js --host 127.0.0.1 --port 0 --data-dir ...`) would start a server and hang indefinitely, requiring manual termination.

**Suggested fix**: Define `--port 0` semantics in the server spec. Either (a) document that `--port 0` means "auto-assign an ephemeral port, print it to stdout, then run" or (b) change the smoke command to use a specific port with a timeout wrapper, e.g., `timeout 5 node ... --port 18990 & sleep 2 && curl http://127.0.0.1:18990/health; kill %1`.

### Finding 5 — severity: low — No explicit schema alignment validation

**Location**: Acceptance criteria and validation commands.

**Issue**: The plan says "Records should align with Stage 15 field names where practical" and "Add tests or schema checks for required field names on core records." The validation commands use `node --test` which can cover this, but there is no explicit validation that key field names match the Stage 15 model (e.g., `user_id` not `id`, `chain_id` not `chain`). A schema drift could silently create mismatches.

**Suggested fix**: Add a dedicated schema validation test or a static check step that core record field names match the Stage 15 type definitions in `scripts/platform-model/types.ts`.

### Finding 6 — severity: low — Node.js `node:test` availability not qualified

**Location**: Validation Commands section.

**Issue**: `node --test` was experimental until Node 19 and stable from Node 20 LTS. The plan does not specify a minimum Node version. If the implementation environment uses an older Node, the validation would fail. `agent.md` does not declare a Node version requirement.

**Suggested fix**: Add a note to the plan about the minimum Node version (>=20 for stable `node:test` support, or >=18 for experimental). The `node --check` commands work on any modern Node version, so the primary concern is the test runner.

### Finding 7 — severity: low — Monitor decoupling not validated

**Location**: Non-Goals and validation commands.

**Issue**: The plan explicitly states "Stage 18 should not silently couple the platform backend skeleton to the monitor package." The validation commands do not include a check for this (e.g., verifying no `require('...monitor/...')` or `import ... from '...monitor/...'` exists in the backend code).

**Suggested fix**: Add `! rg -q 'monitor/' scripts/platform-backend/` to validation commands, or add a task item to verify no monitor coupling.

## Decision

**`approved-with-required-fixes`**

The plan has a concrete, bounded goal matching the roadmap; scope and non-goals are well-defined and prevent scope creep into Stage 19/20; current facts are verified and honest (especially the `chain_role` metadata status and `verified_at: null` admission); stop conditions are comprehensive and appropriate; risks are explicitly documented; and the dependency-free approach using Node built-in modules is consistent with the project's current `package.json` state. The plan is executable by an implementation agent without architectural invention.

Three issues must be resolved before implementation:

## Required Fixes

1. **Formal doc verification (F1)**: Add `test -f docs/implementation/platform-backend-skeleton.md` to the validation commands section. Per Stage 12 lessons learned, every Execution Record file claim requires explicit verification.

2. **`docs/README.md` update verification (F2)**: Add `grep -q platform-backend-skeleton docs/README.md` to validation commands, or add an explicit acceptance criterion that the index entry exists. Per agent.md documentation lifecycle rules, formal docs must be synchronized with the index.

3. **`.gitignore` effectiveness verification (F3)**: After the test run, verify that no generated runtime data appears in `git status --short`. If a project runtime directory (e.g., `var/platform-backend/`) is introduced, add `git check-ignore var/platform-backend/` to confirm the pattern is active. The repository hygiene risk is explicitly documented in the plan.

## Suggested Improvements

1. Define `--port 0` semantics in the server spec, or replace the optional smoke command with a specific-port + timeout wrapper.
2. Add a schema field-name alignment check against `scripts/platform-model/types.ts`.
3. Note minimum Node version (>=20 for stable `node:test`, >=18 for experimental).
4. Add a monitor-decoupling verification step (`! rg -q 'monitor/' scripts/platform-backend/`).
5. Consider adding a `timeout 5` startup then `curl /health` smoke command to the optional validation section instead of the unbounded `--port 0` variant.
6. The chain-event import path safety test is described in the task list but could be called out more explicitly in validation commands with a specific command (e.g., a test that attempts to import `/etc/passwd` and expects rejection).

## Risks If Unchanged

1. **Missing doc verification (F1)**: Per Stage 12 lessons, documentation-only deliverables without explicit file-existence checks are the highest risk for false completion claims. The Execution Record could claim the doc was created without evidence.

2. **Orphaned formal doc (F2)**: If `docs/README.md` is not updated, the new backend skeleton document becomes undiscoverable from the project index, violating the documentation lifecycle contract (agent.md lines 85-87).

3. **Commited runtime data (F3)**: Without a proper `.gitignore` check, test-generated JSON data or server runtime state could be accidentally committed, polluting the repository with non-code artifacts.

4. **`--port 0` ambiguity (F4)**: An implementation agent might implement `--port 0` as literal port 0 (a reserved port), as auto-assign, or as a validation error. Without specification, the smoke test is not reproducible.

## Questions for Codex/Owner

1. Should the backend skeleton use `--port 0` for ephemeral port auto-assignment (print port then run), or should the smoke test use a fixed port with timeout?
2. Should the stage also commit a small fixture file (e.g., sample events for import testing) under `scripts/platform-backend/test/fixtures/`, or should import tests generate their own data?
3. The plan requires tests "for store behavior, auth flow, protected route rejection, task/evidence/job creation, chain event import, and path safety." Should there be a minimum test count or coverage expectation, or is functional coverage sufficient?

## Verification Performed

```
# Branch verification
git branch --show-current                           → stage/stage18-web-backend-skeleton
git status --short                                   → (clean)

# File existence checks
test -f docs/architecture/platform-business-model.md → yes (380 lines)
test -f docs/implementation/data-trade-cli-api-boundary.md → yes (345 lines)
test -f docs/implementation/chain-event-indexer-state-sync.md → yes (333 lines)
test -f scripts/platform-model/types.ts              → yes (248 lines)
test -f scripts/chain_event_indexer.js               → yes
test -f package.json                                 → yes (6 lines, type: module)
test -f monitor/package.json                         → yes (separate service)
test -f .gitignore                                   → yes (53 lines)

# Absence checks (correct for planning stage)
test -f docs/implementation/platform-backend-skeleton.md    → NOT_FOUND (correct)
test -d scripts/platform-backend/                           → NOT_FOUND (correct)

# Git checks
git log --oneline -5
  → e9ce4dd docs(stage18): plan web backend skeleton
  → b12305b merge: Stage 17 chain event indexer
  → ... (branch is one commit ahead of Stage 17 merge)

# Roadmap alignment
rg "Stage 18" docs/internal/agent-plans/2026-06-28-data-flow-platform-long-term-roadmap.md
  → lines 250-279: Web 后端最小骨架 with matching core functions

# chain_role metadata (Stage 17 local-only, not platform ChainEvent field)
rg "chain_role" docs/implementation/chain-event-indexer-state-sync.md
  → line 140: "Stage 17 script-local metadata"
  → line 153: "not part of the Stage 15 platform ChainEvent object"

# Stage 15 ChainEvent model — no chain_role
rg "chain_role" docs/architecture/platform-business-model.md
  → no matches (confirmed: not a platform ChainEvent field)
rg "chain_role" scripts/platform-model/types.ts
  → no matches (confirmed)
```
