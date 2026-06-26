#!/usr/bin/env python3
"""
生成 main/child chain spec（human-readable → 注入真实 validator 密钥与 chain profile → raw）。

用法：
  python3 scripts/gen_child_specs.py
  python3 scripts/gen_child_specs.py --only main,child1,child6
工作目录：fishbonechain 项目根目录
"""
import argparse
import json
import os
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


def inject_spec_identity(spec: dict, name: str, spec_id: str) -> dict:
    """覆盖 spec 的 name 和 id 字段，使模板生成的 spec 使用目标链身份。"""
    spec["name"] = name
    spec["id"] = spec_id
    return spec


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


def zero_hash() -> str:
    return "0x" + "00" * 32


def inject_chain_profile(spec: dict, profile: dict) -> dict:
    """将链 profile 注入 genesis patch，供 runtime 的 pallet-chain-profile 初始化。"""
    patch = spec["genesis"]["runtimeGenesis"]["patch"]
    patch["chainProfile"] = {
        "profile": {
            "chain_id": profile["chainId"],
            "scene": profile["scene"],
            "settlement": profile["settlement"],
            "params_hash": profile["paramsHash"],
        }
    }
    return spec


def chain_configs() -> list[dict]:
    """返回所有链的 spec 生成配置。"""
    return [
        {
            "name": "main",
            "chain_id": "main-local",
            "binary": BIN_DIR / "fishbone-node",
            "validators": [f"f{i}" for i in range(1, 13)],
            "out": SPECS / "main-custom-raw.json",
            "profile": {
                "chainId": 0,
                "scene": "PlatformOnly",
                "settlement": "None",
                "paramsHash": zero_hash(),
            },
        },
        {
            "name": "child1",
            "chain_id": "child1-local",
            "binary": BIN_DIR / "fishbone-node-crowdsource",
            "validators": ["f1", "f2", "f3"],
            "out": SPECS / "child1-custom-raw.json",
            "profile": {
                "chainId": 0,
                "scene": "Crowdsource",
                "settlement": "FmcTaskBill",
                "paramsHash": zero_hash(),
            },
        },
        {
            "name": "child2",
            "chain_id": "child2-local",
            "binary": BIN_DIR / "fishbone-node-2s",
            "validators": ["f4", "f5", "f6"],
            "out": SPECS / "child2-custom-raw.json",
            "profile": {
                "chainId": 1,
                "scene": "Crowdsource",
                "settlement": "FmcTaskBill",
                "paramsHash": zero_hash(),
            },
        },
        {
            "name": "child3",
            "chain_id": "child3-local",
            "binary": BIN_DIR / "fishbone-node-10mb",
            "validators": ["f7", "f8", "f9"],
            "out": SPECS / "child3-custom-raw.json",
            "profile": {
                "chainId": 2,
                "scene": "Crowdsource",
                "settlement": "FmcTaskBill",
                "paramsHash": zero_hash(),
            },
        },
        {
            "name": "child4",
            "chain_id": "child4-local",
            "binary": BIN_DIR / "fishbone-node-crowdsource",
            "validators": ["f1", "f2", "f3", "f4", "f5", "f6", "f7"],
            "out": SPECS / "child4-custom-raw.json",
            "profile": {
                "chainId": 3,
                "scene": "Crowdsource",
                "settlement": "FmcTaskBill",
                "paramsHash": zero_hash(),
            },
        },
        {
            "name": "child5",
            "chain_id": "child5-local",
            "binary": BIN_DIR / "fishbone-node-1s",
            "validators": ["f10", "f11", "f12"],
            "out": SPECS / "child5-custom-raw.json",
            "profile": {
                "chainId": 4,
                "scene": "Crowdsource",
                "settlement": "FmcTaskBill",
                "paramsHash": zero_hash(),
            },
        },
        {
            "name": "child6",
            "chain_id": "child6-local",
            "binary": BIN_DIR / "fishbone-node-data-trade",
            "validators": ["f1", "f2", "f3", "f4", "f5"],
            "out": SPECS / "child6-custom-raw.json",
            "profile": {
                "chainId": 5,
                "scene": "DataTrade",
                "settlement": "MainEscrow",
                "paramsHash": zero_hash(),
            },
        },
        {
            "name": "child7",
            "chain_id": "child7-local",
            "template_chain_id": "child6-local",
            "spec_name": "Fishbone Child-7 (Business Data Trade, AURA-5)",
            "spec_id": "fishbone_child_7",
            "binary": BIN_DIR / "fishbone-node-data-trade",
            "validators": ["f1", "f2", "f3", "f4", "f5"],
            "out": SPECS / "child7-custom-raw.json",
            "profile": {
                "chainId": 6,
                "scene": "DataTrade",
                "settlement": "MainEscrow",
                "paramsHash": zero_hash(),
            },
        },
    ]


def apply_profile_overrides(chains: list[dict], profile_file: Path | None) -> list[dict]:
    if profile_file is None:
        return chains
    raw = json.loads(profile_file.read_text())
    profiles = raw.get("chains", raw)
    updated = []
    for cfg in chains:
        cfg = dict(cfg)
        profile = profiles.get(cfg["name"])
        if profile:
            runtime_binary = profile.get("runtimeBinary")
            if runtime_binary:
                cfg["binary"] = BIN_DIR / runtime_binary
            if profile.get("validators"):
                cfg["validators"] = profile["validators"]
            cfg["profile"] = {
                "chainId": profile["chainId"],
                "scene": profile["scene"],
                "settlement": profile["settlement"],
                "paramsHash": profile.get("paramsHash", zero_hash()),
            }
        updated.append(cfg)
    return updated


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--only",
        default="",
        help="逗号分隔链列表，例如 main,child1,child6；默认生成全部链",
    )
    parser.add_argument(
        "--profile-file",
        type=Path,
        help="实验 profile JSON，用于覆盖子链 runtime binary 和 genesis chain profile",
    )
    return parser.parse_args()


def main():
    args = parse_args()
    SPECS.mkdir(parents=True, exist_ok=True)

    # ── 主链 + 各子链配置 ─────────────────────────────────────────────────────
    chains = apply_profile_overrides(chain_configs(), args.profile_file)
    if args.only:
        wanted = {name.strip() for name in args.only.split(",") if name.strip()}
        known = {cfg["name"] for cfg in chains}
        unknown = sorted(wanted - known)
        if unknown:
            print(f"✗ unknown chain(s): {', '.join(unknown)}")
            sys.exit(1)
        chains = [cfg for cfg in chains if cfg["name"] in wanted]

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

        # 生成 human-readable spec (template_chain_id allows child7 to reuse child6's preset)
        template_chain_id = cfg.get("template_chain_id", cfg["chain_id"])
        print(f"  build-spec --chain {template_chain_id} ...")
        spec = build_spec(binary, template_chain_id)

        # 注入目标链身份（覆盖模板 preset 的 name/id），仅当配置了 spec_name 时
        if "spec_name" in cfg and "spec_id" in cfg:
            spec = inject_spec_identity(spec, cfg["spec_name"], cfg["spec_id"])

        # 注入真实 validator 密钥
        key_type = cfg.get("key_type", "aura")
        spec = inject_validators(spec, aura_keys, gran_keys, key_type=key_type)
        spec = inject_chain_profile(spec, cfg["profile"])

        # 转 raw 并写入
        print(f"  build-spec --raw ...")
        to_raw(binary, spec, out)

    print("\n✓ 所有 child spec 生成完成")
    for cfg in chains:
        print(f"  {cfg['out'].name}")


if __name__ == "__main__":
    main()
