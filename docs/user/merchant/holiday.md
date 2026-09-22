# Holiday — merchant

`HolidayScreen` · `frontend/merchant/lib/src/screens/holiday_screen.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To close the shop temporarily without changing its hours, and to reopen it.

## How you get here

From the holiday row on [the shop screen](shop.md).

## What you see

* Whether the shop is currently on holiday.
* A **reason** field, which customers do not see — it is the owner's own note
  and the admin's context.
* **Close the shop** — or **reopen the shop**, when it is already closed.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| **close the shop** | `PUT /v1/merchants/me/holiday` with the reason | back to [the shop screen](shop.md), now showing "on holiday" |
| **reopen the shop** | the same endpoint, clearing it | back to [the shop screen](shop.md) |

**Being on holiday is what takes the shop off every customer's list**, so it
is a call to the server rather than a local flag: an app that hid the shop by
itself would still be taking orders.

Holiday mode is the owner closing temporarily, which is deliberately distinct
from an admin suspending the shop. The owner can undo this one.

## States

| State | What you see |
|---|---|
| open | the reason field and **close the shop** |
| on holiday | the reason it was closed, and **reopen the shop** |
| saving | the button is busy |
| refused | the server's message |

## When something goes wrong

* **The write failed** — the message is shown and the shop stays as it was.
  Retry: a shop that believes it is closed while the server believes it is
  open is the one state worth avoiding here.
* **No connection** — the offline message, and nothing is queued for exactly
  that reason.

Orders already accepted are not cancelled by going on holiday. They still have
to be cooked, and they stay on [the board](board.md).
