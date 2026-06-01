# Phase 3：CDT 可定制可验证数据交易

**状态**：待执行  
**前置条件**：Phase 1 完成（`pallet-ccmc` + `pallet-fmc` 可运行）  
**目标**：在主链上实现 CDT 数据交易协议的四个核心 pallet，并配套链下 Go 服务，完成端到端数据交易流程验证。

---

## 一、规划说明

### 1.1 与 Phase 2 的关系

Phase 3 **不依赖 Phase 2**（子链众包基础设施），可以并行或优先执行。

理由：
- CDT 协议（数据交易）和众包协议（任务执行）是独立业务，共享主链资金基础设施但彼此解耦
- Phase 3 先实现**单链版本**：所有 CDT pallet 运行在主链上，不涉及跨链消息
- 单链版本验证完成后，再迁移到专属 CDT 子链（Phase 2 提供的基础设施）

### 1.2 范围界定

**Phase 3 包含**：
- `pallet-groth16-verifier`：BN254 曲线 Groth16 证明的链上验证（4 种约束类型）
- `pallet-hash-verifier`：Keccak256 哈希链的链上迭代验证
- `pallet-data-contract`（DC pallet）：数据发布与 IMT Root 管理
- `pallet-verify-contract`（VC pallet）：5 步数据交付协议，含争议仲裁和资金结算
- 链下 Go 服务：IMT 构建 + ZK 证明生成 + 链上交互

**Phase 3 暂不包含**：
- XCM 跨链通信（单链版本，留给 Phase 2/4 集成）
- PLONK 证明方案（只实现 Groth16，降低复杂度）
- Root Obfuscation 证明（电路最复杂，先实现三种基础约束）
- 完整 weight benchmark（用占位 weight，后续 Phase 4 前补充）

### 1.3 CDT 协议回顾

```
参与方：
  DO（Data Owner）：持有私有数据集，作为数据卖家
  DR（Data Requester）：想购买数据使用权，作为数据买家

五步交付协议：
  1. 协商阶段：DO 发布数据（IMT Root + 描述），DR 创建交易 Session
  2. 资金锁定：DR 锁定 locked_funds（N 轮总额），DO 锁定 deposit（押金）
  3. 分轮交付：DO 依次发送解密密钥，每轮 DR 验证后释放单轮报酬
  4. 争议处理：任意一轮 DR 发现 DO 提供了无效证明，调用链上 ZK 验证惩罚 DO
  5. 最终结算：最后一轮 DO 通过 ZK 证明从 VC 申请最终报酬

链上核心机制：
  - 哈希链：DO 预先构建 H_0 → H_1 → ... → H_N（最终哈希公开），每轮揭示前一轮哈希
  - ZK 证明：DO 对每轮数据约束（range/subset/substr）生成 Groth16 证明，DR 若怀疑可提交链上验证
  - 链上不主动运行 ZK 验证，只在 DR 发起"惩罚请求"时验证
```

### 1.4 四个 Pallet 的职责

```
pallet-groth16-verifier（纯函数库 pallet）
  └── 提供 4 种 Groth16 证明验证函数，供 pallet-verify-contract 调用
  └── 不维护存储，无 extrinsic，只有公开函数接口

pallet-hash-verifier（纯函数库 pallet）
  └── 链上 keccak256 哈希链迭代验证，返回完成轮数
  └── 不维护存储，无 extrinsic

pallet-data-contract（DC pallet）
  ├── 数据发布：DO 提交 IMT Root + 数据描述
  └── IMT Root 更新：DO 更新已发布数据的 IMT Root

pallet-verify-contract（VC pallet，主流程）
  ├── 创建交易 Session（DR 发起）
  ├── 资金锁定：DR 锁定 locked_funds，DO 锁定 deposit
  ├── 争议仲裁：验证 ZK 证明，惩罚提供无效证明的 DO
  ├── 哈希不匹配惩罚（不依赖 ZK，纯链上计算）
  └── 最终结算：DO 提交最终证明申请尾款
```

---

## 二、关键数据类型设计

