# FishboneChain

基于 Substrate 实现的主链 + 多子链众包平台，集成可验证数据交易与高效跨链状态证明。

> 文档持续更新中，待实现完成后补充完整。

## 方案概述

本项目实现三个相互关联的方案：

| 方案 | 说明 | 文档 |
|------|------|------|
| **FishboneChain** | 主链 + 多子链众包基础设施（资金管理、任务分发、子链同步） | [docs/fishbonechain.md](docs/fishbonechain.md) |
| **CDT** | 可定制可验证数据交易（zk-SNARK + 链下多轮交付协议） | [docs/cdt.md](docs/cdt.md) |
| **BPiano** | 高效跨链状态证明（分布式 Plonk + 证明压缩/聚合） | [docs/cross_chain_proof.md](docs/cross_chain_proof.md) |

实现规划见 [docs/implementation-plan.md](docs/implementation-plan.md)。

## 技术栈

- **区块链框架**：Substrate（AURA + GRANDPA，solo chain 模式，后续迁移至 Relay+Parachain）
- **ZK 证明**：Rust 原生实现（ark-bn254、ark-groth16）
- **链下服务**：Go（ZK 证明生成、IMT 构建、BPiano calldata 生成）
- **Rust 版本**：stable 1.96+

## 快速开始

```bash
# 克隆（含 submodule）
git clone --recurse-submodules <repo-url>
cd fishbonechain

# 编译
make build-release

# 启动本地三链网络（主链 + 子链1 + 子链2）
bash scripts/start-network.sh

# 查看出块状态
bash scripts/check-blocks.sh
```

本地节点 RPC 端口：主链 `9944`，子链1 `9945`，子链2 `9947`。

通过 [Polkadot.js Apps](https://polkadot.js.org/apps) 连接（自定义端点 `ws://127.0.0.1:9944`）。

## 项目结构

```
fishbonechain/
├── node/           # fishbone-node（节点启动器、chain spec）
├── runtime/        # fishbone-runtime（FRAME runtime）
├── pallets/        # 业务 pallet（CCMC、FMC 等，开发中）
├── scripts/        # 本地网络启动/检查脚本
├── docs/           # 设计文档与实现规划
└── references/     # 参考资料（git submodule）
    ├── polkadot-sdk-solochain-template/   # 本项目基础
    ├── polkadot-sdk-parachain-template/   # parachain 迁移参考
    ├── polkadot-sdk-minimal-template/     # 最小化 runtime 参考
    ├── frontier-parachain-template/       # EVM 兼容参考（CDT 用）
    ├── polkadot-cookbook/                 # 官方开发文档
    ├── data_trade_code/                   # CDT 参考实现（Go + Solidity）
    └── efficient_cross_chain_proof_code/  # BPiano 参考实现（Go）
```
