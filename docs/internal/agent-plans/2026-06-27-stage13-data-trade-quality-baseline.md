# Stage 13 数据交易全流程质量基线验证计划

> **执行负责人：** Codex。
>
> **用户角色：** 审阅本计划，确认后由 Codex 执行。
>
> **CodeWhale 角色：** 本阶段不参与。Stage 13 需要根据测试结果、环境状态和链上可用性做动态判断，不适合拆给执行型 agent。

## 目标

对 Stage 8-12 已完成的数据交易脚本化闭环做一次全局质量验证，回答三个问题：

1. 当前代码质量是否足以支撑论文实验和后续演示？
2. 脚本级完整数据交易流程是否已经能在无链和有链环境下稳定复现？
3. 如果不能完整复现，问题是代码缺陷、环境不可用、链状态不匹配，还是仍缺少实现？

本阶段不是新增功能阶段，而是一次**质量基线冻结**。最终产物应是一份可审阅的验证报告，明确哪些通过、哪些未运行、哪些失败、失败原因是什么。

## 当前基线

截至 Stage 12 合并后，项目已经具备：

- 动态 dataset/request 输入。
- `range` 与 `multi_range` 约束。
- `fishbone-zk` 链下 gnark Groth16 proof 生成与验证。
- `zk_real_data_trade_flow.js` 动态 happy path。
- `--dry-run-dynamic` 无链 proof pipeline 验证。
- Stage 11 三个失败/争议场景：
  - `invalid-proof-dispute`
  - `invalid-plaintext-dispute`
  - `requester-refuses-payment`
- Stage 12 demo guide 与 evidence index。

仍明确不是目标或尚未实现：

- 前端 DO/DR 操作台。
- on-chain Groth16 verifier。
- trustless bridge / CCMC / Merkle proof settlement。
- verifier quorum / threshold attestation。
- production dynamic IMT。
- subset/substr 约束。
- production timeout/challenge-period dispute 机制。

## 成功标准

Stage 13 通过，不要求所有 live-chain 命令都必须成功。通过标准是：

- 所有本地静态/单元/构建检查结果明确。
- Stage 12 dry-run demo matrix 可以复现，或失败原因明确且可定位。
- 负向验证确实在链交互前 reject。
- RPC/live-chain 状态被实际探测并记录。
- 如果 live-chain 可用：
  - 至少尝试 Stage 12 live happy path；
  - 条件允许时尝试三个 Stage 11 failure/dispute 场景；
  - 记录 evidence 路径、结果字段、关键事件。
- 如果 live-chain 不可用：
  - 明确记录 RPC 不可用，不把 dry-run 当作 live-chain 证据。
- 不夸大安全边界。
- 不引入无关功能改动。

## 非目标

Stage 13 不做以下事情：

- 不新增 runtime/pallet/circuit/artifact schema。
- 不修改 proof digest 或 attestation digest 编码。
- 不实现前端。
- 不实现 on-chain verifier、trustless bridge、verifier quorum。
- 不把 `target/` 下大规模生成物提交到 git。
- 不做 destructive clean redeploy，除非用户明确批准。
- 不为了让测试通过而绕过失败检查。

## 分支与提交策略

建议分支：

```text
feat/data-trade-stage13-quality-baseline
```

提交策略：

1. 计划提交：
   - `plan: define Stage 13 data trade quality baseline`
2. 执行完成后，根据结果提交验证报告：
   - `test: record Stage 13 data trade quality baseline`
3. 如果发现小型文档口径问题，可随验证报告一起修正。
4. 如果发现代码缺陷：
   - 先记录在报告中；
   - 只有当修复很小、风险低、且直接阻塞验证时，才在 Stage 13 内修；
   - 较大修复另开 Stage 14 或单独 fix 分支。

注意：当前工作区可能已有 `agent.md` 未提交改动。Stage 13 不应自动提交或回滚它，除非用户明确要求。

## 产物

新增一份验证报告：

```text
docs/internal/agent-reviews/2026-06-27-data-trade-stage13-quality-baseline.md
```

报告必须包含：

- 执行分支、commit、时间。
- 命令清单与结果。
- dry-run evidence 路径与关键字段。
- negative validation 的退出码与关键错误。
- RPC 探测结果。
- live-chain 命令是否运行。
- 如果 live-chain 运行：
  - evidence 路径；
  - `result` 字段；
  - session/escrow/listing id；
  - 关键事件；
  - 失败时的错误栈和判断。
- 如果 live-chain 未运行：
  - 明确原因，例如 RPC unavailable / not approved / metadata mismatch。
