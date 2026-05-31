# 高效验证的跨链状态证明方案实现文档

> 基于论文：*高效验证的跨链状态证明方案*
> 底层协议：Pianist（基于分布式 Plonk 的双变量多项式框架）

---

## 一、方案总览

### 1.1 核心问题

跨链状态证明需要目标链验证源链上某一状态的真实性与完整性。直接用 zk-SNARK（如 Plonk/Groth16）做跨链证明面临两个瓶颈：

1. **单证明验证开销过高**：链上单次验证消耗超过 60 万 Gas；证明大小约 27 个 G₁ 群元素，作为 calldata 传入验证合约直接转化为 Gas 消耗
2. **并发验证压力大**：跨链场景中同一时间窗口内存在大量并发状态证明请求，逐一验证导致链上资源压力随证明数量线性增长

### 1.2 两层优化方案

本方案在 Pianist 协议基础上提出两个独立机制：

| 机制 | 目标 | 效果 |
|------|------|------|
| **证明压缩（ProofCompress）** | 降低单个证明的大小和验证开销 | 证明大小减少 52.6%，Gas 节省 37.2% |
| **证明聚合（ProofAggregate）** | 将 K 个并发证明批量验证 | 链上配对运算次数从 O(K) 降至常数 4，典型场景 Gas 节省超 60% |

### 1.3 技术栈

- **底层框架**：Pianist 协议（分布式 Plonk，双变量多项式形式）
- **承诺方案**：KZG（DKZG，双变量扩展）
- **椭圆曲线**：BN254（Ethereum 原生预编译支持）
- **实现库**：gnark + gnark-crypto（Go 语言）
- **哈希函数**（电路内）：MiMC / Keccak-256（取决于应用场景）

---

## 二、底层协议：Pianist 概述

Pianist 将 Plonk 扩展为**双变量多项式**形式，支持分布式证明生成：

- **M 个子节点**：各持有规模为 T=N/M 的子电路 Cᵢ，在本地生成 X 轴相关多项式（见证多项式 `a(X), b(X), o(X)`，置换累加多项式 `z(X)`，商多项式 `h(X)`）
- **主节点 P₀**：聚合所有子节点的 X 轴计算结果，生成全局二元多项式，再完成 Y 轴证明

### 2.1 证明结构（原始 Pianist）

原始 Pianist 证明包含约 **27 个 G₁ 群元素 + 15 个域元素**，与电路规模 N 和子节点数 M 无关，但包含大量独立的开启证明（每个待开多项式在 X 和 Y 两个维度各需独立 DKZG 开启证明）。

### 2.2 Pianist 证明流程（三阶段）

```
阶段一：承诺（Commitment）
  子节点：生成本地多项式的 KZG 承诺，上报主节点

阶段二：挑战（Challenge）
  主节点：Fiat-Shamir 变换生成挑战 η, γ, λ
  子节点：生成置换累加多项式 Z(Y,X) 和 X 轴商多项式 Hₓ(Y,X) 的承诺

阶段三：开启（Opening）
  主节点：派生 X 轴挑战 α，各子节点在 α 处取值并批量聚合
  主节点：派生 Y 轴挑战 β，对 Y 方向多项式在 β 处生成 DKZG 第二层开启证明
  输出：见证承诺 + 置换承诺 + 商多项式承诺 + 两层开启证明 π
```

---

## 三、证明压缩（ProofCompress）

### 3.1 压缩目标

原始 Pianist 证明中包含大量独立 KZG 开启证明，每个待开多项式在 X 轴的 α 和 ωₓα 两个不同挑战点均需独立 DKZG 开启证明，导致上链证明中 G₁ 群元素约 27 个。

压缩目标：**将这些独立开启证明聚合，使上链群元素降至约 12 个**。

### 3.2 X 轴聚合（XAgg）

沿 X 轴维度，将多个挑战点处的多项式开启合并为单个商多项式承诺 `com_{Q_x}`：

对每个子节点 Pᵢ，构造聚合商多项式：

```
Q_{x,i}(X) = Σ_{S∈Sₐ} v^{idx(S)} · [S_i(X) - S_i(α)] / (X - α)
            + v^{idx(Z')} · [Z_i(X) - Z_i(ωₓα)] / (X - ωₓα)

com_{Q_x} = g^{R(τ_Y) · Q_{x,i}(τ_X)}
```

- **输入**：全局承诺 `com_A, com_B, com_O, com_Z, com_{Hₓ}`，挑战 η, γ, λ，子节点本地多项式
- **输出**：压缩证明 `π_c`

**X 轴聚合效果**：将针对 α 和 ωₓα 两个挑战点的多个开启证明压缩为单个商多项式承诺，**链上 G₁ 群元素从 27 个降至约 12 个**。

