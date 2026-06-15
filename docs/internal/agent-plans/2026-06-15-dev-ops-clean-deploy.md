# Dev Ops Clean Deploy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 提供配置驱动的开发期部署、停止并清理数据目录、重置后重新部署、VM 状态扫描脚本，避免旧 base path、旧 runtime、旧二进制在开发阶段继续污染验证结果。

**架构：** 以 `deploy/config.toml` 为唯一拓扑来源，复用现有 Python deploy/control 框架。`control.py` 增加 `stop-clean`，负责停止服务并清空链数据目录；`deploy.py` 继续负责推送 binary/spec、生成 node-key、注入 validator keys、安装并启动服务；shell wrapper 只做薄封装，不硬编码节点或链。扫描脚本读取同一份配置，按配置中的 nodes/chains 远程采集服务、进程、监听端口、目录大小、binary/spec 时间。

**技术栈：** Python 3.11+ `tomllib`/Typer/asyncssh wrapper、systemd、SSH、Bash wrapper、unittest。

---

## 文件职责

- `deploy/fishbone/config.py`：新增配置过滤辅助函数，让 deploy/control/scan 共享节点与链过滤逻辑。
- `deploy/cmd/deploy.py`：改用共享过滤逻辑，并支持 `--nodes` 作为 `--only` 的别名。
- `deploy/cmd/control.py`：新增 `stop-clean` 命令，停止指定链服务并清空对应 data dir，保留 `node-key`。
- `deploy/tests/test_remote_system_ssh_callers.py`：补充 stop-clean、配置过滤、原子 binary 上传行为的测试。
- `scripts/dev_deploy_chains.sh`：薄 wrapper，调用 `deploy/cmd/deploy.py`。
- `scripts/dev_stop_clean_chains.sh`：薄 wrapper，调用 `deploy/cmd/control.py stop-clean`。
- `scripts/dev_redeploy_clean_chains.sh`：先 stop-clean，再 deploy，适合开发期一键干净重启。
- `scripts/dev_scan_vms.py`：配置驱动扫描 VM 服务、进程、端口、目录、binary/spec 状态。
- `scripts/dev_scan_vms.sh`：薄 wrapper，调用 `scripts/dev_scan_vms.py`。
- `docs/internal/agent-plans/2026-06-15-dev-ops-clean-deploy.md`：记录执行状态、VM 扫描发现和后续运维问题。

## 关键约束

- 不硬编码 VM 数量、节点名、链名或链角色；默认从 `deploy/config.toml` 读取。
- `stop-clean` 必须保留 `{base_dir}/{chain}/node-key`，否则 peer id 会改变，`deploy/config.toml` 中的 bootnodes 会失效。
- `stop-clean` 删除 `{base_dir}/{chain}` 下除 `node-key` 以外的所有内容，包含链数据库和 validator keystore；后续必须通过 deploy 重新注入 validator keys。
- 清理命令必须带路径安全校验，拒绝清理不在 `cfg.base_dir` 之下的路径。
- wrapper 只传参，不复制拓扑规则。

## 问题记录

