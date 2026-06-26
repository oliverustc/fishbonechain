# Mainchain Capacity Occupancy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the misleading time-window bridge-pressure curve with a measured mainchain-capacity occupancy metric and add a repeatable 18-validator mainchain transfer benchmark.

**Architecture:** The mainchain benchmark produces an empirical `mainchain_max_tps` baseline from normal balance-transfer transactions. The progressive TPS summary then computes `mainchain_capacity_occupancy_pct = theoretical_bridge_tps / mainchain_max_tps * 100`, where `theoretical_bridge_tps = active_child_chains * bridge_extrinsics_per_epoch / epoch_seconds`. The final single figure keeps child-chain aggregate TPS as bars and uses this occupancy percentage as the right-axis line.

**Tech Stack:** Node.js ES modules with `@polkadot/api` for live chain pressure, Python CSV summarization and Matplotlib plotting, `unittest` for regression coverage, Git commits after each coherent checkpoint.

---

### Task 1: Repository Hygiene

**Files:**
- Modify: `.gitignore`

- [ ] **Step 1: Ignore generated progressive experiment runs and Python caches**

Add these entries to `.gitignore`:

```gitignore
# Generated experiment runs and local Python caches
/docs/experiments/progressive_tps/progressive_tps_runs/
__pycache__/
**/__pycache__/
*.py[cod]
```

- [ ] **Step 2: Verify noisy files disappear from `git status --short`**

Run:

```bash
git status --short
```

Expected: historical `progressive_tps_runs/*` and `__pycache__` files no longer appear as untracked entries.

- [ ] **Step 3: Commit hygiene change**

Run:

```bash
git add .gitignore docs/experiments/progressive_tps/mainchain_capacity_occupancy_plan.md
git commit -m "chore: ignore generated progressive experiment runs"
```

### Task 2: Mainchain Transfer Benchmark Script

**Files:**
- Create: `scripts/mainchain_transfer_burst.js`

- [ ] **Step 1: Implement a two-phase benchmark client**

Create `scripts/mainchain_transfer_burst.js` with:

```javascript
#!/usr/bin/env node
/**
 * FishBoneChain mainchain transfer capacity benchmark.
 *
 * Funding phase:
 *   Alice funds deterministic benchmark senders.
 *
 * Measurement phase:
 *   Benchmark senders submit balance transfers concurrently and report accepted
 *   in-block TPS. The result is intended as mainchain capacity, not bridge load.
 */
```

Required behavior:

- Parse `--ws`, `--senders`, `--parallel-per-sender`, `--duration`, `--amount`, `--fund-amount`, `--sender-offset`, `--receiver-mode`, `--submit-mode`, `--report-interval`, `--tx-timeout`, and `--out`.
- Build senders from `//MainBenchSender${offset + i}`. Use Alice as the receiver when `--receiver-mode alice`; otherwise build paired receivers from `//MainBenchReceiver${offset + i}`.
- If sender free balance is below `fundAmount / 2`, fund it from `//Alice` before measurement.
- Submit `balances.transferAllowDeath(receiver, amount)` if available, otherwise fall back to `balances.transfer(receiver, amount)`.
- In `watch` mode count only `status.isInBlock` with no dispatch error as `ok`.
- In `pool` mode count successful RPC submission as `ok` and retry nonce for transaction-pool-limit drops.
- Print periodic lines matching `mainTPS=...`.
- Write a JSON summary to `--out` with `ok`, `fail`, `reject`, `elapsedSeconds`, and `mainchainMaxTps`.

- [ ] **Step 2: Syntax-check the script**

Run:

```bash
node --check scripts/mainchain_transfer_burst.js
```

Expected: no syntax errors.

- [ ] **Step 3: Commit benchmark script**

Run:

```bash
git add scripts/mainchain_transfer_burst.js
git commit -m "feat: add mainchain transfer capacity benchmark"
```

### Task 3: Capacity Occupancy Summary

**Files:**
- Modify: `scripts/summarize_progressive_tps.py`
- Modify: `tests/test_progressive_tps_tools.py`

- [ ] **Step 1: Add regression coverage**

Add a test that feeds a row with:

```python
n = 6
aggregate_child_tps = 1250
mainchain_max_tps = 300
epoch_seconds = 120
bridge_extrinsics_per_epoch = 2
```

Expected:

```python
theoretical_bridge_tps = 0.1
mainchain_capacity_occupancy_pct = 0.0333
```

- [ ] **Step 2: Implement summary fields**

Add optional CLI arguments:

```bash
--mainchain-max-tps
--bridge-epoch-seconds
--bridge-extrinsics-per-epoch
```

Add CSV columns:

```csv
mainchain_max_tps,bridge_epoch_seconds,bridge_extrinsics_per_epoch,theoretical_bridge_tps,mainchain_capacity_occupancy_pct
```

Use:

