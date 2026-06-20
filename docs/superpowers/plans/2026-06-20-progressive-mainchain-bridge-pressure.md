# Progressive Mainchain Bridge Pressure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the N=1..6 progressive TPS experiment produce a defensible nonzero, low mainchain bridge-pressure measurement by actually generating `EpochFinalized -> bridge -> mainchain ccmc/fmc` traffic during every stage.

**Architecture:** The current TPS path measures child-chain throughput correctly, but it does not initialize mainchain CCMC/FMC state, does not start bridge processes, and intentionally stops each stage before the crowdsource epoch can finalize. The fix is to add a progressive-experiment mainchain setup step, launch one crowdsource bridge per active child chain, and force or wait for an epoch finalization inside the measurement window without contaminating the child TPS measurement. Mainchain pressure remains summarized by `metrics_main.js` from accepted `ccmc.submitEpochDigest` and `fmc.submitBill` extrinsics.

**Tech Stack:** Bash orchestration, Node.js Polkadot API scripts, Substrate runtime metadata, Python unittest, Python CSV summarization, Matplotlib plotting.

---

## Why This Is Needed

The previous figure showed `main_bridge_events=0` for N=1..6. That value came from the experimental data, but the data did not prove that the mainchain had no pressure. It proved that the progressive TPS run did not produce bridge traffic.

There are three separate blockers:

1. `scripts/run_exp_progressive_tps.sh` starts `metrics_main.js`, `capacity_monitor.js`, and `worker_burst.js`, but it never starts `scripts/bridges/crowdsource.js`. Without a bridge process, child-chain `EpochFinalized` events are never submitted to the mainchain.
2. The crowdsource runtime uses `CollectingSlotBlocks=600` and `SyncingSlotBlocks=20`. The final run waited for early `Collecting` state, then measured only about 65-90 seconds. The stage ended hundreds of blocks before `EpochFinalized`, so even a running bridge would have had no event to forward.
3. The progressive child setup script only touches child chains. It calls `crowdsource.syncTask` and funds workers, but it does not register child chains, miners, or FMC tasks on the mainchain. A read-only mainchain query showed `ccmc.childChains.entries=0`, `ccmc.miners.entries=0`, `fmc.tasks.entries=0`, and `ccmc.epochDigests.entries=0`.

Therefore the correct next implementation must create bridge traffic deliberately, then measure it. The intended result is not large mainchain load. The intended result is a small, nonzero number such as a few `ccmc.submitEpochDigest` and `fmc.submitBill` extrinsics per stage, while child-chain accepted submissions remain hundreds or thousands per second.

## Measurement Semantics

Keep two metrics separate:

- Child-chain TPS window: from `capacity_monitor.js`, measuring high-pressure accepted crowdsource submissions until each active chain reaches `CAPACITY_CAP`.
- Mainchain bridge window: from `metrics_main.js`, measuring mainchain accepted bridge extrinsics during a window that includes epoch finalization and bridge submission.

Do not claim that `main_bridge_pressure_pct=0` proves low load. The defensible claim is:

> During each N-stage, high-frequency submissions remained on child chains. Mainchain traffic consisted only of low-frequency epoch digest/bill submissions, producing a bridge-to-child pressure percentage far below 1%.

## File Structure

- Modify: `scripts/run_exp_progressive_tps.sh`
  - Add optional mainchain setup before stages.
  - Add bridge startup/shutdown around each stage.
  - Add a finalization phase after the pressure monitor reaches cap.
  - Keep worker stop timing and child TPS calculation unchanged.
- Create: `scripts/setup_progressive_mainchain.js`
  - Idempotently initialize mainchain CCMC/FMC state for the profile file.
  - Register child chains if missing.
  - Join configured miner accounts to each child chain if missing.
  - Create and activate FMC tasks if missing.
- Create: `scripts/finalize_progressive_epochs.js`
  - For each active child chain, wait until `Syncing` or optionally submit empty filler blocks until `Syncing`, then call `crowdsource.finalizeEpoch`.
  - Output a JSON summary recording whether `EpochFinalized` was observed.
- Modify: `scripts/bridges/crowdsource.js`
  - Add a bounded `--exit-after-events N` mode so orchestration can stop bridge processes deterministically after each stage.
- Modify: `scripts/summarize_progressive_tps.py`
  - Preserve existing child TPS calculation.
  - Add columns showing `bridge_finalized_epochs` and `bridge_process_ok`.
  - Fail or warn when `main_bridge_events=0` but bridge measurement is required.
- Modify: `tests/test_progressive_tps_tools.py`
  - Add tests proving the runner starts bridges, runs mainchain setup, finalizes epochs, and refuses to silently accept zero bridge events when bridge measurement is required.
- Create: `tests/test_progressive_mainchain_setup.py`
  - Static/unit tests for profile parsing and idempotent action planning in `setup_progressive_mainchain.js`.

