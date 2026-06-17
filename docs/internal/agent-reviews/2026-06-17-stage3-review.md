# Stage 3 Code Review

Status: Changes requested

Reviewed range: `aced8b2..main`

Context: Stage 3 was already committed to `main` before the GitHub PR workflow was introduced, so this review uses the local fallback review-file path. Future Reasonix work should use GitHub PR review comments instead.

## Findings

- P1: child7 raw spec still identifies itself as child6.
  - File: `scripts/gen_child_specs.py`
  - Evidence:
    - `python3 scripts/gen_child_specs.py --only child7` succeeds but generated `deploy/specs/child7-custom-raw.json` has:
      - `name = "Fishbone Child-6 (Data Trade, AURA-5)"`
      - `id = "fishbone_child_6"`
    - RPC confirms the deployed child7 endpoint reports `Fishbone Child-6 (Data Trade, AURA-5)`.
  - Reason: Stage 3 plan required using `template_chain_id = "child6-local"` only as a template, then overriding `spec["name"]` and `spec["id"]` for child7. The implementation added `template_chain_id` but did not add/call `inject_spec_identity`, and child7 config lacks `spec_name` / `spec_id`.
  - Required fix:
    - Add `spec_name = "Fishbone Child-7 (Business Data Trade, AURA-5)"` and `spec_id = "fishbone_child_7"` to child7 config in `scripts/gen_child_specs.py`.
    - Add and call `inject_spec_identity(spec, cfg)` before validator/profile injection.
    - Regenerate `deploy/specs/child7-custom-raw.json`.
    - Redeploy child7 or record that redeploy is pending.
    - Verify `system.chain` on `ws://10.2.2.11:9951` reports child7, not child6.

- P2: deployment config silently drops child7 peer IDs, so child7 bootnodes are empty.
  - Files:
    - `deploy/fishbone/config.py`
    - `deploy/config.toml`
    - `deploy/fishbone/service.py`
  - Evidence:
    - `deploy/config.toml` contains `[nodes.peer_ids].child7`.
    - `NodePeerIds` only defines `main` through `child6`.
    - `load()` filters peer ids by `NodePeerIds.__dataclass_fields__`, so `child7` is discarded.
    - `cfg.bootnodes("child7")` returns `[]`.
  - Reason: Stage 3 is meant to make additional data-trade subchains configurable. A fixed `NodePeerIds` dataclass requires code edits for every new child chain and weakens repeatable deployment. Current VM nodes still show 4 peers, likely through peer discovery, but the generated systemd services do not contain explicit child7 bootnodes.
  - Required fix:
    - Replace fixed `NodePeerIds` with a dynamic `dict[str, str]` peer-id representation, or otherwise support arbitrary configured chain names without adding a field per child.
    - Ensure `cfg.bootnodes("child7")` returns configured child7 bootnodes after peer IDs are populated.
    - Add/update tests under `deploy/tests/` for dynamic peer ids and `filter_config_to_chains`.
    - Add child7 label to `deploy/fishbone/service.py` or derive labels from config to avoid another hardcoded list.

- P2: `scripts/profiles/chains.json` has no top-level child7 chain profile.
  - File: `scripts/profiles/chains.json`
  - Evidence:
    - `profiles.child7` is `undefined`.
    - `child7-business-trade` exists under `trade_profiles`, but the top-level chain profile map only contains child1-child6.
  - Reason: earlier platform-scene decoupling uses top-level chain profiles as the script-visible chain identity map (`chainId`, `scene`, `settlement`). Stage 3 goal says each data-trade/zk service subchain should have its own profile. Leaving child7 out makes the file internally inconsistent and future scripts that use top-level profiles cannot discover child7.
  - Required fix:
    - Add `"child7": { "chainId": 6, "scene": "DataTrade", "settlement": "MainEscrow" }` to the top-level map.
    - Keep `trade_profiles.child7-business-trade` for E2E/runtime settings.
    - Add a loader smoke check that top-level child7 and trade child7 agree on chain id/scene/settlement.

- P2: implementation docs still describe only child6 and do not record Stage 3 child7 profile/smoke.
  - File: `docs/implementation/data-trade-implementation.md`
  - Evidence:
    - The architecture overview and VM deployment section still describe child6 only.
    - Stage 3 plan Task 4 Step 4 required listing the child7 draft profile and VM smoke result.
  - Reason: The stage execution record says Stage 3 is complete, but the stable implementation doc does not reflect the new multi-subchain profile state.
  - Required fix:
    - Add a Stage 3 subsection documenting child6 and child7 profiles, RPC endpoints, and settlement/verifier defaults.
    - Record the child7 VM smoke result, including the exact command and date.
    - If child7 chain name is still wrong before fix, state that as a known issue rather than claiming full identity correctness.

## Notes

- Current child7 VM path is usable despite the identity issue:
  - `node scripts/zk_real_data_trade_flow.js --profile child7-business-trade` completed successfully.
  - f1-f5 child7 RPC health reports 4 peers and matching block height.
- `target/tools/fishbone-zk` is a local built binary and still prints `masked_value_hash=...`, while source now prints `business_input_hash=...`. The E2E script does not parse this label, but logs are confusing. Rebuild `target/tools/fishbone-zk` before relying on it as evidence.
- `python3 deploy/cmd/status.py --chains child7 --config deploy/config.toml` could not run in the current local environment because `rich` is missing. Direct curl RPC checks were used instead.

## Verification

Commands run:

```bash
node --check scripts/data_trade_flow.js
node --check scripts/zk_attested_data_trade_flow.js
node --check scripts/zk_real_data_trade_flow.js
node --check scripts/lib/trade_profile.js

python3 -m py_compile scripts/gen_child_specs.py scripts/dev_scan_vms.py scripts/deploy_child_chains.py

node --input-type=module -e "import { loadTradeProfile } from './scripts/lib/trade_profile.js'; const a=loadTradeProfile('child6-data-trade'); const b=loadTradeProfile('child7-business-trade'); console.log(a.chain, a.child_ws, b.chain, b.child_ws)"

python3 scripts/gen_child_specs.py --only child7

./deploy/bin/fishbone-node-data-trade build-spec --chain child7-local --disable-default-bootnode >/tmp/child7-spec.json 2>/tmp/child7-spec.err; test $? -ne 0

node --input-type=module -e "import { ApiPromise, WsProvider } from '@polkadot/api'; const api = await ApiPromise.create({ provider: new WsProvider('ws://10.2.2.11:9951') }); console.log((await api.rpc.system.chain()).toString(), !!api.tx.dataRegistry, !!api.tx.tradeSession, !!api.tx.crowdsource); await api.disconnect();"

node scripts/zk_real_data_trade_flow.js --profile child7-business-trade
```

Results:

- JS syntax checks: pass.
- Python compile checks: pass.
- Profile loader smoke: `child6 ws://10.2.2.11:9950 child7 ws://10.2.2.11:9951`.
- `gen_child_specs.py --only child7`: pass, but regenerates child7 with child6 top-level identity.
- `build-spec --chain child7-local`: fails as expected.
- child7 metadata RPC: `dataRegistry=true`, `tradeSession=true`, `crowdsource=false`, but chain name is still child6.
- child7 real ZK E2E: pass.
