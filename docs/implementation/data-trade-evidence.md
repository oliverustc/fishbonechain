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
