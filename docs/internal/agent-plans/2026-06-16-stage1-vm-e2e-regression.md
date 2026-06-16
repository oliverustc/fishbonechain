# Stage 1 VM E2E Regression Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把已手工验证通过的 main+child6 数据交易 VM E2E 固化为可重复执行的一键回归能力。

**Architecture:** 新增一个薄封装回归脚本，复用现有 `scripts/dev_redeploy_clean_chains.sh`、`scripts/data_trade_flow.js`、`scripts/zk_attested_data_trade_flow.js`、`scripts/zk_real_data_trade_flow.js` 和 `target/tools/fishbone-zk`。JSON/Markdown summary 由独立 Node CLI `scripts/lib/vm_regression_summary.js` 维护，bash 只转发参数，避免 heredoc 与 `node --input-type=module` 参数语义冲突。执行中发现 VM clean redeploy 后立即连接 RPC 时 `ApiPromise.create` 存在启动窗口卡住风险，因此新增 `scripts/lib/wait_for_ws_chain.js`，等待 main/child6 均稳定推进区块后再进入业务 E2E。

**Tech Stack:** Bash, Node.js `@polkadot/api`, Go/gnark CLI, existing Python deploy wrappers, Markdown docs.

---

## Files

- Create: `scripts/run_data_trade_vm_regression.sh`
- Create: `scripts/lib/vm_regression_summary.js`
- Create: `scripts/lib/wait_for_ws_chain.js`
- Modify: `docs/implementation/data-trade-implementation.md`
- Modify: `docs/internal/agent-plans/2026-06-16-data-trade-four-stage-roadmap.md`
- Modify: `docs/internal/agent-plans/2026-06-16-stage1-vm-e2e-regression.md`

## Task 1: Add Regression Summary Helper

- [x] Step 1: Create `scripts/lib/vm_regression_summary.js`.

Expected file content:

```js
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
```

- [x] Step 2: Run syntax check.

Run:

```bash
node --check scripts/lib/vm_regression_summary.js
```

Expected: exit code 0.

- [x] Step 3: Commit helper.

Run:

```bash
git add scripts/lib/vm_regression_summary.js
git commit -m "test: add vm regression summary helper"
```

## Task 2: Add One-Command Regression Script

- [x] Step 1: Create `scripts/run_data_trade_vm_regression.sh`.

Expected file content:

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

MAIN_WS="${MAIN_WS:-ws://10.2.2.11:9944}"
CHILD_WS="${CHILD_WS:-ws://10.2.2.11:9950}"
CONFIG="${CONFIG:-deploy/config.toml}"
SUMMARY_DIR="${SUMMARY_DIR:-target/data-trade-vm-regression}"
SUMMARY_JSON="$SUMMARY_DIR/summary.json"
SUMMARY_MD="$SUMMARY_DIR/summary.md"
ZK_VERIFIER_CMD="${ZK_VERIFIER_CMD:-target/tools/fishbone-zk}"
SKIP_DEPLOY="${SKIP_DEPLOY:-0}"

mkdir -p "$SUMMARY_DIR"

record_step() {
  local name="$1"
  local status="$2"
  local detail="${3:-}"
  node scripts/lib/vm_regression_summary.js record --json "$SUMMARY_JSON" --step "$name" --status "$status" --detail "$detail"
}

finish_summary() {
  local status="$1"
  node scripts/lib/vm_regression_summary.js finish --json "$SUMMARY_JSON" --markdown "$SUMMARY_MD" --status "$status"
}

trap 'finish_summary failed' ERR

node scripts/lib/vm_regression_summary.js init --json "$SUMMARY_JSON"

record_step preflight started "main=$MAIN_WS child=$CHILD_WS"

if [[ ! -x "$ZK_VERIFIER_CMD" ]]; then
  mkdir -p target/tools
  (cd tools/data-trade-zk && go build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk)
fi
record_step zk_cli ok "$ZK_VERIFIER_CMD"

if [[ "$SKIP_DEPLOY" != "1" ]]; then
  bash scripts/dev_redeploy_clean_chains.sh --chains main,child6 --config "$CONFIG" --logs
  record_step deploy ok "clean redeploy main,child6"
else
  record_step deploy skipped "SKIP_DEPLOY=1"
fi

node scripts/data_trade_flow.js --main "$MAIN_WS" --child "$CHILD_WS" --scenario happy
record_step base_happy ok "data_trade_flow happy"

node scripts/data_trade_flow.js --main "$MAIN_WS" --child "$CHILD_WS" --scenario invalid-proof
record_step base_invalid_proof ok "data_trade_flow invalid-proof"

node scripts/data_trade_flow.js --main "$MAIN_WS" --child "$CHILD_WS" --scenario requester-refuses-payment
record_step base_refuses_payment ok "data_trade_flow requester-refuses-payment"

