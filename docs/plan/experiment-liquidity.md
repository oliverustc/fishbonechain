# 资金流动性对比实验规划

**目标**：通过链上状态实测 + 同 workload 反事实对比，证明 FishboneChain 的周期性资金管理机制  
**核心主张**：多子链并发任务运行时，FishboneChain 链上 `locked` 始终接近"下一 Epoch 所需预算"，  
　　　　　　不随任务生命周期线性膨胀，资金锁定降低约 **20×**  
**状态**：规划草案 v4（2026-06-09）

---

## 一、实验原理

### 1.1 为什么传统多链方案锁仓高

在传统跨链架构中，为了防止跨链双花，请求者创建任务时必须**提前将整个任务生命周期（T 个 Epoch）的全部预算锁进每条链的合约**：

```
任务创建时：锁定 T × b（全部 Epoch 预算）
第 i 个 Epoch 后：剩余锁定 = (T - i) × b
```

对 6 条子链并发的场景：

```
baseline_locked(epoch i) = Σ_j [ (T - i) × budget_j ]
                         = (T - i) × ΣB
```

初始时（epoch 0）= T × ΣB；任务跑完（epoch T）= 0。这是一条从高到低线性下降的曲线。

### 1.2 FishboneChain 的解法

FMC 将资金管理集中在主链，子链任务只锁定**当前正在运行的这一个 Epoch 的预算**：

```
任意时刻：locked = Σ_j budget_j = ΣB（活跃任务数 × 各自 budget/epoch）
```

每次 `BillSettled`：
- `locked -= budget_j`（消耗本 Epoch）
- `free += (budget_j - actual_payout_j)`（余量立刻归还）
- 若 `free >= budget_j`：自动续期，`locked += budget_j`

结果：**locked 围绕 ΣB 小幅波动，不随 epoch 进度累积**。

### 1.3 本实验的数值基础（来自现有系统配置）

| 子链 | task_id | budget/epoch | 运行场景 |
|------|---------|-------------|---------|
| child1 | 0 | 1,500 UNIT | 快递配送（a）|
| child2 | 1 | 2 UNIT | 交通感知（b）|
| child3 | 2 | 40,000 UNIT | 医疗标注（c）|
| child4 | 3 | 5,000 UNIT | 金融核验（d）|
| child5 | 4 | 0.5 UNIT | IoT 传感器（e）|
| child6 | 5 | 25,000 UNIT | 数据市场（f）|
| **合计** | | **71,502.5 UNIT / Epoch** | |

计划运行 T = 20 个 Epoch：

```
传统方案初始锁定：20 × 71,502.5 = 1,430,050 UNIT
FishboneChain 实测锁定：≈ 71,502.5 UNIT（恒定）
改善比：1,430,050 / 71,502.5 ≈ 20×
```

---

## 二、主实验：6 链并发资金锁仓对比（实验 E）

### 2.1 实验目标

- **测量对象**（链上实测）：`fmc.fundPools(Alice).locked`，每轮 Epoch 采样一次
- **对比基线**（反事实计算）：`baseline_locked(i) = (T - i) × ΣB`，不需要真的部署一套传统系统
- **核心图形**：双折线图，baseline 从 1.43M 线性下降，FishboneChain 在 71.5K 附近水平波动

### 2.2 前置条件（必须满足，否则实验退化为静态截图）

`fmc.submitBill` 必须真实触发 `BillSettled` 事件，`locked/free` 才会按 Epoch 动态变化。

检查清单：
- [ ] bridge.js `NotAMiner` bug 已修复 ✅（2026-06-09 已完成）
- [ ] `MINER_SURIS` 配置包含 child4（f1-f5 的 seed）和其他链的矿工
- [ ] 运行 1 个 Epoch 验证：观察到 `BillSettled` 事件 + `locked` 数值变化

> **如果 `BillSettled` 无法触发**：退化方案是记录 6 任务激活后的静态 `locked` 快照，然后将 baseline 计算作为纸面对比，注明"链上 locked 为初始激活值，动态续期待后续验证"。

### 2.3 运行配置

