# 0009 — One shared Dart package, in a pub workspace

Date: 2026-09-19
Status: Accepted

## Context
Three apps — customer, merchant, partner — with genuinely different jobs and a
large amount of genuinely identical machinery: the same design tokens, the same
theme, the same accessibility floor, the same transport with its ETag cache and
offline fallback, the same state primitives, the same sign-in flow.

P18 built the customer app first, and by the end of it that machinery existed
once, inside `frontend/customer/`. P19 then had to build two more apps.

## Options considered
1. **Copy it into each app.** No coupling; a change to the merchant app cannot
   break the customer one. Also three copies of the sign-in flow, and the
   second bug fixed in two of them.
2. **A shared package, `goklay_core`, with the three apps depending on it.**
   One place for the machinery; a change there lands in three apps at once,
   which is both the point and the risk.
3. **One app with three modes, chosen at launch.** One binary, one APK to
   ship — and a rider's phone carrying the merchant app's code, three sets of
   screens in one bundle against a 20 MB budget, and a role switch as the only
   thing between a customer and a shop's order board.

## Decision
Option 2, and it was done as the **first task of P19** rather than
incrementally: the app-agnostic half of the customer app was promoted into
`goklay_core` before either new app was started, so that merchant and partner
were built on the shared package rather than being refactored onto it
afterwards.

Wiring is **Dart pub workspaces** — `workspace:` in the root `pubspec.yaml`,
`resolution: workspace` in each member — so all four packages share one
lockfile and one `.dart_tool`. A path dependency would have worked; a
workspace additionally makes `flutter pub get` at the root resolve everything
once and keeps four packages from drifting onto four versions of the same
dependency.

What lives in `goklay_core`: tokens, theme, the accessibility floor, the
localisation tables and delegate, `GoklayApiClient`, `OfflineQueue`, `Session`,
`Store`/`ActionRunner`/`AsyncValue`, the shared widgets, the shared models, and
the whole sign-in flow parameterised by role.

What does not: anything that knows about an order, a shop or a delivery. Each
app owns its own endpoint clients, its own models and all of its screens.

## Consequences
- **Merchant and partner are mostly screens**, which is why P19 fitted in one
  phase rather than three.
- **The sign-in flow is written once**, and the role is a parameter — which is
  also why the role is sent to `POST /v1/auth/otp/verify` rather than inferred:
  one screen serves three apps and has to say which it is.
- **A change in `goklay_core` lands in three apps at once.** That is the risk,
  and the mitigation is the coverage gate: `goklay_core` is at 100% on its own,
  and all three apps' suites run on every change.
- **Accepted cost: the apps cannot diverge cheaply.** A merchant-only tweak to
  a shared widget is a change to all three. In practice the shared half is the
  half where divergence would be a bug, which is how the line was drawn.
- **Three APKs, ~17.2–17.5 MB each** against a 20 MB ceiling, recorded in
  `frontend/apk-size-baseline.txt` with a gate that refuses more than 10%
  growth. Option 3 would have been one larger APK and a much worse security
  story.
