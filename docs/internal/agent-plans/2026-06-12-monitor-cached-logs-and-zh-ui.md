# Cached Logs And Chinese Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add cached VM log visibility to Fishbone Monitor and localize the current dashboard into Chinese without allowing frontend requests to trigger VM commands.

**Architecture:** Add a background `LogCollector` to the monitor scheduler. The collector periodically uses `ssh <node>` from `bcg` to read bounded recent log lines from each node's local log files and stores them in the in-memory monitor store; API routes and the frontend read only that cache. Keep future interactive remote actions as a separate extension path, not mixed into logs APIs.

**Tech Stack:** Node.js 18+, TypeScript, built-in `node:child_process`, built-in `node:test`, static HTML/CSS/JS.

---

## Design Constraints

- The frontend must not trigger SSH or remote commands on VMs.
- Log APIs only return `bcg` monitor cache.
- The background collector controls SSH cadence and read size.
- Default collection interval can reuse the scheduler poll interval.
- Each `node+chain` keeps a bounded ring buffer of recent lines.
- Future command/action support must be implemented under a separate `actions/` boundary and separate API namespace; logs APIs must remain read-only.

## File Structure

- Create `monitor/src/logs/types.ts`: log cache and API response types.
- Create `monitor/src/collectors/logCollector.ts`: background SSH log collection with injectable command runner.
- Modify `monitor/src/state/store.ts`: store log snapshots and expose getters.
- Modify `monitor/src/scheduler.ts`: invoke the log collector during `pollOnce()`.
- Modify `monitor/src/api/server.ts`: add cached log read APIs.
- Modify `monitor/public/index.html`: Chinese dashboard labels and log section.
- Modify `monitor/public/assets/app.js`: Chinese labels, fetch cached logs, render log panel.
- Modify `monitor/public/assets/styles.css`: log panel styles.
- Create `monitor/test/logCollector.test.ts`: collector behavior tests.
- Create `monitor/test/logApi.test.ts`: cached log API tests.
- Modify `monitor/test/dashboard.test.ts`: Chinese dashboard and static asset expectations.
- Modify `docs/operations/fishbone-monitor.md`: document cached logs and non-interactive boundary.

## API Contract

New read-only APIs:

```text
GET /api/logs
GET /api/logs/:nodeId/:chainKey
```

`GET /api/logs` returns summaries:

```json
[
  {
    "nodeId": "f1",
    "chainKey": "main",
    "path": "/home/debian/fishbone/logs/main.log",
    "updatedAt": "2026-06-12T00:00:00.000Z",
    "ok": true,
    "lineCount": 120,
    "errors": []
  }
]
```

`GET /api/logs/:nodeId/:chainKey` returns one cached snapshot:

```json
{
  "nodeId": "f1",
  "chainKey": "main",
  "path": "/home/debian/fishbone/logs/main.log",
  "updatedAt": "2026-06-12T00:00:00.000Z",
  "ok": true,
  "lines": ["..."],
  "errors": []
}
```

If no cache exists yet, return `404` with `{ "error": "log cache not found" }`.

## Task 1: Log Cache Store

**Files:**
- Create: `monitor/src/logs/types.ts`
- Modify: `monitor/src/state/store.ts`
- Test: `monitor/test/store.test.ts`

- [x] **Step 1: Add failing tests for log snapshots**

Add tests to `monitor/test/store.test.ts`:

```ts
test("stores and summarizes cached log snapshots", () => {
  const store = new MonitorStore({ staleAfterMs: 15_000 });
  store.upsertLogSnapshot({
    nodeId: "f1",
    chainKey: "main",
    path: "/home/debian/fishbone/logs/main.log",
    updatedAt: "2026-06-12T00:00:00.000Z",
    ok: true,
    lines: ["line 1", "line 2"],
    errors: [],
  });

  assert.deepEqual(store.getLogSummaries(), [{
    nodeId: "f1",
    chainKey: "main",
    path: "/home/debian/fishbone/logs/main.log",
    updatedAt: "2026-06-12T00:00:00.000Z",
    ok: true,
    lineCount: 2,
    errors: [],
  }]);
  assert.equal(store.getLogSnapshot("f1", "main")?.lines.length, 2);
});
```

