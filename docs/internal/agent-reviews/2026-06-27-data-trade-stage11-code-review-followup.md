# Stage 11 Failure/Dispute Scenario Code Review Follow-up

> Review owner: Codex
>
> Implementation owner: CodeWhale
>
> Branch: `feat/data-trade-stage11-failure-scenarios`
>
> Decision: **changes requested**

## Previous Findings

### High: `invalid-proof-dispute` cannot reach the dispute path

Status: **fixed**.

`invalid-proof-dispute` now submits the valid proof digest through `submitRoundProofAccepted()` so `submitDataProof` can pass the pallet's internal digest check. It then calls `disputeInvalidProof()` with a deterministic bad digest that differs from the submitted digest and records both values in `scenario_outcome`.

This resolves the previous blocker.

### Low: Required documentation updates are mostly missing

Status: **partially fixed**.

`docs/architecture/data-trade-security-model.md` was updated, and `docs/implementation/data-trade-implementation.md` had already been updated. However, the plan also required:

- `docs/implementation/data-trade-evidence.md`
- `docs/implementation/data-trade-paper-gap-matrix.md`
- `docs/internal/agent-plans/2026-06-26-data-trade-long-term-scripted-e2e-roadmap.md`

These are still missing Stage 11 updates.

Required fix:

- Add Stage 11 evidence format guidance, especially `scenario`, `scenario_outcome`, expected events, and result strings.
- Update the gap matrix to record that failure/dispute paths are script-demonstrated over existing pallet behavior, while production timeout/trustless dispute verification remains a gap.
- Update the long-term roadmap Stage 11 progress section.

## New Findings

### Medium: Scenario terminal events are not asserted

- File: `scripts/zk_real_data_trade_flow.js:327`
- Plan reference: `docs/internal/agent-plans/2026-06-27-stage11-data-trade-failure-dispute-scenarios.md:426`

The Stage 11 plan requires:

```text
Use findEvent() to assert expected child/main events after the relevant extrinsic.
Record the expected event names in evidence.
Do not rely only on log output.
```

The implementation records event names in `scenario_outcome`, but it does not call `findEvent()` on the relevant transaction results:

- `disputeInvalidProof(...)` should assert `tradeSession.SessionPunished`.
- `disputeInvalidPlaintext(...)` should assert `tradeSession.SessionPunished`.
- `tradeSession.claimLastPayment(...)` should assert `tradeSession.LastPaymentClaimed`.
- `mainEscrow.punishDataOwner(...)` should assert `mainEscrow.EscrowPunished`.
- `mainEscrow.claimLastPayment(...)` should assert `mainEscrow.EscrowSettled`.

Without these checks, a live scenario can record expected event names without proving the expected terminal event occurred. This is exactly the paper-facing evidence risk Stage 11 was meant to avoid.

Required fix:

- Capture each relevant `submitTx()` result.
- Call `findEvent(result, section, method)` immediately after the extrinsic.
- Record either the asserted event names or a small object such as:

```json
"events": [
  "tradeSession.SessionPunished",
  "mainEscrow.EscrowPunished"
]
```

- Keep the existing `scenario_outcome` shape; just make it backed by actual event assertions.

## Validation Performed

Passed:

```bash
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js
go -C tools/data-trade-zk test ./...
go -C tools/data-trade-zk build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
```

Passed:

```bash
rm -rf target/data-trade-zk/session-0-round-0 /tmp/fishbone-stage11-followup-dry
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --evidence-out /tmp/fishbone-stage11-followup-dry/evidence.json \
  --dry-run-dynamic
test -f /tmp/fishbone-stage11-followup-dry/evidence.json
```

Result: passed.

Passed:

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --scenario invalid-proof-dispute \
  --dry-run-dynamic
```

Result: exited with code `2` and rejected non-happy dry-run scenario, as expected.

Passed:

```bash
node scripts/zk_real_data_trade_flow.js --scenario does-not-exist
```

Result: exited with code `2` and rejected unknown scenario.

## Notes

- No live chain scenario E2E was run during this follow-up review.
- The high-risk invalid-proof flow is now structurally aligned with the pallet digest check, but the scenario still needs event assertions before approval.
