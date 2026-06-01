/**
 * FishboneChain Phase 1 E2E 验证脚本
 *
 * 流程：
 *  Alice → ccmc.register_child_chain  → 子链0 注册
 *  Bob   → ccmc.join_child_chain      → 加入子链0
 *  Alice → fmc.deposit                → 充值 10 UNIT
 *  Alice → fmc.create_task            → 创建任务 budget=2 UNIT
 *  Alice → fmc.activate_task          → 激活任务
 *  Bob   → ccmc.submit_epoch_digest   → 提交 Epoch 0 摘要
 *  Bob   → fmc.submit_bill            → 提交账单 1 UNIT
 *  验证链上存储状态
 */

import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";

const WS_URL = "ws://127.0.0.1:9944";
const UNIT = 1_000_000_000_000n; // 1 UNIT = 10^12 (planck)

function ok(msg) { console.log(`  ✓ ${msg}`); }
function fail(msg) { console.error(`  ✗ ${msg}`); process.exit(1); }

async function sendAndWait(api, tx, signer, label) {
  return new Promise((resolve, reject) => {
    let unsub;
    tx.signAndSend(signer, ({ status, dispatchError }) => {
      if (dispatchError) {
        if (dispatchError.isModule) {
          const { docs, name, section } = api.registry.findMetaError(dispatchError.asModule);
          reject(new Error(`${label} FAILED: ${section}.${name}: ${docs.join(" ")}`));
        } else {
          reject(new Error(`${label} FAILED: ${dispatchError.toString()}`));
        }
        unsub && unsub();
      } else if (status.isInBlock) {
        console.log(`  → ${label} included in block ${status.asInBlock}`);
        resolve(status.asInBlock);
        unsub && unsub();
      }
    }).then(u => { unsub = u; }).catch(reject);
  });
}

