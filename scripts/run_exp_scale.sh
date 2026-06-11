#!/usr/bin/env bash
# 扩展性实验：N=1/2/3 条链并发，验证线性扩展
# 每链 50 workers（场景d参数），采集 8 个稳态 Epoch
#
# 设计：3 条链同时运行，事后从 CSV 分别提取不同 N 的聚合吞吐量
#   N=1: child4 alone
#   N=2: child4 + child1
#   N=3: child4 + child1 + child6
#
# 用法：bash scripts/run_exp_scale.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
mkdir -p ~/exp_scale_logs

echo "=== 扩展性实验：N=1/2/3 链 × 50 workers（场景d配置）==="
echo "  N=1: child4  |  N=2: child4+child1  |  N=3: 全部三链"
echo "  目标：采集 8 个稳态 Epoch（约 100min）"
echo ""

# 清理旧数据
[[ -f /tmp/exp_scale_state.csv ]] && \
  cp /tmp/exp_scale_state.csv "/tmp/exp_scale_state_backup_$(date +%Y%m%d_%H%M%S).csv"
rm -f /tmp/exp_scale_state.csv /tmp/exp_scale_epoch.csv

# metrics：监控全部 3 条链
echo "[scale] 启动 metrics（child1/child4/child6）..."
nohup node "${SCRIPT_DIR}/metrics.js" \
  --chains ws://127.0.0.1:9945,ws://127.0.0.1:9948,ws://127.0.0.1:9950 \
  --out /tmp/exp_scale \
  --interval 15 \
  > ~/exp_scale_logs/metrics.log 2>&1 &
METRICS_PID=$!
echo "  metrics PID=${METRICS_PID}"
sleep 3

# 3 条链同时跑，各 50 workers（场景d参数）
echo "[scale] 启动 workers..."

nohup node "${SCRIPT_DIR}/worker.js" \
  --scenario d --workers 50 \
  --ws ws://127.0.0.1:9948 --task-id 3 \
  > ~/exp_scale_logs/worker_child4.log 2>&1 &
echo "  child4 (N=1 基线) PID=$!"
sleep 1

nohup node "${SCRIPT_DIR}/worker.js" \
  --scenario d --workers 50 \
  --ws ws://127.0.0.1:9945 --task-id 0 \
  > ~/exp_scale_logs/worker_child1.log 2>&1 &
echo "  child1 (N=2 新增) PID=$!"
sleep 1

nohup node --max-old-space-size=512 "${SCRIPT_DIR}/worker.js" \
  --scenario d --workers 50 \
  --ws ws://127.0.0.1:9950 --task-id 5 \
  > ~/exp_scale_logs/worker_child6.log 2>&1 &
echo "  child6 (N=3 新增) PID=$!"

echo ""
echo "[scale] 所有进程已启动。"
echo "  监控：tail -f ~/exp_scale_logs/metrics.log"
echo "  CSV：/tmp/exp_scale_state.csv"
echo "  目标：等待 child4 完成 8 个 Epoch（约 100min）后停止"
echo ""
echo "[scale] 停止命令："
echo "  pgrep -f 'metrics.js|worker.js' | xargs kill"