```rust
// ─── pallet-groth16-verifier ─────────────────────────────────────────────────

/// BN254 Groth16 证明（非压缩格式，256 字节）
/// 对应 Solidity 中的 uint256[8] proof
#[derive(Encode, Decode, Clone, PartialEq, Eq, TypeInfo, MaxEncodedLen)]
pub struct Groth16Proof {
    pub a:  [u8; 64],   // G1 点 (x, y)，各 32 字节
    pub b:  [u8; 128],  // G2 点 (x1, x0, y1, y0)，各 32 字节
    pub c:  [u8; 64],   // G1 点 (x, y)，各 32 字节
}

/// RangeHash 约束的公开输入（3 个 Fr 元素）
/// 对应 Solidity: uint256[3] input = [hash, min, max]
pub type RangeHashInput  = [[u8; 32]; 3];

/// SubsetHash 约束的公开输入（4 个 Fr 元素）
/// 对应 Solidity: uint256[4] input = [hash, set_root, ...]
pub type SubsetHashInput = [[u8; 32]; 4];

/// SubstrHash 约束的公开输入（2 个 Fr 元素）
/// 对应 Solidity: uint256[2] input = [hash, substr_hash]
pub type SubstrHashInput = [[u8; 32]; 2];

// ─── pallet-data-contract ────────────────────────────────────────────────────

pub type ListingId = u64;

#[derive(Encode, Decode, Clone, PartialEq, Eq, TypeInfo, MaxEncodedLen)]
pub struct DataListing<AccountId, Hash> {
    pub owner:       AccountId,
    pub imt_root:    Hash,     // 完整性 Merkle 树根哈希
    pub description: BoundedVec<u8, ConstU32<512>>,
    pub created_at:  u32,      // block number
}

// ─── pallet-verify-contract ──────────────────────────────────────────────────

pub type SessionId = u64;
pub type EpochId   = u64;

#[derive(Encode, Decode, Clone, PartialEq, Eq, TypeInfo, MaxEncodedLen)]
pub struct TradingSession<AccountId, Balance, Hash> {
    pub data_requester:  AccountId,
    pub data_owner:      AccountId,
    pub listing_id:      ListingId,
    pub hash_chain_end:  Hash,      // 哈希链最终值（公开）
    pub max_rounds:      u32,       // 数据交付总轮数 N
    pub locked_funds:    Balance,   // DR 锁定资金（N 轮总额）
    pub deposit:         Balance,   // DO 锁定押金
    pub status:          SessionStatus,
}

#[derive(Encode, Decode, Clone, PartialEq, Eq, TypeInfo, MaxEncodedLen)]
pub enum SessionStatus {
    Created,          // Session 已创建，等待 DR 锁定资金
    FundsLocked,      // DR 已锁资金，等待 DO 锁定押金
    Active,           // DO 已锁押金，交付进行中
    Settled,          // 结算完成
    Punished,         // DO 被惩罚，全额归还 DR
}
```

---

## 三、实现步骤

### Step 1：验证 ark-bn254 在 WASM 环境下的可用性

在动手写业务代码之前，先确认核心密码学依赖可以在 Substrate 的 WASM 运行时编译。

**操作**：
```toml
# workspace Cargo.toml 新增
ark-bn254   = { version = "0.5", default-features = false }
ark-groth16 = { version = "0.5", default-features = false }
ark-ec      = { version = "0.5", default-features = false }
ark-ff      = { version = "0.5", default-features = false }
ark-serialize = { version = "0.5", default-features = false }
```

在 `pallets/groth16-verifier` 中写一个最小的 `lib.rs`，仅做 ark-bn254 类型引入，验证 `SKIP_WASM_BUILD=1 cargo build` 通过、`cargo build --release` 通过（含 WASM 构建）。

**预期错误及处理**：
- 若 `ark-bn254` 的某个依赖不支持 `no_std`（如用了 `std::io`），需要找到 no_std 兼容版本或 feature 开关
- 常见替代：使用 `substrate-bn` crate（专为 Substrate WASM 设计的 BN254 实现）

---

### Step 2：实现 `pallet-groth16-verifier`

该 pallet 是纯函数库，**无存储、无 extrinsic、无事件**，只暴露公开函数供其他 pallet 调用。

**目录结构**：
```
pallets/groth16-verifier/
├── Cargo.toml
└── src/
    ├── lib.rs        # pallet 定义（最小化）
    ├── types.rs      # Groth16Proof, RangeHashInput 等
    ├── vk.rs         # 各约束类型的 VK 常量（从 Solidity 合约中提取）
    ├── verifier.rs   # 核心验证逻辑
    └── tests.rs      # 使用 gnarkzkp 生成的测试向量验证
```

