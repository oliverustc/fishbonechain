# FishboneChain

FishboneChain 是一个基于 Substrate/Polkadot SDK 的主链 + 多子链众包实验系统，用于验证多条专用子链并行承载异构众包任务时的吞吐量扩展和资金流动性优势。

当前仓库已由 Codex 接管维护。面向后续 agent 协作的入口说明见 [agent.md](agent.md)。

## 当前状态

- 主链 runtime 已集成 `pallet-ccmc`、`pallet-fmc`、`pallet-crowdsource`
- 默认 AURA 节点、1s/2s/10MB 变体、BABE 变体均已具备构建目标
- `deploy/` 已包含 12 台 VM、1 主链 + 6 子链的部署框架和 chain spec
- `scripts/` 已包含 worker、bridge、metrics、实验初始化、结果分析和绘图脚本
- 吞吐量实验 A/B/C/D 已形成报告，资金流动性实验 E 已有采集数据和图表，正式结论仍在整理中

## 方案概述

| 方案 | 说明 | 文档 |
|------|------|------|
| FishboneChain | 主链 + 多子链众包基础设施，主链管资金和摘要，子链管数据收集 | [docs/architecture/fishbonechain.md](docs/architecture/fishbonechain.md) |
| CDT | 可定制可验证数据交易，作为后续可部署在子链上的数据交易协议 | [docs/architecture/cdt.md](docs/architecture/cdt.md) |
| BPiano | 高效跨链状态证明，作为后续替换链下 bridge 信任假设的证明机制 | [docs/architecture/cross-chain-proof.md](docs/architecture/cross-chain-proof.md) |

## 技术栈

- 区块链框架：Substrate / Polkadot SDK，solo-chain 实验形态
- 共识：AURA + GRANDPA，另有 BABE + GRANDPA 变体
- Runtime：Rust FRAME pallet
- 链下脚本：Node.js + `@polkadot/api`
- 部署工具：Python + `uv` + systemd + SSH
- Rust：`rust-toolchain.toml` 固定 stable 1.96

## 快速开始

```bash
# 安装依赖
npm install

# 编译默认 release 节点，并复制到 deploy/bin/
make build-release

# 编译实验用变体
make build-2s
make build-1s
make build-10mb
make build-babe

# 本地启动三链开发网络
bash scripts/start-network.sh

# 查看本地出块状态
bash scripts/check-blocks.sh
```

本地开发 RPC 端口：主链 `9944`，子链 1 `9945`，子链 2 `9947`。

## 常用验证命令

```bash
# 快速 Rust 检查，跳过 WASM 构建
make check

# 全量测试
make test

# 单 pallet 测试
SKIP_WASM_BUILD=1 cargo test -p pallet-ccmc -p pallet-fmc -p pallet-crowdsource

# Phase 1 E2E 验证
node scripts/e2e-verify.js
```

## 项目结构

```text
fishbonechain/
├── agent.md        # Codex/agent 接管指南
├── node/           # fishbone-node，CLI、chain spec、AURA/BABE 服务
├── runtime/        # fishbone-runtime，AURA/BABE runtime 分文件组织
├── pallets/        # ccmc、fmc、crowdsource 和模板 pallet
├── scripts/        # 本地网络、bridge、worker、metrics、实验脚本
├── deploy/         # 12 VM 部署框架、spec、keys、二进制产物目录
├── docs/           # 分类后的架构、实现、开发、运维和实验文档
└── references/     # 外部参考代码和论文实现，默认不在维护任务中修改
```

## 文档入口

- [agent.md](agent.md)：后续 agent 工作前必须先读的项目上下文
- [docs/README.md](docs/README.md)：文档索引
- [docs/implementation/implementation-record.md](docs/implementation/implementation-record.md)：当前实现记录
- [docs/experiments/experiment-report.md](docs/experiments/experiment-report.md)：实验报告
- [docs/experiments/linear-scaling-mainchain-load.md](docs/experiments/linear-scaling-mainchain-load.md)：N=1..6 子链线性扩展与主链负载实验
- [docs/experiments/liquidity-experiment.md](docs/experiments/liquidity-experiment.md)：资金流动性实验记录

## 维护边界

`references/` 是外部参考资料和上游模板快照。除非明确任务要求，后续维护应只修改本项目代码、脚本和文档，不改动参考子模块内容。
