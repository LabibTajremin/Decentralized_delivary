# Catalogue

What a shop sells: sections, items, the choices on them, and bundles.

## The per-type schema differences are one table

This is the phase's acceptance criterion, and it is the thing to read first.

A restaurant dish, a bag of rice and a strip of paracetamol are not the same
kind of thing. One has add-ons and no shelf count; one has a unit of sale and a
count; one has a strength and may need a prescription. Modelling them as one bag
of optional fields would leave every consumer guessing which fields apply.

So the differences live in `domain.Capabilities`, and every rule reads from
there:

| | restaurant | grocery | pharmacy |
|---|---|---|---|
| Variants | ✓ | ✓ | ✓ |
| Add-ons | ✓ | | |
| Combos | ✓ | ✓ | |
| Counts stock | | ✓ | ✓ |
| Requires a unit | | ✓ | ✓ |
| Prescriptions | | | ✓ |

A fourth kind of shop is a row in `CapabilitiesFor` rather than a hunt through
the use cases for the switches that forgot it.
`TestCapabilitiesAreExactlyThis` pins every cell.

The reasoning behind the rows:

- **Add-ons are a restaurant's alone.** "Extra cheese" on a box of paracetamol
  is not a gap in the product, it is a category error, and allowing it would put
  a meaningless control on every grocery item screen.
- **Only a shop with a shelf counts stock.** A kitchen cooks to order; a count
  there is a number nobody updates. A grocer and a pharmacy oversell if we do
  not count.
- **A pharmacy does not bundle medicines.** Making that easy is not something we
  should do.
- **Every type has variants,** because every type sells the same thing in more
  than one size. That is why nothing checks `Capabilities.Variants` before
  accepting variant groups — a branch no input can reach is a branch no test can
  cover. `OptionUseCase.SetVariantGroups` says so where the check would go.

The table is **served**, at `GET
/v1/merchants/{id}/catalogue/capabilities`, so the merchant app renders the
right form rather than deciding the rules itself (2.9).

## Three reasons an item is not orderable

Kept apart on purpose, because they are different things to tell a customer:

| Reason | Code | Who decides |
|---|---|---|
| The owner took it down | `unavailable` | `Item.Active` |
| It is sold out | `out_of_stock` | `Stock` |
| It is off the menu right now | `not_available_now` | `Availability` |

Collapsing them into one message is how a customer waits for a dish that is
never coming back. Being hidden outranks the rest: an owner who took an item
down does not want a customer told it is merely sold out.

`Stock.Tracked` is not "quantity > 0". A kitchen has no count *at all*, and
representing that as zero would show every dish as sold out. A counted shop with
no count given starts at zero — guessing the other way sells something the shop
does not have.

## Money

`shared/money`, new in this phase. Minor units in an `int64`, never a float: a
`float64` cannot hold 0.1, and money that cannot be added twice without drifting
is money a customer will one day see two of.

It also owns the **display string**. The thin-client rule says the app performs
no arithmetic on money and renders what the server formatted, so the formatting
has to live somewhere the server owns — and two places deciding whether a price
reads `৳1240` or `৳ 1,240` is how one screen ends up disagreeing with a receipt.

Every price crosses a module boundary as both forms. A combo's **savings** are
computed by the server for the same reason: a client working out what a bundle
saves is doing arithmetic on money, which 2.9 forbids outright.

Money lives in the shared kernel, which a domain may import — see **ADR 0007**
for why rule 2.2 was amended and how narrowly.

## Variants and add-ons

A **variant** is a mutually-exclusive choice within one item — a size, a
strength, a pack. Its price is a **signed delta**, because a small size
legitimately costs less and modelling that as a separate item would double every
menu.

An **add-on** is an extra bought alongside, and carries a **price**, not a
delta: an extra is a thing with a price, and expressing "extra cheese, ৳30" as an
adjustment to the pizza makes a receipt impossible to read. An add-on may not
cost less than nothing — a negative one is a discount wearing a disguise.

Groups are **replaced wholesale**, never edited one option at a time. A group has
invariants across its members — how many may be chosen, no two names alike — and
patching one option at a time means every intermediate state has to be legal,
which "pick exactly one of" cannot survive while the last option is being
removed.

**Option ids are kept when the owner sends them back.** A cart holds the option
id a customer picked; minting a fresh one on every save would invalidate every
cart in flight the moment a typo was fixed.

Two options in a group may not share a name, case-insensitively. The name is
what a customer picks by, and two "Large" on one pizza is a support call
whichever one the kitchen makes.

