# Register — partner

`RegisterScreen` · `frontend/partner/lib/src/screens/register_screen.dart`
· no Figma frames — the design file contains no partner screens at all
(`docs/design-gaps.md`)

## Purpose

To sign up to carry orders.

## How you get here

Immediately after signing in, whenever `GET /v1/partner` answers `404` — that
is, this account is not yet a delivery partner. There is no other screen until
it is.

## What you see

* **Name.**
* **Phone** — the number a shop and a customer will call on arrival.
* **Vehicle** — free text.
* **Register.**

**There is no area to choose and nothing to approve against a map.** This is
D1 from the rider's side: a rider may work anywhere in Bangladesh, and where
they can work is decided every time they report a location. So the form asks
for a name, a number and a vehicle, and nothing about geography.

Vehicle is free text rather than a picker because the list of things people
deliver on in Bangladesh would be wrong within a month.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| **register** | `POST /v1/partner` | [the partner shell](main-shell.md) — the four tabs |

A new rider starts **off shift**, at 100% acceptance, carrying nothing.

## States

| State | What you see |
|---|---|
| a required field empty | the button is disabled |
| complete | live |
| registering | busy |
| refused | the server's message above the button, with the fields kept |

## When something goes wrong

* **This account is already a partner** — one partner record per account; a
  second would give somebody two feeds and two concurrent-job budgets. Refused,
  and said so.
* **The phone number is not valid** — the server normalises to E.164 and says
  so when it cannot.
* **No connection** — the offline message. Registration is not queued: there
  is nothing to do with a rider record that has not been created yet.
