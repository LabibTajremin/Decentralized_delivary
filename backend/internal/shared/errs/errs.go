// Package errs is the single error vocabulary for the backend.
//
// An error carries a Kind (what class of failure this is), a stable machine
// Code the API returns verbatim, and a Message safe to show a user. Transport
// maps Kind to an HTTP status; nothing in here knows about HTTP, so domain and
// application code can return these freely without importing a transport
// concern.
package errs

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Kind classifies a failure. Transport maps it to a status code; use cases
// branch on it. It is deliberately small — a longer list invites disagreement
// about which one applies.
type Kind uint8

const (
	// KindInternal is an unexpected failure. It is the zero value so an
	// uninitialised Kind is never accidentally treated as a client error.
	KindInternal Kind = iota
	// KindInvalid means the caller sent something malformed or out of range.
	KindInvalid
	// KindNotFound means the addressed resource does not exist.
	KindNotFound
	// KindConflict means the request collides with current state.
	KindConflict
	// KindUnauthorized means the caller is not authenticated.
	KindUnauthorized
	// KindForbidden means the caller is authenticated but not permitted.
	KindForbidden
	// KindRateLimited means the caller exceeded an allowance.
	KindRateLimited
	// KindUnavailable means a dependency is down or timed out.
	KindUnavailable
)

// String renders the Kind for logs.
func (k Kind) String() string {
	switch k {
	case KindInvalid:
		return "invalid"
	case KindNotFound:
		return "not_found"
	case KindConflict:
		return "conflict"
	case KindUnauthorized:
		return "unauthorized"
	case KindForbidden:
		return "forbidden"
	case KindRateLimited:
		return "rate_limited"
	case KindUnavailable:
		return "unavailable"
	case KindInternal:
		return "internal"
	default:
		return "internal"
	}
}

// Error is the backend's error type.
type Error struct {
	Kind    Kind
	Code    string
	Message string
	fields  map[string]string
	wrapped error
}

// Error implements the error interface.
func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(e.Code)
	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}
	if e.wrapped != nil {
		b.WriteString(": ")
		b.WriteString(e.wrapped.Error())
	}
	return b.String()
}

// Unwrap exposes the wrapped cause to errors.Is and errors.As.
func (e *Error) Unwrap() error { return e.wrapped }

// Fields returns the structured context attached to this error, sorted by key
// so log output and test assertions are stable. The returned map is a copy.
func (e *Error) Fields() map[string]string {
	out := make(map[string]string, len(e.fields))
	for k, v := range e.fields {
		out[k] = v
	}
	return out
}

// FieldKeys returns the attached field names in sorted order.
func (e *Error) FieldKeys() []string {
	keys := make([]string, 0, len(e.fields))
	for k := range e.fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// With returns a copy carrying an extra structured field. The receiver is not
// modified, so an error value shared across goroutines stays safe.
func (e *Error) With(key, value string) *Error {
	clone := &Error{
		Kind:    e.Kind,
		Code:    e.Code,
		Message: e.Message,
		wrapped: e.wrapped,
		fields:  make(map[string]string, len(e.fields)+1),
	}
	for k, v := range e.fields {
		clone.fields[k] = v
	}
	clone.fields[key] = value
	return clone
}

// New builds an error with no cause.
func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

// Wrap attaches a Kind, Code and Message to an existing cause. Wrapping nil
// returns nil so callers can write `return errs.Wrap(err, ...)` unconditionally.
func Wrap(err error, kind Kind, code, message string) *Error {
	if err == nil {
		return nil
	}
	return &Error{Kind: kind, Code: code, Message: message, wrapped: err}
}

// Newf is New with a formatted message.
func Newf(kind Kind, code, format string, args ...any) *Error {
	return New(kind, code, fmt.Sprintf(format, args...))
}

// KindOf reports the Kind of any error. An error that is not an *Error — and a
// nil error — is reported as KindInternal, so an unclassified failure is never
// mistaken for a client mistake.
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindInternal
}

// CodeOf returns the stable machine code of an error, or "internal_error" when
// the error carries none.
func CodeOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return "internal_error"
}

// MessageOf returns a message safe to show a user. Errors that are not *Error
// may carry infrastructure detail, so they are replaced with a generic string
// rather than leaked.
func MessageOf(err error) string {
	var e *Error
	if errors.As(err, &e) && e.Message != "" {
		return e.Message
	}
	return "Something went wrong. Please try again."
}

// Is reports whether err has the given Kind.
func Is(err error, kind Kind) bool { return KindOf(err) == kind }
