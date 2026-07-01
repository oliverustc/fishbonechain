Now let me read all backend source and test files systematically.
Here is the complete audit report.

---

## Backend Architecture Audit: SciDataHub

---

### 1. Scope inspected

`references/SciDataHub/backend/` — all 29 source files (`src/`) and 14 test files (`test/`), plus `package.json`, `README.md`.

---

### 2. Confirmed facts with evidence

**Tech stack**
- Node.js ESM (`.mjs`), Express 4.x, SQLite3, `@hyperledger/fabric-gateway` 1.5, `kubo-rpc-client` 5.2.
- CORS allows all origins (`server_config.mjs:5`), listens on `:3101`.

**Architecture layers** — a clean 4-tier pattern (no ORM, manual SQL):

```
Routes (.mjs with express.Router)
  → Service (.mjs — request parsing, validation, response formatting)
    → Table (.mjs — raw SQL via db.mjs helpers)
      → db.mjs (SQLite3 connection, promisified dbRun/dbGet/dbAll)
```

Evidence:
- `usersRoutes.mjs:11-13` → `usersService.mjs` → `usersTable.mjs` → `db.mjs`
- Same pattern for blockchains, datasets, trade orders, service orders

**SQLite schemas** — single file `database/database.db`, 6 table types:
| Table | Key columns |
|---|---|
| `users` | username, password (PBKDF2 hash) |
| `blockchains` | name, fullName, description |
| `datasets_{chain}` | name, fullName, description, owner, isPublic, canMaskingShare, canCustomMaskingTrade, canDataService, hash, maskingDatasetIPFSAddress |
| `trade_orders_{chain}` | title, description, blockchainName, datasetName, datasetOwner, requester, maskingRules (JSON), status |
| `service_orders_{chain}` | title, description, blockchainName, datasetName, datasetOwner, requester, serviceType, serviceConfig (JSON), status |

The `_{chain}` suffix pattern means tables are dynamically created per blockchain name at `datasetsTable.mjs:8`, `tradeOrdersTable.mjs:9`, `serviceOrdersTable.mjs:9`.

**API routes** — 35 endpoints across 5 modules:

| Module | Routes | Methods |
|---|---|---|
| Users (`usersRoutes.mjs`) | `/register`, `/login`, `/user/username/:username` | POST, POST, GET |
| Blockchains (`blockchainsRoutes.mjs`) | `/getblockchains`, `/blockchain/:name` | GET, POST, PUT, DELETE |
| Datasets (`datasetsRoutes.mjs`) | `/:chain/addDataset`, `/:chain/datasets`, `/:chain/datasets/owner/:owner`, `/:chain/getAlldatasetsByOwner/:name`, `/:chain/getDatasetByDatasetName/:name`, `/:chain/updateDatasetInfo/:name`, `/:chain/updateDatasetPublicLevel/:name`, `/:chain/updateDatasetHash/:name`, `/:chain/updateMaskingDatasetIPFSAddress/:name`, `/getPublicDatasets` | POST, GET, DELETE |
| Orders (`orderRoutes.mjs`) | `/:chain/trade-orders`, `/:chain/service-orders` (CRUD for both) | POST, GET, PUT, DELETE |
| Chaincode (`chaincodeRoutes.mjs`) | `/bcAddUser`, `/bcGetUser`, `/bcGetTokenBalance`, `/bcSetTokenBalance`, `/bcAddTokenBalance`, `/bcTransferLockedTokenBalance`, `/bcTransferTokens`, `/bcGetDataset`, `/bcAddDataset`, `/bcGetAllDatasets`, `/bcGetDatasetOwner`, `/bcCreateOrder`, `/bcGetOrder`, `/bcUpdateOrderStatus`, `/bcCompleteOrder`, `/bcGetAllOrders` | POST, GET, PUT |

**Hyperledger Fabric integration** (`chaincode.mjs`)
- Connects via gRPC + TLS using cert material from `blockchain/test-network/organizations/` (`blockchain_config.mjs:45-85`).
- `initializeContract()` creates a fresh Fabric Gateway connection on every chaincode API call (no connection reuse).
- `initLedger()` called at startup (`app.mjs:25`), swallows errors silently (`chaincode.mjs:93-95`).

