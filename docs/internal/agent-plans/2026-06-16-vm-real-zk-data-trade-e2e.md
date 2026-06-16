# VM Real ZK Data Trade E2E Execution Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 VM 集群上重新部署主链与数据交易子链 child6，验证 base data-trade、dev-zk-attested、real gnark-zk-attested 三条数据交易流程，并把真实执行结果写回文档。

**Architecture:** 使用 `deploy/config.toml` 作为 VM、链、端口和二进制的唯一配置来源；使用现有开发期 clean redeploy 脚本停止并清理 `main,child6` 数据目录；使用 `fishbone-zk` CLI 在链下生成/验证 gnark proof，再通过 `scripts/zk_real_data_trade_flow.js` 将 proof digest 与 verifier attestation 提交到 child6，并在主链 `mainEscrow` 完成资金结算。

**Tech Stack:** Rust/Substrate release binaries, Python deploy CLI, systemd/SSH, Node.js `@polkadot/api`, Go/gnark CLI, Tailscale/headscale direct subnet access.

---

## Scope

本计划只负责“刚提交的数据交易/ZK 流程能否在 VM 上真实跑通”。不在本轮实现新的论文业务 witness、电路扩展、CCMC/Merkle trustless bridge 或多 verifier quorum。

## Target Topology

- 主链 RPC：`ws://10.2.2.11:9944`
- 数据交易子链 child6 RPC：`ws://10.2.2.11:9950`
- 主链服务：`fishbone-main`
- child6 服务：`fishbone-child6`
- 主链 binary：`deploy/bin/fishbone-node`
- child6 binary：`deploy/bin/fishbone-node-data-trade`
- ZK CLI：`target/tools/fishbone-zk`

## Task 1: 本地预检与计划记录

- [x] Step 1: 确认工作区状态。
  - Run: `git status --short`
  - Expected: 没有会影响部署的未提交源码修改；若出现修改，记录到 Execution Record。

- [x] Step 2: 扫描 main/child6 相关 VM 状态。
  - Run: `bash scripts/dev_scan_vms.sh --config deploy/config.toml --chains main,child6`
  - Expected: f1-f12 可 SSH；main/child6 服务状态、监听端口、binary/spec 时间戳可见。

- [x] Step 3: 在本文件 Execution Record 中记录扫描摘要。

## Task 2: 构建部署产物与 ZK CLI

- [x] Step 1: 构建主链 release binary。
  - Run: `make build-main`
  - Expected: `deploy/bin/fishbone-node` 存在且可执行。

- [x] Step 2: 构建数据交易子链 release binary。
  - Run: `make build-data-trade-child`
  - Expected: `deploy/bin/fishbone-node-data-trade` 存在且可执行。

- [x] Step 3: 重新生成 main/child6 raw spec。
  - Run: `python3 scripts/gen_child_specs.py --only main,child6`
  - Expected: `deploy/specs/main-custom-raw.json` 与 `deploy/specs/child6-custom-raw.json` 时间戳更新。

- [x] Step 4: 构建 `fishbone-zk` CLI。
  - Run: `mkdir -p target/tools && cd tools/data-trade-zk && go build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk`
  - Expected: `target/tools/fishbone-zk` 存在且可执行。

- [x] Step 5: 本地 smoke 验证 `fishbone-zk`。
  - Run: `rm -rf target/vm-e2e-zk-smoke && target/tools/fishbone-zk fixture --out target/vm-e2e-zk-smoke --request-hash 0x1111111111111111111111111111111111111111111111111111111111111111 --session-id 0 --round-index 0 --ro-depth 10 && target/tools/fishbone-zk verify --artifact target/vm-e2e-zk-smoke/artifact.json`
  - Expected: 输出包含 `accepted`。

## Task 3: Clean Redeploy main + child6

- [x] Step 1: 停止、清理并重新部署 `main,child6`。
  - Run: `bash scripts/dev_redeploy_clean_chains.sh --chains main,child6 --config deploy/config.toml --logs`
  - Expected: deploy 命令成功，main/child6 systemd 服务启动。

- [x] Step 2: 扫描 redeploy 后状态。
  - Run: `bash scripts/dev_scan_vms.sh --config deploy/config.toml --chains main,child6`
  - Expected: f1-f5 的 `main` 与 `child6` binary/spec 使用本次构建时间；RPC 端口 `9944/9950` 在 f1 监听。

