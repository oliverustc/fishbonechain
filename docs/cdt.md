# 实现文档总览

本文档汇总两篇论文的核心方案，供后续实现参考：

1. **CDT**（可定制可验证数据交易）— 基于 zk-SNARKs 的数据交易协议
2. **FishboneChain**（鱼骨链众包平台）— 基于主链 + 多子链的可扩展众包平台

两者可结合：FishboneChain 提供众包基础设施，CDT 提供数据交易的隐私验证层。

---

# 第一部分：可定制可验证数据交易方案（CDT）

> 基于论文：*Customizable Data Trading via Blockchain: A Verifiable Approach Based on zk-SNARKs*

---

## 一、方案总览

本方案实现数据所有者（DO）与数据请求者（DR）之间的可定制、可验证、公平的数据交易。核心技术栈：

- **zk-SNARK**：证明数据合规性与完整性，不暴露敏感信息
- **四层完整性 Merkle 树（IMT）**：组织数据，支持高效 ZK 证明生成
- **链下通道（Off-chain Channel）**：多轮数据交付，降低链上成本
- **智能合约（DC + VC）**：链上仲裁与资金管理

整个交易分为四个阶段：数据上传 → 数据请求 → 数据交付与验证 → 资金结算。

---

## 二、角色定义

| 角色 | 符号 | 职责 |
|------|------|------|
| 数据所有者 | DO | 收集数据、构建 IMT、生成 ZK 证明、按需脱敏交付数据 |
| 数据请求者 | DR | 发起请求、指定约束与脱敏规则、多轮验证质量、付款 |
| 数据合约 | DC | 由 DO 部署，存储 IMT 根哈希与数据描述 |
| 验证合约 | VC | 由 DR 部署，管理资金、DO 押金、链上 ZK 证明验证与争议仲裁 |

**威胁模型**：DO 和 DR 均视为潜在恶意方。

- 恶意 DO：伪造、篡改或提交低质量数据
- 恶意 DR：拒绝付款、或从证明中推断数据规模/内容

---

## 三、数据模型

### 3.1 数据集结构

数据集 `DS = {A, E}`：

- **属性集 A**（静态特征）：键值对集合 `A = {(k_i, v_i)}`，如车辆型号、出厂日期
- **条目集 E**（动态记录）：结构相同的记录集合 `E = {e_i}`，每条 `e_i = (e_i^{f_1}, ..., e_i^{f_t})`，如时序传感器数据

### 3.2 四层完整性 Merkle 树（IMT）

从叶到根依次为：

#### 第一层：Entry Layer（条目层）

- 每条数据记录 `e_i` 构建一棵 Merkle 树 `MT_{e_i}`
- 叶节点为各字段值加随机盐的哈希：`H^{f_j}_{e_i} = H(e_i^{f_j} || r_{e_i}^{f_j})`
- 细粒度设计：DR 可对每个字段单独提出约束，ZK 证明输入极短

#### 第二层：Dataset Layer（数据集层）

- 每个数据集 `DS_i` 构建树 `MT_{DS_i}`，包含两个子树：
  - **属性子树**：叶节点 = `H(k_i || v_i || r_{k_i})`
  - **条目子树**：叶节点 = Entry Layer 的根节点 `H_{e_i}`
- 两棵子树合并，根为 `H_{DS_i}`（数据集摘要）

#### 第三层：Aggregate Layer（聚合层）

- 将多个数据集的摘要 `H_{DS_1}, ..., H_{DS_n}` 作为叶节点构建 Merkle 树
- 根为 `H_{DS}`，DO 只需上链一个哈希即可承诺所有数据集

#### 第四层：Padding Layer（填充层）

- DO 预定义最大深度 `D_max`，通过随机填充将所有 IMT 扩展至 `D_max`
- **目的**：防止 DR 从树深推断数据规模（条目数量、数据集数量）
- 填充节点随机作为左/右子树，不要求构成完整二叉树
- 确保 ZK 证明的约束数量固定统一

