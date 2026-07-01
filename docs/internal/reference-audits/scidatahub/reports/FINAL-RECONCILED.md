# SciDataHub Reference Audit — Final Reconciled Report

**Date**: 2026-07-01
**Repo**: `references/SciDataHub` (commit `d75aebcddd87`, branch `dev`)
**Report count**: 6 source reports reconciled (backend, benchmarks, chain, domain, frontend, inherit) +
2 context-only (inventory, synthesis are empty)
**Deterministic context**: 7 files under `context/`

---

## 1. Scope Inspected

| Report | Scope | Files |
|--------|-------|-------|
| backend.md | Backend 29 src + 14 test | `backend/src/`, `backend/test/` |
| benchmarks.md | Caliper + Go tests | `caliper/`, `blockchain/sci-data-trade/chaincode-go/*_test.go` |
| chain.md | Fabric chaincode + identity | `blockchain/sci-data-trade/chaincode-go/`, `blockchain_config.mjs` |
| domain.md | Domain model end-to-end | All frontend views, chaincode, backend, IPFS |
| frontend.md | Vue.js SPA | `frontend/src/` (22 routes, ~30 views) |
| inherit.md | 27-item inheritance matrix | Cross-cutting, all modules |

---

## 2. Confirmed Facts with Evidence

All facts below are corroborated by **2+ independent reports** or by **deterministic context cross-references**.

### 2.1 Architecture (uncontested)

**Backend**: Node.js ESM, Express 4.x, SQLite3, Fabric Gateway gRPC, kubo-rpc-client
- Clean 4-layer pattern: `Routes → Service → Table → db.mjs`
- 35 REST endpoints spanning users, blockchains, datasets, trade/service orders, chaincode proxy
- `backend/src/database/` tables (6 types): `users`, `blockchains`, `datasets_{chain}`, `trade_orders_{chain}`, `service_orders_{chain}`
- Domain tables are dynamically created per blockchain name at startup
- 5 hardcoded chains: Physics, Biology, Medicine, ArtificialIntelligence, CyberSecurity

**Chaincode**: Go (`fabric-contract-api-go/v2`), 5 source files + 5 test files + counterfeiter mocks
- 3 structs: `User`, `Dataset`, `Order` (minimal on-chain footprint)
- 19 exported functions; `transferTokens` is lowercase/unexported — unreachable via Invoke
- `asset-transfer-basic/` (~200 files) and `test-network/` (~90 files) are **verbatim Fabric samples** with zero SciDataHub integration

**Frontend**: Vue 3 + Pinia + Vue Router 4 + Vite, Creative Tim "Vue Material Kit 2" commercial template
- 22 routes, ~30 views, `localStorage`-based auth, hardcoded LAN IP
- Two competing order UIs (`views/Order/` old + `views/orders/` new/mock-only)

### 2.2 Domain Model (uncontested, domain.md + chain.md + inherit.md agree)

| Entity | On-chain (Fabric world state) | Off-chain (SQLite) |
|--------|------|------|
| User | `{Username, TokenBalance, LockedTokenBalance}` | `{username, password (PBKDF2), created_at}` |
| Dataset | `{Hash (SHA256), Owner}` | 15 fields incl. `isPublic`, `canMaskingShare`, `canCustomMaskingTrade`, `canDataService`, `maskingDatasetIPFSAddress` |
| TradeOrder | `{ID, DatasetHash, HashChainEnd, TokenUnit, Buyer, Seller, Status, Timestamp}` | 11 fields incl. `maskingRules (JSON)`, `title`, `description` |
| ServiceOrder | **None — no chaincode support** | 12 fields incl. `serviceType`, `serviceConfig (JSON)` |

**4-flag permission model** (datasetsTable.mjs:12-16, DatasetEdit.vue):
- `isPublic` — visible in listings
- `canMaskingShare` — static masking download via IPFS
- `canCustomMaskingTrade` — buyer initiates hash-chain trade
- `canDataService` — buyer requests computation/verification

**Dual exchange modes**: Trade Orders (with on-chain `CompleteOrder` settlement) + Service Orders (SQLite-only, no chaincode counterpart, no on-chain guarantees).

