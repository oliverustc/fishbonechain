const USER_FIELDS = ['user_id', 'display_name', 'role', 'password_hash', 'created_at', 'updated_at'];
const CHAIN_ACCOUNT_FIELDS = ['account_id', 'user_id', 'chain_id', 'address', 'scene_kind', 'verified_at', 'created_at'];
const BUSINESS_TASK_FIELDS = ['task_id', 'module', 'initiator_account_id', 'counterparty_account_id', 'reference_ids', 'status', 'created_at', 'updated_at'];
const WORKFLOW_RUN_FIELDS = ['run_id', 'task_id', 'session_id', 'escrow_id', 'round_count', 'status', 'started_at', 'completed_at'];
const EVIDENCE_FIELDS = ['evidence_id', 'run_id', 'category', 'status', 'scenario', 'result', 'command', 'log_path', 'artifacts', 'chain_tx_hashes', 'chain_event_refs', 'constraints', 'settlement', 'scenario_outcome', 'error', 'created_at'];
const CHAIN_EVENT_FIELDS = ['event_id', 'chain_id', 'block_number', 'block_hash', 'extrinsic_index', 'event_index', 'pallet', 'variant', 'fields', 'cursor', 'ingested_at'];
const OFFCHAIN_JOB_FIELDS = ['job_id', 'job_type', 'status', 'input_refs', 'output_refs', 'worker_id', 'digest', 'error', 'evidence_id', 'created_at', 'started_at', 'completed_at'];

const COLLECTION_FIELDS = {
  users: USER_FIELDS,
  sessions: ['token', 'user_id', 'created_at'],
  chain_accounts: CHAIN_ACCOUNT_FIELDS,
  business_tasks: BUSINESS_TASK_FIELDS,
  workflow_runs: WORKFLOW_RUN_FIELDS,
  evidence: EVIDENCE_FIELDS,
  chain_events: CHAIN_EVENT_FIELDS,
  offchain_jobs: OFFCHAIN_JOB_FIELDS,
};

function missingFields(record, collection) {
  const allowed = COLLECTION_FIELDS[collection];
  if (!allowed) return [`unknown collection: ${collection}`];
  const missing = [];
  for (const field of allowed) {
    if (!(field in record)) {
      missing.push(field);
    }
  }
  return missing;
}

export function validate(record, collection) {
  const missing = missingFields(record, collection);
  if (missing.length > 0) {
    return { valid: false, errors: missing.map(f => `missing required field: ${f}`) };
  }
  return { valid: true, errors: [] };
}
