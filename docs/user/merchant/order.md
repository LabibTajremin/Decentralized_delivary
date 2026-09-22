# Order — merchant

`OrderScreen` · `frontend/merchant/lib/src/screens/order_screen.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To show one order in the detail a kitchen needs, and to reject one with a
reason.

## How you get here

By tapping a card on [the board](board.md), or by tapping **reject** there.

## What you see

* The status line, the code, and whether it is paid online or on delivery.
* **Order lines** — each item, its quantity, the options chosen, and the
  customer's note to the shop.
* **Destination** — where it is going, as the server renders it.
* The receipt, as it was frozen when the order was placed.
* The actions `next_actions` currently allows: **accept**, **start
  preparing**, **mark ready**, **reject**.
* With **reject**: a **reason** field.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| **accept**, **start preparing**, **mark ready** | the matching transition | stays here; the order re-reads |
| **reject** | `.../reject` with the typed reason | back to [the board](board.md) |
| back | — | [the board](board.md) |

**The rejection reason is the shop's own words**, typed here and shown to the
customer. It is not a code picked from a list, because the reason a shop
cannot fill an order is not something a list can anticipate — and the customer
deserves the real one.

## States

| State | What you see |
|---|---|
| loading | a spinner |
| the order | lines, destination, receipt, and whatever is allowed |
| rejecting with no reason | the reject button is disabled — the server requires one |
| an action in flight | the buttons are disabled |
| refused | the server's message above the actions, and the order re-read |
| no actions left | the order is finished, or somebody else moved it |

## When something goes wrong

* **The transition was refused** — the order moved under you. Its sentence
  says how, and the screen re-reads so the buttons match.
* **The reason was empty** — refused by the server; the button guards against
  it first.
* **Someone else's order** — `404`, the same answer an id that never existed
  gets.
* **No connection** — the cached order under the banner. Transitions are not
  queued, for the reason on [the board](board.md).
