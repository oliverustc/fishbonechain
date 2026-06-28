# 多链数据流通平台长期路线图（初稿）

日期：2026-06-28
维护负责人：Codex

## 定位

本文是 Stage 14 及后续阶段的上层路线图，不是某一个 Stage 的详细执行 plan。后续每个 Stage 开始前，仍必须由 Codex 基于当时的代码、文档、链环境和阶段目标重新编写详细 plan。

本文的作用是防止后续阶段只围绕单一数据交易 demo 继续堆功能，导致未来无法扩展到完整的数据流通平台。

## 长期总目标

最终目标是实现一个基于多链平台的数据流通平台，支持：

- 数据收集。
- 数据交易。
- 数据跨域流通。
- 数据可验证处理。
- 数据可验证训练。
- 多业务子链隔离。
- 主链统一结算、治理或全局记录。
- 中心化 Web 前端和后端用于用户管理、业务状态记录、任务编排、证据索引和可视化。

平台应遵守一个基本边界：

- 链上核心操作仍由用户通过自己的链上身份签名并调用区块链接口完成。
- Web 后端不替用户保管私钥，不把数据库状态伪装成链上可信状态。
- Web 后端负责用户、任务、证据、链上事件、链下任务和业务状态的记录与编排。
- 链上事件和链下 evidence 应能反向校验 Web 后端记录。

## 当前基线

截至 Stage 13 合并到 `main` 后，数据交易模块已经具备：

- 动态 dataset/request 输入。
- `range` 与 `multi_range` 约束。
- 链下 `fishbone-zk` proof 生成与验证。
- proof digest、business input hash、request hash 绑定。
- 子链 listing/session/proof/attestation 流程。
- 主链 escrow 结算。
- live-chain happy path 已通过。
- 三个 live-chain 异常路径已通过：
  - `invalid-proof-dispute`
  - `invalid-plaintext-dispute`
  - `requester-refuses-payment`
- child6 环境恢复经验和子链部署 runbook 已记录。

这说明系统已经越过了“协议流程是否能跑通”的关键线。后续重点应从单点功能推进转向：

- 可复现。
- 可审计。
- 可抽象。
- 可查询。
- 可接口化。
- 可平台化。

## 架构原则

### 1. 数据交易只是第一个业务模块

后续不能把平台模型写死成 data trade。数据交易应作为第一个业务模块，用来反推平台通用抽象。

未来至少要能容纳：

```text
data_collection
data_trade
cross_domain_flow
verifiable_training
```

### 2. 中心化后端是业务编排层，不是可信替代层

Web 后端可以记录：

- 用户资料。
- 链上账户绑定关系。
- 数据资源元信息。
- 业务任务状态。
- 链上事件索引。
- 链下 job 状态。
- evidence metadata。
- 前端展示所需缓存。

Web 后端不应承担：

- 用户私钥托管。
- 代表用户无授权签名。
- 将数据库状态作为协议最终状态。
- 绕过链上状态机完成结算或争议。

### 3. 所有业务都必须有统一 evidence 思维

无论是数据收集、数据交易、跨域流通还是可验证训练，都应记录：

- 输入。
- 操作。
- 链上交易。
- 链上事件。
- 链下 artifact。
- digest / proof / attestation。
- 结果。
- 错误。
- 审计日志。

这将同时服务于论文、调试、前端展示和责任追踪。

### 4. 平台对象应优先抽象

后续 Web 后端不要直接从 `trade_session` 开始建模。应先抽象通用平台对象：

```text
User
ChainAccount
Dataset
DataAsset
BusinessTask
WorkflowRun
Evidence
ChainEvent
OffchainJob
```

业务模块再扩展自己的字段和状态机。

### 5. 链上事件索引必须成为基础设施

后端状态不能只依赖脚本返回值。平台需要能从链上事件恢复或校验业务状态。

后续所有业务模块都应复用同一类能力：

- 连接多条链。
- 拉取事件。
- 解析事件。
- 关联业务任务。
- 记录事件 cursor。
- 支持重放扫描。
- 支持状态修复。

## 推荐阶段路线

### Stage 14：数据交易实验固化与一键复现

目标：把 Stage 13 的手工补跑验证固化成可复现实验资产，并为未来业务模块建立第一套验证模板。

主要产物：

- 一键验证脚本，例如 `scripts/run_data_trade_validation.sh`。
- 自动输出目录结构：

```text
summary.json
summary.md
dry-run/
live-happy/
live-invalid-proof/
live-invalid-plaintext/
live-requester-refuses-payment/
```

- evidence index。
- 面向论文的实验报告。
- 初版 evidence summary schema。

约束：

- 不新增协议功能。
- 不做 Web 后端。
- 不把 `/tmp` 或 `target` 生成物直接提交为长期代码资产。
- summary/evidence 结构要考虑未来复用于数据收集、跨域流通和可验证训练。

难度：中等。

### Stage 15：平台通用业务模型设计

目标：在进入 Web 后端实现前，先设计平台通用业务对象，避免数据交易模型污染所有未来模块。

主要产物：

- 平台业务模型文档。
- 初版 JSON schema 或 TypeScript type 草案。
- 数据交易到通用模型的映射表。
- 未来数据收集、跨域流通、可验证训练的占位映射。

重点对象：

```text
User
ChainAccount
Dataset
DataAsset
BusinessTask
WorkflowRun
Evidence
ChainEvent
OffchainJob
```

难度：中高。

### Stage 16：数据交易 CLI / API 边界标准化

