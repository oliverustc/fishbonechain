# Data Trade E2E Binding And ZK Attestation Implementation Plan

## 执行前评审处理结果（Pre-Execution Review Resolved）

以下外部评审意见已经固化为执行决策，执行 agent 不需要再次选择方案：

- **Stage 1 / Stage 2 断裂处理：采用方案 B。** Stage 1 在 Task 1-5 先跑当前非 ZK VM E2E；Task 7 引入 verifier attestation 后，必须同步更新 `scripts/data_trade_flow.js` 的所有正向 proof 路径，在 `submit_data_proof` 和 `submit_proof_signature` 之间加入 Charlie 签名的 `attest_data_proof(..., true)`。Task 10 的最终验证矩阵不再把该脚本称为“非 ZK”，而称为“base attested E2E”。
- **Verifier account 配置：使用 runtime `AccountId`。** `runtime/src/configs/mod.rs` 已经引入 `AccountId`，计划中使用 `pub VerifierAuthorityAccount: AccountId = sp_keyring::Sr25519Keyring::Charlie.to_account_id();`，并以 `cargo check --features scene-data-trade` 作为验收。
- **Mock ListingProvider 名称：使用现有 `TestListingProvider`。** 不创建 `MockListingProvider`。
- **Dispute 语义：保留 DR 最终争议权。** 即使 verifier 已将 proof attested 为 accepted，DR 仍可在 `InDelivery` session 中调用 `dispute_invalid_proof`，这代表链下发现 proof 或交付数据不一致后的最终争议入口。
- **Settlement claim 安全性：必须增加 completed rounds 校验。** `claim_settlement` 必须拒绝 DO 声明超过已完成轮次数的结算，不允许只依赖主链 hash chain 兜底。
- **Verifier Charlie endowment：必须加入 dev genesis。** `development_config_genesis()` 显式加入 Charlie，`local_config_genesis()` 保持 `Sr25519Keyring::iter()`。

---

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. 遇到阻塞、发现设计不一致、或测试结果和预期不符时，先把问题、命令、输出摘要写回本文件的"执行记录"小节，再继续修复。

**Goal:** 先把数据交易论文流程在真实 VM 多链环境中跑通、去掉脚本硬编码、补上 session-escrow 绑定验证，再接入链下 gnark verifier + 链上授权 attestation 的第一版 ZK 验证路径。

**Architecture:** 第一阶段只强化现有主链 `pallet-main-escrow`、child6 `pallet-data-registry`/`pallet-trade-session`、Node.js E2E/bridge 脚本，不引入真实 ZK。第二阶段使用"链下 verifier 服务验证 gnark proof，授权 verifier 账户在 child6 上提交 attestation"的路径，避免把 gnark/Solidity verifier 直接搬进 Substrate runtime。所有涉及资金、权限、跨链绑定、状态机的任务必须写测试和验收不变量。

**Tech Stack:** Substrate FRAME Rust pallets, `pallet-balances`, runtime features `role-main`/`scene-data-trade`, Node.js `@polkadot/api`, existing deploy VM scripts, gnark reference code under `references/data_trade_code`.

---

## Current Baseline

- `pallet-data-registry` 已有 listing、IMT root、价格、最大轮数、押金提示、proof 参数哈希。
- `pallet-trade-session` 已有交易 session 状态机，但 `ProofVerifier = AlwaysPassVerifier`，ZK 仍是 mock。
- `pallet-main-escrow` 已有主链锁资、押金、结算、惩罚。
- `scripts/data_trade_flow.js` 已有 `happy`、`invalid-proof`、`requester-refuses-payment`，但目前硬编码 `listing_id=0`、`escrow_id=0`、`session_id=0`。
- 当前 child6 不能直接读取主链 escrow 状态；第一阶段的 session-escrow 绑定验证放在 E2E/bridge 侧执行，第二阶段再评估 CCMC/Merkle proof 上链验证。

## File Map

- Modify: `scripts/data_trade_flow.js`  
  负责真实 E2E 流程，必须从事件或链上 storage 获取实际 `listingId`、`escrowId`、`sessionId`。
- Create: `scripts/lib/data_trade_events.js`  
  负责从 finalized/in-block extrinsic events 中提取 pallet event 字段。
- Create: `scripts/lib/data_trade_binding.js`  
  负责查询 child6 listing/session 和 main escrow，执行 session-escrow 绑定断言。
- Modify: `scripts/bridges/data_trade.js`  
  负责监听 child6 交易事件，在执行主链动作前检查绑定缓存或实时绑定。
- Modify: `pallets/trade-session/src/types.rs`  
  第二阶段新增 verifier attestation 相关状态。
- Modify: `pallets/trade-session/src/proof.rs`  
  第二阶段新增 proof bundle hash/attestation 类型边界。
- Modify: `pallets/trade-session/src/lib.rs`  
  第二阶段新增 verifier attestation extrinsic，并调整 proof 状态机。
- Modify: `pallets/trade-session/src/mock.rs`  
  第二阶段配置测试 verifier authority。
- Modify: `pallets/trade-session/src/tests.rs`  
  第二阶段新增 attestation 权限、状态顺序、拒绝无效 verifier 的测试。
- Modify: `runtime/src/configs/mod.rs`  
  第二阶段配置 verifier authority 参数。
