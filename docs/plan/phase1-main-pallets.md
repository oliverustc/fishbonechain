# Phase 1：FishboneChain 主链基础设施

**状态**：待执行  
**前置条件**：Phase 0 完成（`fishbone-node` 可编译运行，三链环境就绪）  
**目标**：在主链 runtime 中实现 FishboneChain 的两个核心 pallet：`pallet-fmc`（资金管理）和 `pallet-ccmc`（子链管理），并通过完整的单元测试验证业务逻辑正确性。

---

## 一、规划说明

### 1.1 范围界定

Phase 1 聚焦于**链上业务逻辑**，暂不涉及：
- XCM 跨链消息（Phase 1 结束后迁移到 Relay+Parachain 时引入）
- BLS 聚合签名（用阈值多签模拟，降低实现复杂度）
- 实际的子链 P2P 通信（链上只存摘要，链下子链独立运行）

这意味着 Phase 1 的 pallet 是完整自洽的，可以在当前 solo chain 模式下独立测试所有业务逻辑。

### 1.2 两个核心 Pallet 的职责

```
pallet-ccmc（子链管理合约）
  ├── 子链注册与参数管理
  ├── 子链矿工管理（加入/退出/押金/惩罚）
  ├── Epoch 摘要提交（Merkle Root + 多签验证）
  └── Merkle 包含性验证（Runtime API）

pallet-fmc（资金管理合约）
  ├── 请求者资金池管理（FB/LB 双余额）
  ├── 任务生命周期（创建 → 激活 → 运行 → 结算 → 终止）
  ├── 账单验证与奖励分发
  └── 双花防护（FB > B 才能激活）
```

### 1.3 关键数据类型设计

```rust
// 链 ID（简单递增整数，主链自行分配）
pub type ChainId = u32;
// 任务 ID（请求者账户下的递增序号）
pub type TaskId  = u32;
// Epoch 编号
pub type EpochId = u64;
```

---

## 二、实现步骤

### Step 1：建立 pallet 目录结构

在 workspace 中添加两个新 pallet：

```
pallets/
├── template/          # 已有，占位
├── ccmc/              # 新建
│   ├── Cargo.toml
│   └── src/
│       ├── lib.rs
│       ├── mock.rs
│       └── tests.rs
└── fmc/               # 新建
    ├── Cargo.toml
    └── src/
        ├── lib.rs
        ├── mock.rs
        └── tests.rs
```

更新根 `Cargo.toml`，将两个 pallet 加入 workspace members 和 workspace.dependencies。

更新 `runtime/Cargo.toml`，将两个 pallet 加入 runtime 依赖。

更新 `runtime/src/lib.rs`，在 `#[frame::runtime]` 中注册两个 pallet。

---

### Step 2：实现 `pallet-ccmc`

#### 2.1 存储设计

```rust
// 子链信息
#[pallet::storage]
pub type ChildChains<T: Config> =
    StorageMap<_, Blake2_128Concat, ChainId, ChainInfo<T::AccountId, BalanceOf<T>>>;

// 子链矿工信息（含押金）
#[pallet::storage]
pub type Miners<T: Config> =
    StorageDoubleMap<_, Blake2_128Concat, ChainId, Blake2_128Concat, T::AccountId, MinerInfo<BalanceOf<T>>>;

// Epoch 状态摘要（Merkle Root）
#[pallet::storage]
pub type EpochDigests<T: Config> =
    StorageDoubleMap<_, Blake2_128Concat, ChainId, Blake2_128Concat, EpochId, T::Hash>;

// 子链 ID 计数器
#[pallet::storage]
pub type NextChainId<T: Config> = StorageValue<_, ChainId, ValueQuery>;

// 矿工对 Epoch 摘要的签名投票（用于阈值多签）
#[pallet::storage]
pub type DigestVotes<T: Config> = StorageNMap<
    _,
    (NMapKey<Blake2_128Concat, ChainId>,
     NMapKey<Blake2_128Concat, EpochId>,
     NMapKey<Blake2_128Concat, T::Hash>),   // 候选 Root
    BTreeSet<T::AccountId>,                 // 投票矿工集合
>;
```

#### 2.2 关键类型定义

```rust
#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub struct ChainInfo<AccountId, Balance> {
    pub creator: AccountId,
    pub name: BoundedVec<u8, ConstU32<64>>,
    pub miner_count: u32,
    pub min_miners: u32,         // 最少矿工数（用于阈值多签）
    pub deposit_required: Balance, // 每个矿工需缴纳的押金
    pub status: ChainStatus,
}

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub enum ChainStatus { Active, Terminated }

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub struct MinerInfo<Balance> {
    pub deposit: Balance,
    pub status: MinerStatus,
}

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub enum MinerStatus { Active, Slashed }
```

