# Stage 13 数据交易全流程质量基线验证报告

日期：2026-06-27
执行分支：`feat/data-trade-stage13-quality-baseline`
执行负责人：Codex
计划文件：`docs/internal/agent-plans/2026-06-27-stage13-data-trade-quality-baseline.md`

## 结论

结论等级：`green`

本地代码质量、ZK 工具链、pallet 单元测试、Stage 12 dry-run demo matrix、负向验证和恢复 child6 后的 live-chain 验证均通过。

因此：

- 当前可以作为论文 dry-run / proof pipeline 证据使用。
- 当前可以作为 Stage 13 live-chain 脚本化数据交易证据使用。
- 当前不需要继续实现更多核心脚本功能才能证明脚本级链上/链下数据交易闭环。

## 初始状态

执行时间：

```text
2026-06-27T22:48:25+08:00
```

初始 HEAD：

```text
b4da8c3 plan: define Stage 13 data trade quality baseline
```

初始分支：

```text
feat/data-trade-stage13-quality-baseline
```

初始工作区：

```text
## feat/data-trade-stage13-quality-baseline
 M agent.md
```

说明：`agent.md` 是执行前已存在的未提交改动。按计划未提交、未回滚、未纳入 Stage 13。

## 基础检查

### JS / Bash syntax

执行：

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
node --check scripts/lib/vm_regression_summary.js
node --check scripts/lib/wait_for_ws_chain.js
bash -n scripts/run_data_trade_vm_regression.sh
```

结果：passed。

### Rust pallet tests

执行：

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-data-registry
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session
SKIP_WASM_BUILD=1 cargo test -p pallet-main-escrow
```

结果：

| Package | Result |
|---------|--------|
| `pallet-data-registry` | 12 passed |
| `pallet-trade-session` | 19 passed |
| `pallet-main-escrow` | 9 passed |

合计：40 passed, 0 failed。

备注：Cargo 输出了 `trie-db v0.30.0` future-incompat warning，不影响本次测试结果。

### Go ZK toolchain

执行：

```bash
go -C tools/data-trade-zk test ./...
go -C tools/data-trade-zk build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
```

结果：passed。

测试包：

```text
fishbone-data-trade-zk/internal/artifact
fishbone-data-trade-zk/internal/business
fishbone-data-trade-zk/internal/dynamic
fishbone-data-trade-zk/internal/gnarkadapter
fishbone-data-trade-zk/internal/imt
```

`target/tools/fishbone-zk` 构建成功。

## Stage 12 positive dry-run matrix

证据目录：

```text
/tmp/fishbone-stage13-quality/
```

### factory temperature range

命令：

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --evidence-out /tmp/fishbone-stage13-quality/factory-temperature-dry-run/evidence.json \
  --dry-run-dynamic
```

结果：passed。

Evidence summary：

```json
{
  "scenario": "happy",
  "mode": "dynamic-dry-run",
  "result": "dry-run-accepted",
  "request_hash": "0x6b9f5a1765adf428a2b7220c2fa6e11ef4f3d8235dc145d7c42b3e26fbd13a01",
  "listing_id": null,
  "escrow_id": null,
  "session_id": 0,
  "settlement": null,
  "constraint_kind": "range",
  "constraints": [
    {
      "field_name": "temperature",
      "proof_digest": "0x55c61a8752de8a650e19e46f88362080a377ea5bd4320a479f3f0b86a9549ebc",
      "business_input_hash": "0x6583996ba8deadd960ffd5369b0f9cd907a5191ede6334be83ffb25854cd9c29",
      "on_chain_bound": true
    }
  ]
}
```

### factory multi-range

命令：

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --evidence-out /tmp/fishbone-stage13-quality/factory-multi-range-dry-run/evidence.json \
  --dry-run-dynamic
```

结果：passed。

Evidence summary：

