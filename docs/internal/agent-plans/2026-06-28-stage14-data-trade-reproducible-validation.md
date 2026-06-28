# Stage 14 数据交易实验固化与一键复现执行计划

日期：2026-06-28
计划负责人：Codex
建议执行者：CodeWhale
长期路线图：`docs/internal/agent-plans/2026-06-28-data-flow-platform-long-term-roadmap.md`
前置基线：Stage 13 已合并到 `main`，结论为 `green`

## 1. 背景与目标

Stage 13 已经证明：通过脚本可以在 child6 上完成数据交易的链上/链下闭环，包括：

- dry-run proof pipeline。
- live-chain happy path。
- `invalid-proof-dispute`。
- `invalid-plaintext-dispute`。
- `requester-refuses-payment`。

但 Stage 13 的执行仍然依赖人工复制命令，证据散落在 `/tmp/fishbone-stage13-quality/`，报告是 agent review 形式。Stage 14 的目标是把这套已跑通能力固化为可复现、可审计、可继续扩展到平台化后端的实验资产。

Stage 14 不追求新增协议能力，而是建立第一套“业务流程验证模板”。后续数据收集、跨域流通、可验证训练也应能复用这套 evidence/summary 思路。

## 2. 当前代码状态

当前已有关键入口：

- `scripts/zk_real_data_trade_flow.js`
  - 支持 `--profile child6-data-trade`
  - 支持 `--dataset`
  - 支持 `--request`
  - 支持 `--dry-run-dynamic`
  - 支持 `--scenario happy|invalid-proof-dispute|invalid-plaintext-dispute|requester-refuses-payment`
  - 支持 `--evidence-out`
- `scripts/lib/wait_for_ws_chain.js`
  - 可检查 main/child RPC 是否持续出块
- `scripts/lib/vm_regression_summary.js`
  - 可作为 summary 记录器参考，但当前字段太简单，且文件名偏 VM regression
- `scripts/run_data_trade_vm_regression.sh`
  - 会默认 clean redeploy `main,child6`，不适合作为 Stage 14 论文复现实验脚本的默认行为
- `docs/implementation/data-trade-demo-guide.md`
  - Stage 12 demo 命令文档，live-chain 部分当时未运行
- `docs/implementation/data-trade-stage12-evidence-index.md`
  - Stage 12 evidence index，live-chain 标记为 not run
- `docs/internal/agent-reviews/2026-06-27-data-trade-stage13-quality-baseline.md`
  - Stage 13 green 基线和 live-chain evidence 摘要

## 3. 本 Stage 成功标准

Stage 14 完成后，应满足：

1. 有一个默认非破坏性的一键验证脚本，可以自动执行 Stage 13 的 dry-run、negative validation、live-chain happy path 和三个 failure/dispute 场景。
2. 脚本输出稳定目录结构，包含每个 scenario 的 `run.log`、`evidence.json` 或 negative validation log。
3. 脚本自动生成 `summary.json` 和 `summary.md`。
4. `summary.json` 采用面向未来平台的通用 evidence 视角，至少包含：
   - run metadata
   - environment
   - scenario list
   - command
   - status
   - evidence path
   - scenario
   - result
   - listing_id / escrow_id / session_id
   - settlement
   - scenario_outcome
   - key events
   - constraints summary
   - error summary
5. 文档说明如何复现、如何阅读 evidence、哪些结论可以写进论文、哪些仍是 prototype / future work。
6. 不引入新协议功能，不改变 Stage 13 已通过的核心交易脚本行为。

## 4. 非目标

Stage 14 不做：

- 不新增 pallet/runtime 功能。
- 不新增 ZK 电路。
- 不实现 Web 后端。
- 不实现前端。
- 不实现链上 Groth16 verifier。
- 不实现 trustless bridge。
- 不实现 verifier quorum。
- 不新增 subset/substr/aggregation 约束。
- 不默认 clean redeploy main 或 child6。
- 不提交 `/tmp`、`target/data-trade-zk/`、大体积 proof artifact。

## 5. 文件级计划

### 新增文件

建议新增：

```text
scripts/run_data_trade_validation.sh
scripts/lib/data_trade_validation_summary.js
docs/experiments/data-trade-validation.md
docs/implementation/data-trade-stage14-evidence-index.md
docs/internal/agent-reviews/2026-06-28-data-trade-stage14-code-review.md   (由 Codex review 时生成，不由 CodeWhale 创建)
```

