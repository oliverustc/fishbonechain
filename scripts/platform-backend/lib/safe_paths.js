import fs from 'node:fs';
import path from 'node:path';

const REPO_ROOT = path.resolve(
  path.dirname(new URL(import.meta.url).pathname),
  '../../..'
);

export function resolveRepoPath(inputPath) {
  const resolved = path.resolve(inputPath);
  if (!resolved.startsWith(REPO_ROOT + path.sep) && resolved !== REPO_ROOT) {
    return { error: `path outside repository: ${inputPath}` };
  }
  if (!fs.existsSync(resolved)) {
    return { error: `file not found: ${inputPath}` };
  }
  const real = fs.realpathSync(resolved);
  if (!real.startsWith(REPO_ROOT + path.sep) && real !== REPO_ROOT) {
    return { error: `path resolves outside repository (symlink): ${inputPath}` };
  }
  return { resolved: real };
}

export function resolveWorkDir(workRoot, jobId) {
  const resolved = path.resolve(workRoot, jobId);
  const repoResolved = path.resolve(REPO_ROOT, '.agents');
  if (!resolved.startsWith(REPO_ROOT + path.sep) && resolved !== REPO_ROOT) {
    return { error: `work directory outside repository: ${resolved}` };
  }
  if (resolved.startsWith(repoResolved + path.sep) || resolved === repoResolved) {
    fs.mkdirSync(resolved, { recursive: true });
    return { resolved };
  }
  fs.mkdirSync(resolved, { recursive: true });
  return { resolved };
}
