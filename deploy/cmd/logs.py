#!/usr/bin/env python3
"""实时聚合多节点日志，带颜色区分。"""
import asyncio
import sys
from pathlib import Path
from datetime import datetime

import typer
from rich.console import Console

sys.path.insert(0, str(Path(__file__).parent.parent))
from fishbone.config import load
from fishbone.remote import RemoteNode, connect_all

app     = typer.Typer()
console = Console()

NODE_COLORS = {
    "f1": "cyan", "f2": "green", "f3": "yellow",
    "f4": "magenta", "f5": "blue", "f6": "red",
}


async def stream_log(remote: RemoteNode, node_id: str, log_path: str, stop_event: asyncio.Event):
    color = NODE_COLORS.get(node_id, "white")
    prefix = f"[{color}][{node_id}][/{color}]"

    # 先打印最后 20 行，再 tail -f
    async with remote._conn.create_process(f"tail -n 20 -f {log_path}") as proc:
        async for line in proc.stdout:
            line = line.rstrip()
            if line:
                console.print(f"{prefix} {line}")
            if stop_event.is_set():
                break


@app.command()
def logs(
    chain: str = typer.Argument("main", help="链名称: main / child1 / child2"),
    nodes: str = typer.Option("", help="节点列表，逗号分隔（默认全部）"),
    lines: int = typer.Option(20, help="历史行数"),
    config: Path = typer.Option(
        Path(__file__).parent.parent / "config.toml", help="配置文件路径"
    ),
):
    """实时聚合所有节点的链日志（Ctrl+C 退出）。"""
    cfg = load(config)
    target_ids = {n.strip() for n in nodes.split(",")} if nodes else None
    target_nodes = [
        n for n in cfg.nodes_for_chain(chain)
        if target_ids is None or n.id in target_ids
    ]

    log_path_template = f"{cfg.log_dir}/{chain}.log"
    console.print(f"\n[bold]实时日志 — {chain}链[/bold]  (Ctrl+C 退出)\n")

    stop_event = asyncio.Event()

    async def _run():
        async with connect_all(target_nodes, cfg.sudo_pass) as remotes:
            tasks = [
                stream_log(remotes[n.id], n.id, log_path_template, stop_event)
                for n in target_nodes
                if n.id in remotes
            ]
            try:
                await asyncio.gather(*tasks)
            except asyncio.CancelledError:
                pass

    try:
        asyncio.run(_run())
    except KeyboardInterrupt:
        console.print("\n[dim]已退出[/dim]")


@app.command()
def journal(
    chain: str = typer.Argument("main"),
    node_id: str = typer.Argument("f1"),
    lines: int = typer.Option(50),
    config: Path = typer.Option(Path(__file__).parent.parent / "config.toml"),
):
    """查看指定节点 systemd journal 日志。"""
    cfg = load(config)
    node = cfg.node(node_id)
    if not node:
        console.print(f"[red]节点 {node_id} 不存在[/red]")
        raise typer.Exit(1)

    async def _run():
        async with connect_all([node], cfg.sudo_pass) as remotes:
            remote = remotes[node_id]
            out = await remote.journal(f"fishbone-{chain}", lines)
            console.print(out)

    asyncio.run(_run())


if __name__ == "__main__":
    app()
