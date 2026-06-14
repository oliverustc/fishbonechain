# 平台层与场景层解耦实施计划

> **给后续 agent/工程执行者：** 实施本计划前，必须使用 `superpowers:subagent-driven-development`（推荐）或 `superpowers:executing-plans`，按任务逐项执行。任务使用 checkbox（`- [ ]`）跟踪进度。

**目标：** 把当前以数据众包为主的 FishboneChain 实验系统整理成一个“安全、可扩展的数据流通平台”，使不同子链可以分别承载数据众包、数据交易、zk+机器学习可验证训练、zkVM 数据不出域分析等场景，而不会默认继承众包语义。

**核心架构：** `pallet-ccmc` 继续作为平台层的子链注册、矿工管理和摘要锚定模块。`pallet-fmc`/后续 `pallet-tmc` 作为可选平台能力，适合周期性预算、任务账单和多 epoch 流动性管理；但数据交易等场景也可以选择传统的主链锁资/押金托管方式。每条子链通过显式的链 profile 声明自身 `chain_id`、场景类型、结算模式和参数，而不是从硬编码 runtime 常量或众包脚本中继承。

**技术栈：** Substrate FRAME、Rust runtime feature、Node.js `@polkadot/api` 脚本、Python 部署配置、现有 `pallet-ccmc`、`pallet-fmc`、`pallet-crowdsource`，以及后续 CDT 场景 pallet。

---

## 设计原则

### 平台层

- `pallet-ccmc` 是平台必需能力，负责子链注册、矿工集合、Epoch 摘要确认和惩罚投票。
- `pallet-fmc` 是平台可选能力，负责周期性预算托管、任务激活、账单多数确认和奖励结算。
- 未来 `pallet-tmc` 可以作为主链任务元数据/任务模板管理能力，但不应把众包专用字段固化为平台协议。
- 平台层不应该依赖 `pallet-crowdsource`、CDT、zkML 或 zkVM 场景 pallet。

### 场景层

- `pallet-crowdsource` 只代表数据众包场景。
- 数据交易场景应新增独立 pallet，例如 `pallet-data-registry`、`pallet-trade-session`、`pallet-hash-verifier`，不复用 `crowdsource` 的 worker/task/bill 模型。
- zk+机器学习训练、zkVM 数据分析等后续场景也应该新增自己的 scene pallet 和 bridge adapter。
- 场景 pallet 可以选择使用 FMC，也可以选择不使用 FMC。

### 结算模式

- `FmcTaskBill`：周期性任务账单模式。典型场景是数据众包，子链聚合账单后提交主链 FMC。
- `MainEscrow`：传统主链锁资/押金模式。典型场景是 CDT 数据交易，DR 锁定资金，DO 锁定押金，合约根据哈希链和争议证据结算。
- `Hybrid`：混合模式。适合未来既有周期性预算，又有复杂争议/最终结算的服务。
- `None`：只使用平台的链身份和摘要锚定，不使用平台资金模块。

### 兼容要求

- 现有众包实验必须能继续运行。
- 现有 `pallet-ccmc`、`pallet-fmc`、`pallet-crowdsource` 测试必须保持通过。
- 第一阶段不实现完整 ZK verifier，只先建立清晰的 runtime、pallet 和脚本边界。

---

## 当前必须排除的隐患

1. `pallet-crowdsource` 当前在 runtime 中使用硬编码 `CrowdsourceChainId = 0`。多条子链共用 runtime 时，事件里的 `chain_id` 会误导 bridge，把不同子链摘要/账单提交到同一个 CCMC id。
2. 当前 AURA/BABE runtime 都固定包含 `Ccmc`、`Fmc`、`Crowdsource`，导致主链、众包子链、数据交易子链没有角色区分。
3. `scripts/bridge.js` 只监听 `crowdsource.EpochFinalized`，不是通用平台 bridge。
4. `scripts/setup_experiment.js`、`scripts/setup_selected_child_chains.js`、`scripts/worker.js` 都把“子链场景”等价为众包提交。
5. 数据交易如果直接塞进 `pallet-fmc` 或 `pallet-crowdsource`，会污染平台模型，也会让 DO/DR、押金、哈希链付款、ZK 争议这些 CDT 语义变得别扭。

