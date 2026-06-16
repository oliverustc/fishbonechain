/**
 * FishboneChain 数据交易 ZK-Attested E2E 流程脚本。
 *
 * 与 data_trade_flow.js 相同但输出 verifier=dev-zk-attested，用于区分 base 和 ZK 模式。
 *
 * 用法：
 *   node scripts/zk_attested_data_trade_flow.js --main ws://... --child ws://...
 */

import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";
import { hashNTimes, paymentPreimageForRemaining } from "./lib/hash_chain.js";
import { sampleListing } from "./lib/data_trade_sample.js";
import { findEvent, eventDataNumber } from "./lib/data_trade_events.js";
import {
  assertEscrowMatchesTradeTerms,
  assertSessionMatchesListingAndEscrow,
} from "./lib/data_trade_binding.js";
import { computeZkAttestationDigest } from "./lib/zk_attestation.js";
import { computeProofDigest } from "./lib/zk_artifact.js";
import { loadTradeProfile, parseProfileArg } from "./lib/trade_profile.js";

function parseArg(flag) {
  const idx = process.argv.indexOf(flag);
  return idx !== -1 ? process.argv[idx + 1] : null;
}

function devProofDigest({ requestHash, sessionId, roundIndex, vkHash, chProofHash, roProofHash, publicInputHash }) {
  return computeProofDigest({
    version: 1,
    proof_system: "gnark-groth16-bn254",
    proof_system_code: 1,
    constraint_kind: "range",
    constraint_kind_code: 1,
    ro_depth: 10,
    request_hash: requestHash,
    session_id: sessionId,
    round_index: roundIndex,
    vk_hash: vkHash,
    ch_proof_hash: chProofHash,
    ro_proof_hash: roProofHash,
    public_input_hash: publicInputHash,
    business_input_hash: "0x0000000000000000000000000000000000000000000000000000000000000000",
  });
}

const PROFILE = parseProfileArg();
const PROFILE_CONFIG = PROFILE ? loadTradeProfile(PROFILE) : null;
const MAIN_WS = parseArg("--main") || PROFILE_CONFIG?.main_ws || "ws://127.0.0.1:9944";
const CHILD_WS = parseArg("--child") || PROFILE_CONFIG?.child_ws || "ws://127.0.0.1:9950";
const VERBOSE = process.argv.includes("--verbose");

