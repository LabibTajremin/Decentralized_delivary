# Production runbook

What an operator needs to run GoKlay: what to set, what to start, what to
watch, and what to do at 3am. Everything here was read out of the code rather
than assumed, and where the code makes a decision for you, this says so.

`README.md` gets you a stack running locally, and `docs/deployment.md` gets it
onto a server for the first time. This is the document after those: the one
for the deployment nobody is watching.

---

## 0. Two things block a production release today

Read this before planning a launch date. Both are deliberate, both fail
closed, and neither is a bug.

**There is no SMS provider.** One-time codes are the only way anybody signs
in. The only sender that exists (`identity/infrastructure/sms.LogSender`)
writes the code to the application log, and its constructor returns an error
when `APP_ENV=production`, so **the binary will not start**. Someone has to
write an adapter for a Bangladeshi SMS gateway behind
`identity/application/ports.SMSSender` first. The seam exists and is the only
thing that has to change.

**There is no payment gateway.** `payment/.../gateway/manual.Gateway` never
moves money and refuses production the same way, behind
`payment/application/ports.Gateway`. Until a provider adapter exists, only
cash-on-delivery works, and `PAYMENT_METHOD=online` cannot be honoured.

Until both exist, staging is the furthest this can be deployed. That is the
correct behaviour — a gateway that always succeeds is a customer who paid
nothing for an order marked paid — but it is a release blocker and not a
footnote.

**A demonstration is possible in the meantime**, and is a supported
configuration rather than a workaround: `DEMO_MODE=true` shows one-time codes
to whoever asks for one and lets the app settle a manual-gateway payment
itself, and the seed provides seven accounts across the three apps.
**`docs/demo.md`** is the whole of it. It cannot be set in production — the
process refuses to start — and a demo is a deployment where anyone can sign in
as anyone, so nothing belongs in its database that would matter if it were
public.

---

## 1. What the system is made of

| Piece | What it is | Notes |
|---|---|---|
| `backend/cmd/api` | the whole API, one binary | stateless; run as many as you like |
| `backend/cmd/migrate` | schema and seed CLI | same migrations the tests run |
| PostgreSQL **with PostGIS** | the database | not optional — see below |
| Redis | sessions, refresh tokens, rate limits | data loss = everyone signs in again |
| a reverse proxy | TLS, rate limiting | the API speaks plain HTTP on purpose |
| a scheduler | the dispatch heartbeat | **required** — see §6 |

**PostGIS is a hard requirement, not a preference.** ALG-01 needs `ST_DWithin`
over a GiST index and ALG-03 needs `ST_Contains`. A plain Postgres fails on
migration `0001`. ADR 0003 records why the portability was traded away.
Managed options that work: RDS with the PostGIS extension, Cloud SQL, Neon,
Supabase. Use `sslmode=require` with any of them.

The API is one process with no leader, no in-memory queue and no local state,
so horizontal scaling is adding replicas behind the proxy. The one caveat is
the dispatch heartbeat in §6.

---

## 2. Configuration

Every variable is in `.env.example`, with a comment saying what it is for.
Read that file; it is the reference. What matters here is the shape of it:

**Deploy-time settings live in the environment. Business rules do not.**
Radius, delivery fee bands, COD limits, commission, offer timeouts, token
TTLs — all of it is in the config module, tunable per area at runtime by an
admin, audited, and unaffected by a restart (ADR 0004). If you are editing an
environment variable to change what the product charges, you are in the wrong
place.

Required in production, enforced at startup:

| Variable | Rule |
|---|---|
| `APP_ENV=production` | turns on every check below |
| `DEMO_MODE` | must be unset or false; `true` refuses to start (`docs/demo.md`) |
| `JWT_SIGNING_KEY` | ≥32 chars, not the development default. `openssl rand -base64 48` |
| `PAYMENT_WEBHOOK_SECRET` | ≥32 chars, not the development default, different per environment |
| `PUBLIC_BASE_URL` | must be `https://…` |
| `DATABASE_URL`, `REDIS_URL` | always required |

A misconfigured production server does not start. That is the design: it is
easier to diagnose a process that refused to come up than one that came up
and quietly issued tokens signed with `insecure-development-signing-key`.

**The signing key is never stored in the database.** Anything in the database
is readable by an admin, and an admin who can read the signing key can mint a
token for any user, including another admin. Environment or secrets manager
only.

`PUBLIC_BASE_URL` is compiled into the mobile apps as
`--dart-define=GOKLAY_API_BASE_URL`. Getting it wrong means a new build, not a
config change. Decide it before the first release.

