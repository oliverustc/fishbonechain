import { describe, it, beforeEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { JsonStore } from '../lib/json_store.js';
import { AuthService } from '../lib/auth.js';
import { hashPassword, verifyPassword, createSessionToken } from '../lib/auth.js';
import { validate } from '../lib/schema.js';
import { generateId, nowISO } from '../lib/ids.js';
import { importChainEventsFromArray, importChainEventsFromFile, resolveImportPath } from '../lib/importers.js';

const TEST_DIR = '.agents/fwf/runs/stage18/backend-test';

describe('JsonStore', () => {
  let store;

  beforeEach(() => {
    const dir = path.join(TEST_DIR, `store-${Date.now()}-${Math.random().toString(36).slice(2)}`);
    store = new JsonStore(dir);
    store.init();
  });

  it('creates and lists records', () => {
    const user = { user_id: generateId(), display_name: 'test', role: 'admin', password_hash: 'x', created_at: nowISO(), updated_at: nowISO() };
    store.create('users', user);
    const users = store.list('users');
    assert.equal(users.length, 1);
    assert.equal(users[0].display_name, 'test');
  });

  it('finds records by predicate', () => {
    store.create('users', { user_id: 'a', display_name: 'one', role: 'admin', password_hash: 'x', created_at: nowISO(), updated_at: nowISO() });
    store.create('users', { user_id: 'b', display_name: 'two', role: 'admin', password_hash: 'x', created_at: nowISO(), updated_at: nowISO() });
    const found = store.find('users', u => u.display_name === 'one');
    assert.equal(found.length, 1);
  });

  it('findOne returns first match', () => {
    store.create('users', { user_id: 'a', display_name: 'one', role: 'admin', password_hash: 'x', created_at: nowISO(), updated_at: nowISO() });
    const found = store.findOne('users', u => u.display_name === 'one');
    assert.ok(found);
    assert.equal(found.user_id, 'a');
  });

  it('findOne returns null for no match', () => {
    const found = store.findOne('users', u => u.display_name === 'nobody');
    assert.equal(found, null);
  });

  it('updates records', () => {
    store.create('users', { user_id: 'a', display_name: 'one', role: 'admin', password_hash: 'x', created_at: nowISO(), updated_at: nowISO() });
    store.update('users', u => u.user_id === 'a', u => { u.display_name = 'updated'; });
    const user = store.findOne('users', u => u.user_id === 'a');
    assert.equal(user.display_name, 'updated');
  });

  it('supports all 8 collections', () => {
    const collections = ['users', 'sessions', 'chain_accounts', 'business_tasks', 'workflow_runs', 'evidence', 'chain_events', 'offchain_jobs'];
    for (const coll of collections) {
      const data = store.list(coll);
      assert.ok(Array.isArray(data));
    }
  });

  it('reads deterministically for sequential writes', () => {
    for (let i = 0; i < 10; i++) {
      store.create('users', { user_id: `u${i}`, display_name: `name${i}`, role: 'admin', password_hash: 'x', created_at: nowISO(), updated_at: nowISO() });
    }
    const users = store.list('users');
    assert.equal(users.length, 10);
    for (let i = 0; i < 10; i++) {
      assert.equal(users[i].user_id, `u${i}`);
    }
  });
});

describe('Schema', () => {
  it('validates Stage 15 field-name alignment for users', () => {
    const result = validate({ user_id: '1', display_name: 'x', role: 'admin', password_hash: 'h', created_at: 't', updated_at: 't' }, 'users');
    assert.ok(result.valid);
  });

  it('validates Stage 15 field-name alignment for chain_accounts', () => {
    const result = validate({ account_id: '1', user_id: '1', chain_id: 'c', address: 'a', scene_kind: 'DataTrade', verified_at: null, created_at: 't' }, 'chain_accounts');
    assert.ok(result.valid);
  });

  it('validates Stage 15 field-name alignment for business_tasks', () => {
    const result = validate({ task_id: '1', module: 'data_trade', initiator_account_id: '1', counterparty_account_id: null, reference_ids: {}, status: 'pending', created_at: 't', updated_at: 't' }, 'business_tasks');
    assert.ok(result.valid);
  });

  it('validates Stage 15 field-name alignment for workflow_runs', () => {
    const result = validate({ run_id: '1', task_id: '1', session_id: null, escrow_id: null, round_count: 0, status: 'running', started_at: null, completed_at: null }, 'workflow_runs');
    assert.ok(result.valid);
  });

  it('validates Stage 15 field-name alignment for evidence', () => {
    const result = validate({ evidence_id: '1', run_id: '1', category: 'dry_run', status: 'passed', scenario: null, result: null, command: null, log_path: null, artifacts: [], chain_tx_hashes: [], chain_event_refs: [], constraints: [], settlement: null, scenario_outcome: null, error: null, created_at: 't' }, 'evidence');
    assert.ok(result.valid);
  });

  it('validates Stage 15 field-name alignment for chain_events', () => {
    const fields = { event_id: '1', chain_id: 'main', block_number: 1, block_hash: '0x', extrinsic_index: null, event_index: 0, pallet: 'test', variant: 'Test', fields: {}, cursor: null, ingested_at: 't' };
    const result = validate(fields, 'chain_events');
    assert.ok(result.valid);
  });

  it('does not require chain_role in chain_events', () => {
    const fields = { event_id: '1', chain_id: 'main', block_number: 1, block_hash: '0x', extrinsic_index: null, event_index: 0, pallet: 'test', variant: 'Test', fields: {}, cursor: null, ingested_at: 't' };
    assert.ok(!('chain_role' in fields));
    const result = validate(fields, 'chain_events');
    assert.ok(result.valid);
  });

  it('validates Stage 15 field-name alignment for offchain_jobs', () => {
    const result = validate({ job_id: '1', job_type: 'proof_generation', status: 'queued', input_refs: [], output_refs: [], worker_id: null, digest: null, error: null, evidence_id: null, created_at: 't', started_at: null, completed_at: null }, 'offchain_jobs');
    assert.ok(result.valid);
  });

  it('rejects records with missing fields', () => {
    const userResult = validate({ user_id: '1' }, 'users');
    assert.ok(!userResult.valid);
    assert.ok(userResult.errors.length > 0);

    const taskResult = validate({ task_id: '1' }, 'business_tasks');
    assert.ok(!taskResult.valid);
    assert.ok(taskResult.errors.some(e => e.includes('missing required field')));

    const eventResult = validate({ event_id: '1' }, 'chain_events');
    assert.ok(!eventResult.valid);
    assert.ok(eventResult.errors.some(e => e.includes('missing required field')));
  });

  it('rejects unknown collections', () => {
    const result = validate({}, 'nonexistent');
    assert.ok(!result.valid);
  });
});

describe('Auth', () => {
  it('hashes and verifies passwords', () => {
    const stored = hashPassword('secret123');
    assert.ok(stored.includes(':'));
    assert.ok(verifyPassword('secret123', stored));
    assert.ok(!verifyPassword('wrong', stored));
  });

  it('creates unique session tokens', () => {
    const t1 = createSessionToken();
    const t2 = createSessionToken();
    assert.equal(t1.length, 64);
    assert.notEqual(t1, t2);
  });

  it('registers a user without storing plaintext password', () => {
    const dir = path.join(TEST_DIR, `auth-${Date.now()}-${Math.random().toString(36).slice(2)}`);
    const store = new JsonStore(dir);
    store.init();
    const auth = new AuthService(store);

    const result = auth.register('alice', 'data_owner', 'secret123');
    assert.ok(result.user);
    assert.equal(result.user.display_name, 'alice');

    const rawUser = store.findOne('users', u => u.user_id === result.user.user_id);
    assert.ok(rawUser);
    assert.ok(!('password' in rawUser));
    assert.ok(rawUser.password_hash);
    assert.ok(!rawUser.password_hash.includes('secret123'));
    assert.ok(rawUser.password_hash.includes(':'));

    fs.rmSync(dir, { recursive: true, force: true });
  });

  it('rejects duplicate display names', () => {
    const dir = path.join(TEST_DIR, `auth-dup-${Date.now()}`);
    const store = new JsonStore(dir);
    store.init();
    const auth = new AuthService(store);

    auth.register('bob', 'data_requester', 'p1');
    const result = auth.register('bob', 'data_owner', 'p2');
    assert.ok(result.error);
    assert.equal(result.status, 409);

    fs.rmSync(dir, { recursive: true, force: true });
  });

  it('rejects invalid roles', () => {
    const dir = path.join(TEST_DIR, `auth-role-${Date.now()}`);
    const store = new JsonStore(dir);
    store.init();
    const auth = new AuthService(store);

    const result = auth.register('eve', 'superuser', 'p');
    assert.ok(result.error);
    assert.equal(result.status, 400);

    fs.rmSync(dir, { recursive: true, force: true });
  });

  it('logs in and returns a session token', () => {
    const dir = path.join(TEST_DIR, `auth-login-${Date.now()}`);
    const store = new JsonStore(dir);
    store.init();
    const auth = new AuthService(store);

    auth.register('carol', 'verifier', 'pass');
    const result = auth.login('carol', 'pass');
    assert.ok(result.token);
    assert.ok(result.user);
    assert.equal(result.user.display_name, 'carol');

    const sessions = store.list('sessions');
    assert.equal(sessions.length, 1);

    fs.rmSync(dir, { recursive: true, force: true });
  });

  it('rejects invalid credentials', () => {
    const dir = path.join(TEST_DIR, `auth-bad-${Date.now()}`);
    const store = new JsonStore(dir);
    store.init();
    const auth = new AuthService(store);

    auth.register('dave', 'admin', 'correct');
    const result = auth.login('dave', 'wrong');
    assert.ok(result.error);
    assert.equal(result.status, 401);

    fs.rmSync(dir, { recursive: true, force: true });
  });

  it('authenticates valid tokens', () => {
    const dir = path.join(TEST_DIR, `auth-token-${Date.now()}`);
    const store = new JsonStore(dir);
    store.init();
    const auth = new AuthService(store);

    auth.register('frank', 'data_owner', 'pwd');
    const login = auth.login('frank', 'pwd');

    const user = auth.authenticate(login.token);
    assert.ok(user);
    assert.equal(user.display_name, 'frank');
    assert.ok(!user.password_hash);

    fs.rmSync(dir, { recursive: true, force: true });
  });

  it('rejects invalid tokens', () => {
    const dir = path.join(TEST_DIR, `auth-bad-token-${Date.now()}`);
    const store = new JsonStore(dir);
    store.init();
    const auth = new AuthService(store);

    const user = auth.authenticate('invalid-token');
    assert.equal(user, null);

    fs.rmSync(dir, { recursive: true, force: true });
  });
});

describe('Importers', () => {
  let store;

  beforeEach(() => {
    const dir = path.join(TEST_DIR, `import-${Date.now()}-${Math.random().toString(36).slice(2)}`);
    store = new JsonStore(dir);
    store.init();
  });

  it('imports chain events from array without requiring chain_role', () => {
    const events = [{ event_id: 'e1', chain_id: 'main', block_number: 1, block_hash: '0x01', pallet: 'test', variant: 'Test', fields: {} }];
    const result = importChainEventsFromArray(store, events);
    assert.equal(result.count, 1);
    assert.equal(result.imported[0].event_id, 'e1');
    assert.ok(!('chain_role' in result.imported[0]));
  });

  it('imports chain events with all Stage 15 fields', () => {
    const events = [{
      event_id: 'e2', chain_id: 'child6', block_number: 42, block_hash: '0xabc',
      extrinsic_index: 0, event_index: 1, pallet: 'dataRegistry', variant: 'DataPublished',
      fields: { listing_id: 1 }, cursor: 'child:43', ingested_at: '2026-01-01T00:00:00.000Z'
    }];
    const result = importChainEventsFromArray(store, events);
    assert.equal(result.count, 1);
    const e = result.imported[0];
    assert.equal(e.event_id, 'e2');
    assert.equal(e.chain_id, 'child6');
    assert.equal(e.block_number, 42);
    assert.equal(e.block_hash, '0xabc');
    assert.equal(e.extrinsic_index, 0);
    assert.equal(e.event_index, 1);
    assert.equal(e.pallet, 'dataRegistry');
    assert.equal(e.variant, 'DataPublished');
    assert.equal(e.cursor, 'child:43');
    assert.equal(e.ingested_at, '2026-01-01T00:00:00.000Z');
  });

  it('imports events from a JSONL file', () => {
    const dir = path.join(TEST_DIR, `import-file-${Date.now()}`);
    fs.mkdirSync(dir, { recursive: true });
    const filePath = path.join(dir, 'events.jsonl');
    fs.writeFileSync(filePath, JSON.stringify({ event_id: 'e3', chain_id: 'main', block_number: 1, block_hash: '0x', pallet: 't', variant: 'T', fields: {} }) + '\n', 'utf-8');

    const result = importChainEventsFromFile(store, filePath);
    assert.equal(result.count, 1);
    assert.equal(result.imported[0].event_id, 'e3');

    fs.rmSync(dir, { recursive: true, force: true });
  });

  it('rejects unsafe absolute paths outside repository', () => {
    const result = resolveImportPath('/etc/passwd');
    assert.ok(result.error);
    assert.ok(result.error.includes('outside repository'));
  });

  it('rejects non-existent files inside repository', () => {
    const result = resolveImportPath(path.join(process.cwd(), 'nonexistent-file.jsonl'));
    assert.ok(result.error);
    assert.ok(result.error.includes('not found'));
  });

  it('accepts repo-relative paths', () => {
    const dir = path.join(TEST_DIR, `import-rel-${Date.now()}`);
    fs.mkdirSync(dir, { recursive: true });
    const filePath = path.join(dir, 'events.jsonl');
    fs.writeFileSync(filePath, '{"event_id":"e4","chain_id":"main","block_number":1,"block_hash":"0x","pallet":"t","variant":"T","fields":{}}\n', 'utf-8');

    const resolved = resolveImportPath(filePath);
    assert.ok(resolved.resolved);
    assert.ok(!resolved.error);

    fs.rmSync(dir, { recursive: true, force: true });
  });

  it('generates IDs for imported events that lack event_id', () => {
    const events = [{ chain_id: 'main', block_number: 1, block_hash: '0x', pallet: 't', variant: 'T' }];
    const result = importChainEventsFromArray(store, events);
    assert.equal(result.count, 1);
    assert.ok(result.imported[0].event_id);
    assert.ok(result.imported[0].event_id.length > 0);
  });
});

describe('ChainEvents do not require chain_role', () => {
  it('imported chain events omit chain_role', () => {
    const dir = path.join(TEST_DIR, `no-chainrole-${Date.now()}`);
    const store = new JsonStore(dir);
    store.init();

    const events = [{ event_id: 'e', chain_id: 'main', block_number: 1, block_hash: '0x', pallet: 't', variant: 'T' }];
    importChainEventsFromArray(store, events);

    const stored = store.list('chain_events');
    assert.equal(stored.length, 1);
    assert.ok(!('chain_role' in stored[0]));

    fs.rmSync(dir, { recursive: true, force: true });
  });
});