### 可修改文件

允许修改：

```text
docs/implementation/data-trade-demo-guide.md
docs/implementation/data-trade-evidence.md
docs/implementation/data-trade-paper-gap-matrix.md
```

只在必要时修改，并且不得把 Stage 14 文档变成过度宣传。

### 默认不应修改

除非发现直接阻塞 Stage 14 的小 bug，否则不要修改：

```text
scripts/zk_real_data_trade_flow.js
pallets/*
runtime/*
tools/data-trade-zk/*
```

如果必须修改 `scripts/zk_real_data_trade_flow.js`，应先确认：

- 不是为了重构而重构。
- 不改变 Stage 13 通过的命令语义。
- 有对应 `node --check` 和至少一条 dry-run 验证。

## 6. 输出目录规范

默认输出目录：

```text
target/data-trade-validation/stage14-<timestamp>/
```

脚本应支持：

```bash
scripts/run_data_trade_validation.sh --out /tmp/fishbone-data-trade-validation
```

建议目录结构：

```text
<out>/
  summary.json
  summary.md
  commands.log
  readiness/
    run.log
  dry-run/
    factory-temperature/
      run.log
      evidence.json
    factory-multi-range/
      run.log
      evidence.json
    vehicle-speed/
      run.log
      evidence.json
  negative/
    factory-temperature-out-of-range/
      run.log
    factory-multi-range-out-of-range/
      run.log
  live/
    happy-multi-range/
      run.log
      evidence.json
    invalid-proof/
      run.log
      evidence.json
    invalid-plaintext/
      run.log
      evidence.json
    requester-refuses-payment/
      run.log
      evidence.json
  postcheck/
    run.log
```

## 7. CLI 设计

`scripts/run_data_trade_validation.sh` 应支持：

```bash
scripts/run_data_trade_validation.sh [options]

Options:
  --profile child6-data-trade       Trade profile. Default: child6-data-trade.
  --main ws://...                   Override main RPC.
  --child ws://...                  Override child RPC.
  --out PATH                        Output directory.
  --zk-cmd PATH                     ZK verifier command. Default: target/tools/fishbone-zk.
  --skip-live                       Run only syntax/build/dry-run/negative checks.
  --skip-dry-run                    Skip dry-run matrix.
  --skip-negative                   Skip negative validation.
  --timeout-seconds N               Per-scenario timeout. Default: 300.
  --no-build-zk                     Do not auto-build target/tools/fishbone-zk.
  -h, --help                        Show help.
```

默认行为：

- 不执行 deploy。
- 不执行 clean redeploy。
- 如果 `target/tools/fishbone-zk` 不存在且未指定 `--no-build-zk`，自动构建。
- live-chain 前先运行 readiness check。
- 如果 readiness 失败，live scenarios 标记为 `skipped` 或 `failed`，不要伪造 evidence。

## 8. Summary schema 初稿

`summary.json` 建议结构：

```json
{
  "version": 1,
  "kind": "data_trade_validation",
  "stage": "stage14",
  "status": "passed|failed|partial",
  "started_at": "...",
  "finished_at": "...",
  "environment": {
    "profile": "child6-data-trade",
    "main_ws": "ws://10.2.2.11:9944",
    "child_ws": "ws://10.2.2.11:9950",
    "zk_cmd": "target/tools/fishbone-zk",
    "git_commit": "...",
    "git_branch": "..."
  },
  "scenarios": [
    {
      "id": "live-happy-multi-range",
      "category": "live-chain",
      "status": "passed",
      "command": "...",
      "log_path": "...",
      "evidence_path": "...",
      "scenario": "happy",
      "result": "accepted",
      "listing_id": 0,
      "escrow_id": 0,
      "session_id": 0,
      "settlement": {
        "completed_rounds": 2,
        "remaining_rounds": 1
      },
      "scenario_outcome": null,
      "events": [],
      "constraints": [
        {
          "round_index": 0,
          "field_name": "temperature",
          "proof_digest": "...",
          "business_input_hash": "...",
          "on_chain_bound": true
        }
      ],
      "error": null
    }
  ]
}
```

