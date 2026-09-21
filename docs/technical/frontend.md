# Frontend foundation

> P17. What the customer, merchant and partner apps all stand on: the design
> tokens read out of Figma, the theme built from them, the accessibility floor,
> the Bengali-first localisation, and the API transport. No screens — P18 and
> P19 build those.

## Layout: one workspace, one lockfile

```
frontend/
  pubspec.yaml          # Dart pub workspace root — not a shippable package
  analysis_options.yaml # the lint rules every package here inherits
  coverage-exclusions.txt
  goklay_core/          # this phase
  customer/ merchant/ partner/   # P18, P19
```

`frontend/pubspec.yaml` is a pub workspace root. One `flutter pub get`
resolves every package under `frontend/` against a single lockfile, and one
`flutter analyze` at that directory covers all of them — which is exactly what
`scripts/verify.sh` already ran before any of this existed.

`goklay_core` is a package, not an app. It has no `android/` directory because
nothing here is installable on its own, which also means `apk-size-check.sh`
correctly reports "no Android app yet" until P18 adds one.

One change was needed in `scripts/flutter-coverage-gate.sh`: it walks every
`pubspec.yaml` under `frontend/` and runs `flutter test` in each, and a
workspace root has no tests of its own. It now skips a pubspec that declares
`workspace:` *and* has no `test/` directory. A package with both is still
measured, so nothing can hide behind the skip.

## Tokens come from Figma, and say where from

Read from `NlVjn8OuvmLjbm8z8TDVlR` on 2026-09-21. Every token carries the
Figma variable name or the node id it came from, so it can be checked against
the file without guessing which screen it was on.

The whole product uses **seven colours**, which is what the file defines:

| Token | Figma variable | Value |
|---|---|---|
| `black` | Black | `#0F0F0F` |
| `gray` | Gray | `#656565` |
| `white` | White | `#FFFFFF` |
| `softGray` | Soft Gray | `#F8F9FA` |
| `primary` | Primary Color | `#22874F` |
| `emerald` | Emerald Green | `#10B981` |
| `softRed` | Soft Red | `#FF5A5F` |

Plus one derived token, `primaryTint` — the brand at 15% over white, which the
design uses behind a promo code (node `1:289`). It is stored **opaque** rather
than translucent, because a translucent fill has no contrast ratio of its own
and the contrast test has to be able to check it. A test asserts it really is
the composite rather than a colour someone eyeballed.

The theme names every colour explicitly instead of using
`ColorScheme.fromSeed`. Seeding generates a tonal palette from one input, whose
primary is not even the colour you seeded with — a test asserts exactly that,
so nobody reaches for the convenient constructor later and quietly ships a
different product from the design.

Type, read from nodes `1:72`, `1:73`, `1:286`, `1:287`, `1:290`, `1:298`:
Manrope Bold 30 for display; Poppins at Light/Regular/Medium/SemiBold for
everything else, at 12, 14 and 16.

## The design's fonts cannot render the product's language

Manrope and Poppins contain **no Bengali glyphs**. Not "poor coverage" —
zero; the cmap of every shipped weight was checked, and `U+0995` (ক) is absent
from all of them. Bengali is this product's default language (1.4).

Shipping the design's two fonts alone means every Bengali screen renders
through whatever the device substitutes, which on a low-end Android is a
different face per line or a row of empty boxes.

So Noto Sans Bengali ships alongside them and is the `fontFamilyFallback` on
every style in the scale. Latin text keeps the faces the design chose; Bengali
falls through to one that can draw it. Noto Sans Bengali covers Latin as well,
so "GoKlay" or a digit inside a Bengali sentence stays in one face rather than
switching mid-line. A test walks the whole scale and fails any style that
loses the fallback.

Ten font files, 1.4 MB total, against a 20 MB APK ceiling.

## The accessibility floor is a test, not a convention

**48dp touch targets.** The Figma button is 12pt of padding around a 14pt
label — about 41dp, under the floor Material and WCAG 2.2 both ask for. The
answer is not to redraw the design bigger. `GoklayTapTarget` lets the control
paint at its designed size and makes the *hit area* 48dp around it, so the
screen matches the file and the control matches the standard. `GoklayButton`
goes through it, and `ThemeData.materialTapTargetSize` is `padded` so a stock
`IconButton` gets the same treatment.

**Contrast.** `GoklayContrast` implements WCAG 2.1 ratios, and a test asserts
every foreground/background pair this library actually paints clears AA. Three
pairings the design uses do not, and each is recorded as its own passing test
rather than a comment nobody reads:

| Pairing | Ratio | Verdict |
|---|---|---|
| brand green on its own 15% tint (the promo code, node `1:290`) | 3.73:1 | large text only — the design sets it at 16px SemiBold, which WCAG does not count as large |
| Soft Red on white | 3.05:1 | large text only; a status colour, not a caption colour |
| Emerald on white | 2.54:1 | never text at any size; a fill or a dot beside a label |

For the promo code specifically, the test also asserts the way out:
`textPrimary` on that same tint clears AA comfortably.

