# Phase 2：子链基础设施

**状态**：待执行  
**前置条件**：Phase 1 完成（`pallet-ccmc` + `pallet-fmc` 可运行）  
**目标**：在子链 runtime 中实现众包协议核心 pallet（`pallet-crowdsource`），完成子链 Epoch 生命周期管理、数据收集与 Merkle 摘要生成，并通过链下桥接脚本验证主子链联动。

---

## 一、规划说明

### 1.1 范围界定

**Phase 2 包含**：
- `pallet-crowdsource`：子链核心众包逻辑（任务管理、数据收集、Epoch 结算、摘要生成）
- Epoch 参数化：通过 `Config` trait 支持收集时隙长度 `S_c`、数据验证器等可配置项
- Runtime 集成：子链（`child1-local`）使用包含 `pallet-crowdsource` 的 runtime
- 链下桥接脚本：读取子链 Epoch 事件，自动向主链 CCMC 提交摘要和向 FMC 提交账单

**Phase 2 暂不包含**：
- XCM 跨链通信（Phase 2 用链下桥接替代；XCM 留到 Relay+Parachain 迁移时引入）
- 完整的子链节点拆分（主链和子链暂共用同一 runtime，通过 chain spec 区分）
- 独立的子链 runtime crate（单独拆分留到 Phase 4 前）
- weight benchmark（用占位 weight，后续补充）

### 1.2 论文 Epoch 结构回顾

```
|<──────────── epoch e ────────────>|
|── S_c（收集时隙）──|── S_s（同步时隙）──|

S_c：子链接受工作者数据提交，矿工验证并打包进区块
S_s：矿工代表计算 Merkle 根和账单，向主链提交摘要
```

论文要求的四步流程（Phase 2 实现前三步，第四步通过链下桥接完成）：
1. **发布任务**：主链 FMC 激活任务（Phase 1 已完成）
2. **收集任务**：子链同步时隙结束时，拉取主链的激活任务到子链存储
3. **提交数据**：工作者在 `S_c` 时隙内提交数据，预算耗尽则拒绝
4. **结算账单**：子链计算 Merkle 根 + 账单，代表矿工提交至主链（链下桥接）

### 1.3 跨链通信的 Phase 2 简化方案

由于当前是 solo chain 架构（主链和子链是独立进程，无 XCM），Phase 2 使用**链下桥接**：

```
子链 pallet-crowdsource
  → finalize_epoch() 成功
  → 发出事件 Event::EpochFinalized { chain_id, epoch, merkle_root, bill_amounts }

链下桥接脚本（scripts/bridge.js）
  → 监听子链事件
  → 调用主链 ccmc.submit_epoch_digest(chain_id, epoch, merkle_root)
  → 调用主链 fmc.submit_bill(requester, task_id, epoch, bill_amounts)
```

`pallet-crowdsource` 对外暴露的接口设计为可替换（`type MainChainRelay: SubmitDigestInterface`），Phase 4 迁移到 XCM 时只需更换实现，业务逻辑不变。

---

## 二、关键数据类型设计

```rust
// ─── 类型别名 ────────────────────────────────────────────────────────────────

pub type TaskId  = pallet_fmc::types::TaskId;   // u32，与主链 FMC 对齐
pub type ChainId = pallet_ccmc::types::ChainId; // u32，与主链 CCMC 对齐
pub type EpochId = pallet_ccmc::types::EpochId; // u64

// ─── 任务信息（从主链同步来，子链存储副本）────────────────────────────────────

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub struct TaskDetail<AccountId, Balance> {
    pub requester:        AccountId,            // 主链 FMC 资金所有者
    pub budget_per_epoch: Balance,              // 每 epoch 可用预算 B
    pub description:      BoundedVec<u8, ConstU32<256>>,
    pub status:           TaskStatus,
}

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub enum TaskStatus {
    Active,      // 有预算，接受提交
    Exhausted,   // 本 epoch 预算耗尽
}

// ─── 数据提交记录 ────────────────────────────────────────────────────────────

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub struct Submission<AccountId, Balance> {
    pub worker:  AccountId,
    pub reward:  Balance,                        // 本次提交对应的奖励
    pub data:    BoundedVec<u8, ConstU32<1024>>, // 原始数据（用于 Merkle 计算）
    pub task_id: TaskId,
}

// ─── Epoch 状态 ──────────────────────────────────────────────────────────────

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub struct EpochInfo {
    pub epoch_id:    EpochId,
    pub phase:       EpochPhase,
    pub start_block: u32,        // 本 epoch 起始区块号
}

#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo, MaxEncodedLen)]
pub enum EpochPhase {
    Collecting, // S_c：接受数据提交
    Syncing,    // S_s：结算中，不接受提交
}

// ─── 账单条目（与 pallet-fmc::submit_bill 的格式对齐）─────────────────────

pub type BillEntry<AccountId, Balance> = (AccountId, Balance);
```

