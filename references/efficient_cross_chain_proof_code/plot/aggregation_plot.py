"""
aggregation_plot.py
===================
聚合证明综合对比图（2×2 四子图）

子图 (a)：聚合证明大小对比     — aggregation_proof_size_*.csv
子图 (b)：证明生成时间对比     — aggregation_prove_time_*.csv
子图 (c)：验证时间对比         — aggregation_verify_time_*.csv
子图 (d)：链上 Gas 开销对比    — aggregation_verify_gas_cost_*.csv

每个 CSV 均自动选取 data/ 目录中最新的时间戳版本。
输出：figures/aggregation_{ts}.png（时间戳取四个 CSV 中最新的那个）

用法（在 plot/ 目录下执行）：
    python aggregation_plot.py
"""

from pathlib import Path
import numpy as np
import pandas as pd
import matplotlib.pyplot as plt
import matplotlib.ticker as mticker

SCRIPT_DIR = Path(__file__).parent

# ── 代表性 K 值（仅用于证明大小子图的柱状图）─────────────────────────────────
TABLE_K = [2, 10, 30, 50, 100]

# ── 配色 ──────────────────────────────────────────────────────────────────────
COLOR_BASE   = "#D3D3D3"   # Piano：浅灰
COLOR_PROP   = "#4C72B0"   # BPiano：深蓝
COLOR_SAVING = "#E05C5C"   # 节省率 / 加速比：红


# ── 工具函数 ──────────────────────────────────────────────────────────────────
def latest_csv(data_dir: Path, prefix: str) -> Path:
    """返回 data_dir 中最新的 {prefix}_*.csv（按文件名字典序）。"""
    files = sorted(data_dir.glob(f"{prefix}_*.csv"))
    if not files:
        raise FileNotFoundError(f"data/ 中找不到 {prefix}_*.csv")
    return files[-1]


def ts_from_path(p: Path) -> str:
    """从文件名末尾提取时间戳，例如 aggregation_proof_size_20260330_203451 → 20260330_203451。"""
    parts = p.stem.split("_")
    return "_".join(parts[-2:])


# ── 子图绘制函数 ───────────────────────────────────────────────────────────────

def plot_proof_size(ax: plt.Axes, axr: plt.Axes, size_csv: Path) -> None:
    """子图 (a)：聚合证明大小柱状图 + 节省率折线（次 Y 轴）。"""
    df = pd.read_csv(size_csv)
    df_a = df[df["K"].isin(TABLE_K)].copy().reset_index(drop=True)
    df_a["piano_kb"]  = df_a["piano_size_bytes"]  / 1024
    df_a["bpiano_kb"] = df_a["bpiano_size_bytes"] / 1024

    x     = np.arange(len(df_a))
    width = 0.35

    bars_p = ax.bar(x - width / 2, df_a["piano_kb"],  width,
                    label="Piano",  color=COLOR_BASE, edgecolor="#888888", linewidth=1)
    bars_b = ax.bar(x + width / 2, df_a["bpiano_kb"], width,
                    label="BPiano", color=COLOR_PROP, edgecolor="#333333", linewidth=1)

    ax.set_xlabel("聚合证明数量 K", fontsize=11, fontweight="bold")
    ax.set_ylabel("聚合证明大小（KB）", fontsize=11, fontweight="bold")
    ax.set_xticks(x)
    ax.set_xticklabels([f"K={k}" for k in TABLE_K], fontsize=10)
    ax.tick_params(axis="both", direction="in", length=4)
    ax.yaxis.grid(True, linestyle=":", alpha=0.6, color="gray")
    ax.set_axisbelow(True)
    ax.spines["top"].set_visible(False)
    ax.set_ylim(0, df_a["piano_kb"].max() * 1.28)

    # 在 BPiano 柱顶标注节省率
    for rect, row in zip(bars_b, df_a.itertuples()):
        ax.annotate(
            f"节省\n{row.size_saving_pct:.1f}%",
            xy=(rect.get_x() + rect.get_width() / 2, rect.get_height()),
            xytext=(0, 4), textcoords="offset points",
            ha="center", va="bottom", fontsize=8,
            color=COLOR_SAVING, fontweight="bold",
        )

    # 次 Y 轴：节省率折线
    axr.plot(x, df_a["size_saving_pct"],
             color=COLOR_SAVING, marker="D", markersize=5,
             linewidth=1.5, linestyle="--", label="大小节省率")
    axr.set_ylabel("大小节省率（%）", fontsize=11, fontweight="bold", color=COLOR_SAVING)
    axr.set_ylim(0, 100)
    axr.tick_params(axis="y", colors=COLOR_SAVING, direction="in", length=4)
    axr.spines["top"].set_visible(False)

    h1, l1 = ax.get_legend_handles_labels()
    h2, l2 = axr.get_legend_handles_labels()
    ax.legend(h1 + h2, l1 + l2, fontsize=9, loc="upper left", frameon=False)
    ax.set_title("(a) 聚合证明大小对比", fontsize=11, fontweight="bold", pad=8)


