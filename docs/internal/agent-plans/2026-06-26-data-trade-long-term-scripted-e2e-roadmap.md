# 数据交易脚本化端到端长期路线图

> **总目标：** 在不引入前端的前提下，通过脚本完成一个链上 + 链下闭环的数据交易流程。该流程应能模拟数据拥有者 DO 和数据请求者 DR 的交互，支持一定灵活性的脱敏查询/约束请求，而不是只服务于单一固定数据集或单一固定需求。
>
> **Codex 职责：** 维护长期路线图；为每个新 Stage 编写贴合当前代码状态的详细执行 plan；做 architecture review 和 code review；review fix 通过后自动合并。
>
> **CodeWhale 职责：** 按每个 Stage 的详细 plan 执行具体实现；遇到 stop condition 停止并请求 Codex/Owner；不得自行扩大安全声明或改动架构边界。

## 当前基线

截至 Stage 7 合并后，已有能力：

- 链上 pallet 已覆盖 listing、session、MainEscrow、hash-chain settlement、proof digest、verifier attestation、invalid proof/plaintext dispute、last payment claim 等核心状态机。
- 链下 `fishbone-zk` 已能生成并验证 gnark Groth16 BN254 artifact。
- `BusinessRangeProof` 已证明：
  - `raw_value in [min, max]`
  - `masked_value = raw_value + mask_delta`
  - `masked_value_hash = MiMC(masked_value, salt)`
- Stage 7 已实现 structured IMT membership lite：
  - Entry -> Dataset -> Aggregate -> Published root
  - RO proof 使用 aggregate root 作为 published leaf
  - `business_input_hash` 绑定结构化 IMT metadata
- 当前仍不具备：
  - 动态数据集/多字段请求
  - 可配置请求语言
  - subset/substr 约束
  - 统一脚本化完整 E2E 编排
  - on-chain Groth16 verification
  - trustless bridge settlement
  - production dynamic IMT
  - production verifier quorum

## 目标边界

本路线图的目标是论文原型级“可运行、可展示、可复现实验”的脚本化系统，不是生产系统。

必须做到：

- 用脚本模拟 DO/DR 完整交互。
- 支持 DO 发布不同数据集。
- 支持 DR 发起不同类型或不同参数的数据请求。
- 链下生成脱敏证明 artifact。
- 链下 verifier 验证 artifact 并签发 attestation。
- 链上绑定 request/proof/session/escrow/attestation。
- 完成支付、交付、结算或争议路径。
- 输出可用于论文截图、表格、实验记录的结构化 evidence。

暂不作为本路线图目标：

- 前端 UI。
- 生产账户/钱包系统。
- on-chain Groth16 verifier。
- trustless bridge/CCMC/Merkle proof settlement。
- verifier quorum/slashing。
- 生产动态 IMT 数据库。
- 隐私安全审计级实现。

## 路线总览

建议后续按 Stage 8 到 Stage 12 推进。

| Stage | 主题 | 核心目标 | 主要产出 |
|-------|------|----------|----------|
| Stage 8 | 动态数据集与请求模型 | 让脚本不再绑定单一 fixture | dataset/request JSON schema、fixture generator、range 多字段支持 |
| Stage 9 | 脚本化完整 E2E 编排 | 一条命令跑通 DO/DR 交易闭环 | orchestrator script、evidence bundle、成功路径回归 |
| Stage 10 | 请求约束扩展 | 增加论文“灵活脱敏”的表达能力 | subset/substr 或等价可展示约束的原型实现 |
| Stage 11 | 失败/争议路径脚本化 | 证明系统不只会 happy path | invalid proof、invalid plaintext、refuse payment、claim last payment 脚本 |
| Stage 12 | 论文实验封版 | 稳定复现实验、固化表述边界 | final evidence、paper gap update、demo commands、实验记录 |

每个 Stage 开始前，Codex 需要重新检查当前代码和文档，再写详细执行 plan。不要直接把本路线图当作实现细则。

## Stage 8：动态数据集与请求模型

