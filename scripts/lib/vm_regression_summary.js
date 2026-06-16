import { appendFileSync, existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname } from "node:path";

function parseArgs(argv) {
  const out = { _: [] };
  for (let i = 0; i < argv.length; i++) {
    const item = argv[i];
    if (item.startsWith("--")) {
      out[item.slice(2)] = argv[++i];
    } else {
      out._.push(item);
    }
  }
  return out;
}

function readSummary(path) {
  if (!existsSync(path)) {
    return {
      started_at: new Date().toISOString(),
      finished_at: null,
      status: "running",
      steps: [],
    };
  }
  return JSON.parse(readFileSync(path, "utf8"));
}

function saveSummary(path, summary) {
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, `${JSON.stringify(summary, null, 2)}\n`);
}

export function writeMarkdownSummary(jsonPath, markdownPath, summary) {
  mkdirSync(dirname(markdownPath), { recursive: true });
  const lines = [
    "# Data Trade VM Regression Summary",
    "",
    `- Status: ${summary.status}`,
    `- Started: ${summary.started_at}`,
    `- Finished: ${summary.finished_at ?? ""}`,
    `- JSON: ${jsonPath}`,
    "",
    "| Step | Status | Detail |",
    "|------|--------|--------|",
    ...summary.steps.map((step) => {
      const detail = Object.entries(step)
        .filter(([key]) => !["name", "status", "at"].includes(key))
        .map(([key, value]) => `${key}=${String(value).replaceAll("|", "\\|")}`)
        .join("<br>");
      return `| ${step.name} | ${step.status} | ${detail} |`;
    }),
    "",
  ];
  writeFileSync(markdownPath, lines.join("\n"));
  appendFileSync(markdownPath, "\n");
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  const command = args._[0];
  if (!command) throw new Error("usage: vm_regression_summary.js <init|record|finish> --json PATH ...");
  if (!args.json) throw new Error("--json is required");

  const summary = readSummary(args.json);
  if (command === "init") {
    saveSummary(args.json, summary);
    return;
  }
  if (command === "record") {
    if (!args.step || !args.status) throw new Error("record requires --step and --status");
    summary.steps.push({
      name: args.step,
      status: args.status,
      at: new Date().toISOString(),
      detail: args.detail ?? "",
    });
    saveSummary(args.json, summary);
    return;
  }
  if (command === "finish") {
    if (!args.status) throw new Error("finish requires --status");
    summary.status = args.status;
    summary.finished_at = new Date().toISOString();
    saveSummary(args.json, summary);
    if (args.markdown) writeMarkdownSummary(args.json, args.markdown, summary);
    return;
  }
  throw new Error(`unknown command: ${command}`);
}

main();
