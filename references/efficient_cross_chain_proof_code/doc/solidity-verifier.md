# Solidity 链上验证器：Piano & BPiano 设计与实现计划

---

## 一、目标与完成状态

**最终目标：** 给定任意电路，生成 Piano / BPiano 证明后，输出：
1. 一个可直接部署的 Solidity 验证合约（VK 常量已硬编码）
2. 对应的 ABI 编码调用数据（calldata），传入 `verify()` 即可完成链上验证

| 子目标 | 状态 |
|--------|------|
| BPiano Solidity 验证合约 (`BPianoVerifier.sol`) | ✅ 完成，Forge 测试全部通过 |
| BPiano Go calldata 生成器（`solgen/`） | ✅ 完成 |
| Piano Solidity 验证合约 (`PianoVerifier.sol`) | ✅ 完成，Forge 测试全部通过 |
| Piano Go calldata 生成器（`solgen/piano_calldata.go`） | ✅ 完成 |
| BPiano `ExportSolidity`（硬编码 VK 的可部署合约） | ✅ 完成 |
| Piano `ExportSolidity` | ✅ 完成 |
| ABI 编码 calldata 输出（替代 JSON fixture） | ✅ 完成 |

---

## 二、技术基础

### 2.1 EVM BN254 预编译合约

| 地址 | 功能 | Gas（近似） |
|------|------|-------------|
| 0x05 | 模幂（BigModExp） | 可变 |
| 0x06 | G1 点加（ecAdd） | 150 |
| 0x07 | G1 标量乘（ecMul） | 6,000 |
| 0x08 | 多配对检验（ecPairing） | 45,000 + 34,000·k |

**关键限制：EVM 没有 G2 标量乘预编译。**
所有含 G2 标量乘的点必须在 Go 侧预计算，以 calldata 形式传入。

### 2.2 Fiat-Shamir 转录格式（gnark-crypto）

```
challenge[0] = SHA256(name_bytes || bound_value_1 || ...)
challenge[k] = SHA256(name_bytes || challenge[k-1] || bound_value_1 || ...)
```

- G1 点绑定：`G1Affine.Bytes()` 的 **32 字节压缩格式**
  - 无穷远点：MSB = `0x40`（`mCompressedInfinity`）
  - 普通点：MSB = `0x80`（Y ≤ HALF_FP）或 `0xC0`（Y > HALF_FP）
- Fr 元素绑定：`fr.Element.Bytes()` 的 **32 字节大端格式**
- SHA-256 输出 mod Fr → 得到挑战值

### 2.3 DKZG 内部批量折叠哈希（非 FS 转录）

DKZG 的批量折叠随机数使用独立的哈希函数（`deriveBatchGamma`）：

```
gamma = SHA256(point_fr_bytes || comF_0_compressed || comF_1_compressed || ...)
```

- 第一个输入是 Fr 元素的字节（非压缩 G1）
- 后续是 G1 点的压缩字节
- 格式与 gnark-crypto FS 转录**不同**

### 2.4 G2 点 EVM 格式

EVM ecPairing 的 G2 输入：`(x.A1, x.A0, y.A1, y.A0)` 各 32 字节（虚部在前）。

---

## 三、BPiano 验证器（已完成）

### 3.1 验证流程

```
步骤 1：重放 FS 挑战（7 个）
步骤 2：代数约束检验（纯域运算）
步骤 3：推导 ρ（SHA256）
步骤 4：构建 4 个 G1 点
步骤 5：4-pairing 检验
```

### 3.2 验证方程

```
e(ComQX, ZTG2) · e(ρ·Pi1AggH, TauYBetaG2) · e(P2, G2[0]) · e(P3, G2[1]) = 1
```

其中 `ZTG2` 和 `TauYBetaG2` 由 Go 端预计算传入。

### 3.3 Fiat-Shamir 挑战（7 个）

| 挑战 | SHA256 输入 |
|------|------------|
| gamma | `"gamma"` \|\| VK.{Ql..S3}（压缩G1）\|\| PI（Fr）\|\| LRO[0..2]（压缩G1） |
| eta | `"eta"` \|\| gammaHash |
| lambda | `"lambda"` \|\| etaHash \|\| Z（压缩G1） |
| alpha | `"alpha"` \|\| lambdaHash \|\| Hx[0..2]（压缩G1） |
| nu | `"nu"` \|\| alphaHash \|\| foldedHxDig \|\| LRO[0..2] \|\| Z |
| beta | `"beta"` \|\| nuHash \|\| ComQX \|\| ComVFAlpha \|\| ComVFZS |
| mu | `"mu"` \|\| betaHash |

### 3.4 代数约束

```
lhs = gate + λ·(λ·boundary + perm)
rhs = (α^T−1)·hx + (β^M−1)·hy
断言 lhs == rhs
```

