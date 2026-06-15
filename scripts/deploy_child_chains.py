#!/usr/bin/env python3
"""
部署 child3-child6：推送 binary + spec、生成 node-key、注入密钥、安装 service。

用法：python3 scripts/deploy_child_chains.py [--only f7,f8]
工作目录：fishbonechain 项目根目录
"""
import argparse
import json
import os
import re
import subprocess
import sys
import time
from pathlib import Path

ROOT    = Path(__file__).parent.parent
KEYS    = ROOT / "deploy" / "keys"
SPECS   = ROOT / "deploy" / "specs"
BIN_DIR = ROOT / "deploy" / "bin"

# ── 子链→binary 映射 ──────────────────────────────────────────────────────────
CHAIN_BINARY = {
    "main":   "fishbone-node",
    "child1": "fishbone-node-crowdsource",
    "child2": "fishbone-node-2s",
    "child3": "fishbone-node-10mb",
    "child4": "fishbone-node-crowdsource",
    "child5": "fishbone-node-1s",
    "child6": "fishbone-node-data-trade",
}

# 各节点参与的子链（与 config.toml 一致）
NODE_ROLES = {
    "f1":  ["main", "child1", "child4", "child6"],
    "f2":  ["main", "child1", "child4", "child6"],
    "f3":  ["main", "child1", "child4", "child6"],
    "f4":  ["main", "child2", "child4", "child6"],
    "f5":  ["main", "child2", "child4", "child6"],
    "f6":  ["main", "child2", "child4"],
    "f7":  ["main", "child3", "child4"],
    "f8":  ["main", "child3"],
    "f9":  ["main", "child3"],
    "f10": ["main", "child5"],
    "f11": ["main", "child5"],
    "f12": ["main", "child5"],
}

REMOTE_BASE = "/home/debian/fishbone"
NEW_CHAINS  = ["child3", "child4", "child5", "child6"]


def ssh(node: str, cmd: str, check=True) -> str:
    result = subprocess.run(
        ["ssh", "-o", "StrictHostKeyChecking=no", node, cmd],
        capture_output=True, text=True
    )
    if check and result.returncode != 0:
        raise RuntimeError(f"[{node}] SSH failed: {cmd!r}\n{result.stderr}")
    return result.stdout.strip()


def scp(local: Path, node: str, remote: str) -> None:
    """Copy local file to node:remote, using a temp file + mv for atomic replacement
    (handles the case where the destination binary is currently running)."""
    tmp = remote + ".new"
    subprocess.run(["scp", "-q", str(local), f"{node}:{tmp}"], check=True)
    ssh(node, f"mv -f {tmp} {remote}")


def load_env(node_id: str) -> dict:
    path = KEYS / f"{node_id}.env"
    env = {}
    for line in path.read_text().splitlines():
        line = line.strip()
        if line and not line.startswith("#") and "=" in line:
            k, v = line.split("=", 1)
            env[k] = v.strip('"')
    return env


# ── Step 2：推送新 binary ──────────────────────────────────────────────────────

def push_binaries(node: str) -> None:
    roles = NODE_ROLES[node]
    needed = {CHAIN_BINARY[c] for c in roles}
    remote_bin_dir = f"{REMOTE_BASE}/bin"
    ssh(node, f"mkdir -p {remote_bin_dir}")

    for bin_name in sorted(needed):
        local = BIN_DIR / bin_name
        if not local.exists():
            print(f"  [{node}] ⚠ {bin_name} not found locally, skip")
            continue
        remote_path = f"{remote_bin_dir}/{bin_name}"
        # 只推送比远端新的文件
        remote_mtime = ssh(node, f"stat -c %Y {remote_path} 2>/dev/null || echo 0", check=False)
        local_mtime  = int(local.stat().st_mtime)
        if int(remote_mtime or 0) >= local_mtime:
            print(f"  [{node}] {bin_name} up-to-date, skip")
            continue
        print(f"  [{node}] → {bin_name} ({local.stat().st_size // 1024 // 1024} MB)")
        scp(local, node, remote_path)
        ssh(node, f"chmod +x {remote_path}")

    print(f"  [{node}] ✓ binaries")


# ── Step 3：推送 child3-6 spec ────────────────────────────────────────────────

