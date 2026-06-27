# Data Trade Stage 11 Plan Review

## Scope

Reviewed: `docs/internal/agent-plans/2026-06-27-stage11-data-trade-failure-dispute-scenarios.md` (567 lines)

## Reviewer

CodeWhale (plan executor)

## Inputs Read

- Stage 11 plan (full 567 lines)
- `scripts/zk_real_data_trade_flow.js` for current happy-path structure
- `scripts/data_trade_flow.js` for existing dev-attested failure scenarios
- `pallets/trade-session/src/lib.rs` for dispute/invalid-proof/claim-last-payment entry points
- `pallets/main-escrow/src/lib.rs` for punishDataOwner/claimLastPayment

## Findings

### 1. Medium: Refactoring Scope Has No Upper Bound

- Severity: Medium
- Plan reference: Task 2 lines 301-347
- Issue: The plan asks to extract 5 shared helpers (`setupTrade`, `makeRoundEvidence`, `generateAndVerifyRoundArtifacts`, `submitRoundProofAccepted`, `completeDeliveryAndPayment`) while saying "Refactor carefully. Do not rewrite the whole script." and "Avoid broad style-only refactors." The boundary between "necessary extraction" and "unnecessary rewrite" is left entirely to the executor's judgment.
- Why it matters: The current `zk_real_data_trade_flow.js` is ~450 lines and already moderately complex. Extracting 5 functions from the inline `main()` body could touch 60-80% of the file. Without a clear stopping point, the refactoring could easily expand beyond what's needed for the failure scenarios.
- **Recommendation: Extract only the minimum needed for scenario reuse.** The plan already names the 5 helpers — use those as the upper bound. If any extraction requires touching more than those 5 logical blocks, stop. The happy path should remain functionally identical after refactoring.

### 2. Medium: `invalid-proof-dispute` Bad Digest Derivation Is Fragile

- Severity: Medium
- Plan reference: Scenario semantics lines 103-140
- Issue: The plan says "derive a bad digest by flipping one byte/hex nibble" of a valid artifact's proof digest. This works today because `submitDataProof` + `attestDataProof` are the gates before `submitProofSignature`, and `disputeInvalidProof` operates on the session state after proof submission. But if future pallet changes validate proof digest against on-chain data before accepting the extrinsic, the "flip one byte" approach would become invalid. The plan doesn't ask for a test that verifies the flipped digest actually differs from the valid one.
- Why it matters: The disputed scenario is meant to demonstrate the pallet's dispute behavior. If the "bad digest" accidentally equals the valid digest (e.g., due to field element wrapping), the scenario would submit the same digest twice and not trigger a real dispute.
- **Recommendation:** In the scenario implementation, explicitly assert that `badDigest !== artifact.proof_digest` before submitting. This is a simple one-line guard that prevents a silently broken test.

### 3. Low: `verifier-rejection` Has Clear Deferral Conditions

- Severity: Low
- Plan reference: Lines 226-254, Task 3 line 367
- Issue: The optional `verifier-rejection` scenario requires an `expectTxFailure` helper for expected dispatch errors, which doesn't exist in the current codebase. The plan explicitly says "If this is not simple... skip it in Stage 11. Do not overcomplicate the script."
- No action required. This is well-scoped — the scenario is gated on "if simple" and can be deferred without blocking Stage 11 acceptance.

### 4. Informational: Evidence Field Name Undecided

- Severity: Informational
- Plan reference: Line 401
- Issue: The plan proposes both `"dispute"` and `"scenario_outcome"` as evidence field names and leaves the choice open: "The exact field may be named `scenario_outcome` instead of `dispute` if that reads better." This is a cosmetic decision with no functional impact.
- **Recommendation:** Use `"scenario_outcome"` — it's more general (applies to `requester-refuses-payment` which is not strictly a dispute) and avoids overloading the word "dispute" which has specific pallet semantics.

### 5. Informational: Live Chain Validation Is All Optional

- Severity: Informational
- Plan reference: Task 6 lines 476-524
- Issue: All live chain scenario validation is explicitly marked as optional and gated on RPC availability. This is honest and practical — the plan doesn't require fabricated evidence.
- No action required.

## Positive Observations

1. **Scenario semantics are concrete and implementable.** Each scenario has explicit step-by-step paths, expected terminal events, and evidence shapes. No guesswork about what "dispute" means.

2. **Backward compatibility is comprehensive.** The plan lists every existing feature that must survive: `happy`, `--dry-run-dynamic`, dynamic range/multi_range, legacy `--business-witness`. The stop conditions explicitly protect all of them.

3. **Plan Review Checklist is self-aware.** Lines 555-563 ask the reviewer to check specific implementation concerns: refactoring safety, when to dispute (before/after attestation), whether to include `verifier-rejection`, and evidence field naming.

4. **Evidence shapes are predefined.** Each scenario has a concrete JSON template for expected events, result values, and dispute metadata. No ambiguity about what to record.

5. **`--dry-run-dynamic` rejects non-happy scenarios.** Line 286-290 explicitly rejects `--scenario other-than-happy` in dry-run mode, preventing confusion between proof-pipeline validation and chain-state scenario testing.

## Accepted Risks

- This plan modifies a single JS file extensively with refactoring. No Rust, Go, artifact schema, or proof digest changes.
- Live chain scenario validation may not run in this session if RPC is unavailable. The plan explicitly allows recording "not run due to unavailable RPC."
- The `verifier-rejection` scenario is optional and can be deferred. No acceptance criteria require it.
- The `invalid-proof-dispute` bad digest derivation is fragile but can be guarded with a simple assertion during implementation.

## Decision

`approved`

The two medium findings (refactoring scope, bad digest fragility) are resolvable during execution with the guardrails above. No plan changes required.
