# Stage 15 Plan: Platform Business Model Design

Date: 2026-06-29
Stage: Stage 15
Roadmap: `docs/internal/agent-plans/2026-06-28-data-flow-platform-long-term-roadmap.md`
Branch: `stage/stage15-platform-business-model`

## Goal

Design the first platform-level business object model before any Web backend implementation, so data trade becomes one business module mapped onto shared platform objects instead of becoming the platform's default data model.

## Background

The long-term roadmap's Stage 15 requires a platform business model document, initial JSON schema or TypeScript type draft, a data-trade mapping table, and placeholder mappings for data collection, cross-domain flow, and verifiable training.

Current repository facts verified for this plan:

- `docs/architecture/platform-architecture.md` defines the current platform/scene split: `pallet-ccmc` and `pallet-chain-profile` are mandatory platform capabilities, `pallet-fmc` is optional, and scene pallets remain independent.
- `docs/implementation/data-trade-implementation.md` records data trade as `pallet-data-registry`, `pallet-trade-session`, `pallet-main-escrow`, off-chain `fishbone-zk`, bridge scripts, and E2E scripts.
- `docs/experiments/data-trade-validation.md` and `docs/implementation/data-trade-stage14-evidence-index.md` define Stage 14 evidence outputs and already map `summary.scenarios[]` toward future `WorkflowRun` / `Evidence` objects.
- `scripts/zk_real_data_trade_flow.js` writes per-run evidence with listing, escrow, session, request, round constraints, settlement, events, and result fields.
- The root `package.json` only declares ESM plus `@polkadot/api`; `monitor/package.json` has TypeScript tooling scoped to `monitor/`.
- There is no current backend model/schema package for platform business objects.

## Scope

Allowed changes:

- Add a formal platform model document, preferably `docs/architecture/platform-business-model.md`.
- Add an initial schema/type draft under a lightweight source path that does not imply a backend service exists yet. Preferred path: `scripts/platform-model/`.
- Add data-trade to platform-object mapping tables.
- Add placeholder mapping sections for data collection, cross-domain flow, and verifiable training.
- Update `docs/README.md` to index the new formal document.
- Optionally update `docs/implementation/data-trade-stage14-evidence-index.md` only to point readers to the new platform model if it reduces duplication.

Out of scope unless Codex/Owner approves a follow-up plan:

- Building a Web backend, database schema, migrations, API server, authentication, UI, or event indexer.
- Changing pallets, runtime types, extrinsics, proof digest fields, settlement rules, verifier authority, bridge trust assumptions, or experiment metrics.
- Changing Stage 14 validation behavior or evidence contents.
- Adding a new dependency for schema validation or code generation.
- Claiming production trust, privacy, or security guarantees beyond current docs.

## Current Facts

- Stage 15 target objects from the roadmap are `User`, `ChainAccount`, `Dataset`, `DataAsset`, `BusinessTask`, `WorkflowRun`, `Evidence`, `ChainEvent`, and `OffchainJob`.
- Data trade current chain objects include data-registry listings, trade-session sessions/rounds, main-escrow escrows, proof metadata, verifier attestations, settlement claims, and dispute events.
- Current data trade limitations that must stay visible in mappings:
  - chain verifies proof digest and verifier attestation, not on-chain Groth16;
  - verifier authority is a single dev key, Charlie;
  - bridge/session-escrow binding is checked off-chain;
  - `MainEscrow` is implemented, while `FmcAssisted` and `Hybrid` remain future modes.
- Stage 14 evidence output is runtime-generated and must not be committed, but its schema is a useful input for `WorkflowRun`, `Evidence`, `ChainEvent`, and `OffchainJob`.

## Risks

- Security risk: a platform model can accidentally imply the backend is trusted or signs for users. The design must state that Web/backend records are orchestration and audit metadata, not chain finality.
- Data integrity risk: object IDs and references can blur chain state, evidence artifacts, and database records. The model must separate chain identity, artifact digest/path, and platform record identity.
- Experiment validity risk: mapping Stage 14 evidence into platform objects must not change metric definitions or paper-facing claims.
- Compatibility risk: choosing a schema format that requires new tooling could slow later stages. Prefer plain TypeScript interfaces or JSON schema that can be checked with existing tools.
- Scope risk: backend table design, API endpoints, and event indexing are tempting but belong to Stage 17/18 unless kept as non-normative notes.
- Paper-facing claim risk: mappings for future modules must be placeholders, not claims of implemented support.

## Stop Conditions

Implementation agent must stop and ask Codex/Owner before:

- changing chain protocol fields, proof digest/attestation encoding, hash algorithms, settlement semantics, or verifier assumptions;
- changing Stage 14 evidence schema, validation scenario definitions, experiment metrics, or graph/report labels;
- introducing schema libraries, backend frameworks, database dependencies, or generated code;
- turning placeholder modules into claimed implemented capabilities;
- changing deployment topology, chain IDs, specs, keys, or live-chain runbooks;
- finding conflicting formal docs about current data trade behavior.

## Branch And Commit Plan

- Continue on branch `stage/stage15-platform-business-model`.
- This is a docs/schema design stage. Commit is allowed after the formal doc, schema/type draft, docs index update, and Execution Record are complete and verified.
- Keep generated evidence and local validation output under `.agents/fwf/runs/stage15/...` if any is needed; do not commit `.agents/`.
- Recommended implementation commit message:

```text
docs(platform): define stage15 business model

Plan: docs/internal/agent-plans/2026-06-29-stage15-platform-business-model.md
Validation:
- <commands run>
```

## Task List

- [ ] Re-read the required inputs: `agent.md`, `docs/internal/agent-collaboration.md`, this plan, `docs/architecture/platform-architecture.md`, `docs/implementation/data-trade-implementation.md`, `docs/experiments/data-trade-validation.md`, and `docs/implementation/data-trade-stage14-evidence-index.md`.
- [ ] Create `docs/architecture/platform-business-model.md` with a clear status note that it is the Stage 15 design baseline, not backend implementation.
- [ ] Define shared model principles: user-signed chain actions, backend as orchestration/indexing layer, chain events as recoverable state source, evidence as audit metadata, and scene-specific extensions instead of data-trade hard-coding.
- [ ] Specify the core objects: `User`, `ChainAccount`, `Dataset`, `DataAsset`, `BusinessTask`, `WorkflowRun`, `Evidence`, `ChainEvent`, and `OffchainJob`.
- [ ] For each object, document purpose, required identity fields, key references, lifecycle/status values, chain/evidence links, and what must not be stored or trusted.
- [ ] Define cross-object relationships, especially `User` to `ChainAccount`, `DataAsset` to `Dataset`, `BusinessTask` to `WorkflowRun`, `WorkflowRun` to `Evidence`, `Evidence` to `ChainEvent`, and `OffchainJob` to artifacts/results.
- [ ] Add a data-trade mapping table from current fields/events to platform objects, covering listing, request, escrow, session, round constraints, proof digest, business input hash, verifier attestation, settlement, dispute, and Stage 14 evidence summary fields.
- [ ] Add placeholder mapping tables for data collection, cross-domain flow, and verifiable training. Mark them as target abstractions only and avoid claiming they are implemented.
- [ ] Add an initial schema/type draft under `scripts/platform-model/`. Prefer `types.ts` with exported TypeScript interfaces and literal union status types because the repo already has TypeScript tooling under `monitor/` but no root schema validation dependency.
- [ ] Keep the type draft dependency-free and backend-neutral. It may use `string` aliases such as `HexHash`, `ChainKey`, `PlatformId`, and `IsoTimestamp`.
- [ ] If TypeScript checking is awkward outside `monitor/`, add a minimal syntax-only validation path rather than a new dependency.
- [ ] Update `docs/README.md` to index `docs/architecture/platform-business-model.md`.
- [ ] Update this plan's Execution Record with files changed, validation commands, skipped validations, deviations, and remaining risks.

## Acceptance Criteria

- A formal platform model document exists and is indexed from `docs/README.md`.
- The document covers all Stage 15 core objects and explicitly separates platform metadata from chain-trusted state.
- Data trade is mapped onto the shared platform model without redefining the platform as data trade.
- Future data collection, cross-domain flow, and verifiable training mappings are present and clearly marked as placeholders.
- A dependency-free schema/type draft exists and can be inspected or syntax-checked with repository tooling.
- No chain code, proof code, deployment config, experiment metric, or Stage 14 scenario behavior changes.
- The Execution Record contains actual command evidence for file existence, diff summary, and validation.

## Validation Commands

Run these at minimum:

```bash
git status --short
test -f docs/architecture/platform-business-model.md
test -f scripts/platform-model/types.ts
rg -n "platform-business-model" docs/README.md
rg -n "User|ChainAccount|Dataset|DataAsset|BusinessTask|WorkflowRun|Evidence|ChainEvent|OffchainJob" docs/architecture/platform-business-model.md
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/data_trade_validation_summary.js
```

If a TypeScript checker is available without adding dependencies, also run one of:

```bash
npx tsc --noEmit --target ES2022 --module NodeNext --moduleResolution NodeNext scripts/platform-model/types.ts
```

or, if the implementation places the type draft under `monitor/src/`, run:

```bash
cd monitor && npm run build
```

If the TypeScript check cannot run because local dependencies are unavailable, record the exact reason and still verify the file exists and is dependency-free by inspection.

## Validation Output Paths

No generated evidence is required for this docs/schema stage. If implementation writes command logs for review, use:

```text
.agents/fwf/runs/stage15/
```

Do not commit `.agents/fwf/runs/stage15/`.

## Documentation Updates

Required:

- `docs/architecture/platform-business-model.md`
- `docs/README.md`

Optional if useful:

- `docs/implementation/data-trade-stage14-evidence-index.md`

Do not update paper-facing experiment conclusions unless Codex/Owner explicitly approves that scope.

## Plan-Review Focus

Ask opencode plan review to focus on:

- whether the plan gives enough detail to create a platform model without backend/API scope creep;
- whether the selected schema/type location is appropriate and lightweight;
- whether validation commands are sufficient for a docs/schema stage;
- whether any current data-trade facts or limitations are missing from the mapping requirements;
- whether future-module placeholders are clearly non-claims.

## Execution Record

### 2026-06-29 Codex Plan Authoring

- Branch: `stage/stage15-platform-business-model`
- Commits:
- Tasks completed:
  - Authored initial Stage 15 plan from the long-term roadmap.
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
  - Plan review should confirm the type draft location before implementation, because the repository has TypeScript tooling scoped under `monitor/` but not at the root.
