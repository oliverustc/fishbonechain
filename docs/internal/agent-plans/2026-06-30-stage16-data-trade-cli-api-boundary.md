# Stage 16 Plan: Data Trade CLI / API Boundary Standardization

Date: 2026-06-30
Stage: Stage 16
Roadmap: `docs/internal/agent-plans/2026-06-28-data-flow-platform-long-term-roadmap.md`
Branch: `stage/stage16-data-trade-cli-api-boundary`

## Goal

Define and implement a stable, backend-ready command boundary for data-trade operations while preserving current Stage 14 data-trade validation behavior and avoiding protocol, pallet, settlement, verifier, or backend changes.

## Background

The long-term roadmap's Stage 16 target is to organize the current demo-oriented data-trade scripts into clear operation boundaries for future backend calls and frontend acceptance. Candidate boundaries are:

```text
publish-listing
create-request
create-escrow
open-session
generate-proof
submit-delivery
settle
dispute
inspect
run-flow
```

Current repository facts verified for this plan:

- `docs/architecture/platform-business-model.md` is the Stage 15 baseline and defines `DataAsset`, `BusinessTask`, `WorkflowRun`, `Evidence`, `ChainEvent`, and `OffchainJob` as future platform-facing objects. Backend records remain orchestration/audit metadata and do not replace chain finality.
- `docs/implementation/data-trade-implementation.md` records the current data-trade modules: `pallet-data-registry`, `pallet-trade-session`, `pallet-main-escrow`, off-chain `fishbone-zk`, `scripts/zk_real_data_trade_flow.js`, and `scripts/bridges/data_trade.js`.
- `scripts/zk_real_data_trade_flow.js` is the current primary live/dry-run flow. It supports `--profile`, `--main`, `--child`, `--business-witness`, `--dataset`, `--request`, `--evidence-out`, `--verbose`, `--dry-run-dynamic`, and `--scenario` values `happy`, `invalid-proof-dispute`, `invalid-plaintext-dispute`, and `requester-refuses-payment`.
- `scripts/zk_real_data_trade_flow.js` currently contains reusable operation-like helpers (`setupTrade`, `generateAndVerifyRoundArtifacts`, `submitRoundProofAccepted`, `completeDeliveryAndPayment`) but exposes only end-to-end scenario execution and dynamic dry-run mode.
- `scripts/run_data_trade_validation.sh` is the Stage 14 reproducible validation entrypoint. It calls `scripts/zk_real_data_trade_flow.js` for dry-run, negative, and live-chain scenarios and writes summary/evidence under the selected output directory.
- `docs/experiments/data-trade-validation.md` and `docs/implementation/data-trade-stage14-evidence-index.md` define current Stage 14 evidence layout and scenario expectations.
- `scripts/profiles/chains.json` defines current `child6-data-trade` and `child7-business-trade` profiles with main/child RPCs, verifier mode, verifier authority, ZK command, and proof parameters.
- Current security boundaries remain: off-chain gnark proof verification plus on-chain digest/attestation binding, single dev verifier Charlie, off-chain bridge/session-escrow coordination, and `MainEscrow` as the only implemented settlement mode.

## Scope

Allowed changes:

- Add a new data-trade CLI boundary entrypoint, preferably `scripts/data_trade_cli.js`, that exposes subcommands matching the Stage 16 operation vocabulary.
- Add a lightweight shared helper module under `scripts/lib/` only when it removes duplication and keeps existing behavior unchanged.
- Refactor operation-like code out of `scripts/zk_real_data_trade_flow.js` only if the refactor is mechanical and validated by existing Stage 14 dry-run/negative checks.
- Keep `scripts/zk_real_data_trade_flow.js` and `scripts/run_data_trade_validation.sh` backward-compatible. Existing Stage 14 commands must continue to work.
- Add a formal CLI/API boundary document, preferably `docs/implementation/data-trade-cli-api-boundary.md`, with command semantics, roles, chain/off-chain classification, inputs, outputs, evidence fields, and backend-call suitability.
- Update `docs/README.md` and relevant data-trade docs to point to the new boundary document.
- Add syntax, help, and no-live-chain dry-run validation for the new CLI boundary.
- In Stage 16, independently chain-mutating subcommands are boundary definitions only. `publish-listing`, `create-escrow`, `open-session`, `submit-delivery`, `settle`, and `dispute` must be documented and exposed in help as `planned` / not independently executable. The only chain-mutating execution surface in this stage remains the existing full-flow `run-flow` wrapper, which preserves escrow/session binding.

