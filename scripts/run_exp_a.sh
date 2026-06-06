#!/usr/bin/env bash
# 实验 A：基准测试
# 在 child1 (ws://10.2.2.11:9945) 上同时跑场景 a/b/c/e，
# 观察高频场景（b/e）在默认链上的容量瓶颈。
#
# 用法：
#   bash scripts/run_exp_a.sh           # 连接真实集群
#   CHILD1_WS=ws://127.0.0.1:9945 bash scripts/run_exp_a.sh  # 本地调试

set -euo pipefail

CHILD1_WS="${CHILD1_WS:-ws://10.2.2.11:9945}"
OUT_DIR="${OUT_DIR:-/tmp/exp_a}"
TASK_ID="${TASK_ID:-0}"
DURATION="${DURATION:-}"          # 空 = 无限；设为秒数则自动停止

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=== 实验 A：基准测试（child1 容量瓶颈） ==="
echo "子链 RPC : ${CHILD1_WS}"
echo "输出前缀 : ${OUT_DIR}"
echo "task_id  : ${TASK_ID}"
echo "时长     : ${DURATION:-无限（Ctrl+C 停止）}"
echo ""

DURATION_FLAG=""
if [[ -n "${DURATION}" ]]; then
  DURATION_FLAG="--duration ${DURATION}"
fi

# ── 启动指标采集 ──────────────────────────────────────────────────────────────
echo "[exp_a] 启动 metrics.js..."
node "${SCRIPT_DIR}/metrics.js" \
  --chains "${CHILD1_WS}" \
  --out "${OUT_DIR}" \
  --interval 15 &
METRICS_PID=$!
echo "[exp_a] metrics.js PID=${METRICS_PID}"

sleep 2   # 等 metrics 先连上链

# ── 启动 4 个场景（a=快递基准，b=高频交通，c=长周期医疗，e=超高频传感器）─────
echo "[exp_a] 启动场景 a（快递，300 workers）..."
node "${SCRIPT_DIR}/worker.js" \
  --scenario a \
  --ws "${CHILD1_WS}" \
  --task-id "${TASK_ID}" \
  ${DURATION_FLAG} \
  2>&1 | sed 's/^/[a] /' &
PID_A=$!

echo "[exp_a] 启动场景 b（交通，2000 workers）..."
node "${SCRIPT_DIR}/worker.js" \
  --scenario b \
  --ws "${CHILD1_WS}" \
  --task-id "${TASK_ID}" \
  ${DURATION_FLAG} \
  2>&1 | sed 's/^/[b] /' &
PID_B=$!

echo "[exp_a] 启动场景 c（医疗，200 workers）..."
node "${SCRIPT_DIR}/worker.js" \
  --scenario c \
  --ws "${CHILD1_WS}" \
  --task-id "${TASK_ID}" \
  ${DURATION_FLAG} \
  2>&1 | sed 's/^/[c] /' &
PID_C=$!

echo "[exp_a] 启动场景 e（传感器，5000 workers）..."
node "${SCRIPT_DIR}/worker.js" \
  --scenario e \
  --ws "${CHILD1_WS}" \
  --task-id "${TASK_ID}" \
  ${DURATION_FLAG} \
  2>&1 | sed 's/^/[e] /' &
PID_E=$!

echo ""
echo "[exp_a] 所有进程已启动："
echo "  metrics PID=${METRICS_PID}"
echo "  scenario_a PID=${PID_A}"
echo "  scenario_b PID=${PID_B}"
echo "  scenario_c PID=${PID_C}"
echo "  scenario_e PID=${PID_E}"
echo ""
echo "[exp_a] 等待 3 个 Epoch（约 36 min = 3×12 min）后按 Ctrl+C 终止。"
echo "[exp_a] 预期：ε_a≈97%  ε_b≈1%  ε_c≈50%  ε_e≈0.1%"
echo "[exp_a] 输出 CSV：${OUT_DIR}_state.csv  ${OUT_DIR}_epoch.csv"

# ── 优雅退出：Ctrl+C 同时停所有子进程 ───────────────────────────────────────
cleanup() {
  echo ""
  echo "[exp_a] 收到退出信号，终止所有工作者..."
  kill "${METRICS_PID}" "${PID_A}" "${PID_B}" "${PID_C}" "${PID_E}" 2>/dev/null || true
  wait 2>/dev/null || true
  echo "[exp_a] 完成。数据已写入 ${OUT_DIR}_*.csv"
}
trap cleanup SIGINT SIGTERM

if [[ -n "${DURATION}" ]]; then
  wait "${PID_A}" "${PID_B}" "${PID_C}" "${PID_E}" 2>/dev/null || true
  cleanup
else
  wait
fi
