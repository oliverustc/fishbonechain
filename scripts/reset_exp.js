/**
 * 重置流动性实验：终止 task 0/3/5 → 重新激活（epoch 从 0 重置）
 *
 * 用法（在 10.2.2.11 scripts/ 目录下执行）：
 *   node reset_exp.js
 *
 * 注意：会先 pkill 所有 bridge/worker/metrics 进程。
 */

import { ApiPromise, WsProvider, Keyring } from "/home/debian/fishbone/scripts/node_modules/@polkadot/api/index.js";

const MAIN_WS  = process.env.MAIN_WS || "ws://127.0.0.1:9944";
const TASK_IDS = [0, 3, 5];

function log(msg) { console.log(`[reset_exp] ${msg}`); }

async function sendTx(api, tx, signer, label) {
  return new Promise((resolve, reject) => {
    tx.signAndSend(signer, ({ status, dispatchError }) => {
      if (dispatchError) {
        if (dispatchError.isModule) {
          const { name } = api.registry.findMetaError(dispatchError.asModule);
          log(`  [skip] ${label}: ${name}`);
          resolve(null);
          return;
        }
        reject(new Error(`${label} failed: ${dispatchError}`));
      } else if (status.isInBlock) {
        log(`  ✓ ${label}`);
        resolve(status.asInBlock);
      }
    }).catch(reject);
  });
}

async function main() {
  log(`连接主链 ${MAIN_WS} ...`);
  const api = await ApiPromise.create({ provider: new WsProvider(MAIN_WS) });

  const keyring = new Keyring({ type: "sr25519" });
  const alice   = keyring.addFromUri("//Alice");
  log(`Alice = ${alice.address}`);

  // 1. 查询各任务当前状态
  log("\n── Step 1：查询任务当前状态 ──");
  for (const id of TASK_IDS) {
    const t = await api.query.fmc.tasks(alice.address, id);
    if (t.isSome) {
      const d = t.unwrap();
      log(`  task${id}: status=${d.status} currentEpoch=${d.currentEpoch ?? d.current_epoch}`);
    } else {
      log(`  task${id}: 不存在`);
    }
  }

  // 2. 终止所有任务（释放 budget 到 free）
  log("\n── Step 2：terminateTask 0/3/5 ──");
  for (const id of TASK_IDS) {
    await sendTx(api, api.tx.fmc.terminateTask(id), alice, `fmc.terminateTask(${id})`);
    await new Promise(r => setTimeout(r, 1000));
  }

  // 3. 等待链确认，查询 free 余额
  await new Promise(r => setTimeout(r, 3000));
  const poolRaw = await api.query.fmc.fundPools(alice.address);
  const pool    = poolRaw.isSome ? poolRaw.unwrap() : poolRaw.toJSON();
  const UNIT    = 1_000_000_000_000n;
  const freeU   = BigInt((pool.free ?? pool.toJSON?.()?.free ?? 0).toString()) / UNIT;
  const lockedU = BigInt((pool.locked ?? pool.toJSON?.()?.locked ?? 0).toString()) / UNIT;
  log(`\n  terminate 后 pool: free=${freeU} UNIT  locked=${lockedU} UNIT`);

  // 4. 重新激活（epoch 从 0 开始）
  log("\n── Step 3：activateTask 0/3/5 ──");
  for (const id of TASK_IDS) {
    await sendTx(api, api.tx.fmc.activateTask(id), alice, `fmc.activateTask(${id})`);
    await new Promise(r => setTimeout(r, 1000));
  }

  // 5. 最终状态确认
  await new Promise(r => setTimeout(r, 3000));
  const poolAfter = await api.query.fmc.fundPools(alice.address);
  const pa   = poolAfter.isSome ? poolAfter.unwrap() : poolAfter.toJSON();
  const freeA   = BigInt((pa.free ?? pa.toJSON?.()?.free ?? 0).toString()) / UNIT;
  const lockedA = BigInt((pa.locked ?? pa.toJSON?.()?.locked ?? 0).toString()) / UNIT;
  log(`\n  activate 后 pool: free=${freeA} UNIT  locked=${lockedA} UNIT`);

  for (const id of TASK_IDS) {
    const t = await api.query.fmc.tasks(alice.address, id);
    if (t.isSome) {
      const d = t.unwrap();
      log(`  task${id}: status=${d.status} currentEpoch=${d.currentEpoch ?? d.current_epoch}`);
    }
  }

  log("\n重置完成，可以运行 run_exp_fund.sh 了。");
  await api.disconnect();
  process.exit(0);
}

main().catch(e => {
  console.error("[reset_exp] 错误:", e.message);
  process.exit(1);
});