**Order states**: `pending → processing → completed/cancelled/failed`; `rejected` exists only in frontend mock data, not in backend validators.

### 2.3 Hash-Chain Settlement Protocol (uncontested, all reports confirm)

Location: `utils.go:23-48` + `order.go:103-137` + `utils.mjs:18-33`
1. Seller computes `GenerateHashChain(N)` — iterates SHA-256 N times from random seed
2. Seller publishes tail (`hashChainEnd`) in order
3. Buyer escrows tokens via `TransferLockedTokenBalance`
4. Seller reveals pre-image to buyer
5. Buyer calls `CompleteOrder(orderID, preImage)` — on-chain `ComputeSha256Times` counts iterations until `hashChainEnd` match
6. `tokenAmount = steps * tokenUnit` transferred from buyer locked → seller balance
7. MAX 200 iterations (hardcoded in `utils.go:38`)

### 2.4 Tests & Benchmarks (uncontested, benchmarks.md + inherit.md agree)

- **Go unit tests**: 5 test files (user, dataset, order, chaincode, utils) using counterfeiter mocks
  - `CompleteOrder` test cases (`order_test.go:167,176,182`) are **all commented out**
- **Backend tests**: 14 Jest+supertest files — all mock `db.mjs`, test route/table/service layers; no integration tests
- **Frontend tests**: Zero
- **Caliper benchmarks**: Only 1 round actually ran (`SetTokenBalance` at 100 TPS, 188/2000 failures)
  - 5 benchmark YAMLs exist; ~29 rounds defined but most are commented out or aspirational
  - Reproducibility broken by hardcoded keystore path, chaincode name mismatch (`datatrading` vs `scidatahub`), missing `init-contract-benchmark.yaml`
  - 5 legacy Caliper workloads target a deprecated chaincode API
- **Manual test script** (`chaincode.test.mjs`): no assertions, just console logging

---

## 3. Gaps, Demo Shortcuts, Correctness Risks

### 3.1 Critical Bugs (confirmed by 2+ reports)

| Bug | Reports | Evidence |
|-----|---------|----------|
| `bcCompleteOrder` sends 1 arg, chaincode expects 2 (orderID, preImage) | chain.md, inherit.md, benchmarks.md | `orderService.mjs:78` vs `order.go:103` |
| `bcTransferTokens` calls unexported Go function `transferTokens` — unreachable via Fabric Invoke | chain.md | `userService.mjs:125-141`, `user.go:112` (lowercase) |
| `CompleteOrder` Go tests all commented out — core settlement logic untested | benchmarks.md, inherit.md, domain.md | `order_test.go:167,176,182` |
| Zero authentication middleware — `req.user` never populated | backend.md, domain.md, frontend.md | No auth middleware in `app.mjs`, login returns no token/JWT/session |
| No chaincode access control — `GetClientIdentity()` never called | chain.md, domain.md | All chaincode `.go` files, zero `GetClientIdentity` usage |

### 3.2 Severe Demo Shortcuts (confirmed by 2+ reports)

| Shortcut | Reports | Evidence |
|----------|---------|----------|
| SQLite DB dropped/recreated on every server start | backend.md, chain.md, domain.md, inherit.md | `app.mjs:32-69`, comment: "正式版删除后续代码" |
| Frontend hardcodes `192.168.8.22:3101` | frontend.md, chain.md | `axios.js:5` |
| IPFS download is unimplemented stub ("还不会如何下载") | backend.md, chain.md, domain.md | `ipfs.mjs:92` |
| `generateIPFSCID()` produces fake CIDs (`"Qm" + randStr(44)`) | backend.md, domain.md | `utils.mjs:19-21` |
| `authStore.publicKey` referenced but undefined (FinanceView, BuyView, SellView, CreateOrder) | frontend.md (primary), domain.md (Finance) | `stores/auth.js` has only `username` + `isLoggedIn` |
| Single Fabric identity for ALL transactions (`User1@org1.example.com`) | chain.md, inherit.md | `blockchain_config.mjs:62-81`, Caliper YAMLs |
| 25 hardcoded SHA-256 hex strings as demo dataset "hashes" — not real files | chain.md, inherit.md, domain.md | `chaincode.go:35-62`, `datasetsDemo.mjs:5-31` |
| TradeOrders.vue, ServiceOrders.vue use 100% `generateMockTradeOrders()` | frontend.md, domain.md | No API calls, `setTimeout(800)` mock latency |

