# 数据交易论文流程实现计划

> **给后续 agent/工程执行者：** 实施本计划前，必须使用 `superpowers:subagent-driven-development`（推荐）或 `superpowers:executing-plans`，按任务逐项执行。任务使用 checkbox（`- [ ]`）跟踪进度。严格 TDD：每个 pallet/脚本行为先写失败测试，再实现。

**目标：** 在 FishboneChain 上完整实现数据交易论文中的 DO/DR 可验证数据交易流程，而不是只保留当前 `data-registry` / `trade-session` 骨架。

**架构：** 数据交易部署在 `child6` 数据交易子链；`pallet-data-registry` 承担论文中的 DC，负责 DO 发布 IMT root、数据描述、价格和交易参数；`pallet-trade-session` 承担论文中的 VC 会话状态机，负责请求、押金状态、证明/签名/轮次状态和争议入口；主链新增 `pallet-main-escrow` 承担传统主链锁资模式，保存 DR 资金和 DO 押金，并根据 child6/bridge 提交的结算指令释放资金。第一阶段使用可替换的 proof-verifier trait 和 mock proof/evidence，使论文状态机可端到端运行；第二阶段接入 gnark 生成的 CH/RO proof verifier 与 IMT 工具链。

**技术栈：** Substrate FRAME Rust pallets、`pallet-balances` reservable currency、runtime feature `scene-data-trade`、Node.js `@polkadot/api` E2E 脚本、现有 `references/data_trade_code` 的 Solidity/Fund/Verify 与 gnark 参考实现、现有 VM 部署脚本。

---

## 0. 当前核定结论

当前代码**没有**完整实现论文数据交易流程，只实现了工程基础：

- `child6` 已 clean redeploy 为 DataTrade runtime，metadata 已确认包含 `ChainProfile`、`DataRegistry`、`TradeSession`，且不再包含 `Crowdsource`。
- `pallet-data-registry` 当前只记录 `owner`、`imt_root`、`description`。
- `pallet-trade-session` 当前只支持本链余额模拟锁资/押金、单次 hash-chain claim、punish。
- 当前缺少：listing 与 session 绑定、主链 escrow、交易请求格式、每轮交付状态、DR proof 签名、争议证据、按轮付款、剩余退款、DO 押金惩罚、E2E 场景脚本、ZK verifier 接口/接入。

本计划的完成标准：能在 VM 上运行一条完整流程：

```text
DO publish listing on child6
DR lock n*b funds on main chain
DO lock deposit on main chain
DR create trade request/session on child6
DR/DO perform off-chain round protocol
DO claim payment for k completed rounds
main chain releases k*b to DO, refunds (n-k)*b to DR, returns/slashes DO deposit
negative tests cover invalid proof, invalid plaintext hash, and requester refusing last payment
```

## 1. 论文到 FishboneChain 的映射

| 论文概念 | FishboneChain 实现 |
|---|---|
| DO | Substrate account，数据所有者 |
| DR | Substrate account，数据请求者 |
| DC | `pallet-data-registry` on child6 |
| VC | `pallet-trade-session` on child6，负责交易协议状态和证据 |
| Fund | `pallet-main-escrow` on main chain，负责真实资金 reserve/repatriate/slash |
| IMT root `R` | `DataListing.imt_root`，后续支持 root list / params hash |
| data request `R={Ra,Re}` | `TradeTerms.request_hash` + 可选 bounded encoded request |
| hash-chain anchor `H^(n)` | `TradeTerms.hash_chain_anchor` |
| per-round price `b` | `TradeTerms.price_per_round` |
| max rounds `n` | `TradeTerms.max_rounds` |
| DO deposit `D_DO` | main chain escrow reserved deposit |
| `π_pc` | 第一阶段用 `PaymentCommitmentProof` hash/preimage 验证；第二阶段接 proof verifier |
| `π^CH || π^RO` | 第一阶段用 `ProofBundle` + mock verifier；第二阶段接 gnark verifier |
| DR signature `σ=Sign(vk_DR,H(π))` | child6 `submit_proof_signature`，验证 sr25519/ecdsa 签名或先验证 signer account |
| claim settlement `H^m` | `claim_settlement(session_id, preimage, remaining_rounds)` |

## 1.1 明确端到端调用顺序

后续实现者必须按以下顺序组织 E2E，不要倒置 session 和 escrow：

1. DO 在 child6 调用 `dataRegistry.publishData(imt_root, description, price_per_round, max_rounds, deposit_hint, request_schema_hash, proof_params_hash)`，得到 `listing_id`。listing 中包含 `data_owner`、`price_per_round`、`max_rounds`、`deposit_hint`、`request_schema_hash`、`proof_params_hash`。
2. DR 从 listing 读取交易参数，链下与 DO 协商 request，生成 `request_hash`、`hash_chain_anchor = H^(n)(s)`。
3. DR 在 main 调用 `mainEscrow.openEscrow(data_owner, max_rounds, price_per_round, deposit_hint, hash_chain_anchor)`，得到 `escrow_id`。
4. DR 在 main 调用 `mainEscrow.lockFunds(escrow_id)`，reserve `max_rounds * price_per_round`。
5. DO 在 main 调用 `mainEscrow.lockDeposit(escrow_id)`，reserve `deposit_hint`。
6. DR 在 child6 调用 `tradeSession.createSession(listing_id, escrow_id, data_owner, request_hash, price_per_round, max_rounds, hash_chain_anchor, MainEscrow)`。
7. `tradeSession.createSession` 必须通过 listing provider 验证 listing 存在、处于 Active、owner 等于 `data_owner`，并且 price/max_rounds/deposit 参数与 listing 一致。
8. DO 在 child6 调用 `tradeSession.acceptSession(session_id)` 后进入多轮交付。

这个顺序对应论文 Phase 1/2：先有 DC/listing，再有主链 Fund/escrow，再有 VC/session。

## 2. 文件职责

### 新增文件

