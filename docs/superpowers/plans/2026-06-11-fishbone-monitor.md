# Fishbone Monitor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a flexible monitoring backend on `bcg` that discovers FishboneChain nodes/chains from configuration, collects chain and host state through the `10.2.2.0/24` network, and exposes stable APIs for a future frontend without blocking later Prometheus/Grafana integration.

**Architecture:** Add an independent `monitor/` TypeScript service that runs on `bcg:18080`. It reads `deploy/config.toml`, normalizes nodes/chains into an inventory, runs plugin-style collectors for Substrate JSON-RPC and Prometheus text endpoints, stores latest state in memory, and exposes REST, SSE, and Prometheus-compatible `/metrics` endpoints.

**Tech Stack:** Node.js 18+, TypeScript, Fastify, `@polkadot/api`, `toml`, `prom-client`, built-in `node:test`, systemd on `bcg`.

---

## Current Environment

- `bcg` SSH alias resolves to `192.168.8.41`, user `debian`.
- `bcg` has `ens18 = 192.168.8.41/24` and `ens19 = 10.2.2.1/24`.
- `bcg` can directly reach `10.2.2.11:9944` JSON-RPC and `10.2.2.11:9615/metrics`.
- Port `80` is already in use. Use `18080` for the first monitor deployment.
- Keep existing SSH ProxyJump deployment tooling. The monitor is an observability gateway, not a replacement for deployment control.

## Compatibility Rules

- Do not design frontend-only data shapes. Every runtime status must have a canonical backend model first.
- Keep labels Prometheus-friendly: low cardinality, stable names, no raw peer IDs or hashes as labels.
- Expose `/metrics` in Prometheus text format from the first backend milestone.
- Keep future Prometheus/Grafana optional: the monitor must work standalone, but Prometheus can later scrape the same `/metrics` endpoint.
- Treat `deploy/config.toml` as the initial inventory source, but isolate it behind an inventory interface so later dynamic chain registration does not require rewriting collectors.

## File Structure

- Create `monitor/package.json`: independent monitor package scripts and dependencies.
- Create `monitor/tsconfig.json`: TypeScript compiler settings for Node 18.
- Create `monitor/src/config.ts`: environment variable parsing.
- Create `monitor/src/inventory/types.ts`: normalized `NodeInventory`, `ChainInventory`, and `InventorySnapshot` types.
- Create `monitor/src/inventory/tomlInventory.ts`: load and normalize `deploy/config.toml`.
- Create `monitor/src/collectors/types.ts`: collector contracts and result envelopes.
- Create `monitor/src/collectors/substrateRpc.ts`: JSON-RPC health, header, finalized head, runtime version.
- Create `monitor/src/collectors/substratePrometheus.ts`: parse selected Substrate `/metrics` values.
- Create `monitor/src/state/store.ts`: in-memory latest-state store with timestamps and stale detection.
- Create `monitor/src/metrics/prometheus.ts`: `prom-client` registry and metric export mapping.
- Create `monitor/src/api/server.ts`: Fastify REST, SSE, health, and `/metrics`.
- Create `monitor/src/index.ts`: process entry point and scheduler wiring.
- Create `monitor/test/*.test.ts`: unit tests for inventory, collectors, state, and metric rendering.
- Create `deploy/systemd/fishbone-monitor.service`: systemd unit template for `bcg`.
- Create `docs/fishbone-monitor.md`: operator guide and API reference.

## Data Model

Use these canonical shapes internally and in REST output.

