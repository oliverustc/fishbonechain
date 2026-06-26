# Agent Collaboration Protocol

This document defines how agents collaborate on FishboneChain. It is intentionally practical and versioned in the repository so any new agent session can recover the working model without relying on chat history.

The protocol will evolve. When a workflow breaks down, update this file in the same change set that fixes the process.

## Roles

### Owner

The human owner decides project priority, accepts tradeoffs, and chooses when imperfect results are good enough for the paper, defense, or experiment.

Owner responsibilities:

- Pick the next milestone or ask Codex to propose one.
- Decide whether accepted risks are tolerable.
- Assign implementation work to CodeWhale or ask Codex to intervene directly.
- Approve destructive or environment-changing operations when needed.

### Codex

Codex is the architecture, planning, and review agent. Use Codex when the work needs judgment, cross-file reasoning, security boundaries, experiment methodology, or recovery from ambiguity.

Codex responsibilities:

- Write implementation plans with clear scope, non-goals, ordered tasks, acceptance criteria, and validation commands.
- Review CodeWhale plans before implementation when the task is risky or underspecified.
- Review CodeWhale code after implementation, focusing on correctness, security, regressions, missing tests, and documentation drift.
- Answer CodeWhale questions when requirements, architecture, or safety boundaries are ambiguous.
- Update collaboration/process docs when the team workflow changes.
- Intervene with direct implementation only when:
  - the issue blocks progress and needs high-context reasoning;
  - the change is small and unblocks CodeWhale;
  - the owner explicitly asks Codex to implement;
  - a review finding is subtle enough that delegating the fix would be slower or risky.

Codex should not be used as the default bulk implementation agent when a clear plan can be executed by CodeWhale.

### CodeWhale

CodeWhale is the implementation agent. Use CodeWhale for planned code changes, tests, docs updates, experiment scripts, and mechanical follow-through.

CodeWhale responsibilities:

- Read `agent.md`, this protocol, and the assigned plan before editing.
- Execute the plan task by task.
- Keep changes scoped to the assigned task.
- Run the validation commands listed in the plan, or explicitly record why a command could not be run.
- Update the plan `Execution Record` with commits, tests, deviations, and unresolved questions.
- Stop and ask Codex or the owner before changing scope, security assumptions, experiment metrics, settlement semantics, proof/digest fields, VM topology, or data used for paper claims.

CodeWhale should not silently redesign architecture, broaden scope, or convert an accepted limitation into a claimed guarantee.

## Default Workflow

Use this flow for non-trivial tasks.

1. Owner asks Codex for a plan.
2. Codex writes or updates a plan in `docs/internal/agent-plans/`.
3. Optional: Owner asks another agent to review the plan.
4. CodeWhale executes the plan on a task branch.
5. CodeWhale records execution details in the plan.
6. Codex performs code review and writes a review record in `docs/internal/agent-reviews/`.
7. CodeWhale fixes required review findings.
8. Codex or owner accepts remaining risks.
9. The final change updates formal docs when behavior, commands, metrics, deployment, or paper claims changed.
10. Merge or commit with messages that reference the plan and review records.

For small tasks, steps can be compressed, but the final commit should still be understandable from code, tests, and docs.

## Required Reading

Every agent session starts with:

1. `agent.md`
2. `docs/internal/agent-collaboration.md`
3. The assigned plan or review file
4. Relevant formal docs under `docs/`
5. Relevant code and tests

Do not treat `docs/internal/agent-plans/` as current truth. Plans are process records. Current truth comes from code and formal docs, especially:

- `docs/README.md`
- `docs/development/developer-guide.md`
- `docs/implementation/implementation-record.md`
- task-specific formal docs such as `docs/implementation/data-trade-implementation.md`

## Plan Standard

Plans live in `docs/internal/agent-plans/YYYY-MM-DD-topic.md`.

Each plan should include:

- Goal: one concrete outcome.
- Scope: files, modules, or behavior that may change.
- Non-goals: tempting work that must not be done.
- Current facts: what is already implemented and where.
- Risks: security, data integrity, experiment validity, deployment, performance, or compatibility risks.
- Task list: checkbox steps in execution order.
- Acceptance criteria: what must be true at the end.
- Validation commands: exact commands to run.
- Documentation updates: formal docs that must be updated.
- Execution Record: append-only notes from the implementing agent.

Plan tasks should be small enough that CodeWhale can execute them without architectural invention.

## Review Standard

Reviews live in `docs/internal/agent-reviews/YYYY-MM-DD-topic-review.md`.

