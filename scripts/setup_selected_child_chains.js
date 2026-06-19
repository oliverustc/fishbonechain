/**
 * Reinitialize selected child chains after their local databases are reset.
 *
 * This script touches child chains only:
 *   - crowdsource.syncTask(task_id)
 *   - fund //Worker{i}
 *
 * Usage:
 *   node setup_selected_child_chains.js --chains child4,child1,child6
 */

import { readFileSync } from "fs";
import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";

const DEFAULT_PROFILE_FILE = new URL("./profiles/chains.json", import.meta.url);

const UNIT = 1_000_000_000_000n;

const CHAINS = {
  child1: {
    ws: process.env.CHILD1_WS || "ws://10.2.2.11:9945",
    taskId: 0,
    budgetPerEpoch: 1500n * UNIT,
    description: "City delivery crowdsource (AURA-3, 6s)",
    workers: 300,
  },
  child2: {
    ws: process.env.CHILD2_WS || "ws://10.2.2.14:9946",
    taskId: 1,
    budgetPerEpoch: 2n * UNIT,
    description: "Traffic sensing (AURA-3, same-config capacity run)",
    workers: 2000,
  },
  child3: {
    ws: process.env.CHILD3_WS || "ws://10.2.2.17:9947",
    taskId: 2,
    budgetPerEpoch: 40000n * UNIT,
    description: "Medical annotation (AURA-3, same-config capacity run)",
    workers: 200,
  },
  child4: {
    ws: process.env.CHILD4_WS || "ws://10.2.2.11:9948",
    taskId: 3,
    budgetPerEpoch: 5000n * UNIT,
    description: "Financial verification (AURA-7, 60min Epoch)",
    workers: 100,
  },
  child5: {
    ws: process.env.CHILD5_WS || "ws://10.2.2.20:9949",
    taskId: 4,
    budgetPerEpoch: UNIT / 2n,
    description: "IoT sensor network (AURA-3, same-config capacity run)",
    workers: 5000,
  },
  child6: {
    ws: process.env.CHILD6_WS || "ws://10.2.2.11:9950",
    taskId: 5,
    budgetPerEpoch: 25000n * UNIT,
    description: "Data market (AURA-5, 60min Epoch)",
    workers: 500,
  },
};

function parseArgs() {
  const args = process.argv.slice(2);
  const get = (flag, def) => {
    const i = args.indexOf(flag);
    return i !== -1 ? args[i + 1] : def;
  };
  const chains = get("--chains", "")
    .split(",")
    .map(s => s.trim())
    .filter(Boolean);
  if (chains.length === 0) {
    throw new Error("--chains is required, e.g. --chains child4,child1");
  }
  for (const chain of chains) {
    if (!CHAINS[chain]) {
      throw new Error(`unknown chain: ${chain}`);
    }
  }
  return {
    chains,
    profileFile: get("--profile-file", ""),
    chunk: Number(get("--chunk", "200")),
    maxWorkers: Number(get("--max-workers", "0")),
    workerUnit: BigInt(get("--worker-unit", "10")) * UNIT,
  };
}

function loadProfiles(profileFile) {
  const path = profileFile ? new URL(profileFile, `file://${process.cwd()}/`) : DEFAULT_PROFILE_FILE;
  const profiles = JSON.parse(readFileSync(path, "utf8"));
  return profiles.chains ?? profiles;
}

function log(message) {
  console.log(`[setup_selected ${new Date().toISOString()}] ${message}`);
}

async function sendTx(api, tx, signer, label, options = {}) {
  return new Promise((resolve, reject) => {
    tx.signAndSend(signer, options, ({ status, dispatchError }) => {
      if (dispatchError) {
        reject(new Error(`${label} failed: ${dispatchError.toString()}`));
      } else if (status.isInBlock) {
        resolve();
      }
    }).catch(reject);
  });
}

async function fundWorkers(api, alice, keyring, count, amount, chunkSize, chain) {
  const recipients = Array.from({ length: count }, (_, i) => keyring.addFromUri(`//Worker${i}`).address);
  const startNonce = (await api.rpc.system.accountNextIndex(alice.address)).toNumber();
  log(`${chain}: fund ${count} workers, start_nonce=${startNonce}`);

  for (let i = 0; i < recipients.length; i += chunkSize) {
    const chunk = recipients.slice(i, i + chunkSize);
    await Promise.all(chunk.map((dest, j) =>
      sendTx(
        api,
        api.tx.balances.transferKeepAlive(dest, amount),
        alice,
        `${chain}: transfer worker ${i + j}`,
        { nonce: startNonce + i + j },
      )
    ));
    log(`${chain}: funded ${Math.min(i + chunkSize, recipients.length)}/${recipients.length}`);
  }
}

async function setupChain(chain, cfg, profile, keyring, alice, options) {
  log(`${chain}: connect ${cfg.ws}`);
  const api = await ApiPromise.create({ provider: new WsProvider(cfg.ws) });
  const [chainName, header] = await Promise.all([
    api.rpc.system.chain(),
    api.rpc.chain.getHeader(),
  ]);
  log(`${chain}: chain=${chainName.toString()} block=${header.number.toString()}`);

  if (!profile) {
    await api.disconnect();
    throw new Error(`missing chain profile: ${chain}`);
  }
  if (profile.scene !== "Crowdsource") {
    log(`${chain}: skip crowdsource setup for scene=${profile.scene}`);
    await api.disconnect();
    return;
  }

  const descBytes = Array.from(new TextEncoder().encode(cfg.description));
  const syncNonce = (await api.rpc.system.accountNextIndex(alice.address)).toNumber();
  log(`${chain}: sync task ${cfg.taskId}, nonce=${syncNonce}`);
  await sendTx(
    api,
    api.tx.crowdsource.syncTask(cfg.taskId, alice.address, cfg.budgetPerEpoch, descBytes),
    alice,
    `${chain}: syncTask(${cfg.taskId})`,
    { nonce: syncNonce },
  );
  log(`${chain}: synced task ${cfg.taskId}`);

  const workerCount = options.maxWorkers > 0 ? Math.min(cfg.workers, options.maxWorkers) : cfg.workers;
  await fundWorkers(api, alice, keyring, workerCount, options.workerUnit, options.chunk, chain);

  const [task, worker0, epoch, subs] = await Promise.all([
    api.query.crowdsource.activeTasks(cfg.taskId),
    api.query.system.account(keyring.addFromUri("//Worker0").address),
    api.query.crowdsource.currentEpoch(),
    api.query.crowdsource.epochSubmissions(),
  ]);
  log(`${chain}: verify task=${task.isSome ? "some" : "none"} worker0=${worker0.data.free.toString()} epoch=${JSON.stringify(epoch.toHuman())} submissions=${subs.length}`);
  await api.disconnect();
}

async function main() {
  const options = parseArgs();
  const chainProfiles = loadProfiles(options.profileFile);
  const keyring = new Keyring({ type: "sr25519" });
  const alice = keyring.addFromUri("//Alice");
  for (const chain of options.chains) {
    await setupChain(chain, CHAINS[chain], chainProfiles[chain], keyring, alice, options);
  }
  log("done");
}

main().catch(e => {
  console.error("[setup_selected fatal]", e.message);
  console.error(e.stack);
  process.exit(1);
});
