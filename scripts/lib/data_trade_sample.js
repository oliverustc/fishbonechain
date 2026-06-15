/**
 * FishboneChain data trade sample data generators.
 *
 * Produces mock IMT roots, request hashes, and proof bundles for E2E testing.
 */
import { blake2AsHex } from "@polkadot/util-crypto";

/**
 * Generate a sample data listing matching the child6 data-trade scenario.
 * @returns {{ description: string, rows: object[], imtRoot: string, requestHash: string, proofParamsHash: string, dataHash: string }}
 */
export function sampleListing() {
  const description = "vehicle telemetry: time,power,battery_temp,location(maskable)";
  const rows = [
    { time: 20230101, power: 42, battery_temp: 31, location: "31.2304,121.4737" },
    { time: 20230102, power: 45, battery_temp: 33, location: "31.2305,121.4738" },
  ];
  const encoded = JSON.stringify(rows);
  return {
    description,
    rows,
    imtRoot: blake2AsHex(encoded),
    requestHash: blake2AsHex("range:time=2023;mask:location=city"),
    proofParamsHash: blake2AsHex("mock-proof-v1"),
    dataHash: blake2AsHex(encoded),
  };
}

/**
 * Generate a mock proof bundle for a given round.
 * Phase 1: dummy hashes for dev testing.
 */
export function sampleProofBundle(round) {
  return {
    constraintKind: "Range",
    chProofHash: `0x${String(round + 11).padStart(64, "0")}`,
    roProofHash: `0x${String(round + 21).padStart(64, "0")}`,
    publicInputHash: `0x${String(round + 31).padStart(64, "0")}`,
  };
}
