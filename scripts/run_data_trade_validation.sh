#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PROFILE="${PROFILE:-child6-data-trade}"
MAIN_WS="${MAIN_WS:-ws://10.2.2.11:9944}"
CHILD_WS="${CHILD_WS:-ws://10.2.2.11:9950}"
OUT_DIR=""
ZK_CMD="${ZK_CMD:-target/tools/fishbone-zk}"
SKIP_LIVE=0
SKIP_DRY_RUN=0
SKIP_NEGATIVE=0
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-300}"
NO_BUILD_ZK=0

DATASET_DIR="scripts/fixtures/data_trade_datasets"
REQUEST_DIR="scripts/fixtures/data_trade_requests"
FLOW_SCRIPT="scripts/zk_real_data_trade_flow.js"
SUMMARY_TOOL="scripts/lib/data_trade_validation_summary.js"
READINESS_TOOL="scripts/lib/wait_for_ws_chain.js"

print_help() {
  cat <<EOF
Usage: $0 [options]

Options:
  --profile NAME            Trade profile. Default: child6-data-trade.
  --main URL                Override main RPC.
  --child URL               Override child RPC.
  --out PATH                Output directory.
  --zk-cmd PATH             ZK verifier command. Default: target/tools/fishbone-zk.
  --skip-live               Run only syntax/build/dry-run/negative checks.
  --skip-dry-run            Skip dry-run matrix.
  --skip-negative           Skip negative validation.
  --timeout-seconds N       Per-scenario timeout. Default: 300.
  --no-build-zk             Do not auto-build target/tools/fishbone-zk.
  -h, --help                Show help.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --profile)
      PROFILE="$2"; shift 2 ;;
    --main)
      MAIN_WS="$2"; shift 2 ;;
    --child)
      CHILD_WS="$2"; shift 2 ;;
    --out)
      OUT_DIR="$2"; shift 2 ;;
    --zk-cmd)
      ZK_CMD="$2"; shift 2 ;;
    --skip-live)
      SKIP_LIVE=1; shift ;;
    --skip-dry-run)
      SKIP_DRY_RUN=1; shift ;;
    --skip-negative)
      SKIP_NEGATIVE=1; shift ;;
    --timeout-seconds)
      TIMEOUT_SECONDS="$2"; shift 2 ;;
    --no-build-zk)
      NO_BUILD_ZK=1; shift ;;
    -h|--help)
      print_help; exit 0 ;;
    *)
      echo "unknown option: $1" >&2; print_help; exit 2 ;;
  esac
done

if [[ -z "$OUT_DIR" ]]; then
  TS="$(date -u +%Y%m%dT%H%M%SZ)"
  OUT_DIR="target/data-trade-validation/stage14-${TS}"
fi

SUMMARY_JSON="$OUT_DIR/summary.json"
SUMMARY_MD="$OUT_DIR/summary.md"
COMMANDS_LOG="$OUT_DIR/commands.log"
READINESS_DIR="$OUT_DIR/readiness"
DRY_RUN_DIR="$OUT_DIR/dry-run"
NEGATIVE_DIR="$OUT_DIR/negative"
LIVE_DIR="$OUT_DIR/live"
POSTCHECK_DIR="$OUT_DIR/postcheck"

OVERALL_STATUS="passed"
LIVE_HAPPY_PASSED=0

mkdir -p "$READINESS_DIR" "$DRY_RUN_DIR" "$NEGATIVE_DIR" "$LIVE_DIR" "$POSTCHECK_DIR"

log_command() {
  echo "[$(date -Iseconds)] $1" >> "$COMMANDS_LOG"
}

summary_init() {
  local git_commit git_branch
  git_commit="$(git rev-parse HEAD 2>/dev/null || echo "")"
  git_branch="$(git branch --show-current 2>/dev/null || echo "")"

  node "$SUMMARY_TOOL" init \
    --json "$SUMMARY_JSON" \
    --profile "$PROFILE" \
    --main "$MAIN_WS" \
    --child "$CHILD_WS" \
    --zk-cmd "$ZK_CMD" \
    --git-commit "$git_commit" \
    --git-branch "$git_branch"
}

