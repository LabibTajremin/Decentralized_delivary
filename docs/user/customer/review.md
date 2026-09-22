# Review — customer

`ReviewScreen` · `frontend/customer/lib/src/screens/review_screen.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To rate the parties who actually handled a finished order.

## How you get here

From **leave a review** on [the order screen](order.md), which appears once
the order is delivered.

## What you see

* **Rate the shop** — five stars.
* **Rate the rider** — five stars, *but only if a rider collected the order*.
* One comment field.
* **Submit.**

**Only the subjects the order really had.** The shop is always there; the
rider appears only once one picked the order up. P16 refuses a review whose
subject was not on the order, so offering a rider who never existed would
produce a `403` the customer could do nothing about.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| tap a star | sets that subject's rating | — |
| **submit** | `POST /v1/reviews`, once per subject rated | stays here, showing thanks |

**Each subject is submitted separately**, because they are separate reviews on
the server: one may be accepted while the other is refused as a duplicate, and
a single combined request would have to fail both.

## States

| State | What you see |
|---|---|
| nothing rated | the submit button is disabled |
| at least one rating given | live |
| submitting | busy |
| done | "thank you", and the button is gone |
| partly refused | thanks for what was accepted, and the server's message for what was not |

## When something goes wrong

* **Already reviewed** — the server refuses the duplicate and says so. A
  rating is not silently overwritten.
* **Not your order, or it never got there** — `403` with the reason.
* **No connection** — the offline message; the stars and comment stay as you
  set them.
