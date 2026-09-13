# Decentralized Delivery Platform — Claude Code Build Instruction

**Version:** 2.0 · September 2026
**Owner:** Rootlogic Lab
**Backend:** Go (payment module isolated for later .NET migration)
**Frontend:** Flutter (cross-platform)
**Design source:** Figma `NlVjn8OuvmLjbm8z8TDVlR`
**Design documentation:** GoKlay Design Audit

This document is the single authoritative instruction for the AI agent building this system. Every rule here is binding. Where this document and any other source disagree, this document wins.

---

# PART 0 — AGENT OPERATING PROTOCOL

Read Part 0 at the start of every session. Never load the entire document in one session.

## 0.1 Session rules

**R1 — Load at most four files per session:** `00-rules.md` (this part), `STATE.md`, `05-architecture.md`, and the one phase file being executed.

**R2 — Define once, reference by ID.** Screens, components, modules, config keys and algorithms all have stable IDs. After definition, refer to them by ID only. Never re-describe something that already exists.

**R3 — Output goes to disk, never to chat.** Report only: file paths, IDs, test results, commit SHA. Never echo file contents back.

**R4 — `STATE.md` is the only memory.** A fresh session with zero conversation history must read `STATE.md` and know exactly what is done, what is in progress, and what is next.

**R5 — One phase per branch. One task at a time.** Never work two phases in parallel.

**R6 — Never mark a task done until its tests pass and coverage holds at 100%.**

## 0.2 STATE.md format

Maintained at repo root. Updated after every task, not every phase.

```markdown
# BUILD STATE
last_updated: 2026-09-13T14:22:00Z
current_phase: P04
current_task: P04.T03
current_branch: phase/04-merchant-catalogue
status: IN_PROGRESS
blocked: false
blocker_reason: ""

## Phase status
P00 DONE   commit a1b2c3d  PR #1  merged
P01 DONE   commit e4f5g6h  PR #2  merged
P02 DONE   commit i7j8k9l  PR #3  merged
P03 DONE   commit m1n2o3p  PR #4  merged
P04 IN_PROGRESS
P05..P19 TODO

## Current phase tasks
P04.T01 DONE  merchant domain entities
P04.T02 DONE  merchant repository interfaces
P04.T03 IN_PROGRESS  catalogue use cases
P04.T04 TODO  merchant HTTP handlers
P04.T05 TODO  100% coverage verification

## Coverage
backend total: 100.0%
last verified: 2026-09-13T14:10:00Z

## Notes for next session
<anything the next session must know>
```

## 0.3 Resume protocol

Every session begins with exactly this sequence:

```bash
cat STATE.md
git status
git branch --show-current
cd backend/tests && go test ./... -coverpkg=../... -cover
```

Then:
- `status: IN_PROGRESS` → continue from `current_task`
- `status: PHASE_COMPLETE` → open the next phase file, create its branch, begin
- `blocked: true` → report the blocker and stop. Do not attempt a workaround.

## 0.4 Token-reset restart

**Claude Code cannot start itself.** There is no built-in trigger that fires when a quota window resets. What this document provides instead is a *resumable* build: `STATE.md` plus the resume protocol means any new session picks up mid-task with no context loss.

To make restart automatic, the operator schedules it externally. Example for Linux/macOS, attempting a resume every hour:

```bash
# crontab -e
0 * * * * cd /path/to/repo && /usr/local/bin/claude -p "Read STATE.md and follow docs/build/00-rules.md section 0.3. Resume the build." >> build.log 2>&1
```

The agent must make this safe by ensuring every session is **idempotent**: re-running a completed task must detect completion from `STATE.md` and exit without duplicating work.

## 0.5 Stop condition

The agent stops when, and only when, all of the following are true:

1. Every phase P00–P19 is `DONE` in `STATE.md`.
2. Every PR is merged with all checks green.
3. Backend coverage is 100.0% against the declared exclusion list.
4. Flutter coverage is 100.0% against the declared exclusion list.
5. The full E2E suite passes.
6. `main` builds clean.

