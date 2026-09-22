# 0013 — Authorisation by resource ownership outside the admin surface

Date: 2026-09-22
Status: Accepted

## Context
P20's security review found that the most important thing about this API's
authorisation model was not written down anywhere.

Ten of the fourteen modules mount their non-admin routes behind
`authenticator.Authenticated()`, which is `Require(AllRoles()...)`. So the
token's *role* is not what keeps a customer out of `/v1/partner/cod` or
`/v1/merchants/me/hours`. What keeps them out is that they own no partner
record and no shop: the handler runs, finds nothing of theirs, and answers
`404`.

Only `/v1/admin/…` (plus `/v1/config/definitions` and
`/v1/config/effective`, which predate the prefix convention) is guarded by
role, with `Require(RoleAdmin)`.

This was never decided in one place — it accreted, one module at a time, from
P05 onwards. The review's finding is that it is nonetheless a coherent model
and worth stating as one, together with what it costs.

## Options considered
1. **Role guards on every route group**, in addition to the ownership checks.
   A customer reaching a partner route would be a 403 at the middleware,
   before any handler ran.
2. **Ownership only**, which is what exists: one authenticated guard, and each
   handler establishes what the caller owns.
3. **Change it now, in P20.** Ten modules, each with a handler signature, its
   unit tests and its documented responses to update.

## Decision
Option 2 is **accepted as the model**, and stated: outside the admin surface,
authorisation in this API is by resource ownership, not by token role.

It is coherent, and in two places it is actively better than a role guard
would be:

* **A 404 rather than a 403 is the right answer** for somebody else's
  resource. "That order exists but is not yours" lets anybody with a list of
  ids find out which ones are real, and the whole API answers 404 to that
  question — for addresses, orders, shops, menus, tracking streams and
  payments alike.
* **Some routes have no id to check.** `GET /v1/partner/cod` takes no partner
  id at all: the ledger returned is the one belonging to the token, so there
  is no parameter for a caller to change. Ownership is not an extra check
  there, it is the only sensible design.

Option 1 is recorded as a recommendation in `docs/security-review.md` §4.3
rather than done here, because doing it properly is a phase of its own and
P20 is not the place to touch ten modules' handler signatures.

## Consequences
- **Role guards fail closed for a whole class of route; ownership checks fail
  closed only if each handler remembers to make one.** A new handler that
  forgets is protected by nothing, and no middleware would notice. That is a
  defence-in-depth gap, not a live vulnerability, and it is the cost being
  accepted.
- **The admin surface is guarded by role, and that guard is now swept rather
  than listed.** `backend/tests/e2e/security_test.go` reads
  `api/openapi.yaml`, takes every path under `/v1/admin/`, and requires all of
  them to refuse an anonymous caller and each of the three non-admin roles.
  Nothing enumerates the routes, so an admin route mounted without the guard
  fails the build whether or not anybody wrote a test for it. Verified by
  pointing it at `/v1/me/` instead, where it reported every operation under
  that prefix — correctly, since those are guarded by ownership rather than by
  role.
- **Ownership is what has to be tested, so it is.**
  `TestOwnershipIsWhatProtectsThePartnerSurface` proves the positive case as
  well as the negative one: a rider gets their own ledger, and another rider's
  is reachable only through the admin route, which refuses them.
- **If Option 1 is ever done, the sweep extends to the whole surface** rather
  than only the admin prefix, which is the real prize.