```json
{
  "scenario": "happy",
  "mode": "dynamic-dry-run",
  "result": "dry-run-accepted",
  "request_hash": "0xc1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2",
  "listing_id": null,
  "escrow_id": null,
  "session_id": 0,
  "settlement": null,
  "constraint_kind": "multi_range",
  "constraints": [
    {
      "field_name": "temperature",
      "proof_digest": "0xd12aba3e1e95e0fdf2c496c0cb7e955d6170bd3f9df6f9d30074f2ce2ae8210a",
      "business_input_hash": "0x6583996ba8deadd960ffd5369b0f9cd907a5191ede6334be83ffb25854cd9c29",
      "on_chain_bound": true
    },
    {
      "field_name": "pressure",
      "proof_digest": "0xd8adb500809235c9451a57c5a1591efe91d768a59af8173d7743da889d7f8684",
      "business_input_hash": "0x81ea05461b33946c4ec7a64312a3e929aae4d3c27d054f6dd873df33b72982ab",
      "on_chain_bound": false
    }
  ]
}
```

### vehicle speed range

命令：

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/vehicle_telematics.json \
  --request scripts/fixtures/data_trade_requests/vehicle_speed_range.json \
  --evidence-out /tmp/fishbone-stage13-quality/vehicle-speed-dry-run/evidence.json \
  --dry-run-dynamic
```

结果：passed。

Evidence summary：

```json
{
  "scenario": "happy",
  "mode": "dynamic-dry-run",
  "result": "dry-run-accepted",
  "request_hash": "0x7c8d9e0f1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5",
  "listing_id": null,
  "escrow_id": null,
  "session_id": 0,
  "settlement": null,
  "constraint_kind": "range",
  "constraints": [
    {
      "field_name": "speed",
      "proof_digest": "0xa56fd0b8520c2742fcb2058e1b1dfc9d8fbf9856bb2a97785ae11641954d966f",
      "business_input_hash": "0xc90cf615a992024547ad9e9360bd2bf227db39dd470881a44369bb052730362b",
      "on_chain_bound": true
    }
  ]
}
```

## Negative validation

### factory temperature out of range

命令：

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_out_of_range.json \
  --dry-run-dynamic
```

结果：passed as expected, exit code `1`。

关键错误：

```text
build witness: field value 42 outside request range [100, 200]
fishbone-zk make-witness failed: 1
```

### factory multi-range out of range

命令：

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range_out_of_range.json \
  --dry-run-dynamic
```

结果：passed as expected, exit code `1`。

关键错误：

```text
build witnesses: constraint "pressure": field value 1013 outside request range [2000, 3000]
fishbone-zk make-witness failed: 1
```

结论：两个负向验证都在 `make-witness` 阶段失败，未进入链上交互。

## RPC / live-chain 初次探测

执行：

```bash
node scripts/lib/wait_for_ws_chain.js \
  --main ws://10.2.2.11:9944 \
  --child ws://10.2.2.11:9950 \
  --min-blocks 1 \
  --timeout-ms 30000
```

结果：failed，exit code `1`。

观测：

```text
main Fishbone Main #102582 advanced=0/1
main Fishbone Main #102583 advanced=1/1
child not ready: timed out after 15000ms
child ws://10.2.2.11:9950 did not become ready before deadline: timed out after 15000ms
```

判断：

- main RPC 可用并推进了 1 个块。
- child6 RPC 不可用或未在超时窗口内 ready。
- 初次执行时未运行 live-chain happy path。
- 初次执行时未运行 Stage 11 live-chain failure/dispute 场景。
- 未执行 clean redeploy 或任何 destructive 操作。

## Child6 恢复后 live-chain 补跑

child6 后续已恢复。恢复过程记录见：

```text
docs/internal/agent-reviews/2026-06-27-data-trade-stage13-child6-recovery.md
```

恢复后重新执行 readiness 检查：

```bash
node scripts/lib/wait_for_ws_chain.js \
  --main ws://10.2.2.11:9944 \
  --child ws://10.2.2.11:9950 \
  --min-blocks 2 \
  --timeout-ms 120000
```

结果：passed。

```text
main Fishbone Main #102875 -> #102877
child Fishbone Child-6 (Data Trade, AURA-5) #94 -> #96
```

### live-chain happy path

命令：

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --evidence-out /tmp/fishbone-stage13-quality/live-happy-multi-range/evidence.json
```

结果：passed。

Evidence：

