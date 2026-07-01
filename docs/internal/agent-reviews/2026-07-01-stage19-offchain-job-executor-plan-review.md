# Plan Review: Stage 19 Offchain Job Executor

**Date**: 2026-07-01
**Reviewer**: opencode
**Plan**: `docs/internal/agent-plans/2026-06-30-stage19-offchain-job-executor.md`
**Plan commit**: `46761f2`
**Branch**: `stage/stage19-offchain-job-executor`

## Scope Reviewed

The entire plan, including goal, scope, non-goals, current facts, design constraints, task list, acceptance criteria, validation commands, documentation updates, risks, and stop conditions.

## Inputs Read

- `agent.md` — project context, commands, work conventions
- `docs/internal/agent-collaboration.md` — collaboration protocol
- `docs/internal/agent-plans/2026-06-30-stage19-offchain-job-executor.md` — the plan under review
- `docs/architecture/platform-business-model.md` — Stage 15 object model, OffchainJob/Evidence definitions
- `docs/implementation/platform-backend-skeleton.md` — Stage 18 backend docs
- `docs/implementation/data-trade-cli-api-boundary.md` — Stage 16 generate-proof CLI contract
- `docs/README.md` — current documentation index
- `scripts/platform-backend/lib/json_store.js` — JSON store implementation
- `scripts/platform-backend/lib/schema.js` — schema validation, OffchainJob field definitions
- `scripts/platform-backend/lib/routes.js` — current HTTP routes
- `scripts/platform-backend/test/backend_store.test.js` — existing test patterns
- `scripts/platform-backend/server.js` — entrypoint verified via `--check`
- `scripts/data_trade_cli.js` — generate-proof help verified
- `scripts/profiles/chains.json` — profile `child6-data-trade` verified under `trade_profiles`
- `git log`, `git status --short`, `git branch --show-current` — repository state confirmed

## Verified Current Facts

- Branch `stage/stage19-offchain-job-executor` is active and working tree is clean.
- Stage 18 was merged to `main` at `2ad73af`.
- `JsonStore.update()` exists and uses atomic rename. (confirmed in `json_store.js:70-84`)
- `OffchainJob` schema matches plan's field list. (confirmed in `schema.js:7`)
- Routes expose `POST /api/offchain-jobs` and `GET /api/offchain-jobs` but no `GET /api/offchain-jobs/:id`. (confirmed in `routes.js:211-241`)
- `generate-proof` CLI accepts `--profile`, `--dataset`, `--request`, `--evidence-out` as claimed. (confirmed via `--help` output)
- `.gitignore` covers `var/platform-backend/` and `.agents/`. (confirmed via `git check-ignore`)
- Node.js v22.21.1 is available; all existing platform-backend source files pass `node --check`.

## Findings

### Required Fixes (must resolve before implementation)

**F1 — Evidence `category` unspecified for executor-created records**

The plan states the executor creates an `Evidence` record on success and task item 10 says "mark result/category so it cannot be mistaken for a real ZK proof." However, the plan never specifies the actual `Evidence.category` value(s) to use. The Stage 15 Evidence model (`docs/architecture/platform-business-model.md`) defines `category` as a typed `EvidenceCategory` with values `dry_run | negative | live_chain | postcheck`. Without an explicit assignment:
- An implementer could use a wrong or non-standard category value.
- Dry-run evidence could be misclassified as real proof evidence.
- The schema validator in `schema.js` would not reject an invalid category string because the current validator only checks field presence, not enum membership, but the formal model contract should be respected.

**Required**: Add to the plan (acceptance criteria or task item 10): dry-run executor evidence MUST use `category: "dry_run"`; real execution evidence MUST use an explicit category (recommend `"postcheck"` for off-chain proof generation with no chain finality). Update the Evidence creation task (item 10) to name both category values.

**F2 — Evidence `run_id` dependency not addressed**

The Stage 15 Evidence schema (`schema.js:5`) requires `run_id` as a mandatory field. When the executor creates an Evidence record for a completed job, it must supply a `run_id` referencing a `WorkflowRun`. The current plan:
- Job input contract does not include a `run_id` or `task_id` reference.
- The task for Evidence creation (item 10) says "create an Evidence record ... linking it to job.evidence_id" but does not explain where `run_id` comes from.
- The `OffchainJob` schema has no `run_id` field to carry this reference.
- The plan says "linked if one was already created" (line 152) but does not define how the executor discovers or resolves a pre-existing Evidence record.

