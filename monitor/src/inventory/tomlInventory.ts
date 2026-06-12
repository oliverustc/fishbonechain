import { readFile } from "node:fs/promises";
import { parse } from "toml";

import type { ChainInventory, InventorySnapshot, NodeInventory } from "./types.js";

type RawChain = {
  id: string;
  spec: string;
  p2p_port: number;
  rpc_port: number;
  prom_port: number;
  binary?: string;
};

type RawNode = {
  id: string;
  ip: string;
  ssh: string;
  roles: string[];
};

type RawConfig = {
  cluster: {
    name: string;
    binary: string;
    base_dir: string;
    log_dir: string;
  };
  gateway?: {
    ssh: string;
    ip: string;
  };
  chains: Record<string, RawChain>;
  nodes: RawNode[];
};

export async function loadTomlInventory(configPath: string): Promise<InventorySnapshot> {
  const content = await readFile(configPath, "utf8");
  const raw = parse(content) as RawConfig;

  const nodes: NodeInventory[] = raw.nodes.map((node) => ({
    id: node.id,
    ip: node.ip,
    ssh: node.ssh,
    roles: [...node.roles],
  }));

  const chains: ChainInventory[] = Object.entries(raw.chains).map(([key, chain]) => {
    const validators = nodes
      .filter((node) => node.roles.includes(key))
      .map((node) => node.id);
    const validatorNodes = validators
      .map((nodeId) => nodes.find((node) => node.id === nodeId))
      .filter((node): node is NodeInventory => node !== undefined);

    return {
      key,
      chainId: chain.id,
      spec: chain.spec,
      p2pPort: chain.p2p_port,
      rpcPort: chain.rpc_port,
      prometheusPort: chain.prom_port,
      binary: chain.binary,
      validators,
      rpcEndpoints: validatorNodes.map((node) => `http://${node.ip}:${chain.rpc_port}`),
      wsEndpoints: validatorNodes.map((node) => `ws://${node.ip}:${chain.rpc_port}`),
      prometheusEndpoints: validatorNodes.map(
        (node) => `http://${node.ip}:${chain.prom_port}/metrics`,
      ),
    };
  });

  return {
    name: raw.cluster.name,
    binary: raw.cluster.binary,
    baseDir: raw.cluster.base_dir,
    logDir: raw.cluster.log_dir,
    gateway: raw.gateway ? { ssh: raw.gateway.ssh, ip: raw.gateway.ip } : undefined,
    nodes,
    chains,
    loadedAt: new Date().toISOString(),
  };
}
