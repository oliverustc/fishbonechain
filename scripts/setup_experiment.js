/**
 * FishboneChain 实验初始化脚本
 *
 * 执行顺序：
 *   1. 主链：给 f1-f12 验证人账户充值（用于支付手续费 + CCMC 押金）
 *   2. 主链：Alice 向 FMC 充值（实验预算）
 *   3. 主链：注册 6 条子链到 CCMC（得到 chain_id 0-5）
 *   4. 主链：矿工账户 join 对应子链（bridge 投票账户必须在此注册）
 *   5. 主链：Alice 创建并激活 6 个任务
 *   6. 各子链：sync_task（Alice 调用，无权限限制）
 *   7. 各子链：给工作者账户充值（//Worker{i}）
 *
 * 用法：
 *   node scripts/setup_experiment.js
 *   node scripts/setup_experiment.js --dry-run   # 只打印计划不执行
 *   node scripts/setup_experiment.js --step 3    # 只执行第 3 步
 *
 * bridge.js 账单投票说明：
 *   fmc.submit_bill 要求调用方是已注册矿工（Miners 存储），且达到 2/3 阈值才结算。
 *
 *   单机测试模式（推荐）：
 *     CHAINS 中 miners 只填 ["alice"]，bridge 用 MINER_SURI="//Alice"。
 *     miner_count=1 → threshold=1 → 单次投票即可结算。
 *
 *   多矿工模式（生产仿真）：
 *     CHAINS 中 miners 填所有参与投票的 validator id（如 f1-f7）。
 *     bridge 用 MINER_SURIS="seed1,seed2,...,seed5"（至少 2/3 数量）。
 *     本脚本默认使用多矿工模式；如需切换到单机测试模式，
 *     将下方 CHAINS 每项的 miners 改为 ["alice"]。
 */

import { readFileSync } from "fs";
import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";

const CHAIN_PROFILES = JSON.parse(
  readFileSync(new URL("./profiles/chains.json", import.meta.url), "utf8")
);

const DRY_RUN   = process.argv.includes("--dry-run");
const ONLY_STEP = process.argv.includes("--step")
  ? parseInt(process.argv[process.argv.indexOf("--step") + 1], 10)
  : null;

const MAIN_WS = process.env.MAIN_WS || "ws://10.2.2.11:9944";

const CHILD_WS = {
  0: process.env.CHILD1_WS || "ws://10.2.2.11:9945",
  1: process.env.CHILD2_WS || "ws://10.2.2.14:9946",
  2: process.env.CHILD3_WS || "ws://10.2.2.17:9947",
  3: process.env.CHILD4_WS || "ws://10.2.2.11:9948",
  4: process.env.CHILD5_WS || "ws://10.2.2.20:9949",
  5: process.env.CHILD6_WS || "ws://10.2.2.11:9950",
};

const UNIT = 1_000_000_000_000n; // 1 UNIT = 10^12 planck

