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
};

async function getJson(path) {
  const response = await fetch(path, { cache: "no-store" });
  if (!response.ok) throw new Error(`${path} ${response.status}`);
  return response.json();
}

async function refresh() {
  try {
    const [summary, inventory, chains, nodes, collectors] = await Promise.all([
      getJson("/api/status/summary"),
      getJson("/api/inventory"),
      getJson("/api/chains"),
      getJson("/api/nodes"),
      getJson("/api/collectors"),
    ]);

    renderSummary(summary, inventory);
    renderChains(chains);
    renderNodes(nodes, inventory);
    renderCollectors(collectors);
    els.subtitle.textContent = `${inventory.name} via ${inventory.gateway?.ip ?? "monitor gateway"}`;
    els.updatedAt.textContent = new Date().toLocaleString();
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
  els.chainCount.textContent = `${chains.length} endpoints`;
  els.chainsBody.innerHTML = "";
  if (chains.length === 0) {
    els.chainsBody.innerHTML = `<tr><td colspan="8" class="empty">No chain data</td></tr>`;
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
      <td>${item.isSyncing ? "yes" : "no"}</td>
      <td>${escapeHtml(item.runtimeVersion ?? "-")}</td>
    `;
    els.chainsBody.appendChild(tr);
  }
}

function renderNodes(nodes, inventory) {
  els.nodeCount.textContent = `${nodes.length || inventory.nodes?.length || 0} nodes`;
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
  els.collectorCount.textContent = `${collectors.length} collectors`;
  els.collectorsGrid.innerHTML = "";
  if (collectors.length === 0) {
    els.collectorsGrid.innerHTML = `<div class="empty">No collector runs yet</div>`;
    return;
  }

  for (const collector of collectors) {
    const div = document.createElement("div");
    div.className = "collector-item";
    div.innerHTML = `
      <div class="node-head">
        <span class="node-id">${escapeHtml(collector.name)}</span>
        <span class="status ${collector.lastOk ? "ok" : "bad"}">${collector.lastOk ? "ok" : "error"}</span>
      </div>
      <div class="chain-tags">
        <span class="tag">${formatNumber(collector.lastDurationMs)} ms</span>
        <span class="tag">${escapeHtml(collector.lastFinishedAt ?? "-")}</span>
      </div>
    `;
    els.collectorsGrid.appendChild(div);
  }
}

function statusBadge(item) {
  const state = item.stale ? "warn" : item.healthy ? "ok" : "bad";
  const text = item.stale ? "stale" : item.healthy ? "healthy" : "down";
  return `<span class="status ${state}">${text}</span>`;
}

function nodeBadge(node) {
  if (node.stale) return `<span class="status warn">stale</span>`;
  const healthy = Object.values(node.chains ?? {}).some((chain) => chain.healthy && !chain.stale);
  return `<span class="status ${healthy ? "ok" : "bad"}">${healthy ? "healthy" : "down"}</span>`;
}

function formatNumber(value) {
  if (value === null || value === undefined) return "-";
  return Number(value).toLocaleString();
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

void refresh();
setInterval(() => {
  void refresh();
}, 5000);
