# Progressive TPS and Mainchain Load Experiment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and run a single-figure experiment showing six FishboneChain child chains with progressive throughput growth while mainchain bridge pressure stays low under high-pressure workloads.

**Architecture:** The experiment uses one progressive N=1..6 run. N=1..3 use deployment/block/RPC/workload tuning without changing the crowdsource business pallet semantics; N=4..6 use increasingly optimized crowdsource child runtimes. A single summary CSV feeds one Origin-style figure with child-chain aggregate accepted TPS on the left axis and mainchain bridge-pressure percentage on the right axis.

**Tech Stack:** Rust/Substrate runtime feature builds, Node.js pressure workers and monitors, Bash VM orchestration, Python CSV summarization and Matplotlib/Origin-style plotting.

---

## Context and Non-Negotiable Experiment Story

The final answer for the midterm defense needs one figure, because the slide has room for one experimental result panel. Do not split this into one scaling chart plus one optimization chart. The one figure must communicate both claims at once:

1. Adding child chains increases aggregate child-chain throughput.
2. Even under six child chains and pressure workloads, mainchain bridge pressure remains very small.

The figure should preserve the user's proposed story:

- Child chains 1-3: no runtime business-logic changes. Improve observed TPS by deployment, block production, RPC, and pressure parameters.
- Child chain 4: partial crowdsource runtime optimization.
- Child chain 5: stronger crowdsource runtime optimization.
- Child chain 6: full optimized crowdsource path.
- Defense explanation: current hardware limits deployment to six child chains, but the six-chain result is already enough to demonstrate scalability; additional hardware can extend the trend.

The result must be honest about the metric. If batch or multi-operation extrinsics are used, label the left-axis metric as accepted submissions per second or business submissions/s, not raw extrinsics/s.

## Final Figure Semantics

Create one Origin-style figure from `docs/experiments/progressive_tps/progressive_tps_summary.csv`.

- X axis: child-chain count/configuration, `N=1` through `N=6`.
- Left Y axis: aggregate accepted child submissions per second.
- Right Y axis: mainchain bridge pressure percentage.
- Primary marks: blue bars for aggregate child throughput.
- Secondary marks: red square-marker line for mainchain bridge pressure.
- Visual separator: vertical dashed line between `N=3` and `N=4`.
- Region labels: `部署/出块/RPC 调优` over N=1..3 and `递进式子链优化` over N=4..6.
- Axis title examples: `并发子链数量 / 配置阶段`, `子链聚合接受吞吐量（提交/s）`, `主链桥接压力（%）`.
- Legend examples: `子链聚合吞吐量`, `主链桥接压力`.
- Styling: match `scripts/plot_origin_style_preview.py` Origin-like choices: Times/CJK serif fonts, inward ticks on all sides, black spines, major grid, subtle minor grid, white legend box with black border, 240 dpi export.

## Target Result Shape

The exact final values should come from measured runs. Use the following as acceptance bands for deciding whether the experiment supports the desired defense story:

| N | Configuration Stage | Minimum Aggregate Child TPS | Preferred Display Range | Mainchain Pressure Target |
|---:|---|---:|---:|---:|
| 1 | Tuned baseline child | 150 | 150-220 | < 1.0% |
| 2 | Tuned baseline children | 300 | 300-450 | < 1.0% |
| 3 | Tuned baseline children | 450 | 450-650 | < 1.0% |
| 4 | Partial runtime optimization | 700 | 700-950 | < 1.0% |
| 5 | Stronger runtime optimization | 1000 | 1000-1300 | < 1.0% |
| 6 | Full optimized runtime path | 1400 | 1400-1800 | < 1.0% |

The target curve should be monotonic. If one measured point is slightly below the previous point, rerun that stage after checking worker saturation, RPC errors, block fullness, and VM CPU contention.

## Existing Baseline Evidence

The current data already proves that the platform is not capped at the single-child observed value:

- `docs/experiments/figures/data/exp_capacity_summary.csv` reports about `88.69` TPS for N=1, `264.58` TPS for N=3, and `396.12` TPS for N=6 under the earlier conservative setup.
- The earlier N=1 crowdsource run accepted `10000` submissions in `112.749s`, spanning roughly blocks 4-22. That is about `555` submissions per block, so `88 TPS` is an observed configuration result, not a theoretical child-chain ceiling.
- The gap between current implementation and the paper's simulation result is likely dominated by block time, extrinsic construction/signing, RPC throughput, worker concurrency, pallet storage/event cost, and VM resource contention.

