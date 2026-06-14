import test from "node:test";
import assert from "node:assert/strict";

test("monitor package exposes a startup marker", async () => {
  const mod = await import("../src/index.js");
  assert.equal(mod.MONITOR_NAME, "fishbone-monitor");
});
