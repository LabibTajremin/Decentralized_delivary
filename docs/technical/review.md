# Review

> P16. Ratings for the three things a customer's opinion can be about —
> merchant, delivery partner, item — and the support ticket queue a refund
> decision goes through. Depends on P11 (order) for eligibility, P13
> (payment) for the refund a resolved ticket may trigger.

## Three subjects, not one review per order

An order's outcome is really three separate opinions: was the shop good, was
the rider good, was the thing itself good. Collapsing them into a single
"how was your order" rating would tell a shop nothing about whether a late
delivery was its fault or the rider's — so `domain.Subject` is one of
`merchant`, `partner` or `item`, and a customer rates each independently.

## Eligibility, from order alone

Appendix A scopes `ReviewContract` deliberately: it depends on
`OrderContract` and nothing else — not `DispatchContract`, not
`CatalogueContract`. Every eligibility check `SubmitReviewUseCase` makes is
built from what an order already carries:

- **The rater must be the order's own customer.** `ord.CustomerID` against
  the authenticated caller.
- **The order must be delivered.** No reviewing a shop before the food
  arrives.
- **The subject must be something this specific order actually involved.**
  A merchant review's subject id is checked against `ord.MerchantID`; an
  item review's against `ord.Lines[].ItemID`; a partner review's against the
  `ActorID` on the order's own `"delivered"` event.

The last two needed two small, targeted additions to `order/contract`:
`Line` gained `ItemID` and `Event` gained `ActorID` — the same "the
consumer's own dependency is enough, add one field rather than a second
cross-module seam" precedent P13 and P14 established (`PartnerOfUser`,
`PhoneFor`). Without `ActorID`, review would have needed `DispatchContract`
just to answer "who delivered this," which Appendix A does not grant it.

One review per rater per subject per order is enforced at the use case
(`ExistsForRater`) and backed by a database `UNIQUE` constraint — the same
belt-and-suspenders pattern used everywhere idempotency matters in this
codebase.

## Support tickets and the refund workflow

`payment/contract/contract.go`'s own doc comment already named this before
P16 existed: *"Consumed by: order..., review/support (P16's refund
workflow)."* `ResolveTicketUseCase` is that workflow: an agent decides
`refunded` or `rejected`, and only `refunded` calls through to
`PaymentContract.Refund` — via `review/external/payment`, a bare interface
satisfied structurally by `payment/application.Service`, no adapter, the
established pattern.

The refund happens **before** the ticket is saved as resolved:

```go
if resolution == domain.ResolutionRefunded {
    if err := uc.payment.Refund(ctx, ticket.OrderID, "support ticket "+ticket.ID); err != nil {
        return domain.Ticket{}, refundError(err)
    }
}
if err := uc.repo.SaveTicket(ctx, ticket); err != nil { ... }
```

A ticket recorded as "refunded" whose refund never actually went through
would tell a customer money is coming that will never arrive — worse than
the resolution simply failing and the agent trying again. `Refund` is
idempotent on the order id (P13), so a resolution that fails after the
refund succeeded and is retried does not ask the gateway to give the money
back twice.

**A COD order has no gateway payment to refund.** `PaymentContract.Refund`
looks up the order's most recent gateway attempt; a cash order never has
one, so the call fails and the ticket's resolution surfaces `refund_failed`
(503) rather than silently succeeding. This phase does not build a
cash-specific refund path — an agent resolving a cash order's complaint has
`rejected` and whatever process already exists outside this API for
reconciling cash, the same way P13 left ALG-09-adjacent problems it had no
signal for to a later phase.

## Why ratings and reviews are read-authenticated, not public

Unlike a shop's menu or discovery's search — public because browsing is
what happens before anyone signs in — reading a subject's rating or reviews
sits behind `authenticator.Authenticated()`. There is no P16 requirement
either way; keeping every route behind one guard type kept the module's
authorization surface to two shapes (authenticated, admin-only) instead of
three, and nothing in this phase depends on an anonymous read. A later phase
can open the two GET routes if a public reviews screen needs it — the
handlers already read their required parameters before touching a use case
either way, so the change would be one line in `routes()`.

## API

| Route | Purpose |
|---|---|
| `POST /v1/reviews` | Submit a review, checked against the order it names |
| `GET /v1/reviews` | A subject's own reviews, newest first |
| `GET /v1/ratings` | A subject's aggregate rating — mean and count |
| `POST /v1/support/tickets` | Raise a ticket about an order |
| `GET /v1/me/support/tickets` | A customer's own tickets |
| `GET /v1/admin/support/tickets` | The queue: tickets nobody has resolved yet |
| `POST /v1/admin/support/tickets/resolve` | Resolve a ticket, refunding through payment when warranted |

`ReviewContract` (`MerchantRating`, `PartnerRating`) is built and unit-tested
to 100%, ready for merchant's and dispatch's own screens to consume per
Appendix A — wiring it into their transport responses is left to whichever
phase builds the screen that shows it, the same way P14's `TrackingContract`
existed before P18 had a screen to put it on.