---

## 三、实现步骤

### Step 1：建立 pallet 目录结构

新增 `pallets/crowdsource/` crate，加入 workspace。

```
pallets/
└── crowdsource/
    ├── Cargo.toml
    └── src/
        ├── lib.rs      # pallet 主体
        ├── types.rs    # TaskDetail, Submission, EpochInfo 等
        ├── epoch.rs    # Epoch 生命周期逻辑（from on_initialize）
        ├── mock.rs     # 测试 mock
        └── tests.rs    # 单元测试
```

**Cargo.toml 核心依赖**：
```toml
[dependencies]
pallet-ccmc = { workspace = true }
pallet-fmc  = { workspace = true }
sp-io       = { workspace = true }       # keccak_256、hashing
sp-runtime  = { workspace = true }       # binary_merkle_tree
```

**workspace Cargo.toml** 新增：
```toml
[workspace.members]
"pallets/crowdsource"
```

---

### Step 2：实现 `pallet-crowdsource` 存储与 Config

**Config trait**：

```rust
#[pallet::config]
pub trait Config: frame_system::Config + pallet_ccmc::Config + pallet_fmc::Config {
    type RuntimeEvent: From<Event<Self>> + IsType<<Self as frame_system::Config>::RuntimeEvent>;

    /// 本子链在主链 CCMC 中注册的 ChainId（chain spec 里配置，ConstU32）
    #[pallet::constant]
    type ChainId: Get<ChainId>;

    /// 收集时隙长度（区块数），epoch 前 CollectingSlotBlocks 个块为 S_c
    #[pallet::constant]
    type CollectingSlotBlocks: Get<u32>;

    /// 同步时隙长度（区块数），epoch 后 SyncingSlotBlocks 个块为 S_s
    #[pallet::constant]
    type SyncingSlotBlocks: Get<u32>;

    /// 每个 epoch 最大提交总数（防止 DoS）
    #[pallet::constant]
    type MaxSubmissionsPerEpoch: Get<u32>;

    /// 数据验证器接口（可插拔，测试时用 AlwaysValidate）
    type DataValidator: ValidateSubmission<Self::AccountId>;

    type WeightInfo: WeightInfo;
}

/// 数据验证器接口
pub trait ValidateSubmission<AccountId> {
    fn validate(task_id: TaskId, worker: &AccountId, data: &[u8]) -> bool;
}

/// 默认：接受所有提交（测试/演示用）
pub struct AlwaysValidate;
impl<AccountId> ValidateSubmission<AccountId> for AlwaysValidate {
    fn validate(_: TaskId, _: &AccountId, _: &[u8]) -> bool { true }
}
```

**存储**：

