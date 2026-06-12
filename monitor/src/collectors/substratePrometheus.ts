import type { CollectorResult } from "./types.js";

export type SubstratePrometheusMetrics = {
  bestBlock: number | null;
  finalizedBlock: number | null;
};

export type CollectSubstratePrometheusInput = {
  endpoint: string;
  timeoutMs?: number;
  fetchText?: (endpoint: string, timeoutMs: number) => Promise<string>;
  now?: () => string;
};

export function parseSubstratePrometheusText(text: string): SubstratePrometheusMetrics {
  const metrics: SubstratePrometheusMetrics = {
    bestBlock: null,
    finalizedBlock: null,
  };

  for (const line of text.split(/\r?\n/)) {
    if (!line.startsWith("substrate_block_height{")) continue;

    const status = /status="([^"]+)"/.exec(line)?.[1];
    const value = Number.parseFloat(line.trim().split(/\s+/).at(-1) ?? "");
    if (!Number.isFinite(value)) continue;

    if (status === "best") {
      metrics.bestBlock = value;
    } else if (status === "finalized") {
      metrics.finalizedBlock = value;
    }
  }

  return metrics;
}

export async function collectSubstratePrometheus(
  input: CollectSubstratePrometheusInput,
): Promise<CollectorResult<SubstratePrometheusMetrics>> {
  const now = input.now ?? (() => new Date().toISOString());
  const timeoutMs = input.timeoutMs ?? 3000;
  const fetchText = input.fetchText ?? defaultFetchText;
  const startedAt = now();

  try {
    const text = await fetchText(input.endpoint, timeoutMs);
    const data = parseSubstratePrometheusText(text);
    const finishedAt = now();
    return {
      ok: true,
      data,
      errors: [],
      startedAt,
      finishedAt,
    };
  } catch (error) {
    const finishedAt = now();
    return {
      ok: false,
      data: null,
      errors: [error instanceof Error ? error.message : String(error)],
      startedAt,
      finishedAt,
    };
  }
}

async function defaultFetchText(endpoint: string, timeoutMs: number): Promise<string> {
  const response = await fetch(endpoint, {
    signal: AbortSignal.timeout(timeoutMs),
  });
  if (!response.ok) {
    throw new Error(`prometheus HTTP ${response.status}`);
  }
  return response.text();
}
