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

Task: Produce an inheritance matrix.

Use the whole SciDataHub repo plus prior generated reports if present. Create a table with: candidate item, source files, current maturity, why it matters, inherit/rewrite/discard decision, FishboneChain target area, and migration risk. Pay special attention to data-market workflows, evidence/hash/IPFS concepts, frontend UX, API boundaries, and benchmark scaffolding.
