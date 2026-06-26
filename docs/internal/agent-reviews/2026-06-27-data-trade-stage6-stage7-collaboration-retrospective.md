# Stage 6-7 Agent Collaboration Retrospective

## Purpose

This note records only the collaboration lessons that should affect future plans. It is intentionally short; avoid turning every review detail into a permanent rule.

## What Worked

- Detailed stop conditions prevented scope creep into runtime, artifact schema, JS digest encoding, and frontend work.
- Plan review before implementation caught ambiguity early, especially around function ownership and backward compatibility.
- Code review findings were most useful when they were tied to precise file/line behavior and required tests.
- Follow-up review plus automatic merge reduced handoff friction once fixes were verified.

## Lessons For Future Plans

1. **Avoid "A or B" implementation choices when CodeWhale will execute.**
   If one path is safer, prescribe it. Stage 7 plan review had to clarify `PrepareProof` vs `PrepareStructuredProof`.

2. **Defaulting and compatibility rules must include conflict cases.**
   Stage 7 needed explicit alias rules for `depth/published_depth` and tests for invalid legacy values.

3. **Require exact tests for serialization and hash preimages.**
   Stage 6 needed byte-exact string encoding tests. Future stages that introduce schemas or hash inputs should specify concrete test vectors.

4. **Documentation cleanup must name stale wording patterns.**
   If a stage deprecates old candidate wording, list the exact strings to remove.

5. **Keep long-term route maps separate from stage execution plans.**
   The roadmap should define direction. Each stage plan must still be written from the current code state.

## Applied To Stage 8

Stage 8 should:

- Prescribe one concrete dataset/request schema instead of leaving schema shape open.
- Keep the circuit path range-only for this stage.
- Add deterministic test vectors for dataset/request to witness conversion.
- Explicitly preserve `business-fixture` backward compatibility.
- Avoid runtime, JS digest, artifact schema, and frontend changes.
