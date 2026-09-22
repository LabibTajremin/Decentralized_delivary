# Running GoKlay as a public demo

The product cannot be deployed to production yet: there is no SMS provider and
no payment gateway, and the binary refuses to start without them
(`docs/runbook.md` §0). It *can* be deployed as a demonstration, and this is
how.

A demo is a real deployment of the real binary against a real database. Nothing
is mocked, no code path is skipped, and every rule the product enforces is
enforced. Two things are different, and both are listed below with what they
cost.

---

## The two differences

### 1. One-time codes are shown instead of sent

A visitor to a demo types a phone number they do not own, or one nobody owns.
No SMS can reach them, so with an ordinary sender a demo has a sign-in screen
nobody can get past.

With `DEMO_MODE=true` the sender is `sms.DemoSender`, which delivers nothing
and lets the code come back in the response to
`POST /v1/auth/otp/request`, as `demo_code`. All three apps show it on the
code screen, with a line saying why it is there, and fill the field in.

**Everything else about the code is unchanged.** It is generated with the same
entropy, stored only as a hash, rate limited, locked out after too many wrong
attempts, consumed on first use and expired on a timer. A wrong code is still
refused. The demo does not weaken authentication in any way except the one it
has to: who is told the code.

### 2. Payments can be completed without a bank

The only gateway that exists is the manual stand-in, which never moves money.
Without help, an order placed with "pay online" sits at `pending_payment`
forever and the demo's prepaid path dead-ends.

So the checkout response carries `demo_completion: true` — set only where the
gateway is the stand-in **and** the deployment mounts
`POST /v1/payments/manual/complete`, which production does not. The customer
app shows a **complete the demo payment** button when the server says so, and
never otherwise.

A visitor may only complete their own payment. Somebody else's payment id
answers `404`, the same as one that never existed.

---

## What demo mode cannot be

**It cannot be production.** Three separate fences, none of which trusts the
others:

1. `APP_ENV=production` with `DEMO_MODE=true` fails at startup:
   `config: DEMO_MODE must not be set in production`.
2. `sms.DemoSender`'s constructor refuses a production flag.
3. `migrate seed` refuses to run against a production database, so the demo
   accounts below cannot exist there.

The rule to remember: **a demo is a deployment where anyone can sign in as
anyone.** The code is handed to whoever asks for it, so the phone number is no
longer a credential. Put nothing in a demo database that would matter if it
were public, and never point one at real data.

**Still set a real `JWT_SIGNING_KEY`.** Outside production it defaults to
`insecure-development-signing-key-do-not-use`, which is in the source: without
your own key, anybody could mint an admin token for your demo. Your value is
honoured on staging. Do the same for `PAYMENT_WEBHOOK_SECRET`.

---

## The demo accounts

Seeded by `backend/migrations/seed/0009_demo_accounts.sql`. These are ordinary
accounts — nothing here grants a privilege the same person could not get by
signing up. What they have is a head start, so that every app opens on
something to do rather than on an empty screen.

| Sign in with | In the app | And you are |
|---|---|---|
| **01700000001** | Customer | আয়েশা রহমান, with a saved Dhanmondi address — Home already has somewhere to search from |
| **01700000002** | Customer | তানভীর হাসান, in Mirpur. The same search, a different answer: this is radius-scoped discovery |
| **01700000011** | Merchant | the owner of **কাচ্চি ভাই ধানমন্ডি**, approved, stocked, open around the clock |
| **01700000012** | Merchant | the owner of **স্বপ্ন মিরপুর**, a grocery — shelf counts and units, where the restaurant has options |
| **01700000021** | Partner | করিম উদ্দিন, motorcycle, positioned in Dhanmondi, **off shift** |
| **01700000022** | Partner | রফিকুল ইসলাম, bicycle, in Mirpur, short-distance only (D4) |
| **01700000031** | — (API only) | an admin: the approval queue, the config surface, the dispatch sweep |

There is no admin app — the admin surface is HTTP only, so that account is for
whoever is operating the demo.

Two deliberate choices in that table:

**The riders start off shift.** Going on shift is the first thing a rider does
and the step a demo most needs to show: dispatch's first offer round asks only
partners who are already available, so a rider who has not gone on shift sees
an empty feed *with a notice explaining why*. Seeding them online would hide
the mechanism.

**The two demo shops are open 00:00–24:00.** The other twelve seeded shops keep
realistic 09:00–22:00 hours, which is right — a demo showing every shop open at
four in the morning would teach something false. But "come back at nine" is not
an answer for somebody looking at your demo from nine time zones away, so the
two shops a visitor signs in as are always open.

