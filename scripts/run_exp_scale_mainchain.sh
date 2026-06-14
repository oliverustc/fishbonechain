#!/usr/bin/env bash
# N=1..6 子链线性扩展 + 主链负载实验。
#
# 运行位置：需要能访问 10.2.2.x RPC 的机器，推荐 bcg。
#
# 输出：
#   ${OUT_DIR}/exp_scale_main_n<N>_state.csv
#   ${OUT_DIR}/exp_scale_main_n<N>_epoch.csv
#   ${OUT_DIR}/exp_scale_main_n<N>_main_blocks.csv
#   ${LOG_DIR}/...

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RUN_ID="${RUN_ID:-$(date +%Y%m%d_%H%M%S)}"
OUT_DIR="${OUT_DIR:-/tmp/fishbone_scale_mainchain_${RUN_ID}}"
LOG_DIR="${LOG_DIR:-$HOME/exp_scale_mainchain_logs/${RUN_ID}}"

MAIN_WS="${MAIN_WS:-ws://10.2.2.11:9944}"
REQUESTER="${REQUESTER:-5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY}"

WORKERS="${WORKERS:-50}"
RATE="${RATE:-0.005}"
REWARD="${REWARD:-1000000000}"      # 0.001 UNIT，适配 child2/child5 的小预算任务
DATA_SIZE="${DATA_SIZE:-800}"
METRICS_INTERVAL="${METRICS_INTERVAL:-15}"
MAIN_INTERVAL="${MAIN_INTERVAL:-6}"

N_START="${N_START:-1}"
N_END="${N_END:-6}"

# 默认采集 3 个完整 epoch 左右，并额外留出切换/出块缓冲。
DURATION_N1="${DURATION_N1:-3900}"  # child4 20min epoch -> 约 65min
DURATION_N2="${DURATION_N2:-3900}"
DURATION_N3="${DURATION_N3:-3900}"
DURATION_N4="${DURATION_N4:-5700}"  # child3 30min epoch -> 约 95min
DURATION_N5="${DURATION_N5:-5700}"
DURATION_N6="${DURATION_N6:-5700}"

declare -A WS=(
  [child1]="ws://10.2.2.11:9945"
  [child2]="ws://10.2.2.14:9946"
  [child3]="ws://10.2.2.17:9947"
  [child4]="ws://10.2.2.11:9948"
  [child5]="ws://10.2.2.20:9949"
  [child6]="ws://10.2.2.11:9950"
)

declare -A TASK_ID=(
  [child1]="0"
  [child2]="1"
  [child3]="2"
  [child4]="3"
  [child5]="4"
  [child6]="5"
)

declare -A CHAIN_ID=(
  [child1]="0"
  [child2]="1"
  [child3]="2"
  [child4]="3"
  [child5]="4"
  [child6]="5"
)

F1="0x52390bf081065e3ff296ab72c42bc234cedbdf9ddc40b6c7b6aee5fd01e08880"
F2="0x17f21fff17006faad4fa003c3db215718fc4d3bbc86054a435666affe704e71b"
F3="0x4151713ff93e1333474f1380cb2bc4fce9183942790830c1e8f48d3752e232fc"
F4="0x8b7aeb4590e1607db466c3cea45b4096f0b912364da41689f1be5166df3fee83"
F5="0x69416e5975a353736603d14d57e13efdadab8dd11667498d7892588abf50e70a"
F6="0x91bd2803edfcbb7e8d7f06c7df94f98d26300e56b968ee51855d994e4308d1e8"
F7="0x92ed7c0c05a5b080b5193514043e2bbd33401e2428e50e85cfbd2a20558b5652"
F8="0xb9b4d65352af6ab4f5c7bf0b765d17053cb5e1c39868a8ff7b5600340f114d56"
F9="0xdd20f92b0d61c5dd4ba76aac2c7d2e9957746adc5dd45d4dc3339f42f7ae1c4b"
F10="0xcb829a57912d649c46808a673d2f466b9f954208ab15ddd748567af6bbf81082"
F11="0x6b006d6f22d84f120c61d9f4366bc0d2390472aad7b0345c17a51ccf3a1538d4"
F12="0xf20ecd8e0f4aabc67e991ca6be62522b37cdf256819498d84bafea34d5146817"

declare -A MINERS=(
  [child1]="${F1},${F2},${F3}"
  [child2]="${F4},${F5},${F6}"
  [child3]="${F7},${F8},${F9}"
  [child4]="${F1},${F2},${F3},${F4},${F5},${F6},${F7}"
  [child5]="${F10},${F11},${F12}"
  [child6]="${F1},${F2},${F3},${F4},${F5}"
)

