# Decentralized Delivery Platform

A radius-scoped delivery marketplace for Bangladesh covering **food, grocery and
pharmacy**.

"Decentralized" here means **radius-scoped discovery** — not a distributed or
blockchain system. Anyone can register from anywhere in the country, but a user
only sees merchants inside a radius of their delivery address. When nothing is
nearby they can widen that radius, which costs more to deliver, and the widening
stops hard at their division boundary.

| Role | What they do |
|---|---|
| User | Orders food, groceries and medicine |
| Delivery Partner | Delivers it, choosing long- or short-distance work |
| Merchant | Restaurant, grocery shop or pharmacy |
| Admin | Operations, and per-area tuning of every variable |

## Quickstart

Requires Go 1.25 and Docker. Building the apps additionally needs Flutter
3.47.5, which CI is pinned to.

```bash
git clone https://github.com/LabibTajremin/Decentralized_delivary.git
cd Decentralized_delivary

docker compose up -d postgres redis    # PostGIS + Redis
cd backend && go build ./... && go run ./cmd/api
curl localhost:8080/healthz            # {"status":"ok"}  — liveness
curl localhost:8080/readyz             # {"status":"ready"} — both stores reachable
```

One command runs every gate CI runs, in CI's order, and starts Postgres and
Redis for you:

```bash
./scripts/verify.sh
```

It keeps going after a failure so you see all of them, and prints a list at
the end. Nothing in this repository is considered finished until it exits 0.
The individual gates are also runnable on their own:

```bash
./scripts/coverage-gate.sh          # backend tests, gated at 100%
./scripts/thin-client-lint.sh       # no business rules in the Flutter apps
./scripts/flutter-coverage-gate.sh  # all four Dart packages, gated at 100%
./scripts/apk-size-check.sh         # three release APKs against a 20 MB ceiling
./scripts/vuln-scan.sh              # govulncheck; needs network, so not in verify.sh
```

Tests live in their own module, so they can also be run directly:

```bash
cd backend/tests
go test ./... -count=1
```

`-count=1` matters: the architecture guard tests read files outside their own
package, so Go's test cache can otherwise return a stale pass.

## Repository layout

```
backend/                Go API
  cmd/api/              main wiring only
  internal/modules/     14 modules, each with domain / application /
                        infrastructure / transport / external
  internal/shared/      errors, logging, ids, time
  internal/platform/    db, cache, queue, http
  migrations/
backend/tests/          separate Go module — every test lives here
frontend/               Flutter: customer, merchant, partner
docs/build/             the build instruction, split into loadable parts
docs/technical/         one file per source file
docs/user/              one file per screen, per role
docs/decisions/         ADRs
scripts/                CI checks that are not `go test`
STATE.md                build state — the only memory between sessions
```

## Build protocol

The build runs in phases P00–P20, one phase per branch, one PR per phase. A
session reads `STATE.md` and `docs/build/00-rules.md` and knows exactly where it
is. Nothing is marked done until its tests pass and coverage holds.

## Technical decisions

Each of these has an ADR in `docs/decisions/`.

**Go for the backend.** One deployable, strong concurrency story, and a fast
test loop. The payment module is written for a later move to .NET, so it talks
only through a versioned contract with transport-neutral DTOs and shares no Go
types with anything else — making that migration a deployment change rather than
a rewrite.

**Flutter for the frontend.** One codebase for three apps across both platforms,
on an audience that is overwhelmingly Android.

**Adapters at every edge.** Nothing outside the process is called directly.
The database, Redis, the SMS gateway and the payment gateway each sit behind a
port the application layer owns, with the concrete implementation in
`infrastructure/`. Two of those ports currently hold working stand-ins —
`sms.LogSender` and `gateway/manual.Gateway` — and **both refuse to be
constructed when `APP_ENV=production`**, so a production deployment with no
real provider does not start rather than accepting sign-ins it writes to the
log or marking orders paid for free. Writing the real provider means writing
one file behind an existing port; `docs/runbook.md` §0 treats both as release
blockers.

**Flutter with no state-management package** (ADR 0008). `ChangeNotifier`,
Navigator 1.0, an `InheritedWidget` for injection, hand-written `fromJson`.
Because the client decides nothing, client state is only ever "loading, data,
or failed" — and every code-generating package would be a new hole in the 100%
coverage gate. `frontend/coverage-exclusions.txt` is deliberately empty.

