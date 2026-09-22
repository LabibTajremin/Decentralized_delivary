# Notification

> P14 (the other half — see also `tracking.md`). Push first, falling back
> to SMS. Depends on P04 (identity, for the fallback's phone lookup).

## Two nouns

A **notification** is one message this platform told somebody, and where
it actually went — `push`, `sms`, or `none` when neither channel could
deliver it. A **device token** is one phone a user's app has registered to
receive push on, at most one per platform per user: a reinstalled app
registers a new token, replacing the old one rather than accumulating a
stale one nothing would ever clean up (`SaveDeviceToken`'s
`ON CONFLICT (user_id, platform) DO UPDATE`).

## Composing is the caller's job, delivering is this module's

`NotificationContract.Notify(ctx, userID, title, body)` takes an
already-composed message (2.9 — the module that knows what happened, and
in which language, composes it; notification only knows how to reach the
person). Order is the one real caller this phase wires: `tellCustomer` in
`order/application/transitions.go` fires on the milestones worth
interrupting a customer for — pickup, delivery, cancellation, rejection,
a failed delivery — each with a Bengali message naming the order's own
code, and stays silent on the moves that happen far more often (accepted,
preparing) that would just be noise mid-wait. Composed in Bengali only:
this is a system-triggered message with no request to read a `lang` from
the way a screen read has one, so it uses 1.4's default rather than adding
a dependency on the customer's own language preference for a feature this
phase does not otherwise need.

Order reaches notification the same way it reaches dispatch and payment —
its own `external/notification` seam, wired with a `UseNotification`
setter after construction in `cmd/api/main.go`. Not a service-level cycle
the way dispatch and payment are (notification never calls back into
order), but wired identically anyway, so cmd/api's cross-module wiring
reads one way rather than half by constructor argument and half by
setter. Best-effort, the same tolerance dispatch and payment outages get:
a notification outage must not stop a shop, a rider or an admin moving an
order along.

## Push, tried on every device, falling back to SMS only if none worked

`NotifyUseCase.Notify` sends to **every** device a user has registered,
not just the first one that accepts it — a person signed in on a phone and
a tablet should hear on both, and stopping at the first success would
leave the second silent for no reason the first device's own state
explains. SMS is attempted only when push reached nobody: no device was
ever registered, or every device refused it. Whichever channel actually
delivered is recorded on the notification (`GET /v1/me/notifications`,
the caller's own inbox); a delivery on no channel at all is recorded too,
as `channel: none, status: failed`, and `Notify` itself returns an error
for the caller.

The phone SMS falls back to is `IdentityContract.PhoneFor` — a small,
targeted addition to identity (P04), the same precedent P13 set adding
`DispatchContract.PartnerOfUser` for payment's COD ledger: the one number
this platform already holds for a person is the one they signed in with,
so a second phone-collection flow was never needed. Identity had never
before needed to construct its own `Service` in `cmd/api/main.go` —
nothing consumed `IdentityContract` until this phase — so it is wired in
for the first time here, alongside `notificationRepo`, `notificationPush`
and `notificationSMS`.

## The adapters — 2.4

`infrastructure/push/log` and `infrastructure/sms/log` are both log-only
stand-ins until a real provider (FCM/APNs, an SMS gateway) is chosen, and
both refuse to be constructed in production — the same guarantee
identity's own log SMS sender makes for OTP codes, for the same reason: a
message written to the log is readable by anyone with log access, which in
a real deployment is a great many more people than should see what was
sent to whom.

## A real bug a naive fix would have re-introduced

The very first version of the "every device gets tried" loop still broke:
a test asserting both of a user's two devices were attempted was flaky,
failing on some runs and passing on others. The cause was Go's map
iteration order — `DeviceTokensForUser`'s in-memory fake stored tokens in
a `map[Platform]DeviceToken`, and the *original* loop stopped at the
first successful push, so which device "won" (and therefore whether the
second was ever tried) depended on iteration order the language
deliberately randomizes. The fix was not to sort the fake's iteration —
that would have hidden a real design question — but the one described
above: try every device unconditionally. That is simultaneously the more
correct product behavior and the fix that makes the outcome
order-independent, which is why "the test is flaky" turned out to be
pointing at a genuine bug in the use case, not a test that needed to be
more careful.

## What P14's tests found

- **`httpx.Logging`'s wrapper silently broke every future SSE endpoint.**
  See `tracking.md` — `statusRecorder` embedding `http.ResponseWriter` as
  an interface field promotes only that interface's methods, never
  `Flush`. Caught by reading the middleware chain before writing the
  stream handler, not by a failing test; the fix (`statusRecorder.Flush`
  forwarding to the real writer) is covered directly in
  `backend/tests/unit/httpx`.
- **The full coverage gate had not actually been run before P13 was
  committed.** A background verification process this session inherited
  reported 100% and "every gate passed," and P13 was committed on the
  strength of that log rather than a fresh run. Restarting the full gate
  at the start of this phase found the same payment repository and
  handler gaps the prior session's own notes had already identified as
  outstanding — they had never been closed. Closed now, alongside this
  phase's own gaps, as part of getting `./scripts/verify.sh` genuinely
  green rather than trusting a stale log a second time.
- **"Try every device" is both the correct design and the fix for a real
  test flake.** See above.

## What notification does not do

It never decides what a message says or who it is about — that is every
caller's own job, kept out of this module on purpose so a second caller
(dispatch offering a rider work, identity confirming a sign-in — both
listed as future consumers in Appendix A) can reach `Notify` without
notification growing any opinion about orders. It never retries a failed
send on its own; the record of a failed attempt is the whole recovery
story this phase needs, and a retry policy is a decision for whichever
real gateway eventually replaces the log-only stand-ins.