- [x] **Step 2: Run test and verify it fails**

```bash
cd monitor
npm test -- store.test.ts
```

Expected: TypeScript errors because log methods do not exist.

- [x] **Step 3: Implement types and store methods**

Create:

```ts
export type LogSnapshot = {
  nodeId: string;
  chainKey: string;
  path: string;
  updatedAt: string;
  ok: boolean;
  lines: string[];
  errors: string[];
};

export type LogSummary = Omit<LogSnapshot, "lines"> & {
  lineCount: number;
};
```

Add to store:

```ts
upsertLogSnapshot(snapshot: LogSnapshot): void
getLogSnapshot(nodeId: string, chainKey: string): LogSnapshot | undefined
getLogSummaries(): LogSummary[]
```

- [x] **Step 4: Run test and commit**

```bash
cd monitor
npm test -- store.test.ts
git add monitor/src/logs/types.ts monitor/src/state/store.ts monitor/test/store.test.ts
git commit -m "feat(monitor): cache collected logs"
```

## Task 2: Background Log Collector

**Files:**
- Create: `monitor/src/collectors/logCollector.ts`
- Create: `monitor/test/logCollector.test.ts`

- [x] **Step 1: Add failing collector tests**

Test with an injected runner:

```ts
test("collects bounded log snapshots for node chain roles", async () => {
  const result = await collectLogs({
    inventory,
    maxLines: 3,
    runCommand: async (host, command) => {
      assert.equal(host, "f1");
      assert.equal(command, "tail -n 3 /home/debian/fishbone/logs/main.log");
      return { code: 0, stdout: "a\nb\n", stderr: "" };
    },
    now: () => "2026-06-12T00:00:00.000Z",
  });
  assert.equal(result.snapshots[0].ok, true);
  assert.deepEqual(result.snapshots[0].lines, ["a", "b"]);
});
```

Add an error test where runner returns `{ code: 1, stdout: "", stderr: "missing" }`.

- [x] **Step 2: Run test and verify it fails**

```bash
cd monitor
npm test -- logCollector.test.ts
```

Expected: module missing.

- [x] **Step 3: Implement collector**

Implementation:

- Iterate each node role from inventory.
- Log path is `${inventory.logDir}/${chainKey}.log`.
- Command is `tail -n ${maxLines} ${path}`.
- Default runner uses `ssh <node.ssh> <command>` with `execFile`.
- No frontend code calls this collector.
- Return `{ snapshots, errors }`.

- [x] **Step 4: Run test and commit**

```bash
cd monitor
npm test -- logCollector.test.ts
git add monitor/src/collectors/logCollector.ts monitor/test/logCollector.test.ts
git commit -m "feat(monitor): collect vm logs in background"
```

## Task 3: Scheduler Integration

**Files:**
- Modify: `monitor/src/scheduler.ts`
- Test: `monitor/test/schedulerLogs.test.ts`

- [x] **Step 1: Add failing scheduler test**

Create a scheduler with `collectLogs` dependency injected. Call `pollOnce()` and assert `store.getLogSnapshot("f1", "main")` returns cached lines.

- [x] **Step 2: Implement scheduler log collection**

Add options:

```ts
collectLogs?: typeof collectLogs
logMaxLines?: number
```

In `pollOnce()`, call log collection after chain status collection and upsert snapshots into store.

- [x] **Step 3: Run test and commit**

```bash
cd monitor
npm test -- schedulerLogs.test.ts
git add monitor/src/scheduler.ts monitor/test/schedulerLogs.test.ts
git commit -m "feat(monitor): schedule cached log collection"
```

## Task 4: Cached Log APIs

**Files:**
- Modify: `monitor/src/api/server.ts`
- Create: `monitor/test/logApi.test.ts`

