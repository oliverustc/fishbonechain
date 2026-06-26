# Stage 8 Agent Collaboration Retrospective

## Purpose

Record only lessons that affect Stage 9 planning.

## Lessons

1. Tests must fail for the intended reason.
   Stage 8 initially had an overflow test that could pass because of malformed JSON. Future plans should require checking the error path or error text when testing rejection cases.

2. Documentation tasks should name exact rows or stale phrases.
   Stage 8 updated the main range witness row but initially missed `Custom constraint kind: range`. Future plans should identify specific gap-matrix rows to update.

3. Keeping the proof path stable worked.
   `make-witness -> business-fixture -> verify` added flexibility without touching gnark circuits, artifact schema, JS digest logic, or runtime code. Stage 9 should preserve that shape and compose it into the chain E2E script.

## Applied To Stage 9

- Extend the existing E2E script with a dynamic mode instead of rewriting the whole flow.
- Preserve the old `--business-witness` path.
- Add checks that dynamic requests change the on-chain request hash and generated artifact evidence.
- If live chain RPC is unavailable, do not fake success; record the E2E run as environment-not-run.
