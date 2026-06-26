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

## Stage 6 IMT Coupling

The `business-fixture` command now links the range business witness to a deterministic
IMT fixture (depth 10, leaf 0 = `masked_value_hash`, 9 padding leaves, 4 decoy roots
derived from fixture metadata). The `business_input_hash` includes `dataset_id`,
`field_name`, `depth`, `leaf_index`, and `root_list_index` using 4-byte LE length-prefixed
string encoding. This is a deterministic fixture coupling — not full production IMT.

## Build

```bash
cd tools/data-trade-zk
go build -o ../../target/tools/fishbone-zk ./cmd/fishbone-zk
```
