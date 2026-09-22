# Job — partner

`JobScreen` · `frontend/partner/lib/src/screens/job_screen.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To carry one delivery: where to collect it, where it goes, and the one thing
to do next.

**This is the screen the offline queue exists for.**

## How you get here

By tapping a card on [the feed](feed.md) or a row on [Jobs](jobs.md).

## What you see

* The job's code and the server's status line.
* **Pickup** — the shop's name, its phone, and its address as the server
  renders it.
* **Drop-off** — the same for the customer.
* The whole distance, and the band.
* One action, whichever applies: **accept**, **decline**, **collected**,
  **delivered**, or **could not deliver** with a reason.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| **accept** / **decline** | the matching call | stays here, re-read |
| **collected** | `POST /v1/partner/jobs/{id}/collect` | stays here; the next action becomes **delivered** |
| **delivered** | `.../deliver` | stays here; the job is finished. A cash order's collection lands on [the ledger](cash.md) |
| **could not deliver** | `.../fail` with the typed reason | stays here, with the reason recorded |

### Why this screen queues, and the feed does not

A rider taps "collected" at a counter with no signal and "delivered" in a
stairwell with none either. Those two taps must arrive **in that order or not
at all** — the order state machine refuses `delivered` from an order that
never reached `picked_up`.

So a tap that cannot reach the server is queued rather than lost, the queue
replays strictly oldest-first, and it stops at the first action the network
still cannot carry. The banner on the [shell](main-shell.md) says how many are
waiting, from every tab.

## States

| State | What you see |
|---|---|
| offered | **accept** and **decline** |
| accepted | **collected** |
| collected | **delivered**, and **could not deliver** |
| finished | the status line, and no actions |
| an action in flight | the button is busy |
| queued offline | "will be sent when you are back online" — the tap was taken, and the banner above now shows it |
| refused | the server's message; the job re-reads so the button matches |

## When something goes wrong

* **No connection** — the tap is queued and the screen says so. This is the
  designed path, not a failure.
* **A queued action is later refused** — it is dropped and reported in the
  banner. Somebody else completed the job, or an admin moved it; replaying
  would never succeed.
* **The transition is not allowed** — the server's sentence, and the job is
  re-read.
* **Someone else's job** — `404`, the same answer an id that never existed
  gets.
* **"Could not deliver" with no reason** — refused. The reason is what an
  operator and the customer are owed.

**There is no background location reporting.** The rider's position is
reported from [the account screen](account.md) when they choose to, because
this build has no background-location plugin and no notification channel to
justify one. Recorded in `docs/design-gaps.md`.
