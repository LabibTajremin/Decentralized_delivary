# BUILD STATE
last_updated: 2026-09-15T00:00:00Z
current_phase: P08
current_task: P08.T01
current_branch: claude/goklay-design-system-9z500x
status: IN_PROGRESS
blocked: false
blocker_reason: ""

## Phase status
P00 DONE       foundation, gates, CI
P01 DONE       shared kernel, 100% covered
P02 DONE       geo module — domain, PostGIS repository, HTTP transport, OpenAPI, docs
P03 DONE       config module — D5 registry, area resolution, audit log, admin API
P04 DONE       identity — phone+OTP, JWT, Redis refresh rotation, RBAC, auto-login
P05 DONE       user — profile, address book, map pin, default address, area resolution
P06 DONE       merchant — nationwide registration (D1), documents, approval workflow, hours, holiday mode
P07 DONE       catalogue — per-type schema differences, variants, add-ons, combos, stock, availability, bulk update
P08..P20 TODO

## Current phase tasks
P02.T01 DONE  geo domain — coordinate, polygon, division/district/area
P02.T02 DONE  PostGIS repository — ALG-01 radius search, ALG-03 containment
P02.T03 DONE  shared httpx transport layer — respond, request, middleware
P02.T04 DONE  geo HTTP transport, mounted in cmd/api behind the middleware chain
P02.T05 DONE  OpenAPI paths + schemas, with a drift check in CI
P02.T06 DONE  docs/technical/geo.md

## P03 tasks
P03.T01 DONE  domain — six value kinds, scopes, the Appendix B registry
P03.T02 DONE  resolution area -> district -> division -> global
P03.T03 DONE  use cases — set/clear with immutability, tuner limits, pins, audit
P03.T04 DONE  migration 0002, PostgreSQL repository with transactional audit
P03.T05 DONE  contract + service, consumed by pricing/discovery/dispatch/order
P03.T06 DONE  admin HTTP transport, OpenAPI, docs/technical/config.md
P03.T07 DONE  unit, integration and E2E tests at 100%

## P04 tasks
P04.T01 DONE  domain — Phone, OTP, Role, Session, RefreshToken, Claims
P04.T02 DONE  ports — OTPStore, SessionStore, TokenSigner, RateLimiter, SMSSender, UserDirectory
P04.T03 DONE  use cases — RequestOTP, VerifyOTP, RefreshSession, Logout, ListSessions
P04.T04 DONE  infrastructure — Redis stores with atomic Lua, hand-rolled HS256, log SMS sender
P04.T05 DONE  migration 0003, account directory with a race-free upsert
P04.T06 DONE  transport — /v1/auth endpoints, RBAC middleware, explicit route table
P04.T07 DONE  seven auth.* config keys added to Appendix B, none auto-tunable
P04.T08 DONE  config endpoints put behind the admin role
P04.T09 DONE  contract + service for consuming modules
P04.T10 DONE  unit, integration (real Redis) and E2E tests at 100%
P04.T11 DONE  docs/technical/identity.md, ADR 0005 addendum

## P05 tasks
P05.T01 DONE  domain — Profile, Address, Pin, Placement, ChooseDefault
P05.T02 DONE  ports + external/geo seam for area resolution
P05.T03 DONE  use cases — profile read/update, address CRUD, default handling
P05.T04 DONE  migration 0004, repository with a one-default partial unique index
P05.T05 DONE  UserContract (an addition to Appendix A) + service
P05.T06 DONE  transport — /v1/me and /v1/me/addresses, caller-scoped
P05.T07 DONE  unit, integration and E2E tests at 100%
P05.T08 DONE  docs/technical/user.md, Appendix A note

