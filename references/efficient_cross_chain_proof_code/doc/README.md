# BPiano：基于 Pianist 的批量证明压缩方案

> 论文实验实现文档
> Go 模块：`github.com/oliverustc/bpiano`

---

## 一、背景与目标

本项目基于论文 [paper.md](paper.md)（以及 [pianist.pdf](pianist.pdf) 作为底层协议参考），
实现了一套完整的 **BPiano（Batched Piano）** 零知识证明系统，包括：

1. **DKZG** —— 二元多项式 KZG 承诺方案（从零实现）
2. **Piano** —— 数据并行的单电路 Plonk 协议（基于 Pianist 论文，用新版 gnark 重写）
3. **BPiano** —— 证明压缩与批量聚合协议（论文核心贡献）
4. **Solidity 链上验证器** —— 将 BPiano 压缩证明提交至 EVM 合约验证

**核心指标（论文声明）：**
- 单个压缩证明：验证配对次数为 **O(1)**，与子节点数 M 无关
- 批量 K 个证明的聚合验证：配对次数仍为 **O(1)**，与 K 无关

---

## 二、代码结构

```
bpiano/                          # Go 主模块
├── go.mod                       # module github.com/oliverustc/bpiano
│
├── dkzg/                        # 双变量 KZG 承诺方案
│   ├── srs.go                   # SRS 生成（Lagrange 形式）
│   ├── commit.go                # CommitLocal、AggregateDigests、CommitGlobal
│   ├── open.go                  # LocalOpenX、AggregateOpenX、OpenY
│   ├── verify.go                # Verify（2-pairing 验证）
│   ├── batch.go                 # BatchVerify、PrepareProof
│   └── *_test.go                # 25 个测试（全通过）
│
├── piano/                       # Pianist 协议（数据并行版）
│   ├── keys.go                  # ProvingKey、VerifyingKey 结构体
│   ├── setup.go                 # SetupWithTrapdoors（生成 PK/VK）
│   ├── prove.go                 # Prove（生成 Piano 单证明）
│   ├── verify.go                # Verify（验证 Piano 证明）
│   ├── utils.go                 # Permutation、BuildPermutation 等工具函数
│   ├── export.go                # CircuitInfo 接口
│   └── piano_test.go            # 5 个测试（全通过）
│
├── bpiano/                      # BPiano 核心协议
│   ├── compress.go              # Compress：压缩 M 个 Piano 证明为一个
│   ├── aggregate.go             # Aggregate：聚合 K 个压缩证明
│   ├── verify.go                # VerifyCompressed、VerifyAggregated
│   ├── bpiano_test.go           # 正确性测试
│   └── debug_test.go            # 调试用中间值打印
│
├── keccak/                      # Keccak-256 基准电路
│   ├── circuit.go               # 纯 Go Keccak-256 实现（用于生成见证）
│   ├── gnark_circuit.go         # Gnark 约束电路定义
│   ├── keccak.go                # 辅助函数
│   └── *_test.go                # 正确性测试
│
├── circuit/                     # 通用电路构建器
│   └── builder.go               # 将 gnark SparseR1CS 转为 Piano CircuitInfo
│
├── bench/                       # 基准测试
│   └── bench_test.go            # Piano vs BPiano 证明大小/时间/配对次数对比
│
├── solgen/                      # Solidity calldata 生成（Go 端）
│   ├── encode.go                # G1/G2/Fr 序列化为 EVM 格式
│   ├── bpiano_calldata.go       # GenerateBPianoCalldata：重放 FS、预计算 G2 点
│   ├── vk_calldata.go           # ExtractVKSolidity：VK 字段提取
│   └── *_test.go                # 单元测试 + Forge 端到端测试
│
├── sol/                         # Solidity 链上验证合约（Foundry 项目）
│   ├── src/
│   │   ├── Pairing.sol          # BN254 配对库（封装 EVM 预编译）
│   │   └── BPianoVerifier.sol   # BPiano 压缩证明链上验证合约
│   └── test/
│       ├── BPianoVerifierTest.t.sol    # Forge 测试（无公开输入）
│       └── BPianoVerifierPITest.t.sol  # Forge 测试（含公开输入）
│
├── cmd/
│   └── bench_gnark_plonk/       # 对照组：gnark 原生 Plonk 基准
│
└── mpt/                         # Merkle Patricia Trie（辅助模块）
    └── mpt.go
```

---

## 三、各模块设计说明

每个模块有独立的设计说明文档，详述数学基础、实现与旧版代码的差异，以及关键决策：

| 模块 | 文档 | 状态 |
|------|------|------|
| DKZG | [dkzg.md](dkzg.md) | ✅ 完成，25 个测试通过 |
| Piano | [piano.md](piano.md) | ✅ 完成，5 个测试通过 |
| BPiano | [bpiano-ref.md](bpiano-ref.md)（协议参考） | ✅ 完成，全测试通过 |
| Solidity 验证器 | [solidity-verifier.md](solidity-verifier.md) | 🔧 合约已实现，端到端测试待运行 |

**底层协议参考：**
- [bpiano-ref.md](bpiano-ref.md) —— BPiano 方案精简数学参考
- [pianist-ref.md](pianist-ref.md) —— Pianist 底层协议数学参考

---

## 四、依赖关系

```
gnark-crypto  ──→  dkzg
                     ↓
gnark ─────────→  piano  ──→  bpiano  ──→  solgen  ──→  sol
                     ↓                         ↓
                  circuit                   keccak
                     ↓
                   bench
```