注意：

- `summary.json` 是平台化 evidence metadata 的雏形，不是最终 Web 后端数据库 schema。
- 字段应尽量通用，后续数据收集/跨域/训练也能参考。
- 不要把完整 proof artifact 塞进 summary。

## 9. 实现步骤

### Step 1：创建 Stage 14 分支

建议：

```bash
git checkout -b feat/data-trade-stage14-reproducible-validation
```

确认：

```bash
git status --short --branch
```

如果存在用户未提交改动，只记录，不提交、不回滚。

### Step 2：实现 summary 工具

新增：

```text
scripts/lib/data_trade_validation_summary.js
```

职责：

- `init`
- `record`
- `finish`
- 从 `evidence.json` 抽取摘要
- 写 `summary.json`
- 写 `summary.md`

建议 CLI：

```bash
node scripts/lib/data_trade_validation_summary.js init --json <summary.json> ...
node scripts/lib/data_trade_validation_summary.js record --json <summary.json> --scenario-id ... --status ... --log ... --evidence ...
node scripts/lib/data_trade_validation_summary.js finish --json <summary.json> --markdown <summary.md> --status passed
```

实现要求：

- 使用 Node 标准库即可。
- 不引入新 npm 依赖。
- 读取 evidence 失败时，scenario 状态应记录为 failed，并保留错误摘要。
- Markdown summary 应包含 scenario 表格。

### Step 3：实现一键验证脚本

新增：

```text
scripts/run_data_trade_validation.sh
```

实现要求：

- `set -euo pipefail`
- 支持第 7 节 CLI 参数。
- 自动创建输出目录。
- 每个 scenario 的 stdout/stderr 写入对应 `run.log`。
- 每个 scenario 调用前把完整命令写入 `commands.log`。
- 每个 scenario 执行后调用 summary 工具记录状态。
- negative validation 预期 exit non-zero；如果命令 exit 0，应判定为 failed。
- live scenarios 默认按顺序执行：
  1. `happy`
  2. `invalid-proof-dispute`
  3. `invalid-plaintext-dispute`
  4. `requester-refuses-payment`
- 如果 live happy path 失败，后续 live failure/dispute 默认不继续跑，除非实现了显式 `--continue-on-live-failure`。Stage 14 可以不实现该参数。
- 如果 readiness 失败，live scenarios 标记 skipped，整体状态应为 `partial`，不是 `passed`。

必须避免：

- 不要调用 `scripts/dev_redeploy_clean_chains.sh`。
- 不要调用 `scripts/dev_deploy_chains.sh`。
- 不要删除链数据。

### Step 4：基础检查

执行：

```bash
node --check scripts/lib/data_trade_validation_summary.js
bash -n scripts/run_data_trade_validation.sh
node --check scripts/zk_real_data_trade_flow.js
```

如果修改了其它 JS，也要加入 `node --check`。

### Step 5：dry-run / negative 快速验证

先跑无链部分：

```bash
scripts/run_data_trade_validation.sh \
  --skip-live \
  --out /tmp/fishbone-stage14-dry-run
```

预期：

- 三个 dry-run scenario passed。
- 两个 negative validation passed as expected。
- `summary.json` status 为 `passed`。
- `summary.md` 可读。

### Step 6：live-chain 验证

如果 child6 可用，执行完整验证：

```bash
scripts/run_data_trade_validation.sh \
  --profile child6-data-trade \
  --main ws://10.2.2.11:9944 \
  --child ws://10.2.2.11:9950 \
  --out /tmp/fishbone-stage14-full
```

预期：

- readiness passed。
- live happy path `result = accepted`。
- `invalid-proof-dispute` `result = expected-dispute-accepted`。
- `invalid-plaintext-dispute` `result = expected-plaintext-dispute-accepted`。
- `requester-refuses-payment` `result = expected-last-payment-claimed`。
- postcheck passed。
- `summary.json` status 为 `passed`。

如果 RPC 不可用：

- 不做 redeploy。
- 报告为 `partial`。
- 文档明确 live-chain 未运行原因。

### Step 7：实验文档

新增：

```text
docs/experiments/data-trade-validation.md
```

内容应包括：