```json
{
  "scenario": "happy",
  "mode": "dynamic",
  "result": "accepted",
  "listing_id": 0,
  "escrow_id": 0,
  "session_id": 0,
  "settlement": {
    "completed_rounds": 2,
    "remaining_rounds": 1
  },
  "rounds": [
    {
      "round_index": 0,
      "constraint_kind": "multi_range",
      "constraints": [
        {
          "field_name": "temperature",
          "proof_digest": "0xda4d477ab28f94b266425991ec84bd9a02056f1a08be1fce06c0337c31e4143c",
          "business_input_hash": "0x6583996ba8deadd960ffd5369b0f9cd907a5191ede6334be83ffb25854cd9c29",
          "on_chain_bound": true
        },
        {
          "field_name": "pressure",
          "proof_digest": "0xe5bb83512965043d975bb9f0bd67a3d7cabf0a6dc52f6c7eb21111092a19584c",
          "business_input_hash": "0x81ea05461b33946c4ec7a64312a3e929aae4d3c27d054f6dd873df33b72982ab",
          "on_chain_bound": false
        }
      ]
    },
    {
      "round_index": 1,
      "constraint_kind": "multi_range",
      "constraints": [
        {
          "field_name": "temperature",
          "proof_digest": "0x2dbdc2c49bf10699e90b3bb647cf15dd902d22750085f94e875251ecb3d3c66f",
          "business_input_hash": "0x6583996ba8deadd960ffd5369b0f9cd907a5191ede6334be83ffb25854cd9c29",
          "on_chain_bound": true
        },
        {
          "field_name": "pressure",
          "proof_digest": "0xa6c55f9c1f61bf1b15871f34a0e8c9a6e9de004b4b58066f6771904ffc734bb6",
          "business_input_hash": "0x81ea05461b33946c4ec7a64312a3e929aae4d3c27d054f6dd873df33b72982ab",
          "on_chain_bound": false
        }
      ]
    }
  ]
}
```

日志关键结果：

```text
claimSettlement (2/3 rounds)...
settleByPreimage on main...
Real ZK-attested path 完成
verifier=gnark-groth16-bn254 (off-chain proof, on-chain attestation)
```

### live-chain failure/dispute scenarios

#### invalid-proof-dispute

命令：

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --scenario invalid-proof-dispute \
  --evidence-out /tmp/fishbone-stage13-quality/live-invalid-proof/evidence.json
```

结果：passed。

Evidence summary：

```json
{
  "scenario": "invalid-proof-dispute",
  "mode": "dynamic",
  "result": "expected-dispute-accepted",
  "listing_id": 1,
  "escrow_id": 1,
  "session_id": 1,
  "scenario_outcome": {
    "type": "invalid-proof",
    "child_event": "tradeSession.SessionPunished",
    "main_event": "mainEscrow.EscrowPunished",
    "submitted_digest": "0x7be0dde11dccf21554f47113d997ce23f180882655fb72ab79d551750f8a2bfe",
    "evidence_bad_digest": "0x8be0dde11dccf21554f47113d997ce23f180882655fb72ab79d551750f8a2bfe",
    "bad_digest_differs_from_submitted": true,
    "events": [
      "tradeSession.SessionPunished",
      "mainEscrow.EscrowPunished"
    ]
  }
}
```

#### invalid-plaintext-dispute

命令：

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --scenario invalid-plaintext-dispute \
  --evidence-out /tmp/fishbone-stage13-quality/live-invalid-plaintext/evidence.json
```

结果：passed。

Evidence summary：

```json
{
  "scenario": "invalid-plaintext-dispute",
  "mode": "dynamic",
  "result": "expected-plaintext-dispute-accepted",
  "listing_id": 2,
  "escrow_id": 2,
  "session_id": 2,
  "scenario_outcome": {
    "type": "invalid-plaintext",
    "child_event": "tradeSession.SessionPunished",
    "main_event": "mainEscrow.EscrowPunished",
    "events": [
      "tradeSession.SessionPunished",
      "mainEscrow.EscrowPunished"
    ]
  }
}
```

#### requester-refuses-payment

命令：

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --scenario requester-refuses-payment \
  --evidence-out /tmp/fishbone-stage13-quality/live-requester-refuses-payment/evidence.json
