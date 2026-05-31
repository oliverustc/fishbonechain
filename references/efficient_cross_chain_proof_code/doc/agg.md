# BPiano 证明聚合实现计划

> **目标：** 按照论文 §4.3 完整实现 BPiano 证明聚合方案，包括：
> - §4.3.1 两轮挑战协调（共享 α、β）
> - §4.3.2 承诺聚合（AggregatedProof）
> - §4.3.3 批量验证（VerifyBatch，常数 4 次 pairing）
>
> **原则：** 现有 `Compress`、`VerifyCompressed` 等函数**一字不改**；
> 仅在 `compress.go` 末尾**追加**三个新函数，在 `aggregate.go` 中**改写**已有桩函数。
>
> **执行方式：** 按阶段依次执行，每阶段完成后更新状态标记与执行记录。

---

## 技术背景

### 为什么必须共享 α、β

聚合 K 个证明的 pairing 等式要求各证明的 G2 元素相同：
- `g₂^{Z_T(τ_X)} = g₂^{(τ_X-α)(τ_X-ωα)}`：依赖 α
- `g₂^{τ_Y-β}`：依赖 β

若 α_k、β_k 各不相同，G2 元素各异，无法做随机线性组合。

### 为什么需要在 compress.go 中追加新函数

现有 `Compress` 内部独立地从自己的 Hx^(k) 派生 α（第 350 行），
从自己的 ComQX^(k) 派生 β（第 639 行）。
挑战协调要求这两个挑战由**所有 K 个证明的数据**联合派生，
因此需要三个新的分阶段函数，将 `Compress` 在这两处"暂停"。
现有 `Compress` 保持原样，三个新函数是纯追加。

### compress.go 关键分割点

| 阶段 | 起止行 | 边界条件 |
|------|--------|----------|
| Stage1 | 第 80 行 → 第 348 行 | 止于 Hx[0..2] 聚合完成，α 派生之前（原第 350 行） |
| Stage2 | 第 350 行 → 第 637 行 | 接受外部 sharedAlpha；止于 ComQX/ComVFAlpha/ComVFZS 计算完成，β 派生之前（原第 639 行） |
| Stage3 | 第 639 行 → 第 750 行 | 接受外部 sharedBeta；完成 Y 轴聚合，返回 CompressedProof |

---

## 阶段规划

```
阶段 0 ─ 确定中间状态字段（ProveState1、ProveState2）
阶段 1 ─ 追加 CompressStage1/2/3 到 compress.go
阶段 2 ─ 实现 CoordinateChallenges（改写 aggregate.go 中的桩）
阶段 3 ─ 定义 AggregatedProof + 实现 AggregateProofs
阶段 4 ─ 实现 VerifyBatch（真正的聚合验证）
阶段 5 ─ 单元测试
```

---

## 阶段 0：确定中间状态字段

**状态：** ✓ 已完成

**ProveState1**（Stage1 完成后，保存继续计算所需的全部中间状态）：

```go
type ProveState1 struct {
    pk           *piano.ProvingKey
    witnesses    []piano.WitnessInstance
    publicInputs [][]fr.Element

    // 已派生的挑战（γ/η/λ 独立于 α，可保留）
    gamma, eta, lambda fr.Element

    // 已计算的承诺
    lro [3]dkzg.Digest
    z   dkzg.Digest
    hx  [3]dkzg.Digest

    // 每个子节点的中间多项式（Stage2 需要）
    zLagrange [][]fr.Element       // zLagrange[i]：M×T
    hx123     []hx123Polys         // hx1/hx2/hx3 规范系数（M 组）

    // 预计算的 coset 数据（仅依赖 pk，Stage2 复用）
    qlCoset, qrCoset, qmCoset, qoCoset, qkCoset []fr.Element
    s1Coset, s2Coset, s3Coset                   []fr.Element
    l0Coset                                     []fr.Element
    vanXCosetInv                                []fr.Element
}

type hx123Polys struct {
    hx1, hx2, hx3 []fr.Element // 规范系数形式，长度 T
}
```

