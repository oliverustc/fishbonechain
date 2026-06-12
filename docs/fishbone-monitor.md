# Fishbone Monitor

Fishbone Monitor is the observability gateway for the lab deployment. It runs on `bcg`
(`192.168.8.41`) and directly polls the `10.2.2.0/24` node network from there.
Browsers and future dashboards should talk to the monitor API instead of trying to
reach `f1`-`f12` directly.

## Runtime

- Host: `bcg`
- Default listen address: `0.0.0.0:18080`
- Inventory source: `/home/debian/fishbone/deploy/config.toml`
- Process manager: systemd unit `fishbone-monitor`
- Metrics endpoint: `/metrics` in Prometheus text format

## Deploy

From the project root:

```bash
rsync -a --delete monitor/ bcg:/home/debian/fishbone/monitor/
rsync -a deploy/systemd/fishbone-monitor.service bcg:/home/debian/fishbone/deploy/systemd/
ssh bcg 'cd /home/debian/fishbone/monitor && npm ci && npm run build'
ssh bcg 'sudo cp /home/debian/fishbone/deploy/systemd/fishbone-monitor.service /etc/systemd/system/'
ssh bcg 'sudo systemctl daemon-reload && sudo systemctl enable --now fishbone-monitor'
```

## Verify

```bash
curl -sf http://192.168.8.41:18080/healthz
curl -sf http://192.168.8.41:18080/api/inventory
curl -sf http://192.168.8.41:18080/api/status/summary
curl -sf http://192.168.8.41:18080/metrics | head
ssh bcg 'systemctl status fishbone-monitor --no-pager'
```

Expected inventory shape:

- 12 nodes: `f1` through `f12`
- 7 chains: `main`, `child1`, `child2`, `child3`, `child4`, `child5`, `child6`
- endpoints generated from `deploy/config.toml`

## API

- `GET /healthz`: process health; does not poll remote nodes.
- `GET /api/inventory`: normalized nodes, chains, endpoints and gateway.
- `GET /api/status/summary`: aggregate health counters.
- `GET /api/nodes`: latest status grouped by node.
- `GET /api/nodes/:nodeId`: latest status for one node.
- `GET /api/chains`: latest status grouped by chain endpoint.
- `GET /api/chains/:chainKey`: latest status for one chain.
- `GET /api/collectors`: last collector run metadata.
- `GET /api/events`: SSE stream for dashboard updates.
- `GET /metrics`: Prometheus-compatible metrics.

All API responses include:

```text
X-Fishbone-Monitor-Api-Version: 1
```

## Configuration

Environment variables:

```text
FISHBONE_MONITOR_HOST=0.0.0.0
FISHBONE_MONITOR_PORT=18080
FISHBONE_CONFIG_PATH=/home/debian/fishbone/deploy/config.toml
FISHBONE_POLL_INTERVAL_MS=5000
FISHBONE_STALE_AFTER_MS=15000
```

For a stricter access boundary, bind to localhost and use SSH forwarding:

```bash
ssh -L 18080:127.0.0.1:18080 bcg
```

## Prometheus Compatibility

The monitor works standalone, but it is safe for Prometheus to scrape later:

```yaml
scrape_configs:
  - job_name: fishbone-monitor
    static_configs:
      - targets: ["127.0.0.1:18080"]
```

The first stable metric names are:

```text
fishbone_chain_up{chain,node,source}
fishbone_chain_best_block{chain,node,source}
fishbone_chain_finalized_block{chain,node,source}
fishbone_chain_peers{chain,node,source}
fishbone_chain_syncing{chain,node,source}
fishbone_collector_duration_seconds{collector}
fishbone_collector_errors_total{collector,chain,node}
fishbone_inventory_chains_total
fishbone_inventory_nodes_total
```

Keep labels low-cardinality. Do not use block hashes, peer IDs, account IDs, or error
messages as Prometheus labels.

## Operations

```bash
ssh bcg 'sudo systemctl restart fishbone-monitor'
ssh bcg 'journalctl -u fishbone-monitor -f'
ssh bcg 'sudo systemctl disable --now fishbone-monitor'
```

The monitor never reads `deploy/keys/*.env` and must not expose validator seeds or
private material through APIs.
