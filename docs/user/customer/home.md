# Home — customer

`HomeScreen` · `frontend/customer/lib/src/screens/home_screen.dart`
· Figma `01__Home Screen` (`1:280`, `1:859`, `1:1401` — the same screen with
each type chip selected)

## Purpose

To show the shops that can actually deliver to you, nearest first, with the
delivery fee already quoted.

## How you get here

The first tab of the [main shell](main-shell.md). It is what a signed-in
customer sees on launch.

## What you see

* **Deliver to** — the label of the address being searched from, at the top.
* A search field.
* Four type chips: all, restaurant, grocery, pharmacy.
* The area name and the notice the server composed for this search.
* A card per shop: its name, its type, its area, how far away it is, and the
  delivery fee. All four of those are strings the server wrote.
* Sometimes, at the bottom: **search a wider area**.

**There is no device location, and that is deliberate.** Every figure on a
shop card — the distance, the fee, whether the shop is in range at all — is
computed by discovery from a *delivery point*, and the delivery point is an
address the customer saved, not wherever the handset is standing. Searching
from a GPS fix would quote a fee for a delivery to a street corner.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| type in the search field | re-runs `GET /v1/discovery/merchants` with the query | — |
| tap a type chip | re-runs the search filtered to that type | — |
| tap a shop card | — | [the shop's menu](shop.md) |
| **search a wider area** | re-runs the search at `expansion.next_level` | — |
| **add an address** (when there are none) | — | [add address](add-address.md), then back here with the new address selected |

**Widening is not a number the app picks.** It sends
`expansion.next_level`, only when `expansion.can_expand` is true, and it stops
offering the button when `expansion.at_ceiling` says the division boundary has
been reached. That boundary is D3, the one rule no setting can disable: the
app cannot widen past it and does not pretend it can.

## States

| State | What you see |
|---|---|
| loading addresses | a spinner |
| no saved address | "you have no saved addresses", and a button to add one — there is nothing to search from until there is one |
| searching | a spinner over the list |
| results | the cards, plus the server's notice |
| nothing nearby | "no shops here yet", with the widen button if widening is still possible |
| at the division ceiling | the widen button is gone. The notice explains why |
| served from cache | a "showing saved information" banner above the list |

## When something goes wrong

* **The search failed** — an error view with the server's message and a
  **try again** button. The previous results stay on screen underneath rather
  than being blanked.
* **No connection** — if this search has been made before, the cached response
  is shown with the "showing saved information" banner. If it has not, the
  offline message appears with a retry.
* **The address is outside every division** — cannot happen from here; the
  [add-address form](add-address.md) refuses such a point before it can be
  saved.
