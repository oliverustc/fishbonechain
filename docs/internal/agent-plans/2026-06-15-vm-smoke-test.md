# VM Smoke Test Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 将平台/场景解耦后的 FishboneChain 做一次最小 VM smoke test，验证主链、众包子链和数据交易子链在真实部署环境中的 binary、chain spec、metadata 和基础流程没有混用。

**架构：** 先修正部署前置配置，让 chain spec 生成、deploy 配置和 systemd service 都使用正确的 runtime profile binary。再构建 main/crowdsource/data-trade 三类二进制，生成 raw spec，最后在 VM 上优先验证 main、child1、child6 三条链的启动、metadata、profile 和场景边界。

**技术栈：** Substrate/Polkadot SDK、Rust runtime feature、Python deploy 脚本、Node.js `@polkadot/api` 验证脚本、systemd VM 服务。

---

## 文件职责

- `scripts/gen_child_specs.py`：根据链 profile 选择正确 binary 生成 human-readable spec 和 raw spec。
- `deploy/config.toml`：VM systemd 部署的链级 binary 映射单一真相来源。
- `scripts/deploy_child_chains.py`：旧部署脚本的 binary 映射，避免 child6 仍使用 BABE 众包 runtime。
- `deploy/fishbone/service.py`：服务 label 文案，避免 child6 被描述为旧 BABE 数据市场。
- `docs/internal/agent-plans/2026-06-15-vm-smoke-test.md`：执行状态、问题记录和 smoke test 结果。

## 问题记录

- 2026-06-15 计划创建时发现：`scripts/gen_child_specs.py` 中 child1/child6 仍使用 `deploy/bin/fishbone-node`；这会让 child1 缺少 crowdsource runtime，child6 缺少 data-trade runtime。
- 2026-06-15 计划创建时发现：`scripts/deploy_child_chains.py` 中 child6 仍使用 `fishbone-node-babe`，与当前 `scene-data-trade` 目标不一致。
- 2026-06-15 计划创建时发现：`deploy/config.toml` 仍依赖全局 `fishbone-node`，未声明 child1/child6 的 profile binary。
- 2026-06-15 执行 Task 1 时发现：`deploy/cmd/deploy.py` 当前只支持 `--only` 和 `--start/--no-start`，不支持计划中假设的 `--chains` 参数；后续 VM smoke 部署需要使用现有部署脚本能力、临时配置或补充 CLI 过滤能力，避免误触完整链集合。
- 2026-06-15 执行 deploy CLI 单测时发现：系统 `python3` 未安装 `typer`，需要使用仓库的 `deploy/.venv/bin/python` 执行 deploy 相关测试和 CLI。
- 2026-06-15 执行完整 deploy 单测时发现：`test_push_binaries_uses_remote_upload_without_asyncssh_connection` 仍期待直接上传到目标 binary 路径，但当前 `push_binaries` 已采用 `.new` 临时文件加 `mv -f` 的原子替换逻辑；测试需要同步为当前部署安全行为。
- 2026-06-15 执行 no-start 部署时发现：`deploy/main.py` 仍是 `Hello from deploy!` 模板入口，并没有挂载 deploy Typer app；本次改用 `deploy/cmd/deploy.py` 直接执行部署命令。
- 2026-06-15 执行 VM RPC smoke 时发现：`fishbone-main` 在 f1-f5 均为 active 但 `127.0.0.1:9944` 未返回 header；`fishbone-child6` 在 f1-f3 返回 header，f4/f5 未返回。需要检查 service 参数、监听端口、日志和旧 base path 状态。
- 2026-06-15 执行 RPC 汇总脚本时发现：直接把 JSON payload 传给远端 shell 会触发 zsh brace 解析，需要使用 `shlex.quote` 或单引号包裹 payload。
- 2026-06-15 清理 Python 缓存时发现：仓库中已有一个被追踪的 `scripts/__pycache__/render_topology_diagram.cpython-313.pyc`；误删后已立即恢复，后续清理应先看 `git status`，避免删除已追踪产物。
- 2026-06-15 执行本地 Node metadata 查询时发现：本机到 `ws://10.2.2.11:*` 不可直连，查询会卡在 WS 连接；需要通过 SSH 到 VM 内部执行，或先建立端口转发。
- 2026-06-15 执行 VM metadata/profile 验证时发现重大问题：main、child1、child6 的运行中 metadata 均显示 `Crowdsource=true`、`DataRegistry=false`、`TradeSession=false`，且 `chainProfile` 查询入口不存在；这说明 VM 上服务虽然启动和出块，但仍未运行到预期的 main/crowdsource/data-trade profile runtime，需要继续定位 feature 构建或远端 binary/spec 状态。
- 2026-06-15 定位上述 metadata 问题后确认：VM service 复用了 `/home/debian/fishbone/{main,child1,child6}` 旧 base path，已有数据库会继续使用链上既有 Wasm runtime；新的 raw chain spec 只在空数据库创世时生效。因此 profile runtime 验证必须使用空 base path/新 chain id，或走 runtime upgrade，不能只重启旧服务。
- 2026-06-15 启动临时干净节点时发现：远端 `pkill -f /tmp/fishbone-smoke-*` 会匹配当前 SSH 命令行并断开会话；后续清理临时节点改为读取 pid 文件后精确 `kill`。
- 2026-06-15 定位 `chainId` 始终为 0 时发现：`scripts/gen_child_specs.py` 外层 `chainProfile` 是正确字段名，但内部 profile 使用了 `chainId`/`paramsHash`；genesis builder 实际需要 `chain_id`/`params_hash`，否则 `chain_id` 会退回默认值。
- 2026-06-15 将 profile patch 外层误改为 `chain_profile` 后重新生成 raw spec 失败；`build-spec --raw` stderr 明确提示外层字段必须是 `chainProfile`。已修正方案为外层保持 `chainProfile`，仅转换内部字段名。
- 2026-06-15 在临时干净 child1/child6 单节点上执行 extrinsic smoke 时，交易长时间未入块；需要检查临时节点是否进入 authoring，或改用足够 validator 节点的干净网络/已有服务 runtime upgrade 后再做交易级验证。

