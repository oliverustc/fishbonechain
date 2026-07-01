# SciDataHub Audit: Tests, Caliper Workloads & Reproducibility

## 1. Scope Inspected

| Area | Files |
|---|---|
| Chaincode unit tests (Go) | `chaincode_test.go`, `user_test.go`, `dataset_test.go`, `order_test.go`, `utils_test.go` + `mocks/` (3 files) |
| Backend tests (Jest) | 14 files across `test/database/` and `test/utils/`, `test/services/` |
| Backend manual scripts | `test/services/chaincode.test.mjs`, `test/services/ipfs.test.mjs`, `test/utils/utils.test.mjs` |
| Caliper benchmark YAMLs | `user-management-benchmark.yaml`, `dataset-management-benchmark.yaml`, `order-management-benchmark.yaml`, `comprehensive-benchmark.yaml`, `stress-test-benchmark.yaml`, `myAssetBenchmark.yaml` |
| Caliper workload JS | `user-management-test.js`, `dataset-management-test.js`, `order-management-test.js`, `comprehensive-test.js` |
| Caliper legacy workloads | `test.js`, `init.js`, `query.js`, `test2.js`, `createuser.js` |
| Caliper network config | `networks/networkConfig.yaml` |
| Shell scripts | `deploy-chaincode.sh`, `run-tests.sh`, `run-performance-test.sh` |
| Report | `caliper/report.html` |
| Chaincode production code | `chaincode.go`, `user.go`, `dataset.go`, `order.go`, `utils.go` |

---

## 2. Confirmed Facts with Evidence

### 2a. Chaincode Tests Are Mock-Only Unit Tests

All 5 Go test files test against `counterfeiter`-generated mocks (`mocks/ChaincodeStub`, `mocks/TransactionContext`, `mocks/StateQueryIterator`). No real Fabric network, no Docker containers, no chaincode lifecycle simulation.

- `chaincode_test.go:31-42` — tests `InitLedger` success + one PutState failure path. Does not validate all 25 seeded datasets are written.
- `user_test.go:14-89` — `GetUser`, `GetTokenBalance`, `SetTokenBalance` all tested with mock state.
- `dataset_test.go:15-136` — CRUD + iterator pattern tested.
- `order_test.go:14-231` — `CreateOrder`, `GetOrder`, `UpdateOrderStatus`, `GetAllOrders` tested. Each covers happy path, not-found, and state-write failure.
- `utils_test.go:9-30` — `RandStr`, `GenerateHashChain`, `ComputeSha256Times` tested with real crypto (package `chaincode`, not `chaincode_test`, so no mocks).

**Quality**: Decent error-path coverage for unit tests. Good starting point for FishboneChain to learn the entity API surface.

### 2b. CompleteOrder Chaincode Test Is Broken

`order_test.go:134-184` — The `TestCompleteOrder` function sets up mocks for order completion, buyer/seller tokens, and transfer, but **the actual calls to `smartContract.CompleteOrder()` are commented out** on lines 167, 176, 182. The test asserts on uninitialized `err` values (nil from `require.NoError` on line 22, reused). This test passes vacuously — it would fail if uncommented because the mock `GetStateReturnsOnCall` setup is incomplete.

```go
// order_test.go:160 - smartContract never instantiated
// order_test.go:167 - actual call commented out
// err = smartContract.CompleteOrder(transactionContext, "testorder")
require.NoError(t, err) // err is still nil from line 22's json.Marshal
```

### 2c. Only One Caliper Benchmark Round Was Actually Run

`caliper/report.html:154-156` — The report shows exactly one round executed:
- **Name**: "代币余额设置性能测试" (SetTokenBalance)
- **Results**: 1812 succ / 188 fail / 100.2 TPS send rate / 0.51s avg latency / 99.8 TPS throughput
- **No other rounds** appear in the report summary or detail sections

`user-management-benchmark.yaml:8-90` — 5 of 6 rounds are **commented out with YAML `#` prefix**; only the `setTokenBalance` round (lines 50-62) is active.

### 2d. ~9.4% Failure Rate in the Actual Run

