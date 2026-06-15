/**
 * FishboneChain 数据交易 E2E 流程脚本。
 *
 * 运行论文完整数据交易流程：DO publish listing → DR lock funds → DO lock deposit →
 * DR create session → DO accept → round delivery → DO claim settlement → main chain settle.
 *
 * 所有 ID 从链上事件提取，不硬编码。
 *
 * 用法：
 *   node scripts/data_trade_flow.js --main ws://... --child ws://... --scenario happy
 *   node scripts/data_trade_flow.js --main ws://... --child ws://... --scenario invalid-proof
 *   node scripts/data_trade_flow.js --main ws://... --child ws://... --scenario requester-refuses-payment
 */

import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";
import { hashNTimes, paymentPreimageForRemaining } from "./lib/hash_chain.js";
import { sampleListing } from "./lib/data_trade_sample.js";
import { findEvent, eventDataNumber } from "./lib/data_trade_events.js";
import {
  assertEscrowMatchesTradeTerms,
  assertSessionMatchesListingAndEscrow,
} from "./lib/data_trade_binding.js";

function parseArg(flag) {
  const idx = process.argv.indexOf(flag);
  return idx !== -1 ? process.argv[idx + 1] : null;
}

const MAIN_WS = parseArg("--main") || "ws://127.0.0.1:9944";
const CHILD_WS = parseArg("--child") || "ws://127.0.0.1:9950";
const SCENARIO = parseArg("--scenario") || "happy";
const VERBOSE = process.argv.includes("--verbose");

function log(msg) {
  console.log(`[data-trade-e2e ${new Date().toISOString()}] ${msg}`);
}

async function submitTx(signer, tx, label) {
  return new Promise((resolve, reject) => {
    tx.signAndSend(signer, ({ status, dispatchError, events }) => {
      if (dispatchError) {
        if (dispatchError.isModule) {
          const decoded = tx.registry.findMetaError(dispatchError.asModule);
          reject(new Error(`${label}: ${decoded.section}.${decoded.name}: ${decoded.docs.join(" ")}`));
        } else {
          reject(new Error(`${label}: ${dispatchError.toString()}`));
        }
        return;
      }
      if (status.isInBlock || status.isFinalized) {
        if (VERBOSE) log(`  ✅ ${label} in block ${status.isInBlock ? status.asInBlock : status.asFinalized}`);
        resolve({ status, events: events || [] });
      }
    }).catch(reject);
  });
}

/**
 * Setup: publish listing, open escrow, lock funds, lock deposit, create session, accept.
 * Returns { listingId, escrowId, sessionId } from on-chain events.
 */
async function setupTrade(mainApi, childApi, alice, bob, sample, maxRounds, pricePerRound, depositHint, hashChainAnchor) {
  // ── publish listing ──
  const publishResult = await submitTx(bob, childApi.tx.dataRegistry.publishData(
    sample.imtRoot, sample.description, pricePerRound, maxRounds, depositHint,
    sample.requestHash, sample.proofParamsHash,
  ), "publishData");
  const listingId = eventDataNumber(findEvent(publishResult, "dataRegistry", "DataPublished"), "listingId");
  log(`  listing_id=${listingId}`);

  // ── open escrow ──
  const escrowResult = await submitTx(alice, mainApi.tx.mainEscrow.openEscrow(
    bob.address, maxRounds, pricePerRound, depositHint, hashChainAnchor,
  ), "openEscrow");
  const escrowId = eventDataNumber(findEvent(escrowResult, "mainEscrow", "EscrowOpened"), "escrowId");
  log(`  escrow_id=${escrowId}`);

  // ── lock funds ──
  await submitTx(alice, mainApi.tx.mainEscrow.lockFunds(escrowId), "lockFunds");
  log(`  reserved ${maxRounds * pricePerRound}`);

  // ── lock deposit ──
  await submitTx(bob, mainApi.tx.mainEscrow.lockDeposit(escrowId), "lockDeposit");
  log(`  reserved ${depositHint}`);

  // ── binding check: escrow is Ready and matches terms ──
  await assertEscrowMatchesTradeTerms(mainApi, escrowId, {
    requester: alice.address,
    dataOwner: bob.address,
    maxRounds,
    pricePerRound,
    deposit: depositHint,
    hashChainAnchor,
  });

  // ── create session ──
  const sessionResult = await submitTx(alice, childApi.tx.tradeSession.createSession(
    listingId, escrowId, bob.address, sample.requestHash,
    pricePerRound, maxRounds, hashChainAnchor, "MainEscrow",
  ), "createSession");
  const sessionId = eventDataNumber(findEvent(sessionResult, "tradeSession", "SessionCreated"), "sessionId");
  log(`  session_id=${sessionId}`);

  // ── binding check: session matches listing and escrow ──
  await assertSessionMatchesListingAndEscrow(childApi, sessionId, {
    listingId,
    escrowId,
    requester: alice.address,
    dataOwner: bob.address,
    maxRounds,
    pricePerRound,
    hashChainAnchor,
  });

  // ── accept session ──
  await submitTx(bob, childApi.tx.tradeSession.acceptSession(sessionId), "acceptSession");

  return { listingId, escrowId, sessionId };
}

