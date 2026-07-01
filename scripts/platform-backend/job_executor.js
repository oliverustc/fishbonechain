#!/usr/bin/env node

import path from 'node:path';
import { JsonStore } from './lib/json_store.js';
import { runJob } from './lib/job_runner.js';

const USAGE = `
Usage: node scripts/platform-backend/job_executor.js [options]

Options:
  --data-dir <p>  Path to Stage 18 backend JSON store directory (required)
  --job-id <id>   Run a specific queued job
  --once          Find one queued job and run at most one job, then exit
  --work-dir <p>  Root for generated evidence/artifacts/logs (required)
  --worker-id <s> Executing worker identifier (default: local-worker)
  --dry-run       Validate inputs without invoking proof CLI
  --help          Print this help and exit

Example:
  node scripts/platform-backend/job_executor.js \\
    --data-dir var/platform-backend/ \\
    --job-id <id> \\
    --work-dir .agents/fwf/runs/stage19/dry-run-smoke/work

  node scripts/platform-backend/job_executor.js \\
    --data-dir var/platform-backend/ \\
    --once \\
    --work-dir .agents/fwf/runs/stage19/dry-run-smoke/work \\
    --dry-run
`.trim();

function parseArgs() {
  const args = process.argv.slice(2);
  const opts = { dataDir: null, jobId: null, once: false, workDir: null, workerId: 'local-worker', dryRun: false };
  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--help' || args[i] === '-h') {
      console.log(USAGE);
      process.exit(0);
    } else if (args[i] === '--data-dir' && i + 1 < args.length) {
      opts.dataDir = args[++i];
    } else if (args[i] === '--job-id' && i + 1 < args.length) {
      opts.jobId = args[++i];
    } else if (args[i] === '--once') {
      opts.once = true;
    } else if (args[i] === '--work-dir' && i + 1 < args.length) {
      opts.workDir = args[++i];
    } else if (args[i] === '--worker-id' && i + 1 < args.length) {
      opts.workerId = args[++i];
    } else if (args[i] === '--dry-run') {
      opts.dryRun = true;
    }
  }
  return opts;
}

function validateOpts(opts) {
  if (!opts.dataDir) {
    return 'Error: --data-dir is required';
  }
  if (!opts.jobId && !opts.once) {
    return 'Error: specify --job-id or --once';
  }
  if (opts.jobId && opts.once) {
    return 'Error: --job-id and --once are mutually exclusive';
  }
  if (!opts.workDir) {
    return 'Error: --work-dir is required';
  }
  return null;
}

function main() {
  const opts = parseArgs();

  const validationError = validateOpts(opts);
  if (validationError) {
    console.error(validationError);
    console.log(USAGE);
    process.exit(1);
  }

  const store = new JsonStore(opts.dataDir);
  store.init();

  const workRoot = path.resolve(opts.workDir);
  console.log(`data-dir: ${opts.dataDir}`);
  console.log(`work-dir: ${workRoot}`);
  console.log(`worker-id: ${opts.workerId}`);
  if (opts.dryRun) {
    console.log('mode: dry-run');
  }

  let job = null;
  if (opts.jobId) {
    job = store.findOne('offchain_jobs', j => j.job_id === opts.jobId);
    if (!job) {
      console.error(`Error: job not found: ${opts.jobId}`);
      process.exit(1);
    }
    if (job.status !== 'queued') {
      console.error(`Error: job ${opts.jobId} status is "${job.status}", not "queued"`);
      process.exit(1);
    }
  } else if (opts.once) {
    const queued = store.find('offchain_jobs', j => j.status === 'queued');
    if (queued.length === 0) {
      console.log('No queued jobs found.');
      process.exit(0);
    }
    job = queued[0];
    console.log(`Selected queued job: ${job.job_id} (${job.job_type})`);
  }

  const result = runJob(store, job, workRoot, opts.workerId, opts.dryRun);

  if (result.error) {
    console.error(`Job failed: ${result.error}`);
    console.log(JSON.stringify({ job_id: job.job_id, status: 'failed', error: result.error }, null, 2));
    process.exit(1);
  }

  console.log(`Job completed: ${job.job_id}`);
  console.log(JSON.stringify({
    job_id: result.job.job_id,
    status: result.job.status,
    evidence_id: result.evidence_id,
    digest: result.dig,
    dry_run: result.dryRun,
  }, null, 2));
}

main();
