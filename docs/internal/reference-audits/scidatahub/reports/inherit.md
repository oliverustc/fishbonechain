Now I have comprehensive evidence from the SciDataHub codebase. Let me write the inheritance matrix report.
The inheritance matrix has been written to `docs/internal/reference-audits/scidatahub/reports/inherit.md`. Here's the summary:

**27 candidate items** evaluated across 4 categories:

- **INHERIT (algorithm/logic, rewrite in Rust/Substrate)**: 15 items — hash-chain settlement, domain model (User/Dataset/Order), token escrow, order state machine, IPFS upload pattern, Caliper workload designs, frontend route UX, dual exchange concept, multi-domain partitioning

- **DISCARD entirely**: 11 items — Fabric gateway, SQLite metadata, localStorage auth, Material Kit template, demo seed data, `asset-transfer-basic/` samples, `test-network/` scaffolding, genezio.yaml, hardcoded IP config, single-channel deployment

- **INHERIT pattern + DISCARD implementation**: 1 item — chaincode bridge abstraction (pattern is generic; gRPC/Fabric specifics must be discarded)

**Critical finding**: The `bcCompleteOrder` bridge sends 1 argument but the chaincode expects 2 (`orderID`, `preImage`) — `orderService.mjs:78` vs `order.go:103`. This means the hash-chain settlement path has never been tested end-to-end. The algorithm itself is sound, but this bug must be addressed before trusting the logic for Substrate migration.
t.add()`, MFS copy for timestamped folders; **download is an acknowledged TODO** (`ipfs.mjs:92`)

### 2.2 Domain Model

Three core entities, minimal on-chain, richer off-chain:

**On-chain (Go structs):**
```go
// user.go:12-15
User   { Username, TokenBalance, LockedTokenBalance }

// dataset.go:12-15
Dataset { Hash (SHA256), Owner }