**外部依赖（go.mod）：**
- `github.com/consensys/gnark v0.14.0`
- `github.com/consensys/gnark-crypto v0.19.2`

---

## 五、快速上手

### 5.1 环境搭建

#### 系统要求

| 工具 | 版本 | 安装方式 |
|------|------|---------|
| Go | ≥ 1.21 | [go.dev/dl](https://go.dev/dl/) |
| git | 任意 | 系统包管理器 |
| Foundry (`forge`) | 最新稳定版 | 见下 |

#### 安装 Foundry

```bash
curl -L https://foundry.paradigm.xyz | bash
foundryup
forge --version   # 确认安装成功
```

#### 克隆仓库

```bash
git clone <repo-url> paper
cd paper/bpiano
```

本项目使用本地替换的 `gnark-crypto`（`go.mod` 中有 `replace` 指令指向 `../gnark-crypto`），
克隆后目录结构须保持 `paper/bpiano/` 与 `paper/gnark-crypto/` 平级。

#### 下载 Go 依赖

```bash
cd bpiano
go mod download
```

#### 构建 Solidity 合约

```bash
cd sol
forge build
cd ..
```

---

### 5.2 运行正确性测试

```bash
cd bpiano

# DKZG（25 个测试）
go test ./dkzg/... -v

# Piano（5 个测试）
go test ./piano/... -v

# BPiano 核心（压缩 + 聚合 + 验证）
go test ./bpiano/... -v

# Keccak-256 基准电路
go test ./keccak/... -v

# Solidity calldata 编码
go test ./solgen/... -run TestEncode -v
```

---

### 5.3 运行基准测试（一条命令）

```bash
go test ./bench/ -v -run TestBench -timeout 300m
```

自动完成：单证明压缩对比 → Solidity Gas 测试 → 聚合验证对比 → 聚合 Gas 测试，
结果写入 `bench/results/` 目录（4 个 CSV，含时间戳）。

详见 [bench.md](bench.md)。

---

### 5.4 端到端链上验证

```bash
# 单压缩证明 → Forge
go test ./solgen/... -run TestForgeE2E -v -timeout 30m

# 聚合证明（所有 K 值）→ Forge
go test ./solgen/... -run TestForgeE2E_Agg_AllK -v -timeout 30m
```

---

## 六、核心实验结果

> 完整数据见 `bench/results/`，绘图脚本见 `plot/`。

### 6.1 证明压缩效果（单证明，K=1）

以 Keccak-256 电路（T≈2¹⁸，M=2）为基准，Piano vs BPiano 单证明对比：

| 指标 | Piano（基准） | BPiano（本方案） | 变化 |
|---|---|---|---|
| 证明大小 | 1824 字节 | 864 字节 | **−52.6%** |
| 证明/压缩时间 | 7871 ms | 4580 ms | **−41.8%** |
| 验证时间 | 2.20 ms | 2.64 ms | +20%（见注） |
| 链上验证 Gas | 627,254 | 394,068 | **−37.2%** |

> 注：BPiano 单证明验证需额外计算 2 次 G2 标量乘（构造 [Z_T(τ_X)]₂ 和 [τ_Y-β]₂），
> 这是将 X/Y 轴验证合并为单次 4-pairing 的必要代价，属算法层面的固有开销。

![证明压缩效果](../plot/figures/compress_performance.png)

---

### 6.2 聚合验证效果（K 个证明）

**证明大小 & 验证时间**（以 Keccak-256 电路为基准）：

BPiano 聚合证明始终约为 Piano 累积大小的 **47–53%**；
验证时间在 K≥10 后 BPiano 持续优于 Piano，加速比稳定在约 **1.2×**。

![聚合大小与验证时间](../plot/figures/aggregation_figure1.png)

---

**链上验证 Gas 开销**：

Piano 需对 K 个证明逐一调用验证合约，Gas 与 K 线性正比（斜率 ≈ 627K/次）。
BPiano 利用常数次配对（4 次，与 K 无关）将 K 个证明合并验证，
Gas 增速显著更低，**K=20 时节省峰值达 63.8%，K=100 时仍节省 55.4%**。

![链上 Gas 开销](../plot/figures/aggregation_figure2.png)

---

## 七、关键设计决策

### 7.1 DKZG 从零实现

旧的 `pianist-gnark-crypto/dkzg` 与 MPI 硬编码耦合，类型系统与新版 gnark-crypto 不兼容，
无法直接复用。详见 [dkzg.md](dkzg.md)。

### 7.2 单进程模拟分布式

原 Pianist 使用 `simpleMPI` 实现真实进程间通信。本实现**在单进程内模拟**：
M 个子节点的数据按下标区分，论文核心指标（证明大小、配对次数、验证时间）与是否真实分布无关。

### 7.3 SRS 采用 Lagrange 形式

```
Ux[i][j] = [R_i(τ_Y) · L_j(τ_X)]₁
```
与 gnark 新版 Plonk 一致，承诺输入直接是 Lagrange 系数，无需对见证多项式做 IFFT。

### 7.4 G2 标量乘在链下预计算

EVM 没有 G2 标量乘预编译（仅有 G1 的 ecMul 0x07）。
`solgen.GenerateBPianoCalldata` 在 Go 端预计算两个 G2 点：
- `ZTG2 = G2[2] - (α+ωα)·G2[1] + α·ωα·G2[0]`
- `TauYBetaG2 = G2Y[1] - β·G2Y[0]`

并以 calldata 形式传入 Solidity 合约。

---

## 八、原始论文

- `paper.md` —— 论文方案描述（Markdown 版）
- `pianist.pdf` —— Pianist 底层协议原始论文
