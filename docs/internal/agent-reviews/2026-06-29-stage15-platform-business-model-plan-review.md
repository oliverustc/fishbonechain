# Stage 15 Plan Review: Platform Business Model Design

- **Date**: 2026-06-29
- **Branch**: `stage/stage15-platform-business-model`
- **Plan reviewed**: `docs/internal/agent-plans/2026-06-29-stage15-platform-business-model.md`
- **Reviewer**: opencode (plan review)

## Scope Reviewed

The entire Stage 15 plan: goal, scope, non-goals, stop conditions, current facts, risks, task list, acceptance criteria, validation commands, documentation updates, and Execution Record.

## Inputs Read

- `agent.md`
- `docs/internal/agent-collaboration.md`
- `docs/internal/agent-plans/2026-06-29-stage15-platform-business-model.md`
- `docs/internal/agent-plans/2026-06-28-data-flow-platform-long-term-roadmap.md`
- `docs/architecture/platform-architecture.md`
- `docs/implementation/data-trade-implementation.md`
- `docs/experiments/data-trade-validation.md`
- `docs/implementation/data-trade-stage14-evidence-index.md`
- `docs/README.md`
- `package.json` (root and `monitor/`)
- `scripts/lib/data_trade_validation_summary.js`
- `scripts/zk_real_data_trade_flow.js` (header)
- `git status --short`, `git branch --show-current`

## Current Fact Verification

| Claim in plan | Verified? | Evidence |
|---|---|---|
| `platform-architecture.md` defines platform/scene split | Yes | File exists, `docs/architecture/platform-architecture.md:7-24` |
| `data-trade-implementation.md` records data trade modules | Yes | File exists, covers registry, session, escrow, zk, bridge, E2E |
| `data-trade-validation.md` defines evidence schema | Yes | `docs/experiments/data-trade-validation.md:118-137` |
| `data-trade-stage14-evidence-index.md` maps `summary.scenarios[]` to platform objects | Yes | `docs/implementation/data-trade-stage14-evidence-index.md:140-151` |
| `scripts/zk_real_data_trade_flow.js` writes per-run evidence | Yes | File exists (567 lines), supports `--evidence-out` flag |
| Root `package.json` only has ESM + `@polkadot/api` | Yes | `package.json:1-5` |
| `monitor/package.json` has TypeScript tooling | Yes | `monitor/package.json:7-21` (typescript ^5.7, tsc, tsx) |
| No current backend model/schema package | Yes | No `scripts/platform-model/` directory, no schema packages |
| `data_trade_validation_summary.js` exists | Yes | `scripts/lib/data_trade_validation_summary.js:1-311` |

All current facts in the plan are backed by repository evidence.

## Decision

**approved-with-required-fixes**

## Required Fixes

### 1. Resolve TypeScript type draft location ambiguity

The plan proposes `scripts/platform-model/types.ts` as the preferred path but also mentions `monitor/src/` as an alternative. The validation command (`npx tsc --noEmit ...`) can only work at the root if `typescript` is globally installed or reachable from `monitor/node_modules/.bin/`, which is not guaranteed. The implementer should not need to choose between two locations.

**Fix**: Commit to one location before implementation:
- **Option A** (`scripts/platform-model/types.ts`): Accept that `npx tsc --noEmit` will not work without installing `typescript` at root. Fall back to `node --check` (valid syntax-only check for type-only .ts) or manual inspection. This keeps the draft lightweight and dependency-free.
- **Option B** (`monitor/src/platform-model/types.ts`): Use `cd monitor && npm run build` as the TypeScript check. This ties the platform model to the monitor package scope.

Recommend Option A. The plan should state the chosen path and the exact validation fallback.

### 2. Define "dependency-free by inspection" concretely

The plan says "If the TypeScript check cannot run ... still verify the file exists and is dependency-free by inspection." This is too vague to audit in the Execution Record.

