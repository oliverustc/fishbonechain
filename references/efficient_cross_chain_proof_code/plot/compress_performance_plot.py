"""
compress_performance_plot.py
============================
读取 data/compress_performance.csv，生成 Piano vs BPiano 单证明压缩效果对比图，
保存至 figures/compress_performance.png。

用法（在 plot/ 目录下执行）：
    python compress_performance_plot.py
"""

from pathlib import Path
import pandas as pd
import matplotlib.pyplot as plt
import numpy as np

# ── 路径配置 ──────────────────────────────────────────────────────────────────
SCRIPT_DIR = Path(__file__).parent


def latest_csv(data_dir: Path, prefix: str) -> Path:
    """返回 data_dir 中最新的 {prefix}_*.csv（按文件名字典序）。"""
    files = sorted(data_dir.glob(f"{prefix}_*.csv"))
    if not files:
        raise FileNotFoundError(f"data/ 中找不到 {prefix}_*.csv")
    return files[-1]


def ts_from_path(p: Path) -> str:
    """从文件名末尾提取时间戳，例如 compress_performance_20260330_203451 → 20260330_203451。"""
    parts = p.stem.split("_")
    return "_".join(parts[-2:])

# X 轴标签（与 CSV 中 metric 行的顺序一致）
X_LABELS = [
    "证明大小\n（字节）",
    "证明时间\n（ms）",
    "验证时间\n（ms）",
    "链上 Gas 开销",
]

def format_val(val: float) -> str:
    """千分位格式：>=10 显示整数+逗号，否则保留两位小数。"""
    return f"{val:,.0f}" if val >= 10 else f"{val:.2f}"

def generate_plot(csv_file: Path, output_file: Path) -> None:
    # ── 1. 载入数据 ───────────────────────────────────────────────────────────
    df = pd.read_csv(csv_file)
    df["piano_pct"]  = 100.0
    df["bpiano_pct"] = (df["bpiano"] / df["piano"]) * 100

    # ── 2. 布局与配色 ─────────────────────────────────────────────────────────
    plt.rcParams.update({
        "font.size": 12,
        "font.sans-serif": ["SimHei", "WenQuanYi Micro Hei", "Noto Sans CJK SC", "DejaVu Sans"],
        "font.family": "sans-serif",
        "axes.unicode_minus": False,   # 防止负号显示为方块
    })
    fig, ax = plt.subplots(figsize=(10, 6.5))

    x     = np.arange(len(df))
    width = 0.35
    color_base = "#D3D3D3"   # 浅灰：Baseline 不抢眼
    color_prop = "#4C72B0"   # 深蓝：Proposed 核心焦点

    # ── 3. 绘制柱状图 ─────────────────────────────────────────────────────────
    rects1 = ax.bar(x - width / 2, df["piano_pct"],  width,
                    label="基准方案（Piano）",  color=color_base,
                    edgecolor="#888888", linewidth=1)
    rects2 = ax.bar(x + width / 2, df["bpiano_pct"], width,
                    label="本方案（BPiano）", color=color_prop,
                    edgecolor="#333333", linewidth=1)

    # ── 4. 坐标轴 ─────────────────────────────────────────────────────────────
    ax.set_ylabel("归一化比值（%，Piano = 100%）", fontsize=14, fontweight="bold")
    ax.set_xticks(x)
    ax.set_xticklabels(X_LABELS, fontsize=12, fontweight="bold")

    # Y 轴上限：取所有柱高最大值 + 25，为标注留出空间
    max_pct = max(df["bpiano_pct"].max(), 100.0)
    ax.set_ylim(0, max_pct + 30)
    ax.tick_params(axis="y", labelsize=11)
    ax.tick_params(axis="both", direction="in", length=5)

    # ── 5. 参考线与网格 ───────────────────────────────────────────────────────
    ax.axhline(100, color="gray", linewidth=1.2, linestyle="--", alpha=0.7)
    ax.yaxis.grid(True, linestyle=":", alpha=0.6, color="gray")
    ax.set_axisbelow(True)

    # ── 6. 图例 ───────────────────────────────────────────────────────────────
    ax.legend(fontsize=12, loc="upper right", frameon=False)

    # ── 7. 数据标注 ───────────────────────────────────────────────────────────
    # Baseline：仅显示绝对值
    for i, rect in enumerate(rects1):
        ax.annotate(
            format_val(df["piano"].iloc[i]),
            xy=(rect.get_x() + rect.get_width() / 2, rect.get_height()),
            xytext=(0, 5), textcoords="offset points",
            ha="center", va="bottom", fontsize=11,
            color="#444444", fontweight="bold",
        )

    # Proposed：显示"相对百分比\n(绝对值)"
    for i, rect in enumerate(rects2):
        pct = df["bpiano_pct"].iloc[i]
        val_str = format_val(df["bpiano"].iloc[i])
        ax.annotate(
            f"{pct:.1f}%\n({val_str})",
            xy=(rect.get_x() + rect.get_width() / 2, rect.get_height()),
            xytext=(0, 5), textcoords="offset points",
            ha="center", va="bottom", fontsize=11,
            color="#111111", fontweight="bold",
        )

    # ── 8. 去除上/右边框，紧凑排版，保存 ─────────────────────────────────────
    ax.spines["top"].set_visible(False)
    ax.spines["right"].set_visible(False)

    output_file.parent.mkdir(parents=True, exist_ok=True)
    fig.tight_layout()
    fig.savefig(output_file, dpi=300, bbox_inches="tight")
    plt.close(fig)
    print(f"Saved: {output_file}")

if __name__ == "__main__":
    data_dir = SCRIPT_DIR / "data"
    csv_file = latest_csv(data_dir, "compress_performance")
    ts       = ts_from_path(csv_file)
    out_file = SCRIPT_DIR / "figures" / f"compress_performance_{ts}.png"
    generate_plot(csv_file, out_file)