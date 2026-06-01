#!/usr/bin/env python3
"""控制节点服务：start / stop / restart。"""
import asyncio
import sys
from pathlib import Path

import typer
from rich.console import Console

sys.path.insert(0, str(Path(__file__).parent.parent))
from fishbone.config import load
from fishbone.remote import connect_all
from fishbone.service import service_name

app     = typer.Typer()
console = Console()


def _node_chain_pairs(cfg, node_filter: str, chain_filter: str):
    node_ids = {n.strip() for n in node_filter.split(",")} if node_filter else None
    chains   = [c.strip() for c in chain_filter.split(",")] if chain_filter else list(cfg.chains)
    nodes    = [n for n in cfg.nodes if node_ids is None or n.id in node_ids]
    return nodes, chains


async def _control(action: str, node_filter: str, chain_filter: str, config: Path):
    cfg = load(config)
    nodes, chains = _node_chain_pairs(cfg, node_filter, chain_filter)

    async with connect_all(nodes, cfg.sudo_pass) as remotes:
        tasks = []
        for n in nodes:
            if n.id not in remotes:
                continue
            remote = remotes[n.id]
            for chain in chains:
                if chain in n.roles:
                    svc = service_name(chain)
                    tasks.append((n.id, chain, remote.sudo(f"systemctl {action} {svc}", check=False)))

        results = await asyncio.gather(*[t[2] for t in tasks], return_exceptions=True)
        for (nid, chain, _), res in zip(tasks, results):
            ok = not isinstance(res, Exception) and (not hasattr(res, "returncode") or res.returncode == 0)
            mark = "✓" if ok else "✗"
            console.print(f"  [{nid}] {mark} fishbone-{chain} {action}")


@app.command()
def start(
    nodes:  str  = typer.Option("", help="节点列表，逗号分隔（默认全部）"),
    chains: str  = typer.Option("", help="链名称，逗号分隔（默认全部）"),
    config: Path = typer.Option(Path(__file__).parent.parent / "config.toml"),
):
    """启动节点服务。"""
    asyncio.run(_control("start", nodes, chains, config))


@app.command()
def stop(
    nodes:  str  = typer.Option("", help="节点列表"),
    chains: str  = typer.Option("", help="链名称"),
    config: Path = typer.Option(Path(__file__).parent.parent / "config.toml"),
):
    """停止节点服务。"""
    asyncio.run(_control("stop", nodes, chains, config))


@app.command()
def restart(
    nodes:  str  = typer.Option("", help="节点列表"),
    chains: str  = typer.Option("", help="链名称"),
    config: Path = typer.Option(Path(__file__).parent.parent / "config.toml"),
):
    """重启节点服务。"""
    asyncio.run(_control("restart", nodes, chains, config))


if __name__ == "__main__":
    app()
