# Stage 14 Code Review: Data Trade Validation Runner

日期：2026-06-29
审查者：Codex
计划文件：`docs/internal/agent-plans/2026-06-28-stage14-data-trade-reproducible-validation.md`
审查分支：`stage/stage14-data-trade-validation`
审查范围：`main..HEAD` through `ed1afa6`

## Findings

### F1 (Required): `--skip-live` validation cannot satisfy the plan's required passed status

`scripts/run_data_trade_validation.sh:401-418` records all live scenarios and postcheck as `skipped` for `--skip-live`, then changes `OVERALL_STATUS` from `passed` to `partial`.

The plan's Step 5 explicitly requires the no-chain quick validation command:

```bash
scripts/run_data_trade_validation.sh --skip-live --out /tmp/fishbone-stage14-dry-run
```

to produce:

- three dry-run scenarios passed;
- two negative validations passed;
- `summary.json` status `passed`;
- readable `summary.md`.

Actual evidence from `.agents/fwf/runs/stage14/dry-run/summary.json` and `.agents/fwf/runs/stage14/final-check/summary.json` shows top-level `"status": "partial"` despite all dry-run and negative scenarios passing. This prevents the required validation command from meeting acceptance criteria and makes the Stage 14 runner report a successful no-chain validation as partial.

Required fix:

- For explicit `--skip-live`, treat the requested scope as dry-run + negative validation and keep top-level status `passed` when all executed scenarios pass.
- If live scenarios are still listed in `summary.json`, their `skipped` status must not force top-level `partial` for an intentional `--skip-live` run. Alternatively, omit live scenarios from the summary for scoped no-chain runs, but keep documentation consistent.
- Re-run `scripts/run_data_trade_validation.sh --skip-live --out .agents/fwf/runs/stage14/<new-path>` and record that `summary.json.status == "passed"`.

### F2 (Required): Full live validation can abort after happy path due to top-level `local`

`scripts/run_data_trade_validation.sh:340-342` executes:

```bash
if [[ "$LIVE_HAPPY_PASSED" == "1" ]]; then
  local scenario spec sid sdir sname sevidence scmd sedge
```

This block is at script top level, not inside a function. In bash, `local` is only valid inside a function. With `set -e`, the script will abort immediately after a successful live happy path and before running `invalid-proof-dispute`, `invalid-plaintext-dispute`, `requester-refuses-payment`, or postcheck.

I verified this shell behavior with:

```bash
bash -lc 'if [[ "1" == "1" ]]; then local x; fi'
```

which prints `local: can only be used in a function` and exits `1`.

Required fix:

- Replace the top-level `local` declaration with ordinary assignments/declarations, or move the live scenario dispatch into a function where `local` is valid.
- Re-run `bash -n scripts/run_data_trade_validation.sh`.
- If live RPC is unavailable, at minimum add a targeted shell test or review evidence showing the failure/dispute dispatch block can execute without the `local` runtime error. The preferred validation is the full live command when child6 is available.

## Required Fixes

1. Fix `--skip-live` status semantics so the plan's required no-chain validation produces top-level `passed` when dry-run and negative validation pass.
2. Remove the invalid top-level `local` in the live failure/dispute dispatch path.
3. Update the plan Execution Record after fixes with exact commands and observed results, including the corrected `summary.json.status`.

## Suggested Improvements

- Consider adding a small internal helper for the three live failure/dispute scenarios instead of repeating the `spec` parsing block. This is optional; the required issue is the invalid `local` and validation behavior.
- Consider escaping pipe characters in summary markdown table fields beyond `error`, especially `events`, if future event strings include separators. Not required for current evidence.

## Accepted Risks

- Live-chain scenarios were not rerun in the implementation pass because child6 RPC availability was unknown. This is acceptable only if the runner correctly records skipped live scenarios when readiness fails and does not claim live evidence without running it.
- `docs/implementation/data-trade-evidence.md` was intentionally left unchanged. The Execution Record gives a reasonable plan-compliant explanation because newer formal docs now point to Stage 13/14 current evidence.

## Verification Performed

- Read `agent.md`.
- Read `docs/internal/agent-collaboration.md`.
- Read plan and latest Execution Record in `docs/internal/agent-plans/2026-06-28-stage14-data-trade-reproducible-validation.md`.
- Reviewed commits:
  - `1a9d1ef test: add reproducible data trade validation runner`
  - `70e9f86 docs: document data trade validation evidence`
  - `ed1afa6 record: update Stage 14 Execution Record with commit hashes`
- Inspected:
  - `scripts/run_data_trade_validation.sh`
  - `scripts/lib/data_trade_validation_summary.js`
  - `docs/experiments/data-trade-validation.md`
  - `docs/implementation/data-trade-stage14-evidence-index.md`
  - updated formal docs under `docs/implementation/` and `docs/README.md`
- Ran:
  - `git status --short --branch`
  - `git log --oneline --decorate --reverse main..HEAD`
  - `git diff --name-status main..HEAD`
  - `node --check scripts/lib/data_trade_validation_summary.js`
  - `bash -n scripts/run_data_trade_validation.sh`
  - `node --check scripts/zk_real_data_trade_flow.js`
  - `git diff --check main..HEAD`
  - `jq '{status, scenarios: [.scenarios[] | {id, category, status, result, error}]}' .agents/fwf/runs/stage14/dry-run/summary.json`
  - `jq '{status, scenarios: [.scenarios[] | {id, category, status, result, error}]}' .agents/fwf/runs/stage14/final-check/summary.json`
  - `bash -lc 'if [[ "1" == "1" ]]; then local x; fi'`
  - `git ls-files` for new scripts/docs/review files

## Branch and Commit Assessment

- Branch is `stage/stage14-data-trade-validation`.
- Implementation commits are scoped to Stage 14 scripts, docs, and Execution Record.
- The worktree still has an unrelated `.gitignore` modification that predates this review; it was not reviewed and must not be mixed into review-fix commits unless explicitly authorized.
- Process records are committed and reference the plan. The code-review record is the only file written by this review pass.

## Decision

`approved-with-required-fixes`

The branch is not ready for owner-approved merge to `main`. It needs a review-fix pass for F1 and F2, followed by targeted validation and an updated Execution Record.
