# FishboneChain 文档索引

本目录保存 FishboneChain 的设计说明、实现记录、开发指南、运维说明和实验文档。后续维护时请优先更新这里的索引，再更新具体文档。

## 接管入口

- [../agent.md](../agent.md)：Codex/agent 工作入口，包含当前状态、命令和维护边界
- [internal/agent-collaboration.md](internal/agent-collaboration.md)：Codex、CodeWhale 和其他 agent 的协作规范
- [development/developer-guide.md](development/developer-guide.md)：开发者指南，介绍 runtime、pallet、部署框架和开发流程
- [implementation/implementation-record.md](implementation/implementation-record.md)：项目实现记录，覆盖当前 1 主链 + 6 子链实验系统

## 架构与设计

- [architecture/platform-architecture.md](architecture/platform-architecture.md)：平台层/场景层边界、chain profile 与结算模式
- [architecture/platform-business-model.md](architecture/platform-business-model.md)：平台通用业务对象模型（Stage 15 设计基线）
- [architecture/fishbonechain.md](architecture/fishbonechain.md)：FishboneChain 主链 + 多子链众包架构
- [architecture/cdt.md](architecture/cdt.md)：可定制可验证数据交易协议
- [architecture/cross-chain-proof.md](architecture/cross-chain-proof.md)：BPiano / 高效跨链状态证明
- [architecture/data-trade-security-model.md](architecture/data-trade-security-model.md)：数据交易安全模型，覆盖参与方、信任假设、攻击场景与论文对齐矩阵
- [architecture/topology-diagram.md](architecture/topology-diagram.md)：部署矩阵图和图表入口

## 实现与开发

- [implementation/implementation-record.md](implementation/implementation-record.md)：当前实现记录
- [implementation/data-trade-implementation.md](implementation/data-trade-implementation.md)：数据交易场景实现记录
- [implementation/data-trade-cli-api-boundary.md](implementation/data-trade-cli-api-boundary.md)：数据交易 CLI/API 边界定义（Stage 16，面向后续后端集成）
- [implementation/chain-event-indexer-state-sync.md](implementation/chain-event-indexer-state-sync.md)：链上事件索引与状态同步（Stage 17，文件级事件索引、cursor、状态推导和 evidence 关联）
- [implementation/data-trade-evidence.md](implementation/data-trade-evidence.md)：数据交易实现证据记录，本地验证、VM 回归摘要和复现命令
- [implementation/data-trade-paper-gap-matrix.md](implementation/data-trade-paper-gap-matrix.md)：数据交易论文要求与当前实现的差距矩阵
- [implementation/data-trade-flow.md](implementation/data-trade-flow.md)：数据交易 E2E 运行手册（happy path + negative scenarios）
- [implementation/data-trade-stage14-evidence-index.md](implementation/data-trade-stage14-evidence-index.md)：Stage 14 数据交易 evidence 索引
- [implementation/implementation-plan.md](implementation/implementation-plan.md)：早期总体实现规划，已被当前实现记录覆盖的部分仅作历史参考
- [development/developer-guide.md](development/developer-guide.md)：开发者指南
- [development/rust-setup.md](development/rust-setup.md)：Rust/Substrate 环境说明

## 运维

- [operations/fishbone-monitor.md](operations/fishbone-monitor.md)：Fishbone Monitor 部署、API 和运维说明

## 实验

- [experiments/experiment-report.md](experiments/experiment-report.md)：实验报告，包含吞吐量 A-D 和资金流动性 E 的正式整理
- [experiments/isolation-experiment.md](experiments/isolation-experiment.md)：跨场景隔离实验数据口径、图表和结论
- [experiments/linear-scaling-mainchain-load.md](experiments/linear-scaling-mainchain-load.md)：N=1..6 子链线性扩展与主链负载实验规划、采集口径和绘图说明
- [experiments/capacity-experiment.md](experiments/capacity-experiment.md)：MaxSubmissions=10000 高压容量实验、最终 TPS 数据和复现方法
- [experiments/liquidity-experiment.md](experiments/liquidity-experiment.md)：资金流动性实验数据、图表和口径说明
- [experiments/data-trade-validation.md](experiments/data-trade-validation.md)：数据交易实验固化与一键复现（Stage 14）

## 内部资料

- [internal/agent-collaboration.md](internal/agent-collaboration.md)：多 agent 分工、计划、执行记录、review 和提交追踪规范
- [internal/agent-plans/](internal/agent-plans/)：仍需保留的 agent 实施计划，作为过程记录，不作为当前事实来源
- [internal/agent-reviews/](internal/agent-reviews/)：Codex/其他 agent 的 plan review 与 code review 记录
- `plan/`：旧临时计划区；内容已合并到正式文档并清理，后续不要作为事实来源
- [../env-setup/README.md](../env-setup/README.md)：Nix/flake 环境配置
- [../references/](../references/)：外部参考代码和文档，默认不修改

## 更新约定

- 实验数据和图表结论优先落在 `experiments/experiment-report.md`
- 计划、执行状态、审阅发现若需要保留，落在 `docs/internal/agent-plans/`；任务完成后必须沉淀到正式文档
- 面向后续 agent 的操作约定优先落在根目录 `agent.md`
- 旧规划和实际实现不一致时，以 `implementation/implementation-record.md`、`development/developer-guide.md`、`experiments/experiment-report.md` 与当前代码为准
