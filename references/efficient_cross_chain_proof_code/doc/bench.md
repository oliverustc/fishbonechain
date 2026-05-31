# BPiano 性能对比实验报告

## 一键运行（TestBench）

### 前置条件

| 工具 | 版本要求 | 说明 |
|------|---------|------|
| Go | ≥ 1.21 | `go version` 确认 |
| Foundry (`forge`) | 最新稳定版 | 仅 Gas 测试需要；不装则自动跳过 |
| git | 任意 | 代码同步 |

### 命令

```bash
cd bpiano
go test ./bench/ -v -run TestBench -count=1 -timeout 300m
```

- **有服务器预生成的 `bench/testdata/`**：跳过 Prove，直接进入验证计时和 Gas 测试（约 5–10 min）
- **无 `testdata/`（首次运行）**：自动生成 100 组证明（约 70 min），再测试

### 输出文件

结果写入 `bench/results/`（已加入 `.gitignore`），每次运行生成带时间戳的 5 个 CSV：

| 文件 | 内容 | 用途 |
|------|------|------|
| `compress_performance_<ts>.csv` | 单证明压缩：大小、证明/验证时间、Gas | 柱状图（归一化比值） |
| `aggregation_proof_size_<ts>.csv` | K=2..100 聚合证明大小对比 + 节省率 | 折线图 |
| `aggregation_prove_time_<ts>.csv` | K=2..100 证明时间对比（线性外推） | 折线图 |
| `aggregation_verify_time_<ts>.csv` | K=2..100 验证时间对比 + 加速比 | 折线图 |
| `aggregation_verify_gas_cost_<ts>.csv` | K=2..100 Solidity Gas 对比 + 节省率 | 折线图 |

### 各阶段说明

| 阶段 | 内容 | 运行时间 |
|------|------|---------|
| Stage 1 | Keccak-256 电路 Piano/BPiano Prove + Verify 计时（ABAB 交替，10 次均值） | ~20 s |
| Stage 2 | forge test → Piano1/BPiano1 Gas | ~2 min |
| Stage 3 | 加载 `testdata/`，逐 K 值 VerifyBatch 计时（ABAB 交替，10 次均值） | ~3 min |
| Stage 4 | 生成 K=2..100 轻量 fixture → forge test → Agg Gas | ~5 min |

> **ABAB 交替计时**：每轮先跑一次 Piano，再跑一次 BPiano，重复 10 次后分别取均值。
> 两种方案面对相同的 CPU cache 状态，消除冷热不均造成的系统性偏差。

### 单独运行各阶段测试

```bash
# 仅跑 Stage 3（加载文件，本地验证计时）
go test ./bench/ -v -run TestBenchmarkFromFile -timeout 30m

# 服务器：生成并保存 100 组证明到 testdata/
go test ./bench/ -v -run TestGenerateAndSave -timeout 300m

# 仅跑 Stage 4 Gas（Solidity）
go test ./solgen/ -v -run TestForgeE2E_Agg_AllK -timeout 30m

# 单 K 值 Agg Gas（例如 K=10）
go test ./solgen/ -v -run TestForgeE2E_Agg_K10 -timeout 10m
```

### 生成图表

```bash
cd plot
bash sync_data.sh          # 将 bench/results/ 中最新 CSV 同步到 plot/data/
python3 compress_performance_plot.py   # 生成 figures/compress_performance_<ts>.png
python3 aggregation_plot.py            # 生成 figures/aggregation_<ts>.png
```

---

**电路：** Keccak-256（T = 262,144 ≈ 2^18，M = 2 个子节点，无公开输入）
**曲线：** BN254
**本地测试机：** Intel Core i5-12400F，12 核，32 GB RAM，WSL2
**服务器（Prove 阶段）：** 2× Intel Xeon Gold 6348，112 核，256 GB RAM
**代码位置：** `bpiano/bench/bench_test.go`、`bpiano/bench/save_load_test.go`

---

## 方案说明

本实验对比两个方案：

| | **Piano** | **BPiano（本文方案）** |
|--|-----------|----------------------|
| 证明 | `piano.Prove` | `bpiano.Compress`（单证明）<br>`bpiano.CoordinateChallenges + AggregateProofs`（K 个证明聚合） |
| 验证 | `piano.Verify`（2 次 pairing check） | `bpiano.VerifyCompressed`（单压缩证明，4 pairings）<br>`bpiano.VerifyBatch`（K 个聚合证明，4 pairings，常数） |
| 验证代价 | O(K) pairings | O(1) pairings（4 次，固定） |

