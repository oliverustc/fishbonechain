/**
 * FishboneChain 真实 Gnark ZK E2E 流程脚本。
 *
 * 调用 fishbone-zk CLI 生成/验证真实 Groth16 proof，在 child6 上完成完整数据交易闭环。
 *
 * 用法：
 *   ZK_VERIFIER_CMD=target/tools/fishbone-zk \
 *   node scripts/zk_real_data_trade_flow.js --main ws://... --child ws://...
 *
 * Note: This script is NOT yet VM-verified. It is a staged integration point for the
 * gnark CH/RO proof pipeline. See docs/implementation/data-trade-zk-verifier-plan.md.
 */

import { ApiPromise, WsProvider, Keyring } from "@polkadot/api";
import { spawnSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { hashNTimes, paymentPreimageForRemaining } from "./lib/hash_chain.js";
import { sampleListing } from "./lib/data_trade_sample.js";
import { findEvent, eventDataNumber } from "./lib/data_trade_events.js";
import {
  assertEscrowMatchesTradeTerms,
  assertSessionMatchesListingAndEscrow,
} from "./lib/data_trade_binding.js";
import { computeZkAttestationDigest } from "./lib/zk_attestation.js";
import { assertValidZkArtifact, readZkArtifact } from "./lib/zk_artifact.js";

function parseArg(flag) {
  const idx = process.argv.indexOf(flag);
  return idx !== -1 ? process.argv[idx + 1] : null;
}

const MAIN_WS = parseArg("--main") || "ws://127.0.0.1:9944";
const CHILD_WS = parseArg("--child") || "ws://127.0.0.1:9950";
const ZK_CMD = process.env.ZK_VERIFIER_CMD || "target/tools/fishbone-zk";
const BUSINESS_WITNESS = parseArg("--business-witness") || "scripts/fixtures/data_trade_business_sample.json";
const VERBOSE = process.argv.includes("--verbose");

function log(msg) {
  console.log(`[zk-real-e2e ${new Date().toISOString()}] ${msg}`);
}

function verifierAccepted(output) {
  return output.split(/\r?\n/).some((line) => line.trim() === "accepted");
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
  log(`ZK CLI: ${ZK_CMD}`);

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
  log("verifier=gnark-groth16-bn254");
  log(`主链: ${await mainApi.rpc.system.chain()}  子链: ${await childApi.rpc.system.chain()}`);

  const sample = sampleListing();
  const maxRounds = 3;
  const pricePerRound = 100;
  const depositHint = 500;
  const seed = "zk-real-data-trade-secret";
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

    // ── Round delivery with real gnark ZK proof ──
    for (let round = 0; round < 2; round++) {
      log(`Round ${round} delivery (real gnark ZK)...`);

      const outDir = `target/data-trade-zk/session-${sessionId}-round-${round}`;
      mkdirSync(outDir, { recursive: true });

      // 1. Generate fixture via fishbone-zk CLI
      const fixResult = spawnSync(ZK_CMD, [
        "business-fixture", "--witness", BUSINESS_WITNESS, "--out", outDir,
        "--request-hash", sample.requestHash,
        "--session-id", String(sessionId),
        "--round-index", String(round),
      ], { stdio: "inherit" });
      if (fixResult.status !== 0) throw new Error(`fishbone-zk fixture failed: ${fixResult.status}`);

      // 2. Read and validate artifact
      const artifactPath = `${outDir}/artifact.json`;
      const artifact = assertValidZkArtifact(readZkArtifact(artifactPath));

      // 3. Verify artifact via CLI
      const verifyResult = spawnSync(ZK_CMD, ["verify", "--artifact", artifactPath], {
        encoding: "utf8",
      });
      if (verifyResult.status !== 0 || !verifierAccepted(verifyResult.stdout)) {
        throw new Error(`fishbone-zk verify rejected: ${verifyResult.stderr || verifyResult.stdout}`);
      }

      // 4. Submit round
      const ch = hashNTimes(`round-${round}`, 1);
      await submitTx(alice, childApi.tx.tradeSession.openRound(sessionId, round, ch), `openRound(${round})`);
      await submitTx(alice, childApi.tx.tradeSession.submitPaymentProof(sessionId, round, ch), `submitPaymentProof(${round})`);

      // 5. Submit ZK proof metadata on-chain
      await submitTx(bob, childApi.tx.tradeSession.submitDataProof(
        sessionId, round,
        "GnarkGroth16Bn254", "Range", 10,
        artifact.ch_proof_hash, artifact.ro_proof_hash,
        artifact.public_input_hash, artifact.vk_hash,
        artifact.business_input_hash, artifact.proof_digest,
      ), `submitDataProof(${round})`);

      // 6. Verifier attestation (Charlie)
      const attDigest = computeZkAttestationDigest({
        sessionId, roundIndex: round, proofDigest: artifact.proof_digest,
        accepted: true, verifierAccount: charlie.addressRaw,
      });
      await submitTx(charlie, childApi.tx.tradeSession.attestDataProof(sessionId, round, artifact.proof_digest, true, attDigest), `attestDataProof(${round})`);

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
    log("✅ Real ZK-attested path 完成！");
    log("verifier=gnark-groth16-bn254 (off-chain proof, on-chain attestation)");
  } finally {
    await Promise.all([mainApi.disconnect(), childApi.disconnect()]);
  }
}

main().catch((e) => {
  console.error("[zk-real-e2e 致命错误]", e.message);
  console.error(e.stack);
  process.exit(1);
});