- 2026-06-15 扫描发现：f1-f5 已有 2026-06-15 新 main/crowdsource/data-trade binary 与 spec，但 f6-f12 多数仍是 2026-06-13 旧 binary/spec。
- 2026-06-15 扫描发现：f6-f12 主链 base path 多数已经增长到 6.4G-7.3G，旧数据会让新 genesis runtime profile 验证失效。
- 2026-06-15 扫描发现：f1-f5 的 `child4` base path 约 3.0G，f6/f7 的 `child4` 约 3.9G-4.2G，需要开发期清理入口。
- 2026-06-15 扫描发现：`deploy/main.py` 仍是 `Hello from deploy!` 模板入口，当前仍需直接调用 `deploy/cmd/*.py` 或 wrapper。
- 2026-06-15 扫描命令发现：复杂远端 awk quoting 容易出错，正式扫描脚本应使用简单 shell 输出格式，并尽量在本地 Python 汇总。
- 2026-06-15 脚本实现发现：SSH 远端多参数执行带换行脚本时有兼容风险，`dev_scan_vms.py` 已改为向 SSH 传递单个 `sh -lc <quoted-script>` 命令参数。
- 2026-06-15 小范围只读扫描发现：f1 在扫描 `main,child6` 时仍存在运行中的 `child4` 进程，说明开发期 VM 上可能存在配置外或本次选择范围外的残留服务，需要全量扫描后统一清理。
- 2026-06-15 全量只读扫描结果：f1-f12 均可 SSH 连接，未发现 `/tmp/fishbone-smoke-*` 残留。
- 2026-06-15 全量只读扫描结果：f1-f5 的 `main` 与 `child6` 已更新到 2026-06-15 binary/spec，`child1` 在 f1-f3 也已更新到 2026-06-15；但 f1-f5 的 `child4` spec 仍是 2026-06-13，f4-f5 的 `child2` spec 仍是 2026-06-13。
- 2026-06-15 全量只读扫描结果：f6-f12 的 `main` binary/spec 仍是 2026-06-13，`child2/child3/child5` 也仍是 2026-06-13；这些 VM 不应继续作为新 runtime/profile 的验证依据，除非先执行 clean redeploy。
- 2026-06-15 全量只读扫描结果：f6/f7 按当前 `deploy/config.toml` 的 child4 binary 配置扫描时缺少 `/home/debian/fishbone/bin/fishbone-node-crowdsource`，但实际运行的 child4 进程使用 `/home/debian/fishbone/bin/fishbone-node`，说明旧服务单元与当前配置存在漂移，需要通过重新部署统一。
- 2026-06-15 全量只读扫描结果：旧数据目录仍然较大，典型值包括 f6 main 6.9G、f7 main 7.3G、f8/f9 main 6.5G、f10/f11 main 6.5G、f12 main 6.4G；child2/child3/child5 多为 1.7G，child4 在 f6/f7 为 4.2G/3.9G。

## Task 1：共享配置过滤能力

**文件：**

- 修改：`deploy/fishbone/config.py`
- 修改：`deploy/cmd/deploy.py`
- 修改：`deploy/tests/test_remote_system_ssh_callers.py`

- [x] **Step 1：写配置过滤单测**

在 `deploy/tests/test_remote_system_ssh_callers.py` 中增加测试，验证共享过滤函数：

```python
from fishbone.config import filter_config_to_chains

def test_shared_filter_config_to_chains_preserves_original_roles(self):
    cfg = SimpleNamespace(
        chains={"main": object(), "child1": object(), "child6": object()},
        nodes=[
            SimpleNamespace(id="f1", roles=["main", "child1", "child6"]),
            SimpleNamespace(id="f8", roles=["main"]),
        ],
    )

    filtered = filter_config_to_chains(cfg, {"main", "child6"})

    self.assertEqual(list(filtered.chains), ["main", "child6"])
    self.assertEqual(filtered.nodes[0].roles, ["main", "child6"])
    self.assertEqual(filtered.nodes[1].roles, ["main"])
    self.assertEqual(cfg.nodes[0].roles, ["main", "child1", "child6"])
```

- [x] **Step 2：运行单测确认失败**

运行：

```bash
deploy/.venv/bin/python -m unittest deploy.tests.test_remote_system_ssh_callers.RemoteSystemSshCallerTests.test_shared_filter_config_to_chains_preserves_original_roles
```

预期：失败，提示 `filter_config_to_chains` 不存在。

- [x] **Step 3：实现共享过滤函数**

在 `deploy/fishbone/config.py` 中实现：

```python
def csv_set(value: str) -> set[str] | None:
    items = {v.strip() for v in value.split(",") if v.strip()}
    return items or None

def filter_config_to_chains(cfg, selected_chains: set[str]):
    filtered = copy.copy(cfg)
    filtered.chains = {
        name: chain
        for name, chain in cfg.chains.items()
        if name in selected_chains
    }
    filtered.nodes = []
    for node in cfg.nodes:
        filtered_node = copy.copy(node)
        filtered_node.roles = [role for role in node.roles if role in filtered.chains]
        filtered.nodes.append(filtered_node)
    return filtered
```