### 3.3 Y 轴折叠（YAgg）

沿 Y 轴维度，将多个一元多项式的独立开启折叠为单个聚合开启证明 `π_{1,agg}`：

```
G_Y(Y) = Σ_{P∈V_Y} μ^{idx(P)} · P(Y)     // 聚合多项式
Q_G(Y) = (G_Y(Y) - G_Y(β)) / (Y - β)      // 商多项式
π_{1,agg} = g^{Q_G(τ_Y)}
com_{G_Y} = g^{Q_G(τ_Y)}
```

### 3.4 ProofCompress 算法（Algorithm 1）

```
输入：全局承诺 com_A, com_B, com_O, com_Z, com_{Hₓ}
      挑战 η, γ, λ
      子节点 Pᵢ 持有本地多项式 aᵢ, bᵢ, oᵢ, zᵢ, hₓᵢ

输出：压缩证明 π_c

// 主节点 P₀ 执行：
1: α ← H(com_A, com_B, com_O, com_Z, com_{Hₓ})，ν ← H(α)，分发至各子节点

// 各子节点 Pᵢ（i=0,...,M-1）并行执行：
2: 构造 Q_{x,i}(X)，计算 KZG 承诺 com_{Q_x}
3: 将 {S_i(α)}_{S∈Sₐ}, Z_i(ωₓα), com_{Q_x} 发送至 P₀

// 主节点 P₀ 执行：
4: com_{Q_x} ← Π_{i=0}^{M-1} com_{Q_{x,i}}    // X 轴全局聚合
5: foreach S∈Sₐ: v_S(Y) ← Σ_{i=0}^{M-1} R_i(Y) · S_i(α)
6: v_Z(Y) ← Σ_{i=0}^{M-1} R_i(Y) · Z_i(ωₓα)
   H_x(Y,α) ← (G(Y,α) + λP₀(Y,α) + λ²P₁(Y,α) - V_x(α)H_x(Y,α)) / V_y(Y)
7: β ← H(α, ν, com_{Q_x})，μ ← H(β)
8: G_Y(Y) ← Σ_{P∈V_Y} μ^{idx(P)} · P(Y)
9: Q_G(Y) ← (G_Y(Y) - G_Y(β)) / (Y - β)
10: π_{1,agg} ← g^{Q_G(τ_Y)}，com_{G_Y} ← g^{Q_G(τ_Y)}
11: foreach S∈Sₐ: S(β,α) ← v_S(β)
12: Z(β,ωₓα) ← v_Z(β)；H_x(Y,α).Eval(β)
13: 返回 π_c ← ({com_A,com_B,com_O,com_Z,com_{Hₓ},com_{Q_x},com_{G_Y}, π_{1,agg},
                 {S(β,α)}_{S∈Sₐ}, Z(β,ωₓα), H_x(β,α)})
```

### 3.5 压缩证明的验证

验证者通过 Fiat-Shamir 重算挑战后检查：
- X 轴和 Y 轴的多点聚合将多个独立开启证明分别压缩为单个商多项式承诺和单个聚合开启证明
- 验证等式：`e(com_{Q_x}, [Z_T(τ_X)]₂) · e(π_{1,agg}, g^{β-τ_Y})^ρ = e(D_m, g₂) · e(D_r, g₂^{-1})`
- X 轴验证需 3 次配对，Y 轴需 2 次配对，两者仍为相互独立的验证等式

---

## 四、证明聚合（ProofAggregate）

### 4.1 聚合目标

跨链场景中同一时间窗口内存在 K 个并发状态证明请求，若逐一验证，链上 Gas 与 K 线性相关。

聚合目标：**将 K 个压缩证明合并为单个批量证明 `π_batch`，使链上配对运算次数降至常数 4 次**。

### 4.2 挑战共享机制

KZG 承诺加法同态性允许将多个承诺聚合为一个，但前提是所有 K 个证明必须**共享相同的挑战点 α 和 β**（由 DKZG 配对等式中的 G₂ 元素决定）。

**问题**：若直接公开挑战点，证明者可在生成承诺前预知挑战值，构造满足验证等式但不持有有效见证的恶意多项式，破坏知识可靠性。

**解决方案**：两轮交互挑战协商机制：

