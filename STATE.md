# BUILD STATE
last_updated: 2026-09-13T00:00:00Z
current_phase: P01
current_task: P02.T01
current_branch: phase/00-foundation
status: IN_PROGRESS
blocked: false
blocker_reason: ""

## Phase status
P00 DONE   PR #1 (operator merges all phases at the end)
P01 DONE   shared kernel, 100% covered
P02..P20 TODO

## Current phase tasks
P01.T01 DONE  errs — error vocabulary
P01.T02 DONE  clock — injectable time
P01.T03 DONE  id — time-ordered Crockford base32 ids
P01.T04 DONE  logging — JSON with central redaction
P01.T05 DONE  paging — cursor pagination
P01.T06 DONE  result — value-or-error for batch work
P01.T07 DONE  config — process config, multi-error reporting
P01.T08 DONE  tests at 100%, technical docs

## Coverage
backend total: 100.0% (real statements now — 7 shared packages)
last verified: 2026-09-13

## Notes for next session

- Run tests with `-count=1`. The architecture guard tests in
  `backend/tests/unit/architecture_test.go` read files outside their own
  package, so Go's test cache can return a stale PASS and hide a real
  violation. `scripts/coverage-gate.sh` already passes the flag.
- `flutter` and `gh` are not installed in this environment. The Flutter CI
  steps self-skip until a `pubspec.yaml` exists (P17); PRs are opened through
  the GitHub MCP tools instead of the `gh` CLI.
- The operator added a binding auth requirement mid-build: JWT access token,
  Redis-backed refresh token with rotation, and **silent auto-login when a
  valid refresh token exists**. This is captured in `docs/build/phases/P04.md`
  and `docs/decisions/0005-redis-session-store.md`, and Redis is already in
  `docker-compose.yml` and the CI service matrix. Nothing to do until P04.
- Next phase is **P01 — Shared kernel**. Branch `phase/01-shared-kernel`.
  P01 is where the coverage gate first has real code to measure, so expect the
  gate to do actual work from that point.
