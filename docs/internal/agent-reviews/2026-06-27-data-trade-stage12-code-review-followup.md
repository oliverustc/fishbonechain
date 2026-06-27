# Stage 12 Paper Experiment Freeze Code Review Follow-up

Date: 2026-06-27
Branch: `feat/data-trade-stage12-paper-experiment-freeze`
Reviewed commit: `8824b68 fix: deliver Stage 12 demo guide and evidence index`

## Decision

Changes requested.

The missing Stage 12 deliverables from the first review have been added, and the safe validation results are reproducible. However, several paper-facing metadata statements are still inaccurate and should be fixed before merge.

## Findings

### Medium: Stage 12 docs repeatedly claim 7 demo commands, but the matrix contains 9

The Stage 12 demo matrix currently contains:

- 3 positive dry-run commands;
- 2 negative validation commands;
- 1 live-chain happy path command;
- 3 live-chain failure/dispute commands.

That is 9 demo commands total, not 7.

Files/lines with the incorrect count:

- `docs/implementation/data-trade-evidence.md`: Stage 12 demo guide summary says "7 个可复制的 dry-run/live-chain 命令".
- `docs/implementation/data-trade-implementation.md`: Stage 12 boundary note says "demo guide（7 命令）".
- `docs/internal/agent-plans/2026-06-26-data-trade-long-term-scripted-e2e-roadmap.md`: Stage 12 completion note says "demo guide（7 命令）".
- `docs/internal/agent-plans/2026-06-27-stage12-data-trade-paper-experiment-freeze.md`: execution record says "`data-trade-demo-guide.md` (7 commands, copy-pasteable)".
- Existing plan review text also says "7 commands"; if left unchanged, add a later note clarifying that the final implementation matrix contains 9 commands.

This matters because Stage 12 is the paper experiment freeze. The command count is part of the reproducibility index and should not contradict the actual demo guide/evidence index.

Required fix:

- Change these summaries to "9 commands" or use wording like "complete demo matrix" to avoid count drift.
- If preserving the original plan review as historical text, add a correction note in the Stage 12 execution record rather than rewriting history.

### Low: Stage 12 execution record has the wrong branch name

The execution record in `docs/internal/agent-plans/2026-06-27-stage12-data-trade-paper-experiment-freeze.md` says:

```text
Branch: `feat/data-trade-stage12-paper-freeze`
```

The actual branch is:

```text
feat/data-trade-stage12-paper-experiment-freeze
```

Required fix:

- Update the execution record branch name to the actual branch.

## Positive Checks

The core Stage 12 deliverables now exist and are reviewable:

- `docs/implementation/data-trade-demo-guide.md`
- `docs/implementation/data-trade-stage12-evidence-index.md`
- Stage 12 updates in implementation/evidence/security/roadmap docs.

Safe validation was re-run during review:

```bash
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js
go -C tools/data-trade-zk test ./...
go -C tools/data-trade-zk build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
```

Three positive dry-runs passed using temporary evidence paths under `/tmp/fishbone-stage12-review/`:

- `factory_temperature_range`: `result = dry-run-accepted`, `mode = dynamic-dry-run`, 1 range constraint.
- `factory_multi_range`: `result = dry-run-accepted`, `mode = dynamic-dry-run`, 2 constraints.
- `vehicle_speed_range`: `result = dry-run-accepted`, `mode = dynamic-dry-run`, 1 range constraint.

Two negative validation commands rejected before chain interaction:

- `factory_temperature_out_of_range`: exit code 1, make-witness rejected value 42 outside `[100, 200]`.
- `factory_multi_range_out_of_range`: exit code 1, make-witness rejected pressure 1013 outside `[2000, 3000]`.

No `target/data-trade-stage12/`, `target/data-trade-zk/`, or `.deepseek/` files are tracked by git.

No live-chain E2E was run during this review.

## Merge Status

Not merged. Fix the Stage 12 metadata/count inconsistencies, then request another review.
