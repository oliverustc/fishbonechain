# FishboneChain 实现记录

**最后更新**：2026-06-14  
**当前状态**：平台层与场景层已开始解耦；数据众包场景已实现并完成吞吐量实验，数据交易场景已具备 `data-registry` 与 `trade-session` 骨架。

## 当前架构

```text
主链 fishbone_main，12 validators（f1-f12）
├── pallet-ccmc   子链注册 / 矿工管理 / Epoch 摘要确认
├── pallet-fmc    可选任务资金托管 / 账单投票 / 奖励结算
└── pallet-chain-profile 链身份 / 场景类型 / 结算模式

众包子链
├── child1  城市快递配送确认  AURA-3  6s     f1 f2 f3
├── child2  实时交通感知      AURA-3  2s     f4 f5 f6
├── child3  医疗影像标注      AURA-3  6s     f7 f8 f9   10MB 区块
├── child4  金融凭证核验      AURA-7  6s     f1-f7
└── child5  IoT 传感器网络    AURA-3  1s     f10 f11 f12

数据交易子链
└── child6  数据交易市场      AURA-5  6s     f1-f5   MainEscrow

链下中继
└── scripts/bridges/  按场景监听事件，提交摘要、账单或交易观察事件
```

当前实验形态是多个独立 solo chain，通过链下 bridge 串联，不是 Relay+Parachain/XCM 架构。

## 阶段记录

| 阶段 | 日期 | 状态 | 产出 |
|------|------|------|------|
| Phase 0 | 2026-05-31 | 完成 | 基于 Polkadot SDK solo-chain 模板建立可运行节点 |
| Phase 1 | 2026-05-31 | 完成 | `pallet-ccmc`、`pallet-fmc`、本地 E2E 验证 |
| Phase 2 | 2026-06-01 | 完成 | `pallet-crowdsource`、Epoch 生命周期、Merkle root、账单事件 |
| Phase 3 | 2026-06-01 | 完成 | Python 多节点部署框架、12 VM 配置 |
| Phase 4 | 2026-06-03 | 完成 | 1s/2s/10MB 二进制变体、6 子链部署、实验脚本 |
| Phase 5 | 2026-06-04 | 完成 | BABE runtime/service 修复，child6 BABE 出块和 GRANDPA finality |
| 实验 A-D | 2026-06-04 至 2026-06-06 | 完成 | 单链瓶颈、专用链改善、6 链并发、线性扩展图表 |
| 实验 E | 2026-06-11 | 部分完成 | `metrics_fund.js`、资金数据 CSV、fig7a/fig7b 图表 |

## 已实现代码

- `pallets/ccmc/`：子链管理、矿工管理、Epoch digest 多数确认
- `pallets/fmc/`：资金池、任务生命周期、账单投票和结算
- `pallets/chain-profile/`：链身份、场景类型和结算模式
- `pallets/crowdsource/`：子链任务、提交、Epoch 状态机、Merkle root
- `pallets/data-registry/`：数据交易 listing、IMT root 和描述
- `pallets/trade-session/`：数据交易 MainEscrow 会话、锁资、押金和哈希链 claim
- `runtime/src/runtime_main.rs`：平台主链 runtime
- `runtime/src/runtime_crowdsource.rs`：数据众包子链 runtime
- `runtime/src/runtime_data_trade.rs`：数据交易子链 runtime
- `runtime/src/runtime_aura.rs`：旧 AURA runtime 兼容文件
- `runtime/src/runtime_babe.rs`：BABE runtime
- `node/src/service.rs`：AURA 节点服务
- `node/src/service_babe.rs`：BABE 节点服务
- `deploy/`：12 VM 部署和 systemd 管理
- `scripts/`：setup、worker、bridge、metrics、plot、analyze、runtime upgrade

## 构建目标

```bash
make build-release    # 默认 6s AURA
make build-main       # 平台主链
make build-crowdsource-child
make build-data-trade-child
make build-2s         # child2
make build-1s         # child5
make build-10mb       # child3
make build-babe       # child6
```

## 重要实现决策

- 主链和子链通过 runtime profile、chain profile、chain spec 和部署配置区分角色。
- `pallet-crowdsource` 不是平台强制能力，只属于数据众包场景。
- 数据交易第一版使用 `MainEscrow`，不调用 FMC；FMC/FmcAssisted/Hybrid 留给需要周期性预算的服务。
- `pallet-crowdsource` 的默认 Epoch 为 `100 Collecting + 20 Syncing` blocks。
- 常规 runtime 的 `MaxSubmissionsPerEpoch` 为 1000；容量实验曾使用扩大到 10000、`CollectingSlotBlocks=600` 的实验构建，不应和默认部署口径混写。
- 跨链摘要和账单通过链下 `bridge.js` 中继，不使用原生 XCM。
- BABE 需要真实 `pallet_babe` 生成 `NextEpochData` digest；stub BabeApi 无法正常出块。
- `Makefile` 中保留 `WASM_BUILD_RUSTFLAGS="-C link-arg=--allow-undefined"` 以兼容当前 WASM 构建。

## 实验产物

- 吞吐量报告：[../experiments/experiment-report.md](../experiments/experiment-report.md)
- 资金流动性记录：[../experiments/liquidity-experiment.md](../experiments/liquidity-experiment.md)
- 图表：`docs/experiments/figures/`
- 数据：`docs/experiments/figures/data/`

## 已知限制

- 当前跨链安全依赖链下 bridge；BPiano 仍停留在设计/参考实现阶段。
- CDT 已有 `data-registry` 与 `trade-session` 骨架；完整 ZK verifier 和争议流程尚未接入。
- `MaxSubmissionsPerEpoch` 是编译期常量，无法链上治理动态调整。
- 实验中 worker 进程曾因单机内存限制 OOM；这属于实验部署限制，不是链协议必然限制。
- FMC bill 结算和资金流动性已开始验证，但需要继续把脚本数据、图表和报告口径统一。
