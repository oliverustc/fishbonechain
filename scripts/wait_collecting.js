/**
 * Wait until all given child chains are in Collecting phase and have at most
 * the configured number of existing epoch submissions.
 */

import { ApiPromise, WsProvider } from "@polkadot/api";

function parseArgs() {
  const args = process.argv.slice(2);
  const get = (flag, def) => {
    const i = args.indexOf(flag);
    return i !== -1 ? args[i + 1] : def;
  };
  return {
    chains: get("--chains", "ws://127.0.0.1:9945").split(",").filter(Boolean),
    maxSubs: Number(get("--max-subs", "50")),
    minRemainingCollectingBlocks: Number(get("--min-remaining-collecting-blocks", "80")),
    interval: Number(get("--interval", "10")),
    timeout: Number(get("--timeout", "1800")),
  };
}

async function chainState(api) {
  const [header, epochInfo, subs] = await Promise.all([
    api.rpc.chain.getHeader(),
    api.query.crowdsource.currentEpoch(),
    api.query.crowdsource.epochSubmissions(),
  ]);
  const block = header.number.toNumber();
  const startBlock = epochInfo.startBlock?.toNumber() ?? epochInfo.start_block?.toNumber() ?? 0;
  const collectingSlotBlocks = api.consts.crowdsource.collectingSlotBlocks.toNumber();
  const elapsedBlocks = Math.max(block - startBlock, 0);
  const remainingCollectingBlocks = Math.max(collectingSlotBlocks - elapsedBlocks, 0);
  return {
    block,
    epoch: epochInfo.epochId?.toNumber() ?? epochInfo.epoch_id?.toNumber() ?? 0,
    phase: epochInfo.phase?.type ?? epochInfo.phase?.toString() ?? "?",
    startBlock,
    elapsedBlocks,
    collectingSlotBlocks,
    remainingCollectingBlocks,
    submissions: subs.length,
  };
}

async function main() {
  const cfg = parseArgs();
  const apis = await Promise.all(cfg.chains.map(async url => ({
    url,
    api: await ApiPromise.create({ provider: new WsProvider(url) }),
  })));

  const start = Date.now();
  while (true) {
    const states = await Promise.all(apis.map(async item => ({
      url: item.url,
      ...(await chainState(item.api)),
    })));

    const line = states
      .map(s => `${s.url} epoch=${s.epoch} phase=${s.phase} subs=${s.submissions}` +
        ` block=${s.block} elapsed=${s.elapsedBlocks}/${s.collectingSlotBlocks}` +
        ` remaining_collecting=${s.remainingCollectingBlocks}`)
      .join(" | ");
    console.log(`[wait_collecting] ${line}`);

    const ready = states.every(s =>
      s.phase === "Collecting" &&
      s.submissions <= cfg.maxSubs &&
      s.remainingCollectingBlocks >= cfg.minRemainingCollectingBlocks
    );
    if (ready) {
      await Promise.all(apis.map(({ api }) => api.disconnect()));
      console.log("[wait_collecting] ready");
      return;
    }

    if ((Date.now() - start) / 1000 > cfg.timeout) {
      await Promise.all(apis.map(({ api }) => api.disconnect()));
      throw new Error(`timeout after ${cfg.timeout}s`);
    }

    await new Promise(r => setTimeout(r, cfg.interval * 1000));
  }
}

main().catch(e => {
  console.error("[wait_collecting fatal]", e.message);
  process.exit(1);
});
