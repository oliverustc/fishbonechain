# 数据交易场景实现记录

数据交易场景作为 CDT 的第一阶段工程实现，部署在专用数据交易子链 (child6)，并通过 `SceneKind::DataTrade` 与 `SettlementMode::MainEscrow` 声明身份。

## 当前实现 (2026-06-15 更新)

### 已实现

- **`pallet-data-registry`** (DC)：完整 listing 管理，含 `price_per_round`、`max_rounds`、`deposit_hint`、`request_schema_hash`、`proof_params_hash`、`ListingStatus` (Active/Suspended/Retired)。
- **`pallet-trade-session`** (VC)：完整论文 VC 状态机 (12 extrinsics)：
  - `create_session`、`accept_session`
  - 轮次协议：`open_round` → `submit_payment_proof` → `submit_data_proof` → `submit_proof_signature` → `submit_data_delivery_hash` → `submit_payment_preimage`
  - 结算：`claim_settlement`
  - 争议：`dispute_invalid_proof`、`dispute_invalid_plaintext`
  - 救济：`claim_last_payment`
- **`pallet-main-escrow`** (Fund)：主链锁资/押金 pallet：
  - `open_escrow`、`lock_funds`、`lock_deposit`
  - `settle_by_preimage` (hash-chain 验证 + 按轮付款 + 退款)
  - `punish_data_owner` (slash deposit)
  - `claim_last_payment`
- **跨 pallet 集成**：
  - `data-registry` 实现 `ListingProvider` trait，供 `trade-session` 校验 listing
  - `trade-session` 使用可插拔 `DataTradeProofVerifier` trait (Phase 1: mock)
- **E2E 脚本** (`scripts/data_trade_flow.js`)：happy path + invalid-proof + requester-refuses-payment
- **Bridge** (`scripts/bridges/data_trade.js`)：观察器 + 可选 `--execute --dev-keys` 协调器
- **Runtime 配置**：`role-main` 包含 `MainEscrow`，`scene-data-trade` 包含 `DataRegistry` + `TradeSession` (不含 Crowdsource)

### 仍未实现

- 真实上链 zk-SNARK verifier（当前使用 `AlwaysPassVerifier` mock）
- CCMC/Merkle proof 接入主链 escrow（trustless 跨链桥）
- FmcAssisted / Hybrid 结算模式
- 生产环境的 bridge 安全签名

### 边界

- 数据交易场景独立于数据众包 pallet
- 第一版采用 `MainEscrow`，不调用 FMC
- ZK verifier 当前为 mock，只在协议状态机层面工作

## 测试状态

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-data-registry    # 12 passed
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session    # 12 passed
SKIP_WASM_BUILD=1 cargo test -p pallet-main-escrow      # 9 passed
```

## 后续方向

- 接入真实 ZK verifier (see `data-trade-zk-verifier-plan.md`)
- 将 trustless CCMC/Merkle proof 接入主链 escrow 验证
- 不同数据交易类型的子链 profile 和参数哈希
