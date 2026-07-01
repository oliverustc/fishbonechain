# SciDataHub Inheritance Review

Date: 2026-07-01

This is the maintainer-facing review of the opencode-generated SciDataHub audit
reports under `docs/internal/reference-audits/scidatahub/reports/`. I spot-checked
the highest-risk claims against source files in `references/SciDataHub`.

## Bottom Line

SciDataHub should be treated as a product/reference prototype, not as code to
port. The useful inheritance is mostly domain design and protocol ideas:

- hash-chain based proportional settlement;
- User / Dataset / Order marketplace model;
- dataset permission flags;
- trade-order and service-order distinction;
- masking-rule request UX;
- benchmark workload shapes.

The implementation must be rewritten for FishboneChain. In particular, the
Fabric backend-submitted transaction model is not acceptable for a real
blockchain system.

## Verified Critical Findings

### Single Backend Fabric Identity

`backend/src/config/blockchain_config.mjs` defaults every gateway call to the
same Fabric test-network identity:

- `MSP_ID=Org1MSP`;
- `User1@org1.example.com` certificate path;
- `User1@org1.example.com` private-key path.

There is no per-user blockchain identity. Backend handlers accept usernames,
buyers, sellers, and owners as plain strings.

FishboneChain must replace this with `ensure_signed(origin)` and Substrate
`AccountId`; caller identity must never be passed as a normal request field.

### No Chaincode Access Control

The real chaincode files under
`blockchain/sci-data-trade/chaincode-go/chaincode/` do not call
`GetClientIdentity()`. The only `GetClientIdentity` hits are generated mocks.

Dangerous functions are unrestricted:

- `SetTokenBalance`;
- `AddTokenBalance`;
- `UpdateOrderStatus`;
- `CreateOrder`;
- `AddDataset`.

This confirms SciDataHub's chain is an append-only demo state machine behind a
trusted backend, not a permissioned application with real user authorization.

### Settlement Path Is Broken End-To-End

`backend/src/services/chaincode/orderService.mjs` calls:

```js
contract.submitTransaction('CompleteOrder', orderID)
```

but `chaincode/order.go` defines:

```go
CompleteOrder(ctx, orderID string, preImage string)
```

The required `preImage` is not sent, so the backend bridge cannot settle orders
as implemented. The same backend file also references undefined `resultJson` in
`bcCreateOrder` after receiving `resultBytes`.

The hash-chain algorithm is still worth inheriting, but only after independent
Rust tests and weight benchmarking.

### SQLite Is The Real Authority

Important data lives only in SQLite:

- dataset metadata and permission flags;
- IPFS address;
- masking rules;
- service order config;
- service order status.

The chaincode only stores minimal `User`, `Dataset`, and `Order` records.
Service orders have no chaincode counterpart at all.

FishboneChain should put critical state in pallet storage or content-addressed
metadata anchored on-chain, not in a mutable backend database.

### Prototype Shortcuts Are Pervasive

Verified examples:

- `backend/src/app.mjs` drops/recreates database tables on every startup.
- `frontend/src/stores/auth.js` stores auth as `localStorage.isLoggedIn`.
- `frontend/src/api/axios.js` hardcodes `http://192.168.8.22:3101`.
- `backend/src/services/ipfs.mjs` upload mostly works, but download is an
  acknowledged TODO.
- frontend order screens include mock-only flows and undefined key fields.

## Inherit

### P0: Protocol And Pallet Design

1. Hash-chain settlement algorithm

Source: `chaincode/utils.go`, `chaincode/order.go`

Target: `pallet-trade-session` or a dedicated verifier helper.

Design notes:

- store `hash_chain_end`;
- buyer reserves funds;
- completion submits `preimage`;
- pallet iterates SHA-256 up to a bounded max;
- transfer `steps * token_unit`;
- benchmark worst-case weight.

2. Dataset permission model

Source: `datasetsTable.mjs`

Flags:

- `isPublic`;
- `canMaskingShare`;
- `canCustomMaskingTrade`;
- `canDataService`.

Target: `pallet-data-registry` asset policy fields, preferably as a compact
bitflag or bounded policy struct.

3. User / Dataset / Order model

Use the model shape, not the code. Map usernames to `AccountId`, token balances
to native balances/assets, dataset hash/CID to bounded storage, and order status
to a Rust enum.

4. Trade vs service workflows

Trade order maps to data exchange and settlement. Service order should become a
separate on-chain workflow or an off-chain job with on-chain evidence/proof.
Do not copy SciDataHub's SQLite-only service-order implementation.

### P1: UX And API Ideas

- masking-rule builder from `RequestDataTrade.vue`;
- dataset permission badges/actions;
- marketplace navigation: domain list → dataset list → dataset detail → request
  trade/service → order processing;
- notification/toast pattern if a frontend is built later.

### P2: Experiment Ideas

- weighted workload mix from Caliper scripts;
- staged read/write/query benchmark shapes;
- chaincode mock-test organization as inspiration for FRAME mock-runtime tests.

## Do Not Inherit

- Fabric gateway and test-network code.
- `asset-transfer-basic/` sample code.
- Backend-owned wallet/certificate model.
- Plain-text caller identity fields.
- SQLite as authoritative state.
- `localStorage` boolean auth.
- unrestricted mint/balance/status mutation APIs.
- fake IPFS CID generation and demo seeds.
- frontend Material Kit template as product foundation.
- mock-only order pages as implementation.

## FishboneChain Mapping

| SciDataHub concept | FishboneChain target |
| --- | --- |
| `AddDataset(hash, owner)` | `data-registry::register_dataset(origin, hash, cid, policy)` |
| dataset permission flags | `DatasetPolicy` in pallet storage |
| `CreateOrder` | `trade-session::create_order(origin, dataset_id, terms)` |
| `TransferLockedTokenBalance` | `Currency::reserve` or assets hold/freeze |
| `CompleteOrder(orderID, preImage)` | `trade-session::complete(origin, order_id, preimage)` |
| `UpdateOrderStatus` | restricted state transitions only |
| service order SQLite row | job request + evidence/proof lifecycle |
| Caliper workload | Substrate tx workload scripts |

## Immediate Follow-Up For FishboneChain

1. Add a Rust hash-chain verifier test fixture based on SciDataHub's algorithm,
   but do not import Go/JS code.
2. Decide whether hash-chain settlement is a core path or a compatibility path
   beside the existing ZK data-trade direction.
3. Extend `data-registry` design with permission flags if not already modeled.
4. Treat service orders as a fresh design problem: request, execution evidence,
   verification, timeout, dispute, and settlement.
5. Use SciDataHub frontend only as UX reference, not codebase reference.
