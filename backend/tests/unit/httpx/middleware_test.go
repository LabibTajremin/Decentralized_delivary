package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
)

// fixedGen returns the same id every time, so an assertion can name it.
type fixedGen struct{ value string }

func (f fixedGen) New(string) string { return f.value }

// capture runs a handler through middleware and returns the response plus the
// structured log lines it produced.
func capture(t *testing.T, h http.Handler, r *http.Request, mw ...func(*slog.Logger) httpx.Middleware) (*httptest.ResponseRecorder, []map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	chain := make([]httpx.Middleware, 0, len(mw))
	for _, m := range mw {
		chain = append(chain, m(logger))
	}
	rec := httptest.NewRecorder()
	httpx.Chain(h, chain...).ServeHTTP(rec, r)

	var lines []map[string]any
	for _, raw := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if raw == "" {
			continue
		}
		var line map[string]any
		if err := json.Unmarshal([]byte(raw), &line); err != nil {
			t.Fatalf("log line is not JSON: %q", raw)
		}
		lines = append(lines, line)
	}
	return rec, lines
}

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("ok"))
})

func TestChainAppliesMiddlewareOutermostFirst(t *testing.T) {
	var order []string
	mark := func(name string) httpx.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	rec := httptest.NewRecorder()
	httpx.Chain(okHandler, mark("first"), mark("second"), mark("third")).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if strings.Join(order, ",") != "first,second,third" {
		t.Errorf("order = %v, want the first listed to run first", order)
	}
}

func TestRequestIDGeneratesAndEchoesAnID(t *testing.T) {
	var seen string
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = httpx.RequestIDFrom(r.Context())
	})
	rec := httptest.NewRecorder()
	httpx.Chain(h, httpx.RequestID(fixedGen{value: "req_generated"})).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if seen != "req_generated" {
		t.Errorf("context id = %q", seen)
	}
	if got := rec.Header().Get(httpx.RequestIDHeader); got != "req_generated" {
		t.Errorf("echoed id = %q", got)
	}
}

func TestRequestIDKeepsAnInboundID(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(httpx.RequestIDHeader, "trace-from-the-gateway")

	var seen string
	h := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = httpx.RequestIDFrom(r.Context())
	})
	httpx.Chain(h, httpx.RequestID(fixedGen{value: "generated"})).
		ServeHTTP(httptest.NewRecorder(), r)

	if seen != "trace-from-the-gateway" {
		t.Errorf("id = %q, want the inbound id preserved for tracing", seen)
	}
}

// TestRequestIDRejectsLogInjection is a security property. The id is written
// into log lines, so a caller who can put newlines in it can forge log entries
// and hide their own activity. A partly-hostile value is discarded entirely
// rather than salvaged — half-trusting a hostile input is not a position worth
// defending.
func TestRequestIDRejectsLogInjection(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(httpx.RequestIDHeader, "abc\n{\"level\":\"INFO\",\"msg\":\"all clear\"}")

	var seen string
	h := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = httpx.RequestIDFrom(r.Context())
	})
	httpx.Chain(h, httpx.RequestID(fixedGen{value: "generated"})).
		ServeHTTP(httptest.NewRecorder(), r)

	if strings.ContainsAny(seen, "\n\r\"{} ") {
		t.Errorf("id = %q still carries characters that can forge a log line", seen)
	}
	if seen != "generated" {
		t.Errorf("id = %q, want the hostile value discarded for a generated one", seen)
	}
}

// An overlong id is rejected rather than truncated: a 5 KB "id" repeated in
// every log line is a denial of service against the log pipeline, not a trace.
func TestRequestIDRejectsAnOverlongInboundID(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(httpx.RequestIDHeader, strings.Repeat("a", 5000))

	var seen string
	h := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = httpx.RequestIDFrom(r.Context())
	})
	httpx.Chain(h, httpx.RequestID(fixedGen{value: "generated"})).
		ServeHTTP(httptest.NewRecorder(), r)

	if seen != "generated" {
		t.Errorf("id = %q, want an overlong id discarded", seen)
	}
}

