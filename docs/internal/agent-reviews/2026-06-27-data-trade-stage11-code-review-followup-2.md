# Stage 11 Failure/Dispute Scenario Code Review Follow-up 2

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

The scenario now submits the valid digest first, then calls `disputeInvalidProof()` with a deterministic bad digest. This is aligned with the pallet's `submitDataProof` digest check.

### Medium: Scenario terminal events are not asserted

Status: **fixed**.

The script now captures relevant `submitTx()` results and calls `findEvent()` for expected terminal events:

- `tradeSession.SessionPunished`
- `mainEscrow.EscrowPunished`
- `tradeSession.LastPaymentClaimed`
- `mainEscrow.EscrowSettled`

This satisfies the Stage 11 requirement that evidence-backed scenarios assert expected chain events instead of only recording expected names.

### Low: Required documentation updates are mostly missing

Status: **partially fixed, still open**.

Fixed:

- `docs/architecture/data-trade-security-model.md`
- `docs/implementation/data-trade-implementation.md`
- `docs/internal/agent-plans/2026-06-26-data-trade-long-term-scripted-e2e-roadmap.md`

Still missing:

- `docs/implementation/data-trade-evidence.md`
- `docs/implementation/data-trade-paper-gap-matrix.md`

Required fix:

- Add Stage 11 evidence guidance to `docs/implementation/data-trade-evidence.md`, including:
  - `--scenario` names;
  - `scenario_outcome`;
  - asserted event names;
  - result strings;
  - statement that live chain scenario evidence was not produced unless actually run.
- Update `docs/implementation/data-trade-paper-gap-matrix.md` to reflect:
  - failure/dispute paths are now script-demonstrated in the ZK/dynamic script;
  - production timeout/challenge-period logic, trustless dispute verification, and on-chain ZK verification remain gaps.

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
rm -rf target/data-trade-zk/session-0-round-0 /tmp/fishbone-stage11-final-dry
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --evidence-out /tmp/fishbone-stage11-final-dry/evidence.json \
  --dry-run-dynamic
test -f /tmp/fishbone-stage11-final-dry/evidence.json
```

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

- No live chain scenario E2E was run during this review.
- The remaining work is documentation only, but it is still required because Stage 11 evidence is paper-facing.