- Modify: `scripts/lib/data_trade_sample.js`  
  第二阶段生成 mock proof bundle/public input hash。
- Create: `scripts/zk_attested_data_trade_flow.js`  
  第二阶段跑 `verifier=zk-attested` E2E。
- Modify: `docs/implementation/data-trade-zk-verifier-plan.md`  
  更新实际选择路径和当前限制。

---

## Stage 1: VM E2E, De-Hardcode, Session-Escrow Binding

### Task 1: VM Clean Deploy Smoke Test

**Purpose:** 先证明 main + child6 能在真实 VM 网络中 clean deploy，并暴露 RPC/WS。

**Files:**
- Read: `deploy/config.toml`
- Read: `scripts/dev_redeploy_clean_chains.sh`
- Read: `scripts/dev_scan_vms.py`
- No code changes expected in this task unless commands fail due to stale config.

- [ ] **Step 1: Scan VM state before deployment**

Run:

```bash
bash scripts/dev_scan_vms.sh --config deploy/config.toml
```

Expected:

```text
main/child chain processes and data directories are listed per configured VM.
No SSH ProxyJump requirement appears in command output.
```

- [ ] **Step 2: Clean redeploy main and child6**

Run:

```bash
bash scripts/dev_redeploy_clean_chains.sh --chains main,child6 --config deploy/config.toml
```

Expected:

```text
main and child6 services are stopped, data directories are removed, specs/services are deployed, and both nodes start successfully.
```

- [ ] **Step 3: Verify RPC reachability**

Run:

```bash
node -e 'import("@polkadot/api").then(async ({ApiPromise,WsProvider})=>{const m=await ApiPromise.create({provider:new WsProvider(process.env.MAIN_WS||"ws://10.2.2.11:9944")}); const c=await ApiPromise.create({provider:new WsProvider(process.env.CHILD6_WS||"ws://10.2.2.11:9950")}); console.log(String(await m.rpc.system.chain())); console.log(String(await c.rpc.system.chain())); await m.disconnect(); await c.disconnect();})'
```

Expected:

```text
Two chain names are printed and the process exits with status 0.
```

- [ ] **Step 4: Record VM smoke result**

Append this block under "执行记录":

```markdown
### VM smoke test result
- Date:
- MAIN_WS:
- CHILD6_WS:
- Deploy command:
- Scan summary:
- RPC chain names:
```

**验收不变量:**
- main WS 可连接。
- child6 WS 可连接。
- child6 runtime metadata exposes `dataRegistry` and `tradeSession`.
- main runtime metadata exposes `mainEscrow`.

---

### Task 2: Extract IDs From Events In E2E Script

**Purpose:** 去掉 `scripts/data_trade_flow.js` 中对 `0` 号 listing/escrow/session 的依赖，使脚本能在非空链状态下运行。

**Files:**
- Create: `scripts/lib/data_trade_events.js`
- Modify: `scripts/data_trade_flow.js`

- [ ] **Step 1: Add event extraction helpers**

Create `scripts/lib/data_trade_events.js`:

```js
export function findEvent(result, section, method) {
  for (const { event } of result.events || []) {
    if (event.section === section && event.method === method) {
      return event;
    }
  }
  const seen = (result.events || [])
    .map(({ event }) => `${event.section}.${event.method}`)
    .join(", ");
  throw new Error(`event ${section}.${method} not found; seen=[${seen}]`);
}

export function eventDataNumber(event, field) {
  const value = event.data[field];
  if (value === undefined) {
    throw new Error(`event ${event.section}.${event.method} missing field ${field}`);
  }
  return value.toNumber ? value.toNumber() : Number(value);
}
```

- [ ] **Step 2: Modify `submitTx` callers to capture results**

In `scripts/data_trade_flow.js`, import helpers:

```js
import { findEvent, eventDataNumber } from "./lib/data_trade_events.js";
```

Replace publish/open/create calls with captured IDs:

```js
const publishResult = await submitTx(bob, childApi.tx.dataRegistry.publishData(
  sample.imtRoot,
  sample.description,
  pricePerRound,
  maxRounds,
  depositHint,
  sample.requestHash,
  sample.proofParamsHash,
), "publishData");
const listingId = eventDataNumber(findEvent(publishResult, "dataRegistry", "DataPublished"), "listingId");

const escrowResult = await submitTx(alice, mainApi.tx.mainEscrow.openEscrow(
  bob.address,
  maxRounds,
  pricePerRound,
  depositHint,
  hashChainAnchor,
), "openEscrow");
const escrowId = eventDataNumber(findEvent(escrowResult, "mainEscrow", "EscrowOpened"), "escrowId");

const sessionResult = await submitTx(alice, childApi.tx.tradeSession.createSession(
  listingId,
  escrowId,
  bob.address,
  sample.requestHash,
  pricePerRound,
  maxRounds,
  hashChainAnchor,
  "MainEscrow",
), "createSession");
const sessionId = eventDataNumber(findEvent(sessionResult, "tradeSession", "SessionCreated"), "sessionId");
```

Then replace all later hardcoded `0` session/escrow/listing references in each scenario with `sessionId`, `escrowId`, or `listingId`.

- [ ] **Step 3: Verify no ID hardcoding remains in data trade flow**

Run:

