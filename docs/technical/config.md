# Config module

Every business rule an operator can tune lives here: radius, fees, dispatch
bounds, COD limits. Implements D5.

## Why this is not environment configuration

`.env` holds deployment settings — where the database is, what signs a token.
Those need a restart to change, and that is correct for them.

Business rules are different. "The delivery fee in Mirpur should be ৳60" is a
decision someone makes on a Tuesday afternoon, and it must not require a deploy.
A rule that needs a redeploy to change is a rule in the wrong place (ADR 0004),
so these live in the database, resolve per area, and change at runtime.

The two are deliberately different code paths with different names —
`internal/shared/config` for deployment, `internal/modules/config` for business
rules — so it is hard to put something in the wrong one by accident.

## Resolution

```
area → district → division → global
```

First match wins. Every key has a global default, so resolution is total: a
caller asking for the delivery fee in an area nobody has ever configured gets a
number, not an error and not a zero.

The precedence is applied in the domain, not in SQL. One query fetches every
override that *could* apply to a placement, and `domain.Resolve` picks. The
ordering that decides every fee in the country is then one readable function
with unit tests, rather than an `ORDER BY` clause nobody reviews.

`Level`'s numeric order *is* the resolution order. Inserting a level between two
others means renumbering there and nowhere else.

## Types, and why there are six of them

`count`, `bool`, `ratio`, `money_minor`, `distance_m`, `duration_s`.

Money, distance and duration could all have been "a number". They are not,
because the unit is the part people get wrong: a delivery fee of 40 and a radius
of 40 are not interchangeable, and a type system that cannot tell them apart
will eventually let one be used as the other.

Consequences that follow from that choice:

- Money is always minor units (poisha), never a float, anywhere (2.9).
- Distance is always metres. Appendix B is written in kilometres and the
  registry converts once, so no consumer has to remember a scale factor.
- Reading a value as the wrong type is an error, not a zero. A silent zero for a
  fee read as a radius is a bug that ships.
- Values are transported as strings, not JSON numbers, so money cannot
  round-trip through a float on the way to a client.

## The one immutable variable

`discovery.division_ceiling` enforces D3. Nothing may change it: not an admin,
not the auto-tuner, not a per-area override.

It is enforced in two places on purpose:

1. `SetOverrideUseCase` refuses the write, before even looking at who is asking
   — the answer is the same for every actor.
2. `domain.Resolve` ignores any override for it that is already stored.

The second is what makes the guarantee real. A row inserted directly into the
database — by a migration, a console session, or a bug — still cannot turn the
ceiling off.

It is in the registry so it is *visible and auditable*, not so it is adjustable.
The admin UI shows it as a locked fact rather than hiding it.

## Limits on the auto-tuner (ALG-09)

Three, all enforced in the use case rather than in the tuner:

1. **Only auto-tunable variables.** `discovery.max_expansions` is a policy
   decision, not something to optimise into.
2. **Only within bounds.** Every tunable variable has an admin-set min and max.
   An auto-tuner with no upper bound on a delivery fee is one bad input away
   from quoting a fee nobody would pay; the bounds are what make it safe to run
   unattended.
3. **Never a pinned variable.** A pin is an admin saying "I have decided this
   one". The pin is checked at the exact scope being written, so pinning Gulshan
   does not freeze Mirpur.

Putting these in the use case rather than in ALG-09 matters: when P15 writes the
tuner, the limits are already true of every path that writes configuration, and
a bug in the tuner cannot exceed them.

## The audit log

Two tables, not one. `config_overrides` is what the settings *are*;
`config_changes` is how they got that way.

The log is not derived from the overrides table, because an override that is
deleted would take its history with it — and "why did this area go back to the
default?" is exactly the question the log exists to answer.

- Every write records the old value, the new value, the actor, and a reason.
- A reason is **required**. A log that answers "what changed" but never "why"
  does not answer the question people actually ask.
- Clearing an override is audited like any other change.
- The override and its audit entry are written in one transaction. A change that
  is applied but not recorded is precisely the change someone will need to
  explain later; an integration test forces the audit insert to fail and asserts
  the override does not survive.

## Failure behaviour

A repository failure during resolution is an error, not a silent fall back to
defaults. Quietly serving global defaults during a database outage would shrink
a 15 km area to 5 km and tell customers there are no merchants near them — a
wrong answer delivered confidently is worse than an error the caller can retry.

Individual *rows* are treated differently. A stored override whose key is no
longer in the registry, or whose text no longer parses as its declared type, is
skipped: one stale row must not stop the whole system loading its configuration.
Audit entries are stricter — an unreadable one is reported rather than skipped,
because a hole in the record is how a missing change goes unnoticed.

## API

| Route | Purpose |
|---|---|
| `GET /v1/config/definitions` | Every variable with bounds, type and whether it is tunable |
| `GET /v1/config/effective` | Resolved values for a placement, each with its source scope |
| `PUT /v1/config/overrides` | Set a variable at one scope |
| `DELETE /v1/config/override` | Reset a variable to the next scope up |

These are admin endpoints. Authentication and role checks arrive in P04 and wrap
them without any change here, which is the point of keeping transport thin. The
use case already treats the actor as authoritative, so P04 changes where the
actor comes from and nothing else.

`/v1/config/effective` reports the *source* of every value. An admin looking at
a fee needs to know whether it comes from the area, the district or the global
default before they can sensibly change it.
