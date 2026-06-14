# FishboneChain 实现规划

> 在 Substrate/Polkadot 生态上整合三个方案：FishboneChain 多链众包基础设施、CDT 数据交易协议、BPiano 跨链状态证明

> **状态说明（2026-06-11）**：本文是早期总体规划，包含 Relay+Parachain、HRMP/XCM、CDT 和 BPiano 的目标架构设想。当前已落地系统是多个独立 Substrate solo chain + 链下 bridge 的实验形态；当前事实以 [implementation-record.md](implementation-record.md)、[developer-guide.md](../development/developer-guide.md) 和当前代码为准。本文保留为历史规划和后续演进参考。

---

## 一、整体架构决策

### 1.1 三个方案的关系

```
BPiano（跨链状态证明）
  └─ 解决：子链向主链/目标链高效证明自身状态有效性
  └─ 服务于：FishboneChain 的主子链同步 + CDT 的数据来源可信性

CDT（可定制可验证数据交易）
  └─ 运行在：FishboneChain 的某条子链上（或主链）
  └─ 利用：BPiano 证明数据来源链的状态

FishboneChain（主链+多子链基础设施）
  └─ 是前两者的运行载体
  └─ 主链管资金和任务，子链管数据和验证
```

**核心思路**：先建好 FishboneChain 基础设施，再在其上实现 CDT 数据交易，最后用 BPiano 强化主子链之间的状态证明机制。

---

### 1.2 Substrate 架构选型

论文中的"主链+子链"在 Polkadot 生态中有三种映射方式，选择最符合论文语义的方案：

**选择：主链作为本地 Relay Chain，子链作为 Parachain**

```
本地 Relay Chain（主链角色）
  ├─ 提供共识安全性（等价于论文中主链的 PoS 角色）
  ├─ 持有 CCMC、FMC 等核心合约（作为 System Parachain）
  └─ 通过 HRMP 通道与各子链通信

Parachain 1（子链：数据众包场景）
Parachain 2（子链：CDT 数据交易场景）
Parachain N（子链：可扩展）
  └─ 每条子链独立共识（AURA/BABE）、独立区块参数
  └─ 定期向 Relay Chain 提交 Merkle 状态摘要
```

**理由**：
- Relay Chain 天然扮演"安全锚"角色，与论文中主链保障资金和仲裁的定位一致
- Parachain 可独立配置共识、区块时间、pallet 组合，完全对应论文"子链按业务定制"
- XCM 提供原生的跨链通信，对应论文中主子链同步机制
- 开发阶段用 Zombienet 在本地模拟完整网络，无需真实中继链插槽

**开发阶段的简化路径**：先把主链和子链都作为独立的 Substrate Solo Chain 开发和测试核心逻辑，再迁移到 Relay+Parachain 架构。

---

### 1.3 论文概念 → Substrate 组件映射

| 论文概念 | Substrate 实现方案 |
|---------|-----------------|
| 主链（Main Chain） | Relay Chain + System Parachain（`pallet-fishbone-main`）|
| 子链（Child Chain） | Parachain（独立 Substrate 节点）|
| CCMC（子链管理合约） | `pallet-ccmc`（主链上的 pallet）|
| FMC（资金管理合约） | `pallet-fmc`（主链上的 pallet）|
| TMC（任务管理合约） | `pallet-task`（子链上的 pallet）|
| 子链矿工押金 | `pallet-ccmc` 中的保证金存储，通过 `pallet-balances` 管理 |
| 聚合签名验证 | `pallet-ccmc` 中集成多签验证（Threshold Signature 或 BLS）|
| 主子链状态同步 | XCM 消息 + HRMP 通道，子链提交 Merkle Root 至 CCMC |
| 双花防护 | `pallet-fmc` 的 FB/LB 双余额机制（与 `pallet-balances` 集成）|
| CDT 的 DC 合约 | `pallet-data-contract`（子链上）|
| CDT 的 VC 合约 | `pallet-verify-contract`（子链或主链上）|
| CDT 的 ZK 验证 | `pallet-groth16-verifier`（集成 BN254 precompile）|
| CDT 的哈希链验证 | `pallet-hash-verifier`（链上纯计算）|
| BPiano 链上验证 | `pallet-bpiano-verifier`（4次配对验证，集成 BN254）|
| BPiano calldata 生成 | 链下 Go 服务（`solgen` 包，现有代码直接复用）|

---

## 二、分阶段实现规划

### Phase 0：开发环境与基础框架（2~3 周）

**目标**：搭建可运行的本地多链开发环境

