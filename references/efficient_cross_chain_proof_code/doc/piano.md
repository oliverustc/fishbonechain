# Piano 协议设计与实现说明

**状态：** ✅ 完成，5 个测试全部通过

---

## 一、与旧版 pianist-gnark 的关键差异

### 1.1 包设计：独立于 gnark 编译器

旧版 pianist-gnark 直接操作 `cs.SparseR1CS`（gnark 编译的约束系统）。
新实现完全独立于 gnark 编译器：调用方将电路描述为 `CircuitInfo`
（选择子 + 置换，Lagrange 形式），将见证描述为 `[]WitnessInstance`。

这使得 Piano 可作为独立密码学库使用，不绑定特定电路前端。

### 1.2 SRS：统一使用 DKZG，无独立 Y 轴 KZG

旧代码用标准 KZG SRS 提交 Y 轴多项式（`globalSRS *kzg.SRS`）。
新实现统一使用 DKZG SRS 的 `Vy[i]` 元素：
```
Y 轴承诺：com_Y = Σ_i v_i · Vy[i]   其中 Vy[i] = [R_i(τ_Y)]₁
```
因此 `ProvingKey` 和 `VerifyingKey` 中只有 `DKZGSRS *dkzg.SRS`，无独立 kzg.SRS。

### 1.3 选择器多项式存储：全 Lagrange 形式

旧代码将选择器以 Canonical 形式存储（IFFT+BitReverse 后）。
新实现全部以 Lagrange 形式（X 轴单位根上的求值）存储，可直接用于 `CommitLocal`。

### 1.4 余集 FFT：使用全域 OnCoset 方案

旧代码使用 `FFTPart`（分批余集 FFT）。
新实现使用 `domain.FFT(a, DIF, OnCoset())` 对整个大域一次性计算。

---

## 二、代码结构

```
bpiano/piano/
├── keys.go       # CircuitInfo, WitnessInstance, ProvingKey, VerifyingKey, Proof
├── export.go     # BuildPermutation（供外部调用）
├── setup.go      # Setup, SetupWithTrapdoors, computePermutationPolys
├── prove.go      # Prove（6 轮证明 + 辅助函数）
├── verify.go     # Verify, verifyAlgebraicConstraint, verifyDKZGProofs
├── utils.go      # 各类工具函数
└── piano_test.go # 端到端测试（5 个测试）
```

---

## 三、数据结构（keys.go）

### CircuitInfo（电路描述，调用方提供）

```go
type CircuitInfo struct {
    Ql, Qr, Qm, Qo, Qk []fr.Element  // 选择子，Lagrange 形式，长度 T
    Permutation          []int64       // 连线置换，长度 3T
    NbPublicInputs       int           // 公开输入行数（L 列最前面若干行）
}
```

### WitnessInstance（单个子节点的见证）

```go
type WitnessInstance struct {
    L, R, O []fr.Element  // 各长度 T，Lagrange 形式
}
```

### ProvingKey

```go
type ProvingKey struct {
    Vk       *VerifyingKey
    DomainX  fft.Domain    // 大小 T
    DomainXL fft.Domain    // 大小 4T（商多项式用）
    DomainY  fft.Domain    // 大小 M
    DomainYL fft.Domain    // 大小 4M

    Ql, Qr, Qm, Qo, Qk []fr.Element  // 选择子，Lagrange 形式，长度 T
    S1, S2, S3          []fr.Element  // 置换多项式，Lagrange 形式，长度 T
    Permutation         []int64       // 原始置换索引，长度 3T

    DKZGSRS *dkzg.SRS
}
```

### VerifyingKey

```go
type VerifyingKey struct {
    SizeX, SizeY   uint64
    NbPublicInputs int
    GeneratorX     fr.Element  // ω_X（T 次本原单位根）
    GeneratorY     fr.Element  // ω_Y（M 次本原单位根）
    CosetShift     fr.Element  // u（陪集偏移，DomainX 的乘法生成元）

    DKZGSRS *dkzg.SRS

    Ql, Qr, Qm, Qo, Qk dkzg.Digest  // 选择子 DKZG 全局承诺
    S1, S2, S3          dkzg.Digest  // 置换多项式 DKZG 全局承诺
}
```

### Proof