**关键实现**：

```rust
// lib.rs 暴露的公开接口（无 #[pallet::call]）
impl<T: Config> Pallet<T> {
    /// 验证 RangeHash Groth16 证明
    pub fn verify_range_hash_proof(
        proof: &Groth16Proof,
        input: &RangeHashInput,
    ) -> bool { ... }

    /// 验证 SubsetHash Groth16 证明
    pub fn verify_subset_hash_proof(
        proof: &Groth16Proof,
        input: &SubsetHashInput,
    ) -> bool { ... }

    /// 验证 SubstrHash Groth16 证明
    pub fn verify_substr_hash_proof(
        proof: &Groth16Proof,
        input: &SubstrHashInput,
    ) -> bool { ... }
}
```

**VK 提取**：从 `references/data_trade_code/foundry/src/gnark/groth16RangeHashProofVerifier.sol` 中提取 `ALPHA_X`, `ALPHA_Y`, `BETA_NEG_X_0` 等常量，转换为 Rust `[u8; 32]` 格式。

**测试向量**：运行 Go 程序生成测试证明，序列化后硬编码进 `tests.rs`，验证 Rust 验证结果与 Go/Solidity 一致。

---

### Step 3：实现 `pallet-hash-verifier`

对应 `references/data_trade_code/foundry/src/HashVerifier.sol` 的链上移植。

**目录结构**：
```
pallets/hash-verifier/
├── Cargo.toml
└── src/
    ├── lib.rs
    └── tests.rs
```

**核心实现**：

```rust
impl<T: Config> Pallet<T> {
    /// 验证哈希链：从 pre_image 出发迭代 keccak256，
    /// 找到与 target 匹配时返回完成轮数，
    /// 超过 max_rounds 或不匹配返回 None。
    pub fn verify_hash_chain(
        pre_image: &[u8],
        target: &T::Hash,
        max_rounds: u32,
    ) -> Option<u32>
}
```

**注意**：Substrate 的 `sp_io::hashing::keccak_256` 可直接使用，不需要额外依赖。
`max_rounds` 上限建议设为 ConstU32<10_000>，并在 weight 中按线性计量。

---

### Step 4：实现 `pallet-data-contract`（DC pallet）

对应 CDT 论文中的 DC 合约（数据发布端）。

**目录结构**：
```
pallets/data-contract/
├── Cargo.toml
└── src/
    ├── lib.rs
    ├── types.rs
    ├── mock.rs
    └── tests.rs
```

**存储**：
```rust
// 数据列表
#[pallet::storage]
pub type DataListings<T: Config> =
    StorageMap<_, Blake2_128Concat, ListingId, DataListing<T::AccountId, T::Hash>>;

// 全局 listing ID 计数器
#[pallet::storage]
pub type NextListingId<T: Config> = StorageValue<_, ListingId, ValueQuery>;
```

**Dispatchable 函数**：
```rust
// DO 发布数据（提交 IMT Root 和数据描述）
fn publish_data(
    origin: OriginFor<T>,
    imt_root: T::Hash,
    description: BoundedVec<u8, ConstU32<512>>,
) -> DispatchResult

// DO 更新 IMT Root（数据集版本更新时）
fn update_imt_root(
    origin: OriginFor<T>,
    listing_id: ListingId,
    new_root: T::Hash,
) -> DispatchResult
```

---

### Step 5：实现 `pallet-verify-contract`（VC pallet）

这是 Phase 3 工程量最大的部分，对应 `Fund.sol` + `Verify.sol` 的合并移植。

**目录结构**：
```
pallets/verify-contract/
├── Cargo.toml
└── src/
    ├── lib.rs
    ├── types.rs
    ├── mock.rs
    └── tests.rs
```

**存储**：
```rust
#[pallet::storage]
pub type Sessions<T: Config> = StorageMap<
    _, Blake2_128Concat, SessionId,
    TradingSession<T::AccountId, BalanceOf<T>, T::Hash>,
>;

#[pallet::storage]
pub type NextSessionId<T: Config> = StorageValue<_, SessionId, ValueQuery>;

/// 按轮次记录已结算的报酬，防止重复结算
#[pallet::storage]
pub type SettledRounds<T: Config> = StorageDoubleMap<
    _, Blake2_128Concat, SessionId,
    Blake2_128Concat, u32,   // round index
    bool,
>;
```

