# Tracking — customer

`TrackingScreen` · `frontend/customer/lib/src/screens/tracking_screen.dart`
· Figma `10__Order Tracking` / `11__Order Tracking` (`1:10633`, `1:10691`)

Two frames: before and after a rider is assigned.

## Purpose

To follow a delivery while it is happening.

## How you get here

From **track order** on [the order screen](order.md), while the order is live.

## What you see

A live stream, `GET /v1/track/{orderId}`, server-sent events:

* The status, as the server's line.
* Before anybody has collected it: "waiting for a rider".
* Once a rider has: their name, their phone number, their vehicle, and the
  position the server last reported for them.
* When the stream ends: "tracking has ended".

**Which of the two frames is showing is the stream's decision.** While the
snapshot's `partner` is null nobody has collected the order and the screen
says so. It ends on the frame carrying `live: false`, so the app never holds
its own list of which statuses are terminal — the order state machine can gain
a state without this screen learning about it.

**No map tile is drawn.** There is no maps plugin in this build and no map key
in the environment. The rider's coordinates are shown as numbers — the
position the server last reported. Making that a map is platform work, not a
change to what the app knows. Recorded in `docs/design-gaps.md`.

## Actions, and where they go

| Action | Where you go |
|---|---|
| back | [the order screen](order.md) |

Nothing else. This screen only watches. Closing it closes the stream.

## States

| State | What you see |
|---|---|
| connecting | a spinner |
| no rider yet | the status line and "waiting for a rider" |
| a rider carrying it | their details and last reported position, updating as frames arrive |
| ended | "tracking has ended" — the last frame said `live: false` |

## When something goes wrong

* **The stream could not be opened** — the error view with the server's
  message and a retry. Somebody else's order answers `404`, not `403`, so a
  list of order ids cannot be used to find out which are real.
* **The stream drops mid-delivery** — the screen shows the last frame it got
  and the error; reopening reconnects and the next frame is a full snapshot,
  so nothing has to be replayed.
* **No connection** — the offline message. There is no cached stream: a
  *stale* position on a live delivery is worse than none, because it looks
  current.
