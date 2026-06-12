import test from "node:test";
import assert from "node:assert/strict";

import { createScheduler } from "../src/scheduler.js";
import { MonitorStore } from "../src/state/store.js";
import type { InventorySnapshot } from "../src/inventory/types.js";
import type { RpcClient } from "../src/collectors/substrateRpc.js";

const inventory: InventorySnapshot = {
  name: "fishbone-testnet",
  binary: "",
  baseDir: "",
  logDir: "/home/debian/fishbone/logs",
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
      prometheusEndpoints: [],
    },
  ],
  loadedAt: "2026-06-12T00:00:00.000Z",
};

const rpcClient: RpcClient = {
  async systemHealth() {
    return { peers: 1, isSyncing: false, shouldHavePeers: true };
  },
  async chainHeader() {
    return { number: 12 };
  },
  async finalizedHeader() {
    return { number: 10 };
  },
  async runtimeVersion() {
    return "fishbone-1";
  },
};

test("scheduler stores cached logs collected during poll", async () => {
  const store = new MonitorStore({ staleAfterMs: 15_000 });
  const scheduler = createScheduler({
    inventory,
    store,
    pollIntervalMs: 5000,
    createRpcClient: () => rpcClient,
    collectLogs: async () => ({
      snapshots: [
        {
          nodeId: "f1",
          chainKey: "main",
          path: "/home/debian/fishbone/logs/main.log",
          updatedAt: "2026-06-12T00:00:00.000Z",
          ok: true,
          lines: ["cached line"],
          errors: [],
        },
      ],
      errors: [],
    }),
  });

  await scheduler.pollOnce();

  assert.deepEqual(store.getLogSnapshot("f1", "main")?.lines, ["cached line"]);
});

test("scheduler collects logs on a slower interval than status polling", async () => {
  let now = 0;
  let logCollections = 0;
  const store = new MonitorStore({ staleAfterMs: 15_000 });
  const scheduler = createScheduler({
    inventory,
    store,
    pollIntervalMs: 5000,
    logCollectionIntervalMs: 60_000,
    nowMs: () => now,
    createRpcClient: () => rpcClient,
    collectLogs: async () => {
      logCollections += 1;
      return { snapshots: [], errors: [] };
    },
  });

  await scheduler.pollOnce();
  now = 5000;
  await scheduler.pollOnce();
  now = 60_000;
  await scheduler.pollOnce();

  assert.equal(logCollections, 2);
});
