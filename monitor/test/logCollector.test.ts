import test from "node:test";
import assert from "node:assert/strict";

import { collectLogs } from "../src/collectors/logCollector.js";
import type { InventorySnapshot } from "../src/inventory/types.js";

const inventory: InventorySnapshot = {
  name: "fishbone-testnet",
  binary: "",
  baseDir: "/home/debian/fishbone",
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
      prometheusEndpoints: ["http://10.2.2.11:9615/metrics"],
    },
  ],
  loadedAt: "2026-06-12T00:00:00.000Z",
};

test("collects bounded log snapshots for node chain roles", async () => {
  const result = await collectLogs({
    inventory,
    maxLines: 3,
    runCommand: async (host, command) => {
      assert.equal(host, "debian@10.2.2.11");
      assert.equal(command, "tail -n 3 /home/debian/fishbone/logs/main.log");
      return { code: 0, stdout: "a\nb\n", stderr: "" };
    },
    now: () => "2026-06-12T00:00:00.000Z",
  });

  assert.deepEqual(result.errors, []);
  assert.equal(result.snapshots[0]?.ok, true);
  assert.deepEqual(result.snapshots[0]?.lines, ["a", "b"]);
});

test("allows overriding the ssh user while still using inventory ip addresses", async () => {
  await collectLogs({
    inventory,
    maxLines: 1,
    sshUser: "fishbone",
    runCommand: async (host) => {
      assert.equal(host, "fishbone@10.2.2.11");
      return { code: 0, stdout: "", stderr: "" };
    },
    now: () => "2026-06-12T00:00:00.000Z",
  });
});

test("limits concurrent ssh commands while collecting logs", async () => {
  let active = 0;
  let peak = 0;
  const result = await collectLogs({
    inventory: {
      ...inventory,
      nodes: [
        { id: "f1", ip: "10.2.2.11", ssh: "f1", roles: ["main", "child1"] },
        { id: "f2", ip: "10.2.2.12", ssh: "f2", roles: ["main", "child1"] },
      ],
    },
    maxLines: 1,
    maxConcurrency: 2,
    runCommand: async () => {
      active += 1;
      peak = Math.max(peak, active);
      await new Promise((resolve) => setTimeout(resolve, 5));
      active -= 1;
      return { code: 0, stdout: "", stderr: "" };
    },
    now: () => "2026-06-12T00:00:00.000Z",
  });

  assert.equal(result.snapshots.length, 4);
  assert.equal(peak, 2);
});

test("stores failed log collection as a cache snapshot", async () => {
  const result = await collectLogs({
    inventory,
    maxLines: 100,
    runCommand: async () => ({ code: 1, stdout: "", stderr: "missing" }),
    now: () => "2026-06-12T00:00:00.000Z",
  });

  assert.equal(result.errors.length, 1);
  assert.deepEqual(result.snapshots[0], {
    nodeId: "f1",
    chainKey: "main",
    path: "/home/debian/fishbone/logs/main.log",
    updatedAt: "2026-06-12T00:00:00.000Z",
    ok: false,
    lines: [],
    errors: ["missing"],
  });
});
