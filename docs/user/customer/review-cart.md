# Review cart — customer

`ReviewCartScreen` · `frontend/customer/lib/src/screens/review_cart_screen.dart`
· Figma `07__Review Cart` (`1:10108`)

## Purpose

To choose where the order is going, which is what makes the bill real.

## How you get here

From **review cart** on [the cart screen](cart.md).

## What you see

* **Delivery address** — every saved address, each showing its label and the
  server's own one-line rendering. The current choice has a filled radio.
* The cart's lines again, in brief.
* The receipt, redrawn from the cart that came back after the address was
  bound.
* **Place order.**

On opening, one address is chosen for you: the one already on the cart if
there is one, otherwise the default, otherwise the first.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| tap an address | `PUT /v1/cart/address`; the receipt below redraws from the cart returned | — |
| **place order** | — | [place order](place-order.md), carrying the cart and the chosen address |

**Why choosing an address is a request.** Until the cart has a destination
there is no distance, and without a distance there is no delivery fee to
quote. So the choice goes to the server and the receipt comes back — the app
never adjusts a figure itself.

## States

| State | What you see |
|---|---|
| loading addresses | a spinner |
| no saved address | "you have no saved addresses" — there is nothing to deliver to, and the place-order button stays disabled |
| binding an address | the rows are not tappable and the button is disabled until the response lands |
| bound and orderable | the button is live |
| bound and not orderable | the button is disabled, with `blocker_text` beneath — most often because this address is in a different division from the shop (D3) |

## When something goes wrong

* **The address cannot be delivered to from this shop** — the server refuses
  the binding, its sentence appears, and the receipt keeps showing the last
  state that was real. D3 is the usual cause and it is not negotiable: a
  delivery across a division boundary is refused by the system, not by a
  setting.
* **Binding failed for another reason** — the message appears in the list and
  the previous choice stays selected.
* **No connection** — the offline message. The address cannot be bound, so the
  place-order button stays disabled rather than letting an order be placed
  against a bill nobody quoted.
