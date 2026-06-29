import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { basename, dirname } from "node:path";

function parseArgs(argv) {
  const out = { _: [] };
  for (let i = 0; i < argv.length; i++) {
    const item = argv[i];
    if (item.startsWith("--")) {
      const value = argv[i + 1];
      if (value === undefined || value.startsWith("--")) {
        out[item.slice(2)] = true;
      } else {
        out[item.slice(2)] = value;
        i++;
      }
    } else {
      out._.push(item);
    }
  }
  return out;
}

function readSummary(path) {
  if (!existsSync(path)) {
    throw new Error(`summary not found: ${path}. Run init first.`);
  }
  return JSON.parse(readFileSync(path, "utf8"));
}

function saveSummary(path, summary) {
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, `${JSON.stringify(summary, null, 2)}\n`);
}

function extractScenarioMeta(evidencePath) {
  if (!evidencePath || !existsSync(evidencePath)) return {};

  let evidence;
  try {
    evidence = JSON.parse(readFileSync(evidencePath, "utf8"));
  } catch {
    return { error: `failed to parse evidence: ${evidencePath}` };
  }

  const meta = {};

  if (evidence.scenario != null) meta.scenario = evidence.scenario;
  if (evidence.result != null) meta.result = evidence.result;
  if (evidence.listing_id != null) meta.listing_id = evidence.listing_id;
  if (evidence.escrow_id != null) meta.escrow_id = evidence.escrow_id;
  if (evidence.session_id != null) meta.session_id = evidence.session_id;

  if (evidence.settlement && typeof evidence.settlement === "object") {
    meta.settlement = {};
    if (evidence.settlement.completed_rounds != null)
      meta.settlement.completed_rounds = evidence.settlement.completed_rounds;
    if (evidence.settlement.remaining_rounds != null)
      meta.settlement.remaining_rounds = evidence.settlement.remaining_rounds;
  }

  if (evidence.scenario_outcome) {
    meta.scenario_outcome = evidence.scenario_outcome;
    if (evidence.scenario_outcome.events && Array.isArray(evidence.scenario_outcome.events)) {
      meta.events = evidence.scenario_outcome.events;
    }
  }

  if (!meta.events) meta.events = [];

  const constraints = [];
  if (evidence.constraints && Array.isArray(evidence.constraints)) {
    constraints.push(...evidence.constraints);
  }
  if (evidence.rounds && Array.isArray(evidence.rounds)) {
    for (const round of evidence.rounds) {
      if (round.constraints && Array.isArray(round.constraints)) {
        for (const c of round.constraints) {
          constraints.push({
            round_index: round.round_index ?? 0,
            field_name: c.field_name || "",
            proof_digest: c.proof_digest || "",
            business_input_hash: c.business_input_hash || "",
            on_chain_bound: c.on_chain_bound ?? false,
          });
        }
      }
    }
  }
  if (constraints.length > 0) meta.constraints = constraints;

  return meta;
}

function cmdInit(args) {
  const jsonPath = args.json;
  if (!jsonPath) throw new Error("init requires --json");

  const gitCommit = args["git-commit"] || "";
  const gitBranch = args["git-branch"] || "";

  const summary = {
    version: 1,
    kind: "data_trade_validation",
    stage: "stage14",
    status: "running",
    started_at: new Date().toISOString(),
    finished_at: null,
    environment: {
      profile: args.profile || "",
      main_ws: args.main || "",
      child_ws: args.child || "",
      zk_cmd: args["zk-cmd"] || "",
      git_commit: gitCommit,
      git_branch: gitBranch,
    },
    readiness: {},
    scenarios: [],
  };

  saveSummary(jsonPath, summary);
}

function cmdRecord(args) {
  const jsonPath = args.json;
  if (!jsonPath) throw new Error("record requires --json");

  const scenarioId = args["scenario-id"];
  if (!scenarioId) throw new Error("record requires --scenario-id");

  const category = args.category;
  if (!category) throw new Error("record requires --category");

  const status = args.status;
  if (!status) throw new Error("record requires --status");

  const command = args.command || "";
  const logPath = args.log || "";
  const evidencePath = args.evidence || "";
  const errorMsg = args.error || null;

  const evidenceMeta = extractScenarioMeta(evidencePath);

  const scenario = {
    id: scenarioId,
    category,
    status,
    command: evidenceMeta.command || command,
    log_path: evidenceMeta.log_path || logPath,
    evidence_path: evidencePath || null,
    scenario: evidenceMeta.scenario || null,
    result: evidenceMeta.result || null,
    listing_id: evidenceMeta.listing_id ?? null,
    escrow_id: evidenceMeta.escrow_id ?? null,
    session_id: evidenceMeta.session_id ?? null,
    settlement: evidenceMeta.settlement || null,
    scenario_outcome: evidenceMeta.scenario_outcome || null,
    events: evidenceMeta.events || [],
    constraints: evidenceMeta.constraints || [],
    error: evidenceMeta.error || errorMsg,
  };

  const summary = readSummary(jsonPath);

  const existingIdx = summary.scenarios.findIndex((s) => s.id === scenarioId);
  if (existingIdx >= 0) {
    summary.scenarios[existingIdx] = scenario;
  } else {
    summary.scenarios.push(scenario);
  }

  saveSummary(jsonPath, summary);
}

