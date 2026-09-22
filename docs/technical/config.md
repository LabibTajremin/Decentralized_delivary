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
| `GET /v1/admin/config/changes` | The audit log, filterable by key, scope and a result limit |
| `POST /v1/admin/config/autotune` | Run ALG-09 for one area |

These are admin endpoints. Authentication and role checks arrive in P04 and wrap
them without any change here, which is the point of keeping transport thin. The
use case already treats the actor as authoritative, so P04 changes where the
actor comes from and nothing else.

`/v1/config/effective` reports the *source* of every value. An admin looking at
a fee needs to know whether it comes from the area, the district or the global
default before they can sensibly change it.

## P15: the actor comes from the caller, not the request body

`setOverrideRequest`/`clearOverrideRequest` used to carry an `actor_id` field
that the handler trusted directly — a leftover from before P04 wired identity
through, and never closed out once it was. Any authenticated admin could
attribute a change to any other admin's id in the audit log, which defeats the
log's whole purpose: it exists so "why did this change" has an answer, and an
answer that can be forged is not one.

P15 removes the field. `confighttp.NewHandler` now takes a `PrincipalOf`
function — the same shape every other module's transport handler already uses
— and every write reads the actor from the authenticated request instead:

```go
adminID, ok := h.caller(w, r)
if !ok {
    return // 401, unauthenticated
}
...
Actor: domain.AdminActor(adminID),
```

`NewHandler` panics on a nil `principal`, the same way it already panicked on
a nil guard: a config write with no real actor behind it is exactly the hole
the audit log is supposed to make impossible.

## ALG-09: auto-tuning radius

Appendix B: "Auto-tuning radius... merchant density and order-failure rate."
`domain.Tune` is that rule as one pure function — given a definition, the
current value, an area's merchant-density threshold and a signal, it decides
the next value, a human-readable reason, and whether anything changed at all.

**Scope.** ALG-09 tunes exactly two keys: `discovery.base_radius` and
`dispatch.partner_radius` — the two whose whole purpose is "how far to reach
for supply." Every other auto-tunable key (pricing, `order.cod_limit`) has no
signal Appendix B defines for it, so ALG-09 leaves those to the admin rather
than inventing a feedback loop the spec never asked for.

**The rule.** Widen when supply is thin (fewer merchants nearby than the
area's own `discovery.min_merchants`) or when deliveries are failing more
than ALG-09's own accepted rate — either reading says the served area is too
small. Narrow only when density is comfortably above that threshold (twice
it, not merely at it) *and* delivery success is fine, so a stable area does
not keep shrinking pass after pass toward its floor. Everything else is left
alone. A move is always one step — 10% of the current value, or one metre on
a radius too small for 10% to round to anything — clamped to the
definition's own bounds; a value already at a bound reports no change rather
than an error.

**Where it writes.** `AutoTuneUseCase.Execute` resolves the area, then for
each tunable key calls the *same* `SetOverrideUseCase.Execute` an admin's own
PUT goes through — bounds, the pin check, and the audit entry are enforced
once there, not duplicated in the tuner. A pinned variable comes back as
`ErrPinned`, which the use case turns into `TuneOutcome{Applied: false,
Skipped: "pinned"}` rather than failing the whole pass — an admin pinning one
key in one area must not stop the tuner from touching the other key, or the
other areas.

**The actor.** Every auto-tuned change is attributed to `domain.TunerActor()`,
not an admin — the audit log distinguishes "an admin decided this" from "the
algorithm decided this," which is the whole reason `domain.Actor` carries a
`Kind` at all.

**Trigger.** `POST /v1/admin/config/autotune` takes the signal directly in the
request body (`merchants_nearby`, `order_failure_rate`) rather than computing
it server-side: this phase has no scheduled job or telemetry pipeline to
supply that signal automatically, so an operator or a future cron calls the
endpoint with numbers it already has. What ALG-09 does with the signal is
what Appendix B specifies; how the signal is gathered is deliberately left
open for a later phase.