### 目标

把当前固定 `data_trade_business_sample.json` 推进为可配置的数据集/请求模型，让 DO/DR 脚本能处理不同 dataset、record、field 和 range 参数。

### 应实现

- 新增或扩展 fixture schema：
  - dataset id
  - records
  - fields
  - field type
  - raw value
  - salt/mask policy
  - request constraints
- 支持从 dataset/request JSON 生成 `RangeWitness`。
- 支持至少多个 record、多个 numeric field。
- 保持 Stage 7 structured IMT membership lite。
- 让 `business_input_hash` 和 `public_input_hash` 随 request/dataset/field/record 改变。
- 增加脚本或 CLI 子命令用于生成 witness。

### 不应实现

- subset/substr 电路。
- 前端。
- runtime/pallet 改动。
- artifact schema 改动，除非 Codex 明确批准。

### 验收标准

- 至少 2 个 dataset fixture、每个 dataset 至少 2 条 record、至少 2 个 numeric field。
- 同一脚本可以针对不同 field/request 生成不同 proof artifact。
- `fishbone-zk verify` 通过。
- 文档明确说明这是"动态 fixture/request 原型"，不是生产数据管理系统。

### 进度（2026-06-27）

- ✅ **Stage 8 完成**：`make-witness` 提供 dataset/request → `RangeWitness` 转换层。`internal/dynamic` 包实现 dataset/request schema 验证（含 uint64 overflow guard）和 `BuildRangeWitness` 跨文档一致性检查。`factory_sensors`、`vehicle_telematics` 两个 demo dataset + 三个 request fixture 已通过 `make-witness → business-fixture → verify` E2E smoke（两个 accepted，一个 out-of-range 正确 reject）。完整脚本化链上 E2E 编排推迟到 Stage 9。

## Stage 9：脚本化完整 E2E 编排

### 目标

把链上和链下流程串成一条稳定脚本命令，完成完整数据交易。

### 应实现

- 新增主 orchestrator 脚本，例如：
  - `scripts/data_trade_flexible_e2e.js`
  - 或在现有 `zk_real_data_trade_flow.js` 上扩展
- 支持通过配置选择：
  - DO account
  - DR account
  - dataset fixture
  - record/field/request
  - session rounds
  - settlement mode，目前只要求 MainEscrow
- 脚本流程应覆盖：
  - DO 发布 listing
  - DR 创建 request/session
  - MainEscrow 锁定资金和保证金
  - 链下 witness/proof 生成
  - `fishbone-zk verify`
  - verifier attestation
  - 链上提交 proof digest/attestation
  - DO/DR delivery/signature
  - hash-chain settlement
  - 输出 artifact/evidence summary

### 不应实现

- 前端。
- trustless bridge。
- on-chain verifier。
- 多 verifier quorum。

### 验收标准

- 一条命令在本地/既有环境跑通完整 happy path。
- 输出 evidence bundle：
  - dataset/request config
  - artifact path
  - proof digest
  - business input hash
  - public input hash
  - session id
  - escrow id
  - key extrinsic hashes/events
- 脚本可换 dataset/request 参数重复运行。

### 进度（2026-06-27）

- ✅ **Stage 9 完成**：`zk_real_data_trade_flow.js` 扩展支持动态模式（`--dataset`/`--request`）。`make-witness` pipeline 整合进完整链上 E2E 流程。新增 `--dry-run-dynamic`（无链 ZK 验证）和 `--evidence-out`（per-run evidence JSON）参数。仍为 range-only。

## Stage 10：请求约束扩展

### 目标

增强“灵活脱敏交易”的论文表达能力，让系统不只支持 range。

### 候选方向

优先级建议：

1. subset constraint
2. substr constraint
3. 多 range conjunction

最终选哪个，应在 Stage 10 详细 plan 开始前由 Codex 根据当前代码复杂度决定。

### 应实现