### A whole delivery, in one sitting

The demo exists to be walked. This is the shortest route through everything:

1. **Customer** (01700000001) — Home, tap **কাচ্চি ভাই ধানমন্ডি**, pick a dish,
   add it, review the cart, choose the address, place the order with **pay
   online**.
2. Still the customer — **pay now**, then **complete the demo payment**. The
   order leaves `pending_payment`.
3. **Merchant** (01700000011) — the order is on the board. Accept, start
   preparing, mark ready. *That last tap is what offers the job to a rider.*
4. **Partner** (01700000021) — go on shift **before** step 3's last tap if you
   can; the first offer round runs once, at `ready`. Accept, collected,
   delivered.
5. **Customer** again — watch it on the tracking screen between collected and
   delivered, then leave a review and raise a support ticket.
6. **Admin** (01700000031) — resolve the ticket, which triggers the refund.

If you get to step 4 and the feed is empty, the rider was not on shift when
the shop said ready. Run the dispatch sweep (below) and the job will be
offered.

---

## Deploying it

```bash
# 1. Postgres with PostGIS, and Redis. Then:
export APP_ENV=staging
export DEMO_MODE=true
export DATABASE_URL='postgres://user:pass@db:5432/goklay_demo?sslmode=require'
export REDIS_URL='redis://redis:6379/0'
export PUBLIC_BASE_URL='https://demo.example.com'
export JWT_SIGNING_KEY="$(openssl rand -base64 48)"
export PAYMENT_WEBHOOK_SECRET="$(openssl rand -base64 48)"

# 2. Schema, then the demo world and the demo accounts.
go run ./backend/cmd/migrate up
go run ./backend/cmd/migrate seed

# 3. The API.
./api

curl -fsS $PUBLIC_BASE_URL/readyz    # {"status":"ready"}
```

`APP_ENV` can be `development` or `staging`; anything but `production`. Use
`staging` so the logs say what this is.

**Build the apps against the same `PUBLIC_BASE_URL`**, which ships inside the
APK as `--dart-define=API_BASE_URL`. Getting it wrong means a new build, not a
config change.

**Put TLS in front of it.** The API speaks plain HTTP; a demo is still a public
service and its sign-ins still cross the internet.

**Schedule the dispatch sweep.** `POST /v1/admin/dispatch/sweep` every 10–15
seconds, with the admin account above. Without it a demo reaches step 4 and
stops, and it looks like a bug in the product rather than a missing cron.
`docs/runbook.md` §6 has the details.

### Raise the OTP rate limit

`auth.otp_requests_per_hour` defaults to **5 per number per hour**. That is
right for a real deployment, where a number belongs to one person. On a demo
everybody shares seven numbers, so five is a few minutes of traffic.

It is an ordinary admin setting, not anything demo mode changes. Raise it to
its maximum as the demo admin:

```
PUT /v1/config/overrides
{"key":"auth.otp_requests_per_hour","level":"global","code":"",
 "value":"20","reason":"shared demo numbers"}
```

Twenty an hour per number is still a limit. If a demo is busier than that,
give out more numbers rather than trying to remove the cap: the lockout that
follows too many *wrong* codes is a separate and much more important one, and
it stays.

### Resetting it

Visitors leave orders, reviews and tickets behind. The seed is idempotent, so
re-running it restores the accounts and shops but does not remove what people
added. A clean slate is a full rebuild:

```bash
go run ./backend/cmd/migrate down 0   # everything
go run ./backend/cmd/migrate up
go run ./backend/cmd/migrate seed
```

Redis holds sessions and rate-limit counters. Flushing it signs everybody out
and clears the limits, which is usually what you want alongside a reset.

A nightly reset is a reasonable default for a demo that strangers can reach.

---

## What a demo still cannot show

Honest limits, so nobody demonstrates something that is not there:

* **No real SMS, ever.** That is the point of demo mode.
* **No real money.** The manual gateway records an attempt and settles it when
  told to. Refunds succeed for the same reason: there is nothing to give back.
* **No map.** Tracking and the address forms show coordinates, not tiles —
  there is no maps plugin in this build (`docs/design-gaps.md`).
* **No file picker** for merchant documents, and **no background location** for
  riders. Both are platform work, both are recorded in the same place.
* **The eight designed screens with no backend** — Offers, Promos, Referral,
  Payment methods, Safety, Permissions — say "not available yet". They are
  honest placeholders, not bugs.
