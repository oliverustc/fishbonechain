import test from "node:test";
import assert from "node:assert/strict";

import { buildServer } from "../src/api/server.js";
import { createMetricsRegistry } from "../src/metrics/prometheus.js";
import { MonitorStore } from "../src/state/store.js";
import type { InventorySnapshot } from "../src/inventory/types.js";

const inventory: InventorySnapshot = {
  name: "fishbone-testnet",
  binary: "",
  baseDir: "",
  logDir: "/home/debian/fishbone/logs",
  nodes: [],
  chains: [],
  loadedAt: "2026-06-12T00:00:00.000Z",
};

test("serves cached log summaries and snapshots", async () => {
  const store = new MonitorStore({ staleAfterMs: 15_000 });
  store.upsertLogSnapshot({
    nodeId: "f1",
    chainKey: "main",
    path: "/home/debian/fishbone/logs/main.log",
    updatedAt: "2026-06-12T00:00:00.000Z",
    ok: true,
    lines: ["line 1", "line 2"],
    errors: [],
  });
  const app = buildServer({ inventory, store, metrics: createMetricsRegistry() });

  const summaries = await app.inject({ method: "GET", url: "/api/logs" });
  assert.equal(summaries.statusCode, 200);
  assert.deepEqual(JSON.parse(summaries.body), [
    {
      nodeId: "f1",
      chainKey: "main",
      path: "/home/debian/fishbone/logs/main.log",
      updatedAt: "2026-06-12T00:00:00.000Z",
      ok: true,
      lineCount: 2,
      errors: [],
    },
  ]);

  const snapshot = await app.inject({ method: "GET", url: "/api/logs/f1/main" });
  assert.equal(snapshot.statusCode, 200);
  assert.deepEqual(JSON.parse(snapshot.body).lines, ["line 1", "line 2"]);

  const missing = await app.inject({ method: "GET", url: "/api/logs/f9/main" });
  assert.equal(missing.statusCode, 404);
  assert.deepEqual(JSON.parse(missing.body), { error: "log cache not found" });

  await app.close();
});