// ── 验证人账户信息（AURA_SEED 作为签名密钥）────────────────────────────────
const VALIDATORS = {
  f1:  { seed: "0x52390bf081065e3ff296ab72c42bc234cedbdf9ddc40b6c7b6aee5fd01e08880",
          ss58: "5HBbnWPSqVtGAu4vQJCJTxh25QTy9N2UvqxyfgFCuBXNxXAF" },
  f2:  { seed: "0x17f21fff17006faad4fa003c3db215718fc4d3bbc86054a435666affe704e71b",
          ss58: "5GHBUPMGJGHWcKbKEYH3GbkKr3BxHagJYhjbYMgSwGX47aX1" },
  f3:  { seed: "0x4151713ff93e1333474f1380cb2bc4fce9183942790830c1e8f48d3752e232fc",
          ss58: "5CmEkuB6gH1mTUeWRaeCVykEjotb9NWkE5wT2iHR2FSKohoX" },
  f4:  { seed: "0x8b7aeb4590e1607db466c3cea45b4096f0b912364da41689f1be5166df3fee83",
          ss58: "5CcWRmCnVGibzBootgavjefaNETwMeMNf94J4Yn9h4HwS6bp" },
  f5:  { seed: "0x69416e5975a353736603d14d57e13efdadab8dd11667498d7892588abf50e70a",
          ss58: "5HCFV9HbiSdwY6Pr1VNM2wkRi7xqJhFehLTmXeDmo73icGrj" },
  f6:  { seed: "0x91bd2803edfcbb7e8d7f06c7df94f98d26300e56b968ee51855d994e4308d1e8",
          ss58: "5Fe84wzHg6LXnBBqynCKLSWrDvb2dGs65tm53QjdBRbD4s1X" },
  f7:  { seed: "0x92ed7c0c05a5b080b5193514043e2bbd33401e2428e50e85cfbd2a20558b5652",
          ss58: "5EEeeJX26yaE8aZPEjiRdj9MAKcv7nx8Ho5eHz8M6DyZqN3o" },
  f8:  { seed: "0xb9b4d65352af6ab4f5c7bf0b765d17053cb5e1c39868a8ff7b5600340f114d56",
          ss58: "5HQJRwPdsFZqsEiMERGoRT6eGvtLkBSsmqNuai6X1auTksT8" },
  f9:  { seed: "0xdd20f92b0d61c5dd4ba76aac2c7d2e9957746adc5dd45d4dc3339f42f7ae1c4b",
          ss58: "5HK6Gc1p4kEaqP2jLAkRDHLTRrm7ekbt51a1NWkvoCebEzEt" },
  f10: { seed: "0xcb829a57912d649c46808a673d2f466b9f954208ab15ddd748567af6bbf81082",
          ss58: "5Hdvywv8w5TdvrAcjMWoPWHWo93TXPtmDi6SBBmXw771jPZg" },
  f11: { seed: "0x6b006d6f22d84f120c61d9f4366bc0d2390472aad7b0345c17a51ccf3a1538d4",
          ss58: "5Enn3hk7bVigxJtxhgMUBp73zqm74CDEGhYwTs7m6NzYufHd" },
  f12: { seed: "0xf20ecd8e0f4aabc67e991ca6be62522b37cdf256819498d84bafea34d5146817",
          ss58: "5CcBRCAi8JAJqdhVNfCYw6Dr9huRaWWggGZsRqqmg73AuJbF" },
};

// ── 子链配置 ────────────────────────────────────────────────────────────────
const CHAINS = [
  {
    chain_id: 0, name: "child1-Delivery",
    ws: CHILD_WS[0],
    miners: ["f1","f2","f3"],
    task_id: 0,
    budget_per_epoch: 1500n * UNIT,
    description: "City delivery crowdsource (AURA-3, 6s)",
    workers: 300,
  },
  {
    chain_id: 1, name: "child2-Traffic",
    ws: CHILD_WS[1],
    miners: ["f4","f5","f6"],
    task_id: 1,
    budget_per_epoch: 2n * UNIT,
    description: "Traffic sensing (AURA-3, 2s)",
    workers: 2000,
  },
  {
    chain_id: 2, name: "child3-Medical",
    ws: CHILD_WS[2],
    miners: ["f7","f8","f9"],
    task_id: 2,
    budget_per_epoch: 40000n * UNIT,
    description: "Medical annotation (AURA-3, 10MB, 30min Epoch)",
    workers: 200,
  },
  {
    chain_id: 3, name: "child4-Finance",
    ws: CHILD_WS[3],
    miners: ["f1","f2","f3","f4","f5","f6","f7"],
    task_id: 3,
    budget_per_epoch: 5000n * UNIT,
    description: "Financial verification (AURA-7, 20min Epoch)",
    workers: 100,
  },
  {
    chain_id: 4, name: "child5-IoT",
    ws: CHILD_WS[4],
    miners: ["f10","f11","f12"],
    task_id: 4,
    budget_per_epoch: UNIT / 2n,   // 0.5 UNIT
    description: "IoT sensor network (AURA-3, 1s, 60s Epoch)",
    workers: 5000,
  },
  {
    chain_id: 5, name: "child6-Market",
    ws: CHILD_WS[5],
    miners: ["f1","f2","f3","f4","f5"],
    task_id: 5,
    budget_per_epoch: 25000n * UNIT,
    description: "Data market (BABE-5, 200-slot Epoch)",
    workers: 500,
  },
].map((chain) => {
  const profileKey = `child${chain.chain_id + 1}`;
  const profile = CHAIN_PROFILES[profileKey];
  if (!profile) {
    throw new Error(`missing chain profile: ${profileKey}`);
  }
  return { ...chain, profileKey, profile };
});