```
第一轮：
  各证明组 P^(k) 独立完成承诺阶段，将承诺 com_{Hₓ}^(k) 提交给协商主节点 P₀^(0)
  协商主节点将所有承诺作为 Fiat-Shamir 变换输入派生共享挑战 α，广播至所有证明组

第二轮：
  各证明组以共享 α 完成 X 轴聚合计算并将结果提交
  协商主节点再次绑定所有 X 轴聚合结果派生共享挑战 β 并广播

挑战确认后：
  协商主节点由 Fiat-Shamir 变换对全部压缩证明派生随机聚合系数 {rₖ}_{k=0}^{K-1}
  对 K 个压缩证明的 X 轴承诺、Y 轴聚合开启证明及相关线性组合量分别做随机线性组合
  得到批量验证所需的聚合量，输出批量证明 π_batch
```

由于两轮共享挑战的派生均绑定了所有证明组已提交的承诺信息，任何证明组都无法在提交承诺之前预知挑战值，从而在保证挑战点统一性的同时维护了知识可靠性。

### 4.3 ProofAggregate 算法（Algorithm 2）

```
输入：K 个证明组 {P^(k)}_{k=0}^{K-1}，每组持有相同源链至目标链的子电路实例
输出：批量证明 π_batch

// 各证明组 P^(k)（k=0,...,K-1）并行执行：
1: com_{Hₓ}^(k) ← ProofCompress.Commit(P^(k))
   将 com_{Hₓ}^(k) 发送至协商主节点 P₀^(0)

// 协商主节点 P₀^(0) 执行：
2: α ← H(pp, com_{Hₓ}^(0), ..., com_{Hₓ}^(K-1))，将 α 广播至所有证明组

// 各证明组 P^(k) 并行执行：
3: v^(k) ← H(α, k)
   (com_{Q_x}^(k), V_x^(k)) ← ProofCompress.XAgg(P^(k), α, v^(k))
   将 (com_{Q_x}^(k), V_x^(k)) 发送至 P₀^(0)

// 协商主节点 P₀^(0) 执行：
4: β ← H(α, com_{Q_x}^(0), V_x^(0), ..., com_{Q_x}^(K-1), V_x^(K-1))
   将 β 广播至所有证明组

// 各证明组 P^(k) 并行执行：
5: μ^(k) ← H(β, k)
   π_c^(k) ← ProofCompress.YAgg(P^(k), β, μ^(k))
   将 π_c^(k) 发送至 P₀^(0)

// 协商主节点 P₀^(0) 执行：
6: 返回 π_batch ← {π_c^(k)}_{k=0}^{K-1}
```

### 4.4 批量证明验证

验证者：
1. 通过 Fiat-Shamir 重算所有挑战 α, β，以及各证明组的随机聚合系数 {rₖ}
2. 利用聚合系数 rₖ 对 K 个压缩证明的商多项式承诺、Y 轴聚合开启证明及相关线性组合量进行加权求和，构造聚合验证量
3. 代入单个四配对等式完成批量验证

**关键性能收益**：配对运算次数固定为常数 **4 次**，与证明数量 K 无关。

---

## 五、安全性

### 5.1 三大安全性质

| 性质 | 描述 |
|------|------|
| **完备性** | 对任意有效电路-见证对，诚实证明者生成的批量证明 `π_batch` 能以概率 1 通过验证 |
| **知识可靠性** | 代数群模型 + 随机预言机模型下，假设 q-DLOG 困难，攻击者无法伪造有效批量证明；伪造成功概率 ≤ ε_DLOG + (K+1)d/|𝔽| |
| **零知识性** | 随机预言机模型下，存在概率多项式时间模拟器 S，其生成的证明视图与真实证明者-验证者交互视图计算不可区分 |

### 5.2 安全假设

- 底层 Pianist 协议的安全性
- **q-DLOG 困难假设**（不引入额外安全假设）
- 代数群模型（AGM）
- 随机预言机模型（ROM）

### 5.3 零知识性保证关键点

- 两轮共享挑战中，Fiat-Shamir 变换绑定所有已提交承诺，证明者无法预知挑战
- 聚合操作仅涉及公开承诺和开启证明的线性组合，不引入额外见证信息
- 模拟器可独立随机选取共享挑战 α, β ∈ 𝔽，以及聚合随机数 ν, μ, {rₖ} ∈ 𝔽，通过 KZG 的多项式插值反推构造满足配对等式的承诺和开启证明

---

## 六、性能数据

### 6.1 实验环境

- 操作系统：Debian Linux 6.6.87.2
- 处理器：Intel Core i5-12400F
- 内存：32 GB RAM，1 TB 固态硬盘
- 测试电路：Keccak-256 哈希函数电路（验证 64 字节消息的 Keccak-256 哈希计算正确性）
  - 以太坊 MPT 状态树节点哈希核心算法，代表典型跨链状态证明场景
  - 对 Keccak-f[1600] 置换的 24 轮迭代展开，约 **30 万个 Plonk 门约束**
- 系统部署：2 个子节点，总电路规模 2²¹

