#!/usr/bin/env python3
"""
部署 fishbone-node 到所有节点。
包含：传二进制、传 spec、注入密钥、生成 service 文件、启动服务。
"""
import asyncio
import sys
from pathlib import Path

import typer
from rich.console import Console

sys.path.insert(0, str(Path(__file__).parent.parent))
from fishbone.config import load, csv_set, filter_config_to_chains
from fishbone.remote import connect_all, RemoteNode
from fishbone.service import render_service, service_name

app     = typer.Typer()
console = Console()

DEPLOY_DIR = Path(__file__).parent.parent
KEYS_DIR   = DEPLOY_DIR / "keys"
SPECS_DIR  = DEPLOY_DIR / "specs"
BIN_DIR    = DEPLOY_DIR / "bin"


# ── 单节点操作 ─────────────────────────────────────────────────────────────────

async def push_binaries(remote: RemoteNode, cfg, node_id: str):
    """推送该集群所有 binary 变体（按 chains 定义去重）。"""
    needed: set[str] = set()
    for chain in cfg.chains.values():
        needed.add(chain.binary or cfg.binary)

    dest_dir = str(Path(cfg.binary).parent)
    await remote.run(f"mkdir -p {dest_dir}")

    for remote_path in sorted(needed):
        bin_name = Path(remote_path).name
        local = BIN_DIR / bin_name
        if local.exists():
            tmp_path = f"{remote_path}.new"
            await remote.upload(str(local), tmp_path)
            await remote.run(f"chmod +x {tmp_path} && mv -f {tmp_path} {remote_path}")
        else:
            console.print(f"  [{node_id}] [yellow]⚠ 本地未找到 {local}，跳过[/yellow]")

    console.print(f"  [{node_id}] ✓ binaries")


async def push_specs(remote: RemoteNode, cfg, node_id: str):
    spec_dir = f"{cfg.base_dir}/specs"
    await remote.run(f"mkdir -p {spec_dir}")
    for chain in sorted(cfg.chains):
        local = SPECS_DIR / f"{chain}-custom-raw.json"
        if local.exists():
            await remote.upload(str(local), f"{spec_dir}/{chain}-custom-raw.json")
    console.print(f"  [{node_id}] ✓ specs")


async def setup_dirs(remote: RemoteNode, cfg, node_id: str):
    chain_dirs = " ".join(f"{cfg.base_dir}/{chain}" for chain in cfg.chains)
    await remote.run(f"mkdir -p {chain_dirs} {cfg.log_dir}")
    console.print(f"  [{node_id}] ✓ dirs")


async def generate_node_keys(remote: RemoteNode, cfg, node_id: str):
    for chain in cfg.chains:
        key_path = f"{cfg.base_dir}/{chain}/node-key"
        exists = await remote.exists(key_path)
        if not exists:
            bin_path = cfg.chain_binary(chain)
            await remote.run(
                f"{bin_path} key generate-node-key --file {key_path} 2>/dev/null"
            )
    console.print(f"  [{node_id}] ✓ node keys")


async def inject_validator_keys(remote: RemoteNode, cfg, node: object):
    env_file = KEYS_DIR / f"{node.id}.env"
    if not env_file.exists():
        console.print(f"  [{node.id}] [red]✗ 没有 keys/{node.id}.env[/red]")
        return

    env = {}
    for line in env_file.read_text().splitlines():
        line = line.strip()
        if line and not line.startswith("#") and "=" in line:
            k, v = line.split("=", 1)
            env[k] = v.strip('"')

    aura_phrase = env.get("AURA_PHRASE", "")
    gran_phrase = env.get("GRAN_PHRASE", "")

    for chain in node.roles:
        chain_cfg = cfg.chains[chain]
        spec_path = f"{cfg.base_dir}/{chain_cfg.spec}"
        base_path = f"{cfg.base_dir}/{chain}"
        bin_path  = cfg.chain_binary(chain)

        await remote.run(
            f"{bin_path} key insert "
            f"--base-path {base_path} "
            f"--chain {spec_path} "
            f"--scheme sr25519 --key-type aura "
            f'--suri "{aura_phrase}" 2>/dev/null'
        )
        await remote.run(
            f"{bin_path} key insert "
            f"--base-path {base_path} "
            f"--chain {spec_path} "
            f"--scheme ed25519 --key-type gran "
            f'--suri "{gran_phrase}" 2>/dev/null'
        )

    console.print(f"  [{node.id}] ✓ validator keys injected")


