# Place order — customer

`PlaceOrderScreen` · `frontend/customer/lib/src/screens/place_order_screen.dart`
· Figma `08__Place Order` and `09__Place Order Active` (`1:10282`, `1:10457`)

Two frames, one screen: the second is its in-flight state.

## Purpose

To confirm the destination, choose how to pay, and place the order.

## How you get here

From **place order** on [the review-cart screen](review-cart.md), carrying the
address-bound cart.

## What you see

* **Delivery address** — the chosen one, as the server renders it.
* **Payment method** — two choices: **cash on delivery** and **pay online**.
* The receipt, as the cart last returned it.
* **Place order.**

**Two methods, because P13 has two.** There are no saved cards and no wallet.
The design draws frames for both; `docs/design-gaps.md` records them.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| choose a payment method | local until you place | — |
| **place order** | `POST /v1/orders` with the address, the method and an idempotency key | [the order screen](order.md) for the new order |

**The idempotency key is generated once, when the screen is built, and reused
for every attempt from it.** That is the whole defence against a double tap on
a slow connection: the server returns the order it already made rather than
making a second one. A key generated per tap would defend against nothing.

## States

| State | What you see |
|---|---|
| ready | the button is live |
| placing | the button is busy and cannot be tapped again — the design's "active" frame |
| refused | the server's message above the button, and the button live again |

## When something goes wrong

* **The cart stopped being orderable** — the shop closed, an item sold out, or
  the total fell below the minimum between screens. The server refuses with
  `409` and its own sentence, which is shown; go back to the cart to fix it.
* **A cash order is over the area's COD limit** — refused, with the limit
  stated in the server's words. Choosing **pay online** is the way through,
  and the limit is a per-area config value rather than anything in the app.
* **The address is in another division** — refused on D3. Not something any
  retry or setting will change.
* **The tap was sent twice** — you get one order. That is the idempotency key
  doing its job, and it is proven end to end in the E2E suite.
* **No connection** — the offline message. Nothing is queued: placing an order
  whose price nobody has confirmed is exactly what must not be replayed later.
