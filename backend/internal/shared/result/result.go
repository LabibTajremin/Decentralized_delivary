// Package result carries a value or an error as one type.
//
// Go's idiomatic (T, error) pair is preferred in ordinary code. Result exists
// for the places where a fallible value must be stored or moved as a single
// item — a channel of outcomes, a batch where each element succeeds or fails
// independently — which is common in dispatch and notification fan-out.
package result

// Result holds either a value or an error, never both.
type Result[T any] struct {
	value T
	err   error
}

// Ok wraps a successful value.
func Ok[T any](v T) Result[T] { return Result[T]{value: v} }

// Err wraps a failure.
func Err[T any](err error) Result[T] { return Result[T]{err: err} }

// IsOk reports whether the result holds a value.
func (r Result[T]) IsOk() bool { return r.err == nil }

// IsErr reports whether the result holds an error.
func (r Result[T]) IsErr() bool { return r.err != nil }

// Error returns the error, or nil on success.
func (r Result[T]) Error() error { return r.err }

// Unpack returns the value and error together, for callers crossing back into
// ordinary Go style.
func (r Result[T]) Unpack() (T, error) { return r.value, r.err }

// ValueOr returns the value on success, or the fallback on failure.
func (r Result[T]) ValueOr(fallback T) T {
	if r.err != nil {
		return fallback
	}
	return r.value
}

// Map applies fn to a successful value, passing any error through untouched.
// It is a free function rather than a method because Go methods cannot
// introduce a new type parameter.
func Map[T, U any](r Result[T], fn func(T) U) Result[U] {
	if r.err != nil {
		return Err[U](r.err)
	}
	return Ok(fn(r.value))
}

// Partition splits a batch into the values that succeeded and the errors that
// did not, preserving order. Fan-out callers use it to act on the successes
// while still reporting every failure.
func Partition[T any](results []Result[T]) ([]T, []error) {
	values := make([]T, 0, len(results))
	errs := make([]error, 0)
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}
		values = append(values, r.value)
	}
	return values, errs
}
