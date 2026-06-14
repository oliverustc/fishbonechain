#!/usr/bin/env python3
"""Render a compact PPT-friendly FishboneChain deployment matrix."""

from __future__ import annotations

import argparse
import html
import subprocess
import tomllib
from pathlib import Path
from shutil import which


ROOT = Path(__file__).resolve().parents[1]
CONFIG = ROOT / "deploy" / "config.toml"
DEFAULT_SVG = ROOT / "docs" / "figures" / "fishbone_topology_workload.svg"
DEFAULT_PNG = ROOT / "docs" / "figures" / "fishbone_topology_workload.png"
TITLE = "FishboneChain"

SCENARIOS = {
    "child1": ("快递配送", "3V"),
    "child2": ("交通感知", "3V"),
    "child3": ("医疗标注", "3V"),
    "child4": ("金融核验", "7V"),
    "child5": ("IoT 传感", "3V"),
    "child6": ("数据市场", "5V"),
}

COLORS = {
    "main": "#183153",
    "child1": "#2F80ED",
    "child2": "#F2994A",
    "child3": "#27AE60",
    "child4": "#9B51E0",
    "child5": "#00A6A6",
    "child6": "#EB5757",
}


def esc(value: object) -> str:
    return html.escape(str(value), quote=True)


def role_nodes(cfg: dict, chain: str) -> list[str]:
    return [node["id"] for node in cfg["nodes"] if chain in node.get("roles", [])]


def svg_text(x: float, y: float, value: object, size: int, weight: int | str = 600,
             fill: str = "#183153", anchor: str = "start") -> str:
    return (
        f'<text x="{x:.1f}" y="{y:.1f}" text-anchor="{anchor}" '
        f'font-size="{size}" font-weight="{weight}" fill="{fill}">{esc(value)}</text>'
    )


def svg_box(x: float, y: float, w: float, h: float, fill: str, stroke: str = "#DDE3EA",
            radius: float = 14, sw: float = 1.2) -> str:
    return (
        f'<rect x="{x:.1f}" y="{y:.1f}" width="{w:.1f}" height="{h:.1f}" '
        f'rx="{radius:.1f}" fill="{fill}" stroke="{stroke}" stroke-width="{sw}"/>'
    )


def deployment_model(cfg: dict) -> dict:
    child_chains = [name for name in cfg["chains"] if name.startswith("child")]
    child_memberships = sum(len(role_nodes(cfg, chain)) for chain in child_chains)
    return {
        "nodes": cfg["nodes"],
        "child_chains": child_chains,
        "child_memberships": child_memberships,
        "summary": f"实验规模：12 台 VM 节点 | 1 条主链 + 6 条业务子链 | {child_memberships} 个子链验证席位 | N=6 吞吐 396 TPS",
    }


def render_svg(cfg: dict, out: Path) -> None:
    model = deployment_model(cfg)
    nodes = model["nodes"]
    child_chains = model["child_chains"]
    width, height = 1600, 760

    left_x = 64
    matrix_x = 520
    matrix_y = 300
    col_gap = 76
    row_gap = 67
    node_centers = {node["id"]: matrix_x + i * col_gap for i, node in enumerate(nodes)}

    parts = [
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" viewBox="0 0 {width} {height}">',
        "<defs>",
        "<style>text{font-family:'Noto Sans CJK SC','Microsoft YaHei','PingFang SC','Arial',sans-serif;}</style>",
        "</defs>",
        f'<rect width="{width}" height="{height}" fill="#F8FAFC"/>',
        svg_text(64, 78, TITLE, 48, 850),
    ]

    parts.append(svg_text(64, 128, model["summary"], 31, 700, "#334155"))

    parts.append(svg_box(64, 170, 394, 500, "#FFFFFF"))
    parts.append(svg_text(96, 216, "子链场景", 31, 850))
    for idx, chain in enumerate(child_chains):
        y = matrix_y + idx * row_gap
        scenario, validators = SCENARIOS[chain]
        color = COLORS[chain]
        parts.append(svg_box(96, y - 29, 94, 42, color, color, 13))
        parts.append(svg_text(143, y, f"C{chain[-1]}", 24, 850, "#FFFFFF", "middle"))
        parts.append(svg_text(214, y - 3, scenario, 27, 800))
        parts.append(svg_text(385, y - 3, validators, 25, 850, color, "end"))

    parts.append(svg_box(486, 170, 970, 500, "#FFFFFF"))
    parts.append(svg_text(522, 218, "验证人分布", 31, 850))

    main_y = 250
    x1, x2 = node_centers[nodes[0]["id"]], node_centers[nodes[-1]["id"]]
    parts.append(f'<line x1="{x1:.1f}" y1="{main_y:.1f}" x2="{x2:.1f}" y2="{main_y:.1f}" stroke="{COLORS["main"]}" stroke-width="14" stroke-linecap="round"/>')
    for node in nodes:
        x = node_centers[node["id"]]
        parts.append(f'<circle cx="{x:.1f}" cy="{main_y:.1f}" r="15" fill="#FFFFFF" stroke="{COLORS["main"]}" stroke-width="7"/>')
        parts.append(svg_text(x, 715, node["id"].upper(), 27, 850, "#183153", "middle"))

    for idx, chain in enumerate(child_chains):
        y = matrix_y + idx * row_gap
        members = role_nodes(cfg, chain)
        color = COLORS[chain]
        parts.append(f'<line x1="{min(node_centers[m] for m in members):.1f}" y1="{y:.1f}" x2="{max(node_centers[m] for m in members):.1f}" y2="{y:.1f}" stroke="{color}" stroke-width="10" stroke-linecap="round" opacity="0.88"/>')
        for node in nodes:
            x = node_centers[node["id"]]
            if node["id"] in members:
                parts.append(f'<circle cx="{x:.1f}" cy="{y:.1f}" r="18" fill="#FFFFFF" stroke="{color}" stroke-width="8"/>')
            else:
                parts.append(f'<circle cx="{x:.1f}" cy="{y:.1f}" r="6" fill="#CBD5E1"/>')

    parts.append("</svg>")
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text("\n".join(parts), encoding="utf-8")


