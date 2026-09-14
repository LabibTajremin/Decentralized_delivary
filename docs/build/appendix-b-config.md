# Appendix B — configuration variables

> Split from `CLAUDE_CODE_BUILD_INSTRUCTION.md` v2.0, which remains the single
> authority. These files are its parts, unchanged in meaning.

# APPENDIX B — CONFIGURATION VARIABLES

Implements `D5`. Every variable has a global default, is overridable per area by the admin, and where marked, is adjustable by the auto-tuner within admin-set bounds.

Resolution order: **area → district → division → global default**.

| Key | Default | Unit | Auto-tuned | Purpose |
|---|---|---|---|---|
| `discovery.base_radius` | 5 | km | yes | Initial search radius |
| `discovery.expansion_step` | 5 | km | yes | Increment per expansion |
| `discovery.max_expansions` | 4 | count | no | Hard cap on steps |
| `discovery.min_merchants` | 5 | count | no | Threshold triggering expansion offer |
| `discovery.auto_expand` | false | bool | no | Expand without asking when below threshold |
| `discovery.division_ceiling` | true | bool | **no — never disableable** | Enforces `D3` |
| `pricing.delivery_base` | 40 | BDT | yes | Base delivery fee |
| `pricing.delivery_per_km` | 10 | BDT | yes | Distance rate |
| `pricing.expansion_multiplier` | 1.5 | ratio | yes | Surcharge when radius extended, per `D2` |
| `pricing.free_delivery_threshold` | 500 | BDT | yes | Order value for free delivery |
| `dispatch.short_distance_max` | 5 | km | yes | Upper bound of "short distance" per `D4` |
| `dispatch.long_distance_max` | 25 | km | yes | Upper bound of "long distance" |
| `dispatch.partner_radius` | 7 | km | yes | Radius of a partner's job feed |
| `dispatch.assignment_timeout` | 30 | sec | no | Before reoffering |
| `dispatch.max_concurrent_jobs` | 3 | count | no | Batching limit |
| `order.cancellation_window` | 120 | sec | no | Free cancellation period |
| `order.cod_limit` | 5000 | BDT | yes | Maximum COD order value |
| `auth.otp_requests_per_hour` | 5 | count | no | OTP requests per phone number per hour |
| `auth.otp_verify_attempts` | 5 | count | no | Wrong codes before lockout |
| `auth.otp_lockout_window` | 900 | sec | no | Lockout duration after too many wrong codes |
| `auth.otp_ttl` | 300 | sec | no | OTP validity |
| `auth.access_token_ttl` | 900 | sec | no | Access token lifetime |
| `auth.refresh_token_ttl` | 5184000 | sec | no | Refresh token lifetime (60 days) |
| `auth.max_sessions_per_user` | 5 | count | no | Active devices; oldest evicted beyond this |

> The `auth.*` keys were added in P04. They are business rules rather than
> deployment settings: an operator seeing OTP abuse in one division must be able
> to tighten the limit there, without a redeploy and without tightening it
> everywhere. None is auto-tunable — a tuner that can lengthen a token lifetime
> or loosen a brute-force limit is a tuner that can weaken authentication.

**Auto-tuner (`ALG-09`)** runs per area on a schedule. It adjusts only variables marked auto-tuned, only within admin-configured min/max bounds, and writes every change to the config audit log with its reasoning. The admin can pin any variable, disabling auto-tuning for it in that area.

`discovery.division_ceiling` can never be disabled by admin or tuner. It is the one hard invariant of the system.
