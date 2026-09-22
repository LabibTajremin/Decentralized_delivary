# Account — merchant

`MerchantAccountScreen` ·
`frontend/merchant/lib/src/screens/account_screen.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To be the owner's own account, as distinct from their shop.

## How you get here

From the account row inside the Shop tab of the
[merchant shell](main-shell.md).

## What you see

* A greeting using `profile.display_name`, the server's own string.
* A **language** row.
* **Sign out.**

Everything about the *shop* — its details, hours, holiday, documents — is on
[the shop screen](shop.md), not here. The split matters because a shop can
change hands: the account is a person, the shop is a business.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| **language** | — | [the language picker](../customer/language.md), which behaves identically in all three apps |
| **sign out** | `POST /v1/auth/logout`, then the stored tokens are cleared | the sign-in screen |

## States

| State | What you see |
|---|---|
| loading the profile | a spinner where the greeting goes |
| loaded | the greeting |
| signing out | the button is busy |
| served from cache | the saved-information banner |

## When something goes wrong

* **The profile failed to load** — error view with a retry; the rows still
  work.
* **Sign-out failed** — the tokens are cleared locally anyway. The refresh
  token is revoked on the next successful call and the access token expires
  within fifteen minutes; a logout that left somebody signed in because the
  network hiccuped is the wrong failure to have.
* **No connection** — the cached profile under the banner.
