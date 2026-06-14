/**
 * FishboneChain 工作者负载模拟工具
 *
 * 并发 N 个工作者账户，按配置频率向子链调用 crowdsource.submit_data
 *
 * 用法：
 *   node scripts/worker.js [选项]
 *   node scripts/worker.js --scenario a          # 快捷预设（见下表）
 *
 * 参数：
 *   --ws <url>           子链 WebSocket RPC（默认 ws://127.0.0.1:9945）
 *   --task-id <n>        目标任务 ID（默认 0）
 *   --workers <n>        并发工作者数（默认 10）
 *   --rate <req/s>       每个工作者每秒请求数（默认 0.1）
 *   --reward <planck>    每次提交奖励（默认 5000000000 = 5 UNIT）
 *   --data-size <bytes>  随机数据大小（默认 256）
 *   --duration <s>       运行时长（默认无限）
 *   --scenario <a-f>     使用场景预设（覆盖以上参数）
 *   --protocol <name>    负载协议（默认 crowdsource）
 *
 * 场景预设（对应 6 条子链）：
 *   a 快递物流:   300 workers, 0.02 req/s, 5 UNIT,     512B
 *   b 交通感知:  2000 workers, 0.1  req/s, 0.001 UNIT, 128B
 *   c 医疗数据:   200 workers, 0.008 req/s,200 UNIT,   900B
 *   d 金融分析:   100 workers, 0.005 req/s, 50 UNIT,   800B
 *   e 传感器网络:5000 workers, 0.2  req/s, 0.0001 UNIT, 64B
 *   f 数据市场:   500 workers, 0.02 req/s,  50 UNIT,  1024B
 */

import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";
import { randomBytes } from "crypto";

// ─── 场景预设 ───────────────────────────────────────────────────────────────

const UNIT = 1_000_000_000_000n; // 1 UNIT = 10^12 planck

const SCENARIOS = {
  a: { workers: 300,  rate: 0.02,  reward: 5n * UNIT,             dataSize: 512  },
  b: { workers: 2000, rate: 0.1,   reward: UNIT / 1000n,          dataSize: 128  },
  c: { workers: 200,  rate: 0.008, reward: 200n * UNIT,           dataSize: 900  },
  d: { workers: 100,  rate: 0.005, reward: 50n * UNIT,            dataSize: 800  },
  e: { workers: 5000, rate: 0.2,   reward: UNIT / 10_000n,        dataSize: 64   },
  f: { workers: 500,  rate: 0.02,  reward: 50n * UNIT,            dataSize: 1024 },
};

// ─── CLI 解析 ────────────────────────────────────────────────────────────────

function parseArgs() {
  const args = process.argv.slice(2);
  const get = (flag, def) => {
    const i = args.indexOf(flag);
    return i !== -1 ? args[i + 1] : def;
  };

  const scenario = get("--scenario", null);
  const preset   = scenario ? SCENARIOS[scenario] : null;
  if (scenario && !preset) {
    console.error(`未知场景 --scenario ${scenario}，支持: ${Object.keys(SCENARIOS).join(", ")}`);
    process.exit(1);
  }

  return {
    protocol: get("--protocol", "crowdsource"),
    ws:       get("--ws",        "ws://127.0.0.1:9945"),
    taskId:   Number(get("--task-id",   "0")),
    workers:  Number(get("--workers",   preset?.workers  ?? 10)),
    rate:     Number(get("--rate",      preset?.rate     ?? 0.1)),
    reward:   BigInt(get("--reward",    (preset?.reward  ?? 5n * UNIT).toString())),
    dataSize: Number(get("--data-size", preset?.dataSize ?? 256)),
    duration: get("--duration", null) ? Number(get("--duration", null)) : null,
    scenario,
  };
}

// ─── 统计 ────────────────────────────────────────────────────────────────────

class Stats {
  constructor() {
    this.ok      = 0;
    this.fail    = 0;
    this.reject  = 0;
    this.startMs = Date.now();
    this.lastOk  = 0;
    this.lastTs  = Date.now();
  }

  tick(result) {
    if      (result === "ok")     this.ok++;
    else if (result === "fail")   this.fail++;
    else if (result === "reject") this.reject++;
  }

