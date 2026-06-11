# FishboneChain 实现记录

**最后更新**：2026-06-11  
**当前状态**：1 条主链 + 6 条子链实验系统已实现并完成吞吐量实验；资金流动性实验已有采集脚本、数据和图表，仍需继续完善结论口径。

## 当前架构

```text
主链 fishbone_main，12 validators（f1-f12）
├── pallet-ccmc   子链注册 / 矿工管理 / Epoch 摘要确认
└── pallet-fmc    任务资金托管 / 账单投票 / 奖励结算

子链
├── child1  城市快递配送确认  AURA-3  6s     f1 f2 f3
├── child2  实时交通感知      AURA-3  2s     f4 f5 f6
├── child3  医疗影像标注      AURA-3  6s     f7 f8 f9   10MB 区块
├── child4  金融凭证核验      AURA-7  6s     f1-f7
├── child5  IoT 传感器网络    AURA-3  1s     f10 f11 f12
└── child6  去中心化数据市场  BABE-5  6s     f1-f5

链下中继
└── scripts/bridge.js  监听子链 EpochFinalized，提交主链 digest/bill
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
- `pallets/crowdsource/`：子链任务、提交、Epoch 状态机、Merkle root
- `runtime/src/runtime_aura.rs`：AURA runtime
- `runtime/src/runtime_babe.rs`：BABE runtime
- `node/src/service.rs`：AURA 节点服务
- `node/src/service_babe.rs`：BABE 节点服务
- `deploy/`：12 VM 部署和 systemd 管理
- `scripts/`：setup、worker、bridge、metrics、plot、analyze、runtime upgrade

## 构建目标

```bash
make build-release    # 默认 6s AURA
make build-2s         # child2
make build-1s         # child5
make build-10mb       # child3
make build-babe       # child6
```

## 重要实现决策

- 主链和子链当前使用同一套业务 runtime，通过 chain spec、二进制 feature 和部署配置区分角色。
- `pallet-crowdsource` 的默认 Epoch 为 `100 Collecting + 20 Syncing` blocks。
- `MaxSubmissionsPerEpoch` 当前固定为 1000。
- 跨链摘要和账单通过链下 `bridge.js` 中继，不使用原生 XCM。
- BABE 需要真实 `pallet_babe` 生成 `NextEpochData` digest；stub BabeApi 无法正常出块。
- `Makefile` 中保留 `WASM_BUILD_RUSTFLAGS="-C link-arg=--allow-undefined"` 以兼容当前 WASM 构建。

## 实验产物

- 吞吐量报告：[experiment-report.md](experiment-report.md)
- 资金流动性记录：[liquidity-experiment.md](liquidity-experiment.md)
- 图表：`docs/figures/`
- 数据：`docs/figures/data/`

## 已知限制

- 当前跨链安全依赖链下 bridge；BPiano 仍停留在设计/参考实现阶段。
- CDT/BPiano 尚未实现为可运行 pallet 或服务。
- `MaxSubmissionsPerEpoch` 是编译期常量，无法链上治理动态调整。
- 实验中 worker 进程曾因单机内存限制 OOM；这属于实验部署限制，不是链协议必然限制。
- FMC bill 结算和资金流动性已开始验证，但需要继续把脚本数据、图表和报告口径统一。
