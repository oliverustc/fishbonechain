# Stage 14 Plan Review: 数据交易实验固化与一键复现

日期：2026-06-29
审查者：opencode (FWF Plan Review)
被审计划：`docs/internal/agent-plans/2026-06-28-stage14-data-trade-reproducible-validation.md`
当前分支：`stage/stage14-data-trade-validation`

## Scope Reviewed

- 计划全体（14 节），含目标、范围、非目标、当前事实、任务列表、验收标准、验证命令、文档更新和 stop conditions。
- 计划所引用的路线图（`docs/internal/agent-plans/2026-06-28-data-flow-platform-long-term-roadmap.md` Stage 14 节）。
- 计划声称的当前事实在仓库中的可验证性。

## Inputs Read

- `agent.md`
- `docs/internal/agent-collaboration.md`
- `docs/internal/agent-plans/2026-06-28-data-flow-platform-long-term-roadmap.md`（Stage 14 节）
- `docs/internal/agent-reviews/2026-06-27-data-trade-stage13-quality-baseline.md`
- `docs/README.md`
- `docs/implementation/data-trade-demo-guide.md`
- `docs/implementation/data-trade-evidence.md`
- `docs/implementation/data-trade-paper-gap-matrix.md`
- `docs/implementation/data-trade-stage12-evidence-index.md`
- `scripts/zk_real_data_trade_flow.js`（CLI 参数验证）
- `scripts/lib/wait_for_ws_chain.js`（存在性确认）
- `scripts/lib/vm_regression_summary.js`（存在性确认）
- `scripts/lib/trade_profile.js`（profile 解析确认）
- `scripts/run_data_trade_vm_regression.sh`（clean redeploy 行为确认）
- `scripts/fixtures/data_trade_datasets/`、`scripts/fixtures/data_trade_requests/`（fixture 存在性确认）
- `target/tools/fishbone-zk`（binary 存在性确认）
- Git log 和 branch 状态

## Verification Performed

| Claim | Verification | Result |
|-------|-------------|--------|
| `scripts/zk_real_data_trade_flow.js` 存在 | `ls` | ✅ 存在 |
| 支持 `--profile`, `--dataset`, `--request`, `--dry-run-dynamic`, `--scenario`, `--evidence-out` | `grep` on source | ✅ 均在源码中定义 |
| `scripts/lib/wait_for_ws_chain.js` 存在 | `ls` | ✅ 存在 |
| `scripts/lib/vm_regression_summary.js` 存在 | `ls` | ✅ 存在 |
| `scripts/run_data_trade_vm_regression.sh` 存在且含 clean redeploy | `grep` on source | ✅ 存在，line 60 调用 `dev_redeploy_clean_chains.sh` |
| `docs/implementation/data-trade-demo-guide.md` 存在 | `ls` | ✅ 存在 |
| `docs/implementation/data-trade-stage12-evidence-index.md` 存在 | `ls` | ✅ 存在 |
| `docs/implementation/data-trade-evidence.md` 存在 | `ls` | ✅ 存在 |
| `docs/implementation/data-trade-paper-gap-matrix.md` 存在 | `ls` | ✅ 存在 |
| `target/tools/fishbone-zk` 存在 | `ls` | ✅ 存在 |
| 4 个 live-chain scenario 均已通过 Stage 13 | 审阅了 `2026-06-27-data-trade-stage13-quality-baseline.md` | ✅ 均有 evidence |
| `data-trade-demo-guide.md` 声称 live-chain 未运行 | `grep` | ✅ 有 stale 声明（line 91） |
| `data-trade-stage12-evidence-index.md` 声称 live-chain 未运行 | `grep` | ✅ 有 stale 声明（lines 5, 19-25, 76-79） |
| `data-trade-paper-gap-matrix.md` line 39 引用过期时间戳 | `grep` | ✅ "2026-06-26 child RPC check timed out" 已过期 |
| 当前分支为 `stage/stage14-data-trade-validation` | `git branch --show-current` | ✅ |
| 工作区仅 `.gitignore` 有未提交修改 | `git status --short` | ✅（与计划相符） |

## Findings

### F1 (Required): 可修改文件列表与 Step 9 不一致

Section 5（文件级计划）的"可修改文件"仅列出：

```text
docs/implementation/data-trade-demo-guide.md
docs/implementation/data-trade-evidence.md
docs/implementation/data-trade-paper-gap-matrix.md
```

但 Step 9 明确要求更新 `data-trade-stage12-evidence-index.md`（"应指向 Stage 14/Stage 13 green 结果，避免读者误解当前状态"）。该文件不存在于 Section 5 的可修改列表中，也不在新增文件列表中。

**需要修复**：将 `docs/implementation/data-trade-stage12-evidence-index.md` 加入 Section 5 可修改文件列表，或澄清该文件的处理策略。

### F2 (Required): `docs/implementation/data-trade-evidence.md` 修改目的不明确

Section 5 将 `docs/implementation/data-trade-evidence.md` 列入可修改，但 Step 9 未给出该文件的任何最低更新要求。the "只在必要时修改" 的限制合理，但执行者需要知道在什么条件下才"必要"。当前状态的 evidence 文件引用了 Stage 5 的 commit (`afe0720`) 而 Stage 13 的 live-chain evidence 未收录其中。

