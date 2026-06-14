import test from "node:test";
import assert from "node:assert/strict";

import { MonitorStore } from "../src/state/store.js";
import type { ChainStatus } from "../src/status/types.js";

function status(overrides: Partial<ChainStatus> = {}): ChainStatus {
  return {
    key: "main",
    nodeId: "f1",
    healthy: true,
    bestBlock: 100,
    finalizedBlock: 98,
    peers: 11,
    isSyncing: false,
    runtimeVersion: "fishbone-1",
    updatedAt: "2026-06-12T00:00:00.000Z",
    stale: false,
    errors: [],
    ...overrides,
  };
}

test("stores latest status by chain and node", () => {
  const store = new MonitorStore({
    staleAfterMs: 15_000,
    nowMs: () => Date.parse("2026-06-12T00:00:05.000Z"),
  });

  store.upsertChainNodeStatus("main", "f1", status());
  store.upsertChainNodeStatus("child1", "f1", status({ key: "child1", peers: 2 }));

  assert.equal(store.getChains().length, 2);
  assert.equal(store.getNodes().length, 1);
  assert.equal(store.getNodes()[0]?.chains.child1?.peers, 2);
});

test("marks status as stale based on updatedAt", () => {
  const store = new MonitorStore({
    staleAfterMs: 15_000,
    nowMs: () => Date.parse("2026-06-12T00:00:20.000Z"),
  });

  store.upsertChainNodeStatus("main", "f1", status());

  const [chain] = store.getChains();
  assert.equal(chain?.stale, true);
  assert.equal(store.getNodes()[0]?.stale, true);
});

test("summarizes cluster health", () => {
  const store = new MonitorStore({
    staleAfterMs: 15_000,
    nowMs: () => Date.parse("2026-06-12T00:00:05.000Z"),
  });

  store.upsertChainNodeStatus("main", "f1", status());
  store.upsertChainNodeStatus("main", "f2", status({ nodeId: "f2", healthy: false, errors: ["down"] }));

  assert.deepEqual(store.getSummary(), {
    totalChains: 1,
    healthyChains: 1,
    totalNodes: 2,
    healthyNodes: 1,
    staleNodes: 0,
    errorCount: 1,
  });
});

test("stores and summarizes cached log snapshots", () => {
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

  assert.deepEqual(store.getLogSummaries(), [
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
  assert.equal(store.getLogSnapshot("f1", "main")?.lines.length, 2);
});
