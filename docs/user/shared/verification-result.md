# Verified, or could not verify — all three apps

`VerificationResultScreen` ·
`frontend/goklay_core/lib/src/auth/verification_result_screen.dart`
· Figma `09__Verification Successfull` (`1:219`) and `10__Verification Failed`
(`1:261`)

Two frames, one screen, told which it is at construction.

## Purpose

To confirm, in one unambiguous frame, that sign-in either worked or did not —
and to give exactly one way onward from each.

## How you get here

From the code screen. Success when the server issued a token pair, failure
when it refused one.

## What you see

**Verified:** a tick in the brand colour, "verified", "you can get started",
and one button — **get started**.

**Could not verify:** a warning glyph in the danger colour, "could not
verify", and the reason. The reason is the server's own sentence when it sent
one; the generic "the code did not match, or it expired" appears only when it
did not. One button — **start over**.

## Actions, and where they go

| Screen | Button | Where you go |
|---|---|---|
| verified | **get started** | the app's main shell, signed in |
| could not verify | **start over** | back to the phone-number screen, with the stack cleared |

Starting over really does start over. The failed attempt is not kept, and the
number is asked for again, because the most common cause of a refused code is
a number typed wrong one screen earlier.

## States

Two, and which one is showing was decided before the screen was built. It has
no loading state and makes no request of its own — everything it needs
happened on the previous screen.

## When something goes wrong

Nothing here can fail. That is the point of putting it in its own screen: the
network work is finished, and this frame's only job is to say so.
