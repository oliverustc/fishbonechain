#!/usr/bin/env node

import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";
import { writeFileSync } from "fs";

function parseArgs() {
  const args = process.argv.slice(2);
  const get = (flag, def = "") => {
    const i = args.indexOf(flag);
    return i === -1 ? def : args[i + 1];
  };
  return {
    chains: get("--chains").split(",").map((s) => s.trim()).filter(Boolean),
    out: get("--out", "/tmp/progressive_epoch_finalize.json"),
    waitSyncing: Number(get("--wait-syncing-seconds", "900")),
    pollMs: Number(get("--poll-ms", "3000")),
  };
}

function phaseName(epoch) {
  const human = epoch.toHuman();
  return human.phase || human.Phase || JSON.stringify(human);
}

function epochIdValue(epoch) {
  const human = epoch.toHuman();
  return Number(epoch.epochId?.toString?.() ?? epoch.epoch_id?.toString?.() ?? human.epochId ?? human.epoch_id ?? human.EpochId ?? 0);
}

async function sendTx(api, tx, signer, label) {
  return new Promise((resolve, reject) => {
    tx.signAndSend(signer, async ({ status, dispatchError }) => {
      if (dispatchError) {
        reject(new Error(`${label} failed: ${dispatchError.toString()}`));
      } else if (status.isInBlock) {
        resolve(status.asInBlock.toString());
      }
    }).catch(reject);
  });
}

async function waitForSyncing(api, timeoutSeconds, pollMs) {
  const deadline = Date.now() + timeoutSeconds * 1000;
  while (Date.now() < deadline) {
    const [epoch, header] = await Promise.all([
      api.query.crowdsource.currentEpoch(),
      api.rpc.chain.getHeader(),
    ]);
    const phase = phaseName(epoch);
    console.log(`[finalize_epochs] block=${header.number.toString()} phase=${phase} epoch=${JSON.stringify(epoch.toHuman())}`);
    if (phase === "Syncing") return epoch;
    await new Promise((resolve) => setTimeout(resolve, pollMs));
  }
  throw new Error(`timeout waiting for Syncing after ${timeoutSeconds}s`);
}

async function blockHasEpochFinalized(api, blockHash) {
  const events = await api.query.system.events.at(blockHash);
  return events.some(({ event }) => event.section === "crowdsource" && event.method === "EpochFinalized");
}

async function finalizeOne(ws, keyring, cfg) {
  console.log(`[finalize_epochs] connect ${ws}`);
  const api = await ApiPromise.create({ provider: new WsProvider(ws) });
  const alice = keyring.addFromUri("//Alice");
  try {
    const before = await waitForSyncing(api, cfg.waitSyncing, cfg.pollMs);
    let blockHash = "";
    let sawFinalized = false;
    let finalizeError = "";
    try {
      blockHash = await sendTx(
        api,
        api.tx.crowdsource.finalizeEpoch(),
        alice,
        "crowdsource.finalizeEpoch",
      );
      sawFinalized = await blockHasEpochFinalized(api, blockHash);
    } catch (e) {
      finalizeError = e.message;
    }
    const after = await api.query.crowdsource.currentEpoch();
    const alreadyFinalized = epochIdValue(after) > epochIdValue(before);
    if (finalizeError && !alreadyFinalized) {
      throw new Error(finalizeError);
    }
    return {
      ws,
      before: before.toHuman(),
      after: after.toHuman(),
      blockHash,
      sawFinalized,
      alreadyFinalized,
      finalizeError,
    };
  } finally {
    await api.disconnect();
  }
}

async function main() {
  const cfg = parseArgs();
  if (cfg.chains.length === 0) throw new Error("--chains is required");
  const keyring = new Keyring({ type: "sr25519" });
  const results = [];
  for (const ws of cfg.chains) {
    results.push(await finalizeOne(ws, keyring, cfg));
  }
  writeFileSync(cfg.out, `${JSON.stringify({ results }, null, 2)}\n`);
}

main().catch((e) => {
  console.error(`[finalize_progressive_epochs fatal] ${e.message}`);
  process.exit(1);
});
