# Stage 13 子链 6 环境恢复记录

日期：2026-06-27
执行分支：`feat/data-trade-stage13-quality-baseline`
执行负责人：Codex

## 结论

子链 6 已恢复。

最终验证结果：

- main RPC：`ws://10.2.2.11:9944` 可连接并持续出块。
- child6 RPC：`ws://10.2.2.11:9950` 可连接并持续出块。
- child6 链名：`Fishbone Child-6 (Data Trade, AURA-5)`。
- child6 Aura authorities 已恢复为 f1-f5 五个预期密钥。

本次只清理并重部署了 child6，未清理 main 链。

## 背景

Stage 13 质量基线执行时，live-chain 验证未运行，原因是 child6 RPC 在探测窗口内不可用。用户要求先恢复子链 6 环境。

## 诊断过程

初始观察：

- 当前机器无本地 child6 节点进程和端口。
- 实际链 RPC 指向远端 VM：
  - main：`ws://10.2.2.11:9944`
  - child6：`ws://10.2.2.11:9950`
- 远端 f1-f5 起初未发现可用的 `fishbone-child6.service`，旧日志停留在 2026-06-19。

第一次按现有脚本部署 child6 后，服务和 RPC 恢复可达，但 child6 卡在 `#0`，没有继续出块。

进一步检查发现，已部署 child6 genesis 中的 `aura.authorities` 是错误的三密钥集合，而不是 f1-f5 的五个密钥集合。这会导致当前 f1-f5 节点无法按预期参与出块。

错误 authorities：

```text
0x6c849d3961003bbfef2a77dad84a43762a2baff5fd281631837eaa54e63a7e1c
0xbc5d58d23265ad04304751a7cc5fdbeaea55bc10fafa023e1b3c4ecfa0eef559
0x8a96d6f96be9f6e978de9562b8eeb12ec0b205dd34cf3e2f4c6136bddefc4c78
```

## 恢复动作

执行过一次错误路径的部署命令：

```bash
scripts/dev_deploy_chains.sh --chains child6 --config deploy/config.toml
```

该命令失败，原因是脚本会进入 `deploy/` 目录后再解析配置路径，导致实际查找 `deploy/deploy/config.toml`。

随后执行非清理部署：

```bash
scripts/dev_deploy_chains.sh --chains child6
```

服务恢复，但链卡在 `#0`。确认原因是 child6 genesis authorities 与 f1-f5 不匹配后，重新生成 child6 spec：

```bash
python3 scripts/gen_child_specs.py --only child6
```

然后执行 child6-only clean redeploy：

```bash
scripts/dev_redeploy_clean_chains.sh --chains child6 --logs
```

影响范围说明：

- 清理范围仅限 child6 的链数据和日志。
- main 链没有清理、没有重部署。
- `deploy/specs/child6-custom-raw.json` 重新生成后与仓库当前版本无差异，因此没有 spec 文件变更需要提交。

## 最终验证

readiness 检查：

```bash
node scripts/lib/wait_for_ws_chain.js \
  --main ws://10.2.2.11:9944 \
  --child ws://10.2.2.11:9950 \
  --min-blocks 2 \
  --timeout-ms 120000
```

结果：

```text
main Fishbone Main #102804 -> #102806
child Fishbone Child-6 (Data Trade, AURA-5) #23 -> #25
```

child6 authority 检查：

```text
chain Fishbone Child-6 (Data Trade, AURA-5)
number 28
0xe26f721e6c3e5915a0c427d7c3ca2485e79fdc4c093fa1fb3155505ced337723
0xba74d77fabd2a58c63807de150a502ba8bfdbd1ab2bffba250ac5d8258369a37
0x1ee9d630c9374b09320a4ac53de4626357a45acc88c709e994f5c6f3618a5a2f
0x184160cc1483e0dc4151f8f5dc3596c91411bb63baaf281e6788454b1fc73b1f
0xe2ee5b7dec316ca4e40ab81d3d01bd88de4b7f0c0703f60e07d3ddd33400f966
```

这些 keys 与 `deploy/keys/f1.env` 到 `deploy/keys/f5.env` 的 child6 Aura keys 一致。

## 后续建议

child6 已具备继续执行 Stage 13 live-chain 验证的前置条件。下一步可以运行 Stage 12 happy path live-chain 流程，并在通过后补充 Stage 13 质量基线报告中的 live-chain 证据。
