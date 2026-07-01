Now let me dig into the key original code areas and the upstream samples to properly classify everything.
Let me check a few more files to complete the classification.
Now I have all the evidence needed. Here is the complete report.

---

# SciDataHub Repository Inventory Report

## 1. Scope inspected

- `/references/SciDataHub` — entire repo tree (708 files), all top-level README/docs, 5 runtime directories (backend, frontend, blockchain, caliper, ipfs)
- `/docs/internal/reference-audits/scidatahub/context/*` — 7 pre-generated deterministic context files
- Git log (`d75aebc` HEAD on `dev`, 23+ commits)

## 2. Confirmed facts with evidence

### 2.1 Repository purpose

A Hyperledger Fabric-based scientific data marketplace platform. Users register, upload datasets to IPFS, trade/request data via on-chain orders with a token + hash-chain payment mechanism. The project name in go.mod is `github.com/infolab-bcg/SciDataHub` (`blockchain/sci-data-trade/chaincode-go/go.mod:1`).

### 2.2 Architecture (3-tier)

| Tier | Technology | Original? | Evidence |
|------|-----------|-----------|----------|
| Chaincode | Go, fabric-contract-api-go/v2 | **YES** | `blockchain/sci-data-trade/chaincode-go/` |
| Backend | Node.js Express + SQLite | **YES** | `backend/src/app.mjs:1-30` |
| Frontend | Vue 3 + Vite + Pinia on Creative Tim kit | **Hybrid** | `frontend/package.json:2` name=`vue-material-kit-2` |
| Storage | IPFS (kubo-rpc-client) | **YES** | `backend/src/services/ipfs.mjs:1-6` |
| Benchmarks | Hyperledger Caliper | **YES (custom workloads)** | `caliper/workload/` |

### 2.3 Domain model (blockchain state)

Three on-chain entity types, stored as JSON in Fabric world state with key prefixes:

- **User**: `{username, tokenBalance, lockedTokenBalance}` — keyed as `user_<name>` (`blockchain/sci-data-trade/chaincode-go/chaincode/user.go:11-15`)
- **Dataset**: `{hash, owner}` — keyed as `dataset_<hash>` (`blockchain/sci-data-trade/chaincode-go/chaincode/dataset.go:11-14`)
- **Order**: `{id, datasetHash, hashChainEnd, tokenUnit, buyer, seller, status, timestamp}` — keyed as `order_<id>` (`blockchain/sci-data-trade/chaincode-go/chaincode/order.go:13-22`)

The off-chain SQLite backend adds richer metadata (fullName, description, isPublic, canMaskingShare, canCustomMaskingTrade, canDataService, maskingDatasetIPFSAddress) in per-blockchain tables (`backend/src/database/datasets/datasetsTable.mjs:4-28`).

### 2.4 Payment mechanism

Hash-chain based: buyer provides `hashChainEnd`, seller reveals `preImage` on completion, `ComputeSha256Times` calculates how many hash iterations match → `tokenAmount = iterations * tokenUnit` (`blockchain/sci-data-trade/chaincode-go/chaincode/utils.go:35-48`, `order.go:102-136`).

### 2.5 Routes (Express)

All mounted at root `/` (no `/api` prefix):
- Users: `POST /register`, `POST /login`, `GET /user/username/:username`
- Blockchains: CRUD at `/getblockchains`, `/blockchain/:name`
- Datasets: per-chain CRUD at `/:blockchainName/addDataset`, `/:blockchainName/datasets`, etc.
- Orders: trade vs service split at `/:blockchainName/trade-orders`, `/:blockchainName/service-orders`
- Chaincode proxy: `/bcAddUser`, `/bcGetUser`, `/bcGetTokenBalance`, etc.

Source: `backend/src/app.mjs:18-23`, all `*Routes.mjs` files, context file `03-routes-api-hints.txt`.

## 3. Module classification

### 3.1 Original SciDataHub logic (HIGH migration value)

| Directory | Files | What it does |
|-----------|-------|-------------|
| `blockchain/sci-data-trade/chaincode-go/chaincode/` | 8 `.go` files | Smart contract: users, datasets, orders, hash-chain utils, mocks, tests |
| `backend/src/` (all except config boilerplate) | ~30 `.mjs` files | Express API, SQLite DAL, chaincode gateway, IPFS, logger |
| `backend/test/` | ~15 `.test.mjs` | Jest unit tests for all database tables and services |
| `frontend/src/views/` | ~24 `.vue` files | All business views (datasets, orders, blockchain, login, market) |
| `frontend/src/router/` | 1 file | 25 routes with auth guards |
| `frontend/src/stores/` | 1 file | Pinia auth store |
| `frontend/src/api/` | 1 file | axios instance |
| `caliper/workload/` | 9 `.js` files | Custom Caliper workloads for SciDataHub chaincode |
| `caliper/benchmarks/` | 7 `.yaml` files | Benchmark config referencing SciDataHub chaincode |
| `blockchain/prepare.md` | 1 file | Original Fabric setup guide |