### 3.5 证明字段（BPiano）

- **G1 承诺**（12 个）：LRO[3], Z, Hx[3], ComQX, ComVFAlpha, ComVFZS, ComGY, Pi1AggH
- **Fr 求值**（15 个）：A, B, O, Z, ZS, Hx, Hy, Ql, Qr, Qm, Qo, Qk, S1, S2, S3
- **G2 预计算**（2 个，Go 端）：ZTG2, TauYBetaG2

### 3.6 代码位置

```
bpiano/
├── solgen/
│   ├── bpiano_calldata.go     # GenerateBPianoCalldata
│   ├── vk_calldata.go         # ExtractVKSolidity
│   └── forge_gen_test.go      # Forge 端到端测试
└── sol/
    ├── src/BPianoVerifier.sol
    └── test/BPianoVerifierTest.t.sol
        BPianoVerifierPITest.t.sol
```

---

## 四、Piano 验证器（待实现）

### 4.1 验证流程（4 pairings）

```
步骤 1：重放 FS 挑战（5 个：gamma, eta, lambda, alpha, beta）
步骤 2：代数约束检验（同 BPiano，在 (β, α) 处求值）
步骤 3：X 轴 DKZG 验证（2 pairings）
步骤 4：Y 轴 DKZG 验证（2 pairings）
```

### 4.2 Fiat-Shamir 挑战（5 个）

| 挑战 | SHA256 输入 |
|------|------------|
| gamma | `"gamma"` \|\| VK.{Ql..S3}（压缩G1）\|\| PI（Fr）\|\| LRO[0..2]（压缩G1） |
| eta | `"eta"` \|\| gammaHash |
| lambda | `"lambda"` \|\| etaHash \|\| Z（压缩G1） |
| alpha | `"alpha"` \|\| lambdaHash \|\| Hx[0..2]（压缩G1） |
| beta | `"beta"` \|\| alphaHash \|\| BatchedProofX.H \|\| ClaimedDigests[0..12] \|\| Hy[0..2] |

> `ClaimedDigests[k]` 是 13 个 comVF 点（G1），来自 BatchedProofX。

### 4.3 X 轴 DKZG 验证（2 pairings → 合并为 1 次 2-pairing 调用）

对应 Go 端 `VerifyBatchedAndAggregatedX`：

```
batchComFs[13] = [foldedHxDig, LRO[0], LRO[1], LRO[2],
                   Ql, Qr, Qm, Qo, Qk, S1, S2, S3, Z]

γ_X = SHA256(alpha_fr_32B || comF_0_compressed || ... || comF_12_compressed)
      （使用 deriveBatchGamma 格式，非 FS 转录格式）

foldedComF  = Σ_k γ_X^k · batchComFs[k]          （13 ecMul + 12 ecAdd）
foldedComVF = Σ_k γ_X^k · ClaimedDigests[k]       （13 ecMul + 12 ecAdd）

r = SHA256(alpha_fr_32B || foldedComF_compressed || Z_compressed)

LHS1 = foldedComF  - foldedComVF + α·BatchedProofX.H
LHS2 = Z           - ZShiftedProofX.ComVF + ωα·ZShiftedProofX.H
LHS  = LHS1 + r·LHS2
RHS  = BatchedProofX.H + r·ZShiftedProofX.H

检验：e(LHS, G2[0]) · e(-RHS, G2[1]) = 1
```

**注意：** X 轴配对中 G2[0] 和 G2[1] 是固定 SRS 点，无需 G2 标量乘。
即 X 轴验证完全可以链上完成，无需 Go 端预计算 G2 点。

### 4.4 Y 轴 DKZG 验证（2 pairings → 合并为 1 次 2-pairing 调用）

对应 Go 端 `VerifyBatchedProofY`：

```
comVFs[15] = ClaimedDigests[0..12] || ZShiftedProofX.ComVF || foldedHyDig
（其中 foldedHyDig = Hy[0] + β^M·Hy[1] + β^{2M}·Hy[2]）

γ_Y = SHA256(beta_fr_32B || comVF_0_compressed || ... || comVF_14_compressed)

foldedComVF_Y = Σ_k γ_Y^k · comVFs[k]             （15 ecMul + 14 ecAdd）
foldedValue   = Σ_k γ_Y^k · ClaimedValues[k]       （纯域运算，廉价）

diff = foldedComVF_Y - foldedValue·g1

TauYBetaG2 = G2Y[1] - β·G2Y[0]                    （Go 端预计算，同 BPiano）

检验：e(diff, G2Y[0]) · e(-BatchedProofY.H, TauYBetaG2) = 1
```

