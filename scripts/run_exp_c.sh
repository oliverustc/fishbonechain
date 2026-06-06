#!/usr/bin/env bash
# 实验 C：6 链并发（核心论文数据）
# 6 条子链同时运行各自场景，验证多链吞吐量线性扩展。
#
# 用法：
#   bash scripts/run_exp_c.sh
#
# 前置条件：
#   - setup_experiment.js 已完整执行（task_id 0-5 均已激活并同步）
#   - 各子链工作者账户已充值
#   - f1 可通过 SSH 访问（bcg 跳板）

set -euo pipefail

MAIN_WS="${MAIN_WS:-ws://10.2.2.11:9944}"
ALICE="5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY"
F1_SEED="${F1_SEED:-0x52390bf081065e3ff296ab72c42bc234cedbdf9ddc40b6c7b6aee5fd01e08880}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=== 实验 C：6 链并发基准 ==="
echo ""

mkdir -p ~/exp_c_logs

# ── metrics：监控全部 6 条链 ───────────────────────────────────────────────────
echo "[exp_c] 启动 metrics（6 链）..."
nohup node "${SCRIPT_DIR}/metrics.js" \
  --chains ws://10.2.2.11:9945,ws://10.2.2.14:9946,ws://10.2.2.17:9947,ws://10.2.2.11:9948,ws://10.2.2.20:9949,ws://10.2.2.11:9950 \
  --out /tmp/exp_c \
  --interval 15 \
  > ~/exp_c_logs/metrics.log 2>&1 &
echo "[exp_c] metrics PID=$!"
sleep 2

# ── bridge：child1 跨链中继（f1 作为矿工，task_id=0） ────────────────────────
echo "[exp_c] 启动 bridge@child1..."
nohup env \
  CHILD_WS=ws://10.2.2.11:9945 \
  MAIN_WS="${MAIN_WS}" \
  MINER_SURI="${F1_SEED}" \
  REQUESTER="${ALICE}" \
  TASK_ID=0 CHAIN_ID=0 \
  node "${SCRIPT_DIR}/bridge.js" \
  > ~/exp_c_logs/bridge_child1.log 2>&1 &
echo "[exp_c] bridge@child1 PID=$!"

sleep 1

# ── 6 个工作者场景，各连自己的专用链 ────────────────────────────────────────
echo "[exp_c] 启动 6 个场景..."

nohup node "${SCRIPT_DIR}/worker.js" --scenario a \
  --ws ws://10.2.2.11:9945 --task-id 0 \
  > ~/exp_c_logs/worker_a.log 2>&1 &
echo "  scenario_a (child1) PID=$!"

nohup node "${SCRIPT_DIR}/worker.js" --scenario b \
  --ws ws://10.2.2.14:9946 --task-id 1 \
  > ~/exp_c_logs/worker_b.log 2>&1 &
echo "  scenario_b (child2) PID=$!"

nohup node "${SCRIPT_DIR}/worker.js" --scenario c \
  --ws ws://10.2.2.17:9947 --task-id 2 \
  > ~/exp_c_logs/worker_c.log 2>&1 &
echo "  scenario_c (child3) PID=$!"

nohup node "${SCRIPT_DIR}/worker.js" --scenario d \
  --ws ws://10.2.2.11:9948 --task-id 3 \
  > ~/exp_c_logs/worker_d.log 2>&1 &
echo "  scenario_d (child4) PID=$!"

nohup node "${SCRIPT_DIR}/worker.js" --scenario e \
  --ws ws://10.2.2.20:9949 --task-id 4 \
  > ~/exp_c_logs/worker_e.log 2>&1 &
echo "  scenario_e (child5) PID=$!"

nohup node "${SCRIPT_DIR}/worker.js" --scenario f \
  --ws ws://10.2.2.11:9950 --task-id 5 \
  > ~/exp_c_logs/worker_f.log 2>&1 &
echo "  scenario_f (child6) PID=$!"

echo ""
echo "[exp_c] 所有进程已启动。目标：采集 5 个 Epoch（约 60 min）"
echo "[exp_c] 采集文件：/tmp/exp_c_state.csv  /tmp/exp_c_epoch.csv"
echo "[exp_c] 实时日志：~/exp_c_logs/"
echo ""
echo "[exp_c] Ctrl+C 终止所有进程"

cleanup() {
  echo "[exp_c] 终止中..."
  pkill -f "worker.js" 2>/dev/null || true
  pkill -f "metrics.js" 2>/dev/null || true
  pkill -f "bridge.js"  2>/dev/null || true
  wait 2>/dev/null || true
  echo "[exp_c] 完成"
}
trap cleanup SIGINT SIGTERM
wait
