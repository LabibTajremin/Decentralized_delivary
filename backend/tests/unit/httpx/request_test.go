package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

type payload struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

// jsonRequest builds a POST with a JSON content type.
func jsonRequest(body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestDecodeJSONReadsAValidBody(t *testing.T) {
	var got payload
	if err := httpx.DecodeJSON(httptest.NewRecorder(), jsonRequest(`{"name":"rice","quantity":2}`), &got); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if got.Name != "rice" || got.Quantity != 2 {
		t.Errorf("decoded = %+v", got)
	}
}

// TestDecodeJSONRejectsUnknownFields is the one that matters for correctness:
// a client sending "quantiy" must be told, not have the value silently dropped
// and the order placed wrong.
func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	var got payload
	err := httpx.DecodeJSON(httptest.NewRecorder(), jsonRequest(`{"name":"rice","quantiy":2}`), &got)
	if errs.CodeOf(err) != "unknown_field" {
		t.Fatalf("error = %v, want unknown_field", err)
	}
	var e *errs.Error
	if !asErr(err, &e) || e.Fields()["field"] != "quantiy" {
		t.Errorf("error must name the offending field, got %v", err)
	}
}

func TestDecodeJSONRejectsAWrongFieldType(t *testing.T) {
	var got payload
	err := httpx.DecodeJSON(httptest.NewRecorder(), jsonRequest(`{"quantity":"two"}`), &got)
	if errs.CodeOf(err) != "invalid_field_type" {
		t.Errorf("error = %v, want invalid_field_type", err)
	}
}

// A truncated body ends as an unexpected EOF rather than a syntax error, so
// both shapes have to reach the same client-facing answer.
func TestDecodeJSONRejectsATruncatedBody(t *testing.T) {
	var got payload
	err := httpx.DecodeJSON(httptest.NewRecorder(), jsonRequest(`{"name":`), &got)
	if errs.CodeOf(err) != "malformed_json" {
		t.Errorf("error = %v, want malformed_json", err)
	}
}

func TestDecodeJSONRejectsASyntaxError(t *testing.T) {
	var got payload
	err := httpx.DecodeJSON(httptest.NewRecorder(), jsonRequest(`{"name": }`), &got)
	if errs.CodeOf(err) != "malformed_json" {
		t.Errorf("error = %v, want malformed_json", err)
	}
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Errorf("error = %v, want a json.SyntaxError underneath so the cause survives", err)
	}
}

func TestDecodeJSONRejectsAnEmptyBody(t *testing.T) {
	var got payload
	err := httpx.DecodeJSON(httptest.NewRecorder(), jsonRequest(``), &got)
	if errs.CodeOf(err) != "empty_body" {
		t.Errorf("error = %v, want empty_body", err)
	}
}

// TestDecodeJSONRejectsASecondDocument: taking only the first would silently
// ignore whatever else the client sent.
func TestDecodeJSONRejectsASecondDocument(t *testing.T) {
	var got payload
	err := httpx.DecodeJSON(httptest.NewRecorder(), jsonRequest(`{"name":"a"}{"name":"b"}`), &got)
	if errs.CodeOf(err) != "malformed_json" {
		t.Errorf("error = %v, want a single-object requirement", err)
	}
}

func TestDecodeJSONRejectsANonJSONContentType(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "text/xml")
	var got payload
	if err := httpx.DecodeJSON(httptest.NewRecorder(), r, &got); errs.CodeOf(err) != "unsupported_media_type" {
		t.Errorf("error = %v, want unsupported_media_type", err)
	}
}

func TestDecodeJSONAcceptsAContentTypeWithCharset(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"a"}`))
	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	var got payload
	if err := httpx.DecodeJSON(httptest.NewRecorder(), r, &got); err != nil {
		t.Errorf("DecodeJSON: %v", err)
	}
}

func TestDecodeJSONAcceptsAMissingContentType(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"a"}`))
	var got payload
	if err := httpx.DecodeJSON(httptest.NewRecorder(), r, &got); err != nil {
		t.Errorf("DecodeJSON: %v", err)
	}
}

