# 数据交易场景实现记录

数据交易场景作为 CDT 的第一阶段工程实现，部署在专用数据交易子链 (child6)，并通过 `SceneKind::DataTrade` 与 `SettlementMode::MainEscrow` 声明身份。

## 当前实现 (2026-06-16 更新)

### 架构总览

```
┌──────────────────────────────────────────────────────┐
│ main chain (role-main)                                │
│   pallet-main-escrow  — DR lockFunds / DO lockDeposit │
│                       — settleByPreimage              │
│                       — punishDataOwner               │
│                       — claimLastPayment              │
├──────────────────────────────────────────────────────┤
│ child6 (scene-data-trade)                             │
│   pallet-data-registry  — DO publish listing          │
│   pallet-trade-session  — VC state machine            │
│     • VerifierAuthority attestation (Charlie=dev)     │
│     • ZK proof digest binding                         │
│     • DR retains dispute right after attestation      │
├──────────────────────────────────────────────────────┤
│ off-chain                                              │
│   fishbone-zk (Go CLI)  — gnark CH/RO proof gen/verify│
│   scripts/data_trade_flow.js  — 3 scenario E2E        │
│   scripts/bridges/data_trade.js  — observer/coordinator│
└──────────────────────────────────────────────────────┘
```

### pallet-data-registry (DC)

完整 listing 管理：

- 字段：`owner`、`imt_root`、`description`、`price_per_round`、`max_rounds`、`deposit_hint`、`request_schema_hash`、`proof_params_hash`
- 状态：`Active` / `Suspended` / `Retired`
- 校验：price/rounds/deposit 非零
- 提供 `ListingProvider` trait 供 `trade-session` 查询 listing

### pallet-trade-session (VC)

完整论文 VC 状态机 (13 extrinsics)：

| Extrinsic | 调用者 | 说明 |
|-----------|--------|------|
| `create_session` | DR | 绑定 listing + escrow，验证 listing 存在且 Active |
| `accept_session` | DO | 接受交易 |
| `open_round` | DR | 开启轮次，提交 payment commitment |
| `submit_payment_proof` | DR | DR 提交付款证明 |
| `submit_data_proof` | DO | 提交 ZK proof metadata（10 参数含 proof digest + vk hash + public input hash；pallet 重算 digest 校验绑定） |
| `attest_data_proof` | Verifier | 授权验证者提交 attestation（6 参数含 attestation hash；pallet 校验权限 + attestation payload） |
| `submit_proof_signature` | DR | DR 签名（需 `DataProofVerified`） |
| `submit_data_delivery_hash` | DO | 交付数据 |
| `submit_payment_preimage` | DR | DR 付款 |
| `claim_settlement` | DO | 声明结算（校验 `completed_rounds` 防超领） |
| `dispute_invalid_proof` | DR | 争议无效 proof（verifier accept 后仍可调用） |
| `dispute_invalid_plaintext` | DR | 争议 hash 不匹配 |
| `claim_last_payment` | DO | DR 拒付时的救济 |

ZK 绑定字段（`RoundState` 新增 8 个字段）：`proof_system`、`constraint_kind`、`ro_depth`、`ch_proof_hash`、`ro_proof_hash`、`public_input_hash`、`vk_hash`、`verifier_attestation_hash`

Proof digest 校验：pallet 用 `compute_zk_proof_digest` 重算 digest，确保 proof 绑定到 session/round/request/vk/hashes，不匹配返回 `InvalidProof`。此函数的 byte encoding 与 Go (`tools/data-trade-zk`) 和 Node.js (`scripts/lib/zk_artifact.js`) 层完全一致。

Attestation payload 校验：pallet 用 `compute_zk_attestation_digest` 重算，确保 attestation 绑定到 session/round/digest/accepted/verifier_account，不匹配返回 `InvalidAttestation`。

### pallet-main-escrow (Fund)

主链锁资/押金 pallet（部署于 main chain，`role-main` runtime）：

