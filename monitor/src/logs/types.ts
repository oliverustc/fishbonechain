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
