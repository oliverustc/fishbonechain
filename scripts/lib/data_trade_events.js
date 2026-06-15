/**
 * FishboneChain event extraction helpers for E2E scripts.
 *
 * Extracts pallet event fields from signAndSend results.
 */
export function findEvent(result, section, method) {
  for (const { event } of result.events || []) {
    if (event.section === section && event.method === method) {
      return event;
    }
  }
  const seen = (result.events || [])
    .map(({ event }) => `${event.section}.${event.method}`)
    .join(", ");
  throw new Error(`event ${section}.${method} not found; seen=[${seen}]`);
}

export function eventDataNumber(event, field) {
  const value = event.data[field];
  if (value === undefined) {
    throw new Error(`event ${event.section}.${event.method} missing field ${field}`);
  }
  return value.toNumber ? value.toNumber() : Number(value);
}
