import { createServer, type IncomingMessage, type Server, type ServerResponse } from "node:http";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

import type { InventorySnapshot } from "../inventory/types.js";
import { renderPrometheusMetrics, type MetricsRegistry } from "../metrics/prometheus.js";
import type { MonitorStore } from "../state/store.js";

export const API_VERSION = "1";

export type InjectResponse = {
  statusCode: number;
  headers: Record<string, string>;
  body: string;
};

export type MonitorServer = {
  inject(request: { method: string; url: string }): Promise<InjectResponse>;
  listen(options: { host: string; port: number }): Promise<void>;
  close(): Promise<void>;
};

export function buildServer(deps: {
  inventory: InventorySnapshot;
  store: MonitorStore;
  metrics: MetricsRegistry;
}): MonitorServer {
  let server: Server | null = null;
  const publicDir = resolve(process.cwd(), "public");

  async function dispatch(method: string, rawUrl: string): Promise<InjectResponse> {
    if (method !== "GET") {
      return json(405, { error: "method not allowed" });
    }

    const pathname = new URL(rawUrl, "http://monitor.local").pathname;
    const staticResponse = await tryStaticAsset(publicDir, pathname);
    if (staticResponse) return staticResponse;

    if (pathname === "/healthz") {
      return json(200, { ok: true, name: "fishbone-monitor" });
    }
    if (pathname === "/api/inventory") {
      return json(200, deps.inventory);
    }
    if (pathname === "/api/status/summary") {
      return json(200, deps.store.getSummary());
    }
    if (pathname === "/api/nodes") {
      return json(200, deps.store.getNodes());
    }
    if (pathname.startsWith("/api/nodes/")) {
      const nodeId = decodeURIComponent(pathname.slice("/api/nodes/".length));
      const node = deps.store.getNodes().find((item) => item.id === nodeId);
      return node ? json(200, node) : json(404, { error: "node not found" });
    }
    if (pathname === "/api/chains") {
      return json(200, deps.store.getChains());
    }
    if (pathname.startsWith("/api/chains/")) {
      const chainKey = decodeURIComponent(pathname.slice("/api/chains/".length));
      const statuses = deps.store.getChains().filter((item) => item.key === chainKey);
      return statuses.length > 0 ? json(200, statuses) : json(404, { error: "chain not found" });
    }
    if (pathname === "/api/collectors") {
      return json(200, deps.store.getCollectorHealth());
    }
    if (pathname === "/metrics") {
      return text(200, await renderPrometheusMetrics(deps.metrics, deps.inventory, deps.store), {
        "content-type": "text/plain; version=0.0.4",
      });
    }

    return json(404, { error: "not found" });
  }

  return {
    inject(request) {
      return dispatch(request.method, request.url);
    },
    listen(options) {
      server = createServer((request, response) => {
        void handleHttpRequest(deps, dispatch, request, response);
      });
      return new Promise((resolve, reject) => {
        server?.once("error", reject);
        server?.listen(options.port, options.host, () => resolve());
      });
    },
    close() {
      return new Promise((resolve, reject) => {
        if (!server) {
          resolve();
          return;
        }
        server.close((error) => (error ? reject(error) : resolve()));
        server = null;
      });
    },
  };
}

async function tryStaticAsset(publicDir: string, pathname: string): Promise<InjectResponse | null> {
  const assets: Record<string, { file: string; contentType: string }> = {
    "/": { file: "index.html", contentType: "text/html; charset=utf-8" },
    "/assets/app.js": { file: "assets/app.js", contentType: "text/javascript; charset=utf-8" },
    "/assets/styles.css": { file: "assets/styles.css", contentType: "text/css; charset=utf-8" },
  };
  const asset = assets[pathname];
  if (!asset) return null;

  try {
    const body = await readFile(resolve(publicDir, asset.file), "utf8");
    return text(200, body, { "content-type": asset.contentType });
  } catch {
    return null;
  }
}

async function handleHttpRequest(
  deps: { store: MonitorStore },
  dispatch: (method: string, rawUrl: string) => Promise<InjectResponse>,
  request: IncomingMessage,
  response: ServerResponse,
): Promise<void> {
  if (request.url === "/api/events") {
    response.writeHead(200, {
      "content-type": "text/event-stream",
      "cache-control": "no-cache",
      connection: "keep-alive",
      "x-fishbone-monitor-api-version": API_VERSION,
    });
    const writeStatus = () => {
      response.write("event: status\n");
      response.write(
        `data: ${JSON.stringify({ updatedAt: new Date().toISOString(), summary: deps.store.getSummary() })}\n\n`,
      );
    };
    writeStatus();
    const interval = setInterval(writeStatus, 3000);
    request.on("close", () => clearInterval(interval));
    return;
  }

  const result = await dispatch(request.method ?? "GET", request.url ?? "/");
  response.writeHead(result.statusCode, result.headers);
  response.end(result.body);
}

function json(statusCode: number, body: unknown): InjectResponse {
  return text(statusCode, JSON.stringify(body), { "content-type": "application/json" });
}

function text(statusCode: number, body: string, extraHeaders: Record<string, string> = {}): InjectResponse {
  return {
    statusCode,
    headers: {
      "x-fishbone-monitor-api-version": API_VERSION,
      ...extraHeaders,
    },
    body,
  };
}