summary_record() {
  local scenario_id="$1" category="$2" status="$3" cmd="$4" log_path="$5"
  local evidence_path="${6:-}"
  local error_msg="${7:-}"

  local args=(
    --json "$SUMMARY_JSON"
    --scenario-id "$scenario_id"
    --category "$category"
    --status "$status"
    --command "$cmd"
    --log "$log_path"
  )
  if [[ -n "$evidence_path" ]]; then
    args+=(--evidence "$evidence_path")
  fi
  if [[ -n "$error_msg" ]]; then
    args+=(--error "$error_msg")
  fi

  node "$SUMMARY_TOOL" record "${args[@]}"
}

summary_readiness() {
  local main_ready="$1" child_ready="$2" main_diag="$3" child_diag="$4"

  node "$SUMMARY_TOOL" readiness \
    --json "$SUMMARY_JSON" \
    --main-ready "$main_ready" \
    --child-ready "$child_ready" \
    --main-diagnostic "$main_diag" \
    --child-diagnostic "$child_diag"
}

summary_finish() {
  node "$SUMMARY_TOOL" finish \
    --json "$SUMMARY_JSON" \
    --markdown "$SUMMARY_MD" \
    --status "$OVERALL_STATUS"
}

run_scenario() {
  local scenario_id="$1" category="$2" expect_nonzero="$3"
  local scenario_dir="$4" scenario_name="$5" evidence_out="$6"
  shift 6

  local run_log="$scenario_dir/run.log"
  local full_cmd="$*"

  log_command "$full_cmd"

  local exit_code=0

  if timeout "${TIMEOUT_SECONDS}s" env ZK_VERIFIER_CMD="$ZK_CMD" "$@" > "$run_log" 2>&1; then
    exit_code=0
  else
    exit_code=$?
  fi

  if [[ "$expect_nonzero" == "1" ]]; then
    if [[ $exit_code -ne 0 ]]; then
      summary_record "$scenario_id" "$category" "passed" "$full_cmd" "$run_log" "$evidence_out"
      return 0
    else
      summary_record "$scenario_id" "$category" "failed" "$full_cmd" "$run_log" "" "expected non-zero exit but got 0"
      OVERALL_STATUS="failed"
      return 1
    fi
  fi

  if [[ $exit_code -eq 0 ]]; then
    summary_record "$scenario_id" "$category" "passed" "$full_cmd" "$run_log" "$evidence_out"
    return 0
  else
    summary_record "$scenario_id" "$category" "failed" "$full_cmd" "$run_log" "$evidence_out" "exit code $exit_code"
    return 1
  fi
}

trap 'summary_finish' EXIT

echo "=== Stage 14 Data Trade Validation ==="
echo "Profile:    $PROFILE"
echo "Main:       $MAIN_WS"
echo "Child:      $CHILD_WS"
echo "Output:     $OUT_DIR"
echo "ZK cmd:     $ZK_CMD"
echo "Timeout:    ${TIMEOUT_SECONDS}s"
echo ""

summary_init

# ---- Preflight ----
if [[ "$NO_BUILD_ZK" != "1" ]] && [[ ! -x "$ZK_CMD" ]]; then
  echo "[preflight] building ZK verifier..."
  mkdir -p "$(dirname "$ZK_CMD")"
  (cd tools/data-trade-zk && go build -o ../../"$ZK_CMD" ./cmd/fishbone-zk)
fi

if [[ ! -x "$ZK_CMD" ]]; then
  echo "[preflight] WARNING: ZK verifier not found at $ZK_CMD; dry-run/negative/live scenarios will likely fail" >&2
fi