Then write a final summary to `BUILD_COMPLETE.md` and stop. Do not continue to optional work.

---

# PART 1 — PRODUCT SPECIFICATION

## 1.1 What this is

A decentralized delivery platform for Bangladesh covering **food, grocery and pharmacy**.

"Decentralized" here means **radius-scoped discovery**, not a distributed or blockchain system. Use the term **radius-scoped** in code and technical docs; keep "decentralized" only in business-facing material.

## 1.2 Roles

| ID | Role | Description |
|---|---|---|
| ROLE-USER | User | Normal people who order online |
| ROLE-PARTNER | Delivery Partner | The person who delivers the product |
| ROLE-MERCHANT | Merchant | Sells the product — restaurant, grocery shop, pharmacy |
| ROLE-ADMIN | Admin | Platform operations and configuration |

## 1.3 The decentralization model — the core of this product

These rules are the product. Everything else is standard delivery-app functionality.

**D1 — Nationwide registration, local visibility.**
- A user can order food, pharmacy and grocery **from anywhere in Bangladesh**.
- A delivery partner can provide delivery **anywhere in Bangladesh**.
- A merchant can register **from anywhere in Bangladesh**.
- But a user only sees restaurants, groceries and pharmacies **inside a fixed radius** of their delivery location.

**D2 — Radius expansion with a cost.**
- When there is no restaurant or order option nearby, the user is offered the option to **extend the radius**.
- When the radius is extended, **the delivery charge increases**.
- Expansion is stepwise, not unlimited.

**D3 — Division-level ceiling.**
- Radius expansion **stops at division level**. A user's search can never cross beyond their division boundary, regardless of how far they expand.
- Bangladesh has eight divisions. Division is the hard geographic boundary of the system.

**D4 — Delivery partner distance choice.**
- A delivery partner can choose between **long-distance delivery** and **short-distance delivery**.
- A delivery partner is shown **only the available delivery options inside their fixed radius**.

**D5 — Automatic control, minimal admin interaction.**
- The application controls this decentralization **automatically**, with minimum admin interaction.
- The admin **must be given every option** to interact with and control it, but the application tunes the variables itself by default.
- Every variable ships with a **default value**. The admin can change those variables **per area**.
- Example: search radius may differ area by area. Dhaka city needs a small radius; a rural upazila needs a large one.

> **Design consequence.** D5 means the config system is not a settings screen bolted on at the end. It is a first-class subsystem — see Phase 11 and Appendix B — with per-area overrides, defaults, and an auto-tuner that adjusts within admin-set bounds.

## 1.4 Audience constraint

Primary users are rural, many with low digital literacy on low-end Android devices.

- Minimum tap target 48dp.
- Icon **plus** label for every primary action, never icon alone.
- Bengali-first. Layouts must be tested at Bengali string lengths.
- Minimum steps to complete an order.
- Works on slow connections: skeletons, optimistic UI, explicit offline messaging.
- **Cash on delivery is the primary payment path**, not an edge case.

## 1.5 UI source of truth

All screens come from the Figma file and the design documentation. The agent does not invent screens. If a screen is needed and not in Figma, the agent records it in `docs/design-gaps.md` and continues against a documented placeholder — it does not silently design something new.

---

# PART 2 — ARCHITECTURE RULES

## 2.1 Repository layout

**One repository. Top-level folders separate frontend and backend.**

```
/
├── backend/                    Go — the API
│   ├── go.mod
│   ├── cmd/api/                main wiring only
│   ├── internal/
│   │   ├── modules/
│   │   │   ├── identity/
│   │   │   ├── geo/
│   │   │   ├── merchant/
│   │   │   ├── catalogue/
│   │   │   ├── discovery/
│   │   │   ├── cart/
│   │   │   ├── order/
│   │   │   ├── pricing/
│   │   │   ├── dispatch/
│   │   │   ├── payment/
│   │   │   ├── tracking/
│   │   │   ├── notification/
│   │   │   ├── review/
│   │   │   └── config/
│   │   ├── shared/             cross-cutting: errors, logging, ids, time
│   │   └── platform/           db, cache, queue, http server
│   └── migrations/
├── backend/tests/              SEPARATE GO MODULE — all tests live here
│   ├── go.mod
│   ├── unit/
│   ├── integration/
│   └── e2e/
├── frontend/                   Flutter
│   ├── customer/
│   ├── merchant/
│   └── partner/
├── docs/
│   ├── build/                  this document, split by part
│   ├── technical/              developer documentation
│   ├── user/                   user documentation
│   └── decisions/              ADRs
├── go.work
├── STATE.md
└── README.md
```