def plot_prove_time(ax: plt.Axes, axr: plt.Axes, prove_csv: Path) -> None:
    """子图 (b)：证明生成时间折线 + 加速比折线（次 Y 轴）。"""
    df = pd.read_csv(prove_csv)
    k_vals   = df["K"].values
    piano_s  = df["piano_prove_s"].values
    bpiano_s = df["bpiano_prove_s"].values
    speedup  = df["prove_speedup"].values

    ax.plot(k_vals, piano_s,
            color=COLOR_BASE, linewidth=2, marker="o", markersize=5,
            markeredgecolor="#888888", markeredgewidth=1,
            label="Piano 总生成时间")
    ax.plot(k_vals, bpiano_s,
            color=COLOR_PROP, linewidth=2, marker="s", markersize=5,
            markeredgecolor="#333333", markeredgewidth=1,
            label="BPiano 总生成时间")

    ax.set_xlabel("聚合证明数量 K", fontsize=11, fontweight="bold")
    ax.set_ylabel("总生成时间（秒）", fontsize=11, fontweight="bold")
    ax.set_xlim(-2, 107)
    ax.set_ylim(0, max(piano_s) * 1.2)
    ax.set_xticks(k_vals)
    ax.tick_params(axis="x", labelsize=8)
    ax.tick_params(axis="both", direction="in", length=4)
    ax.yaxis.grid(True, linestyle=":", alpha=0.6, color="gray")
    ax.set_axisbelow(True)
    ax.spines["top"].set_visible(False)

    # 标注终点
    ax.annotate(f"{piano_s[-1]:.0f} s",
                xy=(k_vals[-1], piano_s[-1]),
                xytext=(3, 4), textcoords="offset points",
                fontsize=8, color="#888888", fontweight="bold")
    ax.annotate(f"{bpiano_s[-1]:.0f} s",
                xy=(k_vals[-1], bpiano_s[-1]),
                xytext=(3, -11), textcoords="offset points",
                fontsize=8, color=COLOR_PROP, fontweight="bold")

    # 次 Y 轴：生成时间加速比
    axr.plot(k_vals, speedup,
             color=COLOR_SAVING, linewidth=1.5, marker="D", markersize=4,
             linestyle="--", label="生成时间加速比")
    axr.axhline(1.0, color=COLOR_SAVING, linewidth=0.8, linestyle=":", alpha=0.6)
    axr.set_ylabel("生成时间加速比（×）", fontsize=11, fontweight="bold", color=COLOR_SAVING)
    axr.set_ylim(0, 2.5)
    axr.tick_params(axis="y", colors=COLOR_SAVING, direction="in", length=4)
    axr.spines["top"].set_visible(False)
    axr.annotate(f"{speedup[-1]:.2f}×",
                 xy=(k_vals[-1], speedup[-1]),
                 xytext=(-35, 8), textcoords="offset points",
                 fontsize=8, color=COLOR_SAVING, fontweight="bold")

    h1, l1 = ax.get_legend_handles_labels()
    h2, l2 = axr.get_legend_handles_labels()
    ax.legend(h1 + h2, l1 + l2, fontsize=9, loc="upper left", frameon=False)
    ax.set_title("(b) 证明生成时间对比", fontsize=11, fontweight="bold", pad=8)

    # 数据来源注释
    ax.annotate("† 总生成时间 = 单证明均值 × K（线性外推）",
                xy=(0.5, -0.18), xycoords="axes fraction",
                ha="center", fontsize=7.5, color="#888888", style="italic")