Out of scope unless Codex/Owner explicitly approves a follow-up plan:

- Building a Web backend, REST/RPC server, database schema, migrations, authentication, or frontend.
- Changing pallets, runtime configuration, extrinsics, proof digest fields, attestation payloads, verifier authority, settlement semantics, bridge trust assumptions, chain IDs, specs, or deployment topology.
- Replacing Stage 14 scenario definitions, evidence metrics, or paper-facing claims.
- Adding new npm, Go, Rust, or system dependencies.
- Requiring live-chain execution as the only validation path.
- Implementing production key custody or signing on behalf of users.

## Current Facts

- Current role mapping:
  - DO is Bob in dev E2E and owns listings, accepts sessions, submits data proof, delivers data hash, claims settlement or last payment.
  - DR is Alice in dev E2E and creates escrow/session, locks funds, opens rounds, submits payment proof/signature/preimage, and disputes.
  - Verifier is Charlie in dev E2E and submits proof attestation.
- Current operation chain/off-chain classification:
  - `publish-listing`: DO, child-chain transaction (`dataRegistry.publishData`).
  - `create-request`: off-chain dataset/request fixture or future backend object; currently represented by request JSON and `request_hash`.
  - `create-escrow`: DR/DO on main chain; current flow does `mainEscrow.openEscrow`, `lockFunds`, and `lockDeposit`.
  - `open-session`: DR/DO on child chain; current flow does `tradeSession.createSession` and `acceptSession`.
  - `generate-proof`: off-chain ZK pipeline using `fishbone-zk make-witness`, `business-fixture`, and `verify`.
  - `submit-delivery`: mixed child-chain round operations; current flow opens round, submits payment proof, submits data proof, attests, signs, delivers hash, and submits payment preimage.
  - `settle`: DO/user-signed settlement on child and main chain with `claimSettlement` and `settleByPreimage`, or `claimLastPayment`.
  - `dispute`: DR/user-signed dispute and main-chain punishment path in current failure scenarios.
  - `inspect`: query-only operation for profile, listing/session/escrow, or evidence artifacts.
  - `run-flow`: compatibility command for the current full flow/scenario execution.
- Stage 14 validation output is the current evidence contract and should remain the compatibility baseline.
- Stage 16 implementation approach decision: implement no-live-chain `inspect`, no-live-chain `generate-proof`, and compatibility `run-flow`; document independently chain-mutating subcommands as planned until a later stage designs state handoff and binding recovery across separate invocations.

## Risks

- Security risk: a command boundary can imply backend signing or key custody. The Stage 16 CLI must state and enforce that chain-changing commands require user/dev signers and do not make a backend trusted.
- Data integrity risk: splitting the flow into subcommands may lose session/listing/escrow binding checks. Reuse or preserve `assertEscrowMatchesTradeTerms` and `assertSessionMatchesListingAndEscrow`.
- Experiment validity risk: changing evidence fields, scenario names, or result strings can break Stage 14 paper evidence. Keep existing `run_data_trade_validation.sh` behavior as a regression gate.
- Compatibility risk: a new CLI could diverge from `zk_real_data_trade_flow.js`. Prefer wrapping or sharing existing logic over introducing a second independent protocol implementation.
- Deployment risk: live-chain commands require RPC availability. Required validation must include no-live-chain checks and may include live checks only when RPC readiness passes.
- Maintainability risk: too many subcommands with partial behavior can become misleading. It is acceptable for Stage 16 to document some subcommands as `planned` or `query/dry-run only`, but their status must be explicit.

## Stop Conditions

Implementation agent must stop and ask Codex/Owner before:

- changing proof digest fields, business input hash encoding, attestation digest payloads, hash algorithms, verifier authority, or proof verification assumptions;
- changing settlement rules, who can release funds, dispute semantics, or escrow/session binding semantics;
- changing Stage 14 evidence schema, scenario IDs, expected result strings, experiment metrics, or paper-facing conclusions;
- adding dependencies or introducing a backend/server/database;
- requiring clean redeploy, chain reset, key/spec changes, or live-chain mutation outside the plan;
- removing or breaking existing `scripts/zk_real_data_trade_flow.js` or `scripts/run_data_trade_validation.sh` compatibility;
- claiming trustless bridge, on-chain Groth16 verification, production verifier quorum, or production IMT support.

