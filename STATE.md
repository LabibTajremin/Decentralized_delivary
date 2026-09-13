# BUILD STATE
last_updated: 2026-09-13T00:00:00Z
current_phase: P00
current_task: P00.T07
current_branch: phase/00-foundation
status: IN_PROGRESS
blocked: false
blocker_reason: ""

## Phase status
P00 IN_PROGRESS
P01..P20 TODO

## Current phase tasks
P00.T01 DONE  repo layout, go.work, backend + tests modules
P00.T02 DONE  build instruction split into docs/build/
P00.T03 DONE  architecture guard tests (verified against a deliberate violation)
P00.T04 DONE  thin-client lint + coverage gate (both verified against violations)
P00.T05 DONE  docker compose (PostGIS + Redis), CI with all eleven checks
P00.T06 DONE  STATE.md, README, ADR template + ADRs, OpenAPI skeleton, PR body
P00.T07 IN_PROGRESS  verification, PR, merge

## Coverage
backend total: 100.0% (no coverable statements outside the exclusion list —
cmd/api is the only excluded package and the modules are still skeletons)
last verified: 2026-09-13T00:00:00Z

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