- [x] **Step 1: Add failing API tests**

Test:

- `GET /api/logs` returns summaries.
- `GET /api/logs/f1/main` returns snapshot.
- unknown log returns 404.

- [x] **Step 2: Implement routes**

Add routes to `dispatch()` before final 404:

```ts
if (pathname === "/api/logs") return json(200, deps.store.getLogSummaries());
if (pathname.startsWith("/api/logs/")) { ... }
```

Decode node and chain path segments.

- [x] **Step 3: Run test and commit**

```bash
cd monitor
npm test -- logApi.test.ts
git add monitor/src/api/server.ts monitor/test/logApi.test.ts
git commit -m "feat(monitor): expose cached log api"
```

## Task 5: Chinese Dashboard And Logs Panel

**Files:**
- Modify: `monitor/public/index.html`
- Modify: `monitor/public/assets/app.js`
- Modify: `monitor/public/assets/styles.css`
- Modify: `monitor/test/dashboard.test.ts`

- [x] **Step 1: Add failing dashboard test expectations**

Expect `/` contains `FishboneChain 监控`, `子链状态`, `节点状态`, and `运行日志`.

- [x] **Step 2: Localize static text**

Translate:

- `Refresh` -> `刷新`
- `Chains` -> `子链状态`
- `Nodes` -> `节点状态`
- `Collectors` -> `采集器`
- `Healthy Chains` -> `健康子链`
- `Healthy Nodes` -> `健康节点`
- `Best` -> `最高区块`
- `Finalized` -> `最终确认区块`
- `Peers` -> `连接数`
- `Syncing` -> `同步中`
- `Runtime` -> `运行时版本`

- [x] **Step 3: Add logs panel**

Add:

- node select
- chain select
- refresh logs button
- cached updated time
- `<pre id="logContent">`

The JS fetches `/api/logs`, fills selects, and fetches `/api/logs/:node/:chain`. It must not call any endpoint that executes remote commands.

- [x] **Step 4: Run tests and commit**

```bash
cd monitor
npm test -- dashboard.test.ts
git add monitor/public monitor/test/dashboard.test.ts
git commit -m "feat(monitor): localize dashboard and show cached logs"
```

## Task 6: Docs, Verification, Deployment

**Files:**
- Modify: `docs/operations/fishbone-monitor.md`

- [x] **Step 1: Document cached logs**

Add:

- Logs are background-collected from VM local log files.
- Frontend reads only monitor cache.
- No interactive VM command path is exposed yet.
- Future action APIs must use a separate route namespace with auth/audit/limits.

- [x] **Step 2: Run full checks**

```bash
cd monitor
npm test
npm audit --omit=dev
```

Expected: tests pass, 0 vulnerabilities.

- [x] **Step 3: Deploy to `bcg`**

```bash
tar --exclude=node_modules --exclude=dist -C monitor -cf - . | ssh bcg 'tar -C /home/debian/fishbone/monitor -xf -'
ssh bcg 'cd /home/debian/fishbone/monitor && npm ci && npm run build && echo debian | sudo -S systemctl restart fishbone-monitor && systemctl is-active fishbone-monitor'
```

- [x] **Step 4: Verify remote**

```bash
curl -sf http://192.168.8.41:18080/ | grep 'FishboneChain 监控'
curl -sf http://192.168.8.41:18080/api/logs | head
curl -sf http://192.168.8.41:18080/api/status/summary
```

- [x] **Step 5: Commit docs**

```bash
git add docs/operations/fishbone-monitor.md
git commit -m "docs(monitor): document cached log collection"
```

## Self-Review

- Spec coverage: The plan covers background log collection, cache-only APIs, Chinese UI, deployment, and the future action-channel boundary.
- Placeholder scan: No implementation placeholders remain.
- Type consistency: `LogSnapshot`, `LogSummary`, collector result, store API, and REST route names are consistent.
- Scope check: The plan intentionally avoids real-time tail, interactive remote commands, auth, and persistent log storage.