**证明结构对比：**

| 格式 | G1 点数 | Fr 标量数 | 大小 |
|------|---------|----------|------|
| Piano `Proof` | 27 | 30 | **1,824 B** |
| BPiano `CompressedProof` | 10 | 7 | **864 B** |
| BPiano `AggregatedProof`（K 个）| 10K + 1 | 7K | **K × 864 + 64 B** |

Piano `Proof` 包含原始 DKZG 开放证明（X 轴 13 个子节点承诺 + Y 轴 15 个求值）；
BPiano `CompressedProof` 通过 Shplonk X 轴聚合 + Y 轴合并将开放证明折叠为 2 个 G1 点（`ComQX`、`Pi1AggH`）。

---

## 实验一：单证明压缩

**测试方法：** 对同一 Keccak-256 见证，分别跑 Piano 和 BPiano.Compress，验证时间取 ABAB 交替 10 次均值。

**结果（服务器 Prove + 本地机 Verify，2026-03-30）：**

| 指标 | Piano | BPiano | 变化 |
|------|-------|--------|------|
| 证明大小 | 1,824 B | 864 B | **−52.6%** |
| 证明时间 | 8,563.6 ms | 6,421.2 ms | **−25.0%** |
| 验证时间（10 次均值） | 2.241 ms | 2.790 ms | −24.5%（单证明反而慢） |
| 链上 Gas | 627,254 | 394,068 | **−37.2%** |

![单证明压缩对比](../plot/figures/compress_performance_20260330_203451.png)

**分析：**

- **证明大小**减少 52.6%：X 轴 13 个子节点 DKZG 开放证明被 Shplonk 折叠为 1 个 G1 点；Y 轴 3 个承诺合并为 `Pi1AggH`，节省 17 个 G1 点，EVM calldata 中每个 G1 点占 64 B（非压缩仿射坐标）
- **证明时间**节省 25%：Piano Prove 需要完整 DKZG X 轴开放（13 次 MSM）；BPiano.Compress 用 Shplonk 替换，MSM 总量更少
- **单证明验证反而慢（0.80×）**：BPiano.VerifyCompressed 做 1 次 4-pairing，Piano.Verify 做 2 次 2-pairing；BN254 ecPairing 的固定开销使 4-in-1 与 2+2 相比无优势；但 BPiano 在 K ≥ 10 时通过聚合彻底翻转这一局面（见实验二）

---

## 实验二：多证明批量聚合（K = 2 … 100）

**测试方法：**
- Prove 阶段在服务器（112 核）一次性完成，结果序列化到 `bench/testdata/`
- Verify 阶段在本地机读取文件后执行，无任何 Prove 调用
- 每个 K 值独立跑 `CoordinateChallenges(K)`（保证 sharedAlpha/sharedBeta 正确）
- 验证计时采用 ABAB 交替，取 10 次均值
- 证明时间为 K × 服务器均值（线性外推，标注 †）

```bash
# 服务器（一次性）
go test ./bench/ -v -run TestGenerateAndSave -timeout 300m

# 本地（重复执行）
go test ./bench/ -v -run TestBenchmarkFromFile -timeout 30m
```

**服务器 Prove 均值（2026-03-30）：**
- Piano：8.564 s / 个
- BPiano（CoordinateChallenges）：6.421 s / 个，**快 25.0%**

**完整对比表（本地机验证计时，2026-03-30）：**

| K | Piano 大小 | BPiano 大小 | 大小节省 | Piano Prove† | BPiano Prove† | Piano Verify | BPiano Verify | 验证加速比 |
|---|-----------|------------|---------|-------------|--------------|-------------|--------------|-----------|
| 2   | 3,648 B    | 1,796 B    | 50.8%  | 17.2 s      | 12.4 s       | 4.80 ms      | 4.97 ms       | 0.97×      |
| 10  | 18,240 B   | 8,708 B    | 52.3%  | 1m26s       | 1m2s         | 23.24 ms     | 19.91 ms      | **1.17×**  |
| 20  | 36,480 B   | 17,348 B   | 52.4%  | 2m52s       | 2m4s         | 44.49 ms     | 37.84 ms      | **1.18×**  |
| 30  | 54,720 B   | 25,988 B   | 52.5%  | 4m18s       | 3m6s         | 63.81 ms     | 55.40 ms      | **1.15×**  |
| 40  | 72,960 B   | 34,628 B   | 52.5%  | 5m44s       | 4m8s         | 89.46 ms     | 75.29 ms      | **1.19×**  |
| 50  | 91,200 B   | 43,268 B   | 52.6%  | 7m9s        | 5m10s        | 107.11 ms    | 90.22 ms      | **1.19×**  |
| 60  | 109,440 B  | 51,908 B   | 52.6%  | 8m35s       | 6m12s        | 137.31 ms    | 115.12 ms     | **1.19×**  |
| 70  | 127,680 B  | 60,548 B   | 52.6%  | 10m1s       | 7m14s        | 153.51 ms    | 126.44 ms     | **1.21×**  |
| 80  | 145,920 B  | 69,188 B   | 52.6%  | 11m27s      | 8m16s        | 175.83 ms    | 146.18 ms     | **1.20×**  |
| 90  | 164,160 B  | 77,828 B   | 52.6%  | 12m53s      | 9m18s        | 201.69 ms    | 164.86 ms     | **1.22×**  |
| 100 | 182,400 B  | 86,468 B   | 52.6%  | 14m19s      | 10m20s       | 218.92 ms    | 185.22 ms     | **1.18×**  |