---

## 文件规划

### 新增文件

- `pallets/chain-profile/`：链身份与场景 profile pallet。
- `pallets/data-registry/`：CDT 数据合约 DC 的 Substrate 骨架，负责数据 listing、IMT root 和描述。
- `pallets/trade-session/`：CDT 验证/资金会话 VC/Fund 骨架，负责 DR 锁资、DO 押金、哈希链 claim 和争议入口。
- `scripts/bridges/crowdsource.js`：众包场景 bridge。
- `scripts/bridges/data_trade.js`：数据交易场景 bridge 初版。
- `scripts/profiles/chains.json`：脚本读取的子链 profile 配置。
- `docs/architecture/platform-architecture.md`：平台层/场景层边界说明。
- `docs/implementation/data-trade-implementation.md`：数据交易场景实现记录。

### 修改文件

- `Cargo.toml`：加入新 pallet workspace 成员。
- `runtime/Cargo.toml`：加入新 pallet 依赖和 runtime feature。
- `runtime/src/configs/mod.rs`：接入 chain profile，移除众包 chain id 硬编码。
- `runtime/src/runtime_aura.rs`、`runtime/src/runtime_babe.rs` 或拆分后的 runtime 文件：按角色/场景组织 pallet。
- `runtime/src/lib.rs`：增加 runtime profile feature 选择和非法 feature 组合保护。
- `pallets/crowdsource/src/lib.rs`：从 chain profile 获取 chain id。
- `scripts/bridge.js`：变成兼容 wrapper。
- `scripts/setup_experiment.js`、`scripts/setup_selected_child_chains.js`、`scripts/worker.js`：识别场景 profile，不再默认所有子链都是众包。
- `scripts/gen_child_specs.py`：生成 raw spec 时注入 chain profile genesis。
- `deploy/fishbone/config.py`、`deploy/config.toml`：允许子链带 scene/settlement 元数据。
- `README.md`、`agent.md`、`docs/development/developer-guide.md`、`docs/implementation/implementation-record.md`、`docs/README.md`：更新项目定位。

---

## Task 1：新增 `pallet-chain-profile`

**目的：** 让每条链在链上显式声明自己的平台身份和场景类型，解决硬编码 `chain_id` 和场景混淆问题。

**文件：**

- 新建：`pallets/chain-profile/Cargo.toml`
- 新建：`pallets/chain-profile/src/lib.rs`
- 新建：`pallets/chain-profile/src/types.rs`
- 新建：`pallets/chain-profile/src/mock.rs`
- 新建：`pallets/chain-profile/src/tests.rs`
- 修改：`Cargo.toml`

- [ ] **Step 1：先写失败测试**

在 `pallets/chain-profile/src/tests.rs` 中覆盖：

- genesis profile 可读取；
- Root 可以更新 profile；
- 普通 signed 账户不能更新 profile；
- `SceneKind` 和 `SettlementMode` 能从 storage 正确读回。

测试函数名：

```rust
#[test]
fn genesis_profile_is_readable() {}

#[test]
fn root_can_update_profile() {}

#[test]
fn signed_update_fails() {}
```

运行：

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-chain-profile
```

预期：crate 或符号还不存在，测试失败。

- [ ] **Step 2：定义 profile 类型**

在 `pallets/chain-profile/src/types.rs` 中定义：

```rust
pub type ChainId = u32;

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub enum SceneKind {
    PlatformOnly,
    Crowdsource,
    DataTrade,
    VerifiableTraining,
    ZkVmAnalytics,
    Custom,
}

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub enum SettlementMode {
    FmcTaskBill,
    MainEscrow,
    Hybrid,
    None,
}

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub struct ChainProfileInfo {
    pub chain_id: ChainId,
    pub scene: SceneKind,
    pub settlement: SettlementMode,
    pub params_hash: sp_core::H256,
}
```

- [ ] **Step 3：实现 provider trait**

在 `pallets/chain-profile/src/lib.rs` 暴露：

```rust
pub trait ChainIdentityProvider {
    fn chain_id() -> pallet_ccmc::types::ChainId;
    fn scene_kind() -> SceneKind;
    fn settlement_mode() -> SettlementMode;
}
```

`pallet-chain-profile::Pallet<T>` 实现该 trait，从 storage 读取 profile。

- [ ] **Step 4：实现 storage 和 root 更新**

Storage：

```rust
#[pallet::storage]
pub type Profile<T: Config> = StorageValue<_, ChainProfileInfo, ValueQuery>;
```

Extrinsic：

```rust
pub fn set_profile(origin, profile: ChainProfileInfo) -> DispatchResult
```

要求：

```rust
ensure_root(origin)?;
```

- [ ] **Step 5：加入 workspace**

修改根 `Cargo.toml`：

```toml
members = [
    "node",
    "pallets/template",
    "pallets/ccmc",
    "pallets/fmc",
    "pallets/crowdsource",
    "pallets/chain-profile",
    "runtime",
]

