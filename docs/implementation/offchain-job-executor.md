# Offchain Job Executor

**Status**: Stage 19 implementation. Minimal, dependency-free offchain job executor that picks queued `OffchainJob` records from the Stage 18 platform backend store, executes the data-trade `proof_generation` job type, and updates job/evidence metadata.

**Date**: 2026-07-01
**Stage**: Stage 19
**Plan**: `docs/internal/agent-plans/2026-06-30-stage19-offchain-job-executor.md`

## 1. Overview

The offchain job executor is a one-shot CLI that reads queued jobs from the Stage 18 JSON file store, executes supported job types, and records results as Evidence. It uses Node.js built-in modules only and does not add dependencies.

Key design points:

- **Single-shot execution.** No daemon, no scheduler, no queue server, no retry service.
- **No chain operations.** The executor does not hold private keys, sign extrinsics, or call live-chain flows.
- **Direct store access.** The executor reads and writes the Stage 18 JSON store files directly. It does not require the backend HTTP server to be running.
- **Evidence-first.** Every completed job creates an Evidence record with explicit category, scenario, and result markers.
- **Dry-run support.** `--dry-run` validates mechanics without invoking proof tools and generates clearly marked dummy evidence.

## 2. Executor CLI

```bash
node scripts/platform-backend/job_executor.js --help
```

### Options

| Option | Description |
|--------|-------------|
| `--data-dir <p>` | Path to Stage 18 backend JSON store directory (required) |
| `--job-id <id>` | Run a specific queued job |
| `--once` | Find one queued job and run at most one job, then exit |
| `--work-dir <p>` | Root for generated evidence/artifacts/logs (required) |
| `--worker-id <s>` | Executing worker identifier (default: `local-worker`) |
| `--dry-run` | Validate inputs without invoking proof CLI |
| `--help` | Print usage and exit |

Exactly one of `--job-id` or `--once` is required.

### Examples

```bash
# Run a specific queued job as dry-run
node scripts/platform-backend/job_executor.js \
  --data-dir var/platform-backend/ \
  --job-id <id> \
  --work-dir .agents/fwf/runs/stage19/dry-run-smoke/work \
  --dry-run

# Pick the first queued job and run it
node scripts/platform-backend/job_executor.js \
  --data-dir var/platform-backend/ \
  --once \
  --work-dir .agents/fwf/runs/stage19/dry-run-smoke/work \
  --dry-run

# Real proof generation (requires fishbone-zk binary)
node scripts/platform-backend/job_executor.js \
  --data-dir var/platform-backend/ \
  --job-id <id> \
  --work-dir .agents/fwf/runs/stage19/real-proof-smoke/work
```

## 3. Supported Job Types

### proof_generation

The only executable job type in Stage 19. Executes the Stage 16 `generate-proof` no-chain off-chain ZK pipeline via `scripts/data_trade_cli.js generate-proof`.

#### Input Contract

A `proof_generation` job requires four input artifacts via `input_refs`:

| `artifact_type` | `path` semantics | Required |
|-----------------|-----------------|----------|
| `workflow_run` | A `run_id` in the `workflow_runs` collection | Yes |
| `profile` | Profile string for `generate-proof --profile` | Yes |
| `dataset` | Filesystem path to dataset JSON fixture | Yes |
| `request` | Filesystem path to request JSON fixture | Yes |

The `workflow_run` reference is used to populate `Evidence.run_id`. The executor looks up the `workflow_runs` collection; if the referenced record is missing, the job fails without creating Evidence.

#### Real Execution

Spawns `node scripts/data_trade_cli.js generate-proof --profile <p> --dataset <d> --request <r> --evidence-out <work-dir>/<job-id>/evidence.json`. On success:

- Job status → `completed`
- `output_refs` includes the evidence JSON path with artifact type `"evidence"`
- `digest` records SHA-256 hex digest of the evidence file
- An Evidence record is created with `category: "postcheck"` (off-chain, no chain finality)