构建完成后，DO 将 IMT 根 `R` 及数据描述上传至 `DC`。

---

## 四、数据请求格式

请求 `R = {R_a, R_e}`，包含属性请求和条目请求，每类又分明文和脱敏两种。

### 4.1 属性请求 `R_a = {R_a^p, R_a^m}`

- **明文属性请求** `R_a^p = {(k_i, c_i^a)}`：
  - `c_i^a = ∅`：直接提供 `v_i`
  - `c_i^a ≠ ∅`：只提供满足约束的值
  - 例：`{(vehicle_model, =Tesla Model Y), (battery_capacity, ∅)}`

- **脱敏属性请求** `R_a^m = {(k_i, c_i^a, mr_i)}`：
  - 额外指定脱敏规则 `mr_i`
  - 例：`{(manufacture_date, year=2020, YYYY-XX-XX)}`

### 4.2 条目请求 `R_e = {R_e^p, R_e^m}`

- **明文字段请求** `R_e^p = {(f_i, c_i)}`：
  - 例：`{(time, year=2023), (power_consumption, ∅), (battery_temp, ∅)}`

- **脱敏字段请求** `R_e^m = {(f_i, c_i^f, mr_i)}`：
  - 例：`{(location, {lat∈[...], lon∈[...]}, precision=1')}`

### 4.3 定制化数据集（Tailored Dataset）

DO 按请求 R 生成定制化数据集 `DS_T = {A_T, E_T}`：

```
A_T: 对每个 k_i ∈ K_Ra^p ∪ K_Ra^m：
  - 若 k_i ∈ K_Ra^p 且 v_i ⊨ c_i^a → 保留原值 v_i
  - 若 k_i ∈ K_Ra^m 且 v_i ⊨ c_i^a → 应用脱敏规则 mr_i(v_i)

E_T: 对每个 e_i 的每个字段 f_j：
  - 若 f_j ∈ F_Re^p 且 e_i^{f_j} ⊨ c_j^f → 保留原值
  - 若 f_j ∈ F_Re^m 且 e_i^{f_j} ⊨ c_j^f → 应用 mr_j
  - 只有所有请求字段均满足约束，该条目才纳入 E_T
```

---

## 五、ZK 证明机制

### 5.1 约束-哈希证明（Constraint-Hash Proof，CH Proof）

将数据内容的有效性验证与哈希值绑定，通过 ZK 证明对外不暴露原始值。

**属性值的 CH 证明**：
```
π_a^CH(k_i, v_i) = ZKP{(k_i, v_i, r_{k_i}; h_i) :
  v_i ⊨ c_i^a  ∧  H(k_i || v_i || r_{k_i}) = h_i}
```
- 私有输入：`k_i, v_i, r_{k_i}`
- 公开输入：`h_i`（叶节点哈希）

**条目字段值的 CH 证明**：
```
π_e^CH(e_j^{f_t}) = ZKP{(e_j^{f_t}, r_{e_j}^{f_t}; h_j^{f_t}) :
  e_j^{f_t} ⊨ c_t^f  ∧  H(e_j^{f_t} || r_{e_j}^{f_t}) = h_j^{f_t}}
```

**三种约束类型**（需分别实现电路）：
- **范围约束（Range）**：证明值在区间 `[min, max]` 内
- **子集约束（Subset）**：证明值属于预定义集合
- **子串约束（Substr）**：证明公开部分是原始值的子串，同时验证整体哈希

### 5.2 根混淆证明（Root Obfuscation Proof，RO Proof）

证明哈希叶节点存在于链上承诺的 IMT 中，同时隐藏数据规模与结构信息。

```
π_h^RO = ZKP{(MP(h), i, RL) : V(MP(h)) = RL[i]}
```
- 私有输入：Merkle 证明路径 `MP(h)`、根在根列表中的索引 `i`
- 公开输入：根列表 `RL`

