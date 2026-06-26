# 数据交易论文实现差距矩阵

本文将 CDT 论文目标与 FishboneChain 当前数据交易实现逐项对齐。状态值只表示当前仓库实现，不代表未来规划已经完成。

## Status Legend

| Status | Meaning |
|--------|---------|
| `implemented` | 当前代码和测试已经直接实现该能力 |
| `prototype-supported` | 当前原型支持主要流程，但依赖开发期假设或链下协调 |
| `partially-supported` | 已有相关字段、流程或简化实现，但未达到论文完整语义 |
| `not-implemented` | 当前未实现 |
| `future-work` | 明确保留给后续阶段 |

## Gap Matrix

| Paper Requirement | Current Implementation | Status | Evidence | Gap | Candidate Next Step |
|-------------------|------------------------|--------|----------|-----|---------------------|
| DO/DR roles | Bob/Alice style actors in E2E; pallets enforce listing owner, requester, data owner roles | `implemented` | `pallets/data-registry`, `pallets/trade-session`, `scripts/data_trade_flow.js` | Production identity and permission model not hardened | Keep role checks covered in regression |
| DC data registry / listing | `pallet-data-registry` stores listing owner, `imt_root`, description, price, rounds, deposit, request/proof hashes, status | `implemented` | `pallets/data-registry/src/lib.rs`, 12 pallet tests | DC is a Substrate pallet rather than one contract per DO | No immediate blocker |
| VC session state machine | `pallet-trade-session` implements create/accept/open/proof/signature/delivery/payment/dispute/claim flow | `implemented` | `pallets/trade-session/src/lib.rs`, 19 pallet tests | Uses verifier attestation instead of on-chain ZK verification | Decide between verifier quorum and on-chain verifier |
| MainEscrow funds and deposits | `pallet-main-escrow` locks DR funds and DO deposit | `implemented` | `pallets/main-escrow/src/lib.rs`, 9 pallet tests | Settlement is on main chain and coordinated off-chain | Trustless bridge hardening |
| Hash-chain payment / settlement | `settle_by_preimage` validates hash-chain preimage and pays completed rounds | `implemented` | `pallets/main-escrow/src/lib.rs` | Payment commitment proof `π_pc` is not a ZK circuit | Optional payment-proof circuit later |
| Multi-round delivery | E2E and pallet state machine support repeated rounds; Stage 9 `zk_real_data_trade_flow.js` supports dynamic dataset/request-driven witness per round | `implemented` | `scripts/data_trade_flow.js`, `scripts/zk_real_data_trade_flow.js`, `claim_settlement` completed-round tests | Off-chain channel is represented by scripts, not a production channel service | Production channel/bridge service later |
| DR dispute invalid proof | `dispute_invalid_proof` marks session punished; MainEscrow can slash DO deposit | `prototype-supported` | `pallets/trade-session/src/tests.rs`, VM regression historical summary | Chain does not verify invalid Groth16 proof itself | Verifier quorum or on-chain verifier |
| DR dispute invalid plaintext | `dispute_invalid_plaintext` supports hash mismatch dispute | `prototype-supported` | `pallets/trade-session/src/tests.rs` | Plaintext constraint verification is not generalized for paper's full request language | Full request-language validation later |
| DO claim last payment | `claim_last_payment` exists in TradeSession/MainEscrow path | `implemented` | `pallets/main-escrow/src/tests.rs`, `pallets/trade-session/src/tests.rs` | Evidence handling is simplified relative to paper | Extend evidence model if needed |
| ZK proof artifact generation | `fishbone-zk` produces artifact with proof hashes; Stage 9 E2E evidence records per-round digest/hashes | `implemented` | `tools/data-trade-zk`, `target/data-trade-zk/session-*/artifact.json`, `session-*-evidence.json` | Dev per-run setup, no VK registry | VK artifact/version management |
| Circuit-level range business witness | `BusinessRangeProof` proves range, masked value relation, MiMC masked commitment; Stage 8 `make-witness` generates dynamic witness from multi-record/multi-field dataset + request | `implemented` | `tools/data-trade-zk/internal/gnarkadapter/business_range_proof.go`, `internal/dynamic/witness.go` | Dynamic range witness with structured IMT; multi-field/multi-record range only | Subset/substr constraints, broader constraint kinds |
| Root obfuscation proof | Stage 7 four-layer structured IMT (Entry→Dataset→Aggregate→Published) feeds RO proof with aggregate root as published leaf | `prototype-supported` | `tools/data-trade-zk/internal/imt/structured.go` | Deterministic structured prototype; not full dynamic production IMT | Production dynamic IMT |
| Full IMT membership | Stage 7 four-layer structured IMT lite prototype: Entry/Dataset/Aggregate/Published layers with deterministic padding | `partially-supported` | `tools/data-trade-zk/internal/imt/structured.go` | Lite deterministic structured prototype; not dynamic production IMT | Dynamic production IMT |
| Custom constraint kind: range | `constraint_kind = range`; range circuit implemented; Stage 8 `make-witness` supports dynamic multi-record/multi-field witness generation | `implemented` | `BusinessRangeProof`, artifact schema, `internal/dynamic/witness.go` | Range only; subset/substr not implemented | Subset/substr constraints |
| Custom constraint kinds: subset/substr | Mentioned in paper/reference docs only | `not-implemented` | `docs/architecture/cdt.md`, references | No circuits, artifact paths, or runtime handling | Post-IMT future work |
| On-chain ZK verification | Runtime uses `AlwaysPassVerifier`; chain verifies digest and attestation | `not-implemented` | `runtime/src/configs/mod.rs`, `pallets/trade-session/src/proof.rs` | No Groth16 verifier pallet or VK registry | Post-IMT security hardening |
| Verifier authority / attestation | Single `VerifierAuthority = Charlie` signs attestation digest | `prototype-supported` | `runtime/src/configs/mod.rs`, `attest_data_proof` | Dev key, no quorum, no slashing | Multi-verifier quorum hardening |
| Multi-verifier quorum | Not present | `not-implemented` | Security model marks future work | Single trusted verifier remains central trust assumption | Add verifier set + threshold policy |
| Trustless cross-chain settlement | Bridge/session-escrow binding checked by scripts | `not-implemented` | `scripts/lib/data_trade_binding.js`, security model | MainEscrow does not verify child proof on-chain | Future trustless bridge hardening |
| Multiple data-trade child chains | `child6-data-trade` and `child7-business-trade` profiles configured | `prototype-supported` | `scripts/profiles/chains.json`, Stage 3 record | Current 2026-06-26 child RPC check timed out; production deployment needs hardening | Refresh child7 VM smoke before claiming live availability |
| FmcAssisted / Hybrid settlement | Reserved in docs/security model | `future-work` | `docs/architecture/data-trade-security-model.md` | No wired pallet/script flow | Lower priority unless paper needs Fishbone funding integration |