目标：把当前偏 demo 的数据交易脚本整理成清楚的操作边界，为后端调用和前端验收做准备。

候选命令边界：

```text
publish-listing
create-request
create-escrow
open-session
generate-proof
submit-delivery
settle
dispute
inspect
run-flow
```

重点不是形式，而是明确：

- 哪些操作属于 DO。
- 哪些操作属于 DR。
- 哪些操作属于 verifier。
- 哪些操作是链下计算。
- 哪些操作是链上交易。
- 哪些操作只是查询。

难度：中高。

### Stage 17：链上事件索引与状态同步

目标：建立未来 Web 后端可复用的链上事件索引基础能力。

主要能力：

- 多链 RPC 配置。
- 事件扫描。
- cursor 保存。
- event normalization。
- listing/session/escrow 状态解析。
- evidence 与链上事件关联。
- 重放扫描。

此阶段可先以脚本或轻量服务形式实现，不急于完整 Web 后端。

难度：中等到中高。

### Stage 18：Web 后端最小骨架

目标：建立中心化平台后端的最小可用框架。

核心功能：

- 用户注册/登录。
- 链上账户绑定。
- 业务任务表。
- 链上事件表。
- evidence 表。
- offchain job 表。
- 基础 API skeleton。

后端职责：

- 记录业务任务。
- 保存 evidence metadata。
- 保存链上事件索引。
- 提供前端查询 API。
- 编排链下 job。

不做：

- 私钥托管。
- 替用户无授权签名。
- 复杂前端。
- 全业务模块一次性接入。

难度：高。

### Stage 19：链下任务执行器

目标：将 proof 生成、数据处理、未来训练任务抽象为统一 job。

统一 job 类型：

```text
proof_generation
data_preprocessing
anonymization
verification
training
```

每个 job 至少记录：

```text
input
status
worker
artifact_path
digest
error
created_at
completed_at
evidence_id
```

数据交易 proof generation 是第一个落地 job 类型。

难度：中高。

### Stage 20：数据交易 Web API

目标：把数据交易模块接入 Web 后端。

候选 API：

```text
POST /api/data-trade/listings
POST /api/data-trade/requests
POST /api/data-trade/sessions
POST /api/data-trade/proof-jobs
GET  /api/data-trade/sessions/:id
GET  /api/data-trade/evidence/:id
```

原则：

- 链上交易仍由前端或用户钱包使用用户链上身份签名。
- 后端可以返回待签名 call 的结构化信息。
- 后端接收交易 hash 后，通过 indexer 监听事件并更新状态。
- proof job 和 evidence 由后端记录和编排。

难度：高。

### Stage 21：数据交易 Web 前端操作台

目标：做第一个真正可操作的 Web 流程。

支持：

- 用户登录。
- 绑定链上账户。
- DO 发布数据资源。
- DR 创建数据请求。
- 用户签名链上交易。
- 展示 listing/session/escrow 状态。
- 展示 proof/evidence。
- 展示 settlement/dispute 结果。

验收标准应复用 Stage 14/16 的脚本结果作为 oracle。

难度：高。

### Stage 22：数据收集模块接入

目标：验证平台通用模型能支持第二类业务，而不是只支持数据交易。

可能对象：

```text
collection_task
worker_submission
quality_check
reward_settlement
```

需要复用：

- BusinessTask。
- WorkflowRun。
- ChainEvent。
- Evidence。
- OffchainJob。

难度：中高。

### Stage 23：跨域数据流通模块

目标：支持不同域、不同子链、不同参与方之间的数据流通记录和证明。

初期重点不是一步到位实现最强 trustless bridge，而是先建立可运行业务流程：

- 跨域请求。
- 授权。
- 数据摘要。
- 来源证明。
- 目标域接收。
- 状态记录。
- evidence 关联。

难度：高。

### Stage 24：可验证训练模块

目标：把数据流通扩展到“数据被用于训练”的场景。

候选流程：

```text
数据授权
训练任务创建
训练输入承诺
训练 job 执行
模型摘要/指标提交
证明或 attestation
结果结算
```

初期可先使用 attestation 模式，后续再研究更强的 verifiable training 证明。

难度：高到很高。

## 更远期研究增强

以下不建议插入近期 Stage，除非论文或实验明确需要：

- on-chain Groth16 verifier。
- verifier quorum / threshold attestation。
- trustless bridge / cross-chain proof。
- subset/substr/aggregation 等更多约束类型。
- production dynamic IMT。
- timeout / challenge-period dispute 机制。
- 更完整的数据可验证训练证明系统。

这些属于研究级增强，难度明显高于平台工程化阶段。

## 推荐近期顺序

近期建议：

```text
Stage 14：数据交易实验固化与一键复现
Stage 15：平台通用业务模型设计
Stage 16：数据交易 CLI / API 边界标准化
Stage 17：链上事件索引与状态同步
```

这样可以先把现有跑通能力变成可复现资产，再建立平台通用抽象，然后才进入后端和前端。

## 每个后续 Stage 的计划要求

后续每个 Stage 的详细 plan 都必须包含：

- 当前代码状态回顾。
- 本 Stage 与长期路线图的关系。
- 明确目标与非目标。
- 文件级修改范围。
- 命令和验收标准。
- evidence 或测试产物。
- 对平台扩展性的影响。
- stop condition。
- CodeWhale 可执行的清晰步骤。

如果某个 Stage 发现当前路线图与实际代码冲突，应先更新本路线图或写补充说明，再继续执行。
