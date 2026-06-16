/**
 * FishboneChain ZK artifact reader and digest calculator.
 *
 * Reads gnark proof artifact JSON, computes and validates proof_digest,
 * matching the Go implementation in tools/data-trade-zk/internal/artifact/schema.go.
 */
import fs from "node:fs";
import { blake2AsHex } from "@polkadot/util-crypto";
import { hexToU8a, u8aConcat, stringToU8a } from "@polkadot/util";

function le32(n) {
  const b = new Uint8Array(4);
  new DataView(b.buffer).setUint32(0, n, true);
  return b;
}

export function readZkArtifact(path) {
  return JSON.parse(fs.readFileSync(path, "utf8"));
}

/**
 * Compute proof_digest from artifact fields.
 * Matches Go Blake2_256(domain || proofSystemCode(u8) || constraintKindCode(u8) || ...)
 */
export function computeProofDigest(a) {
  return blake2AsHex(u8aConcat(
    stringToU8a("FISHBONE:DATA_TRADE:ZK_PROOF:v1"),
    new Uint8Array([a.proof_system_code]),
    new Uint8Array([a.constraint_kind_code]),
    le32(a.ro_depth),
    hexToU8a(a.request_hash),
    le32(a.session_id),
    le32(a.round_index),
    hexToU8a(a.vk_hash),
    hexToU8a(a.ch_proof_hash),
    hexToU8a(a.ro_proof_hash),
    hexToU8a(a.public_input_hash),
    hexToU8a(a.business_input_hash),
  ));
}

export function assertValidZkArtifact(a) {
  const digest = computeProofDigest(a);
  if (digest !== a.proof_digest) {
    throw new Error(`proof_digest mismatch: expected ${digest}, got ${a.proof_digest}`);
  }
  if (a.version !== 1) throw new Error("unsupported artifact version");
  if (a.proof_system !== "gnark-groth16-bn254") throw new Error("unsupported proof_system");
  if (a.proof_system_code !== 1) throw new Error("proof_system_code must be 1");
  const kindCodes = { range: 1, subset: 2, substr: 3 };
  if (kindCodes[a.constraint_kind] !== a.constraint_kind_code) {
    throw new Error(`constraint_kind ${a.constraint_kind} does not match code ${a.constraint_kind_code}`);
  }
  if (!/^0x[0-9a-fA-F]{64}$/.test(a.business_input_hash)) {
    throw new Error(`invalid business_input_hash: ${a.business_input_hash}`);
  }
  return a;
}
