Now I have all the evidence needed. Here is the audit report:

---

# SciDataHub Reference Audit: Scientific Data Circulation Domain Model

## 1. Scope Inspected

- `blockchain/sci-data-trade/chaincode-go/chaincode/` — all 5 Go source files (chaincode.go, user.go, dataset.go, order.go, utils.go) + tests
- `backend/src/` — all 4 database modules (users, blockchains, datasets, orders), all 3 chaincode service modules, IPFS service, Express app, config, utils
- `frontend/src/` — router (index.js), Pinia store (auth.js), API config (axios.js), 20+ Vue views/pages
- `ipfs/Readme.md`, root `README.md`, `caliper/` benchmarks + workloads
- Context files: 00-overview.md, 03-routes-api-hints.txt, 04-blockchain-identity-hints.txt, 05-domain-model-hints.txt

---

## 2. Confirmed Facts with Evidence

### 2.1 Entities

| Entity | Source | Key Fields |
|---|---|---|
| **User** | `chaincode/user.go:11-15` (on-chain); `backend/src/database/users/usersTable.mjs:8-14` (off-chain) | `Username`, `TokenBalance`, `LockedTokenBalance` (on-chain); `username`, `password` (PBKDF2-salted, off-chain) |
| **Dataset** | `chaincode/dataset.go:11-14` (on-chain); `backend/src/database/datasets/datasetsTable.mjs:7-22` (off-chain) | `Hash` (SHA-256), `Owner` (on-chain); `name`, `fullName`, `description`, `owner`, `isPublic`, `canMaskingShare`, `canCustomMaskingTrade`, `canDataService`, `hash`, `maskingDatasetIPFSAddress` (off-chain) |
| **TradeOrder** | `chaincode/order.go:13-22` (on-chain); `backend/src/database/orders/tradeOrdersTable.mjs:8-22` (off-chain) | `ID`, `DatasetHash`, `HashChainEnd`, `TokenUnit`, `Buyer`, `Seller`, `Status`, `Timestamp` (on-chain); `id`, `title`, `description`, `blockchainName`, `datasetName`, `datasetOwner`, `requester`, `maskingRules` (JSON), `status` (off-chain) |
| **ServiceOrder** | `backend/src/database/orders/serviceOrdersTable.mjs:8-23` (off-chain only) | `id`, `title`, `description`, `blockchainName`, `datasetName`, `datasetOwner`, `requester`, `serviceType`, `serviceConfig` (JSON), `status` |
| **Blockchain** | `backend/src/database/blockchains/blockchainsTable.mjs:8-15` | `name`, `fullName`, `description` (5 demo domains: Physics, Biology, Medicine, AI, CyberSecurity) |

### 2.2 Lifecycle States (Order Status)

Evidence: all 6 statuses are confirmed in both backend validators and frontend views.

- **`pending`** → initial state after creation (`tradeOrdersService.mjs` validates `['pending', 'processing', 'completed', 'cancelled', 'failed']`)
- **`processing`** → owner accepts order; triggers cyclic delivery protocol
- **`completed`** → all cycles finished successfully
- **`rejected`** → owner rejects (frontend-only, not in backend validators — see Gap below)
- **`cancelled`** → either party terminates during processing
- **`failed`** → processing error

Evidence: `frontend/src/views/orders/TradeOrders.vue` renders all 6 badges with Chinese labels; `ServiceOrders.vue` identically.

### 2.3 User Roles (Implicit, not modeled)

No formal role enum, database column, or chaincode field. Roles are inferred from:
- Demo usernames: `demoDataOwner`, `demoDataRequester` (`backend/src/database/users/usersDemo.mjs:13-16`)
- Frontend conditional rendering based on `order.datasetOwner === authStore.username` vs `order.requester === authStore.username` (`TradeOrders.vue`, `ServiceOrders.vue`)
- A single user can be both owner and requester — they are *functions*, not identity roles

### 2.4 Dataset Permission Model

Four independent boolean flags (`backend/src/database/datasets/datasetsTable.mjs:12-16`):
- **`isPublic`** — visible in public listings
- **`canMaskingShare`** — static masking download allowed (IPFS link)
- **`canCustomMaskingTrade`** — buyer can initiate custom-masking order
- **`canDataService`** — buyer can request data service (computation, verification)

Evidence: `frontend/src/views/dataset/DatasetView.vue` renders conditional action buttons based on each flag; `DatasetEdit.vue` toggles each independently.

### 2.5 Trade Workflow (Custom Masking Trade)

Confirmed end-to-end from `RequestDataTrade.vue` → `tradeOrdersTable.mjs` → `chaincode/order.go:103`:

1. Requester selects a dataset with `canCustomMaskingTrade=true`, fills masking rules (field name, type, constraint)
2. Off-chain: `POST /:blockchainName/trade-orders` → creates `trade_orders_*` row with `status=pending`
3. On-chain: `POST /bcCreateOrder` → calls `CreateOrder(datasetHash, hashChainEnd, tokenUnit, buyer, seller)` → creates on-chain `Order` with `status=pending`
4. Owner accepts → status moves to `processing`
5. **10-round cyclic delivery** (`TradeOrderProcessingDetail.vue`): per round, owner emits encrypted data subset (64-char hash), requester emits payment proof (64-char hash)
6. On-chain settlement: `CompleteOrder(orderID, preImage)` verifies hash-chain proof, computes `tokenAmount = times * TokenUnit`, transfers tokens via `transferTokens(buyer, seller, amount)` → `LockedTokenBalance` debit, `TokenBalance` credit

### 2.6 Service Workflow (Data Service)

Same pattern as trade but:
- Form includes `serviceType` (verification/analysis/processing/custom) + arbitrary Rust code + query rules (`RequestDataService.vue`)
- **5-round cyclic execution** (`ServiceOrderProcessingDetail.vue`): owner provides encrypted execution result + ZKVM correctness proof per round
- **No on-chain order** — service orders exist only in SQLite (confirmed by inspecting all backend route handlers and chaincode functions)

### 2.7 Token Model

Confirmed from `chaincode/user.go:11-15,98-109,112-146`:

- **Two-tier balance**: `TokenBalance` (free) + `LockedTokenBalance` (escrowed for pending orders)
- **Lock**: `TransferLockedTokenBalance(username, amount)` → moves `free → locked` (`user.go:98`)
- **Transfer**: `transferTokens(from, to, amount)` → debits from *locked* balance (`user.go:131-132`), credits to free (`user.go:138`)
- **No mint/burn on chaincode** — the finance page (`FinanceView.vue`) calls `POST /mint` and `POST /burn` but no chaincode handler exists for these

### 2.8 Hash Chain Mechanism

Confirmed from `chaincode/utils.go:23-32,35-48` and `backend/src/utils/utils.mjs:18-33`:

- `GenerateHashChain(n)` produces `n` SHA-256 hashes by iterative hashing of a random secret
- The last element (`hashChainEnd`) is stored in the order
- On `CompleteOrder`, buyer submits a `preImage`; `ComputeSha256Times(preImage, hashChainEnd)` counts how many SHA-256 iterations until equality
- `tokenAmount = hashSteps * TokenUnit` → buyer pays proportionally to the hash-chain position

### 2.9 IPFS Integration

Evidence: `backend/src/services/ipfs.mjs:76-97`, `ipfs/Readme.md`, `frontend/package.json` includes `kubo-rpc-client`

- `uploadToIPFS(path, mfsRoot)` → stores files with MFS path `/YYYY/MM/DD/HH/mm/ss/`
- `downloadFromIPFS(cid)` → **stub** — confirms existence but no actual download implemented (`ipfs.mjs:86-97`)
- Demo datasets use `generateIPFSCID()` → fake `"Qm" + randStr(44)` (`utils.mjs:9-11`)
- Encryption: `crypto-js` in frontend, `elliptic` (elliptic curve) — used for client-side hash computation and signatures but no on-chain encryption model

### 2.10 Dual Storage Architecture

Critical pattern: every entity exists both in SQLite and (for dataset/order/user) on Fabric chaincode:

| Entity | Off-chain (SQLite) | On-chain (Fabric) |
|---|---|---|
| User | Yes (username + password) | Yes (username + balances) |
| Dataset | Yes (12 columns, full metadata) | Yes (hash + owner only) |
| TradeOrder | Yes (11 columns, full metadata) | Yes (8 fields, hash-chain + status) |
| ServiceOrder | Yes (12 columns) | No |

---

## 3. Gaps, Demo Shortcuts, Correctness Risks

### 3.1 Critical Gaps

