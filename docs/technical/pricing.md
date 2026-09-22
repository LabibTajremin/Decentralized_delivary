# Pricing

> P10. `ALG-05` — the delivery fee — and nothing else. Depends on P03.

## One implementation, deliberately

Discovery quotes a fee on every shop card. The cart quotes one at checkout.
Order will freeze one. Before this phase, discovery carried a provisional copy
of the arithmetic (`discovery/infrastructure/fees`, introduced in P08 and
deleted here) because it needed a number on a card and pricing did not exist
yet.

Two implementations of a price are two prices, and the customer sees both — the
fee on the shop card and the fee on the receipt. Everything now goes through
`pricing/domain`.

The module has **no HTTP surface**. A price is never asked for on its own; it is
asked for *about* something, so the module that owns that thing quotes it and
returns it with the rest of the answer. It also owns **no storage**: the rule is
in the domain and the numbers are in config.

## ALG-05

```
bands = ceil(distance_m / 1000)
fee   = (pricing.delivery_base + pricing.delivery_per_km × bands)
        × pricing.expansion_multiplier   ← only when the radius was widened
```

`O(1)`. Piecewise-linear in distance, with one step at each band edge.

**Banding rather than a continuous rate.** A fee that changes by a few poisha as
a phone's GPS drifts across a street looks broken, and a customer who saw ৳50 on
the shop card must be charged ৳50. One kilometre is coarse enough to be stable
and fine enough that a 9 km delivery does not cost the same as a 2 km one.

**Exact at the edges**, which is the phase's first acceptance criterion:

| Distance | Bands | Fee (Appendix B defaults) |
|---|---|---|
| 0 m | 0 | ৳40 |
| 1 m | 1 | ৳50 |
| 1000 m | 1 | ৳50 |
| 1001 m | 2 | ৳60 |
| 25 000 m | 25 | ৳290 |

The base fee covers the pickup; each kilometre adds. Any part of a kilometre is
a kilometre.

**The multiplier is applied to the whole fee**, not to the distance part alone.
Expansion costs the platform more than extra kilometres: it is a rider out of
their usual area, further from their next job, which the base fee covers as much
as the rate does. It is applied **once**, whatever the expansion level — a
customer four rungs out is already paying for the distance through the rate.

Rounding is to the nearest poisha rather than truncating, so a 1.5× multiplier
on an odd amount does not quietly favour the platform.

`NewTariff` refuses a multiplier below 1. Appendix B already bounds it there;
the reason is D2 — a multiplier below one would make a longer, dearer journey
cost the customer *less*, which is the opposite of the rule the expansion ladder
exists to enforce.

## Free delivery, and the D2 tension

`pricing.free_delivery_threshold` waives the fee above an order value. Two
decisions worth stating, because both could reasonably have gone the other way:

**It applies only at the base radius.** Letting it survive an expansion would
give a ৳500 order a free ride across the division, and D2 — "when the radius is
extended, the delivery charge increases" — would stop being true above the
threshold. Free delivery is a promotion on a *local* order. A shop that wants a
wider free radius raises `discovery.base_radius` for its area, which is the knob
that means what it says.

**A threshold of zero switches the promotion off** rather than making every
delivery free. Nobody configures "free delivery on orders over nothing", and
reading it the other way would give away every delivery in an area the moment
somebody cleared the field.

## The quote is a receipt

`Quote` carries the rows already ordered and already worded, because a client
that assembled a receipt would assemble it differently from the one the customer
gets afterwards (2.9):

```
subtotal → delivery → [expansion_surcharge] → [free_delivery] → total
```

Only the rows that say something appear. A receipt with a zero "expansion
surcharge" line on every local order trains people to stop reading it.

The **surcharge is derived**, not tracked: it is the difference between the
expanded fee and what the same journey would have cost unexpanded. Deriving it
is what makes it impossible for the two numbers to disagree. It comes back from
`DeliveryFee` alongside the fee rather than from a second call, so no caller
validates the distance twice and handles an error the first validation already
ruled out.

`AwayFromFreeDelivery` lets a client say "৳120 more for free delivery" without
holding the threshold itself, which 2.9 forbids outright.

## Tariff is a snapshot

`PricingContract.Tariff(ctx, placement)` returns a resolved snapshot, and the
fee and quote methods on it take no context. The same shape as config's
`Settings`, for the same reason: a search prices twenty shop cards, and if
pricing were asked per card that would be twenty configuration reads which could
in principle quote two shops against different settings.

## Consumers

| Module | Uses |
|---|---|
| `discovery` | one tariff per search page, `DeliveryFee` per shop card |
| `cart` | one tariff per cart read, `Quote` for the receipt |
| `order` (P11) | re-quotes authoritatively at placement |

The cart hands pricing three things it already has — the subtotal it computed,
and the distance and expansion level `DiscoveryContract.Reach` returned. That is
why `Reach` gained the resolved placement in this phase: the answer was already
in discovery's hand, and a second geo call for it would have been waste.

A cart with no address, or an address the shop cannot deliver to, is **not
priced at all**. A delivery charge for a journey that cannot happen is a number
the customer would reasonably take for a promise.