```rust
/// 从主链同步来的激活任务
#[pallet::storage]
pub type ActiveTasks<T: Config> = StorageMap<
    _, Blake2_128Concat, TaskId,
    TaskDetail<T::AccountId, BalanceOf<T>>,
>;

/// 本 epoch 的提交记录
#[pallet::storage]
pub type EpochSubmissions<T: Config> = StorageValue<
    _, BoundedVec<Submission<T::AccountId, BalanceOf<T>>, T::MaxSubmissionsPerEpoch>,
    ValueQuery,
>;

/// 各任务本 epoch 已消耗的预算
#[pallet::storage]
pub type SpentBudget<T: Config> =
    StorageMap<_, Blake2_128Concat, TaskId, BalanceOf<T>, ValueQuery>;

/// 当前 Epoch 状态
#[pallet::storage]
pub type CurrentEpoch<T: Config> = StorageValue<_, EpochInfo, ValueQuery>;

/// 已确认的历史 Epoch Merkle Root（用于链下查询）
#[pallet::storage]
pub type EpochRoots<T: Config> =
    StorageMap<_, Blake2_128Concat, EpochId, T::Hash>;
```

---

### Step 3：实现 Epoch 生命周期（`on_initialize`）

`on_initialize` 在每个区块开始时自动推进 Epoch 状态机，无需人工触发：

```rust
#[pallet::hooks]
impl<T: Config> Hooks<BlockNumberFor<T>> for Pallet<T> {
    fn on_initialize(now: BlockNumberFor<T>) -> Weight {
        let epoch = CurrentEpoch::<T>::get();
        let now_u32 = now.saturated_into::<u32>();
        let epoch_start = epoch.start_block;
        let sc = T::CollectingSlotBlocks::get();
        let ss = T::SyncingSlotBlocks::get();

        let elapsed = now_u32.saturating_sub(epoch_start);

        match epoch.phase {
            EpochPhase::Collecting if elapsed >= sc => {
                // S_c 结束 → 进入 S_s
                CurrentEpoch::<T>::mutate(|e| e.phase = EpochPhase::Syncing);
                Self::deposit_event(Event::SyncingSlotStarted {
                    epoch: epoch.epoch_id,
                    block: now_u32,
                });
                Weight::from_parts(5_000, 0)
            }
            EpochPhase::Syncing if elapsed >= sc + ss => {
                // Epoch 结束 → 自动 finalize + 开启新 epoch
                Self::auto_finalize_epoch();
                Weight::from_parts(50_000, 0)
            }
            _ => Weight::zero(),
        }
    }
}
```

**`auto_finalize_epoch` 内部逻辑**（私有方法，也可由矿工代表手动提前触发）：
```
1. 收集 EpochSubmissions 中所有提交的 data 哈希
2. 用 binary_merkle_tree 计算 Merkle Root
3. 构建 bill_amounts：Vec<(AccountId, Balance)>（按 worker+miner 聚合）
4. 将 root 存入 EpochRoots
5. 发出 Event::EpochFinalized { epoch_id, merkle_root, bill_amounts }（链下桥接监听此事件）
6. 清空 EpochSubmissions 和 SpentBudget
7. 递增 epoch_id，重置为 Collecting 阶段
8. 从 ActiveTasks 更新任务状态（Exhausted → Active，重置已耗尽任务）
```

---

### Step 4：实现 Dispatchable 函数

