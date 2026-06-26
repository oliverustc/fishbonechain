# Data Trade Stage 9 Plan Review

## Scope

Reviewed: `docs/internal/agent-plans/2026-06-27-stage9-data-trade-dynamic-scripted-e2e.md` (431 lines)

## Reviewer

CodeWhale (plan executor)

## Inputs Read

- Stage 9 plan (full 431 lines)
- `scripts/zk_real_data_trade_flow.js` for existing E2E structure
- `scripts/lib/data_trade_sample.js` for current listing/request sample values
- `scripts/profiles/chains.json` for existing profile defaults

## Findings

### 1. Clarification: Evidence Doc Update Scope Not Specified

- Severity: Medium
- Plan reference: Task 5 line 335
- Issue: The plan lists `docs/implementation/data-trade-evidence.md` as a file to update, but unlike the other documentation targets, there is no "Required wording" section specifying what to change. The evidence doc from Stage 5 records local/VM validation results and reproduction commands — adding Stage 9 dynamic evidence format details could either enrich it or create redundancy with the new per-run evidence JSON files.
- Why it matters: The evidence doc is a formal paper-facing artifact. An imprecise update could confuse readers about what evidence is available (historical VM regression vs. current dynamic dry-run).
- Recommendation: During execution, add a short section noting that Stage 9 introduces per-run evidence JSON (`session-<id>-evidence.json`) with dynamic mode metadata, and reference the new `--dry-run-dynamic` smoke command as a reproducible validation target.

### 2. Informational: Mode Selection Logic Requires Careful Implementation

- Severity: Informational
- Plan reference: Design Decision lines 92-99
- Issue: The 6-case mode selection (dynamic, legacy, ambiguous, profile-default interaction, neither-inputs-fallback) is precisely specified but non-trivial. Edge cases like `--dataset` without `--request` (or vice versa) need explicit rejection with a clear error message.
- No action required. The plan's exhaustive enumeration of cases eliminates ambiguity. The executor should test each case in the dry-run path.

### 3. Informational: Dry-Run Mode Is A Pragmatic Design Choice

- Severity: Informational
- Plan reference: Task 4 lines 273-302
- Issue: The dry-run mode (`--dry-run-dynamic`) isolates the ZK pipeline validation from chain RPC availability. This is the right choice for a session where VM endpoints may be unreachable, and it preserves the ability to validate the full `make-witness → business-fixture → verify → evidence` path without chain interaction.
- No action required.

## Positive Observations

1. **Mode selection is exhaustively specified.** Six cases cover every combination of `--dataset`, `--request`, `--business-witness`, and profile defaults. No ambiguous states remain for the executor to design.

2. **Extend, don't duplicate.** The plan explicitly prefers extending the existing `zk_real_data_trade_flow.js` rather than creating a second script, avoiding code drift between two full E2E paths.

3. **Evidence format is concrete.** The JSON schema (lines 146-174) has specific fields for every stage of the pipeline — no guessing about what to record.

4. **Dry-run is the automated validation gate.** Task 4's `--dry-run-dynamic` provides a chainless end-to-end test that exercises the full ZK pipeline. This is more maintainable than relying on VM availability for every stage.

5. **Live chain is explicitly optional.** The plan marks live chain E2E as "optional depending on environment availability" and says "only record it as passed if it actually ran" — honest and practical.

6. **Stop conditions cover JS-specific risks.** "Replacing the existing E2E script with a full rewrite" and "removing or breaking `--business-witness`" are explicit stop conditions.

## Accepted Risks

- This plan modifies a single JS file (`zk_real_data_trade_flow.js`) extensively. No Rust, Go, artifact schema, or proof digest changes.
- Dry-run mode does not test chain interaction — it validates the ZK pipeline and evidence generation in isolation. Live chain testing is deferred to environment availability.
- The evidence doc update is scoped narrowly (add a reference to the new per-run evidence format). No risk of overwriting historical Stage 5 evidence.

## Decision

`approved`

The one medium finding (evidence doc update scope) is resolvable during execution. No plan changes required.
