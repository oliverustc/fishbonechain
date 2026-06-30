/**
 * FishboneChain Data Trade CLI Boundary Entrypoint
 *
 * Stage 16: standardized command boundary for data-trade operations.
 * This is a CLI/API boundary definition, NOT a backend/server implementation.
 *
 * Usage:
 *   node scripts/data_trade_cli.js <subcommand> [options]
 *
 * Implemented subcommands (no-live-chain):
 *   inspect        profile inspection or evidence summary
 *   generate-proof off-chain ZK proof pipeline (dry-run)
 *   run-flow       compatibility wrapper for full E2E flow
 *
 * Planned subcommands (boundary definition only, not independently executable):
 *   publish-listing create-request create-escrow open-session
 *   submit-delivery settle dispute
 */

import { spawnSync } from "node:child_process";
import { readFileSync, writeFileSync, mkdirSync } from "node:fs";

const ROOT = import.meta.dirname + "/..";
const FLOW_SCRIPT = `${ROOT}/scripts/zk_real_data_trade_flow.js`;
const PROFILES_PATH = `${ROOT}/scripts/profiles/chains.json`;

function parseArg(flag) {
  const idx = process.argv.indexOf(flag);
  return idx !== -1 ? process.argv[idx + 1] : null;
}

function hasArg(flag) {
  return process.argv.includes(flag);
}

// ── Subcommand dispatch ──

const SUBCOMMANDS = new Set([
  "publish-listing", "create-request", "create-escrow", "open-session",
  "generate-proof", "submit-delivery", "settle", "dispute",
  "inspect", "run-flow",
]);

const PLANNED_ONLY = new Set([
  "publish-listing", "create-request", "create-escrow", "open-session",
  "submit-delivery", "settle", "dispute",
]);

function printTopHelp() {
  console.log(`FishboneChain Data Trade CLI (Stage 16)
  https://docs/fishbonechain

Usage:
  node scripts/data_trade_cli.js <subcommand> [options]

Subcommands:
  publish-listing   [planned]  DO publishes a data listing
  create-request    [planned]  Create an off-chain data request
  create-escrow     [planned]  DR opens main-chain escrow
  open-session      [planned]  DR/DO open a trade session
  generate-proof    [no-chain] Generate and verify ZK proof off-chain
  submit-delivery   [planned]  Submit per-round proof and delivery
  settle            [planned]  DO claims settlement on child + main
  dispute           [planned]  DR disputes invalid proof or plaintext
  inspect           [no-chain] Query profile config or evidence data
  run-flow          [wrapper]  Run full E2E flow (compatible with zk_real_data_trade_flow.js)

Chain-mutating subcommands marked [planned] expose the API boundary but are not
independently executable in Stage 16. Use run-flow for full-flow transaction execution.

Use node scripts/data_trade_cli.js <subcommand> --help for subcommand options.
`);
}

function printPlannedHelp(name, desc) {
  console.log(`fishbonechain data-trade-cli ${name} [planned]

  ${desc}

Status: PLANNED — boundary definition only, not independently executable in Stage 16.

This subcommand documents the API boundary for future backend integration and
independent execution. Stage 16 does NOT submit independent transactions through
this subcommand because:

  - Escrow/session binding checks (assertEscrowMatchesTradeTerms,
    assertSessionMatchesListingAndEscrow) require full-flow context.
  - State handoff between separate invocations is not yet designed.

Transaction-safe execution: use the run-flow subcommand, which preserves full
escrow/session binding through the existing zk_real_data_trade_flow.js E2E flow.

Required signer safeguards for future implementation:
  - All chain transactions require user/dev signer keys (e.g., //Alice, //Bob, //Charlie).
  - The CLI boundary does not make a backend trusted.
  - No private key custody or signing on behalf of users.
`);
}

function planned(name, desc) {
  if (hasArg("--help") || hasArg("-h")) {
    printPlannedHelp(name, desc);
    process.exit(0);
  }
  console.error(`error: '${name}' is planned / not independently executable in Stage 16`);
  console.error("Use run-flow for full-flow execution or --help for boundary documentation.");
  process.exit(1);
}

// ── inspect ──

function inspectHelp() {
  console.log(`fishbonechain data-trade-cli inspect [no-chain]

Query profile configuration or evidence data without chain connection.

Subcommands:
  inspect profile    Load and display a trade profile from chains.json
  inspect evidence   Read evidence JSON and write a structured summary

Usage:
  node scripts/data_trade_cli.js inspect profile --profile <id> --out <path>
  node scripts/data_trade_cli.js inspect evidence --evidence <path> --out <path>
`);
}

