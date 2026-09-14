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