## Main Design Choice: How To Get EpochFinalized In A Short Experiment

Do not reduce `CollectingSlotBlocks` globally in the runtime. That would require rebuilding all child binaries and would change the business epoch semantics for the throughput experiment.

Use manual finalization after the child TPS cap is reached:

1. Start `metrics_main.js` before pressure begins.
2. Start bridge processes before or during pressure.
3. Run child pressure until `capacity_monitor.js` reaches cap.
4. Stop pressure workers so child TPS measurement remains clean.
5. Wait until each child reaches `Syncing`, or actively advance enough blocks by waiting. If this timeout is hit in Task 7, stop the run and implement the short-epoch fallback described in Task 7 Step 3 before attempting the full experiment.
6. Call `crowdsource.finalizeEpoch` on each active child during `Syncing`.
7. Bridge processes observe `EpochFinalized` and submit mainchain `ccmc.submitEpochDigest` and `fmc.submitBill`.
8. Keep `metrics_main.js` running until those mainchain extrinsics are observed, then stop it.

If step 5 takes too long because `CollectingSlotBlocks=600`, add a second implementation phase with an experiment-only short-epoch crowdsource binary. That phase must be explicit and reported as a bridge-pressure measurement profile, not as the raw TPS profile.

## Task 1: Add Runner Tests For Mainchain Setup And Bridge Lifecycle

**Files:**
- Modify: `tests/test_progressive_tps_tools.py`
- Modify in Task 5: `scripts/run_exp_progressive_tps.sh`

- [ ] **Step 1: Write the failing test**

Add this test to `ProgressiveTpsToolsTest` in `tests/test_progressive_tps_tools.py`:

```python
    def test_runner_initializes_mainchain_and_runs_bridges_for_pressure_measurement(self):
        script = RUN_SCRIPT.read_text(encoding="utf-8")

        self.assertIn("SETUP_MAINCHAIN_FOR_BRIDGE", script)
        self.assertIn("setup_progressive_mainchain.js", script)
        self.assertIn("start_bridges_for_stage", script)
        self.assertIn("scripts/bridges/crowdsource.js", script)
        self.assertIn("finalize_progressive_epochs.js", script)
        self.assertIn("n${n}_bridge_${child}.log", script)
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
python3 -m unittest tests.test_progressive_tps_tools.ProgressiveTpsToolsTest.test_runner_initializes_mainchain_and_runs_bridges_for_pressure_measurement
```

Expected: `FAIL` because the runner does not yet contain these bridge setup strings.

- [ ] **Step 3: Do not implement yet**

Commit only after Task 2 and Task 3 provide the scripts the runner will call. This test defines the contract first.

## Task 2: Create An Idempotent Mainchain Setup Script

**Files:**
- Create: `scripts/setup_progressive_mainchain.js`
- Create: `tests/test_progressive_mainchain_setup.py`

- [ ] **Step 1: Write the failing profile parsing test**

Create `tests/test_progressive_mainchain_setup.py`:

```python
import importlib.util
import json
import subprocess
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "setup_progressive_mainchain.js"


class ProgressiveMainchainSetupTest(unittest.TestCase):
    def test_dry_run_lists_child_registration_miner_join_and_fmc_tasks(self):
        with tempfile.TemporaryDirectory() as tmp:
            profile = Path(tmp) / "profile.json"
            profile.write_text(
                json.dumps(
                    {
                        "main": {"scene": "PlatformOnly"},
                        "child1": {
                            "chainId": 0,
                            "scene": "Crowdsource",
                            "settlement": "FmcTaskBill",
                            "validators": ["f1", "f2", "f3"],
                            "taskId": 0,
                            "budgetPerEpochUnit": "1500",
                            "description": "Progressive TPS baseline child 1",
                        },
                        "child2": {
                            "chainId": 1,
                            "scene": "Crowdsource",
                            "settlement": "FmcTaskBill",
                            "validators": ["f4", "f5", "f6"],
                            "taskId": 1,
                            "budgetPerEpochUnit": "1500",
                            "description": "Progressive TPS baseline child 2",
                        },
                    }
                ),
                encoding="utf-8",
            )

            result = subprocess.run(
                [
                    "node",
                    str(SCRIPT),
                    "--profile-file",
                    str(profile),
                    "--chains",
                    "child1,child2",
                    "--dry-run",
                ],
                cwd=ROOT,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )

        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("registerChildChain chain_id=0", result.stdout)
        self.assertIn("joinChildChain chain_id=0 validator=f1", result.stdout)
        self.assertIn("createTask task_id=0 chain_id=0 budget_unit=1500", result.stdout)
        self.assertIn("activateTask task_id=0", result.stdout)
        self.assertIn("registerChildChain chain_id=1", result.stdout)


if __name__ == "__main__":
    unittest.main()
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
python3 -m unittest tests.test_progressive_mainchain_setup
```

Expected: `FAIL` because `scripts/setup_progressive_mainchain.js` does not exist.