- `pallets/main-escrow/Cargo.toml`：主链锁资 pallet crate。
- `pallets/main-escrow/src/types.rs`：`EscrowId`、`EscrowStatus`、`EscrowTerms`、`EscrowAccountState`。
- `pallets/main-escrow/src/lib.rs`：DR 锁资、DO 锁押金、按轮结算、惩罚和退款。
- `pallets/main-escrow/src/mock.rs`：余额测试 runtime。
- `pallets/main-escrow/src/tests.rs`：主链资金状态机测试。
- `pallets/trade-session/src/proof.rs`：proof/evidence 类型和 verifier trait。
- `scripts/data_trade_flow.js`：VM E2E happy-path 与 negative-path 脚本。
- `scripts/lib/hash_chain.js`：生成 hash chain、round preimage、校验工具。
- `scripts/lib/data_trade_sample.js`：生成最小示例数据、IMT root、mock proof bundle。
- `docs/implementation/data-trade-flow.md`：实现记录与运行说明。

### 修改文件

- `Cargo.toml`：加入 `pallets/main-escrow` workspace 成员。
- `runtime/Cargo.toml`：加入 `pallet-main-escrow` optional dependency 和 feature。
- `runtime/src/runtime_main.rs`：主链 runtime 接入 `MainEscrow`。
- `runtime/src/runtime_data_trade.rs`：保持 `DataRegistry`、`TradeSession`，不接 `Crowdsource`。
- `runtime/src/genesis_config_presets.rs`：按 profile 生成 main/child6 genesis，确保 child6 为 `DataTrade/MainEscrow`。
- `runtime/src/configs/mod.rs`：为扩展后的 `DataRegistry`、`TradeSession`、`MainEscrow` 补齐 Config 关联类型。
- `pallets/data-registry/src/types.rs`：扩展 listing 字段。
- `pallets/data-registry/src/lib.rs`：扩展 publish/update/listing lifecycle。
- `pallets/data-registry/src/tests.rs`：扩展 DC 行为测试。
- `pallets/trade-session/src/types.rs`：扩展交易 session、round、evidence 状态。
- `pallets/trade-session/src/lib.rs`：实现论文 VC 状态机。
- `pallets/trade-session/src/mock.rs`：加入 mock verifier / balances / account setup。
- `pallets/trade-session/src/tests.rs`：覆盖 happy path 和争议路径。
- `scripts/bridges/data_trade.js`：从观察脚本升级为 main escrow bridge / dispatcher。
- `docs/implementation/data-trade-implementation.md`：更新当前实现边界和验收状态。

## 3. 总体验收命令

完成所有任务后必须通过：

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-data-registry
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session
SKIP_WASM_BUILD=1 cargo test -p pallet-main-escrow
SKIP_WASM_BUILD=1 cargo check -p fishbone-runtime --features role-main
SKIP_WASM_BUILD=1 cargo check -p fishbone-runtime --features scene-data-trade
node --check scripts/data_trade_flow.js
node --check scripts/bridges/data_trade.js
deploy/.venv/bin/python deploy/cmd/status.py --chains main,child6
node scripts/data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --scenario happy
node scripts/data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --scenario invalid-proof
node scripts/data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --scenario requester-refuses-payment
```

---

## Task 1：扩展 `pallet-data-registry` 为论文 DC

**目的：** DO 发布的数据 listing 必须足以支撑论文 Phase 1“数据上传”和 Phase 2“数据请求”：IMT root、描述、字段 schema、价格、最大轮数、默认押金、proof 参数哈希。

**Files:**

- Modify: `pallets/data-registry/src/types.rs`
- Modify: `pallets/data-registry/src/lib.rs`
- Modify: `pallets/data-registry/src/tests.rs`
- Modify: `pallets/data-registry/src/mock.rs`

- [ ] **Step 1：写失败测试 `publish_listing_includes_trade_terms`**

在 `pallets/data-registry/src/tests.rs` 增加：

```rust
#[test]
fn publish_listing_includes_trade_terms() {
	new_test_ext().execute_with(|| {
		let root = sp_core::H256::repeat_byte(1);
		let request_schema_hash = sp_core::H256::repeat_byte(2);
		let proof_params_hash = sp_core::H256::repeat_byte(3);

		assert_ok!(DataRegistry::publish_data(
			RuntimeOrigin::signed(1),
			root,
			description(),
			1_000,
			10,
			5_000,
			request_schema_hash,
			proof_params_hash,
		));

		let listing = Listings::<Test>::get(0).expect("listing exists");
		assert_eq!(listing.owner, 1);
		assert_eq!(listing.imt_root, root);
		assert_eq!(listing.price_per_round, 1_000);
		assert_eq!(listing.max_rounds, 10);
		assert_eq!(listing.deposit_hint, 5_000);
		assert_eq!(listing.request_schema_hash, request_schema_hash);
		assert_eq!(listing.proof_params_hash, proof_params_hash);
	});
}
```

- [ ] **Step 2：运行测试确认失败**

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-data-registry publish_listing_includes_trade_terms
```

预期：编译失败，`publish_data` 参数数量或 `DataListing` 字段不存在。

- [ ] **Step 3：扩展 `DataListing` 类型**

在 `pallets/data-registry/src/types.rs` 改为：

```rust
pub type ListingId = u32;
pub type DataDescription = BoundedVec<u8, frame_support::traits::ConstU32<512>>;

#[derive(Encode, Decode, DecodeWithMemTracking, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub enum ListingStatus {
	Active,
	Suspended,
	Retired,
}

#[derive(Encode, Decode, DecodeWithMemTracking, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub struct DataListing<AccountId, Balance, Hash> {
	pub owner: AccountId,
	pub imt_root: Hash,
	pub description: DataDescription,
	pub price_per_round: Balance,
	pub max_rounds: u32,
	pub deposit_hint: Balance,
	pub request_schema_hash: Hash,
	pub proof_params_hash: Hash,
	pub status: ListingStatus,
}
```

- [ ] **Step 4：更新 storage 泛型**

`pallets/data-registry/src/lib.rs` 中加入 Currency 关联类型：

