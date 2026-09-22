# 0008 — Flutter state, routing and injection without a package

Date: 2026-09-18
Status: Accepted

## Context
P17 had to choose a foundation for three apps that the next three phases would
be built on. The Flutter ecosystem's usual answer is a package per concern:
Riverpod or Bloc for state, go_router for navigation, get_it for injection,
freezed and json_serializable for models, intl's ARB pipeline for strings.

Three facts about this product make that default worth re-examining.

The first is that **the client decides nothing** (ADR 0004). Every price, every
status line, every enabled state arrives from the server already made. What is
left for client state is narrow: a request is in flight, or it returned data,
or it failed. That is a much smaller problem than the one a state-management
package is designed for.

The second is the **100% coverage gate**. Every code-generating package
produces files that the gate then has to be taught to ignore, and each
exclusion needs an ADR of its own. Six packages of generated code would mean
six holes in the one number this project uses to know whether it is finished.

The third is the **APK budget**. Three apps, 20 MB each, most of it already
spent on the Flutter engine and the ten font files Bengali needs.

## Options considered
1. **The ecosystem default** — Riverpod, go_router, get_it, freezed,
   json_serializable, intl/ARB. Familiar to any Flutter developer joining, and
   each package is better at its job than a hand-rolled equivalent would be at
   a larger problem.
2. **Nothing but the framework** — `ChangeNotifier` for state, Navigator 1.0
   for routing, an `InheritedWidget` for injection, hand-written `fromJson`,
   hand-written string tables.

## Decision
Option 2, with three primitives in `goklay_core` and nothing else:

* **`Store<T>`** holds a sealed `AsyncValue<T>` — loading, data (with whether
  it came from cache and when), or failure. One class, one state machine, the
  same three cases on every screen.
* **`ActionRunner`** is the write-side equivalent: busy, or not, with the last
  error. It is what disables a button while a request is in flight.
* **`Dependencies`** is a plain object handed down an `InheritedWidget`. No
  global registry, so a widget test constructs exactly what the screen it is
  testing needs and nothing more.

Navigation is Navigator 1.0 with ordinary pushes, and each app's `routes.dart`
is the one file that knows what any callback pushes. Screens take callbacks,
which is what keeps them testable on their own.

Models are hand-written `fromJson`. String tables are hand-written classes
(see ADR 0011).

## Consequences
- **Nothing is generated**, so the coverage gate needed no new exclusions for
  the whole frontend: `frontend/coverage-exclusions.txt` is deliberately
  empty, and all four Dart packages are at 100%.
- **The primitives are tested directly**, and every screen inherits that
  testing rather than each screen re-testing loading and error paths.
- **Screens are testable without a harness that boots the app**, because a
  screen's destinations are callbacks the test supplies.
- **The deepest stack is five screens** — shop → item → cart → review → place
  order — which is well inside what Navigator 1.0 handles without a URL table.
- **Accepted cost: a Flutter developer joining will look for Riverpod and not
  find it.** The three primitives are about 200 lines and documented in
  `docs/technical/frontend.md`; the bet is that reading 200 lines is cheaper
  than the generated code and the exclusions the alternative brings.
- **Accepted cost: no deep links.** Navigator 1.0 without a router means no URL
  handling. Nothing in the product needs one today — there is no web surface
  and no email link into a screen. If push notifications ever need to open a
  specific order, this is the decision to revisit, and it means adding a
  router rather than undoing anything.
