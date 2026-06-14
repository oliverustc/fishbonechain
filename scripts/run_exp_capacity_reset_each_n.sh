#!/usr/bin/env bash
# Capacity benchmark that resets active child chains before every N.
#
# This avoids waiting for a full long Collecting epoch after a previous run
# has filled MaxSubmissionsPerEpoch.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RUN_ID="${RUN_ID:-capacity_reset_each_n_$(date +%Y%m%d_%H%M%S)}"
OUT_DIR="${OUT_DIR:-/tmp/fishbone_capacity_${RUN_ID}}"
LOG_DIR="${LOG_DIR:-$HOME/exp_capacity_logs/${RUN_ID}}"

WORKERS="${WORKERS:-100}"
PARALLEL_PER_WORKER="${PARALLEL_PER_WORKER:-8}"
REWARD="${REWARD:-0}"
DATA_SIZE="${DATA_SIZE:-64}"
DURATION="${DURATION:-180}"
SUBMIT_MODE="${SUBMIT_MODE:-pool}"
CAPACITY_CAP="${CAPACITY_CAP:-10000}"
CAPACITY_MONITOR_INTERVAL_MS="${CAPACITY_MONITOR_INTERVAL_MS:-200}"
CAPACITY_MONITOR_TIMEOUT="${CAPACITY_MONITOR_TIMEOUT:-300}"
WAIT_MAX_SUBS="${WAIT_MAX_SUBS:-50}"
WAIT_MIN_REMAINING_BLOCKS="${WAIT_MIN_REMAINING_BLOCKS:-300}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-900}"
SETUP_MAX_WORKERS="${SETUP_MAX_WORKERS:-0}"

N_START="${N_START:-1}"
N_END="${N_END:-6}"

declare -A WS=(
  [child1]="${CHILD1_WS:-ws://10.2.2.11:9945}"
  [child2]="${CHILD2_WS:-ws://10.2.2.14:9946}"
  [child3]="${CHILD3_WS:-ws://10.2.2.17:9947}"
  [child4]="${CHILD4_WS:-ws://10.2.2.11:9948}"
  [child5]="${CHILD5_WS:-ws://10.2.2.20:9949}"
  [child6]="${CHILD6_WS:-ws://10.2.2.11:9950}"
)

declare -A TASK_ID=(
  [child1]="0"
  [child2]="1"
  [child3]="2"
  [child4]="3"
  [child5]="4"
  [child6]="5"
)

ORDER=(child4 child1 child6 child3 child2 child5)
PIDS=()

mkdir -p "$OUT_DIR" "$LOG_DIR"

log() {
  printf '[capacity-reset %s] %s\n' "$(date --iso-8601=seconds)" "$*"
}

active_for_n() {
  local n="$1"
  printf '%s\n' "${ORDER[@]:0:n}"
}

cleanup() {
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
}
trap cleanup INT TERM

write_meta() {
  cat > "${OUT_DIR}/meta.txt" <<EOF
run_id=${RUN_ID}
started_at=$(date --iso-8601=seconds)
workers=${WORKERS}
parallel_per_worker=${PARALLEL_PER_WORKER}
reward_planck=${REWARD}
data_size=${DATA_SIZE}
duration=${DURATION}
submit_mode=${SUBMIT_MODE}
capacity_cap=${CAPACITY_CAP}
capacity_monitor_interval_ms=${CAPACITY_MONITOR_INTERVAL_MS}
capacity_monitor_timeout=${CAPACITY_MONITOR_TIMEOUT}
wait_max_subs=${WAIT_MAX_SUBS}
wait_min_remaining_blocks=${WAIT_MIN_REMAINING_BLOCKS}
setup_max_workers=${SETUP_MAX_WORKERS}
collecting_slot_blocks=600
reset_each_n=true
order=${ORDER[*]}
runtime_limit=MaxSubmissionsPerEpoch=10000
EOF
}

join_by_comma() {
  local IFS=","
  echo "$*"
}

run_one_n() {
  local n="$1"
  local prefix="${OUT_DIR}/exp_capacity_n${n}"
  local active=()
  mapfile -t active < <(active_for_n "$n")

  log "N=${n} active=${active[*]}"
  log "reset active child chains"
  "${SCRIPT_DIR}/reset_child_chains.sh" "${active[@]}" > "${LOG_DIR}/n${n}_reset.log" 2>&1

  local active_csv
  active_csv="$(join_by_comma "${active[@]}")"
  log "setup active child chains: ${active_csv}"
  node "${SCRIPT_DIR}/setup_selected_child_chains.js" \
    --chains "$active_csv" \
    --max-workers "$SETUP_MAX_WORKERS" \
    > "${LOG_DIR}/n${n}_setup.log" 2>&1

  local urls=""
  for child in "${active[@]}"; do
    urls+="${WS[$child]},"
  done
  urls="${urls%,}"

  log "wait Collecting and clean submissions"
  node "${SCRIPT_DIR}/wait_collecting.js" \
    --chains "$urls" \
    --max-subs "$WAIT_MAX_SUBS" \
    --min-remaining-collecting-blocks "$WAIT_MIN_REMAINING_BLOCKS" \
    --interval 5 \
    --timeout "$WAIT_TIMEOUT" \
    > "${LOG_DIR}/n${n}_wait_collecting.log" 2>&1

  rm -f "${prefix}_precise.csv" "${prefix}_precise_summary.json" "${prefix}_monitor.ready" "${prefix}_monitor.start"
  PIDS=()

  nohup node "${SCRIPT_DIR}/capacity_monitor.js" \
    --chains "$urls" \
    --out "${prefix}_precise.csv" \
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
      --report-interval 5 \
      > "${LOG_DIR}/n${n}_burst_${child}.log" 2>&1 &
    worker_pids+=("$!")
  done

  local failed=0
  if ! wait "$monitor_pid"; then
    log "WARN: capacity monitor pid=${monitor_pid} failed or timed out"
    failed=1
  fi

  for pid in "${worker_pids[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
  sleep 2
  for pid in "${worker_pids[@]}"; do
    wait "$pid" 2>/dev/null || true
  done

  cleanup
  PIDS=()
  log "N=${n} done failed=${failed}"
}

write_meta
log "OUT_DIR=${OUT_DIR}"
log "LOG_DIR=${LOG_DIR}"

for n in $(seq "$N_START" "$N_END"); do
  run_one_n "$n"
done

echo "finished_at=$(date --iso-8601=seconds)" >> "${OUT_DIR}/meta.txt"
log "all done: ${OUT_DIR}"