## Branch And Commit Plan

- Continue on branch `stage/stage16-data-trade-cli-api-boundary`.
- This is a non-trivial implementation/docs stage. Commit only after:
  - the boundary document exists and is indexed;
  - the CLI entrypoint and any helper modules pass syntax/help/dry-run validation;
  - existing Stage 14 no-live validation still passes or any skipped validation is explicitly justified;
  - this plan's Execution Record is updated with exact command evidence.
- Recommended implementation commit message:

```text
feat(data-trade): standardize CLI operation boundary

Plan: docs/internal/agent-plans/2026-06-30-stage16-data-trade-cli-api-boundary.md
Validation:
- <commands run>
```

## Task List

- [ ] Re-read required inputs: `agent.md`, `docs/internal/agent-collaboration.md`, this plan, roadmap Stage 16, `docs/architecture/platform-business-model.md`, `docs/implementation/data-trade-implementation.md`, `docs/experiments/data-trade-validation.md`, `docs/implementation/data-trade-stage14-evidence-index.md`, `scripts/zk_real_data_trade_flow.js`, `scripts/run_data_trade_validation.sh`, and relevant `scripts/lib/` helpers.
- [ ] Create `docs/implementation/data-trade-cli-api-boundary.md` with a status note that this is a CLI/API boundary for future backend integration, not a backend/server implementation.
- [ ] In the boundary document, define each command: `publish-listing`, `create-request`, `create-escrow`, `open-session`, `generate-proof`, `submit-delivery`, `settle`, `dispute`, `inspect`, and `run-flow`.
- [ ] For each command, document actor/role, whether it is DO/DR/verifier/backend-orchestration/query, chain/off-chain classification, current implementation status, required inputs, outputs/evidence, signer expectations, and failure behavior.
- [ ] Mark independently chain-mutating boundaries (`publish-listing`, `create-escrow`, `open-session`, `submit-delivery`, `settle`, `dispute`) as `planned` / not independently executable in Stage 16. Their docs/help must describe required signer safeguards and binding checks, but the CLI must not submit those independent transactions yet.
- [ ] Add `scripts/data_trade_cli.js` as the Stage 16 CLI boundary entrypoint.
- [ ] Implement `--help` and subcommand help output for every Stage 16 operation name.
- [ ] Implement `inspect` in a no-live-chain-safe way for:
  - `profile` inspection from `scripts/profiles/chains.json`;
  - local evidence JSON inspection or summary of `--evidence <path>`.
- [ ] Implement `generate-proof` as a no-live-chain command that uses the existing dynamic dataset/request ZK path or delegates to `zk_real_data_trade_flow.js --dry-run-dynamic` without changing evidence semantics.
- [ ] Implement `run-flow` as a compatibility wrapper around the existing `scripts/zk_real_data_trade_flow.js` options, preserving this explicit flag surface: `--profile`, `--main`, `--child`, `--business-witness`, `--dataset`, `--request`, `--scenario`, `--evidence-out`, `--dry-run-dynamic`, `--verbose`, and environment variable `ZK_VERIFIER_CMD`.
- [ ] Do not implement independent transaction submission for `publish-listing`, `create-escrow`, `open-session`, `submit-delivery`, `settle`, or `dispute` in this stage. Their help output should explain that Stage 16 exposes their API boundary and that transaction-safe execution is available only via `run-flow`.
- [ ] If considering extraction from `scripts/zk_real_data_trade_flow.js`, first run `node --check scripts/zk_real_data_trade_flow.js` and the Stage 14 no-live compatibility command as a pre-refactor baseline. After extraction, rerun both commands. If extraction requires changing helper signatures, module-level state ownership, evidence accumulator semantics, or signer flow, stop and ask Codex.
- [ ] Ensure output/evidence files for new no-live validation commands can be written under `.agents/fwf/runs/stage16/...`.
- [ ] Update `docs/README.md` to index the new boundary document.
- [ ] Update `docs/implementation/data-trade-implementation.md` with a forward reference to `docs/implementation/data-trade-cli-api-boundary.md`.
- [ ] Update other existing data-trade docs only with forward references if useful; do not rewrite paper-facing conclusions.
- [ ] Update this plan's Execution Record with commits, files changed, exact validation commands, skipped validations, deviations, and remaining risks.

