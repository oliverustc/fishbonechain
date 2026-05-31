# plot — 实验数据可视化

## 环境要求

- Python 3.8+
- pip

## 快速搭建

```bash
cd plot
python -m venv .venv
source .venv/bin/activate      # Windows: .venv\Scripts\activate
pip install pandas matplotlib numpy
```

### 中文字体（Linux / WSL）

图表使用中文标签，需要系统中存在 CJK 字体，脚本按优先级依次尝试
`SimHei → WenQuanYi Micro Hei → Noto Sans CJK SC`。

```bash
# Ubuntu / Debian / WSL
sudo apt install fonts-wqy-microhei
# 安装后清除 matplotlib 字体缓存
python -c "import matplotlib; import shutil; shutil.rmtree(matplotlib.get_cachedir())"
```

Windows 和 macOS 通常已内置 SimHei / PingFang，无需额外安装。

## 绘图脚本

| 脚本 | 输出图片 | 说明 |
|---|---|---|
| `compress_performance_plot.py` | `figures/compress_performance.png` | 图0：单证明压缩效果（Piano vs BPiano，4项指标柱状图） |
| `aggregation_figure1_plot.py` | `figures/aggregation_figure1.png` | 图一：聚合证明大小对比（子图a）+ 验证时间加速比（子图b） |
| `aggregation_figure2_plot.py` | `figures/aggregation_figure2.png` | 图二：链上 Gas 开销对比（双折线 + 节省率次坐标轴） |

### 运行

```bash
# 在 plot/ 目录下执行（需先激活虚拟环境）
python compress_performance_plot.py
python aggregation_figure1_plot.py
python aggregation_figure2_plot.py
```

## 目录结构

```
plot/
├── data/                               # 输入数据（从 bench/results/ 手动复制并去掉时间戳）
│   ├── compress_performance.csv        # 来自 compress_performance_<ts>.csv
│   ├── aggregation_size.csv            # 来自 aggregation_size_<ts>.csv
│   ├── aggregation_time.csv            # 来自 aggregation_time_<ts>.csv
│   ├── aggregation_gas_cost.csv        # 来自 aggregation_gas_cost_<ts>.csv
│   └── aggregation_table.csv          # 来自 aggregation_table_<ts>.csv（论文表格数据）
├── figures/                            # 输出目录（自动创建）
├── compress_performance_plot.py
├── aggregation_figure1_plot.py
├── aggregation_figure2_plot.py
└── README.md
```

## 更新数据

重新运行 bench 后，将 `bench/results/` 下带时间戳的 CSV 复制到 `data/`，去掉时间戳部分，再执行对应绘图脚本即可。

```bash
# 示例（将 <ts> 替换为实际时间戳）
cp bench/results/compress_performance_<ts>.csv    plot/data/compress_performance.csv
cp bench/results/aggregation_size_<ts>.csv        plot/data/aggregation_size.csv
cp bench/results/aggregation_time_<ts>.csv        plot/data/aggregation_time.csv
cp bench/results/aggregation_gas_cost_<ts>.csv    plot/data/aggregation_gas_cost.csv
cp bench/results/aggregation_table_<ts>.csv       plot/data/aggregation_table.csv
```