- 结论：
  - 当前是否可以作为论文 dry-run 证据；
  - 当前是否可以作为 live-chain 证据；
  - 下一步建议。

## 执行步骤

### Step 0：准备与状态冻结

执行：

```bash
git status --short --branch
git log --oneline -8 --decorate
git branch --show-current
```

记录：

- 当前分支。
- 当前 HEAD。
- 是否存在未提交改动。

如果存在非 Stage 13 文件改动，例如 `agent.md`，只记录，不提交、不回滚。

### Step 1：静态与语法检查

执行 JS syntax checks：

```bash
node --check scripts/data_trade_flow.js
node --check scripts/zk_attested_data_trade_flow.js
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/bridges/data_trade.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_attestation.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/data_trade_events.js
node --check scripts/lib/data_trade_binding.js
node --check scripts/lib/hash_chain.js
node --check scripts/lib/data_trade_sample.js
node --check scripts/lib/vm_regression_summary.js
node --check scripts/lib/wait_for_ws_chain.js
```

执行 shell syntax checks：

```bash
bash -n scripts/run_data_trade_vm_regression.sh
```

如果某个脚本不存在或已移动，记录实际情况，不盲目跳过。

### Step 2：Rust pallet 单元测试

执行：

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-data-registry
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session
SKIP_WASM_BUILD=1 cargo test -p pallet-main-escrow
```

记录：

- passed/failed。
- 测试数量。
- 失败详情。

如果出现非数据交易相关的 workspace 问题，先记录，不立即扩大范围跑全 workspace。

### Step 3：Go ZK 工具链测试与构建

执行：

```bash
go -C tools/data-trade-zk test ./...
go -C tools/data-trade-zk build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
```

记录：

- Go test 结果。
- build 结果。
- `target/tools/fishbone-zk` 是否生成。

### Step 4：Stage 12 dry-run demo matrix

使用临时目录，避免污染 Stage 12 约定路径：

```text
/tmp/fishbone-stage13-quality/
```

执行三条 positive dry-run：

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --evidence-out /tmp/fishbone-stage13-quality/factory-temperature-dry-run/evidence.json \
  --dry-run-dynamic
```

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --evidence-out /tmp/fishbone-stage13-quality/factory-multi-range-dry-run/evidence.json \
  --dry-run-dynamic
```

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/vehicle_telematics.json \
  --request scripts/fixtures/data_trade_requests/vehicle_speed_range.json \
  --evidence-out /tmp/fishbone-stage13-quality/vehicle-speed-dry-run/evidence.json \
  --dry-run-dynamic
```

对每个 evidence 读取并记录：

- `scenario`
- `mode`
- `result`
- `request_hash`
- `listing_id`
- `escrow_id`
- `session_id`
- `settlement`
- `rounds[0].constraint_kind`
- `rounds[0].constraints.length`
- 每个 constraint 的 `proof_digest` 与 `business_input_hash`

预期：

- `mode = dynamic-dry-run`
- `result = dry-run-accepted`
- `listing_id = null`
- `escrow_id = null`
- `settlement = null`
- `session_id = 0`
- `multi_range` evidence 有 2 个 constraints

### Step 5：负向 dry-run validation

执行：

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_out_of_range.json \
  --dry-run-dynamic
```

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range_out_of_range.json \
  --dry-run-dynamic
```

记录：

- 退出码。
- stderr/stdout 关键错误。
- 是否在 `make-witness` 阶段失败。

预期：

- exit non-zero。
- 不发生链连接。
- 不生成有效 evidence。

### Step 6：RPC/live-chain 可用性探测

先执行 read-only 探测：

```bash
node scripts/lib/wait_for_ws_chain.js \
  --main ws://10.2.2.11:9944 \
  --child ws://10.2.2.11:9950 \
  --min-blocks 1 \
  --timeout-ms 30000
```

记录：

- main RPC 是否可用。
- child6 RPC 是否可用。
- 当前块高或错误信息。

决策：

- 如果 RPC 不可用：停止 live-chain 部分，进入报告汇总。
- 如果 RPC 可用：继续 Step 7。
- 如果 RPC 可用但 metadata 明显不匹配当前脚本：停止 live-chain 部分，记录为 metadata mismatch，不做 destructive redeploy。

### Step 7：live-chain happy path

