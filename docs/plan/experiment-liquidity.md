# 资金流动性对比实验规划

**目标**：量化 FMC 按 Epoch 动态锁定 vs 传统预存押金模型的资金锁定率差异  
**依据**：fishbonechain.md §7.4 资金锁定率分析  
**状态**：规划草案 v2（2026-06-09 修订）

---

## 一、论文中的对比模型

### 1.1 传统预存押金模型（基线）

请求者创建任务时必须**一次性锁定整个任务周期的全部预算**：

```
任务创建时：LB = T × b
              ↑           ↑
          计划运行的总 epoch 数   每 epoch 预算

每 epoch 结算后：LB -= b（消耗一份）

第 i 个 epoch 结束后的锁定量：LB(i) = (T - i) × b
锁定率：LFR_trad(i) = (T - i) / T
```

初始 LFR = 100%，每 epoch 减少 1/T，到任务结束降为 0。

**本质**：合约要求请求者把"合同期内的所有工资"一次性打入托管账户，而不是按月结算。

### 1.2 FMC 动态锁定模型（本系统）

请求者只需保证 `FB ≥ b` 就能激活任务，**任意时刻锁定量固定为 1 个 epoch 的预算**：

```
activate_task()：LB += b，FB -= b
每 epoch 结算：
  → 支付工作者 bill.amount
  → LB -= b
  → FB += (b - bill.amount)    ← 未用完部分立刻归还，可用于其他任务
  → 若 FB ≥ b：自动续期，LB += b，FB -= b
```

**任意时刻锁定量**：`LB = N × b`（N = 活跃任务数，与任务已运行多少 epoch 无关）

### 1.3 论文中的数值示例（§7.4）

| 场景 | 传统模型 | FMC 模型 |
|------|---------|---------|
| 10 条子链，T=20 epoch，b=$500/epoch | 初始锁定 $100,000 | 初始锁定 $5,000（约 5%）|
| 25% 进度（epoch 5）时的锁定量 | $75,000（75%）| $5,000~$20,000（≤20%）|

> 注：FMC 的 $5,000~$20,000 范围包含 LB（$5,000 固定）+ 为维持任务持续运行所需保留的 FB 缓冲。  
> 极端情况（仅保留 1 epoch 缓冲）：锁定 = $5,000 + $5,000 = $10,000；保留 3 epoch：= $5,000 + $15,000 = $20,000。

---

## 二、指标定义

### 2.1 锁定资金占比（LFR）

```
LFR(i) = LB(i) / D
```

其中 D = 请求者的**总充值金额**（进入 FMC 的全部资金，含 FB + LB）。

### 2.2 可用资金占比（Free Ratio）

```
FR(i) = FB(i) / D = 1 - LFR(i)
```

FR 越高，请求者当前可灵活调配（激活新任务、提现、分配到其他子链）的资金越多。

### 2.3 资金使用效率（Capital Efficiency）

```
CE(i) = actual_payout(i) / LB(i)
```

每单位锁定资金在本 epoch 实际支付出去的比例。传统模型锁定了大量资金但每 epoch 实际支出仅 b，CE 随进度增加而提升；FMC 每 epoch CE ≈ utilization（实际出账/b）。

---

## 三、两种模型的理论对比

### 3.1 单任务场景（1 条子链，运行 T 个 epoch）

设总充值 D = T × b（请求者恰好充入完成整个任务所需的资金）：

| 时间点 | 传统 LFR | FMC LFR |
|--------|---------|---------|
| epoch 0（开始）| (T-0)/T = **100%** | b/(T×b) = **1/T** |
| epoch T/4（25%）| (T-T/4)/T = **75%** | 1/T |
| epoch T/2（50%）| 50% | 1/T |
| epoch 3T/4（75%）| 25% | 1/T |
| epoch T（结束）| 0% | 0%（任务终止）|

FMC 的 LFR 是一条水平线，传统的是从 100% 线性下降到 0 的斜线。

**改善比**（epoch i 时）：

```
LFR_trad(i) / LFR_FMC = (T - i) / 1 = T - i 倍
```

任务开始时改善比最大（T 倍），随任务接近完成而减小。对长任务（T 大）优势更显著。

### 3.2 多任务并发场景（N 条子链同时运行，各运行 T 个 epoch）