```go
type Proof struct {
    LRO [3]dkzg.Digest  // com_A, com_B, com_O
    Z   dkzg.Digest     // com_Z

    Hx [3]dkzg.Digest  // com_{H_X,0..2}（X 轴商多项式三段）
    Hy [3]dkzg.Digest  // com_{H_Y,0..2}（Y 轴商多项式三段，用 Vy MSM）

    // X 轴批量开启证明（对 α），共 13 个多项式：
    // [foldedHx, A, B, O, Ql, Qr, Qm, Qo, Qk, S1, S2, S3, Z]
    BatchedProofX  dkzg.BatchedProofX
    // Z 在 ω_X·α 处的独立 X 轴开启
    ZShiftedProofX dkzg.BatchedProofX

    // Y 轴批量开启证明（对 β），共 7 个多项式：
    // [A(Y,α), B(Y,α), O(Y,α), Z(Y,α), Z(Y,ω·α), foldedHx(Y,α), foldedHy(Y)]
    BatchedProofY dkzg.BatchedProofY

    // 评估值（用于代数约束检查）
    EvalA, EvalB, EvalO, EvalZ, EvalZS, EvalHx, EvalHy fr.Element
    EvalQl, EvalQr, EvalQm, EvalQo, EvalQk             fr.Element
    EvalS1, EvalS2, EvalS3                              fr.Element
}
```

---

## 四、关键 API

```go
// Setup 随机生成 SRS（实际使用）
func Setup(ci CircuitInfo, M, T uint64) (*ProvingKey, *VerifyingKey, error)

// SetupWithTrapdoors 使用指定陷门（仅测试用）
func SetupWithTrapdoors(ci CircuitInfo, M, T uint64, tauX, tauY *big.Int) (*ProvingKey, *VerifyingKey, error)

// BuildPermutation 从 lro 连线映射构建置换向量
func BuildPermutation(lro []int, nbVariables, T int) []int64

// Prove 生成 Piano 证明
func Prove(pk *ProvingKey, witnesses []WitnessInstance, publicInputs [][]fr.Element) (*Proof, error)

// Verify 验证 Piano 证明
func Verify(vk *VerifyingKey, proof *Proof, publicInputs [][]fr.Element) error
```

---

## 五、Prove 算法（6 轮）

### 第 1 轮：见证承诺

```
// 并行 goroutine，子节点 i：
com_a[i] = dkzg.CommitLocal(i, witnesses[i].L, srs)
com_b[i] = dkzg.CommitLocal(i, witnesses[i].R, srs)
com_o[i] = dkzg.CommitLocal(i, witnesses[i].O, srs)

// 主节点聚合：
proof.LRO[k] = dkzg.AggregateDigests(com_k)

// Fiat-Shamir（bindPublicData 绑定 Ql..S3 承诺 + 公开输入）：
gamma = FS("gamma", vk.Ql..vk.S3, publicInputs, proof.LRO)
eta   = FS("eta")
```

### 第 2 轮：置换累加多项式 Z

```
// 子节点 i 计算 z_i（Lagrange 形式），满足：
// z_i[0] = 1
// z_i[j+1] = z_i[j] · (a+η·id_a[j]+γ)(b+η·id_b[j]+γ)(o+η·id_o[j]+γ)
//                   / (a+η·S1[j]+γ)(b+η·S2[j]+γ)(o+η·S3[j]+γ)
// 验证 z_i[T] = 1（产品守恒）

proof.Z = AggregateDigests(CommitLocal(z_i) for i)
lambda  = FS("lambda", proof.Z)
```

### 第 3 轮：X 轴商多项式

在 4T 大域余集上计算：
```
// coset FFT：natural order → bit-reversed order
cosetEvals_a = DomainXL.FFT(canonical_a, DIF, OnCoset())

// ⚠️ 关键：vanishingOnCoset 的结果是 natural order，
//    但 coset FFT 的输出是 bit-reversed order。
//    必须对 vanishingOnCoset 结果调用 fft.BitReverse() 后再逐点相除。

vanishingXCoset = computeVanishingOnCoset(DomainXL, T)
fft.BitReverse(vanishingXCoset)  // ← 必须做，否则顺序不对应

// 逐点：h[j] = num[j] / vanishingXCoset[j]
// IFFT 回 canonical，分三段 hx1, hx2, hx3

// 转 Lagrange 再提交
hx1_lag = DomainX.FFT(hx1, DIF)
com_hx1[i] = CommitLocal(i, hx1_lag, srs)
```

