#!/usr/bin/env bash
set -euo pipefail

repo="${FISHBONE_REPO:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
ref="${SCIDATAHUB_REF:-$repo/references/SciDataHub}"
out="${SCIDATAHUB_AUDIT_DIR:-$repo/docs/internal/reference-audits/scidatahub}"
model="${SCIDATAHUB_AUDIT_MODEL:-${FWF_MODEL:-deepseek/deepseek-v4-pro}}"
session="${SCIDATAHUB_AUDIT_SESSION:-}"
dry_run=1
skip_summary=0
selected_task=""

usage() {
  cat <<'USAGE'
Usage:
  opencode_scidatahub_audit.sh [--execute] [--task TASK_ID] [--skip-summary]

Task IDs:
  inventory          repository map, runnable modules, dependency surface
  domain             scientific data marketplace domain model and workflows
  chain              Fabric chaincode, gateway usage, identities, trust boundaries
  backend            Express API, persistence, IPFS, integration behavior
  frontend           Vue routes, UX flows, API contracts, reusable UI ideas
  benchmarks         Caliper workloads, metrics, reproducibility limits
  inherit            what FishboneChain should inherit, rewrite, or discard
  synthesis          cross-report reconciliation and migration checklist

Environment:
  FISHBONE_REPO=/home/swt/fishbonechain
  SCIDATAHUB_REF=/home/swt/fishbonechain/references/SciDataHub
  SCIDATAHUB_AUDIT_DIR=/home/swt/fishbonechain/docs/internal/reference-audits/scidatahub
  SCIDATAHUB_AUDIT_MODEL=deepseek/deepseek-v4-pro
  SCIDATAHUB_AUDIT_SESSION=ses_xxx      optional: reuse one opencode session
  OPENCODE_EXTRA_ARGS="--auto"          optional extra opencode flags

Output:
  - context/*.txt|md deterministic grep/find summaries
  - reports/<task>.md opencode scan outputs
  - prompts/<task>.md exact prompts sent to opencode
  - index.md report index and execution metadata
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      dry_run=1
      shift
      ;;
    --execute)
      dry_run=0
      shift
      ;;
    --skip-summary)
      skip_summary=1
      shift
      ;;
    --task)
      selected_task="${2:-}"
      if [[ -z "$selected_task" ]]; then
        echo "--task requires a task id" >&2
        exit 2
      fi
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ ! -d "$ref" ]]; then
  echo "SciDataHub reference repo not found: $ref" >&2
  exit 1
fi

mkdir -p "$out/reports" "$out/prompts"
bash "$repo/scripts/reference-audit/collect_scidatahub_context.sh"

common_prompt() {
  cat <<'PROMPT'
You are auditing a deprecated reference repository so FishboneChain can inherit only the useful ideas.

Ground rules:
- Treat references/SciDataHub as read-only. Do not edit files.
- Prefer direct file evidence with paths and line numbers where possible.
- Be skeptical: distinguish implemented behavior, demo-only stubs, copied Fabric samples, incomplete TODOs, and assumptions.
- Keep the report concise but complete enough for a human maintainer to verify without re-reading the whole repo.
- Focus on migration value for FishboneChain's Substrate data-trade direction, not on preserving SciDataHub as a product.

Required report format:
1. Scope inspected
2. Confirmed facts with evidence
3. Gaps, demo shortcuts, or correctness risks
4. Reusable ideas/assets
5. Migration notes for FishboneChain
6. Follow-up files/questions

Useful deterministic context files are in docs/internal/reference-audits/scidatahub/context/.
PROMPT
}

task_prompt() {
  local task="$1"
  common_prompt
  echo
  case "$task" in
    inventory)
      cat <<'PROMPT'
Task: Build a repository inventory.

Inspect the top-level README/docs, package files, backend/frontend/caliper/blockchain/ipfs folders, and generated context inventory. Classify modules as original SciDataHub logic, upstream Hyperledger sample code, generated assets, tests/benchmarks, or deployment scaffolding. Identify which directories deserve deeper migration attention and which can be ignored.
PROMPT
      ;;
    domain)
      cat <<'PROMPT'
Task: Recover the scientific data circulation domain model.

Inspect backend database/services, frontend views/stores/router, README/docs, and chaincode names. Extract entities, lifecycle states, user roles, dataset/order/service workflows, IPFS/hash/encryption concepts, and any finance/payment semantics. Map each concept to FishboneChain data-trade concepts where possible.
PROMPT
      ;;
    chain)
      cat <<'PROMPT'
Task: Audit blockchain integration and trust boundaries.

Inspect blockchain/sci-data-trade, relevant asset-transfer chaincode, backend chaincode services/routes/config, and Fabric gateway samples. Determine exactly which actor identity submits transactions, whether end-user signatures are enforced, what state is actually on-chain, what remains off-chain, and which parts are demo shortcuts unsuitable for a real blockchain design.
PROMPT
      ;;
    backend)
      cat <<'PROMPT'
Task: Audit backend architecture.

Inspect backend/src and backend/test. Summarize Express routes, database tables/services, IPFS integration, logging, validation, error handling, authentication/authorization assumptions, test coverage, and frontend/API contracts. Highlight implementation patterns worth inheriting versus code that should be rewritten.
PROMPT
      ;;
    frontend)
      cat <<'PROMPT'
Task: Audit frontend product flows.

Inspect frontend/src router, views, stores, API client, and reusable components. Summarize user-facing workflows, UI pages, state management, local cryptography/signature behavior if any, and API dependencies. Identify concepts or screens that could inform a FishboneChain data-trade UI without copying weak implementation details.
PROMPT
      ;;
    benchmarks)
      cat <<'PROMPT'
Task: Audit tests, Caliper workloads, and reproducibility.

Inspect backend tests, blockchain chaincode tests, caliper benchmarks/workloads/network config, shell scripts, and report.html if useful. Determine what is actually measured, which workloads are synthetic, what parameters matter, what cannot be trusted, and whether any benchmark ideas can be reused for FishboneChain experiments.
PROMPT
      ;;
    inherit)
      cat <<'PROMPT'
Task: Produce an inheritance matrix.

Use the whole SciDataHub repo plus prior generated reports if present. Create a table with: candidate item, source files, current maturity, why it matters, inherit/rewrite/discard decision, FishboneChain target area, and migration risk. Pay special attention to data-market workflows, evidence/hash/IPFS concepts, frontend UX, API boundaries, and benchmark scaffolding.
PROMPT
      ;;
    synthesis)
      cat <<'PROMPT'
Task: Reconcile all generated SciDataHub audit reports.

Read docs/internal/reference-audits/scidatahub/reports/*.md and deterministic context. Resolve contradictions, flag weak claims that need manual verification, and produce a final checklist for the main Codex agent to review. Include a short "do not inherit" section for unsafe blockchain shortcuts, especially backend-submitted transactions under one identity.
PROMPT
      ;;
    *)
      echo "Unknown task id: $task" >&2
      exit 2
      ;;
  esac
}

all_tasks=(inventory domain chain backend frontend benchmarks inherit)
if [[ "$skip_summary" -eq 0 ]]; then
  all_tasks+=(synthesis)
fi

if [[ -n "$selected_task" ]]; then
  all_tasks=("$selected_task")
fi

{
  echo "# SciDataHub Opencode Audit Index"
  echo
  echo "- generated_at: $(date -Iseconds)"
  echo "- model: $model"
  echo "- reference_repo: references/SciDataHub"
  echo "- session: ${session:-new session per task}"
  echo "- dry_run: $dry_run"
  echo
  echo "## Reports"
} > "$out/index.md"

for task in "${all_tasks[@]}"; do
  prompt_file="$out/prompts/$task.md"
  report_file="$out/reports/$task.md"
  task_prompt "$task" > "$prompt_file"
  echo "- [$task](reports/$task.md) prompt: prompts/$task.md" >> "$out/index.md"

  if [[ "$dry_run" -eq 1 ]]; then
    echo "DRY RUN: would run task '$task' with prompt $prompt_file"
    continue
  fi

  echo "Running opencode SciDataHub audit task: $task"
  args=(run -m "$model" --dir "$repo" --title "SciDataHub audit: $task")
  if [[ -n "$session" ]]; then
    args=(run -s "$session" -m "$model" --dir "$repo")
  fi
  if [[ -n "${OPENCODE_EXTRA_ARGS:-}" ]]; then
    # shellcheck disable=SC2206
    extra=(${OPENCODE_EXTRA_ARGS})
    args+=("${extra[@]}")
  fi
  NO_COLOR=1 opencode "${args[@]}" "$(cat "$prompt_file")" |
    perl -pe 's/\e\[[0-9;]*[A-Za-z]//g' |
    tee "$report_file"
done

echo "SciDataHub audit artifacts are in $out"
