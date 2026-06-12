import { buildServer } from "./api/server.js";
import { loadMonitorConfig, type MonitorConfig } from "./config.js";
import { loadTomlInventory } from "./inventory/tomlInventory.js";
import { createMetricsRegistry } from "./metrics/prometheus.js";
import { createScheduler } from "./scheduler.js";
import { MonitorStore } from "./state/store.js";

export const MONITOR_NAME = "fishbone-monitor";

export async function createMonitorRuntime(config: MonitorConfig) {
  const inventory = await loadTomlInventory(config.configPath);
  const store = new MonitorStore({ staleAfterMs: config.staleAfterMs });
  const metrics = createMetricsRegistry();
  const scheduler = createScheduler({
    inventory,
    store,
    pollIntervalMs: config.pollIntervalMs,
    logCollectionIntervalMs: config.logCollectionIntervalMs,
    logMaxConcurrency: config.logMaxConcurrency,
  });
  const server = buildServer({ inventory, store, metrics });

  return { inventory, store, metrics, scheduler, server };
}

async function main() {
  const config = loadMonitorConfig();
  const runtime = await createMonitorRuntime(config);
  runtime.scheduler.start();
  await runtime.server.listen({ host: config.host, port: config.port });

  const shutdown = async () => {
    runtime.scheduler.stop();
    await runtime.server.close();
  };
  process.once("SIGINT", () => {
    void shutdown().then(() => process.exit(0));
  });
  process.once("SIGTERM", () => {
    void shutdown().then(() => process.exit(0));
  });
}

if (process.argv[1] && import.meta.url.endsWith(process.argv[1])) {
  void main();
}