[workspace.dependencies]
pallet-chain-profile = { path = "./pallets/chain-profile", default-features = false }
```

- [ ] **Step 6：验证**

运行：

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-chain-profile
```

预期：全部通过。

---

## Task 2：移除 `pallet-crowdsource` 的硬编码 chain id

**目的：** 防止多条子链事件都携带 `chain_id=0`，避免摘要和账单误归属。

**文件：**

- 修改：`pallets/crowdsource/src/lib.rs`
- 修改：`pallets/crowdsource/src/mock.rs`
- 修改：`pallets/crowdsource/src/tests.rs`
- 修改：`pallets/crowdsource/Cargo.toml`

- [ ] **Step 1：补测试**

新增测试：mock profile 返回 `chain_id = 5` 时，`EpochFinalized` 事件也必须携带 `chain_id = 5`。

运行：

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-crowdsource emits_configured_chain_id
```

预期：旧实现下失败。

- [ ] **Step 2：替换 Config**

把：

```rust
type ChainId: Get<ChainId>;
```

替换为：

```rust
type ChainProfile: pallet_chain_profile::ChainIdentityProvider;
```

把事件中的：

```rust
chain_id: T::ChainId::get(),
```

替换为：

```rust
chain_id: T::ChainProfile::chain_id(),
```

- [ ] **Step 3：更新 mock**

在 `pallets/crowdsource/src/mock.rs` 中实现：

```rust
pub struct MockChainProfile;

impl pallet_chain_profile::ChainIdentityProvider for MockChainProfile {
    fn chain_id() -> pallet_ccmc::types::ChainId { 5 }
    fn scene_kind() -> pallet_chain_profile::types::SceneKind {
        pallet_chain_profile::types::SceneKind::Crowdsource
    }
    fn settlement_mode() -> pallet_chain_profile::types::SettlementMode {
        pallet_chain_profile::types::SettlementMode::FmcTaskBill
    }
}
```

- [ ] **Step 4：验证**

运行：

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-crowdsource
```

预期：全部通过。

---

## Task 3：在 runtime 中接入 Chain Profile

**目的：** 让 runtime 能从 genesis/profile storage 获取链身份，而不是在 `configs/mod.rs` 写死。

**文件：**

- 修改：`runtime/Cargo.toml`
- 修改：`runtime/src/configs/mod.rs`
- 修改：`runtime/src/runtime_aura.rs`
- 修改：`runtime/src/runtime_babe.rs`
- 修改：`runtime/src/genesis_config_presets.rs`

- [ ] **Step 1：加入依赖**

在 `runtime/Cargo.toml` 添加：

```toml
pallet-chain-profile.workspace = true
```

并加入 `std`、`runtime-benchmarks`、`try-runtime` feature 列表。

- [ ] **Step 2：加入 pallet index**

为了尽量不破坏已有 pallet index，使用未占用的 index：

```rust
#[runtime::pallet_index(13)]
pub type ChainProfile = pallet_chain_profile;
```

- [ ] **Step 3：配置 pallet**

在 `runtime/src/configs/mod.rs` 中加入：

```rust
impl pallet_chain_profile::Config for Runtime {
    type RuntimeEvent = RuntimeEvent;
    type WeightInfo = ();
}
```

把 `pallet_crowdsource::Config` 中的：

```rust
type ChainId = CrowdsourceChainId;
```

替换为：

```rust
type ChainProfile = pallet_chain_profile::Pallet<Runtime>;
```