- [ ] **Step 3: Implement minimal dry-run script**

Create `scripts/setup_progressive_mainchain.js`:

```javascript
#!/usr/bin/env node

import { readFileSync } from "fs";

function parseArgs() {
  const args = process.argv.slice(2);
  const get = (flag, def = "") => {
    const i = args.indexOf(flag);
    return i === -1 ? def : args[i + 1];
  };
  return {
    profileFile: get("--profile-file"),
    chains: get("--chains").split(",").map((s) => s.trim()).filter(Boolean),
    dryRun: args.includes("--dry-run"),
  };
}

function loadProfiles(path) {
  const raw = JSON.parse(readFileSync(path, "utf8"));
  return raw.chains || raw;
}

function selectedCrowdsourceProfiles(profiles, chains) {
  return chains
    .map((name) => [name, profiles[name]])
    .filter(([, profile]) => profile && profile.scene === "Crowdsource" && profile.settlement === "FmcTaskBill");
}

async function main() {
  const cfg = parseArgs();
  if (!cfg.profileFile) throw new Error("--profile-file is required");
  if (cfg.chains.length === 0) throw new Error("--chains is required");
  const profiles = loadProfiles(cfg.profileFile);

  for (const [name, profile] of selectedCrowdsourceProfiles(profiles, cfg.chains)) {
    const chainId = profile.chainId;
    const taskId = profile.taskId;
    const budget = profile.budgetPerEpochUnit;
    console.log(`registerChildChain chain_id=${chainId} name=${name}`);
    for (const validator of profile.validators || []) {
      console.log(`joinChildChain chain_id=${chainId} validator=${validator}`);
    }
    console.log(`createTask task_id=${taskId} chain_id=${chainId} budget_unit=${budget}`);
    console.log(`activateTask task_id=${taskId}`);
  }
}

main().catch((e) => {
  console.error(`[setup_progressive_mainchain fatal] ${e.message}`);
  process.exit(1);
});
```

- [ ] **Step 4: Run the test to verify it passes**

Run:

```bash
python3 -m unittest tests.test_progressive_mainchain_setup
```

Expected: `OK`.

- [ ] **Step 5: Extend the script from dry-run to real chain writes**

Modify `scripts/setup_progressive_mainchain.js` to import Polkadot API and add real execution when `--dry-run` is not present:

```javascript
import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";
```

Add constants and helpers:

```javascript
const UNIT = 1_000_000_000_000n;
const MAIN_WS = process.env.MAIN_WS || "ws://10.2.2.11:9944";

const VALIDATOR_SURIS = {
  f1: "//Alice",
  f2: "//Bob",
  f3: "//Charlie",
  f4: "//Dave",
  f5: "//Eve",
  f6: "//Ferdie",
  f7: "//One",
  f8: "//Two",
  f9: "//Alice//f9",
  f10: "//Alice//f10",
  f11: "//Alice//f11",
  f12: "//Alice//f12",
  f13: "//Alice//f13",
  f14: "//Alice//f14",
  f15: "//Alice//f15",
  f16: "//Alice//f16",
  f17: "//Alice//f17",
  f18: "//Alice//f18",
};

async function sendTx(api, tx, signer, label) {
  return new Promise((resolve, reject) => {
    tx.signAndSend(signer, ({ status, dispatchError }) => {
      if (dispatchError) {
        reject(new Error(`${label} failed: ${dispatchError.toString()}`));
      } else if (status.isInBlock) {
        console.log(`ok ${label}`);
        resolve();
      }
    }).catch(reject);
  });
}
```

Use storage checks before writes:

```javascript
async function ensureMainchainState(api, keyring, name, profile) {
  const alice = keyring.addFromUri("//Alice");
  const chainId = profile.chainId;
  const child = await api.query.ccmc.childChains(chainId);
  if (child.isNone) {
    const nameBytes = Array.from(new TextEncoder().encode(name));
    await sendTx(api, api.tx.ccmc.registerChildChain(nameBytes, 1, 0), alice, `registerChildChain(${chainId})`);
  } else {
    console.log(`skip registerChildChain chain_id=${chainId}`);
  }

  for (const validator of profile.validators || []) {
    const suri = VALIDATOR_SURIS[validator];
    if (!suri) throw new Error(`missing validator SURI for ${validator}`);
    const signer = keyring.addFromUri(suri);
    const joined = await api.query.ccmc.miners(chainId, signer.address);
    if (joined.isNone || joined.isFalse) {
      await sendTx(api, api.tx.ccmc.joinChildChain(chainId), signer, `joinChildChain(${chainId}) by ${validator}`);
    } else {
      console.log(`skip joinChildChain chain_id=${chainId} validator=${validator}`);
    }
  }

  const task = await api.query.fmc.tasks(alice.address, profile.taskId);
  if (task.isNone) {
    const descBytes = Array.from(new TextEncoder().encode(profile.description || name));
    const budget = BigInt(profile.budgetPerEpochUnit) * UNIT;
    await sendTx(api, api.tx.fmc.createTask(chainId, budget, descBytes), alice, `createTask(${profile.taskId})`);
    await sendTx(api, api.tx.fmc.activateTask(profile.taskId), alice, `activateTask(${profile.taskId})`);
  } else {
    console.log(`skip fmc task task_id=${profile.taskId}`);
  }
}
```