/**
 * Run one complete delivery round (DR=alice, DO=bob).
 */
async function completeRound(childApi, alice, bob, sessionId, roundIndex, charlie) {
  const ch = hashNTimes(`round-${roundIndex}`, 1);
  await submitTx(alice, childApi.tx.tradeSession.openRound(sessionId, roundIndex, ch), `openRound(${roundIndex})`);
  await submitTx(alice, childApi.tx.tradeSession.submitPaymentProof(sessionId, roundIndex, ch), `submitPaymentProof(${roundIndex})`);
  await submitTx(bob, childApi.tx.tradeSession.submitDataProof(sessionId, roundIndex, ch), `submitDataProof(${roundIndex})`);
  // Dev verifier attestation (Charlie)
  await submitTx(charlie, childApi.tx.tradeSession.attestDataProof(sessionId, roundIndex, ch, true), `attestDataProof(${roundIndex})`);
  await submitTx(alice, childApi.tx.tradeSession.submitProofSignature(sessionId, roundIndex, ch), `submitProofSignature(${roundIndex})`);
  await submitTx(bob, childApi.tx.tradeSession.submitDataDeliveryHash(sessionId, roundIndex, ch), `submitDataDeliveryHash(${roundIndex})`);
  await submitTx(alice, childApi.tx.tradeSession.submitPaymentPreimage(sessionId, roundIndex, ch), `submitPaymentPreimage(${roundIndex})`);
}

async function happyPath(mainApi, childApi, alice, bob, charlie) {
  log("=== Happy Path ===");
  const sample = sampleListing();
  const maxRounds = 3;
  const pricePerRound = 100;
  const depositHint = 500;
  const seed = "data-trade-secret-42";
  const hashChainAnchor = hashNTimes(seed, maxRounds);
  log(`hash_chain_anchor (H^${maxRounds}(seed)): ${hashChainAnchor}`);

  const { escrowId, sessionId } = await setupTrade(
    mainApi, childApi, alice, bob, sample, maxRounds, pricePerRound, depositHint, hashChainAnchor,
  );

  // ── Round delivery (2 rounds) ──
  for (let round = 0; round < 2; round++) {
    log(`Round ${round} delivery...`);
    await completeRound(childApi, alice, bob, sessionId, round, charlie);
  }

  // ── DO claims settlement ──
  const remainingRounds = 1;
  const preimage = paymentPreimageForRemaining(seed, remainingRounds);
  log(`DO claimSettlement (${maxRounds - remainingRounds}/${maxRounds} rounds)...`);
  await submitTx(bob, childApi.tx.tradeSession.claimSettlement(sessionId, preimage, remainingRounds), "claimSettlement");

  // ── DO calls mainEscrow.settleByPreimage ──
  log("DO settleByPreimage on main...");
  await submitTx(bob, mainApi.tx.mainEscrow.settleByPreimage(escrowId, preimage, remainingRounds), "settleByPreimage");
  log("  结算完成：DO 获得 2 轮付款，DR 退款 1 轮，押金释放");

  const bobBal = await mainApi.query.system.account(bob.address);
  const aliceBal = await mainApi.query.system.account(alice.address);
  log(`  验证：Bob free = ${bobBal.data.free}, reserved = ${bobBal.data.reserved}`);
  log(`         Alice free = ${aliceBal.data.free}, reserved = ${aliceBal.data.reserved}`);
  log("✅ Happy path 完成！");
}