仅在 Step 6 通过时执行。

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --evidence-out /tmp/fishbone-stage13-quality/live-happy-multi-range/evidence.json
```

记录 evidence：

- `result`
- `listing_id`
- `escrow_id`
- `session_id`
- `settlement`
- 每轮 constraints。

预期：

- `result = accepted`
- 两个 completed rounds。
- settlement 完成。

如果失败：

- 保存错误日志。
- 判断是代码问题、链状态问题、余额/nonce 问题、metadata mismatch，还是 RPC 中断。
- 不直接重置链。

### Step 8：live-chain failure/dispute 场景

仅在 Step 6 通过，且 Step 7 不显示环境严重异常时执行。

依次尝试：

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --scenario invalid-proof-dispute \
  --evidence-out /tmp/fishbone-stage13-quality/live-invalid-proof/evidence.json
```

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --scenario invalid-plaintext-dispute \
  --evidence-out /tmp/fishbone-stage13-quality/live-invalid-plaintext/evidence.json
```

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --scenario requester-refuses-payment \
  --evidence-out /tmp/fishbone-stage13-quality/live-requester-refuses-payment/evidence.json
```

预期：

| 场景 | 预期 result | 预期事件 |
|------|-------------|----------|
| `invalid-proof-dispute` | `expected-dispute-accepted` | `tradeSession.SessionPunished`, `mainEscrow.EscrowPunished` |
| `invalid-plaintext-dispute` | `expected-plaintext-dispute-accepted` | `tradeSession.SessionPunished`, `mainEscrow.EscrowPunished` |
| `requester-refuses-payment` | `expected-last-payment-claimed` | `tradeSession.LastPaymentClaimed`, `mainEscrow.EscrowSettled` |

如果某个场景失败：

- 不继续强行跑后续场景，除非失败明显是该场景独有且不会污染链状态。
- 记录错误并判断原因。

### Step 9：报告与结论

写入：

```text
docs/internal/agent-reviews/2026-06-27-data-trade-stage13-quality-baseline.md
```

报告结论分级：

```text
green   本地检查 + dry-run + live-chain 全通过
yellow  本地检查 + dry-run 通过，但 live-chain 因环境不可用未运行
orange  本地检查通过，但 dry-run 或 live-chain 暴露可修复问题
red     核心单元测试/构建失败，当前不适合继续论文实验证据整理
```

报告必须明确：

- 当前能否作为论文 dry-run 证据。
- 当前能否作为论文 live-chain 证据。
- 是否建议进入前端阶段。
- 是否建议先开修复 stage。

## 可能结果与处理

### 结果 A：本地检查与 dry-run 全通过，RPC 不可用

这是目前最可能结果。

处理：

- 报告标记 `yellow`。
- 当前代码质量可支撑论文 dry-run/proof pipeline 证据。
- live-chain 证据仍停留在历史 VM 结果，不新增 Stage 13 live-chain claim。
- 下一步可选择：
  - 准备一个受控 VM 环境，专门跑 live-chain validation；
  - 或先进入论文写作，明确 live-chain evidence 来源。

### 结果 B：本地检查与 dry-run 全通过，live-chain 也通过

处理：

- 报告标记 `green`。
- 固化 live-chain evidence 路径和摘要。
- 可以进入论文实验证据整理或前端规划。

### 结果 C：本地检查通过，但 dry-run 失败

处理：

- 报告标记 `orange` 或 `red`，取决于失败范围。
- 优先修复，因为 dry-run 是无链基础能力，不能忽略。

### 结果 D：Rust/Go/JS 基础检查失败

处理：

- 报告标记 `red`。
- 不执行 live-chain。
- 先开修复任务。

### 结果 E：live-chain 失败但 dry-run 通过

处理：

- 判断失败性质：
  - RPC/environment 问题：`yellow`
  - metadata/runtime mismatch：`orange`
  - 脚本逻辑问题：`orange`
  - pallet 状态机 bug：`red`
- 不做 destructive redeploy，除非用户另行批准。

## 审阅关注点

请重点审阅：

- 是否允许我在 RPC 可用时运行 live-chain happy path 和 dispute 场景。
- 是否坚持“不做 clean redeploy”，还是你希望我在必要时申请后执行 redeploy。
- 报告路径是否合适。
- 是否需要把 `agent.md` 的 Stage 12 lessons learned 一并纳入某个正式提交，还是继续保留为本地工作区改动。

## 当前建议

我建议你批准后按本计划执行，并保持默认策略：

- 允许 read-only RPC 探测。
- RPC 可用时允许运行 live-chain demo commands。
- 不允许 destructive redeploy，除非你另行明确批准。
- 不提交 `target/` evidence，只在报告中记录路径和摘要。
- `agent.md` 暂时不纳入 Stage 13，避免混入验证报告提交。
