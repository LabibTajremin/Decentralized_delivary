# Payment

> P13. Cash on delivery is already the path an order takes end to end; this
> phase adds the rest — a gateway checkout, its webhook, and the COD ledger a
> rider carries between a delivery and a reconciliation. Depends on P11
> (order) and P12 (dispatch, for a partner's own ledger).

## Two nouns

A **payment** is one gateway attempt to collect money online: pending,
captured, failed, or refunded. A **collection** is cash a partner is
physically holding after a cash-on-delivery drop-off, held until a
reconciliation remits it. They do not share a lifecycle or a table, because
they do not share a failure mode — a gateway payment can be declined before
any money moves; a cash collection already happened the moment the rider
recorded it, and the only question left is whether it has been handed in.

## 2.6 — the strictest boundary in the system

This module is Go today and a plan to move to a separate .NET service
tomorrow, so `PaymentContract` is built entirely from primitives —
`contract.Money` duplicates `shared/money`'s shape rather than aliasing it,
and every method takes and returns strings, `int64`s, and its own local
structs. No other module's package is imported into `payment/domain` or
`payment/contract`; `TestPaymentContractIsSelfContained` and
`TestPaymentDomainBorrowsNoOtherModulesTypes`
(`backend/tests/unit/architecture_test.go`) hold the line, each proven against
a deliberately reintroduced violation before being trusted.

The same rule reaches into payment's own `external/` seams, which is the
opposite of how every other module's seams are written. Elsewhere in this
codebase, a consuming module's seam interface is satisfied by the target
module's own concrete `Service` type directly, with no adapter, whenever the
signatures already match — see `order/external/dispatch` and, new this
phase, `payment/external/dispatch` and `order/external/payment` (both bare
interfaces, `dispatchapp.Service` and `paymentapp.Service` satisfying them
with zero glue code). `payment/external/order` is the one seam in this phase
that does **not** take that shortcut: it copies every field off
`ordercontract.OrderContract`'s `Order` into a locally owned
`orderx.Order` struct, on purpose, so that the day payment moves to another
process, nothing about that move requires touching a type any other module
declared. `TestTheOrderSeamCopiesFieldsRatherThanAliasing`
(`backend/tests/unit/payment/seams_test.go`) is the regression test for that
choice.

## Checkout and the webhook

`CheckoutUseCase.Start` is idempotent on the order id: a resumed checkout
before the first attempt is answered returns the same payment rather than
asking the gateway twice, backed by a partial unique index
(`WHERE status = 'pending'`) rather than a check that races with itself. A
losing writer reads back the attempt that won and returns it — the same
pattern P12's job-creation race uses.

A payment can only be started for an order that is `pending_payment`, paid
`online`, owned by the caller, and has something to collect — a zero total is
never something a checkout is legitimately for.

The webhook (`POST /v1/payments/manual/webhook`) is the one route in this
system with no identity guard at all (2.7 does not apply to a caller that is
not a person): it authenticates the payload's own HMAC-SHA256 signature
instead. It is idempotent by construction rather than by a dedup table — a
replayed capture finds the payment already `captured` and `Capture` itself
refuses the second call, which the webhook use case treats as the answer, not
an error; a late failure arriving after a capture is refused the same way, so
good news, once recorded, cannot be undone by a stale retry. The gateway's
confirmed amount is checked against what was asked for
(`domain.ErrAmountMismatch`) before anything is captured. A genuine race —
two webhooks reading the same pending payment before either writes — is
caught by `Save`'s own compare-and-set, and the loser's outcome is silence,
not an error, because the winner already did the one thing that mattered.

## Simulate — demoing without a real gateway

`WebhookUseCase.Simulate` runs the exact same `apply()` code path a real
webhook uses, skipping only the network hop and the signature check — so
demoing a capture exercises the real webhook logic, not a shortcut around it.
It is reachable only through `POST /v1/payments/manual/complete`, which
exists behind two independent guards: the route itself is never mounted when
`cfg.IsProduction()` (`transport/http/handler.go`'s `devTools` flag, wired
from `cmd/api/main.go`), and `Simulate` separately refuses to run against any
gateway whose `Name()` is not `"manual"`. Either guard alone would be enough;
both exist because a route that vanished from wiring is not the only thing
that should stand between a demo tool and a production deployment.

## The adapter — 2.4

`infrastructure/gateway/manual.Gateway` is the one gateway this phase ships,
and it refuses to construct at all when told it is running in production
(`New(secret, production bool)`, mirroring `identity/infrastructure/sms.LogSender`'s
refusal to be the SMS sender for a real deployment). `ports.Gateway` is the
seam a real processor (SSLCommerz, bKash, or whichever the business picks)
is built behind later, without touching a use case.

`PAYMENT_WEBHOOK_SECRET` follows the same production rules as
`JWT_SIGNING_KEY`: required outside development, rejected if it is still the
checked-in dev value, rejected under 32 characters
(`internal/shared/config/config.go`).

## Cash collection and the COD ledger

`RecordCashCollection` is order's delivery hook — `order/application/transitions.go`'s
`tellPayment` calls it the moment a cash order reaches `delivered`, using the
partner id already recorded on that transition's own event
(`event.ActorID`) rather than a second lookup, and the order's frozen total
(2.9's money type) as the amount. It is idempotent on the order id, guarded
by `ErrCollectionExists`: a retried delivery event records the same
collection once.

A rider's own ledger (`GET /v1/partner/cod`) is scoped through a new,
narrowly targeted addition to an already-finished module —
`DispatchContract.PartnerOfUser` — rather than a second identity lookup
invented here; the same principle P10 and P11 used when a genuine downstream
need arose against an earlier contract.

**Reconcile is the one genuinely atomic operation in this phase.** An
operator remits a batch of collection ids for one partner in a single
`pgx.Tx`: every collection either moves to `remitted` together or none of
them do. An earlier design called `domain.Collection.Remit()` and
`repo.Save()` once per id in a loop; it was redesigned before it ever
shipped, because a batch where item three fails validation would leave items
one and two already written — an operator staring at a partial remittance
with no way to tell which succeeded. `ports.CollectionRepository.Remit`
carries the whole operation, checked row by row inside one transaction
(`WHERE id=$1 AND status=$2`, rolled back on any zero-`RowsAffected()` with a
single `errs.KindConflict` — "collection_not_remittable" — answer), so
`application/collection.go`'s `Reconcile` has nothing left to coordinate.

## Everything a screen shows is composed here

Status labels for both a payment (`pending`/`captured`/`failed`/`refunded`)
and a held collection are composed server-side in both languages
(`application/view.go`), Bengali-first (1.4) with `?lang=en`, verified by
walking a real lifecycle through each status rather than calling the label
function directly — a screen only ever sees a label already attached to a
view.

## What P13's tests found

- **A per-item remit loop was a correctness bug caught before it ever became a
  failing test.** See "Cash collection and the COD ledger" above — the fix is
  `repo.Remit`'s single transaction, not a retry or a compensating write.
- **A refund could have moved money before validating the request.** An early
  draft of `RefundUseCase.Refund` called the gateway before checking the
  reason was non-empty. Reordered so the reason check, then `domain.Refund`'s
  own validation, both run before the gateway is ever called — money only
  moves once nothing else can still say no.
- **Postgres transaction poisoning (SQLSTATE 25P02) shaped one integration
  test's structure, not its coverage.** A test that induced a real unique
  violation and then tried to read again inside the same `pgx.Tx` failed for
  a reason that had nothing to do with the code under test — Postgres
  poisons a transaction after any error inside it. Split into two separate
  transactions, one per assertion, matching the precedent already set in
  dispatch's own integration suite.
- **The structural-typing shortcut used everywhere else in this codebase does
  not apply uniformly to one module's seams.** Working out, seam by seam,
  whether a target's existing method signature already matched what a
  consumer needed (`payment/external/dispatch`, `order/external/payment` — yes)
  versus needed real translation (`payment/external/order` — no, and must
  not, per 2.6) is now written down both in code comments and in two new
  architecture tests, so the next module that touches payment does not have
  to rediscover the distinction.

## What payment does not do

It never guesses at a fact another module owns. It does not decide an order
is paid — `MarkPaid`/`MarkPaymentFailed` are order's own transition, called
through `payment/external/order` the same way dispatch calls `Advance`
rather than writing an order's status directly. It does not resolve who a
partner is on its own — that is `DispatchContract.PartnerOfUser`. And an
outage in either seam does not block the payment or collection itself: a
webhook that captures a payment but cannot reach order returns an error for
the caller to retry, while the payment's own captured state — the thing that
actually matters, the money — already stands.