function inspectProfile() {
  const profileId = parseArg("--profile");
  const outPath = parseArg("--out");

  if (!profileId) {
    console.error("error: --profile <id> is required");
    process.exit(2);
  }
  if (!outPath) {
    console.error("error: --out <path> is required");
    process.exit(2);
  }

  const raw = JSON.parse(readFileSync(PROFILES_PATH, "utf8"));
  const profiles = raw.trade_profiles;
  if (!profiles || typeof profiles !== "object") {
    console.error("error: missing trade_profiles in chains.json");
    process.exit(1);
  }

  const p = profiles[profileId];
  if (!p) {
    console.error(`error: unknown trade profile: ${profileId}`);
    console.error(`available: ${Object.keys(profiles).join(", ")}`);
    process.exit(1);
  }

  const output = {
    version: 1,
    command: "inspect profile",
    profile_id: profileId,
    profile: p,
    all_profile_ids: Object.keys(profiles),
  };

  const dir = outPath.substring(0, outPath.lastIndexOf("/"));
  if (dir) mkdirSync(dir, { recursive: true });
  writeFileSync(outPath, JSON.stringify(output, null, 2) + "\n");
  console.log(`profile=${profileId} written to ${outPath}`);
}

function inspectEvidence() {
  const evidencePath = parseArg("--evidence");
  const outPath = parseArg("--out");

  if (!evidencePath) {
    console.error("error: --evidence <path> is required");
    process.exit(2);
  }
  if (!outPath) {
    console.error("error: --out <path> is required");
    process.exit(2);
  }

  let raw;
  try {
    raw = JSON.parse(readFileSync(evidencePath, "utf8"));
  } catch (e) {
    console.error(`error: failed to read evidence file: ${e.message}`);
    process.exit(1);
  }

  const summary = {
    version: raw.version || 1,
    command: "inspect evidence",
    source: evidencePath,
    scenario: raw.scenario || null,
    mode: raw.mode || null,
    profile: raw.profile || null,
    result: raw.result || null,
    request_hash: raw.request_hash || null,
    listing_id: raw.listing_id ?? null,
    escrow_id: raw.escrow_id ?? null,
    session_id: raw.session_id ?? null,
    settlement: raw.settlement || null,
    scenario_outcome: raw.scenario_outcome || null,
    round_count: Array.isArray(raw.rounds) ? raw.rounds.length : 0,
    constraints: [],
    errors: raw.error ? [raw.error] : [],
  };

  if (Array.isArray(raw.rounds)) {
    for (const r of raw.rounds) {
      if (Array.isArray(r.constraints)) {
        for (const c of r.constraints) {
          summary.constraints.push({
            round_index: r.round_index,
            field_name: c.field_name || null,
            proof_digest: c.proof_digest || null,
            business_input_hash: c.business_input_hash || null,
            on_chain_bound: c.on_chain_bound ?? null,
          });
        }
      }
    }
  }

  const dir = outPath.substring(0, outPath.lastIndexOf("/"));
  if (dir) mkdirSync(dir, { recursive: true });
  writeFileSync(outPath, JSON.stringify(summary, null, 2) + "\n");
  console.log(`evidence summary written to ${outPath}`);
  console.log(`  scenario: ${summary.scenario || "N/A"}`);
  console.log(`  result:   ${summary.result || "N/A"}`);
  console.log(`  rounds:   ${summary.round_count}`);
  console.log(`  constraints: ${summary.constraints.length}`);
}

function inspectDispatch() {
  const sub = process.argv[3];
  if (hasArg("--help") || hasArg("-h") || sub === "--help" || sub === "-h") {
    inspectHelp();
    process.exit(0);
  }
  if (sub === "profile") {
    inspectProfile();
  } else if (sub === "evidence") {
    inspectEvidence();
  } else {
    console.error(`error: unknown inspect subcommand: ${sub || "<none>"}`);
    console.error("available: profile, evidence");
    inspectHelp();
    process.exit(2);
  }
}

// ── generate-proof ──

function generateProofHelp() {
  console.log(`fishbonechain data-trade-cli generate-proof [no-chain]

Generate and verify off-chain ZK proof using the existing gnark pipeline.
Delegates to zk_real_data_trade_flow.js --dry-run-dynamic.

Usage:
  node scripts/data_trade_cli.js generate-proof \\
    --profile <id> \\
    --dataset <path> \\
    --request <path> \\
    --evidence-out <path>

Options:
  --profile       Trade profile ID (e.g., child6-data-trade)
  --dataset       Path to dataset JSON fixture
  --request       Path to request JSON fixture
  --evidence-out  Path for evidence JSON output
  --verbose       Verbose output
  -h, --help      Show this help

Environment:
  ZK_VERIFIER_CMD   Path to fishbone-zk binary (defaults to profile setting)

Evidence output fields: proof_digest, business_input_hash, public_input_hash,
constraints[], mode="dynamic-dry-run", result="dry-run-accepted".
`);
}

