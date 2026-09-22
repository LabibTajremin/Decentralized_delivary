package tracking

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/application"
	dispatchx "github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/external/dispatch"
	trackinghttp "github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/transport/http"
)

// server mounts the handler over a use case. The guard passes everything
// through — what is under test is what the handler does with a caller, not
// the middleware that identifies one.
func server(snapshot *application.SnapshotUseCase, interval time.Duration, userID string) *http.ServeMux {
	mux := http.NewServeMux()
	open := func(next http.Handler) http.Handler { return next }
	trackinghttp.NewHandler(snapshot, interval, open,
		func(*http.Request) (string, bool) { return userID, userID != "" },
	).Register(mux)
	return mux
}

// events splits an SSE body into its "data: ..." payloads.
func events(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "data: ") {
			out = append(out, strings.TrimPrefix(line, "data: "))
		}
	}
	return out
}

func TestPatternsMatchWhatIsMounted(t *testing.T) {
	patterns := trackinghttp.Patterns()
	if len(patterns) != 1 || patterns[0] != "GET /v1/track/{orderId}" {
		t.Fatalf("Patterns() = %v", patterns)
	}
}

// A zero interval falls back to a sane default rather than a ticker that
// fires as fast as the CPU allows.
func TestNewHandlerDefaultsAZeroInterval(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = nonLiveOrder("ord_1", "usr_1")
	mux := server(r.snapshot, 0, "usr_1")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/track/ord_1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestNewHandlerPanicsWithoutAGuard(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewHandler did not panic with a nil guard")
		}
	}()
	trackinghttp.NewHandler(nil, 0, nil, func(*http.Request) (string, bool) { return "", false })
}

func TestNewHandlerPanicsWithoutAPrincipalReader(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewHandler did not panic with a nil principal reader")
		}
	}()
	trackinghttp.NewHandler(nil, 0, func(next http.Handler) http.Handler { return next }, nil)
}

func TestStreamRefusesAnUnauthenticatedCaller(t *testing.T) {
	r := newRig()
	mux := server(r.snapshot, time.Millisecond, "")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/track/ord_1", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestStreamRefusesACallerWhoMayNotWatch(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = liveOrder("ord_1", "usr_owner")
	mux := server(r.snapshot, time.Millisecond, "usr_stranger")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/track/ord_1", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// A terminal order gets exactly one frame and the stream ends there — there
// is nothing left for a ticker to watch.
func TestStreamOnATerminalOrder(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = nonLiveOrder("ord_1", "usr_1")
	mux := server(r.snapshot, time.Millisecond, "usr_1")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/track/ord_1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q", ct)
	}
	got := events(rec.Body.String())
	if len(got) != 1 || !strings.Contains(got[0], `"status":"delivered"`) {
		t.Fatalf("events = %v", got)
	}
}

// A live order streams an update once the fake starts reporting a changed
// status, deterministically — the fake counts calls rather than the test
// racing a real clock to produce a particular sequence.
func TestStreamSendsAnUpdateWhenSomethingChanges(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = liveOrder("ord_1", "usr_1")
	r.order.toggleAfter = 1
	r.order.later = nonLiveOrder("ord_1", "usr_1")
	r.dispatch.jobs["ord_1"] = dispatchx.Job{
		OrderID: "ord_1", Live: true,
		Partner: dispatchx.Partner{ID: "PTR-1", Name: "Karim", Lat: 23.7, Lng: 90.4},
	}

	mux := server(r.snapshot, 2*time.Millisecond, "usr_1")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/track/ord_1", nil).WithContext(ctx))

	got := events(rec.Body.String())
	if len(got) < 2 {
		t.Fatalf("events = %v, want at least the initial frame and one update", got)
	}
	if !strings.Contains(got[0], `"status":"picked_up"`) || !strings.Contains(got[0], `"id":"PTR-1"`) {
		t.Fatalf("first event = %s", got[0])
	}
	last := got[len(got)-1]
	if !strings.Contains(last, `"status":"delivered"`) {
		t.Fatalf("last event = %s, want the delivered status", last)
	}
}

// The client going away ends the loop — proven by the handler returning
// before its own context timeout, on a request whose context is already
// cancelled.
func TestStreamEndsWhenTheClientGoesAway(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = liveOrder("ord_1", "usr_1")
	mux := server(r.snapshot, time.Millisecond, "usr_1")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/track/ord_1", nil).WithContext(ctx))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the stream did not stop when the client's context was already cancelled")
	}
}

// A storage failure partway through a stream ends it rather than looping
// forever retrying.
func TestStreamStopsOnAMidStreamStorageFailure(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = liveOrder("ord_1", "usr_1")
	r.order.failAfter = 1

	mux := server(r.snapshot, 2*time.Millisecond, "usr_1")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/track/ord_1", nil).WithContext(ctx))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the stream did not stop after the underlying lookup started failing")
	}
}

// notAFlusher wraps a real ResponseWriter behind the bare interface, so only
// http.ResponseWriter's own methods are promoted — not Flush, which
// httptest.ResponseRecorder also has as a concrete method. This is what
// stands in for whatever the real server might hand the chain in a context
// where streaming was never possible.
type notAFlusher struct{ http.ResponseWriter }

// A caller reaching the handler without a Flusher gets a clean failure
// rather than a stream that silently never delivers anything.
func TestStreamRefusesAWriterThatCannotFlush(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = liveOrder("ord_1", "usr_1")
	mux := server(r.snapshot, time.Millisecond, "usr_1")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(notAFlusher{rec}, httptest.NewRequest(http.MethodGet, "/v1/track/ord_1", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}
