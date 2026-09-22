# Notifications — customer

`NotificationsScreen` ·
`frontend/customer/lib/src/screens/notifications_screen.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To show what the customer has been told about their orders.

## How you get here

From **notifications** on [the account screen](account.md).

## What you see

A row per notification, `GET /v1/me/notifications`, newest first: the title,
the body, and when it was sent. A notification that failed to deliver says so.

**Both the title and the body are composed by the server**, in the customer's
language. There is nothing to design around, which is why this screen is a
list of what it was given and nothing more. The design has no frame for it;
P14 built the endpoint, and `docs/design-gaps.md` records why the screen
exists anyway.

## Actions, and where they go

| Action | Where you go |
|---|---|
| back | [the account screen](account.md) |

Rows are not tappable. A notification is a record of something already said,
not a way to reach the order — and the order is one tab away.

## States

| State | What you see |
|---|---|
| loading | a spinner |
| notifications | the list |
| none | "nothing to show" |
| one that failed to send | the row says so, which is how a customer discovers why they never got an SMS |
| served from cache | the saved-information banner |

## When something goes wrong

* **The read failed** — error view with the server's message and a retry.
* **No connection** — the cached list under the banner.
