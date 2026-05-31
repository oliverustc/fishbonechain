# DKZG 设计与实现说明

**状态：** ✅ 完成，25 个测试全部通过

---

## 一、数学基础

### 符号约定

| 符号 | 含义 |
|------|------|
| M | 子节点数量（2的幂，≥2） |
| T | 单个子电路规模（2的幂，≥2） |
| ω_X | T次单位根的生成元 |
| ω_Y | M次单位根的生成元 |
| τ_X, τ_Y | 随机陷门（SRS生成后销毁） |
| g1, g2 | G1, G2的生成元 |
| [a]₁ | a·g1（G1上的标量乘） |
| [a]₂ | a·g2（G2上的标量乘） |
| L_j(X) | X轴第j个拉格朗日基函数（T次单位根） |
| R_i(Y) | Y轴第i个拉格朗日基函数（M次单位根） |

### 拉格朗日基函数公式（含 ω^j 因子）

> ⚠️ 注意：公式中的 ω^j 因子必须保留，缺失会导致单位划分验证失败。

```
L_j(α) = ω_X^j · (α^T - 1) / (T · (α - ω_X^j))    j = 0,...,T-1
R_i(β) = ω_Y^i · (β^M - 1) / (M · (β - ω_Y^i))    i = 0,...,M-1
```

**为什么需要 ω^j？** 标准 Lagrange 基定义为 `L_j(α) = Π_{k≠j}(α-ω^k) / Π_{k≠j}(ω^j-ω^k)`。
分子是 `(α^T-1)/(α-ω^j)`，分母化简为 `T·ω^{-j}·(ω^j-ω^0)...(ω^j-ω^{j-1})...`
最终结果需乘以 `ω^j` 才等于上述公式。忽略该因子会使 `Σ_j L_j(α) ≠ 1`，导致验证失败。

### SRS 结构

```
Ux[i][j] = [R_i(τ_Y) · L_j(τ_X)]₁    i∈[0,M), j∈[0,T)   （M×T 个G1元素）
Vy[i]    = [R_i(τ_Y)]₁                 i∈[0,M)             （M 个G1元素）

G2[0]    = g2
G2[1]    = [τ_X]₂
G2[2]    = [τ_X²]₂     ← BPiano Shplonk 两点验证需要
G2Y[0]   = g2
G2Y[1]   = [τ_Y]₂
```

注：`Vy[i] = Σ_j Ux[i][j]`（G1加法），可由 Ux 推导，但为效率单独存储。
注：`G2[2]` 是相比原始 Pianist 方案新增的，用于构造 `[Z_T(τ_X)]₂`（见 BPiano 说明）。

### 承诺算法（子节点 i）

输入：`evals[0..T-1]`，即 `f_i(X)` 在 X 轴 T 次单位根上的求值（Lagrange 系数）

```
com_i = Σ_j evals[j] · Ux[i][j]   （MSM）
       = [R_i(τ_Y) · f_i(τ_X)]₁
```

### 全局承诺聚合（主节点）

```
com_F = Σ_i com_i
       = [Σ_i R_i(τ_Y) · f_i(τ_X)]₁
       = [F(τ_Y, τ_X)]₁
```

### X 轴开启（子节点 i，在点 α 处）

**商多项式（Lagrange 形式）：**
```
q_i[k] = (evals[k] - v_i) / (ω_X^k - α)    k = 0,...,T-1
其中 v_i = f_i(α) = Σ_k evals[k] · L_k(α)
```

**X 轴开启证明：**
```
π_{0,i} = MSM(Ux[i][:], q_i[:])
         = [R_i(τ_Y) · q_i(τ_X)]₁
```

**聚合（主节点）：**
```
π_{0,F} = Σ_i π_{0,i}
```
同时构造 `V_F(Y) = Σ_i v_i · R_i(Y)` 的承诺：`com_VF = Σ_i v_i · Vy[i]`

### Y 轴开启（主节点，在点 β 处）

输入：`claimedValues[i] = v_i = f_i(α)`（M 个标量，`V_F(Y)` 的 Lagrange 系数）

```
z = V_F(β) = Σ_i v_i · R_i(β)
q_Y[i] = (v_i - z) / (ω_Y^i - β)    i = 0,...,M-1
π_{1,F} = MSM(Vy[:], q_Y[:])
         = [q_Y(τ_Y)]₁
```

### 验证方程

```
e(com_F - [z]₁, g2) = e(π_{0,F}, [τ_X - α]₂) · e(π_{1,F}, [τ_Y - β]₂)
```

等价（PairingCheck 形式）：
```
e(com_F - [z]₁, g2) · e(-π_{0,F}, G2[1] - α·G2[0]) · e(-π_{1,F}, G2Y[1] - β·G2Y[0]) = 1
```

---

## 二、代码结构