// order.go:14-22
Order  { ID, DatasetHash, HashChainEnd, TokenUnit, Buyer, Seller, Status, Timestamp }
```

**Off-chain (SQLite):** 15+ extra columns — `isPublic`, `canMaskingShare`, `canCustomMaskingTrade`, `canDataService`, `maskingDatasetIPFSAddress`, `processingState`, `dataDescription` — with **no sync to chaincode state** (gap confirmed at `repository-inventory.md:45-46`).

### 2.3 Hash-Chain Settlement Protocol (Core Innovation)

`blockchain/sci-data-trade/chaincode-go/chaincode/utils.go:23-48` + `order.go:103-137`:

1. `GenerateHashChain(N)` — iterates SHA-256 N times from a 64-char random secret, creates chain `[h1,...,hN]`
2. Seller publishes `HashChainEnd = hN` (tail) when creating order
3. Buyer calls `TransferLockedTokenBalance` to escrow tokens (`user.go:98-109`)
4. Seller reveals pre-image (original secret) to buyer
5. `CompleteOrder` calls `ComputeSha256Times(preImage, hashChainEnd)` — iterates SHA-256 from preImage, counts steps until match (max 200 iterations at `utils.go:38`)
6. Tokens transferred: `steps * TokenUnit` from buyer locked → seller balance (`order.go:118-137`)

**Bug noted**: `CompleteOrder` in `orderService.mjs:78` calls `submitTransaction('CompleteOrder', orderID)` with only **one argument**, but the Go chaincode expects `(orderID, preImage)` — two arguments. This means the backend bridge **cannot successfully settle orders** as wired.

### 2.4 Dual Exchange Modes

| Mode | Chaincode Support | SQLite Tables | Frontend Views |
|------|-------------------|---------------|----------------|
| **Trade Orders** (脱敏数据交易) | Full: CreateOrder/GetOrder/CompleteOrder | `trade_orders_{name}` | `RequestDataTrade.vue`, `TradeOrders.vue`, `TradeOrderDetail.vue`, `TradeOrderProcessingDetail.vue` |
| **Service Orders** (数据服务) | None — no chaincode functions | `service_orders_{name}` | `RequestDataService.vue`, `ServiceOrders.vue`, `ServiceOrderDetail.vue`, `ServiceOrderProcessingDetail.vue` |

Service orders exist only in SQLite + frontend — they have routes (e.g., `POST /:blockchainName/service-orders`) and full CRUD in `serviceOrdersService.mjs`, but **no on-chain settlement** counterpart.

### 2.5 Demo-only Operations

`backend/src/app.mjs:32-69`: On every restart, all tables dropped/recreated, demo users + blockchains seeded. Chinese comment confirms: "正式版删除后续代码" (delete this block for production).

### 2.6 Benchmark Suite

`caliper/` contains 5 YAML benchmark configs + 9 JS workload modules. The `comprehensive-benchmark.yaml:7-132` defines 7 rounds: init, light (20 TPS), medium (40 TPS), heavy (30→80 TPS linear), query-intensive (100 TPS), write-intensive (25 TPS), peak load (120 TPS). Workload modules use `WorkloadModuleBase` with pre-created test users/datasets/orders. One file (`myAssetBenchmark.yaml`) is a leftover Fabric sample, not SciDataHub-specific. `init-contract-benchmark.yaml` is referenced in README but doesn't exist on disk.

## 3. Gaps, Demo Shortcuts, or Correctness Risks

| Issue | Severity | Location |
|-------|----------|----------|
| `bcCompleteOrder` sends 1 arg, chaincode expects 2 (orderID, preImage) | **Critical** | `orderService.mjs:78` vs `order.go:103` |
| No authenticated session — `localStorage` boolean gates all routes | **High** | `auth.js:5`, `router/index.js:170-180` |
| CORS `*` allows any origin | **Medium** | `server_config.mjs:5` |
| `downloadFromIPFS` is unimplemented (Chinese comment: "还不会如何下载") | **Medium** | `ipfs.mjs:92` |
| `CompleteOrder` chaincode unit tests all commented out | **Medium** | `order_test.go:167,176,182` |
| No sync between SQLite metadata (15 fields) and chaincode state (2 fields) | **Medium** | `dataset.go:12-15` vs `datasetsTable.mjs` |
| `init-contract-benchmark.yaml` referenced but missing | **Low** | `caliper/README.md:86` |
| `genezio.yaml` — abandoned deployment config | **Low** | `frontend/genezio.yaml` |
| 25 hardcoded SHA-256 hex strings as demo dataset "hashes" — not real IPFS CIDs | **Low** | `datasetsDemo.mjs:5-31`, `chaincode.go:35-61` |

## 4. Inheritance Matrix

| # | Candidate Item | Source Files | Maturity | Why It Matters | Decision | FishboneChain Target | Migration Risk |
|---|---------------|-------------|----------|----------------|----------|---------------------|----------------|
| **A. Core Protocol & Domain** |
| 1 | Hash-chain settlement protocol | `chaincode/order.go:103-137`, `utils.go:23-48` | **Partial** — algorithm implemented; `CompleteOrder` test commented out; backend bridge has argument mismatch bug | Zero-trust proportional payment: buyer pays per verified hash step. Directly applicable to Substrate data-trade pallet. | **INHERIT** (algorithm), **REWRITE** (in Rust/ink!) | `pallet_data_trade` | Low algorithm risk. O(n) hash loop needs Substrate weight benchmarking. MAX=200 iterations is tunable. |
| 2 | `ComputeSha256Times` verification function | `utils.go:34-48` | **Working** — logic correct, 200-iteration cap | On-chain hash verification gate for payment settlement. | **INHERIT** (logic), **REWRITE** (Rust) | `pallet_data_trade::verify_hash_chain` | Low. SHA-256 available in Substrate's `sp_core::hashing`. |
| 3 | `GenerateHashChain` | `utils.go:23-32` | **Working** — 64-char random seed, SHA-256 iteration | Off-chain hash chain generation by data seller. Should run client-side or in off-chain worker. | **INHERIT** (logic), **REWRITE** (client-side JS/off-chain worker) | `scripts/generate_hash_chain` or `offchain_worker::generate` | Low. Pure computation, no state dependency. |
| 4 | Domain model: User/Dataset/Order structs | `user.go:12-15`, `dataset.go:12-15`, `order.go:14-22` | **Working** — clean, minimal on-chain footprint | Archetypal data marketplace entities. Order ties buyer/seller/dataset with hash-chain settlement. | **INHERIT** (schema), **REWRITE** (Substrate storage items) | `pallet_data_trade::StorageMap` items | Low. Well-defined fields, clear ownership semantics. |
| 5 | Token locking (escrow) pattern | `user.go:98-145` | **Working** — `TransferLockedTokenBalance` + `transferTokens` | Escrow before settlement prevents double-spend. Matches Substrate's `reserve`/`unreserve` family. | **INHERIT** (pattern), **REWRITE** (Substrate `Currency` trait) | `pallet_balances` or custom token pallet | Low. Standard escrow pattern. Substrate has native `reserve` support. |
| 6 | Order state machine | `order.go:24-136` | **Working** — `pending → completed/cancelled` with verification gate | Well-defined transitions: pending → CompleteOrder verifies hash-chain → completed. | **INHERIT** (states + transitions), **REWRITE** (Rust enum + dispatchable) | `pallet_data_trade::OrderStatus` enum | Low. Simple FSM. |
| 7 | Dataset ownership registration | `dataset.go:36-60` | **Working** — simple `{Hash, Owner}` put/get | On-chain proof of data ownership before listing. Prevents fraudulent listings. | **INHERIT** (concept), **REWRITE** (as pallet extrinsic) | `pallet_data_trade::register_dataset` | Low. |
| **B. Backend / API** |
| 8 | Chaincode bridge (`chaincode.mjs`) | `backend/src/services/chaincode/chaincode.mjs:1-99` | **Working** — Fabric Gateway gRPC connection with TLS identity signing | Entirely Fabric-specific. Pattern of gateway→network→contract abstraction is generic. | **DISCARD** (Fabric-specific), **INHERIT** (abstraction pattern) | `substrate_rpc.rs` or `polkadot.js` API | Medium. Substrate uses WebSocket RPC + signed extrinsics, not gRPC. Pattern of `initializeContract → submitTransaction` maps to `api.tx.pallet.function().signAndSend()`. |
| 9 | REST route separation (`*Routes.mjs` / `*Service.mjs` / `*Table.mjs`) | `backend/src/database/**/*Routes.mjs`, `*Service.mjs`, `*Table.mjs` | **Working** — clean 3-layer CRUD pattern | Standard express.js pattern. Should be replaced with Substrate RPC. | **DISCARD** | N/A (Substrate uses extrinsics + events) | None. This is generic CRUD. |
| 10 | SQLite per-blockchain table partitioning | `app.mjs:58-67`, `datasetsTable.mjs` | **Working** — dynamically creates `datasets_Physics`, etc. | Multi-domain concept for scientific fields. | **DISCARD** (SQLite), **INHERIT** (multi-domain concept) | `pallet_registry` with domain instances or separate parachains | Medium. Substrate can support domain separation via parachains or configurable pallet instances. |
| 11 | IPFS file upload with MFS folders | `ipfs.mjs:8-84` | **Partial** — upload + MFS cp work; download is TODO | Timestamped IPFS folder organization is clean. Kubo RPC client pattern is reusable. | **INHERIT** (upload pattern), **REWRITE** (add download) | Off-chain worker or client-side `ipfs-client` | Low for upload. Download gap must be closed. |
| 12 | Demo database reset on every startup | `app.mjs:32-69` | **Purposely volatile** — drops/recreates all tables | Explicitly marked for deletion in production. | **DISCARD** | N/A | None. |
| **C. Frontend** |
| 13 | Vue Router marketplace structure | `frontend/src/router/index.js:27-167` | **Working** — 20+ routes with `requiresAuth` meta | Good UX pattern for data marketplace: domain list → public datasets → dataset detail → trade/service request → order management → processing detail. | **INHERIT** (UX pattern/route tree) | Frontend routing of `fishbone-app` | Low. Conceptual pattern only; no code to migrate. |
| 14 | Dual order type UI (Trade + Service) | `views/dataset/dataTrade/`, `views/dataset/dataService/`, `views/orders/` | **Partial** — trade flow detailed (768-line `RequestDataTrade.vue`); service flow less developed | Two distinct exchange experiences: hash-chain data trade vs. custom data service request. | **INHERIT** (two-flow concept) | Frontend UX design | Low. Conceptual reference only. |
| 15 | Pinia auth store (localStorage-based) | `stores/auth.js:1-28` | **Working** but insecure — plain boolean in localStorage | Simplest possible auth — no JWT, no session token, no keypair signing. | **DISCARD** | Replace with Substrate keypair via Polkadot.js extension | None. |
| 16 | Axios instance with hardcoded IP | `api/axios.js:1-36` | **Working** — `http://192.168.8.22:3101` | Demo shortcut; no environment config. | **DISCARD** | Replace with Polkadot.js `ApiPromise` + env-configured backend | None. |
| 17 | Material Kit 2 Pro template | `src/assets/scss/`, `src/components/Material*.vue` | **Commercial template** — 300+ files of vendor CSS/JS | Not SciDataHub original work. | **DISCARD** | FishboneChain UI framework | None. |
| 18 | Market view dataset listing | `views/Market/MarketView.vue:49-80` | **Partial** — hardcoded placeholder item mixes with API data, uses `/getDatasets` (undocumented non-chaincode endpoint) | Shows basic market browse pattern. API call uses endpoint not matching the documented chaincode routes. | **DISCARD** (implementation), **INHERIT** (browse UX concept) | Frontend market browse | None. Implementation is confused. |
| 19 | `crypto-js` + `elliptic` dependencies | `frontend/package.json:55-87` | **Declared but usage unclear** — imported in package.json but no obvious use in the referenced views | Potentially for client-side hash-chain generation or ECDSA signing. | **INHERIT** (concept of client-side crypto) | Client-side hash-chain generation | Low. But actual usage must be verified before migrating. |
| **D. Benchmarks & Testing** |
| 20 | Caliper benchmark YAML configs (5 files) | `caliper/benchmarks/*.yaml` | **Working** — 7-round comprehensive config with fixed/linear/composite rate control | Rich performance test design: light→medium→heavy→query→write→peak. Rate control patterns (fixed, linear-ramp, composite-phase) are directly reusable. | **INHERIT** (test design + rate control patterns) | Substrate transaction throughput benchmarks | Medium. Caliper is Fabric-specific, but the YAML structure and rate control patterns map to custom Substrate benchmark harnesses. |
| 21 | Caliper workload modules (9 JS files) | `caliper/workload/*.js` | **Working** — `WorkloadModuleBase` subclasses with pre-created state, weighted operation selection | Well-structured transaction workload generation. `comprehensive-test.js` has 557 lines of realistic multi-scenario mixed workloads. | **INHERIT** (workload patterns), **REWRITE** (for Substrate) | Custom `substrate-bench` workload scripts | Medium. Algorithm generation is language-agnostic; SUT adapter is Fabric-specific. |
| 22 | Go chaincode unit tests with counterfeiter mocks | `chaincode/*_test.go`, `chaincode/mocks/` | **Working** — 5 test files, all entity operations tested except `CompleteOrder` (commented out) | Shows solid Fabric chaincode testing pattern. Counterfeiter mocks for `ChaincodeStub`, `TransactionContext` are well-structured. | **INHERIT** (test organization pattern) | `#[cfg(test)]` mods with mock runtime | Low. Testing philosophy translates; tooling differs. |
| 23 | Backend Jest tests (14 files) | `backend/test/**/*.test.mjs` | **Working** — uses supertest + mock `db.mjs` | Covers REST endpoints with mocked DB. Standard pattern. | **DISCARD** (Fabric-specific routes), **INHERIT** (test structure) | Integration tests with Substrate test-runtime | Low. |
| **E. Chaincode / Smart Contract** |
| 24 | `InitLedger` with 25 hardcoded dataset hashes | `chaincode.go:15-90` | **Demo-only** — seeds `demoDataRequester`/`demoDataOwner` with 100 tokens and 25 fixed hex strings | Scaffolding for development. | **DISCARD** | Genesis config or migration pallet | None. |
| 25 | Single-channel / single-chaincode architecture | `blockchain_config.mjs:40-42` | **Working** — `mychannel` / `scidatahub` | Fabric-specific deployment model. | **DISCARD** | Substrate runtime (parachain or solo chain) | High. Entirely different blockchain paradigm. |
| 26 | `blockchain/asset-transfer-basic/` (~200 files) | `blockchain/asset-transfer-basic/**` | **Fabric sample** — verbatim from `fabric-samples` | Zero SciDataHub originality. | **DISCARD** | N/A | None. |
| 27 | `blockchain/test-network/` (~90 files) | `blockchain/test-network/**` | **Fabric scaffolding** — standard test network scripts, compose files, MSP configs | Standard Fabric deployment tooling. | **DISCARD** | N/A | None. |

