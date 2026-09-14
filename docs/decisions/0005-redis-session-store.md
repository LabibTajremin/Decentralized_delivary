# 0005 — Redis-backed refresh tokens, rotation, and silent auto-login

Date: 2026-09-13
Status: Accepted

## Context
The operator requires JWT authentication with separate access and refresh
tokens, and specifically that **a logged-out user holding a valid refresh token
is signed back in automatically**, without seeing a login screen. The audience
is rural users on intermittent connections, for whom a surprise re-login is a
real drop-off point.

## Options considered
1. **Stateless JWT only, no refresh** — nothing to revoke; a stolen token is
   valid until it expires. Rejected.
2. **Refresh tokens in Postgres** — works, but every refresh costs a write to
   the primary transactional store, and expiry needs a sweeper job.
3. **Refresh tokens in Redis, hashed, with TTL and rotation.**

## Decision
Option 3.

- **Access token**: JWT, 15 minutes, verified by signature alone so the hot path
  never touches Redis.
- **Refresh token**: opaque 256-bit random value, 60 days. The server stores only
  its SHA-256 hash; the raw value exists solely on the device.
- **Rotation**: every refresh consumes the token and issues a new one, applied as
  an atomic Lua script so concurrent refreshes from one device cannot both mint.
- **Reuse detection**: presenting an already-rotated token revokes the whole
  session family, on the assumption it was stolen.
- **Auto-login**: on app start, or on any `401`, the client exchanges its stored
  refresh token for a new pair and continues. The client never inspects token
  validity itself — it reacts to the server's answer.

## Consequences
- Revocation and logout-all are O(1), and expiry is automatic through TTL, so
  there is no sweeper job.
- Redis becomes a hard dependency of the auth path; it is in `docker-compose.yml`
  and in the CI service matrix from P00 so it is never an afterthought.
- Access tokens cannot be revoked mid-life. The 15-minute lifetime is the
  deliberate bound on that window.
- Losing Redis logs everyone out at their next refresh but does not invalidate
  in-flight access tokens — degradation, not an outage.


## Addendum — as built (P04)

The model above survived implementation with three refinements worth recording.

**A spent-token index, not a family scan.** Telling "already rotated" from
"never issued" was originally to be answered by looking in the session's family
set. That needs the session id, which is precisely what an unknown token does
not give you — so answering it would have meant scanning every family, O(n) in
sessions. A `refresh_spent:<hash>` key answers it in O(1), and is deleted along
with the family when a session is revoked so an ordinary logout is never
reported as a theft.

**The attempt counter is one script, not INCR then EXPIRE.** As two commands, a
process dying between them leaves a lockout counter with no expiry — a permanent
lockout of that phone number that only a manual Redis edit could clear. This was
found while chasing an untested branch, which is the argument for the coverage
gate in one sentence.

**Session ordering breaks ties on the id.** "Evict the oldest session" is
undefined when two sign-ins share an instant, and would have removed an
arbitrary device. Session ids are time-ordered, so they are a meaningful
tie-break rather than an arbitrary one.