### 3.3 Single-Report Findings (needs manual verification)

| Finding | Source | Verification check |
|---------|--------|--------------------|
| `getPublicDatasets` reads `req.params.blockchainName` which is undefined at that route | backend.md | Test frontend `PublicDatasets.vue:179` which calls `/${blockchainName}` variant |
| `resultJson` undefined variable in `orderService.mjs:14-18` | backend.md | Read source file, verify `resultBytes` → `resultJson` decode gap |
| Route conflict: `/dataset/:id` shadows `/dataset/:blockchainName/:name` | frontend.md | Check `router/index.js:59-63` vs `:125-128` |
| Fabric connection leak: new gateway per call, no `gateway.close()` | backend.md | `chaincode.mjs:61-86` |
| Passwords exposed in debug-level log | backend.md | `usersTable.mjs:37` |
| `rejected` status frontend-only, not in backend validators | domain.md | Compare `TradeOrders.vue` badge list to `tradeOrdersService.mjs` valid statuses |

### 3.4 Weak Claims Needing Manual Verification

| Claim | Reason flagged |
|-------|---------------|
| "`elliptic` package never imported" (frontend.md) | Grep confirms no import in `.vue` or `.js` files, but could be transitively loaded; verify with `npm ls elliptic` |
| "Old `views/Order/` references dead API `/getBuyOrders`, `/submitSecret`, etc." (frontend.md) | Check if these are actually registered in the backend as undocumented routes |
| "Only 1 Caliper round actually ran" (benchmarks.md) | `caliper/report.html` may contain results; check file contents |

---

## 4. Contradictions Between Reports (Resolved)

| Contradiction | Resolution |
|---------------|------------|
| domain.md says 6 order statuses; chain.md says 3 | chain.md lists only on-chain states. domain.md correctly notes `rejected` is frontend-only (not in backend validator). `processing` is an implicit intermediate state. Both accurate at their layers. |
| domain.md describes cyclic delivery as "confirmed end-to-end"; frontend.md says orders are mock-only | No contradiction. domain.md describes the **protocol design**; frontend.md correctly notes the **Vue UI** uses mock data generators, not live API calls. The protocol flow is well-specified; the frontend binding is not implemented. |
| inherit.md says 15 INHERIT items; domain.md says 8 reusable ideas | inherit.md is more granular (breaks out individual algorithms, patterns, test structures). No factual conflict — different taxonomy depth. |
| Caliper `chaincodeName` mismatch (`datatrading` in deploy vs `scidatahub` in config) | `deploy-chaincode.sh:34` uses `-n datatrading` but `blockchain_config.mjs:41` defaults to `scidatahub`. The Caliper test was configured for a different chaincode deployment than the backend. This is a configuration bug, not a contradiction. |

---

## 5. Reusable Ideas / Assets (P0-P2 Priority)

### P0 — core protocol & domain (inherit logic, rewrite in Rust/Substrate)

| Item | Source files | Target |
|------|-------------|--------|
| Hash-chain settlement algorithm (`ComputeSha256Times`) | `utils.go:34-48` | `pallet_data_trade::verify_hash_chain` |
| `GenerateHashChain(N)` off-chain hash chain generation | `utils.go:23-32`, `utils.mjs:23-33` | Off-chain worker or client-side |
| 3-entity domain model (User / Dataset / Order) | `user.go`, `dataset.go`, `order.go` | Substrate `StorageMap` items |
| Token escrow pattern (lock → verify → transfer) | `user.go:98-145` | Substrate `Currency::reserve/unreserve/transfer` |
| Order state machine (`pending → completed/cancelled`) | `order.go:24-136` | Rust enum + dispatchable validation |
| 4-flag permission model (`isPublic` / `canMaskingShare` / `canCustomMaskingTrade` / `canDataService`) | `datasetsTable.mjs:12-16` | Asset visibility policy pallet |
| Dual exchange concept (trade + service) | All module pairs | Two pallet workflows within `data-trade` |

### P1 — patterns & UX (inherit structure, rewrite implementation)

