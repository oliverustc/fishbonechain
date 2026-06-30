# Stage 18 Code Review: Web Backend Minimal Skeleton

Date: 2026-06-30
Reviewer: Codex
Plan: `docs/internal/agent-plans/2026-06-30-stage18-web-backend-skeleton.md`
Review type: code review

## Scope Reviewed

- Stage 18 implementation commit `2771161` (`feat(backend): add minimal platform backend skeleton`).
- Stage 18 plan, latest Execution Record, formal backend document, tests, and validation output.
- Backend server, route handlers, JSON store, auth, schema validation, importers, docs, and git hygiene.

## Inputs Read

1. `agent.md`
2. `docs/internal/agent-collaboration.md`
3. `docs/internal/agent-plans/2026-06-30-stage18-web-backend-skeleton.md`
4. `docs/implementation/platform-backend-skeleton.md`
5. `docs/README.md`
6. `.gitignore`
7. `scripts/platform-backend/server.js`
8. `scripts/platform-backend/lib/*.js`
9. `scripts/platform-backend/test/*.test.js`
10. `git status --short`
11. `git diff --stat main...HEAD`
12. Stage state in `.agents/fwf/state/stages.json`

## Findings

### 1. Medium: Registration accepts a missing password and creates a loginable account with the literal `"undefined"` password

Location: `scripts/platform-backend/lib/auth.js:25-46`, `scripts/platform-backend/lib/routes.js:26-31`

`POST /api/users/register` passes `body.password` directly into `AuthService.register()`, and `register()` never checks that it is a non-empty string. If the request omits `password`, `hashPassword(undefined)` hashes `salt + undefined`, stores the account, and the user can then log in by sending the string `"undefined"` as the password. This contradicts the Stage 18 goal of a basic login skeleton and is a real authentication correctness issue even for a development-grade backend.

Required fix:

- Reject missing, non-string, or empty passwords at registration with a structured `400` error.
- Add a focused API or auth test proving password omission is rejected and no account is created.

### 2. Medium: Chain-event file import path check can be bypassed through repo-local symlinks

Location: `scripts/platform-backend/lib/importers.js:10-18`

`resolveImportPath()` confines `path.resolve(inputPath)` to the repository prefix, but it does not resolve symlinks. A repo-local symlink can point outside the repository; passing that symlink path will satisfy the prefix check and then read the outside target. The plan specifically required file-path import to reject unsafe paths outside the repository, so the confinement check needs to validate the real path.

Required fix:

- Use `fs.realpathSync()` (or equivalent) after the path exists and require the real path to stay under the repository root.
- Add a regression test that creates a repo-local symlink to an outside file and verifies import is rejected. If symlink creation is unavailable on the platform, the test may skip with an explicit reason, but Linux should support it here.

## Required Changes

1. Validate registration password input before hashing or storing the user.
2. Harden chain-event file import path confinement against symlink escape.
3. Rerun the Stage 18 validation commands and update the plan Execution Record with the review-fix pass.

## Accepted Risks

- The backend remains development-grade and file-backed; single-process writes, no session expiry, SHA-256+salt password hashing, and no cryptographic chain-account ownership verification are documented limitations and acceptable for Stage 18.
- List endpoints for business tasks, workflow runs, evidence, chain events, and offchain jobs are global rather than owner-scoped. The plan only required a minimal skeleton and did not define authorization policy beyond token-gated access.
- Request body parsing has no size limit. This is a skeleton limitation, but it should be addressed before any production or externally reachable deployment.

## Verification Performed

```bash
git branch --show-current
# stage/stage18-web-backend-skeleton

git status --short
# clean before review record

node --check scripts/platform-backend/server.js
# passed

find scripts/platform-backend/lib -name '*.js' -print -exec node --check {} \;
# passed for ids.js, auth.js, schema.js, json_store.js, importers.js, http.js, routes.js

node --test scripts/platform-backend/test/*.test.js
# 57 tests, 57 pass, 0 fail

node scripts/platform-backend/server.js --help
# printed usage and exited 0

test -f docs/implementation/platform-backend-skeleton.md
# passed

grep -q platform-backend-skeleton docs/README.md
# passed

! rg -q 'monitor/' scripts/platform-backend/
# passed

git check-ignore var/platform-backend/
# var/platform-backend/
```

Additional inspection:

- Read backend route, auth, importer, schema, store, and test code.
- Verified `docs/implementation/platform-backend-skeleton.md` documents Stage 18 trust boundaries.
- Verified no package dependency or lockfile changes were introduced.

## Branch and Commit Assessment

- Branch: `stage/stage18-web-backend-skeleton`
- Implementation commit: `2771161`
- The implementation commit references the plan and validation commands.
- Working tree was clean before writing this review record.
- No generated `.agents/fwf/runs/` data is tracked.

## Decision

`approved-with-required-fixes`

Do not merge Stage 18 yet. The two required fixes are narrow and should be handled by opencode on the same stage branch, followed by another `fwf codereview`.