## 2.2 Clean architecture

Four layers per module. Dependencies point inward only.

```
domain/          entities, value objects, domain rules — imports nothing
application/     use cases, port interfaces, DTOs — imports domain only
infrastructure/  DB, HTTP clients, storage — implements application ports
transport/       HTTP handlers — imports application only
```

Compile-time enforcement: a CI lint step fails the build if `domain/` imports anything outside itself, or if `transport/` imports `infrastructure/`.

## 2.3 Repository pattern

Every data access goes through an interface declared in `application/ports/`. Implementations live in `infrastructure/persistence/`. No use case ever touches a database handle.

## 2.4 Adapter pattern for swappable infrastructure

The database engine must be swappable — MySQL to Postgres or back — without touching business logic.

- All SQL lives behind repository implementations.
- No engine-specific types leak past the repository boundary.
- A driver is selected by config, resolved through a factory.
- Engine-specific SQL lives in `infrastructure/persistence/<engine>/`.

The same adapter discipline applies to cache, queue, storage and SMS.

> **Exception worth recording as an ADR:** the geospatial layer. Radius search performance depends on PostGIS-specific indexing. Keep the port interface engine-neutral (`FindMerchantsWithinRadius`), but accept that the Postgres implementation will be materially faster and document that the MySQL path is a fallback, not a peer.

## 2.5 Modular monolith that becomes microservices

**The system ships as one deployable.** Each module is fully separated so any module can later become its own service.

**Every cross-module call goes through one common service class under the calling module.**

```
internal/modules/order/
├── domain/
├── application/
├── infrastructure/
├── transport/
└── external/
    ├── order_payment_service.go       ← ALL code that talks to payment
    ├── order_dispatch_service.go      ← ALL code that talks to dispatch
    └── order_notification_service.go
```

Rules:
- A module never imports another module's `domain/`, `application/` or `infrastructure/`.
- The only permitted cross-module import is a module's own `external/` package calling the target module's **public contract interface**.
- Each `external/` service is independently testable with a fake.
- Extraction to a real microservice must require changing **only that one file** — swapping an in-process call for HTTP or gRPC — with zero change to business logic.

## 2.6 Language rules

- **Backend: Go.**
- **Payment module: Go for now, migrating to .NET later.** Because of this, payment gets the strictest boundary in the system. It communicates **only** over a versioned contract (`PaymentContract`), uses no shared Go types with other modules, and its `external/` callers serialize to a transport-neutral DTO from day one. Building payment this way makes the later .NET move a deployment change rather than a rewrite.
- **Frontend: Flutter**, for cross-platform availability.
- **Frontend and backend are fully separated.** Backend is API-based only. No server-rendered views.

## 2.7 Auth

- JWT access tokens, short-lived, with refresh token rotation.
- Refresh tokens stored hashed, single-use, revocable.
- Phone + OTP as the primary factor.
- Role-based access control enforced in middleware and re-checked in the use case.
- Every endpoint declares its required role and scope explicitly. No implicit public endpoints.
- Rate limiting on OTP request, OTP verify and login.

## 2.8 Algorithms and data structures

Use a real algorithm where the problem calls for one. Document the choice and its complexity in a comment above the implementation and in the ADR.