On failure (non-zero exit code, missing output, or spawn error):

- Job status → `failed`
- `error` records a concise error summary
- No Evidence record is created

#### Dry-Run Execution

With `--dry-run`, the executor does NOT invoke the proof CLI. Instead it:

- Validates all four input artifacts
- Resolves the `workflow_run` reference
- Writes a dummy evidence JSON containing `executor_dry_run: true`
- Creates an Evidence record with `category: "dry_run"`, `scenario: "executor-dry-run"`, `result: "executor-dry-run-completed"`

Dry-run output is inspectably distinct from real proof generation output.

### Future Job Types

`data_preprocessing`, `anonymization`, `verification`, and `training` are known but not executable. Attempting to execute them produces a `failed` job with `error: "unsupported execution type: <type>"`.

## 4. Code Layout

```text
scripts/platform-backend/
  job_executor.js          CLI entrypoint
  lib/
    job_runner.js          Core execution orchestrator
    job_types.js           Job type definitions and input validation
    artifact_digest.js     SHA-256 hex digest computation
    safe_paths.js          Real-path resolution and repo-boundary checks
  test/
    job_executor.test.js   25 tests covering executor mechanics
```

## 5. Evidence Categories

| Execution Path | `category` | `scenario` | `result` |
|---------------|------------|------------|----------|
| Dry-run | `dry_run` | `executor-dry-run` | `executor-dry-run-completed` |
| Real proof generation | `postcheck` | `proof_generation` | `completed` |

## 6. Trust Boundaries

- **Executor is not a chain signer.** It never holds private keys, mnemonics, or chain signing material.
- **Job completion is not proof of correctness.** Dry-run evidence is inspectably marked. Real proof generation evidence uses `category: "postcheck"` because it records off-chain proof output without chain finality or live-chain verification.
- **Single-process only.** The JSON store is not safe for concurrent multi-worker use. Executor tests run sequentially.
- **Path safety.** Input paths are validated against the repository root using real-path resolution. Output paths are constrained to the configured work root.
- **No production claims.** This is a development-grade executor. It is not a production daemon, queue server, or distributed worker pool.

## 7. Non-Goals

- No daemon, scheduler, process supervisor, retry service, or distributed worker pool.
- No dependencies, package managers, databases, message queues, or Docker services.
- No chain transactions, private keys, or extrinsics signing.
- No changes to pallets, runtime, proof digest fields, verifier assumptions, or settlement rules.
- No Stage 20 data-trade Web API.
- No full implementation of `data_preprocessing`, `anonymization`, `verification`, or `training` engines.

## 8. Validation

```bash
# Syntax checks
node --check scripts/platform-backend/server.js
node --check scripts/platform-backend/job_executor.js
find scripts/platform-backend/lib -name '*.js' -print -exec node --check {} \;

# Run all tests
node --test scripts/platform-backend/test/*.test.js

# CLI help
node scripts/platform-backend/job_executor.js --help

# Verify no shell injection
! rg -q 'child_process.*shell:\s*true|shell:\s*true' scripts/platform-backend/
```

## 9. Known Limitations

- Single-process only. Concurrent writes to the same JSON store can cause data loss.
- Only `proof_generation` is executable. Other job types are known but not implemented.
- Real proof generation requires a working `fishbone-zk` binary via `ZK_VERIFIER_CMD`.
- No job scheduling, retry, or priority queuing.
- File-backed storage is not suitable for production multi-worker use.

## 10. References

- [Platform Business Model](../architecture/platform-business-model.md) — Stage 15 object model, OffchainJob/Evidence definitions
- [Platform Backend Skeleton](platform-backend-skeleton.md) — Stage 18 backend
- [Data Trade CLI/API Boundary](data-trade-cli-api-boundary.md) — Stage 16 generate-proof CLI
- [Stage 19 Plan](../internal/agent-plans/2026-06-30-stage19-offchain-job-executor.md)