```rust
use frame_support::traits::Currency;

pub type BalanceOf<T> =
	<<T as Config>::Currency as Currency<<T as frame_system::Config>::AccountId>>::Balance;

#[pallet::config]
pub trait Config: frame_system::Config {
	type RuntimeEvent: From<Event<Self>> + IsType<<Self as frame_system::Config>::RuntimeEvent>;
	type Currency: Currency<Self::AccountId>;
	type WeightInfo: WeightInfo;
}

#[pallet::storage]
pub type Listings<T: Config> =
	StorageMap<_, Blake2_128Concat, ListingId, DataListing<T::AccountId, BalanceOf<T>, T::Hash>>;
```

- [ ] **Step 5：更新 `publish_data` 签名和事件**

`publish_data` 改为：

```rust
pub fn publish_data(
	origin: OriginFor<T>,
	imt_root: T::Hash,
	description: DataDescription,
	price_per_round: BalanceOf<T>,
	max_rounds: u32,
	deposit_hint: BalanceOf<T>,
	request_schema_hash: T::Hash,
	proof_params_hash: T::Hash,
) -> DispatchResult
```

增加校验：

```rust
ensure!(!price_per_round.is_zero(), Error::<T>::InvalidTradeTerms);
ensure!(max_rounds > 0, Error::<T>::InvalidTradeTerms);
ensure!(!deposit_hint.is_zero(), Error::<T>::InvalidTradeTerms);
```

事件扩展：

```rust
DataPublished { listing_id: ListingId, owner: T::AccountId, price_per_round: BalanceOf<T>, max_rounds: u32 }
ListingStatusChanged { listing_id: ListingId, status: ListingStatus }
```

- [ ] **Step 6：加入 suspend/retire**

增加 extrinsic：

```rust
pub fn set_listing_status(
	origin: OriginFor<T>,
	listing_id: ListingId,
	status: ListingStatus,
) -> DispatchResult
```

要求 owner 才能调用；非 owner 返回 `NotListingOwner`。

- [ ] **Step 7：更新 mock**

`pallets/data-registry/src/mock.rs` 加入 `Balances`：

```rust
construct_runtime!(
	pub enum Test {
		System: frame_system,
		Balances: pallet_balances,
		DataRegistry: crate,
	}
);

impl crate::Config for Test {
	type RuntimeEvent = RuntimeEvent;
	type Currency = Balances;
	type WeightInfo = ();
}
```

- [ ] **Step 8：运行测试**

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-data-registry
```

预期：全部通过。

---

## Task 2：新增主链 `pallet-main-escrow`

**目的：** 实现论文 Fund 合约的主链版本。DR 资金和 DO 押金必须锁在主链，而不是 child6 本地模拟。

**Files:**

- Create: `pallets/main-escrow/Cargo.toml`
- Create: `pallets/main-escrow/src/types.rs`
- Create: `pallets/main-escrow/src/lib.rs`
- Create: `pallets/main-escrow/src/mock.rs`
- Create: `pallets/main-escrow/src/tests.rs`
- Modify: `Cargo.toml`
- Modify: `runtime/Cargo.toml`
- Modify: `runtime/src/runtime_main.rs`
- Modify: `runtime/src/configs/mod.rs`

- [ ] **Step 1：写失败测试 `dr_locks_funds_and_do_locks_deposit`**

`pallets/main-escrow/src/tests.rs`：

```rust
#[test]
fn dr_locks_funds_and_do_locks_deposit() {
	new_test_ext().execute_with(|| {
		let anchor = sp_core::H256::repeat_byte(9);

		assert_ok!(MainEscrow::open_escrow(RuntimeOrigin::signed(1), 2, 5, 100, 300, anchor));
		assert_ok!(MainEscrow::lock_funds(RuntimeOrigin::signed(1), 0));
		assert_ok!(MainEscrow::lock_deposit(RuntimeOrigin::signed(2), 0));

		let escrow = Escrows::<Test>::get(0).expect("escrow exists");
		assert_eq!(escrow.requester, 1);
		assert_eq!(escrow.data_owner, 2);
		assert_eq!(escrow.total_funds, 500);
		assert_eq!(escrow.deposit, 300);
		assert_eq!(escrow.status, EscrowStatus::Ready);
		assert_eq!(Balances::reserved_balance(1), 500);
		assert_eq!(Balances::reserved_balance(2), 300);
	});
}
```

- [ ] **Step 2：运行测试确认失败**

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-main-escrow dr_locks_funds_and_do_locks_deposit
```

预期：crate 不存在。

- [ ] **Step 3：创建 crate 与类型**

`pallets/main-escrow/src/types.rs`：

```rust
pub type EscrowId = u32;

#[derive(Encode, Decode, DecodeWithMemTracking, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub enum EscrowStatus {
	Opened,
	Funded,
	Ready,
	Settled,
	Punished,
	Cancelled,
}

#[derive(Encode, Decode, DecodeWithMemTracking, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub struct Escrow<AccountId, Balance, Hash> {
	pub requester: AccountId,
	pub data_owner: AccountId,
	pub max_rounds: u32,
	pub price_per_round: Balance,
	pub total_funds: Balance,
	pub deposit: Balance,
	pub hash_chain_anchor: Hash,
	pub paid_rounds: u32,
	pub status: EscrowStatus,
}
```

- [ ] **Step 4：实现主链 extrinsics**

`pallets/main-escrow/src/lib.rs` 必须提供：

```rust
pub fn open_escrow(origin, data_owner, max_rounds, price_per_round, deposit, hash_chain_anchor) -> DispatchResult
pub fn lock_funds(origin, escrow_id) -> DispatchResult
pub fn lock_deposit(origin, escrow_id) -> DispatchResult
pub fn settle_by_preimage(origin, escrow_id, pre_image: Vec<u8>, remaining_rounds: u32) -> DispatchResult
pub fn punish_data_owner(origin, escrow_id) -> DispatchResult
pub fn claim_last_payment(origin, escrow_id, round_index: u32) -> DispatchResult
```

规则：