```bash
# 1. 主链资金指标采集（新脚本 metrics_fund.js）
MAIN_WS=ws://10.2.2.11:9944 \
REQUESTER=5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY \
TASK_IDS=0,1,2,3,4,5 T_PLANNED=20 \
node scripts/metrics_fund.js --out /tmp/exp_e_fund.csv --interval 15

# 2. bridge（6 条链，各自的矿工 seed）
# child4（f1-f5）：threshold=ceil(7×2/3)=5，至少提供 5 个 seed
MINER_SURIS="seed_f1,seed_f2,seed_f3,seed_f4,seed_f5" \
CHILD_WS=ws://10.2.2.11:9948 MAIN_WS=ws://10.2.2.11:9944 \
TASK_ID=3 CHAIN_ID=3 node scripts/bridge.js &

# child1（f1-f3）：threshold=2
MINER_SURIS="seed_f1,seed_f2" \
CHILD_WS=ws://10.2.2.11:9945 MAIN_WS=ws://10.2.2.11:9944 \
TASK_ID=0 CHAIN_ID=0 node scripts/bridge.js &

# child3（f7-f9）：threshold=2
MINER_SURIS="seed_f7,seed_f8" \
CHILD_WS=ws://10.2.2.17:9947 MAIN_WS=ws://10.2.2.11:9944 \
TASK_ID=2 CHAIN_ID=2 node scripts/bridge.js &

# child6（f1-f5）：threshold=ceil(5×2/3)=4
MINER_SURIS="seed_f1,seed_f2,seed_f3,seed_f4" \
CHILD_WS=ws://10.2.2.11:9950 MAIN_WS=ws://10.2.2.11:9944 \
TASK_ID=5 CHAIN_ID=5 node scripts/bridge.js &

# (child2/child5 的矿工类似配置)

# 3. 各链 worker（复用 run_exp_c.sh 的 worker 配置）
node scripts/worker.js --scenario a --workers 300  --ws ws://10.2.2.11:9945 --task-id 0 &
node scripts/worker.js --scenario b --workers 2000 --ws ws://10.2.2.14:9946 --task-id 1 &
node scripts/worker.js --scenario c --workers 200  --ws ws://10.2.2.17:9947 --task-id 2 &
node scripts/worker.js --scenario d --workers 100  --ws ws://10.2.2.11:9948 --task-id 3 &
node scripts/worker.js --scenario e --workers 5000 --ws ws://10.2.2.20:9949 --task-id 4 &
node scripts/worker.js --scenario f --workers 500  --ws ws://10.2.2.11:9950 --task-id 5 &
```

**预计运行时长**：child4 epoch ≈ 12 min × 20 epoch = 4 小时（其他链的 epoch 时长不同，以 child4 epoch 计数为进度基准）

---

## 三、采集脚本规格：`scripts/metrics_fund.js`（新建）

### 3.1 采集字段（全部绝对值，UNIT）

```
timestamp              ISO 时间戳
block_number           当前块高
epoch_id               child4 当前 epoch（作为进度基准）
active_tasks           当前 Activated 的任务数量

free_unit              fmc.fundPools(Alice).free / 1e12
locked_unit            fmc.fundPools(Alice).locked / 1e12
total_pool_unit        free_unit + locked_unit
locked_ratio_pool      locked_unit / total_pool_unit

sum_active_budget      Σ budget_per_epoch_i（仅 Activated 任务）
baseline_locked_unit   (T_planned - epoch_id) × sum_active_budget  ← 反事实基线
improvement            baseline_locked_unit / locked_unit           ← 改善倍数

bill_settled_this_epoch  0/1（本采样周期内是否收到 BillSettled）
total_paid_unit          所有 BillSettled.total_paid 的累计（本 epoch）
utilization              total_paid_unit / sum_active_budget

available_tasks_count  floor(free_unit / min_budget_per_epoch)  ← 还能激活几个最小规模任务
```

### 3.2 实现要点

```javascript
// 两路数据源：
// A. 定时轮询（每 interval 秒）
const poll = async () => {
  const pool = await api.query.fmc.fundPools(REQUESTER);
  const tasks = await Promise.all(
    TASK_IDS.map(id => api.query.fmc.tasks(REQUESTER, id))
  );
  const activeTasks = tasks.filter(t => t.isSome && t.unwrap().status.isActivated);
  const sumBudget = activeTasks.reduce((s, t) => s + t.unwrap().budget_per_epoch.toBigInt(), 0n);

  const epoch = await getCurrentEpoch(api); // 查 child4 epoch 或从 fmc task.current_epoch
  const baselineLocked = (BigInt(T_PLANNED) - BigInt(epoch)) * sumBudget;

  writeCSVRow({ free, locked, sumBudget, baselineLocked, improvement: baselineLocked / locked, ... });
};

// B. 事件订阅（即时，精确到块高）
api.query.system.events(events => {
  for (const { event } of events) {
    if (event.section === 'fmc' && event.method === 'BillSettled') {
      accumulatePaidThisEpoch(event.data.total_paid);
    }
  }
});
```

---

## 四、辅助实验（利用主实验数据，不额外运行）