## Task 1：修正部署前置 binary 映射

**文件：**

- 修改：`scripts/gen_child_specs.py`
- 修改：`deploy/config.toml`
- 修改：`scripts/deploy_child_chains.py`
- 修改：`deploy/fishbone/service.py`

- [x] **Step 1：让 `gen_child_specs.py` 支持 profile binary**

将链配置改为：

```python
"binary": BIN_DIR / "fishbone-node-crowdsource"
```

用于 child1、child4；child2/child3/child5 保持对应 2s/10mb/1s crowdsource 变体；child6 改为：

```python
"binary": BIN_DIR / "fishbone-node-data-trade"
```

- [x] **Step 2：给 `gen_child_specs.py` 增加 `--only` 参数**

支持：

```bash
python3 scripts/gen_child_specs.py --only main,child1,child6
```

用于 smoke test 只生成三条链 spec。

- [x] **Step 3：更新 VM deploy binary 映射**

在 `deploy/config.toml` 中设置：

```toml
[chains.child1]
binary = "/home/debian/fishbone/bin/fishbone-node-crowdsource"

[chains.child6]
binary = "/home/debian/fishbone/bin/fishbone-node-data-trade"
```

同时把 child6 注释改为 `DataTrade/MainEscrow`。

- [x] **Step 4：更新旧部署脚本映射**

在 `scripts/deploy_child_chains.py` 中：

```python
"child1": "fishbone-node-crowdsource",
"child4": "fishbone-node-crowdsource",
"child6": "fishbone-node-data-trade",
```

去掉 child6 的 BABE key 插入逻辑。

- [x] **Step 5：验证脚本语法**

运行：

```bash
python3 -m py_compile scripts/gen_child_specs.py scripts/deploy_child_chains.py
```

预期：退出码 0。

## Task 2：本地构建 smoke test 所需二进制

**文件：**

- 读取：`Makefile`
- 产物：`deploy/bin/fishbone-node`
- 产物：`deploy/bin/fishbone-node-crowdsource`
- 产物：`deploy/bin/fishbone-node-data-trade`