**ProveState2**（Stage2 完成后）：

```go
type ProveState2 struct {
    ProveState1

    // 来自协调的共享挑战
    sharedAlpha    fr.Element
    alphaShifted   fr.Element
    alphaPowT      fr.Element

    // Stage2 新派生的挑战
    nu    fr.Element
    nuPow []fr.Element // [ν^0..ν^13]

    // Stage2 产出的承诺
    comQX      dkzg.Digest
    comVFAlpha dkzg.Digest
    comVFZS    dkzg.Digest

    // 共享多项式在 α 处的求值（用于 Stage3 Y 轴计算）
    qlAlpha, qrAlpha, qmAlpha, qoAlpha, qkAlpha fr.Element
    s1Alpha, s2Alpha, s3Alpha                   fr.Element

    // X 轴求值向量（每个子节点在 α 处，长度 M）
    aVec, bVec, oVec, zVec, zsVec, fhxVec []fr.Element

    // foldedHxDig（用于 Stage3 FS）
    foldedHxDig dkzg.Digest

    // Hy 承诺与 Lagrange（Stage3 Y 轴聚合需要）
    hyDig    [3]dkzg.Digest
    hy1Lag, hy2Lag, hy3Lag []fr.Element
}
```

> **注：** Stage2 中，ν 的派生仍使用独立的 per-proof FS transcript
> （绑定 `foldedHxDig, LRO[0..2], Z`，与原 `Compress` 相同），
> 因为 ν 仅依赖单个证明的数据，不需要协调。

**检查点：**
- [ ] 在本文件中确认所有字段无遗漏（通过逐行审读 compress.go 第 80–750 行）

---

## 阶段 1：追加 CompressStage1/2/3

**状态：** ✓ 已完成

**文件：** `bpiano/bpiano/compress.go`（末尾追加，现有代码零改动）

### CompressStage1

```go
// CompressStage1 执行证明生成第一阶段：witness 承诺 → Z 多项式 → X 轴商多项式 → Hx 承诺。
// 返回的 ProveState1 用于：
//  (a) 向协调节点提交 Hx[0..2]，参与共享 α 的推导；
//  (b) 在获得共享 α 后，传入 CompressStage2 继续生成。
func CompressStage1(pk *piano.ProvingKey, witnesses []piano.WitnessInstance,
    publicInputs [][]fr.Element) (*ProveState1, error)
```

逻辑：直接复制 Compress 第 80–348 行，改为填充 ProveState1 并返回（不调用 deriveChallengeBP "alpha"）。

### CompressStage2

```go
// CompressStage2 在共享 α 下执行第二阶段：Shplonk X 轴聚合 → ComQX/ComVFAlpha/ComVFZS。
// sharedAlpha 由 CoordinateChallenges 协调派生，所有 K 个证明使用同一值。
// 返回的 ProveState2 用于：
//  (a) 向协调节点提交 ComQX/ComVFAlpha/ComVFZS，参与共享 β 的推导；
//  (b) 在获得共享 β 后，传入 CompressStage3 完成生成。
func CompressStage2(state *ProveState1, sharedAlpha fr.Element) (*ProveState2, error)
```

逻辑：
- 以 `sharedAlpha` 直接作为 α（不经 FS），计算 `alphaShifted`、`alphaPowT`
- 复制 Compress 第 355–637 行（Shplonk 部分），结果填入 ProveState2
- ν 的派生使用**独立 per-proof FS**：`newFS("nu"); bind(foldedHxDig, LRO, Z); derive`
- Y 轴商多项式（第 471–594 行的 hY 计算）也在 Stage2 完成（它只依赖 α，不依赖 β）

### CompressStage3

```go
// CompressStage3 在共享 β 下完成证明生成：Y 轴聚合 → ComGY → Pi1AggH → 填充求值。
// sharedBeta 由 CoordinateChallenges 协调派生。
func CompressStage3(state *ProveState2, sharedBeta fr.Element) (*CompressedProof, error)
```

