# Security review

P20's third acceptance criterion. This is a review of the code that exists, on
2026-09-22, at commit `9dec340` plus the P20 work in front of it — not a
checklist copied from a standard. Every claim below was checked against the
source or against the running binary, and where a check produced a fix, the
fix is in this phase's commit.

The review is organised by what it found, not by category, because a list of
categories with "OK" beside each is not information.

---

## What was fixed

### 1. A reachable vulnerability in a dependency — `golang.org/x/text`

`govulncheck` had never been run against this repository. It was, and it found
one vulnerability the code can actually reach:

```
Vulnerability #1: GO-2026-5970
    Infinite loop on invalid input in golang.org/x/text
  Module: golang.org/x/text   Found in: v0.29.0   Fixed in: v0.39.0
  Example trace: migrate.run → pgx.Connect → norm.Form.Properties
```

Reachable, not theoretical: `pgx.Connect` normalises connection-string text on
every connection, so the path is taken by the migrator and by the API at
startup. Fixed by `go get golang.org/x/text@v0.39.0`. Both modules now scan
clean.

That the scan had never run is the more interesting finding, so it is now a
gate: `scripts/vuln-scan.sh` runs `govulncheck` over both Go modules, and CI
runs it as its own `security` job.

It is deliberately **not** in `./scripts/verify.sh`. `govulncheck` downloads
the Go vulnerability database on every run, and `verify.sh` has to work on a
container with no outbound network. A gate that fails when the network is
down is a gate people learn to ignore, and an ignored gate is worse than an
absent one.

### 2. No authorisation sweep over the admin surface

Every module's suite proved its own guards, and every one of those tests named
the routes it checked by hand. That is the weakness: a route added next month
is guarded by whoever remembers to extend a list.

`backend/tests/e2e/security_test.go` now sweeps instead of listing. It reads
`api/openapi.yaml`, takes every path under `/v1/admin/`, and requires all of
them — 17 operations today — to refuse an anonymous caller (401) and each of
the three non-admin roles (403). Nothing enumerates the routes, so an admin
route mounted without the admin guard fails the build without anybody writing
a test for it.

Verified by feeding it a violation, the way `CLAUDE.md` asks: pointed at
`/v1/me/` instead, the sweep immediately reported every operation under that
prefix — correctly, because those routes are guarded by ownership rather than
by role. It distinguishes the two cases rather than passing on a technicality.

### 3. Payment had no end-to-end suite at all

Payment was the one module with no `_test.go` under `backend/tests/e2e/`. Its
authorisation is not role-based — every customer may pay — so the sweep above
cannot reach it, and what protects a payment is ownership.

Two tests were added, and both pass:

* `TestPaymentBelongsToWhoeverIsPaying` — a second customer cannot start a
  checkout against somebody else's order, cannot read its payment, and gets
  the same `404` for a real order that is not theirs as for an order id that
  never existed.
* `TestTheWebhookTrustsNothingButItsSignature` — the gateway callback, the one
  route with no bearer token behind it, refuses an absent signature, a
  non-hex signature, a valid signature over different bytes, and a signature
  made with the wrong secret. All four answer `400`, not `404`, which is the
  assertion that matters: a `404` would mean the server had already gone to
  the database on an unsigned request.

The whole-product regression (`regression_test.go`, P20.T02) also drives the
prepaid path for the first time end to end, including a replayed callback, to
prove the capture is idempotent against real Postgres rather than a fake.

---

## What is by design, with its cost stated

### Authorisation is by ownership, not by token role, outside `/v1/admin/`

This is the single most important thing to understand about this API's
security model, and it was not written down anywhere before this review.

Ten of the fourteen modules mount their non-admin routes behind
`authenticator.Authenticated()`, which is `Require(AllRoles()...)`. The
token's role is therefore *not* what keeps a customer out of
`/v1/partner/cod` or `/v1/merchants/me/hours`. What keeps them out is that
they own no partner record and no shop: the handler runs, finds nothing of
theirs, and answers `404`.

Checked, and it holds — a customer calling `/v1/partner/cod` gets `404`, and
`TestOwnershipIsWhatProtectsThePartnerSurface` proves the positive case too: a
rider's ledger route takes no partner id at all, so there is no parameter for
a caller to change, and reading another rider's ledger by id is possible only
through the admin route, which refuses them.

