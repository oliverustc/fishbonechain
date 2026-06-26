/**
 * FishboneChain 众包场景桥接脚本
 *
 * 监听子链 crowdsource.EpochFinalized 事件，自动向主链提交：
 *   ccmc.submitEpochDigest(chainId, epoch, merkleRoot)
 *   fmc.submitBill(requester, taskId, epoch, billAmounts)
 */

import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";

const CHILD_WS = process.env.CHILD_WS || "ws://127.0.0.1:9945";
const MAIN_WS = process.env.MAIN_WS || "ws://127.0.0.1:9944";
const TASK_ID = parseInt(process.env.TASK_ID ?? "0", 10);
const CHAIN_ID = parseInt(process.env.CHAIN_ID ?? "0", 10);
const EXIT_AFTER_EVENTS = (() => {
  const i = process.argv.indexOf("--exit-after-events");
  return i === -1 ? 0 : Number(process.argv[i + 1] || "0");
})();
const ONCE = process.argv.includes("--once") || EXIT_AFTER_EVENTS === 1;

const REQUESTER = process.env.REQUESTER || "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY";

const minerSuris = process.env.MINER_SURIS
  ? process.env.MINER_SURIS.split(",").map((s) => s.trim()).filter(Boolean)
  : [process.env.MINER_SURI || "//Alice"];

function log(msg) {
  console.log(`[crowdsource-bridge ${new Date().toISOString()}] ${msg}`);
}

async function sendTx(api, tx, signer, label) {
  return new Promise((resolve, reject) => {
    tx.signAndSend(signer, ({ status, dispatchError }) => {
      if (dispatchError) {
        if (dispatchError.isModule) {
          const { name } = api.registry.findMetaError(dispatchError.asModule);
          if (name === "AlreadyVoted") {
            log(`  [跳过] ${label}: AlreadyVoted`);
            resolve({ skipped: true, settled: false });
            return;
          }
          if (name === "NotAMiner") {
            log(`  [跳过] ${label}: NotAMiner`);
            resolve({ skipped: true, settled: false });
            return;
          }
        }
        reject(new Error(`${label} failed: ${dispatchError}`));
      } else if (status.isInBlock) {
        log(`  OK ${label} 已上链 block=${status.asInBlock}`);
        resolve({ skipped: false, settled: true });
      }
    }).catch(reject);
  });
}

function eventChainId(rawChainId) {
  return rawChainId?.toNumber?.() ?? rawChainId;
}

async function submitToMainChain(mainApi, miners, chainId, epoch, merkleRoot, billAmounts) {
  log(`提交 Epoch=${epoch} 摘要到主链 (chain_id=${chainId})...`);
  await sendTx(
    mainApi,
    mainApi.tx.ccmc.submitEpochDigest(chainId, epoch, merkleRoot),
    miners[0],
    `ccmc.submitEpochDigest(chain=${chainId}, epoch=${epoch})`,
  );

  if (billAmounts.length === 0) {
    log("  本 Epoch 无账单，跳过 FMC 提交");
    return;
  }

  log(`  提交账单 task_id=${TASK_ID}, requester=${REQUESTER.slice(0, 8)}..., 共 ${billAmounts.length} 笔`);
  log(`  矿工投票（共 ${miners.length} 个矿工）...`);

  const amounts = billAmounts.map(([addr, amt]) => [addr.toString(), amt.toString()]);

  let voteCount = 0;
  for (const miner of miners) {
    const { skipped } = await sendTx(
      mainApi,
      mainApi.tx.fmc.submitBill(REQUESTER, TASK_ID, epoch, amounts),
      miner,
      `fmc.submitBill(task=${TASK_ID}, epoch=${epoch}, miner=${miner.address.slice(0, 8)}...)`,
    );
    if (!skipped) voteCount++;
  }
  log(`  所有矿工投票完毕（${voteCount}/${miners.length} 票有效，阈值由 pallet 自动判断）`);
}

