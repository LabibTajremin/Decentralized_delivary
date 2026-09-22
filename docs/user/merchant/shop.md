# Shop — merchant

`ShopScreen` · `frontend/merchant/lib/src/screens/shop_screen.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To show the shop's own record, its documents, and exactly where it stands in
the approval workflow.

## How you get here

The third tab of the [merchant shell](main-shell.md), and the first screen an
owner with an unapproved shop sees.

## What you see

* The shop's name and the server's **open status** line — a composed sentence,
  not a flag the app phrases.
* Where it is in review, and the admin's **review note** when there is one: a
  rejection or suspension is never unexplained.
* **Documents** — what has been filed, and **still needed** listing what has
  not.
* **Submit for review**, when it is allowed.
* Rows to [details](shop-details.md), [hours](hours.md) and
  [holiday](holiday.md).

**Three server-decided fields drive the whole screen.** `missing_documents`
says what is still wanted — the app does not subtract the uploaded list from
the required one. `can_submit` says whether the submit button is live.
`open_status` and `review_note` are sentences the server composed, printed as
they arrived.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| **add a document** | — | [documents](document.md); on save, back here reloaded |
| **submit for review** | `POST /v1/merchants/me/submit` | stays here; the status becomes "awaiting review" |
| shop row | — | [edit details](shop-details.md) |
| hours row | — | [hours](hours.md) |
| holiday row | — | [holiday](holiday.md) |

## States

| State | What you see |
|---|---|
| loading | a spinner |
| draft, papers missing | the missing list, and submit disabled |
| draft, everything filed | submit is live |
| awaiting review | "awaiting review", and no submit button |
| approved | the open status, and the shop is visible to customers |
| rejected or suspended | the admin's note, and whatever `can_submit` allows next |
| on holiday | "on holiday" — the shop is off every customer's list until it reopens |
| submitting | the button is busy |

## When something goes wrong

* **Submitting was refused** — a paper is still missing, or the shop is not in
  a state that can be submitted. The server's sentence says which, and the
  record is re-read so the button matches reality.
* **The read failed** — error view with a retry.
* **No connection** — the cached record under the saved-information banner.
