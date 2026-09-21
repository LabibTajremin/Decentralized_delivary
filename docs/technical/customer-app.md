# The customer app

> P18. Find a shop, fill a cart, place an order, watch it arrive — and, when
> it has, say whether it was any good. Twenty-seven screens on the foundation
> P17 built, and not one business rule among them.

## The screen registry, and what is missing from both sides

Read from Figma `NlVjn8OuvmLjbm8z8TDVlR` on 2026-09-21 and reconciled against
`api/openapi.yaml` before a line of it was written. The reconciliation is
`docs/design-gaps.md`; the short version:

* The file draws the shopping flow **three times** — food (`1:7128`…),
  pharmacy (`1:10754`…), grocery (`1:16055`…) — and the three are the same
  eleven screens with different items in them. Built once. The vertical is a
  `type` filter on discovery, which is how the backend models it too.
* The home screen is drawn three times for the same reason: one screen, three
  chips.
* **Eight Figma screens have no backend at all** — offers, promo codes,
  referrals, a wallet, a business profile, safety copy. Each is built as a
  placeholder that says, in the customer's language, that it is not available
  yet. None of them invents a balance, a code or an offer: a plausible-looking
  fake is worse than an honest blank, because somebody eventually believes it.
* **Four backend surfaces have no Figma screen** — notification history,
  support tickets, reviews, active sessions. Three of them were built in P14
  and P16, after the design. They are plain screens reached from Account.

## The bottom bar is not the design's

The file's bar is Home, Offers, Activity, Account. Offers is one of the eight
with nothing behind it, and spending one of four tab slots on a screen that
can only apologise is worse than moving it into Account — which is what this
does. The cart takes the slot: it is the screen a customer returns to
constantly, and the design otherwise reaches it only from inside a shop.

## No rule, no arithmetic, no sentence of its own

`scripts/thin-client-lint.sh` is the automated half of 2.9 and it passes, but
the interesting part is what the screens read instead:

| The screen wants to know | It reads | It does **not** |
|---|---|---|
| whether checkout is live | `Cart.orderable` | check the minimum, the hours, the stock or the address |
| why it is not | `Cart.blocker_text` | map a code to a sentence |
| what a line costs | `CartLine.line_total` | multiply a price by a quantity |
| what the bill is | `CartPricing.rows` | add anything up |
| how much more earns free delivery | `away_from_free_delivery` | subtract a total from a threshold |
| whether an item can be added | `PublicItem.orderable` | look at stock or the clock |
| how far a shop is | `DiscoveryMerchant.distance` | compute a haversine or format a number |
| whether a shop is open | `is_open_now` / `open_status` | compare the time to a schedule |
| whether widening is possible | `expansion.can_expand` / `at_ceiling` | know the ladder, the surcharge or D3 |
| whether an order can be cancelled | `cancel.allowed` / `cancel.text` | count seconds down to zero and decide |
| what may happen next | `Order.next_actions` | keep a copy of the state machine |
| whether tracking is finished | `TrackingSnapshot.live` | keep a list of terminal statuses |
| what a shop is rated | `Rating.average` | average the reviews on screen |

The one thing the app does compute is the *shape of a picker*: an
`OptionGroup` says required or not and how many choices it takes, and the item
screen uses that to decide whether "add to cart" is live. `POST /v1/cart/items`
checks the same rules again and refuses a selection that breaks them, so a
mistake here produces a rejected request rather than a bad cart.

`CustomerStrings` has about 120 strings and every one of them is chrome: a
heading, a field label, a tab name, or something the app must be able to say
before any server has answered. Nothing in it interpolates a number, an amount
or a status. Both tables are tested getter by getter, and a test asserts the
Bengali one is really in the Bengali block `U+0980..U+09FF`.

## What P18 needed that the backend did not have

**`merchant_id` and `partner_id` on an order.** P16 refuses a review whose
subject was not actually on the order, and the customer's order response named
neither the shop nor the rider — so the review screen had no id it was
entitled to send. Both were added to the order view:

* `merchant_id` comes straight off the order.
* `partner_id` comes from the `picked_up` event's actor, because the order
  holds no rider field — dispatch owns the assignment, and the event is the
  order's own record of who actually took it. It is absent until somebody has.

The review screen offers the rider only when there is one, which is also the
rule that stops it producing a `403` the customer could do nothing about.

## The three screens that are honest about a missing platform

None of these is a gap in the backend; each is a platform capability this
build does not have, and inventing one would have been worse than saying so.