async function main() {
  log("启动众包桥接服务");
  log(`子链 RPC:  ${CHILD_WS}`);
  log(`主链 RPC:  ${MAIN_WS}`);
  log(`CHAIN_ID:  ${CHAIN_ID}`);
  log(`TASK_ID:   ${TASK_ID}`);
  log(`REQUESTER: ${REQUESTER.slice(0, 12)}...`);

  const [childApi, mainApi] = await Promise.all([
    ApiPromise.create({ provider: new WsProvider(CHILD_WS) }),
    ApiPromise.create({ provider: new WsProvider(MAIN_WS) }),
  ]);

  const keyring = new Keyring({ type: "sr25519" });
  const miners = minerSuris.map((uri) => keyring.addFromUri(uri));
  log(`矿工账户（${miners.length} 个）: ${miners.map((m) => `${m.address.slice(0, 8)}...`).join(", ")}`);
  log(`子链: ${await childApi.rpc.system.chain()}  主链: ${await mainApi.rpc.system.chain()}`);

  let processedCount = 0;

  const unsub = await childApi.query.system.events(async (events) => {
    for (const { event } of events) {
      if (event.section !== "crowdsource" || event.method !== "EpochFinalized") continue;

      const data = event.data;
      const rawChainId = data.chain_id ?? data.chainId;
      const rawEpoch = data.epoch ?? data.epochId;
      const rawRoot = data.merkle_root ?? data.merkleRoot;
      const rawBillAmts = data.bill_amounts ?? data.billAmounts ?? [];

      const observedChainId = eventChainId(rawChainId);
      if (observedChainId !== undefined && observedChainId !== CHAIN_ID) {
        throw new Error(`event chain_id ${observedChainId} does not match configured CHAIN_ID ${CHAIN_ID}`);
      }

      const chainId = CHAIN_ID;
      const epoch = rawEpoch?.toNumber?.() ?? rawEpoch;
      const merkleRoot = rawRoot;
      const billAmounts = rawBillAmts;

      log("\n-- EpochFinalized ----------------------");
      log(`  chain_id  = ${chainId}`);
      log(`  epoch     = ${epoch}`);
      log(`  root      = ${merkleRoot}`);
      log(`  accounts  = ${billAmounts.length}`);

      try {
        await submitToMainChain(mainApi, miners, chainId, epoch, merkleRoot, billAmounts);
        processedCount++;
        log(`  [OK] Epoch ${epoch} 处理完毕（累计 ${processedCount}）`);
      } catch (e) {
        log(`[错误] ${e.message}`);
      }

      if (EXIT_AFTER_EVENTS > 0 && processedCount >= EXIT_AFTER_EVENTS) {
        log(`\n--exit-after-events=${EXIT_AFTER_EVENTS}: 已处理 ${processedCount} 个 Epoch，退出`);
        unsub();
        await Promise.all([childApi.disconnect(), mainApi.disconnect()]);
        process.exit(0);
      }

      if (ONCE) {
        log(`\n--once 模式：处理 ${processedCount} 个 Epoch，退出`);
        unsub();
        await Promise.all([childApi.disconnect(), mainApi.disconnect()]);
        process.exit(0);
      }
    }
  });

  if (ONCE) {
    await new Promise((_, reject) => {
      setTimeout(() => reject(new Error("超时：5 分钟内未收到 EpochFinalized")), 300_000);
    });
  } else {
    log("持续监听中（Ctrl+C 退出）...");
    process.on("SIGINT", async () => {
      log("收到 SIGINT，退出...");
      unsub();
      await Promise.all([childApi.disconnect(), mainApi.disconnect()]);
      process.exit(0);
    });
    await new Promise(() => {});
  }
}

main().catch((e) => {
  console.error("[crowdsource-bridge 致命错误]", e.message);
  process.exit(1);
});
