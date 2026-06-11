#!/usr/bin/env bash
# 实验 E：资金流动性对比（FishboneChain vs 传统预锁方案）
#
# 运行 3 条活跃子链（child1/child4/child6）的完整 bridge + worker + 资金指标采集。
# 采集主链 FMC 状态：task_locked / pool_locked / baseline_locked（反事实）。
#
# 前置条件（在此脚本运行前确认）：
#   - task_id=0,3,5 均处于 Activated 状态
#   - task_id=1,2,4 均已 Terminated（dead 链）
#   - Alice FMC free >= 31,500 UNIT
#   - fishbone-main / child1 / child4 / child6 systemd 服务正在运行
#
# 用法（在 10.2.2.11 上执行）：
#   cd /home/debian/fishbone && bash scripts/run_exp_fund.sh
#
# 输出：
#   /tmp/exp_e_fund_state.csv    资金指标时序数据
#   ~/exp_fund_logs/             各组件日志

set -euo pipefail

MAIN_WS="${MAIN_WS:-ws://127.0.0.1:9944}"
ALICE="5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY"

# 矿工 seed（与 setup_experiment.js VALIDATORS 一致）
F1="0x52390bf081065e3ff296ab72c42bc234cedbdf9ddc40b6c7b6aee5fd01e08880"
F2="0x17f21fff17006faad4fa003c3db215718fc4d3bbc86054a435666affe704e71b"
F3="0x4151713ff93e1333474f1380cb2bc4fce9183942790830c1e8f48d3752e232fc"
F4="0x8b7aeb4590e1607db466c3cea45b4096f0b912364da41689f1be5166df3fee83"
F5="0x69416e5975a353736603d14d57e13efdadab8dd11667498d7892588abf50e70a"
F6="0x91bd2803edfcbb7e8d7f06c7df94f98d26300e56b968ee51855d994e4308d1e8"

# 实际 CCMC 注册矿工数（chain0=4/chain3=8/chain5=6），threshold=ceil(n×2/3)
# chain0 threshold=3 → F1,F2,F3
# chain3 threshold=6 → F1,F2,F3,F4,F5,F6
# chain5 threshold=4 → F1,F2,F3,F4

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=== 实验 E：资金流动性对比 ==="
echo "  主链：${MAIN_WS}"
echo "  任务：task0(child1, 1500U, threshold=3)  task3(child4, 5000U, threshold=6)  task5(child6, 25000U, threshold=4)"
echo "  目标：T_PLANNED=17，EPOCH_OFFSET=14，即从当前 epoch 起再跑 3 个 child4 周期（约 60min）"
echo ""

mkdir -p ~/exp_fund_logs

# ── 1. metrics_fund.js：主链资金状态采集 ────────────────────────────────────
echo "[exp_fund] 启动 metrics_fund..."
nohup env \
  MAIN_WS="${MAIN_WS}" \
  REQUESTER="${ALICE}" \
  TASK_IDS="0,3,5" \
  T_PLANNED="3" \
  EPOCH_OFFSET="14" \
  node "${SCRIPT_DIR}/metrics_fund.js" \
    --out /tmp/exp_e_fund \
    --interval 10 \
  > ~/exp_fund_logs/metrics_fund.log 2>&1 &
METRICS_PID=$!
echo "[exp_fund] metrics_fund PID=${METRICS_PID}"
sleep 2

# ── 2. bridge@child1（chain_id=0, task_id=0, miners=4人, threshold=3） ────────
echo "[exp_fund] 启动 bridge@child1..."
nohup env \
  CHILD_WS="ws://127.0.0.1:9945" \
  MAIN_WS="${MAIN_WS}" \
  MINER_SURIS="${F1},${F2},${F3}" \
  REQUESTER="${ALICE}" \
  TASK_ID="0" CHAIN_ID="0" \
  node "${SCRIPT_DIR}/bridge.js" \
  > ~/exp_fund_logs/bridge_child1.log 2>&1 &
echo "[exp_fund] bridge@child1 PID=$!"
sleep 1

# ── 3. bridge@child4（chain_id=3, task_id=3, miners=8人, threshold=6） ────────
echo "[exp_fund] 启动 bridge@child4..."
nohup env \
  CHILD_WS="ws://127.0.0.1:9948" \
  MAIN_WS="${MAIN_WS}" \
  MINER_SURIS="${F1},${F2},${F3},${F4},${F5},${F6}" \
  REQUESTER="${ALICE}" \
  TASK_ID="3" CHAIN_ID="3" \
  node "${SCRIPT_DIR}/bridge.js" \
  > ~/exp_fund_logs/bridge_child4.log 2>&1 &
echo "[exp_fund] bridge@child4 PID=$!"
sleep 1

# ── 4. bridge@child6（chain_id=5, task_id=5, miners=6人, threshold=4） ────────
echo "[exp_fund] 启动 bridge@child6..."
nohup env \
  CHILD_WS="ws://127.0.0.1:9950" \
  MAIN_WS="${MAIN_WS}" \
  MINER_SURIS="${F1},${F2},${F3},${F4}" \
  REQUESTER="${ALICE}" \
  TASK_ID="5" CHAIN_ID="5" \
  node "${SCRIPT_DIR}/bridge.js" \
  > ~/exp_fund_logs/bridge_child6.log 2>&1 &
echo "[exp_fund] bridge@child6 PID=$!"
sleep 2

# ── 5. workers（3 个场景） ────────────────────────────────────────────────────
echo "[exp_fund] 启动 workers..."

nohup node "${SCRIPT_DIR}/worker.js" --scenario a \
  --ws ws://127.0.0.1:9945 --task-id 0 \
  > ~/exp_fund_logs/worker_a.log 2>&1 &
echo "  scenario_a (child1, 300 workers) PID=$!"

nohup node "${SCRIPT_DIR}/worker.js" --scenario d \
  --ws ws://127.0.0.1:9948 --task-id 3 \
  > ~/exp_fund_logs/worker_d.log 2>&1 &
echo "  scenario_d (child4, 100 workers) PID=$!"

nohup node "${SCRIPT_DIR}/worker.js" --scenario f --workers 100 \
  --ws ws://127.0.0.1:9950 --task-id 5 \
  > ~/exp_fund_logs/worker_f.log 2>&1 &
echo "  scenario_f (child6, 100 workers) PID=$!"

echo ""
echo "[exp_fund] 所有进程已启动。"
echo ""
echo "  指标采集：/tmp/exp_e_fund_state.csv"
echo "  日志目录：~/exp_fund_logs/"
echo "  Epoch 进度：tail -f ~/exp_fund_logs/bridge_child4.log"
echo "  资金状态：tail -f ~/exp_fund_logs/metrics_fund.log"
echo ""
echo "  预期：~60min 内完成 3 个 child4 epoch + 5 个 child1/child6 epoch"
echo "  验证命令："
echo "    grep BillSettled ~/exp_fund_logs/metrics_fund.log"
echo "    tail -5 /tmp/exp_e_fund_state.csv"
echo ""
echo "[exp_fund] 按 Ctrl+C 停止所有进程"

cleanup() {
  echo ""
  echo "[exp_fund] 终止所有进程..."
  pkill -f "metrics_fund.js" 2>/dev/null || true
  pkill -f "worker.js"       2>/dev/null || true
  pkill -f "bridge.js"       2>/dev/null || true
  wait 2>/dev/null || true
  echo "[exp_fund] 已停止。数据在 /tmp/exp_e_fund_state.csv"
}
trap cleanup SIGINT SIGTERM
wait
