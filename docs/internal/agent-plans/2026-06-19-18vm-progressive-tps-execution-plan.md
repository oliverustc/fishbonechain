# 18-VM Progressive TPS Execution Plan

> **For future Codex sessions:** execute this plan step by step. Commit every coherent change. Do not start long N=1..6 runs until deployment mapping, smoke tests, and N=1 single-chain validation are clean.

**Goal:** Produce a better-looking and defensible TPS pressure-test result for the midterm figure: N=1..3 should show near-linear tuned-baseline scaling around 150 TPS per child chain, and N=4..6 should show further throughput jumps from progressively optimized child-chain runtimes while mainchain pressure stays tiny.

**Current State:** f13-f18 are available, reachable, and normalized to the existing deployment convention:

- SSH aliases `f13`-`f18` now log in as `debian`.
- Remote directories exist: `/home/debian/fishbone/{bin,specs,logs}`.
- `debian` has passwordless sudo on f13-f18.
- f13-f18 each have 8 vCPU, about 16GB RAM, 4GB swap, and about 97GB disk.
- f13-f18 are on `10.2.3.11`-`10.2.3.16`; old f1-f12 are on `10.2.2.11`-`10.2.2.22`; the two networks can ping each other.

---

## Experimental Story

The final figure remains one combined Origin-style chart:

- Bars: aggregate accepted child-chain business submissions per second.
- Line: mainchain bridge pressure percentage.
- N=1..3: no crowdsource business-runtime optimization; improve through deployment isolation, block/RPC configuration, and pressure parameters.
- N=4..6: progressively optimized crowdsource runtimes.

Target shape:

| N | Stage | Target Aggregate TPS | Interpretation |
|---:|---|---:|---|
| 1 | tuned baseline | about 150 | single isolated baseline child reaches target |
| 2 | tuned baseline | about 300 | adding a comparable child roughly doubles throughput |
| 3 | tuned baseline | about 450 | three isolated comparable baseline children scale linearly |
| 4 | partial runtime optimization | 700-950 | first optimized child adds a visible jump |
| 5 | indexed/aggregated optimization | 1000-1300 | stronger optimized child raises aggregate curve |
| 6 | batch/full optimization | 1400-1800 | final optimized child demonstrates further scalability |

The metric must remain honest: if N=6 uses batch extrinsics, label the plotted value as `business submissions/s`, not raw extrinsics/s.

## Deployment Principle

The old deployment overlaid multiple high-pressure child chains on the same VMs. That caused child3-style CPU/resource interference. The new deployment should dedicate one 3-validator VM group per child chain:

| Chain | Validators | Runtime Stage | Notes |
|---|---|---|---|
| child1 | f1, f2, f3 | baseline crowdsource | tuned baseline |
| child2 | f4, f5, f6 | baseline crowdsource | tuned baseline |
| child3 | f7, f8, f9 | baseline crowdsource | tuned baseline, no child4 overlap |
| child4 | f10, f11, f12 | runtime v1 | partial optimization |
| child5 | f13, f14, f15 | runtime v2 | indexed/aggregated optimization |
| child6 | f16, f17, f18 | runtime v3 | full/batch optimization |

Mainchain validators may continue to run on all f1-f18, because mainchain pressure is expected to be tiny. If mainchain service load becomes visible, reduce mainchain validators to a representative subset only after documenting the reason.

Do not run pressure workers on validator VMs for the final measurement. Run workers from the control host, or from separate worker VMs if needed.

## Work Breakdown

### Phase 1: Normalize 18-VM Deployment Configuration

- [ ] Add an explicit progressive 18-VM deployment profile instead of mutating the data-trade/default deployment flow.
- [ ] Update host mapping for child1-child6 to the 3-VM-per-child layout above.
- [ ] Update IP handling to include `f13`-`f18` as `10.2.3.11`-`10.2.3.16`.
- [ ] Ensure `reset_child_chains.sh` uses the new progressive host mapping when running this experiment.
- [ ] Ensure `setup_selected_child_chains.js` still targets correct child RPC endpoints after remapping.
- [ ] Keep child6 data-trade workflows intact by isolating this profile from ordinary `deploy/config.toml` usage where possible.

Expected commit: `feat: add 18-vm progressive deployment profile`.

### Phase 2: Regenerate Specs and Deploy Cleanly

- [ ] Regenerate raw specs for progressive child1-child6.
- [ ] Verify each chain's authority set matches its assigned 3 validators.
- [ ] Push binaries/specs/node keys to the assigned VMs.
- [ ] Install systemd services using `/home/debian/fishbone` and `User=debian`.
- [ ] Start all child services and verify:
  - RPC is reachable.
  - peers are connected.
  - blocks are produced at expected intervals.
  - no child chain is accidentally running on an old overlapping host.

Expected commit: `chore: deploy 18-vm progressive chain specs` if repository config/spec files change.

### Phase 3: N=1..3 Baseline Validation

Use the currently proven baseline pressure point as the starting point:

```bash
RESET_EACH_STAGE=1 \
SETUP_MAX_WORKERS=200 \
WORKERS=200 \
PARALLEL_PER_WORKER=4 \
DURATION=180 \
DATA_SIZE=8 \
N_START=1 \
N_END=3 \
bash scripts/run_exp_progressive_tps.sh
```

Validation gates:

- [ ] N=1 reaches about 150 TPS on child1.
- [ ] child2 single-chain spot check reaches about 150 TPS before trusting N=2.
- [ ] child3 single-chain spot check reaches about 150 TPS before trusting N=3.
- [ ] N=2 reaches about 300 aggregate TPS.
- [ ] N=3 reaches about 450 aggregate TPS, or the cause is clearly deployment-independent.
- [ ] Worker `pool` ok counts are not used as accepted TPS; `capacity_monitor.js` remains authoritative.