| ID | Problem | Approach | Complexity |
|---|---|---|---|
| ALG-01 | Merchants within radius | PostGIS `ST_DWithin` with GiST spatial index | O(log n + k) |
| ALG-02 | Radius expansion | Stepwise expansion until merchant count ≥ threshold or division boundary reached | O(s·log n), s = steps |
| ALG-03 | Division boundary check | Point-in-polygon against division geometry, `ST_Contains` | O(log n) |
| ALG-04 | Delivery partner assignment | Min-heap scored on distance, current load, acceptance rate | O(n log n) |
| ALG-05 | Delivery fee | Piecewise-linear banding by distance, with expansion multiplier | O(1) |
| ALG-06 | ETA | Prep time + route time + queue depth, exponentially weighted moving average on historicals | O(1) |
| ALG-07 | Search ranking | Weighted score: relevance, distance, rating, availability | O(n log n) |
| ALG-08 | Partner job feed | Bounded priority queue per partner radius, refreshed on location change | O(log n) per insert |
| ALG-09 | Auto-tuning radius | Feedback loop on merchant density and order-failure rate, clamped to admin bounds | O(1) per area per cycle |

## 2.9 Thin client — all business logic lives in the backend

**Every business rule is implemented in Go. Flutter is a presentation layer only.** The app stays lightweight and fast because it renders what the server computed; it never computes anything itself.

### The client must NEVER

- Calculate prices, delivery fees, surcharges, taxes, discounts or totals
- Decide whether radius expansion is offered, or by how much
- Compute distances, ETAs or route times
- Decide which merchants, items or offers are visible
- Validate a business rule authoritatively — minimum order, COD limit, cart eligibility, operating hours
- Determine order or job state transitions
- Decide which actions a user is allowed to take
- Hold a copy of any configuration variable from Appendix B

### The decision test

Apply in order. The first rule that matches wins.

1. **Does it decide an outcome?** Money, eligibility, visibility, permission, state transition → **backend, always.** No exceptions, regardless of how simple the calculation looks.
2. **Is it computationally or data heavy?** Geospatial math, ranking, aggregation, large sorts, image processing → **backend.** Heavy work on a low-end Android device drains battery and blocks the UI thread.
3. **Would a round trip make it feel broken?** Typing, scrolling, tapping, animating → **frontend**, but never authoritative. The server re-validates.
4. **Everything else** → frontend, as presentation only.

### Keep in the client — these make the app feel fast

Removing these does not make the app lighter; it makes it feel broken.

- **Rendering and layout state** — scroll position, tab selection, animation, expansion state
- **Input feedback** — masks, character counts, inline format hints, "this field is required". A UX convenience only; the server re-validates everything
- **Local filtering and sorting of an already-fetched list** — if 20 restaurants are on screen, filtering them by a chip is instant locally and must not cost a request. Filters that change *which* results exist still go to the server
- **Cached reads** — last-seen home feed, cart contents, order status, addresses. The app opens showing cached content immediately, then reconciles
- **Optimistic UI** — quantity steppers, favouriting, cart add. Apply immediately, reconcile against the server response, roll back visibly on failure
- **Request coalescing and debounce** — search-as-you-type debounced locally rather than firing per keystroke
- **Device concerns** — permissions, GPS read, camera capture, local notifications, connectivity detection
- **Offline queue** — actions taken offline are queued locally and replayed on reconnect. Partner delivery status updates especially, since riders lose signal constantly

### Move to the backend — these make the app heavy

- Image resizing, cropping and compression. **The server returns pre-sized, WebP thumbnails in the exact dimensions each screen needs.** The app never ships an image-processing library
- Distance and geospatial calculation — PostGIS does it faster than a phone, and it's authoritative anyway
- Ranking and relevance scoring
- Any aggregation, report or chart data — the server returns plotted points, not raw rows for the client to crunch
- PDF and invoice generation
- Full-text search
- Any list long enough to need client-side pagination — paginate server-side instead

### App weight rules

- No client library included for work the server can do. Every added dependency is justified in the PR description.
- Images: server-generated variants, WebP, lazy-loaded, with explicit dimensions to prevent layout shift.
- No bundled asset packs beyond icons and fonts. Everything else is fetched and cached.
- One state management approach across all three apps, not several.
- Build with tree-shaking, split per ABI, and track APK size in CI — **fail the build if the release APK grows more than 10% in a single PR without justification.**
- Target: customer app under 20 MB installed.

