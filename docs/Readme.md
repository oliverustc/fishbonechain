# FishboneChain 文档索引

本目录保存 FishboneChain 的设计说明、实现记录、开发指南和实验文档。后续维护时请优先更新这里的索引，再更新具体文档。

## 接管入口

- [../agent.md](../agent.md)：Codex/agent 工作入口，包含当前状态、命令和维护边界
- [developer-guide.md](developer-guide.md)：开发者指南，介绍 runtime、pallet、部署框架和开发流程
- [implementation-record.md](implementation-record.md)：项目实现记录，覆盖当前 1 主链 + 6 子链实验系统

## 设计文档

- [fishbonechain.md](fishbonechain.md)：FishboneChain 主链 + 多子链众包架构
- [cdt.md](cdt.md)：可定制可验证数据交易协议
- [cross_chain_proof.md](cross_chain_proof.md)：BPiano / 高效跨链状态证明
- [implementation-plan.md](implementation-plan.md)：早期总体实现规划，已被当前实现记录覆盖的部分仅作历史参考

## 实验文档

- [experiment-report.md](experiment-report.md)：实验报告，包含吞吐量 A-D 和资金流动性 E 的正式整理
- [isolation-experiment.md](isolation-experiment.md)：跨场景隔离实验数据口径、图表和结论
- [linear-scaling-mainchain-load.md](linear-scaling-mainchain-load.md)：N=1..6 子链线性扩展与主链负载实验规划、采集口径和绘图说明
- [capacity-experiment.md](capacity-experiment.md)：MaxSubmissions=10000 高压容量实验、最终 TPS 数据和复现方法
- [liquidity-experiment.md](liquidity-experiment.md)：资金流动性实验数据、图表和口径说明
- `plan/`：临时计划区，已加入 `.gitignore`，后续不作为正式事实来源
- [figures/](figures/)：实验报告图表

## 环境与参考

- [rust-setup.md](rust-setup.md)：Rust/Substrate 环境说明
- [../env-setup/README.md](../env-setup/README.md)：Nix/flake 环境配置
- [../references/](../references/)：外部参考代码和文档，默认不修改

## 更新约定

- 实验数据和图表结论优先落在 `experiment-report.md`
- 计划、执行状态、审阅发现可以先落在 `docs/plan/`，但任务完成后必须沉淀到 `docs/` 下的正式文档
- 面向后续 agent 的操作约定优先落在根目录 `agent.md`
- 旧规划和实际实现不一致时，以 `implementation-record.md`、`developer-guide.md`、`experiment-report.md` 与当前代码为准
