# Stage 18 Code Review Follow-up: Web Backend Minimal Skeleton

Date: 2026-06-30
Reviewer: Codex
Plan: `docs/internal/agent-plans/2026-06-30-stage18-web-backend-skeleton.md`
Previous review: `docs/internal/agent-reviews/2026-06-30-stage18-web-backend-skeleton-code-review.md`
Review type: follow-up code review after required fixes

## Scope Reviewed

- Review-fix commit `b7378c7` (`fix(backend): validate password input and harden import path against symlinks`).
- Previous required findings:
  1. registration accepted missing password and created a loginable `"undefined"` password account;
  2. chain-event file import path confinement could be bypassed through repo-local symlinks.
- Stage 18 implementation and formal docs for merge readiness.

## Inputs Read

1. `agent.md`
2. `docs/internal/agent-collaboration.md`
3. `docs/internal/agent-plans/2026-06-30-stage18-web-backend-skeleton.md`
4. `docs/internal/agent-reviews/2026-06-30-stage18-web-backend-skeleton-code-review.md`
5. `docs/implementation/platform-backend-skeleton.md`
6. `docs/README.md`
7. `.gitignore`
8. `scripts/platform-backend/lib/auth.js`
9. `scripts/platform-backend/lib/importers.js`
10. `scripts/platform-backend/test/backend_api.test.js`
11. `scripts/platform-backend/test/backend_store.test.js`
12. `git diff 5c868e6..HEAD`
13. Stage validation command output from this review session

## Findings

No required findings.

The two required findings from the previous review are resolved:

- Password registration validation now rejects missing, empty, and non-string passwords before hashing or storing a user (`scripts/platform-backend/lib/auth.js:25-28`).
- Chain-event file import now validates the real path with `fs.realpathSync()` before accepting repo-local paths (`scripts/platform-backend/lib/importers.js:18-22`), closing the repo-local symlink escape.

## Required Changes

None.

## Accepted Risks

- The backend remains a development skeleton with SHA-256+salt password hashing, no session expiration, single-process JSON file storage, and no chain-account signature verification. These are documented in `docs/implementation/platform-backend-skeleton.md` and are within Stage 18 scope.
- File-path import intentionally accepts repo-local paths. The symlink escape is fixed, but this remains a development/admin-oriented import surface and should not be exposed to untrusted users in production.
- List endpoints are token-gated but not tenant-scoped for all resources. The Stage 18 plan required a minimal skeleton and did not define a full authorization model.

## Verification Performed

```bash
git branch --show-current
# stage/stage18-web-backend-skeleton

git status --short
# clean before writing this review record

node --check scripts/platform-backend/server.js
# passed

find scripts/platform-backend/lib -name '*.js' -print -exec node --check {} \;
# passed for ids.js, auth.js, schema.js, json_store.js, importers.js, http.js, routes.js

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

node --test scripts/platform-backend/test/*.test.js
# 63 tests, 63 pass, 0 fail
```

Additional inspection:

- Read the review-fix diff from `5c868e6..HEAD`.
- Confirmed the added API tests cover missing, empty, and non-string registration passwords.
- Confirmed the added importer test covers repo-local symlink escape to `/etc/passwd`.
- Confirmed `package.json` remains dependency-light and no package lock changes were introduced.
- Confirmed the formal backend document and `docs/README.md` index entry exist.

## Branch and Commit Assessment

- Branch: `stage/stage18-web-backend-skeleton`
- Implementation commit: `2771161`
- First code review commit: `5c868e6`
- Review-fix commit: `b7378c7`
- Review-fix commit references both the plan and the code review record and records validation.
- Working tree was clean before this review record was written.

## Decision

`approved`

Stage 18 is ready for the FWF merge gate.
