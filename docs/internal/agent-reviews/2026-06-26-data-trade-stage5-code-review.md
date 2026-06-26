# Data Trade Stage 5 Code Review

## Scope

Reviewed commits on branch `main`:

- `afe0720 docs: plan data trade stage 5 hardening`
- `4fc96c4 docs: record data trade evidence and paper gap matrix`
- `7c9202f docs: recommend data trade stage 6 target`

## Reviewer

CodeWhale

## Inputs Read

- `docs/internal/agent-plans/2026-06-26-stage5-data-trade-paper-grade-hardening.md` (362 lines, full plan + Execution Record)
- `docs/implementation/data-trade-evidence.md` (new, 184 lines)
- `docs/implementation/data-trade-paper-gap-matrix.md` (new, 63 lines)
- `docs/implementation/data-trade-implementation.md` (modified, +5 lines)
- `docs/README.md` (modified, +2 lines)
- `references/data_trade_paper/main.tex` (re-read for paper claim cross-reference)
- `git diff --stat 75f39a7..7c9202f`
- `git diff 75f39a7..7c9202f -- docs/implementation/data-trade-implementation.md docs/README.md`

## Findings

### 1. Recommended: Evidence Document References Plan Commit, Not Deliverable Commit

- Severity: Low
- File: `docs/implementation/data-trade-evidence.md:14`
- Issue: The evidence document records "Current Commit: `afe0720a19ebd22b908f9206fd25817381cf76c4`", which is the plan creation commit, not the commit at which the evidence document itself was written or committed. The local validation was run against the tree at `afe0720`, which differs from the final deliverable tree only in the new documentation files — no code changed between these commits, so the recorded test results remain valid.
- Why it matters: A future reader running `git checkout afe0720` will not find the evidence document because it was created after that commit. If someone attempts to reproduce the recorded validation, they may look at the wrong commit.
- Suggested fix: Add a note that the test commands were run against the plan commit (`afe0720`) before the evidence document was committed, and that no code files changed between the validation commit and the final Stage 5 deliverable. Alternatively, rerun the local validation and JS checks on the final commit and update the commit reference.

### 2. Recommended: Paper Gap Matrix Constraint Category Wording Could Sharpen Range Claim

- Severity: Low
- File: `docs/implementation/data-trade-paper-gap-matrix.md`, row "Circuit-level range business witness"
- Issue: The gap column says "Only range; fixture data static." The Paper Wording Guidance section later expands this well, but the matrix row itself doesn't mention that the circuit already proves three relationships (range inclusion, masked value derivation, MiMC commitment). The distinction between "range" (one word) and "range inclusion + masked derivation + commitment" is important for paper claims.
- Why it matters: The gap matrix is the go-to reference for paper writers. The word "only" undersells what Stage 2.2 already delivers — three circuit-level constraints, not one.
- Suggested fix: Change "Only range; fixture data static" to "Range inclusion, masked derivation, and MiMC commitment are proven at circuit level; fixture data is static and single-field."

### 3. Informational: Stage 5 Diff Range Includes Unrelated Files

- Severity: Informational
- File: `git diff --stat 75f39a7..7c9202f`
- Issue: The diff stat shows `agent.md` (+11/-4) and `agent-collaboration.md` (+253) as changed files in the Stage 5 diff range. These belong to the merge commit `1c45896` and `b8862e5` between Stage 4 and Stage 5, not to Stage 5 commits themselves.
- No action required: This is an artifact of the diff range spanning a merge. The three Stage 5 commits (`afe0720`, `4fc96c4`, `7c9202f`) only touch plan files and formal docs.

## Positive Observations

These are not findings requiring change, but observations worth recording:

1. **Honest VM recording.** When child6/child7 RPC timed out, the Execution Record explicitly marked the VM rerun as blocked rather than fabricating success or silently skipping. The historical regression summary was preserved separately with its own dated evidence. This is exactly the right posture for paper-grade evidence.

2. **Plan self-correction.** The Execution Record notes the initial `wait_for_ws_chain.js` command was wrong (used `--ws` instead of `--main/--child`) and the plan was corrected. Self-correction during execution is a sign of attentive plan execution.

3. **Paper Wording Guidance.** The gap matrix includes a dedicated "Paper Wording Guidance" section that gives safe current claims, prohibited claims, and a recommended thesis phrasing template. This goes beyond what the plan required and directly serves the paper-writing use case.

4. **Stage 6 recommendation quality.** The five-candidate scoring table with five dimensions (paper relevance, security improvement, engineering risk, demonstrability, time cost) is concrete and falsifiable. The recommendation (full IMT membership + business witness coupling) follows naturally from the scores and is scoped to avoid scope creep in Stage 6.

5. **Gap matrix completeness.** All 22 plan-required rows are present, each with a specific file/artifact evidence reference rather than vague hand-waving. The status values (`implemented`, `prototype-supported`, `partially-supported`, `not-implemented`, `future-work`) are applied consistently.

## Accepted Risks

- Stage 5 is docs-only. No Rust, Go, JS, runtime, deployment, or experiment behavior changed.
- VM regression was not rerun during this stage. The historical regression summary (`target/data-trade-vm-regression/summary.md`, dated 2026-06-16) is preserved as the last full clean regression. Codex explicitly recorded this as blocked, not skipped.
- Local validation (Rust 40 tests, Go ZK tests, JS syntax 15 files) was run and passed on the plan commit tree. No code files differ between the validation commit and the final Stage 5 commits.
- The Stage 6 recommendation is recorded but not implemented. This is consistent with the plan's explicit boundary: "Do not start Stage 6 implementation inside this plan."

## Verification Performed

- Confirmed `docs/implementation/data-trade-evidence.md` exists (184 lines).
- Confirmed `docs/implementation/data-trade-paper-gap-matrix.md` exists (63 lines).
- Confirmed `docs/README.md` indexes both new files.
- Confirmed `docs/implementation/data-trade-implementation.md` links to both new files under `### 证据与论文差距`.
- Confirmed all 22 plan-required rows are present in the gap matrix.
- Confirmed no document claims on-chain Groth16 verification, trustless bridge settlement, full IMT membership, subset/substr support, or production verifier quorum.
- Confirmed Execution Record records both blocked VM rerun and historical regression summary separately.
- Did not rerun local tests or JS syntax checks — the Execution Record's recorded results are accepted as evidence (no code changed between validation and final commits).

## Decision

`approved`

No required fixes. The two recommended findings (commit reference clarity, gap matrix wording) are low-severity improvements that can be applied at the author's discretion before merge.
