# Reasonix GitHub PR Workflow

本文档规定 Reasonix 和 Codex 在本仓库中的协作方式。目标是让实现、审阅、修复、验收都沉淀在 GitHub PR 和仓库文件中，避免由项目负责人在多个 agent 聊天窗口之间人工转述。

## 当前 GitHub 状态

- Repository: `oliverustc/fishbonechain`
- Default branch: `main`
- Remote: `git@github.com:oliverustc/fishbonechain.git`
- `gh` CLI: available
- `gh` auth: logged in as `oliverustc`
- Viewer permission: `ADMIN`

## 角色分工

- **项目负责人**：决定阶段目标、是否进入下一阶段、是否允许 merge。
- **Reasonix**：默认作为实现 agent，按照 plan 执行、开分支、提交 commit、创建 PR、根据 review 修复。
- **Codex**：默认作为规划与 review agent，维护 plan，审阅 PR，提出 required changes 或 approve。

角色可以按任务调整，但每个 PR 必须明确谁是 implementer，谁是 reviewer。

## 信息落点

优先级从高到低：

1. GitHub PR description, review comments, commits
2. `docs/internal/agent-plans/*.md`
3. `docs/internal/agent-reviews/*.md`，仅在无法使用 PR 时作为 fallback
4. 聊天记录

审阅意见不得只写在聊天里。只要存在 PR，review findings 必须写到 PR review/comment 中。

## 分支规范

Reasonix 开始任何非 trivial 工作前必须从最新 `main` 创建 feature branch：

```bash
git fetch origin
git switch main
git pull --ff-only origin main
git switch -c agent/reasonix/<stage-or-topic>
```

命名示例：

```text
agent/reasonix/stage3-child7-profiles
agent/reasonix/stage4-security-model
agent/reasonix/zk-subset-circuit
```

禁止直接在 `main` 上实现功能。只有以下情况允许在 `main` 上小改并直接 commit：

- 用户明确要求直接 commit；
- 文档/plan 的极小修正；
- 当前没有 PR 工作流需求。

## Commit 规范

使用清晰、可 review 的 commits。不要把多个无关任务塞进一个 commit。

推荐格式：

```text
feat(stage3): add child7 data trade profile
fix(stage3): make gen_child_specs use template chain id
test(stage3): cover profile loader validation
docs(stage3): record VM smoke result
```

每个 commit 前必须至少运行与改动相关的轻量验证。不要提交明显无法编译或语法错误的代码。

## PR 创建规范

Reasonix 完成一个可审阅单元后创建 PR：

```bash
git push -u origin agent/reasonix/<stage-or-topic>
gh pr create \
  --base main \
  --head agent/reasonix/<stage-or-topic> \
  --title "feat(stage3): add configurable data trade child profiles" \
  --body-file /tmp/pr-body.md
```

PR body 必须包含：

- 关联 plan 文件路径；
- 本 PR 完成了哪些任务；
- 明确未完成/刻意不做的内容；
- 验证命令和结果；
- VM 部署/清理影响，如果有；
- 需要 Codex 特别关注的问题。

如果实现尚未完成但需要提前讨论，创建 draft PR：

```bash
gh pr create --draft ...
```

## Review 规范

Codex review 时使用 GitHub PR 作为唯一审阅入口：

```bash
gh pr checkout <number>
gh pr diff <number>
gh pr view <number> --json title,body,commits,files,reviewDecision,statusCheckRollup
```

审阅结论必须通过 GitHub 写回：

```bash
gh pr review <number> --request-changes --body-file /tmp/review.md
```

或：

```bash
gh pr review <number> --approve --body-file /tmp/review.md
```

Review body 格式：

```markdown
Status: Changes requested

## Findings

- P1: ...
  File: `path/to/file`
  Reason: ...
  Required fix: ...

## Verification

- `command` -> pass/fail

## Notes

- ...
```

严重级别：

- **P0**：会破坏仓库、资金/权限安全、数据丢失，必须立即停止。
- **P1**：核心功能错误、测试无法通过、实现偏离 plan，必须修。
- **P2**：真实风险或扩展性问题，应在本 PR 修。
- **P3**：文档、命名、日志、清理项，可由 reviewer 判断是否本 PR 修。

## 修复规范

Reasonix 收到 review 后：

1. 不要在聊天里逐条辩解。
2. 先检查 review 是否符合代码事实。
3. 如果同意，提交 follow-up commits。
4. 如果不同意，在 PR comment 中给出技术理由，并等待项目负责人或 reviewer 决策。

修复后回复：

```bash
gh pr comment <number> --body "Addressed review findings in <commit-sha>. Verification: <commands>."
```

然后请求 re-review：

```bash
gh pr ready <number>
```

如果 PR 仍是 draft；否则直接 comment 即可。

## Merge 规范

除非项目负责人明确授权，agent 不得自行 merge。

允许 merge 前必须满足：

- PR 已经 Codex approve；
- 必要测试/脚本已通过；
- PR 与 `main` 无冲突；
- PR body 或 comment 记录了验证命令；
- VM clean deploy 的破坏性影响已说明。

推荐 merge 命令：

```bash
gh pr merge <number> --squash --delete-branch
```

如果需要保留 commit 结构，由项目负责人决定使用 merge commit 或 rebase merge。

## 禁止事项

- 禁止未确认就 `git push --force`。
- 禁止 `git reset --hard`、`git checkout -- .`、删除他人改动，除非项目负责人明确要求。
- 禁止绕过 PR 直接推送大功能到 `main`。
- 禁止把 review findings 只写在聊天里。
- 禁止为每条新子链继续在 runtime/node 层硬编码 preset，除非 plan 明确要求。
- 禁止用“应该可以”替代验证命令。

## 无 PR fallback

如果 GitHub 或网络不可用，临时使用本地 review 文件：

```text
docs/internal/agent-reviews/YYYY-MM-DD-<topic>-review.md
```

GitHub 恢复后，应把 review 文件内容迁移到 PR comment，或在 PR body 中链接该文件。

## Reasonix 执行检查表

每次开始任务：

- [ ] 读取最新 plan。
- [ ] 从最新 `origin/main` 创建 `agent/reasonix/...` 分支。
- [ ] 确认工作树干净。
- [ ] 按 plan 小步提交。
- [ ] 运行相关验证。
- [ ] 创建 PR，填写模板。
- [ ] 等待 Codex review。

每次修复 review：

- [ ] 逐条核对 finding。
- [ ] 修复并提交 follow-up commit。
- [ ] 重新运行相关验证。
- [ ] 在 PR comment 中说明修复位置和验证结果。
- [ ] 等待 re-review。