### 6.2 证明压缩性能（vs Pianist 基线）

| 指标 | Pianist | 本方案 | 变化 |
|------|---------|--------|------|
| 证明大小（字节） | 1824 | **864** | -52.6% |
| 证明生成时间（ms） | 7871 | **4580** | -41.8%（1.38x 加速） |
| 证明验证时间（ms） | 2.201 | **2.641** | +20%（需额外 2 次 G₂ 配对） |
| 链上验证 Gas | 627254 | **394068** | -37.2% |

**证明大小说明**：
- X 轴聚合将针对 α 和 ωₓα 两个挑战点的多个开启证明压缩为单个商多项式承诺 `com_{Q_x}`
- Y 轴聚合将 7 个独立一元多项式开启证明折叠为单个 `π_{1,agg}`
- 上链 G₁ 群元素从 27 个降至约 12 个

**生成时间说明**：加速来源于 X 轴 Shplonk 聚合，将 14 个 KZG 开启证明的 MSM 运算约简为单次 MSM（子节点 MSM 运算量减少约 93%）。

**验证时间说明**：略有增加（+20%），因为需要额外计算两个 G₂ 标量乘法；而 Pianist 直接从 SRS 中读取对应 G₂ 元素。

### 6.3 批量证明聚合性能（K 个证明并发，vs Pianist 逐一验证）

| K 值 | Gas 节省率 | 证明大小节省率 | 生成时间加速比 | 验证时间加速比 |
|------|-----------|---------------|---------------|---------------|
| K=1 | 23.1% | 52.6% | 1.38x | ~1.0x |
| K=10 | 62.7% | ~52.5% | 1.38x（恒定）| ~1.15x |
| K=20 | 63.8%（峰值）| ~52.5% | 1.38x | 峰值约 1.24x |
| K=100 | 55.4% | ~52.5% | 1.38x | 趋于稳定 |

**Gas 节省说明**：
- 批量证明的总验证 Gas = 常数配对开销 + K×864 字节 calldata 开销（线性增长，但斜率远小于逐一验证）
- K=20 附近达到峰值节省，之后 calldata 线性项逐渐主导，节省率缓慢回落趋近纯压缩率 52.6%

**证明生成时间说明**：
- K 个证明相互独立并行生成，总时间与 K 线性相关，加速比恒定为 1.38 倍

**验证时间说明**：
- 配对次数固定为 4 次（常数），K ≥ 20 后加速比稳定，不再持续增长（MSM 计算 O(K)G 开销逐渐主导）

### 6.4 批量证明大小

```
单压缩证明大小：864 字节（固定）
批量证明大小：(11K + 2)|G₁| + 7K|𝔽|（与 K 线性相关）
以 BN254 曲线、K=10 为例：批量证明大小约 5824 字节
```

---

## 七、系统架构与实现选型

### 7.1 组件架构

```
证明者侧（分布式）：
  ┌─────────────────────────────────────────┐
  │  协商主节点 P₀^(0)                        │
  │    - 挑战协商（两轮 Fiat-Shamir）           │
  │    - X 轴全局聚合（KZG 加法同态）           │
  │    - Y 轴折叠与输出批量证明                 │
  └──────────────┬──────────────────────────┘
                 │ 广播挑战 / 收集承诺
  ┌──────────────┼──────────────────────────┐
  │ 子节点 P₁    │ 子节点 P₂  ...  子节点 Pₘ  │
  │ - 本地电路   │ - 本地电路               │
  │ - X 轴承诺   │ - X 轴承诺               │
  │ - 并行 XAgg  │ - 并行 XAgg              │
  └─────────────────────────────────────────┘

验证者侧（链上）：
  ┌─────────────────────────────────────────┐
  │  验证合约（Solidity）                     │
  │    - 接收 π_batch（calldata）              │
  │    - Fiat-Shamir 重算挑战                  │
  │    - 执行 4 次配对验证                     │
  └─────────────────────────────────────────┘
```

### 7.2 技术选型

| 组件 | 选型 | 说明 |
|------|------|------|
| ZK 框架 | gnark + gnark-crypto | Go 实现，支持 Groth16 和 Plonk |
| ZK 协议 | **Plonk（Pianist 分布式扩展）** | 双变量多项式，支持分布式子节点 |
| 椭圆曲线 | **BN254** | Ethereum 原生预编译，约 128 位安全性 |
| 承诺方案 | **KZG（DKZG 双变量扩展）** | 支持加法同态聚合 |
| 链上验证 | Solidity + EVM 预编译配对 | `ecPairing` 预编译合约 |
| 测试框架 | gnark benchmark | 证明生成/验证时间测量 |
| 挑战生成 | Fiat-Shamir（SHA256/Poseidon） | 随机预言机实例化 |

