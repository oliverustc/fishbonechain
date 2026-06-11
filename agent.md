# FishboneChain Agent Guide

本文件是 Codex 接管 FishboneChain 后的首要上下文入口。开始任何改动前，先读本文件，再按任务需要继续阅读 `docs/`、`runtime/`、`pallets/`、`scripts/` 或 `deploy/`。

## 项目目标

FishboneChain 用 Substrate 实现主链 + 多子链众包系统：

- 主链负责子链登记、矿工管理、Epoch 摘要确认和任务资金托管
- 子链负责众包任务同步、数据提交、Epoch 结算和 Merkle 摘要生成
- 链下 bridge 监听子链事件，把摘要和账单提交到主链
- 实验重点是异构工作负载下的多链吞吐量扩展，以及 FMC 周期性锁仓带来的资金流动性提升

## 当前实现状态

- `pallet-ccmc`：子链管理、矿工加入/退出、Epoch digest 多数确认、slash 投票
- `pallet-fmc`：资金池、任务创建/激活/终止、账单多数确认和结算
- `pallet-crowdsource`：任务同步、数据提交、Epoch 生命周期、Merkle root 和账单事件
- `runtime/`：默认 AURA runtime；`fishbone-runtime/babe` feature 切换到 BABE runtime
- `node/`：默认 AURA service 和 `service_babe.rs`
- `scripts/`：worker、bridge、metrics、实验初始化、runtime 升级、结果分析和绘图
- `deploy/`：12 台 VM、1 主链 + 6 子链部署配置和 raw spec

截至接管时，吞吐量实验报告已落地；资金流动性实验仍在推进。

## 仓库地图

```text
node/                   节点 CLI、chain spec、service、RPC
runtime/                FRAME runtime，含 AURA/BABE 分支文件
pallets/ccmc/           子链管理 pallet
pallets/fmc/            资金管理 pallet
pallets/crowdsource/    子链众包 pallet
scripts/                Node/Python 实验与运维脚本
deploy/                 VM 部署框架、spec、keys、bin
docs/                   设计、实现记录、实验报告、实验计划
docs/plan/              临时计划区，已加入 .gitignore
references/             外部参考代码，默认不要修改
```

## 常用命令

```bash
# 快速编译检查
make check

# 全量测试
make test

# 单独测试核心 pallet
SKIP_WASM_BUILD=1 cargo test -p pallet-ccmc -p pallet-fmc -p pallet-crowdsource

# 构建 release 二进制
make build-release
make build-2s
make build-1s
make build-10mb
make build-babe

# 本地三链网络
bash scripts/start-network.sh
bash scripts/check-blocks.sh
```

`Makefile` 已带 `WASM_BUILD_RUSTFLAGS="-C link-arg=--allow-undefined"`，用于规避当前 Polkadot SDK/Rust 组合下的 WASM undefined symbol 问题。

## 文档阅读顺序

1. [docs/Readme.md](docs/Readme.md)：文档索引
2. [docs/developer-guide.md](docs/developer-guide.md)：代码结构和核心 pallet 说明
3. [docs/implementation-record.md](docs/implementation-record.md)：阶段实现记录
4. [docs/experiment-report.md](docs/experiment-report.md)：吞吐量实验结果
5. [docs/liquidity-experiment.md](docs/liquidity-experiment.md)：资金实验记录

## 文档生命周期

- `docs/plan/` 是临时计划区，用于任务拆解、审阅发现、执行 checklist 和阶段性草稿；该目录已加入 `.gitignore`，后续不作为版本化事实来源。
- 每个 plan 执行完成后，必须把结论、代码行为、实验结果、运行方式或踩坑记录沉淀到 `docs/` 下的正式文档，例如 `docs/developer-guide.md`、`docs/experiment-report.md`、`docs/implementation-plan.md` 或专题文档。
- 正式文档应写当前事实和可复现信息；历史计划、废弃方案和中间判断留在 `docs/plan/`，不要让它们继续承担项目说明职责。
- 如果计划执行改变了命令、部署方式、实验口径或已知限制，完成前必须同步更新对应正式文档和 [docs/Readme.md](docs/Readme.md) 的索引说明。
- 后续判断项目状态时，以 `agent.md`、`README.md`、`docs/Readme.md`、`docs/developer-guide.md`、`docs/experiment-report.md` 和当前代码为准；`docs/plan/` 仅供追溯执行过程。

## 工作约定

- 修改前先用 `rg` 定位相关代码和文档。
- 优先保持当前架构：FRAME pallet、Node.js `@polkadot/api` 脚本、Python `deploy/` 工具。
- `references/` 是参考材料，除非任务明确要求，保持只读。
- `deploy/keys/*.env`、`deploy/specs/*.json`、`deploy/bin/*` 可能包含环境产物；改动前确认是否真是任务需要。
- 不要随意清理 `target/`、`node_modules/`、`deploy/.venv/` 等本地构建目录。
- Rust 改动后至少运行相关 crate 的 `cargo test` 或 `cargo check`；脚本改动后尽量运行 dry-run、help 或静态检查。
- 文档里涉及实验结论时，优先引用正式 `docs/` 文档和当前代码，不要凭记忆改数值；如果旧 `docs/plan/` 内容与正式文档冲突，以正式文档为准。

## 后续重点

- 完成资金流动性实验 E：验证 `BillSettled`、`locked/free` 动态变化、生成最终图表
- 对 `scripts/bridge.js` 的多矿工、多链配置继续做可靠性检查
- 梳理部署产物和源码产物边界，必要时调整 `.gitignore`
- 将 CDT/BPiano 从设计文档推进到可运行子链模块或链下证明服务
