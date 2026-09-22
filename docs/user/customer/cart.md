# Cart — customer

`CartScreen` · `frontend/customer/lib/src/screens/cart_screen.dart`
· Figma `05__View Cart` (`1:8568`)

## Purpose

To show what is in the cart, what it currently costs, and whether it can be
ordered.

## How you get here

The second tab of the [main shell](main-shell.md). Also reached by adding
something from [the item screen](item.md), which reloads it.

## What you see

* The shop's name — the cart holds one shop at a time.
* A row per line: its name, the options chosen, a quantity stepper, and the
  server's formatted line total.
* The receipt: every row the server composed — subtotal, delivery, any
  surcharge or waiver — each with its own label and amount, and the total.
* When the cart cannot be ordered: the server's sentence saying why.
* **Review cart.**

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| stepper up or down | `PUT /v1/cart/lines/{lineId}`; the whole cart, receipt included, redraws from the response | — |
| set a quantity to zero, or tap remove | `DELETE /v1/cart/lines/{lineId}` | — |
| **review cart** | — | [review cart](review-cart.md) |

**Every change round-trips.** The cart is revalidated against the live shop on
every read and every write — a price can move, an item can sell out, the shop
can close — so the quantity on screen is the one the server just confirmed
rather than an optimistic local count that a rejected change would leave
wrong.

**The checkout button reads `orderable` and nothing else.** That one flag
folds in the minimum order value, the shop's hours, its holiday mode, stock,
whether an address is set, and D3's division rule. An app that tried to infer
it would get a different answer from the one checkout enforces.

## States

| State | What you see |
|---|---|
| loading | a spinner |
| lines | the rows and the receipt |
| empty | "your cart is empty" — the review button is gone |
| not orderable | the button is disabled and `blocker_text` says why, in the server's words |
| a change in flight | the steppers are disabled until the response lands |
| served from cache | the saved-information banner. The button is still driven by the cached `orderable`, and the next write revalidates |

## When something goes wrong

* **A change was refused** — the server's message appears above the receipt
  and the cart is redrawn from what the server actually holds, so the screen
  never shows a quantity the server rejected.
* **The shop closed while you were looking** — the next read comes back not
  orderable, with the reason. Nothing is silently removed.
* **An item sold out** — the same: the cart comes back with the blocker text
  explaining it.
* **No connection** — the cached cart with the banner. Steppers still work
  visually only after a successful write, so nothing can be queued into a
  wrong total.
