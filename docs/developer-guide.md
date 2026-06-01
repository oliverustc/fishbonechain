# FishboneChain 开发者指南

> 适用对象：首次接触本项目的 Substrate/Rust 开发者  
> 覆盖范围：当前已实现的所有内容（Phase 0 + Phase 1 + 多节点部署框架）

---

## 目录

1. [项目概述](#1-项目概述)
2. [仓库结构](#2-仓库结构)
3. [架构设计](#3-架构设计)
4. [Substrate Runtime](#4-substrate-runtime)
5. [pallet-ccmc：子链管理合约](#5-pallet-ccmc子链管理合约)
6. [pallet-fmc：资金管理合约](#6-pallet-fmc资金管理合约)
7. [跨 Pallet 交互](#7-跨-pallet-交互)
8. [链配置与网络拓扑](#8-链配置与网络拓扑)
9. [多节点部署框架](#9-多节点部署框架)
10. [开发流程](#10-开发流程)
11. [当前范围与后续规划](#11-当前范围与后续规划)

---

## 1. 项目概述

FishboneChain 是基于 Substrate 实现的多链众包平台，源自论文
*"FishboneChain: A Scalable and Liquidity-Guaranteed Crowdsourcing Platform based on Multiple Child Chains"*。

**核心设计原则**：资金流（主链）与数据流（子链）分离。

```
主链（Main Chain）
  ├── 管理资金：FMC（资金管理合约）维护请求者的 FB/LB 双余额
  ├── 管理子链：CCMC（子链管理合约）注册矿工、记录 Epoch 摘要
  └── 结算账单：子链矿工提交账单 → 主链验证 → 发放奖励

子链（Child Chain）
  ├── 收集众包数据（Sc 时隙）
  ├── 生成 Epoch Merkle Root 和账单（Ss 时隙）
  └── 通过代表矿工将摘要提交至主链 CCMC
```

**当前实现状态（Phase 1 完成）**：
- 主链两个核心 pallet（CCMC + FMC）完整实现并通过单元测试（22/22）
- 6 节点 3 链真实环境运行（f1-f6，10.2.2.11-16）
- Python 管理框架（部署、状态监控、日志）

---

## 2. 仓库结构

```
fishbonechain/
├── Cargo.toml              # Workspace 根（5 个 crate）
├── rust-toolchain.toml     # Rust stable（当前 1.96.0）
│
├── node/                   # Substrate 节点可执行文件
│   └── src/
│       ├── main.rs         # 入口
│       ├── chain_spec.rs   # 链配置（main-dev / main-local / child1-local / child2-local）
│       ├── cli.rs          # CLI 参数解析
│       ├── command.rs      # 子命令（run / build-spec / key 等）
│       ├── service.rs      # 节点服务组合（consensus / networking / rpc）
│       └── rpc.rs          # RPC 扩展（System / TransactionPayment）
│
├── runtime/                # Substrate Runtime（WASM 编译目标）
│   └── src/
│       ├── lib.rs          # Runtime 定义 + Pallet 注册表
│       └── configs/
│           └── mod.rs      # 每个 Pallet 的 Config impl
│
├── pallets/
│   ├── ccmc/               # pallet-ccmc：子链管理合约
│   │   └── src/
│   │       ├── lib.rs      # Pallet 主体（存储/事件/错误/extrinsic）
│   │       ├── types.rs    # ChainInfo / MinerInfo / ChainStatus 等
│   │       ├── mock.rs     # 单元测试 mock runtime
│   │       └── tests.rs    # 9 个单元测试
│   │
│   ├── fmc/                # pallet-fmc：资金管理合约
│   │   └── src/
│   │       ├── lib.rs      # Pallet 主体（含 CcmcInterface trait + blanket impl）
│   │       ├── types.rs    # FundPool / TaskInfo / TaskStatus 等
│   │       ├── mock.rs     # 单元测试 mock（MockCcmc 替代真实 CCMC）
│   │       └── tests.rs    # 13 个单元测试
│   │
│   └── template/           # 官方模板占位 pallet（保留，未使用）
│
├── scripts/
│   ├── e2e-verify.js       # Phase 1 E2E 验证脚本（@polkadot/api）
│   ├── start-network.sh    # 本地三链启动脚本（开发用）
│   └── check-blocks.sh     # 出块状态检查
│
├── deploy/                 # 多节点生产部署框架（Python）
│   ├── config.toml         # 节点/链参数（单一真相来源）
│   ├── pyproject.toml      # Python 依赖（uv 管理）
│   ├── fishbone/           # Python 包
│   │   ├── config.py       # 读取 config.toml → 类型化对象
│   │   ├── remote.py       # SSH 执行封装（subprocess，读 ~/.ssh/config）
│   │   └── service.py      # systemd service 文件渲染器
│   └── cmd/
│       ├── status.py       # 查看所有节点状态（通过跳板机 RPC 中转）
│       ├── deploy.py       # 全量部署（推二进制 + spec + 密钥 + service）
│       ├── control.py      # start / stop / restart 节点服务
│       └── logs.py         # 实时多节点日志聚合
│
└── docs/
    ├── fishbonechain.md    # 论文业务逻辑说明
    ├── implementation-plan.md  # 整体实现规划（Phase 0-4）
    ├── developer-guide.md  # 本文档
    └── plan/               # 各 Phase 详细执行计划
```

---

## 3. 架构设计

### 3.1 三链拓扑

```
主链（fishbone_main）
  validator：f1 f2 f3 f4 f5 f6（6 个节点）
  pallet：CCMC + FMC + Balances + Sudo + ...
  P2P：30333    RPC：9944

子链1（fishbone_child_1）
  validator：f1 f2 f3（3 个节点）
  pallet：与主链相同 runtime，通过 chain spec 区分链身份
  P2P：30334    RPC：9945

子链2（fishbone_child_2）
  validator：f4 f5 f6（3 个节点）
  P2P：30335    RPC：9946
```

> 当前主链和子链使用同一个 runtime 二进制（`fishbone-node`），通过不同的 chain spec 和 `--chain` 参数区分。子链独立 runtime 将在 Phase 4 迁移到 Relay+Parachain 时拆分。

### 3.2 共识机制

- **出块**：AURA（轮流出块，6 秒/块）
- **终局性**：GRANDPA（BFT 风格，需要 2/3 validator 签名才 finalize）
- 主链：6 个 validator，需 4 个在线才能 finalize
- 子链：3 个 validator，需 2 个在线才能 finalize

### 3.3 关键类型

```rust
// pallets/ccmc/src/types.rs
pub type ChainId = u32;   // 子链 ID，全局递增
pub type EpochId = u64;   // Epoch 编号

// pallets/fmc/src/types.rs
pub type TaskId  = u32;   // 任务 ID，按请求者账户递增
```

---

## 4. Substrate Runtime

### 4.1 Pallet 注册表

`runtime/src/lib.rs` 使用 `#[runtime::pallet_index]` 宏注册：

| Index | 别名 | Pallet |
|-------|------|--------|
| 0 | System | frame_system |
| 1 | Timestamp | pallet_timestamp |
| 2 | Aura | pallet_aura |
| 3 | Grandpa | pallet_grandpa |
| 4 | Balances | pallet_balances |
| 5 | TransactionPayment | pallet_transaction_payment |
| 6 | Sudo | pallet_sudo |
| 7 | Template | pallet_fishbone_template |
| **8** | **Ccmc** | **pallet_ccmc** |
| **9** | **Fmc** | **pallet_fmc** |

### 4.2 Config 实现

`runtime/src/configs/mod.rs` 为两个业务 pallet 实现 Config：

```rust
impl pallet_ccmc::Config for Runtime {
    type RuntimeEvent = RuntimeEvent;
    type Currency     = Balances;   // pallet_balances 作为押金管理货币
    type WeightInfo   = ();         // 占位，后续需 benchmark
}

impl pallet_fmc::Config for Runtime {
    type RuntimeEvent = RuntimeEvent;
    type Currency     = Balances;
    type CcmcPallet   = pallet_ccmc::Pallet<Runtime>;  // 跨 pallet 接口绑定
    type WeightInfo   = ();
}
```

### 4.3 Chain Spec

`node/src/chain_spec.rs` 提供四个链配置函数：

| 函数 | 用途 |
|------|------|
| `main_dev()` | 单节点开发链（`--dev`，Alice 出块）|
| `main_local()` | 本地多节点主链（Alice+Bob，LOCAL_TESTNET）|
| `child1_local()` | 本地子链1 |
| `child2_local()` | 本地子链2 |

生产部署使用 `fishbone-node build-spec` 生成带真实 validator 密钥的 raw spec。

---

## 5. pallet-ccmc：子链管理合约

### 5.1 职责

- 子链注册与状态管理
- 矿工加入 / 退出 / 押金管理
- Epoch 摘要提交（阈值投票，≥ 2/3 矿工）
- 恶意矿工驱逐（slash 投票，≥ 2/3 矿工）
- 对 pallet-fmc 暴露矿工查询接口

### 5.2 存储

```rust
// 子链基本信息
ChildChains:  StorageMap<ChainId, ChainInfo<AccountId, Balance>>

// 矿工信息（含押金）
Miners:       StorageDoubleMap<ChainId, AccountId, MinerInfo<Balance>>

// 已确认的 Epoch Merkle Root（达阈值后写入）
EpochDigests: StorageDoubleMap<ChainId, EpochId, Hash>

// Epoch 摘要投票（矿工对候选 Root 的投票集合）
DigestVotes:  StorageNMap<(ChainId, EpochId, Hash), BoundedBTreeSet<AccountId>>

// 恶意矿工驱逐投票
SlashVotes:   StorageDoubleMap<ChainId, AccountId, BoundedBTreeSet<AccountId>>

// 子链 ID 计数器
NextChainId:  StorageValue<ChainId>
```

### 5.3 关键类型

```rust
pub struct ChainInfo<AccountId, Balance> {
    pub creator:          AccountId,
    pub name:             BoundedVec<u8, ConstU32<64>>,
    pub miner_count:      u32,
    pub min_miners:       u32,           // 子链最少矿工数
    pub deposit_required: Balance,       // 每个矿工须缴的押金
    pub status:           ChainStatus,   // Active | Terminated
}

pub struct MinerInfo<Balance> {
    pub deposit: Balance,
    pub status:  MinerStatus,   // Active | Slashed
}
```

### 5.4 Extrinsic 接口

```rust
// 注册新子链（调用者自动成为第一个矿工）
register_child_chain(
    name: BoundedVec<u8, ConstU32<64>>,
    min_miners: u32,
    deposit_required: Balance,
)

// 矿工加入已有子链（缴纳押金）
join_child_chain(chain_id: ChainId)

// 矿工退出子链（押金 unreserve）
leave_child_chain(chain_id: ChainId)

// 提交 Epoch 摘要投票
// 达到 ceil(miner_count × 2/3) 票时自动确认 Root
submit_epoch_digest(chain_id: ChainId, epoch: EpochId, root: Hash)

// 投票驱逐恶意矿工（达阈值后 slash 押金）
vote_slash_miner(chain_id: ChainId, target: AccountId)

// 终止子链（creator 或 sudo）
terminate_child_chain(chain_id: ChainId)
```

### 5.5 公开查询接口（供其他 pallet 调用）

```rust
impl<T: Config> Pallet<T> {
    // 供 pallet-fmc 验证提交账单的矿工身份
    pub fn is_miner(chain_id: ChainId, who: &T::AccountId) -> bool

    // 供阈值计算
    pub fn miner_count(chain_id: ChainId) -> u32

    // 查询已确认的 Epoch Root（None 表示未确认）
    pub fn epoch_root_confirmed(chain_id: ChainId, epoch: EpochId) -> Option<T::Hash>
}
```

### 5.6 阈值多签设计

Epoch 摘要确认不使用 BLS 聚合签名（Substrate 原生不支持），而是**独立提交 + 链上计票**：

```
矿工1 调用 submit_epoch_digest(chain_id=0, epoch=3, root=0xabc...)
矿工2 调用 submit_epoch_digest(chain_id=0, epoch=3, root=0xabc...)
矿工3 调用 submit_epoch_digest(chain_id=0, epoch=3, root=0xabc...)
          ↓ 第3票达到 ceil(3×2/3)=2 票阈值
DigestVotes 记录确认，写入 EpochDigests，清理投票记录
```

### 5.7 单元测试（9 个，全部通过）

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-ccmc
```

| 测试用例 | 验证内容 |
|---------|---------|
| register_child_chain_works | 注册子链，押金 reserve，事件发出 |
| chain_id_increments | 多次注册 ID 递增 |
| join_and_leave_child_chain | 矿工加入/退出，押金正确流转 |
| join_twice_fails | 重复加入返回 AlreadyAMiner |
| non_miner_cannot_leave | 非矿工退出返回 NotAMiner |
| single_miner_confirms_epoch_immediately | 单矿工时单票即确认 |
| two_of_three_miners_confirm_epoch | 3 矿工时 2 票触发确认 |
| duplicate_vote_rejected | 重复投票返回 AlreadyVoted |
| non_miner_cannot_submit_digest | 非矿工提交返回 NotAMiner |
| creator_can_terminate_chain | Creator 可终止子链 |
| terminated_chain_rejects_new_members | 终止子链拒绝新矿工加入 |
| is_miner_and_epoch_root_query | 公开查询接口正确性 |

---

## 6. pallet-fmc：资金管理合约

### 6.1 职责

- 管理请求者的 **FB（自由余额）** 和 **LB（锁定余额）** 双余额资金池
- 任务生命周期：创建 → 激活 → 运行 → 结算 → 自动续期或终止
- 账单验证与奖励分发（阈值投票，2/3 子链矿工）
- 双花防护：只有 FB ≥ budget 时才能激活任务

### 6.2 存储

```rust
// 每个请求者的资金池（FB + LB）
FundPools:  StorageMap<AccountId, FundPool<Balance>>

// 任务信息（按请求者 + 任务ID 双键索引）
Tasks:      StorageDoubleMap<AccountId, TaskId, TaskInfo<Balance>>

// 任务 ID 计数器（按请求者独立递增）
NextTaskId: StorageMap<AccountId, TaskId>

// 账单投票（矿工对账单哈希的投票集合）
BillVotes:  StorageNMap<(AccountId, TaskId, EpochId, Hash), BoundedBTreeSet<AccountId>>
```

### 6.3 关键类型

```rust
pub struct FundPool<Balance> {
    pub free:   Balance,   // FB：可自由支配
    pub locked: Balance,   // LB：已为激活任务锁定（所有激活任务预算之和）
}

pub struct TaskInfo<Balance> {
    pub target_chain:      ChainId,      // 目标子链 ID（CCMC 中注册的）
    pub budget_per_epoch:  Balance,      // 每个 Epoch 的预算 B
    pub status:            TaskStatus,   // Terminated | Activated | Waiting
    pub current_epoch:     EpochId,      // 当前 Epoch 计数
    pub description:       BoundedVec<u8, ConstU32<256>>,
}
```

### 6.4 Extrinsic 接口

```rust
// 向 FMC 充值（资金进入 FB）
deposit(amount: Balance)

// 从 FMC 提款（只能提取 FB；LB 不可提取）
withdraw(amount: Balance)

// 创建任务（初始状态 Terminated）
create_task(
    target_chain: ChainId,
    budget_per_epoch: Balance,
    description: BoundedVec<u8, ConstU32<256>>,
)

// 激活任务（检查 FB ≥ B，将 B 从 FB 转入 LB）
activate_task(task_id: TaskId)

// 手动终止任务（将该任务的 LB 归还 FB）
terminate_task(task_id: TaskId)

// 提交 Epoch 账单（矿工投票，达 2/3 阈值后自动结算）
submit_bill(
    requester: AccountId,
    task_id: TaskId,
    epoch: EpochId,
    bill_amounts: BoundedVec<(AccountId, Balance), ConstU32<1024>>,
)
```

### 6.5 账单结算逻辑（settle_bill）

```
1. 将 bill_amounts 中各接收方的奖励从 pallet 账户转出（AllowDeath）
2. remainder = budget - total_paid
3. can_renew = (当前 FB + remainder) ≥ budget
4. 更新 FundPool：
   - locked -= budget
   - free   += remainder
   - 若 can_renew：free -= budget，locked += budget（自动续期）
5. 更新任务状态：
   - can_renew → Activated（任务继续下一 Epoch）
   - 否则     → Terminated（FB 不足，等待充值后重激活）
6. current_epoch += 1
```

### 6.6 PalletId 账户

FMC 使用 `PalletId(*b"fishb/fm")` 派生一个专属账户持有用户充入的资金。转账到矿工时使用 `AllowDeath`（允许 pallet 账户余额降至 0）。

### 6.7 单元测试（13 个，全部通过）

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-fmc
```

| 测试用例 | 验证内容 |
|---------|---------|
| deposit_and_withdraw_works | 充值 / 提款，FundPool 正确更新 |
| withdraw_more_than_free_fails | 超额提款返回 InsufficientFreeBalance |
| withdraw_without_deposit_fails | 无资金池时返回 NoFundPool |
| create_and_activate_task | 激活后 FB 减少 / LB 增加 |
| activate_fails_when_insufficient_free_balance | FB < B 时返回 InsufficientFreeBalance |
| double_spend_protection | 同时激活两个任务但 FB 不够时第二个失败 |
| terminate_task_returns_locked_to_free | 终止任务后 LB 归还 FB |
| submit_bill_settles_and_pays_recipients | 账单结算，接收方余额增加，任务自动续期 |
| bill_exceeding_budget_fails | 账单超预算返回 ExceedsBudget |
| task_terminates_when_insufficient_fb_after_settlement | 结算后 FB 不足时任务自动终止 |
| bill_on_terminated_task_fails | 对未激活任务提交账单返回 TaskNotActive |

---

## 7. 跨 Pallet 交互

### 7.1 问题：如何让 pallet-fmc 查询 pallet-ccmc 的矿工信息？

FMC 在处理 `submit_bill` 时需要验证调用者是否为目标子链的注册矿工。但直接依赖 `pallet_ccmc::Pallet<T>` 会导致循环依赖和类型歧义。

### 7.2 解决方案：Trait 抽象 + Blanket Impl

**Step 1**：FMC 定义一个接口 trait（`pallet_fmc/src/lib.rs`）：

```rust
pub trait CcmcInterface<AccountId> {
    fn is_miner(chain_id: ChainId, who: &AccountId) -> bool;
    fn miner_count(chain_id: ChainId) -> u32;
}
```

**Step 2**：FMC 的 Config 通过 trait bound 而非具体类型引用 CCMC：

```rust
#[pallet::config]
pub trait Config: frame_system::Config {
    type CcmcPallet: CcmcInterface<Self::AccountId>;
    // ...
}
```

**Step 3**：在 `pallet_fmc/src/lib.rs` pallet 模块外，提供 blanket impl：

```rust
// 孤儿规则允许：CcmcInterface 是本地 trait，pallet_ccmc::Pallet<T> 是外部类型
impl<T> pallet::CcmcInterface<T::AccountId> for pallet_ccmc::Pallet<T>
where
    T: frame_system::Config + pallet_ccmc::Config,
{
    fn is_miner(chain_id: ChainId, who: &T::AccountId) -> bool {
        pallet_ccmc::Pallet::<T>::is_miner(chain_id, who)
    }
    fn miner_count(chain_id: ChainId) -> u32 {
        pallet_ccmc::Pallet::<T>::miner_count(chain_id)
    }
}
```

**Step 4**：Runtime 直接绑定，零样板代码：

```rust
impl pallet_fmc::Config for Runtime {
    type CcmcPallet = pallet_ccmc::Pallet<Runtime>;
    // ...
}
```

### 7.3 测试中的 Mock

`pallet_fmc/src/mock.rs` 用一个结构体模拟 CCMC（不需要实际运行 pallet_ccmc）：

```rust
pub struct MockCcmc;
impl CcmcInterface<u64> for MockCcmc {
    fn is_miner(_chain_id: u32, _who: &u64) -> bool { true }  // 所有账户都是矿工
    fn miner_count(_chain_id: u32) -> u32 { 1 }                // 1 个矿工，单票即达阈值
}

impl crate::Config for Test {
    type CcmcPallet = MockCcmc;
    // ...
}
```

---

## 8. 链配置与网络拓扑

### 8.1 本地开发（单机）

```bash
# Dev 模式（单节点，Alice 自动出块）
./target/release/fishbone-node --dev --rpc-port 9944 --rpc-cors all

# 本地三链（需要两个节点同时运行）
bash scripts/start-network.sh
```

Dev 模式等价于 `--chain=dev --alice --validator --force-authoring --tmp`。

> **注意**：`--chain main-local` 使用 LOCAL_TESTNET_PRESET（Alice+Bob 两个 validator），单节点下 GRANDPA 无法 finalize，区块不增长。开发测试用 `--dev`。

### 8.2 生产多节点（6 台机器）

Chain spec 生成流程：

```bash
# 1. 生成 validator 密钥（每台机器独立执行）
fishbone-node key generate --scheme sr25519  # AURA 密钥
fishbone-node key generate --scheme ed25519  # GRANDPA 密钥

# 2. 编辑 chain spec，写入各节点的 SS58 地址
fishbone-node build-spec --chain main-local > editable.json
# ... 编辑 patch.aura.authorities 和 patch.grandpa.authorities ...
fishbone-node build-spec --chain editable.json --raw > main-custom-raw.json

# 3. 注入密钥到 keystore
fishbone-node key insert --base-path /data/main --chain main-custom-raw.json \
    --scheme sr25519 --key-type aura --suri "<助记词>"

# 4. 启动（systemd service 管理，见 deploy/fishbone/service.py）
```

### 8.3 测试网络当前状态

| 链 | Validators | RPC 地址 | 当前高度 |
|----|-----------|---------|---------|
| fishbone_main | f1-f6（6 节点）| ws://10.2.2.11:9944 | ~2800+ |
| fishbone_child_1 | f1-f3（3 节点）| ws://10.2.2.11:9945 | ~2700+ |
| fishbone_child_2 | f4-f6（3 节点）| ws://10.2.2.14:9946 | ~2700+ |

所有节点通过 ProxyJump via `bcg`（192.168.8.41）SSH 访问。

---

## 9. 多节点部署框架

### 9.1 架构

Python 管理框架位于 `deploy/`，全部命令通过系统 `ssh` 命令（自动读取 `~/.ssh/config`）操作远端机器。

```
config.toml          ← 所有节点/链参数的单一真相来源
    │
    ├── fishbone/config.py   ← 解析 TOML → 类型化 Python 对象
    ├── fishbone/remote.py   ← asyncio subprocess ssh 封装
    └── fishbone/service.py  ← 生成 systemd service 文件内容
```

### 9.2 配置文件结构（config.toml）

```toml
[cluster]
name      = "fishbone-testnet"
binary    = "/home/debian/fishbone/bin/fishbone-node"
base_dir  = "/home/debian/fishbone"
sudo_pass = "debian"

[gateway]
ssh = "bcg"           # SSH Host alias（跳板机）
ip  = "192.168.8.41"  # 跳板机 IP（用于 RPC 中转查询）

[chains.main]
id       = "fishbone_main"
spec     = "specs/main-custom-raw.json"
p2p_port = 30333
rpc_port = 9944

[[nodes]]
id    = "f1"
ip    = "10.2.2.11"
ssh   = "f1"          # 对应 ~/.ssh/config 中的 Host f1
roles = ["main", "child1"]

[nodes.peer_ids]
main   = "12D3KooWEG8gvUe7RRvrgknZwbzg1snqKQzxhewX5FDaEYGfcDLa"
child1 = "12D3KooWNy8n5FWgmUA991q8pDQdDPYVJqp996sBwCeF7QFZMkLy"
```

> 添加新节点：只需在 `config.toml` 加一个 `[[nodes]]` 条目，所有脚本自动适配。

### 9.3 管理命令

```bash
cd deploy
export PATH="$HOME/.local/bin:$PATH"  # uv 路径

# 查看所有节点状态（区块高度、peer 数、服务状态）
uv run python3 cmd/status.py

# 部署（推二进制 + chain spec + 注入密钥 + 安装 service）
uv run python3 cmd/deploy.py
uv run python3 cmd/deploy.py --only f1,f2   # 只部署指定节点

# 控制服务
uv run python3 cmd/control.py start   --nodes f1 --chains main
uv run python3 cmd/control.py stop    --nodes f4,f5,f6 --chains child2
uv run python3 cmd/control.py restart                   # 全部重启

# 实时日志聚合（多节点日志合并显示，按颜色区分节点）
uv run python3 cmd/logs.py main        # 主链日志
uv run python3 cmd/logs.py child1      # 子链1日志
uv run python3 cmd/logs.py main --nodes f1,f3  # 指定节点
```

### 9.4 Remote 封装原理

`deploy/fishbone/remote.py` 中的 `RemoteNode` 类：

```python
class RemoteNode:
    async def run(self, cmd: str, input: str = None) -> RunResult:
        # 等价于：ssh -o StrictHostKeyChecking=no <host> <cmd>
        # 自动走 ProxyJump，自动用 ~/.ssh/config 中的 IdentityFile
        proc = await asyncio.create_subprocess_exec(
            "ssh", "-o", "StrictHostKeyChecking=no",
            self.ssh_host, cmd, ...
        )

    async def sudo(self, cmd: str) -> RunResult:
        # 通过 echo <pass> | sudo -S 执行
        return await self.run(f"echo {self.sudo_pass!r} | sudo -S {cmd}")
```

---

## 10. 开发流程

### 10.1 环境要求

```bash
rustup show  # 确认 stable toolchain（1.96.0）
# 必须有 wasm32-unknown-unknown target（自动从 rust-toolchain.toml 安装）
```

### 10.2 构建

```bash
# 单元测试（跳过 WASM，极快）
SKIP_WASM_BUILD=1 cargo test -p pallet-ccmc
SKIP_WASM_BUILD=1 cargo test -p pallet-fmc

# 只编译 runtime（验证 pallet 集成）
SKIP_WASM_BUILD=1 cargo build -p fishbone-runtime

# 完整 release 构建（含 WASM，~60s）
WASM_BUILD_RUSTFLAGS="-C link-arg=--allow-undefined" cargo build --release

# 注意：Rust 1.82+ + polkadot-sdk 2512 必须加上述 WASM_BUILD_RUSTFLAGS
# 原因：sp-io WASM builder 对新版链接器行为不兼容（已知 bug）
```

### 10.3 本地运行 E2E 验证

```bash
# 启动 dev 节点
./target/release/fishbone-node --dev --rpc-port 9944 --rpc-cors all &

# 安装 Node.js 依赖
npm install

# 运行 E2E 验证脚本（完整 7 步业务流程）
node --input-type=module < scripts/e2e-verify.js
```

E2E 脚本验证的完整流程：
1. Alice → `ccmc.registerChildChain` → 子链0 注册
2. Bob → `ccmc.joinChildChain` → 加入，miner_count=2
3. Alice → `fmc.deposit(10 UNIT)` → FundPool.free = 10T
4. Alice → `fmc.createTask(budget=2 UNIT)` → Task0 status=Terminated
5. Alice → `fmc.activateTask(0)` → free=8T, locked=2T, status=Activated
6. Bob + Alice → `ccmc.submitEpochDigest` × 2 → 达阈值，Epoch0 Root 确认
7. Bob → `fmc.submitBill(1 UNIT)` → 结算，Bob 余额增加，任务自动续期

### 10.4 添加新 Pallet 的步骤

1. 在 `pallets/` 下新建目录，创建 `Cargo.toml` + `src/lib.rs`
2. 在 `Cargo.toml`（workspace 根）的 `[workspace.members]` 和 `[workspace.dependencies]` 中添加
3. 在 `runtime/Cargo.toml` 的 `[dependencies]` 和 `[features]` 中添加
4. 在 `runtime/src/lib.rs` 中 `impl pallet_xxx::Config for Runtime { ... }`（在 configs/mod.rs）
5. 在 `runtime/src/lib.rs` 中注册 `#[runtime::pallet_index(N)] pub type Xxx = pallet_xxx;`

---

## 11. 当前范围与后续规划

### 已实现（Phase 0 + Phase 1）

| 组件 | 状态 | 说明 |
|------|------|------|
| Substrate 节点（AURA+GRANDPA）| ✅ | 基于 polkadot-sdk-solochain-template |
| pallet-ccmc | ✅ | 子链管理，阈值多签（替代 BLS） |
| pallet-fmc | ✅ | 资金管理，双余额，账单结算 |
| 单元测试（22/22）| ✅ | CCMC 9 个 + FMC 13 个 |
| E2E 验证脚本 | ✅ | 7 步完整业务流程 |
| 多节点真实部署 | ✅ | 6 台机器，3 条独立区块链 |
| Python 部署管理框架 | ✅ | config-driven，支持扩展新节点 |

### 暂未实现（明确排除在当前范围外）

| 组件 | 说明 |
|------|------|
| pallet-crowdsource | Phase 2：子链众包数据收集和 Epoch 结算 |
| XCM 跨链消息 | Phase 2 用链下桥接脚本替代；Relay+Parachain 迁移时引入 |
| pallet-groth16-verifier | Phase 3：BN254 Groth16 ZK 证明链上验证 |
| pallet-verify-contract | Phase 3：CDT 数据交易 5 步协议 |
| pallet-hash-verifier | Phase 3：链上 keccak256 哈希链验证 |
| pallet-bpiano-verifier | Phase 4：BPiano 跨链状态证明 |
| BLS 聚合签名 | 当前用阈值投票替代，Phase 4 后可升级 |
| Weight Benchmark | 当前所有 extrinsic 使用占位 weight |
| 独立子链 Runtime | 当前主子链共用同一 runtime 二进制 |
