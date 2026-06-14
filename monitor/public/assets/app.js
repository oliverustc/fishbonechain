const els = {
  subtitle: document.querySelector("#subtitle"),
  updatedAt: document.querySelector("#updatedAt"),
  refreshButton: document.querySelector("#refreshButton"),
  totalChains: document.querySelector("#totalChains"),
  healthyChains: document.querySelector("#healthyChains"),
  totalNodes: document.querySelector("#totalNodes"),
  healthyNodes: document.querySelector("#healthyNodes"),
  errorCount: document.querySelector("#errorCount"),
  chainCount: document.querySelector("#chainCount"),
  nodeCount: document.querySelector("#nodeCount"),
  collectorCount: document.querySelector("#collectorCount"),
  chainsBody: document.querySelector("#chainsBody"),
  nodesGrid: document.querySelector("#nodesGrid"),
  collectorsGrid: document.querySelector("#collectorsGrid"),
  logStatus: document.querySelector("#logStatus"),
  logNodeSelect: document.querySelector("#logNodeSelect"),
  logChainSelect: document.querySelector("#logChainSelect"),
  refreshLogButton: document.querySelector("#refreshLogButton"),
  logMeta: document.querySelector("#logMeta"),
  logContent: document.querySelector("#logContent"),
};

let logSummaries = [];
let selectedLogNode = "";
let selectedLogChain = "";

async function getJson(path) {
  const response = await fetch(path, { cache: "no-store" });
  if (!response.ok) throw new Error(`${path} ${response.status}`);
  return response.json();
}

async function refresh() {
  try {
    const [summary, inventory, chains, nodes, collectors, logs] = await Promise.all([
      getJson("/api/status/summary"),
      getJson("/api/inventory"),
      getJson("/api/chains"),
      getJson("/api/nodes"),
      getJson("/api/collectors"),
      getJson("/api/logs"),
    ]);

    renderSummary(summary, inventory);
    renderChains(chains);
    renderNodes(nodes, inventory);
    renderCollectors(collectors);
    renderLogControls(logs);
    els.subtitle.textContent = `${inventory.name} 经由 ${inventory.gateway?.ip ?? "monitor gateway"}`;
    els.updatedAt.textContent = new Date().toLocaleString("zh-CN");
    await loadSelectedLog();
  } catch (error) {
    els.subtitle.textContent = error instanceof Error ? error.message : String(error);
  }
}

function renderSummary(summary, inventory) {
  els.totalChains.textContent = String(summary.totalChains ?? inventory.chains?.length ?? 0);
  els.healthyChains.textContent = String(summary.healthyChains ?? 0);
  els.totalNodes.textContent = String(summary.totalNodes ?? inventory.nodes?.length ?? 0);
  els.healthyNodes.textContent = String(summary.healthyNodes ?? 0);
  els.errorCount.textContent = String(summary.errorCount ?? 0);
}

