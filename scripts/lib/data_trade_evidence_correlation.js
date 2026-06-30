/**
 * Data trade evidence-to-event correlation.
 *
 * Matches Stage 14/16 evidence files against normalized chain events
 * by listing_id, session_id, escrow_id, and event names.
 */

/**
 * Correlation result for a single evidence entry.
 *
 * @typedef {object} CorrelationEntry
 * @property {string} evidence_id
 * @property {string} applicability - "matched" | "not_applicable" | "partial"
 * @property {string|null} reason
 * @property {object[]} matched_events
 * @property {object} summary
 */

/**
 * Correlate evidence entries against normalized chain events.
 *
 * @param {object[]} events - normalized ChainEvent records
 * @param {object|object[]} evidenceInput - evidence summary or array of scenarios
 * @returns {{ correlations: CorrelationEntry[], summary: object }}
 */
export function correlate(events, evidenceInput) {
  const scenarios = Array.isArray(evidenceInput) ? evidenceInput : evidenceInput;
  const correlations = [];
  let matchedCount = 0;
  let notApplicableCount = 0;

  for (const scenario of scenarios) {
    const listingId = scenario.listing_id;
    const sessionId = scenario.session_id;
    const escrowId = scenario.escrow_id;
    const expectedEvents = scenario.events || [];

    const hasChainIds = listingId != null || sessionId != null || escrowId != null;

    if (!hasChainIds) {
      notApplicableCount++;
      correlations.push({
        evidence_id: scenario.id || "unknown",
        applicability: "not_applicable",
        reason: "no chain IDs present (dry-run or no-chain evidence)",
        matched_events: [],
        summary: {},
      });
      continue;
    }

    const matched = findMatchingEvents(events, listingId, sessionId, escrowId, expectedEvents);

    if (matched.length > 0) {
      matchedCount++;
      correlations.push({
        evidence_id: scenario.id || "unknown",
        applicability: "matched",
        reason: null,
        matched_events: matched,
        summary: {
          total_matched: matched.length,
          matched_event_ids: matched.map((e) => e.event_id),
          matched_variants: matched.map((e) => `${e.pallet}.${e.variant}`),
        },
      });
    } else {
      correlations.push({
        evidence_id: scenario.id || "unknown",
        applicability: "partial",
        reason: "chain IDs present but no matching events found",
        matched_events: [],
        summary: {},
      });
    }
  }

  return {
    correlations,
    summary: {
      generated_at: new Date().toISOString(),
      total_scenarios: scenarios.length,
      matched: matchedCount,
      not_applicable: notApplicableCount,
      partial: correlations.length - matchedCount - notApplicableCount,
    },
  };
}

function fieldValue(fields, snake, camel) {
  if (snake in fields) return fields[snake];
  if (camel in fields) return fields[camel];
  return undefined;
}

function findMatchingEvents(events, listingId, sessionId, escrowId, expectedEvents) {
  return events.filter((e) => {
    const fields = e.fields || {};

    if (listingId != null) {
      const evtListingId = fieldValue(fields, "listing_id", "listingId");
      if (evtListingId != null && Number(evtListingId) === Number(listingId)) return true;
    }

    if (sessionId != null) {
      const evtSessionId = fieldValue(fields, "session_id", "sessionId");
      if (evtSessionId != null && Number(evtSessionId) === Number(sessionId)) return true;
    }

    if (escrowId != null) {
      const evtEscrowId = fieldValue(fields, "escrow_id", "escrowId");
      if (evtEscrowId != null && Number(evtEscrowId) === Number(escrowId)) return true;
    }

    if (expectedEvents.length > 0) {
      return expectedEvents.includes(`${e.pallet}.${e.variant}`);
    }

    return false;
  });
}
