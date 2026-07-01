#!/usr/bin/env bash
set -euo pipefail
cd "/home/swt/fishbonechain"
export SCIDATAHUB_AUDIT_MODEL="${SCIDATAHUB_AUDIT_MODEL:-deepseek/deepseek-v4-pro}"
mkdir -p "/home/swt/fishbonechain/docs/internal/reference-audits/scidatahub/logs"
bash "/home/swt/fishbonechain/scripts/reference-audit/opencode_scidatahub_audit.sh" --execute --skip-summary --task "benchmarks" 2>&1 | tee "/home/swt/fishbonechain/docs/internal/reference-audits/scidatahub/logs/benchmarks.log"
