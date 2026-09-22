# Phase index

> Split from `CLAUDE_CODE_BUILD_INSTRUCTION.md` v2.0, which remains the single
> authority. These files are its parts, unchanged in meaning.

# PART 5 — PHASES

Every phase must be **completed fully** before the next begins. No partial phases.

Each phase produces: working code, its tests at 100%, its technical documentation, its user documentation where user-facing, a green PR, and an updated `STATE.md`.

| ID | Phase | Key output |
|---|---|---|
| **P00** | Foundation & tooling | Repo layout, `go.work`, both modules, Docker compose, CI with all eleven checks, architecture lint, `STATE.md`, README skeleton, ADR template, OpenAPI skeleton |
| **P01** | Shared kernel | Error types, structured logging, ID generation, time abstraction, pagination, result types, config loader. 100% covered. |
| **P02** | Geo module | Division and district geometry loaded, point-in-polygon, distance calculation, spatial indexes, `ALG-01`, `ALG-03`. **This is the foundation of the whole product — nothing else can be built correctly before it.** |
| **P03** | Config module | Per-area variable store, defaults, admin overrides, resolution order (area → district → division → global), change audit. Implements `D5`. See Appendix B. |
| **P04** | Identity & auth | Phone+OTP, JWT, refresh rotation, RBAC middleware, rate limiting, sessions. All three roles. |
| **P05** | User profile & addresses | Profile, address book, map pin, default address, address-to-area resolution |
| **P06** | Merchant | Registration from anywhere in Bangladesh, business details, documents, approval workflow, hours, holiday mode, merchant types (restaurant / grocery / pharmacy) |
| **P07** | Catalogue | Categories, items, variants, add-ons, combos, stock, scheduled availability, bulk update, per-merchant-type schema differences |
| **P08** | Discovery | Radius search, `ALG-02` expansion, division ceiling, `ALG-07` ranking, filters, search. **Implements `D1`, `D2`, `D3`.** |
| **P09** | Cart | Cart rules, single-merchant enforcement, add-on validation, cart invalidation on address or radius change |
| **P10** | Pricing | `ALG-05` delivery fee banding, **expansion surcharge**, service fee, VAT, promo application, minimum order |
| **P11** | Order | Order lifecycle state machine, COD and prepaid paths, cancellation, idempotency |
| **P12** | Dispatch | Partner availability, `D4` long/short distance choice, radius-scoped job feed `ALG-08`, `ALG-04` assignment, reassignment, proof of delivery, COD reconciliation |
| **P13** | Payment | Isolated module per 2.6. Gateway adapter, COD ledger, webhooks, idempotency, refunds. Contract-only boundary ready for .NET migration. |
| **P14** | Tracking & notifications | Order status stream, partner location stream, push, SMS fallback |
| **P15** | Admin & auto-tuning | Full admin API, all config controls from `D5`, `ALG-09` auto-tuner, live order operations, approvals, reporting, audit log |
| **P16** | Reviews & support | Ratings for merchant, partner and item; reviews; tickets; refund workflow |
| **P17** | Flutter foundation | Design tokens from Figma, component library, routing, state management, API client generated from OpenAPI, response caching and offline reads, Bengali localization, 48dp and contrast enforcement. **Thin-client lint in place before any screen is built** (2.9) |
| **P18** | Flutter customer app | Every customer screen from the Figma registry, wired to the API. Presentation only — no business logic, no money arithmetic, capability flags drive every enabled state |
| **P19** | Flutter merchant & partner apps | Every merchant and partner screen, wired to the API, under the same thin-client rule |
| **P20** | Hardening & release | Full E2E regression, load test on discovery and dispatch, security review, complete documentation, production deploy runbook |

## 5.1 Phase file format

Each `docs/build/phases/Pxx.md` is short by design:

```markdown
# P08 — Discovery
Depends on: P02, P03, P07
Branch: phase/08-discovery
Implements: D1, D2, D3, ALG-02, ALG-07

## Tasks
P08.T01  Domain: SearchArea, Radius, ExpansionStep value objects
P08.T02  Port: MerchantDiscoveryRepository
P08.T03  Use case: FindMerchantsNearby — base radius from config
P08.T04  Use case: ExpandSearchRadius — ALG-02, division ceiling
P08.T05  Use case: RankResults — ALG-07
P08.T06  Infrastructure: PostGIS repository implementation
P08.T07  Transport: discovery endpoints + OpenAPI
P08.T08  Tests: unit, integration, edge cases
P08.T09  Docs: technical + user
P08.T10  Coverage verification, PR, merge

## Acceptance
- A user in a dense urban area sees merchants within the configured base radius
- A user with zero merchants in radius is offered expansion
- Expanding increases the quoted delivery fee
- Expansion stops at the division boundary and returns a clear terminal state
- Two users in different divisions never see each other's merchants
```

---