ORDER=(child4 child1 child6 child3 child2 child5)
PIDS=()

mkdir -p "$OUT_DIR" "$LOG_DIR"

log() {
  printf '[scale-main %s] %s\n' "$(date --iso-8601=seconds)" "$*"
}

duration_for_n() {
  local n="$1"
  local var="DURATION_N${n}"
  printf '%s' "${!var}"
}

active_for_n() {
  local n="$1"
  printf '%s\n' "${ORDER[@]:0:n}"
}

kill_pid() {
  local pid="$1"
  if kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null || true
  fi
}

cleanup() {
  log "清理当前阶段进程..."
  for pid in "${PIDS[@]:-}"; do
    kill_pid "$pid"
  done
}
trap cleanup INT TERM

write_meta() {
  cat > "${OUT_DIR}/meta.txt" <<EOF
run_id=${RUN_ID}
started_at=$(date --iso-8601=seconds)
main_ws=${MAIN_WS}
workers=${WORKERS}
rate=${RATE}
reward_planck=${REWARD}
data_size=${DATA_SIZE}
metrics_interval=${METRICS_INTERVAL}
main_interval=${MAIN_INTERVAL}
n_start=${N_START}
n_end=${N_END}
order=${ORDER[*]}
EOF
}

run_one_n() {
  local n="$1"
  local duration="$2"
  local prefix="${OUT_DIR}/exp_scale_main_n${n}"
  local active=()
  mapfile -t active < <(active_for_n "$n")

  local urls
  urls="$(IFS=,; for c in "${active[@]}"; do printf '%s,' "${WS[$c]}"; done)"
  urls="${urls%,}"

  log "N=${n} 开始：active=${active[*]} duration=${duration}s"
  rm -f "${prefix}_state.csv" "${prefix}_epoch.csv" "${prefix}_main_blocks.csv"
  PIDS=()

  nohup env MAIN_WS="$MAIN_WS" \
    node "${SCRIPT_DIR}/metrics_main.js" \
    --out "$prefix" \
    --interval "$MAIN_INTERVAL" \
    > "${LOG_DIR}/n${n}_main_metrics.log" 2>&1 &
  PIDS+=("$!")

  nohup node "${SCRIPT_DIR}/metrics.js" \
    --chains "$urls" \
    --out "$prefix" \
    --interval "$METRICS_INTERVAL" \
    > "${LOG_DIR}/n${n}_child_metrics.log" 2>&1 &
  PIDS+=("$!")

  sleep 5

  for child in "${active[@]}"; do
    nohup env \
      CHILD_WS="${WS[$child]}" \
      MAIN_WS="$MAIN_WS" \
      MINER_SURIS="${MINERS[$child]}" \
      REQUESTER="$REQUESTER" \
      TASK_ID="${TASK_ID[$child]}" \
      CHAIN_ID="${CHAIN_ID[$child]}" \
      node "${SCRIPT_DIR}/bridge.js" \
      > "${LOG_DIR}/n${n}_bridge_${child}.log" 2>&1 &
    PIDS+=("$!")
  done

  sleep 5

  local worker_pids=()
  for child in "${active[@]}"; do
    nohup node "${SCRIPT_DIR}/worker.js" \
      --ws "${WS[$child]}" \
      --task-id "${TASK_ID[$child]}" \
      --workers "$WORKERS" \
      --rate "$RATE" \
      --reward "$REWARD" \
      --data-size "$DATA_SIZE" \
      --duration "$duration" \
      > "${LOG_DIR}/n${n}_worker_${child}.log" 2>&1 &
    worker_pids+=("$!")
  done

  local failed=0
  for pid in "${worker_pids[@]}"; do
    if ! wait "$pid"; then
      log "WARN: worker pid=${pid} 非零退出"
      failed=1
    fi
  done

  log "N=${n} workers 完成，额外等待 30s 让 metrics/bridge 落盘"
  sleep 30
  cleanup
  sleep 5
  PIDS=()

  log "N=${n} 完成：prefix=${prefix} failed=${failed}"
}

write_meta

log "实验启动：OUT_DIR=${OUT_DIR}"
log "日志目录：LOG_DIR=${LOG_DIR}"
log "统一 workload：workers=${WORKERS}, rate=${RATE}, reward=${REWARD}, data_size=${DATA_SIZE}"

for n in $(seq "$N_START" "$N_END"); do
  run_one_n "$n" "$(duration_for_n "$n")"
done

echo "finished_at=$(date --iso-8601=seconds)" >> "${OUT_DIR}/meta.txt"
log "全部完成：${OUT_DIR}"
