import test from "node:test";
import assert from "node:assert/strict";
import { resolve } from "node:path";

import { loadMonitorConfig } from "../src/config.js";

test("loads monitor config defaults", () => {
  const config = loadMonitorConfig({}, "/repo/monitor");

  assert.equal(config.host, "0.0.0.0");
  assert.equal(config.port, 18080);
  assert.equal(config.configPath, resolve("/repo/monitor", "..", "deploy", "config.toml"));
  assert.equal(config.pollIntervalMs, 5000);
  assert.equal(config.staleAfterMs, 15000);
});

test("loads monitor config from environment", () => {
  const config = loadMonitorConfig(
    {
      FISHBONE_MONITOR_HOST: "127.0.0.1",
      FISHBONE_MONITOR_PORT: "19090",
      FISHBONE_CONFIG_PATH: "/tmp/config.toml",
      FISHBONE_POLL_INTERVAL_MS: "1000",
      FISHBONE_STALE_AFTER_MS: "3000",
    },
    "/repo/monitor",
  );

  assert.equal(config.host, "127.0.0.1");
  assert.equal(config.port, 19090);
  assert.equal(config.configPath, "/tmp/config.toml");
  assert.equal(config.pollIntervalMs, 1000);
  assert.equal(config.staleAfterMs, 3000);
});

test("rejects invalid numeric config", () => {
  assert.throws(
    () => loadMonitorConfig({ FISHBONE_MONITOR_PORT: "not-a-port" }, "/repo/monitor"),
    /FISHBONE_MONITOR_PORT/,
  );
});
