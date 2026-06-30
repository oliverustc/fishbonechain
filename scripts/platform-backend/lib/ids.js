import crypto from 'node:crypto';

export function generateId() {
  return crypto.randomUUID();
}

export function nowISO() {
  return new Date().toISOString();
}
