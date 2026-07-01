import crypto from 'node:crypto';
import fs from 'node:fs';

export function computeFileDigest(filePath) {
  const content = fs.readFileSync(filePath);
  const hash = crypto.createHash('sha256');
  hash.update(content);
  return hash.digest('hex');
}

export function computeStringDigest(str) {
  const hash = crypto.createHash('sha256');
  hash.update(str, 'utf-8');
  return hash.digest('hex');
}