- `open_escrow`：DR 创建 escrow（记录 trade terms + hash_chain_anchor）
- `lock_funds`：DR reserve `max_rounds × price_per_round`
- `lock_deposit`：DO reserve deposit
- `settle_by_preimage`：DO 用 hash chain preimage 结算（验证 preimage → 按轮付款给 DO → 退款给 DR → 释放 DO 押金）
- `punish_data_owner`：DR slash DO deposit
- `claim_last_payment`：DO 在 DR 拒付时 claim 一轮付款

### gnark ZK 工具链

| 组件 | 技术 | 功能 |
|------|------|------|
| `tools/data-trade-zk/` | Go 1.23 + gnark v0.11 | 稳定 CLI + artifact schema |
| `fishbone-zk fixture` | Groth16 BN254 | 生成 range CH proof + depth=10 RO proof |
| `fishbone-zk verify` | Groth16 BN254 | 验证 proof + artifact digest |
| `scripts/lib/zk_artifact.js` | Node.js | 读取 artifact、计算/校验 proof_digest |
| `scripts/lib/zk_attestation.js` | Node.js | 计算 attestation payload digest |
| `scripts/lib/zk_verifier_client.js` | Node.js | 调用外部 verifier CLI（artifact 模式 + legacy 模式） |

Go ↔ JS ↔ Rust 三层的 `proof_digest` 和 `attestation_digest` 使用统一的 canonical byte encoding，逐字节一致。

### Verifier Authority

- `VerifierAuthority = Charlie`（dev 模式，`//Charlie` development key）
- `//Charlie` 已加入 `development_config_genesis()` endowment
- `//Charlie` addressRaw 用于 attestation payload 中的 verifier account 编码

### E2E 脚本

| 脚本 | 模式 | 状态 |
|------|------|------|
| `scripts/data_trade_flow.js` | `verifier=dev-attested`（Charlie 签名 attestation） | ✅ VM 验证通过（3/3 scenarios） |
| `scripts/zk_attested_data_trade_flow.js` | `verifier=dev-zk-attested` | ✅ VM 验证通过 |
| `scripts/zk_real_data_trade_flow.js` | `verifier=gnark-groth16-bn254` | ✅ VM 验证通过（需 `fishbone-zk` CLI 在 PATH 中） |
| `scripts/bridges/data_trade.js` | 观察器 + `--execute --dev-keys` 协调器 | ✅ 含 session-escrow binding 校验 |
| `scripts/run_data_trade_vm_regression.sh` | clean redeploy + base/dev-zk/real-zk 一键回归 | ✅ VM 验证通过，输出 `target/data-trade-vm-regression/summary.md` |

VM E2E 结果（main @ `ws://10.2.2.11:9944`，child6 @ `ws://10.2.2.11:9950`，2026-06-16）：

| Scenario | 结果 |
|----------|------|
| `happy` | ✅ 2 轮交付 → DO claim → settleByPreimage → DO 获付款、DR 退款、押金释放 |
| `invalid-proof` | ✅ DO submitDataProof → DR dispute → session Punished → punishDataOwner |
| `requester-refuses-payment` | ✅ DR 不付最后一轮 → DO claimLastPayment → 释放 1 轮给 DO |
| `dev-zk-attested` | ✅ ZK digest + Charlie attestation → 2 轮交付 → claimSettlement → settleByPreimage |
| `gnark-groth16-bn254` | ✅ 每轮链下生成并验证 gnark proof → 链上 proof digest/attestation → claimSettlement → settleByPreimage |

手动 VM E2E 回归结果（2026-06-16 最终）：

- `scripts/data_trade_flow.js --scenario happy` → ✅
- `scripts/zk_real_data_trade_flow.js` (business witness, gnark proof) → ✅
- `verifier=gnark-groth16-bn254`，Bob/Alice reserved 均为 0

### Stage 3: 多子链 Profile（2026-06-17）

