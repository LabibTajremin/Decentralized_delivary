# Deployment

How to get GoKlay running on a server, start to finish.

`docs/runbook.md` is for operating it once it is up — probes, incidents,
backups, rollback. This is the part before that: what to provision, what to
set, what to run, and how to tell it worked.

Everything here was run against the real binary except the container image
build itself, which needs a Docker daemon.

---

## 0. Read this first: what you can deploy today

**Production is blocked, by design.** One-time codes are the only way anybody
signs in, and the only SMS sender that exists writes the code to the
application log; the only payment gateway that exists never moves money. Both
refuse to be constructed when `APP_ENV=production`, so **the binary will not
start**. Someone has to write the two adapters first — the seams exist
(`identity/application/ports.SMSSender`, `payment/application/ports.Gateway`)
and each is one file.

**A demo is fully supported**, and is what most people want first. With
`DEMO_MODE=true` the code comes back in the response to the request that asked
for it, and the app can settle a manual-gateway payment itself. Seven seeded
accounts across the three apps come with it. See `docs/demo.md`; this guide
deploys exactly that.

So: `APP_ENV=staging`, and everything below works.

---

## 1. Pick a shape

| | **A — one machine** | **B — managed pieces** |
|---|---|---|
| What | A VPS running Docker Compose: Postgres, Redis, the API, Caddy | Neon or Supabase for Postgres, a managed Redis, the API container on Fly.io / Railway / Render |
| Cost | one bill, roughly the price of a 2 GB VPS | free tiers cover a demo; three bills later |
| Time | about ten minutes | about thirty, mostly waiting on dashboards |
| TLS | Caddy does it | the platform does it |
| Backups | yours to set up | included, with point-in-time restore |
| Best for | a demo, a pilot, anything you want to be able to `docker compose logs` | something you expect to grow, or a team that does not want a server |

**Start with A.** It is one `docker compose up -d`, everything is in one place
while you are still learning what the product does, and moving to B later is a
change of connection strings rather than a rewrite. This guide does A in full
and gives B's differences in §6.

### Why not Vercel, or anything serverless

Worth stating plainly, because it is the usual first instinct and it does not
fit here:

* **The tracking screen is server-sent events.** `GET /v1/track/{orderId}`
  holds one response open for the whole delivery. Serverless request timeouts
  kill it, and a customer watching a rider is the demo's best moment.
* **The dispatch heartbeat needs to run every twelve seconds** (§5). That is
  a process, not a request.
* **The API is one long-lived Go binary** with a Postgres connection pool.
  Per-request cold starts would open a new pool each time, and a managed
  Postgres would run out of connections long before traffic did.

A container that stays running is the right shape. Fly.io, Railway, Render and
a plain VPS all do that; Vercel and Lambda do not.

---

## 2. What you need before you start

* **A machine.** 2 GB RAM is comfortable for a demo — Postgres with PostGIS is
  the hungry part. 1 GB works if nothing else is on it. Any Linux with Docker.
* **A domain**, with an `A` record already pointing at that machine. Caddy
  requests a certificate on first start and the request fails if the name does
  not resolve there yet, so do this first and let it propagate.
* **Docker and the compose plugin.**
  ```bash
  curl -fsSL https://get.docker.com | sh
  ```
* **The repository on the machine**, or built elsewhere and pushed to a
  registry. The simplest thing is to clone it.

You do **not** need Go, Flutter, or anything else installed on the server. The
image builds both binaries it needs.

---

## 3. Deploy it

```bash
git clone https://github.com/LabibTajremin/Decentralized_delivary.git
cd Decentralized_delivary/deploy

cp .env.example .env
```

Now edit `.env`. Five things actually matter:

```bash
GOKLAY_DOMAIN=demo.example.com         # the A record you pointed here
TLS_EMAIL=you@example.com              # Let's Encrypt expiry warnings
PUBLIC_BASE_URL=https://demo.example.com

POSTGRES_PASSWORD=$(openssl rand -base64 24)
JWT_SIGNING_KEY=$(openssl rand -base64 48)
PAYMENT_WEBHOOK_SECRET=$(openssl rand -base64 48)
```