```rust
#[pallet::call]
impl<T: Config> Pallet<T> {
    /// 矿工代表同步主链的激活任务（S_s 时隙内调用，或新 epoch 开始时）
    #[pallet::call_index(0)]
    #[pallet::weight(T::WeightInfo::sync_task())]
    pub fn sync_task(
        origin: OriginFor<T>,
        task_id: TaskId,
        requester: T::AccountId,
        budget_per_epoch: BalanceOf<T>,
        description: BoundedVec<u8, ConstU32<256>>,
    ) -> DispatchResult {
        ensure_signed(origin)?;  // 任何矿工都可以同步（后续可加矿工身份验证）
        ensure!(
            CurrentEpoch::<T>::get().phase == EpochPhase::Collecting,
            Error::<T>::NotInCollectingSlot
        );
        ActiveTasks::<T>::insert(task_id, TaskDetail {
            requester,
            budget_per_epoch,
            description,
            status: TaskStatus::Active,
        });
        Self::deposit_event(Event::TaskSynced { task_id });
        Ok(())
    }

    /// 工作者提交数据（仅 S_c 时隙有效）
    #[pallet::call_index(1)]
    #[pallet::weight(T::WeightInfo::submit_data())]
    pub fn submit_data(
        origin: OriginFor<T>,
        task_id: TaskId,
        data: BoundedVec<u8, ConstU32<1024>>,
        reward: BalanceOf<T>,
    ) -> DispatchResult {
        let who = ensure_signed(origin)?;
        ensure!(
            CurrentEpoch::<T>::get().phase == EpochPhase::Collecting,
            Error::<T>::NotInCollectingSlot
        );

        let task = ActiveTasks::<T>::get(task_id).ok_or(Error::<T>::TaskNotFound)?;
        ensure!(task.status == TaskStatus::Active, Error::<T>::BudgetExhausted);
        ensure!(
            T::DataValidator::validate(task_id, &who, &data),
            Error::<T>::InvalidData
        );

        let spent = SpentBudget::<T>::get(task_id);
        let new_spent = spent.checked_add(&reward).ok_or(Error::<T>::Overflow)?;
        ensure!(new_spent <= task.budget_per_epoch, Error::<T>::ExceedsBudget);

        EpochSubmissions::<T>::try_mutate(|subs| -> DispatchResult {
            subs.try_push(Submission { worker: who.clone(), reward, data, task_id })
                .map_err(|_| Error::<T>::SubmissionLimitReached)?;
            Ok(())
        })?;
        SpentBudget::<T>::insert(task_id, new_spent);

        // 预算恰好耗尽时标记任务
        if new_spent >= task.budget_per_epoch {
            ActiveTasks::<T>::mutate(task_id, |t| {
                if let Some(t) = t { t.status = TaskStatus::Exhausted; }
            });
        }

        Self::deposit_event(Event::DataSubmitted { task_id, worker: who, reward });
        Ok(())
    }

    /// 矿工代表手动提前触发 Epoch 结算（S_s 时隙内可调用）
    #[pallet::call_index(2)]
    #[pallet::weight(T::WeightInfo::finalize_epoch())]
    pub fn finalize_epoch(origin: OriginFor<T>) -> DispatchResult {
        ensure_signed(origin)?;
        // 验证当前是 S_s 时隙
        ensure!(
            CurrentEpoch::<T>::get().phase == EpochPhase::Syncing,
            Error::<T>::NotInSyncingSlot
        );
        Self::auto_finalize_epoch();
        Ok(())
    }
}
```

---

### Step 5：实现 Merkle Root 计算和账单生成

**Merkle Root 计算**（使用 `sp_runtime::traits::Hash` + 简单二叉 Merkle Tree）：

```rust
fn compute_epoch_merkle_root(
    submissions: &[Submission<T::AccountId, BalanceOf<T>>]
) -> T::Hash {
    if submissions.is_empty() {
        return T::Hash::default();
    }
    // 用每条提交的 SCALE 编码哈希作为叶节点
    let leaves: Vec<T::Hash> = submissions
        .iter()
        .map(|s| T::Hashing::hash_of(s))
        .collect();

    // 使用 sp_runtime::binary_merkle_tree 计算根
    let root = binary_merkle_tree::merkle_root::<T::Hashing, _>(leaves);
    root
}
```

**账单聚合**（矿工按任务分配奖励，加上矿工手续费）：

```rust
fn build_bill_amounts(
    submissions: &[Submission<T::AccountId, BalanceOf<T>>]
) -> Vec<(T::AccountId, BalanceOf<T>)> {
    // 按 worker 聚合奖励
    let mut rewards: BTreeMap<T::AccountId, BalanceOf<T>> = BTreeMap::new();
    for s in submissions {
        *rewards.entry(s.worker.clone()).or_default() += s.reward;
    }
    rewards.into_iter().collect()
}
```

**`EpochFinalized` 事件携带完整账单**（链下桥接依赖此事件）：

