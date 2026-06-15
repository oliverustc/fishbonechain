#!/usr/bin/env python3
from __future__ import annotations

import argparse
import shlex
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "deploy"))

from fishbone.config import csv_set, filter_config_to_chains, load  # noqa: E402


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Scan FishboneChain VM deployment state.")
    parser.add_argument("--config", default=str(ROOT / "deploy" / "config.toml"))
    parser.add_argument("--nodes", "--only", dest="nodes", default="")
    parser.add_argument("--chains", default="")
    parser.add_argument("--connect-timeout", type=int, default=5)
    return parser.parse_args()


def selected_config(config_path: str, nodes_filter: str, chains_filter: str):
    cfg = load(config_path)
    chain_ids = csv_set(chains_filter)
    if chain_ids:
        unknown = sorted(chain_ids - set(cfg.chains))
        if unknown:
            raise SystemExit(f"unknown chain(s): {', '.join(unknown)}")
        cfg = filter_config_to_chains(cfg, chain_ids)

    node_ids = csv_set(nodes_filter)
    nodes = [node for node in cfg.nodes if node_ids is None or node.id in node_ids]
    if node_ids:
        found = {node.id for node in nodes}
        unknown = sorted(node_ids - found)
        if unknown:
            raise SystemExit(f"unknown node(s): {', '.join(unknown)}")
    return cfg, nodes


def shell_words(values: list[str]) -> str:
    return " ".join(shlex.quote(value) for value in values)


def remote_script(cfg, node) -> str:
    chains = [chain for chain in node.roles if chain in cfg.chains]
    services = [f"fishbone-{chain}" for chain in chains]
    ports: list[int] = []
    for chain in chains:
        chain_cfg = cfg.chains[chain]
        ports.extend([chain_cfg.p2p_port, chain_cfg.rpc_port, chain_cfg.prom_port])

    binary_paths = sorted({cfg.chain_binary(chain) for chain in chains})
    spec_paths = sorted({f"{cfg.base_dir.rstrip('/')}/{cfg.chains[chain].spec}" for chain in chains})
    port_pattern = f":({'|'.join(str(port) for port in sorted(set(ports)))})" if ports else ":$^"

    return f"""
set +e
echo "host=$(hostname) date=$(date +%F_%T)"
echo "[services]"
for svc in {shell_words(services)}; do
  state=$(systemctl is-active "$svc" 2>/dev/null || true)
  enabled=$(systemctl is-enabled "$svc" 2>/dev/null || true)
  printf "%s %s %s\\n" "$svc" "${{state:-unknown}}" "${{enabled:-unknown}}"
done
echo "[processes]"
pgrep -af 'fishbone-node|node .*bridge|node .*worker|node .*metrics|fishbone-monitor' 2>/dev/null | grep -v 'pgrep -af' || true
echo "[listeners]"
ss -ltnp 2>/dev/null | grep -E {shlex.quote(port_pattern)} || true
echo "[data-dirs]"
for chain in {shell_words(chains)}; do
  dir={shlex.quote(cfg.base_dir.rstrip('/'))}/$chain
  if [ -e "$dir" ]; then
    du -sh "$dir" 2>/dev/null
  else
    echo "missing $dir"
  fi
done
echo "[binaries]"
for path in {shell_words(binary_paths)}; do
  if [ -e "$path" ]; then
    stat -c "%n %s %y" "$path" 2>/dev/null
  else
    echo "missing $path"
  fi
done
echo "[specs]"
for path in {shell_words(spec_paths)}; do
  if [ -e "$path" ]; then
    stat -c "%n %s %y" "$path" 2>/dev/null
  else
    echo "missing $path"
  fi
done
echo "[temp-smoke]"
find /tmp -maxdepth 1 -name 'fishbone-smoke-*' -print 2>/dev/null | sort
"""


def scan_node(cfg, node, timeout: int) -> tuple[int, str]:
    remote_cmd = f"sh -lc {shlex.quote(remote_script(cfg, node))}"
    cmd = [
        "ssh",
        "-o",
        "BatchMode=yes",
        "-o",
        f"ConnectTimeout={timeout}",
        node.ssh,
        remote_cmd,
    ]
    proc = subprocess.run(cmd, text=True, capture_output=True, timeout=timeout + 20)
    output = proc.stdout
    if proc.stderr:
        output += proc.stderr
    return proc.returncode, output


def main() -> int:
    args = parse_args()
    cfg, nodes = selected_config(args.config, args.nodes, args.chains)

    for node in nodes:
        print(f"===== {node.id} ({node.ssh}) =====")
        try:
            code, output = scan_node(cfg, node, args.connect_timeout)
        except subprocess.TimeoutExpired:
            print("ERROR timeout")
            continue
        if code != 0:
            print(f"ERROR rc={code}")
        print(output.rstrip())
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
