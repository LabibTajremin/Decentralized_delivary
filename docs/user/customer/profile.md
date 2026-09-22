# Profile — customer

`ProfileScreen` · `frontend/customer/lib/src/screens/profile_screen.dart`
· Figma `05__Profile | Personal info` and `06__Profile Edit` (`1:3289`,
`1:3394`)

Two frames, one screen: the fields are editable and a save button commits
them.

## Purpose

To see and change the customer's name and email.

## How you get here

From **personal info** on [the account screen](account.md).

## What you see

* **Name** and **email**, both editable, pre-filled with what the server
  holds.
* The phone number, which is not editable — it is the identity the account is
  keyed on.
* **Save.**

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| edit a field | the save button becomes live once something differs | — |
| **save** | `PATCH /v1/me` with only the fields that changed | stays here; the form redraws from the response |

**Only the fields that changed are sent.** `PATCH /v1/me` leaves an omitted
field alone, so editing a name cannot blank an email the customer set from
another device.

## States

| State | What you see |
|---|---|
| loading | a spinner |
| unchanged | the save button is disabled |
| changed | live |
| saving | busy |
| saved | the fields show what the server returned, which is the authority on what was stored |
| refused | the server's message above the button; your text stays |

## When something goes wrong

* **The email is not valid** — the server says so, in its own words, and the
  text is kept so it can be corrected rather than retyped.
* **The read failed** — error view with a retry.
* **No connection** — the offline message. Nothing is queued: a profile edit
  replayed later could overwrite a change made in between from another device.
