import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { generateId, nowISO } from './ids.js';
import { computeFileDigest } from './artifact_digest.js';
import { resolveRepoPath, resolveWorkDir } from './safe_paths.js';
import { isKnownType, isSupportedExecution, validateProofGenerationInputs } from './job_types.js';

export function resolveWorkflowRun(store, inputRefs) {
  const workflowRef = inputRefs.find(r => r.artifact_type === 'workflow_run');
  if (!workflowRef || !workflowRef.path) {
    return { error: 'missing workflow_run input reference' };
  }
  const runId = workflowRef.path;
  const run = store.findOne('workflow_runs', r => r.run_id === runId);
  if (!run) {
    return { error: `workflow_run not found: ${runId}` };
  }
  return { runId };
}

export function resolveGenerateProofArgs(job, workDir) {
  const byType = {};
  for (const ref of (job.input_refs || [])) {
    if (!ref.artifact_type) continue;
    byType[ref.artifact_type] = ref;
  }

  const profile = byType.profile?.path;
  const datasetPath = byType.dataset?.path;
  const requestPath = byType.request?.path;

  if (!profile || !datasetPath || !requestPath) {
    return { error: 'missing profile, dataset, or request input' };
  }

  const evidenceOut = path.join(workDir, 'evidence.json');

  return {
    args: [
      'scripts/data_trade_cli.js', 'generate-proof',
      '--profile', profile,
      '--dataset', datasetPath,
      '--request', requestPath,
      '--evidence-out', evidenceOut,
    ],
    evidenceOut,
  };
}

export function transitionJob(job, status) {
  job.status = status;
  if (!job.started_at) {
    job.started_at = nowISO();
  }
  if (status === 'completed' || status === 'failed') {
    job.completed_at = nowISO();
  }
}

export function createEvidenceRecord(store, runId, category, options = {}) {
  const record = {
    evidence_id: generateId(),
    run_id: runId,
    category,
    status: options.status || 'passed',
    scenario: options.scenario || null,
    result: options.result || null,
    command: options.command || null,
    log_path: options.log_path || null,
    artifacts: options.artifacts || [],
    chain_tx_hashes: options.chain_tx_hashes || [],
    chain_event_refs: options.chain_event_refs || [],
    constraints: options.constraints || [],
    settlement: null,
    scenario_outcome: null,
    error: options.error || null,
    created_at: nowISO(),
  };
  store.create('evidence', record);
  return record;
}

export function runJob(store, job, workRoot, workerId, dryRun) {
  if (!job) {
    return { error: 'job is null or undefined' };
  }

  const jobId = job.job_id;
  job.worker_id = workerId;

  if (!isKnownType(job.job_type)) {
    transitionJob(job, 'failed');
    job.error = `unknown job type: ${job.job_type}`;
    store.update('offchain_jobs', j => j.job_id === jobId, j => Object.assign(j, job));
    return { error: job.error };
  }

  if (!isSupportedExecution(job.job_type)) {
    transitionJob(job, 'failed');
    job.error = `unsupported execution type: ${job.job_type}`;
    store.update('offchain_jobs', j => j.job_id === jobId, j => Object.assign(j, job));
    return { error: job.error };
  }

  const inputCheck = validateProofGenerationInputs(job.input_refs || []);
  if (!inputCheck.valid) {
    transitionJob(job, 'failed');
    job.error = inputCheck.error;
    store.update('offchain_jobs', j => j.job_id === jobId, j => Object.assign(j, job));
    return { error: job.error };
  }

  const wfResolved = resolveWorkflowRun(store, job.input_refs);
  if (wfResolved.error) {
    transitionJob(job, 'failed');
    job.error = wfResolved.error;
    store.update('offchain_jobs', j => j.job_id === jobId, j => Object.assign(j, job));
    return { error: job.error };
  }
  const runId = wfResolved.runId;

  const workResolved = resolveWorkDir(workRoot, jobId);
  if (workResolved.error) {
    transitionJob(job, 'failed');
    job.error = workResolved.error;
    store.update('offchain_jobs', j => j.job_id === jobId, j => Object.assign(j, job));
    return { error: job.error };
  }
  const workDir = workResolved.resolved;

  if (!dryRun) {
    const cliResolved = resolveGenerateProofArgs(job, workDir);
    if (cliResolved.error) {
      transitionJob(job, 'failed');
      job.error = cliResolved.error;
      store.update('offchain_jobs', j => j.job_id === jobId, j => Object.assign(j, job));
      return { error: job.error };
    }

    for (const artifactRef of [inputCheck.byType.dataset, inputCheck.byType.request]) {
      if (!artifactRef || !artifactRef.path) continue;
      const resolved = resolveRepoPath(artifactRef.path);
      if (resolved.error) {
        transitionJob(job, 'failed');
        job.error = `input path unsafe: ${artifactRef.path} — ${resolved.error}`;
        store.update('offchain_jobs', j => j.job_id === jobId, j => Object.assign(j, job));
        return { error: job.error };
      }
    }
  }

  transitionJob(job, 'running');
  store.update('offchain_jobs', j => j.job_id === jobId, j => Object.assign(j, job));

  if (dryRun) {
    return executeDryRun(store, job, workDir, workerId, runId);
  }

  return executeRealProofGeneration(store, job, workDir, workerId, runId);
}