**需要修复**：给出该文件是否需要更新的明确判断标准，或从可修改列表中移除。

### F3 (Required): `data-trade-paper-gap-matrix.md` line 39 过期时间戳

`docs/implementation/data-trade-paper-gap-matrix.md` line 39：
> Current 2026-06-26 child RPC check timed out; production deployment needs hardening

此描述与 Stage 13 恢复 child6 后的成功 live-chain 结果矛盾。Step 9 的"paper-gap-matrix 如有 live-chain 未跑之类陈旧描述需要修正"覆盖了这个问题，但未指明具体行号或哪些描述需要修正。

**需要修复**：在 Step 9 中明确指定该文件中需要修正的具体行或描述。

### F4 (Suggested): `data-trade-demo-guide.md` Section 3 的 stale 声明

`docs/implementation/data-trade-demo-guide.md` line 91 声称：
> 当前环境 RPC 不可用，以下命令仅供文档参考，未经此阶段运行。

Stage 13 已经证明所有四个 live-chain 场景在 child6 上通过。此声明在 Stage 14 完成后必须改写。Step 9 正确识别了此问题，但未指定改写后的具体措辞方向。

### F5 (Suggested): Summary tool 的 `--help` 初始化延迟

`scripts/zk_real_data_trade_flow.js --help` 可因 `@polkadot/api` 导入延迟而超时。新写的 shell wrapper 脚本内部不应直接调用 `node scripts/zk_real_data_trade_flow.js --help`，而应通过 `ZK_VERIFIER_CMD` 环境变量指向 Go binary。本文件审查时已验证该脚本的 CLI 参数直接通过 grep 源码确认，不依赖 `--help` 输出。

### F6 (Suggested): 未指定 child6 RPC 不可用时的 summary status

计划规定 readiness 失败时整体 status 为 `partial`，live scenarios 标记 `skipped`。但如果 main RPC 正常、仅 child6 RPC 不可用，summary 应包含哪些诊断信息（如 main/child 分别的 readiness 结果），未明确说明。

### F7 (Suggested): Summary schema 与平台版本号的关联

Section 8 的 `summary.json` schema 使用 `"version": 1`。未来平台后端可能期望 version 字段与平台 API 版本（如 `/api/v1/`）对齐。当前这不是阻塞问题，但建议在 experiment doc 或 evidence index 中注明 version 仅代表 evidence 格式版本。

## Decision

**`approved-with-required-fixes`**

## Required Fixes

1. **F1**：将 `docs/implementation/data-trade-stage12-evidence-index.md` 加入 Section 5 可修改文件列表，并明确修改策略（添加指向 Stage 13 live-chain 结果的链接或注释）。

2. **F2**：为 `docs/implementation/data-trade-evidence.md` 给出明确的更新条件或从可修改列表中移除。

3. **F3**：在 Step 9 中明确指定 `data-trade-paper-gap-matrix.md` 中需要修正的具体条目（至少包括 line 39 的 "child RPC check timed out" 描述）。

## Suggested Improvements

1. 在 shell validation 脚本的 CLI 设计中加入 `--summary-only` 或 `--dry-run-validate` 选项，允许仅运行无链部分和 summary 生成，用于快速文档级验证。

2. `data-trade-demo-guide.md` Section 3 的改写应明确标注 "Stage 13/14 validated" 并移除误导性的 "未经此阶段运行" 声明。

3. `docs/experiments/data-trade-validation.md` 编写完成后，应在 `docs/README.md` 和 `docs/experiments/experiment-report.md` 中添加交叉引用。

4. Summary status 为 `partial` 时，`summary.md` 应包含"哪些场景 skipped / 为何 skipped"的可读解释表。

5. Summary schema 的 `version` 字段与未来平台 API 版本之间的关系，建议在 evidence index 中保留一条注释说明当前仅用于格式演化。

## Risks if Unchanged

1. **文档误导**：`data-trade-demo-guide.md` 和 `data-trade-stage12-evidence-index.md` 中 stale 的 "未运行" 声明可能误导论文读者、防御答辩人或新开发者，让人觉得 live-chain 能力未被验证（实际 Stage 13 已全部验证通过）。

2. **实施歧义**：F1 的不一致可能让实现者不确定是否可以修改 `data-trade-stage12-evidence-index.md`，可能导致 stale 声明被遗漏。

3. **Scope creep 风险低**：计划非目标充分覆盖了不应做的操作（不新增协议、不 clean redeploy、不提交生成物），执行阶段应遵守这些约束。

## Questions for Codex/Owner

1. `data-trade-stage12-evidence-index.md` 是保留为历史记录附加更新注释，还是直接更新原文中的 "not run" 为 "Stage 13 validated"？建议保留历史语境但添加明确的前向引用。

2. child6 当前是否预期处于运行状态？如果 child6 RPC 不可用，是否接受 only dry-run 的 `partial` 结果？计划已处理此情况，确认即可。

3. `docs/implementation/data-trade-evidence.md` 是否需要更新以收录 Stage 13 live-chain 证据，还是该文件应仅保留为 Stage 5 的历史记录？若后者，建议将其从可修改列表中移除。
