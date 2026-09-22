# Shop reviews — customer

`ShopReviewsScreen` ·
`frontend/customer/lib/src/screens/shop_reviews_screen.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To show what other customers said about a shop, and the rating it is actually
judged on.

## How you get here

From the **reviews** row on [the shop screen](shop.md).

## What you see

* The shop's name.
* The average and the number of reviews behind it, from
  `GET /v1/ratings?subject=merchant&subject_id=…`.
* A list of reviews: the rating, the comment when there is one, and when it
  was left.

**The average is not computed here.** It comes from the server, over every
review that exists — not averaged from the page of reviews on screen. Those
two numbers would begin to differ the moment the list was paged, and the one a
customer should see is the one the shop is judged on.

## Actions, and where they go

| Action | Where you go |
|---|---|
| back | [the shop screen](shop.md) |

There is nothing to tap in the list. Leaving a review happens from a finished
order, not from here — P16 only accepts a review from somebody who was on the
order.

## States

| State | What you see |
|---|---|
| loading | a spinner |
| reviews | the list, under the average |
| no reviews yet | "no reviews yet", and "not rated yet" in place of the average |
| served from cache | the saved-information banner |

## When something goes wrong

* **Either read failed** — the error view with the server's message and a
  retry. The rating and the list are separate reads, and a failure in one does
  not blank the other.
* **No connection** — the cached copy with the banner, or the offline message
  if this shop's reviews have never been read.