> **Generate the signing key even for a demo.** Outside production it defaults
> to `insecure-development-signing-key-do-not-use`, which is printed in this
> repository's source. Without your own, anybody could mint an admin token for
> your deployment.

Then:

```bash
docker compose up -d --build
docker compose ps          # postgres and redis healthy, api and caddy up
```

The first build takes a few minutes: it compiles the API and the migrator into
a distroless image with nothing else in it.

### Create the schema

The image ships the migrator alongside the API, so this needs no Go on the
server — just override the entrypoint:

```bash
docker compose run --rm --entrypoint /migrate api up
docker compose run --rm --entrypoint /migrate api status
```

`status` should show twelve migrations applied, nothing pending, not dirty.

`migrate up` is safe to run on every deploy: an up-to-date database applies
nothing. The first run needs a database role that may `CREATE EXTENSION` —
migration `0001` installs PostGIS and `0006` installs `pg_trgm`. The
`postgis/postgis` image and both Neon and Supabase allow this by default.

### Load the world

**This is not optional, and it is not only demo data.** The divisions,
districts and areas in `seed/0001_geo.sql` are real geometry that the product
cannot work without: no areas means no config resolution, no D3 division
ceiling, and no delivery fee.

```bash
docker compose run --rm --entrypoint /migrate api seed
```

Four scripts run: the geography, fourteen approved shops, their menus, and the
seven demo accounts. It is idempotent — running it again restores anything
that was edited without duplicating anything.

Seeding is refused outright when `APP_ENV=production`.

### Check it

```bash
curl -fsS https://demo.example.com/healthz   # {"status":"ok"}
curl -fsS https://demo.example.com/readyz    # {"status":"ready"}
```

`readyz` is the one that matters: it touches both stores and names the one it
could not reach.

Then sign in, which is the real test:

```bash
curl -sS -X POST https://demo.example.com/v1/auth/otp/request \
  -H 'Content-Type: application/json' -d '{"phone":"01700000001"}'
```

In demo mode the response carries `demo_code`. If it does, the whole chain
works — TLS, Caddy, the API, Postgres, Redis and the seed.

---

## 4. Build the three apps

The APKs are built on your machine, not the server, and they are built
**against the URL above**, which is compiled in:

```bash
cd frontend
flutter pub get

for app in customer merchant partner; do
  ( cd "$app" && flutter build apk --release --split-per-abi \
      --dart-define=GOKLAY_API_BASE_URL=https://demo.example.com )
done
```

The APKs land in `frontend/<app>/build/app/outputs/flutter-apk/`. For a phone,
`app-arm64-v8a-release.apk` is the one; the others are for older 32-bit
devices and for emulators.

Three things worth knowing:

* **The define name is `GOKLAY_API_BASE_URL`.** Anything else is silently
  ignored and the app falls back to `http://10.0.2.2:8080`, which is the
  Android emulator's route to its host — so a build with a typo works on an
  emulator and fails on a real phone in a way that looks like a network bug.
* **The URL ships inside the APK.** Changing it means a new build, not a
  config change. Decide the domain before you hand anyone an APK.
* **It must be `https`.** Android blocks cleartext by default, so an app built
  against `http://` fails on every request with no useful error.

Flutter 3.47.5 is what CI pins and what `goklay_core/pubspec.yaml` constrains
against. A different version may or may not build.

---

## 5. Schedule the dispatch heartbeat

**Without this, orders reach `ready` and stop.** There is no background worker
inside the API: something has to call `POST /v1/admin/dispatch/sweep`, which
expires offers nobody answered and offers waiting jobs to somebody. When it is
missing, the symptom is a rider with an empty feed and a customer whose food
never moves — and it looks like a bug in the product rather than a missing
cron.

The compose file already runs it: the `sweep` service, every twelve seconds
(`dispatch.assignment_timeout` is thirty, so a slower cadence adds its own
interval to every reassignment).