```
dkzg/
├── srs.go       # SRS 结构体，NewSRS，NewSRSFromTau（测试用）
├── commit.go    # Digest 类型，CommitLocal，AggregateDigests，CommitGlobal
├── open.go      # OpeningProofX，OpeningProofY，LocalOpenX，AggregateOpenX，OpenY
├── verify.go    # Verify（单组配对验证）
└── batch.go     # BatchOpenResult，BatchedProofX，BatchedProofY，
                 # BatchVerify，AggregateProofX，BatchOpenX，BatchOpenY，
                 # VerifyBatchedAndAggregatedX，VerifyBatchedProofY
```

---

## 三、关键 API（实际实现）

### srs.go

```go
type SRS struct {
    Ux      [][]bn254.G1Affine // [M][T]，X 轴 Lagrange 承诺基
    Vy      []bn254.G1Affine   // [M]，Y 轴承诺基
    G2      [3]bn254.G2Affine  // [g2, g2^{τX}, g2^{τX²}]
    G2Y     [2]bn254.G2Affine  // [g2, g2^{τY}]
    DomainX fft.Domain
    DomainY fft.Domain
}

func NewSRS(M, T uint64) (*SRS, error)
func NewSRSFromTau(M, T uint64, tauX, tauY *big.Int) (*SRS, error) // 仅测试用
```

### commit.go

```go
type Digest = bn254.G1Affine

func CommitLocal(nodeIdx uint64, evals []fr.Element, srs *SRS) (Digest, error)
func AggregateDigests(locals []Digest) Digest
func CommitGlobal(evals []fr.Element, srs *SRS) (Digest, error)  // 对 Y 轴 Lagrange 系数的承诺
```

> ⚠️ 参数顺序：`CommitLocal(nodeIdx, evals, srs)`，nodeIdx 在前。

### open.go

```go
type OpeningProofX struct {
    H            bn254.G1Affine  // X 轴商多项式承诺
    ClaimedValue fr.Element      // f_i(alpha)
}

type OpeningProofY struct {
    H            bn254.G1Affine  // Y 轴商多项式承诺
    ClaimedValue fr.Element      // V_F(beta)
}

func LocalOpenX(nodeIdx uint64, evals []fr.Element, alpha fr.Element, srs *SRS) (OpeningProofX, error)
func AggregateOpenX(localProofs []OpeningProofX, srs *SRS) (comVF bn254.G1Affine, piX bn254.G1Affine, err error)
func OpenY(claimedValues []fr.Element, beta fr.Element, srs *SRS) (OpeningProofY, error)
```

> ⚠️ `AggregateOpenX` 同时构造 `com_VF`，返回 `(comVF, piX, err)` 三个值。

### verify.go

```go
func Verify(com, proofX, proofY Digest, alpha, beta, z fr.Element, srs *SRS) error
```

### batch.go（Piano 使用的批量接口）

```go
type BatchedProofX struct { ... }   // 折叠后的 X 轴聚合证明
type BatchedProofY struct { ... }   // 折叠后的 Y 轴聚合证明

func AggregateProofX(localProofs []OpeningProofX, srs *SRS) (AggregatedProofX, error)
func BatchOpenX(aggProofs []AggregatedProofX, comFs []Digest, alpha fr.Element) (BatchedProofX, error)
func BatchOpenY(comVFs []Digest, yPolysEvals [][]fr.Element, beta fr.Element, srs *SRS) (BatchedProofY, error)

func VerifyBatchedAndAggregatedX(proof BatchedProofX, ..., srs *SRS) error
func VerifyBatchedProofY(comVFs []Digest, proof BatchedProofY, beta fr.Element, srs *SRS) error
```

---

## 四、实现中的关键细节

### 4.1 `fr.BatchInvert` 的返回值语义

```go
// 错误用法（原地修改假设）：
fr.BatchInvert(denoms)          // denoms 不变！

// 正确用法：
denoms = fr.BatchInvert(denoms) // 必须接收返回值
```

`fr.BatchInvert` 返回一个新的切片，不修改输入切片。

### 4.2 Lagrange 求值的实现

对 Lagrange 形式多项式 `f = [f_0, ..., f_{T-1}]`（`f_k = f(ω^k)`）在任意 `α` 处求值：

```
t = α^T - 1
若 t=0：α 是单位根，直接返回对应的 f_k

否则：
  denom[k] = α - ω^k
  denom = BatchInvert(denom)
  v = (t / T) · Σ_k f_k · ω^k · denom[k]   ← ω^k 因子必须在这里
```

### 4.3 G2 数组大小

原 Pianist 的 DKZG 只需 `G2[2]`（g2 和 g2^τX）。
本实现扩展为 `G2[3]`，增加 `G2[2] = g2^{τX²}`，用于 BPiano 的 Shplonk 验证：

```
[Z_T(τX)]₂ = G2[2] - (α + ω·α)·G2[1] + α·ω·α·G2[0]
```

