# Account — customer

`AccountScreen` · `frontend/customer/lib/src/screens/account_screen.dart`
· Figma `04__Account` (`1:2917`)

## Purpose

To be the way to everything about the customer rather than about an order.

## How you get here

The fourth tab of the [main shell](main-shell.md).

## What you see

A greeting using `profile.display_name`, then rows:

| Row | Where it goes |
|---|---|
| Personal info | [profile](profile.md) |
| Saved addresses | [addresses](addresses.md) |
| Security | [security](security.md) |
| Language | [language](language.md) |
| Notifications | [notifications](notifications.md) |
| Support | [support](support.md), list only |
| Offers, Promos, Referral, Payment methods, GoKlay Safety, Permissions | [not available yet](placeholder.md) |

and at the bottom, **sign out**.

**The greeting is the server's.** `display_name` is guaranteed never to be
empty and the server decides the fallback — a customer can order without ever
giving a name, and two clients inventing two different greetings for the same
nameless customer is exactly the drift rule 2.9 exists to stop.

**Why six rows lead to a placeholder.** They are the design's screens that
this backend has nothing behind. They are kept, rather than deleted, because
the design asks for them and a missing row is harder to explain than an honest
one. `docs/design-gaps.md` lists every one with its reason.

## Actions, and where they go

Tapping a row pushes its screen, as above. **Sign out** calls
`POST /v1/auth/logout`, clears the stored tokens, and replaces the whole shell
with the sign-in options screen.

## States

| State | What you see |
|---|---|
| loading the profile | a spinner where the greeting goes; the rows are already usable |
| loaded | the greeting |
| signing out | the button is busy |
| served from cache | the saved-information banner |

## When something goes wrong

* **The profile failed to load** — the error view with a retry. The rows still
  work, because none of them needs the profile.
* **Sign-out failed** — the tokens are cleared locally anyway and you are
  signed out. A logout that leaves somebody signed in because the network
  hiccuped is the wrong failure to have: the refresh token is revoked
  server-side on the next successful call, and the access token expires within
  fifteen minutes (ADR 0005).
* **No connection** — the cached profile under the banner.
