# Stage 14 Evidence Index

本文档定义 `scripts/run_data_trade_validation.sh` 的标准输出证据布局和每个 scenario 的 id、类别、输入、预期结果与预期 evidence 字段。

本文档取代 Stage 12 evidence index 作为当前数据交易实验证据入口。Stage 12 历史语境保留在 `docs/implementation/data-trade-stage12-evidence-index.md`，并已添加前向引用。

## 标准输出目录结构

```text
<out>/
  summary.json
  summary.md
  commands.log
  readiness/
    run.log
  dry-run/
    factory-temperature/
      run.log
      evidence.json
    factory-multi-range/
      run.log
      evidence.json
    vehicle-speed/
      run.log
      evidence.json
  negative/
    factory-temperature-out-of-range/
      run.log
    factory-multi-range-out-of-range/
      run.log
  live/
    happy-multi-range/
      run.log
      evidence.json
    invalid-proof/
      run.log
      evidence.json
    invalid-plaintext/
      run.log
      evidence.json
    requester-refuses-payment/
      run.log
      evidence.json
  postcheck/
    run.log
```

## Scenario 规格

### Dry-Run

| ID | 类别 | Dataset | Request | 约束类型 | 预期 result | 预期 evidence 字段 |
|----|------|---------|---------|----------|------------|-------------------|
| `dry-run-factory-temperature` | dry-run | `factory_sensors` | `factory_temperature_range` | range | `dry-run-accepted` | `scenario: happy`, `mode: dynamic-dry-run`, `listing_id: null`, `escrow_id: null`, `constraints[0].field_name: temperature` |
| `dry-run-factory-multi-range` | dry-run | `factory_sensors` | `factory_multi_range` | multi_range | `dry-run-accepted` | `scenario: happy`, `mode: dynamic-dry-run`, `constraints[].field_name` 包含 `temperature` 和 `pressure` |
| `dry-run-vehicle-speed` | dry-run | `vehicle_telematics` | `vehicle_speed_range` | range | `dry-run-accepted` | `scenario: happy`, `mode: dynamic-dry-run`, `constraints[0].field_name: speed` |

### Negative Validation

| ID | 类别 | Dataset | Request | 预期行为 | 预期 evidence |
|----|------|---------|---------|----------|---------------|
| `neg-factory-temp-out` | negative | `factory_sensors` | `factory_temperature_out_of_range` | exit non-zero, `make-witness` 阶段拒绝 | 无 evidence.json 生成；run.log 含 "field value ... outside request range" |
| `neg-factory-multi-out` | negative | `factory_sensors` | `factory_multi_range_out_of_range` | exit non-zero, `make-witness` 阶段拒绝 | 无 evidence.json 生成；run.log 含约束失败信息 |

### Live-Chain

| ID | 类别 | Scenario | 预期 result | 预期链上事件 |
|----|------|----------|------------|-------------|
| `live-happy-multi-range` | live-chain | `happy` | `accepted` | SessionCreated, proof/attestation events |
| `live-invalid-proof` | live-chain | `invalid-proof-dispute` | `expected-dispute-accepted` | `tradeSession.SessionPunished`, `mainEscrow.EscrowPunished` |
| `live-invalid-plaintext` | live-chain | `invalid-plaintext-dispute` | `expected-plaintext-dispute-accepted` | `tradeSession.SessionPunished`, `mainEscrow.EscrowPunished` |
| `live-requester-refuses-payment` | live-chain | `requester-refuses-payment` | `expected-last-payment-claimed` | `tradeSession.LastPaymentClaimed`, `mainEscrow.EscrowSettled` |

Live-chain 场景需要可用的 main RPC 和 child6 RPC。Readiness 失败时所有 live scenarios 标记为 `skipped`，整体 status 为 `partial`。

### 后置检查

| ID | 类别 | 说明 |
|----|------|------|
| `postcheck` | postcheck | 所有 live scenarios 完成后检查 main/child RPC 仍在出块 |