# ---- Dry-run scenarios ----
if [[ "$SKIP_DRY_RUN" == "0" ]]; then
  echo "--- Dry-run validation ---"

  DRY_RUN_SCENARIOS=(
    "dry-run-factory-temperature|dry-run|$DRY_RUN_DIR/factory-temperature|happy|$DRY_RUN_DIR/factory-temperature/evidence.json|--profile $PROFILE --dataset $DATASET_DIR/factory_sensors.json --request $REQUEST_DIR/factory_temperature_range.json --evidence-out $DRY_RUN_DIR/factory-temperature/evidence.json --dry-run-dynamic"
    "dry-run-factory-multi-range|dry-run|$DRY_RUN_DIR/factory-multi-range|happy|$DRY_RUN_DIR/factory-multi-range/evidence.json|--profile $PROFILE --dataset $DATASET_DIR/factory_sensors.json --request $REQUEST_DIR/factory_multi_range.json --evidence-out $DRY_RUN_DIR/factory-multi-range/evidence.json --dry-run-dynamic"
    "dry-run-vehicle-speed|dry-run|$DRY_RUN_DIR/vehicle-speed|happy|$DRY_RUN_DIR/vehicle-speed/evidence.json|--profile $PROFILE --dataset $DATASET_DIR/vehicle_telematics.json --request $REQUEST_DIR/vehicle_speed_range.json --evidence-out $DRY_RUN_DIR/vehicle-speed/evidence.json --dry-run-dynamic"
  )

  for entry in "${DRY_RUN_SCENARIOS[@]}"; do
    IFS='|' read -r sid cat sdir sname evout args <<< "$entry"
    mkdir -p "$sdir"
    echo "  [$sid] running..."
    if ! run_scenario "$sid" "$cat" "0" "$sdir" "$sname" "$evout" node "$FLOW_SCRIPT" $args; then
      OVERALL_STATUS="failed"
      echo "  [$sid] FAILED"
    else
      echo "  [$sid] passed"
    fi
  done
else
  echo "--- Dry-run skipped ---"
fi

# ---- Negative validation ----
if [[ "$SKIP_NEGATIVE" == "0" ]]; then
  echo "--- Negative validation ---"

  NEG_SCENARIOS=(
    "neg-factory-temp-out|negative|$NEGATIVE_DIR/factory-temperature-out-of-range|happy||--profile $PROFILE --dataset $DATASET_DIR/factory_sensors.json --request $REQUEST_DIR/factory_temperature_out_of_range.json --dry-run-dynamic"
    "neg-factory-multi-out|negative|$NEGATIVE_DIR/factory-multi-range-out-of-range|happy||--profile $PROFILE --dataset $DATASET_DIR/factory_sensors.json --request $REQUEST_DIR/factory_multi_range_out_of_range.json --dry-run-dynamic"
  )

  for entry in "${NEG_SCENARIOS[@]}"; do
    IFS='|' read -r sid cat sdir sname evout args <<< "$entry"
    mkdir -p "$sdir"
    echo "  [$sid] running (expect non-zero exit)..."
    if ! run_scenario "$sid" "$cat" "1" "$sdir" "$sname" "" node "$FLOW_SCRIPT" $args; then
      OVERALL_STATUS="failed"
      echo "  [$sid] FAILED"
    else
      echo "  [$sid] passed (non-zero exit as expected)"
    fi
  done
else
  echo "--- Negative validation skipped ---"
fi

# ---- Readiness check ----
MAIN_READY=0
CHILD_READY=0
MAIN_DIAGNOSTIC=""
CHILD_DIAGNOSTIC=""

