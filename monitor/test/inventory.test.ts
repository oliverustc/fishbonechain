import test from "node:test";
import assert from "node:assert/strict";
import { resolve } from "node:path";

import { loadTomlInventory } from "../src/inventory/tomlInventory.js";

const CONFIG_PATH = resolve("..", "deploy", "config.toml");

test("loads FishboneChain nodes and chains from deploy config", async () => {
  const inventory = await loadTomlInventory(CONFIG_PATH);

  assert.equal(inventory.nodes.length, 12);
  assert.equal(inventory.chains.length, 7);
  assert.equal(inventory.gateway?.ssh, "bcg");
  assert.equal(inventory.gateway?.ip, "192.168.8.41");

  const main = inventory.chains.find((chain) => chain.key === "main");
  assert.ok(main);
  assert.deepEqual(
    main.validators,
    ["f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12"],
  );
  assert.equal(main.rpcEndpoints[0], "http://10.2.2.11:9944");
  assert.equal(main.wsEndpoints[0], "ws://10.2.2.11:9944");
  assert.equal(main.prometheusEndpoints[0], "http://10.2.2.11:9615/metrics");
});

test("normalizes child chain validators and endpoints", async () => {
  const inventory = await loadTomlInventory(CONFIG_PATH);

  const child5 = inventory.chains.find((chain) => chain.key === "child5");
  assert.ok(child5);
  assert.deepEqual(child5.validators, ["f10", "f11", "f12"]);
  assert.deepEqual(child5.rpcEndpoints, [
    "http://10.2.2.20:9949",
    "http://10.2.2.21:9949",
    "http://10.2.2.22:9949",
  ]);

  const child6 = inventory.chains.find((chain) => chain.key === "child6");
  assert.ok(child6);
  assert.deepEqual(child6.validators, ["f1", "f2", "f3", "f4", "f5"]);
  assert.equal(child6.chainId, "fishbone_child_6");
  assert.equal(child6.binary, undefined);
  assert.equal(child6.wsEndpoints[0], "ws://10.2.2.11:9950");
  assert.equal(child6.prometheusEndpoints[0], "http://10.2.2.11:9621/metrics");
});
