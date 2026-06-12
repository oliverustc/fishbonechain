import test from "node:test";
import assert from "node:assert/strict";

import {
  collectSubstrateRpcStatus,
  type RpcClient,
} from "../src/collectors/substrateRpc.js";

test("collects Substrate RPC status into a stable chain status shape", async () => {
  const client: RpcClient = {
    async systemHealth() {
      return { peers: 11, isSyncing: false, shouldHavePeers: true };
    },
    async chainHeader() {
      return { number: 120901 };
    },
    async finalizedHeader() {
      return { number: 120899 };
    },
    async runtimeVersion() {
      return "fishbone-42";
    },
  };

  const result = await collectSubstrateRpcStatus({
    chainKey: "main",
    nodeId: "f1",
    client,
    now: () => "2026-06-12T00:00:00.000Z",
  });

  assert.equal(result.ok, true);
  assert.deepEqual(result.errors, []);
  assert.deepEqual(result.data, {
    key: "main",
    nodeId: "f1",
    healthy: true,
    bestBlock: 120901,
    finalizedBlock: 120899,
    peers: 11,
    isSyncing: false,
    runtimeVersion: "fishbone-42",
    updatedAt: "2026-06-12T00:00:00.000Z",
    stale: false,
    errors: [],
  });
});

test("returns a structured collector error when RPC calls fail", async () => {
  const client: RpcClient = {
    async systemHealth() {
      throw new Error("connection refused");
    },
    async chainHeader() {
      return { number: 0 };
    },
    async finalizedHeader() {
      return { number: 0 };
    },
    async runtimeVersion() {
      return "unused";
    },
  };

  const result = await collectSubstrateRpcStatus({
    chainKey: "child1",
    nodeId: "f1",
    client,
    now: () => "2026-06-12T00:00:00.000Z",
  });

  assert.equal(result.ok, false);
  assert.equal(result.data, null);
  assert.match(result.errors[0], /connection refused/);
  assert.equal(result.startedAt, "2026-06-12T00:00:00.000Z");
  assert.equal(result.finishedAt, "2026-06-12T00:00:00.000Z");
});