### API design consequence

Because the client computes nothing, responses must be **display-ready and self-describing**:

- Money arrives as both a minor-unit integer and a pre-formatted display string. **The Flutter code performs no arithmetic on money, ever.**
- Every price breakdown line arrives already computed and labelled.
- Every response that gates a user action carries explicit capability flags — `can_checkout`, `can_expand_radius`, `can_cancel`, with a reason string when false. The client renders the flag; it never infers the condition.
- State machines expose the current state and the permitted transitions. The client never derives what comes next.

### Enforcement

- CI lint fails on arithmetic involving money or distance types in `frontend/`.
- Code review rejects any Flutter code containing a business threshold, rate or rule.
- If a screen needs a value the API does not return, the fix is a backend change, never a client calculation.

### Why this matters here specifically

Business rules change constantly in this product — radius defaults, fee bands, COD limits, all tunable per area by the admin. Any rule living in the app requires an app-store release and a user who actually updates. Rural users on low-end devices update rarely. **Keeping every rule server-side means a config change takes effect immediately for every user, with no release.**

### Performance budget

Thin does not mean slow. These are the targets the build is measured against:

| Metric | Target |
|---|---|
| Cold start to first meaningful paint | under 2s on a low-end Android device |
| Home feed visible from cache | immediate, before any network response |
| Screen-to-screen navigation | under 100ms, served from cache where possible |
| Requests per screen | 1, batched — never one per widget |
| Release APK size | under 20 MB |

Achieve these through API design, not by moving business rules into the app:

- Shape responses to the screen — one request per screen, not one per component
- Batch endpoints for screens needing several resources
- Cache with ETags and a documented staleness policy per resource
- Every list endpoint paginated and field-selectable
- Stale-while-revalidate: render cached content instantly, refresh behind it

---

# PART 3 — TESTING

## 3.1 Tests live in a separate project

**No test file inside the main backend project.** All tests live in `backend/tests/`, which is its own Go module and can be run alone.

```
go.work
├── backend/          module github.com/rootlogic-lab/delivery/backend
└── backend/tests/    module github.com/rootlogic-lab/delivery/backend/tests
```

`backend/tests/go.mod` requires the backend module via a `replace` directive pointing at `../`.

Run standalone:

```bash
cd backend/tests
go test ./... -coverpkg=github.com/rootlogic-lab/delivery/backend/... -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -1
```

## 3.2 The unexported-code problem — read before Phase 0

This requirement has a real technical consequence the agent must design around rather than discover halfway through.

Go's `-coverpkg` lets an external test module measure coverage of the backend. But an external module **can only call exported identifiers**. Unexported functions are unreachable, so they can never reach 100%.

**Resolution, in priority order:**

1. **Design so there is nothing unexported worth testing separately.** Under clean architecture, every meaningful behaviour sits behind an exported use case or an exported domain method. Unexported helpers should be trivial enough that testing the exported caller covers them fully. This is the intended path and it aligns with the architecture.
2. Where a genuinely complex algorithm must stay unexported, **promote it to its own exported package** under `internal/` — still not importable outside the module boundary in production, but reachable by the test module through the workspace.
3. **Never** add `export_test.go` inside the backend project. That violates the no-test-files rule.

If a piece of logic cannot be covered by rules 1 or 2, that is a design smell. Refactor it rather than lowering the coverage gate.

## 3.3 Coverage gate: 100%

CI fails below 100.0%. Exclusions must be **explicitly listed** in `backend/tests/coverage-exclusions.txt`, with a one-line justification each. Permitted exclusions only:

- Generated code (protobuf, sqlc, mocks)
- `cmd/api/main.go` wiring, covered instead by E2E
- Vendored code

Nothing else. A new exclusion requires an ADR.

## 3.4 Test types

| Type | Location | Scope |
|---|---|---|
| Unit | `tests/unit/` | Domain rules, use cases with faked ports, algorithms |
| Contract | `tests/unit/contracts/` | Every `external/` boundary service against a fake |
| Integration | `tests/integration/` | Repositories against a real database in Docker |
| E2E | `tests/e2e/` | Full API flows against a running stack |

