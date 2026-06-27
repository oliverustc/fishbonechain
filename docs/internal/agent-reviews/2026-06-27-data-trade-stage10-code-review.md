# Stage 10 Multi-Range Constraint Code Review

> Review owner: Codex
>
> Implementation owner: CodeWhale
>
> Branch: `feat/data-trade-stage10-multi-range`
>
> Decision: **changes requested**

## Findings

### Medium: Valid `multi_range` live mode fails during preflight

- File: `scripts/zk_real_data_trade_flow.js:173`
- Relevant code:
  - `generateDynamicWitnessBundle()` correctly uses `--out-dir` for `multi_range` at `scripts/zk_real_data_trade_flow.js:95`.
  - Live preflight still directly calls `fishbone-zk make-witness --out <file>` at `scripts/zk_real_data_trade_flow.js:179`.

The Stage 10 CLI intentionally rejects `--out` for `multi_range` requests. Because live preflight still uses `--out`, a valid multi-range request exits before chain connection:

```text
preflight: validating dynamic dataset/request before chain connection...
--out is not allowed for multi_range requests (use --out-dir)
dynamic request validation failed before chain connection
```

This means `--dry-run-dynamic` works for multi-range, but the actual live scripted E2E path cannot start with a valid multi-range request. Stage 10 requires live preflight to validate all constraints by invoking `make-witness` into a preflight directory before chain connection.

Required fix:

- Make live preflight use the same output-mode decision as the real round path:
  - single range: `make-witness --out <preflight>/witness.json`
  - multi-range: `make-witness --out-dir <preflight>/witnesses`
- Prefer reusing `generateDynamicWitnessBundle({ outDir: preflightDir, sessionId: 0, roundIndex: 0 })` for preflight, or add a small helper so the flag logic cannot diverge again.
- Add or record a validation command showing a valid multi-range request reaches past preflight. If chain RPC is unavailable, it is enough to show that the failure changes from `--out is not allowed` to RPC connection failure after preflight, without submitting transactions.

### Low: Required security/gap documentation was not updated

- Plan reference: `docs/internal/agent-plans/2026-06-27-stage10-data-trade-multi-range-constraints.md:543`
- Missing files:
  - `docs/architecture/data-trade-security-model.md`
  - `docs/implementation/data-trade-paper-gap-matrix.md`

The Stage 10 plan required updating both files. The implementation updated evidence, implementation, roadmap, and plan execution record, but security model still stops at Stage 9 and the gap matrix was not changed.

Required fix:

- Add a Stage 10 security-model entry stating:
  - `multi_range` is an off-chain/scripted conjunction of multiple `BusinessRangeProof` range artifacts.
  - the chain still binds one proof digest per round;
  - live compatibility binds the first verified constraint digest on-chain and records the full proof set in evidence;
  - no subset/substr, aggregate proof, on-chain verifier, trustless bridge, verifier quorum, or frontend.
- Update the gap matrix to reflect improved request flexibility through multi-range, while keeping subset/substr, production aggregate proof, and on-chain verification as gaps.

## Validation Performed

Passed:

```bash
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js
go -C tools/data-trade-zk test ./...
go -C tools/data-trade-zk build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
```

Passed:

```bash
rm -rf /tmp/fishbone-stage10-cli
target/tools/fishbone-zk make-witness \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --out-dir /tmp/fishbone-stage10-cli \
  --session-id 0 \
  --round-index 0
test -f /tmp/fishbone-stage10-cli/manifest.json
test -f /tmp/fishbone-stage10-cli/witness-0.json
test -f /tmp/fishbone-stage10-cli/witness-1.json
```

Passed:

```bash
target/tools/fishbone-zk make-witness \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --out /tmp/should-not-work.json \
  --session-id 0 \
  --round-index 0
```

Result: exited with code `2` and rejected `--out` for `multi_range`.

Passed:

```bash
target/tools/fishbone-zk make-witness \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --out-dir /tmp/should-not-work-dir \
  --session-id 0 \
  --round-index 0
```

Result: exited with code `2` and rejected `--out-dir` for single range.

Passed after cleaning the shared dry-run target directory:

```bash
rm -rf target/data-trade-zk/session-0-round-0 /tmp/fishbone-stage10-review-single
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --evidence-out /tmp/fishbone-stage10-review-single/evidence.json \
  --dry-run-dynamic
test -f /tmp/fishbone-stage10-review-single/evidence.json
```

Result: single-range dry-run passed and evidence used normalized `rounds[].constraints[]`.

Passed:

```bash
rm -rf /tmp/fishbone-stage10-review-multi
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --evidence-out /tmp/fishbone-stage10-review-multi/evidence.json \
  --dry-run-dynamic
test -f /tmp/fishbone-stage10-review-multi/evidence.json
```

Result: multi-range dry-run passed with two constraints. Evidence marked constraint 0 as `on_chain_bound: true` and constraint 1 as `false`.

Passed:

```bash
rm -rf /tmp/fishbone-stage10-review-bad
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range_out_of_range.json \
  --evidence-out /tmp/fishbone-stage10-review-bad/evidence.json \
  --dry-run-dynamic
```

Result: exited non-zero during `make-witness` because `pressure` was outside the requested range, as expected.

Failed as described in finding 1:

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --evidence-out /tmp/fishbone-stage10-review-live-preflight/evidence.json
```

Result: valid multi-range live mode failed in preflight with `--out is not allowed for multi_range requests (use --out-dir)`.

## Notes

- The first parallel dry-run review attempt reused `target/data-trade-zk/session-0-round-0` across concurrent commands and caused a transient single-range verify failure. Sequential rerun after cleaning the directory passed. This shared dry-run output directory is worth improving later, but it is not a Stage 10 blocker by itself.
- No live chain E2E was run during this review.
