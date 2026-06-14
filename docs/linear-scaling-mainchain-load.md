# 子链线性扩展与主链负载实验

## 目标

本实验用于补强线性扩展结论：在并发子链数量从 `N=1` 增加到 `N=6` 时，系统聚合吞吐量应近似线性提升；同时主链只处理跨链摘要、账单和资金结算，主链交易负载随子链数量增长但常数很小，不成为系统瓶颈。

建议将该实验作为吞吐量报告中的核心图之一，用来回答两个问题：

- 多条子链是否能把众包提交吞吐横向扩展出去。
- 开启 6 条子链后，主链是否仍然有充足余量。

## 子链组合

为减少异构任务带来的解释干扰，线性扩展实验应优先采用同规格或近似同规格 workload。若复用现有 6 条子链部署，可以按以下顺序逐步增加活跃子链：

| N | 活跃子链 |
|---|---|
| 1 | child4 |
| 2 | child4, child1 |
| 3 | child4, child1, child6 |
| 4 | child4, child1, child6, child3 |
| 5 | child4, child1, child6, child3, child2 |
| 6 | child4, child1, child6, child3, child2, child5 |

每个 `N` 至少采集 3 个完整 epoch。正式报告中优先使用去掉首尾不完整 epoch 后的平均值。

## 推荐工作负载

建议每条活跃子链使用统一参数，避免“吞吐提升来自某条链任务更轻”的质疑。当前远端部署中 child2 和 child5 的任务预算较小，因此实际执行时统一使用 `0.001 UNIT` 奖励，使 6 条链都能在同一 worker/频率/payload 口径下运行：

- 每条子链 `50` 个 worker。
- 每个 worker 提交间隔 `0.005/s` 或与现有稳定高负载参数一致。
- 每次提交数据大小 `800 bytes`。
- 每次提交奖励 `0.001 UNIT`，即 `1000000000` planck。

若重新初始化任务并为所有子链配置充足预算，也可以使用更高奖励；但正式报告必须说明采用的 reward 口径，避免 child2/child5 因预算耗尽影响吞吐解释。

## 采集方法

子链提交量继续使用现有 `metrics.js`：

```bash
node scripts/metrics.js \
  --chains ws://10.2.2.11:9948,ws://10.2.2.11:9945 \
  --out /tmp/exp_scale_n2 \
  --interval 15
```

完整远端批处理使用：

```bash
bash scripts/run_exp_scale_mainchain.sh
```

该脚本会按 `N=1..6` 顺序启动：

- 子链 `metrics.js`
- 主链 `metrics_main.js`
- 每条活跃子链的 `bridge.js`
- 每条活跃子链 50 个 worker

默认输出在：

```text
/tmp/fishbone_scale_mainchain_<RUN_ID>/
```

默认日志在：

```text
~/exp_scale_mainchain_logs/<RUN_ID>/
```

若只想先做短测，可以覆盖每个 N 的持续时间：

```bash
DURATION_N1=180 DURATION_N2=180 DURATION_N3=180 \
DURATION_N4=180 DURATION_N5=180 DURATION_N6=180 \
bash scripts/run_exp_scale_mainchain.sh
```

主链负载使用新增的 `metrics_main.js`：

```bash
MAIN_WS=ws://10.2.2.11:9944 \
node scripts/metrics_main.js \
  --out /tmp/exp_scale_n2 \
  --interval 6
```

`metrics_main.js` 会记录 finalized 主链区块，并补齐上次采样高度到当前 finalized 高度之间的所有区块，输出：

```text
/tmp/exp_scale_n2_main_blocks.csv
```

字段包括：

```text
timestamp,block_number,block_hash,extrinsics_total,bridge_extrinsics,ccmc_digest_calls,fmc_bill_calls,ccmc_events,fmc_events
```

其中 `bridge_extrinsics` 统计 `ccmc.submitEpochDigest` 和 `fmc.submitBill`，用于衡量每个 epoch 主链实际承接的跨链/结算交易。

