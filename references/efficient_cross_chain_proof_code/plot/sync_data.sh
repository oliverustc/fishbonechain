#!/usr/bin/env bash
# sync_data.sh — 从 bench/results/ 取最新一批 CSV 到 plot/data/
#
# 用法（在 plot/ 目录下执行）：
#   bash sync_data.sh
#
# 逻辑：
#   1. 将 plot/data/ 中现有 csv 移入 plot/backup/<timestamp>/
#   2. 从 bench/results/ 中按前缀各取最新一个 csv，复制到 plot/data/

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA_DIR="$SCRIPT_DIR/data"
BACKUP_DIR="$SCRIPT_DIR/backup/$(date +%Y%m%d_%H%M%S)"
RESULTS_DIR="$SCRIPT_DIR/../bpiano/bench/results"

# ── 1. 备份现有 data/ 中的 csv ─────────────────────────────────────────────
existing=("$DATA_DIR"/*.csv)
if [ -e "${existing[0]}" ]; then
    mkdir -p "$BACKUP_DIR"
    mv "$DATA_DIR"/*.csv "$BACKUP_DIR"/
    echo "备份旧文件到 $BACKUP_DIR"
else
    echo "data/ 中无旧文件，跳过备份"
fi

# ── 2. 按前缀取最新文件 ────────────────────────────────────────────────────
PREFIXES=(
    "compress_performance"
    "aggregation_proof_size"
    "aggregation_verify_time"
    "aggregation_prove_time"
    "aggregation_verify_gas_cost"
)

mkdir -p "$DATA_DIR"

for prefix in "${PREFIXES[@]}"; do
    # 按文件名字典序排序，取最后一个（时间戳格式 YYYYMMDD_HHmmSS 保证有序）
    latest=$(ls "$RESULTS_DIR"/${prefix}_*.csv 2>/dev/null | sort | tail -1)
    if [ -z "$latest" ]; then
        echo "警告：未找到 ${prefix}_*.csv，跳过"
        continue
    fi
    cp "$latest" "$DATA_DIR/"
    echo "复制 $(basename "$latest") → data/"
done

echo "完成。data/ 当前文件："
ls "$DATA_DIR"/*.csv 2>/dev/null | xargs -I{} basename {}
