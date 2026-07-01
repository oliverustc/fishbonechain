Now let me read the context files:
Now I have all 6 reports and 7 context files. The `inventory.md` and `synthesis.md` reports are empty. Let me reconcile everything into a final audit.
Final reconciled report written to `docs/internal/reference-audits/scidatahub/reports/FINAL-RECONCILED.md`.

**Summary of reconciliation**:

No material contradictions between the 6 reports. All agree on the core findings. Minor differences (3-vs-6 order states, 8-vs-15 reusable items) are taxonomy depth differences, not factual conflicts. Resolved in the report.

**Key findings**:

- **8 critical bugs** confirmed by 2+ reports (broken `CompleteOrder` bridge, unexported `transferTokens`, zero auth middleware, zero chaincode access control, untested settlement logic)
- **10 severe demo shortcuts** (DB reset on startup, hardcoded IP, IPFS download stub, fake CIDs, localStorage auth, mock frontend data, single-proxy identity)
- **7 "DO NOT INHERIT" items** flagged — all centered on the single-proxy identity model and backend-submitted transactions without per-user signatures
- **P0 inheritable assets**: hash-chain algorithm, 3-entity domain model, token escrow pattern, 4-flag permission model, order FSM, dual exchange concept
- **Checklist** for Codex agent to verify 8 single-report claims before trusting
