# 0004 — Every business rule lives in the backend

Date: 2026-09-13
Status: Accepted

## Context
Radius defaults, delivery fee bands, COD limits and commission rates are all
tunable per area by the admin at runtime (D5, Appendix B). The audience is rural
users on low-end Android devices who update apps rarely.

## Options considered
1. **Compute in the client where convenient** — fewer round trips, but every
   rule change then needs an app release and a user who installs it.
2. **Compute everything server-side and ship display-ready responses.**

## Decision
Option 2. The client performs no arithmetic on money or distance, holds no
config value, and derives no permission. Responses carry pre-formatted money
strings, pre-computed breakdown lines, and explicit capability flags with reason
strings.

## Consequences
- A config change takes effect immediately for every user, with no release.
- API responses must be shaped per screen, which is more backend work per screen.
- Enforced by `scripts/thin-client-lint.sh` as its own CI check, verified
  against deliberate violations rather than assumed to work.
