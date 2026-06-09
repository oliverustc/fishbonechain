# 资金流动性对比实验规划

**目标**：通过真实系统运行，展示 FMC 动态锁定机制在资金可用性上的实际优势  
**依据**：fishbonechain.md §7.4；所有数据均来自真实链上状态，不做纯理论推导  
**状态**：规划草案 v3（2026-06-09）

---

## 一、核心问题重构

论文的核心主张（§7.4）不只是"锁定率低"，更是：

> **同一笔资金，传统预存模型能做 1 件事，FMC 能同时做 T 件事。**

`T = 任务计划运行的 epoch 数`

用实际数字说：Alice 有 50,000 UNIT，任务每 epoch 需要 b = 5,000 UNIT：

| 问题 | 传统预存 | FMC |
|------|---------|-----|
| 启动 1 个任务需要锁多少？| 50,000 UNIT（T=10 epoch 全锁）| 5,000 UNIT |
| 剩余可用资金？| 0 UNIT | **45,000 UNIT** |
| 能同时激活几个任务？| 1 个 | **最多 10 个** |
| 任务跑完一半时还剩多少可用？| 25,000 UNIT（但任务已绑定）| **45,000 UNIT**（始终可用）|

这是实际系统里可以直接测量的。实验不做理论推导，而是让系统跑起来，用链上数据回答这些问题。

---

## 二、对比模型的实际操作定义

### 2.1 传统预存模型（在 FMC 系统上的模拟方式）

传统模型的本质：**请求者在创建任务时，把整个任务周期（T 个 epoch）的全部预算一次性锁进合约**。

在当前 FMC 系统上，可以用以下方式模拟：
- **将 `budget_per_epoch` 设置为 `T × b_actual`（放大 T 倍）**
- 只充值 1 个 epoch 的预算：`deposit = T × b_actual`（即 `deposit = budget_per_epoch`）
- 激活后：LB = T×b_actual，FB = 0（传统模型无空闲余额）
- 每 epoch 结算实际支付仍约为 b_actual，但 LB 大、FB=0 的状态是传统模型的核心特征

> 这不是修改 pallet，而是用不同的参数配置在同一套系统上运行两个场景。

### 2.2 FMC 动态锁定模型（正常使用）

- `budget_per_epoch = b`（按实际每 epoch 支出设置）
- `deposit = T × b`（同等总资金）
- 激活后：LB = b，FB = (T-1) × b
- 每 epoch 结算后：剩余自动归还 FB，FB ≈ (T-1) × b 保持稳定

### 2.3 关键差异（可直接测量）

```
同等总资金 D = T × b：

传统模型：  FB ≈ 0,      LB ≈ D       （全部锁定）
FMC 模型：  FB ≈ (T-1)b, LB ≈ b       （只锁 1/T）

"可用资金" = FB / b = 可以立即激活的新任务数量
传统：  0 个
FMC：   T-1 ≈ 9 个（T=10 时）
```

---

## 三、实验场景设计（全部使用真实系统）

### 场景 S1：单任务资金对比（child4，T=10）

**问题**：同样充值 50,000 UNIT，两种模式下的资金分布有何不同？

| | 传统模式（模拟）| FMC 正常模式 |
|--|--------------|------------|
| task 配置 | budget_per_epoch = 50,000 | budget_per_epoch = 5,000 |
| 充值 | 50,000 UNIT | 50,000 UNIT |
| 激活后 LB | 50,000 UNIT | 5,000 UNIT |
| 激活后 FB | 0 UNIT | **45,000 UNIT** |
| epoch 1 实际支出 | ~5,000 UNIT（100 workers）| ~5,000 UNIT |
| 结算后 LB | 50,000（不变）| 5,000（续期后不变）|
| 结算后 FB | 0（加回微小余量）| **45,000（加回余量）** |

**运行方式**：
- 先运行 FMC 正常模式 5 个 epoch，采集真实 FB/LB 曲线
- 再运行传统模式 5 个 epoch（新任务，budget_per_epoch 改为 50,000）
- 两组数据叠加在同一图上对比

---

### 场景 S2：FMC 资金复用能力（child1+child3+child4+child6 同时激活）

