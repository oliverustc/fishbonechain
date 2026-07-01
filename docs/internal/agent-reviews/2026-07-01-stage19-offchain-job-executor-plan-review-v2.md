# Plan Review (Re-review): Stage 19 Offchain Job Executor

**Date**: 2026-07-01
**Reviewer**: opencode
**Plan**: `docs/internal/agent-plans/2026-06-30-stage19-offchain-job-executor.md`
**Planned commit**: `46761f2` (original) → `ff7b570` (revised)
**Previous review**: `docs/internal/agent-reviews/2026-07-01-stage19-offchain-job-executor-plan-review.md` (`6054555`) — `approved-with-required-fixes` (F1, F2)
**Branch**: `stage/stage19-offchain-job-executor`

## Resolution of Previous Findings

### F1 — Evidence category specified ✅

Original finding: clean plan lacked explicit `Evidence.category` values.
Resolution (lines 161–164, 230, 245–246):
- Dry-run Evidence: `category: "dry_run"`, `scenario: "executor-dry-run"`, `result: "executor-dry-run-completed"`
- Real off-chain proof Evidence: `category: "postcheck"`
- Task item 10 and acceptance criteria now name both values explicitly.
- On-failure semantics (lines 166–172) preserve the guard that no false success Evidence is created.

### F2 — Evidence run_id dependency resolved ✅

Original finding: Evidence schema requires `run_id`; the plan gave no mechanism to provide it.
Resolution (lines 90, 124–129, 138–139, 225, 230, 247):
- New current fact documents that `Evidence` schema requires `run_id` and `workflow_runs` collection exists.
- `proof_generation` input contract now requires `artifact_type: "workflow_run"` with `path` as a `run_id`.
- Before Evidence creation, executor must look up `workflow_runs` by that `run_id`; missing reference → job fails with error, no Evidence created.
- Rejected approaches explicitly: no placeholder `WorkflowRun`, no null `run_id`, no schema changes.
- New task item 5 to implement this resolution.
- Acceptance criteria updated.

### S1 — Helper module naming ✅

Helper module renamed from `lib/job_executor.js` → `lib/job_runner.js` (lines 36, 58).

### S2 — Profile resolution ✅

`artifact_type: "profile"` path is the `--profile` string for the CLI; optional lightweight validation against `scripts/profiles/chains.json` allowed but must not duplicate CLI behavior (line 141).

### S3 — Dry-run metadata discrimination ✅

Dry-run output must include `executor_dry_run: true` in evidence JSON, plus explicit `scenario`/`result` fields (line 117, 163, 227, 245).

### S4 — implementation-record.md update ✅

Added as required documentation update (lines 47, 239) and validation target (line 270).

## Re-verification of Current Facts

```text
git branch --show-current    → stage/stage19-offchain-job-executor
git status --short            → (clean)
git log --oneline -3          → confirms plan revision at ff7b570
```

All previously verified facts remain current. The plan now accurately documents that `Evidence` schema requires `run_id` and that `workflow_runs` collection exists, both confirmed in `schema.js` and `json_store.js`.

## New Material Reviewed

The `## Plan Review Resolution` section (lines 349–360) accurately describes how each finding was resolved. All six fixes (F1, F2, S1–S4) are reflected in the body of the plan with matching line-level changes.

## Assessment

The revised plan is concrete, executable, and leaves no architectural gaps for the implementer:

- **Goal**: still concrete and bounded — one-shot executor for `proof_generation`
- **Scope**: specific files, modules, and docs named; no ambiguity
- **Non-goals**: unchanged, still sufficient
- **Stop conditions**: unchanged, still sufficient
- **Input contract**: now fully specified (4 required artifact types including `workflow_run`)
- **Evidence resolution**: both `run_id` source and `category` values are explicit
- **Dry-run discrimination**: inspectable via `executor_dry_run: true`, explicit scenario/result markers
- **Task list**: updated with new task (item 5) for workflow_run resolution; all tasks ordered and scoped
- **Acceptance criteria**: updated for all new requirements; measurable
- **Validation commands**: updated (line 270) for `implementation-record.md` grep; otherwise unchanged and sufficient
- **Documentation updates**: all four required docs listed
- **Risk section**: unchanged, still thorough

No hidden scope expansion detected. No security, proof, settlement, or deployment assumption changes.

## Decision

**`approved`**

All required fixes from the previous review have been applied correctly. All suggested improvements have been accepted. The plan is ready for opencode implementation.

## Verification Performed

```text
git branch --show-current    → stage/stage19-offchain-job-executor
git status --short            → (clean)
git log --oneline -3          → ff7b570, 6054555, 46761f2
test -f scripts/fixtures/data_trade_datasets/factory_sensors.json → OK
test -f scripts/fixtures/data_trade_requests/factory_temperature_range.json → OK
git diff 46761f2..ff7b570     → diff inspected, all six changes confirmed
```
