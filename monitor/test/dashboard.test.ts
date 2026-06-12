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
  logDir: "",
  nodes: [],
  chains: [],
  loadedAt: "2026-06-12T00:00:00.000Z",
};

test("serves the static dashboard shell and assets", async () => {
  const app = buildServer({
    inventory,
    store: new MonitorStore({ staleAfterMs: 15_000 }),
    metrics: createMetricsRegistry(),
  });

  const page = await app.inject({ method: "GET", url: "/" });
  assert.equal(page.statusCode, 200);
  assert.equal(page.headers["content-type"], "text/html; charset=utf-8");
  assert.match(page.body, /Fishbone Monitor/);
  assert.match(page.body, /assets\/app.js/);

  const script = await app.inject({ method: "GET", url: "/assets/app.js" });
  assert.equal(script.statusCode, 200);
  assert.equal(script.headers["content-type"], "text/javascript; charset=utf-8");
  assert.match(script.body, /\/api\/status\/summary/);

  const css = await app.inject({ method: "GET", url: "/assets/styles.css" });
  assert.equal(css.statusCode, 200);
  assert.equal(css.headers["content-type"], "text/css; charset=utf-8");

  await app.close();
});
