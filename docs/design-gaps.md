# Design gaps

> Where the Figma source and the built backend do not line up. Recorded rather
> than resolved by invention: §174 of the build instruction says the agent does
> not design a screen that is not in Figma, and does not quietly drop one that
> is.

Read on 2026-09-21 from Figma `NlVjn8OuvmLjbm8z8TDVlR`, page
`06 Source — GoKlay v1`, against `api/openapi.yaml` as of P16.

## The customer screen registry

The file draws the shopping flow three times — food (`1:7128`…), pharmacy
(`1:10754`…) and grocery (`1:16055`…) — and the three are the same eleven
screens with different items in them. The app builds each screen once and
passes the shop type through, because three copies of a cart is three places
to fix a bug in. The vertical is a `type` filter on discovery
(`restaurant | grocery | pharmacy`), which is exactly how the backend models
it.

The home screen is drawn three times as well (`1:280`, `1:859`, `1:1401`):
they are the same screen with the three type chips selected in turn.

## Figma screens with no backend behind them

Each is built as a placeholder that says, in the user's language, that the
feature is not available yet. None of them fabricates a number, an offer or a
balance — a screen that invented a wallet balance would be worse than one that
admits it has none.

| Screen | Figma node | Why there is nothing to wire |
|---|---|---|
| `02__Offers` | `1:1943`, `1:7872` | No promotions module exists. P00–P16 build no offer, campaign or banner endpoint, and none is planned in P17–P20. |
| `06__Offer Popup` | `1:8895` | Same. The nearest real thing is `CartPricing.notice`, which the cart screen already shows. |
| `13__Promos`, `14__Add Promos`, `12__Apply Voucher` | `1:3707`, `1:3798`, `1:19214` | There is no promo-code endpoint. Pricing (P10) has a free-delivery threshold, which the cart already reports through `away_from_free_delivery`, but no redeemable codes. |
| `15__Reffer & Get Discounts` | `1:3920` | No referral module. |
| `08__Business Profile` | `1:3559` | A customer-app frame for a merchant concept. The merchant app (P19) owns merchant profiles; `/v1/merchants/me` admits merchant tokens only. |
| `09__Digital Payment`, `10__Payment Methods`, `11__GoKlay Pay Authorised` | `1:3579`, `1:3609`, `1:3653` | P13 has exactly two methods — `cash` and `online` — chosen at checkout, and no stored instruments, no wallet and no saved cards. The screen lists the two the backend really has. |
| `18__GoKlay Safety` | `1:4187` | Static content with no endpoint to serve it. |
| `17__Permissions` | `1:4123` | Device permissions, not an API surface. Shown as what the app will ask for and why. |

## Backend surfaces with no Figma screen

| Surface | Endpoint | What was done |
|---|---|---|
| Notification history | `GET /v1/me/notifications` | Listed on a plain screen reached from Account. P14 composes both the title and the body server-side, so there is nothing to design around. |
| Support tickets | `POST /v1/support/tickets`, `GET /v1/me/support/tickets` | Raised from an order, listed from Account. P16 built them; the design predates that phase. |
| Reviews | `POST /v1/reviews` | Offered on a delivered order. Same reason. |
| Active sessions | `GET /v1/auth/sessions`, `POST /v1/auth/logout-all` | Shown on `07__Profile | Security`, which the design draws but leaves empty. |

## The merchant and partner apps are not in the file at all

P19 builds two more apps. The Figma source has **no frames for either**: of
the 10,456 named nodes on `06 Source — GoKlay v1`, a search for *merchant*,
*vendor*, *shop owner*, *rider*, *courier* or *driver* returns three
decorative labels inside customer screens — "Tip your rider", "Courier",
"Trained and Verified Drivers" — and nothing else. Every frame in the file is
customer-facing.

This is not a screen missing from a flow; it is two whole products missing.
§174 says the agent does not invent a screen that is not in Figma, and the
answer here is neither to invent one nor to skip the phase:

* **The layout comes from the API.** Each screen exists because an endpoint
  does, and shows what that endpoint returns. Nothing is designed around data
  the backend does not have.
* **The look comes from `goklay_core`**, which *is* the Figma file: the seven
  colours, the type scale, the 48dp floor and the Bengali fallback were all
  read out of it in P17. Applying an existing design system is not designing a
  new one.
* **No visual invention beyond that.** No illustrations, no bespoke charts, no
  marketing copy. Where a merchant or partner screen would want a flourish,
  it gets a card.

If frames for these apps arrive later, the screens are the ones to restyle;
the wiring underneath them is what the endpoints dictate either way.

## Where the design and the accessibility floor disagree

Recorded in `docs/technical/frontend.md` rather than here, because P17 resolved
them rather than leaving them open: the 41dp button keeps its painted size and
gains a 48dp hit area, and the three sub-AA colour pairings are each pinned by
a passing test that states the limit.
