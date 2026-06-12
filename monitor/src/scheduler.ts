import {
  collectSubstrateRpcStatus,
  createHttpJsonRpcClient,
  type RpcClient,
} from "./collectors/substrateRpc.js";
import { collectLogs, type CollectLogsResult } from "./collectors/logCollector.js";
import { collectSubstratePrometheus } from "./collectors/substratePrometheus.js";
import type { InventorySnapshot } from "./inventory/types.js";
import type { MonitorStore } from "./state/store.js";

export type SchedulerOptions = {
  inventory: InventorySnapshot;
  store: MonitorStore;
  pollIntervalMs: number;
  createRpcClient?: (endpoint: string) => RpcClient;
  collectLogs?: (input: {
    inventory: InventorySnapshot;
    maxLines: number;
    maxConcurrency: number;
  }) => Promise<CollectLogsResult>;
  logCollectionIntervalMs?: number;
  logMaxLines?: number;
  logMaxConcurrency?: number;
  nowMs?: () => number;
};

export type MonitorScheduler = {
  pollOnce(): Promise<void>;
  start(): void;
  stop(): void;
};

export function createScheduler(options: SchedulerOptions): MonitorScheduler {
  let timer: NodeJS.Timeout | null = null;
  let inFlight: Promise<void> | null = null;
  const createRpcClient = options.createRpcClient ?? createHttpJsonRpcClient;
  const collectLogSnapshots = options.collectLogs ?? collectLogs;
  const nowMs = options.nowMs ?? Date.now;
  const logCollectionIntervalMs = options.logCollectionIntervalMs ?? 60_000;
  const logMaxLines = options.logMaxLines ?? 300;
  const logMaxConcurrency = options.logMaxConcurrency ?? 4;
  let lastLogCollectedAtMs: number | null = null;

  async function runPollOnce(): Promise<void> {
    const startedAt = nowMs();
    const errors: string[] = [];

    await Promise.all(
      options.inventory.chains.flatMap((chain) =>
        chain.validators.map(async (nodeId, index) => {
          const rpcEndpoint = chain.rpcEndpoints[index];
          if (!rpcEndpoint) return;

          const rpcResult = await collectSubstrateRpcStatus({
            chainKey: chain.key,
            nodeId,
            client: createRpcClient(rpcEndpoint),
          });
          if (rpcResult.data) {
            options.store.upsertChainNodeStatus(chain.key, nodeId, rpcResult.data);
          }
          for (const error of rpcResult.errors) {
            errors.push(`${chain.key}:${nodeId}:${error}`);
          }

          const prometheusEndpoint = chain.prometheusEndpoints[index];
          if (prometheusEndpoint) {
            const promResult = await collectSubstratePrometheus({ endpoint: prometheusEndpoint });
            if (promResult.data) {
              const current = options.store
                .getChains()
                .find((status) => status.key === chain.key && status.nodeId === nodeId);
              if (current) {
                options.store.upsertChainNodeStatus(chain.key, nodeId, {
                  ...current,
                  bestBlock: promResult.data.bestBlock ?? current.bestBlock,
                  finalizedBlock: promResult.data.finalizedBlock ?? current.finalizedBlock,
                });
              }
            }
            for (const error of promResult.errors) {
              errors.push(`${chain.key}:${nodeId}:${error}`);
            }
          }
        }),
      ),
    );

    const shouldCollectLogs =
      lastLogCollectedAtMs === null || startedAt - lastLogCollectedAtMs >= logCollectionIntervalMs;
    if (shouldCollectLogs) {
      lastLogCollectedAtMs = startedAt;
      const logResult = await collectLogSnapshots({
        inventory: options.inventory,
        maxLines: logMaxLines,
        maxConcurrency: logMaxConcurrency,
      });
      for (const snapshot of logResult.snapshots) {
        options.store.upsertLogSnapshot(snapshot);
      }
      errors.push(...logResult.errors);
    }

    options.store.recordCollectorHealth({
      name: "scheduler",
      lastStartedAt: new Date(startedAt).toISOString(),
      lastFinishedAt: new Date().toISOString(),
      lastDurationMs: nowMs() - startedAt,
      lastOk: errors.length === 0,
      errors,
    });
  }

  function pollOnce(): Promise<void> {
    if (inFlight) return inFlight;
    inFlight = runPollOnce().finally(() => {
      inFlight = null;
    });
    return inFlight;
  }

  return {
    pollOnce,
    start() {
      if (timer) return;
      void pollOnce();
      timer = setInterval(() => {
        void pollOnce();
      }, options.pollIntervalMs);
    },
    stop() {
      if (timer) {
        clearInterval(timer);
        timer = null;
      }
    },
  };
}
