import { describe, it, beforeEach, afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { JsonStore } from '../lib/json_store.js';
import { runJob } from '../lib/job_runner.js';
import { computeFileDigest } from '../lib/artifact_digest.js';
import { isKnownType, isSupportedExecution, validateProofGenerationInputs, SUPPORTED_EXEC_TYPES, KNOWN_TYPES } from '../lib/job_types.js';
import { resolveRepoPath, resolveWorkDir } from '../lib/safe_paths.js';
import { generateId, nowISO } from '../lib/ids.js';

const TEST_DIR = '.agents/fwf/runs/stage19/executor-test';

function makeStore(label) {
  const dir = path.join(TEST_DIR, `store-${Date.now()}-${Math.random().toString(36).slice(2)}-${label}`);
  const store = new JsonStore(dir);
  store.init();
  return store;
}

function seedWorkflowRun(store, runId) {
  const record = {
    run_id: runId || generateId(),
    task_id: generateId(),
    session_id: null,
    escrow_id: null,
    round_count: 0,
    status: 'running',
    started_at: null,
    completed_at: null,
  };
  store.create('workflow_runs', record);
  return record;
}

function makeProofGenerationJob(overrides = {}) {
  return {
    job_id: overrides.job_id || generateId(),
    job_type: overrides.job_type || 'proof_generation',
    status: overrides.status || 'queued',
    input_refs: overrides.input_refs || [],
    output_refs: overrides.output_refs || [],
    worker_id: null,
    digest: null,
    error: null,
    evidence_id: null,
    created_at: nowISO(),
    started_at: null,
    completed_at: null,
  };
}

describe('job_types', () => {
  it('KNOWN_TYPES includes all defined types', () => {
    assert.ok(KNOWN_TYPES.has('proof_generation'));
    assert.ok(KNOWN_TYPES.has('data_preprocessing'));
    assert.ok(KNOWN_TYPES.has('anonymization'));
    assert.ok(KNOWN_TYPES.has('verification'));
    assert.ok(KNOWN_TYPES.has('training'));
  });

  it('SUPPORTED_EXEC_TYPES includes only proof_generation', () => {
    assert.equal(SUPPORTED_EXEC_TYPES.size, 1);
    assert.ok(SUPPORTED_EXEC_TYPES.has('proof_generation'));
  });

  it('isKnownType returns true for known types', () => {
    assert.ok(isKnownType('proof_generation'));
    assert.ok(isKnownType('training'));
  });

  it('isKnownType returns false for unknown types', () => {
    assert.ok(!isKnownType('unknown_type'));
    assert.ok(!isKnownType(''));
  });

  it('isSupportedExecution returns true only for proof_generation', () => {
    assert.ok(isSupportedExecution('proof_generation'));
    assert.ok(!isSupportedExecution('training'));
    assert.ok(!isSupportedExecution('data_preprocessing'));
  });

  it('validateProofGenerationInputs rejects missing workflow_run', () => {
    const result = validateProofGenerationInputs([
      { artifact_type: 'profile', path: 'test', digest: null },
      { artifact_type: 'dataset', path: 'd.json', digest: null },
      { artifact_type: 'request', path: 'r.json', digest: null },
    ]);
    assert.ok(!result.valid);
    assert.ok(result.error.includes('workflow_run'));
  });

  it('validateProofGenerationInputs rejects missing multiple', () => {
    const result = validateProofGenerationInputs([]);
    assert.ok(!result.valid);
    assert.ok(result.error.includes('workflow_run'));
    assert.ok(result.error.includes('profile'));
  });

  it('validateProofGenerationInputs accepts valid input refs', () => {
    const result = validateProofGenerationInputs([
      { artifact_type: 'workflow_run', path: 'run-1', digest: null },
      { artifact_type: 'profile', path: 'test', digest: null },
      { artifact_type: 'dataset', path: 'd.json', digest: null },
      { artifact_type: 'request', path: 'r.json', digest: null },
    ]);
    assert.ok(result.valid);
    assert.equal(result.byType.workflow_run.path, 'run-1');
  });
});

describe('safe_paths', () => {
  it('resolveRepoPath rejects paths outside repo', () => {
    const result = resolveRepoPath('/etc/passwd');
    assert.ok(result.error);
    assert.ok(result.error.includes('outside repository'));
  });

  it('resolveRepoPath rejects non-existent paths', () => {
    const result = resolveRepoPath(path.join(TEST_DIR, 'nonexistent-file.json'));
    assert.ok(result.error);
    assert.ok(result.error.includes('not found'));
  });

  it('resolveRepoPath resolves valid repo files', () => {
    const result = resolveRepoPath('scripts/platform-backend/server.js');
    assert.ok(!result.error);
    assert.ok(result.resolved.endsWith(path.normalize('scripts/platform-backend/server.js')));
  });

  it('resolveWorkDir creates and resolves directory', () => {
    const workRoot = path.join(TEST_DIR, `work-${Date.now()}`);
    const jobId = 'test-job-1';
    const result = resolveWorkDir(workRoot, jobId);
    assert.ok(!result.error);
    assert.ok(result.resolved.includes(jobId));
    assert.ok(fs.existsSync(result.resolved));
  });
});

describe('runJob', () => {
  let store;
  let workRoot;
  let runId;

  beforeEach(() => {
    store = makeStore('runJob');
    workRoot = path.join(TEST_DIR, `work-${Date.now()}-${Math.random().toString(36).slice(2)}`);
    fs.mkdirSync(workRoot, { recursive: true });
    runId = generateId();
    seedWorkflowRun(store, runId);
  });

  afterEach(() => {
    try { fs.rmSync(workRoot, { recursive: true, force: true }); } catch (_) { /* ignore */ }
  });

  it('fails unknown job type', () => {
    const job = makeProofGenerationJob({ job_type: 'unknown_type' });
    store.create('offchain_jobs', job);
    const result = runJob(store, job, workRoot, 'test-worker', true);
    assert.ok(result.error);
    assert.ok(result.error.includes('unknown job type'));
    const updated = store.findOne('offchain_jobs', j => j.job_id === job.job_id);
    assert.equal(updated.status, 'failed');
  });

  it('fails unsupported execution type', () => {
    const job = makeProofGenerationJob({ job_type: 'training' });
    store.create('offchain_jobs', job);
    const result = runJob(store, job, workRoot, 'test-worker', true);
    assert.ok(result.error);
    assert.ok(result.error.includes('unsupported execution type'));
    const updated = store.findOne('offchain_jobs', j => j.job_id === job.job_id);
    assert.equal(updated.status, 'failed');
  });

  it('fails missing required inputs', () => {
    const job = makeProofGenerationJob({ input_refs: [] });
    store.create('offchain_jobs', job);
    const result = runJob(store, job, workRoot, 'test-worker', true);
    assert.ok(result.error);
    assert.ok(result.error.includes('missing required inputs'));
    const updated = store.findOne('offchain_jobs', j => j.job_id === job.job_id);
    assert.equal(updated.status, 'failed');
  });

  it('fails missing workflow_run reference', () => {
    const job = makeProofGenerationJob({
      input_refs: [
        { artifact_type: 'workflow_run', path: 'nonexistent-run-id', digest: null },
        { artifact_type: 'profile', path: 'test-profile', digest: null },
        { artifact_type: 'dataset', path: 'scripts/fixtures/data_trade_datasets/factory_sensors.json', digest: null },
        { artifact_type: 'request', path: 'scripts/fixtures/data_trade_requests/factory_temperature_range.json', digest: null },
      ],
    });
    store.create('offchain_jobs', job);
    const result = runJob(store, job, workRoot, 'test-worker', true);
    assert.ok(result.error);
    assert.ok(result.error.includes('workflow_run not found'));
    const updated = store.findOne('offchain_jobs', j => j.job_id === job.job_id);
    assert.equal(updated.status, 'failed');
    const evidenceRecords = store.find('evidence', e => e.run_id === runId || e.run_id === 'nonexistent-run-id');
    assert.equal(evidenceRecords.length, 0);
  });

  it('successful dry-run proof_generation transitions to completed', () => {
    const job = makeProofGenerationJob({
      input_refs: [
        { artifact_type: 'workflow_run', path: runId, digest: null },
        { artifact_type: 'profile', path: 'test-profile', digest: null },
        { artifact_type: 'dataset', path: 'scripts/fixtures/data_trade_datasets/factory_sensors.json', digest: null },
        { artifact_type: 'request', path: 'scripts/fixtures/data_trade_requests/factory_temperature_range.json', digest: null },
      ],
    });
    store.create('offchain_jobs', job);
    const result = runJob(store, job, workRoot, 'test-worker', true);
    assert.ok(!result.error);
    assert.ok(result.success);
    assert.ok(result.dryRun);
    assert.ok(result.dig);
    assert.ok(result.evidence_id);

    const updated = store.findOne('offchain_jobs', j => j.job_id === job.job_id);
    assert.equal(updated.status, 'completed');
    assert.equal(updated.worker_id, 'test-worker');
    assert.ok(updated.started_at);
    assert.ok(updated.completed_at);
    assert.equal(updated.digest, result.dig);
    assert.equal(updated.evidence_id, result.evidence_id);
    assert.equal(updated.output_refs.length, 1);
    assert.equal(updated.output_refs[0].artifact_type, 'evidence');
    assert.equal(updated.output_refs[0].digest, result.dig);
    assert.ok(updated.output_refs[0].path.includes('evidence.json'));

    const evidence = store.findOne('evidence', e => e.evidence_id === result.evidence_id);
    assert.ok(evidence);
    assert.equal(evidence.category, 'dry_run');
    assert.equal(evidence.scenario, 'executor-dry-run');
    assert.equal(evidence.result, 'executor-dry-run-completed');
    assert.equal(evidence.run_id, runId);
    assert.equal(evidence.status, 'passed');

    const evidencePath = path.join(workRoot, job.job_id, 'evidence.json');
    assert.ok(fs.existsSync(evidencePath));
    const evidenceFile = JSON.parse(fs.readFileSync(evidencePath, 'utf-8'));
    assert.equal(evidenceFile.executor_dry_run, true);
    assert.equal(evidenceFile.job_id, job.job_id);
  });

  it('digest computed for evidence file is deterministic', () => {
    const job = makeProofGenerationJob({
      input_refs: [
        { artifact_type: 'workflow_run', path: runId, digest: null },
        { artifact_type: 'profile', path: 'test-profile', digest: null },
        { artifact_type: 'dataset', path: 'scripts/fixtures/data_trade_datasets/factory_sensors.json', digest: null },
        { artifact_type: 'request', path: 'scripts/fixtures/data_trade_requests/factory_temperature_range.json', digest: null },
      ],
    });
    store.create('offchain_jobs', job);
    const result = runJob(store, job, workRoot, 'test-worker', true);
    const evidencePath = path.join(workRoot, job.job_id, 'evidence.json');
    const directDigest = computeFileDigest(evidencePath);
    assert.equal(result.dig, directDigest);
  });

  it('output_refs contains evidence artifact with digest', () => {
    const job = makeProofGenerationJob({
      input_refs: [
        { artifact_type: 'workflow_run', path: runId, digest: null },
        { artifact_type: 'profile', path: 'test-profile', digest: null },
        { artifact_type: 'dataset', path: 'scripts/fixtures/data_trade_datasets/factory_sensors.json', digest: null },
        { artifact_type: 'request', path: 'scripts/fixtures/data_trade_requests/factory_temperature_range.json', digest: null },
      ],
    });
    store.create('offchain_jobs', job);
    const result = runJob(store, job, workRoot, 'test-worker', true);
    const updated = store.findOne('offchain_jobs', j => j.job_id === job.job_id);
    assert.equal(updated.output_refs.length, 1);
    assert.equal(updated.output_refs[0].artifact_type, 'evidence');
    assert.ok(updated.output_refs[0].digest);
    assert.ok(updated.output_refs[0].path);
  });

  it('evidence_id links job to evidence record', () => {
    const job = makeProofGenerationJob({
      input_refs: [
        { artifact_type: 'workflow_run', path: runId, digest: null },
        { artifact_type: 'profile', path: 'test-profile', digest: null },
        { artifact_type: 'dataset', path: 'scripts/fixtures/data_trade_datasets/factory_sensors.json', digest: null },
        { artifact_type: 'request', path: 'scripts/fixtures/data_trade_requests/factory_temperature_range.json', digest: null },
      ],
    });
    store.create('offchain_jobs', job);
    const result = runJob(store, job, workRoot, 'test-worker', true);
    const evidence = store.findOne('evidence', e => e.evidence_id === result.evidence_id);
    assert.ok(evidence);
    assert.equal(evidence.evidence_id, result.evidence_id);
    assert.equal(evidence.run_id, runId);
  });

  it('null job returns error', () => {
    const result = runJob(store, null, workRoot, 'test-worker', true);
    assert.ok(result.error);
    assert.ok(result.error.includes('null'));
  });

  it('failed jobs do not create evidence records', () => {
    const evidenceBefore = store.list('evidence').length;

    const job = makeProofGenerationJob({
      input_refs: [
        { artifact_type: 'workflow_run', path: 'missing-run', digest: null },
        { artifact_type: 'profile', path: 'test', digest: null },
        { artifact_type: 'dataset', path: 'd.json', digest: null },
        { artifact_type: 'request', path: 'r.json', digest: null },
      ],
    });
    store.create('offchain_jobs', job);
    runJob(store, job, workRoot, 'test-worker', true);

    const evidenceAfter = store.list('evidence').length;
    assert.equal(evidenceAfter, evidenceBefore);
    assert.equal(store.findOne('offchain_jobs', j => j.job_id === job.job_id).status, 'failed');
  });

  it('failed job records started_at and completed_at', () => {
    const job = makeProofGenerationJob({ input_refs: [] });
    store.create('offchain_jobs', job);
    runJob(store, job, workRoot, 'test-worker', true);
    const updated = store.findOne('offchain_jobs', j => j.job_id === job.job_id);
    assert.equal(updated.status, 'failed');
    assert.ok(updated.started_at);
    assert.ok(updated.completed_at);
    assert.ok(updated.error);
    assert.ok(!updated.evidence_id);
  });

  it('dry-run output contains executor_dry_run: true marker', () => {
    const job = makeProofGenerationJob({
      input_refs: [
        { artifact_type: 'workflow_run', path: runId, digest: null },
        { artifact_type: 'profile', path: 'test-profile', digest: null },
        { artifact_type: 'dataset', path: 'scripts/fixtures/data_trade_datasets/factory_sensors.json', digest: null },
        { artifact_type: 'request', path: 'scripts/fixtures/data_trade_requests/factory_temperature_range.json', digest: null },
      ],
    });
    store.create('offchain_jobs', job);
    const result = runJob(store, job, workRoot, 'test-worker', true);
    const evidencePath = path.join(workRoot, job.job_id, 'evidence.json');
    const content = JSON.parse(fs.readFileSync(evidencePath, 'utf-8'));
    assert.equal(content.executor_dry_run, true);
    assert.equal(content.mode, 'executor-dry-run');
  });

  it('real execution with no ZK binary fails with error', () => {
    const job = makeProofGenerationJob({
      input_refs: [
        { artifact_type: 'workflow_run', path: runId, digest: null },
        { artifact_type: 'profile', path: 'test-profile', digest: null },
        { artifact_type: 'dataset', path: 'scripts/fixtures/data_trade_datasets/factory_sensors.json', digest: null },
        { artifact_type: 'request', path: 'scripts/fixtures/data_trade_requests/factory_temperature_range.json', digest: null },
      ],
    });
    store.create('offchain_jobs', job);
    const result = runJob(store, job, workRoot, 'test-worker', false);
    const updated = store.findOne('offchain_jobs', j => j.job_id === job.job_id);
    assert.equal(updated.status, 'failed');
    assert.ok(updated.error);
    assert.ok(!updated.evidence_id);
  });
});
