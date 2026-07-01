Now let me read the key source files for detailed analysis.
Now I have all the evidence needed. Here is the audit report:

---

# SciDataHub Frontend Audit — User-Facing Product Flows

## 1. Scope Inspected

```
frontend/src/
├── api/axios.js
├── router/index.js
├── stores/auth.js
├── components/NavbarDefault.vue (+ 13 Material UI wrappers, Notification system)
├── examples/cards/ (12 themed card components)
├── views/
│   ├── Home/HomeView.vue
│   ├── Login/LoginView.vue
│   ├── Finance/FinanceView.vue
│   ├── blockchain/BlockchainList.vue
│   ├── dataset/ (AddDataset, DatasetView, DatasetEdit, MyDatasets, PublicDatasets,
│   │            dataTrade/RequestDataTrade, dataService/RequestDataService)
│   ├── Market/ (MarketView, CreateOrder)
│   ├── Order/ (BuyView, SellView, BuyOrder, SellOrder)
│   └── orders/ (TradeOrders, TradeOrderDetail, TradeOrderProcessingDetail,
│                ServiceOrders, ServiceOrderDetail, ServiceOrderProcessingDetail)
└── package.json (Vue 3 + Pinia + Vue Router + Material Kit 2 template)
```

Backend API dependency: Express server at `http://192.168.8.22:3101` (hardcoded IP in `api/axios.js:5`).

---

## 2. Confirmed Facts with Evidence

### 2a. Tech Stack
- **Vue 3** (Composition API with `<script setup>`), **Pinia** (single store), **Vue Router 4**, **Vite** (`frontend/vite.config.js`)
- **UI framework**: Creative Tim "Vue Material Kit 2" template — wholesale copy of the commercial Bootstrap-based theme (`frontend/src/App.vue:1-15` copyright header, ~100 SCSS files in `assets/scss/`)
- **Dependencies with no frontend code usage**: `elliptic`, `vue-clipboard3`, `vue-count-to`, `typed.js`, `prismjs`, `vue-prism-editor`
- **Dependencies actually used**: `axios`, `bootstrap`, `pinia`, `vue-router`, `crypto-js` (SHA256 only), `@popperjs/core`

### 2b. Auth / Session Model
- **Store** (`stores/auth.js:1-28`): Only `isLoggedIn` (boolean from localStorage) and `username` (string from localStorage). No session token, no JWT, no key material.
- **Login flow** (`views/Login/LoginView.vue:82-121`): POST `/login` or `/register` to backend with plaintext username+password. On success, persists `username` to localStorage.
- **Route guard** (`router/index.js:170-180`): Checks `authStore.isLoggedIn` which reads `localStorage.getItem('isLoggedIn')`. Trivially bypassable by setting localStorage manually.
- **No logout invalidation**: `logout()` just clears localStorage client-side.

### 2c. Router / Navigation Structure
22 routes defined at `router/index.js:29-167`. All require auth except `/` (home) and `/login`.

**Navbar menu** (`components/NavbarDefault.vue:37-79`) exposes 6 navigation items:
1. 区块链列表 → `/blockchainList`
2. 我的科研数据集 → `/mydatasets/:username`
3. 数据交易订单 → `/tradeOrders/Physics` (Physics hardcoded)
4. 数据服务订单 → `/serviceOrders/Physics` (Physics hardcoded)
5. 资金管理 → `/finance`
6. Login/Logout button

### 2d. Two Competing Order UIs (Evidence of Forked Development)
- **`views/Order/` (old/deprecated pattern)**: BuyView, SellView list order cards via `/getBuyOrders`, `/getSellOrders`. BuyOrder, SellOrder are detail pages that submit encryption keys to `/submitSecret`, `/getKey`, `/submitKey`, `/handleOrder`. This API set appears to be from a now-removed Hash Time-Locked Contract (HTLC) flow.
- **`views/orders/` (newer pattern)**: TradeOrders, ServiceOrders, TradeOrderDetail, etc. These use hardcoded `generateMockTradeOrders()` / `generateMockOrder()` generating entirely static demo data with no API calls. `TradeOrders.vue:487-554` contains 4 domain-specific research scenarios (physics/medicine/cybersecurity/biology) with detailed fictional masking rules.

### 2e. Local Cryptography Behavior
**Actual usage of `crypto-js` is limited to 3 locations:**