A required group forces a minimum of one: a pizza has no price until a size is
picked, so leaving it optional would mean charging for something nobody chose.

## Combos

A combo's price is **stated, not derived** from its members. A combo exists to be
cheaper than its parts, and computing it from them would move the discount every
time a member's price changed — including upward.

A combo needs at least two lines, because a "combo" of one item is a price
change. One line per item: "two of these" is the quantity, not a second row.

A combo is orderable only when **every** item in it is. A meal deal whose drink
is sold out is a meal deal the kitchen cannot make, and letting it be ordered
moves the disappointment from the menu screen to the doorstep. A bundle priced
above its parts reports a saving of nothing rather than rendering as
"save -৳40".

Membership is checked against the repository, not trusted from the ids. A combo
that resolves to nothing at checkout is a failure the customer discovers and the
owner does not. The database backs this up: `catalogue_combo_lines.item_id` is
`ON DELETE RESTRICT`.

## Availability

An item's availability is a `shared/schedule` timetable, the same value object
as a shop's opening hours (P06) — extracted in this phase so a second copy of
"parse HH:MM" cannot handle midnight differently. Windows are half-open and may
not wrap past midnight; something running until 2am is two windows.

The default is **always**, which is what almost every item is. A breakfast menu
that ends at 11am is the exception, so it is the thing that takes configuring. A
schedule that never opens is refused — it makes an item invisible in a way that
looks like a bug to its owner, and hiding it is what `Active` is for.

Unlike opening hours there is **no cap on windows a day**: a shop with more than
three shifts is a data-entry accident, but there is no natural ceiling on when an
item may be sold.

## Deleting things

| Deleting | Effect |
|---|---|
| A section with items | **Refused.** Cascading would silently delete a shop's food; reassigning the items would be a decision the owner did not make. Empty it first. |
| An item in a combo | **Refused** by the database, for the reason above. |
| An item | Its option groups cascade. |
| A combo | Its lines cascade; the items are untouched — a bundle is an offer over things that exist independently of it. |

## Bulk update

One transaction. A half-applied re-pricing is worse than none: a menu where the
first thirty items moved and the rest did not is a shop selling at two price
lists, and the owner cannot tell where the boundary fell. An unknown item id
therefore refuses the **whole** request rather than skipping a line — a bulk
update that silently applied to 28 of 30 items and reported success is a shop
that thinks it re-priced its menu.

Every field in a change is a pointer, so "leave this alone" and "set this" are
different instructions. Without that, a shop changing prices would have to send
stock counts back too, and one that forgot would zero its inventory.

## Reading a menu

`GET /v1/catalogue/{merchantId}/menu` returns sections, items and combos in one
call. Three round trips on a village 2G connection is the difference between a
shop that opens and one the customer gives up on.

**Hiding a section takes its items with it**, so an owner hiding "Winter
specials" in February does not have to hide each dish too.

The read is **public**. Requiring a token to browse a menu would put a sign-up
wall in front of the thing customers came for. Nothing on that surface carries a
shelf count, a visibility switch or an availability schedule — see the split
between `PublicItem` and `Item` in the OpenAPI spec, and
`TestAGrocerySeesItsShelfCountAndACustomerDoesNot`.

Inside the repository, a menu costs a fixed number of queries rather than one
per item: the option groups for every item on the page are fetched in two
queries after the item rows are closed. Closing first matters — pgx holds one
connection per open result set, and a nested query on the same connection would
deadlock against the pool under load.

## Ownership

Every owner route carries a merchant id in the path, and every one verifies
against the merchant record that the caller owns that shop. Without it, any
signed-in account could rewrite any menu in the country. `TestOnlyTheOwnerMayChangeAMenu`
walks the whole surface.

The catalogue reaches the merchant module through its own `external/merchant`
package, depending only on `MerchantContract` (2.5). One method: it needs to know
a shop exists, who owns it, and what kind it is — the kind decides the shape of
everything in its catalogue.

## What the contract leaves out

`CatalogueContract` carries what discovery, cart and order need. It does **not**
carry shelf counts, the visibility switch, or the availability schedule: those
are the owner's, and a contract that handed a competitor's stock levels to
discovery would be telling them exactly how a shop is doing.

`Items` returns results **in the order asked for**, so a cart revalidating its
lines can zip the result against its own without a second index. An id that no
longer exists is simply absent rather than failing the batch — the caller is
holding lines from before the shop edited its menu, and it needs to know which
survived.
