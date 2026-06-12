import {
  collectSubstrateRpcStatus,
  createHttpJsonRpcClient,
  type RpcClient,
} from "./collectors/substrateRpc.js";
import { collectSubstratePrometheus } from "./collectors/substratePrometheus.js";
import type { InventorySnapshot } from "./inventory/types.js";
import type { MonitorStore } from "./state/store.js";

export type SchedulerOptions = {
  inventory: InventorySnapshot;
  store: MonitorStore;
  pollIntervalMs: number;
  createRpcClient?: (endpoint: string) => RpcClient;
};

export type MonitorScheduler = {
  pollOnce(): Promise<void>;
  start(): void;
  stop(): void;
};

export function createScheduler(options: SchedulerOptions): MonitorScheduler {
  let timer: NodeJS.Timeout | null = null;
  const createRpcClient = options.createRpcClient ?? createHttpJsonRpcClient;

  async function pollOnce(): Promise<void> {
    const startedAt = Date.now();
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

    options.store.recordCollectorHealth({
      name: "scheduler",
      lastStartedAt: new Date(startedAt).toISOString(),
      lastFinishedAt: new Date().toISOString(),
      lastDurationMs: Date.now() - startedAt,
      lastOk: errors.length === 0,
      errors,
    });
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
