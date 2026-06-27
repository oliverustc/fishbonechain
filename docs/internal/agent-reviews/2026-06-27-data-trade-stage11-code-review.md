# Stage 11 Failure/Dispute Scenario Code Review

> Review owner: Codex
>
> Implementation owner: CodeWhale
>
> Branch: `feat/data-trade-stage11-failure-scenarios`
>
> Decision: **changes requested**

## Findings

### High: `invalid-proof-dispute` cannot reach the dispute path

- File: `scripts/zk_real_data_trade_flow.js:315`
- Relevant code:
  - `badDigest` is derived from `chainArt.proof_digest` at `scripts/zk_real_data_trade_flow.js:321`.
  - `submitDataProof` submits the original `ch/ro/public/vk/business` metadata but replaces only `proof_digest` with `badDigest` at `scripts/zk_real_data_trade_flow.js:328`.
  - `trade-session` recomputes the expected digest from the submitted metadata and rejects mismatches before storing the proof at `pallets/trade-session/src/lib.rs:425`.

Because `submitDataProof` validates `expected_digest == proof_digest`, this scenario will fail at `submitDataProof(... with bad digest)` with `InvalidProof`. It will not reach `disputeInvalidProof`, will not emit `SessionPunished`, and will not call `mainEscrow.punishDataOwner`.

This breaks one of the three required Stage 11 scenarios.

Required fix:

- Do not make `submitDataProof` internally inconsistent.
- Submit a digest that is accepted by `submitDataProof`, then call `disputeInvalidProof` with the disputed proof hash.
- If the scenario still wants to record an alternate bad digest for evidence, keep it as evidence-only metadata, but do not use it in a way that prevents the chain from reaching the dispute extrinsic.
- Add a note in evidence such as:

```json
"scenario_outcome": {
  "type": "invalid-proof",
  "child_event": "tradeSession.SessionPunished",
  "main_event": "mainEscrow.EscrowPunished",
  "submitted_digest": "0x...",
  "evidence_bad_digest": "0x...",
  "bad_digest_differs_from_submitted": true
}
```

- Keep the `badDigest !== validDigest` assertion if retaining `badDigest` for evidence.

### Low: Required documentation updates are mostly missing

- Plan reference: `docs/internal/agent-plans/2026-06-27-stage11-data-trade-failure-dispute-scenarios.md:471`
- Implemented docs update:
  - `docs/implementation/data-trade-implementation.md`
- Missing required docs/records:
  - `docs/implementation/data-trade-evidence.md`
  - `docs/implementation/data-trade-paper-gap-matrix.md`
  - `docs/architecture/data-trade-security-model.md`
  - `docs/internal/agent-plans/2026-06-26-data-trade-long-term-scripted-e2e-roadmap.md`
  - Stage 11 plan Execution Record

The plan required documenting Stage 11 evidence fields, scenario boundaries, and whether live scenario validation was actually run. The current docs only add one implementation bullet.

Required fix:

- Update the missing docs listed above.
- Explicitly state:
  - Stage 11 scripts failure/dispute paths over existing pallet behavior;
  - no new dispute mechanism, production timeout, on-chain ZK verification, trustless bridge, or verifier quorum;
  - live chain scenario validation status. If not run, say not run due to unavailable RPC/environment.
- Update the Stage 11 plan execution record with implemented commits and validation status.

## Validation Performed

Passed:

```bash
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js
go -C tools/data-trade-zk test ./...
go -C tools/data-trade-zk build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
git diff --check f924364..HEAD
```

Passed:

```bash
rm -rf target/data-trade-zk/session-0-round-0 /tmp/fishbone-stage11-review-dry
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --evidence-out /tmp/fishbone-stage11-review-dry/evidence.json \
  --dry-run-dynamic
test -f /tmp/fishbone-stage11-review-dry/evidence.json
```

Result: dynamic dry-run passed and evidence included `scenario: "happy"` with normalized `rounds[].constraints[]`.

Passed:

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_multi_range.json \
  --scenario invalid-proof-dispute \
  --dry-run-dynamic
```

Result: exited with code `2` and rejected non-happy scenario in dry-run mode, as planned.

Passed:

```bash
node scripts/zk_real_data_trade_flow.js --scenario does-not-exist
```

Result: exited with code `2` and rejected unknown scenario.

## Notes

- `happy` scenario is currently handled by the `default` branch in the scenario switch. Because scenario validation happens earlier, this is functionally acceptable, but an explicit `case "happy"` would be easier to review.
- No live chain scenario E2E was run during this review.