It has to sign in, which is the awkward part, because the admin surface needs
a token and tokens come from one-time codes. `deploy/sweep.sh` handles both
cases:

* **On a demo**, it signs itself in with `ADMIN_PHONE` and the revealed code.
  Nothing to set up.
* **Anywhere else**, it uses a refresh token from `/state/refresh`, rotating
  and rewriting it on every use. Put the first one there by hand:

  ```bash
  # sign in as your admin once, however you normally would, then:
  printf '%s' 'the-refresh-token' \
    | docker compose exec -T sweep sh -c 'cat > /state/refresh'
  docker compose restart sweep
  ```

  Refresh tokens last sixty days and rotate on every use, so the job keeps
  itself signed in indefinitely as long as it keeps that file. The volume it
  lives on survives restarts.

Check it is working:

```bash
docker compose logs sweep | tail
# sweep: starting: every 12s against http://api:8080
# sweep: signed in as 01700000031
```

Silence after those two lines is success — it only logs problems.

### Raise the OTP rate limit on a demo

`auth.otp_requests_per_hour` is five per number per hour. That is right when a
number belongs to one person and far too low when everybody shares seven demo
numbers. Raise it to its maximum, as the admin:

```
PUT /v1/config/overrides
{"key":"auth.otp_requests_per_hour","level":"global","code":"",
 "value":"20","reason":"shared demo numbers"}
```

An ordinary admin setting — not something demo mode changes.

---

## 6. Path B: managed pieces

Everything above holds; three things change.

**Postgres — Neon or Supabase.** Both are Postgres with PostGIS available, and
both free tiers are ample for a demo. Neon's branch-per-environment is genuinely
useful here: a staging branch of the demo database costs nothing and resets in
seconds.

Take the connection string, **add `sslmode=require`**, and use it as
`DATABASE_URL`. Run the migrator against it from anywhere that can reach it:

```bash
docker run --rm -e DATABASE_URL='postgres://…?sslmode=require' \
  --entrypoint /migrate goklay-api:latest up
```

Watch the connection limit: each API replica holds its own pool, and free
tiers are stingy. One replica is fine; before scaling out, check the ceiling.

**Redis — any managed Redis.** Sessions, refresh tokens and rate limits live
there, and nothing transactional does, so losing it signs everybody out and
costs nobody an order. Two requirements to check on whatever you pick: the
rate limiter runs a small **Lua script** (`EVAL`) and the OTP store uses
**`GETDEL`**, which needs Redis 6.2 or newer. Use `rediss://` where TLS is
offered.

**The API — Fly.io, Railway or Render.** All three run a container that stays
running, terminate TLS for you, and take the same environment variables. Drop
the `postgres`, `redis` and `caddy` services; keep `api`, and keep `sweep`
somewhere that can reach it (Fly's `[processes]`, a Railway service, or a
Render cron job at its finest granularity).

You will not need the Caddyfile. You will still need every secret in §3.

---

## 7. Day two

### Deploying a new version

Migrations here are additive, so the order is **schema first, then code**: the
old binary keeps working against the new schema, and there is no window where
a half-replaced deployment is broken.

```bash
git pull
docker compose run --rm --entrypoint /migrate api status
docker compose run --rm --entrypoint /migrate api up
docker compose up -d --build api
```

`SIGTERM` starts a graceful shutdown bounded by `SHUTDOWN_TIMEOUT` (ten
seconds), and in-flight requests finish — `TestGracefulShutdown` asserts it
against the real binary, so a redeploy does not drop somebody's order.

### Rolling back

Roll the **code** back first; it is the safe half.

```bash
git checkout <previous tag>
docker compose up -d --build api
```

Rolling the schema back is a decision, not a step. Every migration has a
tested `down` — the integration suite rolls the whole schema back and forward
on every run — but they still drop columns, and a `down` after real traffic
destroys whatever the new version wrote. Restore from a backup instead, unless
the migration was minutes old.

### Backups

```bash
docker compose exec -T postgres \
  pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" | gzip > goklay-$(date +%F).sql.gz
```