def push_specs(node: str) -> None:
    remote_spec_dir = f"{REMOTE_BASE}/specs"
    ssh(node, f"mkdir -p {remote_spec_dir}")

    for chain in NEW_CHAINS:
        spec_file = SPECS / f"{chain}-custom-raw.json"
        if spec_file.exists():
            scp(spec_file, node, f"{remote_spec_dir}/{chain}-custom-raw.json")

    print(f"  [{node}] ✓ specs (child3-6)")


# ── Step 4：生成 P2P node-key 并返回 Peer ID ─────────────────────────────────

def ensure_node_keys(node: str) -> dict:
    """为该节点的所有新子链生成 node-key（若已存在则跳过），返回 {chain: peer_id}。"""
    peer_ids = {}
    roles = NODE_ROLES[node]

    for chain in roles:
        if chain not in NEW_CHAINS:
            continue
        bin_name = CHAIN_BINARY[chain]
        remote_binary = f"{REMOTE_BASE}/bin/{bin_name}"
        key_path = f"{REMOTE_BASE}/{chain}/node-key"

        ssh(node, f"mkdir -p {REMOTE_BASE}/{chain}")
        exists = ssh(node, f"test -f {key_path} && echo yes || echo no", check=False)
        if exists.strip() == "yes":
            print(f"  [{node}] {chain} node-key exists")
        else:
            ssh(node, f"{remote_binary} key generate-node-key --file {key_path} 2>/dev/null")
            print(f"  [{node}] {chain} node-key generated")

        # 读出 Peer ID
        peer_id = ssh(node, f"{remote_binary} key inspect-node-key --file {key_path} 2>/dev/null")
        peer_ids[chain] = peer_id.strip()
        print(f"  [{node}] {chain} PeerID: {peer_id.strip()[:20]}…")

    return peer_ids


# ── Step 5：注入 validator 密钥 ───────────────────────────────────────────────

def inject_keys(node: str) -> None:
    env = load_env(node)
    aura_phrase = env["AURA_PHRASE"]
    gran_phrase  = env["GRAN_PHRASE"]
    roles = NODE_ROLES[node]

    for chain in roles:
        if chain not in NEW_CHAINS:
            continue
        bin_name = CHAIN_BINARY[chain]
        remote_binary = f"{REMOTE_BASE}/bin/{bin_name}"
        base_path = f"{REMOTE_BASE}/{chain}"
        spec_path = f"{REMOTE_BASE}/specs/{chain}-custom-raw.json"

        # AURA (sr25519)
        ssh(node,
            f"{remote_binary} key insert "
            f"--base-path {base_path} --chain {spec_path} "
            f"--scheme sr25519 --key-type aura "
            f"--suri '{aura_phrase}' 2>/dev/null"
        )
        # GRANDPA (ed25519)
        ssh(node,
            f"{remote_binary} key insert "
            f"--base-path {base_path} --chain {spec_path} "
            f"--scheme ed25519 --key-type gran "
            f"--suri '{gran_phrase}' 2>/dev/null"
        )
        print(f"  [{node}] {chain} aura+gran keys injected")


# ── Step 7：安装 systemd service ─────────────────────────────────────────────

CHAIN_PORTS = {
    "child3": (30336, 9947, 9618),
    "child4": (30337, 9948, 9619),
    "child5": (30338, 9949, 9620),
    "child6": (30339, 9950, 9621),
}

CHAIN_LABELS = {
    "child3": "Child-3 (Medical, 10MB)",
    "child4": "Child-4 (Financial, 7-val)",
    "child5": "Child-5 (IoT, 1s)",
    "child6": "Child-6 (Data Trade, AURA-5)",
}


