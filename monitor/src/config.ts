import { isAbsolute, resolve } from "node:path";

export type MonitorConfig = {
  host: string;
  port: number;
  configPath: string;
  pollIntervalMs: number;
  staleAfterMs: number;
};

export function loadMonitorConfig(
  env: NodeJS.ProcessEnv = process.env,
  cwd = process.cwd(),
): MonitorConfig {
  const configPath = env.FISHBONE_CONFIG_PATH ?? resolve(cwd, "..", "deploy", "config.toml");
  return {
    host: env.FISHBONE_MONITOR_HOST ?? "0.0.0.0",
    port: readPositiveInt(env, "FISHBONE_MONITOR_PORT", 18080),
    configPath: isAbsolute(configPath) ? configPath : resolve(cwd, configPath),
    pollIntervalMs: readPositiveInt(env, "FISHBONE_POLL_INTERVAL_MS", 5000),
    staleAfterMs: readPositiveInt(env, "FISHBONE_STALE_AFTER_MS", 15000),
  };
}

function readPositiveInt(env: NodeJS.ProcessEnv, name: string, fallback: number): number {
  const raw = env[name];
  if (raw === undefined || raw === "") return fallback;
  const parsed = Number.parseInt(raw, 10);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error(`${name} must be a positive integer`);
  }
  return parsed;
}
