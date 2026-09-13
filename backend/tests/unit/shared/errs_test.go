package shared

import (
	"errors"
	"fmt"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

func TestKindString(t *testing.T) {
	cases := map[errs.Kind]string{
		errs.KindInternal:     "internal",
		errs.KindInvalid:      "invalid",
		errs.KindNotFound:     "not_found",
		errs.KindConflict:     "conflict",
		errs.KindUnauthorized: "unauthorized",
		errs.KindForbidden:    "forbidden",
		errs.KindRateLimited:  "rate_limited",
		errs.KindUnavailable:  "unavailable",
		errs.Kind(200):        "internal",
	}
	for kind, want := range cases {
		if got := kind.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", kind, got, want)
		}
	}
}

func TestNewAndError(t *testing.T) {
	e := errs.New(errs.KindInvalid, "bad_phone", "Phone number is not valid")
	if got, want := e.Error(), "bad_phone: Phone number is not valid"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if e.Kind != errs.KindInvalid {
		t.Errorf("Kind = %v, want KindInvalid", e.Kind)
	}
	if e.Unwrap() != nil {
		t.Error("Unwrap() should be nil for an error with no cause")
	}
}

func TestErrorWithoutMessage(t *testing.T) {
	e := errs.New(errs.KindNotFound, "not_found", "")
	if got := e.Error(); got != "not_found" {
		t.Errorf("Error() = %q, want %q", got, "not_found")
	}
}

func TestNewf(t *testing.T) {
	e := errs.Newf(errs.KindInvalid, "out_of_range", "radius %d km exceeds %d km", 30, 25)
	want := "out_of_range: radius 30 km exceeds 25 km"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestWrapPreservesCause(t *testing.T) {
	cause := errors.New("connection refused")
	e := errs.Wrap(cause, errs.KindUnavailable, "db_down", "Service is busy")

	if !errors.Is(e, cause) {
		t.Error("errors.Is should find the wrapped cause")
	}
	want := "db_down: Service is busy: connection refused"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestWrapNilReturnsNil(t *testing.T) {
	if e := errs.Wrap(nil, errs.KindInternal, "x", "y"); e != nil {
		t.Errorf("Wrap(nil) = %v, want nil", e)
	}
}

func TestWithCopiesRatherThanMutates(t *testing.T) {
	base := errs.New(errs.KindConflict, "dup", "Already exists").With("zone", "4")
	derived := base.With("merchant", "MER-1")

	if _, ok := base.Fields()["merchant"]; ok {
		t.Error("With must not mutate the receiver")
	}
	got := derived.Fields()
	if got["zone"] != "4" || got["merchant"] != "MER-1" {
		t.Errorf("derived fields = %v, want both zone and merchant", got)
	}
	if keys := derived.FieldKeys(); len(keys) != 2 || keys[0] != "merchant" || keys[1] != "zone" {
		t.Errorf("FieldKeys() = %v, want sorted [merchant zone]", keys)
	}
}

func TestFieldsReturnsCopy(t *testing.T) {
	e := errs.New(errs.KindInvalid, "c", "m").With("k", "v")
	f := e.Fields()
	f["k"] = "tampered"
	if e.Fields()["k"] != "v" {
		t.Error("Fields() must return a copy the caller cannot use to mutate the error")
	}
}

func TestWithPreservesWrappedCause(t *testing.T) {
	cause := errors.New("boom")
	e := errs.Wrap(cause, errs.KindInternal, "c", "m").With("k", "v")
	if !errors.Is(e, cause) {
		t.Error("With must carry the wrapped cause across")
	}
}

func TestKindOf(t *testing.T) {
	if got := errs.KindOf(errs.New(errs.KindForbidden, "c", "m")); got != errs.KindForbidden {
		t.Errorf("KindOf = %v, want KindForbidden", got)
	}
	if got := errs.KindOf(errors.New("plain")); got != errs.KindInternal {
		t.Errorf("KindOf(plain) = %v, want KindInternal", got)
	}
	if got := errs.KindOf(nil); got != errs.KindInternal {
		t.Errorf("KindOf(nil) = %v, want KindInternal", got)
	}
}

func TestKindOfFindsNestedError(t *testing.T) {
	inner := errs.New(errs.KindRateLimited, "slow_down", "Too many attempts")
	outer := fmt.Errorf("handling request: %w", inner)
	if got := errs.KindOf(outer); got != errs.KindRateLimited {
		t.Errorf("KindOf(nested) = %v, want KindRateLimited", got)
	}
}

func TestCodeOf(t *testing.T) {
	if got := errs.CodeOf(errs.New(errs.KindInvalid, "bad_input", "m")); got != "bad_input" {
		t.Errorf("CodeOf = %q, want bad_input", got)
	}
	if got := errs.CodeOf(errors.New("plain")); got != "internal_error" {
		t.Errorf("CodeOf(plain) = %q, want internal_error", got)
	}
}

func TestMessageOfNeverLeaksInfrastructureDetail(t *testing.T) {
	generic := "Something went wrong. Please try again."
	if got := errs.MessageOf(errors.New("pq: relation \"users\" does not exist")); got != generic {
		t.Errorf("MessageOf(plain) = %q, want the generic message", got)
	}
	if got := errs.MessageOf(errs.New(errs.KindInvalid, "c", "")); got != generic {
		t.Errorf("MessageOf(empty message) = %q, want the generic message", got)
	}
	if got := errs.MessageOf(errs.New(errs.KindInvalid, "c", "Pick a delivery address")); got != "Pick a delivery address" {
		t.Errorf("MessageOf = %q, want the supplied message", got)
	}
}

func TestIs(t *testing.T) {
	e := errs.New(errs.KindNotFound, "c", "m")
	if !errs.Is(e, errs.KindNotFound) {
		t.Error("Is should match the error's kind")
	}
	if errs.Is(e, errs.KindConflict) {
		t.Error("Is should not match a different kind")
	}
}