**任务**：
1. 安装 Polkadot SDK（`polkadot-stable2512-3`）、Rust 工具链
2. 基于 `polkadot-sdk-parachain-template v0.0.5` 建立两个节点模板：
   - `fishbone-main`：主链节点（后续添加 CCMC/FMC pallet）
   - `fishbone-child`：子链节点模板（后续按场景扩展）
3. 配置 Zombienet 本地网络（1 个 relay chain + 2 个 parachain）
4. 验证 HRMP 通道建立，确认 XCM 消息可以双向传递
5. 建立 CI 测试框架（单元测试 + Chopsticks Fork 测试）

**产出**：
- 可启动的本地三链网络（relay + 2 parachains）
- 基础 XCM 消息传递验证通过
- 开发流程文档（如何添加 pallet、如何测试、如何升级 runtime）

---

### Phase 1：FishboneChain 主链基础设施（4~6 周）

**目标**：实现 FishboneChain 论文中的主链核心合约

#### 1.1 `pallet-fmc`（资金管理合约）

```rust
// 核心存储
FreeBalance: StorageMap<AccountId, Balance>   // FB：自由余额
LockedBalance: StorageMap<(AccountId, TaskId), Balance> // LB：锁定余额
Tasks: StorageMap<TaskId, TaskInfo>           // 任务状态机

// 关键 Dispatchable
fn deposit(origin, amount)              // 向 FMC 充值（→ FB）
fn activate_task(origin, task_id)       // FB > B 时激活，B 从 FB → LB
fn pay_bill(origin, task_id, bill)      // 验证账单签名 → 发放奖励 → 余额归还 FB
fn reactivate_task(origin, task_id)     // 充值后重新激活终止任务
```

**关键机制**：
- 任务状态机：`Terminated → Activated → Waiting → Terminated`
- `activate_task` 通过检查 `FB > B` 防止双花（不依赖外部信任）
- `pay_bill` 接收子链矿工提交的账单 + 聚合签名，验证后通过 XCM 向子链矿工和 worker 发放奖励

#### 1.2 `pallet-ccmc`（子链管理合约）

```rust
// 核心存储
ChildChains: StorageMap<ChainId, ChainInfo>
Miners: StorageMap<(ChainId, AccountId), MinerInfo>  // 含押金
EpochDigests: StorageMap<(ChainId, EpochId), Hash>  // Merkle Root 历史
MinerDeposits: StorageMap<(ChainId, AccountId), Balance>

// 关键 Dispatchable
fn register_child_chain(origin, params)     // 矿工建立新子链
fn join_child_chain(origin, chain_id)       // 矿工加入（需全体同意）
fn submit_epoch_digest(origin, chain_id, epoch, root, agg_sig) // 同步 Merkle Root
fn slash_miner(origin, chain_id, miner)    // 惩罚恶意矿工（投票机制）
fn terminate_child_chain(origin, chain_id, agg_sig) // 终止子链
fn verify_merkle_proof(chain_id, epoch, block, proof) -> bool  // Runtime API
```

**关键机制**：
- `submit_epoch_digest` 验证矿工聚合签名（BLS 或多签），存储 Merkle Root
- `verify_merkle_proof` 作为 Runtime API 对外暴露，供跨链验证使用
- 押金管理：`join_child_chain` 时锁定，恶意行为时 slash

#### 1.3 主子链同步 XCM 消息设计

```
子链 Epoch 结束时（同步时隙 Ss）：
  子链矿工代表 → 构造 XCM：
    Transact {
      call: ccmc.submit_epoch_digest(chain_id, epoch, merkle_root, agg_sig)
    }
  → 通过 HRMP 通道发送至主链

主链 FMC 接收账单：
  子链矿工代表 → 构造 XCM：
    Transact {
      call: fmc.pay_bill(task_id, bill_amounts, agg_sig)
    }
  → 主链 FMC 验证后，通过 XCM 反向发送奖励至子链工作者账户
```

**测试**：用 Zombienet + Chopsticks 模拟完整 epoch 周期，验证摘要同步和账单结算。

---

### Phase 2：子链基础设施（3~4 周）

**目标**：实现可定制的众包子链

#### 2.1 `pallet-crowdsource`（子链核心 pallet）

```rust
// 核心存储
ActiveTasks: StorageMap<TaskId, TaskDetail>   // 从主链同步来的任务
Submissions: StorageMap<(TaskId, AccountId), SubmissionData>
EpochState: StorageValue<EpochInfo>            // 当前 epoch 状态

// 关键 Dispatchable
fn sync_tasks(origin)                    // 同步时隙：从主链拉取激活任务
fn submit_data(origin, task_id, data)   // 工作者提交数据（预算耗尽时拒绝）
fn finalize_epoch(origin)               // 结算当前 epoch，生成账单和 Merkle Root
fn send_digest_to_main(origin)          // 触发 XCM，将摘要发往主链 CCMC
```