## P06 tasks
P06.T01 DONE  domain — Type, Status with the whole transition table, Merchant, Pin, Placement
P06.T02 DONE  domain — documents per shop type, weekly hours, holiday mode
P06.T03 DONE  ports + external/geo seam for placement and index publication
P06.T04 DONE  use cases — registration, documents, submission, withdrawal
P06.T05 DONE  use cases — moderation (approve/reject/suspend/reinstate) and the admin queue
P06.T06 DONE  use cases — opening hours and holiday mode, outside the review freeze
P06.T07 DONE  geo gains PlaceMerchant, RemoveMerchant and ResolveDivision (D1)
P06.T08 DONE  migration 0005 — merchants, documents, status events
P06.T09 DONE  MerchantContract + service, documents deliberately excluded
P06.T10 DONE  transport — /v1/merchants owner routes, /v1/admin/merchants queue
P06.T11 DONE  demo seed — 14 approved shops behind the points geo already places
P06.T12 DONE  unit, integration and E2E tests at 100%
P06.T13 DONE  OpenAPI paths + schemas, docs/technical/merchant.md, Appendix A note

## P07 tasks
P07.T01 DONE  shared/money — minor units in an int64, and the one display string (2.9)
P07.T02 DONE  shared/schedule — the weekly timetable, extracted from P06's opening hours
P07.T03 DONE  ADR 0007 + a tightened architecture guard for domain-visible value objects
P07.T04 DONE  domain — categories, items, per-type attributes and the Capabilities table
P07.T05 DONE  domain — variants, add-ons, combos, stock, scheduled availability
P07.T06 DONE  use cases — categories, items, options, combos, transactional bulk update
P07.T07 DONE  migration 0006 and a repository that reads a menu in a fixed number of queries
P07.T08 DONE  CatalogueContract + service, shelf counts deliberately excluded
P07.T09 DONE  transport — owner routes behind ownership checks, public menu reads
P07.T10 DONE  OpenAPI paths + schemas, with the public reads declared
P07.T11 DONE  demo seed — menus for all 14 shops, in each type's own shape
P07.T12 DONE  unit tests to 100% on domain, application, transport and both kernel packages
P07.T13 DONE  integration and E2E tests; coverage gate back at 100%
P07.T14 DONE  docs/technical/catalogue.md, Appendix A note

## Next phase
P08 — Discovery. The radius search a customer actually sees: ALG-01 nearest-first
within the division (D3), stepwise expansion when nothing is nearby (D2, ALG-02),
and the merchant list the app renders. Depends on P02, P03 and P07.

## Deployment plumbing (operator request, done)
- `internal/platform/migrate` — versioned migrator, `schema_migrations`,
  dirty-flag marking, every migration reversible. 100% covered.
- `backend/cmd/migrate` — `up | down [n] | status | seed`. Wiring only;
  excluded from coverage per ADR 0006.
- `backend/migrations/` — SQL embedded in the binary. Every statement is
  `IF NOT EXISTS`, so an empty database is built and a partly-migrated one is
  updated by the same file.
- `backend/migrations/seed/` — demo data, upserts, in a *separate* embedded
  filesystem from the schema. `migrate seed` refuses when `APP_ENV=production`.
- `internal/platform/assets` — generated demo imagery embedded and served at
  `/static/demo/`, and never in production.
- `.env.example` — every deployment variable with its meaning and its
  production constraint.

## Coverage
backend total: 100.0%
last verified: 2026-09-15 (local PostGIS 3.4)
exclusions: cmd/api (ADR none — foundational, covered by e2e), cmd/migrate (ADR 0006)

## Notes for next session

- Run tests with `-count=1`. The architecture guard tests in
  `backend/tests/unit/architecture_test.go` read files outside their own
  package, so Go's test cache can return a stale PASS and hide a real
  violation. `scripts/coverage-gate.sh` already passes the flag.
- The integration suite now migrates through the real migrator rather than
  applying SQL its own way, so every integration run is also a test of
  `migrate up` and `migrate down`. It tears the schema down unconditionally
  first, because a database built by hand has nothing recorded in
  `schema_migrations` for `down` to roll back.