def install_service(node: str, chain: str, bootnodes: list[str]) -> None:
    bin_name = CHAIN_BINARY[chain]
    remote_binary = f"{REMOTE_BASE}/bin/{bin_name}"
    p2p_port, rpc_port, prom_port = CHAIN_PORTS[chain]
    label = CHAIN_LABELS[chain]

    bn_args = " \\\n  ".join(f"--bootnodes {b}" for b in bootnodes) if bootnodes else ""

    service = f"""[Unit]
Description=FishboneChain {label} Node
After=network.target
Wants=network.target

[Service]
Type=simple
User=debian
Group=debian
WorkingDirectory={REMOTE_BASE}

ExecStart={remote_binary} \\
  --base-path {REMOTE_BASE}/{chain} \\
  --chain     {REMOTE_BASE}/specs/{chain}-custom-raw.json \\
  --validator \\
  --node-key-file {REMOTE_BASE}/{chain}/node-key \\
  --port      {p2p_port} \\
  --rpc-port  {rpc_port} \\
  --unsafe-rpc-external \\
  --rpc-cors  all \\
  --log       info \\
  --prometheus-external \\
  --prometheus-port {prom_port} \\
  {bn_args}

Restart=always
RestartSec=10
LimitNOFILE=65536

StandardOutput=append:{REMOTE_BASE}/logs/{chain}.log
StandardError=append:{REMOTE_BASE}/logs/{chain}.log

[Install]
WantedBy=multi-user.target
"""
    # 写 service 文件
    escaped = service.replace("'", "'\"'\"'")
    ssh(node, f"mkdir -p {REMOTE_BASE}/logs")
    ssh(node, f"sudo tee /etc/systemd/system/fishbone-{chain}.service > /dev/null << 'SVCEOF'\n{service}\nSVCEOF")
    print(f"  [{node}] ✓ /etc/systemd/system/fishbone-{chain}.service")


# ── 主流程 ────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--only", default="", help="逗号分隔节点列表，如 f7,f8")
    parser.add_argument("--skip-binary", action="store_true", help="跳过 binary 推送")
    parser.add_argument("--skip-service", action="store_true", help="跳过 service 安装")
    parser.add_argument("--start", action="store_true", help="安装后立即启动服务")
    args = parser.parse_args()

    target_nodes = [n.strip() for n in args.only.split(",")] if args.only else sorted(NODE_ROLES)

    print(f"\n=== FishboneChain 子链部署 (child3-child6) ===")
    print(f"目标节点: {', '.join(target_nodes)}\n")

    # Step 4 收集所有节点的 Peer ID
    all_peer_ids: dict[str, dict[str, str]] = {}  # node → {chain → peer_id}

    for node in target_nodes:
        print(f"\n── {node} ──────────────────────────────────────")

        # Step 2
        if not args.skip_binary:
            push_binaries(node)

        # Step 3
        push_specs(node)

        # Step 4
        peer_ids = ensure_node_keys(node)
        all_peer_ids[node] = peer_ids

        # Step 5
        inject_keys(node)

    # Step 6：汇总 Peer ID，构建每条链的 bootnode 列表
    print("\n\n=== Peer ID 汇总 ===")
    chain_bootnodes: dict[str, list[str]] = {c: [] for c in NEW_CHAINS}
    NODE_IPS = {f"f{i}": f"10.2.2.{10+i}" for i in range(1, 13)}

    for node, peer_ids in all_peer_ids.items():
        ip = NODE_IPS[node]
        for chain, peer_id in peer_ids.items():
            if peer_id:
                port = CHAIN_PORTS[chain][0]
                multiaddr = f"/ip4/{ip}/tcp/{port}/p2p/{peer_id}"
                chain_bootnodes[chain].append(multiaddr)
                print(f"  {node}/{chain}: {peer_id[:20]}…")

    # Step 7：安装 service
    if not args.skip_service:
        print("\n\n=== 安装 systemd service ===")
        for node in target_nodes:
            roles = NODE_ROLES[node]
            for chain in NEW_CHAINS:
                if chain not in roles:
                    continue
                # 排除自身 bootnode
                ip = NODE_IPS[node]
                port = CHAIN_PORTS[chain][0]
                bootnodes = [b for b in chain_bootnodes[chain] if f"/ip4/{ip}/tcp/{port}/" not in b]
                install_service(node, chain, bootnodes)

        # daemon-reload
        print("\n  daemon-reload ...")
        for node in target_nodes:
            ssh(node, "sudo systemctl daemon-reload")

    # 打印 Peer ID 表，供 config.toml 填写
    print("\n\n=== 请将以下 Peer ID 填入 deploy/config.toml ===")
    for node, peer_ids in all_peer_ids.items():
        print(f"\n  [{node}]")
        for chain, pid in peer_ids.items():
            print(f"    {chain} = \"{pid}\"")

    if args.start:
        print("\n\n=== 启动子链服务 ===")
        for node in target_nodes:
            roles = NODE_ROLES[node]
            for chain in NEW_CHAINS:
                if chain not in roles:
                    continue
                ssh(node, f"sudo systemctl enable --now fishbone-{chain}", check=False)
                print(f"  [{node}] fishbone-{chain} started")

    print("\n✓ 部署完成")


if __name__ == "__main__":
    main()
