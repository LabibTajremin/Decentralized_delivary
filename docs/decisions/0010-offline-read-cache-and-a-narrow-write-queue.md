# 0010 — A read-through cache everywhere, and a write queue in exactly one place

Date: 2026-09-19
Status: Accepted

## Context
The audience is rural users on low-end Android devices and intermittent
mobile data. Two different problems hide under the word "offline":

**Reads.** A customer on a train wants to see the menu they were looking at,
their order's status, the address they are riding to. None of that has to be
current to be useful, as long as the app says it may be stale.

**Writes.** A rider taps "collected" at a shop counter with no signal and
"delivered" in a stairwell with none either. Those taps are not conveniences;
they are the record of what happened, and they must arrive.

The temptation is to treat these as one problem and build a general offline
layer that queues every write.

## Options considered
1. **Nothing offline.** Every screen needs a connection. Simple, and useless
   on the network this product actually runs on.
2. **A general write queue: every action queued when offline, replayed later.**
   Uniform, and wrong in specific places — a queued "place order" replays a
   price nobody confirmed, a queued "accept job" arrives after the offer
   expired, a queued profile edit overwrites a change made from another device
   in the meantime.
3. **Read-through cache for every GET, and a write queue only where ordering
   is the whole point.**

## Decision
Option 3.

**Reads.** `GoklayApiClient` sends `If-None-Match` and caches every successful
response. On a network failure it serves the cached copy and marks it — the
`AsyncData` carries `fromCache` and `storedAt`, and every screen that shows
cached data shows a "showing saved information" banner above it. Stale is
acceptable; stale and silent is not.

**Writes.** `OfflineQueue` exists, and exactly one screen uses it: the rider's
job screen. It is FIFO, replays strictly oldest-first, and stops at the first
action the network still cannot carry — because the order state machine refuses
`delivered` from an order that never reached `picked_up`, so out-of-order
replay would not merely be wrong, it would be rejected. An action the server
*refuses* (rather than cannot be delivered) is dropped into
`FlushReport.rejected` and reported, not retried forever: somebody else took
the job, and replaying will never succeed.

Everywhere else, a write with no connection shows the offline message and does
nothing. Each of those screens' user documentation says why, because "why is
this not queued?" is a reasonable question to ask of each one.

## Consequences
- **The one queue is visible from everywhere.** The banner lives above the
  partner app's tabs rather than inside a screen, because a rider who does not
  know something is unsent will assume it arrived.
- **Ordering is guaranteed where it matters and nowhere else**, which is the
  only place it is worth the complexity.
- **Accepted cost: the queue is per device and is not sent for you.** A rider
  who signs out with actions pending loses them, which is why the partner
  account screen says so.
- **Accepted cost: a screen showing cached data can show a button that is
  stale** — a cached `next_actions` may offer an action the server will now
  refuse. The refusal is shown and the screen re-reads, which is a worse
  experience than a correct button and a much better one than a blank screen.
- The queue was a dead API for a whole phase: P17 built it and nothing used it
  until P19. That was the right order — the alternative was designing it
  against a guess about what the rider's screen would need.