**Every algorithm in the ALG table gets dedicated tests including edge cases**: empty result set, division boundary crossing, maximum radius reached, zero available partners, concurrent assignment of the same order.

## 3.5 Flutter testing

Mirrors the same standard. Unit tests for logic, widget tests for every screen, integration tests for every user flow. 100% coverage gate via `flutter test --coverage` with an equivalent exclusions file for generated code.

---

# PART 4 — GIT WORKFLOW

The agent uses the `git` and `gh` CLIs directly. Every phase follows this cycle without deviation.

## 4.1 Phase cycle

```bash
# 1. Start from updated main
git checkout main
git pull origin main

# 2. Branch for the phase
git checkout -b phase/04-merchant-catalogue

# 3. Work. Commit per task, not per phase.
git add <specific files>
git commit -m "feat(merchant): add catalogue use cases

- Implement CreateCategory, UpdateItem, SetAvailability
- Add MerchantCatalogueRepository port
- Unit tests, coverage 100%

Phase: P04 Task: P04.T03"

# 4. Before opening a PR, verify locally
cd backend/tests && go test ./... -coverpkg=../... -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -1     # must read 100.0%
cd ../.. && go vet ./... && golangci-lint run

# 5. Push
git push -u origin phase/04-merchant-catalogue

# 6. Open the PR
gh pr create \
  --title "Phase 04 — Merchant registration and catalogue" \
  --body-file .github/pr-body.md

# 7. Wait for checks. ALL must be green.
gh pr checks --watch

# 8. Merge only when green
gh pr merge --squash --delete-branch

# 9. Update STATE.md, commit it to main
git checkout main && git pull origin main
```

## 4.2 Rules

- **A PR is never merged with a failing or pending check.** If a check fails, fix it and push again. Never merge with `--admin`, never bypass.
- Commit messages follow Conventional Commits and always end with the `Phase:` and `Task:` trailers.
- Commit and push **after finishing every phase**, and after every task within a phase.
- One phase per PR. Never combine phases.
- If a phase is abandoned or reworked, record why in `docs/decisions/`.

## 4.3 Required CI checks

`.github/workflows/ci.yml` must run, and all must pass:

1. `go build ./...`
2. `go vet ./...`
3. `golangci-lint run`
4. Architecture lint — layer dependency rules from 2.2
5. `go test` from `backend/tests` with coverage
6. Coverage gate at 100.0%
7. Migration up and down
8. `flutter analyze`
9. Thin-client lint — no money or distance arithmetic, no business thresholds in `frontend/` (2.9)
9b. APK size budget — fail if release APK grows >10% in one PR, or exceeds 20 MB
10. `flutter test --coverage` with gate
11. E2E suite

---

# PART 5 — PHASES

Every phase must be **completed fully** before the next begins. No partial phases.

Each phase produces: working code, its tests at 100%, its technical documentation, its user documentation where user-facing, a green PR, and an updated `STATE.md`.

