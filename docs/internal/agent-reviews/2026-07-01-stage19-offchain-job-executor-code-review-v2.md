# Code Review v2: Stage 19 Offchain Job Executor

**Date**: 2026-07-01
**Reviewer**: Codex
**Plan**: `docs/internal/agent-plans/2026-06-30-stage19-offchain-job-executor.md`
**Prior code review**: `docs/internal/agent-reviews/2026-07-01-stage19-offchain-job-executor-code-review.md`
**Review-fix commit**: `777d18b`
**Branch**: `stage/stage19-offchain-job-executor`
**Decision**: `approved`

## Findings

未发现阻断问题。上一轮 code review 的两个 required fixes 均已修复并复现通过：

- F1 已修复：`scripts/platform-backend/lib/job_runner.js` 现在在 dry-run 和 real execution 之前统一验证 `dataset` / `request` 路径安全。手动复现 `dataset=/etc/passwd`、`request=/etc/passwd` 时，job 现在进入 `failed`，`evidence_id` 为 `null`，`evidence` 记录数为 `0`。
- F2 已修复：`scripts/platform-backend/lib/safe_paths.js` 现在把 `--work-dir` 限制在 ignored repo-local runtime root：`.agents/` 或 `var/platform-backend/`。手动复现 `--work-dir docs/...` 时，job 进入 `failed`，未创建 `docs/` 输出目录，也未创建 Evidence。

非阻断记录细节：

- 计划 Execution Record 的 Pass 2 仍写 `Head commit: ac21507` / `Commits: ac21507 ...`，当前实际 HEAD 是 `777d18b`。`ac21507` 确实存在，且与 `777d18b` 的代码、测试、正式文档内容一致；差异只在计划执行记录短哈希行。本轮 review 以 `777d18b` 和实际命令输出为准。
- `docs/implementation/offchain-job-executor.md` 的 code layout 行仍写 `job_executor.test.js   25 tests covering executor mechanics`，当前测试文件已有 28 个 executor tests，整套 `scripts/platform-backend/test/*.test.js` 为 91 tests。这不影响行为、命令或信任边界，但后续文档清理可顺手更新。

## Scope Reviewed

- Review-fix commit `777d18b`
- `scripts/platform-backend/lib/job_runner.js`
- `scripts/platform-backend/lib/safe_paths.js`
- `scripts/platform-backend/test/job_executor.test.js`
- `docs/implementation/offchain-job-executor.md`
- latest Execution Record in `docs/internal/agent-plans/2026-06-30-stage19-offchain-job-executor.md`
- diff and file set from `main..HEAD`

## Verification Performed

```text
git branch --show-current
→ stage/stage19-offchain-job-executor

git status --short
→ clean

git show --stat --oneline 777d18b
→ review-fix commit touches job_runner.js, safe_paths.js, job_executor.test.js, offchain-job-executor.md, and the plan Execution Record

Manual unsafe dry-run reproduction
→ input path unsafe: /etc/passwd; status failed; evidence_id null; output_refs 0; evidence_count 0

Manual non-ignored work-dir reproduction
→ work directory must be under an ignored runtime root; status failed; evidence_id null; docs_dir_exists false; evidence_count 0

node --check scripts/platform-backend/server.js
node --check scripts/platform-backend/job_executor.js
find scripts/platform-backend/lib -name '*.js' -print -exec node --check {} \;
→ all syntax checks passed

node --test scripts/platform-backend/test/*.test.js
→ 91 tests passed, 0 failed

node scripts/platform-backend/job_executor.js --help
→ printed expected help

test -f docs/implementation/offchain-job-executor.md
grep -q offchain-job-executor docs/README.md
grep -q offchain-job-executor docs/implementation/platform-backend-skeleton.md
grep -q offchain-job-executor docs/implementation/implementation-record.md
! rg -q 'child_process.*shell:\s*true|shell:\s*true' scripts/platform-backend/
→ passed

git diff --stat main..HEAD
git diff --name-status main..HEAD
→ Stage 19 process records, executor code/tests, and formal docs only
```

## Tests Not Run

- Optional real proof-generation smoke was not run; no `fishbone-zk` / `ZK_VERIFIER_CMD` executable is available in this environment.

## Branch and Commit Assessment

- Branch is correct: `stage/stage19-offchain-job-executor`.
- Worktree was clean before writing this review record.
- Implementation and review-fix commits reference the plan and validation commands.
- No new dependency, lockfile, chain, runtime, proof digest, settlement, deployment, experiment metric, or paper-facing measured-data change was introduced.

## Accepted Risks

- Real proof generation remains environment-dependent on `fishbone-zk`; Stage 19 validation appropriately covers dry-run mechanics and failure paths.
- The JSON store remains single-process and development-grade, as documented.
- The process record contains stale short commit hashes, but this review records the verified current commit and command output.

## Required Fixes

None.

## Decision

`approved`