---

## 八、实现关键点与注意事项

### 8.1 X 轴多点聚合（Shplonk 优化）

Shplonk 将多个 KZG 开启证明在不同挑战点处的 MSM 运算约简为单次 MSM：
- 原 Pianist：每个子节点需要执行 **13 次 MSM**（α 和 ωₓα 处各开启多个多项式）
- 本方案：每个子节点只需 **1 次 MSM**，减少约 **93% 的 MSM 运算量**
- 整体加速约 1.38 倍（其他开销如 FFT 占比导致总加速不超过此值）

### 8.2 两轮挑战协商的网络开销

- 第一轮：各证明组向协商主节点发送 1 个 G₁ 元素（承诺 `com_{Hₓ}`），约 **K×32 字节**
- 协商主节点广播 α：约 32 字节
- 第二轮：各证明组发送 X 轴聚合结果，协商主节点广播 β
- 以 BN254、K=10 为例，挑战协商总通信量约 **640 字节**，远小于子节点到主节点的数据通信量，实际部署可接受

### 8.3 证明验证时间略有增加的原因

本方案验证需额外计算 **2 个 G₂ 标量乘法**（聚合商多项式对应的 G₂ 元素），而 Pianist 直接从 SRS（Structured Reference String）中读取预计算的 G₂ 元素。这导致验证时间约增加 20%（2.641ms vs 2.201ms）。

**权衡**：验证时间仅增加约 0.44ms（链下可忽略），而 Gas（链上主要成本）节省 37.2%，整体收益显著。

### 8.4 Y 轴折叠由主节点集中执行的限制

证明压缩中 Y 轴折叠步骤由主节点集中执行，当子节点数 M 极大时，主节点在证明生成过程中承担较多协商工作，可能成为性能瓶颈。

### 8.5 批量聚合的跨机构部署复杂性

证明聚合机制要求 K 个证明组共享相同挑战 α 和 β，需要额外的两轮协商通信。在跨机构分布式场景中增加了部署复杂性，需要可信或去中心化的协商主节点。

### 8.6 当前基于可信设置

本方案基于可信设置（KZG SRS），如何在**透明设置**下实现同等的验证效率，是后续值得探索的方向。

---

## 九、实现优先级建议

1. **Pianist 基础实现**：基于 gnark 实现双变量 Plonk 电路，支持 M 个子节点分布式证明生成，验证完整的三阶段流程（承诺→挑战→开启）

2. **ProofCompress 实现**：
   - X 轴 Shplonk 聚合（`XAgg`）：将多个挑战点处的 KZG 开启合并为单个商多项式承诺
   - Y 轴折叠（`YAgg`）：将多个一元多项式开启折叠为单个聚合开启证明
   - 输出压缩证明 `π_c`（约 864 字节）

3. **链上验证合约（Solidity）**：实现压缩证明的 4 次配对验证逻辑，接受 calldata 格式的 `π_c`

4. **ProofAggregate 实现**：
   - 两轮挑战协商协议（协商主节点 + K 个证明组的交互）
   - 随机线性组合聚合 K 个压缩证明
   - 输出批量证明 `π_batch`

5. **批量验证合约**：实现对 `π_batch` 的单次 4 配对批量验证，验证 K 个并发跨链状态证明

6. **性能测试**：针对 K=1,10,20,50,100 的场景测试证明大小、生成时间、验证时间和链上 Gas 消耗

---

## 十、参考代码结构（`references/efficient_cross_chain_proof_code/`）

> 已有完整原型实现（项目名 BPiano），路径根为 `references/efficient_cross_chain_proof_code/`。Go 模块名为 `github.com/oliverustc/bpiano`。

### 10.1 整体目录结构

