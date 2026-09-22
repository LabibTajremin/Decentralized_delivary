package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/notification/application"
	notificationhttp "github.com/rootlogic-lab/delivery/backend/internal/modules/notification/transport/http"
)

// server mounts the handler over a rig. The guard passes everything
// through — what is under test is what the handler does with a caller, not
// the middleware that identifies one.
func server(register *application.RegisterDeviceUseCase, reads *application.ReadUseCase, userID string) *http.ServeMux {
	mux := http.NewServeMux()
	open := func(next http.Handler) http.Handler { return next }
	notificationhttp.NewHandler(register, reads, open,
		func(*http.Request) (string, bool) { return userID, userID != "" },
	).Register(mux)
	return mux
}

func call(t *testing.T, mux *http.ServeMux, method, target, body string, into any) int {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if into != nil && rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), into); err != nil {
			t.Fatalf("decode %s %s: %v (%s)", method, target, err, rec.Body)
		}
	}
	return rec.Code
}

func TestPatternsMatchWhatIsMounted(t *testing.T) {
	want := []string{"GET /v1/me/notifications", "POST /v1/me/device"}
	got := append([]string(nil), notificationhttp.Patterns()...)
	sort.Strings(got)
	if len(got) != len(want) {
		t.Fatalf("Patterns() = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Patterns() = %v, want %v", got, want)
		}
	}
}

func TestNewHandlerPanicsWithoutAGuard(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewHandler did not panic with a nil guard")
		}
	}()
	notificationhttp.NewHandler(nil, nil, nil, func(*http.Request) (string, bool) { return "", false })
}

func TestNewHandlerPanicsWithoutAPrincipalReader(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewHandler did not panic with a nil principal reader")
		}
	}()
	notificationhttp.NewHandler(nil, nil, func(next http.Handler) http.Handler { return next }, nil)
}

func TestRegisterDeviceOverHTTP(t *testing.T) {
	r := newRig()
	mux := server(r.register, r.reads, "usr_1")

	code := call(t, mux, http.MethodPost, "/v1/me/device", `{"platform":"ios","token":"tok_1"}`, nil)
	if code != http.StatusNoContent {
		t.Fatalf("status = %d", code)
	}
	if r.repo.devices["usr_1"]["ios"].Token != "tok_1" {
		t.Fatalf("devices = %+v", r.repo.devices)
	}
}

func TestRegisterDeviceRejectsAnUnauthenticatedCaller(t *testing.T) {
	r := newRig()
	mux := server(r.register, r.reads, "")

	code := call(t, mux, http.MethodPost, "/v1/me/device", `{"platform":"ios","token":"tok_1"}`, nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", code)
	}
}

func TestRegisterDeviceRejectsAMalformedBody(t *testing.T) {
	r := newRig()
	mux := server(r.register, r.reads, "usr_1")

	code := call(t, mux, http.MethodPost, "/v1/me/device", `not json`, nil)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", code)
	}
}

func TestRegisterDeviceRejectsAnInvalidPlatform(t *testing.T) {
	r := newRig()
	mux := server(r.register, r.reads, "usr_1")

	code := call(t, mux, http.MethodPost, "/v1/me/device", `{"platform":"windows","token":"tok_1"}`, nil)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", code)
	}
}

type notificationBody struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	Channel string `json:"channel"`
	Status  string `json:"status"`
}

func TestMyNotificationsOverHTTP(t *testing.T) {
	r := newRig()
	r.identity.phones["usr_1"] = "+8801700000000"
	if err := r.notify.Notify(context.Background(), "usr_1", "Title", "Body"); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	mux := server(r.register, r.reads, "usr_1")

	var out []notificationBody
	code := call(t, mux, http.MethodGet, "/v1/me/notifications", "", &out)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if len(out) != 1 || out[0].Title != "Title" {
		t.Fatalf("out = %+v", out)
	}
}

func TestMyNotificationsAcceptsALimit(t *testing.T) {
	r := newRig()
	mux := server(r.register, r.reads, "usr_1")

	var out []notificationBody
	code := call(t, mux, http.MethodGet, "/v1/me/notifications?limit=5", "", &out)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
}

func TestMyNotificationsIgnoresAMalformedLimit(t *testing.T) {
	r := newRig()
	mux := server(r.register, r.reads, "usr_1")

	var out []notificationBody
	code := call(t, mux, http.MethodGet, "/v1/me/notifications?limit=not-a-number", "", &out)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
}

func TestMyNotificationsRejectsAnUnauthenticatedCaller(t *testing.T) {
	r := newRig()
	mux := server(r.register, r.reads, "")

	code := call(t, mux, http.MethodGet, "/v1/me/notifications", "", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", code)
	}
}

func TestMyNotificationsSurfacesAStorageFailure(t *testing.T) {
	r := newRig()
	r.repo.listErr = errBoom
	mux := server(r.register, r.reads, "usr_1")

	code := call(t, mux, http.MethodGet, "/v1/me/notifications", "", nil)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", code)
	}
}