#### 2.3 Dispatchable 函数

```rust
// 创建新子链（第一个矿工成为 creator）
fn register_child_chain(
    origin: OriginFor<T>,
    name: BoundedVec<u8, ConstU32<64>>,
    min_miners: u32,
    deposit_required: BalanceOf<T>,
) -> DispatchResult

// 矿工加入子链（缴纳押金，需 creator 批准或 chain 处于开放状态）
fn join_child_chain(origin: OriginFor<T>, chain_id: ChainId) -> DispatchResult

// 矿工退出子链（返还押金）
fn leave_child_chain(origin: OriginFor<T>, chain_id: ChainId) -> DispatchResult

// 提交 Epoch 摘要（任意矿工提交，达到阈值后生效）
fn submit_epoch_digest(
    origin: OriginFor<T>,
    chain_id: ChainId,
    epoch: EpochId,
    root: T::Hash,
) -> DispatchResult

// 惩罚矿工（需超过 2/3 矿工投票通过）
fn vote_slash_miner(
    origin: OriginFor<T>,
    chain_id: ChainId,
    miner: T::AccountId,
) -> DispatchResult

// 终止子链（全体矿工同意，或由 sudo 紧急终止）
fn terminate_child_chain(origin: OriginFor<T>, chain_id: ChainId) -> DispatchResult
```

#### 2.4 Runtime API（链下查询接口）

```rust
// 验证某个 block hash 是否在指定 epoch 的 Merkle Root 下
// 用于子链状态证明的链上验证基础
fn verify_block_in_epoch(
    chain_id: ChainId,
    epoch: EpochId,
    block_hash: Hash,
    proof: Vec<Hash>,   // Merkle 路径
) -> bool
```

#### 2.5 阈值多签设计（替代 BLS）

`submit_epoch_digest` 使用简单的投票聚合：
1. 每个矿工独立调用 `submit_epoch_digest(chain_id, epoch, root)`
2. 链上记录对同一 `(chain_id, epoch, root)` 的投票集合
3. 当投票数 ≥ `ceil(miner_count * 2 / 3)` 时，`root` 被确认，写入 `EpochDigests`
4. 确认后清理该 epoch 的投票记录，节省存储

---

### Step 3：实现 `pallet-fmc`

#### 3.1 存储设计

```rust
// 每个账户的 FMC 资金池（FB + LB）
#[pallet::storage]
pub type FundPools<T: Config> =
    StorageMap<_, Blake2_128Concat, T::AccountId, FundPool<BalanceOf<T>>>;

// 任务信息（由账户+任务ID唯一标识）
#[pallet::storage]
pub type Tasks<T: Config> =
    StorageDoubleMap<_, Blake2_128Concat, T::AccountId, Blake2_128Concat, TaskId, TaskInfo<T>>;

// 账户的下一个任务 ID
#[pallet::storage]
pub type NextTaskId<T: Config> =
    StorageMap<_, Blake2_128Concat, T::AccountId, TaskId, ValueQuery>;
```

#### 3.2 关键类型定义

```rust
#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub struct FundPool<Balance> {
    pub free:   Balance,   // FB：自由余额
    pub locked: Balance,   // LB：锁定余额（所有激活任务预算之和）
}

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub struct TaskInfo<T: Config> {
    pub target_chain: ChainId,        // 目标子链
    pub budget_per_epoch: BalanceOf<T>, // 每 epoch 预算 B
    pub status: TaskStatus,
    pub current_epoch: EpochId,
    pub description: BoundedVec<u8, ConstU32<256>>,
}

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub enum TaskStatus {
    Terminated,  // 余额不足或未激活
    Activated,   // B 已从 FB 锁入 LB，子链可收集
    Waiting,     // 账单已提交，等待 FMC 处理
}
```

#### 3.3 Dispatchable 函数

