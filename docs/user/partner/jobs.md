# Jobs — partner

`JobsScreen` · `frontend/partner/lib/src/screens/jobs_screen.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To list the rider's own deliveries, the current ones and the finished ones.

## How you get here

The second tab of the [partner shell](main-shell.md).

## What you see

Two tabs — **current** and **past** — and a row per job: its code, the
server's status line, the pickup and destination as the server renders them,
and the distance.

Which jobs count as current is the server's `live=true` filter, not a local
one.

## Actions, and where they go

| Action | Where you go |
|---|---|
| tap a row | [that job](job.md) |
| switch tab | re-reads with the other filter |

There are no actions on the rows themselves. Everything that changes a
delivery happens on [the job screen](job.md), because that is where the
offline queue is and a queued action needs one place to have come from.

## States

| State | What you see |
|---|---|
| loading | a spinner |
| jobs | the rows |
| none | "no deliveries" |
| served from cache | the saved-information banner |

## When something goes wrong

* **The read failed** — error view with the server's message and a retry.
* **No connection** — the cached list under the banner, which means a rider
  can still see the address they are riding to.
