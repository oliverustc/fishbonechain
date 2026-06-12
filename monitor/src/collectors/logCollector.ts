import { execFile } from "node:child_process";
import { promisify } from "node:util";

import type { InventorySnapshot } from "../inventory/types.js";
import type { LogSnapshot } from "../logs/types.js";

const execFileAsync = promisify(execFile);

export type CommandResult = {
  code: number;
  stdout: string;
  stderr: string;
};

export type CommandRunner = (host: string, command: string) => Promise<CommandResult>;

export type CollectLogsInput = {
  inventory: InventorySnapshot;
  maxLines: number;
  runCommand?: CommandRunner;
  now?: () => string;
};

export type CollectLogsResult = {
  snapshots: LogSnapshot[];
  errors: string[];
};

export async function collectLogs(input: CollectLogsInput): Promise<CollectLogsResult> {
  const runCommand = input.runCommand ?? runSshCommand;
  const now = input.now ?? (() => new Date().toISOString());
  const maxLines = Math.max(1, Math.min(input.maxLines, 1000));
  const snapshots: LogSnapshot[] = [];
  const errors: string[] = [];

  await Promise.all(
    input.inventory.nodes.flatMap((node) =>
      node.roles.map(async (chainKey) => {
        const path = `${input.inventory.logDir}/${chainKey}.log`;
        const command = `tail -n ${maxLines} ${path}`;
        const updatedAt = now();

        try {
          const result = await runCommand(node.ssh, command);
          const error = result.stderr.trim() || `command exited with code ${result.code}`;
          const snapshot: LogSnapshot = {
            nodeId: node.id,
            chainKey,
            path,
            updatedAt,
            ok: result.code === 0,
            lines: result.code === 0 ? splitLines(result.stdout) : [],
            errors: result.code === 0 ? [] : [error],
          };
          snapshots.push(snapshot);
          if (!snapshot.ok) {
            errors.push(`${node.id}:${chainKey}:${error}`);
          }
        } catch (error) {
          const message = error instanceof Error ? error.message : String(error);
          snapshots.push({
            nodeId: node.id,
            chainKey,
            path,
            updatedAt,
            ok: false,
            lines: [],
            errors: [message],
          });
          errors.push(`${node.id}:${chainKey}:${message}`);
        }
      }),
    ),
  );

  snapshots.sort((a, b) => `${a.chainKey}:${a.nodeId}`.localeCompare(`${b.chainKey}:${b.nodeId}`));
  return { snapshots, errors };
}

async function runSshCommand(host: string, command: string): Promise<CommandResult> {
  try {
    const result = await execFileAsync("ssh", [
      "-o",
      "StrictHostKeyChecking=no",
      "-o",
      "ConnectTimeout=5",
      host,
      command,
    ]);
    return { code: 0, stdout: result.stdout, stderr: result.stderr };
  } catch (error) {
    const maybe = error as { code?: number; stdout?: string; stderr?: string; message?: string };
    return {
      code: typeof maybe.code === "number" ? maybe.code : 1,
      stdout: maybe.stdout ?? "",
      stderr: maybe.stderr ?? maybe.message ?? "ssh command failed",
    };
  }
}

function splitLines(text: string): string[] {
  return text.split(/\r?\n/).filter((line) => line.length > 0);
}