```ts
export type NodeInventory = {
  id: string;
  ip: string;
  ssh: string;
  roles: string[];
};

export type ChainInventory = {
  key: string;
  chainId: string;
  spec: string;
  p2pPort: number;
  rpcPort: number;
  prometheusPort: number;
  binary?: string;
  validators: string[];
  rpcEndpoints: string[];
  prometheusEndpoints: string[];
};

export type ChainStatus = {
  key: string;
  healthy: boolean;
  bestBlock: number | null;
  finalizedBlock: number | null;
  peers: number | null;
  isSyncing: boolean | null;
  runtimeVersion: string | null;
  updatedAt: string;
  stale: boolean;
  errors: string[];
};

export type NodeStatus = {
  id: string;
  ip: string;
  chains: Record<string, ChainStatus>;
  updatedAt: string;
  stale: boolean;
};
```

Prometheus labels:

- `chain`: `main`, `child1`, `child2`, ...
- `node`: `f1`, `f2`, ...
- `role`: `validator`, `gateway`, or `unknown`
- `source`: `rpc`, `substrate_prometheus`, or `monitor`
- `service`: `fishbone-main`, `fishbone-child1`, ...

Initial metrics exposed by monitor:

```text
fishbone_chain_up{chain,node,source}
fishbone_chain_best_block{chain,node,source}
fishbone_chain_finalized_block{chain,node,source}
fishbone_chain_peers{chain,node,source}
fishbone_chain_syncing{chain,node,source}
fishbone_collector_duration_seconds{collector}
fishbone_collector_errors_total{collector,chain,node}
fishbone_inventory_chains_total
fishbone_inventory_nodes_total
```

## Public API

REST:

- `GET /healthz`: process health, no remote calls.
- `GET /api/inventory`: normalized nodes/chains from the active inventory provider.
- `GET /api/status/summary`: cluster summary for dashboard first screen.
- `GET /api/nodes`: latest status grouped by node.
- `GET /api/nodes/:nodeId`: one node detail.
- `GET /api/chains`: latest status grouped by chain.
- `GET /api/chains/:chainKey`: one chain detail across validators.
- `GET /api/collectors`: collector health and last run metadata.
- `GET /metrics`: Prometheus-compatible text exposition.

Streaming:

- `GET /api/events`: SSE stream with `inventory`, `status`, and `collector-error` events.

Do not expose raw private keys, validator seeds, or `deploy/keys/*.env` contents.

## Task 1: Scaffold Monitor Package

**Files:**
- Create: `monitor/package.json`
- Create: `monitor/tsconfig.json`
- Create: `monitor/src/index.ts`
- Create: `monitor/test/smoke.test.ts`

- [ ] **Step 1: Write the failing smoke test**

```ts
// monitor/test/smoke.test.ts
import test from "node:test";
import assert from "node:assert/strict";

test("monitor package exposes a startup marker", async () => {
  const mod = await import("../src/index.js");
  assert.equal(mod.MONITOR_NAME, "fishbone-monitor");
});
```

- [ ] **Step 2: Run the smoke test and verify it fails**

Run:

```bash
cd monitor
npm test -- smoke.test.ts
```

Expected: fail because `monitor/package.json` and compiled `src/index.js` do not exist yet.

- [ ] **Step 3: Add package and TypeScript config**