**Dispatchable 函数**：

```rust
// ── 协议建立 ──────────────────────────────────────────────────────────────────

// DR 创建交易 Session（指定 DO、哈希链终值、总轮数）
fn create_session(
    origin: OriginFor<T>,
    data_owner: T::AccountId,
    listing_id: ListingId,
    hash_chain_end: T::Hash,
    max_rounds: u32,
) -> DispatchResult

// DR 锁定资金（N 轮总额，转入 pallet 账户）
fn lock_funds(
    origin: OriginFor<T>,
    session_id: SessionId,
    amount: BalanceOf<T>,
) -> DispatchResult

// DO 锁定押金（转入 pallet 账户）
fn lock_deposit(
    origin: OriginFor<T>,
    session_id: SessionId,
    amount: BalanceOf<T>,
) -> DispatchResult

// ── 正常结算 ──────────────────────────────────────────────────────────────────

// DO 用哈希链 preImage 申请当前轮次报酬（链上验证哈希，自动结算）
fn claim_round_payment(
    origin: OriginFor<T>,
    session_id: SessionId,
    pre_image: BoundedVec<u8, ConstU32<1024>>,
) -> DispatchResult

// DO 用最终 ZK 证明申请尾款（substrHash + rootObfuscation 各验证多次）
fn claim_last_payment(
    origin: OriginFor<T>,
    session_id: SessionId,
    substr_proof: Groth16Proof,
    substr_input: SubstrHashInput,
) -> DispatchResult

// ── 争议处理（DR 调用，惩罚提供无效证明的 DO）────────────────────────────────

// DO 的 RangeHash 证明无效 → 全额罚款归 DR
fn punish_invalid_range_proof(
    origin: OriginFor<T>,
    session_id: SessionId,
    proof: Groth16Proof,
    input: RangeHashInput,
    signature: BoundedVec<u8, ConstU32<64>>,
) -> DispatchResult

// DO 的 SubsetHash 证明无效 → 全额罚款归 DR
fn punish_invalid_subset_proof(
    origin: OriginFor<T>,
    session_id: SessionId,
    proof: Groth16Proof,
    input: SubsetHashInput,
    signature: BoundedVec<u8, ConstU32<64>>,
) -> DispatchResult

// DO 的 SubstrHash 证明无效 → 全额罚款归 DR
fn punish_invalid_substr_proof(
    origin: OriginFor<T>,
    session_id: SessionId,
    proof: Groth16Proof,
    input: SubstrHashInput,
    signature: BoundedVec<u8, ConstU32<64>>,
) -> DispatchResult

// DO 提供的哈希值与链上计算不符 → 全额罚款归 DR
fn punish_hash_mismatch(
    origin: OriginFor<T>,
    session_id: SessionId,
    message: BoundedVec<u8, ConstU32<1024>>,
    expected_hash: T::Hash,
    given_hash: T::Hash,
    signature: BoundedVec<u8, ConstU32<64>>,
) -> DispatchResult
```

**`punish_*` 系列函数的内部逻辑**（以 `punish_invalid_range_proof` 为例）：
```
1. 验证调用者是 DR
2. 验证签名：signature 是 DO 对 (proof || input) 的签名（sr25519）
3. 调用 pallet_groth16_verifier::verify_range_hash_proof(proof, input)
4. 若验证通过（证明有效）→ 返回 Error::ProofIsValid（DR 不能惩罚有效证明）
5. 若验证失败（证明无效）→ 将 session 所有资金（locked_funds + deposit）转给 DR
6. 更新 session.status = Punished
```

**注意**：签名方案从 Solidity 的 ECDSA 改为 sr25519（Substrate 原生），验证逻辑使用 `sp_io::crypto::sr25519_verify`。

---

### Step 6：链下 Go 服务

对应链上 pallet 的链下配套服务，负责 ZK 证明生成和链上交互。

**目录结构**：
```
offchain/
└── cdt-service/
    ├── go.mod
    ├── main.go
    ├── imt/            # IMT 构建（复用 references/data_trade_code/snarks/gnarkzkp/merkletree/）
    │   └── imt.go
    ├── zkp/            # ZK 证明生成（复用 gnarkwrapper/groth16.go）
    │   └── prover.go
    ├── chain/          # 链上交互客户端（提交 extrinsic）
    │   └── client.go
    └── protocol/       # 5 步协议编排
        └── session.go
```

