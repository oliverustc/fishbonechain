# Stage 12 Evidence Index

本文档定义 `target/data-trade-stage12/` 下期望的证据布局，并记录每个 demo 命令的证据路径、类别、预期结果和论文使用注意事项。**本文保留为 Stage 12 历史记录。**

> **当前状态（2026-06-29）**：Stage 13 已在恢复后的 child6 上验证了 live-chain happy path 和三个 failure/dispute 场景（详见 [Stage 13 Quality Baseline](../internal/agent-reviews/2026-06-27-data-trade-stage13-quality-baseline.md)）。Stage 14 已将完整流程固化为 `scripts/run_data_trade_validation.sh` 一键脚本（详见 [Stage 14 Evidence Index](data-trade-stage14-evidence-index.md) 和 [Data Trade Validation Experiment](../experiments/data-trade-validation.md)）。以下是 Stage 12 撰写时的原始记录，当时 RPC 不可用，live-chain 未运行。

## 证据布局

```text
target/data-trade-stage12/
├── factory-temperature-dry-run/
│   └── evidence.json
├── factory-multi-range-dry-run/
│   └── evidence.json
├── vehicle-speed-dry-run/
│   └── evidence.json
├── factory-temperature-out-of-range/     (negative, no evidence generated)
├── factory-multi-range-out-of-range/     (negative, no evidence generated)
├── live-happy-multi-range/               (not run)
│   └── evidence.json
├── live-invalid-proof/                    (not run)
│   └── evidence.json
├── live-invalid-plaintext/                (not run)
│   └── evidence.json
└── live-requester-refuses-payment/        (not run)
    └── evidence.json
```

## 证据项目

### Dry-Run 验证

这些命令不需要 live chain RPC。证据记录为 `dry-run-accepted`，不声称链上状态。

| # | 类别 | Dataset | Request | 预期结果 | 验证状态 |
|---|------|---------|---------|----------|----------|
| 1 | dry-run | factory_sensors | factory_temperature_range | `result: dry-run-accepted` | ✅ passed |
| 2 | dry-run | factory_sensors | factory_multi_range | `result: dry-run-accepted` | ✅ passed |
| 3 | dry-run | vehicle_telematics | vehicle_speed_range | `result: dry-run-accepted` | ✅ passed |

Dry-run evidence 字段：
```json
{
  "scenario": "happy",
  "mode": "dynamic-dry-run",
  "result": "dry-run-accepted",
  "request_hash": "...",
  "listing_id": null,
  "escrow_id": null,
  "session_id": 0,
  "settlement": null,
  "rounds": [{
    "round_index": 0,
    "constraint_kind": "range|multi_range",
    "chain_binding_mode": "...",
    "constraints": [{...}]
  }]
}
```

### 负向验证

这些命令在 `make-witness` 阶段拒绝无效组合，不产生证据文件。

| # | 类别 | Dataset | Request | 预期结果 | 验证状态 |
|---|------|---------|---------|----------|----------|
| 4 | negative-validation | factory_sensors | factory_temperature_out_of_range | exit non-zero | ✅ passed |
| 5 | negative-validation | factory_sensors | factory_multi_range_out_of_range | exit non-zero | ✅ passed |

### Live-Chain 验证

所有 live-chain 命令都需要可用的 RPC 端点。

| # | 类别 | Scenario | 预期 result | 预期事件 | 运行状态 |
|---|------|----------|------------|----------|----------|
| 6 | live-chain-happy | happy | `"accepted"` | SessionCreated, proof/attestation events | ⚠️ not run |
| 7 | live-chain-dispute | invalid-proof-dispute | `"expected-dispute-accepted"` | SessionPunished, EscrowPunished | ⚠️ not run |
| 8 | live-chain-dispute | invalid-plaintext-dispute | `"expected-plaintext-dispute-accepted"` | SessionPunished, EscrowPunished | ⚠️ not run |
| 9 | live-chain-dispute | requester-refuses-payment | `"expected-last-payment-claimed"` | LastPaymentClaimed, EscrowSettled | ⚠️ not run |

## 论文使用注意事项

### 可声称的

- Groth16 证明（gnark BN254）在链下生成和验证的 pipeline
- 多约束 range 请求（AND）作为 business witness
- 结构化 IMT（四层 deterministic prototype）耦合进 proof
- 动态 dataset/request → witness → proof 管线
- E2E 流程中的失败路径（无效 proof/plaintext 争议、拒付）

### 必须标记为 "prototype" / "future work" 的

- 链上 Groth16 验证（当前是 `AlwaysPassVerifier` + 链外 verify）
- Trustless cross-chain settlement（当前是链下 bridge 脚本）
- 生产 verifier quorum（当前是单一 dev key Charlie）
- Full production IMT（当前是 lite deterministic prototype）
- Subset/substr 约束（未实现）
