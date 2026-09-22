# Add address — customer

`AddAddressScreen` · `frontend/customer/lib/src/screens/add_address_screen.dart`
· Figma `13__Add Address` and the map frames (`docs/design-gaps.md`)

## Purpose

To save a delivery address, with a coordinate the system can serve from.

## How you get here

* From **add an address** on [addresses](addresses.md).
* From **add an address** on [Home](home.md), when there are none and there is
  therefore nothing to search from.

## What you see

Fields: **label**, **recipient name**, **recipient phone**, **address line 1**,
**line 2**, **instructions for the rider**, and **latitude** and **longitude**.
Then **confirm this point**, and **save**.

**The area, district and division are not fields.** P02 resolves them from the
coordinate. A client that guessed would eventually file an address under a
division the order could not be served from — and D3 makes that boundary the
hard edge of the whole system.

**There is no map.** This build has no maps plugin and the environment has no
key for one. Picking a point on a map is platform work and would not change
what the app knows about the address. Recorded in `docs/design-gaps.md`.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| **confirm this point** | `GET /v1/geo/resolve` for the coordinate | stays here; the resolved area name appears as the server wrote it |
| **save** | `POST /v1/me/addresses` | back to wherever you came from, with the new address available |

The point is confirmed *before* saving on purpose: `GET /v1/geo/resolve`
answers `404` for a coordinate outside every division, so the form can say so
while the customer is still looking at it rather than at checkout.

## States

| State | What you see |
|---|---|
| a required field empty, or no coordinate | the save button is disabled |
| everything present | live |
| confirming | that button is busy |
| confirmed | the area name, from the server |
| the point is outside Bangladesh | the server's refusal, and save stays disabled |
| saving | the save button is busy |

## When something goes wrong

* **The coordinate is outside every division** — `404` from resolve, shown as
  the server's sentence. This is the check that stops an unservable address
  existing at all.
* **A field is refused** — the phone number is the usual one; the server
  normalises it and says so when it cannot.
* **Saving failed** — the message appears and nothing you typed is lost.
* **No connection** — the offline message. Neither step is queued: an address
  saved without a resolved area would be an address no order could use.
