# Onboarding — customer

`OnboardingScreen` · `frontend/customer/lib/src/screens/onboarding_screen.dart`
· Figma `1:66`, `1:81`, `1:96`

## Purpose

To say what GoKlay is, in three pages, before asking anybody to sign in.

## How you get here

Straight after the splash, on a first launch — that is, whenever no token is
stored. Somebody already signed in never sees it, and there is no way back to
it once it has been passed.

## What you see

Three pages, swipeable, with a page indicator. Each has a title and a short
body. Below them: **skip** on the left, **next** on the right, and on the last
page **next** becomes the button that finishes.

The words on these three pages are the only product copy in the whole app that
is neither chrome nor composed by the server. They are here rather than in a
response because they are shown before any request is made: a first launch on
a phone with no signal still has to be able to say what this is.

## Actions, and where they go

| Action | Where you go |
|---|---|
| swipe, or **next** | the following page |
| **next** on the last page | the sign-in options screen |
| **skip**, from any page | the sign-in options screen |

## States

Three, one per page, plus which of them is showing. Nothing is loaded and
nothing can be busy.

## When something goes wrong

Nothing here can fail. It makes no request and reads nothing but its own
strings, which is why it works on a phone that has never had a connection.
