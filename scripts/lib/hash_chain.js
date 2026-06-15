/**
 * FishboneChain hash-chain utilities for data trade.
 *
 * Implements paper's H^(n)(s) hash chain: starting from seed s,
 * each round reveals H^(k)(s) as the preimage for round k.
 */
import { u8aToHex } from "@polkadot/util";
import { blake2AsU8a } from "@polkadot/util-crypto";

/**
 * Hash a value once with Blake2-256.
 * @param {string | Uint8Array} value
 * @returns {string} hex-encoded hash
 */
export function hashOnce(value) {
  return u8aToHex(blake2AsU8a(value));
}

/**
 * Hash a seed exactly `rounds` times, matching the runtime verifier:
 * first hash raw seed bytes, then hash the raw 32-byte H256 result.
 * H^(rounds)(seed).
 * @param {string | Uint8Array} seed
 * @param {number} rounds
 * @returns {string} hex-encoded hash
 */
export function hashNTimes(seed, rounds) {
  if (rounds <= 0) {
    throw new Error("rounds must be positive");
  }
  let current = typeof seed === "string" ? new TextEncoder().encode(seed) : seed;
  for (let i = 0; i < rounds; i += 1) {
    current = blake2AsU8a(current);
  }
  return u8aToHex(current);
}

/**
 * Generate the payment preimage for remaining rounds.
 * If n total rounds and k completed, remaining = n - k.
 * The preimage is H^(remaining)(seed) — matches the hash chain anchor
 * after `paid_rounds` hashes from the anchor direction.
 *
 * For the Substrate side:
 *   anchor = H^(n)(seed)
 *   after k rounds: preimage = H^(n-k)(seed)
 *   verify: H^(k)(preimage) == anchor → k rounds paid
 *
 * @param {string | Uint8Array} seed
 * @param {number} remainingRounds — n - k (rounds remaining)
 * @returns {string} hex-encoded preimage
 */
export function paymentPreimageForRemaining(seed, remainingRounds) {
  return hashNTimes(seed, remainingRounds);
}
