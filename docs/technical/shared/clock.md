# shared/clock
Layer: Shared · Module: — · Phase: P01

**Responsibility** — abstract the system clock so time-dependent behaviour is testable without sleeping.

**Inputs / Outputs** — `Clock` interface with `Now() time.Time`; `System` for production, `Fixed` for tests with `Advance` and `Set`.

**Dependencies** — standard library only.

**Rules enforced**
- Every time the backend produces is UTC. Local time exists only at the presentation edge, so stored and compared timestamps are never ambiguous.
- `Fixed` is mutex-guarded, so a test exercising concurrency does not race on the clock itself.

**Algorithms used** — none.

**Failure modes** — none.

**Tests** — `backend/tests/unit/shared/clock_test.go`, including UTC normalisation from a non-UTC input and 100 concurrent advances.