if [[ "$SKIP_LIVE" == "0" ]]; then
  echo "--- Readiness check ---"

  READINESS_LOG="$READINESS_DIR/run.log"
  READINESS_CMD="node $READINESS_TOOL --main $MAIN_WS --child $CHILD_WS --min-blocks 2 --timeout-ms 120000"

  log_command "$READINESS_CMD"

  if timeout 150s node "$READINESS_TOOL" --main "$MAIN_WS" --child "$CHILD_WS" --min-blocks 2 --timeout-ms 120000 > "$READINESS_LOG" 2>&1; then
    MAIN_READY=1
    CHILD_READY=1
    MAIN_DIAGNOSTIC="blocks advancing"
    CHILD_DIAGNOSTIC="blocks advancing"
    echo "  readiness: passed"
  else
    echo "  readiness: FAILED" >&2
    READINESS_EXIT=$?

    if grep -q "main.*advanced=" "$READINESS_LOG" 2>/dev/null; then
      MAIN_READY=1
      MAIN_DIAGNOSTIC="blocks advancing"
    else
      MAIN_DIAGNOSTIC="not ready (exit=$READINESS_EXIT)"
    fi

    if grep -q "child.*advanced=" "$READINESS_LOG" 2>/dev/null; then
      CHILD_READY=1
      CHILD_DIAGNOSTIC="blocks advancing"
    else
      CHILD_DIAGNOSTIC="not ready (exit=$READINESS_EXIT)"
    fi
  fi

  summary_readiness "$MAIN_READY" "$CHILD_READY" "$MAIN_DIAGNOSTIC" "$CHILD_DIAGNOSTIC"
else
  summary_readiness "0" "0" "skipped (--skip-live)" "skipped (--skip-live)"
fi