def render_png(cfg: dict, out: Path) -> None:
    import matplotlib

    matplotlib.use("Agg")
    import matplotlib.pyplot as plt
    from matplotlib.patches import Circle, FancyBboxPatch

    plt.rcParams["font.sans-serif"] = ["WenQuanYi Micro Hei", "Noto Sans CJK SC", "DejaVu Sans"]
    plt.rcParams["axes.unicode_minus"] = False

    model = deployment_model(cfg)
    nodes = model["nodes"]
    child_chains = model["child_chains"]
    fig, ax = plt.subplots(figsize=(16, 7.6), dpi=160)
    fig.subplots_adjust(left=0, right=1, bottom=0, top=1)
    ax.set_xlim(0, 1600)
    ax.set_ylim(760, 0)
    ax.axis("off")

    def box(x, y, w, h, fc="#FFFFFF", ec="#DDE3EA", radius=14, lw=1.2, z=1):
        ax.add_patch(FancyBboxPatch((x, y), w, h, boxstyle=f"round,pad=0,rounding_size={radius}",
                                    facecolor=fc, edgecolor=ec, linewidth=lw, zorder=z))

    def label(x, y, value, size=28, weight="bold", color="#183153", ha="left", z=4):
        ax.text(x, y, value, fontsize=size, fontweight=weight, color=color, ha=ha, va="center", zorder=z)

    ax.add_patch(plt.Rectangle((0, 0), 1600, 760, color="#F8FAFC", zorder=0))
    label(64, 64, TITLE, 39)

    label(64, 126, model["summary"], 24, "bold", "#334155")

    left_x, matrix_x, matrix_y = 64, 520, 300
    col_gap, row_gap = 76, 67
    node_centers = {node["id"]: matrix_x + i * col_gap for i, node in enumerate(nodes)}

    box(left_x, 170, 394, 500)
    label(96, 212, "子链场景", 25)
    for idx, chain in enumerate(child_chains):
        y = matrix_y + idx * row_gap
        scenario, validators = SCENARIOS[chain]
        color = COLORS[chain]
        box(96, y - 29, 94, 42, color, color, 13)
        label(143, y - 1, f"C{chain[-1]}", 20, "bold", "#FFFFFF", "center")
        label(214, y - 3, scenario, 22)
        label(385, y - 3, validators, 21, "bold", color, "right")

    box(486, 170, 970, 500)
    label(522, 212, "验证人分布", 25)

    main_y = 250
    x1, x2 = node_centers[nodes[0]["id"]], node_centers[nodes[-1]["id"]]
    ax.plot([x1, x2], [main_y, main_y], color=COLORS["main"], lw=14, solid_capstyle="round", zorder=2)
    for node in nodes:
        x = node_centers[node["id"]]
        ax.add_patch(Circle((x, main_y), 15, fc="#FFFFFF", ec=COLORS["main"], lw=6, zorder=4))
        label(x, 715, node["id"].upper(), 22, "bold", "#183153", "center")

    for idx, chain in enumerate(child_chains):
        y = matrix_y + idx * row_gap
        members = role_nodes(cfg, chain)
        color = COLORS[chain]
        ax.plot([min(node_centers[m] for m in members), max(node_centers[m] for m in members)],
                [y, y], color=color, lw=10, solid_capstyle="round", alpha=0.88, zorder=2)
        for node in nodes:
            x = node_centers[node["id"]]
            if node["id"] in members:
                ax.add_patch(Circle((x, y), 18, fc="#FFFFFF", ec=color, lw=7, zorder=4))
            else:
                ax.add_patch(Circle((x, y), 6, fc="#CBD5E1", ec="none", zorder=3))

    out.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(out, facecolor="#F8FAFC")
    plt.close(fig)


def convert_png(svg_path: Path, png_path: Path) -> bool:
    tools = [
        ("rsvg-convert", ["rsvg-convert", "-w", "1600", "-h", "760", str(svg_path), "-o", str(png_path)]),
        ("magick", ["magick", "-background", "white", "-density", "160", str(svg_path), str(png_path)]),
        ("convert", ["convert", "-background", "white", "-density", "160", str(svg_path), str(png_path)]),
        ("cairosvg", ["cairosvg", str(svg_path), "-o", str(png_path), "-W", "1600", "-H", "760"]),
    ]
    for executable, command in tools:
        if which(executable):
            subprocess.run(command, check=True)
            return True
    return False


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--svg", type=Path, default=DEFAULT_SVG)
    parser.add_argument("--png", type=Path, default=DEFAULT_PNG)
    parser.add_argument("--no-png", action="store_true")
    args = parser.parse_args()

    cfg = tomllib.loads(CONFIG.read_text(encoding="utf-8"))
    render_svg(cfg, args.svg)

    png_status = "skipped"
    if not args.no_png:
        if convert_png(args.svg, args.png):
            png_status = "written"
        else:
            render_png(cfg, args.png)
            png_status = "written by matplotlib fallback"

    print(f"SVG: {args.svg}")
    print(f"PNG: {args.png} ({png_status})")


if __name__ == "__main__":
    main()
