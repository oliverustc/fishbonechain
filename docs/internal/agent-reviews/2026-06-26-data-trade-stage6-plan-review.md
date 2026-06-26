# Data Trade Stage 6 Plan Review

## Scope

Reviewed: `docs/internal/agent-plans/2026-06-26-stage6-data-trade-imt-business-coupling.md` (402 lines)

## Reviewer

CodeWhale (plan executor)

## Inputs Read

- Stage 6 plan (full 402 lines)
- `docs/architecture/data-trade-security-model.md` for current guarantee/non-guarantee baseline
- `docs/implementation/data-trade-paper-gap-matrix.md` for IMT-related gap rows
- `tools/data-trade-zk/internal/gnarkadapter/root_obfuscation.go` for existing RO assignment API
- `tools/data-trade-zk/internal/business/schema.go` for existing `RangeWitness` structure
- `tools/data-trade-zk/internal/gnarkadapter/business_range_obfuscation.go` for `GenerateBusinessRangeFixture`

## Findings

### 1. Clarification Needed: `business_input_hash` Construction — New String Encoding Convention

- Severity: Medium
- Plan reference: Task 4 / Artifact Semantics (line 170-178)
- Issue: The plan specifies that strings (`dataset_id`, `field_name`) are encoded as "raw UTF-8 bytes prefixed by a 4-byte little-endian length". The existing `business_input_hash` construction (Stage 2.2) encodes only fixed-width integers and `masked_value_hash` in canonical LE. This introduces a new variable-length encoding convention within the same hash input.
- Why it matters: Adding variable-length encoding to a previously fixed-width hash input changes the design complexity. If the length encoding has a bug (e.g., wrong endianness, off-by-one), the hash will silently differ. The plan does not specify test cases for the correctness of the length-prefixed encoding.
- Recommendation: Add a unit test in Task 4 that verifies the exact byte-level output of the string encoding step (e.g., encode `"demo"` → expected hex bytes), and assert that changing only the string value changes `business_input_hash`.

### 2. Recommended: `PreparedProof` Type Location

- Severity: Low
- Plan reference: Task 3 / RO Circuit Assignment (line 140-155)
- Issue: The plan references `imt.PreparedProof` as the return type of the deterministic proof helper. This type must be defined somewhere (either in `internal/imt` or `internal/gnarkadapter`), but the plan does not specify which package owns it.
- Recommendation: Define `PreparedProof` in `internal/imt` (closer to the data model) and let `gnarkadapter` import it. This keeps the IMT package self-contained and testable without gnark.

### 3. Informational: CLI Smoke Uses `go run` Not Compiled Binary

- Severity: Informational
- Plan reference: Task 5 (line 296-318)
- Issue: Task 5 uses `go run ./cmd/fishbone-zk` for smoke testing, while production E2E scripts use the compiled binary `target/tools/fishbone-zk`. Both paths exercise the same Go code, so this is not a correctness concern.
- No action required. The `go run` approach is simpler for iterative development. The compiled binary path is validated separately via existing E2E regression.

## Positive Observations

1. **Design decision is concrete and bounded.** The plan defines a specific deterministic fixture (leaf 0 = `masked_value_hash`, 9 padding leaves, 4 roots with deterministic decoys) rather than leaving the IMT structure as an open design question. This prevents scope creep during execution.

2. **Stop conditions map directly to agent-collaboration.md guardrails.** Every stop condition (lines 377-388) corresponds to an item in the data-trade guardrails section — no adding artifact fields, no changing digest encoding, no touching Rust. This makes it trivial for the executor to know when to stop.

3. **Backward compatibility is explicit.** Task 2 requires `ReadRangeWitness` to fill a default `imt` fixture when the field is omitted, so existing fixture JSON continues to work. This is a good practice for incremental changes.

4. **Determinism requirements are precise.** The plan distinguishes between `business_input_hash` determinism (required, because it's pure hashing) and `proof_digest` determinism (not required across runs, because Groth16 proving involves randomness). This avoids false test failures.

## Accepted Risks

- This plan introduces a new `internal/imt` package and changes `business_input_hash` encoding. Both are Go-only changes; Rust and JS encoding logic is untouched.
- The deterministic IMT fixture is a simplified prototype fixture (single leaf, depth 10, 4 roots), not full paper IMT. The documentation tasks are required to prevent claiming production IMT.
- No VM deployment or E2E regression rerun is required — artifact verification with `fishbone-zk verify` is sufficient for this Go-only change.

## Decision

`approved`

The one medium-severity finding (string encoding test coverage) can be addressed during implementation by adding a byte-level encoding test in Task 4. No plan changes required before execution.