| File | Line | Usage |
|---|---|---|
| `AddDataset.vue:57` | SHA256 of file bytes for content hash | Legitimate (file integrity) |
| `CreateOrder.vue:53` | SHA256 hash chain for "payment proof" | Protocol concept, broken |
| `BuyOrder.vue:6`, `SellOrder.vue:6` | `import CryptoJS from 'crypto-js'` | Dead import, never called |

**Key finding: `elliptic` is listed in `package.json` but never imported in any `.vue` or `.js` file.** There is zero ECDSA key generation, zero transaction signing, zero wallet integration on the frontend.

### 2f. Missing Store Fields
`authStore.publicKey` is referenced in 3 files (`FinanceView.vue:144`, `BuyView.vue:57`, `SellView.vue:57`), and `store.privateKey` in `CreateOrder.vue:50`. Neither field exists in `stores/auth.js`. These would resolve to `undefined` at runtime — all 4 views are non-functional.

---

## 3. Gaps, Demo Shortcuts, and Correctness Risks

### 3a. Mock Data / No Real API Integration
| View | Mock Status |
|---|---|
| `TradeOrders.vue` | `loadOrders()` generates 4 hardcoded orders with `await new Promise(resolve => setTimeout(resolve, 800))` — no API call at all |
| `TradeOrderDetail.vue` | Same pattern, `generateMockOrder()` produces fake data with `setTimeout(800)` |
| `TradeOrderProcessingDetail.vue` | Not read in detail, but filename suggests same mock approach |
| `ServiceOrders.vue` | Same pattern (has `serviceType` filter, mock data generator `generateMockServiceOrders`) |
| `ServiceOrderDetail.vue` | Same mock approach |
| `ServiceOrderProcessingDetail.vue` | Same mock approach |

**Verdict**: The entire post-v2 "orders" module is pure UI scaffolding with no backend integration.

### 3b. Hardcoded Blockchain Reference
`NavbarDefault.vue:57-68` routes to `/tradeOrders/Physics` and `/serviceOrders/Physics`. The `Physics` blockchain name is hardcoded; there is no mechanism for the user to select which blockchain's orders to view.

### 3c. Broken Auth State
`FinanceView.vue:144` reads `authStore.publicKey` — undefined. The `/getUser`, `/mint`, `/burn` endpoints receive `uID` as `undefined`. This view cannot possibly work.

### 3d. IPFS Download is a Generic Gateway Link
`DatasetView.vue:228`: `window.open('https://ipfs.io/ipfs/${dataset.value.maskingDatasetIPFSAddress}', '_blank')` — expects the `maskingDatasetIPFSAddress` field to be a bare CID. No actual IPFS client integration; just opens the public gateway.

### 3e. "Signature" Field is Fake
`AddDataset.vue:111`: `dataset.value.signature = `${generateRandomString(64)}`` — generates a random 64-char string, not a cryptographic signature. No public/private key involved.

### 3f. Hash Chain Payment Proof is Broken
`CreateOrder.vue:50-56`: Client-side SHA256 chain generation using `store.privateKey` (undefined). The seller-side order processing references `store.publicKey` (undefined). This is an incomplete implementation of a hash-chain micropayment protocol.

### 3g. Route Conflict
`router/index.js:125-128` defines `/dataset/:id` → `CreateOrder`, which shadows `/dataset/:blockchainName/:name` → `DatasetView` (line 59-63). Navigating to `/dataset/anything` will match the `:id` route first.

---

## 4. Reusable Ideas / Assets

### 4a. Domain Concepts Worth Migrating
1. **Two-role user model** (data owner vs. data requester) — login page has demo buttons for both roles (`LoginView.vue:50-51`). This binary actor model maps well to FishboneChain's data-trade direction.
2. **Multi-blockchain dataset registry** — `AddDataset.vue` lists available blockchains via `/getblockchains` and lets users choose where to register a dataset. Multi-chain awareness is a useful UI pattern.
3. **Dataset public-level controls** — `DatasetView.vue` shows badges for `isPublic`, `canMaskingShare`, `canCustomMaskingTrade`, `canDataService`. These four permission axes are a useful mental model for data access tiers.
4. **Masking rule configuration UI** — `RequestDataTrade.vue:158-297` has a clean UI for building per-field masking constraints (key name, data type, constraint type: equals/contains/range/min/max). The dynamic constraint type switching UX is well done.
5. **Order status workflow** — The state machine (`pending → processing → completed`) with role-based action visibility provides a good interaction model for data trade negotiations.

