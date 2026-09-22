# Sign-in options — customer

`SignInOptionsScreen` ·
`frontend/customer/lib/src/screens/sign_in_options_screen.dart`
· Figma `05__Sign In Option` (`1:111`)

## Purpose

To offer the ways in. There is one.

## How you get here

After onboarding, after signing out, and after starting over from a failed
verification.

## What you see

The brand mark, a line explaining what signing in gets you, and one button:
**continue with phone**.

**What the design has that this does not.** The Figma frame draws social
sign-in buttons beside the phone one. There is no OAuth anywhere in this
backend — P04 built exactly one method, a phone number and a one-time code —
so this screen offers what exists rather than buttons that would have to
apologise when tapped. Recorded in `docs/design-gaps.md`.

## Actions, and where they go

| Action | Where you go |
|---|---|
| **continue with phone** | the [phone number screen](../shared/phone-sign-in.md) |

## States

One.

## When something goes wrong

Nothing here can fail; no request is made.