function renderChains(chains) {
  els.chainCount.textContent = `${chains.length} 个端点`;
  els.chainsBody.innerHTML = "";
  if (chains.length === 0) {
    els.chainsBody.innerHTML = `<tr><td colspan="8" class="empty">暂无子链状态</td></tr>`;
    return;
  }

  for (const item of chains) {
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td>${escapeHtml(item.key)}</td>
      <td>${escapeHtml(item.nodeId)}</td>
      <td>${statusBadge(item)}</td>
      <td>${formatNumber(item.bestBlock)}</td>
      <td>${formatNumber(item.finalizedBlock)}</td>
      <td>${formatNumber(item.peers)}</td>
      <td>${item.isSyncing ? "是" : "否"}</td>
      <td>${escapeHtml(item.runtimeVersion ?? "-")}</td>
    `;
    els.chainsBody.appendChild(tr);
  }
}

function renderNodes(nodes, inventory) {
  els.nodeCount.textContent = `${nodes.length || inventory.nodes?.length || 0} 个节点`;
  els.nodesGrid.innerHTML = "";
  const source = nodes.length > 0 ? nodes : (inventory.nodes ?? []).map((node) => ({
    id: node.id,
    stale: true,
    chains: Object.fromEntries((node.roles ?? []).map((role) => [role, { key: role, healthy: false, stale: true }])),
  }));

  for (const node of source) {
    const div = document.createElement("div");
    div.className = "node-item";
    const chainValues = Object.values(node.chains ?? {});
    div.innerHTML = `
      <div class="node-head">
        <span class="node-id">${escapeHtml(node.id)}</span>
        ${nodeBadge(node)}
      </div>
      <div class="chain-tags">
        ${chainValues.map((chain) => `<span class="tag">${escapeHtml(chain.key)}</span>`).join("")}
      </div>
    `;
    els.nodesGrid.appendChild(div);
  }
}

function renderCollectors(collectors) {
  els.collectorCount.textContent = `${collectors.length} 个采集器`;
  els.collectorsGrid.innerHTML = "";
  if (collectors.length === 0) {
    els.collectorsGrid.innerHTML = `<div class="empty">暂无采集记录</div>`;
    return;
  }

  for (const collector of collectors) {
    const div = document.createElement("div");
    div.className = "collector-item";
    div.innerHTML = `
      <div class="node-head">
        <span class="node-id">${escapeHtml(collector.name)}</span>
        <span class="status ${collector.lastOk ? "ok" : "bad"}">${collector.lastOk ? "正常" : "异常"}</span>
      </div>
      <div class="chain-tags">
        <span class="tag">${formatNumber(collector.lastDurationMs)} ms</span>
        <span class="tag">${escapeHtml(formatTime(collector.lastFinishedAt))}</span>
      </div>
    `;
    els.collectorsGrid.appendChild(div);
  }
}

function renderLogControls(summaries) {
  logSummaries = [...summaries].sort((a, b) => `${a.nodeId}/${a.chainKey}`.localeCompare(`${b.nodeId}/${b.chainKey}`));
  const nodeIds = [...new Set(logSummaries.map((item) => item.nodeId))];

  if (!nodeIds.includes(selectedLogNode)) {
    selectedLogNode = nodeIds[0] ?? "";
  }

  const chainKeys = [...new Set(logSummaries.filter((item) => item.nodeId === selectedLogNode).map((item) => item.chainKey))];
  if (!chainKeys.includes(selectedLogChain)) {
    selectedLogChain = chainKeys[0] ?? "";
  }

  renderOptions(els.logNodeSelect, nodeIds, selectedLogNode, "暂无节点");
  renderOptions(els.logChainSelect, chainKeys, selectedLogChain, "暂无子链");

  if (logSummaries.length === 0) {
    els.logStatus.textContent = "等待缓存";
    els.logMeta.textContent = "暂无日志";
    els.logContent.textContent = "暂无缓存日志";
    return;
  }

  const current = findSelectedLogSummary();
  els.logStatus.textContent = `${logSummaries.length} 份缓存`;
  els.logMeta.textContent = current
    ? `${current.ok ? "最近采集正常" : "最近采集异常"} · ${current.lineCount} 行 · ${formatTime(current.updatedAt)}`
    : "请选择日志";
}

async function loadSelectedLog() {
  if (!selectedLogNode || !selectedLogChain) return;

  els.logContent.textContent = "正在读取缓存日志...";
  try {
    const snapshot = await getJson(`/api/logs/${encodeURIComponent(selectedLogNode)}/${encodeURIComponent(selectedLogChain)}`);
    els.logMeta.textContent = `${snapshot.path} · ${snapshot.lines.length} 行 · ${formatTime(snapshot.updatedAt)}`;
    els.logContent.textContent = snapshot.lines.length > 0
      ? snapshot.lines.join("\n")
      : (snapshot.ok ? "缓存日志为空" : snapshot.errors.join("\n") || "日志采集异常");
  } catch (error) {
    els.logContent.textContent = error instanceof Error ? error.message : String(error);
  }
}

function renderOptions(select, values, selected, emptyText) {
  select.innerHTML = "";
  if (values.length === 0) {
    const option = document.createElement("option");
    option.value = "";
    option.textContent = emptyText;
    select.appendChild(option);
    select.disabled = true;
    return;
  }

  select.disabled = false;
  for (const value of values) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = value;
    option.selected = value === selected;
    select.appendChild(option);
  }
}

function findSelectedLogSummary() {
  return logSummaries.find((item) => item.nodeId === selectedLogNode && item.chainKey === selectedLogChain);
}

function statusBadge(item) {
  const state = item.stale ? "warn" : item.healthy ? "ok" : "bad";
  const text = item.stale ? "过期" : item.healthy ? "健康" : "离线";
  return `<span class="status ${state}">${text}</span>`;
}

function nodeBadge(node) {
  if (node.stale) return `<span class="status warn">过期</span>`;
  const healthy = Object.values(node.chains ?? {}).some((chain) => chain.healthy && !chain.stale);
  return `<span class="status ${healthy ? "ok" : "bad"}">${healthy ? "健康" : "离线"}</span>`;
}

function formatNumber(value) {
  if (value === null || value === undefined) return "-";
  return Number(value).toLocaleString("zh-CN");
}

function formatTime(value) {
  if (!value) return "-";
  return new Date(value).toLocaleString("zh-CN");
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

els.refreshButton.addEventListener("click", () => {
  void refresh();
});

els.logNodeSelect.addEventListener("change", () => {
  selectedLogNode = els.logNodeSelect.value;
  selectedLogChain = "";
  renderLogControls(logSummaries);
  void loadSelectedLog();
});

els.logChainSelect.addEventListener("change", () => {
  selectedLogChain = els.logChainSelect.value;
  renderLogControls(logSummaries);
  void loadSelectedLog();
});

els.refreshLogButton.addEventListener("click", () => {
  void loadSelectedLog();
});

void refresh();
setInterval(() => {
  void refresh();
}, 5000);
