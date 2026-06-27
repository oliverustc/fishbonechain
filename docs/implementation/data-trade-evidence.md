# 数据交易实现证据记录

本文记录当前 FishboneChain 数据交易实现的可复现证据。它区分“本次本地验证结果”和“历史 VM 回归结果”，避免把未重跑的 VM 结果误写成当前现场结论。

## Scope

覆盖数据交易主线：

- `pallet-data-registry`
- `pallet-trade-session`
- `pallet-main-escrow`
- `tools/data-trade-zk`
- `scripts/data_trade_flow.js`
- `scripts/zk_attested_data_trade_flow.js`
- `scripts/zk_real_data_trade_flow.js`
- `scripts/run_data_trade_vm_regression.sh`

## Current Commit

- Validation commit: `afe0720a19ebd22b908f9206fd25817381cf76c4`
- Branch at validation time: `main`
- Validation time: `2026-06-26 21:17:47 CST`

The local validation commands were run after the Stage 5 plan commit and before the evidence/matrix commits were created. The later Stage 5 commits only changed documentation; no Rust, Go, JavaScript, runtime, deployment, or script code changed between the validation commit and the final Stage 5 deliverable.

## Local Validation

The following commands passed locally on 2026-06-26:

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-data-registry
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session
SKIP_WASM_BUILD=1 cargo test -p pallet-main-escrow
go -C tools/data-trade-zk test ./...
```

Results:

| Component | Command | Result |
|-----------|---------|--------|
| DataRegistry pallet | `SKIP_WASM_BUILD=1 cargo test -p pallet-data-registry` | 12 passed |
| TradeSession pallet | `SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session` | 19 passed |
| MainEscrow pallet | `SKIP_WASM_BUILD=1 cargo test -p pallet-main-escrow` | 9 passed |
| Go ZK toolchain | `go -C tools/data-trade-zk test ./...` | passed |

The following syntax checks also passed:

```bash
node --check scripts/data_trade_flow.js
node --check scripts/zk_attested_data_trade_flow.js
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/bridges/data_trade.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_attestation.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/data_trade_events.js
node --check scripts/lib/data_trade_binding.js
node --check scripts/lib/hash_chain.js
node --check scripts/lib/data_trade_sample.js
node --check scripts/lib/trade_profile.js
node --check scripts/lib/vm_regression_summary.js
node --check scripts/lib/wait_for_ws_chain.js
bash -n scripts/run_data_trade_vm_regression.sh
```

## VM Validation

### Historical Full VM Regression

A prior clean VM regression summary exists at:

- `target/data-trade-vm-regression/summary.md`
- `target/data-trade-vm-regression/summary.json`
- `target/data-trade-vm-regression/artifacts.txt`

Recorded result:

| Field | Value |
|-------|-------|
| Status | passed |
| Started | `2026-06-16T07:16:23.357Z` |
| Finished | `2026-06-16T07:26:32.019Z` |
| Main endpoint | `ws://10.2.2.11:9944` |
| Child endpoint | `ws://10.2.2.11:9950` |
| Deployment | clean redeploy `main,child6` |

Covered steps:

| Step | Result |
|------|--------|
| ZK CLI availability | ok |
| Clean redeploy `main,child6` | ok |
| Main/child6 RPC readiness | ok |
| `data_trade_flow.js --scenario happy` | ok |
| `data_trade_flow.js --scenario invalid-proof` | ok |
| `data_trade_flow.js --scenario requester-refuses-payment` | ok |
| `zk_attested_data_trade_flow.js` | ok |
| `zk_real_data_trade_flow.js` | ok |

### Current VM Reachability Check

On 2026-06-26, a destructive clean redeploy was not run during Stage 5A. A read-only RPC check showed:

| Endpoint | Result |
|----------|--------|
| `ws://10.2.2.11:9944` | main chain reachable and advancing |
| `ws://10.2.2.11:9950` | child6 RPC timed out |
| `ws://10.2.2.11:9951` | child7 RPC timed out |

Commands used:

```bash
node scripts/lib/wait_for_ws_chain.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --min-blocks 1 --timeout-ms 30000
node scripts/lib/wait_for_ws_chain.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9951 --min-blocks 1 --timeout-ms 30000
```

Because child6/child7 were not reachable and the regression script can clean-reset VM chains, the current Stage 5 pass records the VM rerun as blocked rather than rerunning destructively.

## E2E Scenarios Covered

The implementation and historical VM regression cover:

| Scenario | Coverage |
|----------|----------|
| Base happy path | DO publishes listing, DR locks funds, DO locks deposit, two-round delivery, DO claim, main escrow settlement |
| Invalid proof dispute | DR disputes invalid proof, session punished, MainEscrow can punish DO |
| Requester refuses payment | DO claims last payment when DR refuses final payment |
| Dev ZK attestation | ZK digest plus Charlie attestation path |
| Real gnark ZK path | `fishbone-zk` generates/verifies proof artifact, chain records digest and attestation |

## Artifacts and Output Paths

Important local artifact paths:

```text
target/data-trade-vm-regression/summary.md
target/data-trade-vm-regression/summary.json
target/data-trade-vm-regression/artifacts.txt
target/data-trade-zk/session-*/artifact.json
```

Example artifact fields from `target/data-trade-zk/session-0-round-0/artifact.json`:

| Field | Example |
|-------|---------|
| `proof_system` | `gnark-groth16-bn254` |
| `constraint_kind` | `range` |
| `ro_depth` | `10` |
| `business_input_hash` | present |
| `proof_digest` | present |
| `ch_proof` | `artifacts/ch_range.proof` |
| `ro_proof` | `artifacts/ro_depth10.proof` |

## Known Limitations

- No on-chain Groth16 proof verification.
- Runtime uses `AlwaysPassVerifier` and verifier attestation.
- Verifier authority is a single dev account (`//Charlie`).
- Bridge/session-escrow binding is checked off-chain.
- Full IMT membership is not implemented.
- Only the `range` business witness is implemented at circuit level.
- `subset`, `substr`, and multi-field constraint kinds are not implemented.
- `FmcAssisted` and `Hybrid` settlement modes are not wired.

## Reproduction Commands

Local validation:

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-data-registry
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session
SKIP_WASM_BUILD=1 cargo test -p pallet-main-escrow
go -C tools/data-trade-zk test ./...
```

VM regression, destructive because it can clean redeploy `main,child6`:

```bash
MAIN_WS=ws://10.2.2.11:9944 CHILD_WS=ws://10.2.2.11:9950 scripts/run_data_trade_vm_regression.sh
```

Child7 profile smoke, only when `child7` is deployed and reachable:

```bash
node scripts/zk_real_data_trade_flow.js --profile child7-business-trade
```

## Stage 9 动态脚本化 E2E 证据

Stage 9 引入 per-run evidence JSON（`session-\<id\>-evidence.json`），通过 `--dry-run-dynamic` 在无链环境下验证完整 ZK pipeline。

可复现 dry-run smoke 命令：

```bash
go -C tools/data-trade-zk build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --evidence-out /tmp/evidence.json \
  --dry-run-dynamic
```

Dry-run 证据格式示例见 `target/data-trade-zk/session-0-evidence.json`。注意：dry-run 证据不涉及链上交互，`listing_id`、`escrow_id` 和 `settlement` 字段为 `null`。历史 VM/live-chain 证据（Stage 5）不受影响。
## Stage 10 多约束 Evidence

Stage 10 引入 `multi_range` 请求，支持同 record 多 field range AND。Evidence 归一化为 `rounds[].constraints[]` 数组格式。

Multi-range dry-run:
```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --evidence-out /tmp/evidence.json --dry-run-dynamic
```
## Stage 11 脚本化失败/争议证据

Stage 11 通过 `--scenario` 支持以下失败/争议路径：

- `invalid-proof-dispute`：DR 争议无效 proof → `SessionPunished` + `EscrowPunished`
- `invalid-plaintext-dispute`：DR 争议 hash 不匹配 → `SessionPunished` + `EscrowPunished`
- `requester-refuses-payment`：DR 拒付 → `LastPaymentClaimed` + `EscrowSettled`

Evidence 包含 `scenario`、`scenario_outcome`（含 `events` 数组）、`result` 字段。场景通过 `findEvent()` 断言预期链上事件。Live chain 场景验证未运行（RPC 不可用）。

各场景的 `result` 值：
- `invalid-proof-dispute` → `"expected-dispute-accepted"`
- `invalid-plaintext-dispute` → `"expected-plaintext-dispute-accepted"`
- `requester-refuses-payment` → `"expected-last-payment-claimed"`

## Stage 12 论文实验封版

Stage 12 将 Stage 8-11 的产出打包为论文可用的实验材料。

- **Demo guide**: [docs/implementation/data-trade-demo-guide.md](./data-trade-demo-guide.md) — 9 个可复制的 dry-run/live-chain 命令（完整 demo matrix）。
- **Evidence index**: [docs/implementation/data-trade-stage12-evidence-index.md](./data-trade-stage12-evidence-index.md) — 期望证据布局和每个命令的预期结果。

Stage 12 验证结果：
- Dry-run: 3 个 positive `accepted`，2 个 negative 正确 reject。
- Live-chain: 未运行（RPC 不可用）。