- `scripts/lib/trade_profile.js` 提供 profile loader，支持 `--profile <id>` 参数。
- `scripts/profiles/chains.json` 的 `trade_profiles` key 定义每条链的 RPC、settlement、verifier、proof 参数。
- 当前已有两个 profile：`child6-data-trade`（`ws://10.2.2.11:9950`）和 `child7-business-trade`（`ws://10.2.2.11:9951`）。
- `gen_child_specs.py` 支持 `template_chain_id` — child7 复用了 child6 的 DataTrade runtime preset，然后通过 `inject_spec_identity` 覆盖 `name` 和 `id`。
- child7 命名已确认为 `Fishbone Child-7 (Business Data Trade, AURA-5)`。
- `deploy/fishbone/config.py` 的 `NodePeerIds` 已从固定 dataclass 改为 `dict[str, str]`，不再需要为每条新链添加代码。
- `deploy/fishbone/service.py` 添加了 child7 label。

### 边界与限制

- **ZK 验证路径**：`DataTradeProofVerifier = AlwaysPassVerifier`。Groth16 验证在链下 CLI (`fishbone-zk verify`) 完成，链上只验证 attestation 签名和 digest 绑定。
- **业务 witness (Stage 2.2 complete)**：`BusinessRangeProof` 电路已实现，gnark 证明 `raw_value ∈ [min, max]` + `masked_value = raw_value + delta` + `masked_value_hash = MiMC(masked_value, salt)`。`business_input_hash` 包含 `raw_value`、`min/max`、`mask_delta`、`salt`、`masked_value_hash` 的 canonical LE 编码，绑定到 `proof_digest` 和链上 attestation。RO/IMT、subset/substr、链上 verifier、trustless bridge 仍是后续工作。
- **Bridge 非 trustless**：session-escrow 绑定由 E2E/bridge 脚本在链下校验，未接入 CCMC/Merkle proof 做链上跨链验证。
- **Settlement 模式**：仅实现 `MainEscrow`。`FmcAssisted` 和 `Hybrid` 预留为后续。
- **单 verifier**：`VerifierAuthority` 是单一 dev 账户（Charlie），不是多签委员会。

### 安全模型与论文对齐

安全模型与论文对齐见 `docs/architecture/data-trade-security-model.md`。当前实现是链下 gnark proof verification + 链上 verifier attestation，不是链上 Groth16 verifier；bridge/session-escrow 仍是开发期链下协调。

## 测试状态

```bash
# Rust unit tests (40 total)
SKIP_WASM_BUILD=1 cargo test -p pallet-data-registry    # 12 passed
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session    # 19 passed
SKIP_WASM_BUILD=1 cargo test -p pallet-main-escrow      # 9 passed

# Go tests
go -C tools/data-trade-zk test ./...                     # artifact + gnarkadapter passed

# JS syntax checks (all pass)
node --check scripts/data_trade_flow.js
node --check scripts/zk_attested_data_trade_flow.js
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/bridges/data_trade.js
node --check scripts/lib/zk_artifact.js
node --check scripts/lib/zk_attestation.js
node --check scripts/lib/zk_verifier_client.js
node --check scripts/lib/data_trade_events.js
node --check scripts/lib/data_trade_binding.js
node --check scripts/lib/hash_chain.js
node --check scripts/lib/data_trade_sample.js
node --check scripts/lib/vm_regression_summary.js
node --check scripts/lib/wait_for_ws_chain.js
bash -n scripts/run_data_trade_vm_regression.sh
```

## VM 部署

```bash
# 构建
make build-main                    # role-main + MainEscrow → deploy/bin/fishbone-node
make build-data-trade-child        # scene-data-trade → deploy/bin/fishbone-node-data-trade

# 重新生成 spec（runtime 变更后必须）
python3 scripts/gen_child_specs.py --only child6

# 部署
bash scripts/dev_redeploy_clean_chains.sh --chains main,child6 --config deploy/config.toml --logs

# 验证 metadata
node -e '...'  # main 含 MainEscrow, child6 含 DataRegistry+TradeSession, 不含 Crowdsource

# 一键回归（会 clean redeploy main+child6）
MAIN_WS=ws://10.2.2.11:9944 CHILD_WS=ws://10.2.2.11:9950 scripts/run_data_trade_vm_regression.sh
```

## 后续方向

- Trustless 跨链证明：CCMC/Merkle proof 接入主链 escrow
- 多 verifier 委员会 + 阈值 attestation
- FmcAssisted / Hybrid 结算模式
