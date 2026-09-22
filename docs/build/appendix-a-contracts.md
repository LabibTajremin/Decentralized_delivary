# Appendix A — module contract registry

> Split from `CLAUDE_CODE_BUILD_INSTRUCTION.md` v2.0, which remains the single
> authority. These files are its parts, unchanged in meaning.

# APPENDIX A — MODULE CONTRACT REGISTRY

Every module exposes exactly one public contract interface. This is the only surface other modules may call, through their own `external/` service.

| Module | Contract | Consumed by |
|---|---|---|
| geo | `GeoContract` | discovery, dispatch, pricing, merchant, user |
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

---
