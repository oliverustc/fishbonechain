#!/usr/bin/env python3
"""
生成 child3-child6 chain spec（human-readable → 注入真实 validator 密钥 → raw）。

用法：python3 scripts/gen_child_specs.py
工作目录：fishbonechain 项目根目录
"""
import json
import os
import re
import subprocess
import sys
from pathlib import Path

ROOT    = Path(__file__).parent.parent
KEYS    = ROOT / "deploy" / "keys"
SPECS   = ROOT / "deploy" / "specs"
BIN_DIR = ROOT / "deploy" / "bin"


def load_env(node_id: str) -> dict:
    """从 deploy/keys/f{n}.env 解析所有 KEY=VALUE 对。"""
    path = KEYS / f"{node_id}.env"
    env = {}
    for line in path.read_text().splitlines():
        line = line.strip()
        if line and not line.startswith("#") and "=" in line:
            k, v = line.split("=", 1)
            env[k] = v.strip('"')
    return env


def build_spec(binary: Path, chain_id: str) -> dict:
    """用 binary 生成 human-readable chain spec，返回 dict。"""
    result = subprocess.run(
        [str(binary), "build-spec", "--chain", chain_id, "--disable-default-bootnode"],
        capture_output=True, text=True, check=True
    )
    return json.loads(result.stdout)


def to_raw(binary: Path, spec: dict, out_path: Path) -> None:
    """将修改后的 spec dict 转为 raw JSON 并写入 out_path。"""
    import tempfile
    with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
        json.dump(spec, f)
        tmp = f.name
    try:
        result = subprocess.run(
            [str(binary), "build-spec", "--chain", tmp, "--raw", "--disable-default-bootnode"],
            capture_output=True, text=True, check=True
        )
        out_path.write_text(result.stdout)
        print(f"  → {out_path.name} ({len(result.stdout) // 1024} KB)")
    finally:
        os.unlink(tmp)


def inject_validators(spec: dict, aura_ss58: list[str], gran_ss58: list[str], key_type: str = "aura") -> dict:
    """将真实 validator 公钥注入 genesis patch，替换 Alice/Bob dev keys。
    key_type='babe': 注入 babe.authorities，清空 aura.authorities
    key_type='aura': 注入 aura.authorities（默认）
    """
    patch = spec["genesis"]["runtimeGenesis"]["patch"]
    if key_type == "babe":
        # BABE chain: validators listed in babe.authorities (with weight), aura stays empty
        patch["babe"]["authorities"] = [[k, 1] for k in aura_ss58]
        patch["aura"]["authorities"] = []
    else:
        patch["aura"]["authorities"] = aura_ss58
    patch["grandpa"]["authorities"] = [[k, 1] for k in gran_ss58]
    return spec


def main():
    SPECS.mkdir(parents=True, exist_ok=True)

    # ── 主链 + 各子链配置 ─────────────────────────────────────────────────────
    chains = [
        {
            "name":     "main",
            "chain_id": "main-local",
            "binary":   BIN_DIR / "fishbone-node",
            "validators": [f"f{i}" for i in range(1, 13)],
            "out":      SPECS / "main-custom-raw.json",
        },
        {
            "name":     "child1",
            "chain_id": "child1-local",
            "binary":   BIN_DIR / "fishbone-node",
            "validators": ["f1", "f2", "f3"],
            "out":      SPECS / "child1-custom-raw.json",
        },
        {
            "name":     "child2",
            "chain_id": "child2-local",
            "binary":   BIN_DIR / "fishbone-node",
            "validators": ["f4", "f5", "f6"],
            "out":      SPECS / "child2-custom-raw.json",
        },
        {
            "name":     "child3",
            "chain_id": "child3-local",
            "binary":   BIN_DIR / "fishbone-node",
            "validators": ["f7", "f8", "f9"],
            "out":      SPECS / "child3-custom-raw.json",
        },
        {
            "name":     "child4",
            "chain_id": "child4-local",
            "binary":   BIN_DIR / "fishbone-node",
            "validators": ["f1", "f2", "f3", "f4", "f5", "f6", "f7"],
            "out":      SPECS / "child4-custom-raw.json",
        },
        {
            "name":     "child5",
            "chain_id": "child5-local",
            "binary":   BIN_DIR / "fishbone-node",
            "validators": ["f10", "f11", "f12"],
            "out":      SPECS / "child5-custom-raw.json",
        },
        {
            "name":     "child6",
            "chain_id": "child6-local",
            "binary":   BIN_DIR / "fishbone-node",
            "validators": ["f1", "f2", "f3", "f4", "f5"],
            "out":      SPECS / "child6-custom-raw.json",
        },
    ]

    for cfg in chains:
        name    = cfg["name"]
        binary  = cfg["binary"]
        out     = cfg["out"]

        print(f"\n[{name}] binary={binary.name}  validators={cfg['validators']}")

        if not binary.exists():
            print(f"  ✗ binary not found: {binary}")
            sys.exit(1)

        # 读取各 validator 的公钥
        aura_keys = []
        gran_keys = []
        for node in cfg["validators"]:
            env = load_env(node)
            aura_keys.append(env["AURA_SS58"])
            gran_keys.append(env["GRAN_SS58"])
            print(f"  {node}: aura={env['AURA_SS58'][:12]}…  gran={env['GRAN_SS58'][:12]}…")

        # 生成 human-readable spec
        print(f"  build-spec --chain {cfg['chain_id']} ...")
        spec = build_spec(binary, cfg["chain_id"])

        # 注入真实 validator 密钥
        key_type = cfg.get("key_type", "aura")
        spec = inject_validators(spec, aura_keys, gran_keys, key_type=key_type)

        # 转 raw 并写入
        print(f"  build-spec --raw ...")
        to_raw(binary, spec, out)

    print("\n✓ 所有 child spec 生成完成")
    for cfg in chains:
        print(f"  {cfg['out'].name}")


if __name__ == "__main__":
    main()