---

## 3. First deploy

```bash
# 1. Bring up Postgres (with PostGIS) and Redis. Then, from a host that can
#    reach the database:
export DATABASE_URL='postgres://user:pass@db:5432/goklay?sslmode=require'
export APP_ENV=production

# 2. Schema. Idempotent: an up-to-date database applies nothing.
go run ./backend/cmd/migrate up
go run ./backend/cmd/migrate status     # confirm: applied, nothing pending, not dirty

# 3. Geography. THIS IS NOT OPTIONAL and it is not the demo seed.
#    `migrate seed` is refused when APP_ENV=production, because it loads demo
#    shops and fake documents. But the divisions, districts and areas in
#    backend/migrations/seed/0001_geo.sql are real geometry that the product
#    cannot work without: no areas means no config resolution, no D3 ceiling
#    and no delivery fee. Load 0001_geo.sql by hand, or with APP_ENV unset
#    against a database you then point production at.

# 4. Start the API.
./api      # or the container; distroless, nonroot, port 8080
```

Then check, in this order:

```bash
curl -fsS https://api.example.com/healthz   # {"status":"ok"}
curl -fsS https://api.example.com/readyz    # {"status":"ready"}
```

A `503` from `/readyz` names the dependency it could not reach — `database`
or `redis` — which is the first thing to fix.

### The proxy in front is doing three jobs

The API deliberately does none of them:

1. **TLS.** There is no certificate handling in the binary. This is also why
   there is no `HSTS` header in the responses: the header belongs to whoever
   owns the certificate. Set `Strict-Transport-Security` at the proxy.
2. **Rate limiting.** The application limits exactly three paths — OTP
   request, OTP verify and token refresh — because those are the ones worth
   brute-forcing. Everything else, including the unauthenticated gateway
   webhook, is uncapped. Add a per-IP limit at the edge.
   `docs/security-review.md` §4 has the reasoning.
3. **Request body size beyond 1 MiB.** The API caps JSON bodies at 1 MiB
   itself; the proxy should cap the connection.

---

## 4. Rolling out a new version

```bash
go run ./backend/cmd/migrate status   # before anything
go run ./backend/cmd/migrate up       # additive migrations first
# then replace instances one at a time
```

Migrations in this repository are additive, and the deploy order that follows
from that is: **schema first, then code.** The old binary keeps working
against the new schema, so there is no window where a half-replaced fleet is
broken.

**Draining works.** `SIGTERM` starts a graceful shutdown bounded by
`SHUTDOWN_TIMEOUT` (10s by default), and in-flight requests finish.
`TestGracefulShutdown` asserts it against the real binary, so a rolling
deploy does not drop a customer's order placement. Give the orchestrator a
termination grace period longer than `SHUTDOWN_TIMEOUT`.

**Probes.** Wire them the right way round, because getting this wrong turns a
database blip into an outage:

| Probe | Endpoint | Why |
|---|---|---|
| liveness | `GET /healthz` | dependency-free on purpose. Restarting the API because Postgres is down fixes nothing and takes the healthy replicas with it |
| readiness | `GET /readyz` | pings Postgres and Redis, 2s timeout. A failure takes *this instance* out of the load balancer, which is the correct response |

### Rolling back

Roll the **code** back first; it is the safe half. Rolling the schema back is
a decision, not a step:

```bash
go run ./backend/cmd/migrate status
go run ./backend/cmd/migrate down 1     # one step; `down 0` rolls back everything
```

Every migration has a tested `down` — the integration suite rolls the whole
schema back and forward on every run, so the down scripts are exercised
continuously rather than being hoped about. They still drop columns and
tables, and a `down` after real traffic destroys whatever the new version
wrote. Restore from backup instead unless the migration was minutes old.

---

## 5. Backups

| Store | What is lost | What to do |
|---|---|---|
| Postgres | everything that matters | daily base backup + WAL archiving, or the managed equivalent with PITR. Test a restore before you need one |
| Redis | sessions and refresh tokens | AOF is on in the compose file. Losing it signs everybody out; it costs nobody an order |

Redis is deliberately not a system of record. Sessions and refresh tokens live
there with a TTL so revocation and expiry are O(1) (ADR 0005). Nothing
transactional is in it. A Redis restore that comes back empty is an
inconvenience — every client signs in again — not an incident.

Verify a Postgres restore by pointing a staging API at it and running
`migrate status`. A backup nobody has restored is not a backup.

---

## 6. The dispatch heartbeat — the one thing you must schedule