def plot_verify_time(ax: plt.Axes, axr: plt.Axes, time_csv: Path) -> None:
    """子图 (c)：验证时间双折线 + 加速比折线（次 Y 轴）。"""
    df = pd.read_csv(time_csv)
    k_vals    = df["K"].values
    piano_ms  = df["piano_verify_ms"].values
    bpiano_ms = df["bpiano_verify_ms"].values
    speedup   = df["verify_speedup"].values

    ax.plot(k_vals, piano_ms,
            color=COLOR_BASE, linewidth=2, marker="o", markersize=5,
            markeredgecolor="#888888", markeredgewidth=1,
            label="Piano 验证时间")
    ax.plot(k_vals, bpiano_ms,
            color=COLOR_PROP, linewidth=2, marker="s", markersize=5,
            markeredgecolor="#333333", markeredgewidth=1,
            label="BPiano 验证时间")

    ax.set_xlabel("聚合证明数量 K", fontsize=11, fontweight="bold")
    ax.set_ylabel("验证时间（ms）", fontsize=11, fontweight="bold")
    ax.set_xlim(-2, 107)
    ax.set_ylim(0, max(piano_ms) * 1.2)
    ax.set_xticks(k_vals)
    ax.tick_params(axis="x", labelsize=8)
    ax.tick_params(axis="both", direction="in", length=4)
    ax.yaxis.grid(True, linestyle=":", alpha=0.6, color="gray")
    ax.set_axisbelow(True)
    ax.spines["top"].set_visible(False)

    # 标注终点
    ax.annotate(f"{piano_ms[-1]:.0f} ms",
                xy=(k_vals[-1], piano_ms[-1]),
                xytext=(3, 4), textcoords="offset points",
                fontsize=8, color="#666666", fontweight="bold")
    ax.annotate(f"{bpiano_ms[-1]:.0f} ms",
                xy=(k_vals[-1], bpiano_ms[-1]),
                xytext=(3, -11), textcoords="offset points",
                fontsize=8, color=COLOR_PROP, fontweight="bold")

    # 次 Y 轴：加速比
    axr.plot(k_vals, speedup,
             color=COLOR_SAVING, linewidth=1.5, marker="D", markersize=4,
             linestyle="--", label="加速比")
    axr.axhline(1.0, color=COLOR_SAVING, linewidth=0.8, linestyle=":", alpha=0.6)
    axr.set_ylabel("验证时间加速比（×）", fontsize=11, fontweight="bold", color=COLOR_SAVING)
    axr.set_ylim(0, 2.5)
    axr.tick_params(axis="y", colors=COLOR_SAVING, direction="in", length=4)
    axr.spines["top"].set_visible(False)

    h1, l1 = ax.get_legend_handles_labels()
    h2, l2 = axr.get_legend_handles_labels()
    ax.legend(h1 + h2, l1 + l2, fontsize=9, loc="upper left", frameon=False)
    ax.set_title("(c) 验证时间对比", fontsize=11, fontweight="bold", pad=8)


