/**
 * FishboneChain ZK attestation digest calculator.
 *
 * Computes the on-chain attestation payload digest for VerifierAuthority attestation.
 * Matches Go artifact.AttestDomain convention.
 */
import { blake2AsHex } from "@polkadot/util-crypto";
import { hexToU8a, stringToU8a, u8aConcat } from "@polkadot/util";

function le32(n) {
  const b = new Uint8Array(4);
  new DataView(b.buffer).setUint32(0, n, true);
  return b;
}

/**
 * Compute ZK attestation payload digest.
 * verifierAccount must be a Uint8Array (e.g. charlie.addressRaw = 32 bytes).
 */
export function computeZkAttestationDigest({ sessionId, roundIndex, proofDigest, accepted, verifierAccount }) {
  return blake2AsHex(u8aConcat(
    stringToU8a("FISHBONE:DATA_TRADE:ZK_ATTEST:v1"),
    le32(sessionId),
    le32(roundIndex),
    hexToU8a(proofDigest),
    new Uint8Array([accepted ? 1 : 0]),
    verifierAccount,
  ));
}
