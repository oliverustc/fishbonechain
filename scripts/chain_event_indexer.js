#!/usr/bin/env node

/**
 * FishboneChain Event Indexer — Stage 17
 *
 * File-backed chain event indexer and state-sync foundation.
 * Normalizes chain events, maintains resumable cursors, derives
 * data-trade state snapshots, and correlates evidence with events.
 *
 * Usage:
 *   node scripts/chain_event_indexer.js <subcommand> [options]
 *
 * Subcommands:
 *   scan               scan chains for events
 *   replay             rebuild state and summary from events.jsonl
 *   state              derive state from events.jsonl
 *   correlate-evidence correlate evidence with indexed events
 *   inspect            inspect mappings, cursors, state, sample-evidence
 */

import { parseArgs } from "node:util";
import { readFileSync, writeFileSync, existsSync, mkdirSync, statSync } from "node:fs";
import { join } from "node:path";
import {
  BASELINE_EVENTS,
  isBaselineEvent,
  normalizeEvent,
  normalizeEventFields,
  makeCursor,
  parseCursor,
  readJsonlFile,
  writeJsonlFile,
  appendJsonlRecord,
} from "./lib/chain_event_normalizer.js";
import { deriveState, generateStateSummary, generateMarkdownSummary } from "./lib/data_trade_state_sync.js";
import { correlate } from "./lib/data_trade_evidence_correlation.js";
import { loadTradeProfile } from "./lib/trade_profile.js";

function ensureDir(dir) {
  mkdirSync(dir, { recursive: true });
}

function writeJson(path, obj) {
  ensureDir(join(path, ".."));
  writeFileSync(path, JSON.stringify(obj, null, 2) + "\n", "utf8");
}

function parseArg(argv, flag) {
  const idx = argv.indexOf(flag);
  return idx !== -1 ? argv[idx + 1] : null;
}

function cliPositional(args) {
  return args.positionals;
}

function printHelp(subcommand) {
  if (subcommand === "scan") {
    console.log(`Usage: node scripts/chain_event_indexer.js scan [options]

  Scan one or both chains over a bounded block range and write normalized
  chain events to <out>/events.jsonl.

Options:
  --profile <id>      Load main_ws and child_ws from scripts/profiles/chains.json
  --main <ws>         Override main chain RPC endpoint
  --child <ws>        Override child chain RPC endpoint
  --chain <role>      main | child | both (default: both)
  --from <block>      Starting block number (required)
  --to <block>        Ending block number (required)
  --max-blocks <n>    Maximum blocks to scan (stops early)
  --include-all       Record all events, not just baseline data-trade events
  --out <dir>         Output directory (required)`);
  } else if (subcommand === "replay") {
    console.log(`Usage: node scripts/chain_event_indexer.js replay [options]

  Rebuild derived state, summary.json, and summary.md from stored
  events.jsonl without RPC.

Options:
  --events <path>     Input events.jsonl file (required)
  --out <dir>         Output directory (required)`);
  } else if (subcommand === "state") {
    console.log(`Usage: node scripts/chain_event_indexer.js state [options]

  Derive data-trade state snapshots (listings/sessions/escrows)
  from normalized events.

Options:
  --events <path>     Input events.jsonl file (required)
  --out <dir>         Output directory (required)`);
  } else if (subcommand === "correlate-evidence") {
    console.log(`Usage: node scripts/chain_event_indexer.js correlate-evidence [options]

  Correlate evidence JSON files with indexed chain events by listing_id,
  session_id, escrow_id, and event names.

Options:
  --events <path>     Input events.jsonl file (required)
  --evidence <path>   Path to evidence JSON file or summary (required)
  --out <dir>         Output directory (required)`);
  } else if (subcommand === "inspect") {
    console.log(`Usage: node scripts/chain_event_indexer.js inspect <mode> [options]

  Inspect indexer data without RPC.

Modes:
  mappings           Write supported event mappings as JSON
  cursor             Print cursor summary from cursor.json
  counts             Print event/state counts from generated files
  sample-evidence    Create a sample dry-run evidence JSON for correlation testing

  Options for mappings:
    --out <path>     Output JSON path (required)

  Options for cursor:
    --cursor <path>  Path to cursor.json (required)

  Options for counts:
    --events <path>  Path to events.jsonl

  Options for sample-evidence:
    --kind <type>    Evidence kind (dry-run, live-chain)
    --out <path>     Output JSON path (required)`);
  } else {
    console.log(`FishboneChain Event Indexer — Stage 17

Usage:
  node scripts/chain_event_indexer.js <subcommand> [options]

Subcommands:
  scan               Scan chains for events and write events.jsonl + cursor.json
  replay             Rebuild state, summary from events.jsonl without RPC
  state              Derive data-trade state snapshots from events.jsonl
  correlate-evidence Correlate evidence with indexed events
  inspect            Inspect mappings, cursors, state, sample-evidence

Run 'node scripts/chain_event_indexer.js <subcommand> --help' for per-subcommand help.`);
  }
}

