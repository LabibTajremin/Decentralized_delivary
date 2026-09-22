# Sign in with a mobile number — all three apps

`PhoneSignInScreen` · `frontend/goklay_core/lib/src/auth/phone_sign_in_screen.dart`
· Figma `06__Sign In With Mobile Number` (`1:142`)

## Purpose

To ask for a mobile number and get a one-time code sent to it. This is the
only way anybody signs in to any of the three apps.

## How you get here

* **Customer:** from the sign-in options screen, by tapping "continue with
  phone".
* **Merchant and partner:** it is the first screen after the splash, because
  those apps have no onboarding to walk through.

## What you see

* A heading and a short line beneath it.
* One field, labelled "mobile number", hinting the shape `01XXXXXXXXX`. It
  takes the phone keyboard.
* One button: **send code**.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| type a number | the button becomes live as soon as the field is not empty | — |
| **send code** | `POST /v1/auth/otp/request` | the code screen, carrying the number and the server's challenge |

The number is sent **as typed**. Whether it is a valid Bangladeshi mobile
number is the server's decision: it normalises the number to E.164 and answers
`400` with a sentence when it cannot. The app deliberately does not carry a
second regular expression, because a slightly different one would reject
numbers the server would have accepted.

The only local check is that the field is not empty, and that is about not
making a pointless request rather than about validity.

## States

| State | What you see |
|---|---|
| empty field | the button is disabled |
| a number typed | the button is live |
| sending | the button shows it is busy and cannot be tapped again |
| refused | the field stays as you typed it, with the message beneath |

The number is never cleared on failure. Retyping an eleven-digit number
because the network dropped is the sort of small insult that makes people give
up on an app.

## When something goes wrong

* **The number is not one the server accepts** — its own sentence appears
  below the field. Bengali, or English if that is the chosen language.
* **Too many requests** — the rate limiter refuses, and its message says so.
  This is one of only three paths in the whole API that is rate limited, and
  it is rate limited precisely because it sends an SMS that somebody pays for.
* **No connection** — the offline message appears. Nothing is queued: a
  sign-in you cannot complete now is not worth replaying later.
