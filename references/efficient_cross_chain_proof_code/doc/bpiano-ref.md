# BPiano 方案参考手册

> 来源：paper.md（高效验证的跨链状态证明协议）

---

## 一、方案定位

基于 Pianist 协议的**证明压缩 + 批量聚合**方案。目标：常数级验证时间和证明大小。

- 单个压缩证明：验证复杂度 O(1)𝕡
- 批量验证 K 个证明：配对次数仍为 O(1)（与 K 无关）

---

## 二、Pianist 协议回顾（前置）

Pianist 使用**二元多项式** F(Y,X) = Σᵢ Rᵢ(Y)·fᵢ(X)，其中：
- M 个子节点，各持 fᵢ(X)（T 次）
- Rᵢ(Y) 为 Y 轴拉格朗日基（M 次单位根）
- 主节点聚合全局承诺

Pianist 证明包含：
- 承诺：{comA, comB, comO, comZ, comHX, comHY}
- 第一层（X轴）开启证明：{π₀,A, π₀,B, π₀,O, π₀,Z, π₀,HX, π₀,Z'}（6个G₁元素）
- 第二层（Y轴）开启证明：{π₁,A, π₁,B, π₁,O, π₁,Z, π₁,HX, π₁,HY, π₁,Z'}（7个G₁元素）
- 求值：{A(β,α), B(β,α), O(β,α), Z(β,α), Z(β,ωα), HX(β,α), HY(β,α)}

---

## 三、证明压缩

### 3.1 X 轴多点聚合（Shplonk）

打开多项式分两组：
- Sα = {A, B, O, Z, HX}，在 X = α 处打开
- Sω = {Z}，在 X = ωXα 处打开

**聚合商多项式**（全局）：
```
QX(Y,X) = Σ_{S∈Sα} ν^idx(S) · (S(Y,X) - vS(Y))/(X-α)
         + ν^idx(Z') · (Z(Y,X) - vZ'(Y))/(X-ωXα)
```
其中 vS(Y) = S(Y,α) 是 S 在 X=α 处关于 Y 的一元多项式求值。

**子节点本地商多项式**：
```
QX,i(X) = Σ_{S∈Sα} ν^idx(S) · (Sᵢ(X) - Sᵢ(α))/(X-α)
         + ν^idx(Z') · (Zᵢ(X) - Zᵢ(ωXα))/(X-ωXα)
```

子节点提交 comQX,i，主节点聚合：comQX = ∏ comQX,i

### 3.2 Y 轴聚合

主节点获得 Y 轴多项式集合：
```
VY = {vA(Y), vB(Y), vO(Y), vZ(Y), vZ'(Y), HY(Y,α)}
```
（共 6 个关于 Y 的一元多项式，全部在 β 处打开）

引入挑战 μ，聚合：
```
GY(Y) = Σ_{P∈VY} μ^idx(P) · P(Y)
QY(Y) = (GY(Y) - GY(β)) / (Y - β)
```
Y 轴聚合商承诺：`π₁,agg = g^{QY(τY)}`

另外承诺 comGY = g^{GY(τY)}（用于验证方程）

### 3.3 压缩证明格式

```
πc = {
  承诺: comA, comB, comO, comZ, comHX, comQX, comGY
  开启证明: π₁,agg
  求值: A(β,α), B(β,α), O(β,α), Z(β,α), Z(β,ωα), HX(β,α), HY(β,α)
}
```
大小约：**12|G₁| + 7|F|** ≈ 608 字节（BN254）

### 3.4 验证方程（4 配对合并）

X 轴 Shplonk 验证：
```
e(comQX, [ZT(τX)]₂) = e(C₁, [τX-α]₂) · e(C₂, [τX-ωXα]₂)
```
其中：
- ZT(X) = (X-α)(X-ωXα)，即 [ZT(τX)]₂ = [(τX-α)(τX-ωXα)]₂
- C₁ = Σ_{S∈Sα} ν^idx(S) · (comS - [vS(β)]₁)
- C₂ = ν^idx(Z') · (comZ - [vZ'(β)]₁)

Y 轴验证：
```
e(π₁,agg, [τY-β]₂) = e(comGY - [GY(β)]₁, g₂)
```

引入随机数 ρ 合并为 **4 配对检查**：
```
e(comQX, [ZT(τX)]₂) · e(π₁,agg, [τY-β]₂)^ρ = e(D_lin, g₂) · e(D_tau, [τX]₂)
```
其中：
- **D_lin = C₁·ωXα + C₂·α + ρ·(comGY - [GY(β)]₁)**
- **D_tau = -(C₁ + C₂) + ρ·π₁,agg**

---

## 四、证明聚合（批量验证）

### 4.1 前提：共享挑战点 α, β

K 个证明必须使用**相同的 α 和 β**，通过两轮挑战协调实现。

**X 轴挑战协调**：
```
α = H(pp, comHX^(0), comHX^(1), ..., comHX^(K-1))
```

**Y 轴挑战协调**：
```
β = H(pp, α, comQX^(0), VX^(0), ..., comQX^(K-1), VX^(K-1))
```

### 4.2 承诺聚合

聚合系数：rk = H(πc^(0), ..., πc^(K-1), k)

```
comQX,total  = Σ rk · comQX^(k)
C₁,total     = Σ rk · C₁^(k)
C₂,total     = Σ rk · C₂^(k)
π₁,total     = Σ rk · π₁,agg^(k)
DY,total     = Σ rk · (comGY^(k) - [GY^(k)(β)]₁)
```

### 4.3 批量验证流程

1. **Fiat-Shamir 重算**：重算 α, β, ηk, λk, νk, μk, rk
2. **聚合一致性验证**：O(K) 次椭圆曲线运算，验证 total 量的正确性
3. **约束检查**（逐个，共 K 次）：
   ```
   G^(k)(β,α) + λ^(k)·P₀^(k)(β,α) + λ^(k)²·P₁^(k)(β,α)
     = VX(α)·HX^(k)(β,α) + VY(β)·HY^(k)(β,α)
   ```
4. **配对检查**（仅 4 次，与 K 无关）：
   ```
   e(comQX,total, [ZT(τX)]₂) · e(π₁,total, [τY-β]₂)^ρ
     = e(D_lin, g₂) · e(D_tau, [τX]₂)
   ```
   其中 D_lin, D_tau 与单个证明格式相同，但用 total 量替换。

### 4.4 批量证明格式

```
πbatch = {
  各证明承诺: {comA^(k), comB^(k), ..., comGY^(k)} × K
  聚合开启:   comQX,total, π₁,total
  各证明求值: {A^(k)(β,α), ..., HY^(k)(β,α)} × K
}
```
大小：(11K + 2)|G₁| + 7K|F|

---

## 五、复杂度总结

| 指标 | 单个压缩证明 | 批量 K 个证明 |
|------|-------------|---------------|
| 证明大小 | O(1) ≈ 608B | O(K) |
| 验证配对 | O(1) = 4次 | O(1) = 4次 |
| 验证总复杂度 | O(1)𝕡 + O(1)𝔾 | O(1)𝕡 + O(K)𝔾 |
| 子节点通信 | O(M)|G| | O(M)|G| |
| 主节点协调 | — | O(K)|G| |

---

## 六、实现对应关系

| 论文符号 | 代码位置 |
|---------|---------|
| QX,i(X) | `compress.go: computeShplonkQuotient()` |
| comQX | `compress.go: ComQX` |
| GY(Y) | `compress.go: foldedHyLag` + 其他 Y 多项式 |
| π₁,agg | `compress.go: Pi1AggH` |
| comGY | `compress.go: ComGY` |
| D_lin, D_tau | `verify.go` 配对检查部分 |
| vS(β) = GY^(k)(β) | `verify.go: EvalGY` |
| ν (Shplonk 挑战) | `compress.go/verify.go: nu` |
| μ (Y 轴聚合挑战) | `compress.go/verify.go: mu` |
| ρ (配对合并挑战) | `verify.go: rho` |

---

## 七、关键数值（实现参数）

- 椭圆曲线：BN254，|G₁| = 32 字节，|F| = 32 字节
- Sα 中多项式数量：5（A, B, O, Z, HX）+ 1（Z 在 ωα 处）= 6个 ν 权重
- VY 中 Y 多项式数量：6（vA, vB, vO, vZ, vZ', HY_alpha）个 μ 权重
- 验证配对：始终恰好 4 次
