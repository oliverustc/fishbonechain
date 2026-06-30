import { describe, it, before, after } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import { JsonStore } from '../lib/json_store.js';
import { AuthService } from '../lib/auth.js';
import { setupRoutes } from '../lib/routes.js';

const TEST_DIR = '.agents/fwf/runs/stage18/backend-test';

function fetchJSON(port, method, urlPath, opts = {}) {
  return new Promise((resolve, reject) => {
    const headers = { 'content-type': 'application/json', ...opts.headers };
    const body = opts.body ? (typeof opts.body === 'string' ? opts.body : JSON.stringify(opts.body)) : null;
    if (body) {
      headers['content-length'] = Buffer.byteLength(body);
    } else {
      delete headers['content-type'];
    }

    const req = http.request({
      hostname: '127.0.0.1',
      port,
      path: urlPath,
      method,
      headers,
    }, (res) => {
      const chunks = [];
      res.on('data', chunk => chunks.push(chunk));
      res.on('end', () => {
        const raw = Buffer.concat(chunks).toString('utf-8');
        let data = raw;
        try { data = JSON.parse(raw); } catch {}
        resolve({ status: res.statusCode, headers: res.headers, body: data });
      });
    });
    req.on('error', reject);
    if (body) req.write(body);
    req.end();
  });
}

let httpServer;
let serverPort;
let store;

function startServer(dataDir) {
  return new Promise((resolve, reject) => {
    store = new JsonStore(dataDir);
    store.init();
    store.auth = new AuthService(store);
    const handle = setupRoutes(store);

    httpServer = http.createServer(async (req, res) => {
      try { await handle(req, res); } catch (err) {
        res.writeHead(500, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: { status: 500, message: err.message } }));
      }
    });

    httpServer.listen(0, '127.0.0.1', () => {
      serverPort = httpServer.address().port;
      resolve();
    });
    httpServer.on('error', reject);
  });
}

function stopServer() {
  return new Promise(resolve => {
    if (httpServer) httpServer.close(() => resolve());
    else resolve();
  });
}