### 3.2 Upstream Hyperledger sample code (IGNORE for migration)

| Directory | Origin | Evidence |
|-----------|--------|----------|
| `blockchain/asset-transfer-basic/` (entire tree) | Hyperledger fabric-samples | README references, go.mod paths, App.java location, Standard fabric-samples structure with 7 language variants |
| `blockchain/test-network/` (entire tree) | Hyperledger fabric-samples test-network | `network.sh:8` comment: "brings up a Hyperledger Fabric network for testing", copied verbatim |

These two directories contain ~500+ of the 708 files. They are the unmodified `fabric-samples` repository with zero SciDataHub additions.

### 3.3 Third-party template/generated assets (LOW migration value)

| Directory/File | Classification | Notes |
|---------------|---------------|-------|
| `frontend/src/assets/` (entire tree) | Creative Tim "Vue Material Kit 2" commercial UI kit | ~300 CSS/SCSS/img/font files, stock photos, logos |
| `frontend/src/components/` (material-*.vue) | UI kit components | Material Kit wrapper components |
| `frontend/src/examples/` | UI kit demo cards | Not used functionally |
| `frontend/genezio.yaml` | Deployment scaffold | Genezio platform config |
| `frontend/LICENSE` | UI kit license | Not SciDataHub's |

### 3.4 Tests and benchmarks

| Directory | Classification |
|-----------|---------------|
| `backend/test/` | Original Jest unit tests — verify SQL statements, route handlers, chaincode call patterns |
| `blockchain/sci-data-trade/chaincode-go/chaincode/*_test.go` | Original Go unit tests with mock stubs |
| `caliper/` | Original Caliper performance tests — useful reference for Substrate benchmarking patterns |

### 3.5 Deployment scaffolding (LOW migration value)

| Item | Notes |
|------|-------|
| `blockchain/test-network/network.sh`, compose files, CA scripts | Fabric-specific, not reusable |
| `caliper/run-*.sh` | Shell wrappers for Fabric Caliper |
| `ipfs/Readme.md` | IPFS setup guide, no code |
| Root `package.json` | Only `uuid` dependency |

## 4. Gaps, demo shortcuts, or correctness risks

1. **Hardcoded demo data on every restart** (`backend/src/app.mjs:33-69`): The comment admits "开发环境每次启动后端时，都重新初始化数据库，正式版删除后续代码" (dev env resets DB every start; remove for production). Every restart drops/recreates all tables and reloads demo users/datasets.