- [x] Step 3: RPC 元数据 sanity check。
  - Run: `node scripts/e2e-verify.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950`
  - Expected: 主链存在 `mainEscrow`，child6 存在 `dataRegistry/tradeSession`。
  - If command does not support these flags, record the failure and run a small inline Node RPC metadata check instead.

## Task 4: VM E2E - Base Data Trade

- [x] Step 1: 运行 happy path。
  - Run: `node scripts/data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --scenario happy`
  - Expected: 两轮交付完成，child6 claim settlement，主链 `settleByPreimage` 完成，Alice/Bob reserved balance 回到 0。

- [x] Step 2: 运行 invalid-proof path。
  - Run: `node scripts/data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --scenario invalid-proof`
  - Expected: DR dispute invalid proof，session 进入 punished，主链惩罚 DO 押金。

- [x] Step 3: 运行 requester-refuses-payment path。
  - Run: `node scripts/data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --scenario requester-refuses-payment`
  - Expected: DO claim last payment 成功。

## Task 5: VM E2E - Dev ZK Attested

- [x] Step 1: 运行 dev-zk-attested path。
  - Run: `node scripts/zk_attested_data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950`
  - Expected: 输出包含 `verifier=dev-zk-attested`；两轮交付、child6 claim、主链 settlement 完成。

## Task 6: VM E2E - Real Gnark ZK Attested

- [x] Step 1: 运行真实 gnark proof path。
  - Run: `ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950`
  - Expected: 每轮生成 artifact，CLI verify 输出 accepted，child6 `submitDataProof/attestDataProof` 成功，主链 settlement 完成。

- [x] Step 2: 检查 artifact 输出目录。
  - Run: `find target/data-trade-zk -maxdepth 3 -type f | sort | tail -40`
  - Expected: 每轮都有 `artifact.json`、proof、public witness、vk bundle 等文件。

## Task 7: 文档、问题记录与提交

- [x] Step 1: 将执行结果写回本文件 Execution Record。

- [x] Step 2: 更新 `docs/implementation/data-trade-implementation.md` 的 E2E 状态。
  - 如果 Task 6 通过，将 `scripts/zk_real_data_trade_flow.js` 状态改为 VM E2E 通过。
  - 如果 Task 6 未通过，记录具体失败阶段和下一步，不夸大状态。

- [x] Step 3: 运行最终验证。
  - Run: `SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session`
  - Run: `cd tools/data-trade-zk && go test ./...`
  - Run: `node --check scripts/data_trade_flow.js && node --check scripts/zk_attested_data_trade_flow.js && node --check scripts/zk_real_data_trade_flow.js`

- [x] Step 4: 提交文档与必要脚本修复。
  - Run: `git status --short`
  - Commit message: `test: verify data trade zk flow on vm`

## Execution Record

### 2026-06-16

