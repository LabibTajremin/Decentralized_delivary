# Account — partner

`PartnerAccountScreen` ·
`frontend/partner/lib/src/screens/account_screen.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To hold the rider's own settings: their distance choice, their reported
position, and the way out.

## How you get here

The fourth tab of the [partner shell](main-shell.md).

## What you see

* Their name, and their **acceptance** rate as the server computed it.
* **Preference** — three choices: **short jobs**, **long jobs**, **any**. Each
  carries the server's own sentence explaining what it means.
* **Your location** — latitude and longitude, with **update location**.
* A **language** row, and **sign out**.

**The three distance choices are D4's, and `any` is the default** — a partner
who has not chosen has not chosen to exclude anything. The label beside each
is the server's sentence, so the app never explains the rule itself.

**Why location is a field and a button.** There is no background-location
plugin in this build, so position is reported when the rider chooses to. This
is honest rather than ideal, and recorded in `docs/design-gaps.md` — but it is
also the call that decides where the rider can work at all, which is D1 from
the rider's side.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| choose a preference | `PUT /v1/partner/preference` | stays here; [the feed](feed.md) will reflect it |
| **update location** | `PUT /v1/partner/location` | stays here; the feed's radius now centres on the new point |
| **language** | — | [the language picker](../customer/language.md) |
| **sign out** | `POST /v1/auth/logout`, then the tokens are cleared | the sign-in screen |

## States

| State | What you see |
|---|---|
| loading | a spinner |
| the record | acceptance, preference, position |
| a change in flight | the rows are disabled |
| refused | the server's message |
| served from cache | the saved-information banner |

## When something goes wrong

* **The coordinate is outside Bangladesh** — refused with the server's
  sentence. A rider cannot report a position the system cannot place.
* **The preference write failed** — the message is shown and the old choice
  stands, rather than the app showing a preference the server does not hold.
* **Sign-out failed** — the tokens are cleared locally anyway; the refresh
  token is revoked on the next successful call.
* **Signing out with a queued outbox** — the queue is per device and is not
  sent for you. Flush it from the banner on [the shell](main-shell.md) first;
  the banner is visible from this screen too, which is why it lives above the
  tabs.
* **No connection** — the cached record under the banner.
