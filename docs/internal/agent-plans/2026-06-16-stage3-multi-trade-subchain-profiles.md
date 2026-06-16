# Stage 3 Multi Trade Subchain Profiles Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 支持多个数据交易/zk 服务场景子链，每条子链通过配置声明 profile、proof params、settlement mode 和 verifier policy，而不是复制脚本或硬编码 child6。

**Architecture:** 在 `scripts/profiles/chains.json` 与 `deploy/config.toml` 中引入数据交易 profile 描述，新增共享 JS profile loader，让 E2E 脚本从 profile 读取 RPC、settlement mode、ZK CLI、business witness 和 verifier policy。先新增一个 child7 配置化 profile 做 smoke，不要求立即部署完整新 runtime。

**Tech Stack:** JSON profile config, Node.js E2E scripts, Python spec/deploy config, Rust chain-profile runtime metadata, VM deployment scripts.

---

## Files

- Modify: `scripts/profiles/chains.json`
- Create: `scripts/lib/trade_profile.js`
- Modify: `scripts/data_trade_flow.js`
- Modify: `scripts/zk_attested_data_trade_flow.js`
- Modify: `scripts/zk_real_data_trade_flow.js`
- Modify: `deploy/config.toml`
- Modify: `scripts/gen_child_specs.py`
- Modify: `docs/implementation/data-trade-implementation.md`
- Modify: `docs/internal/agent-plans/2026-06-16-data-trade-four-stage-roadmap.md`

## Profile Shape

Every data trade profile must expose:

```json
{
  "id": "child6-data-trade",
  "chain": "child6",
  "main_ws": "ws://10.2.2.11:9944",
  "child_ws": "ws://10.2.2.11:9950",
  "settlement_mode": "MainEscrow",
  "verifier_mode": "gnark-groth16-bn254",
  "verifier_authority": "//Charlie",
  "zk_verifier_cmd": "target/tools/fishbone-zk",
  "business_witness": "scripts/fixtures/data_trade_business_sample.json",
  "proof": {
    "system": "GnarkGroth16Bn254",
    "constraint_kind": "Range",
    "ro_depth": 10
  }
}
```

CLI 参数优先级：显式命令行参数始终覆盖 profile 默认值。也就是说 `--main`、`--child`、`--business-witness` 和环境变量 `ZK_VERIFIER_CMD` 的优先级高于 `scripts/profiles/chains.json` 中的值；profile 只提供默认配置。

## Task 1: Add Profile Loader

- [ ] Step 1: Create failing JS smoke command.

Run:

```bash
node --input-type=module -e "import { loadTradeProfile } from './scripts/lib/trade_profile.js'; console.log(loadTradeProfile('child6-data-trade').chain)"
```

Expected: fail because `trade_profile.js` does not exist.

- [ ] Step 2: Create `scripts/lib/trade_profile.js`.

Expected file:

```js
import { readFileSync } from "node:fs";

const DEFAULT_PATH = "scripts/profiles/chains.json";

export function loadProfiles(path = DEFAULT_PATH) {
  const raw = JSON.parse(readFileSync(path, "utf8"));
  if (!raw.trade_profiles || typeof raw.trade_profiles !== "object") {
    throw new Error(`missing trade_profiles in ${path}`);
  }
  return raw.trade_profiles;
}

export function loadTradeProfile(id, path = DEFAULT_PATH) {
  const profiles = loadProfiles(path);
  const profile = profiles[id];
  if (!profile) {
    throw new Error(`unknown trade profile: ${id}`);
  }
  const required = ["chain", "main_ws", "child_ws", "settlement_mode", "verifier_mode"];
  for (const key of required) {
    if (!profile[key]) throw new Error(`trade profile ${id} missing ${key}`);
  }
  if (!profile.proof || typeof profile.proof !== "object") {
    throw new Error(`trade profile ${id} missing proof config`);
  }
  for (const key of ["system", "constraint_kind", "ro_depth"]) {
    if (!profile.proof[key]) throw new Error(`trade profile ${id} missing proof.${key}`);
  }
  if (!Number.isInteger(profile.proof.ro_depth) || profile.proof.ro_depth <= 0) {
    throw new Error(`trade profile ${id} proof.ro_depth must be a positive integer`);
  }
  return { id, ...profile };
}

export function parseProfileArg(argv = process.argv) {
  const idx = argv.indexOf("--profile");
  return idx === -1 ? null : argv[idx + 1];
}
```

