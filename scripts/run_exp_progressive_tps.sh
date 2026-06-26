#!/usr/bin/env bash
# Progressive six-child-chain TPS and mainchain-pressure experiment.
#
# This launcher keeps the final metric contract fixed:
#   - child workload: accepted crowdsource business submissions per second
#   - mainchain load: bridge-specific accepted records during the same window
#
# N=1..3 use the tuned baseline crowdsource path. N=4..6 use the same pressure
# harness against progressively optimized child runtimes once those runtimes are
# deployed at the configured endpoints.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

RUN_ID="${RUN_ID:-progressive_tps_$(date +%Y%m%d_%H%M%S)}"
BASE_DIR="${BASE_DIR:-${REPO_DIR}/docs/experiments/progressive_tps}"
RUN_DIR="${RUN_DIR:-${BASE_DIR}/progressive_tps_runs/${RUN_ID}}"
LOG_DIR="${LOG_DIR:-${RUN_DIR}/logs}"
PROFILE_FILE="${PROFILE_FILE:-${REPO_DIR}/scripts/profiles/progressive_tps.json}"
DEPLOY_CONFIG="${DEPLOY_CONFIG:-${REPO_DIR}/deploy/config.toml}"
CONTROL_PYTHON="${CONTROL_PYTHON:-${REPO_DIR}/deploy/.venv/bin/python}"
CONTROL_SCRIPT="${CONTROL_SCRIPT:-${REPO_DIR}/deploy/cmd/control.py}"
DEPLOY_SCRIPT="${DEPLOY_SCRIPT:-${REPO_DIR}/deploy/cmd/deploy.py}"

MAIN_WS="${MAIN_WS:-ws://10.2.2.11:9944}"
WORKERS="${WORKERS:-}"
PARALLEL_PER_WORKER="${PARALLEL_PER_WORKER:-}"
REWARD="${REWARD:-0}"
DATA_SIZE="${DATA_SIZE:-8}"
BATCH_SIZE="${BATCH_SIZE:-}"
DURATION="${DURATION:-}"
SUBMIT_MODE="${SUBMIT_MODE:-pool}"
REPORT_INTERVAL="${REPORT_INTERVAL:-5}"
MAIN_INTERVAL="${MAIN_INTERVAL:-3}"
CAPACITY_CAP="${CAPACITY_CAP:-10000}"
CAPACITY_MONITOR_INTERVAL_MS="${CAPACITY_MONITOR_INTERVAL_MS:-200}"
CAPACITY_MONITOR_TIMEOUT="${CAPACITY_MONITOR_TIMEOUT:-300}"
RESET_EACH_STAGE="${RESET_EACH_STAGE:-0}"
SETUP_EACH_STAGE="${SETUP_EACH_STAGE:-$RESET_EACH_STAGE}"
SETUP_MAX_WORKERS="${SETUP_MAX_WORKERS:-0}"
WAIT_COLLECTING="${WAIT_COLLECTING:-$RESET_EACH_STAGE}"
WAIT_MAX_SUBS="${WAIT_MAX_SUBS:-50}"
WAIT_MIN_REMAINING_BLOCKS="${WAIT_MIN_REMAINING_BLOCKS:-300}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-900}"
STOP_ALL_CHILDREN_BEFORE_RUN="${STOP_ALL_CHILDREN_BEFORE_RUN:-1}"
START_ACTIVE_CHILDREN_EACH_STAGE="${START_ACTIVE_CHILDREN_EACH_STAGE:-1}"
SETUP_MAINCHAIN_FOR_BRIDGE="${SETUP_MAINCHAIN_FOR_BRIDGE:-1}"
RESET_MAINCHAIN_EACH_STAGE_FOR_BRIDGE="${RESET_MAINCHAIN_EACH_STAGE_FOR_BRIDGE:-0}"
RUN_BRIDGES_FOR_STAGE="${RUN_BRIDGES_FOR_STAGE:-1}"
BRIDGE_EXIT_AFTER_EVENTS="${BRIDGE_EXIT_AFTER_EVENTS:-1}"
FINALIZE_EPOCHS_FOR_BRIDGE="${FINALIZE_EPOCHS_FOR_BRIDGE:-1}"
FINALIZE_WAIT_SYNCING_SECONDS="${FINALIZE_WAIT_SYNCING_SECONDS:-900}"

