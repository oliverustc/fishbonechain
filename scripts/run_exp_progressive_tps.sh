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

MAIN_WS="${MAIN_WS:-ws://10.2.2.11:9944}"
WORKERS="${WORKERS:-160}"
PARALLEL_PER_WORKER="${PARALLEL_PER_WORKER:-4}"
REWARD="${REWARD:-0}"
DATA_SIZE="${DATA_SIZE:-64}"
DURATION="${DURATION:-180}"
SUBMIT_MODE="${SUBMIT_MODE:-pool}"
REPORT_INTERVAL="${REPORT_INTERVAL:-5}"
MAIN_INTERVAL="${MAIN_INTERVAL:-3}"
CAPACITY_CAP="${CAPACITY_CAP:-10000}"
CAPACITY_MONITOR_INTERVAL_MS="${CAPACITY_MONITOR_INTERVAL_MS:-200}"
CAPACITY_MONITOR_TIMEOUT="${CAPACITY_MONITOR_TIMEOUT:-300}"

N_START="${N_START:-1}"
N_END="${N_END:-6}"

while (($#)); do
  case "$1" in
    --stage)
      if [[ $# -lt 2 ]]; then
        echo "usage: $0 [--stage n1..n6] [--n-start 1] [--n-end 6] [--profile-file path]" >&2
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

ORDER=(child1 child2 child3 child4 child5 child6)
PIDS=()

mkdir -p "$RUN_DIR" "$LOG_DIR"

log() {
  printf '[progressive-tps %s] %s\n' "$(date --iso-8601=seconds)" "$*"
}

active_for_n() {
  local n="$1"
  printf '%s\n' "${ORDER[@]:0:n}"
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

cleanup() {
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
}
trap cleanup INT TERM

write_meta() {
  {
    echo "run_id=${RUN_ID}"
    echo "started_at=$(date --iso-8601=seconds)"
    echo "profile_file=${PROFILE_FILE}"
    echo "main_ws=${MAIN_WS}"
    echo "workers=${WORKERS}"
    echo "parallel_per_worker=${PARALLEL_PER_WORKER}"
    echo "reward_planck=${REWARD}"
    echo "data_size=${DATA_SIZE}"
    echo "duration=${DURATION}"
    echo "submit_mode=${SUBMIT_MODE}"
    echo "main_interval=${MAIN_INTERVAL}"
    echo "capacity_cap=${CAPACITY_CAP}"
    echo "capacity_monitor_interval_ms=${CAPACITY_MONITOR_INTERVAL_MS}"
    echo "capacity_monitor_timeout=${CAPACITY_MONITOR_TIMEOUT}"
    echo "n_start=${N_START}"
    echo "n_end=${N_END}"
    echo "order=${ORDER[*]}"
    for child in "${ORDER[@]}"; do
      echo "${child}_ws=${WS[$child]}"
      echo "${child}_task_id=${TASK_ID[$child]}"
    done
  } > "${RUN_DIR}/meta.txt"
  cp "$PROFILE_FILE" "${RUN_DIR}/profile_manifest.json"
}

run_one_n() {
  local n="$1"
  local prefix="${RUN_DIR}/progressive_tps_n${n}"
  local stage_key
  stage_key="$(stage_key_for_n "$n")"
  local active=()
  mapfile -t active < <(active_for_n "$n")

  local urls=""
  for child in "${active[@]}"; do
    urls+="${WS[$child]},"
  done
  urls="${urls%,}"

  log "N=${n} stage=${stage_key} active=${active[*]}"
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
    nohup node "${SCRIPT_DIR}/worker_burst.js" \
      --ws "${WS[$child]}" \
      --task-id "${TASK_ID[$child]}" \
      --workers "$WORKERS" \
      --parallel-per-worker "$PARALLEL_PER_WORKER" \
      --reward "$REWARD" \
      --data-size "$DATA_SIZE" \
      --duration "$DURATION" \
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

  kill "$main_metrics_pid" 2>/dev/null || true
  wait "$main_metrics_pid" 2>/dev/null || true
  PIDS=()

  {
    echo "n=${n}"
    echo "stage_key=${stage_key}"
    echo "active=${active[*]}"
    echo "failed=${failed}"
    echo "finished_at=$(date --iso-8601=seconds)"
  } > "${prefix}_stage.txt"
  log "N=${n} done failed=${failed}"
}

write_meta
log "RUN_DIR=${RUN_DIR}"
log "LOG_DIR=${LOG_DIR}"
log "workload workers=${WORKERS} parallel=${PARALLEL_PER_WORKER} duration=${DURATION}s submit_mode=${SUBMIT_MODE}"

for n in $(seq "$N_START" "$N_END"); do
  run_one_n "$n"
done

python3 "${SCRIPT_DIR}/summarize_progressive_tps.py" \
  --runs "$RUN_DIR" \
  --log-dir "$LOG_DIR" \
  --out "${BASE_DIR}/progressive_tps_summary.csv"

python3 "${SCRIPT_DIR}/plot_progressive_tps.py" \
  --summary "${BASE_DIR}/progressive_tps_summary.csv" \
  --out-dir "${BASE_DIR}/figures"

echo "finished_at=$(date --iso-8601=seconds)" >> "${RUN_DIR}/meta.txt"
log "all done: ${RUN_DIR}"