- 实验目的。
- 复现命令。
- 输出目录说明。
- scenario matrix。
- evidence 字段解释。
- Stage 13/14 当前能力边界。
- 论文可使用结论。
- prototype / future work 边界。

不要写成 agent 过程记录。它应是面向论文和后续开发者的正式实验说明。

### Step 8：Evidence index

新增：

```text
docs/implementation/data-trade-stage14-evidence-index.md
```

内容应包括：

- 标准输出目录结构。
- 每个 scenario 的 id/category/input/expected result/expected evidence。
- `summary.json` 字段说明。
- 与未来平台对象的映射：
  - `summary.scenarios[]` -> `WorkflowRun` / `Evidence`
  - `listing_id` / `session_id` / `escrow_id` -> `ChainEvent` / chain state reference
  - `log_path` / `evidence_path` -> artifact metadata
- 不提交生成物的说明。

### Step 9：更新现有文档

按需更新：

```text
docs/implementation/data-trade-demo-guide.md
docs/implementation/data-trade-evidence.md
docs/implementation/data-trade-paper-gap-matrix.md
```

最低要求：

- `data-trade-demo-guide.md` 增加 Stage 14 一键复现入口。
- 如果 `data-trade-stage12-evidence-index.md` 仍写 live-chain not run，可以保留历史语境，但应指向 Stage 14/Stage 13 green 结果，避免读者误解当前状态。
- `paper-gap-matrix` 如有“live-chain 未跑”之类陈旧描述，需要修正。

### Step 10：最终检查

必须执行：

```bash
node --check scripts/lib/data_trade_validation_summary.js
bash -n scripts/run_data_trade_validation.sh
scripts/run_data_trade_validation.sh --skip-live --out /tmp/fishbone-stage14-dry-run
```

如果 live-chain 可用，也执行：

```bash
scripts/run_data_trade_validation.sh --out /tmp/fishbone-stage14-full
```

最后检查：

```bash
git status --short --branch
git diff --check
```

## 10. 提交建议

建议拆成 2-3 个提交：

1. 脚本与 summary 工具：

```bash
git commit -m "test: add reproducible data trade validation runner"
```

2. 文档与 evidence index：

```bash
git commit -m "docs: document data trade validation evidence"
```

3. 如果执行 live-chain 并更新实验结果文档：

```bash
git commit -m "test: record Stage 14 validation results"
```

不要提交：

- `/tmp/fishbone-stage14-*`
- `target/data-trade-zk/`
- 大体积 proof artifact
- unrelated user files

## 11. Stop conditions

CodeWhale 遇到以下情况必须停止并请求 Codex：

- 需要修改 pallet/runtime 才能完成 Stage 14。
- 需要修改 ZK 电路或 artifact schema。
- 想要 clean redeploy main 或 child6。
- 发现 Stage 13 已通过命令在未改代码情况下稳定失败。
- 需要引入新 npm 依赖。
- summary schema 需要变成复杂数据库模型。
- 想把 Web 后端或前端纳入 Stage 14。
- live-chain 失败但 dry-run 通过，且无法判断是链环境还是脚本问题。

## 12. Codex review 重点

Stage 14 code review 时，Codex 应重点检查：

- 一键脚本是否默认非破坏性。
- negative validation 是否按预期把 non-zero 当作通过。
- readiness 失败是否不会伪造 live evidence。
- summary schema 是否足够通用，未来平台可复用。
- 文档是否准确区分 dry-run、live-chain、prototype、future work。
- 是否不夸大 off-chain attestation 的安全边界。
- 是否未提交大体积生成物。
- 是否没有把数据交易写成未来平台唯一业务模型。

## 13. 预期完成后的状态

完成后，用户应能执行：

```bash
scripts/run_data_trade_validation.sh --out /tmp/fishbone-stage14-full
```

得到：

```text
/tmp/fishbone-stage14-full/summary.json
/tmp/fishbone-stage14-full/summary.md
/tmp/fishbone-stage14-full/.../evidence.json
/tmp/fishbone-stage14-full/.../run.log
```

并能从仓库文档中清楚看到：

- 如何复现数据交易实验。
- 每个 scenario 的预期结果。
- evidence 字段如何解释。
- 哪些结论可用于论文。
- 哪些能力仍属于 future work。
- 这套 evidence 结构如何服务未来 Web 后端的平台对象。
