#!/usr/bin/env python3
"""Render the cross-scenario isolation comparison figure with Origin style."""

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
DEFAULT_SUMMARY = ROOT / "docs" / "experiments" / "figures" / "data" / "exp_isolation_summary.csv"
DEFAULT_OUT_DIR = ROOT / "docs" / "experiments" / "figures"

BAR_BLUE = "#2374d7"
RED = "#ff0000"
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


def origin_legend(
    ax,
    *,
    loc: str = "upper left",
    ncol: int = 1,
    fontsize: int = 23,
    bbox_to_anchor: tuple[float, float] | None = None,
) -> None:
    legend = ax.legend(
        loc=loc,
        ncol=ncol,
        bbox_to_anchor=bbox_to_anchor,
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


def build_isolation_series(rows: list[dict[str, str]]) -> dict[str, list]:
    labels = [
        row.get("scenario_name", "").strip()
        or row.get("scenario", "").strip()
        or str(i + 1)
        for i, row in enumerate(rows)
    ]
    single = [as_float(row, "single_chain_success_rate") for row in rows]
    dedicated = [as_float(row, "dedicated_chain_success_rate") for row in rows]
    gains = [
        as_float(row, "improvement_x")
        or (dedicated[i] / single[i] if single[i] > 0 else 0.0)
        for i, row in enumerate(rows)
    ]
    return {
        "labels": labels,
        "single": single,
        "dedicated": dedicated,
        "gains": gains,
        "gain_labels": [f"{gain:.2f}x" for gain in gains],
    }


def build_isolation_figure(
    summary: Path = DEFAULT_SUMMARY,
    out_dir: Path = DEFAULT_OUT_DIR,
    formats: tuple[str, ...] = ("pdf", "png"),
) -> list[Path]:
    apply_origin_style()
    rows = read_rows(summary)
    if not rows:
        print("[warn] 跨场景隔离图：无数据")
        return []

    series = build_isolation_series(rows)
    labels = series["labels"]
    single = np.array(series["single"], dtype=float)
    dedicated = np.array(series["dedicated"], dtype=float)
    gain_labels = series["gain_labels"]

    x = np.arange(len(labels), dtype=float)
    width = 0.34

    fig, ax = plt.subplots(figsize=(8.4, 5.9))

    ax.bar(x - width / 2, single, width, color=BAR_BLUE, label="单链混跑")
    ax.bar(x + width / 2, dedicated, width, color=RED, label="多子链隔离")

    for i, value in enumerate(dedicated):
        ax.text(i + width / 2, value + 2.5, f"{value:.0f}%", fontsize=19, ha="center")
    for i, value in enumerate(single):
        ax.text(i - width / 2, value + 2.5, f"{value:.1f}%", fontsize=17, ha="center")
    for i, label in enumerate(gain_labels):
        ax.text(
            i,
            122,
            label,
            fontsize=17,
            color=RED,
            ha="center",
            va="center",
            fontproperties=cjk_font(17, bold=True),
        )

    ax.set_xlabel("应用场景", fontproperties=cjk_font(28, bold=True))
    ax.set_ylabel("提交成功率（%）", fontproperties=cjk_font(28, bold=True))
    ax.set_xticks(x)
    ax.set_xticklabels(labels, fontproperties=cjk_font(20))
    ax.set_ylim(0, 132)

    origin_axes(ax)
    origin_legend(ax, loc="upper center", ncol=2, fontsize=15, bbox_to_anchor=(0.5, 1.16))
    fig.tight_layout(rect=(0, 0, 1, 0.95))
    return save_figure(fig, out_dir, "fig_isolation_comparison_v2", formats)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--summary", type=Path, default=DEFAULT_SUMMARY)
    parser.add_argument("--out-dir", type=Path, default=DEFAULT_OUT_DIR)
    parser.add_argument("--formats", default="pdf,png")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    formats = tuple(fmt.strip().lower() for fmt in args.formats.split(",") if fmt.strip())
    outputs = build_isolation_figure(args.summary, args.out_dir, formats)
    for path in outputs:
        print(f"[saved] {path}")


if __name__ == "__main__":
    main()