设计要点：
- Merkle 路径作为私有输入，防止暴露树深和结构
- 结合 Padding Layer，有效隐藏数据批次、规模和组织关系
- `RL` 为多个根的列表，支持多批次数据

### 5.3 完整证明

```
π = π^CH || π^RO
```

- **脱敏数据**：`π^CH + π^RO` 共同证明约束有效性和数据完整性
- **明文数据**：`π^RO` 证明哈希值存在于 IMT 中；DR 收到明文后自行验证约束和完整性

---

## 六、公平多轮数据交付协议

### 6.1 哈希链付款承诺

DR 选择秘密值 `s`，递归应用哈希函数 `n` 次：
```
H^(n)(s) = H^(n)
```
- `H^(n)` 作为锚点上传至 VC（公开）
- `H^(n-i)` 作为第 `i` 轮的付款承诺（对应 `i` 个子集的付款）

### 6.2 单轮交付的 5 步协议

每轮交付第 `i` 个子集 `DS_{T,i}`：

**Step 1：DR → DO**：付款承诺有效性证明 `π_{pc}`
```
π_{pc} = ZKP{H^(n-i), H^(n+1-i) : H(H^(n-i)) = H^(n+1-i)}
```
私有输入：`H^(n-i)`；公开输入：`H^(n+1-i)`

**Step 2：DO → DR**：ZK 证明 `π = π^CH || π^RO`
- 只发证明，不发数据本身（防止 DR 得到数据后拒绝付款）

**Step 3：DR → DO**：对证明的签名 `σ`
```
σ = Sign(vk_DR, H(π))
```
- 验证通过则签名，表示同意本轮继续
- 验证失败则向 VC 提交无效证明作为证据，扣除 DO 押金

**Step 4：DO → DR**：实际数据 `DS_{T,i}`
- 收到签名后才发数据（防止 DR 拒绝签名后仍索取数据）
- DR 验证明文约束和完整性；不合规则向 VC 提交证据

**Step 5：DR → DO**：付款承诺 `H^(n-i)`
- 数据合规则提供；拒绝则 DO 向 VC 提交证据索赔

### 6.3 争议解决机制

| 争议场景 | 发起方 | 提交证据 | 合约处理 |
|----------|--------|----------|----------|
| Step 2：DO 发送无效 ZK 证明 | DR | 无效的 `π^CH` 或 `π^RO` | VC 验证失败 → 扣除 DO 押金转给 DR，终止 |
| Step 4：DO 发送不合规明文数据 | DR | 违规数据 + 对应哈希 | VC 验证约束失败或哈希不匹配 → 扣除 DO 押金，终止 |
| Step 5：DR 拒绝提供付款承诺 | DO | `π_{pc}` + `σ` + `DS_{T,i}` | VC 验证全部通过 → 将本轮资金转给 DO |
| Step 1：DR 发送无效 `π_{pc}` | DO | — | DO 直接拒绝继续，无需上链 |
| Step 3：DR 拒绝提供签名 | DO | — | DO 拒绝交付数据，无需上链 |

**特殊情况**：DO 若不交付数据直接向 VC 申请奖励，必须提交完整数据集（DR 因此仍能获得数据），且 DO 需额外支付 gas 费，无法占便宜。

---

## 七、完整交易流程

### Phase 1：数据上传

1. DO 收集数据集，按数据模型组织
2. 构建 IMT（Entry → Dataset → Aggregate → Padding）
3. 部署 `DC` 合约，上传 IMT 根 `R` 和数据描述（字段、属性、采集时间、特征等）

### Phase 2：数据请求

**链下协商**：
1. DR 浏览 DC 上的数据描述，选定 DO
2. 建立链下安全通道，协商：
   - 数据请求 `R`（约束 + 脱敏规则）
   - 每轮条目数量
   - 每轮交易金额 `b`
   - 预估总轮数 `n`

**链上操作**：
1. DR 部署 VC，合约内置：
   - 明文约束验证函数
   - CH 证明验证函数（range/subset/substr）
   - RO 证明验证函数
   - 签名验证函数
   - 资金管理逻辑
