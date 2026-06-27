# Data Trade Stage 10 Plan Review

## Scope

Reviewed: `docs/internal/agent-plans/2026-06-27-stage10-data-trade-multi-range-constraints.md` (643 lines)

## Reviewer

CodeWhale (plan executor)

## Inputs Read

- Stage 10 plan (full 643 lines)
- `tools/data-trade-zk/internal/dynamic/schema.go` for current Request/RangeConstraint
- `tools/data-trade-zk/internal/dynamic/witness.go` for current BuildRangeWitness
- `tools/data-trade-zk/cmd/fishbone-zk/main.go` for current makeWitnessCmd
- `scripts/zk_real_data_trade_flow.js` for current dynamic mode
- `scripts/fixtures/data_trade_datasets/factory_sensors.json` to verify available fields

## Findings

### 1. Medium: `--out` File-vs-Directory Overload Is Ambiguous

- Severity: Medium
- Plan reference: Task 3 lines 273-295, Plan Review Checklist line 636
- Issue: The plan proposes overloading `--out` to mean a single file for `range` requests and a directory for `multi_range` requests. This is fragile: if a user accidentally passes a file path for multi-range (or a directory for single range), the behavior is undefined. The plan's own review checklist acknowledges this risk: "Whether the `--out` file-vs-directory behavior for `make-witness` is too ambiguous. If reviewers object, prefer adding `--out-dir` for multi-range instead of weakening compatibility."
- Why it matters: Existing scripts and user habits expect `--out` to be a file. Changing its semantics silently could break backward compatibility in subtle ways (e.g., the `--out` argument becomes a directory, but `business-fixture` expects a witness file at that path).
- **Recommendation: Add `--out-dir` for multi-range output directory.** Keep `--out` as a file path for single range (unchanged). `make-witness` should reject `--out-dir` for single-range and `--out` for multi-range. This is a clean separation with no ambiguity.

### 2. Medium: Evidence Shape Normalization Across Single and Multi-Range

- Severity: Medium
- Plan reference: Task 5 lines 501-505
- Issue: The plan says Stage 9 single-range evidence may either keep old flat fields or include a normalized `constraints` one-element list. This is presented as a choice without a clear decision. If single-range evidence keeps old flat fields while multi-range evidence uses `constraints` arrays, downstream evidence readers (thesis writers, future scripts) must handle two different shapes.
- Why it matters: The evidence JSON is the paper-facing artifact. Having two different round schemas for single-range and multi-range increases the complexity of evidence parsing for paper/figure generation.
- **Recommendation: Normalize all round evidence to use the `constraints` array.** For single-range, produce a one-element `constraints` array. Keep `witness_path` and `artifact_path` as top-level fields only inside each constraint entry. This makes the evidence schema uniform regardless of request type.

### 3. Informational: Plan Review Checklist Is Self-Aware

- Severity: Informational
- Plan reference: Lines 632-639
- Issue: The plan includes an explicit "Plan Review Checklist For Codex" section that pre-identifies the same `--out` ambiguity issue found above. This shows the plan author was aware of the tradeoff and deliberately left it open for the reviewer to decide.
- No action required beyond Finding 1.

### 4. Informational: Multi-Range Cap On Constraints Is Sensible

- Severity: Informational
- Plan reference: Task 1 line 174
- Issue: The plan sets a practical cap of 4 constraints for multi-range requests. This prevents exponential proof generation blow-up while still being sufficient for the demo datasets (factory_sensors has 3 uint64 fields per record). The cap is not configurable — this is appropriate for a prototype.
- No action required.

## Positive Observations

1. **"Why Multi-Range Instead Of Subset/Substr" is architecturally honest.** Lines 33-45 explain exactly why subset/substr would be a larger step and why multi-range gives a better thesis demo story while staying inside the proven range circuit.

2. **Live chain "first constraint digest" limitation is clearly documented.** The plan explicitly says on-chain TradeSession binds one proof digest per round, and multi-range conjunction is off-chain/scripted. Evidence marks which constraint is on-chain-bound. No overstatement of security properties.

3. **Backward compatibility is formalized.** Existing single-range request fixtures must pass unchanged. `BuildRangeWitness` is kept as a wrapper around `BuildRangeWitnesses`. `--business-witness` and `--dry-run-dynamic` continue working.

4. **Stop conditions are comprehensive.** Every boundary is listed: no Rust, no artifact schema changes, no new circuits, no VM deployment, no external dependencies. The addition of "Claiming the chain verifies all multi-range constraints on-chain" as a stop condition is especially important for paper safety.

5. **Evidence `chain_binding_mode` field is a good design pattern.** The `"first_constraint_digest"` mode field makes the on-chain binding limitation explicit and machine-readable, rather than relying on implicit "only the first entry" conventions.

## Accepted Risks

- This plan modifies Go (dynamic package + CLI), JS (E2E script), and adds fixtures. No Rust, artifact schema, or proof digest changes.
- The `--out`/`--out-dir` ambiguity is resolvable with a clean flag addition — no architectural rethinking required.
- Multi-range is inherently an off-chain/scripted conjunction. The live-chain limitation ("first constraint digest only") is honestly documented and does not claim stronger security than the code provides.
- No VM deployment or E2E regression rerun is required.

## Decision

`approved`

The two medium findings (`--out` ambiguity, evidence shape normalization) should be resolved during execution per the recommendations above. No plan changes required before execution.