| Item | Source | Notes |
|------|--------|-------|
| Masking rule configuration UI pattern | `RequestDataTrade.vue:158-297` | Field-level constraints: name, type, constraint. Well-designed UX. |
| Caliper weighted-operation workload structure | `caliper/workload/comprehensive-test.js` | Mixed scenario design for throughput benchmarks |
| Go counterfeiter mock testing pattern | `chaincode_test.go`, `mocks/` | Test chaincode logic without real network — maps to Substrate `#[test]` mock runtime |
| Jest+supertest mock-module integration-test pattern | `backend/test/**/*.test.mjs` | Service+table layer testing with mocked DB |
| IPFS MFS timestamped folder organization | `ipfs.mjs:76-84` | Upload pattern works; download must be built |
| Frontend route tree (marketplace UX) | `router/index.js:27-167` | Login → domain list → public datasets → dataset detail → trade request → order management → processing |
| Multi-domain partitioning concept | `blockchainsTable.mjs`, per-chain tables | Parachain-per-domain or configurable pallet instances |
| 4-layer backend pattern (Routes→Service→Table→db) | All `backend/src/database/` | Clean separation for Substrate RPC → pallet call layer |
| Client-side SHA-256 hashing (avoid uploading blobs) | `AddDataset.vue:57` using crypto-js | File integrity hash before upload |
| Notification toast system | `NotificationManager.js` + `NotificationToast.vue` | Lightweight, dependency-light implementation |

---

## 6. DO NOT INHERIT — Unsafe Blockchain Shortcuts

These patterns MUST NOT be replicated in FishboneChain. Each represents a fundamental security or architecture flaw.

### 6.1 Backend-submitted transactions under a single identity

**The problem**: All Fabric transactions use `User1@org1.example.com` — one cert, one private key, one MSP identity. There is zero per-end-user cryptographic signing. The backend is an omnipotent oracle.

**Evidence**: `blockchain_config.mjs:62-81` loads one cert+key, all gateway connections use this identity. Caliper benchmarks use `Admin@org1.example.com`.

**What FishboneChain must do instead**: Every transaction must be signed by the end-user's Substrate keypair. Use `ensure_signed(origin)` → `AccountId` for all authorization. Never use a backend proxy identity.

### 6.2 Caller identity passed as plain-text parameter

**The problem**: `bcCreateOrder` accepts `{buyer, seller}` strings from `req.body`. `bcAddUser` accepts `{username, tokenBalance}`. The chaincode trusts these unconditionally — no `GetClientIdentity()` call, no MSP check, no signature verification.

**Evidence**: `orderService.mjs:9`, `userService.mjs:9`. Chaincode `order.go:25` accepts `buyer` as a raw string. Any HTTP caller can impersonate any user.

**What FishboneChain must do instead**: Derive caller identity from the signed extrinsic origin only. Never pass identity as a function parameter.

### 6.3 Unrestricted state mutation (no access control in chaincode)

**The problem**: Every chaincode function is open to anyone who can reach the backend:
- `SetTokenBalance` — ANY caller sets ANY user's balance to ANY value
- `AddTokenBalance` — ANY caller mints unlimited tokens for ANY user
- `AddUser` — ANY caller creates users with ANY initial balance
- `UpdateOrderStatus` — ANY caller sets ANY order to ANY status, bypassing `CompleteOrder` validation entirely

**Evidence**: `user.go:78,88,18`, `order.go:87`. Zero `ctx.GetClientIdentity()` anywhere in chaincode.

**What FishboneChain must do instead**: All write operations must check `origin`. Token minting requires root/governance. Order status changes require sender == buyer/seller.

### 6.4 `localStorage` boolean auth

**The problem**: Auth state is `localStorage.setItem('isLoggedIn', 'true')` with a username string. No JWT, no session token, no cryptographic proof of identity. Trivially bypassable by setting localStorage manually. No logout invalidation.

**Evidence**: `stores/auth.js:1-28`, `router/index.js:170-180`.

