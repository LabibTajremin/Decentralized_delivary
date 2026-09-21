# The merchant app

> P19. Register a shop, keep its menu, work the order board. Built on the
> foundation P17 laid and the half of P18 that P19 moved into it.

## There were no frames for this

The Figma source has none. Of 10,456 named nodes, a search for *merchant*,
*vendor*, *shop owner*, *rider*, *courier* or *driver* returns three
decorative labels inside customer screens. Every frame in the file is
customer-facing. `docs/design-gaps.md` records it, and records the answer:
the layout comes from the API — each screen exists because an endpoint does —
and the look comes from `goklay_core`, which *is* the Figma file. Applying an
existing design system is not designing a new one.

## What the screens read instead of deciding

| The screen wants to know | It reads | It does **not** |
|---|---|---|
| whether the shop may be submitted | `Merchant.can_submit` | count the documents it holds |
| what papers are still wanted | `missing_documents` | subtract uploaded from required |
| which papers a type needs at all | `registration-requirements` | keep a table of regulators' rules |
| whether the shop is open | `is_open_now` / `open_status` | compare the clock to `hours` |
| why it was refused | `review_note` | map a status to a sentence |
| what this shop type may sell | `catalogue/capabilities` | branch on `type == 'pharmacy'` |
| whether an item is orderable | `Item.orderable` | combine `active` and `stock_quantity` |
| what the shop may do to an order | `Order.next_actions` | keep a copy of the state machine |

The capabilities response is the one worth dwelling on. It says whether this
shop has variants, add-ons, bundles, a shelf count, a required unit and
prescriptions, and which units to offer. The catalogue screen builds its tabs
from it, the item form builds its fields from it, and neither ever looks at
the shop's type. A field this kind of shop does not have is **refused** by the
server rather than dropped, so an owner never silently loses something they
typed — which is exactly why the form shows only what will be accepted.

## Money goes in as an integer

The item form's price field takes **poisha**, not taka. That is deliberate:
money crosses this wire as a minor-unit integer and nothing else, and a
decimal field would mean parsing one into the other here — an arithmetic step,
on money, in the client. Everything the owner reads back is the server's
formatted string.

## Opening hours are sent as typed

Windows go up exactly as the owner wrote them, keyed by weekday with Sunday as
`"0"`. The server rejects overlaps and windows that wrap past midnight — a
shop trading until 2am enters `22:00-24:00` and `00:00-02:00` on the next day
— and the refusal is shown in its own words. A second validator here would be
a second opinion, and the one that matters is the one that decides whether a
customer can see the shop. A day left blank is left out of the request
entirely, because that is what "closed" is.

## Going on holiday is a call, not a flag

Being on holiday is what takes a shop off every customer's list. An app that
hid the shop by itself would still be taking orders.

## The two gates every session opens with

`GET /v1/merchants/me` answers 404 for an owner with no shop, which is how the
app knows to show the registration form rather than keeping a local "have I
registered yet" flag that a reinstall would lose and a second device would
disagree with. The same shape is used in the partner app.

## What is deliberately absent

* **No file picker** on the document form. This build has no camera or storage
  plugin, so the form takes the location of a file already uploaded.
* **No map** on the address fields, for the same reason the customer app has
  none: picking a point on a map is platform work, and the area, district and
  division are resolved by P02 from the coordinate either way.

## Tests

100%, in the same shape as the customer app's: models, endpoints against a
fake backend that fails loudly on an unrouted path, screens, and one file that
drives the whole app from sign-in through every tab.

Two things worth knowing before adding to them, both in `test/support/`:
`reveal` drags a `ListView` rather than calling `scrollUntilVisible`, because
a form screen has a `Scrollable` inside every `TextField` and naming the right
one is fiddly; and it scrolls back to the top first, because a previous reveal
may have walked past the widget and everything above the fold has been
disposed since.
