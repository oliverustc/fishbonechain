# Child3 Baseline TPS Diagnosis

Goal: determine whether the third tuned-baseline child chain can reach about 150 accepted submissions/s using deployment, RPC, block production, and pressure parameters only.

## Finding

Child1 reaches the single-chain target with the compact 8B payload, but child3 does not reach 150 TPS on the current `f7/f8/f9` deployment. The best child3 result in these trials was about 83.85 TPS.

The main bottleneck is not raw client pressure. In `pool` mode the worker can submit far more transactions than the chain accepts, but this overfills the transaction pool and slows block production. In `watch` mode, block production returns to about 2 seconds per block, but the number of accepted submissions per block drops as `EpochSubmissions` grows. That points back to the current monolithic submission vector write-amplification pattern and the child3 node resource envelope.

## Trials

| Trial | Endpoint Strategy | Submit Mode | Workers | Parallel | Payload | Cap | Accepted | Elapsed | TPS | Avg Accepted/Nonzero Block | Avg Nonzero Block Gap | Result |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| distributed RPC | f7/f8/f9, 100 workers each | pool | 300 | 4 | 8B | 10000 | 10000 | 152.082s | 65.75 | 263.2 | 3.59s | worse than single endpoint |
| low pool pressure | f7 only | pool | 200 | 1 | 8B | 10000 | 10000 | 122.080s | 81.91 | 294.3 | 3.43s | still slow blocks |
| bounded inclusion | f7 only | watch | 200 | 2 | 8B | 10000 | 10000 | 119.267s | 83.85 | 169.5 | 1.97s | stable blocks, not enough per-block capacity |
| higher watch pressure | f7 only | watch | 200 | 4 | 8B | 10000 | 1031 | 239.825s | 4.30 | 343.7 | 1.85s | timed out after early failures |

## Interpretation

- `pool` mode is not an authoritative success signal. It reports acceptance into the transaction pool, while the chain-side `capacity_monitor.js` CSV is the accepted-submission metric.
- Distributed RPC did not improve child3; it increased pressure on all three validators and reduced chain-side TPS.
- `watch` mode proved that child3 can keep roughly 2 second blocks when transaction-pool pressure is bounded.
- The remaining gap is per-block business capacity over a full 10000-submission epoch. As the single `EpochSubmissions` vector grows, later submissions become more expensive.
- Remote process inspection during the diagnosis showed child3 validator processes around 180-210% CPU after pressure runs, and `f7` also hosts the child4 validator. Resource competition is therefore a real deployment factor, but not the only bottleneck.

## Decision

Do not spend more time trying to make child3 reach 150 TPS by pressure parameters alone. The honest next step is one of:

1. Keep N=1 as the single-chain 150 TPS proof and explain that child3 on the current hardware is the baseline bottleneck.
2. Change the N=1..3 deployment mapping so the first three baseline chains use comparable isolated resources.
3. Move forward to N=4..6 runtime optimization, where indexed/aggregated storage and batch business submissions are expected to remove the write-amplification limit.

For the planned defense figure, option 2 or option 3 is required if the N=3 bar must approach 450 TPS.
