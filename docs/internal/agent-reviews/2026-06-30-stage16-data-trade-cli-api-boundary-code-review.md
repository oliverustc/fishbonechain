# Stage 16 Code Review: Data Trade CLI / API Boundary

Date: 2026-06-30
Reviewer: Codex
Plan: `docs/internal/agent-plans/2026-06-30-stage16-data-trade-cli-api-boundary.md`
Branch: `stage/stage16-data-trade-cli-api-boundary`
Reviewed head: `421d45f`

## Scope Reviewed

- `scripts/data_trade_cli.js`
- `docs/implementation/data-trade-cli-api-boundary.md`
- `docs/implementation/data-trade-implementation.md`
- `docs/README.md`
- Stage 16 plan and plan-review records
- Stage 14 validation wrapper compatibility

## Inputs Read

- `agent.md`
- `docs/internal/agent-collaboration.md`
- `.agents/skills/fwf/references/workflow-common.md`
- `.agents/skills/fwf/references/code-review-prompt.md`
- `docs/internal/agent-plans/2026-06-30-stage16-data-trade-cli-api-boundary.md`
- `docs/implementation/data-trade-cli-api-boundary.md`
- `scripts/data_trade_cli.js`
- `git diff --stat main...HEAD`
- `git diff --name-only main...HEAD`

## Findings

No required issues found.

The implementation stays within the Stage 16 boundary: it adds a CLI/API surface, implements only no-live `inspect`, no-live `generate-proof`, and `run-flow`, and leaves independent chain-mutating commands as planned/non-executable. No pallet, runtime, proof digest, settlement, deployment, backend, or dependency changes were introduced.

## Required Fixes

None.

## Accepted Risks

- Live-chain `run-flow` scenarios were not rerun during review because RPC readiness was not available and the plan only required no-live compatibility validation.
- `generate-proof` and `run-flow --dry-run-dynamic` delegate to `scripts/zk_real_data_trade_flow.js` and therefore inherit the existing ZK workspace behavior under `target/data-trade-zk/session-0-round-0`.
- A review harness mistake ran multiple proof-producing commands concurrently. One Stage 14 dry-run scenario failed because the shared ZK workspace was overwritten while verification was in progress. The same Stage 14 validation wrapper passed when rerun serially, which matches the script's normal execution model.

## Verification Performed

- `git status --short --branch`
- `git diff --check main...HEAD`
- `node --check scripts/data_trade_cli.js`
- `node --check scripts/zk_real_data_trade_flow.js`
- `bash -n scripts/run_data_trade_validation.sh`
- `node --check scripts/lib/data_trade_validation_summary.js`
- `node scripts/data_trade_cli.js --help`
- `node scripts/data_trade_cli.js inspect --help`
- `node scripts/data_trade_cli.js generate-proof --help`
- `node scripts/data_trade_cli.js run-flow --help`
- `node scripts/data_trade_cli.js publish-listing --help`
- `node scripts/data_trade_cli.js create-request --help`
- `node scripts/data_trade_cli.js create-escrow --help`
- `node scripts/data_trade_cli.js open-session --help`
- `node scripts/data_trade_cli.js submit-delivery --help`
- `node scripts/data_trade_cli.js settle --help`
- `node scripts/data_trade_cli.js dispute --help`
- `node scripts/data_trade_cli.js publish-listing; test "$?" -eq 1`
- `node scripts/data_trade_cli.js inspect profile --profile child6-data-trade --out .agents/fwf/runs/stage16/review/inspect-profile.json`
- `test -f .agents/fwf/runs/stage16/review/inspect-profile.json`
- `node scripts/data_trade_cli.js generate-proof --profile child6-data-trade --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json --request scripts/fixtures/data_trade_requests/factory_temperature_range.json --evidence-out .agents/fwf/runs/stage16/review/generate-proof-evidence.json`
- `test -f .agents/fwf/runs/stage16/review/generate-proof-evidence.json`
- `node scripts/data_trade_cli.js inspect evidence --evidence .agents/fwf/runs/stage16/review/generate-proof-evidence.json --out .agents/fwf/runs/stage16/review/inspect-evidence.json`
- `test -f .agents/fwf/runs/stage16/review/inspect-evidence.json`
- `node scripts/data_trade_cli.js run-flow --profile child6-data-trade --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json --request scripts/fixtures/data_trade_requests/factory_temperature_range.json --evidence-out .agents/fwf/runs/stage16/review/run-flow-evidence.json --dry-run-dynamic`
- `test -f .agents/fwf/runs/stage16/review/run-flow-evidence.json`
- `scripts/run_data_trade_validation.sh --skip-live --out .agents/fwf/runs/stage16/review/stage14-compat-skip-live-serial`
- `rg -n "data-trade-cli-api-boundary" docs/README.md docs/implementation/data-trade-implementation.md`
- `rg -n "publish-listing|create-request|create-escrow|open-session|generate-proof|submit-delivery|settle|dispute|inspect|run-flow" docs/implementation/data-trade-cli-api-boundary.md`

## Branch And Commit Assessment

- Branch is correctly named `stage/stage16-data-trade-cli-api-boundary`.
- Implementation commit `f174a77` is scoped to the CLI boundary and formal docs.
- Plan execution record was finalized in `421d45f`.
- Generated validation artifacts remain under `.agents/fwf/runs/stage16/...` and were not committed.

## Decision

`approved`