- [ ] Step 3: Extend `scripts/profiles/chains.json`.

Add top-level key:

```json
"trade_profiles": {
  "child6-data-trade": {
    "chain": "child6",
    "main_ws": "ws://10.2.2.11:9944",
    "child_ws": "ws://10.2.2.11:9950",
    "settlement_mode": "MainEscrow",
    "verifier_mode": "gnark-groth16-bn254",
    "verifier_authority": "//Charlie",
    "zk_verifier_cmd": "target/tools/fishbone-zk",
    "business_witness": "scripts/fixtures/data_trade_business_sample.json",
    "proof": {
      "system": "GnarkGroth16Bn254",
      "constraint_kind": "Range",
      "ro_depth": 10
    }
  }
}
```

If `chains.json` is an array or has a different shape, preserve existing keys and add `trade_profiles` without deleting current content.

- [ ] Step 4: Run smoke.

Run:

```bash
node --input-type=module -e "import { loadTradeProfile } from './scripts/lib/trade_profile.js'; const p = loadTradeProfile('child6-data-trade'); if (p.chain !== 'child6') process.exit(1); console.log(p.child_ws)"
```

Expected: prints `ws://10.2.2.11:9950`.

- [ ] Step 5: Commit profile loader.

Run:

```bash
git add scripts/lib/trade_profile.js scripts/profiles/chains.json
git commit -m "feat: add configurable data trade profiles"
```

## Task 2: Make E2E Scripts Profile-Aware

- [ ] Step 1: Modify `scripts/data_trade_flow.js`.

Add imports:

```js
import { loadTradeProfile, parseProfileArg } from "./lib/trade_profile.js";
```

After `parseArg` definition, add:

```js
const PROFILE = parseProfileArg();
const PROFILE_CONFIG = PROFILE ? loadTradeProfile(PROFILE) : null;
```

Change endpoint constants:

```js
const MAIN_WS = parseArg("--main") || PROFILE_CONFIG?.main_ws || "ws://127.0.0.1:9944";
const CHILD_WS = parseArg("--child") || PROFILE_CONFIG?.child_ws || "ws://127.0.0.1:9950";
```

- [ ] Step 2: Apply the same endpoint profile logic to `scripts/zk_attested_data_trade_flow.js`.

Use the same import and constants as Step 1.

- [ ] Step 3: Apply profile logic to `scripts/zk_real_data_trade_flow.js`.

Use:

```js
const ZK_CMD = process.env.ZK_VERIFIER_CMD || PROFILE_CONFIG?.zk_verifier_cmd || "target/tools/fishbone-zk";
const BUSINESS_WITNESS = parseArg("--business-witness") || PROFILE_CONFIG?.business_witness || "scripts/fixtures/data_trade_business_sample.json";
```

- [ ] Step 4: Run syntax checks.

Run:

```bash
node --check scripts/data_trade_flow.js
node --check scripts/zk_attested_data_trade_flow.js
node --check scripts/zk_real_data_trade_flow.js
```

Expected: all pass.

- [ ] Step 5: Run profile smoke without deploy.

Run:

```bash
node scripts/data_trade_flow.js --profile child6-data-trade --scenario happy
```

Expected: if VM is running, happy path passes; if VM is not running, failure must be connection-related, not argument/profile parsing.

- [ ] Step 6: Commit profile-aware scripts.

Run:

```bash
git add scripts/data_trade_flow.js scripts/zk_attested_data_trade_flow.js scripts/zk_real_data_trade_flow.js
git commit -m "feat: load data trade e2e settings from profiles"
```

## Task 3: Add Child7 Profile Draft

- [ ] Step 1: Extend `deploy/config.toml` with `[chains.child7]`.

Add:

```toml
[chains.child7]
# 脱敏可验证数据交易实验链（DataTrade/MainEscrow，业务 witness）
id       = "fishbone_child_7"
spec     = "specs/child7-custom-raw.json"
binary   = "/home/debian/fishbone/bin/fishbone-node-data-trade"
p2p_port = 30340
rpc_port = 9951
prom_port = 9622
```

Add `child7` to f1-f5 roles and empty/default peer ids must be generated by existing deploy key flow.

- [ ] Step 2: Extend `scripts/profiles/chains.json`.

Add:

```json
"child7-business-trade": {
  "chain": "child7",
  "main_ws": "ws://10.2.2.11:9944",
  "child_ws": "ws://10.2.2.11:9951",
  "settlement_mode": "MainEscrow",
  "verifier_mode": "gnark-groth16-bn254",
  "verifier_authority": "//Charlie",
  "zk_verifier_cmd": "target/tools/fishbone-zk",
  "business_witness": "scripts/fixtures/data_trade_business_sample.json",
  "proof": {
    "system": "GnarkGroth16Bn254",
    "constraint_kind": "Range",
    "ro_depth": 10
  }
}
```

- [ ] Step 3: Update `scripts/gen_child_specs.py`.

Ensure it can generate `child7-custom-raw.json` by mapping `child7` to data-trade runtime profile. If current script branches only for `child6`, add `child7` to the same data-trade set:

```python
DATA_TRADE_CHAINS = {"child6", "child7"}
```

- [ ] Step 4: Generate child7 spec.

Run:

```bash
python3 scripts/gen_child_specs.py --only child7
```

Expected: `deploy/specs/child7-custom-raw.json` created.

- [ ] Step 5: Commit child7 draft.

Run:

```bash
git add deploy/config.toml scripts/profiles/chains.json scripts/gen_child_specs.py deploy/specs/child7-custom-raw.json
git commit -m "feat: add child7 data trade profile draft"
```

## Task 4: VM Smoke for Child7

- [ ] Step 1: Clean deploy main+child7.

Run:

```bash
bash scripts/dev_redeploy_clean_chains.sh --chains main,child7 --config deploy/config.toml --logs
```

Expected: f1-f5 child7 services active, f1 listens on `9951`.

Important: this command also clean-resets `main`, because `main` is included in `--chains main,child7`. This is acceptable in the current dev VM environment but must be called out in the Execution Record so nobody mistakes it for a non-destructive smoke.

- [ ] Step 2: Metadata check.

Run:

```bash
node --input-type=module -e "import { ApiPromise, WsProvider } from '@polkadot/api'; const api = await ApiPromise.create({ provider: new WsProvider('ws://10.2.2.11:9951') }); console.log((await api.rpc.system.chain()).toString(), !!api.tx.dataRegistry, !!api.tx.tradeSession); await api.disconnect();"
```

Expected: prints child7 chain name and `true true`.

- [ ] Step 3: Run profile E2E.

Run:

```bash
node scripts/zk_real_data_trade_flow.js --profile child7-business-trade
```

Expected: real-zk path passes against child7.

- [ ] Step 4: Update docs and roadmap.

Modify `docs/implementation/data-trade-implementation.md` to list child7 draft profile and VM smoke result.

- [ ] Step 5: Commit.

Run:

```bash
git add docs/implementation/data-trade-implementation.md docs/internal/agent-plans/2026-06-16-stage3-multi-trade-subchain-profiles.md docs/internal/agent-plans/2026-06-16-data-trade-four-stage-roadmap.md
git commit -m "docs: record multi subchain profile validation"
```

## Execution Record

- Not started.