## Acceptance Criteria

- `docs/implementation/data-trade-cli-api-boundary.md` exists, is indexed, and clearly documents all 10 Stage 16 operation boundaries.
- `scripts/data_trade_cli.js` exists and exposes discoverable help for all 10 operation names.
- `inspect` and `generate-proof` provide no-live-chain usable behavior, or an explicitly documented equivalent that preserves Stage 14 evidence semantics.
- `run-flow` remains compatible with existing `zk_real_data_trade_flow.js` behavior for dry-run and live options.
- Independently chain-mutating subcommands are clearly documented and exposed as planned/non-executable, with no partial transaction paths added.
- Existing Stage 14 validation entrypoint remains usable and its no-live dry-run/negative validation passes.
- No pallet/runtime/proof/settlement/deployment/metric changes are made.
- Execution Record contains verified evidence for file existence, syntax/help checks, no-live validation, docs indexing, and any skipped live-chain validation.

## Validation Commands

Run these at minimum:

```bash
git status --short --branch
test -f docs/implementation/data-trade-cli-api-boundary.md
test -f scripts/data_trade_cli.js
rg -n "data-trade-cli-api-boundary" docs/README.md
rg -n "publish-listing|create-request|create-escrow|open-session|generate-proof|submit-delivery|settle|dispute|inspect|run-flow" docs/implementation/data-trade-cli-api-boundary.md
node --check scripts/data_trade_cli.js
node --check scripts/zk_real_data_trade_flow.js
bash -n scripts/run_data_trade_validation.sh
node scripts/data_trade_cli.js --help
node scripts/data_trade_cli.js inspect --help
node scripts/data_trade_cli.js generate-proof --help
node scripts/data_trade_cli.js run-flow --help
node scripts/data_trade_cli.js publish-listing --help
node scripts/data_trade_cli.js create-escrow --help
node scripts/data_trade_cli.js open-session --help
node scripts/data_trade_cli.js submit-delivery --help
node scripts/data_trade_cli.js settle --help
node scripts/data_trade_cli.js dispute --help
node scripts/data_trade_cli.js inspect profile --profile child6-data-trade --out .agents/fwf/runs/stage16/inspect-profile.json
test -f .agents/fwf/runs/stage16/inspect-profile.json
node scripts/data_trade_cli.js generate-proof \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --evidence-out .agents/fwf/runs/stage16/generate-proof-evidence.json
test -f .agents/fwf/runs/stage16/generate-proof-evidence.json
node scripts/data_trade_cli.js inspect evidence \
  --evidence .agents/fwf/runs/stage16/generate-proof-evidence.json \
  --out .agents/fwf/runs/stage16/inspect-evidence.json
test -f .agents/fwf/runs/stage16/inspect-evidence.json
node scripts/data_trade_cli.js run-flow \
  --profile child6-data-trade \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --evidence-out .agents/fwf/runs/stage16/run-flow-evidence.json \
  --dry-run-dynamic
test -f .agents/fwf/runs/stage16/run-flow-evidence.json
```

Note: `scripts/run_data_trade_validation.sh` is a shell script. Validate it with:

```bash
bash -n scripts/run_data_trade_validation.sh
```

Do not use `node --check` for shell scripts; if that appears in an inherited checklist, replace it with `bash -n`.

Run Stage 14 compatibility validation without live RPC:

```bash
scripts/run_data_trade_validation.sh \
  --skip-live \
  --out .agents/fwf/runs/stage16/stage14-compat-skip-live
```

If ZK CLI is missing and cannot be built in the environment, run:

```bash
node --check scripts/lib/data_trade_validation_summary.js
bash -n scripts/run_data_trade_validation.sh
```

and record the exact reason the full no-live compatibility command could not run. Do not claim proof-pipeline compatibility without running the proof command successfully.

Optional live-chain validation, only if RPC readiness is available and the owner accepts live chain mutation:

```bash
scripts/run_data_trade_validation.sh \
  --out .agents/fwf/runs/stage16/stage14-compat-live
```

