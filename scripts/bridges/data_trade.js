/**
 * FishboneChain 数据交易场景桥接骨架。
 *
 * 初版只观察 dataRegistry/tradeSession 事件，不提交 FMC bill。
 */

import { ApiPromise, WsProvider } from "@polkadot/api";

const CHILD_WS = process.env.CHILD_WS || "ws://127.0.0.1:9950";
const MAIN_WS = process.env.MAIN_WS || "ws://127.0.0.1:9944";
const ONCE = process.argv.includes("--once");

function log(msg) {
  console.log(`[data-trade-bridge ${new Date().toISOString()}] ${msg}`);
}

async function main() {
  log("启动数据交易桥接观察服务");
  log(`子链 RPC: ${CHILD_WS}`);
  log(`主链 RPC: ${MAIN_WS}`);

  const [childApi, mainApi] = await Promise.all([
    ApiPromise.create({ provider: new WsProvider(CHILD_WS) }),
    ApiPromise.create({ provider: new WsProvider(MAIN_WS) }),
  ]);

  log(`子链: ${await childApi.rpc.system.chain()}  主链: ${await mainApi.rpc.system.chain()}`);

  let observedCount = 0;
  const unsub = await childApi.query.system.events(async (events) => {
    for (const { event } of events) {
      if (event.section !== "dataRegistry" && event.section !== "tradeSession") continue;

      observedCount++;
      log(`观察到 ${event.section}.${event.method}`);

      if (ONCE) {
        unsub();
        await Promise.all([childApi.disconnect(), mainApi.disconnect()]);
        process.exit(0);
      }
    }
  });

  if (ONCE) {
    await new Promise((_, reject) => {
      setTimeout(() => reject(new Error("超时：5 分钟内未收到数据交易事件")), 300_000);
    });
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
