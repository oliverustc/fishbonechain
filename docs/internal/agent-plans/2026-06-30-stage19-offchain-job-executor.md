# Stage 19 Plan: Offchain Job Executor

## Goal

Add a minimal, dependency-free offchain job executor that can pick a queued `OffchainJob` from the Stage 18 platform backend store, execute the data-trade `proof_generation` job type through the existing no-chain `generate-proof` boundary, and update job/evidence metadata without introducing private-key handling, chain signing, protocol changes, or production worker claims.

## Background

The long-term roadmap defines Stage 19 as "链下任务执行器":

- abstract proof generation, data processing, and future training into a unified job model;
- support job types `proof_generation`, `data_preprocessing`, `anonymization`, `verification`, and `training`;
- each job records at least `input`, `status`, `worker`, `artifact_path`, `digest`, `error`, `created_at`, `completed_at`, and `evidence_id`;
- data-trade proof generation is the first concrete job type.

Current repository facts verified before writing this plan:

- Stage 18 is merged into `main` at `2ad73af` and added a dependency-free backend skeleton under `scripts/platform-backend/`.
- `docs/implementation/platform-backend-skeleton.md` states the Stage 18 backend is file-backed, development-grade, not a chain signer, and currently treats offchain jobs as metadata stubs.
- `scripts/platform-backend/lib/json_store.js` has `create`, `list`, `find`, `findOne`, and `update` helpers over JSON collection files.
- `scripts/platform-backend/lib/schema.js` defines current `OffchainJob` fields as `job_id`, `job_type`, `status`, `input_refs`, `output_refs`, `worker_id`, `digest`, `error`, `evidence_id`, `created_at`, `started_at`, and `completed_at`.
- `scripts/platform-backend/lib/routes.js` currently exposes `POST /api/offchain-jobs` and `GET /api/offchain-jobs`, and job creation is metadata-only.
- `docs/architecture/platform-business-model.md` defines `OffchainJob` as a platform object and explicitly says job completion is not proof of correctness without output digest verification.
- `docs/implementation/data-trade-cli-api-boundary.md` defines `generate-proof` as the implemented no-live-chain offchain ZK pipeline. It delegates to `scripts/zk_real_data_trade_flow.js --dry-run-dynamic` and requires `--profile`, `--dataset`, `--request`, and `--evidence-out`.
- `scripts/data_trade_cli.js generate-proof` is the current CLI boundary for the no-chain proof pipeline.
- `package.json` remains dependency-light and Stage 18 intentionally used Node built-in modules only.
- `.gitignore` ignores `/var/platform-backend/` and `.agents/`.
- Current branch for this plan: `stage/stage19-offchain-job-executor`.

## Scope

Implementation may add or update:

- a worker CLI under `scripts/platform-backend/job_executor.js`;
- worker helper modules under `scripts/platform-backend/lib/`, for example:
  - `job_runner.js`;
  - `job_types.js`;
  - `artifact_digest.js`;
  - `safe_paths.js` if path safety is not already factored cleanly enough;
- Stage 18 backend route/schema/store code only where needed for executor-safe status transitions or job inspection, such as:
  - `GET /api/offchain-jobs/:id`;
  - stricter `OffchainJob` validation;
  - optional job metadata fields nested under existing `input_refs` / `output_refs` rather than broad schema redesign;
- focused Node tests under `scripts/platform-backend/test/`;
- a formal implementation document, preferably `docs/implementation/offchain-job-executor.md`;
- `docs/implementation/platform-backend-skeleton.md` only to link forward to the new executor behavior and clarify that jobs are no longer metadata-only when the executor is run;
- `docs/implementation/implementation-record.md` with a brief Stage 19 entry;
- `docs/README.md` index entry for the new formal executor document;
- `.gitignore` entries for generated executor runtime output if a new non-ignored runtime directory is introduced;
- this plan's Execution Record.

Recommended file layout:

```text
scripts/platform-backend/
  job_executor.js
  lib/
    job_runner.js
    artifact_digest.js
  test/
    job_executor.test.js
docs/implementation/offchain-job-executor.md
```

## Non-Goals

