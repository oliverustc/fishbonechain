/**
 * Reinitialize child6 after its local chain database is reset.
 *
 * This script intentionally touches only the child6 chain:
 *   1. sync task 5 into crowdsource.ActiveTasks
 *   2. fund //Worker0..//Worker499 with 10 UNIT each
 *
 * It must not re-run main-chain CCMC/FMC registration, because those records
 * already exist and re-running them would allocate duplicate ids.
 */

import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";

const CHILD6_WS = process.env.CHILD6_WS || "ws://10.2.2.11:9950";
const TASK_ID = Number(process.env.CHILD6_TASK_ID || "5");
const WORKERS = Number(process.env.CHILD6_WORKERS || "500");
const CHUNK = Number(process.env.CHILD6_FUND_CHUNK || "100");
const UNIT = 1_000_000_000_000n;
const WORKER_AMOUNT = BigInt(process.env.CHILD6_WORKER_UNIT || "10") * UNIT;
const BUDGET_PER_EPOCH = BigInt(process.env.CHILD6_BUDGET_UNIT || "25000") * UNIT;
const DESCRIPTION = process.env.CHILD6_DESCRIPTION || "Data market (AURA-5, 6s)";

function log(message) {
  console.log(`[setup_child6 ${new Date().toISOString()}] ${message}`);
}

async function sendTx(api, tx, signer, label, options = {}) {
  return new Promise((resolve, reject) => {
    tx.signAndSend(signer, options, ({ status, dispatchError }) => {
      if (dispatchError) {
        reject(new Error(`${label} failed: ${dispatchError.toString()}`));
      } else if (status.isInBlock) {
        log(`  ok: ${label}`);
        resolve();
      }
    }).catch(reject);
  });
}

async function batchTransfer(api, from, recipients, amount) {
  const startNonce = (await api.rpc.system.accountNextIndex(from.address)).toNumber();
  log(`fund workers=${recipients.length}, amount=${amount / UNIT} UNIT, start_nonce=${startNonce}`);

  for (let i = 0; i < recipients.length; i += CHUNK) {
    const chunk = recipients.slice(i, i + CHUNK);
    await Promise.all(chunk.map((dest, j) =>
      sendTx(
        api,
        api.tx.balances.transferKeepAlive(dest, amount),
        from,
        `transfer worker ${i + j}`,
        { nonce: startNonce + i + j },
      ).catch(e => {
        throw new Error(`transfer to ${dest} failed: ${e.message}`);
      })
    ));
    log(`  funded ${Math.min(i + CHUNK, recipients.length)}/${recipients.length}`);
  }
}

async function readState(api, alice, worker0) {
  const [task, worker0Account, epoch, submissions] = await Promise.all([
    api.query.crowdsource.activeTasks(TASK_ID),
    api.query.system.account(worker0.address),
    api.query.crowdsource.currentEpoch(),
    api.query.crowdsource.epochSubmissions(),
  ]);

  return {
    task: task.toHuman(),
    worker0Free: worker0Account.data.free.toHuman(),
    epoch: epoch.toHuman(),
    submissions: submissions.length,
    alice: alice.address,
    worker0: worker0.address,
  };
}

async function main() {
  log(`connect ${CHILD6_WS}`);
  const api = await ApiPromise.create({ provider: new WsProvider(CHILD6_WS) });
  const keyring = new Keyring({ type: "sr25519" });
  const alice = keyring.addFromUri("//Alice");
  const workers = Array.from({ length: WORKERS }, (_, i) => keyring.addFromUri(`//Worker${i}`));

  const chain = await api.rpc.system.chain();
  const header = await api.rpc.chain.getHeader();
  log(`chain=${chain.toString()} block=${header.number.toString()} alice=${alice.address}`);

  const descBytes = Array.from(new TextEncoder().encode(DESCRIPTION));
  await sendTx(
    api,
    api.tx.crowdsource.syncTask(TASK_ID, alice.address, BUDGET_PER_EPOCH, descBytes),
    alice,
    `crowdsource.syncTask(task=${TASK_ID})`,
  );

  await batchTransfer(api, alice, workers.map(w => w.address), WORKER_AMOUNT);

  const state = await readState(api, alice, workers[0]);
  log(`verify=${JSON.stringify(state, null, 2)}`);
  await api.disconnect();
  log("done");
}

main().catch(e => {
  console.error("[setup_child6 fatal]", e.message);
  console.error(e.stack);
  process.exit(1);
});
