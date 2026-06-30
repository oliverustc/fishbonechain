# Stage 16 Plan Review: Data Trade CLI / API Boundary Standardization

Date: 2026-06-30
Reviewer: opencode
Plan: `docs/internal/agent-plans/2026-06-30-stage16-data-trade-cli-api-boundary.md`
Review type: plan review

## Scope Reviewed

- Stage 16 plan in its entirety.
- All referenced current facts, task list, acceptance criteria, validation commands, risks, stop conditions, and documentation updates.

## Inputs Read

1. `agent.md` — current project state, conventions, Stage 12 lessons learned.
2. `docs/internal/agent-collaboration.md` — agent roles, plan/review/execution standards, stop conditions.
3. `docs/internal/agent-plans/2026-06-30-stage16-data-trade-cli-api-boundary.md` — the plan under review.
4. `docs/internal/agent-plans/2026-06-28-data-flow-platform-long-term-roadmap.md` — Stage 16 context in roadmap.
5. `docs/architecture/platform-business-model.md` — Stage 15 baseline, 9 core objects, data trade mapping.
6. `docs/implementation/data-trade-implementation.md` — current data trade modules, ZK toolchain, E2E scripts.
7. `docs/experiments/data-trade-validation.md` — Stage 14 experiment doc, scenario matrix, CLI parameters.
8. `docs/implementation/data-trade-stage14-evidence-index.md` — evidence layout and scenario specs.
9. `docs/README.md` — current doc index (no `data-trade-cli-api-boundary` entry yet).
10. `scripts/zk_real_data_trade_flow.js` (567 lines) — current primary flow script, reusable helpers, all flags.
11. `scripts/run_data_trade_validation.sh` (433 lines) — Stage 14 validation entrypoint.
12. `scripts/lib/` (11 helpers) — existing shared modules.
13. `scripts/profiles/chains.json` — verified `child6-data-trade` and `child7-business-trade` profiles.
14. `git log --oneline -10` — branch `stage/stage16-data-trade-cli-api-boundary` has one commit (plan only).
15. `git diff --stat main..HEAD` — only the plan file changed (1 file, 258 insertions).

## Current Facts Verification

### Verified (backed by repository evidence)

- `docs/architecture/platform-business-model.md` exists and defines all 9 core platform objects. Matches plan claims.
- `docs/implementation/data-trade-implementation.md` exists. Records `pallet-data-registry`, `pallet-trade-session`, `pallet-main-escrow`, `fishbone-zk`, `zk_real_data_trade_flow.js`, `bridges/data_trade.js`. Matches plan claims.
- `scripts/zk_real_data_trade_flow.js` supports `--profile`, `--main`, `--child`, `--business-witness`, `--dataset`, `--request`, `--evidence-out`, `--verbose`, `--dry-run-dynamic`, `--scenario` (`happy`/`invalid-proof-dispute`/`invalid-plaintext-dispute`/`requester-refuses-payment`). Contains helpers `setupTrade`, `generateAndVerifyRoundArtifacts`, `submitRoundProofAccepted`, `completeDeliveryAndPayment`. Line 192-227, 240-297.
- `scripts/run_data_trade_validation.sh` exists and calls `zk_real_data_trade_flow.js`. Supports `--skip-live`, `--out`, `--profile`, `--main`, `--child`, `--zk-cmd`, etc. Matches plan claims.
- `scripts/profiles/chains.json` defines `child6-data-trade` (line 10) and `child7-business-trade` (line 25). Matches plan claims.
- `docs/experiments/data-trade-validation.md` exists with scenario matrix (9 scenarios), output directory layout, CLI params.
- `docs/implementation/data-trade-stage14-evidence-index.md` exists with standard output structure and per-scenario specs.
- Role mapping (DO=Bob, DR=Alice, Verifier=Charlie) verified at `scripts/zk_real_data_trade_flow.js:417-419`.
- Operation classification (chain/off-chain for each operation) accurately reflects current code behavior.
- `scripts/lib/` contains 11 helpers: `data_trade_binding.js`, `data_trade_events.js`, `data_trade_sample.js`, `data_trade_validation_summary.js`, `hash_chain.js`, `trade_profile.js`, `vm_regression_summary.js`, `wait_for_ws_chain.js`, `zk_artifact.js`, `zk_attestation.js`, `zk_verifier_client.js`.
- No existing `scripts/data_trade_cli.js` or `docs/implementation/data-trade-cli-api-boundary.md` — correct for planning stage.
- `bash -n scripts/run_data_trade_validation.sh` passes (shell syntax OK).
- `node --check scripts/lib/data_trade_validation_summary.js` passes (JS syntax OK).

