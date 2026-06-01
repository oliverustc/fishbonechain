"""生成 systemd service 文件内容（纯函数，无 IO）。"""
from __future__ import annotations
from .config import ClusterConfig, NodeConfig


SERVICE_TEMPLATE = """\
[Unit]
Description=FishboneChain {chain_label} Node
After=network.target
Wants=network.target

[Service]
Type=simple
User=debian
Group=debian
WorkingDirectory={base_dir}

ExecStart={binary} \\
  --base-path {base_dir}/{chain_name} \\
  --chain     {base_dir}/{spec} \\
  --validator \\
  --node-key-file {base_dir}/{chain_name}/node-key \\
  --port      {p2p_port} \\
  --rpc-port  {rpc_port} \\
  --unsafe-rpc-external \\
  --rpc-cors  all \\
  --log       info \\
  --prometheus-external \\
  --prometheus-port {prom_port} \\
  {bootnodes_args}

Restart=always
RestartSec=10
LimitNOFILE=65536

StandardOutput=append:{log_dir}/{chain_name}.log
StandardError=append:{log_dir}/{chain_name}.log

[Install]
WantedBy=multi-user.target
"""


def render_service(
    cfg: ClusterConfig,
    node: NodeConfig,
    chain: str,
) -> str:
    """为 node 在 chain 上渲染 systemd service 内容。"""
    chain_cfg = cfg.chains[chain]
    bootnodes = cfg.bootnodes(chain)

    # 每条 bootnode 单独一行，用 \ 续行
    if bootnodes:
        bn_lines = " \\\n  ".join(f"--bootnodes {b}" for b in bootnodes)
        bootnodes_args = bn_lines
    else:
        bootnodes_args = ""

    chain_label = {
        "main":   "Main Chain",
        "child1": "Child Chain 1",
        "child2": "Child Chain 2",
    }.get(chain, chain)

    return SERVICE_TEMPLATE.format(
        chain_label=chain_label,
        chain_name=chain,
        binary=cfg.binary,
        base_dir=cfg.base_dir,
        spec=chain_cfg.spec,
        p2p_port=chain_cfg.p2p_port,
        rpc_port=chain_cfg.rpc_port,
        prom_port=chain_cfg.prom_port,
        log_dir=cfg.log_dir,
        bootnodes_args=bootnodes_args,
    )


def service_name(chain: str) -> str:
    return f"fishbone-{chain}"
