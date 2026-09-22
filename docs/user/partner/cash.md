# Cash — partner

`CashScreen` · `frontend/partner/lib/src/screens/cash_screen.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To show how much cash the rider is carrying, and what it is for.

## How you get here

The third tab of the [partner shell](main-shell.md).

## What you see

`GET /v1/partner/cod`:

* **Outstanding** — what is still owed for delivered cash orders, as the
  server formatted it.
* **Remitted** — what has already been handed over.
* **Held collections** — a row per cash order behind the outstanding figure,
  oldest first: the order it came from and its amount.

**Both totals come from the server**, which recomputes them from the
collections rather than storing them. The screen does not add the held rows up
to check: a rider and an office disagreeing about how much cash is in a bag is
exactly the argument a second calculation causes.

This route takes no partner id. There is no parameter for anybody to change —
the ledger returned is the one belonging to the token.

## Actions, and where they go

| Action | Where you go |
|---|---|
| back / switch tab | elsewhere in the shell |

Nothing here remits. Handing cash over is reconciled by an operator, through
the admin surface, once the money is actually in their hands — which is the
only point at which it is true.

## States

| State | What you see |
|---|---|
| loading | a spinner |
| a balance | the two totals and the held rows |
| nothing owed | "settled" — no held collections |
| served from cache | the saved-information banner |

## When something goes wrong

* **Not registered as a partner** — `404`. Cannot happen from inside the
  shell, which is only reached once a partner record exists.
* **The read failed** — error view with the server's message and a retry.
* **No connection** — the cached ledger under the banner. A figure that may be
  minutes old is still what the rider needs when an operator asks how much
  they are carrying, and the banner says it is saved.