function requireArg(args, flag) {
  const val = parseArg(args, flag);
  if (!val) throw new Error(`missing required option: ${flag}`);
  return val;
}

function optionalArg(args, flag) {
  return parseArg(args, flag);
}

// ── CLI Entry Point ──

const argv = process.argv.slice(2);
const subcommand = argv[0];
const rest = argv.slice(1);

async function main() {
  if (!subcommand || subcommand === "--help" || subcommand === "-h") {
    printHelp();
    return;
  }

  if (rest.includes("--help") || rest.includes("-h")) {
    printHelp(subcommand);
    return;
  }

  switch (subcommand) {
    case "scan":
      await cmdScan(rest);
      break;
    case "replay":
      cmdReplay(rest);
      break;
    case "state":
      cmdState(rest);
      break;
    case "correlate-evidence":
      cmdCorrelate(rest);
      break;
    case "inspect": {
      const mode = rest[0];
      const modeArgs = rest.slice(1);
      switch (mode) {
        case "mappings":
          cmdInspectMappings(modeArgs);
          break;
        case "cursor":
          cmdInspectCursor(modeArgs);
          break;
        case "counts":
          cmdInspectCounts(modeArgs);
          break;
        case "sample-evidence":
          cmdInspectSampleEvidence(modeArgs);
          break;
        default:
          console.error(`Unknown inspect mode: ${mode || "(none)"}`);
          console.error("Available modes: mappings, cursor, counts, sample-evidence");
          process.exit(1);
      }
      break;
    }
    default:
      console.error(`Unknown subcommand: ${subcommand}`);
      console.error("Available subcommands: scan, replay, state, correlate-evidence, inspect");
      printHelp();
      process.exit(1);
  }
}

// ── scan ──

