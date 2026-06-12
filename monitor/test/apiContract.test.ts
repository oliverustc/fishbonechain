import test from "node:test";
import assert from "node:assert/strict";

import { buildServer } from "../src/api/server.js";
import { createMetricsRegistry } from "../src/metrics/prometheus.js";
import { MonitorStore } from "../src/state/store.js";
import type { InventorySnapshot } from "../src/inventory/types.js";

const inventory: InventorySnapshot = {
  name: "fishbone-testnet",
  binary: "/home/debian/fishbone/bin/fishbone-node",
  baseDir: "/home/debian/fishbone",
  logDir: "/home/debian/fishbone/logs",
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

test("api v1 contract keeps summary, chain and node shapes stable", async () => {
  const store = new MonitorStore({ staleAfterMs: 15_000, nowMs: () => Date.parse("2026-06-12T00:00:05.000Z") });
  store.upsertChainNodeStatus("main", "f1", {
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
  });
  const app = buildServer({ inventory, store, metrics: createMetricsRegistry() });

  const summary = await app.inject({ method: "GET", url: "/api/status/summary" });
  assert.equal(summary.headers["x-fishbone-monitor-api-version"], "1");
  assert.deepEqual(JSON.parse(summary.body), {
    totalChains: 1,
    healthyChains: 1,
    totalNodes: 1,
    healthyNodes: 1,
    staleNodes: 0,
    errorCount: 0,
  });

  const chains = await app.inject({ method: "GET", url: "/api/chains" });
  assert.deepEqual(JSON.parse(chains.body)[0], {
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
  });

  const nodes = await app.inject({ method: "GET", url: "/api/nodes" });
  const node = JSON.parse(nodes.body)[0];
  assert.equal(node.id, "f1");
  assert.equal(node.stale, false);
  assert.equal(node.chains.main.bestBlock, 12);

  await app.close();
});