**IPFS integration** (`services/ipfs.mjs`)
- `kubo-rpc-client` connecting to `http://127.0.0.1:5001`.
- `uploadToIPFS()`: uploads file to IPFS, copies to MFS under `/my-file/{date}/` — works.
- `downloadFromIPFS()`: explicitly states at line 92: `"目前还不会如何下载这个文件，需要继续研究"` (doesn't know how to download yet). The `client.get()` call is not awaited.

**Logging** (`utils/log.mjs`)
- Pure `console.log` with ANSI color codes. Supports 4 levels (debug/info/warning/error).
- No structured logging, no log file output, no level filtering at runtime.

**Password hashing** (`usersService.mjs:10-13`)
- `pbkdf2Sync(password, salt, 1000, 64, 'sha512')`. 1000 iterations is below current standards (OWASP recommends 600,000+ for PBKDF2-HMAC-SHA256).

---

### 3. Gaps, demo shortcuts, or correctness risks

**Critical bug — undefined variable in `orderService.mjs`**
At `orderService.mjs:14-18`:
```js
const resultBytes = await contract.submitTransaction('CreateOrder', ...);
logger.info('bcCreateOrder result: ', resultJson);   // resultJson is never defined!
```
`resultBytes` is the Uint8Array output but it is never decoded into `resultJson`. This will log `undefined`. The return line at `:19` also uses `resultJson`.

**Broken endpoint — `/getPublicDatasets`**
`datasetsRoutes.mjs:19` defines:
```js
router.get('/getPublicDatasets', getPublicDatasets);
```
But `getPublicDatasets` (`datasetsService.mjs:115`) reads `req.params.blockchainName`, which is undefined at this route (there's no `:blockchainName` segment). The handler will pass `undefined` to `dbGetPublicDatasets(undefined)` and query `datasets_undefined`.

**Startup self-destructs the database every time**
`app.mjs:32-69` — drops and recreates ALL tables on every server start. The comment on line 32 notes this is for development only, but there's no environment switch to disable it.

**No authentication middleware exists**
Several service handlers check `req.user?.username` (`tradeOrdersService.mjs:88`, `:266`, `serviceOrdersService.mjs:17`, `:272`), but no middleware populates `req.user`. Login (`usersService.mjs:80-126`) returns `{success: true, user: {...}}` with no token, no session cookie, no JWT. There is no `req.user` ever set by the actual application code. The auth check in orders is dead code.

**`generateIPFSCID()` generates fake CIDs**
`utils/utils.mjs:19-21`: `"Qm" + await randStr(44)`. This produces strings that look like IPFS CIDs but aren't valid multihash-formatted CIDs.

**IPFS download is unimplemented**
`ipfs.mjs:86-97` — acknowledged in source: `"目前还不会如何下载这个文件"`. The `client.get()` return value is not handled.

**Inconsistent error logging**
- Some handlers use `logger.error()` (`datasetsService.mjs:55`), some use `console.error()` (`usersService.mjs:71`).
- `ipfs.mjs` uses `console.error()` throughout instead of the logger it imports.

**Fabric connection leak**
`chaincode.mjs:61-86`: `initializeContract()` creates a new gRPC connection and gateway on every call. No connection caching, no `gateway.close()` anywhere. This will leak connections under load.

**Hardcoded blockchain list**
`app.mjs:58`: The 5 blockchain names are hardcoded. If a user adds a new blockchain via the API, its order tables won't be created on restart.

**Missing input sanitization**
No SQL injection protection beyond parameterized queries (which are used). No XSS protection on response data. No rate limiting. No body size limits. No input type validation beyond manual checks.

**Low PBKDF2 iterations**
`usersService.mjs:12`: `pbkdf2Sync(password, salt, 1000, 64, 'sha512')`. 1000 iterations is approximately 1000x below current recommendations.

**Passwords exposed in log (debug level)**
`usersTable.mjs:37`: `logger.debug(\`创建用户: ${username}: ${password}\`)` — logs the raw password before hashing.

---

### 4. Reusable ideas/assets

| Idea | Source | Value for FishboneChain |
|---|---|---|
| **4-layer route→service→table→db pattern** | All modules | Clean separation. Adopt this pattern directly. |
| **Dynamic per-domain table naming** | `datasets/orders tables` | The `_{chainName}` suffix lets one SQLite serve multiple chains. Useful if Substrate pallets map to logical domains. |
| **JSON serialization in order columns** | `tradeOrdersTable.mjs:47`, `serviceOrdersTable.mjs:48` | `maskingRules` and `serviceConfig` stored as JSON strings with parse-on-read. Good pattern for flexible metadata. |
| **Hash chain utility** | `utils/utils.mjs:23-33` | Useful for fair-exchange protocols and commit-reveal schemes in data trading. |
| **PBKDF2 password hashing approach** | `usersService.mjs:10-13` | Correct algorithm choice, just increase iterations to 600,000+. |
| **Dataset visibility levels** | `datasetsTable.mjs:13-16` | The `isPublic`/`canMaskingShare`/`canCustomMaskingTrade`/`canDataService` boolean flags model data access tiers — directly applicable to Substrate data-trade pallet permissions. |
| **Order state machine** | `tradeOrdersService.mjs:229` | `pending → processing → completed/cancelled/failed` — reasonable state transitions to inherit. |
| **Supertest + Jest mock-module testing** | All `test/database/**/*.test.mjs` | The pattern of mocking `db.mjs` and testing through Express routes via supertest is a good integration-test-light approach. |
| **Crypto path auto-discovery** | `blockchain_config.mjs:16-38` | Walks up directories to find `blockchain/` folder. Useful for config path resolution in development. |

---

### 5. Migration notes for FishboneChain

**Rewrite, don't port.** The backend is a demo prototype. Key technical debt preventing direct reuse:

1. **Express → Substrate off-chain worker or sidecar**: The Express REST API pattern won't fit directly. Keep the API shape (routes, request/response schemas) as an OpenAPI spec, then reimplement against Substrate extrinsics and off-chain storage.

2. **SQLite → Substrate storage**: The per-chain dynamic table pattern maps naturally to pallet storage maps. For example, `datasets_{chainName}` maps to a `StorageDoubleMap<ChainId, DatasetName, DatasetInfo>`.

3. **Auth**: The missing token/session layer must be built from scratch. Consider Substrate's account-based identity model directly instead of username/password.

4. **IPFS**: Only the upload pattern is functional. The MFS copy step (`cpToMFS`) is unnecessary if CIDs are stored on-chain. For Substrate, store the CID directly in chain state and let clients fetch via any IPFS gateway.

5. **Chaincode**: All chaincode routes (`/bc*`) map to chaincode functions in `blockchain/sci-data-trade/chaincode-go/`. These would be replaced by Substrate extrinsic calls. The route/service structure is clean and could be adapted for a sidecar pattern.

6. **Dataset metadata model**: The 4 boolean flags (public, masking share, masking trade, data service) are the most reusable domain model. Inherit this taxonomy directly.

7. **Order lifecycle**: The trade order + service order distinction, with masking rules JSON, is worth preserving as a Substrate pallet design.

8. **Keep the test structure**: The Jest+supertest with mock modules pattern provides good coverage of the table and service layers. The actual test logic (assertions on SQL, status codes, JSON responses) should inform integration tests for the Substrate API.

---

### 6. Follow-up files/questions

**Files that need inspection before migration decisions:**
- `references/SciDataHub/blockchain/sci-data-trade/chaincode-go/` — the Fabric chaincode (Go) that defines the on-chain data model. The backend chaincode routes are thin proxies; the actual state logic is in Go.
- `references/SciDataHub/frontend/src/` — to verify the actual API contract used by the UI (the backend has unused/dead endpoints).
- `references/SciDataHub/ipfs/Readme.md` — IPFS deployment instructions to assess if the kubo setup is production-ready.

**Open questions:**
1. Was the chaincode (`sci-data-trade`) ever tested with Hyperledger Caliper under load? The `caliper/` directory suggests intent but no evidence of results.
2. Is there a documented data masking mechanism? The `maskingRules` field is stored as free-form JSON but no masking service code exists in the backend.
3. The `generateHashChain` utility produces 64-char hex hashes — does the chaincode actually verify hash chains for fair exchange, or is this a stub?
4. The `canDataService` flag suggests an execution/computation feature that doesn't seem implemented beyond the schema. Is this aspirational?
