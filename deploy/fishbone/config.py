"""读取 config.toml，提供类型化的配置对象。"""
from __future__ import annotations
import tomllib
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional


@dataclass
class ChainConfig:
    name:     str           # "main" / "child1" / ... / "child6"
    id:       str           # chain spec id
    spec:     str           # 相对于 base_dir 的 spec 路径
    p2p_port: int
    rpc_port: int
    prom_port: int
    binary:   Optional[str] = None  # 若设置则覆盖 cluster.binary


@dataclass
class NodePeerIds:
    main:   str = ""
    child1: str = ""
    child2: str = ""
    child3: str = ""
    child4: str = ""
    child5: str = ""
    child6: str = ""


@dataclass
class NodeConfig:
    id:       str          # "f1" ~ "f12"
    ip:       str          # "10.2.2.11"
    ssh:      str          # SSH host alias
    roles:    list[str]    # ["main", "child1", ...]
    peer_ids: NodePeerIds = field(default_factory=NodePeerIds)


@dataclass
class GatewayConfig:
    ssh: str   # SSH alias for the jump host
    ip:  str   # IP of the jump host


@dataclass
class ClusterConfig:
    name:      str
    binary:    str
    base_dir:  str
    log_dir:   str
    sudo_pass: str
    chains:    dict[str, ChainConfig]
    nodes:     list[NodeConfig]
    gateway:   Optional[GatewayConfig] = None

    def node(self, node_id: str) -> Optional[NodeConfig]:
        return next((n for n in self.nodes if n.id == node_id), None)

    def nodes_for_chain(self, chain: str) -> list[NodeConfig]:
        return [n for n in self.nodes if chain in n.roles]

    def chain_binary(self, chain: str) -> str:
        """返回该链应使用的 binary 路径（优先 per-chain 覆盖，否则 cluster 全局）。"""
        c = self.chains.get(chain)
        if c and c.binary:
            return c.binary
        return self.binary

    def bootnodes(self, chain: str) -> list[str]:
        """返回该链所有节点的 bootnode multiaddr 列表。"""
        c = self.chains[chain]
        result = []
        for n in self.nodes_for_chain(chain):
            pid = getattr(n.peer_ids, chain, "")
            if pid:
                result.append(f"/ip4/{n.ip}/tcp/{c.p2p_port}/p2p/{pid}")
        return result


def load(config_path: str | Path = "config.toml") -> ClusterConfig:
    path = Path(config_path)
    with open(path, "rb") as f:
        raw = tomllib.load(f)

    chains = {
        name: ChainConfig(name=name, **cfg)
        for name, cfg in raw["chains"].items()
    }

    known_fields = set(NodePeerIds.__dataclass_fields__)
    nodes = []
    for n in raw["nodes"]:
        peer_ids_raw = n.pop("peer_ids", {})
        peer_ids = NodePeerIds(**{k: v for k, v in peer_ids_raw.items() if k in known_fields})
        nodes.append(NodeConfig(**n, peer_ids=peer_ids))

    cluster = raw["cluster"]
    gw_raw  = raw.get("gateway")
    gateway = GatewayConfig(**gw_raw) if gw_raw else None

    return ClusterConfig(
        name=cluster["name"],
        binary=cluster["binary"],
        base_dir=cluster["base_dir"],
        log_dir=cluster["log_dir"],
        sudo_pass=cluster["sudo_pass"],
        chains=chains,
        nodes=nodes,
        gateway=gateway,
    )