| ID | Phase | Key output |
|---|---|---|
| **P00** | Foundation & tooling | Repo layout, `go.work`, both modules, Docker compose, CI with all eleven checks, architecture lint, `STATE.md`, README skeleton, ADR template, OpenAPI skeleton |
| **P01** | Shared kernel | Error types, structured logging, ID generation, time abstraction, pagination, result types, config loader. 100% covered. |
| **P02** | Geo module | Division and district geometry loaded, point-in-polygon, distance calculation, spatial indexes, `ALG-01`, `ALG-03`. **This is the foundation of the whole product — nothing else can be built correctly before it.** |
| **P03** | Config module | Per-area variable store, defaults, admin overrides, resolution order (area → district → division → global), change audit. Implements `D5`. See Appendix B. |
| **P04** | Identity & auth | Phone+OTP, JWT, refresh rotation, RBAC middleware, rate limiting, sessions. All three roles. |
| **P05** | User profile & addresses | Profile, address book, map pin, default address, address-to-area resolution |
| **P06** | Merchant | Registration from anywhere in Bangladesh, business details, documents, approval workflow, hours, holiday mode, merchant types (restaurant / grocery / pharmacy) |
| **P07** | Catalogue | Categories, items, variants, add-ons, combos, stock, scheduled availability, bulk update, per-merchant-type schema differences |
| **P08** | Discovery | Radius search, `ALG-02` expansion, division ceiling, `ALG-07` ranking, filters, search. **Implements `D1`, `D2`, `D3`.** |
| **P09** | Cart | Cart rules, single-merchant enforcement, add-on validation, cart invalidation on address or radius change |
| **P10** | Pricing | `ALG-05` delivery fee banding, **expansion surcharge**, service fee, VAT, promo application, minimum order |
| **P11** | Order | Order lifecycle state machine, COD and prepaid paths, cancellation, idempotency |
| **P12** | Dispatch | Partner availability, `D4` long/short distance choice, radius-scoped job feed `ALG-08`, `ALG-04` assignment, reassignment, proof of delivery, COD reconciliation |
| **P13** | Payment | Isolated module per 2.6. Gateway adapter, COD ledger, webhooks, idempotency, refunds. Contract-only boundary ready for .NET migration. |
| **P14** | Tracking & notifications | Order status stream, partner location stream, push, SMS fallback |
| **P15** | Admin & auto-tuning | Full admin API, all config controls from `D5`, `ALG-09` auto-tuner, live order operations, approvals, reporting, audit log |
| **P16** | Reviews & support | Ratings for merchant, partner and item; reviews; tickets; refund workflow |
| **P17** | Flutter foundation | Design tokens from Figma, component library, routing, state management, API client generated from OpenAPI, response caching and offline reads, Bengali localization, 48dp and contrast enforcement. **Thin-client lint in place before any screen is built** (2.9) |
| **P18** | Flutter customer app | Every customer screen from the Figma registry, wired to the API. Presentation only — no business logic, no money arithmetic, capability flags drive every enabled state |
| **P19** | Flutter merchant & partner apps | Every merchant and partner screen, wired to the API, under the same thin-client rule |
| **P20** | Hardening & release | Full E2E regression, load test on discovery and dispatch, security review, complete documentation, production deploy runbook |

## 5.1 Phase file format

Each `docs/build/phases/Pxx.md` is short by design:

```markdown
# P08 — Discovery
Depends on: P02, P03, P07
Branch: phase/08-discovery
Implements: D1, D2, D3, ALG-02, ALG-07

## Tasks
P08.T01  Domain: SearchArea, Radius, ExpansionStep value objects
P08.T02  Port: MerchantDiscoveryRepository
P08.T03  Use case: FindMerchantsNearby — base radius from config
P08.T04  Use case: ExpandSearchRadius — ALG-02, division ceiling
P08.T05  Use case: RankResults — ALG-07
P08.T06  Infrastructure: PostGIS repository implementation
P08.T07  Transport: discovery endpoints + OpenAPI
P08.T08  Tests: unit, integration, edge cases
P08.T09  Docs: technical + user
P08.T10  Coverage verification, PR, merge

## Acceptance
- A user in a dense urban area sees merchants within the configured base radius
- A user with zero merchants in radius is offered expansion
- Expanding increases the quoted delivery fee
- Expansion stops at the division boundary and returns a clear terminal state
- Two users in different divisions never see each other's merchants
```

---

# PART 6 — DOCUMENTATION

Technical **and** user documentation must both be delivered. Technical decisions must be reflected in `README.md`.

## 6.1 Technical documentation
One file per source file in `docs/technical/`, mirroring the source tree:

```markdown
# order/application/PlaceOrderUseCase
Layer: Application · Module: Order · Phase: P11
**Responsibility** — one sentence
**Inputs / Outputs** — types
**Dependencies** — ports and external services, by name
**Rules enforced** — business rules, listed
**Algorithms used** — ALG IDs
**Failure modes** — what it returns and when
**Tests** — file path, cases covered
```

Written in the same task as the code. Never retrofitted.

