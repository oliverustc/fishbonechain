# Stage 10 Multi-Range Constraint Code Review Follow-up

> Review owner: Codex
>
> Implementation owner: CodeWhale
>
> Branch: `feat/data-trade-stage10-multi-range`
>
> Decision: **approved**

## Previous Findings

### Medium: Valid `multi_range` live mode fails during preflight

Status: **fixed**.

`scripts/zk_real_data_trade_flow.js` now reuses `generateDynamicWitnessBundle()` in live preflight. This keeps the preflight output mode aligned with the real round path:

- single range uses `make-witness --out <witness.json>`;
- `multi_range` uses `make-witness --out-dir <witnesses-dir>`.

Review validation showed valid multi-range live mode now passes preflight and writes:

```text
witness[0]=target/data-trade-zk/preflight-.../witnesses/witness-0.json field=temperature
witness[1]=target/data-trade-zk/preflight-.../witnesses/witness-1.json field=pressure
witness_manifest=target/data-trade-zk/preflight-.../witnesses/manifest.json
preflight passed
```

The command was manually interrupted after preflight while waiting for local chain RPC connection. No chain E2E was claimed.

### Low: Required security/gap documentation was not updated

Status: **fixed**.

The security model now records Stage 10 as an off-chain/scripted multi-range conjunction over multiple `BusinessRangeProof` artifacts and states that the chain still binds one proof digest per round.

The paper gap matrix now records Stage 10 `multi_range` support while keeping subset/substr and aggregate proof as gaps.

## Validation Performed

```bash
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js
```

Result: passed.

```bash
go -C tools/data-trade-zk test ./...
go -C tools/data-trade-zk build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
```

Result: passed.

```bash
rm -rf target/data-trade-zk/session-0-round-0 /tmp/fishbone-stage10-followup-single
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --evidence-out /tmp/fishbone-stage10-followup-single/evidence.json \
  --dry-run-dynamic
test -f /tmp/fishbone-stage10-followup-single/evidence.json
```

Result: passed. Evidence uses normalized `rounds[].constraints[]` with one range constraint.

```bash
rm -rf target/data-trade-zk/session-0-round-0 /tmp/fishbone-stage10-followup-multi
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --evidence-out /tmp/fishbone-stage10-followup-multi/evidence.json \
  --dry-run-dynamic
test -f /tmp/fishbone-stage10-followup-multi/evidence.json
```

Result: passed. Evidence records two constraints, with constraint 0 marked `on_chain_bound: true` and constraint 1 marked `false`.

```bash
rm -rf target/data-trade-zk/session-0-round-0 /tmp/fishbone-stage10-followup-bad
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range_out_of_range.json \
  --evidence-out /tmp/fishbone-stage10-followup-bad/evidence.json \
  --dry-run-dynamic
```

Result: exited non-zero during `make-witness`, as expected, because `pressure=1013` is outside `[2000,3000]`.

```bash
rm -rf target/data-trade-zk/preflight-* /tmp/fishbone-stage10-followup-live
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --evidence-out /tmp/fishbone-stage10-followup-live/evidence.json
```

Result: preflight passed and generated a multi-range witness manifest under `target/data-trade-zk/preflight-.../witnesses/`. The process was interrupted after preflight while waiting for local chain RPC connection. This confirms the previous `--out is not allowed for multi_range` failure is fixed.

## Residual Notes

- Dynamic dry-run still writes to `target/data-trade-zk/session-0-round-0`, so concurrent dry-runs can interfere. Sequential use is fine. Consider unique dry-run output directories in a later hardening stage.
- No live chain E2E was completed during this follow-up review.
