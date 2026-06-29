# 数据交易实验固化与一键复现

实验日期：2026-06-29
阶段：Stage 14
前置基线：Stage 13 `green`（live-chain happy path 和三个 failure/dispute 场景已通过）

## 实验目的

Stage 14 将 Stage 13 的手工验证流程固化为可复现、可审计的实验资产。通过一键验证脚本自动执行 dry-run proof pipeline、负向校验和 live-chain 数据交易闭环，输出结构化 evidence 摘要，为论文写作和后续平台化提供可引用的实验证据。

## 环境要求

- Node.js 运行时（脚本使用 `@polkadot/api`）
- Go 工具链（ZK CLI 构建）
- 无链 dry-run 场景不依赖 live RPC
- live-chain 场景需要可用的主链（main, `ws://10.2.2.11:9944`）和 child6 数据交易子链（`ws://10.2.2.11:9950`）RPC 端点

## 一键复现命令

### 仅 dry-run + 负向校验（无链，快速）

```bash
scripts/run_data_trade_validation.sh \
  --skip-live \
  --out .agents/fwf/runs/stage14/dry-run
```

预期用时：约 10-30 秒（不含首次 ZK 构建）。

### 完整验证（含 live-chain）

```bash
scripts/run_data_trade_validation.sh \
  --out .agents/fwf/runs/stage14/full
```

预期用时：约 5-10 分钟（取决于链交易确认速度），超时默认 300s/场景。

### 自定义端点

```bash
scripts/run_data_trade_validation.sh \
  --main ws://<main-host>:9944 \
  --child ws://<child-host>:9950 \
  --out /path/to/output
```

### CLI 参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--profile` | `child6-data-trade` | 交易 profile |
| `--main` | `ws://10.2.2.11:9944` | 主链 RPC |
| `--child` | `ws://10.2.2.11:9950` | 子链 RPC |
| `--out` | `target/data-trade-validation/stage14-<timestamp>/` | 输出目录 |
| `--zk-cmd` | `target/tools/fishbone-zk` | ZK CLI 路径 |
| `--skip-live` | false | 仅运行无链部分 |
| `--skip-dry-run` | false | 跳过 dry-run 矩阵 |
| `--skip-negative` | false | 跳过负向校验 |
| `--timeout-seconds` | 300 | 每场景超时 |
| `--no-build-zk` | false | 不自动构建 ZK CLI |

## 输出目录说明

```
<out>/
├── summary.json          # 结构化 evidence 摘要
├── summary.md            # 可读 Markdown 摘要
├── commands.log          # 所有执行的完整命令
├── readiness/
│   └── run.log           # RPC readiness 检查日志
├── dry-run/
│   ├── factory-temperature/
│   │   ├── run.log
│   │   └── evidence.json
│   ├── factory-multi-range/
│   │   ├── run.log
│   │   └── evidence.json
│   └── vehicle-speed/
│       ├── run.log
│       └── evidence.json
├── negative/
│   ├── factory-temperature-out-of-range/
│   │   └── run.log
│   └── factory-multi-range-out-of-range/
│       └── run.log
├── live/
│   ├── happy-multi-range/
│   │   ├── run.log
│   │   └── evidence.json
│   ├── invalid-proof/
│   │   ├── run.log
│   │   └── evidence.json
│   ├── invalid-plaintext/
│   │   ├── run.log
│   │   └── evidence.json
│   └── requester-refuses-payment/
│       ├── run.log
│       └── evidence.json
└── postcheck/
    └── run.log
```

## Scenario Matrix

| # | ID | 类别 | Dataset | Request | 预期结果 | 需 live RPC |
|---|-----|------|---------|---------|----------|-------------|
| 1 | dry-run-factory-temperature | dry-run | factory_sensors | factory_temperature_range | `result: dry-run-accepted` | 否 |
| 2 | dry-run-factory-multi-range | dry-run | factory_sensors | factory_multi_range | `result: dry-run-accepted` | 否 |
| 3 | dry-run-vehicle-speed | dry-run | vehicle_telematics | vehicle_speed_range | `result: dry-run-accepted` | 否 |
| 4 | neg-factory-temp-out | negative | factory_sensors | factory_temperature_out_of_range | exit non-zero (witness reject) | 否 |
| 5 | neg-factory-multi-out | negative | factory_sensors | factory_multi_range_out_of_range | exit non-zero (witness reject) | 否 |
| 6 | live-happy-multi-range | live-chain | factory_sensors | factory_multi_range | `result: accepted` | 是 |
| 7 | live-invalid-proof | live-chain | factory_sensors | factory_multi_range | `result: expected-dispute-accepted` | 是 |
| 8 | live-invalid-plaintext | live-chain | factory_sensors | factory_multi_range | `result: expected-plaintext-dispute-accepted` | 是 |
| 9 | live-requester-refuses-payment | live-chain | factory_sensors | factory_multi_range | `result: expected-last-payment-claimed` | 是 |