// ── 工具函数 ────────────────────────────────────────────────────────────────

function log(msg) { console.log(`[setup ${new Date().toISOString()}] ${msg}`); }
function skip(step) { return ONLY_STEP !== null && ONLY_STEP !== step; }

function printDryRunPlan() {
  if (!skip(1)) log("Step 1: 给 f1-f12 验证人账户充值 100 UNIT");
  if (!skip(2)) log("Step 2: Alice 向 FMC 充值 500,000 UNIT");
  if (!skip(3)) {
    log("Step 3: 注册子链到 CCMC");
    for (const chain of CHAINS) {
      log(`  ${chain.profileKey}: scene=${chain.profile.scene}, settlement=${chain.profile.settlement}, expected_chain_id=${chain.chain_id}`);
    }
  }
  if (!skip(4)) {
    log("Step 4: 矿工账户加入各自子链");
    for (const chain of CHAINS) {
      log(`  ${chain.profileKey}: miners=${chain.miners.join(",")}`);
    }
  }
  if (!skip(5)) {
    log("Step 5: 创建并激活 FMC 众包任务");
    for (const chain of CHAINS) {
      if (chain.profile.scene !== "Crowdsource" || chain.profile.settlement !== "FmcTaskBill") {
        log(`  ${chain.profileKey}: skip FMC crowdsource task for scene=${chain.profile.scene}, settlement=${chain.profile.settlement}`);
      } else {
        log(`  ${chain.profileKey}: create/activate task_id=${chain.task_id}`);
      }
    }
  }
  if (!skip(6)) {
    log("Step 6: 向众包子链同步任务");
    for (const chain of CHAINS) {
      log(
        chain.profile.scene === "Crowdsource"
          ? `  ${chain.profileKey}: sync crowdsource task_id=${chain.task_id}`
          : `  ${chain.profileKey}: skip crowdsource sync for scene=${chain.profile.scene}`
      );
    }
  }
  if (!skip(7)) {
    log("Step 7: 给众包 worker 账户充值");
    for (const chain of CHAINS) {
      log(
        chain.profile.scene === "Crowdsource"
          ? `  ${chain.profileKey}: fund ${chain.workers} workers`
          : `  ${chain.profileKey}: skip crowdsource worker funding for scene=${chain.profile.scene}`
      );
    }
  }
}

async function sendTx(api, tx, signer, label) {
  if (DRY_RUN) {
    log(`  [dry-run] ${label}`);
    return;
  }
  return new Promise((resolve, reject) => {
    tx.signAndSend(signer, ({ status, dispatchError }) => {
      if (dispatchError) {
        if (dispatchError.isModule) {
          const { name } = api.registry.findMetaError(dispatchError.asModule);
          // 幂等：AlreadyAMiner / ChainNotFound 等不视为致命错误
          if (["AlreadyAMiner", "AlreadyVoted"].includes(name)) {
            log(`  [skip] ${label}: ${name}`);
            resolve();
            return;
          }
        }
        reject(new Error(`${label} failed: ${dispatchError}`));
      } else if (status.isInBlock) {
        log(`  ✓ ${label}`);
        resolve();
      }
    }).catch(reject);
  });
}

