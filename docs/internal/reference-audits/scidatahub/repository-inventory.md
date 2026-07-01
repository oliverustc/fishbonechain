# SciDataHub Repository Inventory

**Audit date**: 2026-07-01  
**Reference repo**: `references/SciDataHub`  
**Branch/commit**: `dev` / `d75aebcddd87`  
**Origin**: `github.com/infolab-bcg/SciDataHub`

---

## 1. Scope Inspected

Full repository scan: top-level `README.md`, all 4 `package.json` files, every `backend/src/` module, `blockchain/sci-data-trade/chaincode-go/` chaincode, `frontend/src/` router/stores/api, `caliper/` benchmarks and workload scripts, `ipfs/` README, and `blockchain/test-network/` and `blockchain/asset-transfer-basic/` directories.

Deterministic context files in `docs/internal/reference-audits/scidatahub/context/` were used as navigational hints and cross-referenced against source files.

---

## 2. Confirmed Facts with Evidence

### 2.1 Architecture Overview

SciDataHub is a **3-tier web application** (Vue.js frontend + Express.js backend + Hyperledger Fabric blockchain) for scientific data management and exchange. A 5-node IPFS cluster provides off-chain storage.

**Technology stack** (confirmed by `package.json` files):
| Layer | Technology | Source |
|-------|-----------|--------|
| Frontend | Vue 3 + Vite + Pinia + Bootstrap 5 + vue-router | `frontend/package.json:55-87` |
| Backend | Express.js (Node) + SQLite3 + multer (file upload) | `backend/package.json:35-44` |
| Blockchain | Hyperledger Fabric 2.x, Go chaincode (contract-api-go v2.2.0) | `blockchain/sci-data-trade/chaincode-go/go.mod:6-8` |
| Fabric Gateway | `@hyperledger/fabric-gateway` v1.5 (JS) | `backend/package.json:19` |
| IPFS | Kubo v0.36.0, `kubo-rpc-client` | `ipfs/Readme.md:6`, `backend/package.json:41` |
| Auth | PBKDF2 password hashing (Node crypto), localStorage-based session | `backend/src/database/users/usersService.mjs:10-14`, `frontend/src/stores/auth.js:5` |
| Benchmarks | Hyperledger Caliper CLI | `caliper/package.json:18-20` |

### 2.2 Domain Model

Three entity types modeled on-chain and mirrored in SQLite:

| Entity | On-chain (Go struct) | Off-chain (SQLite) | Files |
|--------|---------------------|-------------------|-------|
| User | `Username`, `TokenBalance`, `LockedTokenBalance` | `users` table (id, username, password, created_at) | `user.go:12-15`, `usersTable.mjs` |
| Dataset | `Hash` (SHA256), `Owner` | per-blockchain `datasets_{name}` table (name, description, owner, hash, isPublic, canMaskingShare, canCustomMaskingTrade, canDataService, maskingDatasetIPFSAddress) | `dataset.go:12-15`, `datasetsTable.mjs` |
| Order | `ID`, `DatasetHash`, `HashChainEnd`, `TokenUnit`, `Buyer`, `Seller`, `Status`, `Timestamp` | trade_orders_{name} and service_orders_{name} tables (similar schema + requester, processingState, dataDescription) | `order.go:14-22`, `tradeOrdersTable.mjs`, `serviceOrdersTable.mjs` |

**Key insight**: The Go chaincode model is minimal (3 fields for Dataset, 8 for Order). The SQLite schema adds ~15 extra fields (isPublic, canMaskingShare, processingState, etc.) that are **not mirrored on-chain** — the chaincode only stores `{Hash, Owner}`.

### 2.3 Transaction Flow (Hash-Chain Settlement)

The core innovation is a **hash-chain-based micropayment mechanism** (`order.go:103-137`, `utils.go:23-48`):

1. Seller generates a hash-chain of length N via `GenerateHashChain(N)` (`utils.go:23`)
2. Seller uploads data to IPFS and publishes only `hashChain[N-1]` (the tail) with the order
3. Buyer locks tokens via `TransferLockedTokenBalance` (`user.go:98-109`)
4. Seller reveals the pre-image (original secret) to the buyer
5. `CompleteOrder` computes `ComputeSha256Times(preImage, hashChainEnd)` to derive how many hash iterations it took, then transfers `times * TokenUnit` tokens (`order.go:103-137`)

This is a **working zero-trust data delivery protocol** — the buyer pays proportionally to how much of the chain the seller reveals.

### 2.4 What Is Actually Implemented

