# Data Trade Stage 8 Plan Review

## Scope

Reviewed: `docs/internal/agent-plans/2026-06-27-stage8-data-trade-dynamic-dataset-request-model.md` (426 lines)

## Reviewer

CodeWhale (plan executor)

## Inputs Read

- Stage 8 plan (full 426 lines)
- `tools/data-trade-zk/internal/business/schema.go` for current `RangeWitness` and `ReadRangeWitness`
- `tools/data-trade-zk/internal/imt/schema.go` for current `imt.Fixture` and `DefaultFixture`
- `docs/internal/agent-plans/2026-06-26-data-trade-long-term-scripted-e2e-roadmap.md` confirmed exists
- `tools/data-trade-zk/cmd/fishbone-zk/main.go` for existing CLI structure

## Findings

### 1. Clarification: Long-Term Roadmap Update Scope Not Specified

- Severity: Medium
- Plan reference: Task 5 line 359
- Issue: The plan lists `docs/internal/agent-plans/2026-06-26-data-trade-long-term-scripted-e2e-roadmap.md` as a file to update, but unlike the other documentation targets, there is no "Required wording" section specifying what to change. The file exists (confirmed) but the plan leaves the update content entirely to the executor's judgment.
- Why it matters: The long-term roadmap is a shared reference for future stages. An incomplete or misaligned update could misdirect Stage 9 planning.
- Recommendation: During execution, add a short Stage 8 entry that documents the new `make-witness` pipeline and notes that dynamic scripted E2E orchestration is deferred to Stage 9.

### 2. Recommended: `uint64` Overflow Guard In Dataset Validation

- Severity: Low
- Plan reference: Schema Contract lines 135-136
- Issue: The plan says field `value` must fit `uint64` and `mask_delta` must fit `uint64`. Go's `json.Unmarshal` into `uint64` will saturate to `math.MaxUint64` on overflow without error. This means a JSON value `18446744073709551616` would silently become `0` without explicit range checking.
- Why it matters: If a user hand-writes a dataset JSON with an out-of-range value, the overflow could produce a silently wrong witness without any validation error.
- Recommendation: Add an explicit validation check: after `json.Unmarshal` into the struct, if the field type is `uint64`, the raw JSON number should be re-checked against `math.MaxUint64`. Alternatively, use `json.Number` for intermediate parsing. This is a low-severity edge case — realistic uint64 values won't trigger it — but the plan's "must fit uint64" rule should be enforced.

### 3. Informational: Dataset/Request Consistency Rules Are Well-Separated

- Severity: Informational
- Plan reference: Task 2 lines 251-270
- Issue: The plan correctly separates structural validation (Task 1: valid JSON shapes, types, ranges) from cross-document consistency (Task 2: dataset/request ID matching, record/field existence, value-in-range check). This two-phase validation avoids coupling the schema package to cross-document logic.
- No action required. This is good plan design.

### 4. Informational: `masked_value_hash` Delegation Is Correct

- Severity: Informational
- Plan reference: Witness Generation Contract line 189
- Issue: The plan explicitly says `masked_value_hash` should be `""` in the generated witness, and `business-fixture` computes and validates it. This avoids duplicating MiMC hash logic in the `dynamic` package — a clean separation of concerns.
- No action required.

## Positive Observations

1. **Pipeline separation is clean.** `make-witness` produces a `RangeWitness`, `business-fixture` consumes it. No changes to the existing proof path. This is the right architectural choice.

2. **Fixture naming and structure are concrete.** Two datasets (factory_sensors, vehicle_telematics) with specific field names and records. Three request fixtures including one invalid for out-of-range testing. No ambiguity about what to create.

3. **Backward compatibility is explicit.** The existing `business-fixture --witness scripts/fixtures/data_trade_business_sample.json` must continue to work. The old witness JSON is preserved unchanged.

4. **Stop conditions are comprehensive.** Every boundary is listed: no circuit changes, no subset/substr, no Rust, no artifact schema changes, no frontend, no VM deployment.

5. **Task 4 smoke test is end-to-end.** The plan requires `make-witness → business-fixture → verify` for at least two valid requests plus one invalid rejection. This covers the entire new pipeline.

## Accepted Risks

- This plan adds a new `internal/dynamic` Go package and a new `make-witness` CLI command. No changes to Rust, JS, artifact schema, or existing proof paths.
- The `uint64` overflow edge case is a low-probability issue — realistic sensor/telemetry data values are far below `math.MaxUint64`. Can be addressed during execution.
- No VM deployment or E2E regression rerun is required.

## Decision

`approved`

The one medium finding (roadmap update scope) is resolvable during execution. No plan changes required.