| Issue | Evidence | Severity |
|---|---|---|
| **No authentication middleware** | `app.mjs` mounts routes directly without any auth middleware; `req.user?.username` appears in order services but nothing sets it | High — entire system has no access control |
| **No user identity binding** | Chaincode `CreateOrder` accepts `buyer` as a string parameter without verifying caller identity (no `GetClientIdentity`) — `order.go:25` | High — anyone can create orders as any user |
| **CompleteOrder tests commented out** | `order_test.go:167,176,182` — CompleteOrder tests are all commented out | High — core settlement logic untested |
| **Service orders have no on-chain counterpart** | No `serviceOrder` struct or functions in chaincode; service workflow relies entirely on mutable SQLite | High — service orders cannot benefit from blockchain guarantees |
| **Finance page references undefined `publicKey`** | `FinanceView.vue:144` uses `authStore.publicKey` but Pinia store (`stores/auth.js`) only has `username` and `isLoggedIn` | Medium — wallet feature non-functional |
| **Dev mode destroys all data on restart** | `app.mjs:33-67` drops and recreates all tables on every startup | Medium — cannot persist state across restarts |
| **IPFS upload has no error handling for large files** | `ipfs.mjs:76-84` uses `multer` for file upload but no chunking, no file-type validation, no size limits | Medium |
| **Hash chain MAX=200 limits maximum payment** | `utils.go:38` — `ComputeSha256Times` gives up after 200 iterations | Low — limits hash-chain length, but 200 is generous |

### 3.2 Demo/Mock Shortcuts

| Shortcut | Location | Impact |
|---|---|---|
| All 25 datasets use hardcoded hashes | `chaincode/chaincode.go:35-62` | Not real files, no real IPFS CIDs |
| Fake IPFS CIDs: `Qm + randStr(44)` | `utils.mjs:9-11` | Not real IPFS content |
| `ipfsAddress` always `''` in AddDataset | `AddDataset.vue` form data | No actual file upload to IPFS |
| `POST /bcCreateOrder` uses hardcoded test values | `RequestDataTrade.vue:577` — `datasetHash: "test_hash"`, `hashChainEnd: "test_hash_chain_end"` | Trade orders on chaincode are meaningless |
| TradeOrders.vue uses 100% mock data | `TradeOrders.vue` — all orders are hardcoded JavaScript arrays | Demo UI only |
| `downloadFromIPFS` is stub | `ipfs.mjs:86-97` | No real file retrieval |
| All tables dropped each startup | `app.mjs:33-67` | No persistence |

### 3.3 Copied Fabric Samples

- `blockchain/asset-transfer-basic/` — complete copy of Fabric samples (chaincode-java, chaincode-go, application-gateway-*, rest-api-*). Not audit-relevant.
- `blockchain/test-network/` — modified Fabric test-network with org3, explorer, prometheus. Standard Fabric boilerplate.

---

## 4. Reusable Ideas/Assets

| Idea | Source Files | Migration Value for FishboneChain | Map to FishboneChain Concept |
|---|---|---|---|
| **4-flag permission model** (`isPublic`, `canMaskingShare`, `canCustomMaskingTrade`, `canDataService`) | `datasetsTable.mjs:12-16`, `DatasetEdit.vue` | Excellent granularity for data-sharing consent. Fishbone's data-trade can adopt this as asset visibility levels. | Asset visibility policy: Private → MaskableShare → Tradeable → Serviceable |
| **Hash-chain-based pay-per-step** | `utils.go:23-48`, `order.go:103-137` | Novel payment mechanism: buyer pre-funds, seller commits per-step, payment scales with consumption. Fishbone can use as optional payment mode for streaming/iterative data delivery. | Payment channel alternative for data streams |
| **Cyclic delivery protocol** (10/5 rounds) | `TradeOrderProcessingDetail.vue`, `ServiceOrderProcessingDetail.vue` | Escrow-style incremental delivery reduces trust requirements. Fishbone can use for large dataset or computation-results exchange. | Multi-round escrow delivery |
| **Dual storage: metadata in DB, proof on chain** | Full architecture | Reduces on-chain bloat while maintaining audit trail. Fishbone should consider this for non-critical metadata. | Off-chain metadata + on-chain audit |
| **Masking rules as structured JSON** | `RequestDataTrade.vue` form, `tradeOrdersTable.mjs:8` | Concrete schema for data-masking requests: keyName, keyType (string/number), constraintType (equals/contains/range). Reusable for Fishbone's data-trade privacy layer. | Privacy-preserving data trade query schema |
| **Cross-blockchain federation** (per-domain chains) | `blockchainsTable.mjs`, per-chain dataset tables | Multi-domain data silos with separate governance. Fishbone's parachain/domain model maps directly. | Domain-specific data marketplaces |
| **ZKVM proof in service workflow** | `ServiceOrderProcessingDetail.vue` | Hints at verifiable computation. Fishbone can adopt for trustless data services. | Verifiable data service execution |
| **Client-side SHA-256 hashing** | `AddDataset.vue` uses CryptoJS for file hashing | Avoids uploading large files just to compute integrity hash. Fishbone should do same. | Off-chain file integrity verification |

---

## 5. Migration Notes for FishboneChain

### 5.1 What to Inherit