## Localisation: Bengali first, and a deliberately tiny table

`GoklayStrings` has seven strings. That is the point. Every sentence a user
reads about their order, their cart, their delivery or their money is composed
by the server and arrives ready to paint (2.9). What is left is the handful of
words that exist *before* a server answers, or when one never does: retry,
cancel, close, loading, the offline banner, the queued-offline note, and the
fallback for an error that arrived with no message.

Because it is seven strings, it is hand-written rather than generated from ARB
files. Codegen earns its keep across hundreds of strings; across seven it buys
nothing and costs a generated file the 100% coverage gate would then have to be
taught to ignore — which is why `frontend/coverage-exclusions.txt` is empty.

Bengali is first in `supportedLocales`, so a phone set to Hindi, Arabic or
anything else this app has never heard of resolves to Bengali rather than
English. `lang=en` is attached to API requests only for English, because the
server is Bengali-first too.

Tests assert the Bengali table is actually in Bengali script (the block
`U+0980..U+09FF`), that a button fits its Bengali label on a 320dp screen —
narrower than the 430dp the design is drawn at, and about what a budget Android
actually is — and that the longest Bengali sentence wraps rather than
overflowing.

## The transport

`GoklayApiClient` owns the three things that would otherwise be repeated at
every call site and eventually forgotten: the language parameter, the bearer
token (read fresh per request, so a refresh lands without rebuilding the
client), and freshness.

Reads send `If-None-Match` and store the server's `ETag`, so an unchanged
screen costs a 304 and no body. When the network is gone, a read falls back to
the cache — the app has to open in a lift and in a village with one bar of
signal. `cached()` is the first half of stale-while-revalidate: paint that
instantly, then call `get()` and repaint.

Writes never cache and never fall back. Serving a stale body in place of a
failed write would tell somebody their order was placed when it was not.

`OfflineQueue` is for the partner app. A rider taps "collected" and
"delivered" in places with no signal, and those taps can neither be lost nor
replayed out of order — the order state machine refuses `delivered` from an
order that never reached `picked_up`. So replay is strictly FIFO and stops at
the first action the network still cannot carry, leaving it and everything
after it queued. An action the server *actively refuses* is dropped and
reported: a 409 will be a 409 in an hour, and retrying it forever means the
queue never drains.

The client does not retry, back off, or refresh tokens on its own. Those are
policies with product consequences, and burying them in a transport is how an
app ends up hammering a failing server.

## The bug this phase found in the backend

`package:http` decodes a response body using the charset in the `Content-Type`
header and **falls back to latin1** when there is none. The backend was sending
`application/json` with no charset. JSON is UTF-8 by definition (RFC 8259
§8.1), so every Bengali sentence the server composes would have arrived as
mojibake in the client — and since Bengali is the default language, that is
every screen of the product.

It was fixed on both sides, and both fixes are load-bearing:

* `httpx.WriteJSON` now sends `application/json; charset=utf-8`, which helps
  every client, not only this one.
* The transport decodes `bodyBytes` as UTF-8 itself rather than trusting the
  header, because the client must not depend on the server remembering to
  label it. A test sends a deliberately unlabelled Bengali body and asserts it
  survives.

This is the kind of defect that only appears when a client is actually pointed
at the real server's headers, which is why the transport was built in the
foundation phase rather than left to the first screen.

## The thin-client lint, proven

`scripts/thin-client-lint.sh` is the only automated defence for 2.9, and until
this phase it had never had a single Dart file to look at. P17's acceptance
says it has to be wired *before* any screen exists, and a lint nobody has
watched fail is not known to work.

`scripts/thin-client-lint-selftest.sh` feeds it six violations — a fee used as
an operand, a distance scaled, a total computed, a radius computed, a COD limit
hardcoded, a free-delivery threshold hardcoded — and asserts it rejects each
one. It then feeds it three legal patterns — painting a preformatted money
string, reading a minor unit without computing on it, adding two layout
numbers — and asserts it accepts them. It runs in `verify.sh` and in CI,
with or without Dart present.

`Money` is the structural half of the same rule: it carries `minor`,
`currency` and `display`, and has no arithmetic at all. No `operator +`, no
`times`. A client that could add two amounts would eventually be asked to, and
the number it produced would disagree with the receipt.

## What P18 and P19 inherit

* Add the app to the `workspace:` list in `frontend/pubspec.yaml` and depend on
  `goklay_core`.
* Every interactive widget goes through `GoklayTapTarget`, or the 48dp test is
  the thing that notices.
* Lay out in Bengali and check at 320dp. English will fit if Bengali does.
* If a screen needs a number the API does not return, the fix is a backend
  change — never a calculation in the app.
* State management was deliberately left unbuilt. The rule is one approach
  across all three apps (2.9), and the right one depends on what the first real
  screens need; choosing it here, with no screen to test it against, would have
  been guessing. Flutter's own `ValueNotifier`/`ListenableBuilder` is the
  default to beat, and beating it needs a reason written down in the PR.