- 开始执行。当前工作区只有本计划文件为新增文件，代码基线来自 commit `939d908 feat: add zk-attested data trade flow`。
- 预检扫描完成：f1-f12 SSH 可达；main 在 f1-f12 均 active，child6 在 f1-f5 active；f1 RPC 监听 `9944/9950`。远端 main/child6 binary 与 spec 时间戳仍为 2026-06-15，因此本轮需要重新构建并 clean redeploy `main,child6`。
- `make build-main` 成功，生成并复制 `deploy/bin/fishbone-node`。构建输出仅有依赖 `trie-db v0.30.0` future-incompat warning。
- `make build-data-trade-child` 成功，生成并复制 `deploy/bin/fishbone-node-data-trade`。构建输出仅有依赖 `trie-db v0.30.0` future-incompat warning。
- `python3 scripts/gen_child_specs.py --only main,child6` 成功，生成 `deploy/specs/main-custom-raw.json` 与 `deploy/specs/child6-custom-raw.json`。
- `go build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk` 成功，生成 `target/tools/fishbone-zk`。
- 本地 `fishbone-zk fixture -> verify` smoke 成功，输出 `accepted`，proof digest 为 `0xa32c1ec0aeb45a8edae72d51004d0efce2b35ee29821657bc8786e8354ebb693`。
- `bash scripts/dev_redeploy_clean_chains.sh --chains main,child6 --config deploy/config.toml --logs` 成功完成 stop-clean、binary/spec 分发、validator key 注入、systemd 服务安装与启动。
- redeploy 后扫描确认：f1-f5 的 main 与 child6 均 active；f1 监听 `9944/9950`；远端 binary/spec 时间戳更新为 2026-06-16 13:28；main/child6 数据目录清理后降至约 1.4M/1.2M。
- 原计划中的 `scripts/e2e-verify.js --main ... --child ...` 不适用：该脚本是旧 Phase 1 脚本，忽略参数并连接本地 `127.0.0.1:9944`，已中止。改用内联 Node metadata check：主链 `Fishbone Main` 暴露 `mainEscrow`，不暴露 `dataRegistry/tradeSession`；child6 `Fishbone Child-6 (Data Trade, AURA-5)` 暴露 `dataRegistry/tradeSession`，不暴露 `mainEscrow`。
- `data_trade_flow.js --scenario happy` VM E2E 通过：两轮交付后 DO 在 child6 `claimSettlement`，主链 `settleByPreimage` 成功；Bob/Alice reserved 均为 0。
- `data_trade_flow.js --scenario invalid-proof` VM E2E 通过：DR dispute invalid proof，主链 `punishDataOwner` 成功；Bob/Alice reserved 均为 0。
- `data_trade_flow.js --scenario requester-refuses-payment` VM E2E 通过：DR 拒绝支付后 DO `claimLastPayment`，主链 `claimLastPayment` 成功，DO 获得 1 轮付款。
- `zk_attested_data_trade_flow.js` VM E2E 通过：输出 `verifier=dev-zk-attested`；两轮 ZK digest/attestation 交付、child6 `claimSettlement` 与主链 `settleByPreimage` 均成功；Bob/Alice reserved 均为 0。
- 首次运行 `zk_real_data_trade_flow.js` 到 Round 0 时，`fishbone-zk fixture` 生成 proof 成功，`fishbone-zk verify` 实际也输出了 `accepted`；但脚本将 stdout 严格比较为 `accepted`，而 gnark wrapper 会在 stdout 先输出 verifier debug log，导致误判为 rejected。需要修复脚本的 accepted 判定逻辑。
- 已修复 `scripts/zk_real_data_trade_flow.js` 的 verifier 输出解析：现在要求 `status == 0` 且 stdout 中存在独立一行 `accepted`。
- 修复后重跑 `zk_real_data_trade_flow.js` 完整通过，完成两轮 gnark proof 生成/验证、链上 proof metadata、verifier attestation、child6 claim 与主链 settlement。但由于首次失败已经留下未结算 escrow，Alice/Bob reserved 仍显示非 0。为获得干净的最终证据，需要再次 clean redeploy 后单独重跑 real-zk 路径。
- 再次 clean redeploy 后，单独重跑 `zk_real_data_trade_flow.js` 通过：session `0`，两轮真实 gnark proof digest 分别为 `0x2af221087af860f4a82583f3665f9b60f715fa01cce7decb96f2ae6e5756efa8` 与 `0xac71bfbae1eb64e9489089013a53ddbb029bfff1b32caa52141e1402d168ead6`；child6 `claimSettlement` 与主链 `settleByPreimage` 成功；Bob/Alice reserved 均为 0。
- `find target/data-trade-zk -maxdepth 3 -type f` 确认成功轮次 artifact 存在，包括 `artifact.json`、`ch_range.proof/public/vk`、`ro_depth10.proof/public/vk` 与 `vk_bundle.bin`。`target/data-trade-zk` 中同时保留了失败尝试和成功尝试的本地临时 artifact，不纳入 git。
- 已更新 `docs/implementation/data-trade-implementation.md`：`dev-zk-attested` 与 `gnark-groth16-bn254` 均标记为 2026-06-16 VM 验证通过，并新增对应 VM E2E 结果行。
- 最终验证通过：`SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session` 19/19 pass；`cd tools/data-trade-zk && go test ./...` pass；`node --check scripts/data_trade_flow.js scripts/zk_attested_data_trade_flow.js scripts/zk_real_data_trade_flow.js` pass；`git diff --check` pass。Rust 仍有依赖 `trie-db v0.30.0` future-incompat 提醒，不是本次代码 warning。