- Demo imagery is generated by `scripts/generate-demo-assets.py` and the
  output is byte-identical on every run, so regenerating never produces a
  spurious diff. It needs `pillow`.
- `flutter` and `gh` are not installed in this environment. The Flutter CI
  steps self-skip until a `pubspec.yaml` exists (P17); PRs are opened through
  the GitHub MCP tools instead of the `gh` CLI.
- The operator merges everything once, at the end. Do not merge PRs, and do
  not stop to ask for opinions.
- Local dependencies: ./scripts/dev-postgres.sh and ./scripts/dev-redis.sh
  start both, idempotently. The E2E suite needs DATABASE_URL and REDIS_URL.
- Redis is shared across test runs and its TTLs outlive them, so tests that
  touch rate limits or lockouts must use a per-run phone number. The Postgres
  schema is rebuilt per E2E test; Redis is not.
- Auth requirement (binding): JWT access token, Redis-backed refresh token
  with rotation and reuse detection, and **silent auto-login when a valid
  refresh token exists**. Spec is written in `docs/build/phases/P04.md` and
  `docs/decisions/0005-redis-session-store.md`. Redis is already in
  `docker-compose.yml`, the CI service matrix and `.env.example`.
- A real D1 bug surfaced in P06 and is worth remembering: merchant registration
  first used `GeoContract.ResolveArea`, which requires a *mapped area*. A shop in
  an upazila we have not drawn an area for sits inside a division and outside
  every area, and was refused — making whether a merchant may join depend on how
  finely we have mapped their district. Geo now offers `ResolveDivision`
  alongside it (division required, area best-effort). Addresses keep the strict
  call; registration uses the lenient one. Only the E2E test against real seeded
  geometry caught this — the unit fakes resolved everything.
- The demo merchant ids in `seed/0005_merchant.sql` are the same fourteen ids
  `seed/0001_geo.sql` places on the map, and the same ids the generated logos in
  `internal/platform/assets/demo/merchants/` are named after. Changing one list
  means changing all three; `TestTheDemoMerchantsAreRealShops` fails if they
  drift apart.
- `psql -c "a; b; c"` runs the statements in one transaction, so a failure in the
  third rolls back the first two. Resetting the local schema by hand needs
  separate `-c` invocations.
- P07 amended architecture rule 2.2, which is worth knowing before writing
  another module. A `domain/` may now import a shared package, but only one
  that is on a short named list (`money`, `schedule`) *and* verifiably imports
  nothing internal itself. ADR 0007 records why; the guard in
  `backend/tests/unit/architecture_test.go` checks both halves. A purity-only
  guard would have admitted `shared/errs`, which is how that gap was found —
  every version of the guard was probed with a deliberate violation.
- `scripts/migrate-check.sh` applies SQL directly rather than through the
  migrator, so it now clears `schema_migrations` after rolling back. Without
  that it left a developer's database with the tables gone and the ledger
  claiming they existed, and the next `migrate up` skipped everything then
  failed on the first migration referencing a missing table. If the local
  database ever gets into that state: `psql "$DATABASE_URL" -c "DELETE FROM
  schema_migrations"` then `migrate up`.
- The E2E `logTail.lastCode` helper now waits for a code it has not already
  handed out, tracking a count rather than the value. It previously returned as
  soon as any code was in the buffer, so a second sign-in against the same
  server could read the *previous* account's code and fail verification with a
  401 that looked like a rate limit. Tests doing two sign-ins got away with it;
  one doing six did not.
- The demo data is now three seeds that must agree on ids: `0001_geo.sql` places
  fourteen points, `0005_merchant.sql` gives them shops, `0006_catalogue.sql`
  gives those shops menus, and the generated logos under
  `internal/platform/assets/demo/merchants/` are named after the same ids.
  `TestTheDemoShopsHaveRealMenus` fails if they drift apart.
