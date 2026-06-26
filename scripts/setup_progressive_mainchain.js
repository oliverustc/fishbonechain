#!/usr/bin/env node

import { readFileSync } from "fs";
import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";

const UNIT = 1_000_000_000_000n;
const MAIN_WS = process.env.MAIN_WS || "ws://10.2.2.11:9944";

const VALIDATOR_SURIS = {
  f1: "0x52390bf081065e3ff296ab72c42bc234cedbdf9ddc40b6c7b6aee5fd01e08880",
  f2: "0x17f21fff17006faad4fa003c3db215718fc4d3bbc86054a435666affe704e71b",
  f3: "0x4151713ff93e1333474f1380cb2bc4fce9183942790830c1e8f48d3752e232fc",
  f4: "0x8b7aeb4590e1607db466c3cea45b4096f0b912364da41689f1be5166df3fee83",
  f5: "0x69416e5975a353736603d14d57e13efdadab8dd11667498d7892588abf50e70a",
  f6: "0x91bd2803edfcbb7e8d7f06c7df94f98d26300e56b968ee51855d994e4308d1e8",
  f7: "0x92ed7c0c05a5b080b5193514043e2bbd33401e2428e50e85cfbd2a20558b5652",
  f8: "0xb9b4d65352af6ab4f5c7bf0b765d17053cb5e1c39868a8ff7b5600340f114d56",
  f9: "0xdd20f92b0d61c5dd4ba76aac2c7d2e9957746adc5dd45d4dc3339f42f7ae1c4b",
  f10: "0xcb829a57912d649c46808a673d2f466b9f954208ab15ddd748567af6bbf81082",
  f11: "0x6b006d6f22d84f120c61d9f4366bc0d2390472aad7b0345c17a51ccf3a1538d4",
  f12: "0xf20ecd8e0f4aabc67e991ca6be62522b37cdf256819498d84bafea34d5146817",
  f13: "//Alice//f13",
  f14: "//Alice//f14",
  f15: "//Alice//f15",
  f16: "//Alice//f16",
  f17: "//Alice//f17",
  f18: "//Alice//f18",
};

function parseArgs() {
  const args = process.argv.slice(2);
  const get = (flag, def = "") => {
    const i = args.indexOf(flag);
    return i === -1 ? def : args[i + 1];
  };
  return {
    profileFile: get("--profile-file"),
    chains: get("--chains").split(",").map((s) => s.trim()).filter(Boolean),
    dryRun: args.includes("--dry-run"),
    fundValidatorsUnit: BigInt(get("--fund-validators-unit", "10")) * UNIT,
  };
}

function loadProfiles(path) {
  const raw = JSON.parse(readFileSync(path, "utf8"));
  return raw.chains || raw;
}

function selectedCrowdsourceProfiles(profiles, chains) {
  return chains
    .map((name) => [name, profiles[name]])
    .filter(([, profile]) => profile && profile.scene === "Crowdsource" && profile.settlement === "FmcTaskBill");
}

function budgetPlanck(profile) {
  return BigInt(profile.budgetPerEpochUnit) * UNIT;
}

function describePlan(name, profile) {
  const chainId = profile.chainId;
  const taskId = profile.taskId;
  const budget = profile.budgetPerEpochUnit;
  console.log(`registerChildChain chain_id=${chainId} name=${name}`);
  for (const validator of profile.validators || []) {
    console.log(`joinChildChain chain_id=${chainId} validator=${validator}`);
  }
  console.log(`createTask task_id=${taskId} chain_id=${chainId} budget_unit=${budget}`);
  console.log(`activateTask task_id=${taskId}`);
}

async function sendTx(api, tx, signer, label) {
  return new Promise((resolve, reject) => {
    tx.signAndSend(signer, ({ status, dispatchError }) => {
      if (dispatchError) {
        reject(new Error(`${label} failed: ${dispatchError.toString()}`));
      } else if (status.isInBlock) {
        console.log(`ok ${label}`);
        resolve(status.asInBlock.toString());
      }
    }).catch(reject);
  });
}

async function ensureFreeBalance(api, alice, account, minimum, label) {
  if (minimum <= 0n) return;
  const info = await api.query.system.account(account.address);
  const free = BigInt(info.data.free.toString());
  if (free >= minimum) {
    console.log(`skip fund ${label} free=${free}`);
    return;
  }
  await sendTx(
    api,
    api.tx.balances.transferKeepAlive(account.address, minimum - free),
    alice,
    `fund ${label}`,
  );
}