**Required**: Clarify one of:
- **(a)** The executor creates a minimal `WorkflowRun` with a placeholder `task_id` before creating Evidence, or
- **(b)** The Evidence record is pre-created externally and referenced via a new `input_refs` artifact or job metadata field, or
- **(c)** The executor links to Evidence but defers `run_id` to a later stage, creating an Evidence record with a null/reserved `run_id` (this would fail current schema validation and require a schema change — note that runtime code changes are a Non-Goal).
Choose and document the resolution before implementation begins.

### Suggested Improvements (non-blocking)

**S1 — Entrypoint vs helper module naming collision**

The scope's recommended file layout lists both `scripts/platform-backend/job_executor.js` (entrypoint) and `scripts/platform-backend/lib/job_executor.js` (helper). While resolvable via different import paths, identical filenames at different directory levels are easy to confuse during development. Consider naming the entrypoint CLI something more distinctive (e.g., `job_executor_cli.js`) or placing it directly in `scripts/platform-backend/job_executor.js` while naming the lib module differently (e.g., `lib/job_runner.js`).

**S2 — Profile resolution mechanism for `input_refs`**

The input contract uses `{"artifact_type": "profile", "path": "child6-data-trade"}`. The executor maps this to `--profile <profile>`. The plan does not specify whether the executor resolves the profile ID through `scripts/profiles/chains.json` or assumes the ID is already valid for the `generate-proof` CLI. This is resolvable at implementation time, but making it explicit reduces decision burden.

**S3 — `--dry-run` job metadata discrimination**

The `--dry-run` flag is a runtime flag, not stored in the job record itself. Two jobs with identical metadata could have different execution paths (one real, one dry-run). Consider adding a `dry_run: true` flag to the job's `output_refs` evidence artifact or to the Evidence record's `scenario` field to make it inspectable independently of the execution log.

**S4 — `docs/implementation/implementation-record.md` update**

The plan's documentation updates section lists `offchain-job-executor.md`, `docs/README.md`, and `platform-backend-skeleton.md` but not `docs/implementation/implementation-record.md`. Stage 19 adds a significant new component (offchain job executor). Consider adding a brief entry to `implementation-record.md` for project trail consistency.

### Accepted Risks (no action required)

The plan's risk section is thorough and covers the key concerns:
- Path safety: plan requires real-path checks.
- JSON store single-process: plan acknowledges sequentially in tests and docs.
- Dry-run vs real proof: plan explicitly requires dry-run artifacts be clearly marked.
- ZK binary dependency: plan provides optional smoke-test gated on `ZK_VERIFIER_CMD`.
- Scope creep into Stage 20: plan includes explicit non-goals.
- Schema compatibility: plan prefers `input_refs`/`output_refs` over schema redesign.

## Decision

**`approved-with-required-fixes`**

The plan is well-written, well-scoped, and grounded in verified repository facts. The two required fixes (Evidence category specification and Evidence `run_id` dependency) must be resolved before implementation to avoid the implementer needing to invent architecture at the Evidence creation boundary. Neither fix invalidates the plan's architecture; both are clarifications that can be addressed in a single plan revision.

## Verification Performed

```text
git branch --show-current    → stage/stage19-offchain-job-executor
git status --short            → (clean)
git log --oneline -5          → confirms plan commit 46761f2 on correct branch
node --check scripts/platform-backend/server.js           → OK
node --check scripts/data_trade_cli.js                    → OK
node --check scripts/platform-backend/lib/json_store.js   → OK
node --check scripts/platform-backend/lib/schema.js       → OK
node --check scripts/platform-backend/lib/routes.js       → OK
node scripts/data_trade_cli.js generate-proof --help      → matches plan input contract flags
git check-ignore var/platform-backend/                    → ignored
git check-ignore .agents/                                 → ignored
```

## Questions for Codex/Owner

1. For F2 (Evidence `run_id`): Should the executor create a placeholder `WorkflowRun` with a sentinel `task_id` to satisfy the Evidence schema, or should Evidence creation be deferred and only `evidence_id` linking be implemented in Stage 19?
2. For real execution Evidence category: is `"postcheck"` the right value for off-chain proof generation with no chain finality, or should a new category be introduced?
