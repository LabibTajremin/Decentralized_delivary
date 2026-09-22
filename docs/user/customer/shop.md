# Shop — customer

`ShopScreen` · `frontend/customer/lib/src/screens/shop_screen.dart`
· Figma `03__Shop Details` (`1:8113`, and its pharmacy and grocery twins)

## Purpose

To show one shop's whole menu in one screen, so that choosing something takes
one round trip rather than three.

## How you get here

By tapping a shop card on [Home](home.md).

## What you see

* The shop's name as the title, with its area and distance beneath.
* A **reviews** row, showing the shop's rating.
* The menu, `GET /v1/catalogue/{merchantId}/menu`: one section per category,
  in the shop's own order, with an item row each — name, the server's
  formatted price, and a line saying why an item cannot be ordered when it
  cannot.
* A **bundles** section, but only if this shop has any.

**One screen for all three verticals.** The design draws it three times, for
restaurant, grocery and pharmacy, because the *items* differ. The screen does
not, because nothing about the screen does: a hidden section takes its items
with it, and whether an item is orderable is a flag the server set.

## Actions, and where they go

| Action | Where you go |
|---|---|
| tap an orderable item | [the item screen](item.md) |
| tap an item that is not orderable | nowhere — the row is not tappable, and says why |
| tap **reviews** | [the shop's reviews](shop-reviews.md) |
| back | [Home](home.md) |

## States

| State | What you see |
|---|---|
| loading | a spinner |
| a menu | the sections and items |
| an empty menu | "nothing to show" — a shop that is approved but has not stocked anything yet |
| an item out of stock | the row is dimmed, with the server's reason: out of stock, not available now, or unavailable |
| served from cache | the "showing saved information" banner |

Prices are printed, never computed. The screen has no arithmetic in it at all.

## When something goes wrong

* **The menu failed to load** — the error view with the server's message and a
  retry.
* **No connection** — the last menu read for this shop is shown from cache
  under the saved-information banner, which is what makes a menu browsable on
  a train.
* **The shop was suspended between Home and here** — the menu read answers
  `404`, and the error view says so rather than showing a stale menu you could
  order from.