删除 `CrowdsourceChainId` 常量。

- [ ] **Step 4：配置 genesis profile**

主链默认：

```rust
ChainProfileInfo {
    chain_id: 0,
    scene: SceneKind::PlatformOnly,
    settlement: SettlementMode::None,
    params_hash: Default::default(),
}
```

子链 profile 后续由 `scripts/gen_child_specs.py` 注入。

- [ ] **Step 5：验证**

运行：

```bash
SKIP_WASM_BUILD=1 cargo check -p fishbone-runtime
```

预期：runtime 编译通过。

---

## Task 4：生成 chain spec 时注入子链 profile

**目的：** 每条子链的 `chain_id`、场景类型、结算模式来自 spec，而不是代码常量。

**文件：**

- 修改：`scripts/gen_child_specs.py`
- 修改：`node/src/chain_spec.rs`

- [ ] **Step 1：扩展链配置**

在 `scripts/gen_child_specs.py` 的每条链配置中加入：

```python
"profile": {
    "chainId": 5,
    "scene": "DataTrade",
    "settlement": "MainEscrow",
    "paramsHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
}
```

现有众包子链使用：

```python
"scene": "Crowdsource",
"settlement": "FmcTaskBill",
```

数据交易子链使用：

```python
"scene": "DataTrade",
"settlement": "MainEscrow",
```

- [ ] **Step 2：注入 genesis patch**

添加：

```python
def inject_chain_profile(spec: dict, profile: dict) -> dict:
    patch = spec["genesis"]["runtimeGenesis"]["patch"]
    patch["chainProfile"] = {"profile": profile}
    return spec
```

在 validator 注入后调用。

- [ ] **Step 3：验证**

运行：

```bash
python3 scripts/gen_child_specs.py
```

预期：生成的 raw spec 包含 `chainProfile.profile`。

---

## Task 5：把 bridge 拆成场景 adapter

**目的：** 平台不再只有 `crowdsource.EpochFinalized` 这一种事件模型。

**文件：**

- 新建：`scripts/bridges/crowdsource.js`
- 新建：`scripts/bridges/data_trade.js`
- 修改：`scripts/bridge.js`
- 新建：`scripts/profiles/chains.json`

- [ ] **Step 1：移动众包 bridge**

把当前 `scripts/bridge.js` 的主体移动到：

```text
scripts/bridges/crowdsource.js
```

同时修改 chain id 处理逻辑：

```js
const eventChainId = rawChainId?.toNumber?.();
if (eventChainId !== undefined && eventChainId !== CHAIN_ID) {
  throw new Error(`event chain_id ${eventChainId} does not match configured CHAIN_ID ${CHAIN_ID}`);
}
const chain_id = CHAIN_ID;
```

这样即使子链事件错误，也不会把账单提交到错误的主链 chain id。

- [ ] **Step 2：保留兼容 wrapper**

`scripts/bridge.js` 改成：

```js
import "./bridges/crowdsource.js";
```

- [ ] **Step 3：新增数据交易 bridge 骨架**

`scripts/bridges/data_trade.js` 初版只做：

- 连接子链和主链 RPC；
- 监听 `dataRegistry` 和 `tradeSession` 事件；
- `--once` 模式下观察到一个支持事件后退出；
- 不提交 FMC bill。

- [ ] **Step 4：新增 profile 配置**

`scripts/profiles/chains.json`：

```json
{
  "child1": { "chainId": 0, "scene": "Crowdsource", "settlement": "FmcTaskBill" },
  "child2": { "chainId": 1, "scene": "Crowdsource", "settlement": "FmcTaskBill" },
  "child3": { "chainId": 2, "scene": "Crowdsource", "settlement": "FmcTaskBill" },
  "child4": { "chainId": 3, "scene": "Crowdsource", "settlement": "FmcTaskBill" },
  "child5": { "chainId": 4, "scene": "Crowdsource", "settlement": "FmcTaskBill" },
  "child6": { "chainId": 5, "scene": "DataTrade", "settlement": "MainEscrow" }
}
```

- [ ] **Step 5：验证**

运行：

```bash
node --check scripts/bridge.js
node --check scripts/bridges/crowdsource.js
node --check scripts/bridges/data_trade.js
```

