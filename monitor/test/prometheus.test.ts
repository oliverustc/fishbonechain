import test from "node:test";
import assert from "node:assert/strict";

import { createMetricsRegistry, renderPrometheusMetrics } from "../src/metrics/prometheus.js";
import { MonitorStore } from "../src/state/store.js";
import type { InventorySnapshot } from "../src/inventory/types.js";

const inventory: InventorySnapshot = {
  name: "fishbone-testnet",
  binary: "",
  baseDir: "",
  logDir: "",
  nodes: [{ id: "f1", ip: "10.2.2.11", ssh: "f1", roles: ["main"] }],
  chains: [
    {
      key: "main",
      chainId: "fishbone_main",
      spec: "",
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

test("renders prometheus compatible monitor metrics", async () => {
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

  const text = await renderPrometheusMetrics(createMetricsRegistry(), inventory, store);

  assert.match(text, /fishbone_inventory_nodes_total 1/);
  assert.match(text, /fishbone_inventory_chains_total 1/);
  assert.match(text, /fishbone_chain_best_block\{chain="main",node="f1",source="monitor"\} 12/);
});
