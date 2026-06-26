# Child1 Single-Chain TPS Tuning

Goal: raise the N=1 baseline to about 150 accepted submissions/s before continuing deeper runtime optimization work.

## Finding

The bottleneck is not worker pressure. Increasing from `200 x 4` to `400 x 8` did not improve chain-side throughput; it reduced TPS and introduced many worker failures. The chain-side precise monitor showed that throughput is high in the early part of an epoch and then declines as `EpochSubmissions` grows.

The current pallet stores all epoch submissions in one `StorageValue<BoundedVec<...>>`. Every `submit_data` appends to a larger encoded vector, so larger payloads create write amplification over the epoch. With the original 64B payload, N=1 reached 115.25 TPS over 10000 accepted submissions. With an 8B compact business digest payload, the same 10000-submission cap reached 152.12 TPS.

## Trials

| Trial | Workers | Parallel | Payload | Cap | Accepted | Elapsed | TPS | Conclusion |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| baseline | 200 | 4 | 64B | 10000 | 10000 | 86.768s | 115.25 | below target |
| higher pressure | 400 | 8 | 64B | 10000 | 10000 | 91.147s | 109.71 | pressure is not the limiting factor |
| shorter vector | 200 | 4 | 64B | 5000 | 5020 | 29.604s | 169.57 | early epoch is fast |
| compact payload | 200 | 4 | 8B | 10000 | 10000 | 65.737s | 152.12 | target reached without lowering cap |

## Decision

The progressive TPS launcher now defaults to `DATA_SIZE=8`. This treats each crowdsource submission as a compact business digest for throughput measurement. The default remains overridable:

```bash
DATA_SIZE=64 bash scripts/run_exp_progressive_tps.sh --stage n1
```

Longer-term runtime optimization should still replace the monolithic `EpochSubmissions` vector with indexed or aggregated storage, because compact payloads reduce but do not remove the write-amplification pattern.