**One shared Dart package in a pub workspace** (ADR 0009). `goklay_core` holds
the tokens, the theme, the accessibility floor, the localisation, the
transport, the state primitives and the whole sign-in flow; the three apps are
mostly screens. Three APKs at ~17.2–17.5 MB against a 20 MB ceiling, gated
against 10% growth.

**Offline: a read-through cache everywhere, a write queue in one place**
(ADR 0010). Every GET is cached and served stale *with a banner saying so*.
Exactly one screen queues writes — the rider's job screen, where "collected"
and "delivered" must arrive in that order or not at all. Nowhere else, and
each screen's user documentation says why not.

**Bengali-first, with string tables written by hand** (ADR 0011). About thirty
client strings, because everything else is composed by the server. Bengali is
first in `supportedLocales`, so an unrecognised device locale lands on Bengali
rather than English; `?lang=en` is sent only for English. Both tables are
tested, including that the Bengali one is actually in Bengali script.

**The merchant and partner apps are built from the API, not from a design**
(ADR 0012). The Figma file contains no merchant or partner screens — 10,456
nodes searched. Rather than invent two apps' worth of visual design, their
layout comes from the endpoints behind them and their look entirely from
`goklay_core`. `docs/design-gaps.md` records this and every other place the
design and the backend do not line up.

**Authorisation by resource ownership outside `/v1/admin/`** (ADR 0013). The
admin surface is guarded by role, and that guard is swept rather than listed:
a test reads the OpenAPI document and requires every admin operation to refuse
all three non-admin roles. Everywhere else, a caller is stopped by not owning
the resource, and somebody else's resource answers 404 rather than 403.

**Modular monolith, split-ready** (ADR 0001). Fourteen modules with enforced
walls: a module may never import another module's internals, only its public
contract, and only from its own `external/` package. Extraction to a service
should mean rewriting one file. The rule is enforced by a test that parses real
imports and runs as its own CI check.

**Clean architecture, four layers.** `domain` imports nothing, `application`
imports `domain`, `infrastructure` implements ports, `transport` imports
`application`. Violations fail the build.

**PostGIS, honestly** (ADR 0003). The database is swappable behind repository
interfaces, but radius search depends on `ST_DWithin` with a GiST index. The
port stays engine-neutral; PostGIS is the supported path and MySQL is documented
as a fallback rather than an equal.

**Redis for sessions and refresh tokens** (ADR 0005). Access tokens are
short-lived JWTs verified by signature alone, so the hot path never touches
Redis. Refresh tokens are opaque, stored only as hashes with a TTL, rotated on
every use, and a replayed token revokes its whole session family. A user holding
a valid refresh token is signed back in silently, with no login screen.

**Tests in a separate module** (ADR 0002). Required by the build rules, and it
forces a real design constraint: an external module can only reach exported
identifiers, so anything worth testing must sit behind an exported use case or
domain method. Coverage below 100% is treated as a design smell rather than a
testing gap.

**Thin client** (ADR 0004). Flutter computes no money, no distance, no
eligibility and holds no config value. Every rule in this product is tunable per
area by an admin at runtime; a rule living in the app would need an app-store
release and a rural user who actually updates. A CI lint fails the build on
money arithmetic or a hardcoded threshold.

**100% coverage, with a short exclusion list.** Only generated code, `cmd/api`
wiring and vendored code may be excluded, each with a justification in
`backend/tests/coverage-exclusions.txt`. Adding to that list requires an ADR.

## Documentation

- `docs/build/` — how the system gets built
- `docs/technical/` — one file per source file: responsibility, inputs, rules, failure modes
- `docs/user/` — one file per screen, per role
- `docs/decisions/` — why things are the way they are
- `docs/design-gaps.md` — where the design and the backend do not line up, and why
- `docs/security-review.md` — what was checked, what was fixed, what is accepted
- `docs/runbook.md` — running it in production: config, probes, incidents, the one thing you must schedule
- `docs/load-test.md` — the discovery and dispatch load gates, and the last measurements
- `BUILD_COMPLETE.md` — what was built, and what is left
