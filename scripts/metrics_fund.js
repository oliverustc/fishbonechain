/**
 * FishboneChain 资金流动性指标采集
 *
 * 对比 FishboneChain 周期性资金管理 vs 传统方案（全周期预算预锁）。
 *
 * 指标分两层：
 *   task_locked   = Σ budget_per_epoch（Activated 任务）  ← FishboneChain 实际工作锁定
 *   pool_locked   = fmc.fundPools.locked                 ← 链上真实值（含历史幻影锁定）
 *   baseline_locked = (T_planned - epoch_id) × sum_active_budget ← 传统方案反事实
 *
 * 用法：
 *   MAIN_WS=ws://10.2.2.11:9944 \
 *   REQUESTER=5GrwvaEF... \
 *   TASK_IDS=0,3,5 \
 *   T_PLANNED=20 \
 *   node scripts/metrics_fund.js --out /tmp/exp_e_fund --interval 15
 */

import { ApiPromise, WsProvider } from "@polkadot/api";
import { createWriteStream } from "fs";

// ─── CLI / 环境变量 ───────────────────────────────────────────────────────────

function parseArgs() {
  const args = process.argv.slice(2);
  const get  = (flag, def) => { const i = args.indexOf(flag); return i !== -1 ? args[i + 1] : def; };
  return {
    out:      get("--out", "/tmp/exp_e_fund"),
    interval: Number(get("--interval", "15")),
  };
}

const MAIN_WS          = process.env.MAIN_WS          || "ws://127.0.0.1:9944";
const REQUESTER        = process.env.REQUESTER        || "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY";
const TASK_IDS         = (process.env.TASK_IDS        || "0,3,5").split(",").map(Number);
const T_PLANNED        = Number(process.env.T_PLANNED        || "20");
const EPOCH_OFFSET     = Number(process.env.EPOCH_OFFSET     || "0"); // 起始 epoch 偏移，归零用
// 用哪条链的 epoch 作进度基准（-1 = 取所有活跃任务的 max，6链实验设为 3 = child4）
const REFERENCE_TASK_ID = Number(process.env.REFERENCE_TASK_ID ?? "-1");
const UNIT       = 1_000_000_000_000n;  // 1 UNIT = 1e12 planck

function toUnit(planck) {
  // Returns float UNIT value (keeps 4 decimal precision)
  return Number(BigInt(planck.toString()) * 10000n / UNIT) / 10000;
}

// ─── CSV 写入 ────────────────────────────────────────────────────────────────

function csvWriter(path, header) {
  const stream = createWriteStream(path, { flags: "a" });
  let written = false;
  return {
    write(row) {
      if (!written) { stream.write(header + "\n"); written = true; }
      stream.write(row + "\n");
    },
    close() { stream.end(); },
  };
}

function row(...vals) {
  return vals.map(v => String(v ?? "").replace(/,/g, ";")).join(",");
}

// ─── 主逻辑 ──────────────────────────────────────────────────────────────────

