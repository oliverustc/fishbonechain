/**
 * FishboneChain 数据交易桥接 — 观察器与可选协调器。
 *
 * 默认模式：监听 child6 的 tradeSession 事件，打印建议的主链动作。
 * --execute --dev-keys 模式：使用本地 dev key 实际提交主链交易。
 *
 * 权限：不绕过 DO/DR 签名权限。执行模式下必须用 --dev-keys。
 *
 * 用法：
 *   node scripts/bridges/data_trade.js --main ws://... --child ws://... [--once] [--execute --dev-keys]
 */

import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";
import { assertEscrowMatchesTradeTerms } from "../lib/data_trade_binding.js";

const CHILD_WS = process.env.CHILD_WS || "ws://127.0.0.1:9950";
const MAIN_WS = process.env.MAIN_WS || "ws://127.0.0.1:9944";
const ONCE = process.argv.includes("--once");
const EXECUTE = process.argv.includes("--execute");
const DEV_KEYS = process.argv.includes("--dev-keys");

// Parse optional --main and --child from CLI args
function parseArg(flag) {
  const idx = process.argv.indexOf(flag);
  return idx !== -1 ? process.argv[idx + 1] : null;
}

const MAIN_CLI = parseArg("--main");
const CHILD_CLI = parseArg("--child");

function log(msg) {
  console.log(`[data-trade-bridge ${new Date().toISOString()}] ${msg}`);
}

/**
 * Submit a transaction with signAndSend.
 */
async function submitTx(api, signer, tx, label) {
  return new Promise((resolve, reject) => {
    tx.signAndSend(signer, ({ status, dispatchError, events }) => {
      if (dispatchError) {
        if (dispatchError.isModule) {
          const decoded = api.registry.findMetaError(dispatchError.asModule);
          reject(new Error(`${label}: ${decoded.section}.${decoded.name}: ${decoded.docs.join(" ")}`));
        } else {
          reject(new Error(`${label}: ${dispatchError.toString()}`));
        }
      }
      if (status.isInBlock || status.isFinalized) {
        resolve({ status, events });
      }
    }).catch(reject);
  });
}

/**
 * Select a dev signer matching the actor in the event.
 * Alice = DR (requester), Bob = DO (data_owner).
 */
function devSignerForActor(keyring, actorAddress, knownActors) {
  if (!knownActors) throw new Error("no known actors configured");
  if (actorAddress === knownActors.dataOwner) return keyring.addFromUri("//Bob");
  if (actorAddress === knownActors.dataRequester) return keyring.addFromUri("//Alice");
  throw new Error(`no dev signer for actor ${actorAddress}`);
}

async function readSessionTerms(childApi, sessionId) {
  const maybeSession = await childApi.query.tradeSession.sessions(sessionId);
  if (!maybeSession.isSome) {
    throw new Error(`tradeSession.sessions(${sessionId}) is None`);
  }
  const session = maybeSession.unwrap();
  return {
    escrowId: session.escrowId.toNumber(),
    listingId: session.listingId.toNumber(),
    requester: session.requester.toString(),
    dataOwner: session.dataOwner.toString(),
    maxRounds: session.maxRounds.toNumber(),
    pricePerRound: session.pricePerRound.toString(),
    hashChainAnchor: session.hashChainAnchor.toString(),
  };
}