- Do not build a production daemon, queue server, scheduler, process supervisor, retry service, or distributed worker pool.
- Do not add dependencies, package managers, databases, message queues, Docker services, or background service managers unless the owner explicitly approves first.
- Do not execute chain transactions, hold private keys, sign extrinsics, or call live-chain data-trade flows.
- Do not change pallets, runtime, proof digest fields, verifier assumptions, settlement rules, chain specs, deployment topology, experiment metrics, or paper-facing measured data.
- Do not implement Stage 20 data-trade Web API.
- Do not implement full `data_preprocessing`, `anonymization`, `verification`, or `training` engines. They may be accepted as typed queued jobs or rejected as unsupported execution types, but only `proof_generation` should actually execute in Stage 19.
- Do not claim `proof_generation` completion as chain finality or on-chain Groth16 verification.
- Do not replace `scripts/data_trade_cli.js` or `scripts/zk_real_data_trade_flow.js`.

## Current Facts

- Stage 18 backend stores `offchain_jobs` as JSON array records. `JsonStore.update()` can update the first matching record and write the collection atomically via rename.
- Stage 18 `OffchainJob` schema has no explicit generic `input`, `artifact_path`, or metadata field. The existing `input_refs` and `output_refs` arrays are the safest place to encode file-based inputs and outputs without breaking Stage 15 names.
- Stage 15 `OffchainJob` fields already map to Stage 19 roadmap concepts:
  - `input` -> `input_refs`;
  - `worker` -> `worker_id`;
  - `artifact_path` -> `output_refs[].path`;
  - `digest` -> `digest`;
  - `error` -> `error`;
  - `created_at` / `completed_at` -> existing timestamp fields;
  - `evidence_id` -> existing `evidence_id`.
- The current proof-generation CLI can be exercised without chain RPC using `node scripts/data_trade_cli.js generate-proof ... --evidence-out <path>`.
- The proof-generation CLI still depends on a working `fishbone-zk` binary via `ZK_VERIFIER_CMD` or profile config. In environments without the binary, unit tests should mock command execution instead of pretending the proof pipeline ran.
- Stage 18 `Evidence` schema requires `run_id`, and `workflow_runs` already exists as a backend collection. Stage 19 should use an existing `WorkflowRun` reference instead of changing the schema or creating placeholder workflow runs.
- Stage 18 tests use `.agents/fwf/runs/stage18/backend-test/`; Stage 19 validation should use `.agents/fwf/runs/stage19/...`.

## Design Constraints

Use Node built-in modules only:

- `node:fs`, `node:path`, and `node:crypto` for file, path, and digest handling;
- `node:child_process` for invoking the existing CLI;
- `node:test` and `node:assert` for tests.

Executor CLI behavior should be explicit and bounded:

```text
node scripts/platform-backend/job_executor.js --data-dir <dir> --job-id <id> --work-dir <dir> [--worker-id <id>] [--dry-run]
node scripts/platform-backend/job_executor.js --data-dir <dir> --once --work-dir <dir> [--worker-id <id>] [--dry-run]
node scripts/platform-backend/job_executor.js --help
```

Required semantics:

- `--data-dir` points to the Stage 18 backend JSON store.
- `--job-id` runs one specific queued job.
- `--once` finds one queued job and runs at most one job, then exits.
- `--work-dir` is repo-local or under `.agents/fwf/runs/stage19/...`; all generated evidence/artifacts/logs go under a per-job subdirectory.
- `--worker-id` defaults to a stable local identifier such as `local-worker`.
- `--dry-run` must not execute the proof CLI; it should validate inputs, transition the job through `running` to `completed`, write a deterministic dummy artifact/evidence file under `--work-dir`, compute its digest, and create/update metadata. This is for executor mechanics only and must be documented as not a proof result. Dry-run output must be inspectably marked with `executor_dry_run: true`, and the created Evidence must use `scenario: "executor-dry-run"` and `result: "executor-dry-run-completed"`.

`proof_generation` job input contract should be concrete and fit current `input_refs`:

```json
{
  "job_type": "proof_generation",
  "status": "queued",
  "input_refs": [
    {"artifact_type": "workflow_run", "path": "run-id-from-workflow-runs", "digest": null},
    {"artifact_type": "profile", "path": "child6-data-trade", "digest": null},
    {"artifact_type": "dataset", "path": "scripts/fixtures/data_trade_datasets/factory_sensors.json", "digest": null},
    {"artifact_type": "request", "path": "scripts/fixtures/data_trade_requests/factory_temperature_range.json", "digest": null}
  ],
  "output_refs": [],
  "worker_id": null,
  "digest": null,
  "error": null,
  "evidence_id": null
}
```

For `artifact_type: "workflow_run"`, `path` is a platform `run_id`, not a filesystem path. Before creating Evidence, the executor must look up `workflow_runs` by that `run_id`; if the referenced run is missing, the job must fail with a concise error and no Evidence record. The executor must not create a placeholder `WorkflowRun`, must not create Evidence with a null or reserved `run_id`, and must not change the `Evidence` schema to bypass this dependency.

