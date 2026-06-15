#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

NODES=""
CHAINS=""
CONFIG="$ROOT/deploy/config.toml"
WITH_LOGS=0
NO_START=0

usage() {
  cat <<'EOF'
Usage: scripts/dev_redeploy_clean_chains.sh [options]

Options:
  --nodes f1,f2             Limit target nodes. Default: all nodes in deploy/config.toml.
  --only f1,f2              Alias for --nodes.
  --chains main,child6      Limit target chains. Default: all chains in deploy/config.toml.
  --config PATH             Config path. Default: deploy/config.toml.
  --logs                    Also remove per-chain log files during stop-clean.
  --no-start                Deploy after cleaning but do not start services.
  -h, --help                Show this help.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --nodes|--only)
      NODES="${2:?missing value for $1}"
      shift 2
      ;;
    --chains)
      CHAINS="${2:?missing value for --chains}"
      shift 2
      ;;
    --config)
      CONFIG="${2:?missing value for --config}"
      shift 2
      ;;
    --logs)
      WITH_LOGS=1
      shift
      ;;
    --no-start)
      NO_START=1
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
STOP_ARGS=(--config "$CONFIG")
DEPLOY_ARGS=(--config "$CONFIG")

if [[ -n "$NODES" ]]; then
  STOP_ARGS+=(--nodes "$NODES")
  DEPLOY_ARGS+=(--nodes "$NODES")
fi

if [[ -n "$CHAINS" ]]; then
  STOP_ARGS+=(--chains "$CHAINS")
  DEPLOY_ARGS+=(--chains "$CHAINS")
fi

if [[ "$WITH_LOGS" -eq 1 ]]; then
  STOP_ARGS+=(--logs)
fi

if [[ "$NO_START" -eq 1 ]]; then
  DEPLOY_ARGS+=(--no-start)
fi

"$ROOT/scripts/dev_stop_clean_chains.sh" "${STOP_ARGS[@]}"
"$ROOT/scripts/dev_deploy_chains.sh" "${DEPLOY_ARGS[@]}"
