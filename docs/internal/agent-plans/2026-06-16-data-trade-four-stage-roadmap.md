# Data Trade Four-Stage Roadmap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 保存数据交易论文场景后续四阶段实施路线，防止上下文过长后丢失项目记忆，并为后续 agent 执行提供入口。

**Architecture:** 本文件是总索引，不直接修改代码。四个阶段分别落在独立 plan 文件中，每个阶段都有明确目标、输入产物、验收标准和执行顺序；执行时优先按阶段顺序推进，除非上游验收已满足。

**Tech Stack:** Markdown planning docs, FishboneChain VM deployment scripts, Rust/Substrate, Node.js E2E scripts, Go/gnark, project docs.

---

## 总原则

- 先稳定回归，再扩展业务语义。
- 先跑通一个论文级最小业务 witness，再扩展多子链 profile。
- 先明确 trust assumption，再谈去 trustless bridge。
- 所有执行 plan 都必须更新本文件对应阶段状态，并在完成后提交。
- 每个阶段的 plan 执行前，先跑 Stage 1 回归确认 main+child6 基础链路没有被上游修改破坏；如果 Stage 1 还没实现，则先完成 Stage 1。

## 四阶段顺序

1. **Stage 1: VM E2E Regression**
   - Plan: `docs/internal/agent-plans/2026-06-16-stage1-vm-e2e-regression.md`
   - 目标：把当前手工 VM E2E 固化成一键回归能力。
   - 依赖：commit `6cbe72e test: verify data trade zk flow on vm` 已完成真实 gnark VM E2E。
   - 完成标志：一条命令能 clean redeploy main+child6 并运行 base/dev-zk/real-zk 三组 E2E，输出摘要。
   - 状态：已完成，见 `scripts/run_data_trade_vm_regression.sh` 和 `target/data-trade-vm-regression/summary.md`。

2. **Stage 2: Paper Business Witness**
   - Plan: `docs/internal/agent-plans/2026-06-16-stage2-paper-business-witness.md`
   - 目标：分两步推进业务 witness：Stage 2.1 先把业务输入 canonical hash 绑定进 artifact/digest/链上 attestation；Stage 2.2 再替换 gnark 电路 witness，使电路实际证明业务约束。
   - 依赖：Stage 1 回归脚本稳定。
   - 完成标志：Stage 2.1 完成时不得声称“电路证明业务逻辑”，只能声称业务 metadata 已绑定到链上 digest；Stage 2.2 完成后，`zk_real_data_trade_flow.js` 才能声称使用业务 witness 生成并验证 proof。

3. **Stage 3: Multi Trade Subchain Profiles**
   - Plan: `docs/internal/agent-plans/2026-06-16-stage3-multi-trade-subchain-profiles.md`
   - 目标：支持多个数据交易/zk 服务场景子链，每条子链有自己的 profile、proof params、settlement mode 和 verifier policy。
   - 依赖：Stage 2 至少完成一个业务 witness 场景。
   - 完成标志：新增 child7 profile 不复制核心脚本，只通过配置跑通一条数据交易 VM smoke。

4. **Stage 4: Security Model and Paper Alignment**
   - Plan: `docs/internal/agent-plans/2026-06-16-stage4-security-model-paper-alignment.md`
   - 目标：把工程实现、论文流程、安全假设、后续 trustless bridge 演进对齐。
   - 依赖：Stage 1 完成；Stage 2/3 可并行更新事实。
   - 完成标志：文档能清楚说明 ZK、attestation、bridge、MainEscrow、FMC-assisted 各自承担的安全边界。

## 当前基线事实

- `main + child6` 已在 VM 上 clean redeploy 并跑通。
- `scripts/data_trade_flow.js` 三个场景已通过 VM E2E。
- `scripts/zk_attested_data_trade_flow.js` 已通过 VM E2E。
- `scripts/zk_real_data_trade_flow.js` 已通过 VM E2E，链下生成/验证 gnark proof，链上提交 proof digest 与 Charlie attestation，主链 `settleByPreimage` 成功。
- `scripts/run_data_trade_vm_regression.sh` 已提供一键 VM 回归：clean redeploy main+child6，等待 RPC readiness，运行 base/dev-zk/real-zk，并输出 JSON/Markdown summary。
- **Stage 2.2 已完成**：`BusinessRangeProof` gnark 电路证明 `raw_value ∈ [min, max]` + `masked_value` + `masked_value_hash`。Range 业务 witness 已进入电路约束。完整 IMT membership、subset/substr 约束种类和链上 Groth16 verifier 仍是后续工作。
- 当前链上仍是 attestation 模式，`DataTradeProofVerifier = AlwaysPassVerifier`，主链/子链 settlement bridge 仍为链下协调，不是 trustless cross-chain proof。

## 执行规则

- 每次开始执行某阶段前，先读取该阶段 plan 和本 roadmap。
- 每完成一个 task，更新对应 plan 的 checkbox 和 Execution Record。
- 每完成一个阶段，更新本 roadmap 的“阶段状态”。
- VM 部署类任务由主 agent 执行或严审，因为部署现场容易出现 RPC、systemd、spec、旧数据目录、stdout/stderr 等不确定问题。
- 纯代码实现类任务可交给其他 agent，但必须由主 agent code review。

## 阶段状态

- [x] Stage 1: VM E2E Regression
- [x] Stage 2: Paper Business Witness (complete)
- [x] Stage 3: Multi Trade Subchain Profiles (complete)
- [x] Stage 4: Security Model and Paper Alignment

## 历史执行顺序（所有阶段已完成）

- 近期第一步：执行 Stage 2 Task 1-3，先锁定论文业务 witness 的最小数据模型。
- Stage 2 最小业务 proof 跑通后：执行 Stage 3，抽象多子链 profile。
- Stage 4 可在 Stage 1 后启动文档骨架，并随着 Stage 2/3 更新。
