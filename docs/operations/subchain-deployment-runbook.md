# 子链部署与恢复 Runbook

本文记录 FishboneChain 开发环境中新增、重部署、恢复子链时需要检查的关键点。目标是避免 RPC 可达但链不出块、spec 与节点密钥漂移、旧数据污染验证结果等问题。

## 核心原则

`deploy/config.toml` 是部署拓扑的单一真相来源，`scripts/gen_child_specs.py` 是链 genesis/spec 的生成入口。部署一条子链时，必须同时满足：

- `deploy/config.toml` 中该链的 `spec`、`binary`、端口、节点 `roles` 正确。
- `scripts/gen_child_specs.py` 中该链的 validators、profile、binary、输出 spec 正确。
- `deploy/keys/f*.env` 中对应节点的 Aura/Grandpa key 与 spec genesis 中的 authorities 一致。
- 远端 systemd 服务实际使用的 binary/spec/data dir 与当前配置一致。
- 部署后不仅 RPC 可连接，还必须确认链会持续出块。

只看到 RPC ready 不代表部署成功。RPC 可以在共识错误时启动，但链可能卡在 `#0`。

## 标准流程

### 1. 部署前检查

确认待部署子链在 `deploy/config.toml` 中存在：

```toml
[chains.childN]
id       = "fishbone_child_N"
spec     = "specs/childN-custom-raw.json"
binary   = "/home/debian/fishbone/bin/<matching-binary>"
p2p_port = 303xx
rpc_port = 99xx
prom_port = 96xx
```

确认参与验证人的节点 `roles` 包含该子链：

```toml
[[nodes]]
id    = "f1"
roles = ["main", "childN"]
```

确认 `scripts/gen_child_specs.py` 中该子链配置存在，并且 validators 与 `deploy/config.toml` 的 roles 一致：

```python
{
    "name": "childN",
    "binary": BIN_DIR / "<matching-binary>",
    "validators": ["f1", "f2", "f3"],
    "out": SPECS / "childN-custom-raw.json",
    "profile": {
        "chainId": N,
        "scene": "...",
        "settlement": "...",
        "paramsHash": zero_hash(),
    },
}
```

检查重点：

- validators 集合必须等于实际会运行该子链 validator 的节点集合。
- `chainId` 不要与现有子链重复。
- runtime profile 要与论文场景一致，例如数据交易链使用 `DataTrade` / `MainEscrow`。
- `binary` 要匹配 runtime profile，例如数据交易链使用 `fishbone-node-data-trade`。
- `rpc_port`、`p2p_port`、`prom_port` 不要与已有链冲突。

### 2. 生成 spec

从项目根目录执行：

```bash
python3 scripts/gen_child_specs.py --only childN
```

如果同时调整多条链：

```bash
python3 scripts/gen_child_specs.py --only child6,child7
```

生成后检查是否有预期 diff：

```bash
git diff -- deploy/specs/childN-custom-raw.json
```

如果只是恢复环境，且生成后 spec 没有 diff，说明仓库内 spec 本身是当前正确版本，不需要提交 spec 文件。

### 3. 部署或干净重部署

普通部署：

```bash
scripts/dev_deploy_chains.sh --chains childN
```

干净重部署：

```bash
scripts/dev_redeploy_clean_chains.sh --chains childN --logs
```

选择规则：

- 新子链首次部署：通常可以普通部署。
- 改了 genesis/spec authorities、profile、chain id、runtime genesis 配置：必须 clean redeploy。
- 链已经启动但卡在 `#0`：优先怀疑 genesis authorities 或旧数据，通常需要重新生成 spec 后 clean redeploy。
- 只更新 binary 且不改变 genesis：可以先普通部署，但仍要做出块验收。

注意：`dev_redeploy_clean_chains.sh --chains childN` 只清理选中的链。不要为了恢复子链顺手清理 main，除非明确需要重置 main。

### 4. 部署后验收

先确认 main 和子链都持续出块：

```bash
node scripts/lib/wait_for_ws_chain.js \
  --main ws://10.2.2.11:9944 \
  --child ws://10.2.2.11:99xx \
  --min-blocks 2 \
  --timeout-ms 120000
```

再查询子链 authorities：

```bash
node --input-type=module - <<'NODE'
import { ApiPromise, WsProvider } from '@polkadot/api';

const api = await ApiPromise.create({
  provider: new WsProvider('ws://10.2.2.11:99xx'),
});
const [chain, header, authorities] = await Promise.all([
  api.rpc.system.chain(),
  api.rpc.chain.getHeader(),
  api.query.aura.authorities(),
]);

console.log('chain', chain.toString());
console.log('number', header.number.toString());
console.log(authorities.map((a) => a.toString()).join('\n'));
await api.disconnect();
NODE
```

