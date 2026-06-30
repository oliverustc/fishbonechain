import { generateId, nowISO } from './ids.js';
import { validate } from './schema.js';
import { errorResponse, jsonResponse, parseBody, extractToken } from './http.js';
import { importChainEventsFromFile, importChainEventsFromArray } from './importers.js';

function requireAuth(store, req) {
  const token = extractToken(req);
  if (!token) return null;
  return store.auth.authenticate(token);
}

function parseIdFromUrl(url, prefix) {
  const rest = url.slice(prefix.length);
  return rest.startsWith('/') ? rest.slice(1) : rest;
}

export function setupRoutes(store) {
  return async function handle(req, res) {
    const { method, url } = req;

    try {
      if (method === 'GET' && url === '/health') {
        return jsonResponse(res, 200, { status: 'ok', timestamp: nowISO() });
      }

      if (method === 'POST' && url === '/api/users/register') {
        const body = await parseBody(req);
        if (!body) return errorResponse(res, 400, 'invalid JSON body');
        const result = store.auth.register(body.display_name, body.role, body.password);
        if (result.error) return errorResponse(res, result.status, result.error);
        return jsonResponse(res, 201, result);
      }

      if (method === 'POST' && url === '/api/users/login') {
        const body = await parseBody(req);
        if (!body) return errorResponse(res, 400, 'invalid JSON body');
        const result = store.auth.login(body.display_name, body.password);
        if (result.error) return errorResponse(res, result.status, result.error);
        return jsonResponse(res, 200, result);
      }

      if (method === 'GET' && url === '/api/users/me') {
        const user = requireAuth(store, req);
        if (!user) return errorResponse(res, 401, 'authentication required');
        return jsonResponse(res, 200, { user });
      }

      if (method === 'POST' && url === '/api/chain-accounts') {
        const user = requireAuth(store, req);
        if (!user) return errorResponse(res, 401, 'authentication required');
        const body = await parseBody(req);
        if (!body) return errorResponse(res, 400, 'invalid JSON body');
        const record = {
          account_id: generateId(),
          user_id: user.user_id,
          chain_id: body.chain_id || '',
          address: body.address || '',
          scene_kind: body.scene_kind || '',
          verified_at: null,
          created_at: nowISO(),
        };
        const v = validate(record, 'chain_accounts');
        if (!v.valid) return errorResponse(res, 400, v.errors.join('; '));
        store.create('chain_accounts', record);
        return jsonResponse(res, 201, {
          account: record,
          verification_status: 'unverified',
          note: 'cryptographic signature challenge verification is deferred; this binding is metadata only',
        });
      }

      if (method === 'GET' && url === '/api/chain-accounts') {
        const user = requireAuth(store, req);
        if (!user) return errorResponse(res, 401, 'authentication required');
        const accounts = store.find('chain_accounts', a => a.user_id === user.user_id);
        return jsonResponse(res, 200, { accounts });
      }

      if (method === 'POST' && url === '/api/business-tasks') {
        const user = requireAuth(store, req);
        if (!user) return errorResponse(res, 401, 'authentication required');
        const body = await parseBody(req);
        if (!body) return errorResponse(res, 400, 'invalid JSON body');
        const record = {
          task_id: generateId(),
          module: body.module || '',
          initiator_account_id: body.initiator_account_id || '',
          counterparty_account_id: body.counterparty_account_id || null,
          reference_ids: body.reference_ids || {},
          status: body.status || 'pending',
          created_at: nowISO(),
          updated_at: nowISO(),
        };
        const v = validate(record, 'business_tasks');
        if (!v.valid) return errorResponse(res, 400, v.errors.join('; '));
        store.create('business_tasks', record);
        return jsonResponse(res, 201, { task: record });
      }

      if (method === 'GET' && url.startsWith('/api/business-tasks/')) {
        const user = requireAuth(store, req);
        if (!user) return errorResponse(res, 401, 'authentication required');
        const taskId = parseIdFromUrl(url, '/api/business-tasks/');
        if (!taskId) return errorResponse(res, 400, 'missing task id');
        const task = store.findOne('business_tasks', t => t.task_id === taskId);
        if (!task) return errorResponse(res, 404, 'task not found');
        return jsonResponse(res, 200, { task });
      }

      if (method === 'GET' && url === '/api/business-tasks') {
        const user = requireAuth(store, req);
        if (!user) return errorResponse(res, 401, 'authentication required');
        const tasks = store.list('business_tasks');
        return jsonResponse(res, 200, { tasks });
      }

      if (method === 'POST' && url === '/api/workflow-runs') {
        const user = requireAuth(store, req);
        if (!user) return errorResponse(res, 401, 'authentication required');
        const body = await parseBody(req);
        if (!body) return errorResponse(res, 400, 'invalid JSON body');
        const record = {
          run_id: generateId(),
          task_id: body.task_id || '',
          session_id: body.session_id ?? null,
          escrow_id: body.escrow_id ?? null,
          round_count: body.round_count ?? 0,
          status: body.status || 'running',
          started_at: body.started_at || null,
          completed_at: body.completed_at || null,
        };
        const v = validate(record, 'workflow_runs');
        if (!v.valid) return errorResponse(res, 400, v.errors.join('; '));
        store.create('workflow_runs', record);
        return jsonResponse(res, 201, { run: record });
      }

      if (method === 'GET' && url === '/api/workflow-runs') {
        const user = requireAuth(store, req);
        if (!user) return errorResponse(res, 401, 'authentication required');
        const runs = store.list('workflow_runs');
        return jsonResponse(res, 200, { runs });
      }

      if (method === 'POST' && url === '/api/evidence') {
        const user = requireAuth(store, req);
        if (!user) return errorResponse(res, 401, 'authentication required');
        const body = await parseBody(req);
        if (!body) return errorResponse(res, 400, 'invalid JSON body');
        const record = {
          evidence_id: generateId(),
          run_id: body.run_id || '',
          category: body.category || '',
          status: body.status || '',
          scenario: body.scenario || null,
          result: body.result || null,
          command: body.command || null,
          log_path: body.log_path || null,
          artifacts: body.artifacts || [],
          chain_tx_hashes: body.chain_tx_hashes || [],
          chain_event_refs: body.chain_event_refs || [],
          constraints: body.constraints || [],
          settlement: body.settlement || null,
          scenario_outcome: body.scenario_outcome || null,
          error: body.error || null,
          created_at: nowISO(),
        };
        const v = validate(record, 'evidence');
        if (!v.valid) return errorResponse(res, 400, v.errors.join('; '));
        store.create('evidence', record);
        return jsonResponse(res, 201, { evidence: record });
      }

      if (method === 'GET' && url === '/api/evidence') {
        const user = requireAuth(store, req);
        if (!user) return errorResponse(res, 401, 'authentication required');
        const records = store.list('evidence');
        return jsonResponse(res, 200, { evidence: records });
      }

      if (method === 'POST' && url === '/api/chain-events/import') {
        const user = requireAuth(store, req);
        if (!user) return errorResponse(res, 401, 'authentication required');
        const contentType = req.headers['content-type'] || '';
        if (contentType === 'text/plain') {
          const body = await new Promise(resolve => {
            const chunks = [];
            req.on('data', chunk => chunks.push(chunk));
            req.on('end', () => resolve(Buffer.concat(chunks).toString('utf-8')));
          });
          const filePath = body.trim();
          if (!filePath) return errorResponse(res, 400, 'empty file path');
          const result = importChainEventsFromFile(store, filePath);
          if (result.error) return errorResponse(res, result.status, result.error);
          return jsonResponse(res, 201, { imported: result.imported, count: result.count });
        }
        const body = await parseBody(req);
        if (!body) return errorResponse(res, 400, 'invalid JSON body');
        const events = Array.isArray(body) ? body : body.events || [];
        const result = importChainEventsFromArray(store, events);
        return jsonResponse(res, 201, { imported: result.imported, count: result.count });
      }

      if (method === 'GET' && url === '/api/chain-events') {
        const user = requireAuth(store, req);
        if (!user) return errorResponse(res, 401, 'authentication required');
        const events = store.list('chain_events');
        return jsonResponse(res, 200, { events, note: 'records are cached indexed events, not chain finality' });
      }

      if (method === 'POST' && url === '/api/offchain-jobs') {
        const user = requireAuth(store, req);
        if (!user) return errorResponse(res, 401, 'authentication required');
        const body = await parseBody(req);
        if (!body) return errorResponse(res, 400, 'invalid JSON body');
        const record = {
          job_id: generateId(),
          job_type: body.job_type || '',
          status: body.status || 'queued',
          input_refs: body.input_refs || [],
          output_refs: body.output_refs || [],
          worker_id: body.worker_id || null,
          digest: body.digest || null,
          error: body.error || null,
          evidence_id: body.evidence_id || null,
          created_at: nowISO(),
          started_at: body.started_at || null,
          completed_at: body.completed_at || null,
        };
        const v = validate(record, 'offchain_jobs');
        if (!v.valid) return errorResponse(res, 400, v.errors.join('; '));
        store.create('offchain_jobs', record);
        return jsonResponse(res, 201, { job: record, note: 'metadata only; no proof generation or worker execution in this stage' });
      }

      if (method === 'GET' && url === '/api/offchain-jobs') {
        const user = requireAuth(store, req);
        if (!user) return errorResponse(res, 401, 'authentication required');
        const jobs = store.list('offchain_jobs');
        return jsonResponse(res, 200, { jobs });
      }

      errorResponse(res, 404, 'not found');
    } catch (err) {
      errorResponse(res, 500, err.message);
    }
  };
}