async function main() {
  const cfg = parseArgs();
  const csvPath = `${cfg.out}_state.csv`;

  console.log(`[metrics_fund] 主链: ${MAIN_WS}`);
  console.log(`[metrics_fund] 任务: ${TASK_IDS.join(",")}`);
  console.log(`[metrics_fund] T_planned=${T_PLANNED}  interval=${cfg.interval}s`);
  console.log(`[metrics_fund] 输出: ${csvPath}`);

  const api = await ApiPromise.create({ provider: new WsProvider(MAIN_WS) });

  const csv = csvWriter(csvPath,
    "timestamp,block_number,epoch_id,relative_epoch," +
    "free_unit,pool_locked_unit,total_pool_unit,pool_locked_ratio," +
    "active_tasks,sum_active_budget_unit,task_locked_unit,task_locked_ratio," +
    "baseline_locked_unit,improvement," +
    "bill_settled_this_tick,total_paid_unit," +
    "available_tasks_count"
  );

  // BillSettled 事件在每个 tick 内的累计（由事件订阅写入）
  let billSettledCount = 0;
  let totalPaidThisTick = 0n;

  // 订阅 BillSettled 事件（精确到块）
  api.query.system.events(events => {
    for (const { event } of events) {
      if (event.section === "fmc" && event.method === "BillSettled") {
        billSettledCount++;
        const paid = event.data[3] ?? event.data.total_paid ?? 0n;
        totalPaidThisTick += BigInt(paid.toString());
        const ts = new Date().toISOString();
        const taskId = event.data[1]?.toString() ?? "?";
        const epoch  = event.data[2]?.toString() ?? "?";
        console.log(`[${ts}] BillSettled task=${taskId} epoch=${epoch} paid=${toUnit(paid)} UNIT`);
      }
    }
  });

  // 轮询采样函数
  async function sample() {
    const blockHash  = await api.rpc.chain.getFinalizedHead();
    const header     = await api.rpc.chain.getHeader(blockHash);
    const blockNum   = header.number.toNumber();

    // 查询 FundPool
    const poolRaw  = await api.query.fmc.fundPools(REQUESTER);
    const pool     = poolRaw.isSome ? poolRaw.unwrap() : poolRaw;
    const freePlanck   = BigInt((pool.free ?? pool.toJSON()?.free ?? 0).toString());
    const lockedPlanck = BigInt((pool.locked ?? pool.toJSON()?.locked ?? 0).toString());
    const freeUnit     = toUnit(freePlanck);
    const poolLockedUnit = toUnit(lockedPlanck);
    const totalPool    = freeUnit + poolLockedUnit;
    const poolLockedRatio = totalPool > 0 ? (poolLockedUnit / totalPool).toFixed(4) : "0";

    // 查询所有任务状态
    const tasks = await Promise.all(
      TASK_IDS.map(async id => {
        const t = await api.query.fmc.tasks(REQUESTER, id);
        if (!t.isSome) return null;
        const data = t.unwrap();
        return {
          id,
          status:        data.status.toString(),
          budget:        BigInt(data.budgetPerEpoch?.toString() ?? data.budget_per_epoch?.toString() ?? "0"),
          currentEpoch:  Number(data.currentEpoch ?? data.current_epoch ?? 0),
        };
      })
    );

    const activeTasks = tasks.filter(t => t && t.status === "Activated");
    const activeBudgetPlanck = activeTasks.reduce((s, t) => s + t.budget, 0n);
    const activeCount  = activeTasks.length;
    const taskLockedUnit = toUnit(activeBudgetPlanck);
    const taskLockedRatio = totalPool > 0 ? (taskLockedUnit / totalPool).toFixed(4) : "0";

    // epoch_id: 优先用 REFERENCE_TASK_ID 对应任务的 epoch（多链时防快链干扰基线计算）
    const refTask = REFERENCE_TASK_ID >= 0
      ? tasks.find(t => t && t.id === REFERENCE_TASK_ID)
      : null;
    const epochId = refTask
      ? refTask.currentEpoch
      : (activeTasks.length > 0 ? Math.max(...activeTasks.map(t => t.currentEpoch)) : 0);
    // relative_epoch: 相对于实验开始时的 epoch 偏移（归零）
    const relativeEpoch = Math.max(0, epochId - EPOCH_OFFSET);

    // 反事实基线：传统方案第 relativeEpoch 时刻的锁仓
    // baseline(i) = (T_planned - i) × sum_active_budget，i 从 0 开始
    const baselinePlanck = BigInt(Math.max(0, T_PLANNED - relativeEpoch)) * activeBudgetPlanck;
    const baselineUnit   = toUnit(baselinePlanck);

    // 改善比：baseline / task_locked（避免除零）
    const improvement = taskLockedUnit > 0
      ? (baselineUnit / taskLockedUnit).toFixed(2)
      : "∞";

    // available_tasks_count：free 还能激活几个最小 budget 的任务
    const minBudget = activeTasks.length > 0
      ? activeTasks.reduce((m, t) => t.budget < m ? t.budget : m, activeTasks[0].budget)
      : UNIT;  // fallback: 1 UNIT
    const availableTasks = minBudget > 0n ? freePlanck / minBudget : 0n;

    // 读取并重置本 tick 的事件计数
    const settled   = billSettledCount;
    const paidUnit  = toUnit(totalPaidThisTick);
    billSettledCount = 0;
    totalPaidThisTick = 0n;

    const ts = new Date().toISOString();
    csv.write(row(
      ts, blockNum, epochId, relativeEpoch,
      freeUnit, poolLockedUnit, totalPool, poolLockedRatio,
      activeCount, toUnit(activeBudgetPlanck), taskLockedUnit, taskLockedRatio,
      baselineUnit, improvement,
      settled, paidUnit,
      availableTasks.toString()
    ));

    console.log(
      `[${ts}] block=${blockNum} epoch=${epochId} ` +
      `task_locked=${taskLockedUnit} baseline=${baselineUnit} improve=${improvement}× ` +
      `pool_locked=${poolLockedUnit} free=${freeUnit} settled=${settled}`
    );
  }

  // 立即采样一次，然后定时采样
  await sample();
  const timer = setInterval(sample, cfg.interval * 1000);

  process.on("SIGINT", async () => {
    clearInterval(timer);
    csv.close();
    await api.disconnect();
    console.log("\n[metrics_fund] 已停止，数据写入:", csvPath);
    process.exit(0);
  });

  console.log(`[metrics_fund] 持续采集中，间隔 ${cfg.interval}s（Ctrl+C 停止）...`);
  await new Promise(() => {});
}

main().catch(e => {
  console.error("[metrics_fund 致命错误]", e.message);
  process.exit(1);
});