- `open_escrow` 只能 DR 调用，创建 `Opened`。
- `lock_funds` 只能 DR 调用，reserve `max_rounds * price_per_round`，状态 `Funded`。
- `lock_deposit` 只能 DO 调用，必须在 `Funded` 后，reserve `deposit`，状态 `Ready`。
- `settle_by_preimage` 第一版只能 DO 调用，验证 `H^(paid_rounds)(pre_image) == anchor`，其中 `paid_rounds = max_rounds - remaining_rounds`。
- 结算时：`paid_rounds * price_per_round` 从 DR reserved 转给 DO free；剩余 funds unreserve 给 DR；DO deposit unreserve。
- `punish_data_owner` 只能 DR 调用，状态必须 `Ready`；slash DO deposit 并转给 DR，DR funds unreserve。
- `claim_last_payment` 只能 DO 调用，第一阶段要求 session 已经由 child6/bridge 标记 `LastPaymentClaimable` 后调用；若暂不接 bridge，则先要求 `Ready` 并只释放一轮金额，后续 Task 5 接 session proof 后收紧。

**权限决策：** 不要让 bridge 代替 DO/DR 签名调用以上资金 extrinsic。第一版 E2E 中由 DO/DR 客户端分别签名主链交易；bridge 只观察 child6 事件、打印应执行动作，或在 `--execute` 模式下使用事件中指定的 `actor` 对应 dev key 签名。生产语义上，资金动作仍由 DO/DR 授权。后续若要 trustless bridge，另开任务把 CCMC/Merkle proof 接入 `MainEscrow`，而不是用 sudo/root 绕过权限。

- [ ] **Step 5：写资金结算测试**

增加测试：

```rust
#[test]
fn do_claims_partial_payment_and_requester_gets_refund() {
	new_test_ext().execute_with(|| {
		let secret = b"secret".to_vec();
		let anchor = hash_n_times::<Test>(&secret, 5);
		assert_ok!(open_ready_escrow(anchor, 5, 100, 300));

		let preimage_after_two_rounds = hash_n_times_bytes::<Test>(&secret, 3);
		assert_ok!(MainEscrow::settle_by_preimage(
			RuntimeOrigin::signed(2),
			0,
			preimage_after_two_rounds,
			3,
		));

		assert_eq!(Escrows::<Test>::get(0).unwrap().status, EscrowStatus::Settled);
		assert_eq!(Balances::reserved_balance(1), 0);
		assert_eq!(Balances::reserved_balance(2), 0);
		assert_eq!(Balances::free_balance(2), 1_000_000 + 200);
	});
}
```

- [ ] **Step 6：加入 workspace/runtime**

根 `Cargo.toml` 加：

```toml
"pallets/main-escrow",
pallet-main-escrow = { path = "./pallets/main-escrow", default-features = false }
```

`runtime/Cargo.toml` 加 optional dependency，并让主链 feature 包含它：

```toml
pallet-main-escrow = { workspace = true, optional = true }
role-main = ["pallet-main-escrow"]
```

`runtime/src/runtime_main.rs` 加：

```rust
#[runtime::pallet_index(14)]
pub type MainEscrow = pallet_main_escrow;
```

`runtime/src/configs/mod.rs` 加：

```rust
#[cfg(feature = "role-main")]
impl pallet_main_escrow::Config for Runtime {
	type RuntimeEvent = RuntimeEvent;
	type Currency = Balances;
	type WeightInfo = ();
}
```

- [ ] **Step 7：验证**

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-main-escrow
SKIP_WASM_BUILD=1 cargo check -p fishbone-runtime --features role-main
```

---

## Task 3：扩展 `pallet-trade-session` 为论文 VC 状态机

**目的：** child6 的交易会话必须表达论文 Phase 2/3/4：创建请求、绑定 listing/escrow、DO 接受、proof 提交、DR 签名、数据哈希提交、付款承诺提交、争议和结算声明。

**Files:**

- Modify: `pallets/trade-session/src/types.rs`
- Create: `pallets/trade-session/src/proof.rs`
- Modify: `pallets/trade-session/src/lib.rs`
- Modify: `pallets/trade-session/src/mock.rs`
- Modify: `pallets/trade-session/src/tests.rs`
- Modify: `runtime/src/configs/mod.rs`

- [ ] **Step 1：写失败测试 `create_session_binds_listing_and_escrow`**

测试要求 session 包含 `listing_id`、`escrow_id`、`request_hash`、`price_per_round`、`max_rounds`、`hash_chain_anchor`。

```rust
#[test]
fn create_session_binds_listing_and_escrow() {
	new_test_ext().execute_with(|| {
		let anchor = hash_once(b"secret");
		assert_ok!(TradeSession::create_session(
			RuntimeOrigin::signed(1),
			0,
			42,
			2,
			sp_core::H256::repeat_byte(4),
			100,
			5,
			anchor,
			TradeSettlementMode::MainEscrow,
		));

		let session = Sessions::<Test>::get(0).expect("session exists");
		assert_eq!(session.listing_id, 0);
		assert_eq!(session.escrow_id, 42);
		assert_eq!(session.requester, 1);
		assert_eq!(session.data_owner, 2);
		assert_eq!(session.status, SessionStatus::Requested);
	});
}
```

- [ ] **Step 2：运行测试确认失败**

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session create_session_binds_listing_and_escrow
```

- [ ] **Step 3：替换 session 类型**

`pallets/trade-session/src/types.rs`：

```rust
pub type SessionId = u32;
pub type ListingId = u32;
pub type EscrowId = u32;
pub type RoundIndex = u32;

#[derive(Encode, Decode, DecodeWithMemTracking, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub enum SessionStatus {
	Requested,
	Accepted,
	Ready,
	InDelivery,
	SettlementClaimed,
	Settled,
	Disputed,
	Punished,
	Cancelled,
}

#[derive(Encode, Decode, DecodeWithMemTracking, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub enum RoundStatus {
	Opened,
	PaymentProofSubmitted,
	DataProofSubmitted,
	ProofSigned,
	DataDelivered,
	PaymentPreimageSubmitted,
	Disputed,
	Completed,
}

#[derive(Encode, Decode, DecodeWithMemTracking, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub struct TradingSession<AccountId, Balance, Hash> {
	pub listing_id: ListingId,
	pub escrow_id: EscrowId,
	pub requester: AccountId,
	pub data_owner: AccountId,
	pub request_hash: Hash,
	pub price_per_round: Balance,
	pub max_rounds: u32,
	pub hash_chain_anchor: Hash,
	pub latest_payment_preimage: Option<Hash>,
	pub completed_rounds: u32,
	pub status: SessionStatus,
	pub settlement_mode: TradeSettlementMode,
}
```

