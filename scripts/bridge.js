/**
 * FishboneChain Phase 2 链下桥接脚本
 *
 * 监听子链 EpochFinalized 事件，自动向主链提交：
 *   ccmc.submitEpochDigest(chainId, epoch, merkleRoot)
 *   fmc.submitBill(requester, taskId, epoch, billAmounts)
 *
 * 用法：
 *   node --input-type=module < scripts/bridge.js
 *   node --input-type=module < scripts/bridge.js --once   # 处理一次后退出
 */

import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";

const CHILD1_WS = process.env.CHILD_WS  || "ws://127.0.0.1:9945";
const MAIN_WS   = process.env.MAIN_WS   || "ws://127.0.0.1:9944";
const ONCE      = process.argv.includes("--once");

// 代表矿工（提交摘要和账单的签名者）
const MINER_SURI = process.env.MINER_SURI || "//Alice";

function log(msg) {
  console.log(`[bridge ${new Date().toISOString()}] ${msg}`);
}

async function submitToMainChain(mainApi, miner, chainId, epoch, merkleRoot, billAmounts) {
  log(`提交 Epoch ${epoch} 摘要到主链...`);

  // 1. 提交 Epoch 摘要到 CCMC
  await new Promise((resolve, reject) => {
    mainApi.tx.ccmc
      .submitEpochDigest(chainId, epoch, merkleRoot)
      .signAndSend(miner, ({ status, dispatchError }) => {
        if (dispatchError) {
          // AlreadyVoted 是正常情况（其他矿工已提交），不视为错误
          if (dispatchError.isModule) {
            const { name } = mainApi.registry.findMetaError(dispatchError.asModule);
            if (name === "AlreadyVoted") {
              log(`  Epoch ${epoch} 摘要已由他人提交，跳过`);
              resolve();
              return;
            }
          }
          reject(new Error(`ccmc.submitEpochDigest failed: ${dispatchError}`));
        } else if (status.isInBlock) {
          log(`  ✓ 摘要已上链 #${status.asInBlock}`);
          resolve();
        }
      }).catch(reject);
  });

  // 2. 按任务提交账单到 FMC
  // billAmounts 格式：[(accountId, amount), ...]
  // 需要知道每个任务的 requester 和 task_id：从子链存储读取
  // Phase 2 简化：只处理单任务场景（task_id=0, requester 从事件上下文获取）
  // 完整实现需要在 EpochFinalized 事件中携带 per-task 账单信息
  if (billAmounts.length === 0) {
    log("  本 Epoch 无账单，跳过 FMC 提交");
    return;
  }

  log(`  提交账单（${billAmounts.length} 个接收方）...`);
  // TODO Phase 2 完整实现：遍历任务列表，逐任务提交账单
  // 当前简化：打印账单内容，不实际提交（因为 requester/taskId 信息在事件中不完整）
  for (const [addr, amount] of billAmounts) {
    log(`    → ${addr}: ${amount.toHuman()}`);
  }
  log("  [注意] 完整账单提交需 EpochFinalized 事件携带 per-task 信息，已记录待后续实现");
}

async function main() {
  log(`启动桥接服务`);
  log(`子链 RPC: ${CHILD1_WS}`);
  log(`主链 RPC: ${MAIN_WS}`);

  const [childProvider, mainProvider] = [
    new WsProvider(CHILD1_WS),
    new WsProvider(MAIN_WS),
  ];

  const [childApi, mainApi] = await Promise.all([
    ApiPromise.create({ provider: childProvider }),
    ApiPromise.create({ provider: mainProvider }),
  ]);

  const keyring = new Keyring({ type: "sr25519" });
  const miner   = keyring.addFromUri(MINER_SURI);

  log(`矿工账户: ${miner.address}`);

  const childChain = await childApi.rpc.system.chain();
  const mainChain  = await mainApi.rpc.system.chain();
  log(`子链: ${childChain}  主链: ${mainChain}`);

  let processedCount = 0;

  // 订阅子链事件
  const unsub = await childApi.query.system.events(async (events) => {
    for (const { event } of events) {
      if (!childApi.events.crowdsource.EpochFinalized.is(event)) continue;

      const { chain_id, epoch, merkle_root, bill_amounts } = event.data;
      log(`\n收到 EpochFinalized 事件:`);
      log(`  chain_id   = ${chain_id}`);
      log(`  epoch      = ${epoch}`);
      log(`  merkleRoot = ${merkle_root}`);
      log(`  账单笔数   = ${bill_amounts.length}`);

      try {
        await submitToMainChain(
          mainApi, miner,
          chain_id, epoch, merkle_root,
          bill_amounts.map(([addr, amt]) => [addr.toString(), amt]),
        );
        processedCount++;
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
    log("等待第一个 EpochFinalized 事件...");
    // once 模式超时 5 分钟
    await new Promise((_, reject) =>
      setTimeout(() => reject(new Error("超时：未收到 EpochFinalized 事件")), 300_000)
    );
  } else {
    log("持续监听中（Ctrl+C 退出）...");
    process.on("SIGINT", async () => {
      log("收到 SIGINT，正在退出...");
      unsub();
      await Promise.all([childApi.disconnect(), mainApi.disconnect()]);
      process.exit(0);
    });
  }
}

main().catch(e => {
  console.error("[bridge 致命错误]", e.message);
  process.exit(1);
});