逻辑：
- 以 `sharedBeta` 直接作为 β
- μ 的派生使用**独立 per-proof FS**：`newFS("mu"); derive`（与原 Compress 同格式）
- 复制 Compress 第 644–750 行，返回 CompressedProof

### 正确性验证（新增单元测试）

```go
// TestStagesMatchCompress：对同一输入，用 Stage1/2/3 内部派生 α/β
//（即将 Stage1 的 Hx 单独派生 α，Stage2 的 ComQX 单独派生 β，与原 Compress 完全一致）
// 结果应与 Compress(...) 完全相同。
func TestStagesMatchCompress(t *testing.T)
```

**检查点：**
- [ ] `CompressStage1/2/3` 编译通过
- [ ] `TestStagesMatchCompress` 通过（Stage 结果与原 Compress 逐字段相同）

---

## 阶段 2：实现 CoordinateChallenges

**状态：** ✓ 已完成

**文件：** `bpiano/bpiano/aggregate.go`

**共享挑战派生格式：**

```
// 共享 α = SHA256("coord-alpha" || Hx^{(0)}[0] || Hx^{(0)}[1] || Hx^{(0)}[2]
//                              || Hx^{(1)}[0] || ... || Hx^{(K-1)}[2])
// 各 Hx 以 G1 压缩格式（32 字节）写入

// 共享 β = SHA256("coord-beta" || alpha_bytes
//                              || ComQX^{(0)} || ComVFAlpha^{(0)} || ComVFZS^{(0)}
//                              || ComQX^{(1)} || ... || ComVFZS^{(K-1)})
```

**CoordinateChallenges 实现：**

```go
func CoordinateChallenges(
    pks []*piano.ProvingKey,
    witnessSlices [][]piano.WitnessInstance,
    publicInputs [][][]fr.Element,
) ([]*CompressedProof, error) {
    K := len(pks)

    // 阶段一：并行执行 CompressStage1
    states1 := make([]*ProveState1, K)
    for k := 0; k < K; k++ {
        s, err := CompressStage1(pks[k], witnessSlices[k], publicInputs[k])
        if err != nil { return nil, fmt.Errorf("stage1[%d]: %w", k, err) }
        states1[k] = s
    }

    // 派生共享 α
    sharedAlpha := deriveSharedAlpha(states1)

    // 阶段二：并行执行 CompressStage2
    states2 := make([]*ProveState2, K)
    for k := 0; k < K; k++ {
        s, err := CompressStage2(states1[k], sharedAlpha)
        if err != nil { return nil, fmt.Errorf("stage2[%d]: %w", k, err) }
        states2[k] = s
    }

    // 派生共享 β
    sharedBeta := deriveSharedBeta(sharedAlpha, states2)

    // 阶段三：并行执行 CompressStage3
    proofs := make([]*CompressedProof, K)
    for k := 0; k < K; k++ {
        p, err := CompressStage3(states2[k], sharedBeta)
        if err != nil { return nil, fmt.Errorf("stage3[%d]: %w", k, err) }
        proofs[k] = p
    }
    return proofs, nil
}
```

**辅助函数：**

```go
func deriveSharedAlpha(states []*ProveState1) fr.Element
func deriveSharedBeta(sharedAlpha fr.Element, states []*ProveState2) fr.Element
```

**检查点：**
- [ ] `CoordinateChallenges(K=2)` 生成的两个证明，FS 重播后 α 相同、β 相同
- [ ] `CoordinateChallenges(K=4)` 同上

---

## 阶段 3：AggregatedProof + AggregateProofs

**状态：** ✓ 已完成

**文件：** `bpiano/bpiano/aggregate.go`

**结构体定义（对应论文 §4.3.2 的 π_batch）：**

