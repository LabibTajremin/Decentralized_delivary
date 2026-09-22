# Splash — all three apps

`GoklaySplashScreen` · `frontend/goklay_core/lib/src/auth/splash_screen.dart`
· no Figma frame (the design starts at onboarding)

## Purpose

To hold the first frame while stored tokens are read off the device, so that
somebody who is already signed in never sees the sign-in screen flash past.

That is the whole job, and it is worth a screen because the alternative reads
as a bug: storage is asynchronous, and showing the wrong half of the app for
one frame is the kind of flicker users report as "it logged me out".

## How you get here

It is the first thing the app shows, on every launch, before anything else.
You cannot navigate to it and you never come back to it.

## What you see

The brand colour, filling the screen, with "GoKlay" in the display type. No
spinner, no version number, no tagline.

## Actions, and where they go

None. There is nothing to tap.

When `Session.restore()` answers, the app moves on by itself:

| What storage said | Where you land |
|---|---|
| a valid token is stored | straight into the app's main shell |
| nothing is stored | the first pre-sign-in screen (onboarding, for the customer app) |

## States

One. It is on screen for as long as reading local storage takes, which is
milliseconds on a working device and is not something the screen tries to
decorate.

## When something goes wrong

There is nothing to fail loudly. `Session.restore()` treats unreadable storage
as "nobody is signed in" rather than as an error, so a wiped or corrupt token
store sends you to sign-in instead of to an error screen — which is the same
place you would have to go anyway.

No network call is made here. A launch with no connection reaches sign-in
exactly as fast as one with a connection.