增加 `RoundState`：

```rust
pub struct RoundState<AccountId, Hash> {
	pub session_id: SessionId,
	pub round_index: RoundIndex,
	pub payment_commitment_hash: Hash,
	pub proof_hash: Option<Hash>,
	pub proof_signature_hash: Option<Hash>,
	pub delivered_data_hash: Option<Hash>,
	pub payment_preimage_hash: Option<Hash>,
	pub status: RoundStatus,
	pub last_actor: Option<AccountId>,
}
```

- [ ] **Step 4：增加 storage**

```rust
pub type Sessions<T: Config> = StorageMap<_, Blake2_128Concat, SessionId, TradingSession<T::AccountId, BalanceOf<T>, T::Hash>>;
pub type Rounds<T: Config> = StorageDoubleMap<_, Blake2_128Concat, SessionId, Blake2_128Concat, RoundIndex, RoundState<T::AccountId, T::Hash>>;
pub type NextSessionId<T: Config> = StorageValue<_, SessionId, ValueQuery>;
```

- [ ] **Step 5：实现 VC extrinsics**

必须实现：

```rust
create_session(origin, listing_id, escrow_id, data_owner, request_hash, price_per_round, max_rounds, hash_chain_anchor, settlement_mode)
accept_session(origin, session_id)
open_round(origin, session_id, round_index, payment_commitment_hash)
submit_payment_proof(origin, session_id, round_index, proof_hash)
submit_data_proof(origin, session_id, round_index, proof_hash)
submit_proof_signature(origin, session_id, round_index, signature_hash)
submit_data_delivery_hash(origin, session_id, round_index, data_hash)
submit_payment_preimage(origin, session_id, round_index, preimage_hash)
claim_settlement(origin, session_id, latest_preimage_hash, remaining_rounds)
dispute_invalid_proof(origin, session_id, round_index, proof_hash)
dispute_invalid_plaintext(origin, session_id, round_index, data_hash, expected_hash)
claim_last_payment(origin, session_id, round_index)
```

权限：

- DR：`create_session`、`submit_payment_proof`、`submit_proof_signature`、`submit_payment_preimage`、`dispute_*`。
- DO：`accept_session`、`submit_data_proof`、`submit_data_delivery_hash`、`claim_settlement`、`claim_last_payment`。

- [ ] **Step 6：添加 verifier trait**

`pallets/trade-session/src/proof.rs`：

```rust
pub trait DataTradeProofVerifier<Hash> {
	fn verify_payment_commitment(previous: Hash, next: Hash, proof_hash: Hash) -> bool;
	fn verify_data_proof(proof_hash: Hash) -> bool;
	fn verify_plaintext_hash(data_hash: Hash, expected_hash: Hash) -> bool;
	fn verify_signature(proof_hash: Hash, signature_hash: Hash) -> bool;
}

pub struct AlwaysPassVerifier;
impl<Hash: PartialEq + Copy> DataTradeProofVerifier<Hash> for AlwaysPassVerifier {
	fn verify_payment_commitment(_: Hash, _: Hash, _: Hash) -> bool { true }
	fn verify_data_proof(_: Hash) -> bool { true }
	fn verify_plaintext_hash(data_hash: Hash, expected_hash: Hash) -> bool { data_hash == expected_hash }
	fn verify_signature(_: Hash, _: Hash) -> bool { true }
}
```

`Config` 增加：

```rust
type ProofVerifier: DataTradeProofVerifier<Self::Hash>;
```

- [ ] **Step 6.1：添加 listing 查询 trait**

`trade-session` 不能盲信 `listing_id`。在 `pallets/trade-session/src/types.rs` 或 `proof.rs` 增加：

```rust
pub trait ListingProvider<AccountId, Balance, Hash> {
	fn listing_exists(listing_id: ListingId) -> bool;
	fn listing_owner(listing_id: ListingId) -> Option<AccountId>;
	fn listing_active(listing_id: ListingId) -> bool;
	fn listing_terms(listing_id: ListingId) -> Option<(Balance, u32, Balance, Hash)>;
}

pub struct NoopListingProvider;
impl<AccountId, Balance, Hash> ListingProvider<AccountId, Balance, Hash> for NoopListingProvider {
	fn listing_exists(_: ListingId) -> bool { false }
	fn listing_owner(_: ListingId) -> Option<AccountId> { None }
	fn listing_active(_: ListingId) -> bool { false }
	fn listing_terms(_: ListingId) -> Option<(Balance, u32, Balance, Hash)> { None }
}
```

在 `pallets/data-registry/src/lib.rs` 为 `pallet_data_registry::Pallet<T>` 实现该 trait：

```rust
impl<T: Config> pallet_trade_session::ListingProvider<T::AccountId, BalanceOf<T>, T::Hash>
	for Pallet<T>
{
	fn listing_exists(listing_id: ListingId) -> bool {
		Listings::<T>::contains_key(listing_id)
	}

	fn listing_owner(listing_id: ListingId) -> Option<T::AccountId> {
		Listings::<T>::get(listing_id).map(|listing| listing.owner)
	}

	fn listing_active(listing_id: ListingId) -> bool {
		matches!(Listings::<T>::get(listing_id).map(|listing| listing.status), Some(ListingStatus::Active))
	}

	fn listing_terms(listing_id: ListingId) -> Option<(BalanceOf<T>, u32, BalanceOf<T>, T::Hash)> {
		Listings::<T>::get(listing_id).map(|listing| {
			(listing.price_per_round, listing.max_rounds, listing.deposit_hint, listing.proof_params_hash)
		})
	}
}
```

