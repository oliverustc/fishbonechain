import test from "node:test";
import assert from "node:assert/strict";

import {
  collectSubstratePrometheus,
  parseSubstratePrometheusText,
} from "../src/collectors/substratePrometheus.js";

test("parses selected Substrate prometheus block height metrics", () => {
  const parsed = parseSubstratePrometheusText(`
# HELP substrate_block_height Block height info of the chain
# TYPE substrate_block_height gauge
substrate_block_height{status="best",chain="fishbone_main"} 120901
substrate_block_height{status="finalized",chain="fishbone_main"} 120899
substrate_block_height{status="sync_target",chain="fishbone_main"} 120901
`);

  assert.deepEqual(parsed, {
    bestBlock: 120901,
    finalizedBlock: 120899,
  });
});

test("fetches and parses substrate prometheus endpoint", async () => {
  const result = await collectSubstratePrometheus({
    endpoint: "http://node.example/metrics",
    fetchText: async () => `
substrate_block_height{status="best",chain="fishbone_main"} 10
substrate_block_height{status="finalized",chain="fishbone_main"} 8
`,
    now: () => "2026-06-12T00:00:00.000Z",
  });

  assert.equal(result.ok, true);
  assert.deepEqual(result.data, { bestBlock: 10, finalizedBlock: 8 });
  assert.equal(result.startedAt, "2026-06-12T00:00:00.000Z");
  assert.equal(result.finishedAt, "2026-06-12T00:00:00.000Z");
});

test("returns structured error when prometheus endpoint fails", async () => {
  const result = await collectSubstratePrometheus({
    endpoint: "http://node.example/metrics",
    fetchText: async () => {
      throw new Error("timeout");
    },
    now: () => "2026-06-12T00:00:00.000Z",
  });

  assert.equal(result.ok, false);
  assert.equal(result.data, null);
  assert.match(result.errors[0], /timeout/);
});
