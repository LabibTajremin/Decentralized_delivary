# shared/errs
Layer: Shared · Module: — · Phase: P01

**Responsibility** — the single error vocabulary for the backend: classify a failure, carry a stable machine code, and carry a message safe to show a user.

**Inputs / Outputs** — `New(Kind, code, message) *Error`, `Newf`, `Wrap(err, Kind, code, message) *Error`; readers `KindOf`, `CodeOf`, `MessageOf`, `Is`.

**Dependencies** — none. Deliberately: domain code returns these, so the package may not import a transport or infrastructure concern.

**Rules enforced**
- `KindInternal` is the zero value, so an uninitialised or unclassified error is never mistaken for a client mistake.
- `Wrap(nil, …)` returns nil, so callers can wrap unconditionally.
- `With` copies rather than mutates, so an error value shared across goroutines stays safe.
- `MessageOf` returns a generic string for any error that is not an `*Error`, so a driver message such as `pq: relation "users" does not exist` can never reach a user.
- `Fields()` returns a copy; a caller cannot reach in and mutate the error.

**Algorithms used** — none.

**Failure modes** — none; the package only constructs and inspects values.

**Tests** — `backend/tests/unit/shared/errs_test.go`. Covers every `Kind`, wrapping and `errors.Is` traversal, nested errors found through `fmt.Errorf("%w")`, copy-on-write of `With`, and the message-leak guard.