`report.html:155` — 188 failures out of 2000 transactions (9.4%). The report provides no breakdown of failure reasons. Likely causes: pre-created users (`preCreateUsers: 100`) being consumed by multiple workers leading to collisions on `SetTokenBalance`, or the `AddUser` in `initializeWorkloadModule` silently failing for duplicate usernames since it doesn't check for existing users in the chaincode (`user.go:18-29` — `AddUser` does unconditional `PutState`, which should overwrite).

### 2e. Chaincode Name Mismatch

- `caliper/deploy-chaincode.sh:23` — deploys chaincode as **`datatrading`**
- All Caliper benchmark YAMLs and workloads reference `contractId: scidatahub`
- `caliper/networks/networkConfig.yaml:12` — `id: scidatahub`
- `backend/src/config/blockchain_config.mjs:41` — default `CHAINCODE_NAME: 'scidatahub'`

This means the Caliper configs were written against the backend's default chaincode name, but the deployment script uses a different name. You'd need to edit one side.

### 2f. Hardcoded Keystore Path Breaks Reproducibility

`caliper/networks/networkConfig.yaml:21`:
```yaml
clientPrivateKey:
  path: '../blockchain/test-network/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/keystore/412c185f72ad4518684dad17fbcc90eec6448dd7b3501380e4e902a44104b313_sk'
```
The keystore filename (`_sk`) is unique per `cryptogen`/CA invocation. Any fresh `./network.sh up` generates a different filename. This config is **single-use** for the specific deployment that generated that key. The `run-tests.sh` script has no logic to discover the current key filename.

### 2g. init-contract-benchmark.yaml Does Not Exist

`caliper/run-tests.sh:121` references `init-contract-benchmark.yaml` but **no such file exists** in the repository. The `./run-tests.sh init` path always fails.

### 2h. Backend Tests Are Standard CRUD Route Tests

The 10 `*Service.test.mjs` and `*Table.test.mjs` files test Express route handlers + SQLite table operations using `jest.unstable_mockModule` to mock the database layer. Patterns:
- Happy path: mock DB returns data → route returns 200
- Not-found: mock DB returns null → route returns 404
- Error: mock DB throws → route returns 500
- Validation: missing/invalid fields → route returns 400

No blockchain interaction is tested. No integration with Fabric chaincode. Tested against an in-memory Express app with mocked DB, not a real SQLite file.

### 2i. Backend `chaincode.test.mjs` Is a Manual Script, Not a Test

`backend/test/services/chaincode.test.mjs` is invoked via `npm run chaincode` with a CLI argument (`init`, `user`, `dataset`, `order`). It connects to a **real Fabric network** (via `chaincode.mjs` → `fabric-gateway`) and calls chaincode functions sequentially. It is not part of the Jest test suite. It has no assertions — it just calls functions and logs output.

### 2j. Legacy Caliper Workloads Target Deprecated Chaincode API

| Legacy file | Functions called | Exist in current chaincode? |
|---|---|---|
| `test.js` | `InitLedger`, `CreateUser`, `Mint`, `CreateDataset` (8 args) | **No** — current API is: `AddUser`, `AddDataset` (2 args), no `Mint`, no `CreateUser` |
| `init.js` | Same as test.js + `CreateOrder` (3 args), `HandleOrder` (3 args) | **No** — current `CreateOrder` takes 6 args; no `HandleOrder` exists |
| `query.js` | `GetDataset`, `GetOrder` by numeric ID (`dataset1`, `order1`) | **No** — current IDs are hash-based, not numeric |
| `test2.js` | Same pattern, with retry logic | **No** |
| `createuser.js` | `CreateUser`, `Mint` | **No** |

These workloads target a **different chaincode version** that used numeric IDs and different function signatures. They are dead code.

### 2k. Monitor Configuration in Benchmarks

All 6 benchmark YAMLs configure Docker resource monitoring + Prometheus push. `comprehensive-benchmark.yaml` and `stress-test-benchmark.yaml` also configure process-level monitoring. The `report.html` confirms Docker metrics were captured (showing CPU/Mem/Disk for peer0, orderer, chaincode containers). This is the most reusable part.