`pallet_trade_session::Config` 增加：

```rust
type ListingProvider: ListingProvider<Self::AccountId, BalanceOf<Self>, Self::Hash>;
```

`create_session` 中校验：

```rust
ensure!(T::ListingProvider::listing_exists(listing_id), Error::<T>::ListingNotFound);
ensure!(T::ListingProvider::listing_active(listing_id), Error::<T>::ListingNotActive);
ensure!(T::ListingProvider::listing_owner(listing_id) == Some(data_owner.clone()), Error::<T>::ListingOwnerMismatch);
let (listing_price, listing_rounds, _deposit_hint, _proof_params_hash) =
	T::ListingProvider::listing_terms(listing_id).ok_or(Error::<T>::ListingNotFound)?;
ensure!(listing_price == price_per_round, Error::<T>::ListingTermsMismatch);
ensure!(listing_rounds == max_rounds, Error::<T>::ListingTermsMismatch);
```

Mock 中实现 `ListingProvider`，用一个 `StorageMap` 或简单 test helper 写入 listing terms；测试必须覆盖 `create_session_rejects_missing_listing` 和 `create_session_rejects_inactive_listing`。

- [ ] **Step 7：写 happy-path session 测试**

覆盖顺序：

```text
create_session -> accept_session -> open_round -> submit_payment_proof
-> submit_data_proof -> submit_proof_signature -> submit_data_delivery_hash
-> submit_payment_preimage -> claim_settlement
```

断言：

- round 状态最终 `Completed`。
- session `completed_rounds == 1`。
- `claim_settlement` 后 status 为 `SettlementClaimed`，等待 bridge/main escrow 执行资金结算。

- [ ] **Step 8：写争议测试**

至少覆盖：

```rust
dr_can_dispute_invalid_data_proof_and_mark_session_punished()
dr_can_dispute_plaintext_hash_mismatch()
do_can_claim_last_payment_after_signature_and_delivery()
wrong_actor_cannot_advance_round()
round_steps_must_be_in_order()
```

- [ ] **Step 9：验证**

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session
SKIP_WASM_BUILD=1 cargo check -p fishbone-runtime --features scene-data-trade
```

`runtime/src/configs/mod.rs` 中扩展后的配置应为：

```rust
#[cfg(feature = "scene-data-trade")]
impl pallet_data_registry::Config for Runtime {
	type RuntimeEvent = RuntimeEvent;
	type Currency = Balances;
	type WeightInfo = ();
}

#[cfg(feature = "scene-data-trade")]
impl pallet_trade_session::Config for Runtime {
	type RuntimeEvent = RuntimeEvent;
	type Currency = Balances;
	type ListingProvider = DataRegistry;
	type ProofVerifier = pallet_trade_session::proof::AlwaysPassVerifier;
	type WeightInfo = ();
}
```

---

## Task 4：实现 data-trade bridge 观察器与可选协调器

**目的：** 当前 `scripts/bridges/data_trade.js` 只观察事件。第一版不要让 bridge 绕过 DO/DR 权限；bridge 应默认观察 child6 事件并输出应执行的主链动作。仅在 `--execute --dev-keys` 模式下，bridge 可以使用本地 dev keyring 中与事件 actor 匹配的 DO/DR 账户提交主链交易，用于实验自动化。

**Files:**

- Modify: `scripts/bridges/data_trade.js`
- Create: `scripts/lib/hash_chain.js`
- Test/Check: `node --check scripts/bridges/data_trade.js`

- [ ] **Step 1：定义事件映射**

在 `data_trade.js` 中处理：

```text
tradeSession.SessionCreated -> log mapping only
tradeSession.SettlementClaimed { actor: DO } -> suggest/call mainEscrow.settleByPreimage(escrow_id, preimage, remaining_rounds) signed by DO
tradeSession.SessionPunished { actor: DR } -> suggest/call mainEscrow.punishDataOwner(escrow_id) signed by DR
tradeSession.LastPaymentClaimed { actor: DO } -> suggest/call mainEscrow.claimLastPayment(escrow_id, round_index) signed by DO
```

- [ ] **Step 2：实现 CLI**

支持：

```bash
node scripts/bridges/data_trade.js \
  --main ws://10.2.2.11:9944 \
  --child ws://10.2.2.11:9950 \
  --once

node scripts/bridges/data_trade.js \
  --main ws://10.2.2.11:9944 \
  --child ws://10.2.2.11:9950 \
  --once \
  --execute \
  --dev-keys
```

必须解析：

- `--once`：处理一个目标事件后退出。
- `--execute`：实际提交主链交易；默认只打印。
- `--dev-keys`：只允许实验环境使用 `//Alice`/`//Bob` 这类 dev key。没有该参数时，`--execute` 必须报错退出。

- [ ] **Step 3：实现 submit helper**

使用现有脚本风格：

```js
async function submitTx(api, signer, tx, label) {
  return new Promise((resolve, reject) => {
    tx.signAndSend(signer, ({ status, dispatchError, events }) => {
      if (dispatchError) reject(new Error(`${label}: ${dispatchError.toString()}`));
      if (status.isInBlock || status.isFinalized) resolve({ status, events });
    }).catch(reject);
  });
}
```

增加 actor signer 选择：

```js
function signerForActor(keyring, actor, addresses) {
  if (actor === addresses.dataOwner) return keyring.addFromUri("//Bob");
  if (actor === addresses.dataRequester) return keyring.addFromUri("//Alice");
  throw new Error(`no dev signer for actor ${actor}`);
}
```

不要使用单一 `--signer //Alice` 代签所有资金动作。

- [ ] **Step 4：验证**

```bash
node --check scripts/bridges/data_trade.js
```

预期：退出码 0。

---

## Task 5：实现 E2E 数据交易流程脚本

**目的：** 给后续论文实验一个明确入口，自动跑 happy path 和关键恶意路径。

**Files:**