### Verified with qualification

- Security boundaries (off-chain gnark proof verification, on-chain digest/attestation binding, single dev verifier Charlie, off-chain bridge/session-escrow coordination, `MainEscrow` only) match code and formal docs. Plan correctly preserves these.

## Findings

### Finding 1 — severity: medium — Chain-mutating subcommand approach not decided

**Location**: Task list item "For chain-mutating subcommands... either implement them... or mark them as planned/unsafe-to-run independently in help/docs."

**Issue**: The plan gives the implementation agent two options for `publish-listing`, `create-escrow`, `open-session`, `submit-delivery`, `settle`, and `dispute` without prescribing which path to take. The two paths lead to materially different outcomes: one introduces executable chain-mutating commands with shared helper reuse, the other produces documentation-only stubs. This ambiguity risks the implementation agent choosing a middle ground (partial implementation) that breaks escrow/session binding.

**Required fix**: Decide and document the approach for chain-mutating subcommands. Recommend: document all chain-mutating subcommands as `planned` with explicit signer safeguards documented but not independently executable in Stage 16. The existing `run-flow` wrapper already covers full-flow execution. Independent chain-mutating commands are better deferred to a stage that also addresses session/escrow binding re-establishment across subcommands.

### Finding 2 — severity: medium — Missing execution validation for generate-proof and run-flow

**Location**: Validation Commands section. `generate-proof --help` and `run-flow --help` are checked, but no actual execution validation exists for either.

**Issue**: `generate-proof` and `run-flow` are listed as the "right minimal implemented subcommands for Stage 16" (plan-review focus section), yet the validation commands only test `--help` output. The acceptance criteria state "generate-proof provide no-live-chain usable behavior" and "run-flow remains compatible with existing zk_real_data_trade_flow.js behavior." Without execution validation, the implementation agent has no test gate for these subcommands.

**Required fix**: Add execution validation commands:
- `generate-proof`: dry-run execution with `--profile child6-data-trade --dataset ... --request ... --out .agents/fwf/runs/stage16/gen-proof.json`
- `run-flow`: dry-run execution with `--dry-run-dynamic --profile child6-data-trade --dataset ... --request ... --evidence-out .agents/fwf/runs/stage16/run-flow-evidence.json`

### Finding 3 — severity: low — inspect validation coverage incomplete

**Location**: Validation Commands section, only `inspect profile` is tested.

**Issue**: The plan states `inspect` should support "local evidence JSON inspection or summary of `--evidence <path>` when provided" but validation only covers `inspect profile`. If `inspect evidence` is a planned feature, it should either have validation or be explicitly documented as deferred.

**Required fix**: Either add an `inspect evidence` validation command, or document in the plan that `inspect evidence` is a future extension and Stage 16 only implements `inspect profile`.

### Finding 4 — severity: low — Documentation update commitment is conditional

**Location**: Documentation Updates section. "Expected forward references if useful" lists `data-trade-implementation.md`, `data-trade-validation.md`, `data-trade-flow.md`, `data-trade-demo-guide.md` as optional.

**Issue**: The CLI boundary is a new standard invocation surface for data trade. `docs/implementation/data-trade-implementation.md` should include at least a forward reference to the new boundary doc, as the CLI becomes the recommended entrypoint. Leaving this as "if useful" may result in the new boundary being disconnected from existing documentation.

**Suggested fix**: Commit to updating `docs/implementation/data-trade-implementation.md` with a forward reference to `data-trade-cli-api-boundary.md`. Other data-trade docs can remain optional.

### Finding 5 — severity: low — Refactoring guardrails are vague

**Location**: Scope section. "Refactor operation-like code out of `scripts/zk_real_data_trade_flow.js` only if the refactor is mechanical."

**Issue**: "Mechanical" is not defined. The current `zk_real_data_trade_flow.js` uses module-level state (`evidence` accumulator, `log` function, `VERBOSE` flag) that could break during extraction. The plan does not specify which helpers are candidates for extraction or what constitutes a valid mechanical refactor.

