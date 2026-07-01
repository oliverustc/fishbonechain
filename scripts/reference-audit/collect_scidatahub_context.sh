#!/usr/bin/env bash
set -euo pipefail

repo="${FISHBONE_REPO:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
ref="${SCIDATAHUB_REF:-$repo/references/SciDataHub}"
out="${SCIDATAHUB_AUDIT_DIR:-$repo/docs/internal/reference-audits/scidatahub}"

usage() {
  cat <<'USAGE'
Usage:
  collect_scidatahub_context.sh

Environment:
  FISHBONE_REPO=/home/swt/fishbonechain
  SCIDATAHUB_REF=/home/swt/fishbonechain/references/SciDataHub
  SCIDATAHUB_AUDIT_DIR=/home/swt/fishbonechain/docs/internal/reference-audits/scidatahub

Creates deterministic, low-token context files for later opencode scans.
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ ! -d "$ref" ]]; then
  echo "SciDataHub reference repo not found: $ref" >&2
  exit 1
fi

mkdir -p "$out/context"

rel_ref() {
  perl -MFile::Spec -e 'print File::Spec->abs2rel($ARGV[0], $ARGV[1])' "$1" "$repo"
}

{
  echo "# SciDataHub Deterministic Context"
  echo
  echo "- generated_at: $(date -Iseconds)"
  echo "- reference_repo: $(rel_ref "$ref")"
  echo "- audit_dir: $(rel_ref "$out")"
  echo
  echo "## Git"
  git -C "$ref" rev-parse --show-toplevel >/dev/null 2>&1 && {
    echo "- branch: $(git -C "$ref" branch --show-current 2>/dev/null || true)"
    echo "- commit: $(git -C "$ref" rev-parse --short=12 HEAD 2>/dev/null || true)"
    echo "- status:"
    git -C "$ref" status --short 2>/dev/null | sed 's/^/  /' || true
  }
  echo
  echo "## Package Scripts"
  for pkg in package.json backend/package.json frontend/package.json caliper/package.json; do
    if [[ -f "$ref/$pkg" ]]; then
      echo "### $pkg"
      node -e '
        const fs = require("fs");
        const pkg = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
        console.log(JSON.stringify({ name: pkg.name || null, scripts: pkg.scripts || {}, dependencies: Object.keys(pkg.dependencies || {}), devDependencies: Object.keys(pkg.devDependencies || {}) }, null, 2));
      ' "$ref/$pkg"
      echo
    fi
  done
} > "$out/context/00-overview.md"

{
  echo "# File Inventory"
  echo
  find "$ref" \
    -path '*/.git' -prune -o \
    -path '*/node_modules' -prune -o \
    -type f -print |
    SCIDATAHUB_REF_ROOT="$ref" perl -MFile::Spec -ne 'chomp; print File::Spec->abs2rel($_, $ENV{SCIDATAHUB_REF_ROOT}), "\n"' |
    sort
} > "$out/context/01-file-inventory.txt"

{
  echo "# Source Tree"
  echo
  find "$ref" \
    -path '*/.git' -prune -o \
    -path '*/node_modules' -prune -o \
    -type d -print |
    SCIDATAHUB_REF_ROOT="$ref" perl -MFile::Spec -ne 'chomp; print File::Spec->abs2rel($_, $ENV{SCIDATAHUB_REF_ROOT}), "\n"' |
    sort
} > "$out/context/02-directory-inventory.txt"

{
  echo "# Route And API Hints"
  echo
  rg -n --glob '!node_modules/**' --glob '!.git/**' \
    --glob '!frontend/src/assets/**' \
    --glob '!**/package-lock.json' --glob '!**/npm-shrinkwrap.json' \
    --glob '!blockchain/test-network/organizations/**' \
    --glob '!caliper/report.html' \
    'app\.(get|post|put|delete|patch)|router\.(get|post|put|delete|patch)|axios\.|fetch\(|baseURL|/api/|localhost|VITE_' "$ref" || true
} > "$out/context/03-routes-api-hints.txt"

{
  echo "# Blockchain And Identity Hints"
  echo
  rg -n --glob '!node_modules/**' --glob '!.git/**' \
    --glob '!frontend/src/assets/**' \
    --glob '!**/package-lock.json' --glob '!**/npm-shrinkwrap.json' \
    --glob '!blockchain/test-network/organizations/**' \
    --glob '!caliper/report.html' \
    'fabric|gateway|contract|chaincode|submitTransaction|evaluateTransaction|wallet|identity|Org[0-9]|msp|certificate|privateKey|endorse|channel|invoke|query' "$ref" || true
} > "$out/context/04-blockchain-identity-hints.txt"

{
  echo "# Domain Model Hints"
  echo
  rg -n --glob '!node_modules/**' --glob '!.git/**' \
    --glob '!frontend/src/assets/**' \
    --glob '!**/package-lock.json' --glob '!**/npm-shrinkwrap.json' \
    --glob '!blockchain/test-network/organizations/**' \
    --glob '!caliper/report.html' \
    'dataset|order|trade|service|user|provider|buyer|owner|price|license|cid|ipfs|hash|signature|publicKey|privateKey|encrypt|decrypt' "$ref" || true
} > "$out/context/05-domain-model-hints.txt"

{
  echo "# Test And Benchmark Hints"
  echo
  rg -n --glob '!node_modules/**' --glob '!.git/**' \
    --glob '!frontend/src/assets/**' \
    --glob '!**/package-lock.json' --glob '!**/npm-shrinkwrap.json' \
    --glob '!blockchain/test-network/organizations/**' \
    --glob '!caliper/report.html' \
    'describe\(|test\(|it\(|expect\(|benchmark|rateControl|workload|Caliper|tps|latency|rounds|transaction' "$ref" || true
} > "$out/context/06-test-benchmark-hints.txt"

echo "Wrote deterministic SciDataHub context to $out/context"