1. **Permission model**: The 4-flag system (`isPublic` / `canMaskingShare` / `canCustomMaskingTrade` / `canDataService`) is directly mappable to Fishbone's asset policy pallet. Extend with additional flags like `canResell`, `canDerive`, `expiresAt` for richer data-rights management.

2. **Hash-chain payment**: The concept of iterative SHA-256 proving for per-step payment is clean and can be implemented as a Substrate pallet. Replace Fabric's chaincode pattern with a `pallet-hashchain-payment` that:
   - Stores `hash_chain_end` in order storage
   - Verifies via `compute_sha256_times(preimage, hash_chain_end)`
   - Transfers tokens from buyer's reserved balance to seller

3. **Cyclic delivery**: Implement as a Substrate `pallet-escrow-delivery` with:
   - Configurable round count (not hardcoded 10/5)
   - Per-round state: `{ data_submitted: Option<Hash>, payment_submitted: Option<Hash> }`
   - Dispute resolution mechanism (SciDataHub has none — either party can terminate unilaterally)

4. **Domain-specific marketplaces**: Fishbone's parachain model is a natural fit for SciDataHub's per-blockchain domain separation (Physics, Biology, Medicine, AI, CyberSecurity).

### 5.2 What NOT to Inherit

1. **Dual-storage pattern with mutable off-chain DB**: SciDataHub's SQLite holds authoritative dataset metadata while chaincode only stores `(hash, owner)`. Fishbone should keep metadata on-chain or at minimum use content-addressed off-chain storage with on-chain hash verification.

2. **No-access-control chaincode**: SciDataHub's chaincode has zero caller identity verification. Fishbone MUST enforce `ensure_signed` origin checks on all extrinsic calls.

3. **Service orders with no on-chain record**: Either bring service orders fully on-chain or use a verifiable off-chain execution model (e.g., ZKVM proofs verified on-chain).

4. **Flat user model (no roles)**: SciDataHub has no role abstraction. Fishbone should model roles explicitly (DataUploader, DataConsumer, DataValidator, Auditor, Arbitrator).

5. **Demo data generation pattern**: SciDataHub generates fake IPFS CIDs and hardcoded hashes. Fishbone should enforce real content-addressing from day one.

### 5.3 Architecture Recommendations

| Concern | SciDataHub Approach | Fishbone Recommendation |
|---|---|---|
| Identity | Plain username + PBKDF2 | Substrate account-based (`AccountId32`) + optional DID |
| Storage | SQLite + Fabric world state | Substrate storage maps + optional IPFS/Arweave for blobs |
| Payment | Simulated token with 2-tier balance | Native pallet-assets or pallet-balances with reservations |
| Encryption | `crypto-js` / `elliptic` (frontend only) | On-chain key registry + off-chain encryption with on-chain signature verification |
| Computation | Rust code in form (no execution) | ZKVM integration (e.g., RISC Zero, SP1) for verifiable data services |
| Delivery | Fixed 10/5 round cycles with manual steps | Automated round progression with timeout-based dispute triggers |

---

## 6. Follow-up Files/Questions

**Files to inspect for deeper follow-up:**
- `backend/test/database/orders/` — order service table tests for understanding edge cases
- `frontend/src/views/Finance/FinanceView.vue:144` — confirm the `publicKey` bug (only in source, not dynamic only)
- `caliper/workload/order-management-test.js` — real benchmark workload reveals intended production patterns beyond demo
- `blockchain/sci-data-trade/chaincode-go/chaincode/mocks/` — generated Fabric mocks, not relevant but confirms chaincode interface

**Open questions for FishboneChain maintainers:**

1. **Is a cyclic-delivery protocol needed, or is atomic swap sufficient?** SciDataHub's 10-round protocol is heavy. If Fishbone uses Substrate-native escrow + timeout, consider simplifying to single-round or 2-round commit/reveal.

2. **Should service orders be on-chain?** If Fishbone wants verifiable data services, the service request + proof-of-execution must both be on-chain. Consider a `pallet-data-service` with extrinsic flow: `request_service` → `submit_execution_result` → `verify_zk_proof` → `release_payment`.

3. **Hash-chain payment vs. linear pricing:** The hash-chain model is elegant but complex. Evaluate whether simpler per-access pricing (flat fee per dataset, flat fee per query) covers Fishbone's initial use cases before implementing iterative hash proving.

4. **Do the `rejected` and `failed` statuses need consensus?** SciDataHub has `rejected` only in frontend mock data, not in backend validators. Decide early which status transitions require on-chain evidence vs. which are purely off-chain state.

5. **What privacy model does Fishbone need?** SciDataHub's masking rules (equals/contains/range on named fields) is a rudimentary field-level privacy scheme. Fishbone may want homomorphic encryption, differential privacy, or trusted execution environments instead.
