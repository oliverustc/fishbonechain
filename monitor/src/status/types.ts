export type ChainStatus = {
  key: string;
  nodeId: string;
  healthy: boolean;
  bestBlock: number | null;
  finalizedBlock: number | null;
  peers: number | null;
  isSyncing: boolean | null;
  runtimeVersion: string | null;
  updatedAt: string;
  stale: boolean;
  errors: string[];
};

export type NodeStatus = {
  id: string;
  ip: string;
  chains: Record<string, ChainStatus>;
  updatedAt: string;
  stale: boolean;
};
