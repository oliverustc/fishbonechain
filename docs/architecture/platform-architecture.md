# FishboneChain 平台架构边界

FishboneChain 是安全可扩展的数据流通平台。数据众包是已经落地的第一个场景，数据交易、可验证训练和数据不出域分析是后续可部署到专用子链上的场景。

## 平台层

- `pallet-ccmc` 是平台强制能力：负责子链注册、矿工集合、Epoch 摘要锚定和惩罚投票。
- `pallet-chain-profile` 是平台强制能力：每条链通过 `chain_id`、`SceneKind`、`SettlementMode` 和 `params_hash` 声明链身份与场景参数。
- `pallet-fmc` 是平台可选能力：适合周期性预算、任务账单和多 epoch 流动性管理。
- 后续 `pallet-tmc` 可以作为任务元数据或模板管理能力，但不应固化众包字段。

## 场景层

- `pallet-crowdsource` 只代表数据众包场景，不是所有子链的必需模块。
- `pallet-data-registry` 和 `pallet-trade-session` 是数据交易场景骨架，不依赖 `pallet-crowdsource`。
- zk+机器学习训练、zkVM 数据不出域分析等场景应新增独立 pallet 和 bridge adapter。

## 结算模式

- `FmcTaskBill`：周期性任务账单模式，适合数据众包。
- `MainEscrow`：主链锁资/押金托管模式，适合第一版 CDT 数据交易。
- `Hybrid`：混合模式，预留给既有周期预算又有复杂争议结算的服务。
- `None`：只使用链身份和摘要锚定，不使用平台资金模块。

平台层不能依赖具体场景 pallet。场景 pallet 可以选择 FMC，也可以选择传统主链锁资，具体选择由 chain profile 和场景协议决定。