## 汇总数据

完成 `N=1..6` 后，整理为：

```text
docs/figures/data/exp_scale_mainchain_summary.csv
```

CSV schema：

```text
n,active_chains,child_subs_tps,main_bridge_tps,main_total_tps,main_bridge_to_child_tps_pct,main_bridge_share_of_observed_main_tx_pct,child_subs_per_min,child_ok_total,worker_duration_min,child_subs_per_epoch,main_bridge_tx_per_epoch,main_bridge_tx_per_min,main_tx_per_min
```

字段含义：

| 字段 | 含义 |
|------|------|
| `n` | 并发活跃子链数量 |
| `active_chains` | 活跃子链名称，使用 `+` 连接 |
| `child_subs_tps` | worker 日志统计的子链聚合成功提交 TPS，推荐作为线性扩展主指标 |
| `main_bridge_tps` | 主链桥接/结算交易 TPS |
| `main_total_tps` | 主链观测到的总 extrinsics TPS |
| `main_bridge_to_child_tps_pct` | `main_bridge_tps / child_subs_tps`，表示子链高频提交被压缩成主链低频桥接交易的比例 |
| `main_bridge_share_of_observed_main_tx_pct` | `main_bridge_tps / main_total_tps`，表示桥接交易在观测主链交易中的占比；这不是主链容量占用率 |
| `child_subs_per_min` | 子链聚合成功提交 / 分钟，保留作兼容字段 |
| `child_ok_total` | 该 N 配置下所有活跃 worker 的成功提交总数 |
| `worker_duration_min` | 该 N 配置下 worker 实际运行分钟数 |
| `child_subs_per_epoch` | 活跃子链在稳定 epoch 内的聚合提交量均值，仅作辅助口径 |
| `main_bridge_tx_per_epoch` | 同窗口内主链桥接/结算交易均值，仅作辅助口径 |
| `main_bridge_tx_per_min` | 主链桥接/结算交易速率 |
| `main_tx_per_min` | 同窗口内主链总 extrinsics 速率 |

可使用脚本从原始 CSV 生成：

```bash
python3 scripts/summarize_scale_mainchain.py \
  --raw-dir /tmp/fishbone_scale_mainchain_<RUN_ID> \
  --log-dir ~/exp_scale_mainchain_logs/<RUN_ID> \
  --out docs/figures/data/exp_scale_mainchain_summary.csv
```

主链负载解释应优先使用 `child_subs_tps`、`main_bridge_tps` 和 `main_bridge_to_child_tps_pct`。不要把 `main_bridge_share_of_observed_main_tx_pct` 写成主链容量占用率；若要声明“占主链容量 X%”，需要额外进行主链压力测试或采集 block weight 使用率。

## 绘图命令

```bash
python3 scripts/plot_results.py --fig-scale-main
```

输出：

```text
docs/figures/fig_scale_mainchain_load.png
```

图中左侧展示子链聚合成功提交 TPS 与理想线性参考线，右侧展示主链桥接 TPS、主链总 TPS 以及桥接 TPS 相对子链 TPS 的占比。推荐标题为：

```text
子链吞吐线性扩展与主链负载
```

## 报告表述建议

若结果符合预期，可以在报告中表述为：

> 当并发子链数量从 1 增加到 6 时，聚合提交吞吐量随 N 近似线性增长；同时主链仅承担每个 epoch 的摘要、账单与结算交易，主链交易速率保持在较低水平。该结果说明 FishboneChain 将高频众包提交下沉到专用子链后，主链没有随着 worker 提交量线性承压，仍保留充足处理余量。

需要避免的表述：

- 不要仅凭该图宣称主链“无限可扩展”。
- 不要把异构 6 链实验直接解释为严格线性扩展，除非 workload 已统一。
- 不要把主链低负载解释为完整安全性证明；安全性仍依赖 CCMC/FMC 证明和桥接流程的实现假设。