```json
{
  "name": "fishbone-monitor",
  "version": "0.1.0",
  "type": "module",
  "private": true,
  "scripts": {
    "build": "tsc -p tsconfig.json",
    "test": "npm run build && node --test dist/test/*.test.js",
    "start": "node dist/src/index.js",
    "dev": "tsx src/index.ts"
  },
  "dependencies": {
    "@polkadot/api": "^16.5.6",
    "fastify": "^5.0.0",
    "prom-client": "^15.1.0",
    "toml": "^3.0.0"
  },
  "devDependencies": {
    "@types/node": "^22.0.0",
    "tsx": "^4.19.0",
    "typescript": "^5.7.0"
  }
}
```

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "strict": true,
    "esModuleInterop": true,
    "outDir": "dist",
    "rootDir": ".",
    "skipLibCheck": true
  },
  "include": ["src/**/*.ts", "test/**/*.ts"]
}
```

- [ ] **Step 4: Add minimal entry point**

```ts
// monitor/src/index.ts
export const MONITOR_NAME = "fishbone-monitor";
```

- [ ] **Step 5: Run test and commit**

```bash
cd monitor
npm install
npm test
git add monitor/package.json monitor/package-lock.json monitor/tsconfig.json monitor/src/index.ts monitor/test/smoke.test.ts
git commit -m "feat(monitor): scaffold monitor package"
```

Expected: smoke test passes.

## Task 2: Inventory Loader

**Files:**
- Create: `monitor/src/inventory/types.ts`
- Create: `monitor/src/inventory/tomlInventory.ts`
- Create: `monitor/test/inventory.test.ts`

- [ ] **Step 1: Write tests for `deploy/config.toml` normalization**

Test that `main` has 12 validators, `child5` has `f10-f12`, and `child6` exposes `ws/http`-compatible host/ports based on `10.2.2.x`.

- [ ] **Step 2: Implement `loadTomlInventory(configPath)`**

Implementation rules:

- Parse TOML once per load.
- Convert `[chains.*]` and `[[nodes]]` into normalized arrays.
- For each chain, compute `validators` from node roles.
- For each validator, compute:
  - RPC endpoint: `http://<ip>:<rpcPort>`
  - WS endpoint for frontend display: `ws://<ip>:<rpcPort>`
  - Prometheus endpoint: `http://<ip>:<prometheusPort>/metrics`

- [ ] **Step 3: Run tests**

```bash
cd monitor
npm test -- inventory.test.ts
```

Expected: inventory tests pass and no collector code is required.

## Task 3: Substrate RPC Collector

**Files:**
- Create: `monitor/src/collectors/types.ts`
- Create: `monitor/src/collectors/substrateRpc.ts`
- Create: `monitor/test/substrateRpc.test.ts`

- [ ] **Step 1: Write tests with an injected RPC client**

The collector must accept a small client interface so tests do not open real network sockets:

```ts
export type RpcClient = {
  systemHealth(): Promise<{ peers: number; isSyncing: boolean; shouldHavePeers: boolean }>;
  chainHeader(): Promise<{ number: number }>;
  finalizedHeader(): Promise<{ number: number }>;
  runtimeVersion(): Promise<string>;
};
```

- [ ] **Step 2: Implement collector result envelope**

Every collector returns:

```ts
export type CollectorResult<T> = {
  ok: boolean;
  data: T | null;
  errors: string[];
  startedAt: string;
  finishedAt: string;
};
```

- [ ] **Step 3: Implement `collectSubstrateRpcStatus()`**

Map RPC values into `ChainStatus` fields. On failure, return `ok: false`, keep `data: null`, and put a concise error string in `errors`.

- [ ] **Step 4: Add real Polkadot API adapter**

Keep adapter in the same file initially:

```ts
export async function createPolkadotRpcClient(endpoint: string): Promise<RpcClient> {
  // Use WsProvider for ws:// endpoints or HttpProvider for http:// endpoints.
}
```

The collector should not know about `@polkadot/api`; only the adapter does.

- [ ] **Step 5: Run tests**

```bash
cd monitor
npm test -- substrateRpc.test.ts
```

Expected: success and no real network access in tests.

## Task 4: Prometheus Text Collector

**Files:**
- Create: `monitor/src/collectors/substratePrometheus.ts`
- Create: `monitor/test/substratePrometheus.test.ts`

- [ ] **Step 1: Test parsing of selected Substrate metrics**

Use sample text containing:

```text
substrate_block_height{status="best",chain="fishbone_main"} 120901
substrate_block_height{status="finalized",chain="fishbone_main"} 120899
```

Expected parsed result:

```ts
{ bestBlock: 120901, finalizedBlock: 120899 }
```

- [ ] **Step 2: Implement a minimal parser**

Parse only needed gauge lines first. Ignore unknown metrics. Do not parse arbitrary label values into Prometheus labels for monitor output.