function generateProof() {
  if (hasArg("--help") || hasArg("-h")) {
    generateProofHelp();
    process.exit(0);
  }

  const profile = parseArg("--profile");
  const dataset = parseArg("--dataset");
  const request = parseArg("--request");
  const evidenceOut = parseArg("--evidence-out");
  const verbose = hasArg("--verbose");

  if (!profile) {
    console.error("error: --profile <id> is required");
    process.exit(2);
  }
  if (!dataset || !request) {
    console.error("error: --dataset and --request are required");
    process.exit(2);
  }
  if (!evidenceOut) {
    console.error("error: --evidence-out <path> is required");
    process.exit(2);
  }

  const args = [
    FLOW_SCRIPT,
    "--profile", profile,
    "--dataset", dataset,
    "--request", request,
    "--evidence-out", evidenceOut,
    "--dry-run-dynamic",
  ];
  if (verbose) args.push("--verbose");

  console.log(`[generate-proof] delegating to zk_real_data_trade_flow.js --dry-run-dynamic`);
  console.log(`  profile: ${profile}`);
  console.log(`  dataset: ${dataset}`);
  console.log(`  request: ${request}`);

  const result = spawnSync("node", args, { stdio: "inherit" });
  if (result.status !== 0) {
    console.error(`generate-proof failed with exit code ${result.status}`);
    process.exit(result.status || 1);
  }
}

// ── run-flow ──

function runFlowHelp() {
  console.log(`fishbonechain data-trade-cli run-flow [wrapper]

Compatibility wrapper around zk_real_data_trade_flow.js.
Preserves the full E2E flow for dry-run and live-chain execution.

Usage:
  node scripts/data_trade_cli.js run-flow [options]

Options:
  --profile          Trade profile ID
  --main             Main chain RPC URL
  --child            Child chain RPC URL
  --business-witness Path to witness JSON
  --dataset          Path to dataset JSON fixture
  --request          Path to request JSON fixture
  --scenario         happy | invalid-proof-dispute | invalid-plaintext-dispute | requester-refuses-payment
  --evidence-out     Path for evidence JSON output
  --dry-run-dynamic  Run proof pipeline without chain connection
  --verbose          Verbose output
  -h, --help         Show this help

Environment:
  ZK_VERIFIER_CMD   Path to fishbone-zk binary

Examples:
  # Dry-run proof validation (no chain connection)
  data_trade_cli.js run-flow --profile child6-data-trade --dataset <ds> --request <req> --evidence-out <path> --dry-run-dynamic

  # Live-chain happy path
  data_trade_cli.js run-flow --profile child6-data-trade --main ws://... --child ws://... --dataset <ds> --request <req>
`);
}

function runFlow() {
  if (hasArg("--help") || hasArg("-h")) {
    runFlowHelp();
    process.exit(0);
  }

  const forwardedFlags = [
    "--profile", "--main", "--child", "--business-witness",
    "--dataset", "--request", "--scenario", "--evidence-out",
  ];
  const forwardedBoolFlags = ["--dry-run-dynamic", "--verbose"];

  const args = [FLOW_SCRIPT];
  for (const flag of forwardedFlags) {
    const val = parseArg(flag);
    if (val !== null) {
      args.push(flag, val);
    }
  }
  for (const flag of forwardedBoolFlags) {
    if (hasArg(flag)) {
      args.push(flag);
    }
  }

  console.log(`[run-flow] delegating to zk_real_data_trade_flow.js`);

  const result = spawnSync("node", args, { stdio: "inherit" });
  if (result.status !== 0) {
    console.error(`run-flow failed with exit code ${result.status}`);
    process.exit(result.status || 1);
  }
}

// ── create-request help ──

function createRequestHelp() {
  printPlannedHelp("create-request",
    "Create an off-chain data request with request_hash binding.");
}

function createRequestDispatch() {
  if (hasArg("--help") || hasArg("-h")) {
    createRequestHelp();
    process.exit(0);
  }
  planned("create-request", "Create an off-chain data request");
}

// ── Main ──

function main() {
  const subcommand = process.argv[2];

  if (!subcommand || subcommand === "--help" || subcommand === "-h") {
    printTopHelp();
    process.exit(0);
  }

  if (!SUBCOMMANDS.has(subcommand)) {
    console.error(`error: unknown subcommand: ${subcommand}`);
    console.error(`available: ${[...SUBCOMMANDS].sort().join(", ")}`);
    process.exit(2);
  }

  switch (subcommand) {
    case "inspect":
      return inspectDispatch();
    case "generate-proof":
      return generateProof();
    case "run-flow":
      return runFlow();
    case "publish-listing":
      return planned("publish-listing", "DO publishes a data listing on the child chain.");
    case "create-request":
      return createRequestDispatch();
    case "create-escrow":
      return planned("create-escrow", "DR opens a main-chain escrow with funds and deposit locks.");
    case "open-session":
      return planned("open-session", "DR creates a trade session on the child chain; DO accepts.");
    case "submit-delivery":
      return planned("submit-delivery", "DR/DO/Verifier execute per-round proof submission and delivery.");
    case "settle":
      return planned("settle", "DO claims settlement on child and main chain.");
    case "dispute":
      return planned("dispute", "DR disputes invalid proof or plaintext; triggers punishment path.");
    default:
      console.error(`internal error: unhandled subcommand: ${subcommand}`);
      process.exit(2);
  }
}

main();
