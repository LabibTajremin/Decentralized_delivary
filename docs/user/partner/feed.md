# Feed — partner

`FeedScreen` · `frontend/partner/lib/src/screens/feed_screen.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To show what the rider should be looking at right now (ALG-08), and to go on
and off shift.

## How you get here

The first tab of the [partner shell](main-shell.md).

## What you see

* **Go on shift** / **go off shift**, and the server's own availability label.
* The feed, `GET /v1/partner/feed`: a card per job with its pickup, its
  destination, the whole distance, the distance to the pickup, and the band.
* On each offered job: **accept** and **decline**.
* When the list is empty: the server's **notice**, not a blank.

**The list is the server's.** Bounded, nearest-pickup first, already filtered
by the rider's own distance choice (D4). A client that filtered a global list
by its own radius would be deciding visibility — which rule 2.9 forbids, and
which would need a copy of `dispatch.partner_radius` to do at all.

**An empty feed is never just empty.** `reason` says `offline`,
`at_capacity` or `nothing_nearby`, and `notice` is the sentence — so a rider
who forgot to go on shift is told that rather than left staring at a blank
screen.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| **go on shift** | `PUT /v1/partner/availability` | stays here; the feed reloads and offers start arriving |
| **go off shift** | the same, the other way | stays here; the feed empties with the `offline` notice |
| **accept** | `POST /v1/partner/jobs/{id}/accept` | stays here; the job moves to [Jobs](jobs.md) |
| **decline** | `.../decline` | stays here; it goes to another rider |
| tap a card | — | [the job screen](job.md) |

Going on shift matters more than it looks: dispatch's first offer round runs
once, at the moment a shop says ready, and it only asks partners who are
already on shift. A rider who signs on afterwards waits for the next sweep.

## States

| State | What you see |
|---|---|
| loading | a spinner |
| jobs offered | the cards |
| off shift | the `offline` notice and nothing else |
| at capacity | the `at_capacity` notice — the concurrent-job limit is reached |
| nothing nearby | the `nothing_nearby` notice, with the radius the server used |
| an action in flight | that card's buttons are disabled |
| served from cache | the saved-information banner |

## When something goes wrong

* **Somebody else took the job first** — refused, with the reason, and the
  feed reloads. One rider gets each job; the race is decided by a conditional
  update in the database, not by whoever's request arrived first.
* **The offer expired** — the same. The sweep expires offers nobody answered.
* **Accepting would exceed the concurrent limit** — refused, and said so.
* **No connection** — the cached feed under the banner. Accepting and
  declining from *here* are not queued: an offer is time-limited and a queued
  accept would arrive after it expired. The actions that **are** queued are on
  [the job screen](job.md), where they have to be.