**Tracking draws no map.** There is no maps plugin in the build and no key in
the environment. The rider's last reported position is shown as coordinates
beside their name and vehicle. Making that a map is platform work; it does not
change what the app knows.

**Payment launches nothing.** The one gateway P13 ships is the manual one,
which records the attempt and returns an **empty** `redirect_url` — a human
confirms it through the webhook. So the screen shows the payment's state and a
way to re-read it, and shows a link only if a gateway ever supplies one.
`POST /v1/payments/checkout` is safe to call again, which is what makes a
customer who closed the page and came back an ordinary case rather than a
double charge.

**Adding an address takes a coordinate, not a pin.** Same reason. The point is
confirmed through `GET /v1/geo/resolve` before saving, so a coordinate outside
every division is refused while the customer is still looking at the form
rather than at checkout — and the area, district and division are never sent,
because P02 resolves them and a client that guessed would eventually file an
address in a division the order could not be served from (D3).

## Why the home screen has no geolocation

Every figure on a shop card — the distance, the delivery fee, whether the shop
is in range at all — is computed by discovery from a **delivery point**, and
the delivery point is an address the customer saved, not wherever the handset
is standing. So the home screen searches around the default address, and a
customer with no address is asked for one rather than shown a list that would
be priced against a place they are not.

Widening is the one search parameter the customer controls, and even that is
not a number the app picks: it sends `expansion.next_level`, only when
`can_expand` is true, and stops offering when `at_ceiling` says the division
boundary has been reached.

## State, routing and the graph

* **`Store<T>` and `ActionRunner`**, both `ChangeNotifier`. No state-management
  package. A thin client that mostly paints server responses does not have the
  kind of state a framework exists to tame, and `AsyncValue` is sealed so a
  screen cannot forget one of loading / data / failure.
* **`Dependencies`** is a single object built at startup and handed down the
  tree through `AppScope`. Not a locator: a test constructs one over a fake
  HTTP client and gets the whole app wired to it, with no global to reset.
* **Navigator 1.0**, no router package. `lib/src/routes.dart` is the only file
  that knows what any callback pushes; every screen takes callbacks and is
  testable on its own.
* Signing out **pops to the root first** when it happens from a pushed screen.
  The app's two halves are swapped underneath the navigator, so a screen left
  on the stack would keep a signed-out customer looking at their own device
  list. The test for that found the bug.

## The cart is never held locally

Every change round-trips, and the screen paints the cart that came back. The
server revalidates against the live shop on each call — a price can move, an
item can sell out, the shop can close between adding and checking out — so an
optimistic local count would be wrong exactly when it mattered. The same
reasoning is why the cart tab is rebuilt by key after something is added from
a shop rather than being told what was added.

## What the tests caught

Every one of these is a real defect the widget tests found, not a test that
needed adjusting:

* **The save button on the address form never enabled.** `lat` and `lng`
  decide whether there is anything to send, and nothing was listening to those
  two controllers, so typing a coordinate did not rebuild the button.
* **The device list never showed a failure.** "Sign out everywhere" reported
  its error into a subtree that was not listening to the `ActionRunner`, so a
  refused revocation looked like nothing happening at all.
* **Signing out from the device list left the device list on screen** — the
  pop-to-root above.
* **Two different rows read `নিরাপত্তা`.** Account security and the Figma
  "GoKlay Safety" screen had been given the same Bengali word, which a test
  found by matching two rows where it expected one. Safety is now `সুরক্ষা`.

## Coverage

100%, measured by `scripts/flutter-coverage-gate.sh`, with
`frontend/coverage-exclusions.txt` still empty. `lib/main.dart` is not in the
report because no test imports it: it is three lines of `runApp` wiring, and
the tests drive `GoklayCustomerApp` directly.

The last eight uncovered lines were all real controls nobody had tried — the
stepper's minus button, two of the four type chips, the live tab after the
past one, cash after online, the five gap rows, and the fallback that picks
the first address when none is default. Each got a test rather than an
exclusion.

Three things in the suite are worth knowing before adding to it:

* **`pumpAndSettle` never returns over a spinner.** The harness pumps twice
  instead, which is enough for the localisation delegates.
* **Flutter builds no semantics node for an off-screen widget**, so
  `find.bySemanticsLabel` comes back empty for a button that exists but is
  below the fold in a `ListView`. `revealAndTap` scrolls first — through the
  list's own scrollable, because every `TextField` on a form has one too.
* **A stream ending runs on the real event loop**, not the fake one a pump
  drives, so the tracking tests wrap a short delay in `tester.runAsync`.