**Fix**: Add a concrete verification step, e.g.:
```bash
# Verify no imports/requires (dependency-free)
rg -c '^\s*(import|require)\s' scripts/platform-model/types.ts || echo "0 deps"
# Verify parseable as plain object declarations
node -e 'require("fs").readFileSync("scripts/platform-model/types.ts","utf8"); process.exit(0)'
```

Or, if the file uses only TypeScript `interface`/`type`/`export` declarations without runtime code, `node --check` can catch syntax errors even without type-checking.

### 3. Add explicit directory creation step

The task says "Add an initial schema/type draft under `scripts/platform-model/`" but does not list directory creation. Since `scripts/platform-model/` does not exist yet, this is an implicit step that should be explicit.

**Fix**: Add a task item before the type draft: "Create `scripts/platform-model/` directory if using that path."

### 4. Expand data-trade mapping task to cover per-constraint fields

The plan says to map "current fields/events to platform objects" but does not explicitly mention the per-constraint proof binding fields (`constraints[].proof_digest`, `business_input_hash`, `vk_hash`, `public_input_hash`, `on_chain_bound`) that form a significant portion of the evidence schema and chain state.

**Fix**: Add to the data-trade mapping task: "Include per-constraint proof binding fields (proof_digest, business_input_hash, vk_hash, public_input_hash) and their chain state sources (RoundState fields)."

## Suggested Improvements

1. **Cross-reference security model**: The new `platform-business-model.md` should reference `docs/architecture/data-trade-security-model.md` when discussing participant roles and trust assumptions, to avoid reinventing or contradicting existing security analysis.

2. **Validation command addition**: Add `node --check scripts/platform-model/types.ts` as a validation command. TypeScript files that only contain `interface`, `type`, and `export` declarations (no runtime imports) are syntactically valid JavaScript and will pass `node --check`.

3. **Evidence summary schema reading**: The plan could reference `scripts/lib/data_trade_validation_summary.js` as a concrete input for the mapping table, since it defines the exact `summary.scenarios[]` fields (lines 162-169) that need to be mapped to `WorkflowRun`/`Evidence`/`ChainEvent` objects.

4. **docs/README.md placement**: Clarify that `platform-business-model.md` should be placed under an "Architecture" sub-section or a new "平台模型" section in `docs/README.md`. Currently the README has "架构与设计" which is the natural home.

## Risks If Unchanged

1. **TypeScript location ambiguity**: If the implementer chooses a location different from what the reviewer or Codex expects, it creates a review finding that wastes a pass. Low severity but avoidable.

2. **"Dependency-free by inspection" not auditable**: Without a concrete check, a future audit cannot verify that no hidden dependencies were introduced. Low severity for this docs stage, but violates the spirit of the Evidence Rules (every claim must be verifiable by command output).

3. **Missing per-constraint mapping**: The mapping table could omit proof binding fields that are critical for understanding how chain state maps to platform evidence objects. Medium impact — would require a follow-up mapping pass.

## Questions for Codex/Owner

1. Should the platform model document include a lifecycle state diagram for each core object (e.g., pending → active → completed → disputed), or are field-level status values sufficient at this stage?

2. The roadmap mentions both "JSON schema" and "TypeScript type 草案". Is the preference for TypeScript interfaces confirmed, or should a JSON Schema (`types.schema.json`) also be produced for tool-independent validation?

3. Should the `docs/implementation/data-trade-stage14-evidence-index.md` update be required or optional? The plan marks it optional but the stage14 evidence index already has a "与未来平台对象的映射" section that would duplicate or diverge from `platform-business-model.md`.

## Verification Performed

```bash
git status --short                    # clean working tree
git branch --show-current             # stage/stage15-platform-business-model
test -f docs/architecture/platform-business-model.md  # does not exist yet (expected)
test -f scripts/platform-model/types.ts                # does not exist yet (expected)
test -f scripts/zk_real_data_trade_flow.js             # exists
test -f scripts/lib/data_trade_validation_summary.js   # exists
rg -n "platform-architecture" docs/README.md           # line 14
ls scripts/platform-model/ 2>&1                       # directory does not exist
```

Plan facts are consistent with current repository state.