N_START="${N_START:-1}"
N_END="${N_END:-6}"

while (($#)); do
  case "$1" in
    --stage)
      if [[ $# -lt 2 ]]; then
        echo "usage: $0 [--stage n1..n6] [--n-start 1] [--n-end 6] [--profile-file path] [--deploy-config path]" >&2
        exit 2
      fi
      stage="$2"
      if [[ ! "$stage" =~ ^n([1-6])$ ]]; then
        echo "invalid stage: ${stage}; expected n1..n6" >&2
        exit 2
      fi
      N_START="${BASH_REMATCH[1]}"
      N_END="${BASH_REMATCH[1]}"
      shift 2
      ;;
    --n-start)
      if [[ $# -lt 2 ]]; then
        echo "--n-start requires a value" >&2
        exit 2
      fi
      N_START="$2"
      shift 2
      ;;
    --n-end)
      if [[ $# -lt 2 ]]; then
        echo "--n-end requires a value" >&2
        exit 2
      fi
      N_END="$2"
      shift 2
      ;;
    --profile-file)
      if [[ $# -lt 2 ]]; then
        echo "--profile-file requires a path" >&2
        exit 2
      fi
      PROFILE_FILE="$2"
      shift 2
      ;;
    --deploy-config)
      if [[ $# -lt 2 ]]; then
        echo "--deploy-config requires a path" >&2
        exit 2
      fi
      DEPLOY_CONFIG="$2"
      shift 2
      ;;
    --no-stop-all-children)
      STOP_ALL_CHILDREN_BEFORE_RUN=0
      shift
      ;;
    --no-start-active-children)
      START_ACTIVE_CHILDREN_EACH_STAGE=0
      shift
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

declare -A WS=(
  [child1]="${CHILD1_WS:-ws://10.2.2.11:9945}"
  [child2]="${CHILD2_WS:-ws://10.2.2.14:9946}"
  [child3]="${CHILD3_WS:-ws://10.2.2.17:9947}"
  [child4]="${CHILD4_WS:-ws://10.2.2.11:9948}"
  [child5]="${CHILD5_WS:-ws://10.2.2.20:9949}"
  [child6]="${CHILD6_WS:-ws://10.2.2.11:9950}"
)

declare -A TASK_ID=(
  [child1]="${CHILD1_TASK_ID:-0}"
  [child2]="${CHILD2_TASK_ID:-1}"
  [child3]="${CHILD3_TASK_ID:-2}"
  [child4]="${CHILD4_TASK_ID:-3}"
  [child5]="${CHILD5_TASK_ID:-4}"
  [child6]="${CHILD6_TASK_ID:-5}"
)

declare -A BRIDGE_MINER_SURI=(
  [child1]="${CHILD1_BRIDGE_MINER_SURI:-0x52390bf081065e3ff296ab72c42bc234cedbdf9ddc40b6c7b6aee5fd01e08880}"
  [child2]="${CHILD2_BRIDGE_MINER_SURI:-0x8b7aeb4590e1607db466c3cea45b4096f0b912364da41689f1be5166df3fee83}"
  [child3]="${CHILD3_BRIDGE_MINER_SURI:-0x92ed7c0c05a5b080b5193514043e2bbd33401e2428e50e85cfbd2a20558b5652}"
  [child4]="${CHILD4_BRIDGE_MINER_SURI:-0xcb829a57912d649c46808a673d2f466b9f954208ab15ddd748567af6bbf81082}"
  [child5]="${CHILD5_BRIDGE_MINER_SURI:-//Alice//f13}"
  [child6]="${CHILD6_BRIDGE_MINER_SURI:-//Alice//f16}"
)

ORDER=(child1 child2 child3 child4 child5 child6)
PIDS=()
BRIDGE_PIDS=()

declare -A DEFAULT_WORKERS=(
  [1]=160
  [2]=180
  [3]=220
  [4]=260
  [5]=320
  [6]=400
)

declare -A DEFAULT_PARALLEL_PER_WORKER=(
  [1]=4
  [2]=4
  [3]=5
  [4]=5
  [5]=6
  [6]=8
)

declare -A DEFAULT_DURATION=(
  [1]=180
  [2]=180
  [3]=210
  [4]=210
  [5]=240
  [6]=240
)

declare -A DEFAULT_BATCH_SIZE=(
  [1]=1
  [2]=1
  [3]=1
  [4]=1
  [5]=1
  [6]=4
)

load_profile_defaults() {
  local profile="$1"
  if [[ ! -f "$profile" ]]; then
    echo "profile file not found: $profile" >&2
    exit 2
  fi

  while IFS=$'\t' read -r child ws task_id; do
    [[ -n "${child:-}" ]] || continue
    local ws_env="${child^^}_WS"
    local task_env="${child^^}_TASK_ID"
    if [[ -n "${ws:-}" && -z "${!ws_env:-}" ]]; then
      WS[$child]="$ws"
    fi
    if [[ -n "${task_id:-}" && -z "${!task_env:-}" ]]; then
      TASK_ID[$child]="$task_id"
    fi
  done < <(node - "$profile" <<'NODE'
const fs = require("fs");
const path = process.argv[2];
const raw = JSON.parse(fs.readFileSync(path, "utf8"));
const profiles = raw.chains || raw;
for (const [child, profile] of Object.entries(profiles)) {
  const ws = profile.defaultWs || "";
  const taskId = Number.isInteger(profile.taskId) ? String(profile.taskId) : "";
  process.stdout.write(`${child}\t${ws}\t${taskId}\n`);
}
NODE
  )
}

mkdir -p "$RUN_DIR" "$LOG_DIR"
load_profile_defaults "$PROFILE_FILE"
DEPLOY_CONFIG="$(cd "$(dirname "$DEPLOY_CONFIG")" && pwd)/$(basename "$DEPLOY_CONFIG")"

log() {
  printf '[progressive-tps %s] %s\n' "$(date --iso-8601=seconds)" "$*"
}

active_for_n() {
	local n="$1"
	printf '%s\n' "${ORDER[@]:0:n}"
}

join_by_comma() {
	local IFS=","
	echo "$*"
}

stage_key_for_n() {
  case "$1" in
    1|2|3) printf 'baseline-tuned' ;;
    4) printf 'runtime-v1' ;;
    5) printf 'runtime-v2' ;;
    6) printf 'runtime-v3' ;;
    *) printf 'unknown' ;;
  esac
}