**The cost.** Role guards fail closed for a whole class of route; ownership
checks fail closed only if each handler remembers to make one. A new handler
that forgets is protected by nothing, and no middleware would notice. That is
a defence-in-depth gap rather than a live vulnerability, and the honest
recommendation is below.

### Access tokens cannot be revoked mid-life

Logout and logout-all revoke the refresh family in Redis immediately, and
reuse of a rotated refresh token revokes the whole session — proven by
`TestARefreshTokenWorksExactlyOnceAndReuseRevokesEverything`. An access token
already issued stays valid until it expires, because the authenticator holds
only a signer and never consults the session store.

Already recorded and accepted in ADR 0005, with the 15-minute
`auth.access_token_ttl` as the bound. Repeated here because a security review
that omits a known window is not a review. Checking a session on every request
would put Redis in the path of every call; the 15-minute exposure was judged
the better trade, and it is a *judgement*, which means it should be revisited
if the product ever holds something worth fifteen minutes of a stolen phone.

### Production fails closed, and cannot currently ship

Two adapters are working stand-ins that refuse to exist in production, and
both refusals happen at process start:

* `identity/infrastructure/sms.LogSender` writes one-time codes to the log.
  With `APP_ENV=production` its constructor returns an error and the binary
  does not come up — asserted by
  `TestProductionRefusesToStartWithoutAnSMSGateway`.
* `payment/infrastructure/gateway/manual.Gateway` never moves money.
  `New(secret, production)` refuses the same way, and
  `WebhookUseCase.Simulate` checks the gateway's name a second time, so a
  dev-only route accidentally left mounted still cannot mark an order paid.

This is the right behaviour and it is also a release blocker: **the product
cannot be deployed to production until a real SMS provider and a real payment
gateway are written behind those two ports.** `docs/runbook.md` treats it as
one.

### The gateway webhook is not rate limited

`POST /v1/payments/manual/webhook` has no bearer token and no limiter, so an
attacker can send it as fast as the network allows. Each request costs one
HMAC and, once the signature fails, nothing else — no database round trip, as
test 3 above proves. The exposure is CPU, not data, and it belongs behind the
edge rate limit the runbook asks for rather than behind an application
limiter that a legitimate gateway's retry storm would also trip.

---

## Recommendations, in the order they are worth doing

None of these is a live vulnerability. They are the things a second review
would find if this one changed nothing.

1. **Pepper the OTP hash.** Codes are stored as
   `sha256(phone + ":" + code)` (`identity/domain/otp.go:88`), compared in
   constant time, consumed with `GETDEL`, short-TTL'd and attempt-limited —
   all correct. But the hash is salted only by the phone number, so anyone who
   can read Redis can recover a live code in a million hashes, which is
   instant. An `hmac.New(sha256.New, appSecret)` in place of the bare hash
   makes a Redis read insufficient on its own. Cheap, local, no schema change.
   (Refresh tokens do not have this problem: their plaintext is high-entropy
   random, so an unpeppered hash of one is not invertible.)
2. **Set `IdleTimeout` on the HTTP server.** `ReadHeaderTimeout` is 5s, which
   is the slowloris guard and the one that matters. `IdleTimeout` is unset, so
   a keep-alive connection that goes quiet is held indefinitely. `WriteTimeout`
   must stay unset — the tracking stream is server-sent events and a write
   deadline would kill every live delivery — but an idle deadline of a minute
   or two costs nothing and bounds the connection table.
3. **Add role-scoped guards alongside the ownership checks.** Not to replace
   them: to make a forgotten ownership check a 403 rather than a hole. A
   `Require(RolePartner, RoleAdmin)` on the `/v1/partner/…` group and
   `Require(RoleMerchant, RoleAdmin)` on `/v1/merchants/me/…` would change no
   passing behaviour and would extend the sweep in finding 2 to the whole
   surface. It is a ten-module change, which is why it is a recommendation
   here and not a P20 commit: doing it properly means updating each module's
   handler signature, its unit tests and the documented `403` responses, and
   that is a phase of its own, not a footnote to the last one.