设 D = N × T × b（充入所有任务全部周期的总预算）：

| 时间点 | 传统 LFR（所有任务的 LB 之和 / D）| FMC LFR |
|--------|----------------------------------|---------|
| epoch 0 | N×T×b / (N×T×b) = **100%** | N×b / (N×T×b) = **1/T** |
| epoch i | (T-i)/T | 1/T（FMC 不随 epoch 进度变化）|

FMC 的关键优势在多任务场景中同样保持：**与任务数 N 无关，只取决于 T**。

### 3.3 资金复用优势

当 actual_payout < b（利用率不满时），FMC 会在每 epoch 结算后将 `(b - payout)` 归还 FB。
这部分资金可立即用于：
- 激活新任务（开新子链）
- 补充其他任务的 FB 余额
- 提现回请求者账户

传统模型中，对应的资金仍然锁定在合约内，要等到任务自然结束才能释放。

---

## 四、实验设计

### 实验 E1：单任务 LFR 时序（核心实验）

**目的**：实测 FMC 的 FB/LB 随 epoch 变化曲线，与同等条件下传统模型的理论曲线对比。

**参数**：
- 子链：child4（AURA-7，6s，已稳定运行的最优数据）
- b = 5,000 UNIT，T = 10（计划运行 10 个 epoch）
- D = T × b = 50,000 UNIT（重新 deposit 或核查当前状态）
- workers：100（scenario d，满负荷，utilization ≈ 1.0）

**数据采集**：

新增 `scripts/fmc_metrics.js`，每 30 秒轮询主链存储：
```
timestamp, epoch_id, fb, lb, total_deposited, lfr, actual_payout, utilization
```
- `fb`, `lb`：来自 `api.query.fmc.fundPools(alice)`
- `epoch_id`：来自 `api.query.fmc.tasks(alice, task_id).current_epoch`
- `actual_payout`：来自 `BillSettled` 事件的 `total_paid`

**关键图形（图7）**：
```
X 轴：epoch 编号（0 ~ 10）
Y 轴：锁定率 LFR（%）

曲线 1（实测）：FMC LFR = lb / D       ← 水平线，约 10%
曲线 2（模拟）：传统 LFR = (T-i)/T    ← 从 100% 线性下降
阴影区域：两条线之间的"节省面积"
```

**预期结果**（D = 50,000 UNIT，b = 5,000 UNIT，T = 10）：
- FMC 实测 LFR ≈ 10%（b/D，恒定）
- 传统模型：epoch 0 = 100%，epoch 5 = 50%，epoch 10 = 0%
- 平均改善：FMC 比传统减少锁定约 **45 个百分点**（传统平均 50%，FMC 10%）

---

### 实验 E2：多任务并发 LFR（扩展实验）

**目的**：展示 N 条子链同时运行时，FMC 锁定率的多任务扩展特性。

**参数**：同时运行 child1/child3/child4/child6（N=4），各 T=10 epoch，各 b = 各链预算

**对比**：
- 传统：初始锁定 = N × T × b = 4 × 10 × b，LFR = 100%
- FMC：锁定 = N × b，LFR = N×b / (N×T×b) = 1/T ≈ 10%

---

### 实验 E3：低利用率下的资金复用优势

**目的**：验证当实际出账 < b 时，FMC 的 FB 自动增加（传统模型则不会）。

**参数**：child4，workers=10（低负荷，utilization ≈ 10%，payout ≈ 500 UNIT/epoch）

**测量**：
- FMC：每 epoch 结算后 FB 净增量 = b - payout ≈ 4,500 UNIT；这部分可立即用于其他任务
- 传统：LB 虽每 epoch 减少 b，但 FB 不增加（因为 payout < b 的差额在传统模型中无处可去）

> 注：此处"传统模型"的定义是：LB 预先锁定 T×b，每 epoch 从 LB 中取出 b 支付，remainder 无归处（合约内滞留，直到任务结束才退还）。

---

## 五、需要实现的脚本

### 5.1 `scripts/fmc_metrics.js`（新建，最重要）