Use separate files for plan review and code review when both exist:

- `YYYY-MM-DD-topic-plan-review.md`
- `YYYY-MM-DD-topic-code-review.md`

Each review should include:

- Scope reviewed.
- Reviewer.
- Inputs read: commits, files, plan, docs, test output.
- Findings ordered by severity.
- Required changes.
- Accepted risks.
- Verification performed.
- Decision: `approved`, `approved-with-required-fixes`, or `blocked`.

Code review findings must cite file paths and line numbers when possible.

## Execution Record Standard

The implementing agent appends to the assigned plan:

```markdown
## Execution Record

### YYYY-MM-DD CodeWhale Pass N

- Branch:
- Commits:
- Tasks completed:
- Tests run:
- Tests not run:
- Deviations from plan:
- Questions for Codex/Owner:
- Remaining risks:
```

Do not rewrite earlier execution records except to fix factual typos.

## Commit Message Standard

Commit messages should describe the code change and reference process records. Do not put long review transcripts in commit messages.

Recommended format:

```text
type(scope): concise change summary

Plan: docs/internal/agent-plans/YYYY-MM-DD-topic.md
Review: docs/internal/agent-reviews/YYYY-MM-DD-topic-code-review.md
Validation:
- command 1
- command 2
```

Use a short message for tiny changes. Use the body when traceability matters.

## Branching

Use a task branch for implementation when the work is non-trivial:

```text
feature/data-trade-stage4-security-model
fix/data-trade-proof-digest-binding
docs/agent-collaboration-protocol
```

Before merge:

- Working tree must be clean except intentionally ignored local artifacts.
- Validation commands must be recorded.
- Formal docs must reflect current behavior.
- Review records must be committed if a review was performed.

## When CodeWhale Must Stop

CodeWhale must stop and ask Codex or the owner before:

- Changing proof digest fields, encoding, hash algorithms, or attestation payloads.
- Changing settlement rules or who can release funds.
- Changing bridge trust assumptions or claiming trustless behavior.
- Changing experiment metrics, denominators, or graph labels used in paper/defense claims.
- Replacing measured data with synthetic data.
- Mutating deployment topology, VM assignments, chain IDs, genesis specs, or keys outside the plan.
- Introducing a new dependency that affects build, security, or reproducibility.
- Broadening the task to include unrelated refactors.
- Encountering test failures that contradict the plan's assumptions.

## When Codex Should Intervene

Codex should take direct action when:

- A plan is ambiguous enough that CodeWhale would need to invent architecture.
- A review finding requires subtle reasoning about runtime semantics, proof security, or experiment validity.
- The implementation is blocked by conflicting docs or unclear current truth.
- The owner needs a decision memo comparing tradeoffs.
- A failed merge or dirty worktree risks losing user work.

## Documentation Rules

Process records are not formal project truth.

- Put plans and reviews in `docs/internal/`.
- Put stable behavior, commands, architecture, and paper-facing claims in formal `docs/` files.
- Update `docs/README.md` when adding a new formal document.
- Update `agent.md` when the onboarding path or collaboration contract changes.

If a plan is completed, it should either:

- link to the formal doc that now records the current truth; or
- explicitly state that it is a historical process record.

## Data Trade Specific Guardrails

For data-trade work, agents must preserve these current facts unless a plan explicitly changes them:

- `pallet-trade-session` currently verifies proof digest and verifier attestation, not Groth16 proofs on-chain.
- Runtime uses `AlwaysPassVerifier` plus `VerifierAuthority = Charlie` in the current prototype.
- `business_input_hash` is part of the proof digest binding.
- Stage 2.2 business range proof uses MiMC for `masked_value_hash`.
- MainEscrow is implemented; `FmcAssisted` and `Hybrid` remain future settlement modes.
- Bridge/session-escrow binding is checked off-chain, not by a trustless cross-chain proof.

Do not claim stronger security than the code provides.

## Experiment Specific Guardrails

For experiment work:

- Preserve raw data and record which run generated each figure.
- Do not change metric definitions without updating the CSV schema, plotting labels, and formal experiment docs.
- Generated figures must match the plotting scripts committed with them.
- If measured values miss an original target but the owner accepts them, record that explicitly in the plan or formal experiment doc.

## Updating This Protocol

This protocol is expected to improve over time. Any agent may propose an update, but changes should be explicit and committed.

When updating:

- Keep it short enough to be read at session start.
- Add rules only after a real need appears.
- Prefer checklists and stop conditions over vague advice.