## Validation Output Paths

Use repo-local ignored output paths:

```text
.agents/fwf/runs/stage16/
```

Do not write validation output to `/tmp` or commit generated files under `.agents/fwf/runs/stage16/`.

## Documentation Updates

Required:

- `docs/implementation/data-trade-cli-api-boundary.md`
- `docs/README.md`
- `docs/implementation/data-trade-implementation.md` forward reference to the new boundary document

Expected forward references if useful:

- `docs/experiments/data-trade-validation.md`
- `docs/implementation/data-trade-flow.md`
- `docs/implementation/data-trade-demo-guide.md`

Do not alter experiment conclusions, measured results, security claims, or paper-facing statements except to add a pointer to the new boundary document.

## Plan-Review Focus

Ask opencode plan review to focus on:

- whether the plan is clear enough to prevent backend/API scope creep;
- whether the command boundaries are concrete while preserving existing Stage 14 compatibility;
- whether the validation commands are realistic, especially shell-vs-node syntax checks and no-live proof validation;
- whether `generate-proof`, `inspect`, and `run-flow` are the right minimal implemented subcommands for Stage 16;
- whether chain-mutating subcommands should be fully implemented now or documented as planned wrappers with safeguards.

## Plan Review Resolution

Plan review: `docs/internal/agent-reviews/2026-06-30-stage16-data-trade-cli-api-boundary-plan-review.md`

Decision: `approved-with-required-fixes`

Required fixes applied:

- Decided the Stage 16 approach for independently chain-mutating subcommands: `publish-listing`, `create-escrow`, `open-session`, `submit-delivery`, `settle`, and `dispute` are documented/exposed as planned and not independently executable. Full-flow mutation remains available only through `run-flow`.
- Added execution validation, not just help checks, for `generate-proof` and `run-flow`, with outputs under `.agents/fwf/runs/stage16/`.
- Made the `docs/implementation/data-trade-implementation.md` forward reference required.

Accepted suggestions:

- Required `inspect evidence` behavior and validation instead of leaving it implicit.
- Added pre/post-refactor baseline checks and a stop condition for helper extraction that changes signatures, module-level state, evidence semantics, or signer flow.
- Defined the `run-flow` flag surface explicitly.
- Added help checks for planned chain-mutating subcommands so their non-executable status is reviewable.

Rejected suggestions:

- None. All required fixes and useful suggestions were incorporated.

Readiness: implementation may proceed after this plan fix; another plan-review round is not required unless the owner wants one.

## Execution Record

### 2026-06-30 Codex Plan Authoring

- Branch: `stage/stage16-data-trade-cli-api-boundary`
- Commits:
  - `45571a2 docs(stage16): plan data trade CLI boundary`
- Tasks completed:
  - Authored initial Stage 16 plan from the long-term roadmap.
  - Created the Stage 16 branch from current local `main`.
- Tests run:
  - `git status --short --branch`
  - `git branch --show-current`
- Tests not run:
  - Implementation validation commands are for the implementation pass and were not run during plan authoring.
- Deviations from plan:
  - None.
- Questions for Codex/Owner:
  - None.
- Remaining risks:
  - `main` is locally ahead of `origin/main` by the Stage 15 merge commits. The Stage 16 branch is based on this local `main`.

### 2026-06-30 Codex Plan Fix

- Branch: `stage/stage16-data-trade-cli-api-boundary`
- Commits:
- Tasks completed:
  - Applied required fixes from `docs/internal/agent-reviews/2026-06-30-stage16-data-trade-cli-api-boundary-plan-review.md`.
  - Decided chain-mutating subcommands are planned/non-executable in Stage 16.
  - Added execution validation for `generate-proof`, `run-flow`, and `inspect evidence`.
  - Required `docs/implementation/data-trade-implementation.md` forward reference.
  - Added refactor baseline/stop-condition guidance.
- Tests run:
  - `git status --short --branch`
  - `git branch --show-current`
- Tests not run:
  - Implementation validation commands are for the implementation pass and were not run during plan fix.
- Deviations from plan:
  - None.
- Questions for Codex/Owner:
  - None.
- Remaining risks:
  - Implementation still needs to create and validate the CLI boundary entrypoint and docs.