## Evidence 字段解释

`summary.json` 是 platform-oriented evidence metadata 的雏形：

| 字段 | 含义 | 示例 |
|------|------|------|
| `id` | 场景标识 | `"live-happy-multi-range"` |
| `category` | 场景类别 | `"dry-run"`, `"negative"`, `"live-chain"`, `"postcheck"` |
| `status` | 执行结果 | `"passed"`, `"failed"`, `"skipped"` |
| `scenario` | 业务场景名 | `"happy"`, `"invalid-proof-dispute"` |
| `result` | evidence 声明的结果 | `"accepted"`, `"dry-run-accepted"` |
| `listing_id` | 链上 listing ID | `0`（live-chain）或 `null`（dry-run） |
| `escrow_id` | 链上 escrow ID | `0`（live-chain）或 `null`（dry-run） |
| `session_id` | 链上 session ID | `0` |
| `settlement` | 结算信息 | `{"completed_rounds": 2, "remaining_rounds": 1}` |
| `scenario_outcome` | 争议/异常结果 | `{"type": "invalid-proof", "events": [...]}` |
| `events` | 链上事件列表 | `["tradeSession.SessionPunished", "mainEscrow.EscrowPunished"]` |
| `constraints` | 约束摘要 | `[{"field_name": "temperature", "proof_digest": "0x...", ...}]` |
| `error` | 错误摘要 | `null` 或错误消息字符串 |

## 当前能力边界（Stage 13/14）

### 论文可直接使用的结论

- Dry-run proof pipeline：三条不同 dataset/request 组合的 Groth16 BN254 proof 在链下生成和验证，evidence 记录 proof digest、business_input_hash、request_hash 绑定。
- 负向校验：out-of-range 输入在 `make-witness` 阶段被正确拒绝。
- Live-chain happy path：完整 listing→session→proof→attestation→settlement 闭环已通过。
- Live-chain 异常路径：`invalid-proof-dispute`（提交错误 proof digest 被 SessionPunished+EscrowPunished）、`invalid-plaintext-dispute`（明文与 business_input_hash 不匹配被惩罚）和 `requester-refuses-payment`（DO 通过 LastPaymentClaimed 拿回最后一笔款项）三个异常场景均已验证。

### Prototype / Future Work

- **链上 Groth16 验证**：当前使用 `AlwaysPassVerifier`，链上只验证 digest 和 attestation。论文应注明 proof 的链上验证为 prototype 阶段。
- **Trustless bridge**：MainEscrow 结算由脚本在链下协调，不依赖 trustless cross-chain Merkle proof。
- **Verifier quorum**：当前单一 dev key Charlie 签名 attestation。
- **Full production IMT**：当前为 deterministic lite prototype。
- **子链多实例部署**：child6 已验证可用；child7 尚未刷新验证。

## 安全声明

- `scripts/run_data_trade_validation.sh` 默认不执行 clean redeploy 或链数据删除操作。
- 负向校验预期 non-zero exit，不会进入链上交互。
- Readiness 失败时 live scenarios 被标记为 `skipped`，不会伪造 evidence。
- Evidence 中的 `proof_digest` 和 `business_input_hash` 来自链下 proof 管线，不是链上 Groth16 验证结果。

## 相关文档

- [Stage 14 Evidence Index](../implementation/data-trade-stage14-evidence-index.md)
- [Stage 13 Quality Baseline](../internal/agent-reviews/2026-06-27-data-trade-stage13-quality-baseline.md)
- [Stage 14 Plan](../internal/agent-plans/2026-06-28-stage14-data-trade-reproducible-validation.md)
- [Data Trade Demo Guide](../implementation/data-trade-demo-guide.md)
- [Data Trade Paper Gap Matrix](../implementation/data-trade-paper-gap-matrix.md)
