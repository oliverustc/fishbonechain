# 数据交易流程运行手册

## 角色账户

| 角色 | Substrate Account | Dev URI |
|------|-------------------|---------|
| DR (Data Requester) | Alice | `//Alice` |
| DO (Data Owner) | Bob | `//Bob` |

## 链

| 链 | RPC 端口 | Runtime Profile |
|----|----------|-----------------|
| main (主链) | `9944` | `role-main` (含 MainEscrow) |
| child6 (数据交易子链) | `9950` | `scene-data-trade` (含 DataRegistry + TradeSession) |

## 环境准备

```bash
# 1. 构建 binary
make build-main           # 主链: role-main + MainEscrow
make build-data-trade-child  # child6: scene-data-trade

# 2. 干净重部署 main + child6
scripts/dev_redeploy_clean_chains.sh --chains main,child6 --nodes f1,f2,f3,f4,f5 --logs

# 3. 验证 metadata
deploy/.venv/bin/python deploy/cmd/status.py --chains main,child6
# 预期: main 包含 MainEscrow, child6 包含 DataRegistry + TradeSession, 不含 Crowdsource
```

## E2E 测试

```bash
# Happy path: DO 发布 → DR 锁资 → 轮次交付 → DO claim → 结算
node scripts/data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --scenario happy

# Invalid proof: DO 提交 bad proof → DR 争议 → punish DO
node scripts/data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --scenario invalid-proof

# Requester refuses payment: DR 不付最后一轮 → DO claim last payment
node scripts/data_trade_flow.js --main ws://10.2.2.11:9944 --child ws://10.2.2.11:9950 --scenario requester-refuses-payment
```

## Unit Tests

```bash
SKIP_WASM_BUILD=1 cargo test -p pallet-data-registry
SKIP_WASM_BUILD=1 cargo test -p pallet-trade-session
SKIP_WASM_BUILD=1 cargo test -p pallet-main-escrow
```

## 当前限制

- **Verifier**: `verifier=mock` — 证明验证为 AlwaysPass，不代表 ZK 已上链
- **Bridge**: 默认 observe-only；`--execute` 仅实验室环境可用
- **Cross-chain**: Settlement 由 DO 直接签名主链交易，非 trustless 桥接
