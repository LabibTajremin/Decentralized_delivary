# Enter the code — all three apps

`OtpScreen` · `frontend/goklay_core/lib/src/auth/otp_screen.dart`
· Figma `07__OTP Verification` (`1:181`) and `08__OTP Timeout | Resend`
(`1:200`)

The design draws two frames. They are one screen with two states, and the
state belongs to the server.

## Purpose

To take the six digits that were sent by SMS, exchange them for a token pair,
and store it.

## How you get here

Only from the sign-in screen, and only after a code has actually been
requested — it arrives holding the number and the challenge the server
returned.

## What you see

* The heading, and a line saying a six-digit code was sent to your phone.
* The number it went to, so a mistyped digit is visible before you wait for an
  SMS that will never come.
* One field for the code, on the number keypad.
* **Verify.**
* Underneath: either a countdown, or a **send the code again** button.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| **verify** | `POST /v1/auth/otp/verify` with the number, the code and this app's role | the "verified" screen, then into the app |
| **send the code again** | `POST /v1/auth/otp/request` again; the countdown restarts from the new challenge | stays here |

The app's **role** is sent rather than discovered afterwards. The server
refuses to issue a customer token to the merchant app, which is better than
handing a rider an empty shop screen and letting them work out why.

On success the token pair is stored *before* the callback fires, so the next
screen is already signed in rather than racing storage.

## States

| State | What you see |
|---|---|
| waiting to resend | "you can ask again in *n*" — the countdown starts at the server's `resend_after` |
| resend available | the countdown is replaced by the resend button |
| verifying | the verify button is busy |
| refused | you are taken to the "could not verify" screen, which carries the reason |

**The countdown is the server's number, not the app's.** `resend_after` comes
with the challenge, and an admin can change `auth.otp_ttl` without an app
release. An app that hard-coded sixty seconds would be wrong the first time
that value moved.

## When something goes wrong

* **Wrong or expired code** — you land on the failure screen with the server's
  explanation, and one button back to the start. The code is not silently
  re-requested.
* **Too many wrong attempts** — the server locks the number for
  `auth.otp_lockout_window`, and says so. Brute force stops here, and the
  E2E suite proves it.
* **No connection** — the offline message appears in place of the failure
  screen, because nothing was refused; nothing arrived.