async function invalidProofScenario(mainApi, childApi, alice, bob) {
  log("=== Invalid Proof Scenario ===");
  const sample = sampleListing();
  const maxRounds = 3;
  const pricePerRound = 100;
  const depositHint = 500;
  const hashChainAnchor = hashNTimes("bad-proof-secret", maxRounds);

  const { escrowId, sessionId } = await setupTrade(
    mainApi, childApi, alice, bob, sample, maxRounds, pricePerRound, depositHint, hashChainAnchor,
  );

  // Round 0: submit payment proof, submit data proof, then DR disputes
  const ch = hashNTimes("round-0", 1);
  await submitTx(alice, childApi.tx.tradeSession.openRound(sessionId, 0, ch), "openRound(0)");
  await submitTx(alice, childApi.tx.tradeSession.submitPaymentProof(sessionId, 0, ch), "submitPaymentProof(0)");
  await submitTx(bob, childApi.tx.tradeSession.submitDataProof(sessionId, 0, ch), "submitDataProof(0)");

  log("DR 争议无效 proof...");
  await submitTx(alice, childApi.tx.tradeSession.disputeInvalidProof(sessionId, 0, ch), "disputeInvalidProof");

  log("DR 在主链 punishDataOwner...");
  await submitTx(alice, mainApi.tx.mainEscrow.punishDataOwner(escrowId), "punishDataOwner");

  const bobBal = await mainApi.query.system.account(bob.address);
  const aliceBal = await mainApi.query.system.account(alice.address);
  log(`  DO (Bob) reserved = ${bobBal.data.reserved} (应为 0，押金被 slash)`);
  log(`  DR (Alice) reserved = ${aliceBal.data.reserved} (应为 0，资金已退款)`);
  log("✅ Invalid proof 场景完成！DO 被 punish");
}

async function refusesPaymentScenario(mainApi, childApi, alice, bob, charlie) {
  log("=== Requester Refuses Payment Scenario ===");
  const sample = sampleListing();
  const maxRounds = 3;
  const pricePerRound = 100;
  const depositHint = 500;
  const hashChainAnchor = hashNTimes("refuse-secret", maxRounds);

  const { escrowId, sessionId } = await setupTrade(
    mainApi, childApi, alice, bob, sample, maxRounds, pricePerRound, depositHint, hashChainAnchor,
  );

  // Complete up to DataDelivered — but DR refuses to submit PaymentPreimage
  const ch = hashNTimes("round-0", 1);
  await submitTx(alice, childApi.tx.tradeSession.openRound(sessionId, 0, ch), "openRound(0)");
  await submitTx(alice, childApi.tx.tradeSession.submitPaymentProof(sessionId, 0, ch), "submitPaymentProof(0)");
  await submitTx(bob, childApi.tx.tradeSession.submitDataProof(sessionId, 0, ch), "submitDataProof(0)");
  await submitTx(charlie, childApi.tx.tradeSession.attestDataProof(sessionId, 0, ch, true), "attestDataProof(0)");
  await submitTx(alice, childApi.tx.tradeSession.submitProofSignature(sessionId, 0, ch), "submitProofSignature(0)");
  await submitTx(bob, childApi.tx.tradeSession.submitDataDeliveryHash(sessionId, 0, ch), "submitDataDeliveryHash(0)");

  log("DR 拒绝支付！DO 调用 claimLastPayment...");
  await submitTx(bob, childApi.tx.tradeSession.claimLastPayment(sessionId, 0), "claimLastPayment");

  log("DO 在主链 claimLastPayment...");
  await submitTx(bob, mainApi.tx.mainEscrow.claimLastPayment(escrowId, 0), "claimLastPayment");

  const bobBal = await mainApi.query.system.account(bob.address);
  log(`  DO (Bob) free = ${bobBal.data.free} (获得 1 轮付款)`);
  log("✅ Requester refuses payment 场景完成！");
}

async function main() {
  log(`连接主链: ${MAIN_WS}  子链: ${CHILD_WS}`);
  log(`场景: ${SCENARIO}`);

  const [mainApi, childApi] = await Promise.all([
    ApiPromise.create({ provider: new WsProvider(MAIN_WS) }),
    ApiPromise.create({ provider: new WsProvider(CHILD_WS) }),
  ]);

  const keyring = new Keyring({ type: "sr25519" });
  const alice = keyring.addFromUri("//Alice");
  const bob = keyring.addFromUri("//Bob");
  const charlie = keyring.addFromUri("//Charlie");

  log(`Alice (DR): ${alice.address}`);
  log(`Bob   (DO): ${bob.address}`);
  log(`Charlie (Verifier): ${charlie.address}`);
  log("verifier=dev-attested");
  log(`主链: ${await mainApi.rpc.system.chain()}  子链: ${await childApi.rpc.system.chain()}`);

  try {
    switch (SCENARIO) {
      case "happy":
        await happyPath(mainApi, childApi, alice, bob, charlie);
        break;
      case "invalid-proof":
        await invalidProofScenario(mainApi, childApi, alice, bob);
        break;
      case "requester-refuses-payment":
        await refusesPaymentScenario(mainApi, childApi, alice, bob, charlie);
        break;
      default:
        console.error(`未知场景: ${SCENARIO}。可用: happy, invalid-proof, requester-refuses-payment`);
        process.exit(1);
    }
  } finally {
    await Promise.all([mainApi.disconnect(), childApi.disconnect()]);
  }
}

main().catch((e) => {
  console.error("[data-trade-e2e 致命错误]", e.message);
  console.error(e.stack);
  process.exit(1);
});