In `main()`, when not `dryRun`, connect and call `ensureMainchainState`:

```javascript
  if (!cfg.dryRun) {
    const api = await ApiPromise.create({ provider: new WsProvider(MAIN_WS) });
    const keyring = new Keyring({ type: "sr25519" });
    for (const [name, profile] of selectedCrowdsourceProfiles(profiles, cfg.chains)) {
      await ensureMainchainState(api, keyring, name, profile);
    }
    await api.disconnect();
    return;
  }
```

- [ ] **Step 6: Run tests**

Run:

```bash
python3 -m unittest tests.test_progressive_mainchain_setup
```

Expected: `OK`.

- [ ] **Step 7: Commit**

Run:

```bash
git add scripts/setup_progressive_mainchain.js tests/test_progressive_mainchain_setup.py
git commit -m "feat: add progressive mainchain bridge setup"
```

## Task 3: Add Deterministic Bridge Exit Mode

**Files:**
- Modify: `scripts/bridges/crowdsource.js`
- Modify: `tests/test_progressive_tps_tools.py`

- [ ] **Step 1: Write the failing static test**

Add this test:

```python
    def test_crowdsource_bridge_supports_bounded_exit_after_events(self):
        bridge = (ROOT / "scripts" / "bridges" / "crowdsource.js").read_text(encoding="utf-8")

        self.assertIn("EXIT_AFTER_EVENTS", bridge)
        self.assertIn("--exit-after-events", bridge)
        self.assertIn("processedCount >= EXIT_AFTER_EVENTS", bridge)
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
python3 -m unittest tests.test_progressive_tps_tools.ProgressiveTpsToolsTest.test_crowdsource_bridge_supports_bounded_exit_after_events
```

Expected: `FAIL`.

- [ ] **Step 3: Implement bounded exit**

Modify the top of `scripts/bridges/crowdsource.js`:

```javascript
const EXIT_AFTER_EVENTS = (() => {
  const i = process.argv.indexOf("--exit-after-events");
  return i === -1 ? 0 : Number(process.argv[i + 1] || "0");
})();
const ONCE = process.argv.includes("--once") || EXIT_AFTER_EVENTS === 1;
```

Replace the existing `if (ONCE)` exit block after `processedCount++` with:

```javascript
      if (EXIT_AFTER_EVENTS > 0 && processedCount >= EXIT_AFTER_EVENTS) {
        log(`\n--exit-after-events=${EXIT_AFTER_EVENTS}: 已处理 ${processedCount} 个 Epoch，退出`);
        unsub();
        await Promise.all([childApi.disconnect(), mainApi.disconnect()]);
        process.exit(0);
      }

      if (ONCE) {
        log(`\n--once 模式：处理 ${processedCount} 个 Epoch，退出`);
        unsub();
        await Promise.all([childApi.disconnect(), mainApi.disconnect()]);
        process.exit(0);
      }
```

- [ ] **Step 4: Run the test**

Run:

```bash
python3 -m unittest tests.test_progressive_tps_tools.ProgressiveTpsToolsTest.test_crowdsource_bridge_supports_bounded_exit_after_events
```

Expected: `OK`.

- [ ] **Step 5: Commit**

Run:

```bash
git add scripts/bridges/crowdsource.js tests/test_progressive_tps_tools.py
git commit -m "feat: bound crowdsource bridge pressure runs"
```

## Task 4: Add Epoch Finalization Helper

**Files:**
- Create: `scripts/finalize_progressive_epochs.js`
- Modify: `tests/test_progressive_tps_tools.py`

- [ ] **Step 1: Write the failing static test**

Add this test:

```python
    def test_epoch_finalizer_waits_for_syncing_and_calls_finalize_epoch(self):
        finalizer = ROOT / "scripts" / "finalize_progressive_epochs.js"
        self.assertTrue(finalizer.exists())
        content = finalizer.read_text(encoding="utf-8")

        self.assertIn("currentEpoch", content)
        self.assertIn("Syncing", content)
        self.assertIn("finalizeEpoch", content)
        self.assertIn("EpochFinalized", content)
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
python3 -m unittest tests.test_progressive_tps_tools.ProgressiveTpsToolsTest.test_epoch_finalizer_waits_for_syncing_and_calls_finalize_epoch
```

Expected: `FAIL`.

- [ ] **Step 3: Implement the helper**

Create `scripts/finalize_progressive_epochs.js`:

```javascript
#!/usr/bin/env node

import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";
import { writeFileSync } from "fs";

function parseArgs() {
  const args = process.argv.slice(2);
  const get = (flag, def = "") => {
    const i = args.indexOf(flag);
    return i === -1 ? def : args[i + 1];
  };
  return {
    chains: get("--chains").split(",").map((s) => s.trim()).filter(Boolean),
    out: get("--out", "/tmp/progressive_epoch_finalize.json"),
    waitSyncing: Number(get("--wait-syncing-seconds", "900")),
    pollMs: Number(get("--poll-ms", "3000")),
  };
}

async function sendTx(api, tx, signer, label) {
  return new Promise((resolve, reject) => {
    tx.signAndSend(signer, ({ status, dispatchError }) => {
      if (dispatchError) reject(new Error(`${label} failed: ${dispatchError.toString()}`));
      else if (status.isInBlock) resolve(status.asInBlock.toString());
    }).catch(reject);
  });
}

function phaseName(epoch) {
  const human = epoch.toHuman();
  return human.phase || human.Phase || JSON.stringify(human);
}

async function waitForSyncing(api, timeoutSeconds, pollMs) {
  const deadline = Date.now() + timeoutSeconds * 1000;
  while (Date.now() < deadline) {
    const epoch = await api.query.crowdsource.currentEpoch();
    const phase = phaseName(epoch);
    if (phase === "Syncing") return epoch;
    await new Promise((resolve) => setTimeout(resolve, pollMs));
  }
  throw new Error(`timeout waiting for Syncing after ${timeoutSeconds}s`);
}

async function finalizeOne(ws, keyring, cfg) {
  const api = await ApiPromise.create({ provider: new WsProvider(ws) });
  const alice = keyring.addFromUri("//Alice");
  let sawFinalized = false;
  const unsub = await api.query.system.events((events) => {
    for (const { event } of events) {
      if (event.section === "crowdsource" && event.method === "EpochFinalized") {
        sawFinalized = true;
      }
    }
  });
  const before = await waitForSyncing(api, cfg.waitSyncing, cfg.pollMs);
  const blockHash = await sendTx(api, api.tx.crowdsource.finalizeEpoch(), alice, "crowdsource.finalizeEpoch");
  await new Promise((resolve) => setTimeout(resolve, cfg.pollMs));
  const after = await api.query.crowdsource.currentEpoch();
  unsub();
  await api.disconnect();
  return { ws, before: before.toHuman(), after: after.toHuman(), blockHash, sawFinalized };
}

async function main() {
  const cfg = parseArgs();
  if (cfg.chains.length === 0) throw new Error("--chains is required");
  const keyring = new Keyring({ type: "sr25519" });
  const results = [];
  for (const ws of cfg.chains) {
    results.push(await finalizeOne(ws, keyring, cfg));
  }
  writeFileSync(cfg.out, `${JSON.stringify({ results }, null, 2)}\n`);
}

main().catch((e) => {
  console.error(`[finalize_progressive_epochs fatal] ${e.message}`);
  process.exit(1);
});
```

- [ ] **Step 4: Run the test**

Run:

```bash
python3 -m unittest tests.test_progressive_tps_tools.ProgressiveTpsToolsTest.test_epoch_finalizer_waits_for_syncing_and_calls_finalize_epoch
```

Expected: `OK`.

- [ ] **Step 5: Commit**

Run:

```bash
git add scripts/finalize_progressive_epochs.js tests/test_progressive_tps_tools.py
git commit -m "feat: add progressive epoch finalization helper"
```

## Task 5: Wire Mainchain Setup, Bridges, And Finalization Into The Runner

**Files:**
- Modify: `scripts/run_exp_progressive_tps.sh`
- Modify: `tests/test_progressive_tps_tools.py`

- [ ] **Step 1: Re-run the failing runner lifecycle test**

Run:

```bash
python3 -m unittest tests.test_progressive_tps_tools.ProgressiveTpsToolsTest.test_runner_initializes_mainchain_and_runs_bridges_for_pressure_measurement
```

Expected: still `FAIL`.

- [ ] **Step 2: Add runner options and helper functions**

In `scripts/run_exp_progressive_tps.sh`, add near the other environment defaults:

```bash
SETUP_MAINCHAIN_FOR_BRIDGE="${SETUP_MAINCHAIN_FOR_BRIDGE:-1}"
RUN_BRIDGES_FOR_STAGE="${RUN_BRIDGES_FOR_STAGE:-1}"
BRIDGE_EXIT_AFTER_EVENTS="${BRIDGE_EXIT_AFTER_EVENTS:-1}"
FINALIZE_EPOCHS_FOR_BRIDGE="${FINALIZE_EPOCHS_FOR_BRIDGE:-1}"
FINALIZE_WAIT_SYNCING_SECONDS="${FINALIZE_WAIT_SYNCING_SECONDS:-900}"
```

Add to `write_meta()`:

