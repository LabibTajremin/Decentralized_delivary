# shared/result
Layer: Shared · Module: — · Phase: P01

**Responsibility** — carry a value or an error as a single item.

**Inputs / Outputs** — `Ok[T]`, `Err[T]`, `IsOk`, `IsErr`, `Error`, `Unpack`, `ValueOr`, `Map`, `Partition`.

**Dependencies** — none.

**Rules enforced**
- Ordinary code uses Go's `(T, error)` pair. `Result` exists only where a fallible value must be stored or moved as one item — a channel of outcomes, or a batch where each element succeeds independently. Dispatch and notification fan-out are the real callers.
- `Partition` preserves input order in both returned slices, so a failure can be matched back to its input.
- `Map` is a free function, not a method, because Go methods cannot introduce a new type parameter.

**Algorithms used** — none.

**Failure modes** — none.

**Tests** — `backend/tests/unit/shared/result_test.go`, including that `Map` never invokes its function on a failed result.
