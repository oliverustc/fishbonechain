#!/usr/bin/env bash
set -euo pipefail
cd "/home/swt/fishbonechain"
for script in "/home/swt/fishbonechain/docs/internal/reference-audits/scidatahub"/runs/run-[0-9][0-9]-*.sh; do
  bash "$script"
done
bash "/home/swt/fishbonechain/docs/internal/reference-audits/scidatahub/runs/run-final-synthesis.sh"