async function cmdScan(args) {
  const outDir = requireArg(args, "--out");
  const profileId = optionalArg(args, "--profile");
  const mainWs = optionalArg(args, "--main");
  const childWs = optionalArg(args, "--child");
  const chainRole = optionalArg(args, "--chain") || "both";
  if (!["main", "child", "both"].includes(chainRole)) {
    console.error(`invalid --chain value: ${chainRole}`);
    console.error("expected: main | child | both");
    process.exit(1);
  }
  const fromBlock = Number(optionalArg(args, "--from"));
  const toBlock = Number(optionalArg(args, "--to"));
  const maxBlocks = optionalArg(args, "--max-blocks") ? Number(optionalArg(args, "--max-blocks")) : null;
  const includeAll = args.includes("--include-all");

  if (!fromBlock || !toBlock) {
    console.error("scan requires --from <block> and --to <block>");
    process.exit(1);
  }

  if (fromBlock > toBlock) {
    console.error("--from must be <= --to");
    process.exit(1);
  }

  let effectiveTo = toBlock;
  if (maxBlocks && fromBlock + maxBlocks - 1 < effectiveTo) {
    effectiveTo = fromBlock + maxBlocks - 1;
    console.log(`[indexer] --max-blocks ${maxBlocks}: reducing scan to [${fromBlock}, ${effectiveTo}]`);
  }

  let profile = null;
  if (profileId) {
    profile = loadTradeProfile(profileId);
    console.log(`[indexer] loaded profile ${profileId}`);
  }

  const mainEndpoint = mainWs || (profile ? profile.main_ws : null);
  const childEndpoint = childWs || (profile ? profile.child_ws : null);

  if ((chainRole === "main" || chainRole === "both") && !mainEndpoint) {
    console.error("main chain endpoint required for scan of main/both");
    process.exit(1);
  }
  if ((chainRole === "child" || chainRole === "both") && !childEndpoint) {
    console.error("child chain endpoint required for scan of child/both");
    process.exit(1);
  }

  ensureDir(outDir);

  const eventsPath = join(outDir, "events.jsonl");
  const cursorPath = join(outDir, "cursor.json");
  let cursorData = {};
  if (existsSync(cursorPath)) {
    cursorData = JSON.parse(readFileSync(cursorPath, "utf8"));
  }

  try {
    const { ApiPromise, WsProvider } = await import("@polkadot/api");
    const apis = {};

    try {
      if (chainRole === "main" || chainRole === "both") {
        console.log(`[indexer] connecting to main chain: ${mainEndpoint}`);
        apis.main = await ApiPromise.create({ provider: new WsProvider(mainEndpoint) });
      }
      if (chainRole === "child" || chainRole === "both") {
        console.log(`[indexer] connecting to child chain: ${childEndpoint}`);
        apis.child = await ApiPromise.create({ provider: new WsProvider(childEndpoint) });
      }

      await Promise.all([apis.main?.isReady, apis.child?.isReady].filter(Boolean));
      console.log("[indexer] all APIs ready");

      if (apis.main && (chainRole === "main" || chainRole === "both")) {
        await scanChain(apis.main, "main", fromBlock, effectiveTo, eventsPath, cursorData, {
          profile: profileId || "unknown",
          chainId: "main",
          includeAll,
        });
      }

      if (apis.child && (chainRole === "child" || chainRole === "both")) {
        const chainId = profile ? profile.chain : "child";
        await scanChain(apis.child, "child", fromBlock, effectiveTo, eventsPath, cursorData, {
          profile: profileId || "unknown",
          chainId,
          includeAll,
        });
      }
    } finally {
      for (const api of Object.values(apis)) {
        if (api) await api.disconnect();
      }
    }
  } catch (err) {
    console.error(`[indexer] scan error: ${err.message}`);
    process.exit(1);
  }

  writeJson(cursorPath, cursorData);
  console.log(`[indexer] cursor saved to ${cursorPath}`);
  console.log(`[indexer] events saved to ${eventsPath}`);
}

async function scanChain(api, chainRole, fromBlock, toBlock, eventsPath, cursorData, opts) {
  const { profile, chainId, includeAll } = opts;
  let eventCount = 0;

  for (let blockNum = fromBlock; blockNum <= toBlock; blockNum++) {
    try {
      const blockHash = await api.rpc.chain.getBlockHash(blockNum);
      if (!blockHash || blockHash.isEmpty) {
        console.log(`[indexer] ${chainRole} block ${blockNum}: no block hash (skipping)`);
        continue;
      }

      const apiAt = await api.at(blockHash);
      const events = await apiAt.query.system.events();

      for (let extIdx = 0; extIdx < events.length; extIdx++) {
        const record = events[extIdx];
        const pallet = String(record.event.section);
        const variant = String(record.event.method);

        if (!includeAll && !isBaselineEvent(pallet, variant)) continue;

        const normalized = {
          event_id: `evt-${chainRole}-${blockNum}-${extIdx}-${pallet}-${variant}`,
          chain_id: chainId,
          chain_role: chainRole,
          profile,
          block_number: blockNum,
          block_hash: blockHash.toHex(),
          extrinsic_index: record.phase?.isApplyExtrinsic ? Number(record.phase.asApplyExtrinsic) : null,
          event_index: extIdx,
          pallet,
          variant,
          fields: normalizeEventFields(record.event),
          cursor: makeCursor(chainRole, blockNum + 1),
          ingested_at: new Date().toISOString(),
        };

        appendJsonlRecord(eventsPath, normalized);
        eventCount++;
      }

      cursorData[chainRole] = makeCursor(chainRole, blockNum + 1);

      if (blockNum % 50 === 0) {
        console.log(`[indexer] ${chainRole} block ${blockNum}/${toBlock}: ${eventCount} events so far`);
      }
    } catch (err) {
      console.error(`[indexer] ${chainRole} block ${blockNum} error: ${err.message}`);
    }
  }

  console.log(`[indexer] ${chainRole} scan complete: ${fromBlock}-${toBlock}, ${eventCount} events`);
}