- [ ] **Step 3: Implement endpoint fetch with timeout**

Use built-in `fetch` with `AbortSignal.timeout(3000)`.

- [ ] **Step 4: Run tests**

```bash
cd monitor
npm test -- substratePrometheus.test.ts
```

Expected: parser and timeout behavior pass.

## Task 5: In-Memory State Store

**Files:**
- Create: `monitor/src/state/store.ts`
- Create: `monitor/test/store.test.ts`

- [ ] **Step 1: Test upsert and stale marking**

Status becomes stale when `Date.now() - updatedAt > staleAfterMs`.

- [ ] **Step 2: Implement store**

Provide:

```ts
upsertChainNodeStatus(chain: string, node: string, status: ChainStatus): void
getSummary(): ClusterSummary
getChains(): ChainStatus[]
getNodes(): NodeStatus[]
getCollectorHealth(): CollectorHealth[]
```

- [ ] **Step 3: Run tests**

```bash
cd monitor
npm test -- store.test.ts
```

Expected: store tests pass.

## Task 6: REST, SSE, and Metrics API

**Files:**
- Create: `monitor/src/api/server.ts`
- Create: `monitor/src/metrics/prometheus.ts`
- Create: `monitor/test/api.test.ts`
- Create: `monitor/test/prometheus.test.ts`

- [ ] **Step 1: Test route outputs with an injected store**

Use Fastify injection. Verify:

- `/healthz` returns `{ "ok": true }`
- `/api/inventory` returns chains and nodes
- `/api/status/summary` returns counts
- `/metrics` includes `fishbone_chain_up`

- [ ] **Step 2: Implement Fastify server factory**

```ts
export function buildServer(deps: {
  inventory: InventorySnapshot;
  store: MonitorStore;
  metrics: MetricsRegistry;
}) {
  const app = Fastify({ logger: true });
  // register routes
  return app;
}
```

- [ ] **Step 3: Implement Prometheus registry mapping**

Use `prom-client` gauges. Reset and repopulate gauges from latest state on each `/metrics` request.

- [ ] **Step 4: Implement SSE**

Start with heartbeat and status events every 3 seconds:

```text
event: status
data: {"updatedAt":"...","summary":{...}}
```

- [ ] **Step 5: Run tests**

```bash
cd monitor
npm test -- api.test.ts prometheus.test.ts
```

Expected: API tests pass without network.

## Task 7: Scheduler and Runtime Config

**Files:**
- Create: `monitor/src/config.ts`
- Modify: `monitor/src/index.ts`
- Create: `monitor/test/config.test.ts`

- [ ] **Step 1: Test environment parsing**

Defaults:

```text
FISHBONE_MONITOR_HOST=0.0.0.0
FISHBONE_MONITOR_PORT=18080
FISHBONE_CONFIG_PATH=../deploy/config.toml
FISHBONE_POLL_INTERVAL_MS=5000
FISHBONE_STALE_AFTER_MS=15000
```

- [ ] **Step 2: Implement config parser**

Reject invalid numeric values and invalid relative config path resolution.

- [ ] **Step 3: Wire scheduler**

Every poll interval:

- Reload inventory from TOML.
- For each chain validator endpoint, collect RPC status.
- For each Prometheus endpoint, collect selected metrics.
- Merge results into store.
- Record collector duration and error counters.

- [ ] **Step 4: Run tests and local smoke**

```bash
cd monitor
npm test
FISHBONE_CONFIG_PATH=../deploy/config.toml npm run dev
curl -sf http://127.0.0.1:18080/healthz
```

Expected: health check returns JSON and logs show polling attempts.

## Task 8: Deploy on `bcg`

**Files:**
- Create: `deploy/systemd/fishbone-monitor.service`
- Create: `docs/fishbone-monitor.md`

- [ ] **Step 1: Add systemd unit**