2. **IPFS download is unimplemented** (`backend/src/services/ipfs.mjs:91-92`): The `downloadFromIPFS` function contains the comment `"目前还不会如何下载这个文件，需要继续研究"` (don't know how to download yet, needs more research).

3. **Single Fabric identity hardcoded**: `blockchain_config.mjs` points to `User1@org1.example.com` with credentials under `Users/Admin`. No multi-org, multi-user signing separation.

4. **No on-chain access control**: Anyone calling the chaincode can add/update any user's balance. The `CompleteOrder` function trusts the caller to provide a valid `preImage` without any escrow or dispute mechanism.

5. **`ComputeSha256Times` has a silent bug at line 40** (`utils.go:40`): The comparison `if hash == tempHash` happens *before* the first hash computation, so `tempHash` is always `""` on the first iteration. This means finding `hash==""` (empty string) would incorrectly return `count=0`. The first hash result is checked at iteration 1, not 0. The compare occurs before hashing in the first iteration.

6. **`CompleteOrder` test cases are commented out** (`chaincode/order_test.go:167,176,182`): Three test cases for CompleteOrder are `// err =` (commented out), suggesting the test was never finished or was broken.

7. **No authentication middleware on chaincode proxy routes**: `/bcAddUser`, `/bcSetTokenBalance` etc. are exposed without auth checks — any HTTP caller can mint tokens or create users on the ledger.

8. **Frontend uses `crypto-js` and `elliptic`** (`frontend/package.json:14-15`): These are client-side crypto libs not wired up to any blockchain identity flow in the backend — likely vestigial or used for local hashing only.

9. **`blockchain/prepare.md` overlaps with `blockchain/Readme.md`**: Both contain Fabric setup instructions. The chaincode name differs: `prepare.md` uses `basic` (the sample), `Readme.md` uses `scidatahub`.

10. **No `.env` or secrets management**: API keys, ports, and Fabric paths are all hardcoded defaults.

## 5. Reusable ideas/assets for FishboneChain

### 5.1 High-value chaincode patterns

- **Hash-chain payment mechanism** (`chaincode/utils.go`): The `GenerateHashChain` / `ComputeSha256Times` approach for off-chain data reveal → on-chain settlement is directly portable to Substrate pallets.
- **Three-entity model (User/Dataset/Order)** with JSON state: Clean, minimal, easy to understand. The `Dataset{hash, owner}` → `Order{buyer, seller, tokenUnit}` flow is language-neutral.
- **Locked token balance pattern** (`user.go:97-109`): Separates tradable balance from escrowed balance — matches Substrate's reserve/lock pattern well.

### 5.2 Backend architecture patterns

- **Per-blockchain SQLite table sharding** (`datasetsTable.mjs`): The `datasets_<blockchainName>` dynamic table naming allows multiple blockchains in one SQLite DB — maps to Substrate parachain/relaychain data separation.
- **Separation of on-chain (hash+owner) vs off-chain (metadata+permissions)** metadata: The chaincode knows only `{hash, owner}`, while SQLite stores `{fullName, description, isPublic, canMaskingShare, ...}`. This is the correct boundary for Substrate too.
- **Service vs Trade order distinction** (`orderRoutes.mjs`): Two separate order types with identical CRUD patterns — a data service marketplace concept.

### 5.3 UI patterns

- **Business view/wizard flow**: Dataset upload → set privacy → publish → order creation → order management → completion. The 25-route structure in `frontend/src/router/index.js` maps a complete data lifecycle.
- **Multi-blockchain navigation**: Router params carry `blockchainName` through all views (`frontend/src/router/index.js:47-69`).

### 5.4 Benchmark patterns

- **Caliper workload modules** (`caliper/workload/`): The pre-create + random-select pattern for benchmarking (create test data, then run operations against it) translates to Substrate Treasury pallet benchmarking.

## 6. Migration notes for FishboneChain

### Can be ignored entirely
- `blockchain/asset-transfer-basic/` — upstream Hyperledger samples, zero custom logic
- `blockchain/test-network/` — Fabric network deployment scaffolding
- `frontend/src/assets/` — commercial UI kit assets (except maybe CSS variables for theming)
- `frontend/src/components/Material*` and `frontend/src/examples/` — UI kit wrappers
- `frontend/genezio.yaml` — deployment platform config
- `blockchain/.gitignore`, root `package-lock.json` — boilerplate

### Deserves deeper migration attention
1. **`blockchain/sci-data-trade/chaincode-go/chaincode/`** — The 5 `.go` source files (chaincode.go, user.go, dataset.go, order.go, utils.go) are the core business logic. Translate to Substrate pallets.
2. **`backend/src/database/`** — Table schemas and query patterns map to Substrate storage items and off-chain worker queries.
3. **`backend/src/services/chaincode/`** — The bridge between Express and Fabric gateway (`chaincode.mjs:61-86`) shows what calls FishboneChain's Substrate node API will need to replace.
4. **`backend/src/services/ipfs.mjs`** — IPFS upload logic is portable as-is via the same `kubo-rpc-client` library.
5. **`backend/test/`** — Test patterns (route tests, table SQL verification tests) inform Substrate off-chain worker testing.
6. **`caliper/workload/` and `caliper/benchmarks/`** — The operation sequences and TPS targets are useful for Substrate benchmarking config though the framework differs.
7. **`frontend/src/views/`** — Business view design and data flow patterns, though all components need porting to a Substrate-compatible framework.

### Key divergence points
- SCI uses Fabric's `PutState`/`GetState` key-value store → FishboneChain uses Substrate's storage maps + events
- SCI has a centralized backend → Substrate uses off-chain workers and client-side signing
- SCI chaincode has no proper auth model → Substrate requires Origin checks and signed extrinsics
- SCI's token is a chaincode integer → FishboneChain needs native token integration via `pallet-balances` or `pallet-assets`

## 7. Follow-up files/questions

### Files needing closer reading for a second-phase audit
- `backend/src/database/orders/tradeOrdersService.mjs` and `serviceOrdersService.mjs` — the full order lifecycle logic
- `backend/src/services/chaincode/userService.mjs`, `datasetService.mjs`, `orderService.mjs` — chaincode invocation patterns
- `frontend/src/views/dataset/dataTrade/RequestDataTrade.vue` and `dataService/RequestDataService.vue` — the data request wizard flows
- `caliper/networks/networkConfig.yaml` — Fabric network topology reference

### Open questions
1. Is the `frontend/src/stores/auth.js` using server-side sessions or just client-side token storage? (affects Substrate auth design)
2. The SQLite per-blockchain naming convention (`datasets_Physics`, `trade_orders_Physics`, etc.) supports 5 hardcoded blockchains — was multi-chain a design requirement or demo flexibility?
3. Are there any deployment records or network configs that show this was actually run against a live Fabric network, or was it a local-only prototype?
4. The 25 demo dataset hashes in `InitLedger` are pre-computed SHA-256 hashes — of what original data? (important for understanding the masking/reveal flow)
