# Fishbone Data Trade ZK

This tool wraps the paper prototype in `references/data_trade_code/snarks/gnarkzkp`.

## Stage 1 trust boundary

- gnark proofs are generated and verified off-chain.
- child6 does not verify Groth16 in WASM.
- child6 verifies hashes, session binding, and VerifierAuthority attestation.

## Commands

- `fishbone-zk fixture` — generate proof artifact fixture
- `fishbone-zk setup` — compile + trusted setup
- `fishbone-zk prove` — generate proof (alias to fixture for Stage 1)
- `fishbone-zk verify --artifact <path>` — verify artifact and embedded proofs

## Build

```bash
cd tools/data-trade-zk
go build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
```
