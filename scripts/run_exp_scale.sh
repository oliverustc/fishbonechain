#!/usr/bin/env bash
# 扩展性实验：同一配置跑在 N=1/2/4 条链，验证线性扩展
# 每链 50 workers（scenario d 参数），采集 10 个完整 Epoch
#
# 用法：bash scripts/run_exp_scale.sh
#
# 设计：4 条链同时运行，事后从 CSV 分别提取 N=1,2,4 的总吞吐量
#   N=1: child4 alone
#   N=2: child4 + child3
#   N=4: child4 + child3 + child1 + child6

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
mkdir -p ~/exp_scale_logs

echo "=== 扩展性实验：N=1/2/4 链 × 50 workers（scenario d 参数）==="
echo "  50 UNIT/sub, 0.005 req/s, 800B data"
echo "  预计采集 10 个 Epoch（约 2 小时）"
echo ""

# metrics：监控全部 4 条链
echo "[scale] 启动 metrics（child1/3/4/6）..."
nohup node "${SCRIPT_DIR}/metrics.js" \
  --chains ws://10.2.2.11:9945,ws://10.2.2.17:9947,ws://10.2.2.11:9948,ws://10.2.2.11:9950 \
  --out /tmp/exp_scale \
  --interval 15 \
  > ~/exp_scale_logs/metrics.log 2>&1 &
METRICS_PID=$!
echo "[scale] metrics PID=${METRICS_PID}"
sleep 3

# 4 条链同时跑，各 50 workers，使用 scenario d 参数
echo "[scale] 启动 4 条链 worker..."

nohup node "${SCRIPT_DIR}/worker.js" \
  --scenario d --workers 50 \
  --ws ws://10.2.2.11:9948 --task-id 3 \
  > ~/exp_scale_logs/worker_child4.log 2>&1 &
PID4=$!
echo "  child4 (N=1 baseline) PID=${PID4}"

sleep 1

nohup node "${SCRIPT_DIR}/worker.js" \
  --scenario d --workers 50 \
  --ws ws://10.2.2.17:9947 --task-id 2 \
  > ~/exp_scale_logs/worker_child3.log 2>&1 &
PID3=$!
echo "  child3 (for N=2) PID=${PID3}"

sleep 1

nohup node "${SCRIPT_DIR}/worker.js" \
  --scenario d --workers 50 \
  --ws ws://10.2.2.11:9945 --task-id 6 \
  > ~/exp_scale_logs/worker_child1.log 2>&1 &
PID1=$!
echo "  child1 (for N=4) PID=${PID1}"

sleep 1

nohup node "${SCRIPT_DIR}/worker.js" \
  --scenario d --workers 50 \
  --ws ws://10.2.2.11:9950 --task-id 5 \
  > ~/exp_scale_logs/worker_child6.log 2>&1 &
PID6=$!
echo "  child6 (for N=4) PID=${PID6}"

echo ""
echo "[scale] 所有进程已启动。PIDs: metrics=${METRICS_PID}, child4=${PID4}, child3=${PID3}, child1=${PID1}, child6=${PID6}"
echo "[scale] 目标：采集 10 个 Epoch（约 120 分钟）"
echo "[scale] CSV: /tmp/exp_scale_state.csv"
echo ""
echo "[scale] 后台运行中。使用以下命令监控："
echo "  tail -f ~/exp_scale_logs/metrics.log"
echo "  tail -f ~/exp_scale_logs/worker_child4.log"
