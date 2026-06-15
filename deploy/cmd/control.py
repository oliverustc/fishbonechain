#!/usr/bin/env python3
"""控制节点服务：start / stop / restart。"""
import asyncio
import shlex
import sys
from pathlib import Path

import typer
from rich.console import Console

sys.path.insert(0, str(Path(__file__).parent.parent))
from fishbone.config import load, csv_set, filter_config_to_chains
from fishbone.remote import connect_all
from fishbone.service import service_name

app     = typer.Typer()
console = Console()


def _filtered_cfg_nodes_chains(config: Path, node_filter: str, chain_filter: str):
    cfg = load(config)
    chain_ids = csv_set(chain_filter)
    if chain_ids:
        unknown = sorted(chain_ids - set(cfg.chains))
        if unknown:
            raise typer.BadParameter(f"未知链: {', '.join(unknown)}")
        cfg = filter_config_to_chains(cfg, chain_ids)

    node_ids = csv_set(node_filter)
    nodes = [n for n in cfg.nodes if node_ids is None or n.id in node_ids]
    if node_ids:
        found = {n.id for n in nodes}
        unknown = sorted(node_ids - found)
        if unknown:
            raise typer.BadParameter(f"未知节点: {', '.join(unknown)}")
    return cfg, nodes, list(cfg.chains)


async def _control(action: str, node_filter: str, chain_filter: str, config: Path):
    cfg, nodes, chains = _filtered_cfg_nodes_chains(config, node_filter, chain_filter)

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


async def clean_chain_data(remote, cfg, chain: str, clean_logs: bool = False):
    """清空开发期链数据目录，保留 node-key，避免 bootnodes peer id 漂移。"""
    base_path = f"{cfg.base_dir.rstrip('/')}/{chain}"
    log_path = f"{cfg.log_dir.rstrip('/')}/{chain}.log"
    root = cfg.base_dir.rstrip("/")
    script = f"""
set -eu
base={shlex.quote(base_path)}
root={shlex.quote(root)}
case "$base" in
  "$root"/*) ;;
  *) echo "refuse to clean unsafe path: $base" >&2; exit 2 ;;
esac
mkdir -p "$base"
find "$base" -mindepth 1 -maxdepth 1 ! -name node-key -exec rm -rf -- {{}} +
"""
    if clean_logs:
        script += f"\nrm -f -- {shlex.quote(log_path)}\n"
    await remote.sudo(f"sh -lc {shlex.quote(script)}")


async def _stop_clean(node_filter: str, chain_filter: str, clean_logs: bool, config: Path):
    cfg, nodes, chains = _filtered_cfg_nodes_chains(config, node_filter, chain_filter)

    async with connect_all(nodes, cfg.sudo_pass) as remotes:
        stop_tasks = []
        for n in nodes:
            if n.id not in remotes:
                continue
            remote = remotes[n.id]
            for chain in chains:
                if chain in n.roles:
                    svc = service_name(chain)
                    stop_tasks.append((n.id, chain, remote.sudo(f"systemctl stop {svc}", check=False)))

        stop_results = await asyncio.gather(*[t[2] for t in stop_tasks], return_exceptions=True)
        for (nid, chain, _), res in zip(stop_tasks, stop_results):
            ok = not isinstance(res, Exception) and (not hasattr(res, "returncode") or res.returncode == 0)
            mark = "✓" if ok else "✗"
            console.print(f"  [{nid}] {mark} fishbone-{chain} stop")

        clean_tasks = []
        for n in nodes:
            if n.id not in remotes:
                continue
            remote = remotes[n.id]
            for chain in chains:
                if chain in n.roles:
                    clean_tasks.append((n.id, chain, clean_chain_data(remote, cfg, chain, clean_logs)))

        clean_results = await asyncio.gather(*[t[2] for t in clean_tasks], return_exceptions=True)
        for (nid, chain, _), res in zip(clean_tasks, clean_results):
            ok = not isinstance(res, Exception)
            mark = "✓" if ok else "✗"
            console.print(f"  [{nid}] {mark} fishbone-{chain} clean-data")


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


@app.command("stop-clean")
def stop_clean(
    nodes:  str  = typer.Option("", help="节点列表，逗号分隔（默认全部）"),
    chains: str  = typer.Option("", help="链名称，逗号分隔（默认全部）"),
    logs:   bool = typer.Option(False, help="同时删除对应链日志"),
    config: Path = typer.Option(Path(__file__).parent.parent / "config.toml"),
):
    """停止服务并清空链数据目录；保留 node-key，下一次 deploy 会重新注入 validator keys。"""
    asyncio.run(_stop_clean(nodes, chains, logs, config))


if __name__ == "__main__":
    app()