**注意：** Y 轴需要 `TauYBetaG2`（Go 端预计算），与 BPiano 相同。

### 4.5 证明字段（Piano）

- **G1 承诺**（27 个）：
  - LRO[3], Z, Hx[3], Hy[3]（共 10 个）
  - BatchedProofX.H, ClaimedDigests[13]（共 14 个）
  - ZShiftedProofX.H, ZShiftedProofX.ComVF（共 2 个）
  - BatchedProofY.H（共 1 个）
- **Fr 求值**（30 个）：
  - A, B, O, Z, ZS, Hx, Hy, Ql, Qr, Qm, Qo, Qk, S1, S2, S3（共 15 个）
  - BatchedProofY.ClaimedValues[15]（共 15 个）
- **G2 预计算**（1 个，Go 端）：TauYBetaG2

### 4.6 Gas 估算（Piano vs BPiano）

| 操作 | Piano | BPiano |
|------|-------|--------|
| ecMul 次数 | ~50 | ~20 |
| ecAdd 次数 | ~50 | ~20 |
| ecPairing（k=2） | 2 次 × 2 = 4 | 1 次 × 4 |
| Gas 估算 | ~800K–1M | ~450K–600K |

> BPiano 的链上验证成本约为 Piano 的 50%–60%，这是压缩方案的核心价值。

### 4.7 代码位置（已完成）

```
bpiano/
├── solgen/
│   ├── piano_calldata.go          # GeneratePianoCalldata（重放FS+预计算G2）
│   └── forge_gen_piano_test.go    # TestForgeE2EPiano（端到端测试）
└── sol/
    ├── src/PianoVerifier.sol      # Piano 验证合约
    └── test/PianoVerifierTest.t.sol
        test/fixture_piano.json    # 由 TestForgeE2EPiano 自动生成
```

### 4.8 实测 Gas 消耗

- Piano `test_Verify`: **939,657 gas**
- BPiano `test_Verify`: **~577,000 gas**
- 比值：Piano / BPiano ≈ 1.63×（BPiano 节省约 39% gas）

主要差异来源：Piano X 轴需要 13+13=26 个 ecMul，Y 轴需 15+1=16 个 ecMul，合计约 42 个额外 ecMul。

---

## 五、可部署 Solidity 合约生成（ExportSolidity）✅

### 5.1 实现结果

生成的合约（示例）：

```solidity
// Code generated by ExportBPianoVerifier. DO NOT EDIT.
contract BPianoVerifier_MyCircuit {
    uint256 internal constant VK_QL_X         = 0x17bb1895...;
    uint256 internal constant VK_QL_Y         = 0x2c9195e2...;
    // ... 共 33 个 VK 常量 ...

    // 无构造函数，无 storage 状态变量
    function verify(CompressedProof calldata proof, ...) external view returns (bool)
}
```

### 5.2 API

```go
// solgen/export.go

// ExportBPianoVerifier 将 VK 硬编码到 BPianoVerifier 模板，返回完整 .sol 源码字符串。
func ExportBPianoVerifier(vk *piano.VerifyingKey, contractName string) (string, error)

// ExportPianoVerifier 类似，用于 Piano。
func ExportPianoVerifier(vk *piano.VerifyingKey, contractName string) (string, error)
```

### 5.3 实现方案

使用 Go `text/template`，模板与现有合约逻辑完全相同，仅做以下变换：
- 移除 `VerifyingKey public vk;` 状态变量和构造函数
- 移除 `VerifyingKey` struct 定义
- 添加 33 个 `internal constant` 声明（8 G1 × 2 + 3 G2 × 4 + 5 标量）
- 所有 `vk.xxx` 引用替换为常量名（`VK_QL_X` 等）或内联 `Pairing.G1Point(VK_QL_X, VK_QL_Y)` 构造

### 5.4 代码位置

```
bpiano/
├── solgen/
│   ├── export.go            # ExportBPianoVerifier / ExportPianoVerifier
│   └── export_test.go       # TestExportBPianoVerifier / TestExportPianoVerifier
└── sol/
    └── test/
        ├── BPianoVerifierGenTest.t.sol   # 部署无构造函数合约，验证同 fixture.json
        └── PianoVerifierGenTest.t.sol    # 同上，用 fixture_piano.json
```

### 5.5 典型使用流程

```go
// 电路 Setup 后，生成并部署可验证的合约
sol, err := solgen.ExportBPianoVerifier(vk, "BPianoVerifier_MyCircuit")
os.WriteFile("BPianoVerifier_MyCircuit.sol", []byte(sol), 0644)
// → 直接用 forge create / cast 部署，无需额外构造参数

// 生成验证调用数据（下一节实现）
calldata, err := solgen.EncodeBPianoVerifyCalldata(proof, vk, publicInputs)
```