```go
// AggregatedProof 是 K 个共享 α、β 的压缩证明聚合后的批量证明。
//
// 格式：
//   承诺部分：K × {com_A, com_B, com_O, com_Z, com_{H_X}, com_{Q_X}, com_{G_Y}} = K×7 G1
//   求值部分：K × {eval_A, eval_B, eval_O, eval_Z, eval_ZS, eval_{H_X}, eval_{H_Y}} = K×7 Fr
//   聚合 KZG 证明：com_{Q_X,total} + π_{1,total} = 2 G1
//
// 总大小：K×(7+7)×32 + 2×32 = K×448 + 64 字节。
type AggregatedProof struct {
    K      int
    Proofs []*CompressedProof // 保留 K 个完整压缩证明（含承诺、求值，用于验证端重建聚合量）

    // §4.3.2 定义的聚合 KZG 证明
    ComQXTotal dkzg.Digest    // com_{Q_X,total} = Σ r_k · com_{Q_X}^{(k)}
    Pi1Total   dkzg.Digest    // π_{1,total}     = Σ r_k · π_{1,agg}^{(k)}
}
```

**AggregateProofs 实现：**

```go
// AggregateProofs 将 K 个由 CoordinateChallenges 生成的压缩证明聚合为批量证明。
// 前提：proofs 中所有证明共享相同的 α 和 β（通过 CoordinateChallenges 保证）。
func AggregateProofs(proofs []*CompressedProof) (*AggregatedProof, error) {
    K := len(proofs)

    // 1. 派生聚合系数 r_k = SHA256(k_bytes || serialize(π_c^{(0)}) || ... || serialize(π_c^{(K-1)}))
    rk := deriveAggCoeffs(proofs) // 长度 K 的 []fr.Element

    // 2. com_{Q_X,total} = Σ r_k · com_{Q_X}^{(k)}  （G1 MSM）
    // 3. π_{1,total}     = Σ r_k · π_{1,agg}^{(k)}  （G1 MSM）
    ...
}
```

**聚合系数序列化格式（用于 r_k 和 Solidity 端一致）：**
```
serialize(π_c^{(k)}) = LRO[0..2] || Z || Hx[0..2] || ComQX || ComVFAlpha || ComVFZS
                     || ComGY || Pi1AggH  （各 32 字节 G1 compressed）
                     || EvalA..EvalS3     （各 32 字节 Fr）
```

**检查点：**
- [ ] `AggregateProofs` 编译通过
- [ ] `ComQXTotal == Σ r_k · ComQX_k` 可通过独立验证（field 运算验证）

---

## 阶段 4：VerifyBatch（真正的聚合验证）

**状态：** ✓ 已完成

**文件：** `bpiano/bpiano/aggregate.go`（改写现有桩函数）

**函数签名：** `func VerifyBatch(agg *AggregatedProof, vk *piano.VerifyingKey, publicInputs [][][]fr.Element) error`

**验证步骤（§4.3.3）：**

```
1. FS 重算共享 α：
   从 K 个证明的 Hx^{(k)} 重推 sharedAlpha = deriveSharedAlpha（与 CoordinateChallenges 格式相同）

2. FS 重算共享 β：
   从 K 个证明的 ComQX^{(k)}/ComVFAlpha^{(k)}/ComVFZS^{(k)} 重推 sharedBeta

3. 对每个 k，重算 per-proof 挑战（使用共享 α/β）：
   gamma_k, eta_k, lambda_k = per-proof FS（与原 VerifyCompressed 格式相同）
   nu_k = per-proof FS（绑定 foldedHxDig_k, LRO_k, Z_k）
   mu_k = per-proof FS

4. 派生聚合系数 rk = deriveAggCoeffs(proofs)（与 AggregateProofs 完全相同格式）

5. 聚合一致性验证（O(K) G1 标量乘法）：
   C1_k = Σ_{S∈S_α} ν_k^{idx(S)} · (com_S^{(k)} - [v_S^{(k)}(sharedBeta)]_1)
   C2_k = ν_k^{13} · (comZ^{(k)} - [evalZS^{(k)}·g1]_1)  — 修正版同 VerifyCompressed

   C1_total = Σ r_k · C1_k
   C2_total = Σ r_k · C2_k

   验证 agg.ComQXTotal == Σ r_k · ComQX_k          （G1 MSM，O(K)）
   验证 agg.Pi1Total   == Σ r_k · Pi1AggH_k         （G1 MSM，O(K)）

   D_{Y,total} = Σ r_k · (ComGY_k - [GYBeta_k]_1)  （GYBeta_k = Σ μ_k^j · yEval_k^j）

6. 约束检查（O(K) 域运算）：
   对每个 k，用 sharedAlpha/sharedBeta/nu_k/lambda_k/eta_k/gamma_k 验证代数约束

7. 派生 ρ = SHA256(ComQXTotal || D_{Y,total} || Pi1Total || ...)

8. 4-pairing 验证（常数，与 K 无关）：
   ZTG2       = [Z_T(τ_X)]_2 = G2[2] - (α+ωα)·G2[1] + α·ωα·G2[0]
   TauYBeta   = G2Y[1] - β·G2Y[0]
   D_lin      = ωα·C1_total + α·C2_total + ρ·D_{Y,total}
   D_tau      = -(C1_total + C2_total) + ρ·Pi1Total

   PairingCheck([ComQXTotal, ρ·Pi1Total, -D_lin, -D_tau],
                [ZTG2, TauYBeta, G2[0], G2[1]]) == true
```

