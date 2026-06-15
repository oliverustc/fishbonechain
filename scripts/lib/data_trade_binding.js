/**
 * FishboneChain session-escrow binding validation helpers.
 *
 * Validates that child6 session parameters match main-chain escrow
 * and listing terms, preventing "session points to wrong escrow" bugs.
 */

function asString(value) {
  return value && value.toString ? value.toString() : String(value);
}

function asNumber(value) {
  return value && value.toNumber ? value.toNumber() : Number(value);
}

/**
 * Assert main-chain escrow matches expected trade terms.
 */
export async function assertEscrowMatchesTradeTerms(mainApi, escrowId, expected) {
  const maybeEscrow = await mainApi.query.mainEscrow.escrows(escrowId);
  if (!maybeEscrow.isSome) {
    throw new Error(`mainEscrow.escrows(${escrowId}) is None`);
  }
  const escrow = maybeEscrow.unwrap();
  const failures = [];

  if (asString(escrow.requester) !== expected.requester) failures.push("requester");
  if (asString(escrow.dataOwner) !== expected.dataOwner) failures.push("dataOwner");
  if (asNumber(escrow.maxRounds) !== expected.maxRounds) failures.push("maxRounds");
  if (asString(escrow.pricePerRound) !== String(expected.pricePerRound)) failures.push("pricePerRound");
  if (asString(escrow.deposit) !== String(expected.deposit)) failures.push("deposit");
  if (asString(escrow.hashChainAnchor) !== expected.hashChainAnchor) failures.push("hashChainAnchor");
  if (asString(escrow.status) !== "Ready") {
    failures.push(`status=${asString(escrow.status)}`);
  }

  if (failures.length > 0) {
    throw new Error(`escrow ${escrowId} does not match trade terms: ${failures.join(", ")}`);
  }
}

/**
 * Assert child6 session matches listing and escrow binding.
 */
export async function assertSessionMatchesListingAndEscrow(childApi, sessionId, expected) {
  const maybeSession = await childApi.query.tradeSession.sessions(sessionId);
  if (!maybeSession.isSome) {
    throw new Error(`tradeSession.sessions(${sessionId}) is None`);
  }
  const session = maybeSession.unwrap();
  const failures = [];

  if (asNumber(session.listingId) !== expected.listingId) failures.push("listingId");
  if (asNumber(session.escrowId) !== expected.escrowId) failures.push("escrowId");
  if (asString(session.requester) !== expected.requester) failures.push("requester");
  if (asString(session.dataOwner) !== expected.dataOwner) failures.push("dataOwner");
  if (asNumber(session.maxRounds) !== expected.maxRounds) failures.push("maxRounds");
  if (asString(session.pricePerRound) !== String(expected.pricePerRound)) failures.push("pricePerRound");
  if (asString(session.hashChainAnchor) !== expected.hashChainAnchor) failures.push("hashChainAnchor");
  if (asString(session.settlementMode) !== "MainEscrow") failures.push("settlementMode");

  if (failures.length > 0) {
    throw new Error(`session ${sessionId} binding mismatch: ${failures.join(", ")}`);
  }
}