```bash
rg -n "listing_id=0|escrow_id=0|session_id=0|lockFunds\\(0\\)|lockDeposit\\(0\\)|createSession\\(\\s*0|acceptSession\\(0\\)|settleByPreimage\\(0|punishDataOwner\\(0|claimLastPayment\\(0" scripts/data_trade_flow.js
```

Expected:

```text
No output.
```

- [ ] **Step 4: Syntax check**

Run:

```bash
node --check scripts/data_trade_flow.js
node --check scripts/lib/data_trade_events.js
```

Expected:

```text
All commands exit with status 0.
```

**验收不变量:**
- E2E 获取 ID 的唯一来源是链上事件或 storage 查询结果。
- 同一次 scenario 中 `listingId`、`escrowId`、`sessionId` 必须被后续所有交易复用。
- 脚本可以在未清空链状态时创建新交易实例。

---

### Task 3: Add Session-Escrow Binding Assertions To E2E

**Purpose:** 在 child6 创建 session 前后验证主链 escrow 与 child6 listing/session 参数一致，避免"session 指向错误 escrow"。

**Files:**
- Create: `scripts/lib/data_trade_binding.js`
- Modify: `scripts/data_trade_flow.js`

- [ ] **Step 1: Add binding helper**

Create `scripts/lib/data_trade_binding.js`:

```js
function asString(value) {
  return value && value.toString ? value.toString() : String(value);
}

function asNumber(value) {
  return value && value.toNumber ? value.toNumber() : Number(value);
}

export async function assertEscrowMatchesTradeTerms(mainApi, escrowId, expected) {
  const maybeEscrow = await mainApi.query.mainEscrow.escrows(escrowId);
  if (!maybeEscrow.isSome) {
    throw new Error(`mainEscrow.escrows(${escrowId}) is None`);
  }
  const escrow = maybeEscrow.unwrap();
  const failures = [];

  if (asString(escrow.requester) !== expected.requester) failures.push("requester");
  if (asString(escrow.dataOwner) !== expected.dataOwner) failures.push("dataOwner");
  if (asNumber(escrow.maxRounds) !== expected.maxRounds) failures.push("maxRounds");
  if (asString(escrow.pricePerRound) !== String(expected.pricePerRound)) failures.push("pricePerRound");
  if (asString(escrow.deposit) !== String(expected.deposit)) failures.push("deposit");
  if (asString(escrow.hashChainAnchor) !== expected.hashChainAnchor) failures.push("hashChainAnchor");
  if (asString(escrow.status) !== "Ready") failures.push(`status=${asString(escrow.status)}`);

  if (failures.length > 0) {
    throw new Error(`escrow ${escrowId} does not match trade terms: ${failures.join(", ")}`);
  }
}

export async function assertSessionMatchesListingAndEscrow(childApi, sessionId, expected) {
  const maybeSession = await childApi.query.tradeSession.sessions(sessionId);
  if (!maybeSession.isSome) {
    throw new Error(`tradeSession.sessions(${sessionId}) is None`);
  }
  const session = maybeSession.unwrap();
  const failures = [];

  if (asNumber(session.listingId) !== expected.listingId) failures.push("listingId");
  if (asNumber(session.escrowId) !== expected.escrowId) failures.push("escrowId");
  if (asString(session.requester) !== expected.requester) failures.push("requester");
  if (asString(session.dataOwner) !== expected.dataOwner) failures.push("dataOwner");
  if (asNumber(session.maxRounds) !== expected.maxRounds) failures.push("maxRounds");
  if (asString(session.pricePerRound) !== String(expected.pricePerRound)) failures.push("pricePerRound");
  if (asString(session.hashChainAnchor) !== expected.hashChainAnchor) failures.push("hashChainAnchor");
  if (asString(session.settlementMode) !== "MainEscrow") failures.push("settlementMode");

  if (failures.length > 0) {
    throw new Error(`session ${sessionId} binding mismatch: ${failures.join(", ")}`);
  }
}
```

- [ ] **Step 2: Call binding helpers in every scenario**

In `scripts/data_trade_flow.js`, import:

```js
import {
  assertEscrowMatchesTradeTerms,
  assertSessionMatchesListingAndEscrow,
} from "./lib/data_trade_binding.js";
```

After `lockDeposit(escrowId)` and before `createSession(...)`, call:

```js
await assertEscrowMatchesTradeTerms(mainApi, escrowId, {
  requester: alice.address,
  dataOwner: bob.address,
  maxRounds,
  pricePerRound,
  deposit: depositHint,
  hashChainAnchor,
});
```

After `createSession(...)`, call:

```js
await assertSessionMatchesListingAndEscrow(childApi, sessionId, {
  listingId,
  escrowId,
  requester: alice.address,
  dataOwner: bob.address,
  maxRounds,
  pricePerRound,
  hashChainAnchor,
});
```

- [ ] **Step 3: Syntax check**

Run:

```bash
node --check scripts/lib/data_trade_binding.js
node --check scripts/data_trade_flow.js
```

Expected:

```text
All commands exit with status 0.
```

**验收不变量:**
- `mainEscrow.escrows(escrowId).status == Ready` before child6 session creation.
- `escrow.requester == DR` and `escrow.dataOwner == DO`.
- `escrow.maxRounds == listing.maxRounds == session.maxRounds`.
- `escrow.pricePerRound == listing.pricePerRound == session.pricePerRound`.
- `escrow.deposit == listing.depositHint`.
- `escrow.hashChainAnchor == session.hashChainAnchor`.
- `session.escrowId` is the exact main-chain `escrowId` verified in this scenario.