#### 2.2 子链可配置参数（每条子链按业务定制）

| 参数 | 配置位置 | 示例 |
|------|---------|------|
| 共识算法 | `runtime/src/consensus.rs` | AURA（快速）或 BABE（安全）|
| 区块时间 | `MILLISECS_PER_BLOCK` | 配送：6s，天气：3s |
| 数据验证算法 | `pallet-crowdsource` 的 `Config::Validator` | 位置数据验证器、图像验证器 |
| 收集时隙长度 Sc | `pallet-crowdsource::Config::CollectingSlotBlocks` | 可按 epoch 块数配置 |
| 隐私方案 | 可选集成 `pallet-data-encrypt` | 对应不同数据类型 |

---

### Phase 3：CDT 数据交易（5~7 周）

**目标**：在子链上实现可定制可验证数据交易

这是三个方案中工程量最大的，需要将现有 Go 代码的逻辑移植为 Substrate pallet。

#### 3.1 `pallet-groth16-verifier`（ZK 证明链上验证）

这是整个 CDT 方案的密码学基础，需要在 Substrate 上做 BN254 配对验证。

**方案 A（推荐）**：使用 `substrate-bn` crate（Rust 原生）
```rust
// 集成 ark-bn254 或 substrate-bn 做配对运算
// 实现 Groth16 验证逻辑（与现有 Solidity 验证器逻辑等价）
fn verify_groth16(
    proof: Groth16Proof,
    vk: VerifyingKey,
    public_inputs: Vec<Fr>,
) -> bool

// 对外暴露三种约束类型的验证接口
fn verify_range_hash_proof(proof, input: [hash, min, max]) -> bool
fn verify_subset_hash_proof(proof, input: [hash, set_root]) -> bool
fn verify_substr_hash_proof(proof, input: [hash, substr_hash]) -> bool
fn verify_root_obfuscation_proof(proof, input: [leaf, root0..3], depth: u32) -> bool
```

**方案 B**：通过 `pallet-revive`（EVM 兼容层）直接部署现有 Solidity 验证器合约
- 优点：无需重写，直接复用 `references/data_trade_code/foundry/src/gnark/` 下的合约
- 缺点：引入 EVM 依赖，Gas 模型不同，部署复杂度增加

**建议**：阶段早期用方案 B 快速验证端到端流程，后期用方案 A 做原生实现。

#### 3.2 `pallet-data-contract`（DC 合约，子链上）

```rust
// 核心存储
DataListings: StorageMap<ListingId, DataDescription>
IMTRoots: StorageMap<ListingId, Hash>   // IMT 根哈希

// 关键 Dispatchable
fn publish_data(origin, imt_root: Hash, description: DataDescription)
fn update_imt_root(origin, listing_id, new_root: Hash)
```

#### 3.3 `pallet-verify-contract`（VC 合约，子链或主链上）

```rust
// 核心存储
Sessions: StorageMap<SessionId, TradingSession>
  // Session 包含：data_owner, data_requester, hash_chain_end, max_rounds
  // locked_funds, deposit, status

// 关键 Dispatchable
fn create_session(origin, data_owner, hash_chain_end, max_rounds)  // DR 部署
fn lock_funds(origin, session_id)                // DR 锁定资金
fn lock_deposit(origin, session_id)              // DO 锁定押金

// 争议处理（DR 调用）
fn punish_invalid_range_proof(session_id, proof, input, signature)
fn punish_invalid_subset_proof(session_id, proof, input, signature)
fn punish_invalid_substr_proof(session_id, proof, input, signature)
fn punish_hash_mismatch(session_id, message, expected, given, signature)

// DO 结算
fn claim_funds(session_id, pre_image: Vec<u8>)   // 哈希链验证后结算
fn claim_last_payment(session_id, substr_proof, ro_proof)
```

#### 3.4 `pallet-hash-verifier`

```rust
// 链上迭代 keccak256，验证哈希链，返回完成轮数
fn verify_hash_chain(pre_image: Vec<u8>, target: Hash, max_rounds: u32) -> Option<u32>
```

**注意**：链上计算 keccak256 迭代有 weight 限制，需要 benchmark 确定合理的 `max_rounds` 上限，并在权重模型中正确计量。

#### 3.5 IMT（完整性 Merkle 树）的链下实现