## 6.2 User documentation
`docs/user/`, one file per screen, per role. Purpose, entry point, what the user sees, every action and its destination, all states, error handling.

## 6.3 README.md
Must contain: what the system is, quickstart in under ten minutes, repo layout, and a **Technical Decisions** section recording every significant choice with its reasoning — Go, Flutter, modular monolith, the payment isolation, PostGIS, the separate test module, the adapter pattern, and the 100% coverage policy with its exclusion list.

## 6.4 ADRs
`docs/decisions/NNNN-title.md`, one per significant decision. Context, options considered, decision, consequences.

---

# PART 7 — DEFINITION OF DONE

**A task is done** when code is written, its tests pass, coverage holds at 100%, its documentation exists, and it is committed with a proper message.

**A phase is done** when every task is done, the full suite passes, the architecture lint passes, its PR is green and merged, and `STATE.md` is updated.

**The build is done** per section 0.5.

---

# APPENDIX A — MODULE CONTRACT REGISTRY

Every module exposes exactly one public contract interface. This is the only surface other modules may call, through their own `external/` service.

| Module | Contract | Consumed by |
|---|---|---|
| geo | `GeoContract` | discovery, dispatch, pricing, merchant, user |
| config | `ConfigContract` | every module |
| identity | `IdentityContract` | every module |
| merchant | `MerchantContract` | discovery, catalogue, order |
| catalogue | `CatalogueContract` | discovery, cart, order |
| discovery | `DiscoveryContract` | cart |
| cart | `CartContract` | order |
| pricing | `PricingContract` | cart, order |
| order | `OrderContract` | dispatch, payment, tracking, review |
| dispatch | `DispatchContract` | order, tracking |
| payment | `PaymentContract` | order — **transport-neutral DTOs only** |
| tracking | `TrackingContract` | order, notification |
| notification | `NotificationContract` | order, dispatch, identity |
| review | `ReviewContract` | merchant, dispatch |

---

# APPENDIX B — CONFIGURATION VARIABLES

Implements `D5`. Every variable has a global default, is overridable per area by the admin, and where marked, is adjustable by the auto-tuner within admin-set bounds.

Resolution order: **area → district → division → global default**.

| Key | Default | Unit | Auto-tuned | Purpose |
|---|---|---|---|---|
| `discovery.base_radius` | 5 | km | yes | Initial search radius |
| `discovery.expansion_step` | 5 | km | yes | Increment per expansion |
| `discovery.max_expansions` | 4 | count | no | Hard cap on steps |
| `discovery.min_merchants` | 5 | count | no | Threshold triggering expansion offer |
| `discovery.auto_expand` | false | bool | no | Expand without asking when below threshold |
| `discovery.division_ceiling` | true | bool | **no — never disableable** | Enforces `D3` |
| `pricing.delivery_base` | 40 | BDT | yes | Base delivery fee |
| `pricing.delivery_per_km` | 10 | BDT | yes | Distance rate |
| `pricing.expansion_multiplier` | 1.5 | ratio | yes | Surcharge when radius extended, per `D2` |
| `pricing.free_delivery_threshold` | 500 | BDT | yes | Order value for free delivery |
| `dispatch.short_distance_max` | 5 | km | yes | Upper bound of "short distance" per `D4` |
| `dispatch.long_distance_max` | 25 | km | yes | Upper bound of "long distance" |
| `dispatch.partner_radius` | 7 | km | yes | Radius of a partner's job feed |
| `dispatch.assignment_timeout` | 30 | sec | no | Before reoffering |
| `dispatch.max_concurrent_jobs` | 3 | count | no | Batching limit |
| `order.cancellation_window` | 120 | sec | no | Free cancellation period |
| `order.cod_limit` | 5000 | BDT | yes | Maximum COD order value |

**Auto-tuner (`ALG-09`)** runs per area on a schedule. It adjusts only variables marked auto-tuned, only within admin-configured min/max bounds, and writes every change to the config audit log with its reasoning. The admin can pin any variable, disabling auto-tuning for it in that area.

`discovery.division_ceiling` can never be disabled by admin or tuner. It is the one hard invariant of the system.
