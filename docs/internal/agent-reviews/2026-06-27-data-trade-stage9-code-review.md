# Stage 9 Dynamic Scripted E2E Code Review

> Review owner: Codex
>
> Implementation owner: CodeWhale
>
> Branch: `feat/data-trade-stage9-dynamic-e2e`
>
> Decision: **changes requested**

## Findings

### Medium: Dynamic live mode validates dataset/request after chain side effects

- File: `scripts/zk_real_data_trade_flow.js:131`
- Relevant code:
  - Chain connections are opened before dynamic witness validation at `scripts/zk_real_data_trade_flow.js:141`.
  - Listing, escrow, funds/deposit locks, session creation, and session acceptance happen at `scripts/zk_real_data_trade_flow.js:170`.
  - `fishbone-zk make-witness` is first executed inside the delivery loop at `scripts/zk_real_data_trade_flow.js:215`.

In dynamic live mode, the script only reads `request.request_hash` before publishing data and opening escrow/session. The real dataset/request consistency checks still happen when `make-witness` runs after chain transactions have already created listing/session state and locked funds/deposit.

This is unsafe for the scripted E2E contract. A bad dynamic request, mismatched dataset/request pair, unsupported request shape, or out-of-range fixture can fail after irreversible dev-chain side effects. Stage 9's purpose is to compose Stage 8 dynamic inputs into the real E2E flow, so dynamic input validation should fail before the script touches either chain.

Required fix:

- Add a dynamic live-mode preflight before `ApiPromise.create(...)` and before any chain transaction.
- The preflight should call the existing Stage 8 CLI path instead of reimplementing validation in JS, for example:

```bash
fishbone-zk make-witness \
  --dataset <dataset> \
  --request <request> \
  --out <temp-or-target-preflight>/witness.json \
  --session-id 0 \
  --round-index 0
```

- Do not reuse the preflight witness for the real session rounds. Real rounds should still generate session-bound witnesses after the actual `sessionId` is known.
- If preflight fails, exit before connecting to the chains.
- Add a reviewable validation note or command showing that an invalid dynamic request fails before chain connection/logged chain transactions.

### Low: Stage 9 security-model documentation was required but not updated

- File: `docs/architecture/data-trade-security-model.md:20`
- Plan reference: `docs/internal/agent-plans/2026-06-27-stage9-data-trade-dynamic-scripted-e2e.md`

The Stage 9 plan explicitly required updating `docs/architecture/data-trade-security-model.md`, including the current guarantee/non-guarantee language for dynamic scripted E2E. The implementation updated evidence, implementation, gap matrix, and roadmap docs, but the security model still ends its current guarantees at Stage 8.

Required fix:

- Add a Stage 9 current-guarantee entry describing that the real data-trade script can compose dynamic `dataset + request -> make-witness -> business-fixture -> verify` into the existing on-chain escrow/session/proof/attestation/settlement flow.
- Preserve the security boundary: this is still range-only, script-only, off-chain verifier plus on-chain attestation; it is not on-chain ZK verification, trustless bridge settlement, subset/substr support, verifier quorum, or frontend support.

## Validation Performed

The following checks passed during review:

```bash
go -C tools/data-trade-zk build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
rm -rf /tmp/fishbone-stage9-review
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --evidence-out /tmp/fishbone-stage9-review/evidence.json \
  --dry-run-dynamic
test -f /tmp/fishbone-stage9-review/evidence.json
```

Result: dynamic dry-run generated witness, artifact, proof digest, and evidence successfully without chain RPC.

```bash
node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --dry-run-dynamic
```

Result: exited with code `2` and rejected partial dynamic inputs.

```bash
node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_range.json \
  --business-witness scripts/fixtures/data_trade_business_sample.json \
  --dry-run-dynamic
```

Result: exited with code `2` and rejected ambiguous dynamic plus explicit witness inputs.

```bash
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/zk_attestation.js
go -C tools/data-trade-zk test ./...
```

Result: passed.

```bash
ZK_VERIFIER_CMD=target/tools/fishbone-zk node scripts/zk_real_data_trade_flow.js \
  --dataset scripts/fixtures/data_trade_datasets/factory_sensors.json \
  --request scripts/fixtures/data_trade_requests/factory_temperature_out_of_range.json \
  --dry-run-dynamic \
  --evidence-out /tmp/fishbone-stage9-bad/evidence.json
```

Result: failed before chain RPC in dry-run mode, as expected. This validates the Stage 8 rejection path, but it does not cover the live-mode preflight ordering issue above.

## Notes

- The dynamic dry-run path is useful and should remain.
- Evidence output structure is adequate for Stage 9 once the live-mode preflight ordering is fixed.
- No live chain run was performed during this review.
