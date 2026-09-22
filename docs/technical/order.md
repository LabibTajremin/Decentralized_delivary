# Order

> P11. The agreement between a customer and a shop. Depends on P09 (cart),
> P10 (pricing) and P05 (user).

## The lifecycle is one table

`domain/status.go` holds every legal move in a single map, and each move lists
who may make it:

```
pending_payment → placed (system) | cancelled (customer, admin, system)
placed          → accepted, rejected (merchant, admin) | cancelled (customer, admin)
accepted        → preparing (merchant) | rejected (merchant, admin) | cancelled (customer, admin)
preparing       → ready (merchant) | cancelled (admin)
ready           → picked_up (partner) | cancelled (admin)
picked_up       → delivered, failed (partner) | failed (admin)
delivered, cancelled, rejected, failed → nothing
```

**Who** is part of the table rather than a separate check, because "who may do
this" and "what may happen next" are the same question asked twice. A table that
answered only the second would let a customer mark their own order delivered.

The four terminal states have no rows at all — that is what makes them terminal:
there is nowhere to look a move up from.

`CanTransition` returns three distinguishable refusals, because they need three
different answers:

| Refusal | Means |
|---|---|
| `ErrAlreadyFinished` | a fact the customer can see for themselves |
| `ErrIllegalTransition` | the move does not exist from here |
| `ErrNotYourTransition` | somebody is asking for something they should not be able to ask for — worth a different log line |

`NextStatuses(status, actor)` is what the API returns as `next_actions`. The
server answers "what can I do now" rather than the client working it out:
deciding which actions a user may take is a backend decision (2.9), and a client
with its own copy of the table would offer a shop an Accept button on an order
somebody already cancelled.

### `rejected` is not `cancelled`

Two different facts about two different people. A shop that rejects often is a
shop an admin needs to see; a customer who cancels often is a different
conversation entirely.

## Everything on an order is a copy

The lines copy the menu. The destination copies the address book. The pickup
copies the shop. The charges copy what pricing said at the moment of writing.

An order is a record of an agreement, and an agreement that changed afterwards
was not an agreement:

- a shop renaming an item must not rename it on last week's receipt;
- a customer editing an address must not redirect a rider already on the road;
- an admin retuning an area's delivery rate must not change a receipt somebody
  has already paid.

The only live reference is `merchant_id`, and it has no foreign key: the shop
must stay resolvable for tracking, support and reviews long after anything else
about it has changed.

## Prices are read on the way in

The request carries an address id, a way to pay, and nothing else. What the
goods cost, how far it is, whether the shop is open and whether cash is allowed
at this amount are all read here:

| Question | Asked of |
|---|---|
| is this cart orderable, and what is in it | `cart` (revalidated, at today's prices) |
| may this address reach this shop, how far, at what expansion level | `discovery` (`Reach`) |
| what does it come to | `pricing` (`Tariff.Quote`) |
| is the shop taking orders | `merchant` |
| where is it going, and to whom | `user` |
| the COD ceiling and the cancellation window | `config`, **one snapshot** |

A price the customer was shown is not a price the system promised. The cart's
quote was composed for a screen, possibly minutes ago, by a request whose timing
the customer controls.

The two configuration values are read **together, before anything is written**.
Separately would let an area retuned between the two reads check an order
against one configuration and offer a countdown from another; afterwards would
mean a placed order could still fail to describe itself.

## Idempotency

A customer on a village 2G connection taps "Place order", sees nothing happen,
and taps again. `Idempotency-Key` (header or body) makes that one order: the
second request returns the first.

The key is stored in its own table with a composite primary key
`(customer_id, key)`. Scoped to the customer because two customers using the
same client library may generate the same key, and a global unique index would
hand one of them the other's order.

Without a key there is no protection. That is a real risk and the tests say so
out loud rather than leaving it implied.

## The cancellation window

Two questions, kept apart:

1. **Is a cancellation possible from here?** The transition table. Once the
   kitchen has started, no — somebody has already paid for this in ingredients.
2. **Is it still free?** `order.cancellation_window`, from Appendix B.

The window runs from **placement**, not acceptance. A customer changes their
mind about the decision *they* made, and a shop that takes six minutes to accept
should not thereby shorten — or silently extend — the time they had.

`GET /v1/orders/{id}/cancellation` is its own endpoint because a countdown needs
refreshing and a client must not learn the answer by trying it. `seconds_left`
comes from the server's clock, which is the one that decides.

An admin is not bound by the window. Clearing up after a customer who phoned in
is exactly what operations is for.

## Concurrency

`AppendTransition` puts the expected status in the `UPDATE`'s own `WHERE`
clause. Two riders tapping "picked up" at the same moment cannot both succeed:
the second updates no rows and is told the order moved under it.

The state machine says what *may* happen; the database says it happened *once*.

## Contract

`OrderContract` has four methods and deliberately no general "set status":

- `Order` — one order, for dispatch, tracking, payment and review.
- `MarkPaid` / `MarkPaymentFailed` — named, so payment cannot write any status.
- `Advance` — for a party that owns a later stage, with the state machine still
  applied. The actor may be `partner`, `admin` or `system` **only**: a
  customer's cancellation is bound by a window checked on their own path, and a
  shop's authority is "this shop is yours", which only the HTTP layer can
  establish from the merchant record.

The contract carries less than the screen does — no option prices, no customer
note. A rider needs to know they are carrying three biryanis, not what was paid
for the extra raita.

## Two bugs the tests found

Worth recording, because both were invisible to the layer above.

**Options vanished from orders.** `linesByOrder` held `*domain.Line` pointers
into a `map[string][]domain.Line` while appending to those slices. `append`
reallocates, so every pointer taken before a growth silently stopped referring
to the row it was taken from. The unit fakes could not see it; the integration
test against real Postgres did. Lines are now collected flat and grouped at the
end.

**The second order in a process failed.** `NewOrder` built the placement event
with no id, and `order_events.id` is a primary key — so the first order inserted
`''` and the second collided. The unit fakes did not enforce the key and the
integration test set an id by hand; only the E2E, placing two orders against one
server, hit it. `Draft.EventID` is now required, and `NewOrder` refuses a draft
without one.

## Wall-clock dependence in the tests

The E2E suite builds its own always-open shop rather than ordering from a seeded
one. The seed keeps realistic 09:00–22:00 hours, so a suite that used it would
pass in the afternoon and fail overnight — and a test whose result depends on
when it runs is a test nobody trusts the second time it goes red.
