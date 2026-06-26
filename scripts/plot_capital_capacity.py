#!/usr/bin/env python3
"""Render the capital utilisation comparison figure with Origin style."""

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
DEFAULT_OUT_DIR = ROOT / "docs" / "experiments" / "figures"
DEFAULT_SUMMARY = DEFAULT_OUT_DIR / "data" / "exp_liquidity_horizon_summary.csv"

BLUE_DARK = "#1565C0"
BLUE_LIGHT = "#90CAF9"
ORANGE_DARK = "#E65100"
ORANGE_LIGHT = "#FFCC80"
CJK_REGULAR = "/usr/share/fonts/opentype/noto/NotoSerifCJK-Regular.ttc"
CJK_BOLD = "/usr/share/fonts/opentype/noto/NotoSerifCJK-Bold.ttc"


def cjk_font(size: int, *, bold: bool = False) -> fm.FontProperties:
    path = CJK_BOLD if bold and Path(CJK_BOLD).exists() else CJK_REGULAR
    if Path(path).exists():
        return fm.FontProperties(fname=path, size=size)
    return fm.FontProperties(family="WenQuanYi Micro Hei", size=size, weight="bold" if bold else "normal")


def apply_origin_style() -> None:
    plt.rcParams.update({
        "font.family": "serif",
        "font.serif": [
            "Times New Roman", "Times", "Nimbus Roman",
            "Noto Serif CJK SC", "WenQuanYi Micro Hei", "DejaVu Serif",
        ],
        "mathtext.fontset": "stix",
        "figure.dpi": 150,
        "savefig.dpi": 240,
        "savefig.bbox": "tight",
        "savefig.pad_inches": 0.04,
        "axes.linewidth": 1.8,
        "axes.labelsize": 28,
        "axes.labelweight": "bold",
        "xtick.labelsize": 24,
        "ytick.labelsize": 24,
        "legend.fontsize": 23,
        "lines.linewidth": 2.8,
    })


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


def origin_legend(ax, *, loc: str = "upper left", ncol: int = 1, fontsize: int = 23) -> None:
    legend = ax.legend(
        loc=loc,
        ncol=ncol,
        frameon=True,
        fancybox=False,
        framealpha=1,
        facecolor="white",
        edgecolor="black",
        borderpad=0.45,
        handlelength=2.4,
        handletextpad=0.45,
        prop=cjk_font(fontsize),
    )
    legend.get_frame().set_linewidth(0.9)


def save_figure(fig, out_dir: Path, stem: str, formats: tuple[str, ...]) -> list[Path]:
    out_dir.mkdir(parents=True, exist_ok=True)
    outputs = []
    for fmt in formats:
        path = out_dir / f"{stem}.{fmt}"
        fig.savefig(path)
        outputs.append(path)
    plt.close(fig)
    return outputs


def read_rows(path: Path) -> list[dict[str, str]]:
    with path.open(newline="") as f:
        return list(csv.DictReader(f))


def as_float(row: dict[str, str], key: str, default: float = 0.0) -> float:
    try:
        return float(row.get(key, ""))
    except (TypeError, ValueError):
        return default


def build_capacity_group_series(rows: list[dict[str, str]]) -> dict[str, list[int]]:
    horizons = [int(round(as_float(row, "horizon_epochs"))) for row in rows]
    fishbone_groups = [
        max(1, int(round(as_float(row, "locked_reduction_x") or horizon)))
        for row, horizon in zip(rows, horizons)
    ]
    traditional_groups = [1 for _ in rows]
    return {
        "horizons": horizons,
        "traditional_groups": traditional_groups,
        "fishbone_groups": fishbone_groups,
    }


def build_capital_capacity_figure(
    summary: Path = DEFAULT_SUMMARY,
    out_dir: Path = DEFAULT_OUT_DIR,
    formats: tuple[str, ...] = ("pdf", "png"),
) -> list[Path]:
    apply_origin_style()

    series = build_capacity_group_series(read_rows(summary))
    horizons = np.array(series["horizons"], dtype=int)
    traditional = np.array(series["traditional_groups"], dtype=float)
    fishbone = np.array(series["fishbone_groups"], dtype=float)

    x = np.arange(len(horizons), dtype=float)
    width = 0.34

    fig, ax = plt.subplots(figsize=(8.6, 5.7))

    ax.bar(x - width / 2, traditional, width, color=ORANGE_DARK, label="传统预锁方案")
    ax.bar(x + width / 2, fishbone, width, color=BLUE_DARK, label="FishboneChain")

    for xpos, value in zip(x - width / 2, traditional):
        ax.text(xpos, value + 0.45, f"{value:.0f}组", ha="center", fontsize=18, fontproperties=cjk_font(18))
    for xpos, value in zip(x + width / 2, fishbone):
        ax.text(
            xpos,
            value + 0.45,
            f"{value:.0f}组",
            ha="center",
            fontsize=19,
            color=BLUE_DARK,
            fontproperties=cjk_font(19, bold=True),
        )

    for i, value in enumerate(fishbone):
        ax.text(
            i,
            value + 2.3,
            f"{value:.0f}x",
            ha="center",
            fontsize=18,
            color=BLUE_DARK,
            fontproperties=cjk_font(18, bold=True),
        )

    ax.set_xlabel("规划周期（Epoch）", fontproperties=cjk_font(28, bold=True))
    ax.set_ylabel("可支持的6任务组数量", fontproperties=cjk_font(28, bold=True))
    ax.set_xticks(x)
    ax.set_xticklabels([str(h) for h in horizons])
    ax.set_xlim(x.min() - 0.6, x.max() + 0.6)
    ax.set_ylim(0, max(fishbone.max(), traditional.max()) * 1.25)

    origin_axes(ax, minor_y=False)
    origin_legend(ax, loc="upper left", fontsize=18)
    fig.tight_layout()
    return save_figure(fig, out_dir, "fig7b_capital_capacity_v2", formats)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--summary", type=Path, default=DEFAULT_SUMMARY)
    parser.add_argument("--out-dir", type=Path, default=DEFAULT_OUT_DIR)
    parser.add_argument("--formats", default="pdf,png")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    formats = tuple(fmt.strip().lower() for fmt in args.formats.split(",") if fmt.strip())
    outputs = build_capital_capacity_figure(args.summary, args.out_dir, formats)
    for path in outputs:
        print(f"[saved] {path}")


if __name__ == "__main__":
    main()
