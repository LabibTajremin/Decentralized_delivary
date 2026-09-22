# Orders — customer

`OrdersScreen` · `frontend/customer/lib/src/screens/orders_screen.dart`
· Figma `03__Activity` (`1:2059`)

## Purpose

To list the customer's orders, separating the ones still happening from the
ones that are finished.

## How you get here

The third tab of the [main shell](main-shell.md), labelled Activity.

## What you see

Two tabs — **current** and **past** — and a row per order: its code, the
server's status line, the number of items, and the total as the server
formatted it.

**Which orders count as "current" is the server's filter, not a local one.**
The list is read with `live=true`, and which statuses are still going is the
order state machine's business (P11). It can gain a status without this screen
learning about it.

## Actions, and where they go

| Action | Where you go |
|---|---|
| tap a row | [that order](order.md) |
| switch tab | re-reads the list with the other filter |

## States

| State | What you see |
|---|---|
| loading | a spinner |
| orders | the rows, newest first |
| none | "no orders yet" |
| served from cache | the saved-information banner |

## When something goes wrong

* **The read failed** — the error view with the server's message and a retry.
* **No connection** — the cached list with the banner, which means a customer
  can still see their order code and status while out of signal.