**复用策略**：
- `imt/imt.go`：直接复用 `references/.../merkletree/tree.go` 的 IMT 实现
- `zkp/prover.go`：复用 `gnarkwrapper/groth16.go` + 各约束电路，调整输出格式为 Substrate pallet 接受的字节格式
- `chain/client.go`：使用 `gsrpc`（go-substrate-rpc-client）提交 extrinsic

**ZK 证明序列化格式**（适配 Rust 端）：
```go
// Proof 序列化为 256 字节（8 × 32 bytes）
// 与 Groth16Proof Rust 类型的 SCALE 编码兼容
type ProofBytes struct {
    A [64]byte   // G1 (x, y)
    B [128]byte  // G2 (x1, x0, y1, y0) - 注意字节序
    C [64]byte   // G1 (x, y)
}
```

---

### Step 7：集成测试与 E2E 验证

**单元测试覆盖**：

```
pallet-groth16-verifier（≥ 6 个测试用例）
  ✅ 使用 Go 生成的合法证明验证通过
  ✅ 篡改证明后验证失败
  ✅ 错误 input 验证失败
  ✅ 三种约束类型各独立测试

pallet-hash-verifier（≥ 4 个测试用例）
  ✅ 正常哈希链验证，返回正确轮数
  ✅ 超过 max_rounds 返回 None
  ✅ 哈希不匹配返回 None
  ✅ 单轮哈希验证

pallet-data-contract（≥ 4 个测试用例）
  ✅ 发布数据，IMT Root 正确存储
  ✅ 非 owner 更新 Root 失败
  ✅ ListingId 递增
  ✅ 不存在的 Listing 更新失败

pallet-verify-contract（≥ 15 个测试用例）
  ✅ 完整正常流程（创建→锁资金→锁押金→按轮结算→最终结算）
  ✅ DR 用有效证明惩罚失败（Error::ProofIsValid）
  ✅ DR 用无效 RangeHash 证明惩罚 DO 成功，全额归 DR
  ✅ DR 用无效 SubsetHash 证明惩罚 DO 成功
  ✅ DR 用无效 SubstrHash 证明惩罚 DO 成功
  ✅ Hash 不匹配惩罚
  ✅ 签名验证失败（非 DO 签名）
  ✅ 重复结算同一轮次失败
  ✅ 资金不足时锁定失败
  ✅ 顺序错误时操作失败（如未锁资金就锁押金）
```

**E2E 验证脚本**（`scripts/e2e-cdt.js`）：

完整模拟一次数据交易：
```
1. DO 发布数据（调用 dc.publishData）
2. DR 创建 Session（指定 DO、hash_chain_end、max_rounds=10）
3. DR 锁定资金（调用 vc.lockFunds(session_id, 10_UNIT)）
4. DO 锁定押金（调用 vc.lockDeposit(session_id, 1_UNIT)）
5. DO 提交 round 0~8 的 pre_image，链上验证哈希链，DR 逐轮收到付款
6. 第 9 轮（最后一轮）：DO 调用 claimLastPayment 提交 ZK 证明
7. 验证 DO/DR 最终余额符合预期（DO 得到 9/10 资金 + 押金归还，DR 得到 1/10 退款）

附加：模拟争议流程
8. 新建 Session，DO 提供无效证明，DR 调用 punishIfRangeHashProofFailed
9. 验证 DO 被惩罚，全部资金归 DR
```

---

## 四、验证方式

### 验证 1：单元测试全通过

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-groth16-verifier
SKIP_WASM_BUILD=1 cargo test -p pallet-hash-verifier
SKIP_WASM_BUILD=1 cargo test -p pallet-data-contract
SKIP_WASM_BUILD=1 cargo test -p pallet-verify-contract
# 预期：全部 PASSED，≥ 29 个测试用例
```

### 验证 2：Runtime 编译通过

```bash
WASM_BUILD_RUSTFLAGS="-C link-arg=--allow-undefined" cargo build --release -p fishbone-node
# 预期：四个新 pallet 集成进 runtime 后仍可编译
```

### 验证 3：Go 链下服务运行

```bash
cd offchain/cdt-service && go test ./...
# 预期：IMT 构建和 ZK 证明生成测试通过