## 5. Migration Notes for FishboneChain

### 5.1 What to Extract (INHERIT)

| Priority | Item | Action |
|----------|------|--------|
| **P0** | Hash-chain settlement algorithm (`ComputeSha256Times`) | Port to Rust as `pallet_data_trade::verify_hash_chain()`. Benchmark weight with Substrate's frame-benchmarking. |
| **P0** | User/Dataset/Order domain model | Map to Substrate `StorageMap` items with bounded vectors. |
| **P0** | Token escrow pattern (`TransferLockedTokenBalance` → `transferTokens`) | Use Substrate's `frame_support::traits::Currency::reserve` / `unreserve` / `transfer`. |
| **P1** | Order state machine (`pending → completed/cancelled`) | Implement as Rust enum + dispatchable validation. |
| **P1** | Dual exchange mode concept (trade + service) | Design two pallets or two workflows within one pallet. |
| **P1** | Caliper workload patterns and rate control designs | Adapt for Substrate throughput benchmarks (custom scripts with Polkadot.js `tx.batch`). |
| **P2** | IPFS MFS folder organization pattern | Implement in off-chain worker: upload → pin → post CID on-chain. |
| **P2** | Frontend Vue route tree for marketplace UX | Reference for FishboneChain dApp navigation design. |
| **P2** | Multi-domain partitioning concept | Evaluate parachain-per-domain vs. configurable pallet instances. |