### 4b. UI Components Potentially Reusable
- The `RequestDataTrade.vue` masking rule builder (the dynamic form logic for per-field constraints)
- Notification toast system (`NotificationManager.js` + `NotificationToast.vue`) — clean, dependency-light toast implementation

### 4c. What to Explicitly Avoid
- The Material Kit 2 template (~100 SCSS files) — adds massive weight for minimal value
- The `elliptic`/`crypto-js` hash-chain payment flow — incomplete, fundamentally broken, and would need a full redesign for Substrate
- The localStorage-based auth — no session management
- The `orders/` module — 100% mock data, no usable integration logic
- The hardcoded IP `192.168.8.22:3101` pattern

---

## 5. Migration Notes for FishboneChain

### 5a. Structural Recommendations
1. **Auth**: Replace the localStorage auth with Substrate wallet integration (Polkadot.js extension or similar). The concept of a "user" maps to a Substrate account.
2. **API client**: Point at a Substrate node RPC or a thin backend that wraps Substrate extrinsics. The axios instance pattern is fine but needs auth header injection.
3. **Multi-chain awareness**: The blockchain-selector dropdown in `AddDataset.vue` is a reusable pattern, but the list should come from the runtime (parachain IDs, relay chain) rather than a SQLite backend.
4. **Dataset registration**: The form in `AddDataset.vue` (name, description, file hash, blockchain selection) maps cleanly to a Substrate pallet that stores dataset metadata. The signature field should be replaced with an actual extrinsic signature.
5. **Order/trade UI**: The masking rule builder in `RequestDataTrade.vue` is the most valuable piece. It should feed into a pallet call rather than a REST POST.

### 5b. Technology Mapping
| SciDataHub | FishboneChain Equivalent |
|---|---|
| Axios → Express backend | Polkadot.js API → Substrate node / sidecar |
| Pinia `authStore` (localStorage) | Wallet extension + account state |
| SQLite-backed datasets | Substrate pallet storage (on-chain metadata) |
| Hash-chain payment (broken) | Native token transfer or pallet-specific escrow |
| IPFS gateway link | IPFS CID stored on-chain, fetched via gateway or local node |
| Custom backend auth | SR25519/ED25519 signed extrinsics |

### 5c. Files Worth Adapting
- `views/dataset/dataTrade/RequestDataTrade.vue` — masking rule builder form logic
- `views/dataset/DatasetView.vue` — dataset detail + action button layout pattern
- `components/NotificationManager.js` — lightweight toast system
- The permission-axis badge concept (`isPublic`, `canMaskingShare`, `canCustomMaskingTrade`, `canDataService`)

### 5d. Files NOT Worth Porting
- `views/orders/` — 6 files of pure hardcoded mock data
- `views/Order/` — old orders module referencing a dead API
- `views/Finance/FinanceView.vue` — broken (references undefined store fields)
- `views/Market/CreateOrder.vue` — broken hash-chain payment flow
- `components/NavbarDefault.vue` — hardcoded Physics blockchain, no dynamic chain selection
- `examples/` — all 12 card components are theme-specific

---

## 6. Follow-up Files / Questions

### 6a. Files to Investigate Further
- `frontend/src/views/orders/ServiceOrderDetail.vue` and `ServiceOrderProcessingDetail.vue` — likely same mock-data pattern as TradeOrder equivalents; spot-check to confirm
- `frontend/src/views/dataset/DatasetEdit.vue` — not inspected in detail; may contain usable edit-form patterns
- `frontend/src/views/dataset/MyDatasets.vue` and `PublicDatasets.vue` — dataset listing pages; not read in detail
- `frontend/src/views/dataset/dataService/RequestDataService.vue` — data service request form; not read in detail

### 6b. Open Questions
1. Does the backend actually implement `/getBuyOrders`, `/getSellOrders`, `/submitSecret`, `/getKey`, `/submitKey`, `/handleOrder`, `/mint`, `/burn`? The frontend calls them but the views referencing them are likely non-functional.
2. Was the `elliptic` dependency ever implemented and later removed, or was it always dead weight?
3. What IPFS node configuration was intended? The backend has `kubo-rpc-client` but the frontend only uses a public gateway URL.
4. The two-order-UI pattern (`Order/` vs `orders/`) suggests a rewrite was attempted but never completed. Which version was presented in demos/screenshots?