预期：语法检查通过。

---

## Task 6：让初始化和压测脚本感知场景 profile

**目的：** 不能再默认所有子链都执行 `crowdsource.syncTask` 和 `crowdsource.submitData`。

**文件：**

- 修改：`scripts/setup_experiment.js`
- 修改：`scripts/setup_selected_child_chains.js`
- 修改：`scripts/worker.js`

- [ ] **Step 1：读取 profile**

脚本中读取：

```js
import { readFileSync } from "fs";

const CHAIN_PROFILES = JSON.parse(
  readFileSync(new URL("./profiles/chains.json", import.meta.url), "utf8")
);
```

- [ ] **Step 2：只对众包链执行众包初始化**

在 `setup_selected_child_chains.js` 调用 `crowdsource.syncTask` 前检查：

```js
if (CHAIN_PROFILES[chain].scene !== "Crowdsource") {
  log(`${chain}: skip crowdsource setup for scene=${CHAIN_PROFILES[chain].scene}`);
  await api.disconnect();
  return;
}
```

- [ ] **Step 3：限制 worker 协议**

给 `scripts/worker.js` 增加 `--protocol`，默认 `crowdsource`。如果不是 `crowdsource`，报错：

```js
throw new Error("worker.js only supports protocol=crowdsource; use a scene-specific load generator");
```

- [ ] **Step 4：验证**

运行：

```bash
node --check scripts/setup_experiment.js
node --check scripts/setup_selected_child_chains.js
node --check scripts/worker.js
```

预期：语法检查通过。

---

## Task 7：新增 CDT 数据登记 pallet 骨架

**目的：** 为数据交易场景建立 DC 合约等价物，不依赖 `pallet-crowdsource`。

**文件：**

- 新建：`pallets/data-registry/Cargo.toml`
- 新建：`pallets/data-registry/src/lib.rs`
- 新建：`pallets/data-registry/src/types.rs`
- 新建：`pallets/data-registry/src/mock.rs`
- 新建：`pallets/data-registry/src/tests.rs`
- 修改：`Cargo.toml`

- [ ] **Step 1：先写测试**

测试覆盖：

- DO 可以发布 listing；
- listing 包含 IMT root 和描述；
- 只有 owner 可以更新 root；
- listing id 自动递增。

目标接口：

```rust
publish_data(imt_root: T::Hash, description: BoundedVec<u8, ConstU32<512>>)
update_imt_root(listing_id: ListingId, new_root: T::Hash)
```

- [ ] **Step 2：实现最小 storage**

```rust
pub type Listings<T: Config> =
    StorageMap<_, Blake2_128Concat, ListingId, DataListing<T::AccountId, T::Hash>>;

pub type NextListingId<T: Config> = StorageValue<_, ListingId, ValueQuery>;
```

事件：

```rust
DataPublished { listing_id, owner }
ImtRootUpdated { listing_id, new_root }
```

- [ ] **Step 3：验证**

运行：

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-data-registry
```

预期：全部通过。

---

## Task 8：新增 CDT 交易会话 pallet 骨架

**目的：** 为数据交易场景建立 VC/Fund 合约等价物，第一版支持传统主链锁资 `MainEscrow`。

**文件：**

- 新建：`pallets/trade-session/Cargo.toml`
- 新建：`pallets/trade-session/src/lib.rs`
- 新建：`pallets/trade-session/src/types.rs`
- 新建：`pallets/trade-session/src/mock.rs`
- 新建：`pallets/trade-session/src/tests.rs`
- 修改：`Cargo.toml`

- [ ] **Step 1：先写测试**

测试覆盖：

- DR 创建 session；
- DR 锁定资金；
- DO 在资金锁定后锁押金；
- DO 用合法哈希链 preimage claim；
- 非 DO 不能替 DO 锁押金；
- `MainEscrow` 模式不调用 FMC。

- [ ] **Step 2：定义 session 类型**

```rust
pub type SessionId = u32;

pub enum SessionStatus {
    Created,
    Funded,
    DepositLocked,
    Settled,
    Punished,
}

pub enum TradeSettlementMode {
    MainEscrow,
    FmcAssisted,
    Hybrid,
}