IMT 构建和 ZK 证明生成全部在链下（GO 服务）完成，链上只验证：
- 现有代码 `data_trade_code/snarks/gnarkzkp/` 可直接复用
- 需要编写一个链下服务（Go），负责：
  - 数据上传时构建 IMT、生成 ZK 证明
  - 调用链上 DC pallet 提交 IMT 根
  - 在 5 步交付协议中生成各类 ZK 证明
  - 与 VC pallet 交互（提交证明、验证签名、触发结算）

**链下服务 → 链上的接口**：通过 Substrate RPC（`author_submitExtrinsic`）提交 extrinsic。

---

### Phase 4：BPiano 跨链状态证明集成（4~6 周）

**目标**：用 BPiano 协议强化子链向主链的状态证明，替代 Phase 1 中简单的 Merkle Root 提交

#### 4.1 `pallet-bpiano-verifier`（主链上）

将现有 `BPianoVerifier.sol` 的逻辑移植为 Substrate pallet：

```rust
// 核心接口
fn verify_compressed_proof(
    proof: BPianoCompressedProof,
    vk: BPianoVerifyingKey,
    public_inputs: Vec<Vec<Fr>>,
    // G2 点由链下预计算后作为 calldata 传入（EVM 同样的限制在这里也存在）
    zt_g2: G2Affine,
    tau_y_beta_g2: G2Affine,
) -> DispatchResult

fn verify_aggregated_proof(
    proof: BPianoAggregatedProof,
    vk: BPianoVerifyingKey,
    k: u32,
) -> DispatchResult
```

**密码学依赖**：
- `ark-bn254`（BN254 椭圆曲线算术）
- `ark-groth16` 或手写配对验证逻辑
- 4 次配对运算是主要 weight 成本，需要 benchmark

#### 4.2 子链状态证明的完整流程

```
子链 Epoch 结束：
  1. 子链矿工节点（M 个）各自运行 Piano.Prove()，生成本地 witness 证明
  2. 主节点调用 bpiano.CoordinateChallenges() + AggregateProofs()
  3. 链下 Go 服务调用 solgen.GenerateBPianoCalldata() 生成 calldata
     （含 G2 预计算：ZTG2, TauYBetaG2）
  4. 主节点通过 XCM 提交 BPiano 批量证明至主链：
     Transact {
       call: pallet_bpiano_verifier.verify_and_record(chain_id, epoch, proof, vk, calldata)
     }
  5. 主链 pallet_bpiano_verifier 执行 4 次配对验证：
     - 验证通过 → 更新 CCMC 的 epoch digest 为已验证状态
     - 验证失败 → 触发子链矿工惩罚流程
```

#### 4.3 BPiano 的 Keccak 电路与跨链状态的对接

论文中 BPiano 用 Keccak-256 电路作为基准（验证以太坊 MPT 状态树节点哈希）。在 FishboneChain 场景中：

- **证明目标**：子链的区块头 Merkle Root（每个子节点持有部分区块数据）
- **电路**：需要将子链 Substrate 区块头的哈希计算（BLAKE2b）包装为 BPiano 兼容电路
  - 选项 A：继续用 Keccak 电路，但子链改用 Keccak 作为区块哈希（修改 `frame-system` 配置）
  - 选项 B：为 BLAKE2b 编写新的 gnark 电路（现有 `keccak/gnark_circuit.go` 可作参考）

---

## 三、关键技术挑战与对策

### 挑战 1：链上 ZK 证明验证的 Weight 计量

BN254 配对运算计算量大，在 Substrate 的 weight 模型下需要精确 benchmark。

**对策**：
- 使用 `frame-benchmarking` 对每种验证函数单独 benchmark
- Groth16 验证约需 3 次配对，BPiano 需 4 次，作为 weight 上限参考
- 通过 `pallet-referenda` 控制高 weight 操作的提交频率

### 挑战 2：BLS 聚合签名支持

子链矿工对账单和状态摘要的聚合签名，Substrate 原生支持 sr25519/ed25519，不原生支持 BLS。

**对策**：
- 使用 `w3f/bls` crate 或 `blst` crate 实现 BLS12-381 聚合签名验证 pallet
- 或者简化为：要求超过 2/3 矿工独立提交签名，链上计票（牺牲效率换简单性）
- 长期目标：等待 Polkadot SDK 内置 BLS 支持

### 挑战 3：链下 ZK 证明生成服务的可信性

Go 链下服务（生成 IMT、ZK 证明、BPiano calldata）需要与链上状态保持一致。

**对策**：
- 链下服务以 Substrate 的 **Offchain Worker** 形式运行（可访问链状态）
- 或者作为独立服务，通过监听链上事件触发（使用 `subxt` 或 PAPI）
- 关键：链下生成的证明在链上可独立验证，无需信任链下服务

