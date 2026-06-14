# 数据交易场景实现记录

数据交易场景作为 CDT 的第一阶段工程骨架，部署在专用数据交易子链上，并通过 `SceneKind::DataTrade` 与 `SettlementMode::MainEscrow` 声明身份。

## 当前实现

- `pallet-data-registry`：数据拥有者发布 listing，记录 IMT root 和描述；只有 owner 可以更新 root。
- `pallet-trade-session`：创建交易会话、DR 锁定资金、DO 锁定押金、DO 通过哈希链 preimage claim 资金。
- `scripts/bridges/data_trade.js`：数据交易 bridge 骨架，只观察 `dataRegistry` 和 `tradeSession` 事件，不提交 FMC bill。
- `scripts/profiles/chains.json`：`child6` 声明为 `DataTrade/MainEscrow`。

## 边界

- 数据交易场景独立于数据众包 pallet。
- 第一版采用 `MainEscrow`，不会调用 FMC。
- `FmcAssisted` 和 `Hybrid` 只作为后续模式保留，适合需要周期预算的交易、训练或分析服务。
- ZK verifier 暂不接入，等会话状态机、资金状态机和争议入口稳定后再加入。

## 后续方向

- 把论文中的 VC/Fund 争议流程映射到 `trade-session`。
- 接入确定哈希函数和 ZK verifier。
- 为不同数据交易类型生成不同子链 profile 和参数哈希。