```

结果：passed。

Evidence summary：

```json
{
  "scenario": "requester-refuses-payment",
  "mode": "dynamic",
  "result": "expected-last-payment-claimed",
  "listing_id": 3,
  "escrow_id": 3,
  "session_id": 3,
  "scenario_outcome": {
    "type": "requester-refuses-payment",
    "child_event": "tradeSession.LastPaymentClaimed",
    "main_event": "mainEscrow.EscrowSettled",
    "events": [
      "tradeSession.LastPaymentClaimed",
      "mainEscrow.EscrowSettled"
    ]
  }
}
```

### live-chain 后置健康检查

执行：

```bash
node scripts/lib/wait_for_ws_chain.js \
  --main ws://10.2.2.11:9944 \
  --child ws://10.2.2.11:9950 \
  --min-blocks 1 \
  --timeout-ms 60000
```

结果：passed。

```text
main Fishbone Main #102955 -> #102956
child Fishbone Child-6 (Data Trade, AURA-5) #173 -> #174
```

## Git / generated artifacts

仓库状态检查：

```text
## feat/data-trade-stage13-quality-baseline
 M agent.md
```

`agent.md` 是执行前已存在的未提交改动，未纳入本报告提交。

生成证据位于：

```text
/tmp/fishbone-stage13-quality/
```

文件：

```text
/tmp/fishbone-stage13-quality/factory-temperature-dry-run/evidence.json
/tmp/fishbone-stage13-quality/factory-multi-range-dry-run/evidence.json
/tmp/fishbone-stage13-quality/vehicle-speed-dry-run/evidence.json
/tmp/fishbone-stage13-quality/factory-temperature-out-of-range.log
/tmp/fishbone-stage13-quality/factory-multi-range-out-of-range.log
/tmp/fishbone-stage13-quality/live-happy-multi-range/evidence.json
/tmp/fishbone-stage13-quality/live-happy-multi-range/run.log
/tmp/fishbone-stage13-quality/live-invalid-proof/evidence.json
/tmp/fishbone-stage13-quality/live-invalid-proof/run.log
/tmp/fishbone-stage13-quality/live-invalid-plaintext/evidence.json
/tmp/fishbone-stage13-quality/live-invalid-plaintext/run.log
/tmp/fishbone-stage13-quality/live-requester-refuses-payment/evidence.json
/tmp/fishbone-stage13-quality/live-requester-refuses-payment/run.log
```

未跟踪或提交 `target/data-trade-stage12/`、`target/data-trade-zk/`、`.deepseek/` 生成物。

备注：执行过程中有一次检查命令误把 `/tmp/fishbone-stage13-quality` 传给 `git ls-files`，得到 outside repository 错误；随后用仓库内路径重新检查，确认无生成物被 git 追踪。该误用不影响项目验证结果。

## 质量判断

### 可以作为论文证据的部分

- 本地 Rust pallet 状态机单元测试。
- Go ZK 工具链测试与构建。
- Stage 12 三条 dry-run proof pipeline evidence。
- 两条 out-of-range negative validation。

这些证据说明当前脚本级无链 ZK pipeline 和数据/请求动态处理是可复现的。

### 可以作为论文 live-chain 证据的部分

- Stage 13 产出了 live-chain happy path evidence。
- Stage 13 产出了 `invalid-proof-dispute` evidence。
- Stage 13 产出了 `invalid-plaintext-dispute` evidence。
- Stage 13 产出了 `requester-refuses-payment` evidence。

这些证据说明在恢复后的 child6 环境中，脚本级链上注册、托管、会话、proof/attestation、结算、争议惩罚和 last-payment claim 路径可以复现。

## 下一步建议

1. 当前质量基线足够支撑继续整理论文实验材料。
2. 如果要进入前端阶段，建议复用本次 live-chain 命令作为前端验收 oracle。
3. 后续不要把 `/tmp/fishbone-stage13-quality/` 或 `target/data-trade-zk/` 作为 git 产物提交，只在报告中记录路径和摘要。
4. 如果继续部署更多数据交易子链，应先按 `docs/operations/subchain-deployment-runbook.md` 做 spec/key/roles/出块验收。

## 最终结论

Stage 13 当前结论为 `green`：

- 本地代码质量基线通过。
- 脚本级 dry-run 数据交易 proof pipeline 通过。
- 负向验证通过。
- child6 恢复后 live-chain happy path 通过。
- child6 恢复后三个 failure/dispute 场景通过。
