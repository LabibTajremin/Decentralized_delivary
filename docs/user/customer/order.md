# Order — customer

`OrderScreen` · `frontend/customer/lib/src/screens/order_screen.dart`
· no single Figma frame; assembled from the order frames

## Purpose

To show one order — where it has got to, what it cost, and whatever can still
be done to it.

## How you get here

* Straight after placing one, from [place order](place-order.md).
* By tapping a row on [Orders](orders.md).

## What you see

* The status, as the server's own line, with the order's code.
* **Timeline** — every event so far, each with the label the server wrote, who
  did it, and when. A rejection or cancellation shows its reason.
* **Delivery address** — the server's one-line rendering.
* The lines, and the receipt exactly as it was frozen when the order was
  placed.
* Buttons, but only the ones that currently apply.

## Actions, and where they go

| Action | When it appears | Where you go |
|---|---|---|
| **track order** | while the order is live | [tracking](tracking.md) |
| **pay now** | while an online order is waiting for payment | [payment](payment.md) |
| **cancel order** | while `cancellation.allowed` is true | stays here; the order is re-read |
| **leave a review** | once delivered | [review](review.md) |
| **get help** | always | [support](support.md), with this order attached |

**Two flags decide this screen and neither is inferred.** `next_actions` is
the set of transitions the server will currently accept for this caller, and
`cancellation.allowed` is whether the free-cancellation window is still open —
with `cancellation.text` already written for when it is not.

The countdown is *displayed*, not enforced. When it runs out the screen
re-reads the endpoint rather than deciding for itself that the window has
shut, because the server's clock is the one that counts.

## States

| State | What you see |
|---|---|
| loading | a spinner |
| live | the status line, the timeline, and the actions that apply |
| waiting for payment | the receipt, and **pay now** |
| delivered | the full timeline and **leave a review** |
| cancelled or rejected | the reason, in the words of whoever gave it |
| cancelling | the button is busy |
| served from cache | the saved-information banner. The buttons follow the cached `next_actions`, and a stale action is refused by the server rather than half-performed |

## When something goes wrong

* **Cancelling was refused** — usually because the window closed or the shop
  already accepted. The server's sentence appears and the order is re-read, so
  the button disappears rather than staying there to be tapped again.
* **The read failed** — error view with a retry.
* **No connection** — the cached order under the banner.