---

## 3. Gaps, Demo Shortcuts, and Correctness Risks

| Risk | Severity | Detail |
|---|---|---|
| **No CI pipeline** | High | Zero CI config files (no `.github/`, no GitLab CI, no Jenkins). Tests never run automatically. |
| **No integration tests** | High | Chaincode ↔ Backend ↔ Fabric integration is untested. Only mock unit tests + one manual script. |
| **Broken CompleteOrder test** | Medium | `order_test.go` has commented-out assertions. The test function is dead code that happens to pass. |
| **Caliper is mostly unused** | Medium | Only 1 of ~30 defined benchmark rounds was actually run. The rest are aspirational config. |
| **Chaincode name mismatch** | Medium | Deploy script uses `datatrading`, Caliper configs use `scidatahub`. |
| **Keystore hardcoding** | High | NetworkConfig hardcodes a one-time key file; any redeploy breaks Caliper. |
| **Missing init benchmark** | Medium | `init-contract-benchmark.yaml` is referenced but doesn't exist. |
| **188 failures uninvestigated** | Medium | 9.4% failure rate in the one real run; no root cause analysis exists. |
| **Legacy workloads are dead code** | Low | 5 files targeting a predecessor chaincode API. Useful only as structural reference. |
| **IPFS integration untested** | Medium | `ipfs.test.mjs` requires a running IPFS daemon; no mock/IPFS-in-process test. |
| **Frontend has zero tests** | High | 57+ Vue components, no `.test.js`, no `vitest`, no `@vue/test-utils` in devDependencies. |
| **No test for hash chain verification correctness** | Medium | `ComputeSha256Times` is tested, but `CompleteOrder`'s integration of hash-chain ↔ token calculation is only tested in the broken test. |
| **No negative test for CompleteOrder preImage verification** | Medium | What happens when a wrong preImage is provided? Not tested anywhere. |
| **Worker 0 initialization pattern is fragile** | Low | `order-management-test.js:25` gating `InitLedger` to worker 0 with a 5-second sleep for other workers is racy. |

---

## 4. Reusable Ideas/Assets for FishboneChain

### 4a. Chaincode Entity Model (Good Reference)
The `User → Dataset → Order` model with hash-chain-locked token transfer is a coherent design pattern for data trading:
- `User`: `tokenBalance` + `lockedTokenBalance` pattern for escrow
- `Dataset`: minimal on-chain footprint (hash + owner)
- `Order`: links buyer/seller via dataset hash, enforces ownership on creation, transfers tokens on completion
- `CompleteOrder`'s `ComputeSha256Times(preImage, hashChainEnd)` approach: pre-image reveals a position in the hash chain, and `tokenAmount = position * tokenUnit`. This is the core "pay-per-access" mechanism worth studying.

### 4b. Caliper Monitor Configuration (Directly Reusable)
All benchmark YAMLs have:
```yaml
monitors:
  resource:
    - module: docker
      options:
        interval: 5
        containers: [all]
  prometheus:
    - module: prometheus-push
      options:
        url: "http://localhost:9091"
        interval: 5
```
The `comprehensive-benchmark.yaml` and `stress-test-benchmark.yaml` additionally add process + network monitoring. Copy-paste these monitor blocks.

### 4c. Workload Structural Patterns (Adapt, Don't Copy)
The 4 modern workload files (`*-management-test.js`, `comprehensive-test.js`) demonstrate:
- `initializeWorkloadModule` for pre-creating test data
- `submitTransaction` dispatch by `testType`/`testScenario` enum
- Weighted random operation selection for mixed workloads
- Multiple rate-control strategies: `fixed-rate`, `linear-rate`, `composite-rate`

FishboneChain can reuse the **pattern** but must replace the chaincode function names and argument shapes.

### 4d. Test Coverage of Error Paths in Go Chaincode
The Go test pattern of testing get-success, get-not-found, get-error, put-failure for each function is a good template. The mock setup boilerplate (`chaincodeStub := &mocks.ChaincodeStub{}; transactionContext := &mocks.TransactionContext{}; transactionContext.GetStubReturns(chaincodeStub)`) is standard Fabric chaincode test convention.

