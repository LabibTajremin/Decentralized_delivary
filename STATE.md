# BUILD STATE
last_updated: 2026-09-21T16:00:00Z
current_phase: P17
current_task: P18.T01
current_branch: claude/goklay-design-system-9z500x
status: IN_PROGRESS
blocked: false
blocker_reason: ""
model_plan: P13-P16 Sonnet 5, P17-P20 Opus
stop_after: P20          # P17-P20 all run on Opus; no further model switch

## Resume here

A fresh session starts with no memory of how any of this was built. Read
`CLAUDE.md` (the working agreement) and then this file; together they are the
whole context.

Standing order from the user, still in force: **finish all twenty phases, do
not stop, do not merge anything, do not open a pull request, push only to
`claude/goklay-design-system-9z500x`.** Pausing at a usage limit is fine —
resume when it resets. "Continue" means: pick up `current_task` above and keep
going.

**No model switch is left.** P13–P16 ran on Sonnet 5; the user switched to Opus
for P17 and P17–P20 all run there. Keep going to P20.

Next action: **start P18 (Flutter customer app)** — read
`docs/build/phases/P18.md`, write its task list into a `## P18 tasks` section
below, and build it the way every other phase was built (`CLAUDE.md` → "How a
phase goes"). `docs/technical/frontend.md` → "What P18 and P19 inherit" is the
short version of what the foundation already gives you.

Before running anything:

```bash
./scripts/verify.sh      # every gate CI runs, services started for you
```

**The Flutter toolchain is not in the repo, and the container is ephemeral.**
A fresh session has Go, Postgres and Redis but no Flutter, and `verify.sh` will
fail its last three gates without one. Reinstall it the way P17 did — it takes
a few minutes and needs no configuration afterwards:

```bash
curl -sS -o /tmp/flutter.tar.xz \
  https://storage.googleapis.com/flutter_infra_release/releases/stable/linux/flutter_linux_3.47.5-stable.tar.xz
tar -xf /tmp/flutter.tar.xz -C /opt
ln -sf /opt/flutter/bin/flutter /usr/local/bin/flutter
ln -sf /opt/flutter/bin/dart    /usr/local/bin/dart
git config --global --add safe.directory /opt/flutter
flutter config --no-analytics
```

3.47.5 is the version CI is pinned to (`.github/workflows/ci.yml`) and the one
`frontend/goklay_core/pubspec.yaml` constrains against. Running a different one
is how a phase goes green here and red in CI.

That is also how a phase ends: it must exit 0 before the phase is committed as
done.

## What is left

| Phase | What it is | Acceptance, in short |
|---|---|---|
| P13 | Payment | PaymentContract only, COD ledger reconciles, idempotent webhooks |
| P14 | Tracking & notifications | status and location streams deliver; SMS falls back when push fails |
| P15 | Admin & auto-tuning | every Appendix B variable admin-controllable per area; ALG-09 within bounds and logged; **the division ceiling still cannot be disabled** |
| P16 | Reviews & support | ratings for merchant, partner and item; the refund workflow completes |
| P17 | Flutter foundation | tokens from the Figma file, thin-client lint wired *before* any screen, 48dp targets, Bengali string lengths |
| P18 | Flutter customer app | every customer screen in the Figma registry, no money arithmetic in the app, capability flags drive enabled states |
| P19 | Flutter merchant & partner apps | every merchant and partner screen, same thin-client rule |
| P20 | Hardening & release | full E2E regression, load test on discovery and dispatch, security review, production runbook |

The backend seams the later phases are meant to use already exist: payment has
`OrderContract.MarkPaid` / `MarkPaymentFailed` and the system-only
`pending_payment → placed` move; tracking has the order event history and the
dispatch job; admin has the config module's registry and audit log.

## Phase status
P00 DONE       foundation, gates, CI
P01 DONE       shared kernel, 100% covered
P02 DONE       geo module — domain, PostGIS repository, HTTP transport, OpenAPI, docs
P03 DONE       config module — D5 registry, area resolution, audit log, admin API
P04 DONE       identity — phone+OTP, JWT, Redis refresh rotation, RBAC, auto-login
P05 DONE       user — profile, address book, map pin, default address, area resolution
P06 DONE       merchant — nationwide registration (D1), documents, approval workflow, hours, holiday mode
P07 DONE       catalogue — per-type schema differences, variants, add-ons, combos, stock, availability, bulk update
P08 DONE       discovery — local visibility (D1), stepwise expansion with a cost (D2), the division ceiling (D3), ranking
P09 DONE       cart — single merchant, revalidation against live prices and hours, invalidation on address change (D3)
P10 DONE       pricing — ALG-05 banding exact at the edges, the D2 surcharge, free delivery; one implementation for card and receipt
P11 DONE       order — one transition table for four parties, idempotent placement, frozen prices, the free cancellation window
P12 DONE       dispatch — nationwide partners (D1), D4's distance choice in the query, ALG-04 rounds with a clock, ALG-08 feed, the sweep
P13 DONE       payment — PaymentContract only (2.6), idempotent webhooks, atomic COD reconciliation
P14 DONE       tracking & notification — SSE delivery stream, push tried on every device falling back to SMS
P15 DONE       admin & auto-tuning — ALG-09 radius tuning within bounds and pins, audit log endpoint, the actor-identity fix
P16 DONE       review & support — ratings for merchant/partner/item, eligibility built entirely from OrderContract, ticket resolution triggers PaymentContract.Refund
P17 DONE       Flutter foundation — pub workspace + goklay_core, Figma tokens, 48dp/contrast floor enforced by tests, Bengali-first l10n with the font the design lacks, API transport, thin-client lint proven
P18..P20 TODO

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
P15 — Admin & auto-tuning. Every Appendix B variable admin-controllable per
area; ALG-09 auto-tuner within admin-set bounds and logged; **the division
ceiling still cannot be disabled**. Depends on P03 (config).

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
last verified: 2026-09-21 (local PostGIS 3.4)
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

## P08 tasks
P08.T01 DONE  domain — Policy (the ladder), Expansion, ranking (ALG-07), relevance, Bengali distance strings
P08.T02 DONE  external/ seams to geo, merchant and config; no storage of its own
P08.T03 DONE  SearchUseCase — ALG-01 fetch, ALG-02 stepwise expansion, clamped levels
P08.T04 DONE  ReachUseCase + DiscoveryContract, the single-merchant answer cart will ask for
P08.T05 DONE  provisional delivery quote (ALG-05) behind ports.DeliveryQuoter, replaced by pricing in P10
P08.T06 DONE  public /v1/discovery transport, OpenAPI paths and schemas
P08.T07 DONE  unit tests at 100%, E2E over the real seeded geography
P08.T08 DONE  docs/technical/discovery.md

## P09 tasks
P09.T01 DONE  domain — Cart, Line, option snapshots, single-merchant rule, merge-on-identical-selection
P09.T02 DONE  domain — revalidation: CheckLine, Decide, the blocker order, Priced
P09.T03 DONE  external/ seams to catalogue, merchant and discovery
P09.T04 DONE  CartUseCase — add, replace, quantity, remove, address, clear; ids in and prices out
P09.T05 DONE  migration 0007, repository with one-cart-per-user and whole-cart writes
P09.T06 DONE  CartContract + service, the revalidated cart order will freeze
P09.T07 DONE  /v1/cart transport, caller-scoped; OpenAPI paths and schemas
P09.T08 DONE  API-wide snake_case fix: P08 and P09 had shipped camelCase keys
P09.T09 DONE  unit, integration and E2E tests at 100%
P09.T10 DONE  docs/technical/cart.md

## P10 tasks
P10.T01 DONE  domain — Tariff, ALG-05 banding, the D2 multiplier, free delivery only at the base radius
P10.T02 DONE  domain — Quote as a receipt: ordered rows, a derived surcharge, away-from-free
P10.T03 DONE  PricingContract with a resolved Tariff snapshot, so a page of cards costs one config read
P10.T04 DONE  application service — six settings in, Bengali-first receipt rows and notices out
P10.T05 DONE  discovery switched from its provisional quoter (infrastructure/fees deleted) to external/pricing
P10.T06 DONE  DiscoveryContract.Reach now carries the resolved placement, so the cart prices without a second geo call
P10.T07 DONE  cart gained its receipt: external/pricing, a pricing block on the view, wire and contract
P10.T08 DONE  unit tests at 100%, E2E proving the shop-card fee and the receipt fee are the same number
P10.T09 DONE  docs/technical/pricing.md

## P11 tasks
P11.T01 DONE  domain — the lifecycle as one table, actors included; three distinguishable refusals
P11.T02 DONE  domain — Order, frozen lines/charges/destination/pickup, the event history
P11.T03 DONE  domain — the free cancellation window, running from placement rather than acceptance
P11.T04 DONE  external/ seams to cart, pricing, merchant, user, discovery and config
P11.T05 DONE  PlaceUseCase — ids in, prices read here; idempotency; one settings snapshot for both order rules
P11.T06 DONE  TransitionUseCase — one use case for all four parties; ReadUseCase with per-actor views
P11.T07 DONE  migration 0008, repository with a compare-and-set transition and a composite idempotency key
P11.T08 DONE  OrderContract — MarkPaid, MarkPaymentFailed, Advance restricted to partner/admin/system
P11.T09 DONE  transport — customer, shop and admin surfaces; MerchantContract gained OwnedBy
P11.T10 DONE  unit, integration and E2E tests at 100%; two real bugs found (see docs/technical/order.md)
P11.T11 DONE  docs/technical/order.md

## P12 tasks
P12.T01 DONE  domain — Partner (D1: no area, no approval), Availability, Preference (D4), Band
P12.T02 DONE  domain — Job: the lifecycle, the offer clock, passed_by, giving up before collection
P12.T03 DONE  domain — ALG-04 assignment on a min-heap; ALG-08 the bounded feed
P12.T04 DONE  external/ seams to order, geo and config
P12.T05 DONE  OfferUseCase — rounds not broadcasts, idempotent on the order, the two-pass sweep
P12.T06 DONE  PartnerUseCase — shift, location, feed, and the six moves a rider makes
P12.T07 DONE  migration 0009, PostGIS repository: D4 in the WHERE, compare-and-set on every job write
P12.T08 DONE  DispatchContract + the order module's external/dispatch: ready offers, cancellation withdraws
P12.T09 DONE  transport — twelve partner routes and the admin sweep; OpenAPI paths and schemas
P12.T10 DONE  unit, integration and E2E tests at 100%; three real holes found (see the notes below)
P12.T11 DONE  docs/technical/dispatch.md

## What P12's tests found

Three holes the fakes could not see, all found by driving the real thing:

- **A declined job was stranded.** `Sweep` expired dead offers and put them back
  on the board, and nothing ever took them off it again — a decline is the one
  way onto the board that no partner's own action reverses. The sweep now has a
  second pass that re-offers waiting jobs, which is why a job carries its own
  area codes: the settings that govern it are per-area (D2).
- **A rider who gave up before collecting failed the customer's order.** The
  food was still on the shop's counter. `Fail` before collection now returns the
  job to the board and leaves the order alone; only a rider carrying the food
  can fail a delivery.
- **`passed_by` could deadlock a one-rider town.** Skipping the rider who just
  declined is right while somebody else is standing by, and wrong when nobody
  is: the job would never be offered again. The round now falls back to the
  full pool when the filtered one is empty.

## P13 tasks
P13.T01 DONE  domain — Payment (gateway: pending/captured/failed/refunded), Collection (COD: held/remitted)
P13.T02 DONE  ports — Repository, Gateway (the swappable adapter, 2.4)
P13.T03 DONE  external/ seam to order (read, MarkPaid, MarkPaymentFailed); dispatch gains PartnerOfUser for payment's seam
P13.T04 DONE  application — CheckoutUseCase, WebhookUseCase (idempotent), CollectionUseCase (record + reconcile), RefundUseCase
P13.T05 DONE  migration 0010, Postgres repository, manual gateway adapter (refuses in production, like identity's LogSender)
P13.T06 DONE  PaymentContract + service; order gains external/payment and a delivery hook for COD collection
P13.T07 DONE  transport — checkout/read, the open signature-verified webhook, partner COD ledger, admin COD + refund; OpenAPI
P13.T08 DONE  unit, integration and E2E tests at 100%; real bugs found (see the notes below)
P13.T09 DONE  docs/technical/payment.md

## What P13's tests found

- **A per-item COD remit loop was a correctness bug caught before it ever
  became a failing test.** The first design called `domain.Collection.Remit()`
  and `repo.Save()` once per collection id in a batch; a validation failure on
  item three would leave items one and two already written, with no way for
  an operator to tell which succeeded. Redesigned around a single atomic
  `repo.Remit(...)` transaction that either moves every id in the batch or
  none of them.
- **A refund could have moved money before validating the request.** An early
  draft called the gateway before checking the reason was non-empty.
  Reordered so the reason check, then the domain's own validation, both run
  before the gateway is ever called.
- **Postgres transaction poisoning (SQLSTATE 25P02) shaped a test's
  structure.** A single integration test that induced a real unique-constraint
  conflict and then tried to read again inside the same `pgx.Tx` failed for a
  reason unrelated to the code under test — Postgres poisons a transaction
  after any error inside it. Split into two separate tests, each with its own
  transaction, matching the precedent already set in dispatch's suite.
- **The structural-typing seam shortcut used elsewhere in this codebase does
  not apply uniformly within one module.** `payment/external/dispatch` and
  `order/external/payment` are bare interfaces the target's own concrete
  `Service` satisfies directly, with no adapter — their target method's
  signature already matched what the consumer needed. `payment/external/order`
  keeps a real adapter that copies fields rather than aliasing, because 2.6
  forbids sharing a type with any other module regardless of whether the
  signatures happen to line up. Both choices are now backed by architecture
  tests (`TestPaymentContractIsSelfContained`,
  `TestPaymentDomainBorrowsNoOtherModulesTypes`) so the distinction does not
  have to be rediscovered next time a seam is added.

## P14 tasks
P14.T01 DONE  domain — tracking.Snapshot (status + live partner location, change detection); notification.Notification, notification.DeviceToken
P14.T02 DONE  identity gains IdentityContract.PhoneFor (a targeted addition, same precedent as P13's PartnerOfUser); external/ seams: tracking → order + dispatch (both bare interfaces, no adapter — order.Order and dispatch.JobForOrder/PartnerOfUser already match structurally), notification → identity, order → notification
P14.T03 DONE  application — tracking.SnapshotUseCase (scoped to the owning customer or the assigned partner); notification.NotifyUseCase (push tried on every device, falling back to SMS only when none worked), RegisterDeviceUseCase, ReadUseCase
P14.T04 DONE  migration 0011 (notifications, device_tokens), Postgres repository; push/log and sms/log adapters, both refusing construction in production like identity's own LogSender; httpx.statusRecorder gains Flush() so an SSE stream survives the logging middleware
P14.T05 DONE  contract + service — TrackingContract, NotificationContract; order wired to notification the same way it is wired to dispatch and payment (a service-level UseNotification setter), telling the customer on pickup, delivery, cancellation, rejection and a failed delivery
P14.T06 DONE  transport — GET /v1/track/{orderId} (Server-Sent Events), POST /v1/me/device, GET /v1/me/notifications; OpenAPI
P14.T07 DONE  unit, integration and E2E tests at 100%; real bugs found (see the notes below), plus P13's own outstanding gaps closed
P14.T08 DONE  docs/technical/tracking.md, docs/technical/notification.md

## What P14's tests found

- **`httpx.Logging` silently broke every future streaming endpoint.**
  `statusRecorder` embeds `http.ResponseWriter` as an interface field, and Go's
  method promotion through an embedded interface promotes only that
  interface's own methods — never `Flush`, which isn't part of
  `http.ResponseWriter`. Before this phase nothing had ever needed to flush a
  partial response, so the bug was latent rather than caught: a stream's
  `w.(http.Flusher)` assertion would have failed the moment `Logging` sat in
  front of it, always, in every environment. Found by reading the middleware
  chain before writing the SSE handler, not by a failing test. Fixed with a
  `Flush` method on `statusRecorder` that forwards to the real writer, covered
  directly in `backend/tests/unit/httpx`.
- **"Stop at the first device that accepts a push" was both a product bug and
  the source of a flaky test.** A test asserting both of a user's two devices
  were tried passed or failed depending on Go's (deliberately randomized) map
  iteration order, because the use case stopped at the first success. The fix
  was not to make the test's iteration deterministic — that would have hidden
  a real design gap — but to try every registered device unconditionally: a
  person signed in on a phone and a tablet should hear on both, and the fix
  that makes the product behavior correct is the same one that makes the
  outcome order-independent.
- **P13's own coverage gaps had never actually been closed.** A background
  verification process inherited at the start of this phase reported 100%
  coverage and every gate green, and P13 was committed on the strength of that
  log. Running `./scripts/verify.sh` fresh at the start of P14 found the exact
  same payment repository (`Remit`, `Save`, `scanPayment`/`scanCollection`,
  `ForPartner`) and handler (`startCheckout`, `refund`, `adminPayment`,
  `adminLedger`, `reconcile`, `receiveWebhook`, `simulate`) gaps a prior
  session's own notes had already flagged as outstanding — closed now,
  alongside P14's own gaps, rather than trusting a stale log a second time.
  `backend/tests/unit/payment/repository_errors_test.go` is new; the handler
  gaps were closed by extending `handler_test.go`.

## P15 tasks
P15.T01 DONE  domain — ALG-09 as a pure function (`domain.Tune`): widens on thin merchant density or a high order-failure rate, narrows only when comfortably oversupplied and failures are fine, one 10% step (min one metre) clamped to the definition's own bounds
P15.T02 DONE  application — `AutoTuneUseCase` (scoped to `discovery.base_radius` and `dispatch.partner_radius` only — the two keys Appendix B's ALG-09 has a signal for), `ListChangesUseCase` (audit log, filterable by key/scope/limit); both write through the existing `SetOverrideUseCase` so bounds, pins and the audit log are enforced once
P15.T03 DONE  the actor-identity fix — `confighttp.NewHandler` gains a `PrincipalOf` parameter (mirroring every other module's transport handler) and both writes now read the actor from the authenticated caller instead of a client-supplied `actor_id` field, which is removed from the wire format entirely
P15.T04 DONE  transport — `GET /v1/admin/config/changes`, `POST /v1/admin/config/autotune`; OpenAPI paths + `AutoTuneRequest`/`TuneOutcome` schemas, `actor_id` dropped from `SetConfigOverride`/`ClearConfigOverride`
P15.T05 DONE  unit tests at 100% (domain `Tune`, `AutoTuneUseCase`, `ListChangesUseCase`, both new HTTP routes, the new 401-without-a-caller path); full `./scripts/verify.sh` green
P15.T06 DONE  docs/technical/config.md updated with ALG-09 and the actor-identity fix; STATE.md

## What P15's tests found

- **The audit log's actor had been client-supplied since P04 was built, and
  nobody had closed it out.** `setOverrideRequest`/`clearOverrideRequest`
  carried an `actor_id` string that the handler trusted directly, with a
  comment on the code already admitting it was a stand-in "until P04 supplies
  an authenticated identity." P04 shipped two phases ago; every other
  module's transport handler (payment, order, tracking, notification, cart,
  catalogue) had already been wired to read the caller from
  `identityhttp.PrincipalFrom`, but config's own wiring in `cmd/api/main.go`
  never was. Anyone with admin access could attribute a change to any other
  admin's id in a log whose entire purpose is answering "who did this and
  why" — found by reading the existing code's own stale comment, not by a
  failing test. Fixed by giving `confighttp.NewHandler` a `PrincipalOf`
  parameter identical in shape to what every other module already uses, and
  removing `actor_id` from the wire format so a forged actor is no longer
  representable at all, not merely discouraged.
- **Two unreachable defensive branches, found by coverage rather than by
  reasoning about the code first.** `AutoTuneUseCase.Execute`/`tuneOne`
  originally checked errors from `domain.Lookup`, `resolved.Value` and
  `resolved.Int` — all three provably impossible given that `TunableKeys` is
  a fixed list of always-registered keys and `domain.Resolve` guarantees
  every registered key is present in its `Resolved` snapshot
  (`TestResolveAlwaysProducesEveryKey` already established this). Coverage
  analysis (93.8% and 86.7% respectively) is what surfaced them; the fix was
  to delete the branches rather than contrive a test to reach them, with a
  comment at each deletion site explaining why the call is infallible for
  every input the method is actually invoked with.
- **`golangci-lint`'s `revive` rule catches shadowing a Go builtin.**
  `clamp(v, min, max int64)` shadowed the built-in `min` function within its
  own body — harmless here since the body never called the builtin, but the
  gate still failed it. Renamed the parameters to `lo`/`hi`.

## P16 tasks
P16.T01 DONE  domain — review.Subject (merchant/partner/item), review.Review, review.Rating (aggregate); ticket.Ticket, ticket.Resolution, Resolve()
P16.T02 DONE  two small, targeted order/contract additions — Line.ItemID, Event.ActorID — so review's eligibility checks need only OrderContract, per Appendix A's scoping (review consumes order, not dispatch or catalogue); external/order, external/payment (both bare interfaces, no adapter)
P16.T03 DONE  application — SubmitReviewUseCase (rater-is-customer, delivered, subject-belongs-to-this-order, once-per-subject); RatingsUseCase, ListReviewsUseCase; RaiseTicketUseCase, ResolveTicketUseCase (refunded → PaymentContract.Refund before the ticket is saved resolved, idempotent on the order id), MyTicketsUseCase, OpenTicketsUseCase
P16.T04 DONE  migration 0012 (reviews, support_tickets — UNIQUE(order_id, rater_id, subject, subject_id) backs the once-per-subject rule at the database too), PostgreSQL repository
P16.T05 DONE  contract + service — ReviewContract (MerchantRating, PartnerRating), consumed structurally wherever needed per Appendix A; not yet wired into merchant's or dispatch's own transport responses — a deliberate scope decision, see docs/technical/review.md
P16.T06 DONE  transport — POST/GET /v1/reviews, GET /v1/ratings, POST /v1/support/tickets, GET /v1/me/support/tickets, GET+POST /v1/admin/support/tickets(/resolve); OpenAPI
P16.T07 DONE  unit, integration and E2E tests at 100%; E2E walks a real order through pickup→delivery, rates all three subjects, and drives a support ticket through an admin's resolution
P16.T08 DONE  docs/technical/review.md

## What P16's tests found

- **Dispatch's own ordering rule bit the first E2E draft.** A rider who
  comes online *after* the shop marks an order ready never sees the offer —
  ALG-04's assignment round runs once, at `ready`, and only asks partners who
  are already on shift. The first version of `deliveredOrderWithParties`
  called `onShift` after `walkShopToReady`, and the rider's job list came
  back empty. Dispatch's own E2E suite (`TestARiderTakesAnOrderToTheDoor`)
  already documents this ordering in a comment; the fix was reading it and
  matching it, not adding a poll or a retry.
- **`ReviewContract`'s scoping (Appendix A: consumed by merchant and
  dispatch, itself consuming only `OrderContract`) meant review could not
  ask dispatch "who delivered this order" directly.** The order module's own
  `"delivered"` event already records the delivering partner's id as its
  `ActorID` — `DispatchContract.PartnerOfUser`'s doc comment says as much
  ("the same [partner] id recorded as the actor on an order's 'delivered'
  event") — so exposing `ActorID` on `order/contract.Event` let a partner
  review be verified from `OrderContract` alone, with no new cross-module
  dependency. The same reasoning added `ItemID` to `Line` for item reviews.
- **A COD order cannot be refunded through `PaymentContract`.** `Refund`
  looks up the order's most recent gateway attempt, and a cash order never
  has one — so resolving a cash order's ticket as "refunded" fails with
  `refund_failed` (503) rather than silently succeeding or panicking. Proven
  by an application-level test with a failing fake payment service; not
  something this phase builds a cash-specific path around, the same kind of
  honest scoping decision P15 made for ALG-09.

## P17 tasks
P17.T01 DONE  Flutter 3.47.5 toolchain; frontend/ as a Dart pub workspace; goklay_core package; strict analysis_options (public_member_api_docs, matching the backend's documented-or-fail standard); flutter-coverage-gate.sh taught that a workspace root is not an app
P17.T02 DONE  design tokens read from Figma NlVjn8OuvmLjbm8z8TDVlR — seven colours, one derived tint, the Manrope/Poppins type scale — each carrying the variable name or node id it came from; ThemeData built by naming every colour rather than seeding one
P17.T03 DONE  Noto Sans Bengali shipped as the fallback on every style, because neither Manrope nor Poppins contains a single Bengali glyph and Bengali is the default language
P17.T04 DONE  accessibility floor enforced by tests over the real theme — GoklayTapTarget pads any control to 48dp without changing how it paints; GoklayContrast asserts every pair this library renders clears WCAG AA
P17.T05 DONE  Bengali-first localisation: seven hand-written strings (the server composes everything else), Bengali first in supportedLocales, lang=en sent only for English; layout tested at 320dp in Bengali
P17.T06 DONE  API transport — Money with no arithmetic, Capability, the error envelope, ETag/stale-while-revalidate cache with an offline read fallback, and the FIFO OfflineQueue the partner app needs
P17.T07 DONE  thin-client lint proven by scripts/thin-client-lint-selftest.sh, which feeds it six violations and three legal patterns; wired into verify.sh and CI, and it runs with or without Dart present
P17.T08 DONE  113 Flutter tests at 100% coverage with an empty exclusions file; docs/technical/frontend.md; CI pinned to Flutter 3.47.5

## What P17's tests found

- **The backend was about to hand the app mojibake.** `package:http` decodes a
  response body using the charset in the Content-Type header and falls back to
  **latin1** when there is none, and `httpx.WriteJSON` was sending
  `application/json` with no charset. JSON is UTF-8 by definition (RFC 8259
  §8.1), so every Bengali sentence the server composes would have arrived
  corrupted — and since Bengali is the default language, that is every screen
  of the product. Found by a test whose only crime was putting a real Bengali
  message in a mocked error body. Fixed on both sides: the backend now labels
  the charset, and the transport decodes `bodyBytes` as UTF-8 itself rather
  than trusting the header, because the client must not depend on the server
  remembering. Both fixes have tests. This is the third phase running where
  the defect only existed at the seam between two layers.
- **The design's fonts cannot render the design's own product.** Manrope and
  Poppins contain zero Bengali glyphs — checked against the cmap of every
  shipped weight, not assumed — while Bengali is the default language. Without
  a fallback face the entire app renders as substituted faces or empty boxes
  for most of its users. Noto Sans Bengali now ships as the fallback on every
  style, and a test fails any style that loses it.
- **Three colour pairings in the Figma file do not clear WCAG AA**, including
  one the design uses for a promo code: brand green on its own 15% tint is
  3.73:1 at 16px SemiBold, which WCAG does not count as large text. Soft Red on
  white is 3.05:1 and Emerald on white is 2.54:1. The palette was left exactly
  as the design defines it — inventing colours here would be inventing design —
  and each limit is recorded as its own passing test, with the way out
  asserted alongside it (textPrimary on that tint clears AA comfortably).
  White on the brand green passes at 4.52:1, which is close enough that the
  brand green must not be lightened.
- **The design's buttons are under the touch-target floor.** 12pt of padding
  around a 14pt label is about 41dp against a 48dp requirement. Resolved by
  separating what is painted from what is hit rather than by redrawing the
  design.

## P17 pre-brief (executed — kept for the reasoning behind the above)

Written while the backend was fresh, so the next session did not have to
re-derive it.

- **Three apps, one foundation.** `frontend/customer`, `frontend/merchant` and
  `frontend/partner` exist and are empty. P17 builds what all three share;
  P18 is the customer app, P19 the other two.
- **Design source: Figma `NlVjn8OuvmLjbm8z8TDVlR`** (`docs/build/00-rules.md`).
  Tokens come from the file, not from taste. The Figma MCP tools are available
  for reading it.
- **`scripts/thin-client-lint.sh` currently says "no Dart sources yet, nothing
  to check".** It has to be wired and *proven* — by feeding it a deliberate
  violation — before any screen exists. P17's acceptance says before, and it
  means before: a lint added afterwards finds a codebase already full of the
  thing it forbids.
- **What the client must never do (ADR 0004, 2.9):** no money arithmetic, no
  distance or fee calculation, no deciding which actions are available, no
  composing user-facing text. The server already sends, for every screen: money
  as `{minor, display}`, every label and notice, `next_actions`, feed `reason` +
  `notice`, `cancel.{allowed, reason, text, seconds_left}`. If a screen seems to
  need a calculation, the endpoint is missing a field — add it to the backend.
- **Bengali is the default, `?lang=en` the exception.** Lay out for Bengali
  string lengths; they are longer than the English ones, and a row that fits
  "Delivered" does not fit "পৌঁছে দেওয়া হয়েছে".
- **`api/openapi.yaml` is the contract the client is generated from.** It is
  kept honest by `backend/tests/unit/openapi` — an undocumented route or a
  documented route nothing serves both fail CI.
- Gates that will apply once Dart exists and are already wired into
  `scripts/verify.sh`: `flutter analyze`, the thin-client lint, the Flutter
  coverage gate, the APK size budget.