// An id made entirely of unsafe characters falls back to a generated one
// rather than leaving requests uncorrelated.
func TestRequestIDFallsBackWhenAnInboundIDIsAllUnsafe(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(httpx.RequestIDHeader, `!!!@@@###`)

	var seen string
	h := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = httpx.RequestIDFrom(r.Context())
	})
	httpx.Chain(h, httpx.RequestID(fixedGen{value: "req_generated"})).
		ServeHTTP(httptest.NewRecorder(), r)

	if seen != "req_generated" {
		t.Errorf("id = %q, want a generated fallback", seen)
	}
}

func TestRequestIDFromIsEmptyOutsideARequest(t *testing.T) {
	if got := httpx.RequestIDFrom(context.Background()); got != "" {
		t.Errorf("RequestIDFrom = %q, want empty", got)
	}
}

func TestWithRequestIDPutsAnIDOnTheContext(t *testing.T) {
	ctx := httpx.WithRequestID(context.Background(), "req_x")
	if got := httpx.RequestIDFrom(ctx); got != "req_x" {
		t.Errorf("RequestIDFrom = %q", got)
	}
}

// TestRecoverTurnsAPanicIntoA500: one handler's panic must not take down the
// requests in flight that have done nothing wrong.
func TestRecoverTurnsAPanicIntoA500(t *testing.T) {
	boom := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("index out of range")
	})
	rec, lines := capture(t, boom, httptest.NewRequest(http.MethodGet, "/orders", nil), httpx.Recover)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "index out of range") {
		t.Errorf("the panic message must not reach the client: %s", rec.Body.String())
	}
	if len(lines) == 0 || lines[0]["msg"] != "panic in handler" {
		t.Fatalf("the panic must be logged, got %v", lines)
	}
	if lines[0]["path"] != "/orders" {
		t.Errorf("log line = %v, want the path recorded", lines[0])
	}
}

func TestRecoverLeavesANormalResponseAlone(t *testing.T) {
	rec, _ := capture(t, okHandler, httptest.NewRequest(http.MethodGet, "/", nil), httpx.Recover)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Errorf("status = %d body = %q", rec.Code, rec.Body.String())
	}
}

func TestLoggingRecordsOneLinePerRequest(t *testing.T) {
	rec, lines := capture(t, okHandler, httptest.NewRequest(http.MethodGet, "/healthz", nil), httpx.Logging)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1: %v", len(lines), lines)
	}
	line := lines[0]
	if line["method"] != "GET" || line["path"] != "/healthz" || line["status"] != float64(200) {
		t.Errorf("log line = %v", line)
	}
	if line["bytes"] != float64(2) {
		t.Errorf("bytes = %v, want 2", line["bytes"])
	}
	if line["level"] != "INFO" {
		t.Errorf("level = %v, want INFO for a success", line["level"])
	}
}

// TestLoggingSeparatesOurFaultsFromTheirs: an alert on error lines should fire
// for our bugs, not for someone sending a bad postcode.
func TestLoggingSeparatesOurFaultsFromTheirs(t *testing.T) {
	cases := map[int]string{
		http.StatusBadRequest:          "INFO",
		http.StatusNotFound:            "INFO",
		http.StatusInternalServerError: "ERROR",
		http.StatusServiceUnavailable:  "ERROR",
	}
	for status, wantLevel := range cases {
		h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) })
		_, lines := capture(t, h, httptest.NewRequest(http.MethodGet, "/", nil), httpx.Logging)
		if len(lines) != 1 {
			t.Fatalf("status %d: got %d lines", status, len(lines))
		}
		if lines[0]["level"] != wantLevel {
			t.Errorf("status %d logged at %v, want %s", status, lines[0]["level"], wantLevel)
		}
	}
}

