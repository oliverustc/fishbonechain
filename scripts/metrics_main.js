/**
 * FishboneChain main-chain load collector.
 *
 * Collects finalized main-chain blocks and records:
 *   - total extrinsics per block
 *   - bridge-related extrinsics per block
 *   - CCMC/FMC events per block
 *
 * Usage:
 *   MAIN_WS=ws://10.2.2.11:9944 \
 *   node scripts/metrics_main.js --out /tmp/exp_scale_main --interval 6
 *
 * Output:
 *   <out>_main_blocks.csv
 */

import { ApiPromise, WsProvider } from "@polkadot/api";
import { createWriteStream } from "fs";

function parseArgs() {
  const args = process.argv.slice(2);
  const get = (flag, def) => {
    const i = args.indexOf(flag);
    return i !== -1 ? args[i + 1] : def;
  };
  return {
    out: get("--out", "/tmp/exp_scale_main"),
    interval: Number(get("--interval", "6")),
  };
}

const MAIN_WS = process.env.MAIN_WS || "ws://127.0.0.1:9944";

function csvWriter(path, header) {
  const stream = createWriteStream(path, { flags: "a" });
  let written = false;
  return {
    write(row) {
      if (!written) {
        stream.write(header + "\n");
        written = true;
      }
      stream.write(row + "\n");
    },
    close() {
      stream.end();
    },
  };
}

function csv(...vals) {
  return vals.map(v => String(v ?? "").replace(/,/g, ";")).join(",");
}

function isBridgeCall(method) {
  const section = method.section;
  const name = method.method;
  return (
    (section === "ccmc" && name === "submitEpochDigest") ||
    (section === "fmc" && name === "submitBill")
  );
}

async function main() {
  const cfg = parseArgs();
  const csvPath = `${cfg.out}_main_blocks.csv`;

  console.log(`[metrics_main] 主链: ${MAIN_WS}`);
  console.log(`[metrics_main] 输出: ${csvPath}`);
  console.log(`[metrics_main] 轮询间隔: ${cfg.interval}s`);

  const api = await ApiPromise.create({ provider: new WsProvider(MAIN_WS) });
  const writer = csvWriter(
    csvPath,
    [
      "timestamp",
      "block_number",
      "block_hash",
      "extrinsics_total",
      "bridge_extrinsics",
      "ccmc_digest_calls",
      "fmc_bill_calls",
      "ccmc_events",
      "fmc_events",
    ].join(","),
  );

  let lastBlock = -1;

  async function recordBlock(blockNumber) {
    const hash = await api.rpc.chain.getBlockHash(blockNumber);
    const signedBlock = await api.rpc.chain.getBlock(hash);
    const events = await api.query.system.events.at(hash);

    let bridgeExtrinsics = 0;
    let ccmcDigestCalls = 0;
    let fmcBillCalls = 0;

    for (const ext of signedBlock.block.extrinsics) {
      const method = ext.method;
      if (!isBridgeCall(method)) continue;
      bridgeExtrinsics++;
      if (method.section === "ccmc" && method.method === "submitEpochDigest") {
        ccmcDigestCalls++;
      }
      if (method.section === "fmc" && method.method === "submitBill") {
        fmcBillCalls++;
      }
    }

    let ccmcEvents = 0;
    let fmcEvents = 0;
    for (const { event } of events) {
      if (event.section === "ccmc") ccmcEvents++;
      if (event.section === "fmc") fmcEvents++;
    }

    const ts = new Date().toISOString();
    writer.write(csv(
      ts,
      blockNumber,
      hash.toString(),
      signedBlock.block.extrinsics.length,
      bridgeExtrinsics,
      ccmcDigestCalls,
      fmcBillCalls,
      ccmcEvents,
      fmcEvents,
    ));

    console.log(
      `[${ts}] block=${blockNumber} extrinsics=${signedBlock.block.extrinsics.length} ` +
      `bridge=${bridgeExtrinsics} ccmc=${ccmcDigestCalls}/${ccmcEvents} fmc=${fmcBillCalls}/${fmcEvents}`,
    );
  }

  async function sample() {
    const headHash = await api.rpc.chain.getFinalizedHead();
    const headHeader = await api.rpc.chain.getHeader(headHash);
    const headNumber = headHeader.number.toNumber();

    const start = lastBlock < 0 ? headNumber : lastBlock + 1;
    for (let blockNumber = start; blockNumber <= headNumber; blockNumber++) {
      await recordBlock(blockNumber);
    }
    lastBlock = headNumber;
  }

  await sample();
  const timer = setInterval(() => {
    sample().catch(e => console.warn(`[metrics_main] 采样失败: ${e.message}`));
  }, cfg.interval * 1000);

  process.on("SIGINT", async () => {
    clearInterval(timer);
    writer.close();
    await api.disconnect();
    console.log("\n[metrics_main] 已停止");
    process.exit(0);
  });

  await new Promise(() => {});
}

main().catch(e => {
  console.error("[metrics_main 致命错误]", e.message);
  process.exit(1);
});
