# FishboneChain 部署矩阵图

本图用于 PPT 内嵌展示实验网络规模和子链部署工作量。当前版本采用精简部署矩阵，减少 IP、RPC 端口等细节；顶部规模信息改为一行纯文本摘要，避免多个指标卡在 PPT 中显得拥挤。

输出文件：

- `docs/experiments/figures/fishbone_topology_workload.svg`
- `docs/experiments/figures/fishbone_topology_workload.png`

生成脚本：

```bash
python3 scripts/render_topology_diagram.py
```

脚本从 `deploy/config.toml` 读取节点、主链、子链和验证人归属；图中保留以下 PPT 重点信息：

- 12 台 VM 节点
- 1 条主链 + 6 条业务子链
- 24 个子链验证席位
- 6 个业务场景的子链分布
- 子链验证人均来自主链验证人集合
- N=6 容量测试吞吐：396 TPS

如果后续调整节点或子链配置，优先更新 `deploy/config.toml`，再重新执行生成脚本。
