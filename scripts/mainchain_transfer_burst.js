#!/usr/bin/env node
/**
 * FishBoneChain mainchain transfer capacity benchmark.
 *
 * This measures the 18-validator mainchain with ordinary balance transfers.
 * Funding is done before the measured window so the summary represents the
 * mainchain's transaction inclusion capacity, not benchmark setup overhead.
 */

import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";
import { mkdirSync, writeFileSync } from "fs";
import { dirname } from "path";

function parseArgs() {
  const args = process.argv.slice(2);
  const get = (flag, def) => {
    const i = args.indexOf(flag);
    return i !== -1 ? args[i + 1] : def;
  };
  return {
    ws: get("--ws", process.env.MAIN_WS || "ws://127.0.0.1:9944"),
    senders: Number(get("--senders", "300")),
    senderOffset: Number(get("--sender-offset", "0")),
    parallelPerSender: Number(get("--parallel-per-sender", "1")),
    amount: BigInt(get("--amount", "1")),
    fundAmount: BigInt(get("--fund-amount", "1000000000000000")),
    duration: Number(get("--duration", "120")),
    reportInterval: Number(get("--report-interval", "5")),
    txTimeout: Number(get("--tx-timeout", "45")),
    fundBatchSize: Number(get("--fund-batch-size", "50")),
    receiverMode: get("--receiver-mode", "alice"),
    submitMode: get("--submit-mode", "watch"),
    out: get("--out", "docs/experiments/progressive_tps/mainchain_capacity/mainchain_transfer_burst_summary.json"),
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

  elapsedSeconds() {
    return Math.max((Date.now() - this.startMs) / 1000, 0.001);
  }

  report(final = false) {
    const now = Date.now();
    const elapsed = this.elapsedSeconds();
    const window = Math.max((now - this.lastTs) / 1000, 0.001);
    const windowTps = (this.ok - this.lastOk) / window;
    const mainTps = this.ok / elapsed;
    const totalDone = this.ok + this.reject + this.fail;
    const successRate = totalDone ? (this.ok / totalDone * 100).toFixed(1) : "0.0";

    console.log(
      `[mainchain-transfer-burst ${new Date().toISOString()}]` +
      ` elapsed=${elapsed.toFixed(1)}s` +
      ` sent=${this.sent}` +
      ` ok=${this.ok}` +
      ` reject=${this.reject}` +
      ` fail=${this.fail}` +
      ` inflight=${this.inflight}` +
      ` windowTPS=${windowTps.toFixed(2)}` +
      ` mainTPS=${mainTps.toFixed(2)}` +
      ` successRate=${successRate}%` +
      (final ? " final=true" : ""),
    );

    this.lastTs = now;
    this.lastOk = this.ok;
  }
}

function transferTx(api, to, amount) {
  if (api.tx.balances.transferAllowDeath) {
    return api.tx.balances.transferAllowDeath(to, amount);
  }
  if (api.tx.balances.transferKeepAlive) {
    return api.tx.balances.transferKeepAlive(to, amount);
  }
  return api.tx.balances.transfer(to, amount);
}

function classifyDispatchError(api, dispatchError) {
  if (!dispatchError) return "ok";
  if (dispatchError.isModule) {
    const { name } = api.registry.findMetaError(dispatchError.asModule);
    if (["LiquidityRestrictions", "Expendability", "ExistentialDeposit", "InsufficientBalance"].includes(name)) {
      return "reject";
    }
  }
  return "fail";
}

async function included(api, tx, signer, nonce, timeoutSeconds) {
  return new Promise(async resolve => {
    let done = false;
    let unsub = null;

    const finish = kind => {
      if (done) return;
      done = true;
      if (unsub) {
        try { unsub(); } catch { /* ignore */ }
      }
      resolve(kind);
    };

    const timer = setTimeout(() => finish("fail"), timeoutSeconds * 1000);

    try {
      unsub = await tx.signAndSend(signer, { nonce }, ({ status, dispatchError }) => {
        if (dispatchError) {
          clearTimeout(timer);
          finish(classifyDispatchError(api, dispatchError));
        } else if (status.isInBlock || status.isFinalized) {
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

async function freeBalance(api, address) {
  const account = await api.query.system.account(address);
  return BigInt(account.data.free.toString());
}

async function fundSenders(api, alice, senders, cfg) {
  const minFree = cfg.fundAmount / 2n;
  const balances = await Promise.all(senders.map(sender => freeBalance(api, sender.address)));
  const needFunding = senders.filter((_, i) => balances[i] < minFree);
  if (needFunding.length === 0) {
    console.log("[mainchain-transfer-burst] funding skipped: all senders have sufficient balance");
    return;
  }

  console.log(`[mainchain-transfer-burst] funding ${needFunding.length}/${senders.length} senders from //Alice`);
  let aliceNonce = (await api.rpc.system.accountNextIndex(alice.address)).toNumber();
  const chunkSize = Math.max(cfg.fundBatchSize, 1);
  for (let i = 0; i < needFunding.length; i += chunkSize) {
    const chunk = needFunding.slice(i, i + chunkSize);
    const transfers = chunk.map(sender => transferTx(api, sender.address, cfg.fundAmount));
    const tx = api.tx.utility?.batchAll
      ? api.tx.utility.batchAll(transfers)
      : null;

    if (tx) {
      const kind = await included(api, tx, alice, aliceNonce, cfg.txTimeout);
      aliceNonce++;
      if (kind !== "ok") {
        throw new Error(`funding batch failed with status=${kind} at sender ${i}`);
      }
    } else {
      for (const sender of chunk) {
        const kind = await included(api, transferTx(api, sender.address, cfg.fundAmount), alice, aliceNonce, cfg.txTimeout);
        aliceNonce++;
        if (kind !== "ok") {
          throw new Error(`funding transfer failed with status=${kind} for ${sender.address}`);
        }
      }
    }

    const postBalances = await Promise.all(chunk.map(sender => freeBalance(api, sender.address)));
    const underfunded = postBalances.filter(balance => balance < minFree).length;
    if (underfunded > 0) {
      throw new Error(`funding verification failed: ${underfunded}/${chunk.length} senders remain below ${minFree}`);
    }
    console.log(`[mainchain-transfer-burst] funded ${Math.min(i + chunk.length, needFunding.length)}/${needFunding.length}`);
  }
}

async function sendOne(api, signer, receiver, nonce, cfg, stats) {
  const tx = transferTx(api, receiver.address, cfg.amount);
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

  const kind = await included(api, tx, signer, nonce, cfg.txTimeout);
  stats.finish(kind);
  return kind;
}

function writeSummary(path, cfg, stats) {
  const elapsedSeconds = stats.elapsedSeconds();
  const summary = {
    ws: cfg.ws,
    senders: cfg.senders,
    senderOffset: cfg.senderOffset,
    parallelPerSender: cfg.parallelPerSender,
    amount: cfg.amount.toString(),
    fundAmount: cfg.fundAmount.toString(),
    fundBatchSize: cfg.fundBatchSize,
    receiverMode: cfg.receiverMode,
    duration: cfg.duration,
    submitMode: cfg.submitMode,
    sent: stats.sent,
    ok: stats.ok,
    reject: stats.reject,
    fail: stats.fail,
    elapsedSeconds,
    mainchainMaxTps: stats.ok / elapsedSeconds,
    finishedAt: new Date().toISOString(),
  };
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, `${JSON.stringify(summary, null, 2)}\n`);
  console.log(`[mainchain-transfer-burst] summary written: ${path}`);
}

async function main() {
  const cfg = parseArgs();
  console.log(`[mainchain-transfer-burst] ws=${cfg.ws}`);
  console.log(`[mainchain-transfer-burst] senders=${cfg.senders} sender_offset=${cfg.senderOffset} parallel_per_sender=${cfg.parallelPerSender}`);
  console.log(`[mainchain-transfer-burst] amount=${cfg.amount} fund_amount=${cfg.fundAmount} duration=${cfg.duration}s submit_mode=${cfg.submitMode} receiver_mode=${cfg.receiverMode}`);

  const api = await ApiPromise.create({ provider: new WsProvider(cfg.ws) });
  const keyring = new Keyring({ type: "sr25519" });
  const alice = keyring.addFromUri("//Alice");
  const senders = Array.from({ length: cfg.senders }, (_, i) => keyring.addFromUri(`//MainBenchSender${cfg.senderOffset + i}`));
  const receivers = cfg.receiverMode === "alice"
    ? Array.from({ length: cfg.senders }, () => alice)
    : Array.from({ length: cfg.senders }, (_, i) => keyring.addFromUri(`//MainBenchReceiver${cfg.senderOffset + i}`));

  await fundSenders(api, alice, senders, cfg);

  const stats = new Stats();
  const nextNonce = await Promise.all(senders.map(sender => api.rpc.system.accountNextIndex(sender.address)));
  const nonceNums = nextNonce.map(n => n.toNumber());
  const retryNonceQueues = Array.from({ length: cfg.senders }, () => []);
  const retryNonceSets = Array.from({ length: cfg.senders }, () => new Set());
  console.log(`[mainchain-transfer-burst] nonce range loaded for ${nonceNums.length} senders`);

  const takeNonce = senderIndex => {
    if (retryNonceQueues[senderIndex].length > 0) {
      const nonce = retryNonceQueues[senderIndex].shift();
      retryNonceSets[senderIndex].delete(nonce);
      return nonce;
    }
    const nonce = nonceNums[senderIndex];
    nonceNums[senderIndex]++;
    return nonce;
  };

  const retryNonce = (senderIndex, nonce) => {
    if (retryNonceSets[senderIndex].has(nonce)) return;
    retryNonceSets[senderIndex].add(nonce);
    retryNonceQueues[senderIndex].push(nonce);
    retryNonceQueues[senderIndex].sort((a, b) => a - b);
  };

  let running = true;
  const stopTimer = setTimeout(() => { running = false; }, cfg.duration * 1000);
  const reportTimer = setInterval(() => stats.report(), cfg.reportInterval * 1000);
  process.on("SIGTERM", () => { running = false; });
  process.on("SIGINT", () => { running = false; });

  const loops = [];
  for (let i = 0; i < senders.length; i++) {
    for (let lane = 0; lane < cfg.parallelPerSender; lane++) {
      loops.push((async () => {
        while (running) {
          const nonce = takeNonce(i);
          const kind = await sendOne(api, senders[i], receivers[i], nonce, cfg, stats);
          if (cfg.submitMode === "pool" && kind === "retry") {
            retryNonce(i, nonce);
            await new Promise(r => setTimeout(r, 50));
          } else if (kind !== "ok") {
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
  writeSummary(cfg.out, cfg, stats);
  await api.disconnect();
}

main().catch(e => {
  console.error("[mainchain-transfer-burst fatal]", e.message);
  process.exit(1);
});