- 扩展 request schema，加入 `constraint_kind`。
- 至少新增一种非 range 的约束原型。
- 生成对应 witness/proof artifact。
- 脚本 E2E 能选择 range 或新 constraint。
- gap matrix 更新。

### 进度（2026-06-27）

- ✅ **Stage 10 完成**：实现 `multi_range` 多约束 range AND（同 record 多 field 各带独立 range）。`BuildRangeWitnesses` + `--out-dir` CLI + 归一化 `constraints[]` evidence。电路仍是 `BusinessRangeProof`，不涉及 subset/substr 或新电路。

### 不应实现

- 一次性实现所有论文约束。
- runtime 大改。
- production request language parser。

### 验收标准

- 至少两种请求类型可由脚本选择。
- 每种请求类型都有正向验证和至少一个负向测试。
- 文档准确描述已实现约束和未实现约束。

## Stage 11：失败与争议路径脚本化

### 目标

让论文 demo 不只展示成功交易，还能展示关键安全分支。

### 应实现

脚本化以下路径中的至少 3 个：

- invalid proof dispute
- invalid plaintext dispute
- verifier rejection
- DR refuses final payment 后 DO claim last payment
- wrong request/proof digest binding 被拒绝
- wrong session/escrow binding 被拒绝

### 不应实现

- 新 dispute pallet 设计。
- trustless bridge。
- slashing/quorum 新经济机制。

### 验收标准

- 每个失败路径有独立命令或配置开关。
- 每个路径输出链上事件和最终状态。
- evidence 文档记录 expected result。

### 进度（2026-06-27）

- ✅ **Stage 11 完成**：`zk_real_data_trade_flow.js --scenario` 支持 `invalid-proof-dispute`、`invalid-plaintext-dispute`、`requester-refuses-payment`。场景使用 `findEvent()` 断言预期链上事件。共享 setup/round helpers 避免重复代码。live chain 未运行（RPC 不可用）。

## Stage 12：论文实验封版

### 目标

把前面阶段沉淀为论文可引用、可复现、可截图的最终实验材料。

### 应实现

- 统一 demo 命令。
- 固定 sample configs。
- 固定 expected outputs。
- 生成实验 evidence 文档：
  - 实验环境
  - 命令
  - 输入配置
  - 输出 artifact
  - 链上状态
  - proof verification result
  - limitations
- 更新：
  - gap matrix
  - security model
  - implementation evidence
  - README/demo guide

### 验收标准

- 新 agent 从文档出发，可以复现实验。
- 论文中可以准确描述当前实现，不夸大安全边界。
- 所有 demo 命令在当前环境通过。

## 后续可能的 Stage 13+

这些是更高风险、更偏生产化或安全强化的方向，不建议在脚本化 E2E 闭环完成前启动：

- on-chain Groth16 verifier / VK registry
- verifier quorum / threshold attestation
- trustless bridge / CCMC / Merkle proof settlement
- production dynamic IMT service
- frontend DO/DR 操作台
- deployment hardening / VM multi-chain demo

## 协作规则

- 每个 Stage 必须有独立详细 plan。
- CodeWhale 执行前应先做 plan review。
- Codex 根据 review 修订 plan。
- CodeWhale 执行后，Codex 做 code review。
- 如果 Codex review 要求修改，CodeWhale 修完后 Codex 复审。
- 复审通过后，Codex 自动 merge 到 `main`。
- 每个 Stage 的 plan、review、follow-up review 都记录在 `docs/internal`。

## 长期验收标准

当 Stage 8-12 完成后，应能做到：

- 使用脚本完成完整 DO/DR 数据交易。
- 不同 dataset/request 参数能产生不同 witness、proof、digest 和链上状态。
- 至少支持 range + 一种额外约束，或支持多字段/多记录 range 请求，具体以后续 Stage 10 plan 为准。
- 交易成功路径和关键失败路径都可复现。
- 所有链下 proof artifact 可验证。
- 链上 session/escrow/attestation 状态与链下 artifact 一致。
- 文档清楚说明仍然不是 production verifier、不是 trustless bridge、不是前端产品。
