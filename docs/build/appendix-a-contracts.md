# Appendix A — module contract registry

> Split from `CLAUDE_CODE_BUILD_INSTRUCTION.md` v2.0, which remains the single
> authority. These files are its parts, unchanged in meaning.

# APPENDIX A — MODULE CONTRACT REGISTRY

Every module exposes exactly one public contract interface. This is the only surface other modules may call, through their own `external/` service.

| Module | Contract | Consumed by |
|> **P05 addition.** The table above omitted a contract for the user module,
> listing it only as a consumer of geo. Order needs to know where to deliver and
> dispatch needs to know who receives the parcel, so `UserContract` was added.
> The alternative — letting order read the user tables — breaks the rule that a
> module never reaches into another's storage. See docs/technical/user.md.

---|> **P05 addition.** The table above omitted a contract for the user module,
> listing it only as a consumer of geo. Order needs to know where to deliver and
> dispatch needs to know who receives the parcel, so `UserContract` was added.
> The alternative — letting order read the user tables — breaks the rule that a
> module never reaches into another's storage. See docs/technical/user.md.

---|> **P05 addition.** The table above omitted a contract for the user module,
> listing it only as a consumer of geo. Order needs to know where to deliver and
> dispatch needs to know who receives the parcel, so `UserContract` was added.
> The alternative — letting order read the user tables — breaks the rule that a
> module never reaches into another's storage. See docs/technical/user.md.

---|
| geo | `GeoContract` | discovery, dispatch, pricing, merchant, user |
| user | `UserContract` | order, dispatch, tracking — **added in P05** |
| config | `ConfigContract` | every module |
| identity | `IdentityContract` | every module |
| merchant | `MerchantContract` | discovery, catalogue, order |
| catalogue | `CatalogueContract` | discovery, cart, order |
| discovery | `DiscoveryContract` | cart |
| cart | `CartContract` | order |
| pricing | `PricingContract` | cart, order |
| order | `OrderContract` | dispatch, payment, tracking, review |
| dispatch | `DispatchContract` | order, tracking |
| payment | `PaymentContract` | order — **transport-neutral DTOs only** |
| tracking | `TrackingContract` | order, notification |
| notification | `NotificationContract` | order, dispatch, identity |
| review | `ReviewContract` | merchant, dispatch |

> **P05 addition.** The table above omitted a contract for the user module,
> listing it only as a consumer of geo. Order needs to know where to deliver and
> dispatch needs to know who receives the parcel, so `UserContract` was added.
> The alternative — letting order read the user tables — breaks the rule that a
> module never reaches into another's storage. See docs/technical/user.md.

---

> **P06 note.** `GeoContract` gained `PlaceMerchant`, `RemoveMerchant` and
> `ResolveDivision`. The first two exist so the merchant module can publish a
> shop's location without writing to geo's tables. The third exists because D1
> and delivery need different strictness: an address must resolve to a mapped
> area, while a merchant may register from anywhere in Bangladesh — including an
> upazila we have not drawn an area for. See docs/technical/merchant.md.

> **P07 note.** `CatalogueContract` carries money as both a minor-unit integer
> and a preformatted display string, like every other amount that crosses a
> boundary (2.9). It deliberately omits shelf counts, the owner's visibility
> switch and availability schedules: discovery does not need them, and a
> contract that handed a competitor's stock levels over would be telling them
> how a shop is doing. See docs/technical/catalogue.md.

> **P08 note.** `DiscoveryContract` is implemented as one method, `Reach`. Cart
> needs a single answer — may this address still order from this shop, and how
> far is it — and a contract that also exposed the search would put a browse
> endpoint inside a checkout path. The search result shape stays in discovery's
> own application layer, where it belongs to the transport that renders it.

> **P09 note.** `CartContract` carries the cart **revalidated**, at today's
> prices, rather than as it was stored. An order frozen from a stale snapshot is
> an order the shop disputes. It also carries `Clear`, so order asks the cart to
> empty itself instead of deleting rows it does not own. See
> docs/technical/cart.md.

> **P10 note.** `PricingContract` returns a *resolved tariff* rather than
> answering one price at a time, the same shape as `ConfigContract.Settings` and
> for the same reason: a search prices twenty shop cards, and a per-card contract
> would mean twenty configuration reads that could quote two shops against two
> different settings. `DiscoveryContract.Reach` also gained the resolved
> placement in this phase — discovery had already worked it out to answer at all,
> and the cart needed it to price. See docs/technical/pricing.md.

> **P11 note.** `OrderContract` exposes named transitions (`MarkPaid`,
> `MarkPaymentFailed`, `Advance`) rather than a general "set status". A contract
> that let any consumer write any status would put the lifecycle back in the
> hands of every module that imports it. `Advance` further restricts the actor
> to partner, admin or system: a customer's cancellation is bound by a window
> checked on their own path, and a shop's authority over an order is "this shop
> is yours", which only the HTTP layer can establish.
>
> `MerchantContract` also gained `OwnedBy` in this phase, so the order module
> can answer "is this shop yours" before showing a queue without reading the
> merchant table. See docs/technical/order.md.
