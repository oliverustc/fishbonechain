#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG="$ROOT/deploy/config.toml"

usage() {
  cat <<'EOF'
Usage: scripts/stop_all_child_services.sh [options]

Options:
  --nodes f1,f2             Limit target nodes. Default: all nodes in config.
  --config PATH             Config path. Default: deploy/config.toml.
  --disable                 Also disable all fishbone-child*.service units.
  -h, --help                Show this help.
EOF
}

ARGS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --nodes)
      ARGS+=(--nodes "${2:?missing value for --nodes}")
      shift 2
      ;;
    --config)
      CONFIG="${2:?missing value for --config}"
      shift 2
      ;;
    --disable)
      ARGS+=(--disable)
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

CONFIG="$(cd "$(dirname "$CONFIG")" && pwd)/$(basename "$CONFIG")"
exec "$ROOT/deploy/.venv/bin/python" "$ROOT/deploy/cmd/control.py" stop-all-children --config "$CONFIG" "${ARGS[@]}"
