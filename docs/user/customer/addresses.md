# Addresses — customer

`AddressesScreen` · `frontend/customer/lib/src/screens/addresses_screen.dart`
· Figma `12__Saved Address` (`1:3665`)

## Purpose

To manage the address book — which is also the list Home searches from and
checkout delivers to.

## How you get here

From **saved addresses** on [the account screen](account.md).

## What you see

A row per address: its label, the server's own one-line rendering, and a
"default" marker on one of them. Each row offers **make default** and
**delete**. At the bottom, **add an address**.

**Each row shows `single_line`** — the server's rendering, the same one the
receipt and the rider's screen show. The app never joins line 1, line 2 and
the area itself, because a second implementation would eventually join them
differently and a customer would see two versions of their own address.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| **add an address** | — | [add address](add-address.md); on save, back here with the list reloaded |
| **make default** | `POST /v1/me/addresses/{id}/default` | stays here; the list reloads |
| **delete** | `DELETE /v1/me/addresses/{id}` | stays here; the list reloads |

Exactly one address is default at a time, and the server enforces it — making
one default clears the other in the same write, rather than the app doing two
calls and hoping.

## States

| State | What you see |
|---|---|
| loading | a spinner |
| addresses | the rows, default first |
| none | "you have no saved addresses", with the add button |
| a change in flight | the rows are not tappable until it lands |
| served from cache | the saved-information banner |

## When something goes wrong

* **Deleting was refused** — the server's sentence appears above the list. An
  address on a live order is the usual reason.
* **The read failed** — error view with a retry.
* **No connection** — the cached list under the banner. Changes are not
  queued, because an address replayed later might be deleted by then.