```
efficient_cross_chain_proof_code/
├── doc/                            # 技术文档（实现时重要参考）
│   ├── paper.md                    # 论文全文（数学方案完整描述）
│   ├── bpiano-ref.md               # BPiano 协议精简数学参考手册
│   ├── pianist-ref.md              # Pianist 底层协议数学参考
│   ├── dkzg.md                     # 分布式 KZG 承诺方案设计说明
│   ├── piano.md                    # Piano 协议（分布式 Plonk）设计说明
│   ├── agg.md                      # 聚合算法详细说明
│   ├── solidity-verifier.md        # Solidity 验证器设计与实现
│   ├── algorithm-additions.md      # 额外算法与实现细节
│   ├── bench.md                    # 基准测试运行指南
│   └── pianist.pdf                 # Pianist 原始论文（底层基础）
│
├── plot/                           # 论文图表生成
│   ├── compress_performance_plot.py
│   ├── aggregation_plot.py
│   ├── sync_data.sh
│   ├── data/                       # 基准测试输入 CSV
│   └── figures/                    # 输出 PNG 图片
│
└── bpiano/                         # Go 实现（核心）
    ├── dkzg/                       # 分布式 KZG 承诺方案
    │   ├── srs.go                  # SRS 生成（Lagrange 形式）
    │   ├── commit.go               # CommitLocal / AggregateDigests / CommitGlobal
    │   ├── open.go                 # LocalOpenX / AggregateOpenX / OpenY
    │   ├── verify.go               # 2-pairing 验证
    │   ├── batch.go                # 批量验证与证明准备
    │   └── *_test.go               # 25 个单元测试
    │
    ├── piano/                      # Piano 协议（分布式 Plonk）
    │   ├── keys.go                 # ProvingKey / VerifyingKey 结构
    │   ├── setup.go                # SetupWithTrapdoors
    │   ├── prove.go                # Prove（生成单个 Piano 证明）
    │   ├── verify.go               # Verify
    │   ├── utils.go                # Permutation / 置换多项式工具
    │   ├── export.go               # CircuitInfo 接口定义
    │   └── piano_test.go           # 5 个正确性测试
    │
    ├── bpiano/                     # BPiano 核心（论文主要贡献）
    │   ├── compress.go             # Compress：M 个 Piano 证明 → 1 个压缩证明
    │   ├── aggregate.go            # CoordinateChallenges / AggregateProofs
    │   ├── verify.go               # VerifyCompressed / VerifyBatch
    │   ├── stages.go               # 多阶段压缩流程（Stage1/2/3）
    │   ├── bpiano_test.go          # 正确性测试
    │   └── debug_test.go           # 调试中间值打印（可删）
    │
    ├── circuit/
    │   └── builder.go              # PLONK 电路构建器（SparseR1CS → piano.CircuitInfo）
    │
    ├── solgen/                     # Go → Solidity calldata 适配层
    │   ├── encode.go               # G1/G2/Fr 序列化为 EVM 格式
    │   ├── bpiano_calldata.go      # GenerateBPianoCalldata（FS 重放 + G2 预计算）
    │   ├── piano_calldata.go       # Piano 证明 → calldata
    │   ├── agg_calldata.go         # 聚合证明 → calldata
    │   ├── vk_calldata.go          # 验证密钥提取
    │   ├── abi_calldata.go         # ABI 编码
    │   └── *_test.go               # 单元测试 + Forge 集成测试
    │
    ├── keccak/                     # Keccak-256 基准电路（性能对比用）
    │   ├── circuit.go              # 纯 Go 实现
    │   ├── gnark_circuit.go        # gnark 约束电路定义
    │   └── *_test.go
    │
    ├── mpt/
    │   └── mpt.go                  # Merkle Patricia Trie 工具函数
    │
    ├── bench/                      # 基准测试（性能参考，非核心逻辑）
    │   ├── bench_test.go           # Piano vs BPiano 性能对比
    │   ├── bench_unified_test.go   # 统一基准测试套件（主入口）
    │   ├── save_load_test.go       # 证明序列化测试
    │   └── testdata/               # 预生成的二进制测试 fixture
    │
    ├── cmd/
    │   └── bench_gnark_plonk/main.go  # gnark 原生 Plonk 基准对照组
    │
    ├── sol/                        # Solidity 合约层（Foundry 项目）
    │   ├── src/
    │   │   ├── BPianoVerifier.sol      # 核心：BPiano 压缩证明链上验证器
    │   │   ├── AggBPianoVerifier.sol   # 核心：聚合证明验证器
    │   │   ├── PianoVerifier.sol       # Piano 原始证明验证器（对照）
    │   │   ├── BPianoVerifierGen.sol   # 生成的 BPiano 验证器变体
    │   │   ├── PianoVerifierGen.sol    # 生成的 Piano 验证器变体
    │   │   └── Pairing.sol             # BN254 配对库（EVM 预编译包装）
    │   └── test/
    │       ├── BPianoVerifierTest.t.sol
    │       ├── BPianoVerifierPITest.t.sol   # 含公开输入的验证测试
    │       ├── AggBPianoVerifierTest.t.sol
    │       ├── PianoVerifierTest.t.sol
    │       ├── *Gen*.t.sol
    │       ├── fixture_*.json          # 各 K 值的测试 fixture
    │       └── *.hex                   # calldata 十六进制样本
    │
    ├── go.mod                      # 模块：github.com/oliverustc/bpiano
    └── go.sum
```

### 10.2 核心数据结构

#### DKZG 层（`dkzg/`）