---

## 六、ABI 编码 Calldata 输出 ✅

### 6.1 实现结果

`solgen/abi_calldata.go` 提供两个函数，将 Piano / BPiano 证明编码为
可直接用于 `eth_sendTransaction.data` 的字节序列。

不依赖 go-ethereum，使用手动 ABI spec 编码（无额外依赖）。

### 6.2 API

```go
// solgen/abi_calldata.go

// EncodeBPianoVerifyCalldata 将 BPiano 压缩证明编码为 verify() 的 ABI calldata。
// 返回：函数选择器(4B) + ABI 编码参数，可直接用于 eth_sendTransaction.data。
func EncodeBPianoVerifyCalldata(
    proof *bpiano.CompressedProof,
    vk *piano.VerifyingKey,
    publicInputs [][]fr.Element,
) ([]byte, error)

// EncodePianoVerifyCalldata 类似，用于 Piano。
func EncodePianoVerifyCalldata(
    proof *piano.Proof,
    vk *piano.VerifyingKey,
    publicInputs [][]fr.Element,
) ([]byte, error)
```

### 6.3 函数选择器

| 合约 | 函数签名 | 选择器 |
|------|---------|--------|
| BPianoVerifier | `verify(CompressedProof,G2Point,G2Point,uint256[])` | `0xd3691eba` |
| PianoVerifier | `verify(PianoProof,G2Point,uint256[])` | `0x7be45049` |

### 6.4 ABI 布局

**BPiano**（无公开输入时共 1572 字节）：

```
[0   ..  3]   选择器 0xd3691eba
[4   ..1251]  CompressedProof（12 G1×64B + 15 Fr×32B = 1248B）
[1252..1379]  G2Point zTG2（128B）
[1380..1507]  G2Point tauYBetaG2（128B）
[1508..1539]  offset for uint256[]（= 1536，32B）
[1540..1571]  length of publicInputsFlat（32B，= 0 时无后续）
[1572..    ]  publicInputsFlat 元素（每个 32B）
```

**Piano**（无公开输入时共 2884 字节）：

```
[0   ..   3]  选择器 0x7be45049
[4   ..2691]  PianoProof（27 G1×64B + 30 Fr×32B = 2688B）
[2692..2819]  G2Point tauYBetaG2（128B）
[2820..2851]  offset for uint256[]（= 2848，32B）
[2852..2883]  length of publicInputsFlat（32B，= 0 时无后续）
[2884..    ]  publicInputsFlat 元素（每个 32B）
```

### 6.5 典型使用流程

```go
// 生成可部署合约
sol, err := solgen.ExportBPianoVerifier(vk, "BPianoVerifier_MyCircuit")
os.WriteFile("BPianoVerifier_MyCircuit.sol", []byte(sol), 0644)

// 生成验证调用数据（直接发送到链上合约）
calldata, err := solgen.EncodeBPianoVerifyCalldata(proof, vk, publicInputs)
// calldata 可直接用于 eth_sendTransaction.data 或 cast send
```

### 6.6 代码位置

```
bpiano/solgen/
├── abi_calldata.go         # EncodeBPianoVerifyCalldata / EncodePianoVerifyCalldata
└── abi_calldata_test.go    # 结构校验 + Forge 端到端测试
```

---

## 七、实现顺序（已完成）

```
阶段 1：BPiano 验证器 ✅
阶段 2：Piano 验证器 ✅
阶段 3：ExportSolidity（VK 硬编码） ✅
阶段 4：ABI calldata 编码 ✅
```

---

## 八、编译注意事项

```toml
# foundry.toml
[profile.default]
via_ir = true         # 必须，否则 stack-too-deep
optimizer = true
optimizer_runs = 200
fs_permissions = [{ access = "read", path = "test/" }]
```

所有 Yul assembly 块须标注 `assembly ("memory-safe") { ... }`。

---

## 九、已解决的关键难点

| 问题 | 解决方案 |
|------|---------|
| G2 标量乘无预编译 | Go 端预计算 G2 点以 calldata 传入 |
| 无穷远点压缩格式不一致 | `_g1Compressed(0,0)` 特判返回 `0x40<<248`（`mCompressedInfinity`） |
| FS 挑战全部偏差 | 根因是无穷远点编码错误，修复后全部 7 个挑战与 Go 端一致 |
| stack-too-deep | `via_ir = true` + assembly 标注 `memory-safe` |
| `_modexp` 中 `exp` 命名冲突 | 参数改名为 `exponent`（`exp` 是 Yul 内置操作码） |
| `vm.readFile` 权限拒绝 | `foundry.toml` 添加 `fs_permissions` |
| 代数约束 revert vs return false | `verify()` 改为 `if (!_algebraicCheck(...)) return false` |
