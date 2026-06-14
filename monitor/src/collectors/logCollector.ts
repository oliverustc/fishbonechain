import { execFile } from "node:child_process";
import { homedir } from "node:os";
import { join } from "node:path";
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
  maxConcurrency?: number;
  sshUser?: string;
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
  const maxConcurrency = Math.max(1, input.maxConcurrency ?? 4);
  const sshUser = input.sshUser ?? process.env.FISHBONE_VM_SSH_USER ?? "debian";
  const snapshots: LogSnapshot[] = [];
  const errors: string[] = [];

  const jobs = input.inventory.nodes.flatMap((node) =>
    node.roles.map(
      (chainKey) => async () => {
        const path = `${input.inventory.logDir}/${chainKey}.log`;
        const command = `tail -n ${maxLines} ${path}`;
        const host = `${sshUser}@${node.ip}`;
        const updatedAt = now();

        try {
          const result = await runCommand(host, command);
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
      },
    ),
  );
  await runLimited(jobs, maxConcurrency);

  snapshots.sort((a, b) => `${a.chainKey}:${a.nodeId}`.localeCompare(`${b.chainKey}:${b.nodeId}`));
  return { snapshots, errors };
}

async function runLimited(jobs: Array<() => Promise<void>>, maxConcurrency: number): Promise<void> {
  let next = 0;
  const workers = Array.from({ length: Math.min(maxConcurrency, jobs.length) }, async () => {
    while (next < jobs.length) {
      const job = jobs[next];
      next += 1;
      await job();
    }
  });
  await Promise.all(workers);
}

async function runSshCommand(host: string, command: string): Promise<CommandResult> {
  const identityFile = process.env.FISHBONE_VM_SSH_IDENTITY_FILE ?? join(homedir(), ".ssh", "debian-dev");
  const args = [
    "-o",
    "StrictHostKeyChecking=no",
    "-o",
    "BatchMode=yes",
    "-o",
    "ConnectTimeout=5",
  ];
  if (identityFile) {
    args.push("-i", identityFile, "-o", "IdentitiesOnly=yes");
  }
  args.push(host, command);

  try {
    const result = await execFileAsync("ssh", args);
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
