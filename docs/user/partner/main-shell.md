# Main shell — partner

`PartnerShell` · `frontend/partner/lib/src/screens/main_shell.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To hold the four places a rider works in — and to keep the outbox visible from
all of them.

## How you get here

As soon as `GET /v1/partner` returns a record. Before that the app shows
[the registration form](register.md).

## What you see

**The outbox banner, above the tabs**, whenever anything is waiting to be
sent: how many actions are queued, and **send now**.

Beneath it, four destinations:

| Tab | Screen |
|---|---|
| Feed | [what to look at now](feed.md) |
| Jobs | [the rider's own deliveries](jobs.md) |
| Cash | [the cash-on-delivery ledger](cash.md) |
| Account | [distance choice, position, sign out](account.md) |

**Why the banner is above the tabs rather than inside one.** The queue is not
a screen's business: a tap made on [the job screen](job.md) and still unsent
has to be visible from wherever the rider happens to be. A rider who does not
know something is unsent will assume it arrived.

## Actions, and where they go

| Action | What happens |
|---|---|
| tap a tab | switches to it, keeping its data |
| **send now** | flushes the outbox, oldest first, and stops at the first action the network still cannot carry |
| sign out, from Account | the shell is replaced by the sign-in screen |

## States

| State | What you see |
|---|---|
| nothing queued | no banner at all |
| actions waiting | "waiting to send", with the count and **send now** |
| flushing | the button is busy |
| the server refused a queued action | "an action could not be sent" — it is dropped from the queue rather than retried forever, because an action the server rejects will be rejected again |

## When something goes wrong

* **The flush hits the network again** — it stops at the first failure and
  leaves the rest queued, in order. Order matters absolutely here: the order
  state machine refuses `delivered` from an order that never reached
  `picked_up`.
* **An action was refused rather than undeliverable** — it is reported in the
  banner and removed. Somebody else took the job, or it was already
  delivered — replaying it would never succeed.