```go
// 双变量多项式对 F(Y,X) 的承诺，是 G1 中的一个元素
type Digest = bn254.G1Affine

// X 轴开放证明（子节点生成）
type OpeningProofX struct {
    ClaimedValue fr.Element   // v_i = f_i(alpha)
    H            bn254.G1Affine // 商多项式承诺 π_{0,i}
}

// Y 轴开放证明（主节点生成）
type OpeningProofY struct {
    ClaimedValue fr.Element   // z = F(beta, alpha)
    H            bn254.G1Affine // 商多项式承诺 π_{1,F}
    ComVF        bn254.G1Affine // Y 轴多项式 V_F(Y) 的承诺
}

// 关键函数
CommitLocal(nodeIdx, evals, srs)     // 子节点本地承诺
AggregateDigests(localDigests)       // 聚合为全局承诺
LocalOpenX(nodeIdx, evals, alpha, srs) // 子节点 X 轴开放
AggregateOpenX(proofs, alpha, srs)   // 聚合 X 轴开放
OpenY(comF, vfEvals, beta, srs)      // 主节点 Y 轴开放
```

#### BPiano 压缩证明（`bpiano/compress.go`）

```go
// CompressedProof：相比 Piano 原始证明的变化：
//   - BatchedProofX（H + 13 个 ClaimedDigests）+ ZShiftedProofX
//     → ComQX（1 个 G1，Shplonk X 轴聚合商承诺）
//   - Hy[3] + BatchedProofY（H + 15 个 ClaimedValues）
//     → ComGY + Pi1AggH（各 1 个 G1）+ 7 个标量
// 总大小：10 个 G1 + 15 个 fr.Element ≈ 544 字节（BN254）
type CompressedProof struct {
    LRO    [3]dkzg.Digest  // com_A, com_B, com_O
    Z      dkzg.Digest     // com_Z
    Hx     [3]dkzg.Digest  // com_{H_X,0..2}
    ComQX  dkzg.Digest     // Shplonk X 轴聚合商承诺
    ComVFAlpha, ComVFZS dkzg.Digest  // witness Y 轴承诺
    ComGY  dkzg.Digest     // G_Y 聚合承诺
    Pi1AggH dkzg.Digest    // Y 轴开放商承诺
    // 7 个求值标量
    EvalA, EvalB, EvalO, EvalZ, EvalZS, EvalHx, EvalHy fr.Element
    // 8 个共享多项式求值（选择子 + 置换多项式）
    EvalQl, EvalQr, EvalQm, EvalQo, EvalQk fr.Element
    EvalS1, EvalS2, EvalS3 fr.Element
}
```

#### BPiano 批量证明（`bpiano/aggregate.go`）

```go
// AggregatedProof：K 个共享 α/β 的压缩证明聚合结果
type AggregatedProof struct {
    K      int
    Proofs []*CompressedProof  // K 份压缩证明
    // §4.3.2 定义的两个聚合 KZG 证明
    ComQXTotal dkzg.Digest  // Σ r_k · ComQX^(k)
    Pi1Total   dkzg.Digest  // Σ r_k · Pi1AggH^(k)
}

// 两轮挑战协调（生成 K 个共享 α/β 的压缩证明）
func CoordinateChallenges(pks, witnessSlices, publicInputs) ([]*CompressedProof, error)
// 聚合 K 个证明
func AggregateProofs(proofs []*CompressedProof, vk) (*AggregatedProof, error)
// 批量验证（常数 4 次配对）
func VerifyBatch(proof *AggregatedProof, vk, publicInputs) error
```

#### Solidity Calldata（`solgen/bpiano_calldata.go`）

```go
// BPianoCalldata：传入 BPianoVerifier.sol 的全部输入
type BPianoCalldata struct {
    // 证明 G1 承诺（64 字节/个，EVM 非压缩格式）
    LRO [3][64]byte; Z, ComQX, ComVFAlpha, ComVFZS, ComGY, Pi1AggH [64]byte
    Hx [3][64]byte
    // 证明标量求值（32 字节/个，大端序）
    EvalA, EvalB, ..., EvalS3 [32]byte
    // 公开输入
    PublicInputs [][32]byte
    // Go 侧预计算的 G2 点（EVM 无 G2 标量乘预编译）
    ZTG2       [128]byte  // [Z_T(τ_X)]₂
    TauYBetaG2 [128]byte  // [τ_Y - β]₂
    // 验证密钥（内嵌）
    VK VKCalldata
}

// 核心函数：重放 Fiat-Shamir + 预计算 G2 点 → 生成 calldata
func GenerateBPianoCalldata(proof *bpiano.CompressedProof, vk *piano.VerifyingKey,
    publicInputs [][]fr.Element) (*BPianoCalldata, error)
```