- [x] **Step 1：构建主链 binary**

运行：

```bash
make build-main
```

预期：`deploy/bin/fishbone-node` 存在且可执行。

- [x] **Step 2：构建众包子链 binary**

运行：

```bash
make build-crowdsource-child
```

预期：`deploy/bin/fishbone-node-crowdsource` 存在且可执行。

- [x] **Step 3：构建数据交易子链 binary**

运行：

```bash
make build-data-trade-child
```

预期：`deploy/bin/fishbone-node-data-trade` 存在且可执行。

- [x] **Step 4：记录 binary 元数据**

运行：

```bash
ls -lh deploy/bin/fishbone-node deploy/bin/fishbone-node-crowdsource deploy/bin/fishbone-node-data-trade
```

预期：三个文件均存在。

## Task 3：生成 smoke raw spec

**文件：**

- 修改产物：`deploy/specs/main-custom-raw.json`
- 修改产物：`deploy/specs/child1-custom-raw.json`
- 修改产物：`deploy/specs/child6-custom-raw.json`

- [x] **Step 1：生成 main/child1/child6 raw spec**

运行：

```bash
python3 scripts/gen_child_specs.py --only main,child1,child6
```

预期：三条链 raw spec 生成成功。

- [x] **Step 2：检查 chain profile 注入**

运行：

```bash
python3 - <<'PY'
import json
from pathlib import Path
for name in ["main", "child1", "child6"]:
    spec = json.loads(Path(f"deploy/specs/{name}-custom-raw.json").read_text())
    top = spec["genesis"]["raw"]["top"]
    print(name, len(top))
PY
```

预期：能读取 raw top。raw spec 中无法直接人工读出 profile 字段时，以 build-spec 成功作为 genesis patch 解析成功证据。

## Task 4：VM 连接与部署前检查

**文件：**

- 读取：`deploy/config.toml`
- 读取：`deploy/keys/*.env`

- [x] **Step 1：检查 SSH alias 可达**

运行：

```bash
for host in f1 f2 f3 f4 f5; do ssh -o BatchMode=yes -o ConnectTimeout=5 "$host" 'hostname && date'; done
```

预期：f1-f5 都能连接。若失败，记录到问题记录并停止 VM 部署。

- [x] **Step 2：检查远端目录**

运行：

```bash
for host in f1 f2 f3 f4 f5; do ssh "$host" 'mkdir -p /home/debian/fishbone/bin /home/debian/fishbone/specs /home/debian/fishbone/logs && ls -ld /home/debian/fishbone'; done
```

预期：目录存在。

## Task 5：部署 smoke 子集到 VM

**范围：**

- main：f1-f5 最小连通检查。注意完整主链最终仍需要 f1-f12。
- child1：f1-f3 众包子链。
- child6：f1-f5 数据交易子链。

- [x] **Step 1：推送 binary、spec 并安装 systemd service**

运行：

```bash
cd deploy && .venv/bin/python main.py deploy --only f1,f2,f3,f4,f5 --chains main,child1,child6 --no-start
```

预期：只推送 main/child1/child6 相关 binary 和 spec，并只安装这三条链的 service。

- [x] **Step 2：补充 deploy CLI 链过滤能力**

已给 deploy CLI 增加：

```bash
--chains main,child1,child6
```

并通过 `deploy/.venv/bin/python -m unittest deploy.tests.test_remote_system_ssh_callers` 验证过滤行为。

- [x] **Step 3：启动 smoke 服务**

运行：

```bash
for host in f1 f2 f3 f4 f5; do ssh "$host" 'sudo systemctl restart fishbone-main || true'; done
for host in f1 f2 f3; do ssh "$host" 'sudo systemctl restart fishbone-child1 || true'; done
for host in f1 f2 f3 f4 f5; do ssh "$host" 'sudo systemctl restart fishbone-child6 || true'; done
```

预期：服务启动或明确显示正确服务名不存在。

## Task 6：VM smoke 验证

**文件：**

- 使用：`scripts/setup_experiment.js`
- 使用：`scripts/bridges/crowdsource.js`
- 使用：`scripts/bridges/data_trade.js`