**There is no background worker inside the API.** Nothing in
`cmd/api/main.go` runs on a ticker. The dispatch board therefore needs an
external nudge:

```
POST /v1/admin/dispatch/sweep      (admin token)
```

It does two passes: expire offers nobody answered, then offer the jobs sitting
on the board to somebody. Without the first, a job stalls behind a rider who
put their phone in a pocket. Without the second, a declined job waits forever
and the customer's food goes cold while the shop believes a rider is coming.

**Run it every 10–15 seconds.** `dispatch.assignment_timeout` defaults to 30s,
so a slower cadence adds its own interval to every reassignment.

Use a cron, a Kubernetes `CronJob`, a systemd timer — anything, as long as it
is monitored. If the sweep stops, orders stop moving, and the symptom
(`ready` orders with no rider) looks like a dispatch bug rather than a missing
cron.

The admin token it uses is an ordinary one: sign in as an admin and refresh it,
or give the scheduler its own admin account. Access tokens last 15 minutes by
default, so the job has to refresh rather than hold one.

### ALG-09, the radius auto-tuner, is also on demand

```
POST /v1/admin/config/autotune     (admin token, per area)
```

Widens or narrows `discovery.base_radius` and `dispatch.partner_radius` for
one area from a merchant-density and failure-rate signal. Unlike the sweep it
is optional — it is a convenience, not a correctness requirement — and it is
best run daily rather than continuously. Every write goes through the same
path an admin's own change takes, so bounds, pins and the audit log all apply.
Read `GET /v1/admin/config/changes` afterwards to see what it did.

---

## 7. What to watch

There is no metrics endpoint. Logs are structured JSON (`slog`), one line per
request, carrying method, path, status, byte count, duration and request id.
Ship them somewhere queryable; the alerts below are all log queries.

| Alert | Query | Why |
|---|---|---|
| any `level=ERROR` | `level=ERROR` | client errors log at info and server errors at error, so this fires for our bugs and not for someone sending a bad postcode |
| `panic in handler` | message match | recovered, so the process survives — but it is a bug with a stack in the log |
| `/readyz` failing on any instance | probe | a store is unreachable from somewhere |
| the sweep stopped | scheduler | §6. Nothing else notices |
| `SMS is not configured` | message match | should be impossible in production; if it appears, codes are being written to the log and the log is now a credential store |
| p99 request duration climbing | `duration_ms` | see §9 |

`request_id` is echoed in the `X-Request-Id` response header and appears on
every log line for that request, including the panic line. It is what turns
"a customer says it failed at 14:32" into one log query.

---

## 8. Incidents

Each of these starts from a symptom, because that is what you have at 3am.

### `/readyz` says `database`

The API cannot reach Postgres. It is still serving `/healthz`, which is why it
has not been restarted.

1. Is Postgres up? Is it accepting connections? Has it hit `max_connections`?
2. `SELECT count(*) FROM pg_stat_activity;` — each API replica holds a pgx
   pool, so replica count multiplies connections. This is the usual cause
   after a scale-up.
3. Did credentials or the network path change? A rotated database password
   needs a restart: `DATABASE_URL` is read once at process start.

The API recovers on its own once Postgres does. No restart needed.

### `/readyz` says `redis`

Nobody can sign in, refresh a token, or be rate limited; everything already
authenticated keeps working for up to 15 minutes on existing access tokens.

Bring Redis back. If the data is gone, everybody signs in again and no orders
are affected — sessions are the only thing in there.

### Orders reach `ready` and no rider is ever offered one

Almost always the sweep (§6). Check the scheduler first, not the code.

If the sweep is running, the questions in order are: are there partners
`available` in that area at all, is `dispatch.partner_radius` too small for
the area, and does D4 exclude them (a `short`-only rider is not a candidate
for a long job, by design). `GET /v1/config/effective?area=…` shows what the
area is actually running with, and `GET /v1/admin/config/changes` shows who
last changed it — including the auto-tuner.

### Payments stay `pending`

A prepaid order sits at `pending_payment` until the gateway calls back.

1. Did the callback arrive? Look for `POST /v1/payments/manual/webhook` in
   the request log.
2. Did it arrive and get refused? A `400` means the signature did not verify
   — `PAYMENT_WEBHOOK_SECRET` differs between this deployment and what the
   gateway was configured with. A `409` means the amount did not match.
3. Replays are safe. A gateway that retries cannot double-capture: a payment
   leaves `pending` exactly once no matter how many identical callbacks
   arrive, which the E2E regression proves by sending the same one twice.

`GET /v1/admin/payments/{orderId}` is the support agent's view.