batch_size_for_child() {
  local child="$1"
  local stage_default="$2"

  if [[ -n "${BATCH_SIZE:-}" ]]; then
    printf '%s' "$BATCH_SIZE"
  elif [[ "$child" == "child6" ]]; then
    printf '%s' "$stage_default"
  else
    printf '1'
  fi
}

control_children() {
  local action="$1"
  local chains_csv="${2:-}"
  local log_path="$3"

  if [[ "$action" == "stop-all-children" ]]; then
    "$CONTROL_PYTHON" "$CONTROL_SCRIPT" stop-all-children \
      --config "$DEPLOY_CONFIG" \
      > "$log_path" 2>&1
    return
  fi

  "$CONTROL_PYTHON" "$CONTROL_SCRIPT" "$action" \
    --config "$DEPLOY_CONFIG" \
    --chains "$chains_csv" \
    > "$log_path" 2>&1
}

deploy_children() {
  local chains_csv="$1"
  local log_path="$2"

  "$CONTROL_PYTHON" "$DEPLOY_SCRIPT" \
    --config "$DEPLOY_CONFIG" \
    --profile-file "$PROFILE_FILE" \
    --chains "$chains_csv" \
    > "$log_path" 2>&1
}

setup_mainchain_for_bridge() {
  local chains_csv="$1"
  local log_path="$2"

  MAIN_WS="$MAIN_WS" node "${SCRIPT_DIR}/setup_progressive_mainchain.js" \
    --profile-file "$PROFILE_FILE" \
    --chains "$chains_csv" \
    > "$log_path" 2>&1
}

start_bridges_for_stage() {
  local n="$1"
  shift
  local active=("$@")
  BRIDGE_PIDS=()

  for child in "${active[@]}"; do
    local idx="${child#child}"
    local chain_id=$((idx - 1))
    nohup env CHILD_WS="${WS[$child]}" MAIN_WS="$MAIN_WS" TASK_ID="${TASK_ID[$child]}" CHAIN_ID="$chain_id" MINER_SURI="${BRIDGE_MINER_SURI[$child]}" \
      node "${REPO_DIR}/scripts/bridges/crowdsource.js" \
        --exit-after-events "$BRIDGE_EXIT_AFTER_EVENTS" \
      > "${LOG_DIR}/n${n}_bridge_${child}.log" 2>&1 &
    BRIDGE_PIDS+=("$!")
  done
}

