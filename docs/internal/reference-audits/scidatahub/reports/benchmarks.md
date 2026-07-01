Now let me read the actual test implementations and Caliper configurations in detail.
Audit complete. Report written to `docs/internal/reference-audits/scidatahub/audit-tests-caliper-reproducibility.md`.

**Bottom-line findings:**

1. **Only 1 Caliper round was actually run** — `SetTokenBalance` at 100 TPS with 188/2000 failures (9.4%). All other ~29 rounds in the 6 benchmark YAMLs are either commented out or aspirational.

2. **`TestCompleteOrder` in Go is broken** — the actual chaincode calls are commented out (`order_test.go:167,176,182`); the test passes vacuously.

3. **Reproducibility is broken** by hardcoded keystore path in `networkConfig.yaml`, chaincode name mismatch (`datatrading` vs `scidatahub`), and a missing `init-contract-benchmark.yaml` referenced by `run-tests.sh`.

4. **5 legacy Caliper workloads** target a deprecated chaincode API (different function names, different argument shapes) — dead code.

5. **No integration tests exist** anywhere. Backend tests are mock-only CRUD route tests. Frontend has zero tests. The closest thing is a manual `chaincode.test.mjs` script with no assertions.

6. **Reusable for FishboneChain**: the monitor config blocks in benchmark YAMLs, the Go unit test mock patterns, the hash-chain-based token transfer design in `CompleteOrder`, and the weighted-operation workload structure.