async function ensureFmcFreeBudget(api, alice, selectedProfiles) {
  const required = selectedProfiles.reduce((sum, [, profile]) => sum + budgetPlanck(profile), 0n);
  const pool = await api.query.fmc.fundPools(alice.address);
  const free = pool.isSome ? BigInt(pool.unwrap().free.toString()) : 0n;
  if (free >= required) {
    console.log(`skip fmc.deposit free=${free}`);
    return;
  }
  await sendTx(api, api.tx.fmc.deposit(required - free), alice, `fmc.deposit(${required - free})`);
}

async function ensureChildChain(api, alice, name, profile) {
  const chainId = profile.chainId;
  const existing = await api.query.ccmc.childChains(chainId);
  if (existing.isSome) {
    console.log(`skip registerChildChain chain_id=${chainId}`);
    return;
  }
  const nextChainId = (await api.query.ccmc.nextChainId()).toNumber();
  if (nextChainId !== chainId) {
    throw new Error(`cannot register ${name}: nextChainId=${nextChainId}, expected chainId=${chainId}`);
  }
  const nameBytes = Array.from(new TextEncoder().encode(name));
  await sendTx(
    api,
    api.tx.ccmc.registerChildChain(nameBytes, 1, 0),
    alice,
    `registerChildChain(${chainId})`,
  );
}

async function ensureMiner(api, keyring, alice, chainId, validator, fundAmount) {
  const suri = VALIDATOR_SURIS[validator];
  if (!suri) throw new Error(`missing validator SURI for ${validator}`);
  const signer = keyring.addFromUri(suri);
  await ensureFreeBalance(api, alice, signer, fundAmount, validator);

  const joined = await api.query.ccmc.miners(chainId, signer.address);
  if (joined.isSome) {
    console.log(`skip joinChildChain chain_id=${chainId} validator=${validator}`);
    return;
  }
  await sendTx(api, api.tx.ccmc.joinChildChain(chainId), signer, `joinChildChain(${chainId}) by ${validator}`);
}

async function ensureFmcTask(api, alice, profile, name) {
  const task = await api.query.fmc.tasks(alice.address, profile.taskId);
  if (task.isNone) {
    const descBytes = Array.from(new TextEncoder().encode(profile.description || name));
    await sendTx(
      api,
      api.tx.fmc.createTask(profile.chainId, budgetPlanck(profile), descBytes),
      alice,
      `createTask(${profile.taskId})`,
    );
  } else {
    console.log(`skip createTask task_id=${profile.taskId}`);
  }

  const refreshed = await api.query.fmc.tasks(alice.address, profile.taskId);
  if (refreshed.isNone) {
    throw new Error(`task ${profile.taskId} missing after createTask`);
  }
  const status = refreshed.unwrap().status.toString();
  if (status === "Activated") {
    console.log(`skip activateTask task_id=${profile.taskId}`);
    return;
  }
  await sendTx(api, api.tx.fmc.activateTask(profile.taskId), alice, `activateTask(${profile.taskId})`);
}

async function setupMainchain(selected, cfg) {
  const api = await ApiPromise.create({ provider: new WsProvider(MAIN_WS) });
  const keyring = new Keyring({ type: "sr25519" });
  const alice = keyring.addFromUri("//Alice");
  try {
    await ensureFmcFreeBudget(api, alice, selected);
    for (const [name, profile] of selected) {
      await ensureChildChain(api, alice, name, profile);
      for (const validator of profile.validators || []) {
        await ensureMiner(api, keyring, alice, profile.chainId, validator, cfg.fundValidatorsUnit);
      }
      await ensureFmcTask(api, alice, profile, name);
    }
  } finally {
    await api.disconnect();
  }
}

async function main() {
  const cfg = parseArgs();
  if (!cfg.profileFile) throw new Error("--profile-file is required");
  if (cfg.chains.length === 0) throw new Error("--chains is required");

  const profiles = loadProfiles(cfg.profileFile);
  const selected = selectedCrowdsourceProfiles(profiles, cfg.chains);
  if (cfg.dryRun) {
    for (const [name, profile] of selected) {
      describePlan(name, profile);
    }
    return;
  }

  await setupMainchain(selected, cfg);
}

main().catch((e) => {
  console.error(`[setup_progressive_mainchain fatal] ${e.message}`);
  process.exit(1);
});