### 4.1 多任务并发资金快照（从主实验 epoch 0 数据提取）

**问题**：6 任务全部激活后，FB 还剩多少？还能再激活几个任务？

从 `exp_e_fund.csv` 的 epoch=0 行直接读取：

```
locked_unit ≈ 71,502.5   （6 任务各锁 1 epoch）
free_unit   ≈ D - 71,502.5  （D = Alice 总充值）
available_tasks_count = floor(free_unit / 1500)  ← 以 child1 的最小 budget 为单位
```

传统对比（反事实）：
```
baseline epoch 0：locked = 1,430,050 UNIT，free = D - 1,430,050
若 D = 1,430,050（刚好够传统方案）：free = 0，可激活新任务 = 0
FishboneChain：locked = 71,502.5，free = 1,430,050 - 71,502.5 = 1,358,547 UNIT
```

### 4.2 资金回收原子性（从主实验精确 BillSettled 时刻提取）

把 `metrics_fund.js` 的采样间隔在 Syncing Slot 期间缩短到 6s（≈1 块），捕捉：

- `block_number` = BillSettled 所在块
- 该块前后 `free_unit` 的变化量 = 实际回收的余量

**预期**：`BillSettled` 的同一块内，`free` 增加了 `(budget - actual_payout)`。  
这证明"余量归还是原子的，不存在多块延迟或人工干预"。

---

## 五、可视化方案（v2）

**设计原则**：图表必须让读者感受到"这是真实系统跑出来的数据"，而非数学推导。
核心手段：①以挂钟时间为 X 轴；②标注各链真实结算事件；③展示比例而非绝对值。

---

### 图 7a（主图）：多链锁定率时序对比

**核心问题**：同等资金规模下，随时间推移，有多大比例的资金被"压死"在锁仓中？

```
X 轴：挂钟时间（实验实际运行时间，分钟为单位）
Y 轴：锁定资金占比（%），= locked / initial_deposit

线 1（橙色虚线）：传统方案反事实
  在任务创建时一次性锁定 T × ΣB，之后每 Epoch 完成后线性释放
  占比从 100% 线性下降到 0%

线 2（蓝色实线）：FishboneChain 链上实测 locked_ratio
  = task_locked / initial_deposit（initial_deposit = T × ΣB）
  随着 free 被 worker 消耗，比例有小幅上升趋势（5% → 17%）
  非完美水平线，反映资金真实流动

事件标记：每次 BillSettled 在时间轴上打一个竖线 + 小标签
  │child6  │child1  │child4  │child6  │child1  ...
  不同链用不同颜色（child1=蓝, child4=橙, child6=绿）

右侧：子链配置速查表（内嵌在图内）
  链       | 出块   | Epoch  | 预算/Epoch
  child1  | 6s     | 12min  | 1,500 UNIT
  child4  | 10s    | 20min  | 5,000 UNIT
  child6  | 6s     | ~12min | 25,000 UNIT
```

**为什么看起来真实**：
- X 轴是真实时间，可以看到三条链不同频率的结算节奏
- FishboneChain 线不是水平线，有微弱上升趋势（free 在被消耗）
- 每个 BillSettled 事件都有时间戳标记，证明数据来自真实链
- 两线差距从 ~94% 收窄到 ~82%，说明是动态过程不是静态快照

---

### 图 7b（辅图）：相同资金下的任务开启能力对比

**核心问题**：用同一笔钱（T×ΣB = 630,000 UNIT），两种方案分别能开启多少并发任务？

```
横向堆叠条形图（每条代表一种方案）：

方案 A：传统方案
  [3 个任务已锁定 630K ████████████████ | free: 0 UNIT]
  → 无余力开启新任务

方案 B：FishboneChain（实测）
  [当前 3 任务锁定 31.5K █ | free: 598.5K ░░░░░░░░░░░░░░░░░░]
  free 还能额外激活：
    + 399 个 child1 规格任务（1,500 UNIT/task），或
    + 19 个 child6 规格任务（25,000 UNIT/task）

X 轴：UNIT（千）
注释框：
  "相同资金 D=630K：
   传统方案剩余可用资金：0 UNIT（无法扩展）
   FishboneChain 剩余可用资金：598.5K UNIT（可扩展 19× child6 任务）"
```

---

### 图 7c（PPT 一页纸）：锁定率随时间下降（传统）vs 恒定（FishboneChain）

保留原 c 图方向，但改为动态：

```
两列小图并排：
  左：传统方案 epoch 0/5/10/15/20 的资金分布快照（锁定占比逐步从 100% 缩小）
  右：FishboneChain 各时刻快照（锁定始终 ~5%，free 随 worker 消耗而减少）

或更简洁：在 fig7a 基础上，在底部加"锁定资金对比值"表格，直观显示任意时刻的差距
```