**Fully implemented and functional**:
- Blockchain chaincode (`sci-data-trade/`): All 4 modules (user, dataset, order, utils) with unit tests via counterfeiter mocks (`*_test.go` files in `chaincode/`)
- Backend REST API: All routes wired in `app.mjs:19-23` — users, blockchains, datasets, orders (trade + service), chaincode bridge (`chaincodeRoutes.mjs:10-29`)
- Backend ↔ Fabric gateway: `chaincode.mjs` connects via `@hyperledger/fabric-gateway`, reads TLS certs from filesystem, supports submit/evaluate with timeouts
- SQLite persistence: `db.mjs:10` — single database file with per-blockchain table partitioning (`datasets_Physics`, `trade_orders_Physics`, etc.)
- Frontend routes: 20+ routes in `router/index.js:27-167` covering login, blockchain list, datasets, orders (trade + service), market, finance
- User auth: Registration with PBKDF2 hashing + login/verify in `usersService.mjs`
- IPFS integration: File upload to IPFS via `kubo-rpc-client`, MFS copy for timestamped folder structure (`ipfs.mjs:76-84`)
- Backend unit tests: 14 test files using Jest + supertest, mocking `db.mjs` functions (`backend/test/`)

**Partially implemented / containing TODOs**:
- `ipfs.mjs:92`: `downloadFromIPFS` function has comment "目前还不会如何下载这个文件，需要继续研究" (still researching how to download)
- `order_test.go:167,176,182`: `CompleteOrder` test cases are commented out — the main settlement function has **no working unit tests**
- `frontend/src/api/axios.js:5`: Hardcoded backend URL `http://192.168.8.22:3101` — no environment variable support
- `blockchain_config.mjs:40-42`: Hardcoded defaults for channel name (`mychannel`) and chaincode name (`scidatahub`)

### 2.5 Development-Only Demo Operations

`backend/src/app.mjs:32-69` contains a **demo re-initialization block** that runs on every backend startup:

```
// 开发环境每次启动后端时，都重新初始化数据库，正式版删除后续代码
```

This drops and recreates all tables, seeds demo users, datasets, and 5 hardcoded blockchains (Physics, Biology, Medicine, AI, CyberSecurity) — `app.mjs:58`. This means:
- All SQLite data is **volatile** across restarts
- Production-grade deployment would need this block removed

### 2.6 Multi-Blockchain Design

The system supports **multiple "blockchains" as logical domains** (e.g., Physics, Biology), each getting its own SQLite tables (`datasets_Physics`, `trade_orders_Physics`, etc.) — `app.mjs:58-67`. However, the Fabric chaincode runs on a **single channel** (`mychannel`) with **one chaincode** (`scidatahub`). The multi-blockchain concept is implemented as a **SQLite-level partitioning**, not actual Fabric multi-channel deployment.

---

## 3. Gaps, Demo Shortcuts, or Correctness Risks

### 3.1 Security Gaps

| Issue | Severity | Location |
|-------|----------|----------|
| No JWT/token auth — session is plain `localStorage` boolean | **High** | `frontend/src/stores/auth.js:5-6` |
| CORS allows all origins (`origin: '*'`) | **Medium** | `backend/src/config/server_config.mjs:5` |
| Hardcoded IP address in frontend axios config | **Low** | `frontend/src/api/axios.js:5` |
| User passwords stored with PBKDF2 (1000 iterations) — adequate but not modern (argon2 recommended) | **Low** | `backend/src/database/users/usersService.mjs:13` |
| Delete/drop endpoints exposed with no admin guard — any user can drop tables | **High** | `blockchainsRoutes.mjs:17` (`DELETE /blockchain/:name`) |
| No rate limiting on any endpoint | **Medium** | All route modules |

### 3.2 Consistency Gaps

| Issue | Location |
|-------|----------|
| On-chain Dataset has 2 fields (`Hash`, `Owner`); SQLite has 15+ fields. No sync mechanism between them. | `dataset.go:12-15` vs `datasetsTable.mjs` |
| Chaincode config defaults point to `scidatahub` but one script uses `datatrading` | `blockchain_config.mjs:41` vs `caliper/deploy-chaincode.sh:34` |
| `frontend/src/api/axios.js:5` hardcodes `192.168.8.22:3101` — differs from `server_config.mjs:3` (port 3101 matches, but IP is hardcoded) | |

### 3.3 NotImplemented / Stub Areas