---

## 五、为什么从零实现 DKZG

`pianist-gnark-crypto` 中已存在一份 DKZG 实现（`ecc/bn254/fr/dkzg/dkzg.go`），但在基于新版 gnark 实现 Piano 协议时，这份代码**无法直接复用**。原因涉及五个层次。

### 5.1 MPI 硬编码在业务逻辑核心中

老版 DKZG 的分布式逻辑与 `simpleMPI` 深度耦合，无法剥离。

```go
func init() {
    mpi.WorldInit("_", "_", "_")  // 程序启动时强制初始化 MPI 运行时
}

func Commit(p []fr.Element, srs *SRS, nbTasks ...int) (Digest, error) {
    if mpi.SelfRank == 0 {
        for i := 1; i < int(mpi.WorldSize); i++ {
            subComBytes, err := mpi.ReceiveBytes(bn254.SizeOfG1AffineUncompressed, uint64(i))
        }
    } else {
        mpi.SendBytes(G1AffineToBytes(res), 0)
        return Digest{}, nil  // 子节点返回空值！
    }
}
```

`Commit`、`Open` 等所有核心函数内部都直接调用 `mpi.SelfRank`、`mpi.SendBytes`、`mpi.ReceiveBytes`，控制流本身就是分叉的。结果是：不启动 MPI 运行时（`mpirun -n M ./binary`）代码根本无法运行，也无法在 `go test` 中直接调用。

新实现将"子节点本地计算"和"主节点聚合"设计为普通函数调用（`CommitLocal` / `AggregateDigests`），彻底去掉 MPI。

### 5.2 `OpeningProof` 的类型语义根本不同

老版 `OpeningProof.ClaimedDigest` 是 **G1 群元素**（存储 $g^{f_i(\alpha)}$，即评估值的承诺），而新版 gnark-crypto KZG 的 `ClaimedValue` 是**有限域标量**（直接存储 $f(\alpha) \in \mathbb{F}$）。

这反映了 DKZG 的两层结构：X 轴开启得到标量 $v_i$，主节点将 $\{v_i\}$ 聚合为 Y 轴多项式 $V_F(Y)$ 的承诺（G1 元素）。老版把这个 G1 元素塞进 `ClaimedDigest`，导致语义混乱：验证代数约束需要标量，配对验证需要 G1 元素，两者必须**显式分离**。新实现将二者作为独立返回值处理。

### 5.3 SRS 结构不匹配

老版 SRS：
```go
type SRS struct {
    G1 []bn254.G1Affine  // 一维，G1[j] = g^{R_i(τ_Y) · τ_X^j}，i 隐含在 MPI rank 中
    G2 [2]bn254.G2Affine // [g2, g2^{τ_X}]，缺少 g2^{τ_Y}
}
```

三个问题：① 缺 `g2^{τ_Y}`，Y 轴开启证明验证无法进行；② X 轴用 canonical 幂 $\tau_X^j$，新版应使用 Lagrange 基 $L_j(\tau_X)$（省去 IFFT）；③ 一维结构无法在单进程内同时服务 M 个节点。

新实现使用二维 Lagrange 形式 SRS：`Ux[i][j] = g^{R_i(τ_Y) · L_j(τ_X)}`，并包含 `G2Y[1] = g^{τ_Y}`。

### 5.4 Go 模块系统冲突

`pianist-gnark-crypto` 与新版 `gnark-crypto` 拥有**相同的模块路径** `github.com/consensys/gnark-crypto`，但内容不兼容。新版 gnark 依赖新版 gnark-crypto，无法在同一项目中同时引入两者。

### 5.5 缺少 BPiano 所需的新功能

即使解决上述所有问题，老版 DKZG 仍不支持：Shplonk 多点聚合商多项式生成、Y 轴聚合多项式构造与承诺、配对合并验证方程、以及批量 K 个证明的聚合系数计算。这些都是 BPiano 压缩的核心功能。

### 总结

| 不兼容维度 | 老版问题 | 新实现方案 |
|-----------|---------|-----------|
| 分布式模型 | MPI 硬编码，无法脱离 MPI 运行时 | 函数参数化（nodeIdx），goroutine 模拟并发 |
| 类型语义 | `ClaimedDigest` 是 G1 元素，语义模糊 | 标量评估值和 G1 承诺显式分离 |
| SRS 结构 | 一维 canonical 基，缺 $g_2^{\tau_Y}$ | 二维 Lagrange 基，含 $g_2^{\tau_X}$ 和 $g_2^{\tau_Y}$ |
| Go 模块 | 与新版 gnark-crypto 路径冲突 | 从零实现，仅依赖新版 gnark-crypto |
| 功能覆盖 | 仅基础 DKZG，无聚合压缩功能 | 原生支持 Shplonk、Y 轴聚合、配对合并 |