4. **Rate limit at the edge.** The application limiter covers the three
   identity paths that are worth brute-forcing (OTP request, OTP verify,
   refresh) and nothing else. Discovery, the menu reads and the webhook are
   uncapped. A per-IP limit in front of the API is the right place for this —
   `docs/runbook.md` states it as a deployment requirement rather than
   leaving it to be discovered.
5. **`golang.org/x/sys` GO-2026-5024 stays unfixed, on purpose.** The
   remaining `govulncheck` finding is an integer overflow in
   `golang.org/x/sys/windows`. Nothing calls it — the scan reports it as
   present in a required module and absent from any trace — and the
   deployment target is Linux. The fixed version, v0.48.0, requires Go ≥ 1.26,
   which would drag the go directive of both modules and `go.work` up from
   1.25 and change the toolchain CI pins. Trading a toolchain bump for a
   Windows-only unreachable overflow is not a good trade; revisit when this
   codebase moves to Go 1.26 for its own reasons.

---

## What was checked and found sound

Listed so the next reviewer knows where this one already looked. Each was read
in the source, and most have a test named beside them.

**Tokens and sessions**

* HS256 and nothing else. The header's `alg` is checked against a literal, so
  neither `none` nor an `RS256` confusion attack gets past
  (`identity/infrastructure/token/jwt.go:166`).
* The signature is verified *before* the payload is decoded, in that order,
  with a comment saying why. Reading claims first is how algorithm-confusion
  bugs are written.
* `hmac.Equal` for signatures, `subtle.ConstantTimeCompare` for OTPs.
* Issuer is checked, so a staging token does not open production.
* The signing key is never in the database (a standing rule), is required in
  production, must be ≥32 bytes, and cannot be the development default.
* Refresh tokens are single-use, rotated in one atomic Redis script, with a
  spent-token family: presenting a rotated token revokes the whole session,
  and an ordinary logout is not reported as a theft.
* OTP and refresh tokens are stored hashed, never in plaintext.

**Authorisation and information disclosure**

* An `insufficient_role` response never says which role was needed.
* Another user's resource answers `404`, not `403`, throughout — so an id
  list cannot be used to discover which ids are real. Asserted for addresses,
  orders, shops, menus, tracking streams and now payments.
* A `Require()` with no roles panics at wiring time rather than admitting
  nobody at 3am.
* Admin is not implicitly allowed everywhere; a route that wants both lists
  both.
* D3, the division ceiling, cannot be turned off over HTTP
  (`TestTheDivisionCeilingCannotBeTurnedOffOverHTTP`), and cannot be defeated
  by volume either (`load/discovery_load_test.go`).

**Input handling**

* Request bodies are capped at 1 MiB and unknown fields are rejected, so a
  typo'd field name is an error rather than a silently dropped value.
* A second JSON document in one body is refused.
* Content-Type is enforced for JSON endpoints.
* Every SQL statement is parameterised. The audit was explicit: the only
  `fmt.Sprintf` calls into SQL in the whole backend build placeholder indices
  (`$3`) or substitute a table name from a package constant. No user-supplied
  value reaches a query as text.
* Coordinates are validated against Bangladesh's geometry, not merely against
  latitude and longitude ranges.

**Responses and transport**

* A panic becomes a 500 with an opaque message; the stack goes to the log
  only. Internal errors never carry their detail to the client — `WriteError`
  replaces the message and drops the fields for `KindInternal`.
* CORS allows exactly the configured origins. No wildcard, no
  reflect-any-origin, `Vary: Origin` set so a shared cache cannot cross
  origins.
* `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
  `Referrer-Policy: no-referrer` on every response.
* `PUBLIC_BASE_URL` must be `https` in production, enforced at startup.
* Request logs carry method, path, status, size, duration and request id —
  no bodies, no `Authorization` header, no query strings.

**Secrets and artefacts**

* Nothing secret is committed. `.env` and `.env.local` are ignored; only
  `.env.example` is tracked, and every value in it is a placeholder or a
  localhost URL. A scan for private-key blocks and AWS-style keys found
  nothing.
* Demo imagery is embedded in the binary and its handler refuses to mount in
  production, so generated placeholder logos can never appear beside a real
  merchant.

**No TLS in the process, and that is correct.** The API speaks HTTP and
expects TLS to terminate in front of it. That is why there is no HSTS header
here — the header belongs at the proxy that owns the certificate, and
`docs/runbook.md` says so.