For `artifact_type: "profile"`, `path` is the profile string passed to `scripts/data_trade_cli.js generate-proof --profile`. The executor may perform lightweight validation against `scripts/profiles/chains.json` when that is simple, but it must not duplicate the full CLI profile-loading behavior or change the Stage 16 CLI contract.

The executor should map this to:

```bash
node scripts/data_trade_cli.js generate-proof \
  --profile <profile> \
  --dataset <dataset_path> \
  --request <request_path> \
  --evidence-out <work-dir>/<job-id>/evidence.json
```

On success:

- job `status` becomes `completed`;
- `worker_id`, `started_at`, and `completed_at` are set;
- `output_refs` includes at least the evidence JSON path with `artifact_type: "evidence"`;
- `digest` records a digest of the evidence output file;
- a corresponding `Evidence` record is created in the backend store with the validated `run_id`, or linked if a valid existing Evidence reference is explicitly supported by the implementation;
- `evidence_id` points to that record.

Evidence categories must be explicit:

- dry-run executor Evidence uses `category: "dry_run"`, `scenario: "executor-dry-run"`, and `result: "executor-dry-run-completed"`;
- real off-chain proof-generation Evidence uses `category: "postcheck"` because it records off-chain proof output without chain finality or live-chain verification.

On failure:

- job `status` becomes `failed`;
- `error` records a concise error summary;
- `completed_at` is set;
- no false evidence success record is created.
- if the failure is a missing or invalid `workflow_run` reference, no Evidence record is created.

## Risks

- Security risk: a worker that accepts paths can read or write outside intended workspace. The implementation must use real-path checks and constrain repo-local input paths and repo-local ignored output paths.
- Data integrity risk: Stage 18 JSON store is single-process. Executor tests should run sequentially, and docs must state the worker is not safe for concurrent multi-worker production use.
- Evidence validity risk: `--dry-run` executor tests can be mistaken for real proof generation. Dry-run artifacts must be clearly marked and never described as ZK proof evidence.
- Proof correctness risk: real `proof_generation` depends on `fishbone-zk`. If the binary is absent, validation must not claim real proof execution.
- Scope risk: Stage 19 can drift into Stage 20 APIs or a production queue. Keep it to one-shot execution and one concrete job type.
- Compatibility risk: changing `OffchainJob` schema can break Stage 18 tests. Prefer compatible use of `input_refs` and `output_refs`.
- Repository hygiene risk: generated job outputs can be committed accidentally. Use `.agents/fwf/runs/stage19/...` and ignored runtime paths.

## Stop Conditions

The implementation agent must stop and ask Codex or the owner before:

- adding dependencies, lockfile changes, external services, databases, queue servers, or daemon supervisors;
- storing private keys, seed phrases, API secrets, or signer material;
- invoking live-chain flows or any chain transaction from the executor;
- changing data-trade proof digest fields, verifier attestation payloads, settlement behavior, or bridge trust assumptions;
- changing Stage 15 platform model field semantics in a non-backward-compatible way;
- changing Stage 16 CLI contract instead of wrapping it;
- claiming real proof generation when `ZK_VERIFIER_CMD` / `fishbone-zk` was not actually executed successfully;
- writing validation evidence outside repo-local ignored paths;
- broadening implementation beyond `proof_generation` execution and typed stubs for future job types.

## Branch and Commit Plan

- Branch: `stage/stage19-offchain-job-executor`.
- Implementation should stay on this branch.
- Commit after a coherent implementation pass only when:
  - worker code, tests, docs, and this plan's Execution Record are updated;
  - validation commands below have run or skipped commands are explicitly justified;
  - `git status --short` contains only intended Stage 19 changes and ignored local runtime data.

Recommended implementation commit message:

```text
feat(backend): add offchain job executor

Plan: docs/internal/agent-plans/2026-06-30-stage19-offchain-job-executor.md
Validation:
- node --check scripts/platform-backend/job_executor.js
- node --test scripts/platform-backend/test/*.test.js
```

## Task List

