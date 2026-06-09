# FishboneChain 项目实现记录

**最后更新**：2026-06-04  
**状态**：6 条子链全部运行，包括真实 BABE 共识

---

## 目录

1. [项目架构总览](#1-项目架构总览)
2. [实现阶段记录](#2-实现阶段记录)
   - [Phase 0：本地开发环境](#phase-0本地开发环境)
   - [Phase 1：主链核心 Pallet](#phase-1主链核心-pallet)
   - [Phase 2：子链众包协议](#phase-2子链众包协议)
   - [Phase 3：多节点部署](#phase-3多节点部署)
   - [Phase 4：实验准备与 6 链部署](#phase-4实验准备与-6-链部署)
   - [Phase 5：BABE 共识修复](#phase-5babe-共识修复)
3. [已实现接口文档](#3-已实现接口文档)
   - [pallet-fmc](#pallet-fmc资金管理合约)
   - [pallet-ccmc](#pallet-ccmc子链管理合约)
   - [pallet-crowdsource](#pallet-crowdsource子链众包协议)
   - [实验脚本](#实验脚本接口)
4. [部署基础设施](#4-部署基础设施)
5. [实验设计（6 子链）](#5-实验设计6-子链)
6. [经验教训汇总](#6-经验教训汇总)

---

## 1 项目架构总览

FishboneChain 是一个面向去中心化数据众包的多链系统，用于论文实验验证"多链并发吞吐量近似线性扩展"这一核心主张。

```
主链（fishbone_main，12 验证人 f1-f12）
├── pallet-ccmc   子链注册 / 矿工管理 / Epoch 摘要确认
└── pallet-fmc    任务资金托管 / 账单结算

子链（独立出块，与主链无原生消息传递）
├── child1  城市快递配送确认  AURA-3  6s    f1 f2 f3
├── child2  实时交通感知      AURA-3  2s    f4 f5 f6
├── child3  医疗影像标注      AURA-3  6s    f7 f8 f9   [10MB 区块]
├── child4  金融凭证核验      AURA-7  6s    f1-f7
├── child5  IoT 传感器网络    AURA-3  1s    f10 f11 f12
└── child6  去中心化数据市场  BABE-5  6s    f1-f5

跨链中继（链下，每台矿工节点运行）
└── bridge.js  监听子链 EpochFinalized → 提交主链摘要 + 触发账单结算
```

**硬件**：12 台 VM（f1-f12，IP 10.2.2.11-22），通过 bcg（192.168.8.41）跳板访问。

**代码结构**：
```
fishbonechain/
├── runtime/         # Substrate runtime（WASM + native）
│   └── src/
│       ├── runtime_aura.rs  # AURA 共识链的 construct_runtime!
│       └── runtime_babe.rs  # BABE 共识链的 construct_runtime!
├── node/src/
│   ├── service.rs           # AURA 节点服务
│   └── service_babe.rs      # BABE 节点服务
├── pallets/
│   ├── fmc/         # 资金管理合约
│   ├── ccmc/        # 子链管理合约
│   └── crowdsource/ # 子链众包协议
├── scripts/
│   ├── worker.js    # 工作者负载模拟
│   ├── metrics.js   # 指标采集 CSV
│   ├── bridge.js    # 跨链中继
│   └── gen_child_specs.py  # 子链 chain spec 生成
└── deploy/
    ├── config.toml          # 集群单一真相来源
    ├── bin/                 # 编译产物（.gitignore）
    ├── specs/               # chain spec JSON
    ├── keys/                # 节点密钥（f1-f12.env）
    └── fishbone/            # Python 部署框架
```

---

## 2 实现阶段记录

### Phase 0：本地开发环境

**时间**：2026-05-31  
**状态**：✅ 完成

**核心决策**：
- 选用 Rust stable 1.96 + polkadot-sdk-solochain-template
- 三链环境（main + child1 + child2）用独立 `--base-path` 和不同端口在本机运行
- `--chain main-local`、`--chain child1-local`、`--chain child2-local`

**关键产出**：
- `fishbone-node` 可编译运行
- 三个 chain spec（main / child1 / child2）各自独立出块
- `cargo test` 通过

---

### Phase 1：主链核心 Pallet

**时间**：2026-05-31  
**状态**：✅ 完成

**实现内容**：
- `pallet-fmc`：资金管理合约（deposit / withdraw / create_task / activate_task / terminate_task / submit_bill）
- `pallet-ccmc`：子链管理合约（register / join / leave / submit_epoch_digest / vote_slash / terminate）

**关键技术问题**：
- `AllowDeath` vs `KeepAlive`：`pallet_balances::Mutate` 中转移时必须指定 preservation 策略
- 跨 pallet blanket impl 冲突：`pallet_ccmc::Config` 要求 `pallet_fmc::Config`，由 Runtime 统一 `impl Config`，不能在 pallet 内实现
- 事件注册：所有 `Event` variant 需要在 `#[pallet::event]` 里完整声明，缺一会导致 WASM 编译失败

---

### Phase 2：子链众包协议

**时间**：2026-06-01  
**状态**：✅ 完成

**实现内容**：
- `pallet-crowdsource`：子链众包核心协议
  - `sync_task`：从主链同步任务参数
  - `submit_data`：工作者提交数据
  - `finalize_epoch`：Epoch 结束，计算 Merkle root，生成 EpochFinalized 事件（含 per-task 账单）

**关键设计**：
- Epoch 使用 `CollectingSlot（100 blocks）+ SyncingSlot（20 blocks）` 两阶段
- `EpochFinalized` 事件携带 `task_bills: Vec<TaskBill>`，每个 TaskBill 包含 `task_id`、`requester`、`amounts: Vec<(worker, reward)>`
- bridge.js 监听 EpochFinalized，逐 task 调用主链 `fmc.submit_bill`

**测试**：16 个单元测试全部通过（`cargo test -p pallet-crowdsource`）

---

### Phase 3：多节点部署

**时间**：2026-06-01  
**状态**：✅ 完成

**实现内容**：
- Python 部署框架（`deploy/fishbone/`）：config.toml 驱动，统一管理 12 台 VM
- 主链扩展至 12 验证人（f1-f12）
- child1、child2 已部署并运行

**关键流程**：
1. 在开发机生成密钥对（`deploy/keys/f{n}.env`）
2. `generate-node-keys.py` 在各 VM 生成 P2P node-key，读出 Peer ID
3. 将 Peer ID 填入 `deploy/config.toml` `[nodes.peer_ids]`
4. `gen_child_specs.py` 生成含真实 validator 公钥的 raw chain spec
5. `deploy.py` 推送 binary + spec，注入 keystore，安装 systemd service，启动

**重要约束**：
- 子链矿工 ⊆ 主链矿工（论文核心约束）
- 生产环境必须用真实 node-key 生成的 Peer ID，不能用 Alice/Bob 占位

---

### Phase 4：实验准备与 6 链部署

**时间**：2026-06-03  
**状态**：✅ 完成

**新增 binary 变体**（通过 Cargo feature 编译时切换）：
- `fishbone-node-2s`：`--features fishbone-runtime/block-2s`，AURA 2s 出块
- `fishbone-node-1s`：`--features fishbone-runtime/block-1s`，AURA 1s 出块
- `fishbone-node-10mb`：`--features fishbone-runtime/block-10mb`，10MB 区块上限
- `fishbone-node-babe`：`--features fishbone-runtime/babe`，BABE 共识（见 Phase 5）

**Makefile targets**：
```bash
make build-release    # fishbone-node（默认，6s AURA）
make build-2s         # fishbone-node-2s
make build-1s         # fishbone-node-1s
make build-10mb       # fishbone-node-10mb
make build-babe       # fishbone-node-babe（BABE）
```

**实验脚本**：
- `scripts/worker.js`：工作者负载模拟，支持 a-f 六个场景预设
- `scripts/metrics.js`：多链指标采集，输出 CSV
- `scripts/bridge.js`：跨链中继（子链 EpochFinalized → 主链摘要 + 账单）

**6 条子链全部运行**：child1-child6，总共 12 台 VM，各链独立出块。

---

### Phase 5：BABE 共识修复

**时间**：2026-06-04  
**状态**：✅ 完成

**问题根因**：
- `sc-consensus-epochs` 对 genesis 父块硬编码 `UnimportedGenesis(real_slot)`
- 导致 block #1 被判为 `first_in_epoch = true`
- 需要区块头携带 `NextEpochData` digest，而这只有 `pallet_babe::on_finalize` 才能生成
- stub BabeApi（之前的方案）无法产生此 digest

**解决方案**：在 runtime 中加入真正的 `pallet_babe`，用 `--features babe` 编译独立的 BABE binary。

**关键技术挑战与解法**：

| 挑战 | 解法 |
|------|------|
| `#[frame_support::runtime]` 宏剥离 `#[cfg]`，无法条件包含 pallet | 用 `include!("runtime_aura.rs")` / `include!("runtime_babe.rs")` 分文件，在 lib.rs 外层 `#[cfg]` 选择 |
| `impl_runtime_apis!` 不允许同一 API 两个 impl | 在 macro 外定义 feature-gated helper 函数，macro 内只有一个无条件的 `impl BabeApi` 调用 helpers |
| pallet-babe → pallet-session → WASM undefined host functions | `WASM_BUILD_RUSTFLAGS="-C link-arg=--allow-undefined"`（已在 Makefile `WASM_FLAGS` 中） |
| AURA binary 编译需要 BabeApi（service_babe.rs 始终编译） | AuraApi 始终实现；非 babe build 也提供 BabeApi stub（通过 helper 函数） |

**child6 最终状态**：5 个验证人（f1-f5），BABE 共识，6s 出块，GRANDPA 最终确定，peers=4。

---

## 3 已实现接口文档

### pallet-fmc（资金管理合约）

**Extrinsics（可调用函数）**：

| 函数 | 参数 | 说明 |
|------|------|------|
| `deposit(amount)` | `amount: Balance` | 向矿工资金池充值 |
| `withdraw(amount)` | `amount: Balance` | 从矿工资金池提款（仅提 free 余额）|
| `create_task(task_id, budget_per_epoch, max_workers, reward_per_worker)` | 见右 | 任务请求方创建任务，状态 Pending |
| `activate_task(task_id)` | `task_id: TaskId` | 激活任务，从 free 余额锁定 `budget_per_epoch` |
| `terminate_task(task_id)` | `task_id: TaskId` | 终止任务，释放锁定余额 |
| `submit_bill(requester, task_id, epoch, amounts)` | 见右 | 矿工提交账单；2/3 多数确认后自动结算，工作者收款 |

**Events**：

| 事件 | 字段 | 触发时机 |
|------|------|---------|
| `Deposited` | `{ who, amount }` | deposit 成功 |
| `Withdrawn` | `{ who, amount }` | withdraw 成功 |
| `TaskCreated` | `{ requester, task_id }` | create_task 成功 |
| `TaskActivated` | `{ requester, task_id }` | activate_task 成功 |
| `TaskTerminated` | `{ requester, task_id }` | terminate_task 成功 |
| `BillSettled` | `{ requester, task_id, epoch }` | 账单 2/3 多数确认，结算完成 |

**Errors**：`NoFundPool`、`InsufficientFreeBalance`、`TaskNotFound`、`TaskNotActive`、`NotAMiner`、`ExceedsBudget`、`AlreadyVoted`、`Overflow`

**辅助函数（链上查询）**：
```rust
Pallet::<T>::account_id() -> T::AccountId  // FMC 账户 ID（托管资金）
```

---

### pallet-ccmc（子链管理合约）

**Extrinsics**：

| 函数 | 参数 | 说明 |
|------|------|------|
| `register_child_chain(chain_id, name)` | `chain_id: ChainId, name: Vec<u8>` | 注册子链，创建者自动成为第一个矿工 |
| `join_child_chain(chain_id)` | `chain_id: ChainId` | 加入子链成为矿工 |
| `leave_child_chain(chain_id)` | `chain_id: ChainId` | 退出子链（清算后可取回资金）|
| `submit_epoch_digest(chain_id, epoch, root)` | 见右 | 矿工提交子链 Epoch 的 Merkle root；2/3 多数后确认 |
| `vote_slash_miner(chain_id, target)` | 见右 | 投票惩罚恶意矿工（2/3 多数后执行 slash）|
| `terminate_child_chain(chain_id)` | `chain_id: ChainId` | 终止子链（仅创建者可调用）|

**Events**：

| 事件 | 字段 |
|------|------|
| `ChainRegistered` | `{ chain_id, creator }` |
| `MinerJoined` | `{ chain_id, miner }` |
| `MinerLeft` | `{ chain_id, miner }` |
| `EpochDigestConfirmed` | `{ chain_id, epoch, root }` |
| `MinerSlashed` | `{ chain_id, miner }` |
| `ChainTerminated` | `{ chain_id }` |

**Errors**：`ChainNotFound`、`ChainTerminated`、`AlreadyAMiner`、`NotAMiner`、`AlreadyVoted`、`ChainIdOverflow`

**辅助函数**：
```rust
is_miner(chain_id, who) -> bool
miner_count(chain_id) -> u32
epoch_root_confirmed(chain_id, epoch) -> Option<Hash>
```

---

### pallet-crowdsource（子链众包协议）

**Extrinsics**：

| 函数 | 参数 | 说明 |
|------|------|------|
| `sync_task(task_id, reward_per_worker)` | `task_id: TaskId, reward: Balance` | 主链矿工将任务同步到子链 |
| `submit_data(task_id, data)` | `task_id: TaskId, data: Vec<u8>` | 工作者提交数据（CollectingSlot 内有效）|
| `finalize_epoch()` | — | 任意账户可调；SyncingSlot 期间执行，计算 Merkle root，推进 Epoch |

**Events**：

| 事件 | 字段 | 说明 |
|------|------|------|
| `TaskSynced` | `{ task_id, reward_per_worker }` | sync_task 成功 |
| `DataSubmitted` | `{ task_id, worker, epoch_id }` | submit_data 成功 |
| `EpochFinalized` | `{ epoch_id, task_bills, merkle_root }` | Epoch 结束，含完整账单 |

**`EpochFinalized` 事件详解**：
```rust
EpochFinalized {
    epoch_id: u32,
    merkle_root: Hash,
    task_bills: Vec<TaskBill<AccountId, Balance>>,
}

struct TaskBill<AccountId, Balance> {
    task_id:   TaskId,
    requester: AccountId,       // 从 ActiveTasks 查出
    amounts:   Vec<(AccountId, Balance)>,  // (worker, reward)
}
```

**Errors**：`TaskNotFound`、`TaskNotActive`、`EpochNotCollecting`、`EpochNotSyncing`、`AlreadySubmitted`、`DataTooLarge`、`MaxSubmissionsExceeded`

**链上查询**：
```rust
Pallet::<T>::current_epoch() -> EpochInfo { epoch_id, phase, start_block }
```

**Config 参数**（可在 runtime 配置）：
- `CollectingSlotBlocks`：CollectingSlot 时长（默认 100 blocks ≈ 10 min at 6s）
- `SyncingSlotBlocks`：SyncingSlot 时长（默认 20 blocks）
- `MaxSubmissionsPerEpoch`：每 Epoch 最大提交数（默认 1000）

---

### 实验脚本接口

#### worker.js — 工作者负载模拟

```bash
node scripts/worker.js [选项]

选项：
  --ws <url>           子链 WebSocket RPC（默认 ws://127.0.0.1:9945）
  --task-id <n>        目标任务 ID（默认 0）
  --workers <n>        并发工作者数（默认 10）
  --rate <req/s>       每个工作者每秒请求数（默认 0.1）
  --reward <planck>    每次提交奖励（默认 5×10¹² = 5 UNIT）
  --data-size <bytes>  随机数据大小（默认 256）
  --duration <s>       运行时长（默认无限）
  --scenario <a-f>     使用场景预设（覆盖以上参数）
```

**场景预设**：
| 场景 | 子链 | workers | rate(req/s) | reward | data-size |
|------|------|---------|-------------|--------|-----------|
| a | child1（快递）| 300 | 0.02 | 5 UNIT | 512B |
| b | child2（交通）| 2000 | 0.1 | 0.001 UNIT | 128B |
| c | child3（医疗）| 200 | 0.008 | 200 UNIT | 900B |
| d | child4（金融）| 100 | 0.005 | 50 UNIT | 800B |
| e | child5（传感器）| 5000 | 0.2 | 0.0001 UNIT | 64B |
| f | child6（市场）| 500 | 0.02 | 50 UNIT | 1024B |

**输出**（实时控制台）：
```
[worker] 已成功: 142  失败: 3  拒绝: 0  TPS: 2.4  成功率: 97.9%
```

---

#### metrics.js — 指标采集

```bash
node scripts/metrics.js [选项]

选项：
  --chains <urls>   逗号分隔的子链 WS 地址（默认 ws://127.0.0.1:9945）
  --out <prefix>    CSV 输出路径前缀（默认 ./metrics）
  --interval <s>    轮询间隔秒数（默认 30）

示例：
  node scripts/metrics.js \
    --chains ws://10.2.2.11:9945,ws://10.2.2.14:9946 \
    --out /tmp/exp1 \
    --interval 15
```

**输出文件**：
- `<prefix>_state.csv`：定期轮询，字段 `timestamp, chain_url, epoch_id, phase, submissions_count, block_number, finalized`
- `<prefix>_epoch.csv`：事件驱动，字段 `timestamp, chain_url, epoch_id, valid_subs, merkle_root, duration_s`

---

#### bridge.js — 跨链中继

```bash
CHILD_WS=ws://127.0.0.1:9945 \
MAIN_WS=ws://127.0.0.1:9944 \
MINER_SURI=//Alice \
node scripts/bridge.js
```

**环境变量**：
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CHILD_WS` | `ws://127.0.0.1:9945` | 子链 RPC 地址 |
| `MAIN_WS` | `ws://127.0.0.1:9944` | 主链 RPC 地址 |
| `MINER_SURI` | `//Alice` | 矿工账户助记词或 SURI |

**工作流程**：
1. 监听子链 `crowdsource.EpochFinalized` 事件
2. 调用主链 `ccmc.submit_epoch_digest(chain_id, epoch, merkle_root)`
3. 对每个 `task_bill` 调用主链 `fmc.submit_bill(requester, task_id, epoch, amounts)`
4. 自动容错：`AlreadyVoted` 错误静默忽略（其他矿工已提交）

---

## 4 部署基础设施

### 集群配置（deploy/config.toml）

单一真相来源，记录所有链参数、节点角色、P2P Peer ID。

**链定义示例**：
```toml
[chains.child6]
id       = "fishbone_child_6_babe"
spec     = "specs/child6-custom-raw.json"
binary   = "/home/debian/fishbone/bin/fishbone-node-babe"
p2p_port = 30339
rpc_port = 9950
prom_port = 9621
```

**端口分配**：

| 链 | P2P | RPC | Prometheus |
|----|-----|-----|-----------|
| main | 30333 | 9944 | 9615 |
| child1 | 30334 | 9945 | 9616 |
| child2 | 30335 | 9946 | 9617 |
| child3 | 30336 | 9947 | 9618 |
| child4 | 30337 | 9948 | 9619 |
| child5 | 30338 | 9949 | 9620 |
| child6 | 30339 | 9950 | 9621 |

### Binary 与子链对应

| Binary | Cargo feature | 子链 |
|--------|--------------|------|
| `fishbone-node` | 默认 | main + child1 + child4 |
| `fishbone-node-2s` | `block-2s` | child2（2s 出块）|
| `fishbone-node-10mb` | `block-10mb` | child3（10MB 区块）|
| `fishbone-node-1s` | `block-1s` | child5（1s 出块）|
| `fishbone-node-babe` | `babe` | child6（BABE 共识）|

### Chain Spec 生成

```bash
# 生成 child3-child6 raw spec（含真实 validator 公钥）
python3 scripts/gen_child_specs.py

# 为 BABE 链单独生成（babe.authorities 格式不同）
# gen_child_specs.py 自动识别 key_type="babe" 并正确注入
```

### 密钥注入

```bash
# 子链 AURA 节点（child1-5）：注入 aura + gran
fishbone-node key insert --chain <spec> --base-path <dir> \
  --key-type aura --scheme Sr25519 --suri '<seed>'
fishbone-node key insert --chain <spec> --base-path <dir> \
  --key-type gran --scheme Ed25519 --suri '<seed>'

# BABE 节点（child6）：注入 babe + gran（sr25519 密钥相同，key-type 不同）
fishbone-node-babe key insert --chain <spec> --base-path <dir> \
  --key-type babe --scheme Sr25519 --suri '<seed>'
fishbone-node-babe key insert --chain <spec> --base-path <dir> \
  --key-type gran --scheme Ed25519 --suri '<seed>'
```

### 节点角色分配

| 节点 | 主链 | child1 | child2 | child3 | child4 | child5 | child6 |
|------|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| f1 | ✓ | ✓ | | | ✓ | | ✓ |
| f2 | ✓ | ✓ | | | ✓ | | ✓ |
| f3 | ✓ | ✓ | | | ✓ | | ✓ |
| f4 | ✓ | | ✓ | | ✓ | | ✓ |
| f5 | ✓ | | ✓ | | ✓ | | ✓ |
| f6 | ✓ | | ✓ | | ✓ | | |
| f7 | ✓ | | | ✓ | ✓ | | |
| f8 | ✓ | | | ✓ | | | |
| f9 | ✓ | | | ✓ | | | |
| f10 | ✓ | | | | | ✓ | |
| f11 | ✓ | | | | | ✓ | |
| f12 | ✓ | | | | | ✓ | |

---

## 5 实验设计（6 子链）

**实验目标**：验证"多链并发吞吐量近似线性扩展"，同时展示多共识算法（AURA/BABE）共存。

| 子链 | 业务场景 | 共识 | 出块 | 验证人 | Epoch 时长 | 区块上限 |
|------|---------|------|------|--------|-----------|---------|
| child1 | 城市快递配送确认 | AURA-3 | 6s | f1 f2 f3 | 100 blocks≈10min | 5MB |
| child2 | 实时交通感知 | AURA-3 | 2s | f4 f5 f6 | 150 blocks≈5min | 5MB |
| child3 | 医疗影像标注 | AURA-3 | 6s | f7 f8 f9 | 300 blocks≈30min | **10MB** |
| child4 | 金融凭证核验 | **AURA-7** | 6s | f1-f7 | 200 blocks≈20min | 5MB |
| child5 | IoT 传感器网络 | AURA-3 | 1s | f10 f11 f12 | 60 blocks≈60s | 5MB |
| child6 | 去中心化数据市场 | **BABE-5** | 6s | f1-f5 | 200 slots epoch | 5MB |

**实验运行流程**：
```bash
# 1. 启动各子链的工作者负载
node scripts/worker.js --scenario a --ws ws://10.2.2.11:9945  # child1
node scripts/worker.js --scenario b --ws ws://10.2.2.14:9946  # child2
# ... 同理启动 c d e f

# 2. 启动指标采集
node scripts/metrics.js \
  --chains ws://10.2.2.11:9945,ws://10.2.2.14:9946,ws://10.2.2.17:9947,ws://10.2.2.11:9948,ws://10.2.2.20:9949,ws://10.2.2.11:9950 \
  --out /tmp/exp_result \
  --interval 15

# 3. 启动跨链中继（每台矿工机各运行一个实例）
CHILD_WS=ws://localhost:9945 MAIN_WS=ws://10.2.2.11:9944 MINER_SURI='...' \
  node scripts/bridge.js
```

---

## 6 经验教训汇总

### Rust / Substrate 工具链

1. **WASM flag 是必须的**：`WASM_BUILD_RUSTFLAGS="-C link-arg=--allow-undefined"` 对所有含 pallet-session 间接依赖的 runtime 是必须的。已固化到 `Makefile` 的 `WASM_FLAGS`。

2. **包名冲突会导致编译错误**：workspace 中自定义 pallet 名称不能与 substrate 内置名冲突（如避免用 `pallet-template` 作为正式 pallet 名）。

3. **Cargo.lock 必须提交**：runtime 和 node 的依赖版本必须锁定，否则不同机器编译的 WASM 可能不一致导致 chain id 不匹配。

4. **Rust 版本要求精确**：polkadot-sdk 对 Rust 工具链版本非常敏感，使用 `rust-toolchain.toml` 固定到具体版本（stable 1.96）。

5. **`parameter_types!` 内不支持 `#[cfg]`**：同一类型的 `parameter_types!` 宏条目不能用 `#[cfg]` 分成两个版本，需提取为独立的 `const`：
   ```rust
   #[cfg(feature = "block-10mb")]
   const MAX_BLOCK_BYTES: u32 = 10 * 1024 * 1024;
   #[cfg(not(feature = "block-10mb"))]
   const MAX_BLOCK_BYTES: u32 = 5 * 1024 * 1024;
   // 再在 parameter_types! 中引用 MAX_BLOCK_BYTES
   ```

6. **`#[frame_support::runtime]` 宏剥离 `#[cfg]`**：无法在 `construct_runtime!` 中条件包含 pallet。解决方案：将 runtime 定义拆到独立文件，在 `lib.rs` 外层用 `include!()` 条件选择：
   ```rust
   #[cfg(not(feature = "babe"))]
   include!("runtime_aura.rs");
   #[cfg(feature = "babe")]
   include!("runtime_babe.rs");
   ```

7. **`impl_runtime_apis!` 不支持两个同名 API**：即使在不同 `#[cfg]` 下，`impl_runtime_apis!` 也会报"duplicate trait"。解决方案：在 macro 外部定义 feature-gated helper 函数，macro 内的 impl 无条件调用 helper：
   ```rust
   #[cfg(not(feature = "babe"))]
   fn babe_api_configuration() -> BabeConfiguration { /* stub */ }
   #[cfg(feature = "babe")]
   fn babe_api_configuration() -> BabeConfiguration { /* real */ }
   
   impl_runtime_apis! {
       impl BabeApi<Block> for Runtime {
           fn configuration() -> BabeConfiguration { babe_api_configuration() }
       }
   }
   ```

### Pallet 设计

8. **Balance 操作用 `AllowDeath` 还是 `KeepAlive`**：向合约账户（Pot）转账用 `Preservation::Expendable`，向用户账户转账用 `Preservation::Preserve`，防止账户被意外清除。

9. **跨 pallet 实现 trait 要通过 Runtime**：不能在一个 pallet 的 Config 里直接 `impl` 另一个 pallet 的 trait，必须在 runtime 的 `impl pallet_a::Config for Runtime` 中声明 `type OtherPallet = PalletB`。

10. **事件需要完整声明**：`#[pallet::event]` 中所有变体必须显式列出，缺少任何一个都会导致 WASM 编译错误，即使该事件在代码中未被触发。

### 多节点部署

11. **`--dev` vs `--chain main-local`**：`--dev` 在每次重启时重置状态（genesis = 每次不同）；多节点部署必须用 `--chain` 加载固定的 chain spec，否则节点间 genesis hash 不匹配无法组网。

12. **Node-key 决定 Peer ID**：部署前必须先生成 node-key 文件，读出 Peer ID，填入 bootnodes 参数。不能先启动再获取（peer ID 基于私钥，启动时就需要知道其他节点的 Peer ID）。

13. **SCP 替换运行中的 binary 会失败**：系统进程持有 binary 文件锁。正确方式：先 scp 到临时路径，再 `mv -f` 原子替换：
    ```bash
    scp binary node:/tmp/binary.new
    ssh node "mv -f /tmp/binary.new /path/to/binary"
    ```

14. **`--unsafe-rpc-external` 是必须的**：RPC 接口需要对外暴露才能从开发机调用。生产环境应配合防火墙限制访问来源。

### BABE 共识

15. **BABE 需要 `pallet_babe` 在 runtime 中**：`sc-consensus-babe` 的 epoch 树管理对 genesis 父块有特殊处理，block #1 时需要 `NextEpochData` digest，这只有 `pallet_babe::on_finalize` 能生成。没有 `pallet_babe` 就无法产生第一个 BABE 区块。

16. **pallet-babe 的 WASM 依赖链问题**：pallet-babe → pallet-session 使用了存储事务 host function，在 `wasm32v1-none` 目标下需要 `--allow-undefined` 链接器标志，否则 WASM 构建失败。

17. **BABE keystore 类型是 `babe`，不是 `aura`**：BABE 和 AURA 使用相同的 sr25519 密钥材料，但 key-type 不同（`b"babe"` vs `b"aura"`）。BABE 节点 keystore 中必须有 `babe` 类型的条目，否则 VRF 出块无法签名。

---

---

## 7 未实现的未来规划

### Phase 3：CDT 可定制可验证数据交易

**状态**：规划完成，未实现（不属于当前论文实验范围）

**目标**：在主链实现 CDT（Customizable Deliverable Trading）数据交易协议。

**计划 pallet**：
- `pallet-groth16-verifier`：BN254 曲线 Groth16 证明链上验证（4 种约束类型）
- `pallet-hash-verifier`：Keccak256 哈希链迭代验证
- `pallet-data-contract`（DC pallet）：数据发布与 IMT Root 管理
- `pallet-verify-contract`（VC pallet）：5 步数据交付协议，含争议仲裁和资金结算

**计划链下组件**：Go 服务（IMT 构建 + ZK 证明生成 + 链上交互）

**与现有实现的关系**：CDT 和众包协议（pallet-crowdsource）解耦，共享 pallet-fmc 资金基础设施，可独立实现。

---

*本文档由各阶段 plan 文件整合而成（2026-06-04），是 FishboneChain 项目从 Phase 0 到 6 链实验部署的完整实现记录。*
