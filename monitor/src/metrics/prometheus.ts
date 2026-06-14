import { Gauge, Registry } from "prom-client";

import type { InventorySnapshot } from "../inventory/types.js";
import type { MonitorStore } from "../state/store.js";

export type MetricsRegistry = {
  registry: Registry;
  chainUp: Gauge<string>;
  chainBestBlock: Gauge<string>;
  chainFinalizedBlock: Gauge<string>;
  chainPeers: Gauge<string>;
  chainSyncing: Gauge<string>;
  inventoryNodesTotal: Gauge<string>;
  inventoryChainsTotal: Gauge<string>;
  collectorDurationSeconds: Gauge<string>;
  collectorErrorsTotal: Gauge<string>;
};

export function createMetricsRegistry(): MetricsRegistry {
  const registry = new Registry();

  const chainUp = new Gauge({
    name: "fishbone_chain_up",
    help: "Whether the monitor considers the chain endpoint healthy",
    labelNames: ["chain", "node", "source"] as const,
    registers: [registry],
  });
  const chainBestBlock = new Gauge({
    name: "fishbone_chain_best_block",
    help: "Best block observed by the monitor",
    labelNames: ["chain", "node", "source"] as const,
    registers: [registry],
  });
  const chainFinalizedBlock = new Gauge({
    name: "fishbone_chain_finalized_block",
    help: "Finalized block observed by the monitor",
    labelNames: ["chain", "node", "source"] as const,
    registers: [registry],
  });
  const chainPeers = new Gauge({
    name: "fishbone_chain_peers",
    help: "Peer count observed by the monitor",
    labelNames: ["chain", "node", "source"] as const,
    registers: [registry],
  });
  const chainSyncing = new Gauge({
    name: "fishbone_chain_syncing",
    help: "Whether the endpoint is syncing",
    labelNames: ["chain", "node", "source"] as const,
    registers: [registry],
  });
  const inventoryNodesTotal = new Gauge({
    name: "fishbone_inventory_nodes_total",
    help: "Number of nodes in monitor inventory",
    registers: [registry],
  });
  const inventoryChainsTotal = new Gauge({
    name: "fishbone_inventory_chains_total",
    help: "Number of chains in monitor inventory",
    registers: [registry],
  });
  const collectorDurationSeconds = new Gauge({
    name: "fishbone_collector_duration_seconds",
    help: "Last collector duration in seconds",
    labelNames: ["collector"] as const,
    registers: [registry],
  });
  const collectorErrorsTotal = new Gauge({
    name: "fishbone_collector_errors_total",
    help: "Current collector error count by collector, chain and node",
    labelNames: ["collector", "chain", "node"] as const,
    registers: [registry],
  });

  return {
    registry,
    chainUp,
    chainBestBlock,
    chainFinalizedBlock,
    chainPeers,
    chainSyncing,
    inventoryNodesTotal,
    inventoryChainsTotal,
    collectorDurationSeconds,
    collectorErrorsTotal,
  };
}

export async function renderPrometheusMetrics(
  metrics: MetricsRegistry,
  inventory: InventorySnapshot,
  store: MonitorStore,
): Promise<string> {
  metrics.registry.resetMetrics();
  metrics.inventoryNodesTotal.set(inventory.nodes.length);
  metrics.inventoryChainsTotal.set(inventory.chains.length);

  for (const status of store.getChains()) {
    const labels = { chain: status.key, node: status.nodeId, source: "monitor" };
    metrics.chainUp.set(labels, status.healthy && !status.stale ? 1 : 0);
    if (status.bestBlock !== null) {
      metrics.chainBestBlock.set(labels, status.bestBlock);
    }
    if (status.finalizedBlock !== null) {
      metrics.chainFinalizedBlock.set(labels, status.finalizedBlock);
    }
    if (status.peers !== null) {
      metrics.chainPeers.set(labels, status.peers);
    }
    if (status.isSyncing !== null) {
      metrics.chainSyncing.set(labels, status.isSyncing ? 1 : 0);
    }
  }

  for (const health of store.getCollectorHealth()) {
    if (health.lastDurationMs !== null) {
      metrics.collectorDurationSeconds.set({ collector: health.name }, health.lastDurationMs / 1000);
    }
    for (const error of health.errors) {
      const [chain = "unknown", node = "unknown"] = error.split(":");
      metrics.collectorErrorsTotal.set({ collector: health.name, chain, node }, 1);
    }
  }

  return metrics.registry.metrics();
}
