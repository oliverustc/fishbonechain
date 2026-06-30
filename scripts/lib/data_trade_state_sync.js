/**
 * Data trade state derivation from normalized chain events.
 *
 * Derives listing, session, and escrow state snapshots by replaying
 * normalized events in order.  State is a replay/cache of chain events,
 * not a substitute for chain finality.
 */

/**
 * Derive complete data-trade state from normalized events.
 *
 * @param {object[]} events - normalized ChainEvent records sorted by event order
 * @returns {{ listings: object, sessions: object, escrows: object }}
 */
export function deriveState(events) {
  const listings = {};
  const sessions = {};
  const escrows = {};

  for (const event of events) {
    const { pallet, variant, fields, event_id } = event;

    if (pallet === "dataRegistry") {
      deriveListingState(listings, variant, fields, event_id);
    } else if (pallet === "tradeSession") {
      deriveSessionState(sessions, variant, fields, event_id);
    } else if (pallet === "mainEscrow") {
      deriveEscrowState(escrows, variant, fields, event_id);
    }
  }

  return { listings, sessions, escrows };
}

function listingId(fields) {
  return fields.listing_id;
}

function sessionId(fields) {
  return fields.session_id;
}

function escrowId(fields) {
  return fields.escrow_id;
}

function deriveListingState(listings, variant, fields, event_id) {
  const id = listingId(fields);
  if (id == null) return;

  if (!listings[id]) {
    listings[id] = {
      listing_id: Number(id),
      owner: null,
      price_per_round: null,
      max_rounds: null,
      status: "unknown",
      imt_root: null,
      source_events: [],
      last_event_id: null,
    };
  }

  const rec = listings[id];
  rec.source_events.push(event_id);
  rec.last_event_id = event_id;

  if (variant === "DataPublished") {
    rec.owner = fields.owner;
    rec.price_per_round = fields.price_per_round;
    rec.max_rounds = Number(fields.max_rounds);
    rec.status = "active";
  } else if (variant === "ListingStatusChanged") {
    rec.status = fields.status;
  } else if (variant === "ImtRootUpdated") {
    rec.imt_root = fields.new_root;
  }
}

function deriveSessionState(sessions, variant, fields, event_id) {
  const id = sessionId(fields);
  if (id == null) return;

  if (!sessions[id]) {
    sessions[id] = {
      session_id: Number(id),
      requester: null,
      data_owner: null,
      listing_id: null,
      escrow_id: null,
      status: "created",
      rounds: {},
      source_events: [],
      last_event_id: null,
    };
  }

  const rec = sessions[id];
  rec.source_events.push(event_id);
  rec.last_event_id = event_id;

  if (variant === "SessionCreated") {
    rec.requester = fields.requester;
    rec.data_owner = fields.data_owner;
    rec.listing_id = fields.listing_id != null ? Number(fields.listing_id) : null;
    rec.escrow_id = fields.escrow_id != null ? Number(fields.escrow_id) : null;
  } else if (variant === "SessionAccepted") {
    rec.status = "active";
  } else if (variant === "SettlementClaimed") {
    rec.status = "settled";
  } else if (variant === "SessionPunished") {
    rec.status = "punished";
  } else if (variant === "LastPaymentClaimed") {
    rec.status = "last_payment_claimed";
  }

  const roundIdx = fields.round_index;
  if (roundIdx != null) {
    if (!rec.rounds[roundIdx]) {
      rec.rounds[roundIdx] = { round_index: Number(roundIdx), events: [], status: "in_progress" };
    }
    rec.rounds[roundIdx].events.push(variant);
    if (variant === "RoundCompleted") {
      rec.rounds[roundIdx].status = "completed";
    }
  }
}

function deriveEscrowState(escrows, variant, fields, event_id) {
  const id = escrowId(fields);
  if (id == null) return;

  if (!escrows[id]) {
    escrows[id] = {
      escrow_id: Number(id),
      requester: null,
      data_owner: null,
      status: "opened",
      funds_locked: null,
      deposit_locked: null,
      paid_rounds: null,
      refunded: null,
      slashed_deposit: null,
      source_events: [],
      last_event_id: null,
    };
  }

  const rec = escrows[id];
  rec.source_events.push(event_id);
  rec.last_event_id = event_id;

  if (variant === "EscrowOpened") {
    rec.requester = fields.requester;
    rec.data_owner = fields.data_owner;
  } else if (variant === "FundsLocked") {
    rec.funds_locked = fields.amount;
    rec.status = "funded";
  } else if (variant === "DepositLocked") {
    rec.deposit_locked = fields.amount;
    rec.status = "ready";
  } else if (variant === "EscrowSettled") {
    rec.paid_rounds = fields.paid_rounds != null ? Number(fields.paid_rounds) : null;
    rec.refunded = fields.refunded;
    rec.status = "settled";
  } else if (variant === "EscrowPunished") {
    rec.slashed_deposit = fields.slashed_deposit;
    rec.status = "punished";
  }
}