## Paper Wording Guidance

Safe current claims:

- The prototype implements the data-trade workflow on FishboneChain using dedicated data-trade pallets and MainEscrow settlement.
- The prototype supports multi-round delivery, hash-chain settlement, invalid-proof/plaintext disputes, and last-payment recovery.
- The chain binds proof metadata, VK hash, public input hash, `business_input_hash`, and verifier attestation into the on-chain session state.
- The off-chain gnark path generates and verifies Groth16 BN254 artifacts.
- For the range case, Stage 2.2 proves the business witness at circuit level: `raw_value ∈ [min,max]`, `masked_value = raw_value + delta`, and `masked_value_hash = MiMC(masked_value, salt)`.
- The prototype supports multiple data-trade profiles through configuration (`child6-data-trade`, `child7-business-trade`).

Claims that must be phrased as limitations or future work:

- Do not claim on-chain Groth16 verification.
- Do not claim trustless bridge settlement.
- Do not claim full IMT membership verification.
- Do not claim subset/substr constraints are implemented.
- Do not claim production verifier security; current verifier authority is a single dev account.
- Do not claim production trusted setup or VK registry.

Recommended paper phrasing:

> We implement a FishboneChain prototype of the CDT workflow with MainEscrow settlement, profile-configurable data-trade subchains, and an off-chain gnark verifier whose proof artifacts are bound to on-chain session state through digest and verifier attestation. The current prototype implements a circuit-level range business witness; full IMT membership, subset/substr constraints, on-chain proof verification, and trustless cross-chain settlement are left as future hardening steps.
