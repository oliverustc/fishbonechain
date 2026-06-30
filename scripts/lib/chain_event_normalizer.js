/**
 * FishboneChain event normalization helpers.
 *
 * Provides the baseline event mapping and normalized ChainEvent record
 * generation for the Stage 17 chain event indexer.
 */

import { readFileSync, createReadStream } from "node:fs";
import { createInterface } from "node:readline";
import { writeFileSync, mkdirSync, appendFileSync } from "node:fs";
import { join } from "node:path";

/**
 * Baseline data-trade event mapping.
 *
 * Only events in this set are tracked by default.  Other events may be
 * recorded when `--include-all` is set.
 */
export const BASELINE_EVENTS = Object.freeze(
  new Set([
    "dataRegistry.DataPublished",
    "dataRegistry.ImtRootUpdated",
    "dataRegistry.ListingStatusChanged",
    "tradeSession.SessionCreated",
    "tradeSession.SessionAccepted",
    "tradeSession.RoundOpened",
    "tradeSession.PaymentProofSubmitted",
    "tradeSession.DataProofSubmitted",
    "tradeSession.DataProofAttested",
    "tradeSession.ProofSignatureSubmitted",
    "tradeSession.DataDelivered",
    "tradeSession.PaymentPreimageSubmitted",
    "tradeSession.RoundCompleted",
    "tradeSession.SettlementClaimed",
    "tradeSession.SessionPunished",
    "tradeSession.LastPaymentClaimed",
    "mainEscrow.EscrowOpened",
    "mainEscrow.FundsLocked",
    "mainEscrow.DepositLocked",
    "mainEscrow.EscrowSettled",
    "mainEscrow.EscrowPunished",
  ])
);

/**
 * Convert any value to a JSON-compatible representation.
 *
 * Handles BN (from @polkadot util), u128, AccountId (SS58), hashes (hex),
 * bools, enums, vectors, and nulls.
 */
export function toJsonCompatible(value) {
  if (value === null || value === undefined) return null;
  if (typeof value === "boolean") return value;
  if (typeof value === "number") {
    if (Number.isFinite(value)) return value;
    return null;
  }
  if (typeof value === "string") return value;

  if (typeof value.toString === "function") {
    const str = value.toString();
    if (str === "[object Object]") {
      return Object.fromEntries(
        Object.entries(value).map(([k, v]) => [k, toJsonCompatible(v)])
      );
    }
    if (/^\d+$/.test(str)) return str;
    return str;
  }

  if (typeof value === "object") {
    if (Array.isArray(value)) return value.map(toJsonCompatible);
    return Object.fromEntries(
      Object.entries(value).map(([k, v]) => [k, toJsonCompatible(v)])
    );
  }

  return String(value);
}

/**
 * Build a stable field map from a decoded Polkadot.js event record.
 *
 * Prefer named fields when metadata exposes them; fall back to positional
 * indices otherwise.  Returns a plain object of `{ fieldName: value }`
 * with all values converted to JSON-compatible types.
 */
export function normalizeEventFields(eventRecord) {
  const fields = {};

  const metaFields = eventRecord.meta?.fields;
  const data = eventRecord.data;

  if (metaFields && metaFields.length > 0) {
    for (const [idx, metaField] of metaFields.entries()) {
      const name = metaField.name?.toString?.() || String(idx);
      fields[name] = toJsonCompatible(data[idx]);
    }
  } else if (typeof data === "object") {
    for (const [key, value] of Object.entries(data)) {
      fields[String(key)] = toJsonCompatible(value);
    }
  }

  return fields;
}

/**
 * Create a normalized ChainEvent record from a decoded Polkadot.js event.
 *
 * @param {object} decodedEvent - event from api.query.system.events.at()
 * @param {object} options
 * @param {"main"|"child"} options.chainRole
 * @param {string} options.chainId
 * @param {string} options.profile
 * @param {number} options.blockNumber
 * @param {string} options.blockHash
 * @param {number|null} options.extrinsicIndex
 * @param {number} options.eventIndex
 * @param {Date} [options.ingestedAt]
 * @returns {object} normalized ChainEvent record
 */
export function normalizeEvent(decodedEvent, options) {
  const { chainRole, chainId, profile, blockNumber, blockHash, extrinsicIndex, eventIndex } = options;
  const pallet = String(decodedEvent.event.section);
  const variant = String(decodedEvent.event.method);
  const qualifier = `${chainRole}:${blockNumber}.${eventIndex}`;
  const ingestedAt = options.ingestedAt || new Date();

  const event_id = `evt-${chainRole}-${blockNumber}-${eventIndex}-${pallet}-${variant}`;

  const record = {
    event_id,
    chain_id: chainId,
    chain_role: chainRole,
    profile,
    block_number: blockNumber,
    block_hash: blockHash,
    extrinsic_index: extrinsicIndex,
    event_index: eventIndex,
    pallet,
    variant,
    fields: normalizeEventFields(decodedEvent.event),
    cursor: qualifier,
    ingested_at: ingestedAt.toISOString(),
  };

  return record;
}

/**
 * Build a cursor string from chain_role and next block to scan.
 */
export function makeCursor(chainRole, nextBlock) {
  return `${chainRole}:${nextBlock}`;
}

/**
 * Parse a cursor string back into { chainRole, block }.
 */
export function parseCursor(cursor) {
  if (!cursor) return null;
  const idx = cursor.lastIndexOf(":");
  if (idx === -1) return null;
  return {
    chainRole: cursor.slice(0, idx),
    block: Number(cursor.slice(idx + 1)),
  };
}

/**
 * Read a JSONL file, returning parsed objects.
 */
export async function readJsonlFile(path) {
  const records = [];
  const rl = createInterface({ input: createReadStream(path, "utf8"), crlfDelay: Infinity });
  for await (const line of rl) {
    const trimmed = line.trim();
    if (trimmed) records.push(JSON.parse(trimmed));
  }
  return records;
}

/**
 * Write a JSONL file (overwrites).
 */
export function writeJsonlFile(path, records) {
  const dir = join(path, "..");
  mkdirSync(dir, { recursive: true });
  const lines = records.map((r) => JSON.stringify(r)).join("\n") + "\n";
  writeFileSync(path, lines, "utf8");
}

/**
 * Append a single JSONL record to a file.
 */
export function appendJsonlRecord(path, record) {
  mkdirSync(join(path, ".."), { recursive: true });
  appendFileSync(path, JSON.stringify(record) + "\n", "utf8");
}

/**
 * Check if an event is in the baseline mapping.
 */
export function isBaselineEvent(pallet, variant) {
  return BASELINE_EVENTS.has(`${pallet}.${variant}`);
}
