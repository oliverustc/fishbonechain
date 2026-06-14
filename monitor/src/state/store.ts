import type { CollectorHealth } from "../collectors/types.js";
import type { LogSnapshot, LogSummary } from "../logs/types.js";
import type { ChainStatus, NodeStatus } from "../status/types.js";

export type ClusterSummary = {
  totalChains: number;
  healthyChains: number;
  totalNodes: number;
  healthyNodes: number;
  staleNodes: number;
  errorCount: number;
};

export type MonitorStoreOptions = {
  staleAfterMs: number;
  nowMs?: () => number;
};

export class MonitorStore {
  private readonly staleAfterMs: number;
  private readonly nowMs: () => number;
  private readonly statuses = new Map<string, ChainStatus>();
  private readonly collectorHealth = new Map<string, CollectorHealth>();
  private readonly logs = new Map<string, LogSnapshot>();

  constructor(options: MonitorStoreOptions) {
    this.staleAfterMs = options.staleAfterMs;
    this.nowMs = options.nowMs ?? (() => Date.now());
  }

  upsertChainNodeStatus(chain: string, node: string, status: ChainStatus): void {
    this.statuses.set(this.key(chain, node), { ...status, key: chain, nodeId: node });
  }

  recordCollectorHealth(health: CollectorHealth): void {
    this.collectorHealth.set(health.name, { ...health, errors: [...health.errors] });
  }

  upsertLogSnapshot(snapshot: LogSnapshot): void {
    this.logs.set(this.key(snapshot.chainKey, snapshot.nodeId), {
      ...snapshot,
      lines: [...snapshot.lines],
      errors: [...snapshot.errors],
    });
  }

  getLogSnapshot(nodeId: string, chainKey: string): LogSnapshot | undefined {
    const snapshot = this.logs.get(this.key(chainKey, nodeId));
    return snapshot
      ? { ...snapshot, lines: [...snapshot.lines], errors: [...snapshot.errors] }
      : undefined;
  }

  getLogSummaries(): LogSummary[] {
    return [...this.logs.values()]
      .map((snapshot) => ({
        nodeId: snapshot.nodeId,
        chainKey: snapshot.chainKey,
        path: snapshot.path,
        updatedAt: snapshot.updatedAt,
        ok: snapshot.ok,
        lineCount: snapshot.lines.length,
        errors: [...snapshot.errors],
      }))
      .sort((a, b) => `${a.chainKey}:${a.nodeId}`.localeCompare(`${b.chainKey}:${b.nodeId}`));
  }

  getSummary(): ClusterSummary {
    const chains = this.getChains();
    const nodes = this.getNodes();

    return {
      totalChains: new Set(chains.map((status) => status.key)).size,
      healthyChains: this.countHealthyChains(chains),
      totalNodes: nodes.length,
      healthyNodes: nodes.filter((node) => !node.stale && this.nodeHealthy(node)).length,
      staleNodes: nodes.filter((node) => node.stale).length,
      errorCount: chains.reduce((sum, status) => sum + status.errors.length, 0),
    };
  }

  getChains(): ChainStatus[] {
    return [...this.statuses.values()]
      .map((status) => this.withStale(status))
      .sort((a, b) => `${a.key}:${a.nodeId}`.localeCompare(`${b.key}:${b.nodeId}`));
  }

  getNodes(): NodeStatus[] {
    const byNode = new Map<string, Record<string, ChainStatus>>();
    for (const status of this.getChains()) {
      const chains = byNode.get(status.nodeId) ?? {};
      chains[status.key] = status;
      byNode.set(status.nodeId, chains);
    }

    return [...byNode.entries()]
      .map(([id, chains]) => {
        const chainValues = Object.values(chains);
        const updatedAt = this.latestTimestamp(chainValues);
        return {
          id,
          ip: "",
          chains,
          updatedAt,
          stale: chainValues.every((status) => status.stale),
        };
      })
      .sort((a, b) => a.id.localeCompare(b.id, undefined, { numeric: true }));
  }

  getCollectorHealth(): CollectorHealth[] {
    return [...this.collectorHealth.values()].sort((a, b) => a.name.localeCompare(b.name));
  }

  private key(chain: string, node: string): string {
    return `${chain}:${node}`;
  }

  private withStale(status: ChainStatus): ChainStatus {
    const updatedMs = Date.parse(status.updatedAt);
    const stale = !Number.isFinite(updatedMs) || this.nowMs() - updatedMs > this.staleAfterMs;
    return { ...status, stale };
  }

  private latestTimestamp(statuses: ChainStatus[]): string {
    const latest = statuses
      .map((status) => Date.parse(status.updatedAt))
      .filter(Number.isFinite)
      .sort((a, b) => b - a)[0];
    return latest === undefined ? new Date(this.nowMs()).toISOString() : new Date(latest).toISOString();
  }

  private countHealthyChains(statuses: ChainStatus[]): number {
    const byChain = new Map<string, ChainStatus[]>();
    for (const status of statuses) {
      const chainStatuses = byChain.get(status.key) ?? [];
      chainStatuses.push(status);
      byChain.set(status.key, chainStatuses);
    }
    return [...byChain.values()].filter((chainStatuses) =>
      chainStatuses.some((status) => status.healthy && !status.stale),
    ).length;
  }

  private nodeHealthy(node: NodeStatus): boolean {
    return Object.values(node.chains).some((status) => status.healthy && !status.stale);
  }
}