describe('API endpoints', () => {
  let dataDir;

  before(async () => {
    dataDir = path.join(TEST_DIR, `api-server-${Date.now()}-${Math.random().toString(36).slice(2)}`);
    await startServer(dataDir);
  });

  after(async () => {
    await stopServer();
    fs.rmSync(dataDir, { recursive: true, force: true });
  });

  it('GET /health returns ok', async () => {
    const { status, body } = await fetchJSON(serverPort, 'GET', '/health');
    assert.equal(status, 200);
    assert.equal(body.status, 'ok');
    assert.ok(body.timestamp);
  });

  it('POST /api/users/register creates user without leaking password hash', async () => {
    const { status, body } = await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'alice', role: 'data_owner', password: 'secret123' },
    });
    assert.equal(status, 201);
    assert.ok(body.user);
    assert.equal(body.user.display_name, 'alice');
    assert.ok(!body.user.password_hash);
    assert.ok(!body.user.password);
  });

  it('POST /api/users/register rejects missing password', async () => {
    const { status, body } = await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'nopw', role: 'data_owner' },
    });
    assert.equal(status, 400);
    assert.ok(body.error.message.includes('password'));
  });

  it('POST /api/users/register rejects empty password', async () => {
    const { status, body } = await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'emptypw', role: 'data_owner', password: '' },
    });
    assert.equal(status, 400);
    assert.ok(body.error.message.includes('password'));
  });

  it('POST /api/users/register rejects non-string password', async () => {
    const { status, body } = await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'num', role: 'data_owner', password: 12345 },
    });
    assert.equal(status, 400);
    assert.ok(body.error.message.includes('password'));
  });

  it('POST /api/users/register rejects duplicate display_name', async () => {
    await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'bob', role: 'data_requester', password: 'pwd' },
    });
    const { status, body } = await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'bob', role: 'data_owner', password: 'pwd2' },
    });
    assert.equal(status, 409);
    assert.ok(body.error);
  });

  it('POST /api/users/login returns token', async () => {
    await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'carol', role: 'verifier', password: 'testpass' },
    });
    const { status, body } = await fetchJSON(serverPort, 'POST', '/api/users/login', {
      body: { display_name: 'carol', password: 'testpass' },
    });
    assert.equal(status, 200);
    assert.ok(body.token);
    assert.equal(body.user.display_name, 'carol');
  });

  it('POST /api/users/login rejects wrong password', async () => {
    await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'dave', role: 'admin', password: 'correct' },
    });
    const { status } = await fetchJSON(serverPort, 'POST', '/api/users/login', {
      body: { display_name: 'dave', password: 'wrong' },
    });
    assert.equal(status, 401);
  });

  it('GET /api/users/me requires authentication', async () => {
    const { status } = await fetchJSON(serverPort, 'GET', '/api/users/me');
    assert.equal(status, 401);
  });

  it('GET /api/users/me returns user with valid token', async () => {
    await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'eve', role: 'data_owner', password: 'pass' },
    });
    const login = await fetchJSON(serverPort, 'POST', '/api/users/login', {
      body: { display_name: 'eve', password: 'pass' },
    });

    const { status, body } = await fetchJSON(serverPort, 'GET', '/api/users/me', {
      headers: { 'authorization': `Bearer ${login.body.token}` },
    });
    assert.equal(status, 200);
    assert.ok(body.user);
    assert.equal(body.user.display_name, 'eve');
  });

  it('GET /api/users/me rejects invalid token', async () => {
    const { status } = await fetchJSON(serverPort, 'GET', '/api/users/me', {
      headers: { 'authorization': 'Bearer invalid-token-value' },
    });
    assert.equal(status, 401);
  });

  it('protected endpoints reject missing auth', async () => {
    const endpoints = [
      ['POST', '/api/chain-accounts'],
      ['GET', '/api/chain-accounts'],
      ['POST', '/api/business-tasks'],
      ['GET', '/api/business-tasks'],
      ['POST', '/api/workflow-runs'],
      ['GET', '/api/workflow-runs'],
      ['POST', '/api/evidence'],
      ['GET', '/api/evidence'],
      ['POST', '/api/chain-events/import'],
      ['GET', '/api/chain-events'],
      ['POST', '/api/offchain-jobs'],
      ['GET', '/api/offchain-jobs'],
    ];

    for (const [method, url] of endpoints) {
      const { status } = await fetchJSON(serverPort, method, url);
      assert.equal(status, 401, `${method} ${url} should return 401`);
    }
  });

  it('POST /api/chain-accounts creates binding with verified_at null', async () => {
    await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'acct_owner', role: 'data_owner', password: 'pass' },
    });
    const login = await fetchJSON(serverPort, 'POST', '/api/users/login', {
      body: { display_name: 'acct_owner', password: 'pass' },
    });

    const { status, body } = await fetchJSON(serverPort, 'POST', '/api/chain-accounts', {
      headers: { 'authorization': `Bearer ${login.body.token}` },
      body: { chain_id: 'main', address: '5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY', scene_kind: 'DataTrade' },
    });
    assert.equal(status, 201);
    assert.equal(body.account.verified_at, null);
    assert.equal(body.verification_status, 'unverified');
    assert.ok(body.note);
  });

  it('GET /api/chain-accounts lists user accounts', async () => {
    await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'multi_acct', role: 'data_owner', password: 'pass' },
    });
    const login = await fetchJSON(serverPort, 'POST', '/api/users/login', {
      body: { display_name: 'multi_acct', password: 'pass' },
    });

    await fetchJSON(serverPort, 'POST', '/api/chain-accounts', {
      headers: { 'authorization': `Bearer ${login.body.token}` },
      body: { chain_id: 'child6', address: '5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty', scene_kind: 'DataTrade' },
    });

    const { status, body } = await fetchJSON(serverPort, 'GET', '/api/chain-accounts', {
      headers: { 'authorization': `Bearer ${login.body.token}` },
    });
    assert.equal(status, 200);
    assert.equal(body.accounts.length, 1);
    assert.equal(body.accounts[0].chain_id, 'child6');
  });

  it('creates and lists business tasks', async () => {
    await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'tasker', role: 'data_requester', password: 'pass' },
    });
    const login = await fetchJSON(serverPort, 'POST', '/api/users/login', {
      body: { display_name: 'tasker', password: 'pass' },
    });

    const create = await fetchJSON(serverPort, 'POST', '/api/business-tasks', {
      headers: { 'authorization': `Bearer ${login.body.token}` },
      body: { module: 'data_trade', initiator_account_id: 'acc1', status: 'pending' },
    });
    assert.equal(create.status, 201);
    assert.ok(create.body.task.task_id);

    const list = await fetchJSON(serverPort, 'GET', '/api/business-tasks', {
      headers: { 'authorization': `Bearer ${login.body.token}` },
    });
    assert.equal(list.status, 200);
    assert.equal(list.body.tasks.length, 1);
  });

  it('gets a business task by id', async () => {
    await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'tasker2', role: 'admin', password: 'pass' },
    });
    const login = await fetchJSON(serverPort, 'POST', '/api/users/login', {
      body: { display_name: 'tasker2', password: 'pass' },
    });

    const create = await fetchJSON(serverPort, 'POST', '/api/business-tasks', {
      headers: { 'authorization': `Bearer ${login.body.token}` },
      body: { module: 'data_collection', initiator_account_id: 'acc2', status: 'active' },
    });

    const { status, body } = await fetchJSON(serverPort, 'GET', `/api/business-tasks/${create.body.task.task_id}`, {
      headers: { 'authorization': `Bearer ${login.body.token}` },
    });
    assert.equal(status, 200);
    assert.equal(body.task.task_id, create.body.task.task_id);
  });

  it('creates and lists workflow runs', async () => {
    await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'runner', role: 'admin', password: 'pass' },
    });
    const login = await fetchJSON(serverPort, 'POST', '/api/users/login', {
      body: { display_name: 'runner', password: 'pass' },
    });

    const create = await fetchJSON(serverPort, 'POST', '/api/workflow-runs', {
      headers: { 'authorization': `Bearer ${login.body.token}` },
      body: { task_id: 'task1', session_id: 42, round_count: 3, status: 'running' },
    });
    assert.equal(create.status, 201);
    assert.equal(create.body.run.task_id, 'task1');

    const list = await fetchJSON(serverPort, 'GET', '/api/workflow-runs', {
      headers: { 'authorization': `Bearer ${login.body.token}` },
    });
    assert.equal(list.status, 200);
    assert.equal(list.body.runs.length, 1);
  });

  it('creates and lists evidence records', async () => {
    await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'recorder', role: 'verifier', password: 'pass' },
    });
    const login = await fetchJSON(serverPort, 'POST', '/api/users/login', {
      body: { display_name: 'recorder', password: 'pass' },
    });

    const create = await fetchJSON(serverPort, 'POST', '/api/evidence', {
      headers: { 'authorization': `Bearer ${login.body.token}` },
      body: { run_id: 'run1', category: 'dry_run', status: 'passed', scenario: 'happy', result: 'dry-run-accepted' },
    });
    assert.equal(create.status, 201);
    assert.equal(create.body.evidence.scenario, 'happy');

    const list = await fetchJSON(serverPort, 'GET', '/api/evidence', {
      headers: { 'authorization': `Bearer ${login.body.token}` },
    });
    assert.equal(list.status, 200);
    assert.equal(list.body.evidence.length, 1);
  });

  it('imports chain events from JSON array', async () => {
    await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'indexer', role: 'admin', password: 'pass' },
    });
    const login = await fetchJSON(serverPort, 'POST', '/api/users/login', {
      body: { display_name: 'indexer', password: 'pass' },
    });

    const { status, body } = await fetchJSON(serverPort, 'POST', '/api/chain-events/import', {
      headers: { 'authorization': `Bearer ${login.body.token}` },
      body: [{ event_id: 'ev1', chain_id: 'main', block_number: 1, block_hash: '0x01', pallet: 't', variant: 'T' }],
    });
    assert.equal(status, 201);
    assert.equal(body.count, 1);
  });

  it('imports chain events from JSONL file path via text/plain', async () => {
    const fileDir = path.join(TEST_DIR, `api-ce-file-${Date.now()}`);
    fs.mkdirSync(fileDir, { recursive: true });
    const filePath = path.join(fileDir, 'events.jsonl');
    fs.writeFileSync(filePath, '{"event_id":"ev2","chain_id":"child6","block_number":10,"block_hash":"0x0a","pallet":"test","variant":"Test"}\n', 'utf-8');

    await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'file_importer', role: 'admin', password: 'pass' },
    });
    const login = await fetchJSON(serverPort, 'POST', '/api/users/login', {
      body: { display_name: 'file_importer', password: 'pass' },
    });

    const { status, body } = await fetchJSON(serverPort, 'POST', '/api/chain-events/import', {
      headers: {
        'authorization': `Bearer ${login.body.token}`,
        'content-type': 'text/plain',
      },
      body: filePath,
    });
    assert.equal(status, 201);
    assert.equal(body.count, 1);

    fs.rmSync(fileDir, { recursive: true, force: true });
  });

  it('rejects chain event import from unsafe absolute path', async () => {
    await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'bad_importer', role: 'admin', password: 'pass' },
    });
    const login = await fetchJSON(serverPort, 'POST', '/api/users/login', {
      body: { display_name: 'bad_importer', password: 'pass' },
    });

    const { status, body } = await fetchJSON(serverPort, 'POST', '/api/chain-events/import', {
      headers: {
        'authorization': `Bearer ${login.body.token}`,
        'content-type': 'text/plain',
      },
      body: '/etc/passwd',
    });
    assert.equal(status, 400);
    assert.ok(body.error.message.includes('outside repository'));
  });

  it('GET /api/chain-events returns cached events with note', async () => {
    await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'event_lister', role: 'admin', password: 'pass' },
    });
    const login = await fetchJSON(serverPort, 'POST', '/api/users/login', {
      body: { display_name: 'event_lister', password: 'pass' },
    });

    const { status, body } = await fetchJSON(serverPort, 'GET', '/api/chain-events', {
      headers: { 'authorization': `Bearer ${login.body.token}` },
    });
    assert.equal(status, 200);
    assert.ok(Array.isArray(body.events));
    assert.ok(body.note);
  });

  it('chain event import does not require chain_role field', async () => {
    await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'no_role', role: 'admin', password: 'pass' },
    });
    const login = await fetchJSON(serverPort, 'POST', '/api/users/login', {
      body: { display_name: 'no_role', password: 'pass' },
    });

    await fetchJSON(serverPort, 'POST', '/api/chain-events/import', {
      headers: { 'authorization': `Bearer ${login.body.token}` },
      body: [{ event_id: 'ev_nr', chain_id: 'main', block_number: 1, block_hash: '0x01', pallet: 't', variant: 'T' }],
    });

    const { body } = await fetchJSON(serverPort, 'GET', '/api/chain-events', {
      headers: { 'authorization': `Bearer ${login.body.token}` },
    });
    assert.ok(!('chain_role' in body.events[0]));
  });

  it('creates and lists offchain jobs as metadata only', async () => {
    await fetchJSON(serverPort, 'POST', '/api/users/register', {
      body: { display_name: 'jobber', role: 'data_owner', password: 'pass' },
    });
    const login = await fetchJSON(serverPort, 'POST', '/api/users/login', {
      body: { display_name: 'jobber', password: 'pass' },
    });

    const create = await fetchJSON(serverPort, 'POST', '/api/offchain-jobs', {
      headers: { 'authorization': `Bearer ${login.body.token}` },
      body: { job_type: 'proof_generation', status: 'queued' },
    });
    assert.equal(create.status, 201);
    assert.equal(create.body.job.job_type, 'proof_generation');
    assert.ok(create.body.note);

    const list = await fetchJSON(serverPort, 'GET', '/api/offchain-jobs', {
      headers: { 'authorization': `Bearer ${login.body.token}` },
    });
    assert.equal(list.status, 200);
    assert.equal(list.body.jobs.length, 1);
  });

  it('returns 404 for unknown routes', async () => {
    const { status } = await fetchJSON(serverPort, 'GET', '/api/unknown');
    assert.equal(status, 404);
  });

  it('returns structured JSON errors for bad requests', async () => {
    const { status, body } = await fetchJSON(serverPort, 'POST', '/api/users/register');
    assert.equal(status, 400);
    assert.ok(body.error);
    assert.ok(body.error.message);
  });
});
