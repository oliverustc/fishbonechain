import fs from 'node:fs';
import path from 'node:path';

const REPO_ROOT = path.resolve(
  path.dirname(new URL(import.meta.url).pathname),
  '../../..'
);

const IGNORED_ROOTS = [
  path.resolve(REPO_ROOT, '.agents'),
  path.resolve(REPO_ROOT, 'var/platform-backend'),
];

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

  if (!resolved.startsWith(REPO_ROOT + path.sep) && resolved !== REPO_ROOT) {
    return { error: `work directory outside repository: ${resolved}` };
  }

  const isIgnored = IGNORED_ROOTS.some(root =>
    resolved.startsWith(root + path.sep) || resolved === root
  );

  if (!isIgnored) {
    return { error: `work directory must be under an ignored runtime root (.agents/ or var/platform-backend/): ${resolved}` };
  }

  fs.mkdirSync(resolved, { recursive: true });
  return { resolved };
}