async function batchTransfer(api, from, recipients, amount, label) {
  log(`  ${label}（${recipients.length} 个账户，各 ${(amount / UNIT).toString()} UNIT）`);
  if (DRY_RUN) { log(`  [dry-run] batch transfer (${recipients.length} txs)`); return; }

  // 并行提交：使用顺序 nonce，节点可将多笔 tx 打包进同一个块
  const startNonce = (await api.rpc.system.accountNextIndex(from.address)).toNumber();
  log(`  起始 nonce=${startNonce}，并行提交 ${recipients.length} 笔...`);

  const CHUNK = 200; // 每批并发 200 笔，避免连接超时
  for (let i = 0; i < recipients.length; i += CHUNK) {
    const chunk = recipients.slice(i, i + CHUNK);
    await Promise.all(chunk.map((dest, j) =>
      new Promise((resolve, reject) => {
        api.tx.balances.transferKeepAlive(dest, amount)
          .signAndSend(from, { nonce: startNonce + i + j }, ({ status, dispatchError }) => {
            if (dispatchError) reject(new Error(`transfer to ${dest}: ${dispatchError}`));
            else if (status.isInBlock) resolve();
          }).catch(reject);
      })
    ));
    log(`  确认进度: ${Math.min(i + CHUNK, recipients.length)}/${recipients.length}`);
  }
}

// ── 主流程 ──────────────────────────────────────────────────────────────────

