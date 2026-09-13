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

Requires Go 1.24 and Docker.

```bash
git clone https://github.com/LabibTajremin/Decentralized_delivary.git
cd Decentralized_delivary

docker compose up -d postgres redis    # PostGIS + Redis
cd backend && go build ./... && go run ./cmd/api
curl localhost:8080/healthz            # {"status":"ok"}
```

Run the test suite and the coverage gate:

```bash
./scripts/coverage-gate.sh       # backend tests, gated at 100%
./scripts/thin-client-lint.sh    # no business rules in the Flutter apps
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
