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

_CHAIN_LABELS: dict[str, str] = {
    "main":   "Main Chain",
    "child1": "Child-1 (Delivery Crowdsource)",
    "child2": "Child-2 (Traffic Sensing, AURA)",
    "child3": "Child-3 (Medical Annotation, AURA)",
    "child4": "Child-4 (Financial Verification, 7-val)",
    "child5": "Child-5 (IoT Sensor Network, AURA)",
    "child6": "Child-6 (Data Market, AURA-5)",
}


def render_service(
    cfg: ClusterConfig,
    node: NodeConfig,
    chain: str,
) -> str:
    """为 node 在 chain 上渲染 systemd service 内容。"""
    chain_cfg = cfg.chains[chain]
    bootnodes = cfg.bootnodes(chain)

    if bootnodes:
        bootnodes_args = " \\\n  ".join(f"--bootnodes {b}" for b in bootnodes)
    else:
        bootnodes_args = ""

    chain_label = _CHAIN_LABELS.get(chain, chain)
    binary = cfg.chain_binary(chain)

    return SERVICE_TEMPLATE.format(
        chain_label=chain_label,
        chain_name=chain,
        binary=binary,
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