def plot_gas_cost(ax: plt.Axes, axr: plt.Axes, gas_csv: Path) -> None:
    """子图 (d)：Gas 开销双折线 + 节省率折线（次 Y 轴）。"""
    df = pd.read_csv(gas_csv)
    k_vals     = df["K"].values
    piano_gas  = df["piano_gas"].values      / 1e6
    bpiano_gas = df["bpiano_agg_gas"].values / 1e6
    saving     = df["gas_saving_pct"].values

    ax.plot(k_vals, piano_gas,
            color=COLOR_BASE, linewidth=2, marker="o", markersize=5,
            markeredgecolor="#888888", markeredgewidth=1,
            label="Piano（逐一验证）")
    ax.plot(k_vals, bpiano_gas,
            color=COLOR_PROP, linewidth=2, marker="s", markersize=5,
            markeredgecolor="#333333", markeredgewidth=1,
            label="BPiano（聚合验证）")

    ax.set_xlabel("聚合证明数量 K", fontsize=11, fontweight="bold")
    ax.set_ylabel("链上 Gas 开销（×10^6）", fontsize=11, fontweight="bold")
    ax.set_xlim(-2, 107)
    ax.set_ylim(0, piano_gas.max() * 1.15)
    ax.set_xticks(k_vals)
    ax.tick_params(axis="x", labelsize=8)
    ax.tick_params(axis="both", direction="in", length=4)
    ax.yaxis.grid(True, linestyle=":", alpha=0.6, color="gray")
    ax.set_axisbelow(True)
    ax.spines["top"].set_visible(False)

    # 标注终点
    ax.annotate(f"{piano_gas[-1]:.1f}M",
                xy=(k_vals[-1], piano_gas[-1]),
                xytext=(4, 4), textcoords="offset points",
                fontsize=8, color="#666666", fontweight="bold")
    ax.annotate(f"{bpiano_gas[-1]:.1f}M",
                xy=(k_vals[-1], bpiano_gas[-1]),
                xytext=(4, 4), textcoords="offset points",
                fontsize=8, color=COLOR_PROP, fontweight="bold")

    # 次 Y 轴：Gas 节省率
    axr.plot(k_vals, saving,
             color=COLOR_SAVING, linewidth=1.5, marker="D", markersize=4,
             linestyle="--", label="Gas 节省率")
    axr.set_ylabel("Gas 节省率（%）", fontsize=11, fontweight="bold", color=COLOR_SAVING)
    axr.set_ylim(0, 100)
    axr.tick_params(axis="y", colors=COLOR_SAVING, direction="in", length=4)
    axr.spines["top"].set_visible(False)

    # 标注峰值节省率
    peak_idx = int(np.argmax(saving))
    axr.annotate(
        f"峰值\n{saving[peak_idx]:.1f}%",
        xy=(k_vals[peak_idx], saving[peak_idx]),
        xytext=(0, 10), textcoords="offset points",
        ha="center", va="bottom",
        fontsize=8, color=COLOR_SAVING, fontweight="bold",
    )

    h1, l1 = ax.get_legend_handles_labels()
    h2, l2 = axr.get_legend_handles_labels()
    ax.legend(h1 + h2, l1 + l2, fontsize=9, loc="upper left", frameon=False)
    ax.set_title("(d) 链上 Gas 开销对比", fontsize=11, fontweight="bold", pad=8)


# ── 主流程 ────────────────────────────────────────────────────────────────────

def generate_aggregation_figure(
    size_csv: Path, prove_csv: Path, time_csv: Path, gas_csv: Path,
    output_file: Path,
) -> None:
    plt.rcParams.update({
        "font.size": 11,
        "font.sans-serif": ["SimHei", "WenQuanYi Micro Hei", "Noto Sans CJK SC", "DejaVu Sans"],
        "font.family": "sans-serif",
        "axes.unicode_minus": False,
    })

    fig, axes = plt.subplots(2, 2, figsize=(16, 12))

    # 为每个子图创建次 Y 轴（twinx），传入各子图绘制函数
    ax_a, ax_b = axes[0, 0], axes[0, 1]
    ax_c, ax_d = axes[1, 0], axes[1, 1]
    axr_a = ax_a.twinx()
    axr_b = ax_b.twinx()
    axr_c = ax_c.twinx()
    axr_d = ax_d.twinx()

    plot_proof_size(ax_a,  axr_a, size_csv)
    plot_prove_time(ax_b,  axr_b, prove_csv)
    plot_verify_time(ax_c, axr_c, time_csv)
    plot_gas_cost(ax_d,    axr_d, gas_csv)

    output_file.parent.mkdir(parents=True, exist_ok=True)
    fig.tight_layout(pad=3.0, h_pad=4.0, w_pad=3.0)
    fig.savefig(output_file, dpi=300, bbox_inches="tight")
    plt.close(fig)
    print(f"Saved: {output_file}")


if __name__ == "__main__":
    data_dir   = SCRIPT_DIR / "data"
    size_csv   = latest_csv(data_dir, "aggregation_proof_size")
    prove_csv  = latest_csv(data_dir, "aggregation_prove_time")
    time_csv   = latest_csv(data_dir, "aggregation_verify_time")
    gas_csv    = latest_csv(data_dir, "aggregation_verify_gas_cost")

    # 以四个 CSV 中最新的时间戳命名输出图片
    ts = max(ts_from_path(p) for p in [size_csv, prove_csv, time_csv, gas_csv])
    out_file = SCRIPT_DIR / "figures" / f"aggregation_{ts}.png"

    generate_aggregation_figure(size_csv, prove_csv, time_csv, gas_csv, out_file)