```ini
[Unit]
Description=Fishbone Monitor Backend
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=debian
Group=debian
WorkingDirectory=/home/debian/fishbone/monitor
Environment=FISHBONE_MONITOR_HOST=0.0.0.0
Environment=FISHBONE_MONITOR_PORT=18080
Environment=FISHBONE_CONFIG_PATH=/home/debian/fishbone/deploy/config.toml
Environment=FISHBONE_POLL_INTERVAL_MS=5000
Environment=FISHBONE_STALE_AFTER_MS=15000
ExecStart=/usr/bin/node /home/debian/fishbone/monitor/dist/src/index.js
Restart=always
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

- [ ] **Step 2: Document operator commands**

Include:

```bash
rsync -a --delete monitor/ bcg:/home/debian/fishbone/monitor/
ssh bcg 'cd /home/debian/fishbone/monitor && npm ci && npm run build'
ssh bcg 'sudo cp /home/debian/fishbone/deploy/systemd/fishbone-monitor.service /etc/systemd/system/'
ssh bcg 'sudo systemctl daemon-reload && sudo systemctl enable --now fishbone-monitor'
curl -sf http://192.168.8.41:18080/healthz
curl -sf http://192.168.8.41:18080/metrics | head
```

- [ ] **Step 3: Verify from local machine**

Expected:

- `/healthz` responds.
- `/api/inventory` lists 12 nodes and 7 chains.
- `/api/status/summary` shows at least one healthy chain.
- `/metrics` includes `fishbone_inventory_nodes_total 12`.

## Task 9: Frontend-Ready API Contract Freeze

**Files:**
- Modify: `docs/fishbone-monitor.md`
- Create: `monitor/test/apiContract.test.ts`

- [ ] **Step 1: Add contract snapshot tests**

Use explicit JSON assertions for representative `/api/status/summary`, `/api/chains`, and `/api/nodes` output. Do not snapshot timestamps.

- [ ] **Step 2: Document contract version**

Add response header:

```text
X-Fishbone-Monitor-Api-Version: 1
```

- [ ] **Step 3: Define compatibility policy**

Rules:

- Additive fields are allowed.
- Existing field names and types remain stable within API version 1.
- Breaking response changes require `/api/v2`.
- Prometheus metric names remain stable once documented.

- [ ] **Step 4: Run full monitor verification**

```bash
cd monitor
npm test
npm run build
```

Expected: all tests pass.

## Deployment Safety

- Bind to `0.0.0.0:18080` only if the 192.168.8.0/24 network is trusted enough for lab use.
- For stricter access, bind to `127.0.0.1` and reach it with SSH forwarding:

```bash
ssh -L 18080:127.0.0.1:18080 bcg
```

- Do not expose validator seeds or `deploy/keys`.
- Do not run monitor as root.
- Keep `systemd` restart enabled because collectors depend on network state.

## Future Prometheus/Grafana Integration

When ready, deploy Prometheus on `bcg` with one scrape job for monitor:

```yaml
scrape_configs:
  - job_name: fishbone-monitor
    static_configs:
      - targets: ["127.0.0.1:18080"]
```

Optionally add direct Substrate scrape jobs later:

```yaml
scrape_configs:
  - job_name: fishbone-substrate
    static_configs:
      - targets:
          - "10.2.2.11:9615"
          - "10.2.2.11:9616"
          - "10.2.2.14:9617"
```

The frontend should continue to use monitor REST/SSE APIs for domain-aware topology and business state even after Prometheus/Grafana exists.

## Self-Review

- Spec coverage: The plan covers inventory, collection, state, API, streaming, Prometheus compatibility, deployment, and frontend API stability.
- Placeholder scan: No undefined placeholder sections are left for implementers.
- Type consistency: `NodeInventory`, `ChainInventory`, `ChainStatus`, and `NodeStatus` are used consistently across tasks.
- Scope check: This plan builds the backend only. Frontend dashboard implementation and Prometheus/Grafana deployment are intentionally separate later plans.
