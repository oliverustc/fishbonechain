import type { CollectorResult } from "./types.js";
import type { ChainStatus } from "../status/types.js";

export type RpcClient = {
  systemHealth(): Promise<{ peers: number; isSyncing: boolean; shouldHavePeers: boolean }>;
  chainHeader(): Promise<{ number: number }>;
  finalizedHeader(): Promise<{ number: number }>;
  runtimeVersion(): Promise<string>;
};

export type CollectSubstrateRpcStatusInput = {
  chainKey: string;
  nodeId: string;
  client: RpcClient;
  now?: () => string;
};

type JsonRpcResponse<T> = {
  result?: T;
  error?: {
    code: number;
    message: string;
  };
};

export async function collectSubstrateRpcStatus(
  input: CollectSubstrateRpcStatusInput,
): Promise<CollectorResult<ChainStatus>> {
  const now = input.now ?? (() => new Date().toISOString());
  const startedAt = now();

  try {
    const [health, header, finalizedHeader, runtimeVersion] = await Promise.all([
      input.client.systemHealth(),
      input.client.chainHeader(),
      input.client.finalizedHeader(),
      input.client.runtimeVersion(),
    ]);
    const finishedAt = now();

    return {
      ok: true,
      data: {
        key: input.chainKey,
        nodeId: input.nodeId,
        healthy: !health.isSyncing && (!health.shouldHavePeers || health.peers > 0),
        bestBlock: header.number,
        finalizedBlock: finalizedHeader.number,
        peers: health.peers,
        isSyncing: health.isSyncing,
        runtimeVersion,
        updatedAt: finishedAt,
        stale: false,
        errors: [],
      },
      errors: [],
      startedAt,
      finishedAt,
    };
  } catch (error) {
    const finishedAt = now();
    return {
      ok: false,
      data: null,
      errors: [errorMessage(error)],
      startedAt,
      finishedAt,
    };
  }
}

export function createHttpJsonRpcClient(endpoint: string, timeoutMs = 3000): RpcClient {
  async function call<T>(method: string): Promise<T> {
    const response = await fetch(endpoint, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ id: 1, jsonrpc: "2.0", method, params: [] }),
      signal: AbortSignal.timeout(timeoutMs),
    });
    if (!response.ok) {
      throw new Error(`${method} HTTP ${response.status}`);
    }

    const payload = (await response.json()) as JsonRpcResponse<T>;
    if (payload.error) {
      throw new Error(`${method} RPC ${payload.error.code}: ${payload.error.message}`);
    }
    if (payload.result === undefined) {
      throw new Error(`${method} missing result`);
    }
    return payload.result;
  }

  return {
    async systemHealth() {
      return call<{ peers: number; isSyncing: boolean; shouldHavePeers: boolean }>("system_health");
    },
    async chainHeader() {
      const header = await call<{ number: string }>("chain_getHeader");
      return { number: parseHexNumber(header.number) };
    },
    async finalizedHeader() {
      const finalizedHash = await call<string>("chain_getFinalizedHead");
      const header = await callWithParams<{ number: string }>(
        endpoint,
        "chain_getHeader",
        [finalizedHash],
        timeoutMs,
      );
      return { number: parseHexNumber(header.number) };
    },
    async runtimeVersion() {
      const version = await call<{ specName: string; specVersion: number }>("state_getRuntimeVersion");
      return `${version.specName}-${version.specVersion}`;
    },
  };
}

async function callWithParams<T>(
  endpoint: string,
  method: string,
  params: unknown[],
  timeoutMs: number,
): Promise<T> {
  const response = await fetch(endpoint, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ id: 1, jsonrpc: "2.0", method, params }),
    signal: AbortSignal.timeout(timeoutMs),
  });
  if (!response.ok) {
    throw new Error(`${method} HTTP ${response.status}`);
  }

  const payload = (await response.json()) as JsonRpcResponse<T>;
  if (payload.error) {
    throw new Error(`${method} RPC ${payload.error.code}: ${payload.error.message}`);
  }
  if (payload.result === undefined) {
    throw new Error(`${method} missing result`);
  }
  return payload.result;
}

function parseHexNumber(value: string): number {
  const parsed = Number.parseInt(value, 16);
  if (!Number.isFinite(parsed)) {
    throw new Error(`invalid hex number: ${value}`);
  }
  return parsed;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
