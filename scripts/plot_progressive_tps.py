#!/usr/bin/env python3
"""Render the single progressive TPS and mainchain-pressure figure."""

from __future__ import annotations

import argparse
import csv
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.font_manager as fm
import matplotlib.pyplot as plt
from matplotlib.ticker import AutoMinorLocator
import numpy as np


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_SUMMARY = ROOT / "docs" / "experiments" / "progressive_tps" / "progressive_tps_summary.csv"
DEFAULT_OUT_DIR = ROOT / "docs" / "experiments" / "progressive_tps" / "figures"

BAR_BLUE = "#2374d7"
LINE_RED = "#ff0000"
CJK_REGULAR = "/usr/share/fonts/opentype/noto/NotoSerifCJK-Regular.ttc"
CJK_BOLD = "/usr/share/fonts/opentype/noto/NotoSerifCJK-Bold.ttc"


def cjk_font(size: int, *, bold: bool = False) -> fm.FontProperties:
    path = CJK_BOLD if bold and Path(CJK_BOLD).exists() else CJK_REGULAR
    if Path(path).exists():
        return fm.FontProperties(fname=path, size=size)
    return fm.FontProperties(family="WenQuanYi Micro Hei", size=size, weight="bold" if bold else "normal")


def apply_origin_style() -> None:
    plt.rcParams.update(
        {
            "font.family": "serif",
            "font.serif": [
                "Times New Roman",
                "Times",
                "Nimbus Roman",
                "Noto Serif CJK SC",
                "WenQuanYi Micro Hei",
                "DejaVu Serif",
            ],
            "mathtext.fontset": "stix",
            "figure.dpi": 150,
            "savefig.dpi": 240,
            "savefig.bbox": "tight",
            "savefig.pad_inches": 0.04,
            "axes.linewidth": 1.8,
            "axes.labelsize": 26,
            "axes.labelweight": "bold",
            "xtick.labelsize": 21,
            "ytick.labelsize": 22,
            "legend.fontsize": 21,
            "lines.linewidth": 2.8,
        }
    )


def origin_axes(ax, *, minor_y: bool = True) -> None:
    ax.tick_params(which="major", direction="in", top=True, right=True, length=8, width=1.8, pad=8)
    ax.tick_params(which="minor", direction="in", top=True, right=True, length=4, width=1.5)
    ax.xaxis.set_minor_locator(AutoMinorLocator(2))
    if minor_y:
        ax.yaxis.set_minor_locator(AutoMinorLocator(2))
    ax.grid(True, which="major", color="#777777", linewidth=0.7, alpha=0.85)
    ax.grid(True, which="minor", color="#aaaaaa", linewidth=0.45, linestyle=(0, (1, 5)), alpha=0.75)
    ax.set_axisbelow(True)
    for spine in ax.spines.values():
        spine.set_linewidth(1.8)
        spine.set_color("black")


def pressure_axis_upper(values: np.ndarray) -> float:
    if values.size == 0:
        return 0.05
    max_value = float(values.max())
    if max_value <= 0:
        return 0.05
    return max(max_value * 1.8, 0.05)


def read_rows(path: Path) -> list[dict[str, str]]:
    with path.open(newline="") as f:
        return list(csv.DictReader(f))


def as_float(row: dict[str, str], key: str, default: float = 0.0) -> float:
    try:
        return float(row.get(key, ""))
    except ValueError:
        return default


def save_figure(fig, out_dir: Path, stem: str, formats: tuple[str, ...]) -> list[Path]:
    out_dir.mkdir(parents=True, exist_ok=True)
    outputs = []
    for fmt in formats:
        path = out_dir / f"{stem}.{fmt}"
        fig.savefig(path)
        outputs.append(path)
    plt.close(fig)
    return outputs