**问题**：FMC 保留的 FB 在实际中能支持多少并发任务？

**操作**：
1. Alice 充值 D = 30,000 UNIT（= 6 × b，b=5,000）
2. 依次激活 child4 / child6 / child3 / child1（各 b=5,000）
3. 观察：每次 activateTask 后，FB 减少 5,000，LB 增加 5,000
4. 运行 5 个 epoch，同时对 4 条链采集数据

**测量**：
- 每次激活前后的 FB/LB 快照（以块高为时间轴）
- 4 条链同时运行期间：总 LB = 4×5,000 = 20,000；FB = 10,000 剩余
- 传统模式下，同等资金（30,000 UNIT）只能支持 1 个任务跑 6 个 epoch（30,000 = 1×6×5,000）

**关键数据点**：
```
充值 30,000 UNIT 时：
  传统：最多 1 条链跑 6 epoch（LFR=100%，FB=0，无法扩展）
  FMC： 4 条链同时跑（LFR≈67%），或 6 条链用 LFR=100% 但每条各跑多 epoch
        FB 剩余 10,000 → 还可激活 2 个新任务
```

---

### 场景 S3：资金回收速度（settle 后余额多快归还）

**问题**：epoch 结算后，未使用的预算多快回到 FB，可以立刻被重新使用？

**操作**：
- child4，workers=10（低利用率，payout ≈ 500 UNIT，余量 ≈ 4,500 UNIT/epoch）
- 精确记录：
  - `BillSettled` 事件发生的块高 `h_settle`
  - FB 在块高 `h_settle` 之前和之后的值
  - 下一次 activateTask（如果 FB 刚好能激活一个新任务）能发生在哪个块高

**预期**：`BillSettled` 的同一个块内，FB 增加了 `(b - payout) = 4,500 UNIT`。
即：**资金回收是原子的，在结算块内立刻可用**——这是 FMC 相比"月底才退款"的传统模式的直接优势。

---

## 四、数据采集方案

### 4.1 `scripts/fmc_metrics.js`（新建）

采集粒度：每 30 秒轮询一次 + 每次 BillSettled 事件立刻记录

```
输出字段（全部绝对值，UNIT 和 planck 均记录）：

timestamp           ISO 时间戳
block_number        当前块高（精确到事件时刻）
epoch_id            fmc.tasks(alice, task_id).current_epoch
task_status         Activated / Terminated
fb_planck           fmc.fundPools(alice).free（原始值）
lb_planck           fmc.fundPools(alice).locked（原始值）
fb_unit             fb_planck / 1e12
lb_unit             lb_planck / 1e12
total_deposited_unit  fb_unit + lb_unit
budget_per_epoch_unit fmc.tasks(alice, task_id).budget_per_epoch / 1e12
available_tasks_count floor(fb_unit / budget_per_epoch_unit)  ← 可立即激活几个新任务
bill_settled        0 / 1（本次采样是否触发了 BillSettled）
actual_payout_unit  BillSettled.total_paid / 1e12（未触发则 0）
utilization         actual_payout_unit / budget_per_epoch_unit（未触发则 null）
```

> 说明：`available_tasks_count` 是论文 §7.4 "可用性" 的直接量化——FB 能支持多少个同等规模的新任务立刻激活。这个指标比 LFR 百分比更直观。

用法：
```bash
MAIN_WS=ws://10.2.2.11:9944 \
REQUESTER=5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY \
TASK_ID=3 \
node scripts/fmc_metrics.js --out /tmp/exp_e_fmc.csv --interval 30
```

### 4.2 `scripts/fmc_metrics_multi.js`（场景 S2 专用，新建）

同时监控多个任务/多个充值账户的 FB/LB：

```bash
MAIN_WS=ws://10.2.2.11:9944 \
REQUESTER=5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY \
TASK_IDS=3,0,2,5 \      ← child4/child1/child3/child6 对应的 task_id
node scripts/fmc_metrics_multi.js --out /tmp/exp_e2_fmc.csv --interval 30
```

增加字段：`task_id`，`lb_per_task_unit`，`total_lb_unit`（所有任务的 LB 之和）

---

## 五、可视化方案（两种指标都出图）

### 图 7a：绝对金额（UNIT）随 epoch 变化

