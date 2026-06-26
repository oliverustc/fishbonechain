/**
 * FishboneChain high-pressure submission client.
 *
 * This script is for capacity benchmarking, not realistic low-frequency
 * crowdsourcing simulation. It keeps many worker accounts submitting as fast as
 * allowed by the configured in-flight limit and reports accepted TPS.
 *
 * Usage:
 *   node scripts/worker_burst.js \
 *     --ws ws://10.2.2.11:9948 \
 *     --task-id 3 \
 *     --workers 1000 \
 *     --worker-offset 0 \
 *     --parallel-per-worker 1 \
 *     --reward 0 \
 *     --data-size 64 \
 *     --batch-size 1 \
 *     --duration 120 \
 *     --submit-mode pool
 */

import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";
import { randomBytes } from "crypto";

function parseArgs() {
  const args = process.argv.slice(2);
  const get = (flag, def) => {
    const i = args.indexOf(flag);
    return i !== -1 ? args[i + 1] : def;
  };
  return {
    ws: get("--ws", "ws://127.0.0.1:9945"),
    taskId: Number(get("--task-id", "0")),
    workers: Number(get("--workers", "1000")),
    workerOffset: Number(get("--worker-offset", "0")),
    parallelPerWorker: Number(get("--parallel-per-worker", "1")),
    reward: BigInt(get("--reward", "0")),
    dataSize: Number(get("--data-size", "64")),
    batchSize: Number(get("--batch-size", "1")),
    duration: Number(get("--duration", "120")),
    reportInterval: Number(get("--report-interval", "5")),
    txTimeout: Number(get("--tx-timeout", "45")),
    submitMode: get("--submit-mode", "watch"),
  };
}

class Stats {
  constructor() {
    this.sent = 0;
    this.ok = 0;
    this.reject = 0;
    this.fail = 0;
    this.inflight = 0;
    this.startMs = Date.now();
    this.lastTs = Date.now();
    this.lastOk = 0;
  }

  markSent() {
    this.sent++;
    this.inflight++;
  }

  finish(kind) {
    this.inflight = Math.max(0, this.inflight - 1);
    if (kind === "ok") this.ok++;
    else if (kind === "reject") this.reject++;
    else this.fail++;
  }

  report(final = false) {
    const now = Date.now();
    const elapsed = (now - this.startMs) / 1000;
    const window = Math.max((now - this.lastTs) / 1000, 0.001);
    const tps = (this.ok - this.lastOk) / window;
    const totalDone = this.ok + this.reject + this.fail;
    const successRate = totalDone ? (this.ok / totalDone * 100).toFixed(1) : "0.0";

    console.log(
      `[burst ${new Date().toISOString()}]` +
      ` elapsed=${elapsed.toFixed(1)}s` +
      ` sent=${this.sent}` +
      ` ok=${this.ok}` +
      ` reject=${this.reject}` +
      ` fail=${this.fail}` +
      ` inflight=${this.inflight}` +
      ` okTPS=${tps.toFixed(2)}` +
      ` successRate=${successRate}%` +
      (final ? " final=true" : "")
    );

    this.lastTs = now;
    this.lastOk = this.ok;
  }
}

function classifyDispatchError(api, dispatchError) {
  if (!dispatchError) return "ok";
  if (dispatchError.isModule) {
    const { name } = api.registry.findMetaError(dispatchError.asModule);
    if ([
      "BudgetExhausted",
      "ExceedsBudget",
      "NotInCollectingSlot",
      "SubmissionLimitReached",
      "InvalidData",
      "TaskNotActive",
      "Overflow",
    ].includes(name)) {
      return "reject";
    }
  }
  return "fail";
}