```rust
#[pallet::event]
pub enum Event<T: Config> {
    TaskSynced     { task_id: TaskId },
    DataSubmitted  { task_id: TaskId, worker: T::AccountId, reward: BalanceOf<T> },
    SyncingSlotStarted { epoch: EpochId, block: u32 },
    EpochFinalized {
        chain_id:     ChainId,
        epoch:        EpochId,
        merkle_root:  T::Hash,
        /// 账单：(接收方, 金额)，链下桥接读取后提交至主链 FMC
        bill_amounts: Vec<(T::AccountId, BalanceOf<T>)>,
    },
}
```

---

### Step 6：集成进 Runtime + 子链配置

**runtime/Cargo.toml**：
```toml
pallet-crowdsource = { workspace = true }
```

**runtime/src/lib.rs**（注册 pallet）：
```rust
#[runtime::pallet_index(10)]
pub type Crowdsource = pallet_crowdsource;
```

**runtime/src/configs/mod.rs**（子链 Config 实现）：
```rust
use frame_support::traits::ConstU32;

parameter_types! {
    pub const CrowdsourceChainId: pallet_ccmc::types::ChainId = 0;
    pub const CollectingSlotBlocks: u32 = 100;  // ~10 分钟 @ 6s/block
    pub const SyncingSlotBlocks: u32   = 20;    // ~2 分钟
}

impl pallet_crowdsource::Config for Runtime {
    type RuntimeEvent       = RuntimeEvent;
    type ChainId            = CrowdsourceChainId;
    type CollectingSlotBlocks = CollectingSlotBlocks;
    type SyncingSlotBlocks    = SyncingSlotBlocks;
    type MaxSubmissionsPerEpoch = ConstU32<1000>;
    type DataValidator      = pallet_crowdsource::AlwaysValidate;
    type WeightInfo         = ();
}
```

---

### Step 7：编写单元测试

**测试用例覆盖**：

```
✅ Epoch 生命周期
  - 第 101 个区块自动切换为 Syncing 阶段（CollectingSlot=100）
  - 第 121 个区块自动触发 EpochFinalized 并重置（SyncingSlot=20）
  - Syncing 阶段拒绝数据提交（NotInCollectingSlot）

✅ 任务管理
  - sync_task 成功同步任务
  - sync_task 在 Syncing 阶段失败
  - 不存在的任务拒绝数据提交

✅ 数据提交
  - 工作者正常提交，SpentBudget 正确累加
  - 超出预算（ExceedsBudget）
  - 预算恰好耗尽后任务标记为 Exhausted
  - Exhausted 任务拒绝新提交（BudgetExhausted）
  - 超过 MaxSubmissionsPerEpoch 限制拒绝（SubmissionLimitReached）

✅ Epoch 结算
  - Merkle Root 非空时正确计算
  - 无提交时 Merkle Root 为零值
  - 账单聚合：同一 worker 多次提交合并奖励
  - finalize_epoch 后 EpochSubmissions 清空
  - finalize_epoch 后 epoch_id 递增
  - finalize_epoch 在 Collecting 阶段失败（NotInSyncingSlot）
```

---

### Step 8：链下桥接脚本与 E2E 验证

编写 `scripts/bridge.js`：监听子链事件，自动向主链提交摘要和账单。

```javascript
// scripts/bridge.js
// 监听 child1（ws://127.0.0.1:9945）的 EpochFinalized 事件
// 自动在 main chain（ws://127.0.0.1:9944）提交：
//   ccmc.submitEpochDigest(chainId, epoch, merkleRoot)
//   fmc.submitBill(requester, taskId, epoch, billAmounts)
```

**E2E 验证流程**：

```bash
# 1. 启动主链 + 子链1
./target/release/fishbone-node --chain main-local   --alice --validator \
  --base-path /tmp/fb-main --node-key 000...1 --rpc-port 9944 &
./target/release/fishbone-node --chain child1-local --bob   --validator \
  --base-path /tmp/fb-ch1  --node-key 000...2 --rpc-port 9945 &
sleep 10

# 2. 主链：注册子链0、Bob 加入、Alice 充值并创建任务
node --input-type=module < scripts/e2e-setup.js

# 3. 子链：矿工同步任务，工作者提交数据
node --input-type=module < scripts/e2e-child-work.js

# 4. 等待子链 Epoch 结束（或手动触发 finalize_epoch）
# 5. 启动桥接脚本，自动提交摘要和账单到主链
node --input-type=module < scripts/bridge.js --once

# 6. 验证主链 ccmc.EpochDigests(0, 0) 有值
# 7. 验证主链 fmc.FundPools 余额正确更新
```