```rust
// 向 FMC 充值（转入 pallet 账户，记入 FB）
fn deposit(origin: OriginFor<T>, amount: BalanceOf<T>) -> DispatchResult

// 从 FMC 提款（仅能提取 FB，LB 不可提取）
fn withdraw(origin: OriginFor<T>, amount: BalanceOf<T>) -> DispatchResult

// 创建任务（初始状态 Terminated）
fn create_task(
    origin: OriginFor<T>,
    target_chain: ChainId,
    budget_per_epoch: BalanceOf<T>,
    description: BoundedVec<u8, ConstU32<256>>,
) -> DispatchResult

// 激活任务（检查 FB > B，将 B 从 FB 转入 LB）
fn activate_task(
    origin: OriginFor<T>,
    task_id: TaskId,
) -> DispatchResult

// 终止任务（将 LB 中该任务的预算归还 FB）
fn terminate_task(
    origin: OriginFor<T>,
    task_id: TaskId,
) -> DispatchResult

// 提交 Epoch 账单（由子链矿工代表调用，达到阈值后自动结算）
// bill_amounts: Vec<(AccountId, Balance)> — 奖励接收方和金额列表
fn submit_bill(
    origin: OriginFor<T>,
    requester: T::AccountId,
    task_id: TaskId,
    epoch: EpochId,
    bill_amounts: BoundedVec<(T::AccountId, BalanceOf<T>), ConstU32<1024>>,
) -> DispatchResult
```

#### 3.4 `submit_bill` 内部逻辑

```
1. 验证调用者是目标子链的注册矿工（查 CCMC）
2. 记录该矿工对此账单的投票
3. 投票达到阈值（≥ 2/3 子链矿工）后自动执行：
   a. 计算总账单金额 = sum(bill_amounts)
   b. 检查总金额 ≤ budget_per_epoch（防止超支）
   c. 将奖励转账给各接收方（从 LB 扣除）
   d. 将剩余预算（B - total_bill）归还 FB
   e. 根据 FB 余额决定任务下一 epoch 状态：
      - FB ≥ B → Activated（将新的 B 从 FB 锁入 LB）
      - FB < B → Terminated
   f. 清理该 epoch 的投票记录
```

---

### Step 4：集成进 Runtime

在 `runtime/src/lib.rs` 中：

```rust
// 为两个 pallet 实现 Config trait
impl pallet_ccmc::Config for Runtime {
    type RuntimeEvent = RuntimeEvent;
    type Currency = Balances;
    type MinDeposit = ConstU128<1_000_000_000_000>; // 1 UNIT
    type MaxMiners = ConstU32<100>;
    type MaxChains = ConstU32<256>;
    type WeightInfo = pallet_ccmc::weights::SubstrateWeight<Runtime>;
}

impl pallet_fmc::Config for Runtime {
    type RuntimeEvent = RuntimeEvent;
    type Currency = Balances;
    type CcmcPallet = Ccmc;  // 依赖 CCMC 验证矿工身份
    type MaxTasksPerAccount = ConstU32<64>;
    type WeightInfo = pallet_fmc::weights::SubstrateWeight<Runtime>;
}

// 在 construct_runtime! 中注册
#[runtime::pallet_index(8)]
pub type Ccmc = pallet_ccmc;

#[runtime::pallet_index(9)]
pub type Fmc = pallet_fmc;
```

---

### Step 5：编写单元测试

每个 pallet 的 `tests.rs` 覆盖以下场景：

#### CCMC 测试用例

```
✅ 正常流程
  - 创建子链 → 矿工加入 → 提交摘要（单矿工达阈值）→ 验证摘要存储
  - 多矿工提交不同摘要 → 投票最多的摘要胜出
  - 矿工退出 → 押金归还

✅ 错误处理
  - 非矿工调用 submit_epoch_digest → Error::NotAMiner
  - 重复提交同一 epoch → Error::AlreadyVoted
  - 账单超出预算 → Error::ExceedsBudget
  - 对已终止子链操作 → Error::ChainTerminated

✅ 边界条件
  - 矿工数恰好达到 2/3 阈值时 Epoch 确认
  - 矿工数 < 阈值时 Epoch 不确认
  - 子链只有 1 个矿工时（min_miners = 1）单人可确认
```

#### FMC 测试用例

```
✅ 正常流程
  - 充值 → 创建任务 → 激活（FB 减少，LB 增加）
  - 子链提交账单 → 结算（LB 减少，奖励发放，剩余回 FB）
  - 结算后 FB < B → 任务自动 Terminated
  - 充值后重新激活 Terminated 任务

✅ 双花防护
  - FB = 500，B = 1000 → activate_task 失败
  - 同时激活两个任务但 FB 只够一个 → 第二个失败

✅ 错误处理
  - 提款超过 FB → Error::InsufficientFreeBalance
  - 账单金额超过预算 B → Error::ExceedsBudget
  - 非矿工提交账单 → Error::NotAMiner
  - 对 Terminated 任务提交账单 → Error::TaskNotActive

✅ 边界条件
  - FB 恰好等于 B 时可激活（等号成立）
  - 账单金额恰好等于 B 时，下一轮 FB 不变
```

