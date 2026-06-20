# Progressive TPS Experiment Lessons

Date: 2026-06-20

This note records operational lessons from the 18-VM progressive TPS and mainchain bridge-pressure experiments. The goal is to reduce ad hoc recovery during later runs.

## Main Lessons

1. Always separate two experiment goals:
   - Child-chain TPS needs a stable enough accepted-submission window.
   - Mainchain pressure needs observable bridge traffic after `EpochFinalized`.

2. Do not use a very small TPS cap for final presentation data.
   - `CAPACITY_CAP=1280` is useful for smoke tests.
   - It is too short for stable final figures because it may finish in only a few blocks.
   - Use `CAPACITY_CAP=5000` or `10000` for final figures.

3. Short-epoch bridge profiles need cumulative capacity monitoring.
   - `acceptedSubmissionCount` resets at epoch boundaries.
   - The monitor must accumulate across resets.
   - Otherwise large-cap short-epoch runs can timeout or undercount.

4. Mainchain state must be clean for each bridge-pressure stage.
   - Reusing mainchain state while resetting child-chain data causes repeated child epoch IDs.
   - That can produce `AlreadyVoted` or duplicate digest/bill semantics.
   - Use `RESET_MAINCHAIN_EACH_STAGE_FOR_BRIDGE=1` for final bridge-pressure runs.

5. Bridge processes must not all submit with the same account.
   - Multiple concurrent bridge submissions from `//Alice` caused nonce/priority conflicts.
   - Use one funded miner signer per active child chain.

6. Runtime/spec changes require chain database cleanup.
   - Replacing binary/spec is not enough if existing chain data remains.
   - Substrate keeps the old genesis/runtime state.
   - Use `stop-clean` before redeploying a changed runtime profile.

7. Before every throughput run, stop all existing child services.
   - Residual child services can consume ports, CPU, memory, or produce misleading state.
   - Run `scripts/stop_all_child_services.sh --config deploy/config.progressive-18vm.toml`.

8. Treat worker RPC success and chain accepted TPS as different metrics.
   - `worker_burst.js` `okTPS` only means RPC accepted transaction submission.
   - Final TPS must use chain accepted counters from `capacity_monitor.js`.

9. Run long multi-stage experiments under a detached launcher.
   - Interactive PTY/session loss can terminate a valid run during finalization or later stages.
   - Use `nohup` with a launcher log for long N=1..6 runs.
   - Record the run ID and process ID before leaving the command unattended.

## Recommended Final Run Parameters

Use the bridge short-epoch profile, but avoid a very small cap:

```bash
PROFILE_FILE=scripts/profiles/progressive_tps_18vm_bridge.json
DEPLOY_CONFIG=deploy/config.progressive-18vm.toml
RESET_EACH_STAGE=1
RESET_MAINCHAIN_EACH_STAGE_FOR_BRIDGE=1
SETUP_MAX_WORKERS=1400
CAPACITY_CAP=10000
CAPACITY_MONITOR_TIMEOUT=420
WAIT_MIN_REMAINING_BLOCKS=40
SETUP_MAINCHAIN_FOR_BRIDGE=1
RUN_BRIDGES_FOR_STAGE=1
FINALIZE_EPOCHS_FOR_BRIDGE=1
FINALIZE_WAIT_SYNCING_SECONDS=240
N_START=1
N_END=6
```

## Acceptance Checks

After a final run:

1. `progressive_tps_summary.csv` has 6 rows.
2. Every row has `bridge_measurement_status=observed`.
3. `main_bridge_events` is nonzero and normally equals `2 * N`.
4. `main_bridge_pressure_pct` remains far below `1%`.
5. Bridge logs contain no `AlreadyVoted`, `Priority is too low`, `1014`, or `fatal`.
6. N=4 should be evaluated with a stable window; a tiny-cap run is not valid evidence of performance regression.