  report() {
    const nowMs   = Date.now();
    const elapsedS = (nowMs - this.startMs) / 1000;
    const windowS  = (nowMs - this.lastTs)  / 1000 || 1;
    const tps      = (this.ok - this.lastOk) / windowS;
    this.lastOk    = this.ok;
    this.lastTs    = nowMs;

    const total   = this.ok + this.fail + this.reject;
    const succRate = total ? ((this.ok / total) * 100).toFixed(1) : "—";

    console.log(
      `[stats ${new Date().toISOString()}]` +
      `  elapsed=${elapsedS.toFixed(0)}s` +
      `  ok=${this.ok}  fail=${this.fail}  reject=${this.reject}` +
      `  successRate=${succRate}%  TPS≈${tps.toFixed(2)}`
    );
  }
}

// ─── 主逻辑 ──────────────────────────────────────────────────────────────────

async function main() {
  const cfg = parseArgs();
  if (cfg.protocol !== "crowdsource") {
    throw new Error("worker.js only supports protocol=crowdsource; use a scene-specific load generator");
  }

  console.log(`[worker] 场景: ${cfg.scenario ?? "custom"}`);
  console.log(`[worker] 协议: ${cfg.protocol}`);
  console.log(`[worker] 子链 RPC: ${cfg.ws}`);
  console.log(`[worker] 工作者数: ${cfg.workers}  请求率: ${cfg.rate} req/s  数据大小: ${cfg.dataSize}B`);
  console.log(`[worker] task_id=${cfg.taskId}  reward=${cfg.reward} planck`);
  cfg.duration && console.log(`[worker] 运行时长: ${cfg.duration}s`);

  const api = await ApiPromise.create({ provider: new WsProvider(cfg.ws) });
  const keyring = new Keyring({ type: "sr25519" });

  // 生成工作者密钥（//Worker0 … //WorkerN）
  const signers = Array.from({ length: cfg.workers }, (_, i) =>
    keyring.addFromUri(`//Worker${i}`)
  );

  const stats = new Stats();
  const statsInterval = setInterval(() => stats.report(), 10_000);

  let running = true;

  // 每个工作者独立的提交循环
  const workerLoops = signers.map(async (signer, idx) => {
    const intervalMs = Math.round(1000 / cfg.rate);
    // 随机错开启动，避免所有工作者同时发送
    await new Promise(r => setTimeout(r, Math.random() * intervalMs));

    while (running) {
      const data = Array.from(randomBytes(cfg.dataSize));
      try {
        await new Promise((resolve, reject) => {
          api.tx.crowdsource
            .submitData(cfg.taskId, data, cfg.reward)
            .signAndSend(signer, ({ status, dispatchError }) => {
              if (dispatchError) {
                if (dispatchError.isModule) {
                  const { name } = api.registry.findMetaError(dispatchError.asModule);
                  if (["BudgetExhausted", "ExceedsBudget", "NotInCollectingSlot",
                       "SubmissionLimitReached", "InvalidData", "TaskNotActive",
                       "Overflow"].includes(name)) {
                    stats.tick("reject");
                    resolve();
                    return;
                  }
                }
                stats.tick("fail");
                resolve(); // 其他错误也继续运行
              } else if (status.isInBlock) {
                stats.tick("ok");
                resolve();
              }
            }).catch(e => {
              stats.tick("fail");
              resolve();
            });
        });
      } catch {
        stats.tick("fail");
      }

      await new Promise(r => setTimeout(r, intervalMs));
    }
  });

  if (cfg.duration) {
    await new Promise(r => setTimeout(r, cfg.duration * 1000));
    running = false;
    stats.report();
    clearInterval(statsInterval);
    await api.disconnect();
    console.log("[worker] 完成");
    process.exit(0);
  } else {
    process.on("SIGINT", async () => {
      running = false;
      stats.report();
      clearInterval(statsInterval);
      await api.disconnect();
      process.exit(0);
    });
    await Promise.all(workerLoops);
  }
}

main().catch(e => {
  console.error("[worker 致命错误]", e.message);
  process.exit(1);
});
