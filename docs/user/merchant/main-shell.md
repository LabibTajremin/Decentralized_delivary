# Main shell — merchant

`MerchantShell` · `frontend/merchant/lib/src/screens/main_shell.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To hold the three places an owner with a shop record works in.

## How you get here

As soon as `GET /v1/merchants/me` returns a shop. Before that, the app shows
[the registration form](shop-details.md) instead — there is nothing to put in
tabs until a shop exists.

## What you see

Three destinations in a bottom bar:

| Tab | Screen |
|---|---|
| Board | [the order board](board.md) |
| Catalogue | [sections, items and bundles](catalogue.md) |
| Shop | [the shop's record](shop.md) |

The owner's own account lives inside the Shop tab rather than taking a fourth
slot, because an owner opens it roughly never and opens the board constantly.

## Actions, and where they go

| Action | What happens |
|---|---|
| tap a tab | switches to it, keeping the previous tab's data and scroll position |
| sign out, from the account screen | the shell is replaced by the sign-in screen |

## States

Three, one per tab. The shell has no loading or error state of its own.

## When something goes wrong

Nothing at this level; each tab shows its own failures, so a broken catalogue
read does not stop the board being worked.