### Nobody is receiving one-time codes

Once a real SMS adapter exists this is the provider. Before that, in
production, the process would not have started at all (§0).

Check the provider's own dashboard and delivery receipts first. Then confirm
the number is being normalised as expected — phone numbers are stored in
E.164 so one number has one spelling.

### A customer says a price is wrong

It is not arithmetic in the app: the app performs none. Every amount crosses
the wire as a minor-unit integer *and* a preformatted string, and the server
composes both (rule 2.9, ADR 0004). So the answer is in the config for their
area:

```
GET /v1/config/effective?area=DHK-DHM     (admin)
GET /v1/admin/config/changes              (admin — who changed what, and why)
```

Every override carries a reason and an actor. If a fee changed at 14:00, that
log says who.

### The division ceiling is "blocking a legitimate order"

It is not a setting and it cannot be turned off. D3 — no delivery across a
division boundary — is enforced inside the SQL `WHERE` clause, not applied
afterwards, and `TestTheDivisionCeilingCannotBeTurnedOffOverHTTP` exists to
make sure a future change cannot make it optional. A customer in one division
ordering from a shop in another is refused, and no config value, admin action
or auto-tune will change that.

If the boundary itself is wrong, the fix is the geometry in
`backend/migrations/seed/0001_geo.sql`, not the rule.

---

## 9. Capacity

`docs/load-test.md` has the measurements and the method. In short, at 2,000
shops and 2,000 riders on one container, with sixteen concurrent callers:

* a radius search (ALG-01) is ~1.4 ms p50, 6.5 ms p95;
* a whole seven-step expansion (ALG-02) is ~10 ms p50;
* a dispatch candidate pool (ALG-04) is ~8.5 ms p50.

The load gates assert the query *plan*, not just the timing, and that is the
number to care about operationally: both spatial queries must use their GiST
index, and both are asserted to at a scale where the planner chooses freely.
If p99 climbs on discovery or dispatch, the first thing to check is whether
the planner has changed its mind — usually because `ANALYZE` has not run on a
table that grew fast. Autovacuum handles this; if it has been starved, an
explicit `ANALYZE geo_merchant_locations; ANALYZE delivery_partners;` is the
one-line fix.

Scaling levers, cheapest first:

1. **More API replicas.** Stateless. Watch Postgres connections (§8).
2. **A read replica** is not wired up and would need repository changes —
   there is no read/write split in the code today. Do not assume one.
3. **Postgres CPU.** The spatial queries are the only compute-heavy path.
4. **Redis** is a cache and a TTL store, and is very unlikely to be the
   bottleneck before Postgres is.

The tracking stream is the one long-lived connection: `GET /v1/track/{orderId}`
holds a server-sent-events connection open per watching customer, polling every
`TRACKING_STREAM_INTERVAL` (3s). Concurrent deliveries, not requests per
second, is what sizes it. Lowering the interval makes screens feel faster and
costs a query per stream per tick.

---

## 10. Key rotation

**`JWT_SIGNING_KEY`.** Rotating it invalidates every access token
immediately. Refresh tokens live in Redis and survive, so clients recover on
their next refresh rather than being signed out — which is what makes rotation
something you can actually do in the middle of the day. Set the new key,
restart the replicas, and expect a burst of refresh calls.

**`PAYMENT_WEBHOOK_SECRET`.** Change it in the gateway's configuration and in
the environment together. In between, callbacks fail with `400` and the
payments stay `pending` until a retry arrives with the right signature.
Gateways retry, so a short window is recoverable — but do it when a human is
watching.

**Database and Redis credentials.** Both URLs are read once at process start.
Rotating either means a restart, so do it as a rolling deploy rather than an
in-place edit.

---

## 11. Before you call it released

```bash
./scripts/verify.sh        # every gate CI runs; must exit 0
./scripts/vuln-scan.sh     # govulncheck over both modules; needs network
```

Then, against staging with real geometry loaded:

* `/healthz` and `/readyz` both green from outside the cluster;
* one whole order placed, cooked, delivered, paid and reviewed by hand,
  through the three apps rather than through curl — the E2E regression proves
  the API, not the phones;
* the sweep scheduled, running, and monitored;
* a Postgres restore tested;
* TLS, HSTS and edge rate limiting confirmed at the proxy;
* `docs/security-review.md` §4 read, and its four recommendations either done
  or explicitly deferred by someone whose decision that is;
* `DEMO_MODE` unset. It cannot be true in production, but confirming it is
  absent is cheaper than reading a startup failure.