/**
 * Generate a summary object from derived state.
 */
export function generateStateSummary(state) {
  const listingIds = Object.keys(state.listings);
  const sessionIds = Object.keys(state.sessions);
  const escrowIds = Object.keys(state.escrows);

  const summary = {
    generated_at: new Date().toISOString(),
    listing_count: listingIds.length,
    session_count: sessionIds.length,
    escrow_count: escrowIds.length,
    listings: listingIds.map((id) => ({
      listing_id: Number(id),
      status: state.listings[id].status,
      owner: state.listings[id].owner,
      event_count: state.listings[id].source_events.length,
    })),
    sessions: sessionIds.map((id) => ({
      session_id: Number(id),
      status: state.sessions[id].status,
      listing_id: state.sessions[id].listing_id,
      escrow_id: state.sessions[id].escrow_id,
      round_count: Object.keys(state.sessions[id].rounds).length,
      event_count: state.sessions[id].source_events.length,
    })),
    escrows: escrowIds.map((id) => ({
      escrow_id: Number(id),
      status: state.escrows[id].status,
      requester: state.escrows[id].requester,
      data_owner: state.escrows[id].data_owner,
      funds_locked: state.escrows[id].funds_locked,
      deposit_locked: state.escrows[id].deposit_locked,
      event_count: state.escrows[id].source_events.length,
    })),
  };

  return summary;
}

/**
 * Generate a human-readable Markdown summary from the state summary.
 */
export function generateMarkdownSummary(summary) {
  const lines = [];
  lines.push("# Data Trade State Summary");
  lines.push("");
  lines.push(`Generated: ${summary.generated_at}`);
  lines.push("");
  lines.push(`| Entity | Count |`);
  lines.push(`|--------|-------|`);
  lines.push(`| Listings | ${summary.listing_count} |`);
  lines.push(`| Sessions | ${summary.session_count} |`);
  lines.push(`| Escrows | ${summary.escrow_count} |`);
  lines.push("");

  if (summary.listings.length > 0) {
    lines.push("## Listings");
    lines.push("");
    lines.push("| ID | Status | Owner | Events |");
    lines.push("|----|--------|-------|--------|");
    for (const l of summary.listings) {
      lines.push(`| ${l.listing_id} | ${l.status} | ${truncate(l.owner)} | ${l.event_count} |`);
    }
    lines.push("");
  }

  if (summary.sessions.length > 0) {
    lines.push("## Sessions");
    lines.push("");
    lines.push("| ID | Status | Listing | Escrow | Rounds | Events |");
    lines.push("|----|--------|---------|--------|--------|--------|");
    for (const s of summary.sessions) {
      lines.push(
        `| ${s.session_id} | ${s.status} | ${s.listing_id} | ${s.escrow_id} | ${s.round_count} | ${s.event_count} |`
      );
    }
    lines.push("");
  }

  if (summary.escrows.length > 0) {
    lines.push("## Escrows");
    lines.push("");
    lines.push("| ID | Status | Requester | DO | Funds | Deposit | Events |");
    lines.push("|----|--------|-----------|----|-------|---------|--------|");
    for (const e of summary.escrows) {
      lines.push(
        `| ${e.escrow_id} | ${e.status} | ${truncate(e.requester)} | ${truncate(e.data_owner)} | ${e.funds_locked} | ${e.deposit_locked} | ${e.event_count} |`
      );
    }
    lines.push("");
  }

  lines.push("> Indexed state is a replay/cache of chain events — not a substitute for chain finality.");
  lines.push("");

  return lines.join("\n");
}

function truncate(s) {
  if (!s) return "null";
  return s.length > 12 ? s.slice(0, 6) + "..." + s.slice(-6) : s;
}
