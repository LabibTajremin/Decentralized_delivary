# shared/paging
Layer: Shared · Module: — · Phase: P01

**Responsibility** — cursor pagination for every list endpoint.

**Inputs / Outputs** — `NewPage(limit, offset) (Page, error)`; `Cursor.Encode()` / `DecodeCursor`; `NewResult[T](items, page, total) Result[T]`.

**Dependencies** — standard library only.

**Rules enforced**
- Every list is paginated (05-architecture.md 2.9): a low-end device is never handed a list long enough to need slicing on the phone.
- A limit above `MaxLimit` is clamped rather than rejected, so a client bug degrades instead of failing.
- Cursors are opaque base64, so server-side ordering can change without breaking a client that stored one.
- `NewResult` expects `Limit+1` rows: the probe row proves more pages exist without a second `COUNT` query, and is trimmed before return.

**Algorithms used** — none.

**Failure modes** — `ErrInvalidCursor` for a negative offset or a malformed, wrongly-prefixed or non-numeric cursor.

**Tests** — `backend/tests/unit/shared/paging_test.go`. Covers clamping at both ends, cursor round-trip and opacity, every rejection branch, and the probe-row behaviour on first, last and empty pages.