- Create: `scripts/data_trade_flow.js`
- Create: `scripts/lib/hash_chain.js`
- Create: `scripts/lib/data_trade_sample.js`
- Modify: `package.json`（如果已有 scripts 区域，则加入命令；没有则不强制）

- [ ] **Step 1：实现 hash chain 工具**

`scripts/lib/hash_chain.js`：

```js
import { blake2AsHex } from "@polkadot/util-crypto";

export function hashOnceHex(value) {
  return blake2AsHex(value);
}

export function hashNTimesHex(seed, rounds) {
  let current = seed;
  for (let i = 0; i < rounds; i += 1) current = hashOnceHex(current);
  return current;
}

export function paymentPreimageForRemaining(seed, remainingRounds) {
  return hashNTimesHex(seed, remainingRounds);
}
```

- [ ] **Step 2：实现示例数据**

`scripts/lib/data_trade_sample.js`：

```js
import { blake2AsHex } from "@polkadot/util-crypto";

export function sampleListing() {
  const description = "vehicle telemetry: time,power,battery_temp,location(maskable)";
  const rows = [
    { time: 20230101, power: 42, battery_temp: 31, location: "31.2304,121.4737" },
    { time: 20230102, power: 45, battery_temp: 33, location: "31.2305,121.4738" },
  ];
  const encoded = JSON.stringify(rows);
  return {
    description,
    rows,
    imtRoot: blake2AsHex(encoded),
    requestHash: blake2AsHex("range:time=2023;mask:location=city"),
    proofParamsHash: blake2AsHex("mock-proof-v1"),
    dataHash: blake2AsHex(encoded),
  };
}
```

- [ ] **Step 3：实现 happy path**

`scripts/data_trade_flow.js` 必须执行：

```text
connect main/child
select DO=Bob, DR=Alice
DO calls `dataRegistry.publishData(imtRoot, description, pricePerRound, maxRounds, depositHint, requestSchemaHash, proofParamsHash)`
DR calls `mainEscrow.openEscrow(dataOwner, maxRounds, pricePerRound, deposit, hashChainAnchor)`
DR calls `mainEscrow.lockFunds(escrowId)`
DO calls `mainEscrow.lockDeposit(escrowId)`
DR calls `tradeSession.createSession(listingId, escrowId, dataOwner, requestHash, pricePerRound, maxRounds, hashChainAnchor, MainEscrow)`
DO calls `tradeSession.acceptSession(sessionId)`
DR calls `tradeSession.openRound(sessionId, roundIndex, paymentCommitmentHash)`
DR calls `tradeSession.submitPaymentProof(sessionId, roundIndex, paymentProofHash)`
DO calls `tradeSession.submitDataProof(sessionId, roundIndex, proofBundle)`
DR calls `tradeSession.submitProofSignature(sessionId, roundIndex, signatureHash)`
DO calls `tradeSession.submitDataDeliveryHash(sessionId, roundIndex, dataHash)`
DR calls `tradeSession.submitPaymentPreimage(sessionId, roundIndex, preimageHash)`
DO calls `tradeSession.claimSettlement(sessionId, latestPreimageHash, remainingRounds)`
script directly calls mainEscrow.settleByPreimage signed by DO in first version; bridge is tested separately as observer/coordinator
print balances and assert DO gained one round payment, DR refunded remainder
```

CLI：

```bash
node scripts/data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --scenario happy
```

- [ ] **Step 4：实现 negative scenarios**

支持：

```bash
--scenario invalid-proof
--scenario invalid-plaintext
--scenario requester-refuses-payment
```

行为：

- `invalid-proof`：DO 提交 bad proof，DR 调用 `disputeInvalidProof`，bridge/main escrow punish DO。
- `invalid-plaintext`：DO 提交 data hash mismatch，DR 调用 `disputeInvalidPlaintext`，main escrow punish DO。
- `requester-refuses-payment`：流程到 DR 签名和 DO 交付后，DR 不提交 payment preimage；DO 调用 `claimLastPayment`，main escrow 支付一轮。

- [ ] **Step 5：验证**

```bash
node --check scripts/data_trade_flow.js
node scripts/data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --scenario happy
node scripts/data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --scenario invalid-proof
node scripts/data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --scenario requester-refuses-payment
```

---

## Task 6：接入 proof verifier 抽象，保留 ZK 替换点

**目的：** 第一阶段不能假装已经有完整 ZK verifier；必须把 mock proof 与真实 proof 的边界写清楚，并为后续 gnark/arkworks verifier 留接口。

**Files:**

- Modify: `pallets/trade-session/src/proof.rs`
- Modify: `pallets/trade-session/src/tests.rs`
- Create: `docs/implementation/data-trade-zk-verifier-plan.md`

- [ ] **Step 1：定义 proof 类型**

```rust
#[derive(Encode, Decode, DecodeWithMemTracking, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub enum ConstraintKind {
	Range,
	Subset,
	Substr,
}

#[derive(Encode, Decode, DecodeWithMemTracking, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub struct ProofBundle<Hash> {
	pub constraint_kind: ConstraintKind,
	pub ch_proof_hash: Hash,
	pub ro_proof_hash: Hash,
	pub public_input_hash: Hash,
}
```

- [ ] **Step 2：让 `submit_data_proof` 接收 `ProofBundle`**

第一阶段把 bundle hash 存入 round；mock verifier 只判断 `public_input_hash != Default::default()`。

- [ ] **Step 3：文档化真实 ZK 接入**

`docs/implementation/data-trade-zk-verifier-plan.md` 必须说明：

- gnark proof 生成仍在 `references/data_trade_code/snarks/gnarkzkp`。
- Solidity verifier 不能直接复制进 FRAME，需要选择：
  - off-chain verifier + signed attestation；
  - pallet 内置 verifier precompile/host function；
  - arkworks verifier 重写。
- 当前论文流程测试用 mock verifier，只证明协议状态机和公平结算，不声称证明系统已上链。

---

## Task 7：部署与 VM 验收

**目的：** 把实现部署到 `main` 和 `child6`，确认 runtime metadata 与 E2E 都正确。