- [x] **Step 4：让 deploy.py 使用共享函数**

修改 `deploy/cmd/deploy.py`：

```python
from fishbone.config import load, csv_set, filter_config_to_chains
```

并删除本文件内重复的 `filter_config_to_chains` 实现。

- [x] **Step 5：运行 deploy 单测**

运行：

```bash
deploy/.venv/bin/python -m unittest deploy.tests.test_remote_system_ssh_callers
```

预期：全部通过。

## Task 2：实现 stop-clean 控制命令

**文件：**

- 修改：`deploy/cmd/control.py`
- 修改：`deploy/tests/test_remote_system_ssh_callers.py`

- [x] **Step 1：写清理命令单测**

在 `FakeRemote` 中加入：

```python
async def sudo(self, cmd: str, check: bool = True):
    self.commands.append(f"sudo {cmd}")
    return SimpleNamespace(returncode=0, stdout="", stderr="")
```

新增测试：

```python
async def test_clean_chain_data_preserves_node_key_and_removes_chain_state(self):
    remote = FakeRemote()
    cfg = SimpleNamespace(base_dir="/remote/fishbone", log_dir="/remote/fishbone/logs")

    await control_cmd.clean_chain_data(remote, cfg, "child6", clean_logs=True)

    joined = "\n".join(remote.commands)
    self.assertIn("/remote/fishbone/child6", joined)
    self.assertIn("! -name node-key", joined)
    self.assertIn("rm -rf", joined)
    self.assertIn("/remote/fishbone/logs/child6.log", joined)
```

- [x] **Step 2：运行单测确认失败**

运行：

```bash
deploy/.venv/bin/python -m unittest deploy.tests.test_remote_system_ssh_callers.RemoteSystemSshCallerTests.test_clean_chain_data_preserves_node_key_and_removes_chain_state
```

预期：失败，提示 `control_cmd` 或 `clean_chain_data` 不存在。

- [x] **Step 3：实现 `clean_chain_data`**

在 `deploy/cmd/control.py` 中新增：

```python
async def clean_chain_data(remote, cfg, chain: str, clean_logs: bool = False):
    base_path = f"{cfg.base_dir}/{chain}"
    log_path = f"{cfg.log_dir}/{chain}.log"
    script = f"""
set -eu
base={shlex.quote(base_path)}
case "$base" in
  {cfg.base_dir.rstrip('/')!s}/*) ;;
  *) echo "refuse to clean unsafe path: $base" >&2; exit 2 ;;
esac
mkdir -p "$base"
find "$base" -mindepth 1 -maxdepth 1 ! -name node-key -exec rm -rf -- {{}} +
"""
    if clean_logs:
        script += f"\nrm -f -- {shlex.quote(log_path)}\n"
    await remote.sudo(f"sh -lc {shlex.quote(script)}")
```

- [x] **Step 4：实现 `stop-clean` Typer 命令**

新增命令：

```python
@app.command("stop-clean")
def stop_clean(
    nodes: str = typer.Option("", help="节点列表，逗号分隔（默认全部）"),
    chains: str = typer.Option("", help="链名称，逗号分隔（默认全部）"),
    logs: bool = typer.Option(False, help="同时删除对应链日志"),
    config: Path = typer.Option(Path(__file__).parent.parent / "config.toml"),
):
    asyncio.run(_stop_clean(nodes, chains, logs, config))
```

`_stop_clean` 必须先 stop service，再调用 `clean_chain_data`。

- [x] **Step 5：运行 control/deploy 单测**

运行：

```bash
deploy/.venv/bin/python -m unittest deploy.tests.test_remote_system_ssh_callers
```

预期：全部通过。

## Task 3：新增开发期 wrapper 脚本

**文件：**

- 新建：`scripts/dev_deploy_chains.sh`
- 新建：`scripts/dev_stop_clean_chains.sh`
- 新建：`scripts/dev_redeploy_clean_chains.sh`

