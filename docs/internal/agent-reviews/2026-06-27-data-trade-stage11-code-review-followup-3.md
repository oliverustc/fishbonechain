# Stage 11 Failure/Dispute Scenario Code Review Follow-up 3

Date: 2026-06-27
Branch: `feat/data-trade-stage11-failure-scenarios`
Base intent: re-review CodeWhale's documentation supplement after follow-up 2.

## Decision

Changes requested.

The Stage 11 code-level findings remain resolved, but the evidence documentation is still missing one explicit item from the prior review: the concrete `result` strings emitted by each scenario.

## Finding

### Medium: evidence doc still does not list Stage 11 `result` strings

File: `docs/implementation/data-trade-evidence.md`

The latest supplement adds the Stage 11 scenario names, expected asserted events, `scenario_outcome`, and the live-chain caveat. However, it only says evidence contains a `result` field; it does not document the actual values emitted by `scripts/zk_real_data_trade_flow.js`:

- `invalid-proof-dispute` -> `expected-dispute-accepted`
- `invalid-plaintext-dispute` -> `expected-plaintext-dispute-accepted`
- `requester-refuses-payment` -> `expected-last-payment-claimed`

This was called out explicitly in follow-up 2 because the Stage 11 evidence is paper-facing. Without the exact strings, downstream agents and paper evidence readers cannot reliably interpret scenario outputs without inspecting the script.

Required fix:

- Add the three scenario-to-result mappings to `docs/implementation/data-trade-evidence.md`.
- Keep the existing live-chain caveat: no live chain scenario evidence has been produced unless the scenarios are actually run against RPC.

## Checks Performed

```bash
git status --short --branch
git log --oneline -6
git show --stat --oneline --decorate --no-renames HEAD
rg -n "Stage 11|scenario|invalid-proof-dispute|invalid-plaintext-dispute|requester-refuses-payment|SessionPunished|EscrowPunished|LastPaymentClaimed|live chain|challenge|timeout|trustless|on-chain ZK|链上|争议|失败" docs/implementation/data-trade-evidence.md docs/implementation/data-trade-paper-gap-matrix.md docs/architecture/data-trade-security-model.md docs/implementation/data-trade-implementation.md docs/internal/agent-plans/2026-06-26-data-trade-long-term-scripted-e2e-roadmap.md docs/internal/agent-reviews/2026-06-27-data-trade-stage11-code-review-followup-2.md
rg -n "expected-dispute-accepted|expected-plaintext-dispute-accepted|expected-last-payment-claimed|scenario_outcome|result:" scripts/zk_real_data_trade_flow.js docs/implementation/data-trade-evidence.md docs/internal/agent-plans/2026-06-27-stage11-data-trade-failure-dispute-scenarios.md
git diff main...HEAD -- docs/implementation/data-trade-evidence.md docs/implementation/data-trade-paper-gap-matrix.md scripts/zk_real_data_trade_flow.js
```

No live-chain E2E was run during this review.

## Merge Status

Not merged. Merge to `main` should happen only after the evidence doc lists the exact Stage 11 result strings.