### 第 4 轮：X 轴开启（α 处）

折叠商多项式：
```
foldedHx = hx1 + α^T·hx2 + α^{2T}·hx3
```

用 `dkzg.AggregateProofX` + `BatchOpenX` 对 13 个多项式批量开启。
Z 在 `ω_X·α` 处单独开启（`alphaShifted = alpha * DomainX.Generator`）。

### 第 5 轮：Y 轴商多项式

```
// Y 轴多项式的 Lagrange 系数 = 各子节点在 α 处的评估值
aVec = [a_0(α), a_1(α), ..., a_{M-1}(α)]
bVec, oVec, zVec, zsVec, fhxVec 同理

// 在 4M 大域余集上计算 Y 轴商多项式
// 同样需要 fft.BitReverse(vanishingYCoset)

beta = FS("beta", proof.Hy)
```

### 第 6 轮：Y 轴开启（β 处）

```
foldedHy = hy1 + β^M·hy2 + β^{2M}·hy3

// 对 7 个 Y 轴多项式用 dkzg.BatchOpenY 批量开启
```

---

## 六、Verify 算法

```
func Verify(vk, proof, publicInputs) error
```

1. **Fiat-Shamir 重算**：按与 Prove 完全相同的顺序推导 gamma, eta, lambda, alpha, beta。

2. **代数约束检查**（纯域运算，无配对）：
   ```
   G = Ql·A + Qr·B + Qm·A·B + Qo·O + Qk
   P0 = L0(α) · (Z - 1)        // L0(α) = (α^T-1)/(T·(α-1))
   P1 = ZS·(A+η·α+γ)(B+η·u·α+γ)(O+η·u²·α+γ)
      - Z·(A+η·S1+γ)(B+η·S2+γ)(O+η·S3+γ)

   验证：G + λ·P0 + λ²·P1 = (α^T-1)·Hx + (β^M-1)·Hy
   ```

3. **X 轴 DKZG 验证**（`VerifyBatchedAndAggregatedX`）：验证 13 个多项式 + Z shifted 的 X 轴开启。

4. **Y 轴 DKZG 验证**（`VerifyBatchedProofY`）：验证 7 个 Y 轴多项式在 β 处的开启。

---

## 七、实现中的关键 Bug 及修复

### Bug 1：coset FFT 输出顺序与 vanishing 多项式顺序不对应

**现象：** 商多项式计算后 IFFT 回来，系数中混有非零高次项，理论上最高次应为 3T-1 却有 4T-1 次项。

**根因：**
- `DomainXL.FFT(a, DIF, OnCoset())` 输出为 **bit-reversed** 顺序
- `computeVanishingOnCoset()` 按自然顺序返回（点的标准索引顺序）
- 两者逐点相除时索引不对应

**修复：**
```go
vanishingXCoset := computeVanishingOnCoset(&pk.DomainXL, T)
fft.BitReverse(vanishingXCoset)  // ← 对齐到 bit-reversed 顺序
```

同样的修复也需要对 Y 轴的 `vanishingYCoset` 做。

### Bug 2：L0OnCoset 需要独立计算（不能复用 vanishing 商）

`L0(X) = (X^T - 1) / (T·(X-1))` 需要在余集上单独计算，
不能直接用 `vanishingXCoset / (coset_point - 1)`，
因为 `X-1` 的余集值也需要 bit-reverse。

**修复：** 专门的 `l0OnCoset` 函数在 natural order 计算后做 `fft.BitReverse`。

### Bug 3：IFFT 使用 DIT（不是 DIF）

```go
// 正确：
DomainXL.FFTInverse(h, DIT, OnCoset())

// 错误（结果不对）：
DomainXL.FFTInverse(h, DIF, OnCoset())
```

gnark-crypto 的约定：DIF 正变换 + DIT 逆变换，配对使用。

---

## 八、测试（piano_test.go）

| 测试名 | 说明 | 状态 |
|--------|------|------|
| `TestSetup` | Setup 正确性：SRS、选择子承诺、置换多项式 | ✅ |
| `TestProveTrivial` | 最小电路（全零见证），T=4, M=2 | ✅ |
| `TestProveCopyConstraint` | 含复制约束的电路 | ✅ |
| `TestProveM4` | M=4 多节点并行 | ✅ |
| `TestBuildPermutation` | 置换构建辅助函数 | ✅ |