// A handler that writes nothing at all still produced a 200 on the wire.
func TestLoggingRecordsAnImplicit200(t *testing.T) {
	silent := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	_, lines := capture(t, silent, httptest.NewRequest(http.MethodGet, "/", nil), httpx.Logging)
	if lines[0]["status"] != float64(200) {
		t.Errorf("status = %v, want 200", lines[0]["status"])
	}
}

// Only the first WriteHeader counts, as with a real ResponseWriter.
func TestLoggingIgnoresASecondWriteHeader(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.WriteHeader(http.StatusTeapot)
	})
	_, lines := capture(t, h, httptest.NewRequest(http.MethodGet, "/", nil), httpx.Logging)
	if lines[0]["status"] != float64(201) {
		t.Errorf("status = %v, want the first status to win", lines[0]["status"])
	}
}

func TestCORSAllowsAConfiguredOrigin(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Origin", "https://admin.goklay.com")

	rec := httptest.NewRecorder()
	httpx.Chain(okHandler, httpx.CORS([]string{"https://admin.goklay.com"})).ServeHTTP(rec, r)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://admin.goklay.com" {
		t.Errorf("allow-origin = %q", got)
	}
	if got := rec.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("Vary = %q, want Origin so a shared cache does not cross origins", got)
	}
}

// TestCORSRefusesAnUnknownOrigin is the security property: reflecting an
// arbitrary Origin would let any website make credentialed calls on a
// signed-in admin's behalf.
func TestCORSRefusesAnUnknownOrigin(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Origin", "https://evil.example")

	rec := httptest.NewRecorder()
	httpx.Chain(okHandler, httpx.CORS([]string{"https://admin.goklay.com"})).ServeHTTP(rec, r)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("allow-origin = %q, want no CORS grant for an unlisted origin", got)
	}
}

// An empty allow-list is the mobile-only deployment, and must grant nothing.
func TestCORSWithNoConfiguredOriginsGrantsNothing(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Origin", "https://admin.goklay.com")

	rec := httptest.NewRecorder()
	httpx.Chain(okHandler, httpx.CORS(nil)).ServeHTTP(rec, r)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("allow-origin = %q, want nothing", got)
	}
}

func TestCORSAnswersPreflightWithoutReachingTheHandler(t *testing.T) {
	reached := false
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true })

	r := httptest.NewRequest(http.MethodOptions, "/v1/geo/resolve", nil)
	r.Header.Set("Origin", "https://admin.goklay.com")
	rec := httptest.NewRecorder()
	httpx.Chain(h, httpx.CORS([]string{"https://admin.goklay.com"})).ServeHTTP(rec, r)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if reached {
		t.Error("a preflight must not reach the handler")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Errorf("allow-methods = %q", got)
	}
}

func TestCORSLeavesARequestWithNoOriginAlone(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.Chain(okHandler, httpx.CORS([]string{"https://admin.goklay.com"})).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("allow-origin = %q, want nothing for a non-browser call", got)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body = %q, want the handler to have run", rec.Body.String())
	}
}

// TestCORSCannotBeWidenedByMutatingTheCallersSlice: the middleware clones its
// allow-list, so a caller that later appends to the slice it passed cannot
// widen an already-built server's CORS policy.
func TestCORSCannotBeWidenedByMutatingTheCallersSlice(t *testing.T) {
	origins := make([]string, 1, 2)
	origins[0] = "https://admin.goklay.com"
	mw := httpx.CORS(origins)
	origins = append(origins, "https://evil.example")
	_ = origins

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	httpx.Chain(okHandler, mw).ServeHTTP(rec, r)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("allow-origin = %q, want the policy fixed at construction", got)
	}
}

func TestSecurityHeadersAreSet(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.Chain(okHandler, httpx.SecurityHeaders()).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	}
	for header, value := range want {
		if got := rec.Header().Get(header); got != value {
			t.Errorf("%s = %q, want %q", header, got, value)
		}
	}
}