| Area | Evidence |
|------|----------|
| `downloadFromIPFS` marked as not working | `ipfs.mjs:92` — Chinese comment saying "don't know how to download yet" |
| `CompleteOrder` chaincode tests commented out | `order_test.go:167,176,182` — all TestCompleteOrder cases are `//` |
| Finance page registered in router but likely stubbed | `router/index.js:156-160` — `/finance` route exists but no evidence of backend finance logic |
| `init-contract-benchmark.yaml` referenced in Caliper README but **does not exist** in the `benchmarks/` directory | `caliper/README.md:86` refers to `benchmarks/init-contract-benchmark.yaml` which is missing from `caliper/benchmarks/` |
| Frontend `genezio.yaml` at root — suggests a deployment framework (`genezio.com`) that was evaluated but not used | `frontend/genezio.yaml:1` |

### 3.4 Caliper Test Suite Status

The caliper directory has **5 benchmark YAML files** and **9 workload JS files** — this is a substantial effort. However:
- `caliper/README.md:86` references `init-contract-benchmark.yaml` which doesn't exist
- `caliper/run-tests.sh:121` references `init-contract-benchmark.yaml` also missing
- The `benchmarks/myAssetBenchmark.yaml` file looks like a **leftover Fabric sample** (tests basic `asset-transfer-basic` chaincode, not SciDataHub)

---

## 4. Reusable Ideas/Assets

### 4.1 High-Value for FishboneChain (Direct Inheritance)

| Asset | Value | Explanation |
|-------|-------|-------------|
| **Hash-chain settlement protocol** | **High** | `order.go:103-137` + `utils.go:23-48` — zero-trust data delivery with proportional payment. Directly applicable to Substrate-based data trading. The Go implementation can inform a Rust/ink! reimplementation. |
| **Domain model (User/Dataset/Order)** | **High** | Clean, minimal entity design with clear ownership semantics. Dataset = `{Hash, Owner}`, Order = `{datasetHash, hashChainEnd, tokenUnit, buyer, seller, status}`. Maps well to Substrate storage items. |
| **Order state machine** | **Medium** | `pending → completed/cancelled` with `CompleteOrder` verification gate. Well-defined transitions. |
| **Token locking mechanism** | **Medium** | `TransferLockedTokenBalance` escrows tokens before order completion, preventing double-spend. Maps to Substrate's `reserve` pallet pattern. |
| **Multi-domain partitioning** | **Medium** | The "blockchain-as-domain" concept (per-blockchain SQLite tables) could inform Substrate's multi-instance or parachain-per-domain design. |
| **Caliper workload scripts** | **Medium** | The workload JS files in `caliper/workload/` contain well-structured transaction generation logic for user/order/dataset operations. Can be adapted as benchmarking templates. |

### 4.2 Medium Value (Reference Implementation)

| Asset | Value | Notes |
|-------|-------|-------|
| **Go unit tests with counterfeiter mocks** | **Medium** | `mocks/` directory with generated mock stubs — shows solid testing patterns for Fabric chaincode. |
| **Backend service/table/routes separation** | **Low-Medium** | `usersTable.mjs` / `usersService.mjs` / `usersRoutes.mjs` pattern is clean but standard CRUD. |
| **IPFS integration pattern** | **Low-Medium** | `ipfs.mjs` shows MFS-based folder organization but has download TODO. |
| **Frontend UI concept** | **Low** | Vue Material Kit 2 is a commercial template. The custom views (`DatasetView`, `MyDatasets`, `TradeOrders`, etc.) show UX flow for data trading. |

### 4.3 Do NOT Migrate (Ignore)

| Asset | Reason |
|-------|--------|
| `blockchain/asset-transfer-basic/` (~200 files) | Stock Hyperledger Fabric sample (`fabric-samples`). Zero SciDataHub originality. |
| `blockchain/test-network/` (~90 files) | Stock Fabric test network. Standard Fabric deployment scaffolding. |
| `frontend/src/assets/` (~300 SCSS/CSS/JS/img files) | Vue Material Kit 2 commercial template. 300+ files of Bootstrap CSS/JS, images, fonts. |
| `frontend/src/components/Material*.vue` (~12 files) | Material Kit component wrappers. Template boilerplate. |
| `frontend/src/examples/` (~15 files) | Material Kit demo components. Not used by SciDataHub features. |
| `frontend/src/assets/scss/material-kit/bootstrap/` (~100 files) | Bootstrap 5 source SCSS. Template vendor. |
| `caliper/benchmarks/myAssetBenchmark.yaml` | Tests the stock `asset-transfer-basic` chaincode, not SciDataHub. |
| `caliper/report.html` | One-time generated report artifact. |
| `package.json` (root) | Only depends on `uuid`. Placeholder. |
| `frontend/genezio.yaml` | Unused deployment config for a different platform. |

---

## 5. Migration Notes for FishboneChain

### 5.1 What to Extract

