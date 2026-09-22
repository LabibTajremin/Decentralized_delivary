# Architecture rules

> Split from `CLAUDE_CODE_BUILD_INSTRUCTION.md` v2.0, which remains the single
> authority. These files are its parts, unchanged in meaning.

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