### 5.2 What to Discard Entirely

| Category | Reason |
|----------|--------|
| All Fabric-specific code (`chaincode.mjs`, `blockchain/test-network/`, `asset-transfer-basic/`) | Substrate replaces Fabric entirely. |
| SQLite metadata layer (`database/`) | Substrate uses on-chain storage. Sync gap in SciDataHub means this layer has no migration value. |
| localStorage auth (`stores/auth.js`) | Substrate uses keypair signing. |
| Material Kit 2 Pro template (~300 files) | Commercial vendor CSS; not original work. |
| Demo seed data (`datasetsDemo.mjs`, `usersDemo.mjs`, `InitLedger` data) | Hardcoded hex strings, not real datasets. |
| `genezio.yaml` | Abandoned deployment experiment. |

### 5.3 Risk Items for Migration

| Risk | Severity | Mitigation |
|------|----------|------------|
| `CompleteOrder` bridge has argument-count mismatch → hash-chain settlement path is **untested end-to-end** | **High** | Verify algorithm with new unit tests in Rust before trusting the Go logic. |
| On-chain `ComputeSha256Times` loops O(n) — needs Substrate weight benchmarking | **Medium** | Cap at 200 iterations (matching SciDataHub's `MAX=200`). Benchmark worst-case. |
| Service orders have no chaincode counterpart → "dual exchange" concept is partially unimplemented | **Medium** | Design service-order settlement from scratch; don't assume SciDataHub's SQLite stubs are complete. |
| `crypto-js` / `elliptic` in package.json — unknown whether hash-chain generation happens client-side | **Low** | Verify actual usage before deciding client-side vs. off-chain-worker generation. |
| Hardcoded `192.168.8.22` IP throughout — single-machine LAN deployment assumption | **Low** | Ignore; FishboneChain uses its own network topology. |

## 6. Follow-up Files / Questions

### 6.1 Files Worth Inspecting Deeper

| File | Reason |
|------|--------|
| `frontend/src/views/orders/TradeOrderProcessingDetail.vue` | The hash-chain reveal UX — likely contains the pre-image submission form. Critical for understanding user flow of settlement. |
| `caliper/workload/order-management-test.js` | Contains `CompleteOrder` workload — may reveal how pre-image + hashChainEnd are generated in tests, possibly fixing the argument-count bug in production bridge. |
| `backend/src/services/chaincode/userService.mjs` | Check whether `bcCompleteOrder` actually has the 1-arg bug or whether `orderService.mjs` was modified after the read. |
| `frontend/src/views/Order/BuyOrder.vue` and `SellOrder.vue` | Buyer-side (submit pre-image) and seller-side (reveal chain) flow details. |
| `blockchain/sci-data-trade/chaincode-go/chaincode/order_test.go` | Inspect the commented-out `CompleteOrder` tests — may reveal intended test scenarios. |

### 6.2 Questions for Maintainers

1. Has the hash-chain settlement ever been demonstrated end-to-end (seller generates chain → buyer locks tokens → seller reveals pre-image → on-chain settlement completes)? The argument-count bug in `orderService.mjs:78` suggests it was never tested as a full flow.
2. What is the intended role of `crypto-js` and `elliptic` in the frontend? Are they used for client-side hash-chain generation or just loaded speculatively?
3. Are the service orders (数据服务) meant to be settled on-chain eventually, or are they intended as off-chain negotiation-only?
4. Why does the `CompleteOrder` Go test have commented-out test cases? Was the function never verified?
5. Is the IPFS download TODO still relevant, or was download intended to happen client-side via IPFS gateway?

### 6.3 Git History Suggestions

```bash
git -C references/SciDataHub log --oneline -- blockchain/sci-data-trade/chaincode-go/chaincode/order.go
git -C references/SciDataHub log --oneline -- backend/src/services/chaincode/orderService.mjs
git -C references/SciDataHub log --diff-filter=D --oneline -- caliper/benchmarks/
```
