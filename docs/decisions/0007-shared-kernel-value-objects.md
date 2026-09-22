# 0007 — Money and schedules live in a shared kernel the domain may import

- Status: accepted
- Date: 2026-09-14
- Deciders: backend

## Context

Rule 2.2 says a module's `domain/` **imports nothing**, and
`backend/tests/unit/architecture_test.go` fails the build when it does. That
rule earned its place: a domain that reaches outward is a domain that cannot be
tested, reasoned about, or extracted.

P07 hit the first case where obeying it literally makes the code worse.

Two value objects are now needed by more than one module:

- **Money.** Catalogue prices items; cart sums lines; pricing computes fees;
  order freezes a total; payment settles it. That is five modules.
- **A weekly schedule.** A shop's opening hours (P06), an item's availability
  (P07), a delivery partner's shift (P12). That is three.

Both are pure value objects: no I/O, no clock beyond what is passed in, no
dependency on anything in the repository.

## Options considered

**A copy in each module's domain.** Obeys the letter of 2.2. It also means five
implementations of money. They will not stay identical — one will round half-up
and another half-even, one will format `৳1240` and another `৳ 1,240` — and the
first time a customer sees a total that disagrees with a receipt by one poisha,
the cause will be in whichever copy nobody remembered to change. Money is
exactly the kind of thing that must have one implementation.

**Primitives in the domain, value objects in the application layer.** The domain
holds `int64` minor units and the use cases wrap them. This keeps the letter of
2.2 and loses the point of it: the invariants that make money safe — you cannot
add two currencies, you cannot have a negative price — move out of the layer
whose job is invariants, into the layer that is supposed to orchestrate them.

**Amend the rule.** Treat a shared kernel as *part of* the domain layer rather
than outside it, which is the conventional reading in domain-driven design: a
shared kernel is domain code that several bounded contexts agree to share.

## Decision

The third, with the exception made narrow and machine-checked rather than left
to judgement.

A module's `domain/` may import a shared package **only if it is on a short
named list and is verifiably pure** — pure meaning it imports nothing internal
itself. Today that list is `internal/shared/money` and
`internal/shared/schedule`.

Both halves are needed, and the first draft of this guard had only the second.
Purity alone also admits `internal/shared/errs`, which imports nothing internal
and is still not a value object: a domain returning transport-shaped errors is
exactly the coupling the layering exists to prevent. A probe that added such an
import passed the purity-only guard, which is how the gap was found. The list is
the judgement about what belongs in a domain; the purity check is the mechanical
guarantee that what is on the list stays safe to depend on.

That keeps the property the original rule was protecting. A domain still
depends on nothing that depends on anything — the dependency graph below a
domain package is still empty of module code — and a shared package that grows
an import of `application/` or `infrastructure/` breaks the build for every
domain that uses it, which is the loudest possible signal that it stopped being
a value object.

`internal/shared/errs`, `logging`, `paging` and `id` are **not** on the list and
a domain importing one fails the build, even though each is pure. They are used
from `application/` and above; nothing in a domain needs them.

## Consequences

- One implementation of money, with the arithmetic and the display string in
  the same place — which is also what the thin-client rule (2.9) requires, since
  the server owns formatting.
- One implementation of a weekly timetable. P06's opening hours were rewritten
  onto it in this phase; its whole test suite passed unchanged, which is the
  evidence that the extraction preserved behaviour.
- The guard is now stricter in one respect than it was: it checks the shared
  package's own imports, which nothing did before. Both failure modes were
  verified by feeding the guard a deliberate violation — a shared package that
  imports module code, and a domain that imports a shared package not on the
  list.
- A future `shared/` package that is not a value object cannot be imported from
  a domain without first making it one. That is the intended friction.

## What would reverse this

If a shared value object ever needs to differ per module — two modules
legitimately wanting different rounding, say — the answer is not to parameterise
the shared one. It is to move that type back into the domains that disagree and
let them disagree explicitly.