---

### Task 4: Bridge Binding Guard

**Purpose:** bridge 在执行 `punishDataOwner` 或 `claimLastPayment` 前必须验证 child6 session 与主链 escrow 绑定一致。

**Files:**
- Modify: `scripts/bridges/data_trade.js`
- Reuse: `scripts/lib/data_trade_binding.js`

- [ ] **Step 1: Import binding helper**

Add:

```js
import { assertEscrowMatchesTradeTerms } from "../lib/data_trade_binding.js";
```

- [ ] **Step 2: Add helper to read session terms**

Add inside `scripts/bridges/data_trade.js`:

```js
async function readSessionTerms(childApi, sessionId) {
  const maybeSession = await childApi.query.tradeSession.sessions(sessionId);
  if (!maybeSession.isSome) {
    throw new Error(`tradeSession.sessions(${sessionId}) is None`);
  }
  const session = maybeSession.unwrap();
  return {
    escrowId: session.escrowId.toNumber(),
    listingId: session.listingId.toNumber(),
    requester: session.requester.toString(),
    dataOwner: session.dataOwner.toString(),
    maxRounds: session.maxRounds.toNumber(),
    pricePerRound: session.pricePerRound.toString(),
    hashChainAnchor: session.hashChainAnchor.toString(),
  };
}
```

- [ ] **Step 3: Validate before main-chain execution**

Before `mainApi.tx.mainEscrow.punishDataOwner(escrowId)` and before `mainApi.tx.mainEscrow.claimLastPayment(escrowId, ...)`, call:

```js
const terms = await readSessionTerms(childApi, data.sessionId);
const listing = await childApi.query.dataRegistry.listings(terms.listingId);
if (!listing.isSome) {
  throw new Error(`dataRegistry.listings(${terms.listingId}) is None`);
}
await assertEscrowMatchesTradeTerms(mainApi, terms.escrowId, {
  requester: terms.requester,
  dataOwner: terms.dataOwner,
  maxRounds: terms.maxRounds,
  pricePerRound: terms.pricePerRound,
  deposit: listing.unwrap().depositHint.toString(),
  hashChainAnchor: terms.hashChainAnchor,
});
```

Then use `terms.escrowId` for the main-chain call.

- [ ] **Step 4: Syntax check**

Run:

```bash
node --check scripts/bridges/data_trade.js
```

Expected:

```text
Command exits with status 0.
```

**验收不变量:**
- Bridge `--execute --dev-keys` must refuse to submit a main-chain transaction if binding validation fails.
- Bridge must never infer `escrowId` from a hardcoded value.
- Bridge must read `escrowId` from child6 session storage.
- Bridge must read `depositHint` from the session's `listingId`, not from `sessionId`.

---

### Task 5: Run VM E2E Scenarios

**Purpose:** 在真实 main + child6 上跑通非 ZK 交易流程，确认交易骨架和资金语义稳定。

**Files:**
- No code changes expected unless scenarios fail.
- Append results to this plan under "执行记录".

- [ ] **Step 1: Run happy path**

Run:

```bash
MAIN_WS=ws://10.2.2.11:9944 CHILD6_WS=ws://10.2.2.11:9950 \
node scripts/data_trade_flow.js --main "$MAIN_WS" --child "$CHILD6_WS" --scenario happy --verbose
```

Expected:

```text
Happy path completes.
DO receives paid_rounds * price_per_round.
DR reserved balance returns to 0.
DO reserved balance returns to 0.
```

- [ ] **Step 2: Run invalid proof path**

Run:

```bash
MAIN_WS=ws://10.2.2.11:9944 CHILD6_WS=ws://10.2.2.11:9950 \
node scripts/data_trade_flow.js --main "$MAIN_WS" --child "$CHILD6_WS" --scenario invalid-proof --verbose
```

Expected:

```text
Invalid proof path completes.
child6 session reaches Punished.
main escrow reaches Punished.
DR receives DO deposit.
DR reserved funds return to 0.
DO reserved deposit returns to 0 by transfer to DR.
```

- [ ] **Step 3: Run requester refuses payment path**

Run:

```bash
MAIN_WS=ws://10.2.2.11:9944 CHILD6_WS=ws://10.2.2.11:9950 \
node scripts/data_trade_flow.js --main "$MAIN_WS" --child "$CHILD6_WS" --scenario requester-refuses-payment --verbose
```

Expected:

```text
Requester-refuses-payment path completes.
DO receives exactly one round payment.
DR receives refund for unpaid rounds.
DO deposit is released.
```

- [ ] **Step 4: Append results**

Append:

```markdown
### Non-ZK VM E2E result
- Date:
- main chain:
- child6 chain:
- happy result:
- invalid-proof result:
- requester-refuses-payment result:
- observed remaining risks:
```

**验收不变量:**
- Each scenario can run after a previous scenario without cleaning chain state.
- Every scenario uses event-derived IDs.
- All main-chain reserved balances return to 0 after terminal settlement/punishment.
- Terminal child6 session status and main escrow status agree semantically: settlement path settles, punishment path punishes.

---

### Task 6: Enforce Settlement Claim Completed-Rounds Invariant

