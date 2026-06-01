#!/usr/bin/env python3
"""查看所有节点的运行状态和区块高度。"""
import asyncio
import json
import sys
from pathlib import Path

from rich.console import Console
from rich.table import Table

sys.path.insert(0, str(Path(__file__).parent.parent))
from fishbone.config import load, ClusterConfig
from fishbone.remote import connect_all, RemoteNode

console = Console()
CONFIG_PATH = Path(__file__).parent.parent / "config.toml"


async def rpc_via_gateway(gw: RemoteNode, ip: str, port: int, method: str) -> dict:
    """在跳板机上执行 curl，查询远程节点 RPC。"""
    payload = json.dumps({"id": 1, "jsonrpc": "2.0", "method": method, "params": []})
    cmd = (
        f"curl -sf --max-time 4 -X POST "
        f"-H 'Content-Type: application/json' "
        f"-d '{payload}' "
        f"http://{ip}:{port} 2>/dev/null"
    )
    result = await gw.run(cmd, check=False)
    if result.returncode != 0 or not result.stdout.strip():
        return {}
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError:
        return {}


async def get_node_info(gw: RemoteNode, remote: RemoteNode, node, cfg: ClusterConfig):
    info = {"id": node.id, "ip": node.ip}

    for chain in ["main", "child1", "child2"]:
        if chain not in node.roles:
            info[chain] = {"svc": "—", "block": "—", "peers": "—", "finalized": "—"}
            continue

        port = cfg.chains[chain].rpc_port
        svc  = await remote.service_status(f"fishbone-{chain}")

        header = await rpc_via_gateway(gw, node.ip, port, "chain_getHeader")
        health = await rpc_via_gateway(gw, node.ip, port, "system_health")
        fin    = await rpc_via_gateway(gw, node.ip, port, "chain_getFinalizedHead")

        block = "×"
        finalized = "×"
        peers = "×"

        if header.get("result"):
            block = str(int(header["result"]["number"], 16))
        if health.get("result"):
            peers = str(health["result"]["peers"])
        if fin.get("result"):
            # chain_getFinalizedHead 返回 hash，再用 chain_getBlock 或从 Idle log 读
            finalized = fin["result"][:8] + "…"

        info[chain] = {"svc": svc, "block": block, "peers": peers, "finalized": finalized}

    return info


def svc_color(svc: str) -> str:
    if svc == "active":
        return f"[green]{svc}[/green]"
    if svc == "—":
        return "—"
    return f"[red]{svc}[/red]"


async def main():
    cfg = load(CONFIG_PATH)

    if not cfg.gateway:
        console.print("[red]config.toml 缺少 [gateway] 配置[/red]")
        return

    console.print(f"\n[bold cyan]FishboneChain 节点状态[/bold cyan] — {cfg.name}\n")

    # 同时连接跳板机和所有节点
    gw_node_cfg = type("N", (), {"id": "bcg", "ssh": cfg.gateway.ssh, "roles": []})()
    all_nodes   = [gw_node_cfg] + cfg.nodes

    async with connect_all(all_nodes, cfg.sudo_pass) as remotes:
        gw = remotes.get("bcg")
        if not gw:
            console.print("[red]无法连接跳板机 bcg[/red]")
            return

        tasks = [
            get_node_info(gw, remotes[n.id], n, cfg)
            for n in cfg.nodes
            if n.id in remotes
        ]
        infos = await asyncio.gather(*tasks)

    table = Table(show_header=True, header_style="bold magenta")
    table.add_column("节点", style="cyan", width=5)
    table.add_column("IP", width=13)
    table.add_column("主链 svc/block/peers", justify="left", width=22)
    table.add_column("子链1 svc/block", justify="left", width=18)
    table.add_column("子链2 svc/block", justify="left", width=18)

    for info in infos:
        def cell(chain):
            c = info[chain]
            if c["svc"] == "—":
                return "—"
            return f"{svc_color(c['svc'])} #{c['block']} {c['peers']}p"

        table.add_row(
            info["id"],
            info["ip"],
            cell("main"),
            cell("child1"),
            cell("child2"),
        )

    console.print(table)
    console.print()

    gw_ip = cfg.gateway.ip
    for chain, label in [("main", "主链"), ("child1", "子链1"), ("child2", "子链2")]:
        nodes = cfg.nodes_for_chain(chain)
        if nodes:
            port = cfg.chains[chain].rpc_port
            console.print(
                f"  {label}: ws://{nodes[0].ip}:{port}  "
                f"[dim](通过 {gw_ip} 访问)[/dim]"
            )
    console.print()


if __name__ == "__main__":
    asyncio.run(main())
