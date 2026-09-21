package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// decodeError reads the standard error envelope out of a response.
func decodeError(t *testing.T, rec *httptest.ResponseRecorder) httpx.ErrorBody {
	t.Helper()
	var body struct {
		Error httpx.ErrorBody `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error envelope: %v\nbody: %s", err, rec.Body.String())
	}
	return body.Error
}

func TestWriteJSONSetsStatusAndContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.WriteJSON(rec, http.StatusCreated, map[string]string{"id": "abc"})

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type = %q", ct)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"id":"abc"}` {
		t.Errorf("body = %q", rec.Body.String())
	}
}

// TestWriteJSONOnAnUnmarshallableValueFailsLoudly: writing the status first
// would produce a 200 with a truncated body, which a client cannot distinguish
// from a successful response.
func TestWriteJSONOnAnUnmarshallableValueFailsLoudly(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.WriteJSON(rec, http.StatusOK, map[string]any{"bad": make(chan int)})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if got := decodeError(t, rec).Code; got != "response_encoding_failed" {
		t.Errorf("code = %q", got)
	}
}

func TestWriteNoContent(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.WriteNoContent(rec)
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestStatusForMapsEveryKind(t *testing.T) {
	cases := map[errs.Kind]int{
		errs.KindInvalid:      http.StatusBadRequest,
		errs.KindUnauthorized: http.StatusUnauthorized,
		errs.KindForbidden:    http.StatusForbidden,
		errs.KindNotFound:     http.StatusNotFound,
		errs.KindConflict:     http.StatusConflict,
		errs.KindRateLimited:  http.StatusTooManyRequests,
		errs.KindUnavailable:  http.StatusServiceUnavailable,
		errs.KindInternal:     http.StatusInternalServerError,
	}
	for kind, want := range cases {
		if got := httpx.StatusFor(kind); got != want {
			t.Errorf("StatusFor(%v) = %d, want %d", kind, got, want)
		}
	}
	// An out-of-range Kind must fail closed, not as a client error.
	if got := httpx.StatusFor(errs.Kind(200)); got != http.StatusInternalServerError {
		t.Errorf("StatusFor(unknown) = %d, want 500", got)
	}
}

func TestWriteErrorUsesTheErrorsOwnCodeAndMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.WriteError(rec, errs.New(errs.KindNotFound, "merchant_not_found", "We could not find that shop."))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	got := decodeError(t, rec)
	if got.Code != "merchant_not_found" || got.Message != "We could not find that shop." {
		t.Errorf("body = %+v", got)
	}
}

func TestWriteErrorIncludesStructuredDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.WriteError(rec, errs.New(errs.KindInvalid, "invalid_parameter", "That value is not valid.").
		With("parameter", "radius_m"))

	if got := decodeError(t, rec).Details["parameter"]; got != "radius_m" {
		t.Errorf("details = %+v, want the parameter named", decodeError(t, rec).Details)
	}
}

// TestInternalErrorsDoNotLeakTheirMessage is a security property, not a
// cosmetic one: an internal error's message may name a table, a column or a
// connection string, and none of that is the caller's business.
func TestInternalErrorsDoNotLeakTheirMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	leaky := errs.New(errs.KindInternal, "db_failure",
		`pq: relation "users" does not exist on host db-prod-01`).
		With("table", "users")
	httpx.WriteError(rec, leaky)

	body := rec.Body.String()
	for _, secret := range []string{"users", "db-prod-01", "relation"} {
		if strings.Contains(body, secret) {
			t.Errorf("response leaks %q: %s", secret, body)
		}
	}
	got := decodeError(t, rec)
	if got.Details != nil {
		t.Errorf("internal error must not carry details, got %+v", got.Details)
	}
	if !strings.Contains(got.Message, "went wrong") {
		t.Errorf("message = %q, want the generic internal message", got.Message)
	}
}

// A plain error with no Kind is internal by default, so an unclassified failure
// fails closed rather than being reported as the caller's fault.
func TestPlainErrorsAreTreatedAsInternal(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.WriteError(rec, errPlain{})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

type errPlain struct{}

func (errPlain) Error() string { return "something specific about our internals" }