**Purpose:** `claim_settlement` 不能允许 DO 声明超过已完成轮次数的结算。这个任务在接入 ZK attestation 前完成，避免资金结算语义依赖“DO 诚实”。

**Files:**
- Modify: `pallets/trade-session/src/lib.rs`
- Modify: `pallets/trade-session/src/tests.rs`

- [ ] **Step 1: Write failing overclaim test**

Add to `pallets/trade-session/src/tests.rs`:

```rust
#[test]
fn claim_settlement_rejects_more_paid_rounds_than_completed() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);

		complete_round(0, 0);

		assert_noop!(
			crate::Pallet::<Test>::claim_settlement(
				frame_system::RawOrigin::Signed(2).into(),
				0,
				sp_core::H256::repeat_byte(42),
				1,
			),
			Error::<Test>::SettlementRoundsExceedCompleted,
		);
	});
}
```

Explanation: the test listing has `max_rounds = 5`; after one completed round, `remaining_rounds = 1` would claim `4` paid rounds, which must be rejected.

- [ ] **Step 2: Verify test fails for the intended reason**

Run:

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session claim_settlement_rejects_more_paid_rounds_than_completed
```

Expected:

```text
FAIL because Error::<Test>::SettlementRoundsExceedCompleted is not defined.
```

- [ ] **Step 3: Add explicit error**

In `pallets/trade-session/src/lib.rs`, add to `Error<T>`:

```rust
SettlementRoundsExceedCompleted,
```

- [ ] **Step 4: Add completed-rounds guard**

In `claim_settlement`, after `remaining_rounds < session.max_rounds`, add:

```rust
let claimed_paid_rounds = session
	.max_rounds
	.checked_sub(remaining_rounds)
	.ok_or(Error::<T>::RoundStepsOutOfOrder)?;
ensure!(
	claimed_paid_rounds <= session.completed_rounds,
	Error::<T>::SettlementRoundsExceedCompleted,
);
```

- [ ] **Step 5: Verify targeted test passes**

Run:

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session claim_settlement_rejects_more_paid_rounds_than_completed
```

Expected:

```text
PASS.
```

- [ ] **Step 6: Verify existing settlement test still passes**

Run:

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session do_can_claim_settlement_after_rounds
```

Expected:

```text
PASS.
```

**验收不变量:**
- `claimed_paid_rounds = max_rounds - remaining_rounds`.
- `claim_settlement` must reject if `claimed_paid_rounds > completed_rounds`.
- `claim_settlement` still permits exact completed-round settlement.
- This child6 guard is in addition to, not a replacement for, main-chain hash-chain verification.

---

## Stage 2: ZK Attestation Path A

### Task 7: Add Verifier Authority And Attested Proof States

**Purpose:** 将 proof 验证从 `AlwaysPassVerifier` 过渡到"DO 提交 proof bundle hash，授权 verifier 账户提交验证 attestation，DR 才能签收 proof"的状态机。

**Files:**
- Modify: `pallets/trade-session/src/types.rs`
- Modify: `pallets/trade-session/src/lib.rs`
- Modify: `pallets/trade-session/src/mock.rs`
- Modify: `pallets/trade-session/src/tests.rs`

- [ ] **Step 1: Write failing test for verifier authority**

Add to `pallets/trade-session/src/tests.rs`:

```rust
#[test]
fn only_authorized_verifier_can_attest_data_proof() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);

		let ch = sp_core::H256::repeat_byte(1);
		assert_ok!(crate::Pallet::<Test>::open_round(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			ch,
		));
		assert_ok!(crate::Pallet::<Test>::submit_payment_proof(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			ch,
		));
		assert_ok!(crate::Pallet::<Test>::submit_data_proof(
			frame_system::RawOrigin::Signed(2).into(),
			0,
			0,
			ch,
		));

		assert_noop!(
			crate::Pallet::<Test>::attest_data_proof(
				frame_system::RawOrigin::Signed(4).into(),
				0,
				0,
				ch,
				true,
			),
			Error::<Test>::NotVerifier,
		);
		assert_ok!(crate::Pallet::<Test>::attest_data_proof(
			frame_system::RawOrigin::Signed(3).into(),
			0,
			0,
			ch,
			true,
		));
	});
}

#[test]
fn rejected_attestation_cannot_be_signed_by_requester() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);

		let ch = sp_core::H256::repeat_byte(1);
		assert_ok!(crate::Pallet::<Test>::open_round(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			ch,
		));
		assert_ok!(crate::Pallet::<Test>::submit_payment_proof(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			ch,
		));
		assert_ok!(crate::Pallet::<Test>::submit_data_proof(
			frame_system::RawOrigin::Signed(2).into(),
			0,
			0,
			ch,
		));
		assert_ok!(crate::Pallet::<Test>::attest_data_proof(
			frame_system::RawOrigin::Signed(3).into(),
			0,
			0,
			ch,
			false,
		));

		assert_noop!(
			crate::Pallet::<Test>::submit_proof_signature(
				frame_system::RawOrigin::Signed(1).into(),
				0,
				0,
				ch,
			),
			Error::<Test>::RoundStepsOutOfOrder,
		);
	});
}

