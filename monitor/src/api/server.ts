import Fastify from "fastify";

import type { InventorySnapshot } from "../inventory/types.js";
import { renderPrometheusMetrics, type MetricsRegistry } from "../metrics/prometheus.js";
import type { MonitorStore } from "../state/store.js";

export const API_VERSION = "1";

export function buildServer(deps: {
  inventory: InventorySnapshot;
  store: MonitorStore;
  metrics: MetricsRegistry;
}) {
  const app = Fastify({ logger: false });

  app.addHook("onSend", async (_request, reply, payload) => {
    reply.header("x-fishbone-monitor-api-version", API_VERSION);
    return payload;
  });

  app.get("/healthz", async () => ({ ok: true, name: "fishbone-monitor" }));
  app.get("/api/inventory", async () => deps.inventory);
  app.get("/api/status/summary", async () => deps.store.getSummary());
  app.get("/api/nodes", async () => deps.store.getNodes());
  app.get("/api/chains", async () => deps.store.getChains());
  app.get("/api/collectors", async () => deps.store.getCollectorHealth());

  app.get<{ Params: { nodeId: string } }>("/api/nodes/:nodeId", async (request, reply) => {
    const node = deps.store.getNodes().find((item) => item.id === request.params.nodeId);
    if (!node) {
      return reply.code(404).send({ error: "node not found" });
    }
    return node;
  });

  app.get<{ Params: { chainKey: string } }>("/api/chains/:chainKey", async (request, reply) => {
    const statuses = deps.store.getChains().filter((item) => item.key === request.params.chainKey);
    if (statuses.length === 0) {
      return reply.code(404).send({ error: "chain not found" });
    }
    return statuses;
  });

  app.get("/metrics", async (_request, reply) => {
    const text = await renderPrometheusMetrics(deps.metrics, deps.inventory, deps.store);
    return reply.type("text/plain; version=0.0.4").send(text);
  });

  app.get("/api/events", async (_request, reply) => {
    reply.raw.writeHead(200, {
      "content-type": "text/event-stream",
      "cache-control": "no-cache",
      connection: "keep-alive",
    });

    const writeStatus = () => {
      reply.raw.write(`event: status\n`);
      reply.raw.write(`data: ${JSON.stringify({ updatedAt: new Date().toISOString(), summary: deps.store.getSummary() })}\n\n`);
    };
    writeStatus();
    const interval = setInterval(writeStatus, 3000);
    reply.raw.on("close", () => clearInterval(interval));
  });

  return app;
}