验收标准：

- 链名符合预期。
- block number 在检查窗口内增长。
- `aura.authorities` 与该子链 validators 对应的 `deploy/keys/f*.env` key 一致。
- 远端日志中出现 `Imported #...` 和 consensus/session 相关日志。

## 常见问题

### RPC 可达但链卡在 `#0`

优先检查 genesis authorities。

典型原因：

- spec 是旧的，包含上一轮实验的 validator keys。
- `scripts/gen_child_specs.py` 的 validators 与 `deploy/config.toml` 的 node roles 不一致。
- 节点服务使用了旧 spec 或旧 data dir。
- 清理数据后没有重新注入 validator keys。

处理方式：

```bash
python3 scripts/gen_child_specs.py --only childN
scripts/dev_redeploy_clean_chains.sh --chains childN --logs
```

然后重新执行出块和 authorities 验收。

### `--config deploy/config.toml` 路径失败

`scripts/dev_deploy_chains.sh` 会先进入 `deploy/` 目录再调用 Python 部署入口。因此从项目根目录执行时：

```bash
scripts/dev_deploy_chains.sh --chains childN
```

通常不需要显式传 `--config`。

如果必须传配置路径，使用绝对路径，或使用会规范化路径的 wrapper：

```bash
scripts/dev_redeploy_clean_chains.sh --chains childN --config /home/swt/fishbonechain/deploy/config.toml
```

不要传会在 `deploy/` 内再次拼接的相对路径，例如：

```bash
scripts/dev_deploy_chains.sh --chains childN --config deploy/config.toml
```

这类写法可能导致脚本查找 `deploy/deploy/config.toml`。

### 旧服务单元或旧 binary 污染结果

开发环境多轮实验后，远端 VM 可能存在旧 service、旧 binary、旧 spec 或旧 data dir。症状包括：

- systemd service active，但日志显示旧链名或旧 runtime。
- 扫描到的 binary/spec 时间早于当前实验。
- `deploy/config.toml` 已经改了 binary，但远端进程仍在跑旧 binary。
- 同一台 VM 上有本轮不需要的旧子链进程。

处理方式：

```bash
scripts/dev_scan_vms.sh --chains childN
scripts/dev_redeploy_clean_chains.sh --chains childN --logs
```

如果发现配置外残留服务，先记录现象，再按影响范围决定是否单独 stop/clean，避免误清理其它实验所需链。

### Clean redeploy 的影响范围

`scripts/dev_redeploy_clean_chains.sh --chains childN --logs` 会停止并清理指定链的数据目录和日志，然后重新部署。它会保留该链的 `node-key`，避免 peer id 改变导致 bootnodes 失效。

不要在没有明确目标时执行全量 clean redeploy。尤其是 main 链，清理会破坏已有 live-chain 状态和论文实验上下文。

## Child6 事故复盘

2026-06-27 恢复 child6 时遇到的问题：

- 初始 child6 RPC 不可用。
- 普通部署后服务和 RPC 可达，但链卡在 `#0`。
- 查询 `aura.authorities` 后发现 genesis 中是错误的三密钥集合，而 child6 实际应由 f1-f5 五个节点出块。
- 重新执行 `python3 scripts/gen_child_specs.py --only child6` 后，再执行 child6-only clean redeploy，链恢复出块。

恢复后验收结果：

- main 从 `#102804` 增长到 `#102806`。
- child6 从 `#23` 增长到 `#25`。
- child6 authorities 与 f1-f5 keys 一致。

详细记录见：

```text
docs/internal/agent-reviews/2026-06-27-data-trade-stage13-child6-recovery.md
```

## 新增子链 Checklist

- [ ] `deploy/config.toml` 新增 `[chains.childN]`，端口不冲突。
- [ ] 参与验证人的 `roles` 包含 `childN`。
- [ ] `deploy/keys/f*.env` 中参与节点具备对应 Aura/Grandpa key。
- [ ] `scripts/gen_child_specs.py` 新增或更新 childN 配置。
- [ ] validators 与 `deploy/config.toml` roles 一致。
- [ ] runtime binary 与业务场景匹配。
- [ ] `chainId`、`scene`、`settlement` 符合论文实验设定。
- [ ] 执行 `python3 scripts/gen_child_specs.py --only childN`。
- [ ] 检查并提交预期 spec/config diff。
- [ ] 执行普通部署或 clean redeploy。
- [ ] 使用 `wait_for_ws_chain.js` 验证持续出块。
- [ ] 查询 `aura.authorities` 并与 `deploy/keys/f*.env` 对照。
- [ ] 记录部署结论、异常和最终 block range。
