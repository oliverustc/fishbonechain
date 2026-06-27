# Data Trade Demo Guide (Stage 12 freeze)

本文档面向论文写作和演示场景，提供可直接复制的命令来运行数据交易 ZK 证明管线、多约束 range 请求和失败/争议场景。

## 前置条件

```bash
# 构建 ZK CLI 二进制
go -C tools/data-trade-zk build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk

# 检查 JS 脚本语法
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js
```

## 1. 无链 Dry-Run 验证

这些命令不需要 live chain RPC，在普通开发环境中可运行。证据记录为 `dry-run-accepted`，不声称链上状态。

### 1.1 单 range 请求

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --evidence-out target/data-trade-stage12/factory-temperature-dry-run/evidence.json \
  --dry-run-dynamic
```

证明：factory_sensors dataset 的 temperature 字段（值 42）落在请求范围 [18, 65] 内。

### 1.2 多约束 multi_range 请求

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --evidence-out target/data-trade-stage12/factory-multi-range-dry-run/evidence.json \
  --dry-run-dynamic
```

证明：同一 record 的 temperature 和 pressure 字段同时满足各自的 range 约束。

### 1.3 不同 dataset/request 组合

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/vehicle_telematics.json \
  --request scripts/fixtures/data_trade_requests/vehicle_speed_range.json \
  --evidence-out target/data-trade-stage12/vehicle-speed-dry-run/evidence.json \
  --dry-run-dynamic
```

证明：不同 dataset 的 pipeline 产生不同的 `business_input_hash` 和 `proof_digest`。

## 2. 负向验证（应正确 reject）

这些命令验证请求/数据集一致性校验在链交互之前就拒绝无效组合。

### 2.1 值超出 range

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_out_of_range.json \
  --dry-run-dynamic
```

预期：exit non-zero，错误发生在 `make-witness` 阶段（不是链上 dispute，而是 witness 构造拒绝）。

### 2.2 multi_range 中某约束超出 range

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range_out_of_range.json \
  --dry-run-dynamic
```

预期：exit non-zero，pressure 值 1013 不满足 [2000, 3000] 约束。

## 3. Live-Chain 命令

以下命令需要可用的 RPC 端点（`scripts/profiles/chains.json` 中配置）。**当前环境 RPC 不可用，以下命令仅供文档参考，未经此阶段运行。**

### 3.1 Live chain happy path

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --evidence-out target/data-trade-stage12/live-happy-multi-range/evidence.json
```

预期结果：
- 两个已完成的 round
- 链上记录 proof digest + verifier attestation
- MainEscrow 结算完成
- evidence `result` 为 `"accepted"`

### 3.2 Live chain failure/dispute 场景

无效 proof 争议：
```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --scenario invalid-proof-dispute \
  --evidence-out target/data-trade-stage12/live-invalid-proof/evidence.json
```

无效 plaintext 争议：
```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --scenario invalid-plaintext-dispute \
  --evidence-out target/data-trade-stage12/live-invalid-plaintext/evidence.json
```

请求者拒付（DO 索赔最后一笔款项）：
```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --scenario requester-refuses-payment \
  --evidence-out target/data-trade-stage12/live-requester-refuses-payment/evidence.json
```

| 场景 | 预期 `result` | 预期链上事件 |
|------|--------------|-------------|
| `invalid-proof-dispute` | `"expected-dispute-accepted"` | `tradeSession.SessionPunished`, `mainEscrow.EscrowPunished` |
| `invalid-plaintext-dispute` | `"expected-plaintext-dispute-accepted"` | `tradeSession.SessionPunished`, `mainEscrow.EscrowPunished` |
| `requester-refuses-payment` | `"expected-last-payment-claimed"` | `tradeSession.LastPaymentClaimed`, `mainEscrow.EscrowSettled` |

## 安全声明

- **Dry-run 命令**不声称链上状态，不涉及任何信任假设。
- **Live-chain 命令**依赖 Charlie 验证者密钥（开发密钥，非生产多签 quorum）。
- **Proof verification**是链外 gnark Groth16 verify，不是链上 Groth16 verifier。
- **Cross-chain settlement**由 E2E 脚本在链外协调，不是 trustless CCMC/Merkle proof 桥。
