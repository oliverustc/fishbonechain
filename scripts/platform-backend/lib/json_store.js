import fs from 'node:fs';
import path from 'node:path';

const COLLECTIONS = [
  'users',
  'sessions',
  'chain_accounts',
  'business_tasks',
  'workflow_runs',
  'evidence',
  'chain_events',
  'offchain_jobs',
];

export class JsonStore {
  constructor(dataDir) {
    this.dataDir = path.resolve(dataDir);
  }

  init() {
    fs.mkdirSync(this.dataDir, { recursive: true });
    for (const coll of COLLECTIONS) {
      const p = this._path(coll);
      if (!fs.existsSync(p)) {
        fs.writeFileSync(p, '[]', 'utf-8');
      }
    }
  }

  _path(collection) {
    return path.join(this.dataDir, `${collection}.json`);
  }

  _read(collection) {
    const p = this._path(collection);
    if (!fs.existsSync(p)) {
      fs.writeFileSync(p, '[]', 'utf-8');
    }
    const raw = fs.readFileSync(p, 'utf-8');
    return JSON.parse(raw);
  }

  _write(collection, records) {
    const tmp = `${this._path(collection)}.tmp`;
    fs.writeFileSync(tmp, JSON.stringify(records, null, 2), 'utf-8');
    fs.renameSync(tmp, this._path(collection));
  }

  create(collection, record) {
    const records = this._read(collection);
    records.push(record);
    this._write(collection, records);
    return record;
  }

  list(collection) {
    return this._read(collection);
  }

  find(collection, predicate) {
    const records = this._read(collection);
    return records.filter(predicate);
  }

  findOne(collection, predicate) {
    const records = this._read(collection);
    return records.find(predicate) || null;
  }

  update(collection, predicate, updateFn) {
    const records = this._read(collection);
    let updated = null;
    for (const r of records) {
      if (predicate(r)) {
        updateFn(r);
        updated = r;
        break;
      }
    }
    if (updated) {
      this._write(collection, records);
    }
    return updated;
  }
}
