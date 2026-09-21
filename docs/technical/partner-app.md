# The partner app

> P19. The rider's app: go on shift, take a job, collect it, deliver it, and
> settle the cash. This is the app `goklay_core`'s offline queue was built for.

## There were no frames for this either

Same as the merchant app, and recorded in the same place:
`docs/design-gaps.md`. The layout comes from the endpoints; the look comes
from the tokens P17 read out of Figma.

## The offline queue, finally used

P17 built `OfflineQueue` and said what it was for. This is it. A rider taps
"collected" at a counter with no signal and "delivered" in a stairwell with
none either, and those two taps must arrive **in that order or not at all** —
the order state machine refuses `delivered` from an order that never reached
`picked_up`.

So the job screen tells two failures apart and treats them differently:

* **Offline** — the request never reached the server. The tap is queued, the
  screen says so, and the queue replays strictly oldest-first, stopping at the
  first action the network still cannot carry.
* **Refused** — a `409` because another rider took the job first. That is
  shown, not queued: it will be refused just as firmly in an hour, and a queue
  that retried it would never drain.

The banner lives above the tabs rather than inside one, because a tap made on
the job screen and still unsent has to be visible from wherever the rider
happens to be. Signing out clears the outbox along with the tokens: one
rider's unsent taps must never replay under the next rider's token, which on a
shared handset is somebody else's delivery.

## What the screens read instead of deciding

| The screen wants to know | It reads | It does **not** |
|---|---|---|
| which jobs to show | `GET /v1/partner/feed` | filter a global list by a radius |
| why the feed is empty | `reason` + `notice` | guess between offline and nothing nearby |
| how far a job is | `distance` / `to_pickup` | compute a haversine or format a number |
| which distance band it is | `band_label` | know where D4's boundaries fall |
| whether the rider is on shift | `availability` / `availability_label` | infer it from what they are carrying |
| how much cash is owed | `Ledger.outstanding` | add the held collections up |

**`busy` is not something a rider declares.** It is what being at the
concurrent limit is called, and it resolves itself when they finish a
delivery. The app only ever sends `offline` or `available`; letting a rider
set `busy` by hand would give them a way to stay in the pool while refusing
every offer.

**D1 from the partner's side**: a rider may work anywhere in Bangladesh, so
sign-up asks for a name, a number and a vehicle, and nothing about geography.
Where they can work is decided every time they report a location.

## The ledger adds nothing up

Both totals are the server's, recomputed from the collections rather than
stored. The screen does not sum `held` to check. A rider and an office
disagreeing about how much cash is in a bag is exactly the argument a second
calculation causes.

## What is deliberately absent

**No background location.** The account screen reports a position when the
rider asks it to. Continuous location is a platform capability with battery
and permission consequences, and P19 did not invent one; the endpoint it would
call is already there and already used.

## Tests

100%, including an `OfflineBackend` whose every request fails the way a lost
connection fails — which is how the tests tell the queued path from the
refused one apart at all.