stop_bridges_for_stage() {
  for pid in "${BRIDGE_PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  for pid in "${BRIDGE_PIDS[@]:-}"; do
    wait "$pid" 2>/dev/null || true
  done
  BRIDGE_PIDS=()
}

cleanup() {
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  stop_bridges_for_stage
}
trap cleanup INT TERM

write_meta() {
  {
    echo "run_id=${RUN_ID}"
    echo "started_at=$(date --iso-8601=seconds)"
    echo "profile_file=${PROFILE_FILE}"
    echo "deploy_config=${DEPLOY_CONFIG}"
    echo "main_ws=${MAIN_WS}"
    echo "workers_override=${WORKERS:-}"
    echo "parallel_per_worker_override=${PARALLEL_PER_WORKER:-}"
    echo "reward_planck=${REWARD}"
    echo "data_size=${DATA_SIZE}"
    echo "batch_size_override=${BATCH_SIZE:-}"
    echo "duration_override=${DURATION:-}"
    echo "submit_mode=${SUBMIT_MODE}"
    echo "main_interval=${MAIN_INTERVAL}"
    echo "capacity_cap=${CAPACITY_CAP}"
    echo "capacity_monitor_interval_ms=${CAPACITY_MONITOR_INTERVAL_MS}"
    echo "capacity_monitor_timeout=${CAPACITY_MONITOR_TIMEOUT}"
    echo "reset_each_stage=${RESET_EACH_STAGE}"
    echo "setup_each_stage=${SETUP_EACH_STAGE}"
    echo "setup_max_workers=${SETUP_MAX_WORKERS}"
    echo "wait_collecting=${WAIT_COLLECTING}"
    echo "wait_max_subs=${WAIT_MAX_SUBS}"
    echo "wait_min_remaining_blocks=${WAIT_MIN_REMAINING_BLOCKS}"
    echo "wait_timeout=${WAIT_TIMEOUT}"
    echo "stop_all_children_before_run=${STOP_ALL_CHILDREN_BEFORE_RUN}"
    echo "start_active_children_each_stage=${START_ACTIVE_CHILDREN_EACH_STAGE}"
    echo "setup_mainchain_for_bridge=${SETUP_MAINCHAIN_FOR_BRIDGE}"
    echo "reset_mainchain_each_stage_for_bridge=${RESET_MAINCHAIN_EACH_STAGE_FOR_BRIDGE}"
    echo "run_bridges_for_stage=${RUN_BRIDGES_FOR_STAGE}"
    echo "bridge_exit_after_events=${BRIDGE_EXIT_AFTER_EVENTS}"
    echo "finalize_epochs_for_bridge=${FINALIZE_EPOCHS_FOR_BRIDGE}"
    echo "finalize_wait_syncing_seconds=${FINALIZE_WAIT_SYNCING_SECONDS}"
    echo "n_start=${N_START}"
    echo "n_end=${N_END}"
    echo "order=${ORDER[*]}"
    for child in "${ORDER[@]}"; do
      echo "${child}_ws=${WS[$child]}"
      echo "${child}_task_id=${TASK_ID[$child]}"
      echo "${child}_bridge_miner_suri=${BRIDGE_MINER_SURI[$child]}"
    done
  } > "${RUN_DIR}/meta.txt"
  cp "$PROFILE_FILE" "${RUN_DIR}/profile_manifest.json"
}