### 4e. What NOT to Reuse
- Legacy workloads (`test.js`, `init.js`, `query.js`, `test2.js`, `createuser.js`) — dead code for a different API
- Hardcoded keystore paths — use glob discovery or env vars
- The `workerIndex === 0` init pattern with `setTimeout(5000)` — use Caliper's `beforeRound` phase or a dedicated init round
- `init-contract-benchmark.yaml` concept — just use `deploy-chaincode.sh` + `peer chaincode invoke` to init

---

## 5. Migration Notes for FishboneChain

### 5a. Test Infrastructure
- FishboneChain should start with **Go unit tests** for each pallet/chaincode function using mock runtime — SciDataHub's Go test pattern is directly portable to Substrate's `#[test]` + `TestExternalities`.
- Add **integration tests** from day one. SciDataHub's biggest gap is the lack of any automated end-to-end test. Use `substrate-contracts-node` for a local test network.
- Do NOT replicate the "manual script as test" pattern — SciDataHub's `chaincode.test.mjs` has zero assertions.

### 5b. Caliper/Benchmarking
- Caliper **can** work against Substrate via the Caliper Substrate adapter (`@hyperledger/caliper-substrate`). The workload patterns are transferable.
- For FishboneChain, define 3 benchmark configs (not 6 aspirational ones):
  1. **Baseline**: single-type operations at low TPS (validate correctness under load)
  2. **Throughput curve**: linear-rate ramp from 10→100 TPS (find saturation point)
  3. **Soak test**: fixed-rate at 50% of saturation for 10 minutes (stability)
- Replace hardcoded keystore with environment variable or auto-discovery script.
- Audit failure reasons before reporting numbers — SciDataHub's 9.4% failure rate makes the 99.8 TPS throughput number misleading.

### 5c. Chaincode Name & Identity Management
- Use a single source of truth for chaincode/contract names (env vars, a config file, or a constants module).
- Use environment-variable-based identity paths, not hardcoded files.

### 5d. Data Model Ideas Worth Carrying Over
- `lockedTokenBalance` as an escrow mechanism is directly applicable to Substrate's `reserved` balance pattern.
- Hash-chain-based pay-per-access: the `preImage → hashChainEnd` verification via `ComputeSha256Times` is a clean on-chain commitment scheme. In Substrate, this could be implemented in a pallet with the `sp_io::hashing::sha2_256` host function.

### 5e. Frontend Testing
- SciDataHub has zero frontend tests. FishboneChain should use `vitest` + `@vue/test-utils` (or equivalent for its chosen framework) from the start.

---

## 6. Follow-up Files & Questions

### Files a Human Maintainer Should Verify
| File | Why |
|---|---|
| `caliper/networks/networkConfig.yaml:21` | Confirm the keystore path is stale. Generate a fresh network and check if `_sk` filename matches. |
| `caliper/deploy-chaincode.sh:23` vs `caliper/benchmarks/*.yaml` contractId | Confirm the `datatrading` vs `scidatahub` mismatch is intentional. Which name should be canonical? |
| `blockchain/sci-data-trade/chaincode-go/chaincode/order_test.go:160-182` | Verify `TestCompleteOrder` is broken. Try uncommenting the calls and running `go test ./chaincode/...` |
| `caliper/report.html` | If a `report.html` exists from a fuller run elsewhere, was it generated against the current codebase? |

### Open Questions
1. Was the `report.html` generated from the code in this commit, or from an older version where more rounds were uncommented?
2. Are there any Caliper result archives (CSV, JSON) from other runs not committed to the repo?
3. What Fabric version was the test network running when `report.html` was generated? (container ID hints suggest Fabric 2.x with `scidatahub_1.0` chaincode label)
4. Were the 188 failures in `SetTokenBalance` investigated? Was it a user-not-found issue (workers racing on pre-creation)?
5. Does `init-contract-benchmark.yaml` exist in another branch or was it deleted? The README and run-tests.sh both reference it.

---
*Audit generated from commit d75aebcddd87 (branch: dev)*
