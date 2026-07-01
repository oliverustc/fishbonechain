# SciDataHub Reference Audit

This directory stores machine-assisted audit artifacts for the deprecated
`references/SciDataHub` repository. The goal is to extract reusable ideas for
FishboneChain data-trade work while avoiding direct inheritance of demo-only
blockchain shortcuts.

## Scripts

```bash
# Generate deterministic grep/find/package context only.
bash scripts/reference-audit/collect_scidatahub_context.sh

# Prepare standalone opencode job scripts. This does not run opencode.
bash scripts/reference-audit/prepare_scidatahub_opencode_jobs.sh

# Preview prompts without invoking opencode. This is the default behavior.
bash scripts/reference-audit/opencode_scidatahub_audit.sh

# Explicitly run a single topic through opencode.
bash scripts/reference-audit/opencode_scidatahub_audit.sh --execute --task chain
```

Optional environment:

```bash
SCIDATAHUB_AUDIT_MODEL=deepseek/deepseek-v4-pro
SCIDATAHUB_AUDIT_SESSION=ses_xxx
OPENCODE_EXTRA_ARGS=--auto
```

## Output Layout

- `context/`: deterministic local summaries from `find`, `rg`, and
  `package.json` parsing.
- `prompts/`: exact prompts sent to opencode for each audit topic.
- `reports/`: opencode reports for repository inventory, domain model,
  blockchain trust boundaries, backend, frontend, benchmarks, inheritance
  matrix, and final synthesis.
- `runs/`: generated standalone job scripts that can be executed in parallel.
- `logs/`: raw command logs from generated job scripts.
- `index.md`: generated report index and execution metadata.

## Review Focus

When reviewing generated reports, verify these points first:

- Which SciDataHub files are original project logic versus copied Hyperledger
  Fabric sample scaffolding.
- Whether any user-level cryptographic identity or authorization is actually
  enforced, rather than all blockchain writes being submitted by backend-held
  credentials.
- Which data-market concepts map cleanly to FishboneChain pallets, scripts,
  platform backend APIs, or future UI.
- Which IPFS/hash/evidence ideas are valuable independently of the Fabric demo.
- Which Caliper workload patterns are reusable for FishboneChain experiments
  after replacing Fabric-specific assumptions.
