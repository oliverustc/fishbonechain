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
import { mkdirSync, writeFileSync } from "node:fs";
import { readFileSync } from "node:fs";
import { hashNTimes, paymentPreimageForRemaining } from "./lib/hash_chain.js";
import { sampleListing } from "./lib/data_trade_sample.js";
import { findEvent, eventDataNumber } from "./lib/data_trade_events.js";
import {
  assertEscrowMatchesTradeTerms,
  assertSessionMatchesListingAndEscrow,
} from "./lib/data_trade_binding.js";
import { computeZkAttestationDigest } from "./lib/zk_attestation.js";
import { assertValidZkArtifact, readZkArtifact } from "./lib/zk_artifact.js";
import { loadTradeProfile, parseProfileArg } from "./lib/trade_profile.js";

function parseArg(flag) {
  const idx = process.argv.indexOf(flag);
  return idx !== -1 ? process.argv[idx + 1] : null;
}
function hasArg(flag) {
  return process.argv.includes(flag);
}

const PROFILE = parseProfileArg();
const PROFILE_CONFIG = PROFILE ? loadTradeProfile(PROFILE) : null;
const MAIN_WS = parseArg("--main") || PROFILE_CONFIG?.main_ws || "ws://127.0.0.1:9944";
const CHILD_WS = parseArg("--child") || PROFILE_CONFIG?.child_ws || "ws://127.0.0.1:9950";
const ZK_CMD = process.env.ZK_VERIFIER_CMD || PROFILE_CONFIG?.zk_verifier_cmd || "target/tools/fishbone-zk";
const BUSINESS_WITNESS = parseArg("--business-witness");
const DATASET = parseArg("--dataset");
const REQUEST = parseArg("--request");
const EVIDENCE_OUT = parseArg("--evidence-out");
const VERBOSE = hasArg("--verbose");
const DRY_RUN_DYNAMIC = hasArg("--dry-run-dynamic");

// ── Mode selection ──
const HAS_DYNAMIC = DATASET !== null && REQUEST !== null;
const HAS_PARTIAL = (DATASET !== null) !== (REQUEST !== null);
const HAS_EXPLICIT_WITNESS = BUSINESS_WITNESS !== null;

if (HAS_DYNAMIC && HAS_EXPLICIT_WITNESS) {
  console.error("ambiguous: --dataset/--request and --business-witness cannot be used together");
  process.exit(2);
}
if (HAS_PARTIAL) {
  console.error("--dataset and --request must both be provided for dynamic mode");
  process.exit(2);
}
const DYNAMIC_MODE = HAS_DYNAMIC;
const WITNESS_PATH = HAS_EXPLICIT_WITNESS
  ? BUSINESS_WITNESS
  : (PROFILE_CONFIG?.business_witness || "scripts/fixtures/data_trade_business_sample.json");

// ── Read request in dynamic mode ──
let DYNAMIC_REQUEST = null;
if (DYNAMIC_MODE) {
  try {
    DYNAMIC_REQUEST = JSON.parse(readFileSync(REQUEST, "utf8"));
    if (!DYNAMIC_REQUEST.request_hash || typeof DYNAMIC_REQUEST.request_hash !== "string") {
      console.error("request JSON missing request_hash");
      process.exit(2);
    }
  } catch (e) {
    console.error(`failed to read request JSON: ${e.message}`);
    process.exit(2);
  }
}

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

// ── Evidence accumulator ──
const evidence = {
  version: 1,
  mode: DYNAMIC_MODE ? "dynamic" : "legacy-witness",
  profile: PROFILE || null,
  main_ws: MAIN_WS,
  child_ws: CHILD_WS,
  dataset_path: DATASET || null,
  request_path: REQUEST || null,
  request_hash: null,
  listing_id: null,
  escrow_id: null,
  session_id: null,
  rounds: [],
  settlement: null,
  result: null,
};
if (DRY_RUN_DYNAMIC) {
  evidence.mode = "dynamic-dry-run";
}