# ---- Live scenarios ----
if [[ "$SKIP_LIVE" == "0" ]]; then
  if [[ "$MAIN_READY" == "0" || "$CHILD_READY" == "0" ]]; then
    echo "--- Live scenarios SKIPPED (readiness failed) ---"
    OVERALL_STATUS="partial"

    LIVE_IDS=(
      "live-happy-multi-range"
      "live-invalid-proof"
      "live-invalid-plaintext"
      "live-requester-refuses-payment"
    )
    for lid in "${LIVE_IDS[@]}"; do
      summary_record "$lid" "live-chain" "skipped" "" "" "" "readiness failed: main_ready=$MAIN_READY child_ready=$CHILD_READY"
    done
  else
    echo "--- Live scenarios ---"

    # 1. Happy path
    HAPPY_DIR="$LIVE_DIR/happy-multi-range"
    mkdir -p "$HAPPY_DIR"
    HAPPY_EVIDENCE="$HAPPY_DIR/evidence.json"
    HAPPY_CMD="--profile $PROFILE --main $MAIN_WS --child $CHILD_WS --dataset $DATASET_DIR/factory_sensors.json --request $REQUEST_DIR/factory_multi_range.json --evidence-out $HAPPY_EVIDENCE"

    echo "  [live-happy-multi-range] running..."
    if run_scenario "live-happy-multi-range" "live-chain" "0" "$HAPPY_DIR" "happy" "$HAPPY_EVIDENCE" node "$FLOW_SCRIPT" $HAPPY_CMD; then
      LIVE_HAPPY_PASSED=1
      echo "  [live-happy-multi-range] passed"
    else
      OVERALL_STATUS="failed"
      echo "  [live-happy-multi-range] FAILED — skipping subsequent live scenarios"
    fi

    # 2-4. Failure/dispute scenarios (only if happy path passed)
    if [[ "$LIVE_HAPPY_PASSED" == "1" ]]; then
      local scenario spec sid sdir sname sevidence scmd sedge
      sedge=""

      # invalid-proof-dispute
      spec="live-invalid-proof|$LIVE_DIR/invalid-proof|invalid-proof-dispute|$LIVE_DIR/invalid-proof/evidence.json|--profile $PROFILE --main $MAIN_WS --child $CHILD_WS --dataset $DATASET_DIR/factory_sensors.json --request $REQUEST_DIR/factory_multi_range.json --scenario invalid-proof-dispute --evidence-out $LIVE_DIR/invalid-proof/evidence.json"

      IFS='|' read -r sid sdir sname sevidence scmd sedge <<< "$spec"
      mkdir -p "$sdir"
      echo "  [$sid] running..."
      if ! run_scenario "$sid" "live-chain" "0" "$sdir" "$sname" "$sevidence" node "$FLOW_SCRIPT" $scmd; then
        OVERALL_STATUS="failed"
        echo "  [$sid] FAILED"
      else
        echo "  [$sid] passed"
      fi

      # invalid-plaintext-dispute
      spec="live-invalid-plaintext|$LIVE_DIR/invalid-plaintext|invalid-plaintext-dispute|$LIVE_DIR/invalid-plaintext/evidence.json|--profile $PROFILE --main $MAIN_WS --child $CHILD_WS --dataset $DATASET_DIR/factory_sensors.json --request $REQUEST_DIR/factory_multi_range.json --scenario invalid-plaintext-dispute --evidence-out $LIVE_DIR/invalid-plaintext/evidence.json"

      IFS='|' read -r sid sdir sname sevidence scmd sedge <<< "$spec"
      mkdir -p "$sdir"
      echo "  [$sid] running..."
      if ! run_scenario "$sid" "live-chain" "0" "$sdir" "$sname" "$sevidence" node "$FLOW_SCRIPT" $scmd; then
        OVERALL_STATUS="failed"
        echo "  [$sid] FAILED"
      else
        echo "  [$sid] passed"
      fi

      # requester-refuses-payment
      spec="live-requester-refuses-payment|$LIVE_DIR/requester-refuses-payment|requester-refuses-payment|$LIVE_DIR/requester-refuses-payment/evidence.json|--profile $PROFILE --main $MAIN_WS --child $CHILD_WS --dataset $DATASET_DIR/factory_sensors.json --request $REQUEST_DIR/factory_multi_range.json --scenario requester-refuses-payment --evidence-out $LIVE_DIR/requester-refuses-payment/evidence.json"

      IFS='|' read -r sid sdir sname sevidence scmd sedge <<< "$spec"
      mkdir -p "$sdir"
      echo "  [$sid] running..."
      if ! run_scenario "$sid" "live-chain" "0" "$sdir" "$sname" "$sevidence" node "$FLOW_SCRIPT" $scmd; then
        OVERALL_STATUS="failed"
        echo "  [$sid] FAILED"
      else
        echo "  [$sid] passed"
      fi
    fi

    # ---- Postcheck ----
    echo "--- Postcheck ---"
    POSTCHECK_LOG="$POSTCHECK_DIR/run.log"
    POSTCHECK_CMD="node $READINESS_TOOL --main $MAIN_WS --child $CHILD_WS --min-blocks 1 --timeout-ms 60000"
    log_command "$POSTCHECK_CMD"

    if timeout 90s node "$READINESS_TOOL" --main "$MAIN_WS" --child "$CHILD_WS" --min-blocks 1 --timeout-ms 60000 > "$POSTCHECK_LOG" 2>&1; then
      summary_record "postcheck" "postcheck" "passed" "$POSTCHECK_CMD" "$POSTCHECK_LOG"
      echo "  postcheck: passed"
    else
      summary_record "postcheck" "postcheck" "failed" "$POSTCHECK_CMD" "$POSTCHECK_LOG" "" "postcheck exit=$?"
      echo "  postcheck: FAILED" >&2
      if [[ "$OVERALL_STATUS" == "passed" ]]; then
        OVERALL_STATUS="partial"
      fi
    fi
  fi
else
  echo "--- Live scenarios skipped (--skip-live) ---"
  summary_readiness "0" "0" "skipped (--skip-live)" "skipped (--skip-live)"

  LIVE_IDS=(
    "live-happy-multi-range"
    "live-invalid-proof"
    "live-invalid-plaintext"
    "live-requester-refuses-payment"
  )
  for lid in "${LIVE_IDS[@]}"; do
    summary_record "$lid" "live-chain" "skipped" "" "" "" "skipped (--skip-live)"
  done
  summary_record "postcheck" "postcheck" "skipped" "" "" "" "skipped (--skip-live)"

  if [[ "$OVERALL_STATUS" == "passed" ]]; then
    OVERALL_STATUS="partial"
  fi
fi

summary_finish

echo ""
echo "========================================"
echo "Overall status: $OVERALL_STATUS"
echo "Summary JSON:   $SUMMARY_JSON"
echo "Summary MD:     $SUMMARY_MD"
echo "========================================"

if [[ "$OVERALL_STATUS" == "failed" ]]; then
  exit 1
fi