// TestDecodeJSONRejectsAnOversizedBody: without a cap one request can make the
// process allocate until it dies, taking every other request with it.
func TestDecodeJSONRejectsAnOversizedBody(t *testing.T) {
	huge := `{"name":"` + strings.Repeat("x", 2<<20) + `"}`
	var got payload
	err := httpx.DecodeJSON(httptest.NewRecorder(), jsonRequest(huge), &got)
	if errs.CodeOf(err) != "request_too_large" {
		t.Errorf("error = %v, want request_too_large", err)
	}
}

func TestRequiredFloatReadsAValue(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?lat=23.81", nil)
	got, err := httpx.RequiredFloat(r, "lat")
	if err != nil || got != 23.81 {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestRequiredFloatReportsAMissingParameter(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := httpx.RequiredFloat(r, "lat")
	if errs.CodeOf(err) != "missing_parameter" {
		t.Errorf("error = %v, want missing_parameter", err)
	}
}

func TestRequiredFloatReportsABlankParameter(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?lat=%20", nil)
	if _, err := httpx.RequiredFloat(r, "lat"); errs.CodeOf(err) != "missing_parameter" {
		t.Errorf("error = %v, want a blank value treated as missing", err)
	}
}

func TestRequiredFloatReportsANonNumber(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?lat=north", nil)
	if _, err := httpx.RequiredFloat(r, "lat"); errs.CodeOf(err) != "invalid_parameter" {
		t.Errorf("error = %v, want invalid_parameter", err)
	}
}

func TestOptionalIntFallsBackToTheDefault(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	got, err := httpx.OptionalInt(r, "limit", 20)
	if err != nil || got != 20 {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestOptionalIntReportsANonNumber(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=many", nil)
	if _, err := httpx.OptionalInt(r, "limit", 20); errs.CodeOf(err) != "invalid_parameter" {
		t.Errorf("error = %v, want invalid_parameter", err)
	}
}

// TestBoundedIntClampsAboveTheMaximum: a client asking for 10,000 results means
// "as many as you'll give me", and answering with the maximum is more useful
// than refusing.
func TestBoundedIntClampsAboveTheMaximum(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=10000", nil)
	got, err := httpx.BoundedInt(r, "limit", 20, 1, 100)
	if err != nil || got != 100 {
		t.Errorf("got %v, %v; want 100", got, err)
	}
}

// A value below the minimum is a caller bug, and silently substituting the
// minimum would hide it.
func TestBoundedIntRejectsBelowTheMinimum(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=0", nil)
	_, err := httpx.BoundedInt(r, "limit", 20, 1, 100)
	if errs.CodeOf(err) != "invalid_parameter" {
		t.Fatalf("error = %v, want invalid_parameter", err)
	}
	var e *errs.Error
	if !asErr(err, &e) || e.Fields()["minimum"] != "1" {
		t.Errorf("error must state the minimum, got %v", err)
	}
}

func TestBoundedIntPassesThroughAValidValue(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=50", nil)
	got, err := httpx.BoundedInt(r, "limit", 20, 1, 100)
	if err != nil || got != 50 {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestBoundedIntSurfacesAParseFailure(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=lots", nil)
	if _, err := httpx.BoundedInt(r, "limit", 20, 1, 100); err == nil {
		t.Error("BoundedInt must surface a parse failure")
	}
}

// failingBody errors partway through, which is what a client that disconnects
// mid-upload looks like to the server.
type failingBody struct{ err error }

func (f failingBody) Read([]byte) (int, error) { return 0, f.err }
func (f failingBody) Close() error             { return nil }

// TestDecodeJSONSurfacesAnUnreadableBody covers the fallback: the decoder can
// fail for reasons that are not syntax, type or size, and those must still
// become a clear client error rather than an unhandled nil.
func TestDecodeJSONSurfacesAnUnreadableBody(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Content-Type", "application/json")
	r.Body = failingBody{err: errors.New("connection reset by peer")}

	var got payload
	err := httpx.DecodeJSON(httptest.NewRecorder(), r, &got)
	if errs.CodeOf(err) != "malformed_json" {
		t.Errorf("error = %v, want the read failure reported as a bad body", err)
	}
	if !errors.Is(err, err) || err == nil {
		t.Error("DecodeJSON must return an error for an unreadable body")
	}
}