async function main() {
  log(`连接主链: ${MAIN_WS}  子链: ${CHILD_WS}`);
  log(`ZK CLI: ${ZK_CMD}`);

  if (DRY_RUN_DYNAMIC) {
    log("dry-run mode: skipping chain connection");
    await dryRunDynamic();
    return;
  }

  // ── Dynamic live-mode preflight ──
  if (DYNAMIC_MODE) {
    log("preflight: validating dynamic dataset/request before chain connection...");
    const preflightDir = `target/data-trade-zk/preflight-${Date.now()}`;
    mkdirSync(preflightDir, { recursive: true });
    const preflightWitness = `${preflightDir}/witness.json`;
    const mwResult = spawnSync(ZK_CMD, [
      "make-witness", "--dataset", DATASET, "--request", REQUEST,
      "--out", preflightWitness,
      "--session-id", "0", "--round-index", "0",
    ], { stdio: "inherit" });
    if (mwResult.status !== 0) {
      console.error("dynamic request validation failed before chain connection");
      process.exit(1);
    }
    log("preflight passed");
  }

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
  const requestHash = DYNAMIC_MODE ? DYNAMIC_REQUEST.request_hash : sample.requestHash;
  const maxRounds = 3;
  const pricePerRound = 100;
  const depositHint = 500;
  const seed = "zk-real-data-trade-secret";
  const hashChainAnchor = hashNTimes(seed, maxRounds);
  log(`hash_chain_anchor (H^${maxRounds}(seed)): ${hashChainAnchor}`);
  if (DYNAMIC_MODE) {
    log(`mode=dynamic dataset=${DATASET} request=${REQUEST}`);
    log(`dynamic request_hash=${requestHash}`);
  }

  try {
    // ── Setup: publish, escrow, lock, session ──
    const publishResult = await submitTx(bob, childApi.tx.dataRegistry.publishData(
      sample.imtRoot, sample.description, pricePerRound, maxRounds, depositHint,
      requestHash, sample.proofParamsHash,
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
      listingId, escrowId, bob.address, requestHash,
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

      // 0. In dynamic mode, generate witness before fixture
      if (DYNAMIC_MODE) {
        const witnessPath = `${outDir}/witness.json`;
        const mwResult = spawnSync(ZK_CMD, [
          "make-witness", "--dataset", DATASET, "--request", REQUEST,
          "--out", witnessPath,
          "--session-id", String(sessionId), "--round-index", String(round),
        ], { stdio: "inherit" });
        if (mwResult.status !== 0) throw new Error(`fishbone-zk make-witness failed: ${mwResult.status}`);
        if (VERBOSE) log(`  witness=${witnessPath}`);
      }
      const roundWitnessPath = DYNAMIC_MODE ? `${outDir}/witness.json` : WITNESS_PATH;

      // 1. Generate fixture via fishbone-zk CLI
      const fixResult = spawnSync(ZK_CMD, [
        "business-fixture", "--witness", roundWitnessPath, "--out", outDir,
        "--request-hash", requestHash,
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

      // Record round evidence
      evidence.rounds.push({
        round_index: round,
        witness_path: roundWitnessPath,
        artifact_path: artifactPath,
        proof_digest: artifact.proof_digest,
        business_input_hash: artifact.business_input_hash,
        public_input_hash: artifact.public_input_hash,
      });
    }

    // ── Settlement ──
    evidence.request_hash = requestHash;
    evidence.listing_id = listingId;
    evidence.escrow_id = escrowId;
    evidence.session_id = sessionId;

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

    evidence.settlement = { completed_rounds: maxRounds - remainingRounds, remaining_rounds: remainingRounds };
    evidence.result = "accepted";
    writeEvidence();
  } finally {
    if (evidence.result === null) evidence.result = "failed";
    writeEvidence();
    await Promise.all([mainApi.disconnect(), childApi.disconnect()]);
  }
}

async function dryRunDynamic() {
  if (!DYNAMIC_MODE) {
    console.error("--dry-run-dynamic requires --dataset and --request");
    process.exit(2);
  }
  const requestHash = DYNAMIC_REQUEST.request_hash;
  const sessionId = 0;
  const round = 0;
  const outDir = `target/data-trade-zk/session-${sessionId}-round-${round}`;
  mkdirSync(outDir, { recursive: true });

  log(`dry-run: make-witness...`);
  const witnessPath = `${outDir}/witness.json`;
  const mwResult = spawnSync(ZK_CMD, [
    "make-witness", "--dataset", DATASET, "--request", REQUEST,
    "--out", witnessPath,
    "--session-id", String(sessionId), "--round-index", String(round),
  ], { stdio: "inherit" });
  if (mwResult.status !== 0) throw new Error(`make-witness failed: ${mwResult.status}`);

  log(`dry-run: business-fixture...`);
  const fixResult = spawnSync(ZK_CMD, [
    "business-fixture", "--witness", witnessPath, "--out", outDir,
    "--request-hash", requestHash,
    "--session-id", String(sessionId), "--round-index", String(round),
  ], { stdio: "inherit" });
  if (fixResult.status !== 0) throw new Error(`business-fixture failed: ${fixResult.status}`);

  const artifactPath = `${outDir}/artifact.json`;
  const artifact = assertValidZkArtifact(readZkArtifact(artifactPath));

  log(`dry-run: verify...`);
  const verifyResult = spawnSync(ZK_CMD, ["verify", "--artifact", artifactPath], { encoding: "utf8" });
  if (verifyResult.status !== 0 || !verifierAccepted(verifyResult.stdout)) {
    throw new Error(`verify rejected: ${verifyResult.stderr || verifyResult.stdout}`);
  }

  evidence.request_hash = requestHash;
  evidence.session_id = sessionId;
  evidence.rounds.push({
    round_index: round,
    witness_path: witnessPath,
    artifact_path: artifactPath,
    proof_digest: artifact.proof_digest,
    business_input_hash: artifact.business_input_hash,
    public_input_hash: artifact.public_input_hash,
  });
  evidence.result = "dry-run-accepted";
  writeEvidence();

  log(`proof_digest=${artifact.proof_digest}`);
  log(`business_input_hash=${artifact.business_input_hash}`);
  log("✅ dynamic dry-run 完成！");
}

function writeEvidence() {
  const outPath = EVIDENCE_OUT || `target/data-trade-zk/session-${evidence.session_id || 0}-evidence.json`;
  try {
    const dir = outPath.substring(0, outPath.lastIndexOf("/"));
    if (dir) mkdirSync(dir, { recursive: true });
    writeFileSync(outPath, JSON.stringify(evidence, null, 2) + "\n");
    if (VERBOSE) log(`evidence written: ${outPath}`);
  } catch (e) {
    log(`WARNING: failed to write evidence: ${e.message}`);
  }
}

main().catch((e) => {
  console.error("[zk-real-e2e 致命错误]", e.message);
  console.error(e.stack);
  process.exit(1);
});
