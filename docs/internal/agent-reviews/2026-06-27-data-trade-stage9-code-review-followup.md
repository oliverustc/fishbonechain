# Stage 9 Dynamic Scripted E2E Code Review Follow-up

> Review owner: Codex
>
> Implementation owner: CodeWhale
>
> Branch: `feat/data-trade-stage9-dynamic-e2e`
>
> Decision: **approved**

## Previous Findings

### Medium: Dynamic live mode validates dataset/request after chain side effects

Status: **fixed**.

`scripts/zk_real_data_trade_flow.js` now runs a dynamic live-mode preflight before `ApiPromise.create(...)` and before any chain transaction. The preflight uses the existing Stage 8 CLI path:

```bash
fishbone-zk make-witness \
  --dataset <dataset> \
  --request <request> \
  --out target/data-trade-zk/preflight-<timestamp>/witness.json \
  --session-id 0 \
  --round-index 0
```

The preflight witness is not reused for real rounds. Session-bound witnesses are still generated after the actual `sessionId` is known.

### Low: Stage 9 security-model documentation was required but not updated

Status: **fixed**.

`docs/architecture/data-trade-security-model.md` now records Stage 9 as dynamic scripted E2E and preserves the current security boundary: range-only, script-only, off-chain verifier plus on-chain attestation; no on-chain ZK verification, trustless bridge, subset/substr, verifier quorum, or frontend.

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
```

Result: passed.

```bash
go -C tools/data-trade-zk build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
```

Result: passed.

```bash
rm -rf /tmp/fishbone-stage9-followup
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --evidence-out /tmp/fishbone-stage9-followup/evidence.json \
  --dry-run-dynamic
test -f /tmp/fishbone-stage9-followup/evidence.json
```

Result: passed. Dynamic dry-run generated evidence with `result: "dry-run-accepted"`.

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_out_of_range.json \
  --evidence-out /tmp/fishbone-stage9-followup-bad/evidence.json
```

Result: exited with code `1` during dynamic preflight:

```text
preflight: validating dynamic dataset/request before chain connection...
build witness: field value 42 outside request range [100, 200]
dynamic request validation failed before chain connection
```

This confirms invalid dynamic input is rejected before the script reaches `ApiPromise.create(...)` or any chain transaction.

## Residual Notes

- The script still logs the configured main/child chain URLs at startup before preflight. This is only a log message; actual chain connection happens after preflight. It can be made clearer later, but it does not block Stage 9.
- No live chain E2E was run in this follow-up review.