2. DR 锁定总资金 `F_n = n × b`，上传哈希链锚点 `H^(n)`
3. DO 向 VC 锁定安全押金 `D_DO`

### Phase 3：数据交付与验证

1. DO 根据请求 `R` 生成定制化数据集 `DS_T`
2. 将 `DS_T` 分成 `n` 个子集 `DS_{T,1}, ..., DS_{T,n}`
3. 为每个子集预生成 ZK 证明
4. 按 6.2 节的 5 步协议逐轮交付
5. 任一轮出现争议，按 6.3 节处理；正常则继续直至 DR 资金耗尽

### Phase 4：资金结算

1. DO 向 VC 提交最终付款承诺明文 `H^m`（代表完成了 `n-m` 轮）
2. VC 验证：`H^(n-m)(H^m) = H^(n)`（即对 `H^m` 哈希 `n-m` 次等于锚点）
3. VC 自动转账：
   - `(n-m) × b` 转给 DO（已交付轮数的报酬）
   - `m × b` 退还 DR（未完成轮数）
   - 若 DO 未被惩罚，退还押金 `D_DO`

---

## 八、智能合约设计

### 8.1 数据合约（DC）

由 DO 部署，存储：
- IMT 根哈希 `R`
- 数据描述（可用属性、字段、采集时段、数据特征）

### 8.2 验证合约（VC）

由 DR 部署，实现以下函数：

```solidity
// 资金管理
function lockFunds(uint256 amount, bytes32 hashChainAnchor) external payable;
function lockDeposit() external payable;  // DO 调用
function claimFunds(bytes32 preimage) external;  // DO 结算

// ZK 证明验证（链上）
function verifyRangeHashProof(bytes calldata proof, bytes32[] calldata publicInputs) external returns (bool);
function verifySubsetHashProof(bytes calldata proof, bytes32[] calldata publicInputs) external returns (bool);
function verifySubstrHashProof(bytes calldata proof, bytes32[] calldata publicInputs) external returns (bool);
function verifyRootObfuscationProof(bytes calldata proof, bytes32[] calldata rootList) external returns (bool);

// 明文验证
function verifyPlaintextConstraint(bytes calldata data, bytes calldata constraintSpec) external returns (bool);

// 签名验证
function verifySignature(bytes32 messageHash, bytes calldata signature, address signer) external returns (bool);

// 争议处理
function punishInvalidProof(bytes calldata invalidProof, bytes32[] calldata publicInputs) external;
function punishInvalidPlaintext(bytes calldata data, bytes calldata proof) external;
function claimRewardForLastRound(bytes calldata proof, bytes calldata signature, bytes calldata dataset) external;
```

---

## 九、技术实现选择

### 9.1 ZK 证明系统

