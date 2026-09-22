# Product specification

> Split from `CLAUDE_CODE_BUILD_INSTRUCTION.md` v2.0, which remains the single
> authority. These files are its parts, unchanged in meaning.

# PART 1 — PRODUCT SPECIFICATION

## 1.1 What this is

A decentralized delivery platform for Bangladesh covering **food, grocery and pharmacy**.

"Decentralized" here means **radius-scoped discovery**, not a distributed or blockchain system. Use the term **radius-scoped** in code and technical docs; keep "decentralized" only in business-facing material.

## 1.2 Roles

| ID | Role | Description |
|---|---|---|
| ROLE-USER | User | Normal people who order online |
| ROLE-PARTNER | Delivery Partner | The person who delivers the product |
| ROLE-MERCHANT | Merchant | Sells the product — restaurant, grocery shop, pharmacy |
| ROLE-ADMIN | Admin | Platform operations and configuration |

## 1.3 The decentralization model — the core of this product

These rules are the product. Everything else is standard delivery-app functionality.

**D1 — Nationwide registration, local visibility.**
- A user can order food, pharmacy and grocery **from anywhere in Bangladesh**.
- A delivery partner can provide delivery **anywhere in Bangladesh**.
- A merchant can register **from anywhere in Bangladesh**.
- But a user only sees restaurants, groceries and pharmacies **inside a fixed radius** of their delivery location.

**D2 — Radius expansion with a cost.**
- When there is no restaurant or order option nearby, the user is offered the option to **extend the radius**.
- When the radius is extended, **the delivery charge increases**.
- Expansion is stepwise, not unlimited.

**D3 — Division-level ceiling.**
- Radius expansion **stops at division level**. A user's search can never cross beyond their division boundary, regardless of how far they expand.
- Bangladesh has eight divisions. Division is the hard geographic boundary of the system.

**D4 — Delivery partner distance choice.**
- A delivery partner can choose between **long-distance delivery** and **short-distance delivery**.
- A delivery partner is shown **only the available delivery options inside their fixed radius**.

**D5 — Automatic control, minimal admin interaction.**
- The application controls this decentralization **automatically**, with minimum admin interaction.
- The admin **must be given every option** to interact with and control it, but the application tunes the variables itself by default.
- Every variable ships with a **default value**. The admin can change those variables **per area**.
- Example: search radius may differ area by area. Dhaka city needs a small radius; a rural upazila needs a large one.

> **Design consequence.** D5 means the config system is not a settings screen bolted on at the end. It is a first-class subsystem — see Phase 11 and Appendix B — with per-area overrides, defaults, and an auto-tuner that adjusts within admin-set bounds.

## 1.4 Audience constraint

Primary users are rural, many with low digital literacy on low-end Android devices.

- Minimum tap target 48dp.
- Icon **plus** label for every primary action, never icon alone.
- Bengali-first. Layouts must be tested at Bengali string lengths.
- Minimum steps to complete an order.
- Works on slow connections: skeletons, optimistic UI, explicit offline messaging.
- **Cash on delivery is the primary payment path**, not an edge case.

## 1.5 UI source of truth

All screens come from the Figma file and the design documentation. The agent does not invent screens. If a screen is needed and not in Figma, the agent records it in `docs/design-gaps.md` and continues against a documented placeholder — it does not silently design something new.

---
