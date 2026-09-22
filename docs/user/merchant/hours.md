# Hours — merchant

`HoursScreen` · `frontend/merchant/lib/src/screens/hours_screen.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To set the opening hours, which are what decide whether a customer can order
at all.

## How you get here

From the hours row on [the shop screen](shop.md).

## What you see

Seven fields, one per weekday, each holding that day's windows as text —
`09:00-22:00`, or several separated by commas. Above them, a line of help.
Below, **save**.

The weekday names are in the reader's language and Sunday is first, matching
the way the server keys the schedule.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| type windows | — | — |
| **save** | `PUT /v1/merchants/me/hours` with all seven days | back to [the shop screen](shop.md) |

**Windows go to the server exactly as typed.** It rejects overlaps and windows
that wrap past midnight — a shop trading until 2am enters `22:00-24:00` and
`00:00-02:00` on the next day — and the refusal is shown in its own words. A
second validator here would be a second opinion, and the one that matters is
the one that decides whether a customer sees the shop.

A day left empty means closed that day. That is not an error.

## States

| State | What you see |
|---|---|
| the current schedule | the seven fields, pre-filled |
| saving | the button is busy |
| refused | the server's sentence above the button, everything typed kept |

## When something goes wrong

* **Two windows overlap** — refused, naming the day.
* **A window wraps past midnight** — refused. Split it across two days, which
  the help line says.
* **A time is not `HH:MM`** — refused with the format.
* **No connection** — the offline message; the schedule is not queued,
  because a shop's hours arriving hours late is a shop taking orders it
  cannot fill.
