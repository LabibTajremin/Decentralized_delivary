# Security — customer

`SecurityScreen` · `frontend/customer/lib/src/screens/security_screen.dart`
· Figma `07__Profile | Security` (`1:3446`), which the design draws empty

## Purpose

To show where the account is signed in, and to sign it out everywhere at once.

## How you get here

From **security** on [the account screen](account.md).

## What you see

A row per active session, `GET /v1/auth/sessions`: the device label as it was
given at sign-in, and when it was last seen. The current device is marked.

Then **sign out everywhere**.

**The list carries no token and no hash** — only a label and a time. That is
enough to recognise a device, and a list of credentials would be a new place
to leak them from.

**Why "everywhere" rather than per device.** This is the screen somebody
reaches for when they think their account is compromised, and it deliberately
does not require knowing *which* device was taken.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| **sign out everywhere** | `POST /v1/auth/logout-all` | back to the root, then the sign-in options screen |

Signing out from here pops back to the root *before* the app swaps its halves.
The signed-in and signed-out halves are exchanged underneath the navigator, so
a screen left on the stack would keep a signed-out customer looking at their
own device list.

## States

| State | What you see |
|---|---|
| loading | a spinner |
| sessions | the rows, the current device marked |
| signing out | the button is busy |
| the request failed | the server's message above the list, and the button live again |

## When something goes wrong

* **The list failed to load** — error view with a retry. The sign-out button
  still works, which matters: the reason for being on this screen may be that
  something is wrong.
* **Signing out everywhere failed** — the message is shown and you stay signed
  in, rather than the app pretending. Retry.
* **No connection** — the offline message.

One honest limitation, from ADR 0005: revoking sessions takes effect on
refresh tokens immediately, but an access token already issued stays valid
until it expires — at most fifteen minutes by default.
`docs/security-review.md` records the trade.