- [ ] Re-read `agent.md`, `docs/internal/agent-collaboration.md`, this plan, `docs/architecture/platform-business-model.md`, `docs/implementation/platform-backend-skeleton.md`, and `docs/implementation/data-trade-cli-api-boundary.md`.
- [ ] Confirm the branch is `stage/stage19-offchain-job-executor` and record `git status --short`.
- [ ] Add a dependency-free executor entrypoint at `scripts/platform-backend/job_executor.js` with `--help`, `--data-dir`, `--job-id`, `--once`, `--work-dir`, `--worker-id`, and `--dry-run`.
- [ ] Add executor helper logic to load the Stage 18 JSON store, find queued jobs, validate job type, and perform safe status transitions.
- [ ] Define and test the `proof_generation` input contract using required `input_refs` artifact types `workflow_run`, `profile`, `dataset`, and `request`.
- [ ] Resolve the required `workflow_run` input by treating `input_refs[].path` as a `run_id`, verifying that `workflow_runs` contains that record, and failing the job without Evidence if it does not.
- [ ] Implement path safety for input files and output work directories using real paths. Reject unsafe paths outside the repository or non-ignored configured work roots.
- [ ] Implement dry-run execution that writes a clearly marked dummy evidence JSON under `.agents/fwf/runs/stage19/...` with `executor_dry_run: true`, and updates job/evidence metadata without invoking proof tools.
- [ ] Implement real `proof_generation` execution by spawning `node scripts/data_trade_cli.js generate-proof ... --evidence-out <path>` and capturing exit code/stdout/stderr into per-job logs.
- [ ] Compute a deterministic digest, preferably SHA-256 hex, for the generated evidence file and store it in job `digest` plus `output_refs[].digest`.
- [ ] Create an `Evidence` record for completed jobs with Stage 15-compatible fields, the validated `run_id`, and explicit category values: `category: "dry_run"` for executor dry-runs and `category: "postcheck"` for real off-chain proof generation. Link it to `job.evidence_id`.
- [ ] Ensure failed jobs record `status: "failed"`, `error`, `worker_id`, `started_at`, and `completed_at` without creating successful evidence.
- [ ] Keep future job types (`data_preprocessing`, `anonymization`, `verification`, `training`) non-executable for now, returning a clear unsupported-job error and leaving/marking job state consistently.
- [ ] Add tests for successful dry-run `proof_generation`, missing input rejection, unsupported type behavior, failure transition, digest/output_refs/evidence_id linkage, symlink/path escape rejection, and `--once` job selection.
- [ ] Add a help/syntax validation path for the executor.
- [ ] Update or extend backend docs:
  - add `docs/implementation/offchain-job-executor.md`;
  - update `docs/README.md`;
  - update `docs/implementation/platform-backend-skeleton.md` to reference Stage 19 executor behavior;
  - update `docs/implementation/implementation-record.md` with a brief Stage 19 entry.
- [ ] Update this plan's Execution Record with files changed, tests run, deviations, validation output paths, and remaining risks.
- [ ] Run all validation commands and inspect generated output paths before committing.

## Acceptance Criteria

- A queued `proof_generation` job with a valid `workflow_run` input can be executed in `--dry-run` mode, transitions to `completed`, writes repo-local output containing `executor_dry_run: true`, records a digest, and links an `Evidence` record with `category: "dry_run"`, `scenario: "executor-dry-run"`, and `result: "executor-dry-run-completed"`.
- Real off-chain proof-generation success links Evidence with `category: "postcheck"` and the validated `run_id`; it must not claim chain finality or live-chain verification.
- A missing required input, missing referenced `workflow_runs` record, unsafe input path, unsafe output path, unsupported job type, or failed spawned command produces a `failed` job with an error and no false success evidence.
- The executor can run a specified `--job-id` or one queued job via `--once`.
- Real proof execution path is implemented as a wrapper around the existing Stage 16 `generate-proof` CLI and does not change that CLI contract.
- Tests cover executor mechanics without requiring live chain RPC or a real ZK binary, including `workflow_run` lookup, Evidence category assignment, and dry-run marker fields.
- Formal docs describe executor commands, input contract, workflow-run dependency, output layout, Evidence linkage/category behavior, trust boundaries, and known limitations.
- No new package dependency or lockfile changes are introduced.
- Runtime outputs are written only under ignored repo-local paths.

## Validation Commands

Run from repository root:

```bash
git branch --show-current
git status --short
node --check scripts/platform-backend/server.js
node --check scripts/platform-backend/job_executor.js
find scripts/platform-backend/lib -name '*.js' -print -exec node --check {} \;
node --test scripts/platform-backend/test/*.test.js
node scripts/platform-backend/job_executor.js --help
test -f docs/implementation/offchain-job-executor.md
grep -q offchain-job-executor docs/README.md
grep -q offchain-job-executor docs/implementation/platform-backend-skeleton.md
grep -q offchain-job-executor docs/implementation/implementation-record.md
! rg -q 'child_process.*shell:\\s*true|shell:\\s*true' scripts/platform-backend/
git status --short
```

