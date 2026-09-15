# Cart

> P09. One customer, one shop, one delivery address. Depends on P07 (catalogue)
> and P08 (discovery).

## Single merchant, and why it is a column rather than a check

A cart names its shop, and every line belongs to the cart. There is no shape the
data can take that breaks the rule — which is stronger than a `CHECK`
constraint, because a constraint can be dropped and a column cannot be
misinterpreted.

The reason is not convenience. An order is collected by one rider from one
counter; a cart spanning two shops is two orders wearing one checkout button,
and the customer discovers that when half of it arrives.

Adding from a second shop is **refused**, not resolved. Emptying somebody's cart
is a destructive act and it is theirs to take — `POST /v1/cart/replace` is how
they take it deliberately.

## Every read revalidates

A cart is never returned from storage as it was stored. `Current` re-asks:

| Question | Asked of |
|---|---|
| Is the shop listed, and open right now? | `merchant` |
| What does each line cost now, and is it still orderable? | `catalogue`, one batched read |
| Can this address still order from this shop? | `discovery` (`Reach`) |

Three calls regardless of how many lines there are. Combos are the exception —
there is no batch form for them — and a cart rarely holds more than one or two.

### What changed is reported, never fixed

This is the shape of the whole module.

- A price that moved leaves the line **orderable, at the new price, marked
  `price_changed`**. The customer may well still want it; what they must not get
  is one number on the list and a different one at checkout.
- An item the shop deleted stays in the cart, marked `removed`, contributing
  nothing to the subtotal. Dropping it silently would leave the customer at a
  total they cannot account for.
- A variant the customer chose that has since gone makes the line
  **unavailable** rather than repriced without it. Repricing would be quietly
  changing their order.

A combo that has vanished reads as `removed` rather than as an outage: the
answer to the customer is the same either way, and failing the whole cart read
because one line disappeared would take the rest of a good cart with it.

## Blockers, in the order a customer cares about

`Decide` picks one:

```
empty → shop_unavailable → no_address → outside_division / beyond_radius
      → shop_closed → line_problems → (orderable)
```

D3 outranks "closed for the evening" because it is permanent. There is no point
telling somebody that one of their eight lines is out of stock when the address
is in another division and none of it can be delivered.

`orderable` is the single answer checkout branches on. A client deriving it from
the line issues would be deciding eligibility, which 2.9 forbids outright.

## Cart invalidation

`PUT /v1/cart/address` is the operation the phase's second acceptance criterion
turns on. The next read re-asks discovery, so a customer who moves their
delivery address into another division finds out while the cart is still small
rather than at checkout — and the cart survives the block, so switching back
lifts it.

## Money

The cart computes the **goods subtotal** and stops there. Delivery, surcharges
and the grand total belong to pricing (P10): a cart that worked out the fee
itself would be a second implementation of ALG-05, and the second one is always
the one that is wrong.

Arithmetic inside the cart sums minor units directly rather than threading an
error through every caller. A cart is single-currency by construction — every
amount in it was built with `money.Taka`, from a catalogue that prices in taka
only. The day a second currency exists, this is one of the places that has to
change, which is better than a branch nobody ever exercised.

## Adding: ids in, prices out

The request carries ids only — an item and the option ids the customer chose.
The name, the price and whether that combination is even allowed are read from
the catalogue by the server. A request that carried a price would be a request
that could carry a different one.

The group rules (`required`, `min_choices`, `max_choices`) belong to catalogue
and are read from there rather than restated. What happens in the cart is
enforcement at the moment it matters: an item added without its required size
is an order the kitchen cannot make, and finding that out at checkout is finding
it out too late.

One subtlety worth stating: an optional group's minimum applies to a customer
who is *using* it, not to one who ignored it. A "choose at least two toppings"
group must not force toppings onto somebody who wanted none.

## Storage

Three tables (`0007_cart`), and one index that carries a rule: `carts_one_per_user`
is unique. Two open carts is how a customer ends up ordering from a shop they
had forgotten they were in the middle of.

`Save` deletes and rewrites the lines rather than diffing them. A cart is small,
the whole of it changed together, and a diff would be a second implementation of
"what the cart now is" that could disagree with the first.

Two foreign keys are deliberately absent:

- **to `user_profiles`** — a profile row is only written when a customer sets a
  name, and most never do. A key here would refuse a cart to anyone who skipped
  an optional screen.
- **from `cart_lines` to `catalogue_items`** — a line must outlive the item, or
  the cart cannot say "the shop has removed this".

## Contract

`CartContract` is consumed by order. It carries `Current` — the revalidated
cart, at today's prices, with the delivery point it was held against — and
`Clear`, so order asks rather than reaching into the cart's tables.

Order re-checks `Orderable` rather than trusting a client that claims it may
check out: an endpoint that took "this cart is fine" from the caller is an
endpoint that will be told so.
