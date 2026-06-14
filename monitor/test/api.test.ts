import test from "node:test";
import assert from "node:assert/strict";

import { buildServer } from "../src/api/server.js";
import { createMetricsRegistry } from "../src/metrics/prometheus.js";
import { MonitorStore } from "../src/state/store.js";
import type { InventorySnapshot } from "../src/inventory/types.js";
import type { ChainStatus } from "../src/status/types.js";

const inventory: InventorySnapshot = {
  name: "fishbone-testnet",
  binary: "/home/debian/fishbone/bin/fishbone-node",
  baseDir: "/home/debian/fishbone",
  logDir: "/home/debian/fishbone/logs",
  gateway: { ssh: "bcg", ip: "192.168.8.41" },
  nodes: [{ id: "f1", ip: "10.2.2.11", ssh: "f1", roles: ["main"] }],
  chains: [
    {
      key: "main",
      chainId: "fishbone_main",
      spec: "specs/main-custom-raw.json",
      p2pPort: 30333,
      rpcPort: 9944,
      prometheusPort: 9615,
      validators: ["f1"],
      rpcEndpoints: ["http://10.2.2.11:9944"],
      wsEndpoints: ["ws://10.2.2.11:9944"],
      prometheusEndpoints: ["http://10.2.2.11:9615/metrics"],
    },
  ],
  loadedAt: "2026-06-12T00:00:00.000Z",
};

function chainStatus(): ChainStatus {
  return {
    key: "main",
    nodeId: "f1",
    healthy: true,
    bestBlock: 12,
    finalizedBlock: 10,
    peers: 3,
    isSyncing: false,
    runtimeVersion: "fishbone-1",
    updatedAt: "2026-06-12T00:00:00.000Z",
    stale: false,
    errors: [],
  };
}

test("serves health, inventory, summary, nodes, chains and metrics", async () => {
  const store = new MonitorStore({ staleAfterMs: 15_000, nowMs: () => Date.parse("2026-06-12T00:00:05.000Z") });
  store.upsertChainNodeStatus("main", "f1", chainStatus());
  const app = buildServer({ inventory, store, metrics: createMetricsRegistry() });

  const health = await app.inject({ method: "GET", url: "/healthz" });
  assert.equal(health.statusCode, 200);
  assert.deepEqual(JSON.parse(health.body), { ok: true, name: "fishbone-monitor" });

  const inv = await app.inject({ method: "GET", url: "/api/inventory" });
  assert.equal(inv.headers["x-fishbone-monitor-api-version"], "1");
  assert.equal(JSON.parse(inv.body).nodes.length, 1);

  const summary = await app.inject({ method: "GET", url: "/api/status/summary" });
  assert.equal(JSON.parse(summary.body).totalNodes, 1);

  const nodes = await app.inject({ method: "GET", url: "/api/nodes" });
  assert.equal(JSON.parse(nodes.body)[0].id, "f1");

  const chains = await app.inject({ method: "GET", url: "/api/chains" });
  assert.equal(JSON.parse(chains.body)[0].key, "main");

  const metrics = await app.inject({ method: "GET", url: "/metrics" });
  assert.equal(metrics.statusCode, 200);
  assert.match(metrics.body, /fishbone_chain_up\{chain="main",node="f1",source="monitor"\} 1/);

  await app.close();
});