```
Y 轴单位：UNIT
场景 S1：
  ├── FMC FB（实线蓝）：约 45,000 UNIT，接近水平
  ├── FMC LB（实线绿）：约 5,000 UNIT，水平线
  ├── 传统 FB（虚线橙）：约 0 UNIT，贴底
  └── 传统 LB（虚线红）：从 50,000 UNIT 线性下降

右侧辅助 Y 轴（或注释）：
  available_tasks_count（FMC ≈ 9，传统 ≈ 0）
```

### 图 7b：锁定率百分比（LFR）随 epoch 变化

```
Y 轴：LFR = lb / total_deposited（%）
  ├── FMC（蓝实线）：约 10%，水平
  └── 传统（橙虚线）：从 100% 线性下降到 0%
阴影：两线之间的"节省区域"
```

### 图 7c：场景 S2 多任务并发资金结构（堆积条形图，按 epoch）

```
每个 epoch 一根柱子，分三层：
  ├── 已锁定（LB）= 活跃任务数 × b （深蓝）
  ├── 可激活新任务的 FB（浅蓝）= 可以立刻再开几条链
  └── 传统模式下的"等效 LB"= D - 已运行×b（橙色，叠加对比）
```

### 图 7d：场景 S3 结算块精确时序（场景 S3，可选）

```
X 轴：块高
Y 轴：FB 绝对值（UNIT）
  ├── 标记每次 BillSettled 事件（垂直虚线）
  └── 显示 FB 在结算块前后的阶跃（余量立刻归还）
```

---

## 六、执行计划

### Step 0：链上状态确认（30 分钟）

```bash
# 查询主链 task 状态（通过 SSH 连接 f1）
# 在 polkadot.js 或脚本中查询：
#   fmc.tasks(Alice, 3) → {status, budget_per_epoch, current_epoch}
#   fmc.fundPools(Alice) → {free, locked}
```

根据状态决定：
- 若 task_id=3 仍 Activated：直接开始 S1 FMC 模式实验
- 若已 Terminated：`fmc.activateTask(3)` 重新激活（确保 FB ≥ b=5,000）
- 若 FB 不足：`fmc.deposit(50,000 UNIT)` 补充

### Step 1：实现 `fmc_metrics.js`（2 小时）

实现要点：
```javascript
// 轮询（每 interval 秒）
const poll = async () => {
  const pool = await api.query.fmc.fundPools(REQUESTER);
  const task = await api.query.fmc.tasks(REQUESTER, TASK_ID);
  const header = await api.rpc.chain.getHeader();
  // 写入 CSV 行
};

// 事件监听（即时）
api.query.system.events(events => {
  for (const { event } of events) {
    if (event.section === 'fmc' && event.method === 'BillSettled') {
      // 记录 bill_settled=1, actual_payout
    }
  }
});
```

### Step 2：运行场景 S1 FMC 模式（2 小时，10 个 epoch）

```bash
# 三个进程同时运行：
node scripts/fmc_metrics.js --out /tmp/exp_e1_fmc_normal.csv --interval 30 &
MINER_SURIS="seed_f1,...,seed_f5" CHILD_WS=ws://10.2.2.11:9948 \
  MAIN_WS=ws://10.2.2.11:9944 TASK_ID=3 CHAIN_ID=3 node scripts/bridge.js &
node scripts/worker.js --scenario d --workers 100 --ws ws://10.2.2.11:9948 --task-id 3
```

### Step 3：运行场景 S1 传统模式（2 小时，10 个 epoch）

需要先重新创建一个 budget_per_epoch = 50,000 UNIT 的任务：
```bash
# 1. 在主链创建新任务（task_id=7 或下一个可用 id）
#    budget_per_epoch = 50,000 UNIT，只充值 50,000 UNIT（紧凑存款）
#    → 激活后：LB=50,000，FB=0

# 2. 在子链 sync_task（设置相同的任务信息但 budget 更大）

# 3. worker 仍用 100 workers，实际支出约 5,000 UNIT/epoch
#    观察：LB 不变（始终 50,000），FB ≈ 0

node scripts/fmc_metrics.js --out /tmp/exp_e1_fmc_trad.csv \
  --task-id 7 --interval 30 &
MINER_SURIS="..." TASK_ID=7 CHAIN_ID=3 node scripts/bridge.js &
node scripts/worker.js --scenario d --workers 100 --ws ws://10.2.2.11:9948 --task-id 7
```

