# Item — customer

`ItemScreen` · `frontend/customer/lib/src/screens/item_screen.dart`
· Figma `03__More Details` / `04__Add to cart` (`1:11963`, `1:8519`)

## Purpose

To choose an item's options and quantity, and put it in the cart.

## How you get here

By tapping an orderable item on [the shop screen](shop.md).

## What you see

* The item's name, its description, and the server's formatted price.
* One block per option group the item has — variants like a size, add-ons like
  an extra. Each block is shaped by the group's own rules: single choice or
  several, required or not, with the maximum the group allows.
* A quantity stepper.
* A free-text **note to the shop**.
* **Add to cart.**

**Nothing here adds a price up.** The button says "add to cart"; what the line
costs appears in the cart the server returns. There is no running total on this
screen, because a running total is arithmetic on money in a client, which rule
2.9 forbids and which would eventually disagree with the receipt.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| choose an option | selection is kept locally until you add | — |
| stepper | changes the quantity | — |
| type a note | sent with the line | — |
| **add to cart** | `POST /v1/cart/items` | back to [the shop screen](shop.md), with the cart tab now reloaded |

The option rules shape the form and nothing more: `POST /v1/cart/items` checks
the same rules again. So an app that got the shaping wrong produces a rejected
request, not a bad cart.

## States

| State | What you see |
|---|---|
| a required group with nothing chosen | the add button is disabled |
| every requirement met | the add button is live |
| adding | the button is busy |
| refused | the server's message beneath the button; the selection is kept |

## When something goes wrong

* **A rule was broken** — the server's sentence appears. Because the form
  shapes itself from the same rules, this mostly happens when the item changed
  underneath you.
* **The item sold out while you were choosing** — the add is refused with the
  reason, and the item screen stays open so the note and options are not lost.
* **A different shop is already in the cart** — the cart holds one shop at a
  time; the server refuses and says so, which is the point at which a customer
  decides which of the two they want.
* **No connection** — the offline message. Adding to a cart is not queued: the
  cart is revalidated server-side on every write, and replaying a stale add
  would be guessing at a price.