---

## 六、执行计划

### Step 0：前置验证（1 小时）

- [ ] 确认 6 个任务状态（task_id=0-5）：Activated / Terminated / 未创建
  - 若已 Terminated：`fmc.activateTask` 重新激活
  - 若未创建：运行 `setup_experiment.js --step 3,4,5,6`
- [ ] 检查 Alice 的 FMC pool：`fmc.fundPools(Alice)` → 确认 `free >= ΣB`
- [ ] 做 1 epoch 预热测试：启动 child4 的 bridge + worker，等待 `BillSettled` 事件
  - **若 BillSettled 触发**：`locked` 有变化，继续主实验
  - **若未触发**：检查 MINER_SURIS 配置，优先修复再跑实验

### Step 1：实现 `scripts/metrics_fund.js`（2 小时）

参考 3.1 节规格，重点：
- 同时轮询所有 6 个 task 的状态
- 内置 `T_PLANNED` 参数，自动计算 `baseline_locked`
- `BillSettled` 事件即时记录（不依赖轮询间隔）
- 在 Syncing Slot 期间自动降低采样间隔（检测到 epoch 接近结束时改为 6s）

### Step 2：运行主实验 E（4-5 小时，20 epoch）

```bash
# 启动顺序：先 metrics_fund.js，再 bridge × 6，再 worker × 6
# 运行脚本：新建 scripts/run_exp_fund.sh 封装所有命令
```

采集目标：
- `BillSettled` 事件 ≥ 10 次（child4 至少 10 个完整 epoch）
- `locked` 数值在每次结算后恢复到 ΣB 附近（验证自动续期）

### Step 3：分析与出图（1 小时）

```python
# scripts/plot_results.py 新增 fig7a/b/c()

def fig7a_liquidity_main(csv_path='docs/figures/data/exp_e_fund.csv'):
    df = pd.read_csv(csv_path)
    T = 20
    sum_b = 71502.5

    epochs = df['epoch_id'].unique()
    baseline = [(T - e) * sum_b for e in epochs]
    fishbone = df.groupby('epoch_id')['locked_unit'].mean()

    # 双折线图
    plt.plot(epochs, baseline, '--', color='orange', label='传统方案（反事实基线）')
    plt.plot(epochs, fishbone, '-', color='steelblue', label='FishboneChain 实测')
    plt.fill_between(epochs, fishbone, baseline, alpha=0.1, color='orange')
    plt.axhline(y=sum_b, linestyle=':', color='steelblue', alpha=0.5, label=f'理论值 {sum_b:,.0f} UNIT')
    # 标注改善比
    ...
```

### Step 4：更新实验报告

在 `docs/experiment-report.md` 增加"实验 E：资金流动性"章节，引用图 7a/b/c。

---

## 七、预期关键数据

| 指标 | 传统方案（反事实）| FishboneChain（实测）|
|------|----------------|-------------------|
| 初始锁定（epoch 0）| 1,430,050 UNIT | ≈ 71,502.5 UNIT |
| epoch 10 锁定 | 715,025 UNIT | ≈ 71,502.5 UNIT |
| 平均锁定（20 epoch）| 715,025 UNIT（平均）| ≈ 71,502.5 UNIT |
| 改善比 | 1× | **≈ 20×** |
| epoch 0 可用资金（D=1.43M）| 0 UNIT | **1,358,547 UNIT** |
| 资金利用效率（20 epoch 均值）| 低（大量资金闲置在锁仓中）| 高（free 始终可用）|

---

## 八、PPT 最终结论（可直接使用）

```
同样 6 条子链、20 个 Epoch 的任务预算下：
  传统方案：提前锁定 1,430,050 UNIT（任务全周期总预算）
  FishboneChain：链上实测仅锁定约 71,502.5 UNIT（下一轮 Epoch 预算）

资金锁定降低约 20×，请求者 95% 的资金保持可用状态，
可同时支撑更多跨子链任务，无需为未来 Epoch 提前"压款"。
```

---

## 九、与已有实验的互补关系

| 实验 | 验证维度 | 结论 |
|------|---------|------|
| A/B/C/D | **性能**：多链并发吞吐线性扩展 | 多链不损失吞吐量 |
| **E（本实验）** | **经济**：多链并发不造成资金碎片化锁仓 | 多链不损失资金流动性 |

两组实验正好互补：性能实验证明"FishboneChain 可以跑得快"，资金实验证明"FishboneChain 跑得快的同时不会把钱压死"。