If a new project runtime directory is introduced, validate its ignore rule with `git check-ignore <path>`.

Optional real proof-generation smoke, only if `ZK_VERIFIER_CMD` points to an executable `fishbone-zk` binary:

```bash
mkdir -p .agents/fwf/runs/stage19/real-proof-smoke
node scripts/platform-backend/job_executor.js \
  --data-dir .agents/fwf/runs/stage19/real-proof-smoke/data \
  --job-id <seeded-proof-generation-job-id> \
  --work-dir .agents/fwf/runs/stage19/real-proof-smoke/work
```

If the optional real proof smoke is not run, record the reason in the Execution Record. Do not claim real proof execution without command output.

## Validation Output Paths

Use repo-local ignored output paths:

```text
.agents/fwf/runs/stage19/executor-test/
.agents/fwf/runs/stage19/dry-run-smoke/
.agents/fwf/runs/stage19/real-proof-smoke/
```

Do not write validation outputs to `/tmp` unless a tool has no usable repo-local output option. Do not commit `.agents/fwf/runs/`.

## Documentation Updates

Required:

- Add `docs/implementation/offchain-job-executor.md`.
- Update `docs/README.md` implementation/development index.
- Update `docs/implementation/platform-backend-skeleton.md` to point from Stage 18 metadata-only jobs to Stage 19 executor behavior.
- Update `docs/implementation/implementation-record.md` with a brief Stage 19 entry.

The formal executor document must include:

- status and scope for Stage 19;
- executor CLI commands and examples;
- `proof_generation` job input contract, including the required `workflow_run` input reference and profile handling;
- output files, digest behavior, and Evidence linkage/category behavior;
- dry-run vs real proof execution distinction;
- unsupported future job types;
- trust boundaries and non-goals;
- validation commands and known limitations.

Do not update experiment reports, deployment runbooks, paper gap matrices, chain architecture claims, or measured results unless implementation actually changes them. This stage should not change them.

## Execution Record

### YYYY-MM-DD opencode Pass N

- Branch:
- Commits:
- Tasks completed:
- Files changed:
- Tests run:
- Tests not run:
- Validation output paths:
- Deviations from plan:
- Questions for Codex/Owner:
- Remaining risks:

## Plan-Review Focus

opencode should review:

- whether Stage 19 is narrow enough to avoid building a production queue/daemon;
- whether the `proof_generation` input contract using `input_refs` is concrete and compatible with Stage 15/18 objects, including the required `workflow_run` reference for Evidence `run_id`;
- whether dry-run executor behavior is clearly separated from real proof execution;
- whether path-safety and generated-output hygiene requirements are strong enough;
- whether the validation commands can prove executor mechanics without requiring a live chain or a real ZK binary;
- whether any formal docs beyond `offchain-job-executor.md`, `platform-backend-skeleton.md`, `implementation-record.md`, and `docs/README.md` must be updated.

## Plan Review Resolution

Stage 19 opencode plan review `docs/internal/agent-reviews/2026-07-01-stage19-offchain-job-executor-plan-review.md` returned `approved-with-required-fixes`. This revision applies the required fixes and selected suggestions:

- F1 applied: executor-created Evidence categories are now explicit. Dry-run Evidence must use `category: "dry_run"`, `scenario: "executor-dry-run"`, and `result: "executor-dry-run-completed"`; real off-chain proof-generation Evidence must use `category: "postcheck"`.
- F2 applied: `proof_generation` jobs must include a `workflow_run` input reference whose `path` is an existing `workflow_runs.run_id`. The executor must verify it before Evidence creation and fail the job without Evidence if missing. The plan rejects placeholder workflow runs, null `run_id`, and Evidence schema changes.
- S1 accepted: the helper module is named `scripts/platform-backend/lib/job_runner.js` to avoid confusing it with the CLI entrypoint.
- S2 accepted: profile `input_refs[].path` is the `--profile` string for the existing CLI, with optional lightweight validation only.
- S3 accepted: dry-run output must include `executor_dry_run: true` in addition to explicit Evidence scenario/result markers.
- S4 accepted: `docs/implementation/implementation-record.md` is now a required documentation update and validation target.

The revised plan is ready for opencode re-review. Implementation must wait for opencode to approve this revised plan unless the owner explicitly overrides the re-review gate.