**What FishboneChain must do instead**: Substrate wallet extension (Polkadot.js, Talisman) with sr25519/ed25519 signed extrinsics. Server session state if a sidecar exists (JWTs signed by the user's key).

### 6.5 Trusted backend fetches identity strings from SQLite

**The problem**: The backend bridges username/password (SQLite) → on-chain actions using the backend's Fabric identity. There is no cryptographic binding between the human user and the Fabric transaction. The entire chain is a write-only audit log from a single trusted submitter.

**What FishboneChain must do instead**: Either (a) eliminate the backend entirely (user wallets talk directly to Substrate), or (b) the backend is a read-only indexer/event listener.

### 6.6 SQLite as authoritative data store, chain as append-only clone

**The problem**: Rich metadata (permissions, IPFS CIDs, masking rules, service configs) lives exclusively in SQLite. The chain only stores minimal hash+owner. There is no synchronization mechanism. The SQLite DB is the *real* data store; the chain is a demo ornament.

**What FishboneChain must do instead**: On-chain storage for critical metadata (permissions, CIDs, order state). Off-chain (IPFS/Arweave) for large blobs, with on-chain hash anchors. Never rely on a mutable centralized DB as the authority for "on-chain" data.

### 6.7 Demo data as production patterns

**The problem**: `InitLedger` seeds 25 hardcoded hex hashes, `datasetsDemo.mjs` seeds 5 fake IPFS CIDs, frontend `generateMockTradeOrders()` renders 4 scenario-based demo orders. These are not clean genesis configs — they're mixed into application code.

**What FishboneChain must do instead**: Use genesis config or migration pallets for initial state. Keep demo/test data isolated in test fixtures.

---

## 7. Migration Notes for FishboneChain

### 7.1 What to rewrite (architecture migration)

| SciDataHub Layer | FishboneChain Target | Priority |
|------------------|---------------------|----------|
| Fabric chaincode (Go) | Substrate pallets (Rust) | P0 |
| Express REST API | Substrate RPC + events | P0 |
| SQLite off-chain DB | Substrate `StorageMap` + IPFS anchors | P0 |
| `localStorage` auth | Substrate wallet keypair signing | P0 |
| `caliper/` benchmarking | `frame-benchmarking` | P1 |
| Vue.js SPA UI patterns | FishboneChain dApp (framework TBD) | P2 |

### 7.2 Pallet mapping

| SciDataHub chaincode fn | FishboneChain pallet extrinsic |
|--------------------------|-------------------------------|
| `AddUser/GetUser` | `pallet_balances` (native AccountId) or custom `data-registry` |
| `SetTokenBalance/AddTokenBalance` | `pallet_balances::set_balance` (root) or `assets::mint` |
| `AddDataset/GetDataset` | `data-registry::register(origin, hash, cid, permissions)` |
| `CreateOrder` | `trade-session::create_order(origin, dataset_hash, hash_chain_end, token_unit)` |
| `CompleteOrder` | `trade-session::complete(origin, order_id, preimage)` → verify hash chain → transfer |
| `transferTokens` | `pallet_balances::transfer` or escrow pallet |
| `InitLedger` | Genesis config |

### 7.3 Key design divergences from SciDataHub

1. **Identity**: Substrate `AccountId32` replaces plain-text username strings. All authorization via `ensure_signed(origin)`.
2. **Storage**: On-chain `StorageMap` with bounded vectors, not mutable SQLite. Large blobs to IPFS with CID anchors on-chain.
3. **Payment**: Native `pallet_balances` with `reserve`/`unreserve` replaces the custom integer-based token with no supply cap.
4. **Escrow**: Add timeout-based refund and dispute resolution. SciDataHub has neither.
5. **Service orders**: Bring fully on-chain, or use ZKVM proofs verified by pallet. Do not replicate the SQLite-only pattern.
6. **Permission model**: Extend the 4-flag system with `canResell`, `canDerive`, `expiresAt`.
7. **Hash chain → ZK upgrade**: Keep the hash-chain concept as an optional payment mode; add ZK verifier integration for trustless verification.

### 7.4 What is safe to reference (no code to copy)

- The 4-flag permission taxonomy
- The trade/service dual-mode concept
- The hash-chain preimage reveal protocol (algorithm only, not the broken bridge)
- The order FSM state transitions
- The masking rule JSON schema (keyName, keyType, constraintType)
- The Caliper workload design patterns (rate control, weighted operations)
- The frontend route UX flow

---

## 8. Final Checklist for the Main Codex Agent

### Verify before trusting

- [ ] Manually inspect `backend/src/services/chaincode/orderService.mjs:14-19` — confirm `resultJson` is undefined
- [ ] Test `GET /getPublicDatasets` in backend — confirm it queries `datasets_undefined`
- [ ] Grep `frontend/src/` for `import.*elliptic` — confirm zero usage
- [ ] Check if `/getBuyOrders`, `/getSellOrders`, `/submitSecret`, `/getKey`, `/submitKey`, `/handleOrder`, `/mint`, `/burn` exist in backend route registrations
- [ ] Read `caliper/report.html` for actual benchmark results beyond `SetTokenBalance`
- [ ] Check `frontend/src/router/index.js:125-128` vs `:59-63` — confirm route shadowing
- [ ] Verify `datatrading` vs `scidatahub` chaincode name mismatch in Caliper deploy script
- [ ] Check if `init-contract-benchmark.yaml` exists anywhere (referenced by `run-tests.sh` but reported missing)

### Inherit decisions for FishboneChain

- [x] **INHERIT algorithm, REWRITE in Rust**: hash-chain settlement, `ComputeSha256Times`, `GenerateHashChain`
- [x] **INHERIT domain model, REWRITE as pallet storage**: User/Dataset/Order structs, 4-flag permissions
- [x] **INHERIT pattern, REWRITE implementation**: token escrow, order FSM, dual exchange, masking rules schema, IPFS MFS folders
- [x] **INHERIT UX concepts**: marketplace route tree, permission badges, masking rule builder UX
- [x] **DISCARD entirely**: Fabric gateway, Fabric test-network, SQLite layer, localStorage auth, Material Kit template, demo seed data, hardcoded IPs, asset-transfer-basic samples

### DO NOT INHERIT (hard block)

- [x] Single-proxy identity model (one Fabric cert for all users)
- [x] Plain-text caller identity parameters (buyer/seller/username strings)
- [x] No access control chaincode (zero `GetClientIdentity`, zero MSP checks)
- [x] Unrestricted token minting (`SetTokenBalance`, `AddTokenBalance`)
- [x] `UpdateOrderStatus` bypass of `CompleteOrder` verification
- [x] `localStorage` boolean auth
- [x] SQLite as authoritative data store with chain as append-only clone
- [x] Backend-owned wallet trust delegation

### Open design questions for FishboneChain maintainers

1. **Hash-chain vs ZK**: Should `trade-session` include a fallback SHA-256 path for lightweight clients, or go ZK-only?
2. **Single vs two-phase settlement**: Should `CompleteOrder` remain a single atomic extrinsic (verify + settle), or split into two-phase commit (verify → settle)?
3. **Service orders on-chain**: Should service orders be fully on-chain, or can they use an off-chain execution model with on-chain ZK verification?
4. **Cyclic delivery**: Is the 10-round protocol needed, or is a simpler 2-round commit/reveal (escrow + timeout) sufficient?
5. **Backend elimination**: Should the backend become a read-only event indexer, or be eliminated entirely in favor of direct wallet-to-chain communication?

---

## 9. Follow-up Files

**Must-read before migration decisions**:
- `references/SciDataHub/backend/src/services/chaincode/orderService.mjs` — verify argument-count bugs
- `references/SciDataHub/blockchain/sci-data-trade/chaincode-go/chaincode/order.go` — understand `CompleteOrder` logic
- `references/SciDataHub/blockchain/sci-data-trade/chaincode-go/chaincode/utils.go` — port `ComputeSha256Times` to Rust
- `references/SciDataHub/blockchain/sci-data-trade/chaincode-go/chaincode/user.go` — understand token escrow flow
- `references/SciDataHub/frontend/src/views/dataset/dataTrade/RequestDataTrade.vue` — masking rule builder UI reference
- `references/SciDataHub/backend/src/database/datasets/datasetsTable.mjs` — 4-flag permission schema

**Git history probes suggested** (for provenance):
```bash
git -C references/SciDataHub log --oneline -- blockchain/sci-data-trade/chaincode-go/chaincode/order.go
git -C references/SciDataHub log --oneline -- backend/src/services/chaincode/orderService.mjs
```