async def install_services(remote: RemoteNode, cfg, node: object):
    for chain in node.roles:
        content  = render_service(cfg, node, chain)
        svc_name = service_name(chain)
        svc_path = f"/etc/systemd/system/{svc_name}.service"
        await remote.write_file(svc_path, content, sudo=True)

    await remote.sudo("systemctl daemon-reload")
    console.print(f"  [{node.id}] ✓ services installed")


async def start_services(remote: RemoteNode, cfg, node: object, chains: list[str]):
    for chain in chains:
        if chain in node.roles:
            svc = service_name(chain)
            await remote.sudo(f"systemctl enable --now {svc}", check=False)
    console.print(f"  [{node.id}] ✓ services started")


async def deploy_node(remote: RemoteNode, cfg, node):
    await setup_dirs(remote, cfg, node.id)
    await push_binaries(remote, cfg, node.id)
    await push_specs(remote, cfg, node.id)
    await generate_node_keys(remote, cfg, node.id)
    await inject_validator_keys(remote, cfg, node)
    await install_services(remote, cfg, node)


@app.command()
def deploy(
    only: str = typer.Option("", "--only", "--nodes", help="只部署指定节点，逗号分隔（如 f1,f2）"),
    chains: str = typer.Option("", help="只部署指定链，逗号分隔（如 main,child1,child6）"),
    start: bool = typer.Option(True, help="部署后自动启动服务"),
    config: Path = typer.Option(DEPLOY_DIR / "config.toml", help="配置文件路径"),
):
    """部署 fishbone-node 到所有（或指定）节点。"""
    cfg = load(config)

    chain_ids = csv_set(chains)
    if chain_ids:
        unknown = sorted(chain_ids - set(cfg.chains))
        if unknown:
            raise typer.BadParameter(f"未知链: {', '.join(unknown)}")
        cfg = filter_config_to_chains(cfg, chain_ids)

    target_ids = csv_set(only)
    nodes = [n for n in cfg.nodes if target_ids is None or n.id in target_ids]

    console.print(f"\n[bold cyan]FishboneChain 部署[/bold cyan]")
    console.print(f"目标节点: {', '.join(n.id for n in nodes)}\n")
    console.print(f"目标链: {', '.join(cfg.chains)}\n")

    async def _run():
        async with connect_all(nodes, cfg.sudo_pass) as remotes:
            tasks = [
                deploy_node(remotes[n.id], cfg, n)
                for n in nodes
                if n.id in remotes
            ]
            await asyncio.gather(*tasks)

        if start:
            # 1. 先启动主链，等待出块稳定
            console.print("\n[bold]启动主链服务...[/bold]")
            async with connect_all(nodes, cfg.sudo_pass) as remotes:
                await asyncio.gather(*[
                    start_services(remotes[n.id], cfg, n, ["main"])
                    for n in nodes if n.id in remotes
                ])

            await asyncio.sleep(10)

            # 2. 按子链编号顺序启动（高频链 child5 最后，避免 mempool 压力干扰主链同步）
            child_chains = sorted(c for c in cfg.chains if c != "main")
            console.print(f"[bold]启动子链服务（顺序：{', '.join(child_chains)}）...[/bold]")
            async with connect_all(nodes, cfg.sudo_pass) as remotes:
                await asyncio.gather(*[
                    start_services(remotes[n.id], cfg, n, child_chains)
                    for n in nodes if n.id in remotes
                ])

        console.print("\n[green]✓ 部署完成[/green]")

    asyncio.run(_run())


if __name__ == "__main__":
    app()