def build_progressive_tps_figure(
    summary: Path = DEFAULT_SUMMARY,
    out_dir: Path = DEFAULT_OUT_DIR,
    formats: tuple[str, ...] = ("pdf", "png"),
) -> list[Path]:
    apply_origin_style()
    rows = sorted(read_rows(summary), key=lambda row: int(row["n"]))
    ns = np.array([int(row["n"]) for row in rows], dtype=float)
    child_tps = np.array([as_float(row, "aggregate_child_tps") for row in rows])
    main_pressure = np.array([as_float(row, "main_bridge_pressure_pct") for row in rows])
    main_bridge_events = np.array([as_float(row, "main_bridge_events") for row in rows])

    labels = [f"N={int(n)}" for n in ns]
    fig, ax = plt.subplots(figsize=(9.4, 6.1))
    bars = ax.bar(ns, child_tps, width=0.58, color=BAR_BLUE, label="子链聚合吞吐量")

    ax2 = ax.twinx()
    ax2.plot(
        ns,
        main_pressure,
        color=LINE_RED,
        marker="s",
        markersize=9,
        markerfacecolor="white",
        markeredgewidth=1.8,
        label="主链桥接压力",
    )

    for bar, value in zip(bars, child_tps):
        ax.text(
            bar.get_x() + bar.get_width() / 2,
            value + max(child_tps.max() * 0.025, 8),
            f"{value:.0f}",
            ha="center",
            va="bottom",
            fontsize=18,
        )

    ax.axvline(3.5, color="black", linestyle="--", linewidth=1.4, alpha=0.75)
    ax.text(2.0, child_tps.max() * 1.08, "部署/出块/RPC 调优", ha="center", fontproperties=cjk_font(20))
    ax.text(5.0, child_tps.max() * 1.08, "递进式子链优化", ha="center", fontproperties=cjk_font(20))
    if main_pressure.max() <= 0 and main_bridge_events.sum() <= 0:
        ax.text(
            ns.mean(),
            child_tps.max() * 0.065,
            "桥接事件=0",
            ha="center",
            va="center",
            color=LINE_RED,
            fontproperties=cjk_font(18),
            bbox={"facecolor": "white", "edgecolor": LINE_RED, "boxstyle": "square,pad=0.22", "linewidth": 0.9},
        )

    ax.set_xlabel("并发子链数量 / 配置阶段", fontproperties=cjk_font(26, bold=True))
    ax.set_ylabel("子链聚合接受吞吐量（提交/s）", fontproperties=cjk_font(26, bold=True))
    ax2.set_ylabel("主链桥接压力（%）", fontproperties=cjk_font(26, bold=True))
    ax.set_xlim(ns.min() - 0.65, ns.max() + 0.65)
    ax.set_ylim(0, max(child_tps.max() * 1.2, 1))
    ax2.set_ylim(0, pressure_axis_upper(main_pressure))
    ax.set_xticks(ns)
    ax.set_xticklabels(labels)

    origin_axes(ax)
    ax.tick_params(axis="y", which="both", right=False, labelright=False)
    ax2.tick_params(which="major", direction="in", top=True, right=True, length=8, width=1.8, pad=8)
    ax2.tick_params(which="minor", direction="in", top=True, right=True, length=4, width=1.5)
    ax2.yaxis.set_minor_locator(AutoMinorLocator(2))
    for spine in ax2.spines.values():
        spine.set_linewidth(1.8)
        spine.set_color("black")

    handles1, labels1 = ax.get_legend_handles_labels()
    handles2, labels2 = ax2.get_legend_handles_labels()
    legend = ax.legend(
        handles1 + handles2,
        labels1 + labels2,
        loc="upper left",
        frameon=True,
        fancybox=False,
        framealpha=1,
        facecolor="white",
        edgecolor="black",
        prop=cjk_font(21),
    )
    legend.get_frame().set_linewidth(0.9)
    fig.tight_layout()
    return save_figure(fig, out_dir, "progressive_tps_mainchain_load", formats)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--summary", type=Path, default=DEFAULT_SUMMARY)
    parser.add_argument("--out-dir", type=Path, default=DEFAULT_OUT_DIR)
    parser.add_argument("--formats", default="pdf,png")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    formats = tuple(fmt.strip().lower() for fmt in args.formats.split(",") if fmt.strip())
    outputs = build_progressive_tps_figure(args.summary, args.out_dir, formats)
    for path in outputs:
        print(f"[saved] {path}")


if __name__ == "__main__":
    main()