If N=3 still underperforms:

1. First check wrong binary/spec/profile mapping.
2. Then check block interval under load.
3. Then check VM CPU with `ps -C fishbone-* -o pid,pcpu,pmem,args`.
4. Then try RPC endpoint rotation from f7 to f8/f9.
5. If the chain still cannot reach target, ask the user for additional CPU/RAM or permission to move the chain to stronger VMs.

Expected commit: `test: record 18-vm n1-n3 baseline`.

### Phase 4: Implement/Finalize N=4 Runtime v1

N=4 should use the already-started partial optimization:

- Compact high-frequency submission event.
- Avoid unnecessary full-payload event emission.
- Preserve business acceptance semantics.

Run child4 on f10-f12 only.

Validation gates:

- [ ] `cargo test -p pallet-crowdsource`.
- [ ] `cargo check` for runtime v1 feature set.
- [ ] child4 single-chain TPS exceeds baseline child TPS meaningfully.
- [ ] N=4 aggregate is clearly above N=3.

Expected commit if additional code is needed: `feat: refine crowdsource v1 throughput profile`.

### Phase 5: Implement N=5 Runtime v2

N=5 should remove the main known write-amplification bottleneck:

- Replace monolithic `EpochSubmissions: StorageValue<BoundedVec<...>>` hot-path append with indexed or aggregated storage.
- Store per-task/epoch counters separately.
- Preserve enough audit data to reconstruct accepted submissions or prove counts.
- Keep duplicate/error semantics explicit.

Preferred design:

- `SubmissionCountByTaskEpoch(TaskId, EpochId) -> u32`
- `SubmissionDigestByTaskEpochIndex(TaskId, EpochId, Index) -> Digest`
- Compact event with task, epoch, index, worker, digest.

Validation gates:

- [ ] Unit tests for count increments.
- [ ] Unit tests for duplicate/invalid/reward/budget behavior.
- [ ] Migration or clean-genesis behavior is explicit.
- [ ] child5 single-chain TPS is substantially above baseline.
- [ ] N=5 aggregate is monotonic and preferably over 1000 TPS.

Expected commit: `feat: add indexed crowdsource submission storage`.

### Phase 6: Implement N=6 Runtime v3 / Batch Business Submission

N=6 should add a full optimized path:

- Add `submitDataBatch` or equivalent runtime feature.
- One extrinsic may carry multiple business submissions.
- Count accepted business submissions inside the batch explicitly.
- Bound batch size by block weight and payload size.
- Summarizer must record both raw extrinsics/s and business submissions/s.
- Figure uses business submissions/s.

Validation gates:

- [ ] Unit tests prove accepted business submission count equals valid batch items.
- [ ] Invalid items are rejected or skipped according to a documented rule.
- [ ] Batch size defaults are conservative enough to avoid block production stalls.
- [ ] child6 single-chain business TPS creates a visible jump over N=5.
- [ ] N=6 reaches the preferred 1400-1800 business submissions/s band if hardware permits.

Expected commit: `feat: add batched crowdsource submission profile`.

### Phase 7: Mainchain Pressure Measurement

The final chart needs a defensible low-mainchain-pressure line:

- [ ] Run `metrics_main.js` during every stage.
- [ ] Ensure bridge-related records are counted from actual mainchain activity.
- [ ] If all bridge events are zero, configure a shorter bridge settlement/epoch interval or run a longer pressure window to capture nonzero bridge activity.
- [ ] Keep bridge pressure under 1% if possible.

If bridge pressure is exactly zero, do not overclaim. The report should say no bridge records occurred in the measured short pressure window unless a longer/shorter-epoch measurement captures them.

### Phase 8: Final Run, Figure, and Report

- [ ] Run clean N=1..6 with reset/setup per stage.
- [ ] Summarize into `docs/experiments/progressive_tps/progressive_tps_summary.csv`.
- [ ] Generate the single Origin-style figure:
  - `docs/experiments/progressive_tps/figures/progressive_tps_mainchain_load.png`
  - `docs/experiments/progressive_tps/figures/progressive_tps_mainchain_load.pdf`
- [ ] Inspect Chinese axis titles and legend.
- [ ] Write final README with:
  - VM hardware table.
  - deployment mapping.
  - exact command.
  - commit hash.
  - N=1..6 summary table.
  - explanation of why six child chains are enough to demonstrate scalability under current hardware.

Expected commit: `test: record 18-vm progressive tps final run`.

## Risk Register

- **f13-f18 are Ubuntu but normalized via `debian` user:** resolved for SSH/deploy conventions. Recheck before deployment.
- **Child runtime binaries may not match profile names:** verify binary metadata after deployment before pressure runs.
- **N=5/N=6 runtime work may require nontrivial Rust changes:** implement with unit tests before deploying.
- **Worker host may become bottleneck:** if worker CPU saturates, split pressure clients across separate worker VMs or local processes with worker offsets.
- **Mainchain bridge events may be too sparse:** adjust bridge interval or measurement window; do not claim zero pressure as proof without context.
- **PVE resource allocation may still bottleneck if all VMs share cores aggressively:** if validator CPU stays high and block intervals stretch, ask user for more vCPU/RAM or CPU pinning.

## Immediate Next Action

Start with Phase 1 only: add the 18-VM progressive deployment profile and update reset/run scripts to use it explicitly. Then run smoke checks before any long TPS run.
