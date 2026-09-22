# Build complete — P00 to P20

Date: 2026-09-22
Branch: `claude/goklay-design-system-9z500x`

GoKlay is built. Twenty-one phases, P00 through P20, from
`docs/build/CLAUDE_CODE_BUILD_INSTRUCTION.md` v2.0.

This file is the honest summary §0.5 asks for: what exists, what the gates
say, and — because it matters more than the rest — what is deliberately not
here.

---

## Against §0.5's stop condition

| # | Condition | State |
|---|---|---|
| 1 | Every phase P00–P19 `DONE` in `STATE.md` | **Yes**, and P20 with them |
| 2 | Every PR merged with checks green | **Not done, and not mine to do.** The standing instruction is never to merge and never to open a pull request; the user merges once, at the end. Every phase is committed and pushed to the branch above |
| 3 | Backend coverage 100.0% against the declared exclusions | **Yes** — `scripts/coverage-gate.sh` |
| 4 | Flutter coverage 100.0% against the declared exclusions | **Yes** — `scripts/flutter-coverage-gate.sh`, four packages, and the exclusion list is empty |
| 5 | The full E2E suite passes | **Yes**, including P20's whole-product regression |
| 6 | The build is clean | **Yes** — `./scripts/verify.sh` exits 0 |

Condition 2 is the only one outstanding, and it is outstanding by instruction
rather than by omission.

---

## What exists

### The backend

A Go 1.25 modular monolith. **Fourteen modules**, each with
`domain` / `application` / `infrastructure` / `transport`, plus the
`contract` other modules call and the `external/` package through which it
calls them:

`cart` · `catalogue` · `config` · `discovery` · `dispatch` · `geo` ·
`identity` · `merchant` · `notification` · `order` · `payment` · `pricing` ·
`review` · `user`

* **232 Go source files** under `backend/`.
* **101 documented API paths** in `api/openapi.yaml`, checked against the
  routes the server actually mounts in both directions — an undocumented
  route and a documented non-route both fail the build.
* **12 migrations**, each with a `down` that is exercised on every
  integration run rather than hoped about.

### The apps

Flutter 3.47.5, a Dart pub workspace with four members: `goklay_core` and the
customer, merchant and partner apps.

* **172 Dart files**, **46 screens** (24 customer, 11 merchant, 7 partner,
  4 shared).
* **Three release APKs at 17.50 / 17.38 / 17.19 MB** against a 20 MB
  ceiling, gated against 10% growth.
* No state-management package, no router package, no service locator, no code
  generation (ADR 0008). `frontend/coverage-exclusions.txt` is empty as a
  result.

### The tests

* **180 test files**, **1,945 test functions**, in their own Go module
  (ADR 0002).
* Unit tests against fakes, integration tests against real Postgres with
  PostGIS, E2E tests against the **real compiled binary**.
* **Load gates** (`backend/tests/load/`) that assert the *query plan* for
  ALG-01 and ALG-04 at a scale where the planner chooses freely, not just the
  timing.
* **A security sweep** that reads the OpenAPI document and requires every
  admin operation to refuse all three non-admin roles — with no list of routes
  to fall out of date.
* **A whole-product regression** that walks an empty database to a delivered,
  prepaid, reviewed order with a support ticket closed on it, each step taken
  by the party who really owns it.

### Coverage

100.0% on both sides, with a **two-entry** backend exclusion list — `cmd/api`
and `cmd/migrate`, both wiring, both covered in effect by the E2E suite, the
second with its own ADR — and an **empty** frontend one.

### The documentation

* `docs/technical/` — 19 files, one per module plus the frontend.
* `docs/user/` — **47 files**: one per screen, per role, plus an index.
* `docs/decisions/` — **13 ADRs**.
* `docs/design-gaps.md` — every place the design and the backend do not line
  up.
* `docs/security-review.md`, `docs/runbook.md`, `docs/load-test.md` — P20's
  three prose deliverables.
* `docs/demo.md` — deploying it as a public demonstration, the demo accounts,
  and a walkthrough of a whole delivery.