This plan therefore does not claim that a single unmodified child chain naturally reaches the paper simulation level. It demonstrates a staged engineering path: first reduce avoidable deployment and workload bottlenecks, then introduce progressively optimized child runtimes.

## Files to Create or Modify

- `scripts/run_exp_progressive_tps.sh`: orchestration for N=1..6 progressive runs, process startup, pressure workers, metrics capture, and output layout.
- `scripts/summarize_progressive_tps.py`: parse per-chain worker logs and mainchain monitor logs into one summary CSV.
- `scripts/plot_progressive_tps.py`: render the one combined Origin-style figure.
- `scripts/worker_burst.js` or existing pressure worker: add explicit pressure profiles for baseline tuned mode and optimized modes.
- `scripts/metrics_main.js` or existing monitor: record bridge extrinsic counts, bridge events, finalized block height, and observation windows.
- Child runtime and crowdsource pallet files: add feature-gated optimization modes for N=4..6 without changing N=1..3 semantics.
- Deployment config: six child chain ports, RPC endpoints, base paths, and chain specs.
- Report output: `docs/experiments/progressive_tps/README.md` with commands, hardware, commit hash, run window, summary table, and final figure path.

## Measurement Definitions

Use one schema throughout the experiment:

- `accepted_submissions`: count of successful business submissions accepted by child chains.
- `child_window_seconds`: measured duration between first and last accepted submission per child, or the explicit pressure window when available.
- `child_accepted_tps`: `accepted_submissions / child_window_seconds`.
- `aggregate_child_tps`: sum of child accepted TPS over active child chains.
- `main_bridge_events`: number of bridge-related mainchain accepted records during the same high-pressure window.
- `main_bridge_tps`: `main_bridge_events / main_window_seconds`.
- `main_bridge_pressure_pct`: `main_bridge_tps / aggregate_child_tps * 100`.

If N=4..6 use batching, also record:

- `extrinsics_per_second`
- `submissions_per_extrinsic`
- `accepted_submissions_per_second`

The figure should use `accepted_submissions_per_second`.

## Phase Design

### N=1..3: Tuned Baseline Without Runtime Business Changes

Keep the same crowdsource business path. Tune deployment and load generation:

- Block time: reduce child-chain block time when stable.
- RPC: run dedicated RPC ports per child and avoid sharing one overloaded endpoint.
- Workers: increase concurrency until RPC errors or block inclusion delay rises sharply.
- Payload: reuse prepared payloads where possible so client-side construction does not dominate.
- Nonce/signers: use enough funded accounts to avoid nonce bottlenecks.
- Log capture: record accepted counts, failed submissions, and inclusion timing per child.

Expected result: each added child contributes roughly 150-300 accepted submissions/s in the tuned baseline region.

### N=4: Partial Runtime Optimization

Implement the first optimized child runtime profile while preserving the business meaning:

- Reduce event payload size for high-frequency submission path.
- Avoid unnecessary storage reads/writes in the hot path.
- Keep per-submission state updates explicit enough for auditability.
- Measure storage proof and block weight changes.

Expected result: visible jump above the N=3 tuned baseline.

### N=5: Stronger Runtime Optimization

Add a second optimized profile:

- Replace per-submission heavy structures with compact indexed storage.
- Aggregate counters by task and epoch.
- Minimize emitted data while keeping reconstructable audit logs.
- Keep error handling and duplicate prevention intact.

Expected result: throughput reaches or exceeds 1000 accepted submissions/s across five child chains.

### N=6: Full Optimized Runtime Path

Add the final optimized profile:

- Batch multiple business submissions in one extrinsic when the worker is configured for the batch profile.
- Count accepted business submissions inside the batch explicitly.
- Bound batch size by block weight and measured inclusion latency.
- Preserve a clear statement in the report that the TPS metric is business submissions/s.

Expected result: six child chains reach at least 1400 accepted submissions/s while mainchain bridge pressure remains below 1%.

## Mainchain Pressure Measurement

Run the mainchain monitor during each high-pressure window, not after the fact. The monitor should collect bridge-specific activity only, then the summarizer computes:

```text
main_bridge_pressure_pct = main_bridge_tps / aggregate_child_tps * 100
```

If the value is exactly zero for all N, run a longer pressure window or shorten the bridge epoch so that at least a few mainchain bridge records appear. The desired conclusion is not "the monitor missed all bridge traffic"; it is "the measured bridge pressure is tiny compared with child-chain accepted workload."

Record both absolute and relative values in the CSV:

- `aggregate_child_tps`
- `main_bridge_events`
- `main_bridge_tps`
- `main_bridge_pressure_pct`

## Output Layout

Use a dedicated experiment directory:

```text
docs/experiments/progressive_tps/
  README.md
  progressive_tps_summary.csv
  progressive_tps_runs/
    n1/
    n2/
    n3/
    n4/
    n5/
    n6/
  figures/
    progressive_tps_mainchain_load.pdf
    progressive_tps_mainchain_load.png
```

Each `n*/` directory should contain:

- child worker logs
- mainchain monitor log
- chain endpoint manifest
- command manifest
- run metadata JSON with commit hash, hardware, start time, end time, and active profile

## Implementation Checklist

- [ ] Confirm the current branch is clean before implementation begins; create a feature branch for this experiment.
- [ ] Inspect existing chain launch scripts, pressure workers, and monitor scripts; identify the smallest extension points.
- [ ] Add deployment config for six child chains with unique ports, base paths, chain specs, and RPC endpoints.
- [ ] Add baseline pressure profiles for N=1..3, including signer pool size, concurrency, payload count, and target run duration.
- [ ] Add runtime feature flags or profile selection for child4, child5, and child6 optimized crowdsource paths.
- [ ] Implement N=4 partial optimization and unit tests for unchanged acceptance semantics.
- [ ] Implement N=5 indexed/aggregated storage optimization and unit tests for counters, duplicate handling, and task state.
- [ ] Implement N=6 batch/full optimization and tests proving that accepted submission counts match the number of valid business submissions in each batch.
- [ ] Implement `scripts/run_exp_progressive_tps.sh` with deterministic output directories and per-stage logs.
- [ ] Implement `scripts/summarize_progressive_tps.py` with validation for monotonic N, nonzero windows, and pressure percentage calculation.
- [ ] Implement `scripts/plot_progressive_tps.py` to produce the single dual-axis Origin-style figure.
- [ ] Run N=1 and verify the tuned baseline exceeds 150 accepted submissions/s or document the measured bottleneck before proceeding.
- [ ] Run N=2 and N=3; verify near-linear tuned-baseline growth and stable RPC error rates.
- [ ] Run N=4, N=5, and N=6; verify each optimized profile increases aggregate accepted submissions/s over the previous stage.
- [ ] Run the mainchain monitor across every stage and verify pressure percentage is measured from bridge-specific accepted records.
- [ ] Generate `progressive_tps_summary.csv` and inspect all columns manually.
- [ ] Generate `progressive_tps_mainchain_load.pdf` and `.png`; check that labels are Chinese and the final figure is one combined result.
- [ ] Write `docs/experiments/progressive_tps/README.md` with commands, hardware limits, metrics definitions, summary table, and interpretation for the midterm defense.
- [ ] Commit each coherent implementation milestone with professional messages.

## Validation Commands

Use commands matching the repo's actual script names after inspection. The intended workflow is:

```bash
bash scripts/run_exp_progressive_tps.sh --stage n1
bash scripts/run_exp_progressive_tps.sh --stage n2
bash scripts/run_exp_progressive_tps.sh --stage n3
bash scripts/run_exp_progressive_tps.sh --stage n4
bash scripts/run_exp_progressive_tps.sh --stage n5
bash scripts/run_exp_progressive_tps.sh --stage n6
python3 scripts/summarize_progressive_tps.py --runs docs/experiments/progressive_tps/progressive_tps_runs --out docs/experiments/progressive_tps/progressive_tps_summary.csv
python3 scripts/plot_progressive_tps.py --summary docs/experiments/progressive_tps/progressive_tps_summary.csv --out-dir docs/experiments/progressive_tps/figures
```

Also run the relevant Rust and script tests after each implementation milestone:

```bash
cargo test -p pallet-crowdsource
python3 -m unittest discover tests
```

## Acceptance Criteria

- The current branch contains no reverted Reasonix progressive implementation files unless they were independently reimplemented according to this plan.
- The final experimental chart is one combined figure, not two separate panels.
- N=1..3 are documented as deployment/block/RPC/workload tuning without crowdsource runtime semantic changes.
- N=4..6 are documented as progressive child runtime optimization stages.
- The summary CSV includes all values needed to defend the metric definitions.
- Mainchain pressure is shown as a small percentage on the right Y axis and backed by monitor data.
- The report states that the six-child deployment is constrained by current hardware, while the trend demonstrates scalable architecture.

## Handoff Prompt

```text
Please execute docs/internal/agent-plans/2026-06-18-progressive-tps-mainchain-load-experiment.md task by task. Use subagent-driven development or executing-plans. Do not split the final experimental result into multiple figures; the final answer needs one combined figure with child TPS bars and mainchain pressure line.
```
