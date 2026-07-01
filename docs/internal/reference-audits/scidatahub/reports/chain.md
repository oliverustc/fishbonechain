The audit report is written to `docs/internal/reference-audits/scidatahub/reports/chain.md`.

**Key findings in brief:**

1. **Identity**: All Fabric transactions use a single hardcoded identity (`User1@org1.example.com`). Zero per-end-user signing.

2. **Trust boundary**: The backend is an omnipotent oracle. It passes `buyer`/`seller`/`username` as plain-text strings; the chaincode has **no access control** (no `GetClientIdentity()` calls, no MSP checks, no signature verification).

3. **On-chain state**: Only 3 minimal structs — `User` (balance), `Dataset` (hash+owner), `Order` (basic fields). Everything rich (metadata, permissions, masking rules, service configs) is in SQLite off-chain.

4. **Broken code**: `bcTransferTokens` calls an unexported Go function (`transferTokens` — lowercase). `bcCompleteOrder` is missing the required `preImage` parameter. Both would fail at runtime.

5. **Demo shortcuts**: Entire SQLite DB dropped/recreated on every server start (`app.mjs:32`), hardcoded LAN IP in frontend (`192.168.8.22`), IPFS download is a stub with "don't know how to download yet" comment, `asset-transfer-basic/` and `test-network/` are verbatim Fabric samples with zero SciDataHub integration.