- [x] **Step 1：创建 deploy wrapper**

`scripts/dev_deploy_chains.sh`：

```bash
#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT/deploy"
exec .venv/bin/python cmd/deploy.py "$@"
```

- [x] **Step 2：创建 stop-clean wrapper**

`scripts/dev_stop_clean_chains.sh`：

```bash
#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT/deploy"
exec .venv/bin/python cmd/control.py stop-clean "$@"
```

- [x] **Step 3：创建 redeploy-clean wrapper**

`scripts/dev_redeploy_clean_chains.sh` 支持 `--nodes`、`--chains`、`--config`、`--logs`，先 stop-clean，再 deploy：

```bash
scripts/dev_stop_clean_chains.sh --nodes "$NODES" --chains "$CHAINS" --config "$CONFIG" "$LOGS_ARG"
scripts/dev_deploy_chains.sh --nodes "$NODES" --chains "$CHAINS" --config "$CONFIG"
```

- [x] **Step 4：赋予执行权限并语法检查**

运行：

```bash
chmod +x scripts/dev_deploy_chains.sh scripts/dev_stop_clean_chains.sh scripts/dev_redeploy_clean_chains.sh
bash -n scripts/dev_deploy_chains.sh scripts/dev_stop_clean_chains.sh scripts/dev_redeploy_clean_chains.sh
```

预期：退出码 0。

## Task 4：新增 VM 扫描脚本

**文件：**

- 新建：`scripts/dev_scan_vms.py`
- 新建：`scripts/dev_scan_vms.sh`

- [x] **Step 1：实现 Python 扫描脚本**

`scripts/dev_scan_vms.py` 从 `deploy/config.toml` 读取节点、链、端口和 binary/spec 路径，支持：

```bash
python3 scripts/dev_scan_vms.py --nodes f1,f2 --chains main,child6
```

输出每台 VM：

- systemd `fishbone-*` 服务状态；
- fishbone 进程对应的 base path；
- 配置内端口监听情况；
- `{base_dir}/{chain}` 大小；
- 配置内 binary/spec 的大小和 mtime；
- `/tmp/fishbone-smoke-*` 残留。

- [x] **Step 2：实现 scan wrapper**

`scripts/dev_scan_vms.sh`：

```bash
#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec python3 "$ROOT/scripts/dev_scan_vms.py" "$@"
```

- [x] **Step 3：语法检查**

运行：

```bash
python3 -m py_compile scripts/dev_scan_vms.py
chmod +x scripts/dev_scan_vms.sh
bash -n scripts/dev_scan_vms.sh
```

预期：退出码 0。

## Task 5：验证并执行只读扫描

**文件：**

- 读取：`deploy/config.toml`
- 读取：VM f1-f12 状态
- 修改：本计划文件的问题记录

- [x] **Step 1：运行本地测试**

运行：

```bash
deploy/.venv/bin/python -m unittest deploy.tests.test_remote_system_ssh_callers
python3 -m unittest scripts/test_gen_child_specs.py
python3 -m py_compile scripts/gen_child_specs.py scripts/deploy_child_chains.py scripts/dev_scan_vms.py
bash -n scripts/dev_deploy_chains.sh scripts/dev_stop_clean_chains.sh scripts/dev_redeploy_clean_chains.sh scripts/dev_scan_vms.sh
```

预期：全部通过。

- [x] **Step 2：执行只读 VM 扫描**

运行：

```bash
scripts/dev_scan_vms.sh
```

预期：f1-f12 均可连接，输出服务、进程、端口、目录、binary/spec 状态。

- [x] **Step 3：记录扫描发现**

把仍存在的旧 binary/spec、旧数据目录、运行中服务和后续建议写入本计划 `问题记录`。

- [x] **Step 4：最终汇报**

汇报必须包括：

- 新增脚本的使用方法；
- `stop-clean` 清理哪些内容、保留哪些内容；
- 当前 VM 的主要运维问题；
- 已执行的验证命令。