// ── replay ──

async function cmdReplay(args) {
  const eventsPath = requireArg(args, "--events");
  const outDir = requireArg(args, "--out");
  ensureDir(outDir);

  const events = await readJsonlFile(eventsPath);
  console.log(`[replay] read ${events.length} events from ${eventsPath}`);

  writeJsonlFile(join(outDir, "events.jsonl"), events);

  const state = deriveState(events);
  writeJson(join(outDir, "state.json"), state);

  const summary = generateStateSummary(state);
  writeJson(join(outDir, "summary.json"), summary);

  const md = generateMarkdownSummary(summary);
  writeFileSync(join(outDir, "summary.md"), md, "utf8");

  console.log(`[replay] state, summary written to ${outDir}`);
}

// ── state ──

async function cmdState(args) {
  const eventsPath = requireArg(args, "--events");
  const outDir = requireArg(args, "--out");
  ensureDir(outDir);

  const events = await readJsonlFile(eventsPath);
  console.log(`[state] read ${events.length} events from ${eventsPath}`);

  const state = deriveState(events);
  writeJson(join(outDir, "state.json"), state);

  const summary = generateStateSummary(state);
  writeJson(join(outDir, "summary.json"), summary);

  console.log(`[state] state written to ${join(outDir, "state.json")}`);
  console.log(`[state] summary written to ${join(outDir, "summary.json")}`);
}

// ── correlate-evidence ──

async function cmdCorrelate(args) {
  const eventsPath = requireArg(args, "--events");
  const evidencePath = requireArg(args, "--evidence");
  const outDir = requireArg(args, "--out");
  ensureDir(outDir);

  const events = await readJsonlFile(eventsPath);
  console.log(`[correlate] read ${events.length} events from ${eventsPath}`);

  if (!existsSync(evidencePath)) {
    console.error(`[correlate] evidence file not found: ${evidencePath}`);
    process.exit(1);
  }

  const rawEvidence = JSON.parse(readFileSync(evidencePath, "utf8"));
  const scenarios = Array.isArray(rawEvidence)
    ? rawEvidence
    : (rawEvidence.scenarios || [rawEvidence]);

  const result = correlate(events, scenarios);
  writeJson(join(outDir, "correlations.json"), result);

  console.log(`[correlate] correlations written to ${join(outDir, "correlations.json")}`);
  console.log(`[correlate] summary: ${result.summary.total_scenarios} scenarios, ${result.summary.matched} matched, ${result.summary.not_applicable} not_applicable`);
}

// ── inspect ──

