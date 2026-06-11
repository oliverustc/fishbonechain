# FishboneChain 开发者指南

> 适用对象：接手本项目的 Rust/Substrate、脚本和实验维护者  
> 当前范围：1 条主链 + 6 条子链的 solo-chain 实验系统，含 CCMC/FMC/Crowdsource pallet、AURA/BABE 节点变体和链下实验脚本。

## 当前架构

FishboneChain 当前实现的是主链 + 多子链的实验形态，不是 Relay+Parachain/XCM 形态。

```text
主链 fishbone_main
  ├── pallet-ccmc：子链注册、矿工管理、Epoch 摘要确认
  └── pallet-fmc：任务资金托管、账单投票、奖励结算

子链 child1-child6
  └── pallet-crowdsource：任务同步、数据提交、Epoch 结算、Merkle root 和账单事件

链下 bridge
  └── scripts/bridge.js：监听子链 EpochFinalized，向主链提交 ccmc digest 和 fmc bill
```

主链和子链通过独立 Substrate 节点运行。跨链动作由 Node.js bridge 完成，不依赖原生 XCM。

## 仓库结构

```text
node/                   fishbone-node CLI、chain spec、AURA/BABE service
runtime/                fishbone-runtime，AURA/BABE runtime 分文件组织
pallets/ccmc/           子链管理 pallet
pallets/fmc/            资金管理 pallet
pallets/crowdsource/    子链众包 pallet
scripts/                worker、bridge、metrics、setup、plot、runtime upgrade
deploy/                 12 VM 部署配置、spec、keys、Python 管理框架
docs/                   正式项目文档和实验报告
docs/plan/              临时计划区，已加入 .gitignore，不作为事实来源
references/             外部参考代码，默认不要修改
```

## Runtime 组成

`runtime/src/runtime_aura.rs` 和 `runtime/src/runtime_babe.rs` 分别定义 AURA 与 BABE runtime。两者保持相同的业务 pallet index：

| Index | Pallet |
|-------|--------|
| 0 | System |
| 1 | Timestamp |
| 2 | Aura |
| 3 | Grandpa |
| 4 | Balances |
| 5 | TransactionPayment |
| 6 | Sudo |
| 7 | Template |
| 8 | Ccmc |
| 9 | Fmc |
| 10 | Crowdsource |
| 11 | Babe（仅 BABE runtime）|
| 12 | Authorship |

关键配置在 `runtime/src/configs/mod.rs`：

- 默认区块大小 5 MB，`fishbone-runtime/block-10mb` 切换到 10 MB
- `pallet-crowdsource` 默认 `CollectingSlot=100`、`SyncingSlot=20`
- `MaxSubmissionsPerEpoch=1000`
- BABE 通过 `fishbone-runtime/babe` feature 启用

## 链和二进制

| 链 | RPC | 共识/参数 | Binary |
|----|-----|-----------|--------|
| main | `9944` | AURA + GRANDPA，12 validators | `fishbone-node` |
| child1 | `9945` | AURA-3，6s | `fishbone-node` |
| child2 | `9946` | AURA-3，2s | `fishbone-node-2s` |
| child3 | `9947` | AURA-3，6s，10 MB | `fishbone-node-10mb` |
| child4 | `9948` | AURA-7，6s | `fishbone-node` |
| child5 | `9949` | AURA-3，1s | `fishbone-node-1s` |
| child6 | `9950` | BABE-5，6s | `fishbone-node-babe` |

部署拓扑以 `deploy/config.toml` 为准。子链验证人必须是主链验证人的子集。

## 常用命令

```bash
# 快速 Rust 检查，跳过 WASM 构建
make check

# 全量测试
make test

# 业务 pallet 测试
SKIP_WASM_BUILD=1 cargo test -p pallet-ccmc -p pallet-fmc -p pallet-crowdsource

# 构建二进制
make build-release
make build-2s
make build-1s
make build-10mb
make build-babe

# 本地三链开发网络
bash scripts/start-network.sh
bash scripts/check-blocks.sh
```

`Makefile` 中的 `WASM_BUILD_RUSTFLAGS="-C link-arg=--allow-undefined"` 是当前 Polkadot SDK/Rust 组合的必要兼容项。

## 核心 Pallet

### `pallet-ccmc`

职责：

- 注册和终止子链
- 管理矿工加入、退出和押金
- 对 Epoch digest 进行 2/3 多数投票确认
- 为 FMC 提供 `is_miner`、`miner_count` 等查询接口

主要 extrinsic：

- `register_child_chain`
- `join_child_chain`
- `leave_child_chain`
- `submit_epoch_digest`
- `vote_slash_miner`
- `terminate_child_chain`

### `pallet-fmc`

职责：

- 维护请求者 `free` / `locked` 资金池
- 创建、激活、终止任务
- 接收矿工账单投票，达到 2/3 多数后结算
- 结算时将未使用预算归还 `free`，并在余额足够时自动续期

主要 extrinsic：

- `deposit`
- `withdraw`
- `create_task`
- `activate_task`
- `terminate_task`
- `submit_bill`

### `pallet-crowdsource`

职责：

- 子链同步主链任务参数
- 工作者提交数据
- Epoch 从 Collecting 进入 Syncing
- 生成 Merkle root 和 `EpochFinalized` 事件
- 为 bridge 提供账单数据

主要 extrinsic：

- `sync_task`
- `submit_data`
- `finalize_epoch`

## 实验脚本

| 脚本 | 用途 |
|------|------|
| `scripts/setup_experiment.js` | 初始化 CCMC/FMC/子链任务和 worker 账户 |
| `scripts/worker.js` | 模拟 a-f 场景工作者提交 |
| `scripts/metrics.js` | 采集子链吞吐量和 Epoch 状态 |
| `scripts/metrics_fund.js` | 采集主链资金流动性指标 |
| `scripts/bridge.js` | 子链事件到主链 digest/bill 的链下中继 |
| `scripts/analyze_results.py` | 汇总吞吐量实验结果 |
| `scripts/plot_results.py` | 生成论文图表 |

## 文档维护规则

`docs/plan/` 只用于临时计划和执行 checklist。任务完成后，必须把可复现结论写入正式文档：

- 代码结构和运行方式：更新本文档
- 实现阶段和当前状态：更新 `docs/implementation-record.md`
- 实验结果和图表：更新 `docs/experiment-report.md`
- 旧规划和实际实现差异：更新 `docs/implementation-plan.md`

如果正式文档和 `docs/plan/` 内容冲突，以正式文档和当前代码为准。
