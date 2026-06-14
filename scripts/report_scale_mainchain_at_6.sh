#!/usr/bin/env bash
# Wait until 06:00, then collect the N=1..6 scaling run results and generate a report.
#
# Default target is the run started on 2026-06-11:
#   RUN_ID=scale_main_20260611_101435
#
# Usage:
#   nohup bash scripts/report_scale_mainchain_at_6.sh \
#     > /tmp/scale_mainchain_6am_report.log 2>&1 &

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

RUN_ID="${RUN_ID:-scale_main_20260611_101435}"
REMOTE_HOST="${REMOTE_HOST:-bcg}"
TARGET_TIME="${TARGET_TIME:-06:00}"
WAIT_UNTIL_TARGET="${WAIT_UNTIL_TARGET:-1}"

REMOTE_RAW_DIR="${REMOTE_RAW_DIR:-/tmp/fishbone_scale_mainchain_${RUN_ID}}"
REMOTE_LOG_DIR="${REMOTE_LOG_DIR:-/home/debian/exp_scale_mainchain_logs/${RUN_ID}}"

LOCAL_BASE="${LOCAL_BASE:-/tmp/fishbone_scale_mainchain_reports/${RUN_ID}}"
LOCAL_RAW_DIR="${LOCAL_BASE}/raw"
LOCAL_LOG_DIR="${LOCAL_BASE}/logs"

SUMMARY_CSV="${REPO_DIR}/docs/experiments/figures/data/exp_scale_mainchain_summary.csv"
REPORT_MD="${REPO_DIR}/docs/internal/agent-plans/scale-mainchain-6am-report-${RUN_ID}.md"
FIGURE="${REPO_DIR}/docs/experiments/figures/fig_scale_mainchain_load.png"

log() {
  printf '[6am-report %s] %s\n' "$(date --iso-8601=seconds)" "$*"
}

seconds_until_target() {
  python3 - "$TARGET_TIME" <<'PY'
from datetime import datetime, timedelta
import sys

target = sys.argv[1]
hour, minute = map(int, target.split(":"))
now = datetime.now()
deadline = now.replace(hour=hour, minute=minute, second=0, microsecond=0)
if deadline <= now:
    deadline += timedelta(days=1)
print(int((deadline - now).total_seconds()))
print(deadline.isoformat(timespec="seconds"))
PY
}

remote_status() {
  ssh -o BatchMode=yes "$REMOTE_HOST" \
    "pgrep -af 'fishbone_scripts/(run_exp_scale_mainchain|worker|metrics|metrics_main|bridge)' || true"
}

copy_remote_data() {
  mkdir -p "$LOCAL_BASE"
  rm -rf "$LOCAL_RAW_DIR" "$LOCAL_LOG_DIR"
  log "Copy raw data: ${REMOTE_HOST}:${REMOTE_RAW_DIR}"
  scp -r "${REMOTE_HOST}:${REMOTE_RAW_DIR}" "$LOCAL_RAW_DIR"
  log "Copy logs: ${REMOTE_HOST}:${REMOTE_LOG_DIR}"
  scp -r "${REMOTE_HOST}:${REMOTE_LOG_DIR}" "$LOCAL_LOG_DIR"
}

generate_outputs() {
  log "Summarize CSV"
  python3 "${REPO_DIR}/scripts/summarize_scale_mainchain.py" \
    --raw-dir "$LOCAL_RAW_DIR" \
    --log-dir "$LOCAL_LOG_DIR" \
    --out "$SUMMARY_CSV"

  log "Generate figure"
  python3 "${REPO_DIR}/scripts/plot_results.py" --fig-scale-main
}

write_report() {
  local status="$1"
  local generated_at
  generated_at="$(date --iso-8601=seconds)"

  {
    echo "# Scale Mainchain 6AM Report"
    echo
    echo "- generated_at: ${generated_at}"
    echo "- run_id: ${RUN_ID}"
    echo "- remote_status: ${status}"
    echo "- raw_dir: ${LOCAL_RAW_DIR}"
    echo "- log_dir: ${LOCAL_LOG_DIR}"
    echo "- summary_csv: ${SUMMARY_CSV}"
    echo "- figure: ${FIGURE}"
    echo
    echo "## Summary CSV"
    echo
    if [[ -f "$SUMMARY_CSV" ]]; then
      echo '```csv'
      cat "$SUMMARY_CSV"
      echo '```'
    else
      echo "Summary CSV not generated."
    fi
    echo
    echo "## Launcher Log Tail"
    echo
    echo '```text'
    ssh -o BatchMode=yes "$REMOTE_HOST" \
      "tail -n 80 /home/debian/exp_scale_mainchain_logs/${RUN_ID}_launcher.log 2>/dev/null || true"
    echo '```'
  } > "$REPORT_MD"

  log "Report written: $REPORT_MD"
  cat "$REPORT_MD"
}

main() {
  cd "$REPO_DIR"

  if [[ "$WAIT_UNTIL_TARGET" != "0" ]]; then
    mapfile -t target_info < <(seconds_until_target)
    local sleep_seconds="${target_info[0]}"
    local deadline="${target_info[1]}"
    log "Target time: ${deadline}; sleep ${sleep_seconds}s"
    sleep "$sleep_seconds"
  else
    log "WAIT_UNTIL_TARGET=0; run report immediately"
  fi

  log "Wake up; checking remote run"
  local status_text
  status_text="$(remote_status)"

  if [[ -n "$status_text" ]]; then
    log "Remote run still has active processes:"
    printf '%s\n' "$status_text"
    copy_remote_data
    generate_outputs || true
    write_report "still_running"
    exit 2
  fi

  log "Remote run appears finished"
  copy_remote_data
  generate_outputs
  write_report "finished"
}

main "$@"
