import fs from 'node:fs';
import path from 'node:path';
import { generateId, nowISO } from './ids.js';

const REPO_ROOT = path.resolve(
  path.dirname(new URL(import.meta.url).pathname),
  '../../..'
);

export function resolveImportPath(inputPath) {
  const resolved = path.resolve(inputPath);
  if (!resolved.startsWith(REPO_ROOT + path.sep) && resolved !== REPO_ROOT) {
    return { error: `path outside repository: ${inputPath}`, status: 400 };
  }
  if (!fs.existsSync(resolved)) {
    return { error: `file not found: ${inputPath}`, status: 400 };
  }
  return { resolved };
}

export function parseJsonl(content) {
  const lines = content.split('\n').filter(line => line.trim() !== '');
  const events = [];
  for (const line of lines) {
    try {
      events.push(JSON.parse(line));
    } catch {
      continue;
    }
  }
  return events;
}

export function importChainEvents(store, events) {
  const imported = [];
  for (const raw of events) {
    const event = {
      event_id: raw.event_id || generateId(),
      chain_id: raw.chain_id || '',
      block_number: raw.block_number || 0,
      block_hash: raw.block_hash || '',
      extrinsic_index: raw.extrinsic_index ?? null,
      event_index: raw.event_index ?? 0,
      pallet: raw.pallet || '',
      variant: raw.variant || '',
      fields: raw.fields || {},
      cursor: raw.cursor || null,
      ingested_at: raw.ingested_at || nowISO(),
    };
    store.create('chain_events', event);
    imported.push(event);
  }
  return imported;
}

export function importChainEventsFromFile(store, filePath) {
  const resolved = resolveImportPath(filePath);
  if (resolved.error) return resolved;
  const content = fs.readFileSync(resolved.resolved, 'utf-8');
  const events = parseJsonl(content);
  const imported = importChainEvents(store, events);
  return { imported, count: imported.length };
}

export function importChainEventsFromArray(store, events) {
  const imported = importChainEvents(store, events);
  return { imported, count: imported.length };
}