6. **Reusable**: The hash-chain preimage payment mechanism, the 3-entity domain model, and the backend-to-chaincode adapter layer pattern are worth adapting for FishboneChain — but with per-user Substrate key signing replacing the single-proxy trust model.
tity()` and `newSigner()`:
- Load the single cert file from `signcerts/` directory (first file found)
- Load the single private key from `keystore/` directory (first file found)
- All gateway connections use this ONE `(mspId, credentials, signer)` triplet

**Evidence route**: every chaincode service function (`userService.mjs`, `datasetService.mjs`, `orderService.mjs`) calls `await initializeContract()` which always returns a contract bound to `User1@org1.example.com`.

**Caliper benchmarks confirm the single-admin pattern** — `caliper/benchmarks/*.yaml` all set:
```yaml
invokerIdentity: 'Admin@org1.example.com'
```

`caliper/networks/networkConfig.yaml:19-23` configures only one identity (`Admin@org1.example.com`) with its hardcoded key path.

#### 2.2 End-user signatures: absent

There is no cryptographic link between the end user and on-chain actions:

- **Frontend auth** (`frontend/src/stores/auth.js`): login state stored in `localStorage` (key `isLoggedIn`, value `'true'`). No keys, no signatures, no wallet.
- **Backend auth** (`backend/src/database/users/usersService.mjs:80-126`): PBKDF2 password verification against SQLite. Valid login returns `{ username }` — no token, no session ID, no signed challenge.
- **No auth is enforced on blockchain routes**: `chaincodeRoutes.mjs` mounts `/bc*` endpoints with no middleware. No JWT verification, no session check, no API key.
- **Caller identity is passed as a plain-text string parameter**: `bcCreateOrder` takes `{ buyer, seller }` from `req.body` (orderService.mjs:9). `bcAddUser` takes `{ username, tokenBalance }` from `req.body` (userService.mjs:9). The chaincode trusts these strings unconditionally.
- **Any user can impersonate any other user** by simply changing the `username`/`buyer`/`seller`/`owner` string in the HTTP request.

#### 2.3 What is on-chain (Fabric world state)

Three KV-prefix namespaces, all minimal:

| Namespace | Key pattern | Fields | Evidence |
|---|---|---|---|
| Users | `user_{username}` | `username`, `tokenBalance`, `lockedTokenBalance` (int) | `chaincode/user.go:11-15` |
| Datasets | `dataset_{hash}` | `hash` (32-byte hex), `owner` (string) | `chaincode/dataset.go:11-14` |
| Orders | `order_{orderID}` | `id`, `datasetHash`, `hashChainEnd`, `tokenUnit`, `buyer`, `seller`, `status`, `timestamp` | `chaincode/order.go:13-22` |

`InitLedger` (`chaincode/chaincode.go:16-90`) seeds 2 demo users (100 tokens each) + 25 hardcoded 32-byte hex dataset hashes (all owned by `demoDataOwner`) + empty orders array.

**Chaincode accessible functions (19 total):**

| Category | Function | Args | Type |
|---|---|---|---|
| Lifecycle | `InitLedger` | none | Submit |
| User | `AddUser`, `GetUser`, `GetTokenBalance`, `GetLockedTokenBalance`, `SetTokenBalance`, `AddTokenBalance`, `TransferLockedTokenBalance` | username + value(s) | Mixed |
| Dataset | `AddDataset`, `GetDataset`, `GetDatasetOwner`, `GetAllDatasets` | hash + owner | Mixed |
| Order | `CreateOrder`, `GetOrder`, `UpdateOrderStatus`, `CompleteOrder`, `GetAllOrders` | order fields | Mixed |
| (internal) | `transferTokens` | from, to, amount | **unexported — inaccessible via Invoke** |

#### 2.4 What is off-chain (SQLite)

All rich metadata lives in SQLite (`backend/src/database/`):

| Table | Key off-chain fields | Table source |
|---|---|---|
| `users` | `password` (PBKDF2 hash), `created_at` | `usersTable.mjs` |
| `blockchains` | `name`, `fullName`, `description` | `blockchainsTable.mjs` |
| `datasets_{chain}` | `name`, `fullName`, `description`, `isPublic`, `canMaskingShare`, `canCustomMaskingTrade`, `canDataService`, `maskingDatasetIPFSAddress` | `datasetsTable.mjs` |
| `trade_orders_{chain}` | `title`, `description`, `datasetName`, `datasetOwner`, `requester`, `maskingRules` (JSON), `status` | `tradeOrdersTable.mjs` |
| `service_orders_{chain}` | `title`, `description`, `serviceType`, `serviceConfig` (JSON), `status` | `serviceOrdersTable.mjs` |

The `_{chain}` suffix is dynamic: tables are created per-blockchain-name at startup (`app.mjs:58-67`). Five hardcoded chains: Physics, Biology, Medicine, ArtificialIntelligence, CyberSecurity.

The on-chain `Dataset.hash` field is a content hash — but the actual data (IPFS CID, file metadata) lives only in SQLite. The on-chain `Order.datasetHash` links to an on-chain dataset, but the off-chain `trade_orders_{chain}` table stores the actual human-readable order details (title, description, maskingRules).

#### 2.5 Asset-transfer-basic: copied Fabric sample, unused

`blockchain/asset-transfer-basic/` is a verbatim copy of the upstream `fabric-samples/asset-transfer-basic`:
- 5 language variants (Go, Java, JavaScript, TypeScript chaincodes + external builder)
- 3 gateway app variants (Go, Java, JavaScript, TypeScript)
- 2 REST API samples (Go, TypeScript)
- **Zero integration with SciDataHub** — different chaincode contract, different data model (assets with color/size/owner/appraisedValue)

`blockchain/test-network/` is a copy of `fabric-samples/test-network` — all the core YAML files, compose configs, crypto-gen scripts, and Fabric utils are standard sample material.

`blockchain/prepare.md` walks through the **standard** Fabric test-network + `asset-transfer-basic` chaincode setup, not SciDataHub-specific deployment.

---

### 3. Gaps, demo shortcuts, or correctness risks

#### 3.1 Demo-only reset on every startup

`backend/src/app.mjs:32` — explicit developer comment:
> `// 开发环境每次启动后端时，都重新初始化数据库，正式版删除后续代码`
> (Translation: "In dev env, reinitialize DB on every restart. Delete the following code for production.")

Lines 33-69 drop and recreate ALL tables (users, blockchains, datasets, orders) on every server start. This means all user registrations, dataset registrations, and orders are **wiped on restart**. This is showcased as a "feature" for development convenience.

#### 3.2 Broken/impossible function calls

**`bcTransferTokens` route is broken** (`userService.mjs:125-141`):
```javascript
await contract.submitTransaction('transferTokens', from, to, amount);
```
But in Go, `transferTokens` is a **lowercase (unexported) function** (`chaincode/user.go:112`):
```go
func (s *SmartContract) transferTokens(ctx contractapi.TransactionContextInterface, ...)
```
Fabric's `contractapi` only exposes **exported** (uppercase) methods via Invoke. This call will fail at runtime with "function transferTokens not found".

**`bcCompleteOrder` is missing a required parameter** (`orderService.mjs:78`):
```javascript
await contract.submitTransaction('CompleteOrder', orderID);
```
But the Go function signature (`chaincode/order.go:103`):
```go
func (s *SmartContract) CompleteOrder(ctx contractapi.TransactionContextInterface, orderID string, preImage string)
```
Missing the `preImage` argument. This will fail at runtime.

#### 3.3 No chaincode-level access control

Every chaincode function is open to anyone who can reach the backend:

- `SetTokenBalance` — ANY caller can set ANY user's balance to ANY value (user.go:78)
- `AddTokenBalance` — ANY caller can mint unlimited tokens for ANY user (user.go:88)
- `AddUser` — ANY caller can create users with ANY initial balance (user.go:18)
- `UpdateOrderStatus` — ANY caller can set ANY order to ANY status, including "completed", completely bypassing `CompleteOrder` validation logic (order.go:87)
- There is **zero** `ctx.GetClientIdentity().GetMSPID()` or `GetID()` usage anywhere in the chaincode

#### 3.4 `CompleteOrder` verification is trivial to bypass

The hash chain preimage verification (`chaincode/order.go:103-136`):
- Only runs inside `CompleteOrder`, but `UpdateOrderStatus` can set `status = "completed"` without any verification
- `ComputeSha256Times` (`utils.go:34-48`) has a hardcoded `MAX = 200` iteration limit — perfectly fine for a demo, but in a real system this needs to be a configurable parameter or tied to the tokenUnit count
- The function returns `-1` with an error if the hash doesn't match within 200 iterations. A long hash chain beyond 200 would be rejected.

#### 3.5 Token system is unbounded

- No total supply cap, no issuance authority
- Integer-based (`int` type), no overflow protection
- `AddTokenBalance` can mint infinite tokens
- No transaction fees, no slashing conditions
- The `LockedTokenBalance` mechanism is a simple escrow pattern with no timelock or dispute resolution

#### 3.6 IPFS integration is incomplete

`backend/src/services/ipfs.mjs:92`:
```javascript
logger.info("可以在IPFS中查询到文件，但是目前还不会如何下载这个文件，需要继续研究")
// "Can query the file in IPFS, but don't yet know how to download it, need to continue research"
```
Upload path works; download is a stub. The `generateIPFSCID()` in `utils.mjs:19-21` generates fake CIDs (`"Qm" + random 44 chars`).

#### 3.7 Hardcoded developer IP

`frontend/src/api/axios.js:5`:
```javascript
baseURL: 'http://192.168.8.22:3101',
```
This is a LAN IP from a specific developer's machine, not a configurable endpoint.

---

### 4. Reusable ideas/assets

#### 4.1 Domain model mapping

The three-entity separation (User, Dataset, Order) is clean and maps well to FishboneChain's `data-registry` + `trade-session` pallets. The schema is minimal and tested — a good starting point for Substrate storage items.

#### 4.2 Hash chain preimage payment mechanism

The `hashChainEnd` + `preImage` reveal pattern (`utils.go:23-48`) is the core innovation worth adapting:
- Seller commits to `hashChainEnd` (the last hash in a chain)
- Buyer pays proportionally to how many hash preimages are revealed
- `CompleteOrder` computes `sha256(preImage)` iteratively until it reaches the committed hash, then pays `times * tokenUnit`
- This is a well-defined micropayment-by-data-revelation mechanism suitable for ZK-enhanced trade

#### 4.3 Backend chaincode service layer pattern

`backend/src/services/chaincode/` has a clean separation:
- `chaincode.mjs` — Fabric gateway init, identity loading, gRPC connection
- `userService.mjs` — HTTP→chaincode adapters for user operations
- `datasetService.mjs` — HTTP→chaincode adapters for dataset operations
- `orderService.mjs` — HTTP→chaincode adapters for order operations
- `chaincodeRoutes.mjs` — Express router wiring

This pattern can be adapted for a Substrate RPC client layer, replacing `contract.submitTransaction()` with polkadot-js `tx.pallet.method()` calls.

#### 4.4 Dynamic per-chain table scheme

The `_{chain}` suffix pattern in SQLite (`app.mjs:58-67`) creates per-blockchain-namespace tables. The concept of multi-chain data sharding is relevant to FishboneChain's child-chain architecture — though the implementation (SQLite string-interpolation) should be replaced with actual Substrate chain/para-ID scoping.

#### 4.5 Caliper benchmark framework

The Caliper workload and benchmark YAML files provide a performance testing methodology with realistic data access patterns (create users → register datasets → create orders → query → update) that can be adapted to FishboneChain's Substrate benchmarking framework.

#### 4.6 Unused: chaincode unit test patterns

`chaincode/*_test.go` uses `counterfeiter`-generated mocks for the Fabric chaincode stub. The mock pattern (`mocks/chaincodestub.go`, `mocks/transaction.go`, `mocks/statequeryiterator.go`) shows how to test chaincode logic without a real Fabric network — this is transferable to Substrate's `#[test]` mock runtime pattern.

---

### 5. Migration notes for FishboneChain

#### 5.1 Identity: from single-proxy to per-user keys

The single biggest architectural gap. FishboneChain MUST NOT replicate the "one gateway identity speaks for everyone" model.

Recommendations:
- Each end-user controls a Substrate keypair (sr25519/ed25519) in a browser wallet or CLI
- Transactions (`data-registry::register`, `trade-session::create_order`) are signed by the end-user's key, submitted directly or via a thin relay
- On-chain logic uses `ensure_signed(origin)` → `sender` account ID for all authorization
- The backend (if retained) becomes a read-only indexer / event listener, not a transaction submitter

#### 5.2 State migration: what moves on-chain

**Move on-chain (current SciDataHub SQLite → Substrate storage):**
- `Dataset`: hash, owner (already on-chain), plus `isPublic`, `maskingDatasetIPFSAddress` (currently off-chain)
- `Order.buyer/seller/tokenUnit/status` (already on-chain)
- Token balances (already on-chain via `User` struct, migrate to `pallet_balances` or custom asset)

**Keep off-chain or in IPFS:**
- Rich dataset metadata (`fullName`, `description`, `serviceConfig`) — large blobs
- User credentials (passwords) — replace with Substrate key-based auth entirely
- Blockchain configuration metadata (name, description) — use `chain-profile` pallet

**Drop entirely:**
- The `_{chain}` SQLite dynamic table pattern — replace with Substrate chain/para-ID scoping
- Demo seed data (`InitLedger` hardcoded hashes)
- `asset-transfer-basic/` — irrelevant Fabric sample code

#### 5.3 Chaincode logic to pallet mapping

| SciDataHub chaincode | FishboneChain pallet | Notes |
|---|---|---|
| `AddUser` / `GetUser` | `pallet_balances` (native) or `data-registry` (custom account) | Replace string usernames with AccountId |
| `SetTokenBalance` / `AddTokenBalance` | `pallet_balances::set_balance` (root) or `assets::mint` | Must be restricted to governance |
| `AddDataset` / `GetDataset` | `data-registry` pallet | Add hash + owner + metadata digest |
| `CreateOrder` / `CompleteOrder` | `trade-session` pallet | Adapt hash chain preimage reveal to ZK verification |
| `transferTokens` | `pallet_balances::transfer` or escrow pallet | Replace locked balance with reserve/escrow |
| `InitLedger` | Genesis config | Drop demo seed |

#### 5.4 Escrow & payment flow

The SciDataHub `LockedTokenBalance → transferTokens` pattern is a simple 2-step escrow:
1. Buyer calls `TransferLockedTokenBalance` to lock funds
2. On `CompleteOrder`, locked funds transfer from buyer→seller

FishboneChain's `main-escrow` pallet should implement this with:
- Timeout-based refund (currently missing from SciDataHub)
- Dispute resolution (currently missing)
- Multi-party escrow (SciDataHub is buyer-seller only)

#### 5.5 Hash chain → ZK upgrade path

The `ComputeSha256Times` iterative hash verification is exactly the primitive that FishboneChain's `tools/data-trade-zk/` Go module replaces with zero-knowledge proofs. The migration path:
1. SciDataHub: `sha256(preImage)` iterated until `hashChainEnd` matches → pay by count
2. FishboneChain: ZK proof that `preImage` is within a committed Merkle tree / accumulator, verified on-chain via `trade-session` pallet calling the ZK verifier

The domain concept (seller commits to data root, reveals incrementally, payment scales with value) is preserved; the cryptographic mechanism is upgraded.

#### 5.6 What to explicitly NOT migrate

- The single-proxy identity model
- The "backend owns Fabric wallet" trust delegation
- `InitLedger` with hardcoded demo data (use genesis config instead)
- The `bc*` route prefix pattern (move to direct RPC calls)
- The frontend `localStorage` auth (use wallet extension)
- `asset-transfer-basic/` and `test-network/` Fabric samples (Fabric-specific infrastructure)
- `caliper/` benchmarking tooling (replace with Substrate `frame-benchmarking`)
- The `ipfs.mjs` download stub (use FishboneChain's existing IPFS integration or replace)

---

### 6. Follow-up files/questions

**Files referenced for verification:**
- `references/SciDataHub/backend/src/config/blockchain_config.mjs` — identity paths
- `references/SciDataHub/backend/src/services/chaincode/chaincode.mjs` — gateway init
- `references/SciDataHub/backend/src/services/chaincode/userService.mjs` — user service
- `references/SciDataHub/backend/src/services/chaincode/orderService.mjs` — order service (note broken `CompleteOrder`)
- `references/SciDataHub/blockchain/sci-data-trade/chaincode-go/chaincode/` — all chaincode Go files
- `references/SciDataHub/backend/src/app.mjs` — demo reset logic
- `references/SciDataHub/caliper/networks/networkConfig.yaml` — benchmark identity
- `references/SciDataHub/frontend/src/stores/auth.js` — localStorage-only auth
- `references/SciDataHub/frontend/src/api/axios.js` — hardcoded IP

**Open questions for FishboneChain design:**
1. Should the backend become a read-only event indexer, or be eliminated entirely in favor of direct wallet-to-chain communication?
2. For the hash chain→ZK migration: should the `trade-session` pallet include a fallback SHA-256 path (for non-ZK lightweight clients), or go ZK-only?
3. Should the chaincode's `CompleteOrder` combined settlement (verify proof + transfer tokens atomically) remain a single extrinsic, or split into two-phase commit (verify → settle)?
4. Should dataset metadata (name, description, IPFS CID) live on-chain (via `data-registry`) or in a separate off-chain indexer with on-chain hash anchors?