run_one_n() {
  local n="$1"
  local prefix="${RUN_DIR}/progressive_tps_n${n}"
  local stage_key
  stage_key="$(stage_key_for_n "$n")"
  local stage_workers="${WORKERS:-${DEFAULT_WORKERS[$n]}}"
  local stage_parallel="${PARALLEL_PER_WORKER:-${DEFAULT_PARALLEL_PER_WORKER[$n]}}"
  local stage_duration="${DURATION:-${DEFAULT_DURATION[$n]}}"
  local stage_batch_size="${BATCH_SIZE:-${DEFAULT_BATCH_SIZE[$n]}}"
  local active=()
  mapfile -t active < <(active_for_n "$n")

  local urls=""
  for child in "${active[@]}"; do
    urls+="${WS[$child]},"
  done
  urls="${urls%,}"

  log "N=${n} stage=${stage_key} active=${active[*]} workers=${stage_workers} parallel=${stage_parallel} batch_default=${stage_batch_size} duration=${stage_duration}s"

  local active_csv
  active_csv="$(join_by_comma "${active[@]}")"

  if [[ "$START_ACTIVE_CHILDREN_EACH_STAGE" == "1" && "$RESET_EACH_STAGE" != "1" ]]; then
    log "N=${n} start active child services: ${active_csv}"
    control_children start "$active_csv" "${LOG_DIR}/n${n}_start_active_children.log"
  fi

  if [[ "$RESET_EACH_STAGE" == "1" ]]; then
    log "N=${n} stop-clean active chains: ${active_csv}"
    control_children stop-clean "$active_csv" "${LOG_DIR}/n${n}_stop_clean.log"
    log "N=${n} deploy active chains: ${active_csv}"
    deploy_children "$active_csv" "${LOG_DIR}/n${n}_deploy.log"
  fi

  if [[ "$RESET_MAINCHAIN_EACH_STAGE_FOR_BRIDGE" == "1" ]]; then
    log "N=${n} stop-clean mainchain for bridge state"
    control_children stop-clean "main" "${LOG_DIR}/n${n}_main_stop_clean.log"
    log "N=${n} deploy mainchain for bridge state"
    deploy_children "main" "${LOG_DIR}/n${n}_main_deploy.log"
  fi

  if [[ "$SETUP_MAINCHAIN_FOR_BRIDGE" == "1" ]]; then
    log "N=${n} setup mainchain bridge state: ${active_csv}"
    setup_mainchain_for_bridge "$active_csv" "${LOG_DIR}/n${n}_setup_mainchain_bridge.log"
  fi

  if [[ "$SETUP_EACH_STAGE" == "1" ]]; then
    log "N=${n} setup active chains: ${active_csv}"
    node "${SCRIPT_DIR}/setup_selected_child_chains.js" \
      --chains "$active_csv" \
      --profile-file "$PROFILE_FILE" \
      --max-workers "$SETUP_MAX_WORKERS" \
      > "${LOG_DIR}/n${n}_setup.log" 2>&1
  fi

  if [[ "$WAIT_COLLECTING" == "1" ]]; then
    log "N=${n} wait Collecting with clean submissions"
    node "${SCRIPT_DIR}/wait_collecting.js" \
      --chains "$urls" \
      --max-subs "$WAIT_MAX_SUBS" \
      --min-remaining-collecting-blocks "$WAIT_MIN_REMAINING_BLOCKS" \
      --interval 5 \
      --timeout "$WAIT_TIMEOUT" \
      > "${LOG_DIR}/n${n}_wait_collecting.log" 2>&1
  fi

  rm -f "${prefix}_child_precise.csv" "${prefix}_child_precise_summary.json" "${prefix}_main_blocks.csv"
  rm -f "${prefix}_monitor.ready" "${prefix}_monitor.start"
  PIDS=()

  nohup env MAIN_WS="$MAIN_WS" \
    node "${SCRIPT_DIR}/metrics_main.js" \
    --out "$prefix" \
    --interval "$MAIN_INTERVAL" \
    > "${LOG_DIR}/n${n}_main_metrics.log" 2>&1 &
  local main_metrics_pid="$!"
  PIDS+=("$main_metrics_pid")

  if [[ "$RUN_BRIDGES_FOR_STAGE" == "1" ]]; then
    log "N=${n} start crowdsource bridges"
    start_bridges_for_stage "$n" "${active[@]}"
  fi

  nohup node "${SCRIPT_DIR}/capacity_monitor.js" \
    --chains "$urls" \
    --out "${prefix}_child_precise.csv" \
    --ready-file "${prefix}_monitor.ready" \
    --start-file "${prefix}_monitor.start" \
    --cap "$CAPACITY_CAP" \
    --interval-ms "$CAPACITY_MONITOR_INTERVAL_MS" \
    --timeout "$CAPACITY_MONITOR_TIMEOUT" \
    > "${LOG_DIR}/n${n}_capacity_monitor.log" 2>&1 &
  local monitor_pid="$!"
  PIDS+=("$monitor_pid")

  local ready_deadline=$((SECONDS + 120))
  while [[ ! -f "${prefix}_monitor.ready" ]]; do
    if (( SECONDS > ready_deadline )); then
      log "ERROR: capacity monitor not ready for N=${n}"
      return 1
    fi
    sleep 0.1
  done

  node -e 'process.stdout.write(String(Date.now()))' > "${prefix}_monitor.start"

  local worker_pids=()
  for child in "${active[@]}"; do
    local child_batch_size
    child_batch_size="$(batch_size_for_child "$child" "$stage_batch_size")"
    nohup node "${SCRIPT_DIR}/worker_burst.js" \
      --ws "${WS[$child]}" \
      --task-id "${TASK_ID[$child]}" \
      --workers "$stage_workers" \
      --parallel-per-worker "$stage_parallel" \
      --reward "$REWARD" \
      --data-size "$DATA_SIZE" \
      --batch-size "$child_batch_size" \
      --duration "$stage_duration" \
      --submit-mode "$SUBMIT_MODE" \
      --report-interval "$REPORT_INTERVAL" \
      > "${LOG_DIR}/n${n}_burst_${child}.log" 2>&1 &
    worker_pids+=("$!")
  done

  local failed=0
  if ! wait "$monitor_pid"; then
    log "WARN: capacity monitor exited nonzero for N=${n}"
    failed=1
  fi

  for pid in "${worker_pids[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
  sleep 5
  for pid in "${worker_pids[@]}"; do
    wait "$pid" 2>/dev/null || true
  done

  if [[ "$FINALIZE_EPOCHS_FOR_BRIDGE" == "1" ]]; then
    log "N=${n} finalize epochs for bridge measurement"
    node "${SCRIPT_DIR}/finalize_progressive_epochs.js" \
      --chains "$urls" \
      --out "${prefix}_epoch_finalize.json" \
      --wait-syncing-seconds "$FINALIZE_WAIT_SYNCING_SECONDS" \
      > "${LOG_DIR}/n${n}_finalize_epochs.log" 2>&1
    sleep 20
  fi

  stop_bridges_for_stage

  kill "$main_metrics_pid" 2>/dev/null || true
  wait "$main_metrics_pid" 2>/dev/null || true
  PIDS=()

  {
    echo "n=${n}"
    echo "stage_key=${stage_key}"
    echo "active=${active[*]}"
    echo "workers=${stage_workers}"
    echo "parallel_per_worker=${stage_parallel}"
    echo "duration=${stage_duration}"
    echo "batch_default=${stage_batch_size}"
    for child in "${active[@]}"; do
      echo "batch_size_${child}=$(batch_size_for_child "$child" "$stage_batch_size")"
    done
    echo "require_bridge_events=${RUN_BRIDGES_FOR_STAGE}"
    echo "failed=${failed}"
    echo "finished_at=$(date --iso-8601=seconds)"
  } > "${prefix}_stage.txt"
  log "N=${n} done failed=${failed}"
}