## `summary.json` 字段说明

### 顶层

| 字段 | 类型 | 说明 |
|------|------|------|
| `version` | integer | Evidence summary 格式版本（当前为 `1`）。仅表示格式版本，不代表未来 Web/API 版本号。 |
| `kind` | string | 固定为 `"data_trade_validation"` |
| `stage` | string | 固定为 `"stage14"` |
| `status` | string | `"passed"` / `"failed"` / `"partial"` |
| `started_at` | ISO 8601 | 运行开始时间 |
| `finished_at` | ISO 8601 | 运行结束时间 |
| `environment` | object | 运行环境信息 |
| `readiness` | object | RPC readiness 检查结果 |
| `scenarios` | array | 场景记录列表 |

### environment

| 字段 | 说明 |
|------|------|
| `profile` | 交易 profile |
| `main_ws` | 主链 RPC URL |
| `child_ws` | 子链 RPC URL |
| `zk_cmd` | ZK CLI 路径 |
| `git_commit` | 运行时的 git commit |
| `git_branch` | 运行时的 git branch |

### readiness

| 字段 | 说明 |
|------|------|
| `main_ready` | boolean，主链是否通过 readiness |
| `child_ready` | boolean，子链是否通过 readiness |
| `main_diagnostic` | 主链诊断信息 |
| `child_diagnostic` | 子链诊断信息 |
| `checked_at` | 检查时间 |

### scenarios[].entry

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 场景标识 |
| `category` | string | `"dry-run"` / `"negative"` / `"live-chain"` / `"postcheck"` |
| `status` | string | `"passed"` / `"failed"` / `"skipped"` |
| `command` | string | 完整执行命令 |
| `log_path` | string | run.log 文件路径（相对于 out 目录） |
| `evidence_path` | string\|null | evidence.json 路径（dry-run/live-chain）或 null（negative/postcheck） |
| `scenario` | string\|null | 业务场景名 |
| `result` | string\|null | evidence 声明的结果 |
| `listing_id` | integer\|null | 链上 listing ID |
| `escrow_id` | integer\|null | 链上 escrow ID |
| `session_id` | integer\|null | 链上 session ID |
| `settlement` | object\|null | 结算信息（`completed_rounds`, `remaining_rounds`） |
| `scenario_outcome` | object\|null | 争议/异常结果详情 |
| `events` | array | 链上事件名称列表 |
| `constraints` | array | 约束摘要（`round_index`, `field_name`, `proof_digest`, `business_input_hash`, `on_chain_bound`） |
| `error` | string\|null | 错误摘要 |

## 与未来平台对象的映射

| summary 字段 | 未来平台对象 |
|-------------|-------------|
| `summary.scenarios[]` | `WorkflowRun` / `Evidence` 记录 |
| `scenario.listing_id` | `ChainEvent.listing` 引用 |
| `scenario.session_id` | `ChainEvent.session` 引用 |
| `scenario.escrow_id` | `ChainEvent.escrow` 引用 |
| `scenario.log_path` | Artifact metadata |
| `scenario.evidence_path` | Artifact metadata |
| `scenario.constraints[]` | per-constraint proof binding record |
| `scenario.events[]` | 链上事件名 → chain state query |

## 不提交生成物的说明

`<out>/` 目录下的 `run.log`、`evidence.json`、`summary.json`、`summary.md` 和 `commands.log` 是运行时生成物，不提交到 git 仓库。仅通过 summary 和文档记录关键结果摘要。

## 相关文档

- [Stage 14 Plan](../internal/agent-plans/2026-06-28-stage14-data-trade-reproducible-validation.md)
- [Stage 13 Quality Baseline](../internal/agent-reviews/2026-06-27-data-trade-stage13-quality-baseline.md)
- [Stage 12 Evidence Index (历史)](../implementation/data-trade-stage12-evidence-index.md)
- [Data Trade Validation Experiment](../experiments/data-trade-validation.md)