---

### Step 6：端到端集成验证

在本地三链网络上验证完整流程：

1. 通过 Polkadot.js Apps 在主链上：
   - Alice 调用 `ccmc.register_child_chain(...)` 注册子链1
   - Bob 调用 `ccmc.join_child_chain(chain_id)` 加入子链1
   - Alice 调用 `fmc.deposit(amount)` 充值
   - Alice 调用 `fmc.create_task(chain_id, budget, ...)` 创建任务
   - Alice 调用 `fmc.activate_task(task_id)` 激活任务

2. 验证链上存储状态：
   - `fmc.FundPools(Alice)` 中 FB 减少，LB 增加
   - `ccmc.ChildChains(0)` 中记录子链信息
   - `ccmc.Miners(0, Bob)` 中记录矿工信息和押金

3. Bob 调用 `ccmc.submit_epoch_digest(chain_id, 0, root_hash)` 提交摘要
4. 验证 `ccmc.EpochDigests(0, 0)` 存储了对应的 Root

5. Bob 调用 `fmc.submit_bill(Alice, 0, 0, [(Bob, reward)])` 提交账单
6. 验证 Bob 账户余额增加，Alice 的 FMC 资金池余额正确更新

---

## 三、验证方式

### 验证 1：单元测试全通过

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-ccmc
SKIP_WASM_BUILD=1 cargo test -p pallet-fmc
# 预期：所有测试用例 PASSED，无 failed
```

### 验证 2：Runtime 编译成功

```bash
make build-release
# 预期：fishbone-node 重新编译成功，runtime 包含两个新 pallet
```

### 验证 3：链上状态验证脚本

```bash
bash scripts/start-network.sh &
sleep 15

# 通过 RPC 查询主链存储（主链 9944 端口）
# 检查 pallet 已注册、存储可读
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"id":1,"jsonrpc":"2.0","method":"state_getMetadata","params":[]}' \
  http://127.0.0.1:9944 | python3 -c "
import sys, json
meta = json.load(sys.stdin)
# 验证 metadata 包含 Ccmc 和 Fmc pallet
print('metadata 长度:', len(meta['result']))
"
```

### 验证 4：Polkadot.js Apps 手动功能验证

按 Step 6 描述的端到端流程手动操作，截图记录关键存储状态变化。

---

## 四、完成标准

- [ ] `pallet-ccmc` 代码实现完成（lib.rs + mock.rs + tests.rs）
- [ ] `pallet-fmc` 代码实现完成（lib.rs + mock.rs + tests.rs）
- [ ] `pallet-ccmc` 单元测试全部通过（≥ 10 个测试用例）
- [ ] `pallet-fmc` 单元测试全部通过（≥ 10 个测试用例）
- [ ] 两个 pallet 集成进 runtime，`cargo build --release` 成功
- [ ] 端到端流程在本地主链上手动验证通过（子链注册 → 任务发布 → 激活 → 摘要提交 → 账单结算）

---

## 五、关键技术决策备忘

| 问题 | 决策 | 原因 |
|------|------|------|
| 聚合签名 | 阈值投票（≥2/3 矿工独立提交）替代 BLS | BLS 在 Substrate 无原生支持，后期可升级 |
| XCM | 暂不实现，Phase 1 所有调用为直接 extrinsic | 当前 solo chain 模式，迁移 relay+parachain 时再加 |
| 矿工身份验证 | 直接查 CCMC 存储，无需额外 ZK 证明 | Phase 1 信任链上状态，Phase 4 引入 BPiano 强化 |
| 账单结算 | pallet-balances 直接转账 | 简单可靠；后期可升级为 XCM 跨链发放 |
| 子链 ID | `u32` 递增，主链 CCMC 分配 | 全局唯一，无冲突风险 |

---

## 六、执行记录

> 执行过程中实时更新

- [x] Step 1：pallet 目录建立 + workspace 更新  完成时间：2026-05-31
- [x] Step 2：pallet-ccmc 实现  完成时间：2026-05-31
- [x] Step 3：pallet-fmc 实现  完成时间：2026-05-31
- [x] Step 4：集成进 Runtime  完成时间：2026-05-31
- [x] Step 5：单元测试编写与通过  完成时间：2026-05-31（22/22 通过）
- [x] Step 6：端到端集成验证  完成时间：2026-05-31（7步全部 InBlock 确认）