node scripts/zk_attested_data_trade_flow.js --main "$MAIN_WS" --child "$CHILD_WS"
record_step dev_zk_attested ok "zk_attested_data_trade_flow"

ZK_VERIFIER_CMD="$ZK_VERIFIER_CMD" node scripts/zk_real_data_trade_flow.js --main "$MAIN_WS" --child "$CHILD_WS"
record_step real_zk_attested ok "zk_real_data_trade_flow"

find target/data-trade-zk -maxdepth 3 -type f | sort > "$SUMMARY_DIR/artifacts.txt"
record_step artifacts ok "$SUMMARY_DIR/artifacts.txt"

finish_summary passed
echo "Summary JSON: $SUMMARY_JSON"
echo "Summary MD:   $SUMMARY_MD"
```

- [x] Step 2: Make script executable.

Run:

```bash
chmod +x scripts/run_data_trade_vm_regression.sh
```

- [x] Step 3: Run shell syntax check.

Run:

```bash
bash -n scripts/run_data_trade_vm_regression.sh
```

Expected: exit code 0.

- [x] Step 4: Commit script.

Run:

```bash
git add scripts/run_data_trade_vm_regression.sh
git commit -m "test: add data trade vm regression runner"
```

## Task 3: Execute VM Regression

- [x] Step 1: Run full regression on VM.

Run:

```bash
MAIN_WS=ws://10.2.2.11:9944 CHILD_WS=ws://10.2.2.11:9950 scripts/run_data_trade_vm_regression.sh
```

Expected:

- `target/data-trade-vm-regression/summary.json` has `"status": "passed"`.
- `target/data-trade-vm-regression/summary.md` lists all base/dev-zk/real-zk steps as `ok`.
- Real gnark path prints `verifier=gnark-groth16-bn254`.
- Final real-zk balances show Bob/Alice reserved `0`.

- [x] Step 2: If regression fails, record failure in Execution Record.

Failure record format:

```markdown
- Failure at `[step-name]`: command `...` failed with `...`.
- Root cause:
- Fix applied:
- Re-run result:
```

- [x] Step 3: Commit execution docs.

Run:

```bash
git add docs/internal/agent-plans/2026-06-16-stage1-vm-e2e-regression.md docs/implementation/data-trade-implementation.md
git commit -m "docs: record vm regression execution"
```

## Task 4: Update Roadmap

- [x] Step 1: Mark Stage 1 complete in `docs/internal/agent-plans/2026-06-16-data-trade-four-stage-roadmap.md`.

Change:

```markdown
- [ ] Stage 1: VM E2E Regression
```

to:

```markdown
- [x] Stage 1: VM E2E Regression
```

- [x] Step 2: Commit roadmap update.

Run:

```bash
git add docs/internal/agent-plans/2026-06-16-data-trade-four-stage-roadmap.md
git commit -m "docs: mark vm regression stage complete"
```

## Execution Record

- 2026-06-16: `scripts/lib/vm_regression_summary.js` implemented and syntax checked with `node --check scripts/lib/vm_regression_summary.js`.
- 2026-06-16: Found summary helper `init` reused old summary files. Root cause: `init` called `readSummary()` instead of creating a fresh run record. Fix applied: `init` now writes a new `{status: "running", steps: []}` summary. Re-run result: temp-file reset check passed.
- 2026-06-16: `scripts/run_data_trade_vm_regression.sh` implemented and syntax checked with `bash -n scripts/run_data_trade_vm_regression.sh`.
- Failure at `base_happy`: command `MAIN_WS=ws://10.2.2.11:9944 CHILD_WS=ws://10.2.2.11:9950 scripts/run_data_trade_vm_regression.sh` hung after deploy when `data_trade_flow.js` started immediately after RPC service startup.
- Root cause: VM services were listening and producing blocks, but an early `ApiPromise.create` call can hang during the post-redeploy startup window; the runner had no readiness gate before entering business flows.
- Fix applied: added `scripts/lib/wait_for_ws_chain.js`, added `rpc_ready` step to wait for both main and child6 headers to advance, and wrapped each long-running step with `timeout` plus failed-step summary recording.
- Re-run result: full VM regression passed on 2026-06-16. `target/data-trade-vm-regression/summary.json` status is `passed`; summary includes `deploy`, `rpc_ready`, `base_happy`, `base_invalid_proof`, `base_refuses_payment`, `dev_zk_attested`, `real_zk_attested`, and `artifacts` as `ok`.
- Real gnark evidence: `zk_real_data_trade_flow.js` printed `verifier=gnark-groth16-bn254`; generated proof digests `0xbea5eb089d6cbc26527c16000843f3e1777398fd22a775e47a858f38c66655b1` and `0x0b7c0ede0d3085c203c4d0cb5af9c56e421977893fd078d997ba59edc04c8660`; final Bob/Alice reserved balances were both `0`.