### 挑战 4：XCM 消息的原子性与超时处理

子链提交账单至主链 FMC 的 XCM 消息可能因为各种原因失败。

**对策**：
- 实现 XCM `ReportError` 和重试机制
- FMC 添加"待确认账单"队列，超时未确认的账单不发放奖励
- 使用 `pallet-xcm` 的 `send_xcm` + `QueryResponseHandler` 处理异步响应

### 挑战 5：G2 标量乘的链下计算可信性（BPiano）

BPiano 的 `ZTG2` 和 `TauYBetaG2` 必须链下预计算（EVM 和 Substrate 都缺 G2 标量乘预编译）。

**对策**：
- 这两个 G2 点由 verifying key（VK）和挑战值确定性推导，任何人都可验证
- 链上 pallet 验证逻辑：接收 G2 点后，用 G1 配对反向检验其正确性（额外 2 次配对）
- 或：将 G2 预计算结果存入 CCMC，由多个矿工提交一致后采信

---

## 四、开发优先级与时间线

```
Phase 0（周 1-3）：   环境 + 框架
Phase 1（周 4-9）：   主链 CCMC + FMC + 基础 XCM
Phase 2（周 10-13）： 子链众包 pallet + 配置体系
Phase 3（周 14-20）： CDT 数据交易（ZK 验证 + VC + DC）
Phase 4（周 21-26）： BPiano 跨链证明集成

共约 6 个月，各 Phase 可并行推进。
```

**建议先完成的最小可验证系统（MVP）**：
1. Phase 0 + Phase 1 的主链 FMC（不含 XCM，仅单链资金管理）
2. Phase 3 的 CDT 单链版本（主子链都跑在同一条链上，不涉及跨链）
3. 验证端到端数据交易流程（上传数据→发起请求→多轮交付→结算）

MVP 完成后，再逐步扩展到多链架构和 BPiano。

---

## 五、代码组织建议

```
fishbonechain/
├── node/                       # Substrate 节点实现
│   ├── fishbone-main/          # 主链节点
│   └── fishbone-child/         # 子链节点模板
│
├── runtime/
│   ├── fishbone-main-runtime/  # 主链 runtime
│   └── fishbone-child-runtime/ # 子链 runtime 模板
│
├── pallets/
│   ├── pallet-ccmc/            # 子链管理合约
│   ├── pallet-fmc/             # 资金管理合约
│   ├── pallet-task/            # 任务管理
│   ├── pallet-crowdsource/     # 子链众包逻辑
│   ├── pallet-groth16-verifier/ # ZK 验证（CDT 用）
│   ├── pallet-data-contract/   # DC 合约（CDT）
│   ├── pallet-verify-contract/ # VC 合约（CDT）
│   ├── pallet-hash-verifier/   # 哈希链验证（CDT）
│   └── pallet-bpiano-verifier/ # BPiano 链上验证
│
├── offchain/                   # 链下服务（Go）
│   ├── imt-service/            # IMT 构建 + ZK 证明生成（复用 data_trade_code）
│   ├── bpiano-service/         # BPiano 证明生成（复用 efficient_cross_chain_proof_code）
│   └── chain-client/           # 链上交互客户端（subxt 或直接 RPC）
│
├── zombienet/                  # 本地网络配置
│   └── local-testnet.toml      # relay + 2 parachains 配置
│
└── docs/                       # 文档（已有）
```

---

## 六、已确认的关键决策

| 问题 | 决策 | 理由 |
|------|------|------|
| Rust 版本 | **stable 1.96**（方案 B）| 已安装工具（omni-node 0.5.0）指向更新 SDK，逆向降级成本高；1.96 满足 alloy 的 ≥1.91 要求 |
| SDK 版本 | **polkadot-stable2512-3** | cookbook `versions.yml` 中验证过的最新稳定版，与 parachain-template v0.0.5 配套 |
| ZK 验证实现 | **Rust 原生重写**（ark-bn254 + ark-groth16）| 不依赖 EVM 层，pallet 更轻量，逻辑与 Go 参考代码一一对应 |
| CDT 运行位置 | **子链**（专属数据交易子链）| 灵活，不增加主链负担，与论文"子链按场景定制"语义一致 |
| 子链共识 | **AURA**（初期）| 开箱即用，先跑通业务逻辑，后续可替换 |
| 起点策略 | **Phase 0 环境优先**，再实现业务 pallet | 没有稳定的多链环境，业务代码无法集成测试 |
| 初期多链模式 | **多 Solo Chain**（AURA+GRANDPA）| 无需 relay chain 即可出块，迭代最快；Phase 1 后迁移到 Relay+Parachain |
