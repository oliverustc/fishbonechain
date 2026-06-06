/**
 * FishboneChain 指标采集工具
 *
 * 每 30s 轮询各子链 RPC，记录 epoch 状态到 CSV。
 * 同时订阅 EpochFinalized 事件，记录结算摘要。
 *
 * 用法：
 *   node scripts/metrics.js [选项]
 *
 * 参数：
 *   --chains <urls>   逗号分隔的子链 WS 地址（默认 ws://127.0.0.1:9945）
 *   --out <path>      CSV 输出路径前缀（默认 ./metrics）
 *                     最终文件：./metrics_state.csv 和 ./metrics_epoch.csv
 *   --interval <s>    轮询间隔秒数（默认 30）
 *
 * 示例：
 *   node scripts/metrics.js \
 *     --chains ws://127.0.0.1:9945,ws://127.0.0.1:9946 \
 *     --out /tmp/fishbone \
 *     --interval 15
 */

import { ApiPromise, WsProvider } from "@polkadot/api";
import { createWriteStream } from "fs";

// ─── CLI 解析 ────────────────────────────────────────────────────────────────

function parseArgs() {
  const args = process.argv.slice(2);
  const get  = (flag, def) => { const i = args.indexOf(flag); return i !== -1 ? args[i + 1] : def; };
  return {
    chains:   (get("--chains", "ws://127.0.0.1:9945")).split(","),
    out:      get("--out", "./metrics"),
    interval: Number(get("--interval", "30")),
  };
}

// ─── CSV 写入 ────────────────────────────────────────────────────────────────

function csvWriter(path, header) {
  const stream = createWriteStream(path, { flags: "a" });
  let headerWritten = false;

  return {
    write(row) {
      if (!headerWritten) {
        stream.write(header + "\n");
        headerWritten = true;
      }
      stream.write(row + "\n");
    },
    close() { stream.end(); },
  };
}

function tsv(...vals) {
  return vals.map(v => String(v).replace(/,/g, ";")).join(",");
}

// ─── 主逻辑 ──────────────────────────────────────────────────────────────────

async function main() {
  const cfg = parseArgs();

  console.log(`[metrics] 链数: ${cfg.chains.length}  轮询间隔: ${cfg.interval}s`);
  console.log(`[metrics] 输出前缀: ${cfg.out}`);

  const stateCSV = csvWriter(
    `${cfg.out}_state.csv`,
    "timestamp,chain_url,block_number,epoch_id,phase,submissions_count",
  );
  const epochCSV = csvWriter(
    `${cfg.out}_epoch.csv`,
    "timestamp,chain_url,epoch_id,valid_subs,merkle_root,duration_s",
  );

  // 连接所有子链
  const apis = await Promise.all(
    cfg.chains.map(async url => {
      const api = await ApiPromise.create({ provider: new WsProvider(url) });
      console.log(`[metrics] 已连接: ${url} (${await api.rpc.system.chain()})`);
      return { url, api };
    })
  );

  // 订阅每条链的 EpochFinalized 事件
  const epochStartTs = new Map(); // chain_url:epochId → 开始时间戳
  const unsubs = await Promise.all(
    apis.map(({ url, api }) =>
      api.query.system.events(events => {
        for (const { event } of events) {
          if (!api.events.crowdsource?.EpochFinalized?.is(event)) continue;

          const { epoch, task_bills, merkle_root } = event.data;
          const epochId  = epoch.toNumber();
          const key      = `${url}:${epochId}`;
          const now      = new Date().toISOString();
          const startTs  = epochStartTs.get(key) ?? Date.now();
          const durationS = ((Date.now() - startTs) / 1000).toFixed(1);
          const validSubs = task_bills.reduce((s, b) => s + b.amounts.length, 0);

          console.log(`[metrics] ${url} EpochFinalized epoch=${epochId} subs=${validSubs} duration=${durationS}s`);
          epochCSV.write(tsv(now, url, epochId, validSubs, merkle_root.toString(), durationS));

          // 记录下一个 epoch 的开始时间
          epochStartTs.set(`${url}:${epochId + 1}`, Date.now());
        }
      })
    )
  );

  // 初始化 epoch 开始时间
  for (const { url, api } of apis) {
    try {
      const epochInfo = await api.query.crowdsource.currentEpoch();
      const epochId   = epochInfo.epochId?.toNumber() ?? epochInfo.epoch_id?.toNumber() ?? 0;
      epochStartTs.set(`${url}:${epochId}`, Date.now());
    } catch { /* pallet 不可用，跳过 */ }
  }

  // 定期轮询状态
  const pollOnce = async () => {
    const now = new Date().toISOString();
    for (const { url, api } of apis) {
      try {
        const [header, epochInfo, subs] = await Promise.all([
          api.rpc.chain.getHeader(),
          api.query.crowdsource.currentEpoch(),
          api.query.crowdsource.epochSubmissions(),
        ]);

        const blockNum  = header.number.toNumber();
        const epochId   = epochInfo.epochId?.toNumber() ?? epochInfo.epoch_id?.toNumber() ?? "?";
        const phase     = epochInfo.phase?.type ?? epochInfo.phase?.toString() ?? "?";
        const subCount  = subs.length;

        console.log(`[metrics] ${url} block=${blockNum} epoch=${epochId} phase=${phase} subs=${subCount}`);
        stateCSV.write(tsv(now, url, blockNum, epochId, phase, subCount));
      } catch (e) {
        console.warn(`[metrics] ${url} 轮询失败: ${e.message}`);
      }
    }
  };

  await pollOnce();
  const pollTimer = setInterval(pollOnce, cfg.interval * 1000);

  process.on("SIGINT", async () => {
    console.log("[metrics] 正在退出...");
    clearInterval(pollTimer);
    for (const u of unsubs) u();
    stateCSV.close();
    epochCSV.close();
    for (const { api } of apis) await api.disconnect();
    process.exit(0);
  });

  console.log("[metrics] 持续采集中（Ctrl+C 退出）...");
}

main().catch(e => {
  console.error("[metrics 致命错误]", e.message);
  process.exit(1);
});