**Suggested fix**: Add to task list: "Before extracting helpers, run `node --check scripts/zk_real_data_trade_flow.js` and the Stage 14 compat dry-run to establish a pre-refactor baseline. After extraction, both must still pass. If extraction requires changing helper signatures, stop and ask Codex."

## Decision

**`approved-with-required-fixes`**

The plan has a concrete, bounded goal; scope and non-goals are well-defined; current facts are verifiable; stop conditions are appropriate; and the architectural direction aligns with the roadmap. However, three issues must be resolved before implementation:

## Required Fixes

1. **Chain-mutating subcommands**: Decide the approach. Recommend documenting all six chain-mutating subcommands (`publish-listing`, `create-escrow`, `open-session`, `submit-delivery`, `settle`, `dispute`) as `planned` with signer safeguards, not independently executable in Stage 16. The `run-flow` wrapper preserves full-flow capability and avoids binding re-establishment risk.

2. **Execution validation for generate-proof and run-flow**: Add dry-run execution validation commands (not just `--help`) for `generate-proof` and `run-flow`, with expected output paths under `.agents/fwf/runs/stage16/`.

3. **Documentation commitment**: Commit to updating `docs/implementation/data-trade-implementation.md` with a forward reference to the new boundary document.

## Suggested Improvements

1. Clarify `inspect evidence` scope — either add a validation command or document it as deferred.
2. Add pre/post-refactor baseline checks for `zk_real_data_trade_flow.js` extraction.
3. Define `run-flow` flag surface explicitly: should it replicate all `zk_real_data_trade_flow.js` flags or a defined subset?
4. Consider adding a task to verify `run-flow --help` output contains all 10 operation names (not just current flow flags) once the CLI is built.

## Risks If Unchanged

1. **Chain-mutating partial implementation**: An implementation agent might create incomplete mutation paths that lose escrow/session binding checks (`assertEscrowMatchesTradeTerms`, `assertSessionMatchesListingAndEscrow`), since the current helpers assume a single-script execution context.

2. **generate-proof not validated**: A `generate-proof` subcommand that passes `--help` but fails at runtime (e.g., wrong ZK_CMD env, missing dataset/request parsing) would pass the current validation gate but violate the acceptance criteria.

3. **run-flow regression**: Without execution validation, a wrapper that mangles flag forwarding or evidence output could break the Stage 14 compatibility baseline.

4. **Documentation disconnect**: If `data-trade-implementation.md` is not updated, future agents reading the formal docs would not discover the CLI boundary as the recommended operation surface.

## Questions for Codex/Owner

1. Should the 6 chain-mutating subcommands be implemented now or documented as planned? (Reviewer recommends: document as planned.)
2. Should `generate-proof` be a standalone ZK pipeline invocation or delegate to `--dry-run-dynamic`? (Reviewer recommends: standalone for cleaner subcommand semantics.)
3. Should `run-flow` support live-chain scenarios (`--main`, `--child` with RPC) or only dry-run? (Reviewer recommends: full flag support matching `zk_real_data_trade_flow.js`.)

## Verification Performed

```
# Branch verification
git branch --show-current                    → stage/stage16-data-trade-cli-api-boundary
git status --short                           → clean (only plan file staged)

# File existence checks
test -f docs/architecture/platform-business-model.md                        → yes
test -f docs/implementation/data-trade-implementation.md                    → yes
test -f docs/experiments/data-trade-validation.md                           → yes
test -f docs/implementation/data-trade-stage14-evidence-index.md            → yes
test -f docs/internal/agent-plans/2026-06-28-data-flow-platform-long-term-roadmap.md → yes
test -f scripts/zk_real_data_trade_flow.js                                  → yes
test -f scripts/run_data_trade_validation.sh                                → yes
test -f scripts/profiles/chains.json                                        → yes

# Syntax checks
bash -n scripts/run_data_trade_validation.sh                                → OK (no output)
node --check scripts/lib/data_trade_validation_summary.js                   → OK (no output)

# Profile verification
rg -n "child6-data-trade|child7-business-trade" scripts/profiles/chains.json
  → line 10: "child6-data-trade", line 25: "child7-business-trade"

# No premature implementation
test -f scripts/data_trade_cli.js                                           → NOT_FOUND (correct)
test -f docs/implementation/data-trade-cli-api-boundary.md                  → NOT_FOUND (correct)

# Diff verification
git diff --stat main..HEAD
  → 1 file changed, 258 insertions(+) (plan file only)
```
