/**
 * FishboneChain 链下桥接脚本
 *
 * 监听子链 EpochFinalized 事件，自动向主链提交：
 *   ccmc.submitEpochDigest(chainId, epoch, merkleRoot)
 *   fmc.submitBill(requester, taskId, epoch, billAmounts)
 *
 * 用法：
 *   CHILD_WS=ws://... MAIN_WS=ws://... MINER_SURI="//Alice" \
 *   REQUESTER="5Grw..." TASK_ID=0 \
 *   node scripts/bridge.js
 *
 * 环境变量：
 *   CHILD_WS      子链 RPC（默认 ws://127.0.0.1:9945）
 *   MAIN_WS       主链 RPC（默认 ws://127.0.0.1:9944）
 *   MINER_SURI    矿工账户助记词或 //URI（默认 //Alice）
 *   REQUESTER     任务请求方地址（默认 Alice SS58）
 *   TASK_ID       任务 ID（默认 0）
 */

import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";

const CHILD_WS   = process.env.CHILD_WS   || "ws://127.0.0.1:9945";
const MAIN_WS    = process.env.MAIN_WS    || "ws://127.0.0.1:9944";
const MINER_SURI = process.env.MINER_SURI || "//Alice";
const TASK_ID    = parseInt(process.env.TASK_ID ?? "0", 10);
const CHAIN_ID   = parseInt(process.env.CHAIN_ID ?? "0", 10);
const ONCE       = process.argv.includes("--once");

// REQUESTER 默认为 Alice 的 SS58 地址
const REQUESTER  = process.env.REQUESTER  || "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY";

function log(msg) {
  console.log(`[bridge ${new Date().toISOString()}] ${msg}`);
}

async function sendTx(api, tx, signer, label) {
  return new Promise((resolve, reject) => {
    tx.signAndSend(signer, ({ status, dispatchError }) => {
      if (dispatchError) {
        if (dispatchError.isModule) {
          const { name } = api.registry.findMetaError(dispatchError.asModule);
          if (name === "AlreadyVoted") {
            log(`  [跳过] ${label}：AlreadyVoted（其他矿工已提交）`);
            resolve({ skipped: true });
            return;
          }
        }
        reject(new Error(`${label} failed: ${dispatchError}`));
      } else if (status.isInBlock) {
        log(`  ✓ ${label} 已上链 block=${status.asInBlock}`);
        resolve({ skipped: false });
      }
    }).catch(reject);
  });
}

async function submitToMainChain(mainApi, miner, chainId, epoch, merkleRoot, billAmounts) {
  // 1. 提交 Epoch 摘要到 CCMC
  log(`提交 Epoch=${epoch} 摘要到主链 (chain_id=${chainId})...`);
  await sendTx(
    mainApi,
    mainApi.tx.ccmc.submitEpochDigest(chainId, epoch, merkleRoot),
    miner,
    `ccmc.submitEpochDigest(chain=${chainId}, epoch=${epoch})`
  );

  // 2. 提交账单到 FMC
  if (billAmounts.length === 0) {
    log("  本 Epoch 无账单，跳过 FMC 提交");
    return;
  }

  log(`  提交账单 task_id=${TASK_ID}，requester=${REQUESTER.slice(0,8)}…，共 ${billAmounts.length} 笔`);

  // bill_amounts 格式：[(AccountId, Balance), ...]
  const amounts = billAmounts.map(([addr, amt]) => [addr.toString(), amt.toString()]);

  await sendTx(
    mainApi,
    mainApi.tx.fmc.submitBill(REQUESTER, TASK_ID, epoch, amounts),
    miner,
    `fmc.submitBill(task=${TASK_ID}, epoch=${epoch}, recipients=${amounts.length})`
  );

  log(`  账单结算完成（若 2/3 矿工已提交则自动结算）`);
}

async function main() {
  log(`启动桥接服务`);
  log(`子链 RPC:  ${CHILD_WS}`);
  log(`主链 RPC:  ${MAIN_WS}`);
  log(`CHAIN_ID:  ${CHAIN_ID}`);
  log(`TASK_ID:   ${TASK_ID}`);
  log(`REQUESTER: ${REQUESTER.slice(0, 12)}…`);

  const [childApi, mainApi] = await Promise.all([
    ApiPromise.create({ provider: new WsProvider(CHILD_WS) }),
    ApiPromise.create({ provider: new WsProvider(MAIN_WS) }),
  ]);

  const keyring = new Keyring({ type: "sr25519" });
  const miner   = keyring.addFromUri(MINER_SURI);
  log(`矿工账户:  ${miner.address}`);
  log(`子链: ${await childApi.rpc.system.chain()}  主链: ${await mainApi.rpc.system.chain()}`);

  let processedCount = 0;

  const unsub = await childApi.query.system.events(async (events) => {
    for (const { event } of events) {
      // 使用 section/method 匹配，避免自定义 pallet 类型注册问题
      if (event.section !== "crowdsource" || event.method !== "EpochFinalized") continue;

      // 支持两种字段名格式（camelCase 和 snake_case）
      const data = event.data;
      const rawChainId    = data.chain_id    ?? data.chainId;
      const rawEpoch      = data.epoch       ?? data.epochId;
      const rawRoot       = data.merkle_root ?? data.merkleRoot;
      const rawBillAmts   = data.bill_amounts ?? data.billAmounts ?? [];

      // chain_id 优先从事件读取，回退到 CHAIN_ID 环境变量
      const chain_id   = rawChainId?.toNumber?.() ?? CHAIN_ID;
      const epoch      = rawEpoch?.toNumber?.()   ?? rawEpoch;
      const merkle_root = rawRoot;
      const bill_amounts = rawBillAmts;

      log(`\n── EpochFinalized ──────────────────────`);
      log(`  chain_id  = ${chain_id}`);
      log(`  epoch     = ${epoch}`);
      log(`  root      = ${merkle_root}`);
      log(`  accounts  = ${bill_amounts.length}`);

      try {
        await submitToMainChain(
          mainApi, miner,
          chain_id, epoch, merkle_root,
          bill_amounts,
        );
        processedCount++;
        log(`  [OK] Epoch ${epoch} 处理完毕（累计 ${processedCount}）`);
      } catch (e) {
        log(`[错误] ${e.message}`);
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
    await new Promise((_, reject) =>
      setTimeout(() => reject(new Error("超时：5 分钟内未收到 EpochFinalized")), 300_000)
    );
  } else {
    log("持续监听中（Ctrl+C 退出）...");
    process.on("SIGINT", async () => {
      log("收到 SIGINT，退出...");
      unsub();
      await Promise.all([childApi.disconnect(), mainApi.disconnect()]);
      process.exit(0);
    });
    await new Promise(() => {}); // keep alive
  }
}

main().catch(e => {
  console.error("[bridge 致命错误]", e.message);
  process.exit(1);
});
