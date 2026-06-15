/**
 * FishboneChain off-chain ZK verifier client.
 *
 * Calls an external verifier command and returns accepted/rejected.
 * Phase 1: without ZK_VERIFIER_CMD, runs in dev-always-accept mode.
 */
import { spawnSync } from "node:child_process";

export function verifyProofOffchain({ command, proofHash, publicInputHash }) {
  if (!command) {
    return { accepted: true, mode: "dev-always-accept" };
  }

  const result = spawnSync(command, [proofHash, publicInputHash], {
    encoding: "utf8",
    shell: true,
  });

  if (result.status !== 0) {
    throw new Error(`zk verifier command failed: status=${result.status} stderr=${result.stderr}`);
  }

  const output = result.stdout.trim();
  if (output === "accepted") return { accepted: true, mode: "external" };
  if (output === "rejected") return { accepted: false, mode: "external" };
  throw new Error(`zk verifier command must print accepted or rejected, got: ${output}`);
}
