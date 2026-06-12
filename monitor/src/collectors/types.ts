export type CollectorResult<T> = {
  ok: boolean;
  data: T | null;
  errors: string[];
  startedAt: string;
  finishedAt: string;
};

export type CollectorHealth = {
  name: string;
  lastStartedAt: string | null;
  lastFinishedAt: string | null;
  lastDurationMs: number | null;
  lastOk: boolean | null;
  errors: string[];
};
