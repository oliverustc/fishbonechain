# Stage 16 Plan Review Follow-Up: Data Trade CLI / API Boundary Standardization

Date: 2026-06-30
Reviewer: opencode
Plan: `docs/internal/agent-plans/2026-06-30-stage16-data-trade-cli-api-boundary.md` (revised, commits `45571a2` → `97b676c`)
Primary review: `docs/internal/agent-reviews/2026-06-30-stage16-data-trade-cli-api-boundary-plan-review.md` (`954a4c3`)
Review type: plan review follow-up (post plan-fix)

## Purpose

Re-review the revised Stage 16 plan after Codex applied required fixes from the initial plan review (`approved-with-required-fixes`).

## Resolution of Required Fixes

### Fix 1: Chain-mutating subcommand approach (resolved)

**Original finding**: Task list gave implementation agent an either/or choice.

**Resolution**: Plan now explicitly states at Scope line 51: "In Stage 16, independently chain-mutating subcommands are boundary definitions only. `publish-listing`, `create-escrow`, `open-session`, `submit-delivery`, `settle`, and `dispute` must be documented and exposed in help as `planned` / not independently executable."

Task items 127 and 135 reinforce this with concrete steps. Current Facts line 80 codifies the Stage 16 implementation approach decision. Acceptance criterion (line 149) adds: "Independently chain-mutating subcommands are clearly documented and exposed as planned/non-executable, with no partial transaction paths added."

**Status**: ✅ Fully resolved. No ambiguity remains.

### Fix 2: Missing execution validation for generate-proof and run-flow (resolved)

**Original finding**: Only `--help` was checked, no actual execution validation.

**Resolution**: Validation commands section now includes (lines 179-195):
- `generate-proof` execution with `--dataset`, `--request`, `--evidence-out` flags, output verified via `test -f`
- `run-flow` execution with `--dry-run-dynamic`, `--profile`, `--dataset`, `--request`, `--evidence-out`, output verified via `test -f`
- `inspect evidence` execution with `--evidence`, output verified via `test -f`

All outputs target `.agents/fwf/runs/stage16/`. Commands are concrete, reproducible, and exercise the intended subcommand behavior.

**Status**: ✅ Fully resolved.

### Fix 3: Documentation commitment (resolved)

**Original finding**: `docs/implementation/data-trade-implementation.md` update was optional "if useful."

**Resolution**: Task item 139 now reads: "Update `docs/implementation/data-trade-implementation.md` with a forward reference to `docs/implementation/data-trade-cli-api-boundary.md`." Documentation Updates section (line 246) lists it under "Required."

**Status**: ✅ Fully resolved.

## Suggested Improvements Adopted

All four suggested improvements from the initial review were incorporated:

1. **`inspect evidence` scope clarified** (line 132): Now explicitly listed as implemented behavior: "local evidence JSON inspection or summary of `--evidence <path>`." Validation command added (lines 185-188).

2. **Pre/post-refactor baseline checks** (line 136): Task item added with explicit stop condition: "If extraction requires changing helper signatures, module-level state ownership, evidence accumulator semantics, or signer flow, stop and ask Codex."

3. **`run-flow` flag surface defined** (line 134): Explicitly lists all preserved flags: `--profile`, `--main`, `--child`, `--business-witness`, `--dataset`, `--request`, `--scenario`, `--evidence-out`, `--dry-run-dynamic`, `--verbose`, and `ZK_VERIFIER_CMD`.

4. **Planned subcommand help checks** (lines 171-176): Validation commands now include `--help` checks for all six chain-mutating subcommands, making their planned/non-executable status reviewable.

## Remaining Observations

- The plan now includes a "Plan Review Resolution" section (lines 266-289) documenting the fix summary, decision, accepted suggestions, and readiness statement. This is good practice for traceability.
- The plan states "Readiness: implementation may proceed after this plan fix; another plan-review round is not required unless the owner wants one" (line 289). This is a reasonable judgment given the fixes applied.
- No new risks or scope expansions were introduced.

## Verification

```
# Plan file updated
git diff --stat 45571a2..97b676c docs/internal/agent-plans/2026-06-30-stage16-data-trade-cli-api-boundary.md
  → 1 file changed, 83 insertions(+), 8 deletions(-)

# All three required fix keywords present in revised plan
rg -c "planned.*not independently executable|non-executable" docs/internal/agent-plans/2026-06-30-stage16-data-trade-cli-api-boundary.md
  → Found in Scope, Task List, Current Facts, Acceptance Criteria

# Execution validation present
rg -c "generate-proof-evidence|run-flow-evidence|inspect-evidence" docs/internal/agent-plans/2026-06-30-stage16-data-trade-cli-api-boundary.md
  → 5 matches across validation commands

# Forward reference committed
rg -c "Update.*data-trade-implementation.*forward reference" docs/internal/agent-plans/2026-06-30-stage16-data-trade-cli-api-boundary.md
  → 1 match (task item 139)
```

## Decision

**`approved`**

All three required fixes from the initial plan review (`954a4c3`) have been fully and correctly applied. The plan is now concrete, unambiguous, and provides sufficient validation commands for an implementation agent to execute without architecture invention. Implementation may proceed.
