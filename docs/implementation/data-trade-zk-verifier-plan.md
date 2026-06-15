# Data Trade ZK Verifier 接入计划

## 当前状态

- **Phase 1 (当前)**：使用 `AlwaysPassVerifier` mock — 所有 proof 验证通过，唯 plaintext hash 检查实际比对。
- **Protocol state machine**：完整实现了论文 VC 状态机（12 extrinsics），可通过 E2E 脚本端到端运行。
- **Proof types**：`ProofBundle` 已在 `pallets/trade-session/src/proof.rs` 中定义，包含 `constraint_kind`、`ch_proof_hash`、`ro_proof_hash`、`public_input_hash`。

## gnark → Substrate 的差距

gnark proof 生成代码位于 `references/data_trade_code/snarks/gnarkzkp`，Solidity verifier 位于 `references/data_trade_code/foundry/`。这三者不能直接搬到 Substrate FRAME pallet：

| 组件 | gnark | Substrate FRAME |
|------|-------|-----------------|
| 证明生成 | Go 原生 | 不在链上（链下生成） |
| 验证器 | Solidity (EVM) | FRAME pallet (Rust/Wasm) |
| 曲线 | 自定义（BN254 等） | 需 host function 或 precompile |
| 约束系统 | R1CS (groth16) | 需要 arkworks 重写或 host function 桥接 |

## 三种接入路径（按推荐顺序）

### 路径 A：链下 Verifier + 签名验证（最可行）

1. 链下运行 gnark/Solidity verifier
2. Verifier 对 `(proof, public_inputs, result)` 签名
3. Pallet 验证 verifier 签名（已知公钥）
4. 优点：不改 gnark 代码，快速集成
5. 缺点：需要信任 verifier 服务

### 路径 B：Host Function（Polkadot SDK）

1. 将 gnark 验证核心编译为 Rust host function
2. 在 `node/src/service.rs` 中注册 host function
3. Pallet 通过 `sp_io::crypto` 或自定义 host function 调用
4. 优点：真正的链上 ZK 验证
5. 缺点：需要维护 host function ABI；Polkadot SDK 版本升级时可能断裂

### 路径 C：Arkworks 重写（最彻底）

1. 把 gnark R1CS 约束系统用 arkworks (Rust) 重写
2. 直接在 pallet 中编译验证
3. 优点：纯 Rust，无外部依赖
4. 缺点：工程量最大；验证性能需仔细评估（Wasm vs native）

## 当前 E2E 输出中的 verifier 标识

E2E 脚本运行完成后会在控制台输出：

```
verifier=mock
```

表示当前使用 mock verifier，不代表 ZK 证明系统已上链。只有在接入 paths B/C 后才会改为 `verifier=zk`。

## 对后续开发的建议

1. 先稳定协议状态机和资金结算（当前已完成）
2. 验证 E2E 流程的恶意路径（invalid-proof、refuses-payment）
3. 选择路径 A（链下验证 + 签名）作为下一阶段目标
4. 在 `trade-session` 的 `submit_data_proof` 中，用 `ProofBundle` 的 `public_input_hash` 字段承载真实 public inputs
5. 将 verifier 签名结果作为 `proof_signature_hash` 提交
