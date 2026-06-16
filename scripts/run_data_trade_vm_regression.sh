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
FLOW_TIMEOUT_SECONDS="${FLOW_TIMEOUT_SECONDS:-300}"

mkdir -p "$SUMMARY_DIR"

record_step() {
  local name="$1"
  local status="$2"
  local detail="${3:-}"
  node scripts/lib/vm_regression_summary.js record --json "$SUMMARY_JSON" --step "$name" --status "$status" --detail "$detail"
}

run_step() {
  local name="$1"
  local detail="$2"
  shift 2

  record_step "$name" started "$detail"
  if timeout "${FLOW_TIMEOUT_SECONDS}s" "$@"; then
    record_step "$name" ok "$detail"
    return 0
  fi

  local status=$?
  record_step "$name" failed "exit=$status $detail"
  return "$status"
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
  run_step deploy "clean redeploy main,child6" bash scripts/dev_redeploy_clean_chains.sh --chains main,child6 --config "$CONFIG" --logs
else
  record_step deploy skipped "SKIP_DEPLOY=1"
fi

run_step rpc_ready "wait for main,child6 headers to advance" node scripts/lib/wait_for_ws_chain.js --main "$MAIN_WS" --child "$CHILD_WS" --min-blocks 2 --timeout-ms 180000

run_step base_happy "data_trade_flow happy" node scripts/data_trade_flow.js --main "$MAIN_WS" --child "$CHILD_WS" --scenario happy

run_step base_invalid_proof "data_trade_flow invalid-proof" node scripts/data_trade_flow.js --main "$MAIN_WS" --child "$CHILD_WS" --scenario invalid-proof

run_step base_refuses_payment "data_trade_flow requester-refuses-payment" node scripts/data_trade_flow.js --main "$MAIN_WS" --child "$CHILD_WS" --scenario requester-refuses-payment

run_step dev_zk_attested "zk_attested_data_trade_flow" node scripts/zk_attested_data_trade_flow.js --main "$MAIN_WS" --child "$CHILD_WS"

run_step real_zk_attested "zk_real_data_trade_flow" env ZK_VERIFIER_CMD="$ZK_VERIFIER_CMD" node scripts/zk_real_data_trade_flow.js --main "$MAIN_WS" --child "$CHILD_WS"

find target/data-trade-zk -maxdepth 3 -type f | sort > "$SUMMARY_DIR/artifacts.txt"
record_step artifacts ok "$SUMMARY_DIR/artifacts.txt"

finish_summary passed
echo "Summary JSON: $SUMMARY_JSON"
echo "Summary MD:   $SUMMARY_MD"
