export type NodeInventory = {
  id: string;
  ip: string;
  ssh: string;
  roles: string[];
};

export type ChainInventory = {
  key: string;
  chainId: string;
  spec: string;
  p2pPort: number;
  rpcPort: number;
  prometheusPort: number;
  binary?: string;
  validators: string[];
  rpcEndpoints: string[];
  wsEndpoints: string[];
  prometheusEndpoints: string[];
};

export type GatewayInventory = {
  ssh: string;
  ip: string;
};

export type InventorySnapshot = {
  name: string;
  binary: string;
  baseDir: string;
  logDir: string;
  gateway?: GatewayInventory;
  nodes: NodeInventory[];
  chains: ChainInventory[];
  loadedAt: string;
};
