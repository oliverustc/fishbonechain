# Data Trade Stage 7 Plan Review

## Scope

Reviewed: `docs/internal/agent-plans/2026-06-26-stage7-data-trade-structured-imt-membership-lite.md` (450 lines)

## Reviewer

CodeWhale (plan executor)

## Inputs Read

- Stage 7 plan (full 450 lines)
- `tools/data-trade-zk/internal/imt/proof.go` for current `PrepareProof` and `PreparedProof`
- `tools/data-trade-zk/internal/imt/schema.go` for current `imt.Fixture`
- `tools/data-trade-zk/internal/gnarkadapter/business_range_obfuscation.go` for current `GenerateBusinessRangeFixture`
- `docs/architecture/data-trade-security-model.md` for current guarantee baseline
- `docs/implementation/data-trade-paper-gap-matrix.md` for current IMT rows

## Findings

### 1. Clarification: `PrepareProof` vs `PrepareStructuredProof` — One Function or Two?

- Severity: Medium
- Plan reference: Task 3 lines 329-330
- Issue: The plan says "Update `PrepareProof` or add `PrepareStructuredProof` so business fixture uses structured IMT by default." This leaves the choice open. The two options have different implications:
  - **Update `PrepareProof`**: breaks existing callers (Stage 6 tests) unless the signature is widened with backward-compatible defaults.
  - **Add `PrepareStructuredProof`**: keeps Stage 6 `PrepareProof` available for legacy uses, but business fixture must switch to the new function.
- Why it matters: Stage 6 `PrepareProof` is used in both `GenerateBusinessRangeFixture` and in `imt/proof_test.go`. If the signature changes, existing tests may need updating.
- Recommendation: Use `PrepareStructuredProof` as a new function, keep `PrepareProof` unchanged. This is the safer path for existing test stability and matches the plan's explicit backward-compatibility requirement for Stage 6 witness JSON.

### 2. Clarification: `depth` / `published_depth` Field Alias Defaulting

- Severity: Medium
- Plan reference: Schema Contract lines 164-171
- Issue: The plan says `depth == 10` is a backward-compatible alias for `published_depth`, and `leaf_index == 0` for `published_leaf_index`. After defaulting, both the old field and the new field will contain the same value (10 or 0). The plan does not specify:
  - Whether `depth` and `published_depth` should both be stored in the struct, or one mapped to the other during unmarshal.
  - Whether validation should check consistency if both are present but disagree.
- Why it matters: If both fields exist in the struct and the JSON contains `"depth": 10, "published_depth": 10`, validation should accept. If the JSON contains `"depth": 8, "published_depth": 10`, the fixture is inconsistent — should it reject or prefer one over the other?
- Recommendation: During execution, treat `depth` as a deprecated alias. On unmarshal, if `depth` is present and `published_depth` is omitted, copy `depth` into `published_depth`. If both are present and disagree, reject. This is the most defensive approach.

### 3. Informational: Zero-Value Disambiguation Is Called Out But Implementation Approach Is Open

- Severity: Informational
- Plan reference: Schema Contract line 185-187
- Issue: The plan correctly identifies the Go `int` zero-value problem: `json.Unmarshal` cannot distinguish omitted `0` from explicit `0`. The plan suggests "raw JSON/defaulting helper, pointer-backed decode struct, or equivalent approach" without prescribing one.
- No action required. The plan-phrasing of this warning to the executor is appropriate for a plan (it identifies the problem and gives acceptable solution patterns). During execution, I will use a two-pass approach: unmarshal into `map[string]json.RawMessage` to detect omitted keys, then default only omitted fields.

### 4. Informational: `business_input_hash` Field Count Grows Significantly

- Severity: Informational
- Plan reference: Business Hash Contract lines 247-259
- Issue: Stage 7 adds 10 new metadata fields to `business_input_hash`, bringing the total from ~11 fields to ~21. This is expected given the structured IMT model, but it's worth noting for execution: every field addition changes the hash value deterministically, and the `strLE` helper must handle potentially empty `record_id` correctly (it's ASCII non-empty, so zero-length should not occur after validation).
- No action required.

## Positive Observations

1. **Canonical structure is fully specified.** The four-layer model (Entry → Dataset → Aggregate → Published root) has fixed depths, indices, and domain-separated labels for every layer. No ambiguous design decisions remain for the executor.

2. **RO circuit boundary is respected.** The plan explicitly says `RootObfuscationProof.Define` stays unchanged and `AssignFixture` continues to consume `PreparedProof`. This prevents accidental circuit redesign.

3. **Backward compatibility is a formal requirement.** Old Stage 6 JSON, omitted `imt`, and partial IMT with only old fields must all validate. The zero-value disambiguation warning ensures the executor doesn't accidentally treat explicit `0` as omitted.

4. **Stop conditions cover the specific Stage 7 risks.** "Supporting dynamic depth/index values" is explicitly listed as a stop condition — this prevents the executor from generalizing the fixed constants into a configurable system mid-implementation.

5. **Gap matrix update guidance is concrete.** The plan specifies exact status transitions (`partially-supported` → `prototype-supported` for RO proof, `not-implemented` → `partially-supported` for full IMT) and what the gap text should say.

## Accepted Risks

- This plan introduces a 4-layer IMT model in Go only. No Rust, JS, artifact schema, or proof digest encoding changes.
- The zero-value disambiguation problem is a well-known Go JSON limitation. The plan gives acceptable solution patterns; the executor will pick one during implementation.
- `business_input_hash` changes are deterministic and additive. No existing hash values are broken — they simply change because the input set changes, which is expected for a new stage.
- No VM deployment or E2E regression rerun is required.

## Decision

`approved`

The two medium-severity clarifications (PrepareProof vs PrepareStructuredProof, depth/published_depth alias handling) can be resolved during implementation with the recommendations above. No plan changes required before execution.
