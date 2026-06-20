#!/usr/bin/env python3
"""Render the capital utilisation comparison figure with Origin style."""

from __future__ import annotations

import argparse
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.font_manager as fm
import matplotlib.pyplot as plt
from matplotlib.ticker import AutoMinorLocator
import numpy as np

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_OUT_DIR = ROOT / "docs" / "experiments" / "figures"

BLUE_DARK = "#1565C0"
BLUE_LIGHT = "#90CAF9"
ORANGE_DARK = "#E65100"
ORANGE_LIGHT = "#FFCC80"
CJK_REGULAR = "/usr/share/fonts/opentype/noto/NotoSerifCJK-Regular.ttc"
CJK_BOLD = "/usr/share/fonts/opentype/noto/NotoSerifCJK-Bold.ttc"

SUM_BUDGET = 71502.5
T_PLANNED = 3
TOTAL_DEPOSIT = T_PLANNED * SUM_BUDGET
CHILD3_BUDGET = 40000

FB_LOCKED = SUM_BUDGET
FB_FREE = TOTAL_DEPOSIT - FB_LOCKED
TRAD_LOCKED = TOTAL_DEPOSIT
TRAD_FREE = 0.0


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


def origin_legend(ax, *, loc: str = "upper left", ncol: int = 1) -> None:
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
        prop=cjk_font(23),
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


def build_capital_capacity_figure(
    out_dir: Path = DEFAULT_OUT_DIR,
    formats: tuple[str, ...] = ("pdf", "png"),
) -> list[Path]:
    apply_origin_style()

    methods = ["FishboneChain", "传统预锁方案"]
    locked_vals = [FB_LOCKED, TRAD_LOCKED]
    free_vals = [FB_FREE, TRAD_FREE]

    fig, ax = plt.subplots(figsize=(10.0, 4.8))

    ax.barh(
        methods, locked_vals,
        color=[BLUE_DARK, ORANGE_DARK],
        height=0.45,
    )
    ax.barh(
        methods, free_vals, left=locked_vals,
        color=[BLUE_LIGHT, ORANGE_LIGHT],
        height=0.45,
    )

    ax.text(
        TRAD_LOCKED - TOTAL_DEPOSIT * 0.01, 1,
        f"{TRAD_LOCKED:,.0f} U (100%)",
        ha="right", va="center", color="white",
        fontsize=20, fontweight="bold",
    )
    ax.text(
        FB_LOCKED / 2, 0,
        f"{FB_LOCKED:,.0f} U\n({FB_LOCKED / TOTAL_DEPOSIT * 100:.1f}%)",
        ha="center", va="center", color="white",
        fontsize=20, fontweight="bold",
    )
    ax.text(
        FB_LOCKED + FB_FREE / 2, 0,
        f"{FB_FREE:,.0f} U\n({FB_FREE / TOTAL_DEPOSIT * 100:.1f}%)",
        ha="center", va="center", color="#333",
        fontsize=20, fontweight="bold",
    )

    new_tasks_fb = int(FB_FREE / CHILD3_BUDGET)
    ax.text(
        TOTAL_DEPOSIT * 0.62, 0.43,
        f"剩余资金可再激活 {new_tasks_fb} 个 child3 规格任务",
        ha="center", va="center", fontsize=18,
        color=BLUE_DARK, fontweight="bold",
        fontproperties=cjk_font(18),
    )
    ax.text(
        TOTAL_DEPOSIT * 0.5, 1,
        "资金全部锁定",
        ha="center", va="center", color="white",
        fontsize=18, fontstyle="italic", alpha=0.9,
        fontproperties=cjk_font(18),
    )

    ax.set_yticks([0, 1])
    ax.set_yticklabels(methods, fontproperties=cjk_font(24))
    ax.set_xlabel("资金量（UNIT）", fontproperties=cjk_font(28, bold=True))
    ax.set_xlim(0, TOTAL_DEPOSIT * 1.12)
    ax.set_ylim(-0.65, 1.65)

    origin_axes(ax, minor_y=False)
    fig.tight_layout()
    return save_figure(fig, out_dir, "fig7b_capital_capacity_v2", formats)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out-dir", type=Path, default=DEFAULT_OUT_DIR)
    parser.add_argument("--formats", default="pdf,png")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    formats = tuple(fmt.strip().lower() for fmt in args.formats.split(",") if fmt.strip())
    outputs = build_capital_capacity_figure(args.out_dir, formats)
    for path in outputs:
        print(f"[saved] {path}")


if __name__ == "__main__":
    main()
