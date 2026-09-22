# Register a shop, or edit its details — merchant

`ShopDetailsScreen` ·
`frontend/merchant/lib/src/screens/shop_details_screen.dart`
· no Figma frames — the design file contains no merchant screens at all
(`docs/design-gaps.md`)

One form for both registering and editing, because they are the same fields
with the same validation.

## Purpose

To create a shop record, or change its details afterwards.

## How you get here

* **As registration:** immediately after signing in, whenever
  `GET /v1/merchants/me` answers `404` — that is, this account has no shop
  yet. There is no other screen until it does.
* **As an edit:** from the shop row on [the shop screen](shop.md).

## What you see

* **Shop type** — the types `GET /v1/merchants/registration-requirements`
  lists, with the documents each one needs shown beneath, in the server's
  words.
* **Name**, **phone**, **email**, **address line 1**, **line 2**.
* **Latitude** and **longitude**.
* **Register** — or **save**, when editing.

**Neither the type list nor the document list is a constant here.** Which
papers a pharmacy must produce is a rule, and a rule held in the app goes
stale the moment the regulator changes their mind.

**The area, district and division are not fields.** P02 resolves them from the
coordinate, and they become the D3 boundary this shop is forever served
within — not something a form should be able to state.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| choose a type | the required documents beneath it change | — |
| **register** | `POST /v1/merchants` | [the shop screen](shop.md), now that a record exists |
| **save** (editing) | `PATCH /v1/merchants/me` | back to [the shop screen](shop.md) |

## States

| State | What you see |
|---|---|
| loading the requirements | a spinner |
| a required field empty | the button is disabled |
| complete | live |
| submitting | busy |
| refused | the server's message above the button, with everything typed kept |

## When something goes wrong

* **The coordinate is outside Bangladesh** — refused, with the server's
  sentence. There is nothing to work around: a shop outside every division
  cannot be served.
* **This account already has a shop** — one shop per account, enforced by a
  unique constraint rather than a check that two racing registrations could
  both pass. The loser is told it lost.
* **The phone number is not valid** — the server normalises to E.164 and says
  so when it cannot.
* **No connection** — the offline message; nothing typed is lost.