```javascript
// 功能：轮询主链 FMC 存储 + 监听 BillSettled 事件
// 输出：CSV，每行 = 一个采样点

// 字段：
// timestamp        ISO 时间戳
// epoch_id         当前任务所在 epoch（来自 fmc.tasks.current_epoch）
// fb               free balance（planck）
// lb               locked balance（planck）
// total_deposited  fb + lb
// lfr              lb / (fb + lb)
// bill_settled     本 epoch 是否触发了 BillSettled（0/1）
// actual_payout    BillSettled.total_paid（触发时记录，否则 0）
// utilization      actual_payout / budget_per_epoch

// 用法：
// MAIN_WS=ws://10.2.2.11:9944 REQUESTER=5Grw... TASK_ID=3 \
//   node scripts/fmc_metrics.js --out /tmp/exp_e_fmc.csv --interval 30
```

实现要点：
- 轮询 `api.query.fmc.fundPools(REQUESTER)` 得到 `{free, locked}`
- 轮询 `api.query.fmc.tasks(REQUESTER, TASK_ID)` 得到 `{current_epoch, budget_per_epoch, status}`
- 订阅 system.events，过滤 `fmc.BillSettled`，记录 `total_paid`
- 两路数据合并到同一 CSV 行

### 5.2 `scripts/simulate_traditional.py`（新建）

```python
# 功能：基于 fmc_metrics 采集的逐 epoch 数据，模拟传统预存模型的 LFR
# 输入：exp_e_fmc.csv（含 epoch_id, actual_payout, budget_per_epoch）
# 输出：对比 CSV + 图7

def simulate_traditional(payouts, budget_per_epoch, T, D):
    """
    payouts[i]：第 i 个 epoch 实际支出（FMC 实测）
    T：传统模型预设的总 epoch 数
    D：总充值金额 = T × budget_per_epoch
    """
    lb = T * budget_per_epoch   # 初始全部锁定
    results = []
    for i, payout in enumerate(payouts):
        lfr = lb / D
        results.append({'epoch': i, 'lb': lb, 'lfr': lfr})
        lb = max(0, lb - budget_per_epoch)  # 消耗一个 epoch 预算
    return results
```

### 5.3 `scripts/plot_results.py` 扩展

新增 `fig7_liquidity_comparison()` 函数：
- 读取 `docs/figures/data/exp_e_fmc.csv`（实验 E1 FMC 实测数据）
- 调用 `simulate_traditional()` 生成对照曲线
- 输出 `docs/figures/fig7_liquidity_comparison.png`

图形设计：
```
双 Y 轴图或双子图：
  上图：LFR 随 epoch 变化
    - 实线（蓝）：FMC 实测 LFR（水平线附近）
    - 虚线（橙）：传统模型模拟 LFR（从 100% 线性下降）
    - 阴影：两者之间的"节省区域"

  下图：FB 可用资金随 epoch 变化
    - 蓝：FMC FB（高，接近 D-b）
    - 橙：传统模型 FB（0，始终无可用资金）
```

---

## 六、实验执行顺序

### 前置检查（~30 分钟）

- [ ] 确认 child4 仍在出块：`ssh bcg "ssh -i ~/.ssh/debian-dev debian@10.2.2.11 'systemctl status fishbone-child4'"`
- [ ] 确认主链 task_id=3 状态（Activated / Terminated）
  - 若已 Terminated（资金耗尽或未续期）：调用 `fmc.activateTask(3)` 重新激活
  - 确认 Alice 的 FB ≥ b（若不足，先 `fmc.deposit`）
- [ ] 确认 BillSettled 事件从未触发过（因为 bridge 之前有 bug），LB 可能已经停在激活后的初始状态

### Step 1：实现并测试 fmc_metrics.js（~2 小时）

- [ ] 编写 `scripts/fmc_metrics.js`（见 5.1 节）
- [ ] 在本地 WSL 连接 f1 测试：查询一次 FMC 存储，确认字段正确
- [ ] 测试 BillSettled 事件订阅（可先用 `--dry-run` 只打印不写文件）

### Step 2：运行实验 E1（~2.5 小时）