pub struct TradingSession<AccountId, Balance, Hash> {
    pub requester: AccountId,
    pub data_owner: AccountId,
    pub hash_chain_end: Hash,
    pub max_rounds: u32,
    pub locked_funds: Balance,
    pub deposit: Balance,
    pub status: SessionStatus,
    pub settlement_mode: TradeSettlementMode,
}
```

- [ ] **Step 3：实现哈希链验证**

先使用 runtime hashing，后续再按论文需要换成确定的哈希函数：

```rust
fn verify_hash_chain(pre_image: &[u8], target: T::Hash, max_rounds: u32) -> Option<u32>
```

要求：

- 最多迭代 `max_rounds` 次；
- 返回完成轮数；
- 超过上限或无法匹配返回 `None`。

- [ ] **Step 4：实现 dispatchable**

```rust
create_session(data_owner, hash_chain_end, max_rounds, settlement_mode)
lock_funds(session_id, amount)
lock_deposit(session_id, amount)
claim_funds(session_id, pre_image)
punish(session_id)
```

第一版：

- `MainEscrow` 可用；
- `FmcAssisted` 和 `Hybrid` 只记录模式，但返回 `UnsupportedSettlementMode`，后续单独实现 FMC adapter。

- [ ] **Step 5：验证**

运行：

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session
```

预期：全部通过。

---

## Task 9：拆分 runtime profile

**目的：** 主链、众包子链、数据交易子链不再使用同一个全量 runtime。

**文件：**

- 修改：`runtime/Cargo.toml`
- 新建：`runtime/src/runtime_main.rs`
- 新建：`runtime/src/runtime_crowdsource.rs`
- 新建：`runtime/src/runtime_data_trade.rs`
- 修改：`runtime/src/lib.rs`
- 修改：`Makefile`
- 修改：`node/src/chain_spec.rs`

- [ ] **Step 1：增加 runtime feature**

```toml
role-main = []
scene-crowdsource = ["pallet-crowdsource"]
scene-data-trade = ["pallet-data-registry", "pallet-trade-session"]
```

把 `pallet-crowdsource`、`pallet-data-registry`、`pallet-trade-session` 设为 optional。

- [ ] **Step 2：增加非法组合保护**

在 `runtime/src/lib.rs` 中加入：

```rust
#[cfg(all(feature = "role-main", any(feature = "scene-crowdsource", feature = "scene-data-trade")))]
compile_error!("role-main cannot be combined with scene child-chain features");

#[cfg(all(feature = "scene-crowdsource", feature = "scene-data-trade"))]
compile_error!("select exactly one scene feature per child runtime build");
```

- [ ] **Step 3：拆 runtime 文件**

`runtime_main.rs` 包含：

- `System`
- `Timestamp`
- `Aura`
- `Grandpa`
- `Balances`
- `TransactionPayment`
- `Sudo`
- `Ccmc`
- `Fmc`
- `ChainProfile`

`runtime_crowdsource.rs` 在上述基础上加入：

- `Crowdsource`

`runtime_data_trade.rs` 在平台基础上加入：

- `DataRegistry`
- `TradeSession`

并且不包含 `Crowdsource`。

- [ ] **Step 4：更新 Makefile**

新增：

```make
build-main:
	WASM_BUILD_RUSTFLAGS="$(WASM_FLAGS)" cargo build --release -p fishbone-node --features fishbone-runtime/role-main

build-crowdsource-child:
	WASM_BUILD_RUSTFLAGS="$(WASM_FLAGS)" cargo build --release -p fishbone-node --features fishbone-runtime/scene-crowdsource

build-data-trade-child:
	WASM_BUILD_RUSTFLAGS="$(WASM_FLAGS)" cargo build --release -p fishbone-node --features fishbone-runtime/scene-data-trade
```

- [ ] **Step 5：验证**

运行：

```bash
SKIP_WASM_BUILD=1 cargo check -p fishbone-runtime --features role-main
SKIP_WASM_BUILD=1 cargo check -p fishbone-runtime --features scene-crowdsource
SKIP_WASM_BUILD=1 cargo check -p fishbone-runtime --features scene-data-trade
```

预期：三个 runtime profile 都能编译。

---

## Task 10：更新文档