async function sendOne(api, signer, nonce, cfg, stats) {
  const tx = cfg.batchSize > 1
    ? api.tx.crowdsource.submitDataBatch(
        cfg.taskId,
        Array.from({ length: cfg.batchSize }, () => Array.from(randomBytes(cfg.dataSize))),
        cfg.reward,
      )
    : api.tx.crowdsource.submitData(cfg.taskId, Array.from(randomBytes(cfg.dataSize)), cfg.reward);

  stats.markSent();

  if (cfg.submitMode === "pool") {
    try {
      await tx.signAndSend(signer, { nonce });
      stats.finish("ok");
      return "ok";
    } catch (e) {
      const message = String(e?.message ?? e);
      stats.finish("fail");
      if (message.includes("Immediately Dropped") || message.includes("couldn't enter the pool because of the limit")) {
        return "retry";
      }
      return "fail";
    }
  }

  return new Promise(async resolve => {
    let done = false;
    let unsub = null;

    const finish = kind => {
      if (done) return;
      done = true;
      stats.finish(kind);
      if (unsub) {
        try { unsub(); } catch { /* ignore */ }
      }
      resolve(kind);
    };

    const timer = setTimeout(() => finish("fail"), cfg.txTimeout * 1000);

    try {
      unsub = await tx.signAndSend(signer, { nonce }, ({ status, dispatchError }) => {
        if (dispatchError) {
          clearTimeout(timer);
          finish(classifyDispatchError(api, dispatchError));
        } else if (status.isInBlock) {
          clearTimeout(timer);
          finish("ok");
        }
      });
    } catch {
      clearTimeout(timer);
      finish("fail");
    }
  });
}

async function main() {
  const cfg = parseArgs();
  console.log(`[worker_burst] ws=${cfg.ws}`);
  console.log(`[worker_burst] task_id=${cfg.taskId} workers=${cfg.workers} worker_offset=${cfg.workerOffset} parallel_per_worker=${cfg.parallelPerWorker}`);
  console.log(`[worker_burst] reward=${cfg.reward} data_size=${cfg.dataSize} batch_size=${cfg.batchSize} duration=${cfg.duration}s`);
  console.log(`[worker_burst] submit_mode=${cfg.submitMode}`);

  const api = await ApiPromise.create({ provider: new WsProvider(cfg.ws) });
  const keyring = new Keyring({ type: "sr25519" });
  const signers = Array.from({ length: cfg.workers }, (_, i) => keyring.addFromUri(`//Worker${cfg.workerOffset + i}`));
  const stats = new Stats();

  const nextNonce = await Promise.all(signers.map(s => api.rpc.system.accountNextIndex(s.address)));
  const nonceNums = nextNonce.map(n => n.toNumber());
  const retryNonceQueues = Array.from({ length: cfg.workers }, () => []);
  const retryNonceSets = Array.from({ length: cfg.workers }, () => new Set());
  console.log(`[worker_burst] nonce range loaded for ${nonceNums.length} workers`);

  const takeNonce = workerIndex => {
    if (retryNonceQueues[workerIndex].length > 0) {
      const nonce = retryNonceQueues[workerIndex].shift();
      retryNonceSets[workerIndex].delete(nonce);
      return nonce;
    }
    const nonce = nonceNums[workerIndex];
    nonceNums[workerIndex]++;
    return nonce;
  };

  const retryNonce = (workerIndex, nonce) => {
    if (retryNonceSets[workerIndex].has(nonce)) return;
    retryNonceSets[workerIndex].add(nonce);
    retryNonceQueues[workerIndex].push(nonce);
    retryNonceQueues[workerIndex].sort((a, b) => a - b);
  };

  let running = true;
  const stopTimer = setTimeout(() => { running = false; }, cfg.duration * 1000);
  const reportTimer = setInterval(() => stats.report(), cfg.reportInterval * 1000);
  process.on("SIGTERM", () => {
    running = false;
  });
  process.on("SIGINT", () => {
    running = false;
  });

  const loops = [];
  for (let i = 0; i < signers.length; i++) {
    for (let lane = 0; lane < cfg.parallelPerWorker; lane++) {
      loops.push((async () => {
        while (running) {
          const nonce = takeNonce(i);
          const kind = await sendOne(api, signers[i], nonce, cfg, stats);
          if (cfg.submitMode === "pool" && kind === "retry") {
            retryNonce(i, nonce);
            await new Promise(r => setTimeout(r, 50));
          } else if (cfg.submitMode === "pool" && kind === "fail") {
            await new Promise(r => setTimeout(r, 50));
          }
        }
      })());
    }
  }

  await Promise.allSettled(loops);
  clearTimeout(stopTimer);
  clearInterval(reportTimer);
  stats.report(true);
  await api.disconnect();
}

main().catch(e => {
  console.error("[worker_burst fatal]", e.message);
  process.exit(1);
});