Put that in cron and copy it off the machine. **Test a restore before you need
one** — point a staging API at it and run `migrate status`.

Redis needs no backup. It is a TTL store by design (ADR 0005).

### Resetting a demo

Visitors leave orders, reviews and tickets behind. The seed restores the
accounts and shops but removes nothing, so a clean slate is a rebuild:

```bash
docker compose run --rm --entrypoint /migrate api down 0
docker compose run --rm --entrypoint /migrate api up
docker compose run --rm --entrypoint /migrate api seed
docker compose exec redis redis-cli FLUSHALL
docker compose restart sweep    # its stored refresh token just died
```

Nightly is a reasonable default for a demo strangers can reach.

### Logs

```bash
docker compose logs -f api
```

One structured JSON line per request: method, path, status, bytes, duration
and `request_id` — which is echoed in the `X-Request-Id` response header. That
is what turns "a customer says it failed at 14:32" into one query. No bodies,
no `Authorization` header, no query strings.

Alert on `level=ERROR`: client errors log at info and server errors at error,
so it fires for real bugs and not for somebody sending a bad postcode.

---

## 8. When it does not work

| Symptom | Almost always | Fix |
|---|---|---|
| Caddy loops on "obtaining certificate" | the domain does not resolve to this machine yet | check the `A` record; wait for propagation; `docker compose restart caddy` |
| `readyz` says `database` | Postgres is not up, or `DATABASE_URL` is wrong | `docker compose ps`; on a managed Postgres check `sslmode=require` |
| `readyz` says `redis` | same, for Redis | sign-in is down; everything already authenticated keeps working ~15 minutes |
| API exits immediately | a required variable is missing or invalid | `docker compose logs api` — it names the variable |
| `DEMO_MODE must not be set in production` | `APP_ENV=production` with `DEMO_MODE=true` | use `staging` |
| `the logging SMS sender must not be used in production` | `APP_ENV=production` with no SMS adapter | §0 — this is the release blocker, not a misconfiguration |
| `migrate up` fails on `CREATE EXTENSION` | the role cannot install extensions | use a role that can, or install PostGIS and `pg_trgm` once by hand |
| No `demo_code` in the response | `DEMO_MODE` is not true | check it reached the container: `docker compose exec api env` fails (no shell) — use `docker compose config` |
| `429` on requesting a code | five per number per hour | §5, raise it; or wait |
| Orders reach `ready`, no rider is offered | the sweep is not running | `docker compose logs sweep` |
| The app cannot reach the API at all | built with the wrong define, or with `http://` | rebuild with `--dart-define=GOKLAY_API_BASE_URL=https://…` |
| The tracking screen never updates | a proxy is buffering the SSE stream | the supplied Caddyfile sets `flush_interval -1` for `/v1/track/*`; any replacement proxy needs the equivalent |

---

## 9. What this costs

For a demo: **one 2 GB VPS.** Postgres, Redis, the API and Caddy fit
comfortably; the three APKs cost nothing to host because you hand them out as
files. That is the whole bill.

Path B's free tiers cover a demo too — Neon and Supabase both have one, and so
does every managed Redis worth using — but you will be watching three
dashboards instead of one machine, and the free Postgres tiers sleep when
idle, which makes the first request after a quiet night slow.

What would change the arithmetic, roughly in the order you would hit it:

1. **Postgres CPU.** The two spatial queries are the only compute-heavy path.
   `docs/load-test.md` has measurements: at two thousand shops and two
   thousand riders, a radius search is ~1.4 ms and a candidate pool ~8.5 ms,
   both index-assisted and both gated so they stay that way.
2. **Concurrent deliveries**, not requests per second. Each customer watching
   a rider holds one connection open for the length of the delivery.
3. **API replicas**, which multiply Postgres connections. Add them behind
   Caddy or the platform's load balancer; the binary is stateless and holds
   nothing locally.

Nothing here needs a Kubernetes cluster, and putting one in front of it would
cost more than the product does.