**测试：**
```go
// TestVerifyBatch_K1：通过 CoordinateChallenges(K=1) 生成，
//   VerifyBatch 结果应与 VerifyCompressed 一致
// TestVerifyBatch_K2：K=2，不同 witness，聚合验证通过
// TestVerifyBatch_K4：K=4
// TestVerifyBatch_Tamper：篡改 proof[1] 的某个 eval，期望返回 error
```

**检查点：**
- [ ] `TestVerifyBatch_K1` 通过
- [ ] `TestVerifyBatch_K2` 通过
- [ ] `TestVerifyBatch_K4` 通过
- [ ] 篡改测试通过（返回 error）
- [ ] `go test ./bpiano/...` 全部通过（现有测试不受影响）

---

## 阶段 5：完整单元测试

**状态：** ✓ 已完成

**文件：** `bpiano/bpiano/bpiano_test.go`（追加测试函数）

测试场景汇总：

| 测试函数 | K | 说明 |
|----------|---|------|
| `TestStagesMatchCompress` | 1 | Stage1/2/3 与原 Compress 结果完全相同 |
| `TestCoordinateChallengesSharedChallenges` | 2 | 验证 K=2 两个证明共享 α、β |
| `TestVerifyBatch_K1` | 1 | 等价于 VerifyCompressed |
| `TestVerifyBatch_K2` | 2 | 不同 witness，聚合验证通过 |
| `TestVerifyBatch_K4` | 4 | 不同 witness，聚合验证通过 |
| `TestVerifyBatch_Tamper` | 2 | 篡改后聚合验证失败 |

**检查点：**
- [ ] 全部 6 个新测试通过
- [ ] `go test ./...` 无回归（现有全部测试通过）

---

## 问题清单

| # | 阶段 | 文件 | 描述 | 状态 |
|---|------|------|------|------|
| - | - | - | 无 | - |

---

## 执行记录

| 阶段 | 执行时间 | 结论 |
|------|----------|------|
| 阶段 0 | 2026-03-19 | 确认 ProveState1/2 字段完整，无遗漏 |
| 阶段 1 | 2026-03-19 | stages.go 新建；CompressStage1/2/3 + deriveNuCoord/deriveMuCoord 编译通过 |
| 阶段 2 | 2026-03-19 | aggregate.go 改写；CoordinateChallenges 三阶段协调 + deriveSharedAlpha/Beta |
| 阶段 3 | 2026-03-19 | AggregatedProof + AggregateProofs（G1 MSM）+ deriveAggCoeffs；BatchProof 别名兼容 |
| 阶段 4 | 2026-03-19 | VerifyBatch 改写为真正聚合验证（O(K) 代数约束 + 一致性校验 + 常数 4-pairing） |
| 阶段 5 | 2026-03-19 | 全部 11 项测试通过（go test ./... 无回归）；新增 5 个聚合专项测试 |