function executeDryRun(store, job, workDir, workerId, runId) {
  const jobId = job.job_id;

  const evidencePath = path.join(workDir, 'evidence.json');
  const logPath = path.join(workDir, 'dry-run.log');
  const logLines = [
    `executor dry-run for job ${jobId}`,
    `worker_id: ${workerId}`,
    `timestamp: ${nowISO()}`,
    `dry_run: true`,
  ];

  const dryRunEvidence = {
    version: 1,
    job_id: jobId,
    job_type: job.job_type,
    executor_dry_run: true,
    mode: 'executor-dry-run',
    worker_id: workerId,
    run_id: runId,
    timestamp: nowISO(),
    note: 'This is an executor dry-run artifact. It is NOT a ZK proof or chain evidence.',
  };

  fs.writeFileSync(evidencePath, JSON.stringify(dryRunEvidence, null, 2), 'utf-8');
  fs.writeFileSync(logPath, logLines.join('\n') + '\n', 'utf-8');

  const dig = computeFileDigest(evidencePath);

  const evidenceRecord = createEvidenceRecord(store, runId, 'dry_run', {
    scenario: 'executor-dry-run',
    result: 'executor-dry-run-completed',
    command: 'node scripts/platform-backend/job_executor.js --dry-run',
    log_path: logPath,
    artifacts: [
      { artifact_type: 'evidence', path: evidencePath, digest: dig },
    ],
  });

  job.status = 'completed';
  job.completed_at = nowISO();
  job.digest = dig;
  job.evidence_id = evidenceRecord.evidence_id;
  job.output_refs = job.output_refs || [];
  job.output_refs.push({ artifact_type: 'evidence', path: evidencePath, digest: dig });

  store.update('offchain_jobs', j => j.job_id === jobId, j => Object.assign(j, job));

  return {
    success: true,
    job: { ...job },
    evidence_id: evidenceRecord.evidence_id,
    dig,
    dryRun: true,
  };
}

function executeRealProofGeneration(store, job, workDir, workerId, runId) {
  const jobId = job.job_id;

  const cliResolved = resolveGenerateProofArgs(job, workDir);
  if (cliResolved.error) {
    transitionJob(job, 'failed');
    job.error = cliResolved.error;
    store.update('offchain_jobs', j => j.job_id === jobId, j => Object.assign(j, job));
    return { error: job.error };
  }

  const logPath = path.join(workDir, 'proof-generation.log');
  const logStream = fs.openSync(logPath, 'w');

  const result = spawnSync(process.execPath, cliResolved.args, {
    cwd: process.cwd(),
    timeout: 300000,
  });

  const stdout = result.stdout ? result.stdout.toString('utf-8') : '';
  const stderr = result.stderr ? result.stderr.toString('utf-8') : '';
  const exitCode = result.status;
  const signal = result.signal;

  fs.writeSync(logStream, stdout);
  if (stderr) {
    fs.writeSync(logStream, `\n--- STDERR ---\n${stderr}`);
  }
  fs.writeSync(logStream, `\nexit_code: ${exitCode ?? 'null'} signal: ${signal ?? 'null'}\n`);
  fs.closeSync(logStream);

  if (exitCode !== 0) {
    transitionJob(job, 'failed');
    job.error = `proof generation failed: exit ${exitCode}${signal ? ` (signal: ${signal})` : ''}`;
    store.update('offchain_jobs', j => j.job_id === jobId, j => Object.assign(j, job));
    return { error: job.error, exitCode };
  }

  const evidenceOut = cliResolved.evidenceOut;
  if (!fs.existsSync(evidenceOut)) {
    transitionJob(job, 'failed');
    job.error = `proof generation completed but evidence output not found: ${evidenceOut}`;
    store.update('offchain_jobs', j => j.job_id === jobId, j => Object.assign(j, job));
    return { error: job.error };
  }

  const dig = computeFileDigest(evidenceOut);

  const evidenceRecord = createEvidenceRecord(store, runId, 'postcheck', {
    scenario: job.job_type,
    result: 'completed',
    command: cliResolved.args.join(' '),
    log_path: logPath,
    artifacts: [
      { artifact_type: 'evidence', path: evidenceOut, digest: dig },
    ],
  });

  job.status = 'completed';
  job.completed_at = nowISO();
  job.digest = dig;
  job.evidence_id = evidenceRecord.evidence_id;
  job.output_refs = job.output_refs || [];
  job.output_refs.push({ artifact_type: 'evidence', path: evidenceOut, digest: dig });

  store.update('offchain_jobs', j => j.job_id === jobId, j => Object.assign(j, job));

  return {
    success: true,
    job: { ...job },
    evidence_id: evidenceRecord.evidence_id,
    dig,
    dryRun: false,
  };
}