- [x] **Step 1：检查出块**

运行：

```bash
node scripts/check-rpc-blocks.js
```

若该脚本不存在，使用 `curl` 或 `@polkadot/api` 临时命令检查 `main(9944)`、`child1(9945)`、`child6(9950)` 最新块号。

结果：systemd 服务在 f1-f5 上 active；RPC 汇总显示 main/child6/child1 均返回区块高度且 `isSyncing=false`。注意这些服务复用旧 base path，证明的是现有链继续运行，不等价于新 genesis runtime profile 已生效。

- [x] **Step 2：检查 runtime metadata**

使用 `@polkadot/api` 查询：

- main 应包含 `ccmc`、`fmc`、`chainProfile`
- child1 应包含 `crowdsource`、`chainProfile`
- child6 应包含 `dataRegistry`、`tradeSession`、`chainProfile`
- child6 不应包含 `crowdsource`

结果：旧 base path 服务 metadata 仍是旧 crowdsource runtime；改用 f1 上 `/tmp/fishbone-smoke-*` 空 base path 临时启动后验证通过：

- main-clean：`Crowdsource=false`、`ChainProfile=true`、`DataRegistry=false`、`TradeSession=false`
- child1-clean：`Crowdsource=true`、`ChainProfile=true`、`DataRegistry=false`、`TradeSession=false`
- child6-clean：`Crowdsource=false`、`ChainProfile=true`、`DataRegistry=true`、`TradeSession=true`

- [x] **Step 3：检查 profile storage**

查询：

```javascript
api.query.chainProfile.profile()
```

预期：

- main：`PlatformOnly/None`
- child1：`Crowdsource/FmcTaskBill`
- child6：`DataTrade/MainEscrow`

结果：干净启动验证通过：

- main-clean：`chainId=0`、`PlatformOnly/None`
- child1-clean：`chainId=0`、`Crowdsource/FmcTaskBill`
- child6-clean：`chainId=5`、`DataTrade/MainEscrow`

- [x] **Step 4：数据交易最小 extrinsic**

对 child6 执行：

```javascript
dataRegistry.publishData(root, description)
tradeSession.createSession(dataOwner, hashChainEnd, 1, "MainEscrow")
```

预期：事件出现，不需要 FMC。

结果：在 f1 临时干净 child6 validator 节点上执行通过：

- `child6.publishData`：出现 `dataRegistry.DataPublished`
- `child6.createSession`：出现 `tradeSession.SessionCreated`

- [x] **Step 5：众包兼容 smoke**

对 child1 执行一次：

```javascript
crowdsource.syncTask(task_id, requester, budget, description)
```

预期：`TaskSynced` 事件出现。

结果：在 f1 临时干净 child1 validator 节点上执行通过，出现 `crowdsource.TaskSynced`。

## Task 7：收口与记录

- [x] **Step 1：更新本计划问题记录与结果**

把每个失败、绕过和最终结果写入本文件。

- [x] **Step 2：运行最终本地验证**

运行：

```bash
SKIP_WASM_BUILD=1 cargo check -p fishbone-node
python3 -m py_compile scripts/gen_child_specs.py scripts/deploy_child_chains.py
node --check scripts/bridges/crowdsource.js
node --check scripts/bridges/data_trade.js
```

预期：全部通过。

结果：以下命令均通过：

- `SKIP_WASM_BUILD=1 cargo check -p fishbone-node`
- `python3 -m unittest scripts/test_gen_child_specs.py`
- `python3 -m py_compile scripts/gen_child_specs.py scripts/deploy_child_chains.py`
- `deploy/.venv/bin/python -m unittest deploy.tests.test_remote_system_ssh_callers`
- `node --check scripts/bridges/crowdsource.js`
- `node --check scripts/bridges/data_trade.js`

- [x] **Step 3：汇报**

汇报内容必须包括：

- 是否实际部署到 VM；
- 哪些 VM/链启动成功；
- main/child1/child6 metadata 是否符合预期；
- child6 是否确认不包含众包 pallet；
- 遇到的问题及是否已写回本计划。
