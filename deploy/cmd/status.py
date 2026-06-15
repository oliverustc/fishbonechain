#!/usr/bin/env python3
"""查看所有节点的运行状态和区块高度。"""
import argparse
import asyncio
import json
import shlex
import sys
from pathlib import Path

from rich.console import Console
from rich.table import Table

sys.path.insert(0, str(Path(__file__).parent.parent))
from fishbone.config import load, csv_set, ClusterConfig
from fishbone.remote import connect_all, RemoteNode

console = Console()
CONFIG_PATH = Path(__file__).parent.parent / "config.toml"


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="查看 FishboneChain 节点运行状态。")
    parser.add_argument("--config", default=str(CONFIG_PATH), help="配置文件路径")
    parser.add_argument("--chains", default="", help="链名称，逗号分隔（默认全部）")
    parser.add_argument(
        "--via-gateway",
        action="store_true",
        help="通过 config.toml 中的 gateway 执行 RPC curl（默认本机直连 10.2.2.x）",
    )
    return parser.parse_args(argv)


async def run_local_rpc_curl(ip: str, port: int, payload: str):
    proc = await asyncio.create_subprocess_exec(
        "curl", "-sf", "--max-time", "4",
        "-X", "POST",
        "-H", "Content-Type: application/json",
        "-d", payload,
        f"http://{ip}:{port}",
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE,
    )
    stdout, stderr = await proc.communicate()
    return type("R", (), {
        "returncode": proc.returncode,
        "stdout": stdout.decode(errors="replace"),
        "stderr": stderr.decode(errors="replace"),
    })()


async def rpc_query(ip: str, port: int, method: str, gateway: RemoteNode | None = None) -> dict:
    """查询节点 RPC；默认本机直连，显式传入 gateway 时才远端执行 curl。"""
    payload = json.dumps({"id": 1, "jsonrpc": "2.0", "method": method, "params": []})
    if gateway:
        cmd = (
            f"curl -sf --max-time 4 -X POST "
            f"-H {shlex.quote('Content-Type: application/json')} "
            f"-d {shlex.quote(payload)} "
            f"{shlex.quote(f'http://{ip}:{port}')} 2>/dev/null"
        )
        result = await gateway.run(cmd, check=False)
    else:
        result = await run_local_rpc_curl(ip, port, payload)

    if result.returncode != 0 or not result.stdout.strip():
        return {}
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError:
        return {}


async def get_node_info(
    remote: RemoteNode,
    node,
    cfg: ClusterConfig,
    chains: list[str],
    gateway: RemoteNode | None = None,
):
    info = {"id": node.id, "ip": node.ip}

    for chain in chains:
        if chain not in node.roles:
            info[chain] = {"svc": "—", "block": "—", "peers": "—", "finalized": "—"}
            continue

        port = cfg.chains[chain].rpc_port
        svc  = await remote.service_status(f"fishbone-{chain}")

        header = await rpc_query(node.ip, port, "chain_getHeader", gateway=gateway)
        health = await rpc_query(node.ip, port, "system_health", gateway=gateway)
        fin    = await rpc_query(node.ip, port, "chain_getFinalizedHead", gateway=gateway)

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


def selected_chains(cfg: ClusterConfig, chains_filter: str) -> list[str]:
    chain_ids = csv_set(chains_filter)
    if not chain_ids:
        return list(cfg.chains)
    unknown = sorted(chain_ids - set(cfg.chains))
    if unknown:
        raise SystemExit(f"未知链: {', '.join(unknown)}")
    return [chain for chain in cfg.chains if chain in chain_ids]


def chain_label(chain: str) -> str:
    return "主链" if chain == "main" else chain


async def main(argv: list[str] | None = None):
    args = parse_args(argv)
    cfg = load(args.config)
    chains = selected_chains(cfg, args.chains)

    if args.via_gateway and not cfg.gateway:
        console.print("[red]--via-gateway 需要 config.toml 中的 [gateway] 配置[/red]")
        return

    console.print(f"\n[bold cyan]FishboneChain 节点状态[/bold cyan] — {cfg.name}\n")

    if args.via_gateway:
        gw_node_cfg = type("N", (), {"id": "__gateway__", "ssh": cfg.gateway.ssh, "roles": []})()
        all_nodes = [gw_node_cfg] + cfg.nodes
    else:
        all_nodes = cfg.nodes

    async with connect_all(all_nodes, cfg.sudo_pass) as remotes:
        gateway = None
        if args.via_gateway:
            gateway = remotes.get("__gateway__")
            if not gateway:
                console.print(f"[red]无法连接 gateway {cfg.gateway.ssh}[/red]")
                return

        tasks = [
            get_node_info(remotes[n.id], n, cfg, chains, gateway=gateway)
            for n in cfg.nodes
            if n.id in remotes
        ]
        infos = await asyncio.gather(*tasks)

    table = Table(show_header=True, header_style="bold magenta")
    table.add_column("节点", style="cyan", width=5)
    table.add_column("IP", width=13)
    for chain in chains:
        table.add_column(f"{chain_label(chain)} svc/block/peers", justify="left", width=22)

    for info in infos:
        def cell(chain):
            c = info[chain]
            if c["svc"] == "—":
                return "—"
            return f"{svc_color(c['svc'])} #{c['block']} {c['peers']}p"

        table.add_row(
            info["id"],
            info["ip"],
            *[cell(chain) for chain in chains],
        )

    console.print(table)
    console.print()

    access = f"通过 {cfg.gateway.ip} 访问" if args.via_gateway else "本机直连"
    for chain in chains:
        nodes = cfg.nodes_for_chain(chain)
        if nodes:
            port = cfg.chains[chain].rpc_port
            console.print(
                f"  {chain_label(chain)}: ws://{nodes[0].ip}:{port}  "
                f"[dim]({access})[/dim]"
            )
    console.print()


if __name__ == "__main__":
    asyncio.run(main())
