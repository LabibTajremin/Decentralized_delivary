package shared

import (
	"errors"
	"strconv"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/result"
)

func TestOk(t *testing.T) {
	r := result.Ok(42)
	if !r.IsOk() || r.IsErr() {
		t.Error("Ok must report success")
	}
	if r.Error() != nil {
		t.Errorf("Error() = %v, want nil", r.Error())
	}
	v, err := r.Unpack()
	if v != 42 || err != nil {
		t.Errorf("Unpack() = (%v, %v), want (42, nil)", v, err)
	}
}

func TestErr(t *testing.T) {
	cause := errors.New("boom")
	r := result.Err[int](cause)
	if r.IsOk() || !r.IsErr() {
		t.Error("Err must report failure")
	}
	if !errors.Is(r.Error(), cause) {
		t.Errorf("Error() = %v, want the cause", r.Error())
	}
	v, err := r.Unpack()
	if v != 0 || !errors.Is(err, cause) {
		t.Errorf("Unpack() = (%v, %v), want (0, cause)", v, err)
	}
}

func TestValueOr(t *testing.T) {
	if got := result.Ok("real").ValueOr("fallback"); got != "real" {
		t.Errorf("ValueOr on success = %q, want %q", got, "real")
	}
	if got := result.Err[string](errors.New("x")).ValueOr("fallback"); got != "fallback" {
		t.Errorf("ValueOr on failure = %q, want %q", got, "fallback")
	}
}

func TestMap(t *testing.T) {
	got := result.Map(result.Ok(7), strconv.Itoa)
	if v, err := got.Unpack(); v != "7" || err != nil {
		t.Errorf("Map on success = (%q, %v), want (\"7\", nil)", v, err)
	}

	cause := errors.New("nope")
	mapped := result.Map(result.Err[int](cause), func(int) string {
		t.Error("Map must not call fn on a failed result")
		return ""
	})
	if !errors.Is(mapped.Error(), cause) {
		t.Errorf("Map must pass the error through, got %v", mapped.Error())
	}
}

func TestPartition(t *testing.T) {
	e1, e2 := errors.New("one"), errors.New("two")
	values, errs := result.Partition([]result.Result[int]{
		result.Ok(1), result.Err[int](e1), result.Ok(2), result.Err[int](e2), result.Ok(3),
	})

	if len(values) != 3 || values[0] != 1 || values[1] != 2 || values[2] != 3 {
		t.Errorf("values = %v, want [1 2 3] in order", values)
	}
	if len(errs) != 2 || !errors.Is(errs[0], e1) || !errors.Is(errs[1], e2) {
		t.Errorf("errors = %v, want both failures in order", errs)
	}
}

func TestPartitionEmpty(t *testing.T) {
	values, errs := result.Partition([]result.Result[string]{})
	if len(values) != 0 || len(errs) != 0 {
		t.Errorf("empty partition = (%v, %v), want both empty", values, errs)
	}
}
