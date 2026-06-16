/**
 * FishboneChain off-chain ZK verifier client.
 *
 * Modes:
 * 1. dev-always-accept: no ZK_VERIFIER_CMD set
 * 2. artifact mode: ZK_VERIFIER_CMD set, artifactPath provided
 *    Calls CLI: `<cmd> verify --artifact <path>`, expects stdout "accepted" or exit != 0
 * 3. legacy mode: ZK_VERIFIER_CMD set, proofHash + publicInputHash provided
 *    Calls CLI: `<cmd> <proofHash> <publicInputHash>`, expects stdout "accepted" or "rejected"
 */
import { spawnSync } from "node:child_process";
import { assertValidZkArtifact, readZkArtifact } from "./zk_artifact.js";

export function verifyProofOffchain({ command, proofHash, publicInputHash, artifactPath }) {
  if (!command) {
    return { accepted: true, mode: "dev-always-accept" };
  }

  if (artifactPath) {
    // Artifact-based verification (Task 3)
    const artifact = assertValidZkArtifact(readZkArtifact(artifactPath));
    const result = spawnSync(command, ["verify", "--artifact", artifactPath], {
      encoding: "utf8",
      shell: false,
    });
    if (result.status === 0 && result.stdout.trim() === "accepted") {
      return { accepted: true, mode: "external", artifact };
    }
    return {
      accepted: false,
      mode: "external",
      artifact,
      error: result.stderr || result.stdout || `exit code ${result.status}`,
    };
  }

  // Legacy proofHash-based verification
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