1. **Chaincode → Substrate Pallet/ink! Contract**:
   - `chaincode/user.go` → `pallet_user` or single storage map
   - `chaincode/dataset.go` → `pallet_dataset`
   - `chaincode/order.go` → `pallet_order` (the core trading logic)
   - `chaincode/utils.go` (hash chain) → utility crate or off-chain worker
   - The hash-chain settlement in `CompleteOrder` (`order.go:103-137`) is the most valuable algorithm

2. **Backend → Substrate Off-Chain Worker / API**:
   - `chaincode.mjs` (Fabric gateway) — discard; replace with Substrate RPC/Polkadot.js
   - `ipfs.mjs` (server-side IPFS) — could be adapted to an off-chain worker that pins data and posts CIDs on-chain
   - SQLite layer — discard; Substrate uses on-chain storage with optional off-chain indexers

3. **Domain Model Alignment**:
   - The User/Dataset/Order triple maps naturally to Substrate's `pallet` pattern
   - Dataset registration on-chain needs the owner + hash; the extra SQLite fields (isPublic, canMaskingShare, etc.) could become on-chain metadata struct
   - The dual-order system (trade orders vs service orders) represents two exchange modes — could be unified or kept separate

4. **Frontend Concepts**:
   - The router structure (`/blockchainList`, `/publicDataset/:blockchainName`, `/mydatasets/:name`, etc.) provides a UX pattern for data marketplace navigation
   - Data service request flow (`RequestDataService.vue`, `RequestDataTrade.vue`) shows user interaction design for two exchange modes

### 5.2 What to Redesign

- **Auth**: Replace localStorage-based auth with Substrate's keypair-based identity. No passwords — users sign with their private keys.
- **Multi-blockchain**: Replace SQLite table-per-domain with Substrate's native multi-chain (relay chain + parachains, or single chain with domain pallets)
- **Off-chain storage**: Keep IPFS but move the upload logic to the frontend or Substrate off-chain worker
- **CORS/API security**: Not applicable — Substrate uses WebSocket RPC and signed extrinsics

### 5.3 Risk Items

- **On-chain ↔ Off-chain consistency**: SciDataHub has no sync between SQLite metadata and chaincode state. This is a design flaw. In Substrate, all metadata should live on-chain.
- **`ComputeSha256Times` is O(n) on-chain**: The hash-chain verification loops up to 200 iterations (`utils.go:38: const MAX = 200`). Acceptable for Go chaincode, but needs benchmarking under Substrate's weight system.
- **Hardcoded demo data**: 25 pre-seeded dataset hashes in `chaincode.go:35-61` are hardcoded demo data, not real datasets. Production would need actual IPFS CID registration.

---

## 6. Follow-Up Files / Questions

### 6.1 Files to Inspect Deeper in Subsequent Audits

| File | Reason |
|------|--------|
| `backend/src/database/orders/tradeOrdersService.mjs` | Core trade order logic — check if it mirrors chaincode or has divergent behavior |
| `backend/src/database/orders/serviceOrdersService.mjs` | Service order logic — separate exchange mode, worth understanding |
| `frontend/src/views/orders/TradeOrderProcessingDetail.vue` | Frontend for the hash-chain reveal flow — check UX completeness |
| `frontend/src/views/dataset/dataTrade/RequestDataTrade.vue` | Full trade request UI |
| `caliper/workload/comprehensive-test.js` | Complex test scenarios — good for understanding intended transaction patterns |
| `caliper/workload/init.js` | Contract initialization workload — may clarify pre-seeded state |

### 6.2 Questions for Maintainers

1. **Has this system ever been deployed outside the lab?** The `192.168.8.22` IP appears throughout, suggesting a single-machine LAN deployment.
2. **Is the service-order (数据服务) flow fully implemented, or still conceptual?** It has routes, tables, and frontend views but the chaincode has no service-order counterpart.
3. **What is the intended relationship between SQLite metadata and chaincode state?** They appear to be independent — the SQLite stores "dataset description" while chaincode stores "dataset hash + owner" with no reconciliation.
4. **Why does `caliper/README.md` reference `init-contract-benchmark.yaml` when it doesn't exist in `benchmarks/`?** Possibly deleted or never committed.
5. **Is `genezio.yaml` intentional or a leftover?** It appears to be from an abandoned deployment attempt.

### 6.3 Git History Suggestions

- `git log --oneline references/SciDataHub/blockchain/sci-data-trade/` — to see evolution of the core chaincode
- `git log --oneline references/SciDataHub/backend/src/services/` — to see when Fabric gateway / IPFS services stabilized
- `git log --diff-filter=D --oneline references/SciDataHub/` — to identify deleted files that may have contained useful logic
