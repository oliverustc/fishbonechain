You are auditing a deprecated reference repository so FishboneChain can inherit only the useful ideas.

Ground rules:
- Treat references/SciDataHub as read-only. Do not edit files.
- Prefer direct file evidence with paths and line numbers where possible.
- Be skeptical: distinguish implemented behavior, demo-only stubs, copied Fabric samples, incomplete TODOs, and assumptions.
- Keep the report concise but complete enough for a human maintainer to verify without re-reading the whole repo.
- Focus on migration value for FishboneChain's Substrate data-trade direction, not on preserving SciDataHub as a product.

Required report format:
1. Scope inspected
2. Confirmed facts with evidence
3. Gaps, demo shortcuts, or correctness risks
4. Reusable ideas/assets
5. Migration notes for FishboneChain
6. Follow-up files/questions

Useful deterministic context files are in docs/internal/reference-audits/scidatahub/context/.

Task: Audit blockchain integration and trust boundaries.

Inspect blockchain/sci-data-trade, relevant asset-transfer chaincode, backend chaincode services/routes/config, and Fabric gateway samples. Determine exactly which actor identity submits transactions, whether end-user signatures are enforced, what state is actually on-chain, what remains off-chain, and which parts are demo shortcuts unsuitable for a real blockchain design.
