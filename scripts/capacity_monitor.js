/**
 * High-frequency capacity monitor.
 *
 * Records per-chain submission counts and the first time each chain reaches
 * the target cap. The run script uses this as the authoritative timing source
 * for maximum-throughput experiments.
 */

import { ApiPromise, WsProvider } from "@polkadot/api";
import { existsSync, readFileSync, writeFileSync, appendFileSync } from "fs";

function parseArgs() {
  const args = process.argv.slice(2);
  const get = (flag, def) => {
    const i = args.indexOf(flag);
    return i !== -1 ? args[i + 1] : def;
  };
  return {
    chains: get("--chains", "ws://127.0.0.1:9945").split(",").filter(Boolean),
    out: get("--out", "./capacity_precise.csv"),
    readyFile: get("--ready-file", ""),
    startFile: get("--start-file", ""),
    cap: Number(get("--cap", "10000")),
    intervalMs: Number(get("--interval-ms", "200")),
    timeoutSec: Number(get("--timeout", "300")),
  };
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

function csv(...vals) {
  return vals.map(v => String(v).replace(/,/g, ";")).join(",");
}

async function sample(api) {
  const hasAcceptedCounter = Boolean(api.query.crowdsource.acceptedSubmissionCount);
  const [header, epochInfo, countOrSubs] = await Promise.all([
    api.rpc.chain.getHeader(),
    api.query.crowdsource.currentEpoch(),
    hasAcceptedCounter
      ? api.query.crowdsource.acceptedSubmissionCount()
      : api.query.crowdsource.epochSubmissions(),
  ]);
  return {
    block: header.number.toNumber(),
    epoch: epochInfo.epochId?.toNumber() ?? epochInfo.epoch_id?.toNumber() ?? "?",
    phase: epochInfo.phase?.type ?? epochInfo.phase?.toString() ?? "?",
    subs: hasAcceptedCounter ? countOrSubs.toNumber() : countOrSubs.length,
  };
}

async function waitForStartFile(path) {
  while (true) {
    if (existsSync(path)) {
      const value = Number(readFileSync(path, "utf8").trim());
      if (Number.isFinite(value) && value > 0) {
        return value;
      }
    }
    await sleep(20);
  }
}

async function main() {
  const cfg = parseArgs();
  if (!cfg.readyFile || !cfg.startFile) {
    throw new Error("--ready-file and --start-file are required");
  }

  console.log(`[capacity_monitor] chains=${cfg.chains.length} cap=${cfg.cap} interval_ms=${cfg.intervalMs}`);
  const apis = await Promise.all(cfg.chains.map(async url => {
    const api = await ApiPromise.create({ provider: new WsProvider(url) });
    console.log(`[capacity_monitor] connected ${url}`);
    return { url, api };
  }));

  const baseline = {};
  for (const { url, api } of apis) {
    baseline[url] = await sample(api);
  }

  writeFileSync(cfg.out, "timestamp_ms,elapsed_s,chain_url,block_number,epoch_id,phase,submissions_count,delta_from_initial,hit_cap\n");
  writeFileSync(cfg.readyFile, JSON.stringify({
    ready_at_ms: Date.now(),
    cap: cfg.cap,
    baseline,
  }, null, 2));

  const startMs = await waitForStartFile(cfg.startFile);
  console.log(`[capacity_monitor] started start_ms=${startMs}`);

  const hit = new Map();
  const deadline = startMs + cfg.timeoutSec * 1000;

  while (Date.now() <= deadline) {
    const nowMs = Date.now();
    const elapsedS = Math.max((nowMs - startMs) / 1000, 0);

    for (const { url, api } of apis) {
      try {
        const item = await sample(api);
        const initial = baseline[url]?.subs ?? 0;
        const delta = Math.max(0, item.subs - initial);
        const didHit = item.subs >= cfg.cap ? 1 : 0;
        appendFileSync(cfg.out, csv(
          nowMs,
          elapsedS.toFixed(3),
          url,
          item.block,
          item.epoch,
          item.phase,
          item.subs,
          delta,
          didHit,
        ) + "\n");

        if (didHit && !hit.has(url)) {
          hit.set(url, {
            hit_at_ms: nowMs,
            elapsed_s: elapsedS,
            initial_subs: initial,
            cap_subs: item.subs,
            accepted_delta: delta,
            block_number: item.block,
            epoch_id: item.epoch,
          });
          console.log(`[capacity_monitor] hit ${url} elapsed=${elapsedS.toFixed(3)}s delta=${delta}`);
        }
      } catch (e) {
        console.warn(`[capacity_monitor] sample failed ${url}: ${e.message}`);
      }
    }

    if (hit.size === apis.length) {
      break;
    }
    await sleep(cfg.intervalMs);
  }

  const summary = {};
  for (const { url } of apis) {
    summary[url] = hit.get(url) ?? null;
  }
  writeFileSync(cfg.out.replace(/\.csv$/, "_summary.json"), JSON.stringify({
    start_ms: startMs,
    finished_at_ms: Date.now(),
    cap: cfg.cap,
    baseline,
    hit_summary: summary,
  }, null, 2));

  for (const { api } of apis) {
    await api.disconnect();
  }

  process.exit(hit.size === apis.length ? 0 : 2);
}

main().catch(e => {
  console.error("[capacity_monitor fatal]", e.message);
  process.exit(1);
});
