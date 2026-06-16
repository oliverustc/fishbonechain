import { ApiPromise, WsProvider } from "@polkadot/api";

function parseArgs(argv) {
  const out = { _: [] };
  for (let i = 0; i < argv.length; i++) {
    const item = argv[i];
    if (item.startsWith("--")) {
      out[item.slice(2)] = argv[++i];
    } else {
      out._.push(item);
    }
  }
  return out;
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function withTimeout(promise, ms, onTimeout) {
  let timeoutId;
  const timeout = new Promise((_, reject) => {
    timeoutId = setTimeout(() => {
      onTimeout?.();
      reject(new Error(`timed out after ${ms}ms`));
    }, ms);
  });

  try {
    return await Promise.race([promise, timeout]);
  } finally {
    clearTimeout(timeoutId);
  }
}

async function readHeader(url, attemptTimeoutMs) {
  const provider = new WsProvider(url);
  let api;
  try {
    api = await withTimeout(
      ApiPromise.create({ provider }),
      attemptTimeoutMs,
      () => provider.disconnect(),
    );
    const chain = String(await withTimeout(api.rpc.system.chain(), attemptTimeoutMs));
    const header = await withTimeout(api.rpc.chain.getHeader(), attemptTimeoutMs);
    return { chain, block: header.number.toNumber() };
  } finally {
    if (api) {
      await api.disconnect();
    } else {
      provider.disconnect();
    }
  }
}

async function waitForChain(url, label, minBlocks, deadlineMs, attemptTimeoutMs, intervalMs) {
  const started = Date.now();
  let firstBlock = null;
  let lastError = null;

  while (Date.now() - started < deadlineMs) {
    try {
      const header = await readHeader(url, attemptTimeoutMs);
      firstBlock ??= header.block;
      const advanced = header.block - firstBlock;
      console.log(`[wait-for-ws-chain] ${label} ${header.chain} #${header.block} advanced=${advanced}/${minBlocks}`);
      if (advanced >= minBlocks) {
        return;
      }
    } catch (error) {
      lastError = error;
      console.log(`[wait-for-ws-chain] ${label} not ready: ${error.message}`);
    }
    await sleep(intervalMs);
  }

  throw new Error(`${label} ${url} did not become ready before deadline: ${lastError?.message ?? "no successful header reads"}`);
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const main = args.main;
  const child = args.child;
  if (!main || !child) {
    throw new Error("usage: wait_for_ws_chain.js --main WS --child WS [--min-blocks N] [--timeout-ms MS]");
  }

  const minBlocks = Number(args["min-blocks"] ?? 2);
  const deadlineMs = Number(args["timeout-ms"] ?? 180000);
  const attemptTimeoutMs = Number(args["attempt-timeout-ms"] ?? 15000);
  const intervalMs = Number(args["interval-ms"] ?? 5000);

  if (!Number.isInteger(minBlocks) || minBlocks < 0) throw new Error("--min-blocks must be a non-negative integer");
  if (!Number.isInteger(deadlineMs) || deadlineMs <= 0) throw new Error("--timeout-ms must be a positive integer");
  if (!Number.isInteger(attemptTimeoutMs) || attemptTimeoutMs <= 0) throw new Error("--attempt-timeout-ms must be a positive integer");
  if (!Number.isInteger(intervalMs) || intervalMs <= 0) throw new Error("--interval-ms must be a positive integer");

  await waitForChain(main, "main", minBlocks, deadlineMs, attemptTimeoutMs, intervalMs);
  await waitForChain(child, "child", minBlocks, deadlineMs, attemptTimeoutMs, intervalMs);
}

main().catch((error) => {
  console.error(error.message);
  process.exit(1);
});
