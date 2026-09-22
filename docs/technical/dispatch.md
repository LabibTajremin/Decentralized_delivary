# Dispatch

> P12. Getting an order from a counter to a door. Depends on P11 (order),
> P02 (geo) and P03 (config).

## Two nouns

A **partner** is a person who carries orders. A **job** is one delivery.

They are deliberately not the order's own nouns. An order is the customer's view
of the thing; a job is the platform's view of moving it, and the two diverge: an
order sitting at `ready` may have been offered to four riders and declined by
three, none of which is a fact the customer needs.

## D1 from the partner's side

A merchant registers in one area and is approved against a map (D1). A partner
does neither: **a delivery partner may provide delivery anywhere in
Bangladesh**, so there is no area to register in and nothing to approve. Where
they can work is decided every time they report a location, by a spatial query
rather than by a row in a table.

## D4 — the distance choice

`preference` is `short`, `long` or `any`, and it is applied in the candidate
query's own `WHERE` clause rather than as a filter afterwards:

```sql
AND (preference = 'any' OR preference = $4 OR $4 = 'beyond')
```

A job's `band` is computed once, at creation, against that area's
`dispatch.short_distance_max` and `dispatch.long_distance_max`, and stored.
Stored rather than derived, so retuning a ceiling does not silently move live
jobs between riders' feeds while they are looking at them.

`beyond` — further than the long-distance ceiling — is offered to everybody. A
rider who said "short runs only" and is the only person within twenty kilometres
of a delivery that has to happen is still the answer.

## The job lifecycle

```
waiting  → offered (a round picked somebody)
offered  → assigned (accepted) | waiting (declined, or the clock ran out)
assigned → collected | waiting (gave up before collecting) | cancelled
collected → delivered | failed | cancelled
delivered, failed, cancelled → nothing
```

Two things about this are worth saying out loud.

**A decline is not a failure.** It puts the job back on the board with the
rider who passed recorded in `passed_by`, and the next round skips them — a
rider who has already said no is not asked the same question again while
somebody else is standing by. In a town with one rider they are asked again,
because a job nobody can ever be offered is worse than a repeated question.

**Giving up before collection is not a failed delivery.** The food is still on
the shop's counter; there is nothing to fail. The job goes back on the board and
the customer's order carries on. After collection the rider is holding somebody's
dinner, and `failed` means what it says — the order fails with them, with the
reason attached.

## ALG-04 — who gets asked

A min-heap over the candidates, scored low-is-better:

| Term | Weight | Why |
|---|---|---|
| proximity to the **pickup** | 0.55 | the rider has to reach the counter before anything else can happen |
| current load | 0.25 | separates the riders who can take another job |
| 1 − acceptance rate | 0.20 | a rider who declines everything makes every customer wait for the next round |

Distance is to the **shop**, not to the customer. A rider who is already outside
the counter is worth more than one who is nearer the door but forty minutes from
the food.

A heap rather than a sort because the caller almost never wants the whole order:
one offer goes out, the clock runs for `dispatch.assignment_timeout`, and by the
time the second candidate matters the pool has changed and a sorted tail would be
stale anyway.

Ties break on the partner id, so two rounds a second apart cannot offer the same
job to two different people.

## Offers are rounds, not broadcasts

One partner at a time, each with a clock. Broadcasting would be simpler and
would have five riders race to the same counter, four of whom wasted a trip —
which is how a platform loses riders.

The countdown a rider's screen shows is composed from the **server's** clock. A
phone clock is wrong often enough that a countdown from it would expire at the
wrong moment, which on an offer is the difference between a job and a wasted
trip.

## The sweep is the heartbeat

`POST /v1/admin/dispatch/sweep` does two passes:

1. **Expire** the offers nobody answered, and count each one against that
   rider's acceptance rate — to the customer a lapsed offer and a decline were
   the same thing: a rider who was asked and did not come.
2. **Offer** the jobs sitting on the board to somebody.

Both halves are needed. Without the first, a job stalls behind a rider who put
their phone in their pocket. Without the second, a declined job would sit
waiting forever, because a decline is the one way onto the board that no
partner's own action takes it off again.

It is an endpoint rather than a goroutine so the sweep runs where somebody can
see it: an operator can trigger it, a cron can call it, and it is safe to run
twice. A goroutine inside the API would run once per replica and be invisible
when it stopped.

This is why a job carries its own `area_code` / `district_code` /
`division_code`: the settings that govern it are per-area (D2), and the sweeper
that re-offers it ten minutes later has only the job to go on.

## Two partners cannot be assigned the same order

The phase's third acceptance criterion, guarded in three places that do not rely
on each other:

1. `delivery_jobs_order_idx`, a unique index on `order_id` — an order that
   becomes `ready` twice cannot become two jobs. The losing writer reads back
   the job that won and returns it, so a retry is idempotent rather than an
   error.
2. The compare-and-set in `SaveJob`'s own `WHERE status = $n` — two riders
   accepting in the same millisecond cannot both succeed.
3. `Job.heldBy` in the domain — a rider cannot move a job that is not theirs,
   whatever the database says.

The `delivery_jobs_holder` CHECK constraint is the fourth: a job that is
`waiting` has no partner and a job that is not cannot be without one, so no code
path can leave a row that contradicts itself.

## ALG-08 — the feed

Waiting jobs whose pickup is inside the partner's radius, ordered by distance to
the pickup, D4-filtered, capped at 20. A GiST index over `pickup_pin`, partial on
`status = 'waiting'`.

An empty feed is never an empty list on its own. `reason` is one of `offline`,
`at_capacity` or `nothing_nearby`, each with its own sentence — a rider waiting
for work deserves the right one, and "you are offline" and "there is nothing
here" are different facts about different problems.

## Everything the rider's screen shows is composed here

A partner app is used one-handed on a motorbike at a traffic light. Every
string — the status, the band, the distance, the countdown, the empty-feed
notice — is decided by the server and rendered by the app (2.9), Bengali-first
(1.4), with `?lang=en` for English. Distances are coarser than the customer's:
a rider deciding whether to take a job wants "৩ কিমি", not "২.৮ কিমি".

## What dispatch does not do

It never writes an order's status directly. `external/order` calls the order
module's `Advance`, so a rider tapping "delivered" on an order that was
cancelled underneath them is refused by the same transition table that refuses
everybody else. The three moves it makes are `picked_up`, `delivered` and
`failed`, and only the last of those is conditional — see the lifecycle above.

The order module calls dispatch the same way, through its own
`external/dispatch`: `ready` offers the job, and a cancellation withdraws it. A
dispatch outage does not block the shop — a shop that cannot tell a customer
their food is ready because a rider service is down is a worse failure than a
job that has to be swept onto the board a minute later.
