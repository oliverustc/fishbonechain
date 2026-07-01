# Code Review: Stage 19 Offchain Job Executor

**Date**: 2026-07-01
**Reviewer**: Codex
**Plan**: `docs/internal/agent-plans/2026-06-30-stage19-offchain-job-executor.md`
**Implementation commit**: `dcfe328`
**Branch**: `stage/stage19-offchain-job-executor`
**Decision**: `approved-with-required-fixes`

## Scope Reviewed

Reviewed the Stage 19 implementation against the revised plan and plan re-review approval:

- executor CLI and helper modules under `scripts/platform-backend/`;
- executor tests under `scripts/platform-backend/test/job_executor.test.js`;
- formal docs updates under `docs/implementation/` and `docs/README.md`;
- latest Execution Record in the plan.

## Inputs Read

- `agent.md`
- `docs/internal/agent-collaboration.md`
- `docs/internal/agent-plans/2026-06-30-stage19-offchain-job-executor.md`
- `docs/internal/agent-reviews/2026-07-01-stage19-offchain-job-executor-plan-review.md`
- `docs/internal/agent-reviews/2026-07-01-stage19-offchain-job-executor-plan-review-v2.md`
- `scripts/platform-backend/job_executor.js`
- `scripts/platform-backend/lib/job_runner.js`
- `scripts/platform-backend/lib/job_types.js`
- `scripts/platform-backend/lib/safe_paths.js`
- `scripts/platform-backend/lib/artifact_digest.js`
- `scripts/platform-backend/test/job_executor.test.js`
- `docs/implementation/offchain-job-executor.md`
- `docs/implementation/platform-backend-skeleton.md`
- `docs/implementation/implementation-record.md`
- `docs/README.md`
- `git diff --stat main..HEAD`
- `git diff --name-status main..HEAD`
- `git status --short`

## Findings

### F1 Required: Dry-run accepts unsafe dataset/request paths and still creates successful Evidence

`scripts/platform-backend/lib/job_runner.js:132` only runs dataset/request real-path validation when `dryRun` is false. Dry-run mode validates that `workflow_run`, `profile`, `dataset`, and `request` references exist as artifact types, but it does not validate that `dataset` and `request` are safe repo-local paths before writing `completed` job metadata and `category: "dry_run"` Evidence.

This violates the plan requirements that `--dry-run` "should validate inputs" and that "unsafe input path ... produces a `failed` job with an error and no false success evidence." It also contradicts `docs/implementation/offchain-job-executor.md`, which says dry-run "Validates all four input artifacts" and that input paths are validated with real-path resolution.

Reproduction performed during review:

```text
node --input-type=module <inline script creating a dry-run proof_generation job with dataset=/etc/passwd and request=/etc/passwd>
→ {"error":null,"status":"completed","evidence_id":"808ce74c-b430-4b50-b3e1-828af6e837bc","output_refs":1}
```

Required fix:

- Move dataset/request path safety validation so it runs for both dry-run and real execution.
- Add a regression test proving dry-run fails with no Evidence when `dataset` or `request` is outside the repository or resolves outside via symlink.
- Ensure the failure records `status: "failed"`, a concise `error`, `started_at`, and `completed_at`.

### F2 Required: Work directory validation allows generated outputs in tracked repository paths

`scripts/platform-backend/lib/safe_paths.js:24` to `scripts/platform-backend/lib/safe_paths.js:35` accepts any path under the repository as `--work-dir` and creates it. That allows executor output under tracked or normally committable directories such as `docs/<job-id>/` or `scripts/<job-id>/`.

The plan acceptance criteria require "Runtime outputs are written only under ignored repo-local paths." The validation output paths section also directs Stage 19 output to ignored `.agents/fwf/runs/stage19/...`. The current implementation does reject paths outside the repository, but it does not enforce an ignored output root.

Required fix:

- Constrain `--work-dir` to an ignored repo-local runtime root, preferably `.agents/fwf/runs/stage19/...` for Stage validation and existing ignored runtime roots such as `var/platform-backend/` only if explicitly documented.
- Add a regression test proving a non-ignored repo-local work root fails without creating job output or successful Evidence.
- Update formal docs if the accepted output roots are narrower than the current CLI text.

## Positive Checks

- The implementation keeps Stage 19 scope narrow: no daemon, no scheduler, no external service, no chain signing, no dependency or lockfile changes.
- Evidence `run_id` is resolved through an existing `workflow_runs` record before Evidence creation.
- Dry-run Evidence uses `category: "dry_run"`, `scenario: "executor-dry-run"`, and `result: "executor-dry-run-completed"`.
- Real proof-generation Evidence uses `category: "postcheck"` and invokes the existing `scripts/data_trade_cli.js generate-proof` boundary with `spawnSync` and no shell.
- Failed missing `workflow_run` cases do not create Evidence.
- Formal docs and docs index were updated.

## Verification Performed

```text
git branch --show-current
→ stage/stage19-offchain-job-executor

git status --short
→ clean

git diff --stat main..HEAD
→ Stage 19 plan/review records, executor code/tests, and formal docs only

node --check scripts/platform-backend/server.js
node --check scripts/platform-backend/job_executor.js
find scripts/platform-backend/lib -name '*.js' -print -exec node --check {} \;
→ all syntax checks passed

node --test scripts/platform-backend/test/*.test.js
→ 88 tests passed, 0 failed

node scripts/platform-backend/job_executor.js --help
→ printed expected help

test -f docs/implementation/offchain-job-executor.md
grep -q offchain-job-executor docs/README.md
grep -q offchain-job-executor docs/implementation/platform-backend-skeleton.md
grep -q offchain-job-executor docs/implementation/implementation-record.md
! rg -q 'child_process.*shell:\s*true|shell:\s*true' scripts/platform-backend/
→ passed

Manual unsafe dry-run reproduction
→ dry-run accepted /etc/passwd dataset and request paths, completed the job, and created Evidence
```

## Tests Not Run

- Optional real proof-generation smoke was not run; no `fishbone-zk` / `ZK_VERIFIER_CMD` executable was available in this review environment.

## Branch and Commit Assessment

- Branch is correct: `stage/stage19-offchain-job-executor`.
- Implementation commit `dcfe328` references the plan and validation commands.
- Working tree was clean before writing this review record.
- The implementation should not merge until F1 and F2 are fixed and re-reviewed.

## Required Fixes

1. Apply dataset/request real-path safety validation in dry-run mode, not only real execution.
2. Enforce ignored repo-local work roots for generated executor outputs and add regression coverage.

## Accepted Risks

- Real proof generation remains environment-dependent on `fishbone-zk`; dry-run and failure behavior are the appropriate Stage 19 validation baseline.
- The JSON store remains single-process and development-grade, as documented.

## Decision

`approved-with-required-fixes`

The implementation is close and the architecture matches the plan, but the two path-safety/data-hygiene issues are required acceptance criteria and must be fixed before merge.