function log(msg) {
  console.log(`[zk-attested-e2e ${new Date().toISOString()}] ${msg}`);
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

async function main() {
  log(`连接主链: ${MAIN_WS}  子链: ${CHILD_WS}`);

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
  log("verifier=dev-zk-attested");
  log(`主链: ${await mainApi.rpc.system.chain()}  子链: ${await childApi.rpc.system.chain()}`);

  const sample = sampleListing();
  const maxRounds = 3;
  const pricePerRound = 100;
  const depositHint = 500;
  const seed = "zk-attested-data-trade-secret";
  const hashChainAnchor = hashNTimes(seed, maxRounds);
  log(`hash_chain_anchor (H^${maxRounds}(seed)): ${hashChainAnchor}`);

  try {
    // ── Setup: publish, escrow, lock, session ──
    const publishResult = await submitTx(bob, childApi.tx.dataRegistry.publishData(
      sample.imtRoot, sample.description, pricePerRound, maxRounds, depositHint,
      sample.requestHash, sample.proofParamsHash,
    ), "publishData");
    const listingId = eventDataNumber(findEvent(publishResult, "dataRegistry", "DataPublished"), "listingId");
    log(`listing_id=${listingId}`);

    const escrowResult = await submitTx(alice, mainApi.tx.mainEscrow.openEscrow(
      bob.address, maxRounds, pricePerRound, depositHint, hashChainAnchor,
    ), "openEscrow");
    const escrowId = eventDataNumber(findEvent(escrowResult, "mainEscrow", "EscrowOpened"), "escrowId");
    log(`escrow_id=${escrowId}`);

    await submitTx(alice, mainApi.tx.mainEscrow.lockFunds(escrowId), "lockFunds");
    await submitTx(bob, mainApi.tx.mainEscrow.lockDeposit(escrowId), "lockDeposit");

    await assertEscrowMatchesTradeTerms(mainApi, escrowId, {
      requester: alice.address, dataOwner: bob.address,
      maxRounds, pricePerRound, deposit: depositHint, hashChainAnchor,
    });

    const sessionResult = await submitTx(alice, childApi.tx.tradeSession.createSession(
      listingId, escrowId, bob.address, sample.requestHash,
      pricePerRound, maxRounds, hashChainAnchor, "MainEscrow",
    ), "createSession");
    const sessionId = eventDataNumber(findEvent(sessionResult, "tradeSession", "SessionCreated"), "sessionId");
    log(`session_id=${sessionId}`);

    await assertSessionMatchesListingAndEscrow(childApi, sessionId, {
      listingId, escrowId,
      requester: alice.address, dataOwner: bob.address,
      maxRounds, pricePerRound, hashChainAnchor,
    });

    await submitTx(bob, childApi.tx.tradeSession.acceptSession(sessionId), "acceptSession");

    // ── Round delivery with ZK attestation ──
    for (let round = 0; round < 2; round++) {
      log(`Round ${round} delivery (with zk attestation)...`);
      const ch = hashNTimes(`round-${round}`, 1);
      const proofDigest = devProofDigest({
        requestHash: sample.requestHash,
        sessionId, roundIndex: round,
        vkHash: ch, chProofHash: ch, roProofHash: ch, publicInputHash: ch,
      });
      await submitTx(alice, childApi.tx.tradeSession.openRound(sessionId, round, ch), `openRound(${round})`);
      await submitTx(alice, childApi.tx.tradeSession.submitPaymentProof(sessionId, round, ch), `submitPaymentProof(${round})`);
      await submitTx(bob, childApi.tx.tradeSession.submitDataProof(
        sessionId, round, "GnarkGroth16Bn254", "Range", 10, ch, ch, ch, ch, '0x0000000000000000000000000000000000000000000000000000000000000000', proofDigest,
      ), `submitDataProof(${round})`);
      // ZK attestation by verifier (Charlie)
      const attDigest = computeZkAttestationDigest({
        sessionId, roundIndex: round, proofDigest, accepted: true, verifierAccount: charlie.addressRaw,
      });
      await submitTx(charlie, childApi.tx.tradeSession.attestDataProof(sessionId, round, proofDigest, true, attDigest), `attestDataProof(${round})`);
      await submitTx(alice, childApi.tx.tradeSession.submitProofSignature(sessionId, round, ch), `submitProofSignature(${round})`);
      await submitTx(bob, childApi.tx.tradeSession.submitDataDeliveryHash(sessionId, round, ch), `submitDataDeliveryHash(${round})`);
      await submitTx(alice, childApi.tx.tradeSession.submitPaymentPreimage(sessionId, round, ch), `submitPaymentPreimage(${round})`);
    }

    // ── Settlement ──
    const remainingRounds = 1;
    const preimage = paymentPreimageForRemaining(seed, remainingRounds);
    log(`claimSettlement (${maxRounds - remainingRounds}/${maxRounds} rounds)...`);
    await submitTx(bob, childApi.tx.tradeSession.claimSettlement(sessionId, preimage, remainingRounds), "claimSettlement");

    log("settleByPreimage on main...");
    await submitTx(bob, mainApi.tx.mainEscrow.settleByPreimage(escrowId, preimage, remainingRounds), "settleByPreimage");

    const bobBal = await mainApi.query.system.account(bob.address);
    const aliceBal = await mainApi.query.system.account(alice.address);
    log(`Bob free = ${bobBal.data.free}, reserved = ${bobBal.data.reserved}`);
    log(`Alice free = ${aliceBal.data.free}, reserved = ${aliceBal.data.reserved}`);
    log("✅ ZK-attested path 完成！");
  } finally {
    await Promise.all([mainApi.disconnect(), childApi.disconnect()]);
  }
}

main().catch((e) => {
  console.error("[zk-attested-e2e 致命错误]", e.message);
  process.exit(1);
});