go run main.go --generate-test-vectors
# 输出：可用于 Rust 单元测试的证明序列化数据
```

### 验证 4：E2E 端到端验证

```bash
# 启动 dev 节点
./target/release/fishbone-node --dev --rpc-port 9944 &
sleep 5

# 运行 CDT E2E 测试
node --input-type=module < scripts/e2e-cdt.js
# 预期：正常交付流程 + 争议惩罚流程均成功完成
```

---

## 五、完成标准

- [ ] `pallet-groth16-verifier`：实现三种约束类型验证（range/subset/substr），单元测试通过（≥ 6 个）
- [ ] `pallet-hash-verifier`：链上 keccak256 迭代验证，单元测试通过（≥ 4 个）
- [ ] `pallet-data-contract`：数据发布与 IMT Root 管理，单元测试通过（≥ 4 个）
- [ ] `pallet-verify-contract`：完整 5 步协议 + 三种惩罚路径，单元测试通过（≥ 15 个）
- [ ] 四个 pallet 集成进 runtime，`cargo build --release` 成功
- [ ] Go 链下服务可生成测试向量，`go test` 通过
- [ ] E2E：正常交付流程（10 轮，DO 按比例收款）链上验证通过
- [ ] E2E：争议惩罚流程（DO 提供无效证明，全额归 DR）链上验证通过

---

## 六、关键技术决策备忘

| 问题 | 决策 | 原因 |
|------|------|------|
| ZK 库选择 | `ark-bn254` + `ark-groth16`（Rust 原生） | 不引入 EVM 层，逻辑清晰，与 Go 端 gnark 输出格式可对齐 |
| WASM 兼容性 | Step 1 先验证，不行则换 `substrate-bn` | ark 系列对 no_std 支持不完整，需要实测 |
| VK 存储方式 | 硬编码常量（与 Solidity 一致） | 简单，不占存储；如需升级通过 runtime upgrade 更新 |
| 签名方案 | sr25519（替换 Solidity 的 ECDAS/ecrecover） | Substrate 原生，DO 用 Substrate 账户签名 |
| 争议方向 | 只验证 DO 的无效行为（不保护 DO） | 论文设计：DO 主动提供数据，DR 被动验证；DO 应激励自证清白 |
| `max_rounds` 上限 | ConstU32<10_000>，weight 线性计量 | 防止哈希链验证耗尽 block weight |
| Root Obfuscation | Phase 3 暂不实现 | 电路最复杂，依赖 depth 参数；先跑通基础三种后扩展 |
| 单链 vs 多链 | Phase 3 全部在主链，不依赖 Phase 2 | 验证业务逻辑优先；后续迁移到专属子链是配置变更，无需重写 |
| 链下服务语言 | Go（复用现有 gnark 代码） | gnark 是 Go 库，无 Rust 绑定；链上用 Rust，链下用 Go，接口通过 RPC |

---

## 七、文件组织

完成后的新增文件：

```
pallets/
├── groth16-verifier/
│   ├── Cargo.toml
│   └── src/{lib.rs, types.rs, vk.rs, verifier.rs, tests.rs}
├── hash-verifier/
│   ├── Cargo.toml
│   └── src/{lib.rs, tests.rs}
├── data-contract/
│   ├── Cargo.toml
│   └── src/{lib.rs, types.rs, mock.rs, tests.rs}
└── verify-contract/
    ├── Cargo.toml
    └── src/{lib.rs, types.rs, mock.rs, tests.rs}

offchain/
└── cdt-service/
    ├── go.mod
    ├── main.go
    └── {imt/, zkp/, chain/, protocol/}

scripts/
└── e2e-cdt.js

runtime/src/configs/mod.rs   # 新增四个 pallet 的 Config impl
runtime/src/lib.rs            # 注册四个新 pallet（index 10~13）
```

---

## 八、执行记录

> 执行过程中实时更新

- [ ] Step 1：ark-bn254 WASM 兼容性验证  完成时间：
- [ ] Step 2：pallet-groth16-verifier 实现  完成时间：
- [ ] Step 3：pallet-hash-verifier 实现  完成时间：
- [ ] Step 4：pallet-data-contract 实现  完成时间：
- [ ] Step 5：pallet-verify-contract 实现  完成时间：
- [ ] Step 6：Go 链下服务实现  完成时间：
- [ ] Step 7：集成测试与 E2E 验证  完成时间：