### Step 4：运行场景 S2 多任务并发（1.5 小时，5 个 epoch）

```bash
# 先激活 child1/child3/child6 的任务（task_id=0,2,5）
# 然后同时运行 4 链 worker + bridge
node scripts/fmc_metrics_multi.js --out /tmp/exp_e2_multi.csv \
  --task-ids 0,2,3,5 --interval 30
```

### Step 5：运行场景 S3 资金回收速度（1 小时，3 个 epoch）

```bash
# 低利用率：workers=10，payout ≈ 500 UNIT/epoch
node scripts/fmc_metrics.js --out /tmp/exp_e3_recycle.csv \
  --interval 6 &   ← 缩短采样间隔到 6s（=1 个块），精确捕捉结算块
node scripts/worker.js --scenario d --workers 10 --ws ws://10.2.2.11:9948 --task-id 3
```

### Step 6：分析与出图（1 小时）

- [ ] `scripts/simulate_traditional.py`：从 S1 FMC 数据推算等效传统模型的 LB/FB 序列
- [ ] `plot_results.py` 新增 `fig7a/b/c/d()` 四个函数
- [ ] 生成所有图，确认 FMC 曲线与传统曲线的视觉差异足够清晰

---

## 七、预期关键数据点（D = 50,000 UNIT，b = 5,000 UNIT，T = 10）

### S1 结果预测

| epoch | FMC FB | FMC LB | 传统 FB | 传统 LB | FMC 可激活新任务数 |
|-------|--------|--------|---------|---------|-------------------|
| 0（激活后）| 45,000 | 5,000 | 0 | 50,000 | **9 个** |
| 3 | ~45,000 | 5,000 | ~0 | 50,000 | **9 个** |
| 7 | ~45,000 | 5,000 | ~0 | 50,000 | **9 个** |
| 10 | ~45,000 | 5,000 | ~0 | 50,000 | **9 个** |

> 注：传统模式下 LB 不会减少，因为 budget_per_epoch = 50,000，每 epoch 实际支出仅 5,000，LB 始终 50,000。
> （传统模式的 LB 减少取决于任务结束时机，实验中任务持续运行则 LB 不变。）

### S2 结果预测（D = 30,000，4 任务）

| 时间点 | 总 LB | 总 FB | 活跃任务数 | 还能再激活几个 |
|--------|-------|-------|----------|-------------|
| 激活 task3 后 | 5,000 | 25,000 | 1 | 5 个 |
| 激活 task0 后 | 10,000 | 20,000 | 2 | 4 个 |
| 激活 task2 后 | 15,000 | 15,000 | 3 | 3 个 |
| 激活 task5 后 | 20,000 | 10,000 | 4 | 2 个 |
| 运行期间 | ~20,000 | ~10,000 | 4 | 2 个 |

传统模式下，30,000 UNIT 只能支持 1 个任务（budget_per_epoch=30,000），无法同时服务 4 条链。

### S3 结果预测（低利用率，workers=10）

```
每 epoch：payout ≈ 500 UNIT，余量 ≈ 4,500 UNIT
BillSettled 块（h_s）：FB 阶跃 +4,500 UNIT（在同一块内）
下一块（h_s+1）：FB 可用于新任务激活
延迟 = 1 个块（6 秒）
```

---

## 八、与论文 §7.4 的对应关系

| 论文描述 | 本实验对应场景 | 测量方式 |
|---------|------------|---------|
| 传统方案初始锁定 $100,000 | S1 传统模式 epoch 0 | fmc_metrics → lb_unit |
| FishboneChain 仅锁定 ≤20% | S1 FMC 模式全程 | fmc_metrics → lb_unit / total_unit |
| 25% 进度时传统仍锁 $75,000 | S1 传统模式 epoch 5 | 传统 LB 不变（因 budget_per_epoch 大）|
| 多链并发时 FMC 的扩展性 | S2 四链并发 | fmc_metrics_multi → total_lb_unit |
