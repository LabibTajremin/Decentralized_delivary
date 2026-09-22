# Language — customer

`LanguageScreen` · `frontend/customer/lib/src/screens/language_screen.dart`
· Figma `16__Language` (`1:3960`)

## Purpose

To choose between Bengali and English.

## How you get here

From **language** on [the account screen](account.md). The merchant and
partner apps have the same screen in the same place.

## What you see

Two rows — **বাংলা** and **English** — with the current choice ticked.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| tap a language | switches the app's locale, changes the `lang` parameter on every subsequent request, and saves the choice with `PATCH /v1/me` | stays here, now in the chosen language |

**Changing the language does two things, and both matter.** It switches the
widgets, and it switches the `lang` query parameter the app sends. Because
most of what a customer reads is composed by the server, a client that changed
only its own strings would show a Bengali order status under an English
heading.

It is also saved to the profile, so the next device this customer signs in on
starts in the language they chose rather than the one the handset is set to.

Bengali is the default, and not only for Bengali handsets: an unrecognised
device locale resolves to Bengali rather than English, because this product is
Bengali-first.

## States

| State | What you see |
|---|---|
| current | that row is ticked |
| saving | the rows are not tappable while the profile write is in flight |

## When something goes wrong

* **The profile write failed** — the app stays in the language you chose; only
  the *remembering* failed. The next device would start in the old language.
  No error is forced on top of a working screen for that.
* **No connection** — the same: the switch works, the preference is not
  stored.
