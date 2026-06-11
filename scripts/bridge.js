/**
 * FishboneChain 链下桥接脚本
 *
 * 监听子链 EpochFinalized 事件，自动向主链提交：
 *   ccmc.submitEpochDigest(chainId, epoch, merkleRoot)
 *   fmc.submitBill(requester, taskId, epoch, billAmounts)
 *
 * 用法（单矿工，测试场景 miner_count=1）：
 *   CHILD_WS=ws://... MAIN_WS=ws://... MINER_SURI="//Alice" \
 *   REQUESTER="5Grw..." TASK_ID=0 CHAIN_ID=0 \
 *   node scripts/bridge.js
 *
 * 用法（多矿工，模拟生产场景，逗号分隔所有矿工的 SURI）：
 *   MINER_SURIS="seed1,seed2,seed3,seed4,seed5" \
 *   CHILD_WS=ws://... MAIN_WS=ws://... TASK_ID=3 CHAIN_ID=3 \
 *   node scripts/bridge.js
 *
 * 环境变量：
 *   CHILD_WS      子链 RPC（默认 ws://127.0.0.1:9945）
 *   MAIN_WS       主链 RPC（默认 ws://127.0.0.1:9944）
 *   MINER_SURI    单个矿工 SURI（默认 //Alice；被 MINER_SURIS 覆盖）
 *   MINER_SURIS   逗号分隔的多个矿工 SURI（用于达到 2/3 投票阈值）
 *   REQUESTER     任务请求方地址（默认 Alice SS58）
 *   TASK_ID       任务 ID（默认 0）
 *   CHAIN_ID      子链 ID（默认 0）
 */

import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";

const CHILD_WS   = process.env.CHILD_WS   || "ws://127.0.0.1:9945";
const MAIN_WS    = process.env.MAIN_WS    || "ws://127.0.0.1:9944";
const TASK_ID    = parseInt(process.env.TASK_ID ?? "0", 10);
const CHAIN_ID   = parseInt(process.env.CHAIN_ID ?? "0", 10);
const ONCE       = process.argv.includes("--once");

// REQUESTER 默认为 Alice 的 SS58 地址
const REQUESTER  = process.env.REQUESTER  || "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY";

// 支持多矿工：MINER_SURIS 优先，否则降级到 MINER_SURI 单个
const _minerSuris = process.env.MINER_SURIS
  ? process.env.MINER_SURIS.split(",").map(s => s.trim()).filter(Boolean)
  : [process.env.MINER_SURI || "//Alice"];

function log(msg) {
  console.log(`[bridge ${new Date().toISOString()}] ${msg}`);
}

// resolve({ skipped, settled }) — skipped=已跳过, settled=本次触发了结算
async function sendTx(api, tx, signer, label) {
  return new Promise((resolve, reject) => {
    tx.signAndSend(signer, ({ status, dispatchError }) => {
      if (dispatchError) {
        if (dispatchError.isModule) {
          const { name } = api.registry.findMetaError(dispatchError.asModule);
          if (name === "AlreadyVoted") {
            log(`  [跳过] ${label}：AlreadyVoted（本矿工已投票）`);
            resolve({ skipped: true, settled: false });
            return;
          }
          if (name === "NotAMiner") {
            log(`  [跳过] ${label}：NotAMiner（账户未注册为该链矿工，检查 MINER_SURIS 配置）`);
            resolve({ skipped: true, settled: false });
            return;
          }
        }
        reject(new Error(`${label} failed: ${dispatchError}`));
      } else if (status.isInBlock) {
        log(`  ✓ ${label} 已上链 block=${status.asInBlock}`);
        resolve({ skipped: false, settled: true });
      }
    }).catch(reject);
  });
}

async function submitToMainChain(mainApi, miners, chainId, epoch, merkleRoot, billAmounts) {
  // 1. 提交 Epoch 摘要到 CCMC（用第一个矿工提交，AlreadyVoted 说明已提交）
  log(`提交 Epoch=${epoch} 摘要到主链 (chain_id=${chainId})...`);
  await sendTx(
    mainApi,
    mainApi.tx.ccmc.submitEpochDigest(chainId, epoch, merkleRoot),
    miners[0],
    `ccmc.submitEpochDigest(chain=${chainId}, epoch=${epoch})`
  );

  // 2. 提交账单到 FMC（多矿工依次投票，达到 2/3 阈值后结算自动触发）
  if (billAmounts.length === 0) {
    log("  本 Epoch 无账单，跳过 FMC 提交");
    return;
  }

  log(`  提交账单 task_id=${TASK_ID}，requester=${REQUESTER.slice(0,8)}…，共 ${billAmounts.length} 笔`);
  log(`  矿工投票（共 ${miners.length} 个矿工）...`);

  // bill_amounts 格式：[(AccountId, Balance), ...]
  const amounts = billAmounts.map(([addr, amt]) => [addr.toString(), amt.toString()]);

  // 每个矿工依次投票直到全部提交。settled（inBlock）不等于 FMC 已结算——
  // 实际结算由 pallet 在达到 2/3 阈值后自动触发 BillSettled 事件。
  let voteCount = 0;
  for (const miner of miners) {
    const { skipped } = await sendTx(
      mainApi,
      mainApi.tx.fmc.submitBill(REQUESTER, TASK_ID, epoch, amounts),
      miner,
      `fmc.submitBill(task=${TASK_ID}, epoch=${epoch}, miner=${miner.address.slice(0,8)}…)`
    );
    if (!skipped) voteCount++;
  }
  log(`  所有矿工投票完毕（${voteCount}/${miners.length} 票有效，阈值由 pallet 自动判断）`);
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
  const miners  = _minerSuris.map(uri => keyring.addFromUri(uri));
  log(`矿工账户（${miners.length} 个）: ${miners.map(m => m.address.slice(0,8)+'…').join(', ')}`);
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
          mainApi, miners,
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