```bash
    echo "setup_mainchain_for_bridge=${SETUP_MAINCHAIN_FOR_BRIDGE}"
    echo "run_bridges_for_stage=${RUN_BRIDGES_FOR_STAGE}"
    echo "bridge_exit_after_events=${BRIDGE_EXIT_AFTER_EVENTS}"
    echo "finalize_epochs_for_bridge=${FINALIZE_EPOCHS_FOR_BRIDGE}"
    echo "finalize_wait_syncing_seconds=${FINALIZE_WAIT_SYNCING_SECONDS}"
```

Add helper functions before `run_one_n()`:

```bash
setup_mainchain_for_bridge() {
  local chains_csv="$1"
  local log_path="$2"
  MAIN_WS="$MAIN_WS" node "${SCRIPT_DIR}/setup_progressive_mainchain.js" \
    --profile-file "$PROFILE_FILE" \
    --chains "$chains_csv" \
    > "$log_path" 2>&1
}

start_bridges_for_stage() {
  local n="$1"
  shift
  local active=("$@")
  BRIDGE_PIDS=()
  for child in "${active[@]}"; do
    local idx="${child#child}"
    local chain_id=$((idx - 1))
    nohup env CHILD_WS="${WS[$child]}" MAIN_WS="$MAIN_WS" TASK_ID="${TASK_ID[$child]}" CHAIN_ID="$chain_id" \
      node "${SCRIPT_DIR}/bridges/crowdsource.js" \
        --exit-after-events "$BRIDGE_EXIT_AFTER_EVENTS" \
      > "${LOG_DIR}/n${n}_bridge_${child}.log" 2>&1 &
    BRIDGE_PIDS+=("$!")
  done
}

stop_bridges_for_stage() {
  for pid in "${BRIDGE_PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  for pid in "${BRIDGE_PIDS[@]:-}"; do
    wait "$pid" 2>/dev/null || true
  done
  BRIDGE_PIDS=()
}
```

- [ ] **Step 3: Call setup before each stage**

Inside `run_one_n()`, after `active_csv` is computed and before child setup:

```bash
  if [[ "$SETUP_MAINCHAIN_FOR_BRIDGE" == "1" ]]; then
    log "N=${n} setup mainchain bridge state: ${active_csv}"
    setup_mainchain_for_bridge "$active_csv" "${LOG_DIR}/n${n}_setup_mainchain_bridge.log"
  fi
```

- [ ] **Step 4: Start bridge processes before worker pressure**

After `metrics_main.js` starts and before `capacity_monitor.js` starts:

```bash
  if [[ "$RUN_BRIDGES_FOR_STAGE" == "1" ]]; then
    log "N=${n} start crowdsource bridges"
    start_bridges_for_stage "$n" "${active[@]}"
  fi
```

- [ ] **Step 5: Finalize epochs after pressure cap and before stopping main metrics**

After worker processes are stopped and before killing `main_metrics_pid`:

```bash
  if [[ "$FINALIZE_EPOCHS_FOR_BRIDGE" == "1" ]]; then
    log "N=${n} finalize epochs for bridge measurement"
    node "${SCRIPT_DIR}/finalize_progressive_epochs.js" \
      --chains "$urls" \
      --out "${prefix}_epoch_finalize.json" \
      --wait-syncing-seconds "$FINALIZE_WAIT_SYNCING_SECONDS" \
      > "${LOG_DIR}/n${n}_finalize_epochs.log" 2>&1
    sleep 20
  fi

  stop_bridges_for_stage
```

- [ ] **Step 6: Run the lifecycle test**

Run:

```bash
python3 -m unittest tests.test_progressive_tps_tools.ProgressiveTpsToolsTest.test_runner_initializes_mainchain_and_runs_bridges_for_pressure_measurement
```

Expected: `OK`.

- [ ] **Step 7: Run all progressive tool tests**

Run:

```bash
python3 -m unittest tests.test_progressive_tps_tools tests.test_progressive_mainchain_setup tests.test_progressive_tps_profile tests.test_progressive_deploy_profile
```

Expected: all tests pass.

- [ ] **Step 8: Commit**

Run:

```bash
git add scripts/run_exp_progressive_tps.sh tests/test_progressive_tps_tools.py
git commit -m "feat: measure progressive mainchain bridge pressure"
```

## Task 6: Add Summary Guardrails For Missing Bridge Traffic

**Files:**
- Modify: `scripts/summarize_progressive_tps.py`
- Modify: `tests/test_progressive_tps_tools.py`

- [ ] **Step 1: Write a failing test**

Add this test:

```python
    def test_summarizer_marks_missing_required_bridge_traffic(self):
        module = load_module(SUMMARY_SCRIPT, "summarize_progressive_tps_required_bridge")
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            raw_dir = root / "raw"
            log_dir = root / "logs"
            raw_dir.mkdir()
            log_dir.mkdir()
            (raw_dir / "progressive_tps_n1_child_precise_summary.json").write_text(
                json.dumps({"hit_summary": {"ws://child": {"accepted_delta": 1000, "elapsed_s": 10, "cap_subs": 1000}}}),
                encoding="utf-8",
            )
            (raw_dir / "progressive_tps_n1_main_blocks.csv").write_text(
                "timestamp,block_number,block_hash,extrinsics_total,bridge_extrinsics,ccmc_digest_calls,fmc_bill_calls,ccmc_events,fmc_events\n"
                "2026-06-20T00:00:00Z,1,0x1,1,0,0,0,0,0\n",
                encoding="utf-8",
            )
            (raw_dir / "progressive_tps_n1_stage.txt").write_text(
                "require_bridge_events=1\nfailed=0\n",
                encoding="utf-8",
            )

            row = module.summarize_stage(raw_dir, log_dir, 1, module.DEFAULT_ORDER)

        self.assertEqual(row["bridge_measurement_status"], "missing_required_bridge_events")
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
python3 -m unittest tests.test_progressive_tps_tools.ProgressiveTpsToolsTest.test_summarizer_marks_missing_required_bridge_traffic
```

Expected: `FAIL`.

- [ ] **Step 3: Implement status column**

In `scripts/summarize_progressive_tps.py`, load stage key/value data if not already loaded and compute:

```python
def bridge_measurement_status(stage_values: dict[str, str], main: dict[str, float]) -> str:
    required = stage_values.get("require_bridge_events") == "1"
    if required and main["main_bridge_events"] <= 0:
        return "missing_required_bridge_events"
    if main["main_bridge_events"] > 0:
        return "observed"
    return "not_required_or_not_observed"
```

Add `bridge_measurement_status` to each row and to the CSV field list.

- [ ] **Step 4: Update runner stage file**

In `scripts/run_exp_progressive_tps.sh`, add to the stage file block:

```bash
    echo "require_bridge_events=${RUN_BRIDGES_FOR_STAGE}"
```

- [ ] **Step 5: Run tests**

Run:

```bash
python3 -m unittest tests.test_progressive_tps_tools tests.test_progressive_mainchain_setup
```

Expected: `OK`.

- [ ] **Step 6: Commit**

Run:

```bash
git add scripts/summarize_progressive_tps.py scripts/run_exp_progressive_tps.sh tests/test_progressive_tps_tools.py
git commit -m "fix: flag missing progressive bridge measurements"
```

## Task 7: Run A Small Bridge Measurement Smoke Test

**Files:**
- No code changes expected.
- Outputs under a run directory named by `SMOKE_RUN_ID` in `docs/experiments/progressive_tps/progressive_tps_runs/`.

- [ ] **Step 1: Stop all child services**

Run:

```bash
bash scripts/stop_all_child_services.sh --config deploy/config.progressive-18vm.toml
```

Expected: all child services stop.

- [ ] **Step 2: Run N=1 smoke test with bridge measurement enabled**

Run:

```bash
SMOKE_RUN_ID="progressive_bridge_smoke_n1_$(date +%Y%m%d_%H%M%S)"
RUN_ID="$SMOKE_RUN_ID" \
PROFILE_FILE="scripts/profiles/progressive_tps_18vm.json" \
DEPLOY_CONFIG="deploy/config.progressive-18vm.toml" \
RESET_EACH_STAGE=1 \
SETUP_MAX_WORKERS=300 \
CAPACITY_CAP=1000 \
CAPACITY_MONITOR_TIMEOUT=180 \
WAIT_MIN_REMAINING_BLOCKS=250 \
SETUP_MAINCHAIN_FOR_BRIDGE=1 \
RUN_BRIDGES_FOR_STAGE=1 \
FINALIZE_EPOCHS_FOR_BRIDGE=1 \
FINALIZE_WAIT_SYNCING_SECONDS=900 \
N_START=1 \
N_END=1 \
bash scripts/run_exp_progressive_tps.sh
```

Expected:

- Child TPS summary has `accepted_submissions >= 1000`.
- `logs/n1_bridge_child1.log` contains `EpochFinalized` and `ccmc.submitEpochDigest`.
- `progressive_tps_n1_main_blocks.csv` contains at least one row with `bridge_extrinsics > 0`.
- Summary row has `bridge_measurement_status=observed`.

- [ ] **Step 3: If finalization times out, stop and choose short-epoch profile**

If `logs/n1_finalize_epochs.log` shows `timeout waiting for Syncing`, do not fake mainchain pressure. Implement a separate short-epoch profile in a new plan:

- Build child runtime variants with `CollectingSlotBlocks=60`, `SyncingSlotBlocks=5`.
- Use those variants only for bridge-pressure measurement.
- Keep the high-TPS chart annotation explicit: `主链桥接压力：短 epoch 桥接测量窗口`.

- [ ] **Step 4: Commit smoke-test artifact only if successful**

Run:

```bash
git add "docs/experiments/progressive_tps/progressive_tps_runs/${SMOKE_RUN_ID}"
git commit -m "test: record progressive bridge pressure smoke run"
```

## Task 8: Re-run Full N=1..6 Experiment

**Files:**
- Update: `docs/experiments/progressive_tps/progressive_tps_summary.csv`
- Update: `docs/experiments/progressive_tps/figures/progressive_tps_mainchain_load.png`
- Update: `docs/experiments/progressive_tps/figures/progressive_tps_mainchain_load.pdf`
- Add: full run directory named by `FULL_RUN_ID` under `docs/experiments/progressive_tps/progressive_tps_runs/`

- [ ] **Step 1: Run full experiment**

Run:

```bash
FULL_RUN_ID="progressive_18vm_n1_6_bridge_$(date +%Y%m%d_%H%M%S)"
RUN_ID="$FULL_RUN_ID" \
PROFILE_FILE="scripts/profiles/progressive_tps_18vm.json" \
DEPLOY_CONFIG="deploy/config.progressive-18vm.toml" \
RESET_EACH_STAGE=1 \
SETUP_MAX_WORKERS=1400 \
CAPACITY_CAP=10000 \
CAPACITY_MONITOR_TIMEOUT=420 \
WAIT_MIN_REMAINING_BLOCKS=250 \
SETUP_MAINCHAIN_FOR_BRIDGE=1 \
RUN_BRIDGES_FOR_STAGE=1 \
FINALIZE_EPOCHS_FOR_BRIDGE=1 \
FINALIZE_WAIT_SYNCING_SECONDS=900 \
N_START=1 \
N_END=6 \
bash scripts/run_exp_progressive_tps.sh
```

Expected:

- All stages have `failed=0`.
- `main_bridge_events > 0` for every N.
- `main_bridge_pressure_pct < 1.0` for every N.
- Child TPS remains comparable to the previous formal run.

- [ ] **Step 2: Verify summary**

Run:

```bash
python3 - <<'PY'
import csv
from pathlib import Path
rows = list(csv.DictReader(Path("docs/experiments/progressive_tps/progressive_tps_summary.csv").open()))
for row in rows:
    print(row["n"], row["aggregate_child_tps"], row["main_bridge_events"], row["main_bridge_pressure_pct"], row.get("bridge_measurement_status", ""))
    assert float(row["aggregate_child_tps"]) > 0
    assert float(row["main_bridge_events"]) > 0
    assert float(row["main_bridge_pressure_pct"]) < 1.0
PY
```

Expected: no assertion failure.

- [ ] **Step 3: Regenerate figure if needed**

Run:

```bash
python3 scripts/plot_progressive_tps.py \
  --summary docs/experiments/progressive_tps/progressive_tps_summary.csv \
  --out-dir docs/experiments/progressive_tps/figures \
  --formats pdf,png
```

Expected: PNG/PDF regenerated.

- [ ] **Step 4: Run test suite**

Run:

```bash
python3 -m unittest tests.test_progressive_tps_tools tests.test_progressive_mainchain_setup tests.test_progressive_tps_profile tests.test_progressive_deploy_profile
```

Expected: all tests pass.

- [ ] **Step 5: Commit full run**

Run:

```bash
git add docs/experiments/progressive_tps/progressive_tps_summary.csv \
  docs/experiments/progressive_tps/figures/progressive_tps_mainchain_load.png \
  docs/experiments/progressive_tps/figures/progressive_tps_mainchain_load.pdf \
  "docs/experiments/progressive_tps/progressive_tps_runs/${FULL_RUN_ID}"
git commit -m "test: record progressive bridge pressure full run"
```

## Reporting Guidance

Use wording like:

> 在 N=1..6 的高压提交实验中，子链侧聚合吞吐量从约 150 TPS 提升到约 1250 TPS。主链仅在 epoch 结算阶段接收摘要和账单类桥接交易，桥接 TPS 相对子链业务提交 TPS 保持在 1% 以下，说明高频提交被有效下沉到子链，主链没有随 worker 提交量线性承压。

Avoid wording like:

> 主链压力为 0。

or:

> 主链完全不受子链数量影响。

The correct claim is low-frequency bridge load, not zero load.

## Self-Review

- Spec coverage: The plan explains why mainchain initialization, bridge startup, and epoch finalization are all necessary. It includes implementation tasks, tests, smoke run, and full N=1..6 rerun.
- Placeholder scan: No task uses TBD-style placeholders. Run directory names are assigned to `SMOKE_RUN_ID` and `FULL_RUN_ID` before each experiment command and reused in the corresponding commit command.
- Type consistency: Script names and environment variables are consistent across tasks: `setup_progressive_mainchain.js`, `finalize_progressive_epochs.js`, `SETUP_MAINCHAIN_FOR_BRIDGE`, `RUN_BRIDGES_FOR_STAGE`, and `FINALIZE_EPOCHS_FOR_BRIDGE`.
