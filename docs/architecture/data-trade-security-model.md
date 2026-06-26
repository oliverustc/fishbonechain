# Data Trade Security Model

## Scope

This document describes the security model of FishboneChain data-trade scenarios as implemented in the current prototype and how it maps to the paper target.

## Participants

| Participant | Role | Current Trust Assumption | Future Direction |
|-------------|------|--------------------------|------------------|
| DR | Data requester | Pays via MainEscrow, can dispute invalid proof | Same |
| DO | Data owner | Provides data/proof, can claim settlement after completed rounds | Same |
| Verifier | Off-chain proof verifier | Single dev authority Charlie signs attestation; Charlie is a dev key and its private key is public in development | Multi-verifier quorum or on-chain proof verifier |
| Bridge | Off-chain coordinator | Checks session-escrow binding and submits/coordinates main settlement | CCMC/Merkle proof based trustless bridge |
| Main chain | Funds and settlement | MainEscrow enforces hash-chain settlement | FMC-assisted and hybrid settlement |
| Data trade child chain | Session/proof workflow | Stores proof digest and attestation state | Multiple trade profiles |

## Current Guarantees

- MainEscrow prevents payment release without a valid hash-chain preimage.
- TradeSession binds session to listing, escrow, request hash, terms, proof digest, and verifier attestation.
- `fishbone-zk verify` verifies gnark proof artifacts off-chain and rejects proof/VK/public input hash mismatch.
- **Stage 2.2**: `BusinessRangeProof` gnark circuit proves `raw_value ∈ [min, max]`, `masked_value = raw_value + delta`, and `masked_value_hash = MiMC(masked_value, salt)`. `business_input_hash` is bound to the on-chain proof digest and verifier attestation.
- **Stage 3**: Multiple data-trade subchain profiles supported via `trade_profiles` in `chains.json`; child6 and child7 are configured and VM-verified.

## Current Non-Guarantees

- The chain does not verify Groth16 proof on-chain; verification is off-chain via `fishbone-zk verify`.
- Charlie is a single verifier authority in dev mode, and the `//Charlie` private key is known to every operator with development-key access.
- Bridge/session-escrow consistency is checked off-chain, not by a trustless cross-chain proof.
- Full IMT membership, subset/substr constraint kinds, and multi-verifier quorum are future work.

## Settlement Modes

| Mode | Current Status | Security Boundary | Suitable Scenario |
|------|----------------|-------------------|-------------------|
| MainEscrow | Implemented | Main chain locks funds and deposit | Data trading with direct escrow |
| FmcAssisted | Reserved | Main chain FMC/TMC task-fund accounting | Crowdsourcing or task-like data services |
| Hybrid | Reserved | Combines escrow and FMC accounting | Complex services |

## Attack Scenarios

| Scenario | Current Detection | Current Response | Gap |
|----------|-------------------|------------------|-----|
| DO submits invalid proof digest | DR dispute / verifier rejection state | Session punished, MainEscrow can slash deposit | Automated trustless bridge not implemented |
| DR refuses final payment | DO claimLastPayment | MainEscrow releases one round | Requires off-chain coordination |
| Verifier signs false attestation | Not prevented in single-verifier dev mode; `//Charlie` is a public dev key, so any operator can forge the attestation in dev deployments | Operational trust assumption only | Need quorum/slashing/on-chain verifier |
| Bridge submits wrong settlement | Binding helper checks in scripts | Avoided by dev script guard | Need cross-chain proof |

## Paper Alignment Matrix

| Paper Requirement | Current Implementation | Gap | Planned Stage |
|-------------------|------------------------|-----|---------------|
| Desensitized verifiable data | Stage 1 gnark pipeline + Stage 2.2 circuit-level business witness | Full IMT membership, subset/substr constraint kinds | Stage 2 complete; IMT future |
| Multiple data-trade subchains | child6 + child7 profile configured, VM-verified | Production hardening | Stage 3 complete |
| Flexible settlement | MainEscrow implemented, FMC reserved | FmcAssisted/Hybrid not wired | Future |
| Trustless cross-chain settlement | Not implemented | CCMC/Merkle proof bridge | Future |

## Engineering References

- `pallets/trade-session/src/lib.rs`
- `pallets/main-escrow/src/lib.rs`
- `scripts/zk_real_data_trade_flow.js`
- `tools/data-trade-zk/internal/artifact/schema.go`
- `tools/data-trade-zk/internal/gnarkadapter/business_range_obfuscation.go`
- `tools/data-trade-zk/internal/gnarkadapter/business_range_proof.go`
- `docs/implementation/data-trade-implementation.md`

## Review Checklist for Future Changes

- [ ] Does the change alter who can release funds?
- [ ] Does the change alter who can attest proof validity?
- [ ] Does the change alter the proof digest preimage fields?
- [ ] Does the change alter session-escrow binding?
- [ ] Does the change claim trustless behavior that is still off-chain coordinated?
- [ ] Does the VM E2E cover MainEscrow settlement after the change?
- [ ] If a new child chain is added, is its profile documented?