#[test]
fn requester_can_dispute_after_verifier_accepts_proof() {
	new_test_ext().execute_with(|| {
		create_session_helper();
		accept_session_helper(0);

		let ch = sp_core::H256::repeat_byte(1);
		assert_ok!(crate::Pallet::<Test>::open_round(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			ch,
		));
		assert_ok!(crate::Pallet::<Test>::submit_payment_proof(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			ch,
		));
		assert_ok!(crate::Pallet::<Test>::submit_data_proof(
			frame_system::RawOrigin::Signed(2).into(),
			0,
			0,
			ch,
		));
		assert_ok!(crate::Pallet::<Test>::attest_data_proof(
			frame_system::RawOrigin::Signed(3).into(),
			0,
			0,
			ch,
			true,
		));

		assert_ok!(crate::Pallet::<Test>::dispute_invalid_proof(
			frame_system::RawOrigin::Signed(1).into(),
			0,
			0,
			sp_core::H256::repeat_byte(99),
		));
		assert_eq!(
			Sessions::<Test>::get(0).unwrap().status,
			SessionStatus::Punished
		);
	});
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session only_authorized_verifier_can_attest_data_proof
```

Expected:

```text
FAIL because attest_data_proof, NotVerifier, or the new proof attestation statuses are not defined.
```

- [ ] **Step 3: Add statuses and error**

In `pallets/trade-session/src/types.rs`, change `RoundStatus` to include:

```rust
DataProofSubmitted,
DataProofVerified,
DataProofRejected,
```

In `pallets/trade-session/src/lib.rs`, add error:

```rust
NotVerifier,
InvalidProofAttestation,
```

- [ ] **Step 4: Add verifier authority config**

In `pallets/trade-session/src/lib.rs` Config:

```rust
type VerifierAuthority: Get<Self::AccountId>;
```

In `pallets/trade-session/src/mock.rs`:

```rust
parameter_types! {
	pub const VerifierAccount: u64 = 3;
}

impl crate::Config for Test {
	type RuntimeEvent = RuntimeEvent;
	type Currency = Balances;
	type ListingProvider = TestListingProvider;
	type ProofVerifier = AlwaysPassVerifier;
	type VerifierAuthority = VerifierAccount;
	type WeightInfo = ();
}
```

- [ ] **Step 5: Add attestation extrinsic**

In `pallets/trade-session/src/lib.rs`:

```rust
fn attest_data_proof() -> frame_support::weights::Weight;
```

In `impl WeightInfo for ()`:

```rust
fn attest_data_proof() -> Weight { Weight::from_parts(15_000, 0) }
```

In `Event`:

```rust
DataProofAttested { session_id: SessionId, round_index: RoundIndex, accepted: bool },
```

In `impl<T: Config> Pallet<T>`:

```rust
#[pallet::call_index(12)]
#[pallet::weight(T::WeightInfo::attest_data_proof())]
pub fn attest_data_proof(
	origin: OriginFor<T>,
	session_id: SessionId,
	round_index: RoundIndex,
	proof_hash: T::Hash,
	accepted: bool,
) -> DispatchResult {
	let who = ensure_signed(origin)?;
	ensure!(who == T::VerifierAuthority::get(), Error::<T>::NotVerifier);

	Rounds::<T>::try_mutate(session_id, round_index, |maybe_round| -> DispatchResult {
		let round = maybe_round.as_mut().ok_or(Error::<T>::RoundNotFound)?;
		ensure!(
			round.status == RoundStatus::DataProofSubmitted,
			Error::<T>::RoundStepsOutOfOrder,
		);
		ensure!(round.proof_hash == Some(proof_hash), Error::<T>::InvalidProofAttestation);
		round.status = if accepted {
			RoundStatus::DataProofVerified
		} else {
			RoundStatus::DataProofRejected
		};
		round.last_actor = Some(who);
		Ok(())
	})?;

	Self::deposit_event(Event::DataProofAttested { session_id, round_index, accepted });
	Ok(())
}
```

- [ ] **Step 6: Require verified proof before DR signature**

In `submit_proof_signature`, change the required round status from `DataProofSubmitted` to:

```rust
round.status == RoundStatus::DataProofVerified
```

- [ ] **Step 7: Update existing tests to insert verifier attestation**

Every test path that currently calls `submit_data_proof` and then `submit_proof_signature` must call:

```rust
assert_ok!(crate::Pallet::<Test>::attest_data_proof(
	frame_system::RawOrigin::Signed(3).into(),
	session_id,
	round_index,
	ch,
	true,
));
```

before `submit_proof_signature`.

- [ ] **Step 8: Run trade-session tests**

Run:

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session
```

Expected:

```text
All pallet-trade-session tests pass.
```

**验收不变量:**
- Non-verifier account cannot attest proof.
- DR cannot submit proof signature before verifier attestation.
- Rejected proof cannot advance to `ProofSigned`.
- DR retains the right to call `dispute_invalid_proof` while the session is `InDelivery`, including after a verifier accepted attestation; this is the final dispute entrance for off-chain evidence that contradicts the attestation or delivered data.
- Verifier attestation does not move funds; funds still settle only through main escrow.

---

### Task 8: Runtime And E2E For ZK-Attested Mode

**Purpose:** 将 child6 runtime 配置 verifier authority，并添加 `zk-attested` E2E 脚本。第一版用 dev verifier `//Charlie` 模拟链下 gnark verifier 的结果。

**Files:**
- Modify: `runtime/src/configs/mod.rs`
- Modify: `runtime/src/genesis_config_presets.rs`
- Modify: `scripts/data_trade_flow.js`
- Modify: `scripts/lib/data_trade_sample.js`
- Create: `scripts/zk_attested_data_trade_flow.js`

- [ ] **Step 1: Configure runtime verifier authority**

In `runtime/src/configs/mod.rs`, add under `#[cfg(feature = "scene-data-trade")]`:

```rust
parameter_types! {
	pub VerifierAuthorityAccount: AccountId = sp_keyring::Sr25519Keyring::Charlie.to_account_id();
}
```

Then update `impl pallet_trade_session::Config for Runtime`:

```rust
type VerifierAuthority = VerifierAuthorityAccount;
```

- [ ] **Step 2: Ensure Charlie is endowed**

In `runtime/src/genesis_config_presets.rs`, ensure `development_config_genesis()` endowed accounts include:

```rust
Sr25519Keyring::Charlie.to_account_id(),
```

and `local_config_genesis()` still uses `Sr25519Keyring::iter()` so Charlie is included.

- [ ] **Step 3: Add sample proof bundle helper**

In `scripts/lib/data_trade_sample.js`, export:

```js
export function sampleProofBundle(round) {
  return {
    constraintKind: "Range",
    chProofHash: `0x${String(round + 11).padStart(64, "0")}`,
    roProofHash: `0x${String(round + 21).padStart(64, "0")}`,
    publicInputHash: `0x${String(round + 31).padStart(64, "0")}`,
  };
}
```

- [ ] **Step 4: Update base data trade flow for dev attestation**

In `scripts/data_trade_flow.js`, add Charlie:

```js
const charlie = keyring.addFromUri("//Charlie");
log(`Charlie (Verifier): ${charlie.address}`);
log("verifier=dev-attested");
```

In every positive proof path where `submitDataProof(...)` is followed by `submitProofSignature(...)`, insert:

```js
await submitTx(charlie, childApi.tx.tradeSession.attestDataProof(
  sessionId,
  round,
  ch,
  true,
), `attestDataProof(${round})`);
```

For the `requester-refuses-payment` scenario, insert the same call after `submitDataProof(0)` and before `submitProofSignature(0)`.

For the `invalid-proof` scenario, do not insert accepted attestation before `disputeInvalidProof`; this scenario represents DR disputing before accepting the proof.

- [ ] **Step 5: Create zk-attested E2E script**

Create `scripts/zk_attested_data_trade_flow.js` by copying the de-hardcoded happy path from `scripts/data_trade_flow.js`, then make these changes:

```js
const charlie = keyring.addFromUri("//Charlie");
log(`Charlie (Verifier): ${charlie.address}`);
log("verifier=zk-attested");
```

After DO submits data proof:

```js
await submitTx(charlie, childApi.tx.tradeSession.attestDataProof(
  sessionId,
  round,
  ch,
  true,
), `attestDataProof(${round})`);
```

Then DR submits proof signature as before.

- [ ] **Step 6: Syntax check**

Run:

```bash
node --check scripts/data_trade_flow.js
node --check scripts/zk_attested_data_trade_flow.js
node --check scripts/lib/data_trade_sample.js
```

Expected:

```text
Both commands exit with status 0.
```

- [ ] **Step 7: Runtime check**

Run:

```bash
SKIP_WASM_BUILD=1 cargo check -p fishbone-runtime --features scene-data-trade
```

Expected:

```text
Finished dev profile with status 0.
```

**验收不变量:**
- `VerifierAuthority` is Charlie in development/local test profiles.
- `zk_attested_data_trade_flow.js` prints `verifier=zk-attested`.
- `zk-attested` path includes verifier attestation between DO proof submission and DR signature.

---

### Task 9: Wire Real gnark Verifier As Off-Chain Service Boundary

**Purpose:** 明确真实 gnark verifier 的链下边界，先让脚本能够调用外部 verifier 命令并根据结果提交 attestation。

**Files:**
- Create: `scripts/lib/zk_verifier_client.js`
- Modify: `scripts/zk_attested_data_trade_flow.js`
- Modify: `docs/implementation/data-trade-zk-verifier-plan.md`

- [ ] **Step 1: Add verifier client wrapper**

Create `scripts/lib/zk_verifier_client.js`:

```js
import { spawnSync } from "node:child_process";

export function verifyProofOffchain({ command, proofHash, publicInputHash }) {
  if (!command) {
    return { accepted: true, mode: "dev-always-accept" };
  }

  const result = spawnSync(command, [proofHash, publicInputHash], {
    encoding: "utf8",
    shell: true,
  });

  if (result.status !== 0) {
    throw new Error(`zk verifier command failed: status=${result.status} stderr=${result.stderr}`);
  }

  const output = result.stdout.trim();
  if (output === "accepted") return { accepted: true, mode: "external" };
  if (output === "rejected") return { accepted: false, mode: "external" };
  throw new Error(`zk verifier command must print accepted or rejected, got: ${output}`);
}
```

- [ ] **Step 2: Call verifier client before attestation**

In `scripts/zk_attested_data_trade_flow.js`, import:

```js
import { verifyProofOffchain } from "./lib/zk_verifier_client.js";
```

Before `attestDataProof`, call:

```js
const verifierResult = verifyProofOffchain({
  command: process.env.ZK_VERIFIER_CMD || "",
  proofHash: ch,
  publicInputHash: ch,
});
log(`verifier_result=${verifierResult.accepted ? "accepted" : "rejected"} mode=${verifierResult.mode}`);
await submitTx(charlie, childApi.tx.tradeSession.attestDataProof(
  sessionId,
  round,
  ch,
  verifierResult.accepted,
), `attestDataProof(${round})`);
```

- [ ] **Step 3: Syntax check**

Run:

```bash
node --check scripts/lib/zk_verifier_client.js
node --check scripts/zk_attested_data_trade_flow.js
```

Expected:

```text
Both commands exit with status 0.
```

- [ ] **Step 4: Document the actual ZK boundary**

Update `docs/implementation/data-trade-zk-verifier-plan.md` so "当前状态" says:

```markdown
- Phase 2 target: `zk-attested` mode. gnark proof verification runs off-chain. The chain verifies authorization through the verifier account origin and records accepted/rejected attestation in `trade-session`.
- This mode is not trustless on-chain ZK verification. It is a staged integration point for the existing gnark implementation.
```

**验收不变量:**
- Without `ZK_VERIFIER_CMD`, script runs in `dev-always-accept` mode and prints that mode.
- With `ZK_VERIFIER_CMD`, script only accepts stdout `accepted` or `rejected`.
- On `rejected`, child6 round status becomes `DataProofRejected` and DR cannot sign proof.
- Documentation must not claim trustless on-chain ZK verification.

---

### Task 10: Final Verification Matrix

**Purpose:** 汇总所有测试，确保交易流程基础和 ZK-attested 路径都可验证。

**Files:**
- Append results to this plan under "执行记录".

- [ ] **Step 1: Rust unit tests**

Run:

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-data-registry -p pallet-trade-session -p pallet-main-escrow
```

Expected:

```text
All tests pass.
```

- [ ] **Step 2: Runtime checks**

Run:

```bash
SKIP_WASM_BUILD=1 cargo check -p fishbone-runtime --features role-main
SKIP_WASM_BUILD=1 cargo check -p fishbone-runtime --features scene-data-trade
```

Expected:

```text
Both commands finish with status 0.
```

- [ ] **Step 3: Node syntax checks**

Run:

```bash
node --check scripts/data_trade_flow.js
node --check scripts/bridges/data_trade.js
node --check scripts/zk_attested_data_trade_flow.js
node --check scripts/lib/data_trade_events.js
node --check scripts/lib/data_trade_binding.js
node --check scripts/lib/zk_verifier_client.js
```

Expected:

```text
All commands exit with status 0.
```

- [ ] **Step 4: VM E2E base attested flows**

Run:

```bash
MAIN_WS=ws://10.2.2.11:9944 CHILD6_WS=ws://10.2.2.11:9950 node scripts/data_trade_flow.js --main "$MAIN_WS" --child "$CHILD6_WS" --scenario happy
MAIN_WS=ws://10.2.2.11:9944 CHILD6_WS=ws://10.2.2.11:9950 node scripts/data_trade_flow.js --main "$MAIN_WS" --child "$CHILD6_WS" --scenario invalid-proof
MAIN_WS=ws://10.2.2.11:9944 CHILD6_WS=ws://10.2.2.11:9950 node scripts/data_trade_flow.js --main "$MAIN_WS" --child "$CHILD6_WS" --scenario requester-refuses-payment
```

Expected:

```text
All three scenarios complete. Happy and requester-refuses-payment paths print verifier=dev-attested. Invalid-proof still disputes before accepted attestation.
```

- [ ] **Step 5: VM E2E zk-attested**

Run:

```bash
MAIN_WS=ws://10.2.2.11:9944 CHILD6_WS=ws://10.2.2.11:9950 node scripts/zk_attested_data_trade_flow.js --main "$MAIN_WS" --child "$CHILD6_WS"
```

Expected:

```text
Script prints verifier=zk-attested.
Script prints verifier_result=accepted mode=dev-always-accept unless ZK_VERIFIER_CMD is provided.
Scenario completes.
```

- [ ] **Step 6: Record final result**

Append:

```markdown
### Final verification result
- Date:
- Rust unit tests:
- Runtime checks:
- Node syntax checks:
- Base attested VM E2E:
- ZK-attested VM E2E:
- Remaining security assumptions:
  - child6 still trusts bridge/E2E binding checks for main escrow state.
  - zk-attested mode trusts verifier authority account.
  - trustless on-chain ZK verification is not implemented in this stage.
```

---

## Code Review Checklist

Reviewers must check these items before accepting implementation:

- [ ] No hardcoded `listingId=0`, `escrowId=0`, `sessionId=0` remains in data-trade E2E flows.
- [ ] Bridge execution path validates session-escrow binding before submitting main-chain transactions.
- [ ] `depositHint` is checked against main escrow `deposit` in E2E/bridge binding validation.
- [ ] `claim_settlement` rejects `max_rounds - remaining_rounds > completed_rounds`.
- [ ] `submit_proof_signature` cannot run before verifier attestation in zk-attested mode.
- [ ] Rejected verifier attestation cannot advance to payment or settlement.
- [ ] All terminal fund paths leave main-chain reserved balances at 0.
- [ ] Docs clearly distinguish `mock`, `zk-attested`, and trustless on-chain `zk`.
- [ ] VM E2E results are appended to this plan.

## Execution Records

Agents executing this plan append dated results here.
