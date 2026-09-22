# Main shell — customer

`MainShell` · `frontend/customer/lib/src/screens/main_shell.dart`
· the design's bottom bar, with one substitution

## Purpose

To hold the four places a signed-in customer lives, and to keep each one's
state while they move between them.

## How you get here

Immediately after verification, and on every subsequent launch where a token
is stored. It is the root of the signed-in app; everything else is pushed on
top of it.

## What you see

A bottom navigation bar with four destinations, and whichever one is selected
filling the screen above it:

| Tab | Screen |
|---|---|
| GoKlay | [Home](home.md) |
| Cart | [Cart](cart.md) |
| Activity | [Orders](orders.md) |
| Account | [Account](account.md) |

**Why the cart is a tab and Offers is not.** The design's bar is Home, Offers,
Activity, Account. Offers has no backend — `docs/design-gaps.md` says why — and
spending one of four slots on a screen whose only content is "not available
yet" is worse than moving it into Account, which is where it now lives. The
cart takes the slot, because it is the screen a customer returns to constantly
and the one the design otherwise only reaches from inside a shop.

## Actions, and where they go

| Action | What happens |
|---|---|
| tap a tab | switches to it; the previous tab keeps its scroll position and its loaded data |
| add something to the cart, anywhere | the cart tab reloads, so its contents are never one screen behind |
| sign out, from Account | the whole shell is replaced by the sign-in options screen |

Each tab is kept alive rather than rebuilt, so coming back to Home does not
re-run the search you were looking at.

## States

Four, one per tab. The shell itself has no loading or error state — each tab
owns its own.

## When something goes wrong

Nothing at this level. A failure belongs to whichever tab was making the
request, and is shown inside that tab so the rest of the app stays usable.
