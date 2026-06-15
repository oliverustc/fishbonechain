# VM Redeploy Data Trade E2E Execution Plan

> **For agentic workers:** Use the current workspace. Do not create a separate worktree for this execution because the VM deploy must use the current build outputs and current uncommitted data-trade scripts.

**Goal:** Build deployable main/data-trade binaries, clean redeploy main + child6 to the VM cluster, and run the data-trade E2E scenarios against the live RPC endpoints.

**Architecture:** Use the existing `Makefile` targets to produce `deploy/bin/fishbone-node` and `deploy/bin/fishbone-node-data-trade`. Use `scripts/dev_redeploy_clean_chains.sh` to stop-clean-deploy only `main,child6`. Then run the base attested E2E scenarios and the zk-attested dev verifier scenario against `ws://10.2.2.11:9944` and `ws://10.2.2.11:9950`.

**Tech Stack:** Rust/Substrate release builds, existing deploy Python CLI, Node.js `@polkadot/api` E2E scripts, Tailscale/headscale direct SSH/RPC network.

---

## Tasks

- [x] **Task 1: Preflight**
  - Run `git status --short` and confirm expected dirty files are present.
  - Run `bash scripts/dev_scan_vms.sh --config deploy/config.toml` to capture current VM state.

- [x] **Task 2: Build and copy deploy binaries**
  - Run `make build-main`.
  - Run `make build-data-trade-child`.
  - Verify `deploy/bin/fishbone-node` and `deploy/bin/fishbone-node-data-trade` exist and are executable.

- [x] **Task 3: Clean redeploy main + child6**
  - Run `bash scripts/dev_redeploy_clean_chains.sh --chains main,child6 --config deploy/config.toml --logs`.
  - Verify RPC reachability for `MAIN_WS=ws://10.2.2.11:9944` and `CHILD6_WS=ws://10.2.2.11:9950`.

- [x] **Task 4: Run base attested E2E scenarios**
  - Run `happy`.
  - Run `invalid-proof`.
  - Run `requester-refuses-payment`.
  - Check terminal statuses and balance log summaries.

- [x] **Task 5: Run zk-attested E2E scenario**
  - Run `scripts/zk_attested_data_trade_flow.js`.
  - Confirm output includes `verifier=zk-attested`.

- [x] **Task 6: Record result**
  - Append command results, failures, and fixes to this file under Execution Record.

## Execution Record

### 2026-06-15

- 已执行 `git status --short` 与 `bash scripts/dev_scan_vms.sh --config deploy/config.toml`。VM 扫描显示 main 与 child6 原先均在运行，但远端二进制/spec 时间戳旧于本次构建，因此本次采用“重新构建 binary + 重新生成 raw spec + clean redeploy”的顺序。
- 已执行 `make build-main`，产物复制到 `deploy/bin/fishbone-node`。
- 已执行 `make build-data-trade-child`，产物复制到 `deploy/bin/fishbone-node-data-trade`。
- 已执行 `python3 scripts/gen_child_specs.py --only main,child6`，用新 binary 生成：
  - `deploy/specs/main-custom-raw.json`
  - `deploy/specs/child6-custom-raw.json`
- 首次执行 `bash scripts/dev_redeploy_clean_chains.sh --chains main,child6 --config deploy/config.toml --logs` 时失败，根因是 wrapper 将相对路径 `deploy/config.toml` 传给下游 Typer 命令，下游解析 cwd 后找不到文件。已修复 `scripts/dev_redeploy_clean_chains.sh`：参数解析后将 config 规范化为绝对路径，再构造 stop/deploy 参数。
- 已成功 clean redeploy `main,child6`。RPC 探测结果：
  - `ws://10.2.2.11:9944`: chain=`Fishbone Main`，存在 `mainEscrow`，不存在 `dataRegistry/tradeSession`
  - `ws://10.2.2.11:9950`: chain=`Fishbone Child-6 (Data Trade, AURA-5)`，存在 `dataRegistry/tradeSession`，不存在 `mainEscrow`
- 首次运行 `happy` 到最后主链 `settleByPreimage` 失败，错误为 `mainEscrow.InvalidHashPreimage`。根因有两点：
  - `scripts/data_trade_flow.js` 和 `scripts/zk_attested_data_trade_flow.js` 在主链结算时传入了原始 `seed`，而不是 `H^(remaining)(seed)` preimage。
  - `scripts/lib/hash_chain.js` 多轮 hash 时把上一轮 hex 字符串继续 hash，和 runtime/pallet 测试中的 `H256.encode()` 字节链不一致。
- 已修复 hash-chain 工具，使 JS 与 runtime 验证逻辑一致：第一轮 hash 原始 seed bytes，后续 hash 上一轮 32-byte H256 bytes。已增加本地自检：`H^2(paymentPreimageForRemaining(seed, 1)) == H^3(seed)`。
- 已修复两个 E2E 脚本的 `settleByPreimage` 参数，改为传入 `paymentPreimageForRemaining(...)`；同时修复 `submitTx` 在 dispatch error 后仍打印成功日志的问题。
- 修复后再次 clean redeploy，并完成以下 VM E2E：
  - `node scripts/data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --scenario happy`：通过，Bob/Alice reserved 均回到 0。
  - `node scripts/data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --scenario invalid-proof`：通过，DO 押金惩罚，DR 锁定资金释放。
  - `node scripts/data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --scenario requester-refuses-payment`：通过，DO 可 claim last payment。
  - `node scripts/zk_attested_data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950`：通过，输出包含 `verifier=zk-attested`，完成 child6 claim 与主链 escrow settlement。

### 当前遗留/建议

- `zk_attested_data_trade_flow.js` 目前验证的是 dev verifier/attestation 链路，不是完整密码学 ZK proof verifier。后续需要单独计划实现真实 proof 生成、链下验证客户端、verification key 管理与链上 attestation 约束。
- 部署脚本日志显示启动 child6 阶段打印了 12 次 `services started`，虽然实际 RPC 与配置可用，但建议后续审阅 `deploy/cmd` 的链过滤输出，避免多链/多 VM 扩展时日志误导。
