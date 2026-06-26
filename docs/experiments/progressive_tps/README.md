# Progressive TPS N=1..3 Precheck

This precheck validates the experiment harness against the current deployed child chains before deeper N=5/N=6 runtime work.

## Current Tuned Run

- run_id: `progressive_n1_3_data8_20260619_114714`
- branch: `exp/progressive-tps-main-load`
- workload: `WORKERS=200`, `PARALLEL_PER_WORKER=4`, `DURATION=180`, `CAPACITY_CAP=10000`, `DATA_SIZE=8`
- procedure: reset active child chains before each N, setup tasks/workers, then run one stage
- raw_dir: `docs/experiments/progressive_tps/progressive_tps_runs/progressive_n1_3_data8_20260619_114714`
- summary_csv: `docs/experiments/progressive_tps/progressive_tps_summary.csv`

| N | Active Chains | Accepted Submissions | Aggregate TPS | Conservative TPS | Status |
|---:|---|---:|---:|---:|---|
| 1 | child1 | 10000 | 149.65 | 149.65 | single-chain target reached |
| 2 | child1 + child2 | 20000 | 293.62 | 288.55 | close to 2-chain target |
| 3 | child1 + child2 + child3 | 30000 | 365.20 | 192.56 | child3 is the bottleneck |

The N=1 target is effectively reached with the compact 8B business digest payload. N=3 remains below the desired 450 TPS because child3 needs 155.795s to reach 10000 accepted submissions, while child1 and child2 finish near 66-67s.

See `child1_tuning.md` for the N=1 payload finding and `child3_tuning.md` for the child3 diagnosis.

## Earlier 64B Run

- run_id: `progressive_n1_3_reset_each_20260619_1105`
- branch: `exp/progressive-tps-main-load`
- workload: `WORKERS=200`, `PARALLEL_PER_WORKER=4`, `DURATION=180`, `CAPACITY_CAP=10000`, `DATA_SIZE=64`
- procedure: reset active child chains before each N, then setup tasks/workers and run one stage
- raw_dir: `docs/experiments/progressive_tps/progressive_tps_runs/progressive_n1_3_reset_each_20260619_1105`
- summary_csv: `docs/experiments/progressive_tps/progressive_tps_summary.csv`

## Earlier 64B Results

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
- For child3, distributed RPC and pressure-parameter tuning alone did not reach 150 TPS. `watch` mode restored near-2s blocks but exposed per-block capacity decline over a full epoch.

## Decision

Do not proceed directly to final N=5/N=6 runtime optimization before deciding how to handle the N=3 baseline gap. The integrated clean-stage command for the current tuned baseline is:

```bash
RESET_EACH_STAGE=1 SETUP_MAX_WORKERS=200 WORKERS=200 PARALLEL_PER_WORKER=4 DURATION=180 DATA_SIZE=8 N_START=1 N_END=3 bash scripts/run_exp_progressive_tps.sh
```

Follow-up child1 tuning found that `DATA_SIZE=8` reaches about 150 TPS at the same 10000-submission cap. Child3 tuning found that the current `f7/f8/f9` deployment is not expected to reach 150 TPS without deployment remapping or runtime storage optimization.