function cmdReadiness(args) {
  const jsonPath = args.json;
  if (!jsonPath) throw new Error("readiness requires --json");

  const summary = readSummary(jsonPath);
  summary.readiness = {
    main_ready: args["main-ready"] === "1" || args["main-ready"] === "true",
    child_ready: args["child-ready"] === "1" || args["child-ready"] === "true",
    main_diagnostic: args["main-diagnostic"] || "",
    child_diagnostic: args["child-diagnostic"] || "",
    checked_at: new Date().toISOString(),
  };
  saveSummary(jsonPath, summary);
}

function cmdFinish(args) {
  const jsonPath = args.json;
  if (!jsonPath) throw new Error("finish requires --json");

  const finalStatus = args.status;
  if (!finalStatus) throw new Error("finish requires --status");

  const summary = readSummary(jsonPath);
  summary.status = finalStatus;
  summary.finished_at = new Date().toISOString();
  saveSummary(jsonPath, summary);

  if (args.markdown) {
    writeMarkdownSummary(args.markdown, summary);
  }
}

function writeMarkdownSummary(markdownPath, summary) {
  mkdirSync(dirname(markdownPath), { recursive: true });

  const lines = [];

  lines.push("# Data Trade Validation Summary (Stage 14)", "");
  lines.push(`- **Status**: ${summary.status}`);
  lines.push(`- **Started**: ${summary.started_at}`);
  lines.push(`- **Finished**: ${summary.finished_at || "—"}`, "");

  const env = summary.environment;
  if (env) {
    lines.push("## Environment", "");
    lines.push("| Key | Value |");
    lines.push("|-----|-------|");
    lines.push(`| profile | ${env.profile || "—"} |`);
    lines.push(`| main\_ws | ${env.main_ws || "—"} |`);
    lines.push(`| child\_ws | ${env.child_ws || "—"} |`);
    lines.push(`| zk\_cmd | ${env.zk_cmd || "—"} |`);
    lines.push(`| git\_commit | ${env.git_commit || "—"} |`);
    lines.push(`| git\_branch | ${env.git_branch || "—"} |`, "");
  }

  if (summary.readiness && (summary.readiness.main_ready !== undefined)) {
    lines.push("## Readiness", "");
    lines.push("| Endpoint | Ready | Diagnostic |");
    lines.push("|----------|-------|------------|");
    lines.push(
      `| main (${env?.main_ws || ""}) | ${summary.readiness.main_ready ? "yes" : "no"} | ${summary.readiness.main_diagnostic || "—"} |`
    );
    lines.push(
      `| child (${env?.child_ws || ""}) | ${summary.readiness.child_ready ? "yes" : "no"} | ${summary.readiness.child_diagnostic || "—"} |`
    );
    lines.push("");
  }

  if (summary.scenarios && summary.scenarios.length > 0) {
    lines.push("## Scenarios", "");
    lines.push(
      "| ID | Category | Status | Scenario | Result | Listing | Escrow | Session | Settlement | Events | Error |"
    );
    lines.push(
      "|----|----------|--------|----------|--------|---------|--------|---------|------------|--------|-------|"
    );

    for (const s of summary.scenarios) {
      const settlement =
        s.settlement && typeof s.settlement === "object"
          ? `completed=${s.settlement.completed_rounds ?? "—"} remaining=${s.settlement.remaining_rounds ?? "—"}`
          : "—";
      const events =
        s.events && s.events.length > 0 ? s.events.join(", ") : "—";
      const err = s.error ? s.error.replaceAll("|", "\\|") : "—";

      lines.push(
        `| ${s.id} | ${s.category} | ${s.status} | ${s.scenario ?? "—"} | ${s.result ?? "—"} | ${s.listing_id ?? "—"} | ${s.escrow_id ?? "—"} | ${s.session_id ?? "—"} | ${settlement} | ${events} | ${err} |`
      );
    }
    lines.push("");
  }

  lines.push("## Artifacts", "");
  for (const s of summary.scenarios) {
    if (s.log_path) lines.push(`- \`${s.log_path}\``);
    if (s.evidence_path) lines.push(`- \`${s.evidence_path}\``);
  }
  lines.push("");

  writeFileSync(markdownPath, lines.join("\n"));
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  const command = args._[0];

  if (!command) {
    process.stderr.write(
      "usage: data_trade_validation_summary.js <init|record|readiness|finish> --json PATH ...\n"
    );
    process.exit(2);
  }

  try {
    switch (command) {
      case "init":
        cmdInit(args);
        break;
      case "record":
        cmdRecord(args);
        break;
      case "readiness":
        cmdReadiness(args);
        break;
      case "finish":
        cmdFinish(args);
        break;
      default:
        throw new Error(`unknown command: ${command}`);
    }
  } catch (e) {
    process.stderr.write(`${e.message}\n`);
    process.exit(1);
  }
}

main();