| 项目 | 选择 | 说明 |
|------|------|------|
| ZK 框架 | [gnark](https://github.com/ConsenSys/gnark) + gnark-crypto | Go 实现，性能优 |
| 协议 | **Groth16**（推荐）或 Plonk | Groth16 约束数少、验证快（~3ms） |
| 椭圆曲线 | **BN254** | Ethereum 原生预编译支持 |
| 哈希函数（电路内） | **MiMC** | ZK 友好，gnark-crypto 原生支持 |
| 安全参数 | 128 bits | BN254 提供约 128 位安全性 |

### 9.2 性能参考数据

**CH 证明生成**：
- 所有约束类型（range/subset/substr）：< 40ms（Groth16）

**CH 证明验证**：
- 链下：~3ms（Groth16），~6ms（Plonk）
- 链上 gas：~20,000 gas（Ethereum），~1,000,000 gas（zkSync）

**RO 证明生成**（IMT 深度 10~20）：
- Groth16：50ms~100ms，电路约束数 1,000~11,000
- Plonk：~270ms（深度≤18），~400ms（深度 20，触发内存换页）

**链上操作成本（Ethereum Sepolia）**：

| 操作 | Gas 消耗 | ETH 成本 |
|------|----------|----------|
| DR 部署合约 | 10,822,935 | 0.0148 |
| DO 锁定押金 | 75,808 | 0.000104 |
| DO 申请资金 | 95,596 | 0.000131 |
| 惩罚无效 range-hash 证明 | 314,897 | 0.000432 |
| 惩罚无效 subset-hash 证明 | 114,019 | 0.000156 |
| 惩罚无效 substr-hash 证明 | 307,858 | 0.000422 |
| 惩罚无效明文数据 | 100,486 | 0.000137 |
| DO 申请最后一轮奖励 | 3,262,844 | 0.00447 |

### 9.3 链选择

- 主网部署建议：**Ethereum + zkSync**（zkSync gas 消耗量大但 ETH 成本低）
- 测试网：Sepolia（Ethereum）+ zkSync Sepolia
- 合约语言：**Solidity**，使用 Foundry 测试框架

---

## 十、安全性保证

| 属性 | 保证机制 |
|------|----------|
| 敏感信息保密 | ZK-SNARK 零知识性：CH 证明不泄露 `v_i` 和 `e_j^{f_t}` |
| 数据规模保密 | RO 证明 + Padding Layer：不泄露数据条数、批次、层级结构 |
| 约束不可伪造 | ZK-SNARK 可靠性：不满足约束的数据无法生成有效 CH 证明 |
| 数据不可篡改 | 碰撞抗性哈希 + Merkle 树：篡改数据无法生成有效 RO 证明 |
| 公平性 | 多轮协议设计：任何一方的恶意行为要么被即时发现终止，要么被智能合约惩罚 |
| 可接受公平性 | 数据分成小子集（`value_subset ≪ value_dataset`），DR 最大损失 = 一个子集的费用 |

---

## 十一、实现优先级建议

建议按以下顺序实现：

1. **数据模型与 IMT 构建**（Go）：定义数据结构，实现四层 Merkle 树构建与 Merkle 证明生成
2. **ZK 电路实现**（gnark）：
   - CH 证明电路（range / subset / substr 三种）
   - RO 证明电路（可变深度 Merkle 路径验证）
3. **智能合约**（Solidity）：DC 合约、VC 合约（含 ZK 验证器）
4. **链下交付协议**：哈希链生成、五步交付协议实现、签名机制
5. **数据请求处理**：请求解析、数据过滤与脱敏、定制化数据集生成
6. **集成测试**：端到端流程测试，含争议场景

---

## 十二、参考代码结构（`references/data_trade_code/`）

> 已有原型代码，实现时直接参考或复用。路径根为 `references/data_trade_code/`。

### 12.1 整体目录结构

```
data_trade_code/
├── foundry/                        # Solidity 合约层（Foundry 框架）
│   ├── src/
│   │   ├── Fund.sol                # 核心：资金管理合约
│   │   ├── Verify.sol              # 核心：证明验证与争议仲裁合约
│   │   ├── HashVerifier.sol        # 工具：哈希链验证合约
│   │   ├── gnark/                  # 自动生成的 ZK 验证器合约（勿手动修改）
│   │   │   ├── groth16RangeHashProofVerifier.sol
│   │   │   ├── groth16SubsetHashProofVerifier.sol
│   │   │   ├── groth16SubstrHashProofVerifier.sol
│   │   │   ├── groth16RootObfuscationProofVerifier{10..20}.sol  # 深度10~20各一个
│   │   │   ├── plonkRangeHashProofVerifier.sol
│   │   │   ├── plonkSubsetHashProofVerifier.sol
│   │   │   ├── plonkSubstrHashProofVerifier.sol
│   │   │   └── plonkRootObfuscationProofVerifier{10..20}.sol
│   │   └── snark/                  # 验证器基础模板
│   │       ├── GnarkGroth16Verifier.sol
│   │       ├── GnarkPlonkVerifier.sol
│   │       └── SnarkJSGroth16Verifier.sol
│   ├── test/                       # 测试文件（仅供参考，无需执行）
│   │   ├── unit/                   # 核心合约单元测试
│   │   ├── fund/                   # Fund 合约测试
│   │   └── gnark/                  # ZK 验证器测试（groth16/plonk 各9个）
│   └── script/
│       └── DeployContracts.s.sol   # 合约部署脚本
│
└── snarks/
    └── gnarkzkp/                   # Go 实现的 ZKP 框架
        ├── gnarkwrapper/           # 核心：ZKP 抽象框架
        │   ├── wrapper.go          # 接口定义与工厂函数
        │   ├── groth16.go          # Groth16 实现
        │   ├── groth16_io.go       # Groth16 序列化/文件 IO
        │   ├── groth16_solidity.go # Groth16 → Solidity 导出
        │   ├── groth16_recursion.go
        │   ├── plonk.go            # Plonk 实现
        │   ├── plonk_io.go
        │   ├── plonk_solidity.go
        │   ├── plonk_recursion.go
        │   ├── solidity_templates.go # Solidity 代码生成模板
        │   └── param.go            # 曲线参数（BN254 等）
        ├── cmd/
        │   ├── constraint-hash-proof/   # 约束哈希证明命令
        │   │   ├── circuit_range_hash.go
        │   │   ├── circuit_subset_hash.go
        │   │   ├── circuit_substr_hash.go
        │   │   └── main.go
        │   ├── root-obfuscation-proof/  # 根混淆证明命令
        │   │   ├── circuit_root_obfuscation.go
        │   │   └── main.go
        │   └── aggregate/               # 聚合证明命令（空 main，待实现）
        │       ├── circuit_multiple_root_obfuscation.go
        │       ├── circuit_multiple_substr_hash.go
        │       └── main.go
        ├── hash/
        │   ├── mimchash/mimc.go     # MiMC 哈希（ZK 友好，电路内使用）
        │   └── sha/sha256.go        # SHA256 包装
        ├── merkletree/tree.go       # 多叉 Merkle 树（支持自定义哈希函数）
        ├── logger/logger.go         # 结构化日志
        ├── utils/                   # 文件操作、随机数生成
        ├── go.mod                   # 模块名：gnarkabc
        └── Readme.md
```

### 12.2 Solidity 核心合约

#### `Fund.sol` — 资金管理合约

由 DR 部署，构造时传入 `dataOwner` 地址、`hashChainEnd`（哈希链锚点 H^(n)）、`maxRounds`（最大轮数）、`verifyContract` 地址。

```solidity
// 关键状态
address public immutable dataRequester;
address public immutable dataOwner;
bytes32 public immutable hashChainEnd;   // 哈希链锚点 H^(n)
uint256 public lockedFunds;              // DR 锁定的总资金
uint256 public deposit;                  // DO 押金
uint256 public immutable maxRounds;

// 关键函数
lockFunds()          // DR 锁定资金（payable）
lockDeposit()        // DO 锁定押金（必须在 lockFunds 后调用）
claimFunds(preImage) // DO 提交哈希链原像，按完成轮数比例领取报酬 + 押金
punish()             // 仅 VC 可调用：将全部余额转给 DR（惩罚 DO）
claimLastPayment()   // 仅 VC 可调用：向 DO 转账最后一轮的金额
```

`claimFunds` 内部调用 `HashVerifier.verify(preImage, hashChainEnd)` 得到完成轮数 `cycles`，奖励 = `lockedFunds * cycles / maxRounds`。采用"先清零再转账"防重入。

#### `Verify.sol` — 证明验证与争议仲裁合约

由 DR 部署，内部实例化 4 个 ZK 验证器合约（当前硬编码使用 Groth16 + 深度20 的 RO 验证器）。

```solidity
// 初始化时硬编码的验证器
groth16RangeHashProofVerifier   rangeHashVerifier;
groth16SubsetHashProofVerifier  subsetHashVerifier;
groth16SubstrHashProofVerifier  substrHashVerifier;
groth16RootObfuscationProofVerifier20 rootObfuscationVerifier;

// 关键函数（均由 DR 调用）
setFundAddress(address)                           // 关联 Fund 合约（只能设置一次）
punishIfRangeHashProofFailed(proof, input, sig)   // 验证 Range-Hash 证明 + 签名，失败则惩罚
punishIfSubsetHashProofFailed(proof, input, sig)  // 验证 Subset-Hash 证明 + 签名，失败则惩罚
punishIfSubstrHashProofFailed(proof, input, sig)  // 验证 Substr-Hash 证明 + 签名，失败则惩罚
punishIfHashDismatch(msg, expectedHash, givenHash, sig) // 验证明文哈希匹配，不匹配则惩罚

// 由 DO 调用
claimLastPayment(substrProof, substrInput, roProof, roInput) // 多次验证后领取最后一轮报酬
```

签名验证：ECDSA（`ecrecover`），消息哈希 = `keccak256("\x19Ethereum Signed Message:\n32" || keccak256(proofHash || inputHash))`。

#### `HashVerifier.sol` — 哈希链验证

```solidity
// 从 preImage 开始迭代 keccak256，返回到达目标哈希 h 所需的次数
function verify(bytes memory preImage, bytes32 h) returns (uint256 counter)
// 最大迭代 1000 次防止死循环
```

#### 自动生成的 ZK 验证器（`gnark/` 目录）

由 Go 工具链通过 `ExportSolidity()` 自动生成，包含硬编码的验证密钥（VK）。**不要手动修改**，需要重新生成时运行对应 Go 命令。

- 命名规则：`{scheme}{CircuitType}Verifier{depth}.sol`（depth 仅 RO 证明有）
- 对外接口统一：`verifyProof(uint256[8] proof, uint256[N] input) returns (bool)`

### 12.3 Go ZKP 框架

#### 核心接口（`gnarkwrapper/wrapper.go`）

```go
// 所有电路必须实现的接口
type CircuitWrapper interface {
    frontend.Circuit          // gnark 电路标准接口（Define 约束）
    PreCompile(params ...interface{})  // 预编译钩子（设置电路结构参数，如 depth）
    Assign(curveName string, params ...interface{}) // 随机赋值（用于生成测试见证）
}

// ZKP 系统的统一操作接口
type GnarkWrapper interface {
    Compile()              // 编译电路 → 约束系统（CCS/SCS）
    Setup()                // trusted setup → 生成 pk/vk
    SetAssignment(frontend.Circuit)
    GenerateWitness(public bool)
    Prove()                // 生成证明
    Verify()               // 验证证明
    ExportSolidity(filePath string)  // 导出 Solidity 验证器合约
    GenSolProofParams() string       // 生成 Solidity 调用所需的 proof 参数字符串
    GenSolInputParams() string       // 生成 Solidity 调用所需的 input 参数字符串
    WriteCCS/ReadCCS/WritePK/ReadPK/WriteVK/ReadVK/WriteWitness/ReadWitness/WriteProof/ReadProof
    // ... 序列化/反序列化全套
}

// 工厂函数
func NewGnarkWrapper(scheme string, circuit frontend.Circuit, curve ecc.ID) GnarkWrapper
// scheme: "groth16" 或 "plonk"；curve 通常用 BN254
```

#### 电路实现

**`circuit_range_hash.go`（Range-Hash Proof）**
```go
type RangeHashProof struct {
    PreImage frontend.Variable           // 私有：原始值
    Hash     frontend.Variable `gnark:",public"`  // 公开：MiMC(PreImage)
    Min      frontend.Variable `gnark:",public"`  // 公开：约束下界
    Max      frontend.Variable `gnark:",public"`  // 公开：约束上界
}
// 约束：MiMC(PreImage) == Hash  &&  Min ≤ PreImage ≤ Max
```

**`circuit_root_obfuscation.go`（Root-Obfuscation Proof）**
```go
type RootObfuscationProof struct {
    Leaf   frontend.Variable `gnark:",public"` // 公开：叶节点哈希
    Path   []frontend.Variable                 // 私有：Merkle 路径（长度=depth）
    Root0..Root3 frontend.Variable `gnark:",public"` // 公开：4个候选根（其中1个是真实根）
    Index0, Index1 frontend.Variable            // 私有：真实根在4个中的索引（2bit）
}
// 约束：MiMC 路径计算结果 == Lookup2(Index0,Index1, Root0,Root1,Root2,Root3)
// PreCompile(depth int) 设置 Path 长度
// Assign 构造随机 Merkle 树并生成路径，另3个根随机生成
```

**工作流程（三段式，以 Range 为例）**
```bash
# 1. 生成阶段（编译电路 + setup + 生成见证）
./constraint-hash-proof range gen groth16,plonk 20
# 输出: output/{scheme}RangeHashProofCSS/PK/VK/Witness{0..19}

# 2. 证明阶段
./constraint-hash-proof range prove groth16 20
# 输出: output/{scheme}RangeHashProofProof{i} + PublicWitness{i}

# 3. 验证阶段
./constraint-hash-proof range verify groth16 20

# 4. 导出 Solidity
./constraint-hash-proof range sol groth16
# 输出: solidity/groth16RangeHashProofVerifier.sol + 打印 proof 参数字符串
```

#### Merkle 树（`merkletree/tree.go`）

```go
// 多叉 Merkle 树，使用 MiMC 哈希
func New(curveName string, MaxChildren, depth int) *Tree
tree.RandConstruct()                     // 随机构建树
tree.GenerateMerkleProof2(leafIndex int) // 生成叶节点的 Merkle 路径（兄弟节点哈希列表）
```

支持自定义哈希函数（`hash.Hash` 接口），当前所有电路使用 MiMC（BN254 曲线）。

#### `aggregate/` — 聚合证明（待实现）

`main.go` 当前为空，`circuit_multiple_root_obfuscation.go` 和 `circuit_multiple_substr_hash.go` 已有电路定义，但聚合驱动逻辑未实现。

### 12.4 关键依赖关系

```
Verify.sol
  └─ import Fund.sol（通过 IFund 接口调用 punish / claimLastPayment）
  └─ import gnark/groth16*Verifier.sol（硬编码当前使用 Groth16 + depth=20）

Fund.sol
  └─ import HashVerifier.sol

gnark/*Verifier.sol
  └─ 由 gnarkwrapper.ExportSolidity() 自动生成（Go 工具链驱动）

circuit_root_obfuscation.go
  └─ 依赖 merkletree.Tree（构造 Merkle 路径）
  └─ 依赖 mimchash.MiMCHash（计算叶节点/路径哈希）

circuit_*_hash.go
  └─ 依赖 mimchash.MiMCHash（约束内哈希计算）

所有 cmd/ 下代码
  └─ 依赖 gnarkwrapper.NewGnarkWrapper（统一 ZKP 入口）
```

### 12.5 实现时注意事项

1. **Go 模块名**：`go.mod` 中模块名为 `gnarkabc`，import 路径如 `gnarkabc/gnarkwrapper`，迁移时需统一修改

2. **Verify.sol 硬编码验证器**：当前直接 `import` 了 `groth16` + `depth=20` 的验证器，正式实现时应改为可配置（通过构造函数传入验证器地址）

3. **aggregate/ 的 main.go 为空**：聚合证明的驱动逻辑需从零实现，电路定义已有参考

4. **Solidity 验证器重新生成步骤**：
   ```bash
   cd snarks/gnarkzkp/cmd/constraint-hash-proof
   go run . range gen groth16       # 生成 CSS/PK/VK
   go run . range sol groth16       # 导出 Solidity 到 solidity/ 目录
   # 将生成的 .sol 文件复制到 foundry/src/gnark/
   ```

5. **测试文件位置**：`foundry/test/` 下的测试文件保留作参考，包含合约部署和验证器调用的完整示例
