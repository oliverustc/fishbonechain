# Fishbone Data Trade ZK

This tool wraps the paper prototype in `references/data_trade_code/snarks/gnarkzkp`.

## Stage 1 trust boundary

- gnark proofs are generated and verified off-chain.
- child6 does not verify Groth16 in WASM.
- child6 verifies hashes, session binding, and VerifierAuthority attestation.

## Commands

- `fishbone-zk fixture` — generate proof artifact fixture
- `fishbone-zk business-fixture` — generate business range proof with deterministic IMT coupling (Stage 6)
- `fishbone-zk setup` — compile + trusted setup
- `fishbone-zk prove` — generate proof (alias to fixture for Stage 1)
- `fishbone-zk verify --artifact <path>` — verify artifact and embedded proofs

## Stage 6-7 IMT

The `business-fixture` command links the range business witness to a **structured IMT
membership lite** prototype (Stage 7): four-layer deterministic Merkle model with
Entry, Dataset, Aggregate, and Published root layers. The published leaf is the
aggregate root. `business_input_hash` includes `record_id`, `schema_version`, and
all layer depths/indices using 4-byte LE length-prefixed string encoding. This is a
lite prototype — not production dynamic IMT.

## Build

```bash
cd tools/data-trade-zk
go build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
```