* `docs/deployment.md` and `deploy/` — a ready-to-run Compose stack (Postgres
  with PostGIS, Redis, the API, Caddy, the dispatch heartbeat) and the
  step-by-step behind it.

---

## What is deliberately not here

This is the part worth reading. None of it was forgotten.

### Two release blockers, both failing closed

**There is no SMS provider.** One-time codes are the only way anybody signs
in, and the only sender that exists writes the code to the application log.
Its constructor returns an error when `APP_ENV=production`, so **the binary
will not start in production**. Someone has to write an adapter behind
`identity/application/ports.SMSSender`.

**There is no payment gateway.** The manual gateway never moves money and
refuses production the same way, with a second independent check inside
`WebhookUseCase.Simulate` so that a dev-only route left mounted still cannot
mark an order paid.

Both seams exist and are the only thing that has to change. `docs/runbook.md`
§0 treats them as blockers rather than footnotes.

**A demonstration is possible without either**, and is supported rather than
improvised: `DEMO_MODE=true` shows one-time codes to whoever asks for one,
because a visitor has no handset an SMS could reach, and lets the app settle a
manual-gateway payment itself. The seed provides seven accounts across the
three apps, each already owning what their app is about. It refuses to exist
in production three separate ways. **`docs/demo.md`**.

### Three platform capabilities, honestly absent

No map on tracking or the address forms; no browser launch on the payment
screen; no file picker for merchant documents; no background location for
riders. Each is platform work behind a plugin this build does not carry, none
of them changes what the app *knows*, and each is recorded in
`docs/design-gaps.md` and in the affected screen's user documentation.

### Eight designed screens with no backend

Offers, Offer Popup, Promos, Add Promos, Apply Voucher, Referral, Business
Profile, Digital Payment / Payment Methods / GoKlay Pay, Safety, Permissions.
They exist as one honest placeholder saying "not available yet" rather than
as a plausible wallet balance somebody would eventually believe.

### Two apps with no design at all

The Figma file contains no merchant or partner screens — 10,456 nodes
searched. Their layout comes from the endpoints behind them and their look
entirely from `goklay_core`, which *was* extracted from that file. Nothing
visual was invented. ADR 0012.

### One operational requirement that is not in the binary

**There is no background worker.** The dispatch heartbeat —
`POST /v1/admin/dispatch/sweep` — must be scheduled externally, every 10–15
seconds. Without it, declined and expired offers sit on the board forever and
orders stop moving. `docs/runbook.md` §6.

### Four security recommendations, stated and not done

Pepper the OTP hash; set `IdleTimeout`; add role guards alongside the
ownership checks; rate limit at the edge. None is a live vulnerability, each
has its reasoning in `docs/security-review.md` §4, and the third is a
ten-module change that would be a phase of its own.

---

## What the build found, that only the tests could find

Recorded because the pattern held in every single phase, which is itself the
finding: **fakes hide real bugs**, and each phase's genuine defect was visible
only to an integration or E2E test.

* A pointer into a map being reallocated.
* An empty primary key that only collided on the second row.
* A dispatch job nothing ever took off the board.
* A `next_actions` list that offered an action the state machine refused.
* Four real customer-app bugs found by widget tests in P18 — a save button
  that never enabled because two controllers had no listeners; a screen that
  never repainted its own failure; a pushed screen left on the navigator after
  sign-out, showing a signed-out customer their own device list; and two
  account rows sharing one Bengali word.
* A reachable CVE in `golang.org/x/text`, found in P20 the first time
  `govulncheck` was ever run against this repository. Fixed, and the scan is
  now a CI job.

Every gate in this repository was verified by feeding it a deliberate
violation, because a gate nobody has seen fail is not known to work. The
newest three were no exception: the webhook signature check was re-run with a
wrong secret, the log-linear assertion with a quadratic stand-in, and the
admin sweep pointed at a prefix it should not pass.

---

## To run it

```bash
./scripts/verify.sh      # every gate CI runs; must exit 0
./scripts/vuln-scan.sh   # govulncheck; needs network, so not in verify.sh
```

`README.md` for the quickstart, `docs/runbook.md` before deploying anything.
