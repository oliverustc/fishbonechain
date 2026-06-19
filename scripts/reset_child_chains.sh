#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROFILE_FILE="${PROFILE_FILE:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --profile-file)
      if [[ $# -lt 2 ]]; then
        echo "--profile-file requires a path" >&2
        exit 2
      fi
      PROFILE_FILE="$2"
      shift 2
      ;;
    --)
      shift
      break
      ;;
    -*)
      echo "unknown argument: $1" >&2
      exit 2
      ;;
    *)
      break
      ;;
  esac
done

if [[ $# -lt 1 ]]; then
  echo "usage: $0 [--profile-file path] child4 [child1 ...]" >&2
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

if [[ -n "$PROFILE_FILE" ]]; then
  if [[ ! -f "$PROFILE_FILE" ]]; then
    echo "profile file not found: $PROFILE_FILE" >&2
    exit 2
  fi
  while IFS=$'\t' read -r chain hosts; do
    [[ -n "${chain:-}" && -n "${hosts:-}" ]] || continue
    HOSTS[$chain]="$hosts"
  done < <(node - "$PROFILE_FILE" <<'NODE'
const fs = require("fs");
const path = process.argv[2];
const raw = JSON.parse(fs.readFileSync(path, "utf8"));
const profiles = raw.chains || raw;
for (const [chain, profile] of Object.entries(profiles)) {
  if (Array.isArray(profile.validators) && profile.validators.length > 0) {
    process.stdout.write(`${chain}\t${profile.validators.join(" ")}\n`);
  }
}
NODE
  )
fi

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