function cmdInspectMappings(args) {
  const outPath = requireArg(args, "--out");
  const output = {
    version: 1,
    generated_at: new Date().toISOString(),
    supported_events: {
      child: [
        "dataRegistry.DataPublished",
        "dataRegistry.ImtRootUpdated",
        "dataRegistry.ListingStatusChanged",
        "tradeSession.SessionCreated",
        "tradeSession.SessionAccepted",
        "tradeSession.RoundOpened",
        "tradeSession.PaymentProofSubmitted",
        "tradeSession.DataProofSubmitted",
        "tradeSession.DataProofAttested",
        "tradeSession.ProofSignatureSubmitted",
        "tradeSession.DataDelivered",
        "tradeSession.PaymentPreimageSubmitted",
        "tradeSession.RoundCompleted",
        "tradeSession.SettlementClaimed",
        "tradeSession.SessionPunished",
        "tradeSession.LastPaymentClaimed",
      ],
      main: [
        "mainEscrow.EscrowOpened",
        "mainEscrow.FundsLocked",
        "mainEscrow.DepositLocked",
        "mainEscrow.EscrowSettled",
        "mainEscrow.EscrowPunished",
      ],
    },
    state_derivation_events: [
      "dataRegistry.DataPublished",
      "dataRegistry.ImtRootUpdated",
      "dataRegistry.ListingStatusChanged",
      "tradeSession.SessionCreated",
      "tradeSession.SessionAccepted",
      "tradeSession.SettlementClaimed",
      "tradeSession.SessionPunished",
      "tradeSession.LastPaymentClaimed",
      "tradeSession.RoundCompleted",
      "mainEscrow.EscrowOpened",
      "mainEscrow.FundsLocked",
      "mainEscrow.DepositLocked",
      "mainEscrow.EscrowSettled",
      "mainEscrow.EscrowPunished",
    ],
  };
  writeJson(outPath, output);
  console.log(`[inspect] mappings written to ${outPath}`);
}

function cmdInspectCursor(args) {
  const cursorPath = requireArg(args, "--cursor");
  if (!existsSync(cursorPath)) {
    console.error(`[inspect] cursor file not found: ${cursorPath}`);
    process.exit(1);
  }
  const cursor = JSON.parse(readFileSync(cursorPath, "utf8"));
  console.log("Cursor summary:");
  for (const [chain, val] of Object.entries(cursor)) {
    const parsed = parseCursor(val);
    if (parsed) {
      console.log(`  ${chain}: next block ${parsed.block}`);
    } else {
      console.log(`  ${chain}: ${val}`);
    }
  }
}

async function cmdInspectCounts(args) {
  const eventsPath = optionalArg(args, "--events");
  if (!eventsPath) {
    console.log("No --events path provided.");
    return;
  }
  if (!existsSync(eventsPath)) {
    console.log(`Events file not found: ${eventsPath}`);
    return;
  }
  const events = await readJsonlFile(eventsPath);
  console.log(`Event count: ${events.length}`);

  const state = deriveState(events);
  console.log(`Listing count: ${Object.keys(state.listings).length}`);
  console.log(`Session count: ${Object.keys(state.sessions).length}`);
  console.log(`Escrow count: ${Object.keys(state.escrows).length}`);
}

function cmdInspectSampleEvidence(args) {
  const kind = optionalArg(args, "--kind") || "dry-run";
  const outPath = requireArg(args, "--out");

  const sample = {
    id: "sample-dry-run-" + kind,
    category: kind,
    status: "passed",
    command: "inspect sample-evidence --kind dry-run",
    log_path: null,
    evidence_path: null,
    scenario: "happy",
    result: "dry-run-accepted",
    listing_id: null,
    escrow_id: null,
    session_id: null,
    settlement: null,
    scenario_outcome: null,
    events: [],
    constraints: [
      {
        round_index: 0,
        field_name: "temperature",
        proof_digest: "0x0000000000000000000000000000000000000000000000000000000000000abc",
        business_input_hash: "0x0000000000000000000000000000000000000000000000000000000000000def",
        vk_hash: "0x0000000000000000000000000000000000000000000000000000000000000111",
        public_input_hash: "0x0000000000000000000000000000000000000000000000000000000000000222",
        on_chain_bound: false,
      },
    ],
    error: null,
  };

  writeJson(outPath, sample);
  console.log(`[inspect] sample evidence written to ${outPath}`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
