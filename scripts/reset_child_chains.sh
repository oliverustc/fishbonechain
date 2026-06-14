#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 child4 [child1 ...]" >&2
  exit 2
fi

declare -A HOSTS=(
  [child1]="f1 f2 f3"
  [child2]="f4 f5 f6"
  [child3]="f7 f8 f9"
  [child4]="f1 f2 f3 f4 f5 f6 f7"
  [child5]="f10 f11 f12"
  [child6]="f1 f2 f3 f4 f5"
)

for chain in "$@"; do
  if [[ -z "${HOSTS[$chain]:-}" ]]; then
    echo "unknown chain: $chain" >&2
    exit 2
  fi
done

for chain in "$@"; do
  echo "[reset_child] $chain hosts=${HOSTS[$chain]}"

  echo "[reset_child] $chain stop all"
  for host in ${HOSTS[$chain]}; do
    ssh "$host" "sudo systemctl stop fishbone-$chain 2>/dev/null || true"
  done

  echo "[reset_child] $chain delete db on all"
  for host in ${HOSTS[$chain]}; do
    ssh "$host" "find /home/debian/fishbone/$chain/chains -maxdepth 2 \\( -name db -o -name network \\) -exec rm -rf {} + 2>/dev/null || true"
  done

  echo "[reset_child] $chain start all"
  for host in ${HOSTS[$chain]}; do
    ssh "$host" "sudo systemctl start fishbone-$chain"
  done
done

sleep 12