```python
theoretical_bridge_tps = n * bridge_extrinsics_per_epoch / bridge_epoch_seconds
mainchain_capacity_occupancy_pct = theoretical_bridge_tps / mainchain_max_tps * 100
```

If `mainchain_max_tps <= 0`, write zero occupancy.

- [ ] **Step 3: Run focused tests**

Run:

```bash
python3 -m unittest tests.test_progressive_tps_tools
```

Expected: OK.

- [ ] **Step 4: Commit summary change**

Run:

```bash
git add scripts/summarize_progressive_tps.py tests/test_progressive_tps_tools.py
git commit -m "feat: summarize mainchain capacity occupancy"
```

### Task 4: Final Figure Uses Occupancy

**Files:**
- Modify: `scripts/plot_progressive_tps.py`
- Modify: `tests/test_progressive_tps_tools.py`

- [ ] **Step 1: Update plot data source**

Read `mainchain_capacity_occupancy_pct` for the right axis. Keep a fallback to `main_bridge_pressure_pct` only for legacy summary files.

- [ ] **Step 2: Update labels**

Use:

```text
主链容量占用率
主链容量占用率（%）
```

The figure should still show blue child TPS bars and a red line, but the red line now communicates how much measured mainchain capacity would be occupied by full-load child-chain bridge settlement.

- [ ] **Step 3: Run focused tests**

Run:

```bash
python3 -m unittest tests.test_progressive_tps_tools
```

Expected: OK.

- [ ] **Step 4: Commit plot change**

Run:

```bash
git add scripts/plot_progressive_tps.py tests/test_progressive_tps_tools.py
git commit -m "feat: plot mainchain capacity occupancy"
```

### Task 5: Measure, Regenerate, Verify

**Files:**
- Modify: `docs/experiments/progressive_tps/progressive_tps_summary.csv`
- Modify: `docs/experiments/progressive_tps/figures/progressive_tps_mainchain_load.png`
- Modify: `docs/experiments/progressive_tps/figures/progressive_tps_mainchain_load.pdf`
- Create as needed: `docs/experiments/progressive_tps/mainchain_capacity/`

- [ ] **Step 1: Run mainchain capacity benchmark**

Use the 18-validator mainchain endpoint:

```bash
MAIN_WS=ws://10.2.2.11:9944 node scripts/mainchain_transfer_burst.js \
  --ws ws://10.2.2.11:9944 \
  --senders 113 \
  --sender-offset 0 \
  --parallel-per-sender 4 \
  --duration 120 \
  --amount 1 \
  --fund-amount 1000000000000 \
  --receiver-mode alice \
  --submit-mode watch \
  --out docs/experiments/progressive_tps/mainchain_capacity/mainchain_transfer_burst_summary.json
```

This command uses the first 113 already-funded deterministic benchmark senders and transfers to the existing Alice account to avoid receiver existential-deposit effects. Record the measured `mainchainMaxTps`; the 2026-06-20 run measured `75.6638 TPS` with `9341/9341` successful in-block transfers.

- [ ] **Step 2: Regenerate progressive summary with measured capacity**

Run:

```bash
python3 scripts/summarize_progressive_tps.py \
  --runs docs/experiments/progressive_tps/progressive_tps_runs/progressive_18vm_n1_6_bridge_cleanmain_cap10000_20260620_190537 \
  --log-dir docs/experiments/progressive_tps/progressive_tps_runs/progressive_18vm_n1_6_bridge_cleanmain_cap10000_20260620_190537/logs \
  --mainchain-max-tps "$MAINCHAIN_MAX_TPS" \
  --bridge-epoch-seconds 120 \
  --bridge-extrinsics-per-epoch 2 \
  --out docs/experiments/progressive_tps/progressive_tps_summary.csv
```

- [ ] **Step 3: Regenerate final figure**

Run:

```bash
python3 scripts/plot_progressive_tps.py
```

- [ ] **Step 4: Run full progressive tooling tests**

Run:

```bash
python3 -m unittest tests.test_progressive_tps_tools tests.test_progressive_mainchain_setup tests.test_progressive_tps_profile tests.test_progressive_deploy_profile
```

Expected: OK.

- [ ] **Step 5: Commit measured result**

Run:

```bash
git add docs/experiments/progressive_tps/mainchain_capacity/mainchain_transfer_burst_summary.json \
  docs/experiments/progressive_tps/progressive_tps_summary.csv \
  docs/experiments/progressive_tps/figures/progressive_tps_mainchain_load.png \
  docs/experiments/progressive_tps/figures/progressive_tps_mainchain_load.pdf
git commit -m "test: record mainchain capacity occupancy result"
```

## Self-Review

- The plan covers the requested code management cleanup by ignoring generated run directories and caches.
- The plan adds a real mainchain throughput benchmark instead of merely rescaling bridge event counts.
- The plan keeps the final single-figure constraint by replacing the right-axis metric, not adding a second figure.
- The plan explicitly preserves the scientific meaning: child-chain TPS grows quickly, while required mainchain capacity occupancy grows slowly and remains small.