```bash
# 同时启动三个进程（在 f1 上或通过 SSH 代理）：

# 1. FMC 主链指标采集
MAIN_WS=ws://10.2.2.11:9944 REQUESTER=5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY \
TASK_ID=3 node scripts/fmc_metrics.js --out /tmp/exp_e_fmc.csv --interval 30

# 2. bridge（带多矿工投票，确保账单结算）
MINER_SURIS="0x52390bf...,0x17f21ff...,0x41517...,0x8b7aeb...,0x69416e..." \
CHILD_WS=ws://10.2.2.11:9948 MAIN_WS=ws://10.2.2.11:9944 \
TASK_ID=3 CHAIN_ID=3 node scripts/bridge.js

# 3. workers（满负荷，100 workers）
node scripts/worker.js --scenario d --workers 100 \
  --ws ws://10.2.2.11:9948 --task-id 3
```

目标：观察到 10 次 BillSettled 事件，下载 `/tmp/exp_e_fmc.csv`

### Step 3：分析与出图（~1 小时）

- [ ] 实现 `scripts/simulate_traditional.py`
- [ ] 在 `plot_results.py` 中新增 `fig7_liquidity_comparison()`
- [ ] 生成图7，确认 FMC 曲线为水平线、传统曲线线性下降

### Step 4：更新报告

- [ ] 在 `docs/experiment-report.md` 增加实验 E/F 章节
- [ ] 引用图7，说明 FMC 在 T=10 时比传统节省约 45 个百分点的平均锁定率

---

## 七、预期图形与关键数据点

```
锁定率 LFR（%）
100 ┤╲  ← 传统模型（从 100% 线性下降）
 90 ┤ ╲
 80 ┤  ╲
 70 ┤   ╲  阴影区域 = FMC 节省的
 60 ┤    ╲  资金锁定
 50 ┤     ╲
 40 ┤      ╲
 30 ┤       ╲
 20 ┤        ╲
 10 ┤─────────╲──────── ← FMC（约 10%，恒定）
  0 ┤          ╲_____
    └──────────────────→ epoch
    0    2    4    6    8   10
```

| epoch | 传统 LFR | FMC LFR | 节省 |
|-------|---------|---------|------|
| 0 | 100% | 10% | **90%** |
| 2 | 80% | 10% | 70% |
| 5 | 50% | 10% | 40% |
| 8 | 20% | 10% | 10% |
| 10 | 0% | 0% | — |
| **平均** | **50%** | **10%** | **40%** |

---

## 八、与现有实验的关系

| 实验 | 已验证的论文论点 | 本实验引用 |
|------|---------------|-----------|
| C/D | 多链并发吞吐线性扩展 | E2 的 N=4 并发数据可引用 exp_c |
| **E1（新）** | **FMC LFR 水平线（约 1/T）** | 核心新贡献 |
| **E2（新）** | **多任务并发下 LFR 仍为 1/T** | E2 扩展 |
| **E3（新）** | **低利用率下 FB 自动回收** | E3 补充 |

---

## 九、潜在问题

### 9.1 BillSettled 是否能正常触发？

实验 E1 **依赖** bridge.js 的 NotAMiner bug 已被修复（✓ 已修复，2026-06-09）。  
MINER_SURIS 需要包含 f1-f5 的真实 seed（来自 setup_experiment.js），满足 child4 的 5/7 投票阈值。

### 9.2 任务 task_id=3 的当前状态

实验 C 期间 bridge 未能触发 BillSettled，pallet 的自动续期逻辑也未执行。需检查：
- `fmc.tasks(Alice, 3).status` 是否仍为 Activated
- `fmc.fundPools(Alice).locked` 是否还有 5,000 UNIT
- 如已 Terminated（LB=0，FB=D），需重新 activateTask

### 9.3 D 的设定

当前 Alice 充值了 500,000 UNIT，而 b = 5,000 UNIT，D 的规模太大导致 LFR 数字非常小（1%）。  
为了让图形更直观，有两个选项：

**选项 A（推荐）**：将图形的 Y 轴改为绝对金额（UNIT），而非比率。展示：
- FMC LB 恒为 5,000 UNIT  
- 传统模型 LB 从 50,000 UNIT 线性下降到 0

**选项 B**：重新设定"对比实验参数"，将 D 设为 T×b = 50,000 UNIT，使 LFR 归一化到合理范围。

选项 A 更真实（反映实际实验状态），选项 B 更符合论文的理论分析框架，图形更清晰。