**Files:**

- Modify: `deploy/config.toml`（如果新增 main binary 或 runtime feature binary 名称）
- Use: `scripts/dev_redeploy_clean_chains.sh`
- Use: `deploy/cmd/status.py`

- [ ] **Step 1：构建 binary**

根据项目现有构建命令执行：

```bash
SKIP_WASM_BUILD=1 cargo check -p fishbone-runtime --features role-main
SKIP_WASM_BUILD=1 cargo check -p fishbone-runtime --features scene-data-trade
```

如需要 release binary，构建并复制到 `deploy/bin/fishbone-node` 与 `deploy/bin/fishbone-node-data-trade`。

- [ ] **Step 1.1：修正 genesis profile 分支**

当前 `runtime/src/genesis_config_presets.rs` 对默认 non-babe 构建写死：

```rust
chain_id: 0,
scene: SceneKind::PlatformOnly,
settlement: SettlementMode::None,
```

必须改为按 feature 编译期选择：

```rust
fn default_chain_profile() -> ChainProfileInfo {
	#[cfg(feature = "scene-data-trade")]
	{
		return ChainProfileInfo {
			chain_id: 5,
			scene: SceneKind::DataTrade,
			settlement: SettlementMode::MainEscrow,
			params_hash: Default::default(),
		};
	}

	#[cfg(feature = "scene-crowdsource")]
	{
		return ChainProfileInfo {
			chain_id: 0,
			scene: SceneKind::Crowdsource,
			settlement: SettlementMode::FmcTaskBill,
			params_hash: Default::default(),
		};
	}

	#[cfg(all(not(feature = "scene-data-trade"), not(feature = "scene-crowdsource")))]
	{
		ChainProfileInfo {
			chain_id: 0,
			scene: SceneKind::PlatformOnly,
			settlement: SettlementMode::None,
			params_hash: Default::default(),
		}
	}
}
```

然后在 AURA/BABE `testnet_genesis` 中使用：

```rust
chain_profile: ChainProfileConfig { profile: default_chain_profile() },
```

验证：

```bash
SKIP_WASM_BUILD=1 cargo check -p fishbone-runtime --features scene-data-trade
SKIP_WASM_BUILD=1 cargo check -p fishbone-runtime --features role-main
```

- [ ] **Step 2：干净重部署 main 和 child6**

开发期允许清空数据目录：

```bash
scripts/dev_redeploy_clean_chains.sh --chains main,child6 --nodes f1,f2,f3,f4,f5 --logs
```

如果只改 child6，不改 main escrow，则只部署 child6；但本计划新增主链 `MainEscrow`，所以 main 必须重部署或 runtime upgrade。

- [ ] **Step 3：metadata 验证**

运行 Node metadata 检查，必须看到：

```text
main: MainEscrow present
child6: DataRegistry present, TradeSession present, Crowdsource absent
```

- [ ] **Step 4：运行 E2E**

```bash
node scripts/data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --scenario happy
node scripts/data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --scenario invalid-proof
node scripts/data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --scenario requester-refuses-payment
```

---

## Task 8：更新文档，防止误报“已实现”

**Files:**

- Modify: `docs/implementation/data-trade-implementation.md`
- Create: `docs/implementation/data-trade-flow.md`
- Modify: `docs/README.md`

- [ ] **Step 1：更新实现记录**

必须明确区分：

```text
已实现：协议状态机、主链锁资、子链 VC、E2E happy/negative flows。
仍未实现：真实上链 zk-SNARK verifier（如果 Task 6 只做 mock verifier）。
```

- [ ] **Step 2：写运行手册**

`docs/implementation/data-trade-flow.md` 包含：

- 角色账户：Alice=DR，Bob=DO。
- 链：main `9944`，child6 `9950`。
- clean redeploy 命令。
- happy path 命令。
- invalid proof / requester refuses payment 命令。
- 如何检查 metadata。

- [ ] **Step 3：验证文档链接**

```bash
rg -n "data-trade-flow|data-trade-implementation|cdt.md" docs
```

---

## 实施顺序建议

1. Task 1：DC/listing 补齐。
2. Task 2：主链 escrow，先把真实资金位置放对。
3. Task 3：child6 VC/session 状态机。
4. Task 4：bridge 连接 child6 事件和 main escrow。
5. Task 5：E2E 脚本证明论文流程跑通。
6. Task 6：proof verifier 抽象与 ZK 接入边界。
7. Task 7：VM 部署验证。
8. Task 8：文档更新。

## 风险与取舍

- **真实 ZK verifier 风险高。** gnark/Solidity verifier 不能无缝搬到 Substrate runtime。不要让后续 agent 把 mock verifier 误写成“ZK 已实现”。必须在 E2E 输出中打印 `verifier=mock` 或 `verifier=zk`。
- **主链锁资需要重部署 main。** 新增 `pallet-main-escrow` 后，当前 main runtime 不包含它；开发期建议 clean redeploy main+child6。
- **跨链桥不是安全证明。** 第一阶段 bridge 是实验协调器，不是 trustless cross-chain proof。论文公平性可先通过主链 escrow + child session 事件证明实验流程；如要严格无信任跨链，需要后续把 CCMC/Merkle proof 接入 escrow 指令验证。
- **不要复用 crowdsource。** 本计划中任何 data-trade 代码不得调用 `api.tx.crowdsource.*`，不得把 `child6` 当 task/bill 场景。

## 完成定义

只有同时满足以下条件，才能对外说“论文数据交易流程已实现第一版”：

- `pallet-main-escrow`、`pallet-data-registry`、`pallet-trade-session` 单测全部通过。
- main metadata 包含 `MainEscrow`。
- child6 metadata 包含 `DataRegistry`、`TradeSession`，不包含 `Crowdsource`。
- happy path E2E 成功完成 DO 发布、DR 锁资、DO 押金、轮次交付、DO claim、DR refund。
- 至少两个恶意路径 E2E 成功：invalid proof punish DO、requester refuses payment claim last payment。
- 文档明确 proof verifier 当前是 mock 还是 zk。
