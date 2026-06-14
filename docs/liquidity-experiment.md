# 资金流动性实验记录

**最后更新**：2026-06-11  
**状态**：已有采集脚本、链上数据和图表；正式结论仍应以复查后的 `docs/experiment-report.md` 为准。

## 实验目标

验证 FishboneChain 的周期性资金管理机制：

- 传统方案需要在任务开始时预锁 `T × ΣB`
- FishboneChain 只需要锁定当前 Epoch 的任务预算 `ΣB`
- 结算后未使用预算归还 `free`，余额足够时自动续期

## 采集脚本

`scripts/metrics_fund.js` 采集主链 FMC 资金状态，并输出 CSV。

核心字段：

- `free_unit`：FMC pool 中可用资金
- `pool_locked_unit`：链上 `fmc.fundPools(requester).locked`
- `task_locked_unit`：当前 Activated 任务的预算和，用于表示实际工作锁定资金
- `baseline_locked_unit`：传统预锁方案的反事实锁定资金
- `bill_settled_this_tick`：采样间隔内观察到的 `BillSettled` 事件数
- `total_paid_unit`：采样间隔内累计支付金额

`pool_locked_unit` 可能包含历史任务遗留锁定；论文图表主要使用 `task_locked_unit` 与传统反事实基线对比。

## 当前数据

主要数据文件：

- `docs/figures/data/exp_e_fund_state_v5.csv`
- `docs/figures/data/exp_liquidity_horizon_summary.csv`

当前数据概况：

- 采样行数：1092
- 观察到 `BillSettled` 的采样点：17
- 累计 `BillSettled` 事件数：19
- 累计支付：约 200,152 UNIT
- 6 个任务初始 `ΣB`：71,502.5 UNIT / Epoch
- 展示窗口 `T_planned`：3 Epoch
- 传统预锁基线初始值：214,507.5 UNIT
- FishboneChain 初始工作锁定：71,502.5 UNIT
- 初始资金锁定改善：约 3x
- T=10 反事实初始锁定改善：约 10x
- T=20 反事实初始锁定改善：约 20x

## 图表

- `docs/figures/fig7a_liquidity_ratio.png`：锁定资金比例时序对比
- `docs/figures/fig7b_capital_capacity.png`：相同总预算下的可用资金对比
- `docs/figures/fig_liquidity_horizon.png`：T=3/10/20 长周期资金锁定对比

这些图分别由以下命令生成：

```bash
python3 scripts/plot_results.py --fig7
python3 scripts/plot_results.py --fig-liquidity-horizon
```

## 注意事项

- `fig7a/fig7b` 使用 `T=3` 采样展示窗口，改善约 3x。
- `fig_liquidity_horizon` 使用相同 `ΣB` 对 T=10/T=20 做计划周期反事实分析，用于展示资金锁定模型随 T 伸缩到 10x/20x。
- 早期 plan 文档里还出现过 3 任务、31.5K、630K 的图示口径；当前正式图表按 6 任务 `ΣB=71,502.5` 生成。
- 后续如果重新运行更长周期资金实验，应更新本文件、`docs/experiment-report.md` 和 fig7 数据/图表。