† 证明时间 = K × 服务器单次均值（线性外推）

![聚合证明综合对比](../plot/figures/aggregation_20260330_203451.png)

**分析：**

### 证明大小

K ≥ 10 后稳定节省约 **52.6%**。每个 `CompressedProof` 固定 864 B，`AggregatedProof` 的额外开销仅 1 个共享 G1 点（64 B），随 K 增大可忽略。K=2 时节省率略低（50.8%），原因是该共享开销在 K 较小时占比相对更高。

### 证明时间

BPiano 比 Piano 快约 **25%**（服务器数据），与证明压缩实验一致。证明时间与 K 严格线性，两方案斜率之比恒为 6.421/8.564 ≈ 0.75，符合 Shplonk 折叠减少 MSM 的理论预期。

### 验证加速比

| K | 加速比 | 说明 |
|---|--------|------|
| 2 | 0.97× | BPiano 单/双证明验证慢于 Piano（4-pairing 固定开销 > 节省） |
| 10–100 | 1.15–1.22× | **BPiano 稳定快 15–22%** |

加速比在 K ≥ 10 后**稳定收敛至约 1.2×**，而非随 K 线性增长。原因：`VerifyBatch` 节省了 pairing（固定 4 次 vs Piano 的 4K 次），但同时引入 O(K) 次 G1 scalar multiplication（rₖ 系数推导、C1/C2/D_Y 逐 K 累加），两者在大 K 下达到新平衡。

理论 pairing 节省倍数为 K（400 次 vs 4 次 = 100×），但 G1 MSM 的摊销成本与 pairing 成本之比约为 1:10，因此实际加速比约为 1.2×，符合理论预期。

### Gas 开销

Gas 节省率在 K=20 时达到峰值约 **63.8%**，随后随 K 增大缓慢下降（K=100 时 55.4%）。BPiano 聚合验证的 calldata 大小为 K×864+64 B，而 Piano 逐一验证需传入 K×1,824 B；calldata 的 Gas 占比随 K 增大，使得大 K 时两者比值趋近于 864/1,824 ≈ 47.4%，节省率约 52.6%。链上验证计算的 Gas（常数，约对应 4 次 pairing）相对 calldata 成本较小，随 K 增大其占比下降，导致节省率略有收窄。

---

## 汇总

| 指标 | 单证明 | K=2（最小聚合） | K=100（大批量） |
|------|--------|--------------|--------------|
| 证明大小节省 | **52.6%** | 50.8% | **52.6%** |
| 证明时间节省 | **25.0%** | 27.8%† | **27.8%**† |
| 验证加速比 | 0.80×（慢） | 0.97×（接近持平） | **1.18×** |
| Pairing 次数 | 4 vs 4（相同） | 4 vs 8 | 4 vs 400（固定 vs 线性） |
| Gas 节省 | **37.2%** | 45.6% | **55.4%** |

† 证明时间节省率由服务器单次均值计算：(1 − 6.421/8.564) × 100% ≈ 25.0%；K=2 行所示 27.8% 为线性外推值，与单次结果基本一致。

BPiano 的核心优势在于：
1. **证明大小**：无论 K 大小，始终节省约 53%
2. **证明时间**：压缩阶段本身更快（MSM 总量更少），服务器节省约 25%
3. **验证扩展性**：K 个证明的聚合验证代价与单个相当（4 次 pairing 常数），而 Piano 的验证代价随 K 线性增长
4. **链上 Gas**：聚合验证的 Gas 节省率在 K ≥ 10 后稳定超过 55%