write_meta
log "RUN_DIR=${RUN_DIR}"
log "LOG_DIR=${LOG_DIR}"
log "workload overrides workers=${WORKERS:-auto} parallel=${PARALLEL_PER_WORKER:-auto} duration=${DURATION:-auto} submit_mode=${SUBMIT_MODE}"

if [[ "$STOP_ALL_CHILDREN_BEFORE_RUN" == "1" ]]; then
  log "preflight stop all child services in deploy config"
  control_children stop-all-children "" "${LOG_DIR}/preflight_stop_all_children.log"
fi

for n in $(seq "$N_START" "$N_END"); do
  run_one_n "$n"
done

python3 "${SCRIPT_DIR}/summarize_progressive_tps.py" \
  --runs "$RUN_DIR" \
  --log-dir "$LOG_DIR" \
  --n-start "$N_START" \
  --n-end "$N_END" \
  --out "${BASE_DIR}/progressive_tps_summary.csv"

python3 "${SCRIPT_DIR}/plot_progressive_tps.py" \
  --summary "${BASE_DIR}/progressive_tps_summary.csv" \
  --out-dir "${BASE_DIR}/figures"

echo "finished_at=$(date --iso-8601=seconds)" >> "${RUN_DIR}/meta.txt"
log "all done: ${RUN_DIR}"
