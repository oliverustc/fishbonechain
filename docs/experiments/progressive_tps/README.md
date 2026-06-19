# Progressive TPS N=1..3 Precheck

This precheck was run before further N=5/N=6 runtime work to validate the experiment harness against the current deployed child chains.

## Run

- run_id: `progressive_n1_3_reset_each_20260619_1105`
- branch: `exp/progressive-tps-main-load`
- workload: `WORKERS=200`, `PARALLEL_PER_WORKER=4`, `DURATION=180`, `CAPACITY_CAP=10000`, `DATA_SIZE=64`
- procedure: reset active child chains before each N, then setup tasks/workers and run one stage
- raw_dir: `docs/experiments/progressive_tps/progressive_tps_runs/progressive_n1_3_reset_each_20260619_1105`
- summary_csv: `docs/experiments/progressive_tps/progressive_tps_summary.csv`

## Results

| N | Active Chains | Accepted Submissions | Aggregate TPS | Conservative TPS | Status |
|---:|---|---:|---:|---:|---|
| 1 | child1 | 10000 | 115.25 | 115.25 | hit cap |
| 2 | child1 + child2 | 20000 | 197.65 | 188.29 | hit cap |
| 3 | child1 + child2 + child3 | 26406 | 242.18 | 88.05 | child3 did not hit cap |

For N=3, child1 and child2 both reached 10000 submissions, while child3 reached 6406 submissions in the 299.904 second monitor window. The child3 partial chain rate was about 21.36 TPS, so child3 is the immediate baseline bottleneck.

## Findings

- The first run without reset was invalid because N=2 and N=3 inherited filled epoch state from earlier stages. The launcher now supports `RESET_EACH_STAGE=1`, `SETUP_EACH_STAGE=1`, and `WAIT_COLLECTING=1`.
- `submit_mode=pool` worker `ok` counts are not reliable accepted-submission metrics. The authoritative metric is the chain-side `capacity_monitor.js` precise CSV.
- The summarizer now includes partial precise CSV deltas for chains that do not hit cap, so failed stages still report measured accepted workload.
- Mainchain pressure was not measured in this precheck because `metrics_main.js` failed when sampling a pruned old block state. The monitor now starts from the current best block and samples subsequent blocks.

## Decision

Do not proceed directly to deeper N=5/N=6 runtime optimization before addressing the N=1..3 baseline. The next run should use the integrated clean-stage command:

```bash
RESET_EACH_STAGE=1 SETUP_MAX_WORKERS=200 WORKERS=200 PARALLEL_PER_WORKER=4 DURATION=180 N_START=1 N_END=3 bash scripts/run_exp_progressive_tps.sh
```

Follow-up child1 tuning found that `DATA_SIZE=8` reaches 152.12 TPS at the same 10000-submission cap. See `child1_tuning.md`.