### 10.3 链上验证合约（`sol/src/BPianoVerifier.sol`）

验证流程（对应 `bpiano/verify.go`）：

```
1. 重放 Fiat-Shamir 转录（SHA-256），推导 7 个挑战：γ, η, λ, α, ν, β, μ
2. 代数约束检验（纯域运算，验证置换和选择子多项式的约束等式）
3. 推导随机挑战 ρ
4. 构建 4 个 G1 点（ecMul + ecAdd）
5. 4 次配对检验（ecPairing 预编译）
```

**关键设计**：`ZTG2 = [Z_T(τ_X)]₂` 和 `TauYBetaG2 = [τ_Y - β]₂` 两个 G2 点由 Go 侧（`solgen`）链下预计算后作为 calldata 传入，因为 EVM 缺少 G2 标量乘预编译。

### 10.4 关键依赖关系

```
BPianoVerifier.sol
  └─ import Pairing.sol（BN254 ecPairing/ecMul/ecAdd 包装）
  └─ calldata 由 solgen.GenerateBPianoCalldata() 生成

bpiano.Compress()
  └─ 调用 piano.Prove() 获取 witness 多项式
  └─ 调用 dkzg.LocalOpenX/AggregateOpenX 执行 Shplonk 聚合

bpiano.CoordinateChallenges()
  └─ 内部分 3 阶段（stages.go）协调 K 个证明的共享挑战

circuit.Builder
  └─ 将 PLONK gates 转换为 piano.CircuitInfo（对接 gnark SparseR1CS）

keccak.GnarkCircuit
  └─ 基准测试电路（通过 circuit.Builder 转换为 Piano 格式）
```

### 10.5 Go 模块与使用方式

**模块名**：`github.com/oliverustc/bpiano`，本地使用时需在 `go.mod` 中用 `replace` 指令指向本地路径。

**典型调用链**（单证明压缩）：
```go
// 1. Setup
srs, _ := dkzg.NewSRS(T, M, trapdoors)
pk, vk, _ := piano.SetupWithTrapdoors(circuit, srs, trapdoors)

// 2. 生成 Piano 证明（M 个 witness 实例）
pianoProof, _ := piano.Prove(pk, witnesses, publicInputs)

// 3. 压缩为 BPiano 证明
compressed, _ := bpiano.Compress(pk, witnesses, publicInputs)

// 4. 链下验证
bpiano.VerifyCompressed(compressed, vk, publicInputs)

// 5. 生成 Solidity calldata
calldata, _ := solgen.GenerateBPianoCalldata(compressed, vk, publicInputs)
// 调用 BPianoVerifier.sol 的 verify 函数
```

**批量聚合调用链**（K 个证明）：
```go
// 两轮挑战协调（生成 K 个共享 α/β 的压缩证明）
proofs, _ := bpiano.CoordinateChallenges(pks, witnessSlices, publicInputs)

// 聚合
agg, _ := bpiano.AggregateProofs(proofs, vk)

// 批量验证（4 次配对，与 K 无关）
bpiano.VerifyBatch(agg, vk, publicInputs)

// 生成聚合 calldata
solgen.GenerateAggCalldata(agg, vk, publicInputs)
```

### 10.6 实现注意事项

1. **模块路径替换**：迁移时将 `github.com/oliverustc/bpiano` 替换为项目内部路径，在 `go.mod` 中添加 `replace` 指令

2. **G2 标量乘限制**：EVM 无 G2 标量乘预编译，`ZTG2` 和 `TauYBetaG2` **必须**由 Go 侧（`solgen.GenerateBPianoCalldata`）预计算后传入 Solidity，这是与普通 Groth16 验证器的主要区别

3. **Fiat-Shamir 一致性**：Go（`bpiano/verify.go`）和 Solidity（`BPianoVerifier.sol`）必须使用完全相同的 SHA-256 转录顺序和挑战绑定方式，否则链上验证将失败

4. **SRS 规模**：SRS 大小由 `T`（X 轴，子节点电路大小）和 `M`（Y 轴，子节点数）决定，Keccak-256 基准测试中 `T=2^21/M`，生产环境需根据电路规模调整

5. **基准测试入口**：`bench/bench_unified_test.go` 是主基准入口，运行方式：
   ```bash
   cd bpiano && go test ./bench/ -bench=. -benchtime=1x -timeout=60m
   ```

6. **`debug_test.go`**：`bpiano/debug_test.go` 是打印中间值的调试文件，不影响正确性，可在生产版本中删除

7. **与跨链场景集成**：与 FishboneChain 子链集成，将子链的 Merkle 状态根作为跨链状态证明的目标，使用本方案高效向目标链证明子链状态有效性
