# Board — merchant

`BoardScreen` · `frontend/merchant/lib/src/screens/board_screen.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To be the screen a kitchen actually works from: the orders that need
something done, and the one thing to do to each.

## How you get here

The first tab of the [merchant shell](main-shell.md). It is what an owner sees
on opening the app.

## What you see

Two tabs — **current** and **past** — and a card per order:

* The order's code and the server's status line.
* How many items, and the total as the server formatted it.
* Whether it is **paid online** or **pay on delivery** — which is what tells a
  kitchen whether cash is coming.
* One or more action buttons: **accept**, **reject**, **start preparing**,
  **mark ready**.

**Every button on this board comes from `next_actions`** — the order state
machine's answer for this shop, right now. A board holding its own copy of the
transition table would offer Accept on an order the customer cancelled a
second ago, and the server would refuse it in front of the shopkeeper.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| **accept** | `POST /v1/merchants/{id}/orders/{orderId}/accept` | stays here; the card redraws with its next action |
| **start preparing** | `.../preparing` | stays here |
| **mark ready** | `.../ready` | stays here. **This is the moment dispatch offers the job to a rider** |
| **reject** | — | [the order screen](order.md), where a reason is typed — rejecting needs one, and it is shown to the customer |
| tap a card | — | [the order screen](order.md) |
| switch tab | re-reads with the other filter | — |

## States

| State | What you see |
|---|---|
| loading | a spinner |
| orders | the cards, oldest-first on the current tab — a kitchen works a queue |
| nothing to do | "no orders" |
| an action in flight | that card's buttons are disabled |
| refused | the server's message above the list, and the board reloaded |
| served from cache | the saved-information banner. Buttons follow the cached `next_actions`; a stale one is refused server-side rather than half-applied |

## When something goes wrong

* **The action was refused** — the customer cancelled, or somebody on another
  device already did it. The server's sentence appears and the board reloads,
  so the button that is no longer valid disappears.
* **The read failed** — error view with a retry; the previous cards stay
  visible underneath.
* **No connection** — the cached board under the banner. Actions are not
  queued: a shop cannot promise food it has not seen the order for, and a
  queued "ready" arriving late would put a rider outside a shop that had not
  started cooking.