async function main() {
  if (EXECUTE && !DEV_KEYS) {
    console.error("错误：--execute 模式必须同时指定 --dev-keys。退出。");
    process.exit(1);
  }

  const mainWs = MAIN_CLI || MAIN_WS;
  const childWs = CHILD_CLI || CHILD_WS;

  log("启动数据交易桥接服务");
  log(`子链 RPC: ${childWs}  主链 RPC: ${mainWs}`);
  if (EXECUTE) log("🔧 执行模式：将实际提交主链交易（dev keys）");
  else log("📡 观察模式：仅打印建议动作");

  const [childApi, mainApi] = await Promise.all([
    ApiPromise.create({ provider: new WsProvider(childWs) }),
    ApiPromise.create({ provider: new WsProvider(mainWs) }),
  ]);

  const keyring = EXECUTE ? new Keyring({ type: "sr25519" }) : null;

  // Known actor addresses (Alice = DR = 5GrwvaEF..., Bob = DO = 5FHneW46...)
  const knownActors = {
    dataOwner: "5FHneW46xGXgs5mUiveUT4vTy8YfPvAJi11gV2d3jPyBHfQb",     // Bob
    dataRequester: "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY", // Alice
  };

  log(`子链: ${await childApi.rpc.system.chain()}  主链: ${await mainApi.rpc.system.chain()}`);

  let observedCount = 0;

  const unsub = await childApi.query.system.events(async (events) => {
    for (const { event } of events) {
      if (event.section !== "tradeSession") continue;

      observedCount++;
      const { method, data } = event;
      log(`观察到 ${event.section}.${method}`);

      try {
        switch (method) {
          case "SessionCreated": {
            log(`  session_id=${data.sessionId}, requester=${data.requester}, owner=${data.dataOwner}, escrow=${data.escrowId}`);
            break;
          }
          case "SettlementClaimed": {
            const actor = data.actor.toString();
            log(`  session_id=${data.sessionId}, actor=${actor}, remaining=${data.remainingRounds}`);
            if (EXECUTE) {
              // DO claims settlement -> mainEscrow.settleByPreimage signed by DO
              // Note: preimage and escrow_id are obtained from the session state
              // This is a best-effort automation; E2E script handles this directly
              log("  ⚠ 结算需 preimage，bridge 不持有。请用 E2E 脚本直接调用 settleByPreimage。");
            } else {
              log(`  💡 建议：DO 调用 mainEscrow.settleByPreimage(escrow_id, preimage, ${data.remainingRounds})`);
            }
            break;
          }
          case "SessionPunished": {
            log(`  session_id=${data.sessionId}`);
            if (EXECUTE) {
              const terms = await readSessionTerms(childApi, data.sessionId);
              const listing = await childApi.query.dataRegistry.listings(terms.listingId);
              if (!listing.isSome) {
                log(`  ❌ listing ${terms.listingId} not found, skipping`);
                break;
              }
              await assertEscrowMatchesTradeTerms(mainApi, terms.escrowId, {
                requester: terms.requester,
                dataOwner: terms.dataOwner,
                maxRounds: terms.maxRounds,
                pricePerRound: terms.pricePerRound,
                deposit: listing.unwrap().depositHint.toString(),
                hashChainAnchor: terms.hashChainAnchor,
              });
              const signer = devSignerForActor(keyring, knownActors.dataRequester, knownActors);
              await submitTx(
                mainApi, signer,
                mainApi.tx.mainEscrow.punishDataOwner(terms.escrowId),
                `mainEscrow.punishDataOwner(${terms.escrowId})`
              );
              log(`  ✅ 已提交 mainEscrow.punishDataOwner(${terms.escrowId})`);
            } else {
              log(`  💡 建议：DR 调用 mainEscrow.punishDataOwner(escrow_id)`);
            }
            break;
          }
          case "LastPaymentClaimed": {
            log(`  session_id=${data.sessionId}, actor=${data.actor}, round=${data.roundIndex}`);
            if (EXECUTE) {
              const terms = await readSessionTerms(childApi, data.sessionId);
              const listing = await childApi.query.dataRegistry.listings(terms.listingId);
              if (!listing.isSome) {
                log(`  ❌ listing ${terms.listingId} not found, skipping`);
                break;
              }
              await assertEscrowMatchesTradeTerms(mainApi, terms.escrowId, {
                requester: terms.requester,
                dataOwner: terms.dataOwner,
                maxRounds: terms.maxRounds,
                pricePerRound: terms.pricePerRound,
                deposit: listing.unwrap().depositHint.toString(),
                hashChainAnchor: terms.hashChainAnchor,
              });
              const signer = devSignerForActor(keyring, knownActors.dataOwner, knownActors);
              await submitTx(
                mainApi, signer,
                mainApi.tx.mainEscrow.claimLastPayment(terms.escrowId, data.roundIndex),
                `mainEscrow.claimLastPayment(${terms.escrowId}, ${data.roundIndex})`
              );
              log(`  ✅ 已提交 mainEscrow.claimLastPayment(${terms.escrowId}, ${data.roundIndex})`);
            } else {
              log(`  💡 建议：DO 调用 mainEscrow.claimLastPayment(escrow_id, ${data.roundIndex})`);
            }
            break;
          }
        }
      } catch (err) {
        log(`  ❌ 处理事件失败: ${err.message}`);
      }

      if (ONCE && observedCount > 0) {
        unsub();
        await Promise.all([childApi.disconnect(), mainApi.disconnect()]);
        log(`处理完成，退出 (观察到 ${observedCount} 个事件)`);
        process.exit(0);
      }
    }
  });

  if (ONCE) {
    // Wait at most 5 minutes for an event
    setTimeout(() => {
      log("超时：5 分钟内未收到数据交易事件");
      unsub();
      Promise.all([childApi.disconnect(), mainApi.disconnect()]).then(() => process.exit(1));
    }, 300_000);
  } else {
    log("持续监听中（Ctrl+C 退出）...");
    process.on("SIGINT", async () => {
      log(`已观察 ${observedCount} 个数据交易事件，退出...`);
      unsub();
      await Promise.all([childApi.disconnect(), mainApi.disconnect()]);
      process.exit(0);
    });
    await new Promise(() => {});
  }
}

main().catch((e) => {
  console.error("[data-trade-bridge 致命错误]", e.message);
  process.exit(1);
});
