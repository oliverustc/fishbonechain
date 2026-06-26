# Data Trade Stage 4 Code Review

## Scope

Reviewed commits on branch `docs/data-trade-stage4-security-model`:

- `14a31b1 docs: add data trade security model with review checklist`
- `53c3dba docs: link data trade implementation to security model`
- `25aa33f docs: mark security model stage complete`

## Reviewer

Codex

## Inputs Read

- `docs/architecture/data-trade-security-model.md`
- `docs/implementation/data-trade-implementation.md`
- `docs/internal/agent-plans/2026-06-16-data-trade-four-stage-roadmap.md`
- `docs/internal/agent-plans/2026-06-16-stage4-security-model-paper-alignment.md`
- `docs/README.md`
- `references/data_trade_paper/main.tex`
- `git diff --stat HEAD~3..HEAD`
- `git log --oneline --decorate -n 12`

## Findings

### 1. Required: New Formal Architecture Doc Is Not Indexed

- Severity: Medium
- File: `docs/README.md`
- Issue: Stage 4 adds the formal architecture document `docs/architecture/data-trade-security-model.md`, but `docs/README.md` does not link it under "架构与设计" or any other formal-doc section.
- Why it matters: Repository documentation rules require updating `docs/README.md` when adding a new formal document. Without the index link, new agents and humans can miss the security model even though Stage 4 is marked complete.
- Required fix: Add `architecture/data-trade-security-model.md` to `docs/README.md`, preferably under "架构与设计".

### 2. Required: Roadmap Baseline Still Contains Pre-Stage-2 Fact

- Severity: Medium
- File: `docs/internal/agent-plans/2026-06-16-data-trade-four-stage-roadmap.md:55`
- Issue: The roadmap still says the current gnark circuit is a Stage 1 fixture and the witness uses random/fixed samples, even though the same branch marks Stage 2 complete and the security model correctly states Stage 2.2 `BusinessRangeProof` is implemented.
- Why it matters: Stage 4 is specifically about paper/security alignment. A stale baseline in the roadmap contradicts the new security model and can mislead the next agent into thinking business witness remains pending.
- Required fix: Replace the stale bullet with the current Stage 2.2 fact: business range witness is circuit-level for the range case, while full IMT membership and subset/substr constraints remain future work.

### 3. Recommended: Roadmap "Recommended Execution Order" Is Now Historical

- Severity: Low
- File: `docs/internal/agent-plans/2026-06-16-data-trade-four-stage-roadmap.md:73`
- Issue: The roadmap still recommends starting with Stage 2 and then Stage 3, despite all four stages now being marked complete.
- Why it matters: It is not false as historical guidance, but it is confusing after Stage 4 completion.
- Suggested fix: Rename the section to "Historical Recommended Execution Order" or replace it with "Next Directions" such as trustless bridge, verifier quorum, and IMT/subset/substr extensions.

### 4. Recommended: Stage 4 Plan Checkboxes Remain Unchecked

- Severity: Low
- File: `docs/internal/agent-plans/2026-06-16-stage4-security-model-paper-alignment.md`
- Issue: The execution record says Tasks 1-5 are complete, but the task checkboxes remain unchecked.
- Why it matters: The execution record is enough to reconstruct progress, but unchecked tasks reduce scanability and conflict with the plan's checkbox-tracking convention.
- Suggested fix: Mark completed Stage 4 task checkboxes as `[x]`, or add a short note near the checklist saying the checklist is historical and completion is recorded in the Execution Record.

## Accepted Risks

- This is a docs-only stage. No Rust, Go, JS, runtime, deployment, or experiment behavior changed.
- The security model intentionally documents current non-guarantees: no on-chain Groth16 verifier, single dev verifier authority, and non-trustless bridge/session-escrow coordination.
- No VM E2E rerun is required for this documentation-only change.

## Verification Performed

- Confirmed `docs/architecture/data-trade-security-model.md` exists.
- Confirmed `docs/implementation/data-trade-implementation.md` links to the new security model.
- Confirmed Stage 4 roadmap status is marked complete.
- Confirmed `references/data_trade_paper/main.tex` exists and has 579 lines.

## Decision

`approved-with-required-fixes`

Required before merge:

- Add the new security model to `docs/README.md`.
- Update stale Stage 2 baseline text in the four-stage roadmap.