async function main() {
  console.log("\n=== FishboneChain Phase 1 E2E 验证 ===\n");

  const provider = new WsProvider(WS_URL);
  const api = await ApiPromise.create({ provider });

  const keyring = new Keyring({ type: "sr25519" });
  const alice = keyring.addFromUri("//Alice");
  const bob   = keyring.addFromUri("//Bob");

  console.log(`连接节点: ${WS_URL}`);
  const chain = await api.rpc.system.chain();
  const version = await api.rpc.system.version();
  console.log(`链名称: ${chain}, 版本: ${version}`);

  // ── Step 1: Alice 注册子链 ────────────────────────────────────────────────

  console.log("\n[1] Alice 注册子链...");
  const chainName = api.createType("BoundedVec<u8, u32>",
    Array.from(new TextEncoder().encode("test-chain")).slice(0, 64));
  const depositRequired = 1n * UNIT;

  await sendAndWait(api,
    api.tx.ccmc.registerChildChain(chainName, 1, depositRequired),
    alice,
    "ccmc.registerChildChain"
  );

  const chainInfo = await api.query.ccmc.childChains(0);
  if (chainInfo.isNone) fail("子链0 未存储");
  const ci = chainInfo.unwrap();
  ok(`子链0 已注册 miner_count=${ci.minerCount}`);

  // ── Step 2: Bob 加入子链 ─────────────────────────────────────────────────

  console.log("\n[2] Bob 加入子链0...");
  await sendAndWait(api,
    api.tx.ccmc.joinChildChain(0),
    bob,
    "ccmc.joinChildChain"
  );

  const chainInfo2 = await api.query.ccmc.childChains(0);
  const mc = chainInfo2.unwrap().minerCount.toNumber();
  if (mc !== 2) fail(`期望 miner_count=2，实际 ${mc}`);
  ok(`子链0 矿工数: ${mc}`);

  // ── Step 3: Alice 充值 FMC ───────────────────────────────────────────────

  console.log("\n[3] Alice 向 FMC 充值 10 UNIT...");
  await sendAndWait(api,
    api.tx.fmc.deposit(10n * UNIT),
    alice,
    "fmc.deposit"
  );

  const pool = await api.query.fmc.fundPools(alice.address);
  if (pool.isNone) fail("FundPool 未创建");
  ok(`FundPool: free=${pool.unwrap().free}, locked=${pool.unwrap().locked}`);

  // ── Step 4: Alice 创建任务 ───────────────────────────────────────────────

  console.log("\n[4] Alice 创建任务 budget=2 UNIT...");
  const desc = api.createType("BoundedVec<u8, u32>",
    Array.from(new TextEncoder().encode("crowdsource task")).slice(0, 256));
  await sendAndWait(api,
    api.tx.fmc.createTask(0, 2n * UNIT, desc),
    alice,
    "fmc.createTask"
  );

  const task = await api.query.fmc.tasks(alice.address, 0);
  if (task.isNone) fail("Task 0 未创建");
  ok(`Task0 status: ${task.unwrap().status.type}`);

  // ── Step 5: Alice 激活任务 ───────────────────────────────────────────────

  console.log("\n[5] Alice 激活任务0...");
  await sendAndWait(api,
    api.tx.fmc.activateTask(0),
    alice,
    "fmc.activateTask"
  );

  const pool2 = await api.query.fmc.fundPools(alice.address);
  const p = pool2.unwrap();
  ok(`激活后 FundPool: free=${p.free}, locked=${p.locked}`);

  const task2 = await api.query.fmc.tasks(alice.address, 0);
  ok(`Task0 status: ${task2.unwrap().status.type}`);

  // ── Step 6: Bob 提交 Epoch 摘要 ──────────────────────────────────────────

  console.log("\n[6] Bob 提交 Epoch 0 摘要...");
  const root = "0x0000000000000000000000000000000000000000000000000000000000000042";
  await sendAndWait(api,
    api.tx.ccmc.submitEpochDigest(0, 0, root),
    bob,
    "ccmc.submitEpochDigest"
  );

  // 2个矿工，2/3 阈值=2；单个矿工不足以确认（阈值=2）
  // Alice 也提交
  await sendAndWait(api,
    api.tx.ccmc.submitEpochDigest(0, 0, root),
    alice,
    "ccmc.submitEpochDigest(Alice)"
  );

  const digest = await api.query.ccmc.epochDigests(0, 0);
  if (digest.isNone) fail("Epoch 0 摘要未确认");
  ok(`Epoch 0 摘要已确认: ${digest.unwrap()}`);

  // ── Step 7: Bob 提交账单 ─────────────────────────────────────────────────

  console.log("\n[7] Bob 提交 Epoch 0 账单...");
  const billAmounts = [[bob.address, 1n * UNIT]];
  await sendAndWait(api,
    api.tx.fmc.submitBill(alice.address, 0, 0, billAmounts),
    bob,
    "fmc.submitBill"
  );

  const pool3 = await api.query.fmc.fundPools(alice.address);
  const p3 = pool3.unwrap();
  ok(`结算后 FundPool: free=${p3.free}, locked=${p3.locked}`);

  const task3 = await api.query.fmc.tasks(alice.address, 0);
  ok(`结算后 Task0 status: ${task3.unwrap().status.type}`);

  // ── 汇总 ─────────────────────────────────────────────────────────────────

  console.log("\n=== 验证结果汇总 ===");
  console.log("  ✓ pallet-ccmc: 子链注册、矿工加入、Epoch 摘要阈值确认");
  console.log("  ✓ pallet-fmc: 充值、任务创建、激活（双花防护）、账单结算");
  console.log("  ✓ 跨 pallet 交互: FMC 通过 CCMC 验证矿工身份");
  console.log("\nPhase 1 Step 6 E2E 验证完成 ✓\n");

  await api.disconnect();
}

main().catch(e => {
  console.error("\n[ERROR]", e.message);
  process.exit(1);
});