async function main() {
  log("=== FishboneChain 实验初始化 ===");
  if (DRY_RUN) log(">>> DRY RUN 模式：只打印不执行 <<<");
  if (ONLY_STEP !== null) log(`>>> 仅执行 Step ${ONLY_STEP} <<<`);
  if (DRY_RUN) {
    printDryRunPlan();
    log("\n=== dry-run 完成 ===");
    return;
  }

  // ── 连接主链 ────────────────────────────────────────────────────────────
  log(`\n连接主链 ${MAIN_WS}...`);
  const mainApi = await ApiPromise.create({ provider: new WsProvider(MAIN_WS) });
  const keyring = new Keyring({ type: "sr25519" });
  const alice   = keyring.addFromUri("//Alice");
  log(`主链已连接，Alice = ${alice.address}`);

  // 加载所有验证人 keypair
  const valKeys = {};
  for (const [id, v] of Object.entries(VALIDATORS)) {
    valKeys[id] = keyring.addFromUri(v.seed);
  }

  // ── Step 1：给验证人账户充值 ────────────────────────────────────────────
  if (!skip(1)) {
    log("\n── Step 1：给 f1-f12 验证人账户充值 100 UNIT（手续费 + CCMC 押金）");
    const ss58s = Object.values(VALIDATORS).map(v => v.ss58);
    await batchTransfer(mainApi, alice, ss58s, 100n * UNIT, "验证人账户充值");
  }

  // ── Step 2：Alice 向 FMC 充值 ───────────────────────────────────────────
  if (!skip(2)) {
    log("\n── Step 2：Alice 向 FMC 充值 500,000 UNIT");
    await sendTx(mainApi, mainApi.tx.fmc.deposit(500_000n * UNIT), alice,
      "fmc.deposit(500000 UNIT)");
  }

  // ── Step 3：注册 6 条子链到 CCMC ────────────────────────────────────────
  if (!skip(3)) {
    log("\n── Step 3：注册 6 条子链到 CCMC（dep=0, min_miners=1）");
    for (const chain of CHAINS) {
      const nameBytes = Array.from(new TextEncoder().encode(chain.name));
      await sendTx(
        mainApi,
        mainApi.tx.ccmc.registerChildChain(nameBytes, 1, 0),
        alice,
        `ccmc.register("${chain.name}") → 期望 chain_id=${chain.chain_id}`
      );
    }
    log("  ★ 请确认 CCMC 分配的 chain_id 与脚本预期（0-5）一致");
    log("    可在 Polkadot.js 查询 ccmc.childChains 存储核实");
  }

  // ── Step 4：矿工账户加入各自子链（bridge 投票账户必须在此注册）────────
  if (!skip(4)) {
    log("\n── Step 4：矿工账户加入各自子链");
    for (const chain of CHAINS) {
      log(`  child${chain.chain_id + 1} (chain_id=${chain.chain_id})：${chain.miners.join(", ")} 加入`);
      for (const minerId of chain.miners) {
        // 支持 "alice" 特殊值（使用 Alice 账户作为单矿工测试 bridge）
        const signer = minerId === "alice" ? alice : valKeys[minerId];
        if (!signer) {
          log(`  [warn] 未知矿工 ID: ${minerId}，跳过`);
          continue;
        }
        await sendTx(
          mainApi,
          mainApi.tx.ccmc.joinChildChain(chain.chain_id),
          signer,
          `ccmc.joinChildChain(${chain.chain_id}) by ${minerId}`
        );
      }
    }
  }

  // ── Step 5：创建并激活 6 个任务 ──────────────────────────────────────────
  if (!skip(5)) {
    log("\n── Step 5：Alice 创建并激活 6 个任务");
    for (const chain of CHAINS) {
      if (chain.profile.scene !== "Crowdsource" || chain.profile.settlement !== "FmcTaskBill") {
        log(`  ${chain.profileKey}: skip FMC crowdsource task for scene=${chain.profile.scene}, settlement=${chain.profile.settlement}`);
        continue;
      }
      const descBytes = Array.from(new TextEncoder().encode(chain.description));
      await sendTx(
        mainApi,
        mainApi.tx.fmc.createTask(chain.chain_id, chain.budget_per_epoch, descBytes),
        alice,
        `fmc.createTask(chain=${chain.chain_id}, budget=${chain.budget_per_epoch / UNIT} UNIT)`
      );
      await sendTx(
        mainApi,
        mainApi.tx.fmc.activateTask(chain.task_id),
        alice,
        `fmc.activateTask(task_id=${chain.task_id})`
      );
    }
  }

  await mainApi.disconnect();

  // ── Step 6：各子链 sync_task ─────────────────────────────────────────────
  if (!skip(6)) {
    log("\n── Step 6：向各子链同步任务（Alice 调用 crowdsource.sync_task）");
    for (const chain of CHAINS) {
      if (chain.profile.scene !== "Crowdsource") {
        log(`  ${chain.profileKey}: skip crowdsource sync for scene=${chain.profile.scene}`);
        continue;
      }
      log(`  连接 ${chain.ws}...`);
      const api = await ApiPromise.create({ provider: new WsProvider(chain.ws) });
      const descBytes = Array.from(new TextEncoder().encode(chain.description));
      await sendTx(
        api,
        api.tx.crowdsource.syncTask(
          chain.task_id,    // task_id
          alice.address,    // requester
          chain.budget_per_epoch,
          descBytes
        ),
        alice,
        `crowdsource.syncTask(task=${chain.task_id}) on chain${chain.chain_id + 1}`
      );
      await api.disconnect();
    }
  }

  // ── Step 7：给工作者账户充值（各子链上执行）────────────────────────────
  if (!skip(7)) {
    log("\n── Step 7：给工作者账户充值（各子链，每账户 10 UNIT）");
    for (const chain of CHAINS) {
      if (chain.profile.scene !== "Crowdsource") {
        log(`  ${chain.profileKey}: skip crowdsource worker funding for scene=${chain.profile.scene}`);
        continue;
      }
      log(`  连接 ${chain.ws}（${chain.workers} 个 //Worker 账户）...`);
      const api = await ApiPromise.create({ provider: new WsProvider(chain.ws) });

      const workerAddrs = [];
      for (let i = 0; i < chain.workers; i++) {
        workerAddrs.push(keyring.addFromUri(`//Worker${i}`).address);
      }

      await batchTransfer(api, alice, workerAddrs, 10n * UNIT,
        `chain${chain.chain_id + 1} workers`);

      await api.disconnect();
    }
  }

  log("\n=== 初始化完成 ===");
  log("可以开始运行 worker.js 和 metrics.js 了");
  log("\n下一步：");
  log("  实验 A（基准）： node scripts/worker.js --scenario a --ws ws://10.2.2.11:9945 --task-id 0");
  log("  实验 C（并发）： 见 docs/experiments/experiment-report.md");
}

main().catch(e => {
  console.error("[setup 致命错误]", e.message);
  console.error(e.stack);
  process.exit(1);
});