**目的：** 防止后续 agent 再把 FishboneChain 理解成“只能做数据众包”。

**文件：**

- 新建：`docs/architecture/platform-architecture.md`
- 新建：`docs/implementation/data-trade-implementation.md`
- 修改：`README.md`
- 修改：`agent.md`
- 修改：`docs/development/developer-guide.md`
- 修改：`docs/implementation/implementation-record.md`
- 修改：`docs/README.md`

- [ ] **Step 1：写平台边界文档**

`docs/architecture/platform-architecture.md` 必须说明：

- CCMC 是平台强制能力；
- FMC/TMC 是平台可选能力；
- 场景 pallet 不是平台强制能力；
- 子链通过 chain profile 声明 `SceneKind` 和 `SettlementMode`；
- 数据众包只是一个场景，不是平台本体。

- [ ] **Step 2：写数据交易实现记录**

`docs/implementation/data-trade-implementation.md` 必须说明：

- CDT 部署在专用数据交易子链；
- CDT 不依赖 `pallet-crowdsource`；
- 第一版采用 `MainEscrow`；
- `FmcAssisted` 和 `Hybrid` 留给后续需要周期预算的交易/训练/分析服务；
- ZK verifier 在会话和资金状态机稳定后再接入。

- [ ] **Step 3：更新入口文档**

更新：

- `README.md`
- `agent.md`
- `docs/development/developer-guide.md`
- `docs/implementation/implementation-record.md`
- `docs/README.md`

把项目定位改成：

```text
FishboneChain 是安全可扩展的数据流通平台；数据众包是已实现的第一个场景，数据交易是下一阶段场景。
```

- [ ] **Step 4：文档用词检查**

运行：

```bash
rg -n "CDT.*crowdsource|data trade.*crowdsource|Crowdsource.*mandatory|所有链.*Crowdsource|所有子链.*众包" README.md agent.md docs
```

预期：没有把 `crowdsource` 写成所有场景必需能力的表述。

---

## Task 11：全量验证

**目的：** 确认平台解耦没有破坏现有众包实验和新增场景骨架。

- [ ] **Step 1：运行 pallet 测试**

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-ccmc -p pallet-fmc -p pallet-crowdsource -p pallet-chain-profile -p pallet-data-registry -p pallet-trade-session
```

预期：全部通过。

- [ ] **Step 2：运行 runtime check**

```bash
SKIP_WASM_BUILD=1 cargo check -p fishbone-runtime --features role-main
SKIP_WASM_BUILD=1 cargo check -p fishbone-runtime --features scene-crowdsource
SKIP_WASM_BUILD=1 cargo check -p fishbone-runtime --features scene-data-trade
```

预期：全部通过。

- [ ] **Step 3：运行脚本语法检查**

```bash
node --check scripts/bridge.js
node --check scripts/bridges/crowdsource.js
node --check scripts/bridges/data_trade.js
node --check scripts/setup_experiment.js
node --check scripts/setup_selected_child_chains.js
node --check scripts/worker.js
```

预期：全部通过。

- [ ] **Step 4：兼容性 dry run**

```bash
node scripts/setup_experiment.js --dry-run
```

预期：现有众包实验初始化计划可以打印，不抛异常。

---

## 阶段完成标准

完成本计划后，仓库应该满足：

- 主链不再被迫包含众包场景逻辑。
- 数据交易子链不再默认包含 `pallet-crowdsource`。
- 子链身份来自 chain profile，不再依赖硬编码 `CrowdsourceChainId = 0`。
- bridge 和 setup 脚本按场景分发，不再把所有子链当作众包链。
- CDT 已有独立的 `data-registry` 和 `trade-session` 骨架。
- FMC/TMC 被明确定位为可选平台能力，而不是所有数据流通场景的唯一资金模式。

## 自查记录

- 已覆盖你补充的需求：FMC/TMC 可选、传统主链锁资可选、不同数据交易/训练/分析服务部署到不同子链。
- 没有把 CDT 绑定到 `pallet-crowdsource`。
- 没有在第一阶段引入完整 ZK verifier，避免过早扩大范围。
- `MainEscrow`、`FmcTaskBill`、`Hybrid` 三种结算模式在计划中保持一致。