---

## 四、验证方式

### 验证 1：单元测试全通过

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-crowdsource
# 预期：≥ 14 个测试全部 PASSED
```

### 验证 2：Runtime 编译

```bash
WASM_BUILD_RUSTFLAGS="-C link-arg=--allow-undefined" cargo build --release -p fishbone-node
# 预期：包含 Crowdsource pallet 的 runtime 编译成功
```

### 验证 3：双链 E2E（主链 + child1）

```bash
bash scripts/e2e-phase2.sh
# 预期：子链 Epoch 结算事件 → 桥接脚本 → 主链摘要存储 → 主链账单结算
```

---

## 五、完成标准

- [ ] `pallet-crowdsource` 代码实现完成（lib.rs + epoch.rs + types.rs + mock.rs + tests.rs）
- [ ] `on_initialize` 自动推进 Epoch 阶段（Collecting → Syncing → 新 Epoch）
- [ ] 数据提交逻辑完整（预算追踪 + 数据验证器 + 上限防护）
- [ ] Merkle Root 计算正确（`binary_merkle_tree` + SCALE 哈希叶节点）
- [ ] 账单聚合正确（同一 worker 多次提交合并）
- [ ] 单元测试 ≥ 14 个，全部通过
- [ ] runtime 集成成功，`cargo build --release` 通过
- [ ] 链下桥接脚本（`scripts/bridge.js`）可正确转发主子链消息
- [ ] E2E 双链验证：子链 Epoch 结算 → 主链摘要存储 → FMC 账单结算，全流程通过

---

## 六、关键技术决策备忘

| 问题 | 决策 | 原因 |
|------|------|------|
| 跨链通信 | 链下桥接脚本（Phase 2），不用 XCM | solo chain 架构不支持 XCM；桥接脚本等价实现同样功能，业务逻辑不受影响 |
| 主子链 runtime | 共用同一 runtime | 拆分 runtime 工作量大，Phase 2 不必要；单独子链 runtime 在 Relay+Parachain 迁移时自然分离 |
| Epoch 推进方式 | `on_initialize` 自动推进 + 可手动 `finalize_epoch` | 自动化保证时序可预测；手动触发支持提前结算（测试中有用） |
| Merkle 树实现 | `sp_runtime::binary_merkle_tree` | Substrate 内置，WASM 安全，无外部依赖 |
| 叶节点哈希 | 提交记录的 SCALE 编码哈希 | 确定性强，链下可复现，与主链 CCMC 验证逻辑一致 |
| 数据验证器 | 插件式 `Config::DataValidator` trait | 不同子链验证逻辑不同（位置数据/图像/文本），相互独立 |
| 账单提交者 | 任意矿工（无额外身份验证） | Phase 2 简化；Phase 4 集成 BPiano 后可限定为有证明的代表矿工 |
| `MaxSubmissionsPerEpoch` | ConstU32<1000>（可配置）| 防止 `on_initialize` 中 finalize 时 weight 失控 |

---

## 七、执行记录

> 执行过程中实时更新

- [ ] Step 1：pallet 目录建立 + Cargo 依赖配置  完成时间：
- [ ] Step 2：存储与 Config trait 实现  完成时间：
- [ ] Step 3：Epoch 生命周期（on_initialize）  完成时间：
- [ ] Step 4：Dispatchable 函数（sync_task / submit_data / finalize_epoch）  完成时间：
- [ ] Step 5：Merkle Root + 账单生成  完成时间：
- [ ] Step 6：集成进 Runtime + 子链配置  完成时间：
- [ ] Step 7：单元测试编写与通过  完成时间：
- [ ] Step 8：链下桥接脚本 + E2E 双链验证  完成时间：
