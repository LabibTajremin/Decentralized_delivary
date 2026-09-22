package order

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	orderhttp "github.com/rootlogic-lab/delivery/backend/internal/modules/order/transport/http"
)

// server mounts the handler over a rig. The guards pass everything through —
// what is under test here is what the handler does with a caller, not the
// middleware that identifies one.
func server(r *rig, userID string, owned []string) *http.ServeMux {
	mux := http.NewServeMux()
	open := func(next http.Handler) http.Handler { return next }
	orderhttp.NewHandler(
		r.place, r.transitions, r.reads,
		open, open,
		func(*http.Request) (string, bool) { return userID, userID != "" },
		func(*http.Request, string) ([]string, error) { return owned, nil },
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

type orderResponse struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Status      string `json:"status"`
	StatusLabel string `json:"status_label"`
	Live        bool   `json:"live"`
	Count       int    `json:"count"`
	Total       struct {
		Minor   int64  `json:"minor"`
		Display string `json:"display"`
	} `json:"total"`
	Receipt []struct {
		Key   string `json:"key"`
		Label string `json:"label"`
	} `json:"receipt"`
	NextActions []string `json:"next_actions"`
	Cancel      struct {
		Allowed     bool   `json:"allowed"`
		Reason      string `json:"reason"`
		SecondsLeft int    `json:"seconds_left"`
	} `json:"cancel"`
	Events []struct {
		Status string `json:"status"`
		Label  string `json:"label"`
	} `json:"events"`
}

func TestPlacingAnOrderOverHTTP(t *testing.T) {
	r := newRig()
	mux := server(r, "USR-1", nil)

	var body orderResponse
	status := call(t, mux, http.MethodPost, "/v1/orders?lang=en",
		`{"address_id":"ADR-1","payment_method":"cash"}`, &body)
	if status != http.StatusCreated {
		t.Fatalf("status = %d", status)
	}
	if body.Status != "placed" || body.StatusLabel != "Sent to the shop" || !body.Live {
		t.Fatalf("body = %+v", body)
	}
	if body.Code == "" || body.Count != 2 || body.Total.Display == "" {
		t.Fatalf("body = %+v", body)
	}
	if len(body.Receipt) < 3 || body.Receipt[0].Label == "" {
		t.Errorf("receipt = %+v", body.Receipt)
	}
	if !body.Cancel.Allowed || body.Cancel.SecondsLeft != 120 {
		t.Errorf("cancel = %+v", body.Cancel)
	}
}

// The idempotency key is read from the header as well as the body, because a
// generated client may find one easier than the other.
func TestTheIdempotencyKeyIsAcceptedFromEitherPlace(t *testing.T) {
	r := newRig()
	mux := server(r, "USR-1", nil)

	post := func(header, inBody string) orderResponse {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/v1/orders",
			bytes.NewBufferString(`{"address_id":"ADR-1","payment_method":"cash","idempotency_key":"`+inBody+`"}`))
		req.Header.Set("Content-Type", "application/json")
		if header != "" {
			req.Header.Set("Idempotency-Key", header)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
		}
		var body orderResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return body
	}

	fromBody := post("", "body-key")
	againFromBody := post("", "body-key")
	if fromBody.ID != againFromBody.ID {
		t.Fatalf("the body key made two orders: %s, %s", fromBody.ID, againFromBody.ID)
	}

	fromHeader := post("header-key", "")
	againFromHeader := post("header-key", "ignored-because-the-header-wins")
	if fromHeader.ID != againFromHeader.ID {
		t.Fatalf("the header key made two orders: %s, %s", fromHeader.ID, againFromHeader.ID)
	}
	if fromHeader.ID == fromBody.ID {
		t.Error("two different keys returned one order")
	}
}

func TestTheCustomerRoutes(t *testing.T) {
	r := newRig()
	mux := server(r, "USR-1", nil)

	var placed orderResponse
	if status := call(t, mux, http.MethodPost, "/v1/orders",
		`{"address_id":"ADR-1","payment_method":"cash"}`, &placed); status != http.StatusCreated {
		t.Fatalf("place: status = %d", status)
	}

	var list struct {
		Orders []orderResponse `json:"orders"`
		Total  int             `json:"total"`
	}
	if status := call(t, mux, http.MethodGet, "/v1/orders?live=true&limit=5", "", &list); status != http.StatusOK {
		t.Fatalf("list: status = %d", status)
	}
	if list.Total != 1 || len(list.Orders) != 1 {
		t.Fatalf("list = %+v", list)
	}

	var one orderResponse
	if status := call(t, mux, http.MethodGet, "/v1/orders/"+placed.ID+"?lang=en", "", &one); status != http.StatusOK {
		t.Fatalf("read: status = %d", status)
	}
	if one.ID != placed.ID {
		t.Fatalf("read = %+v", one)
	}

	var cancellation struct {
		Allowed     bool `json:"allowed"`
		SecondsLeft int  `json:"seconds_left"`
	}
	if status := call(t, mux, http.MethodGet, "/v1/orders/"+placed.ID+"/cancellation", "", &cancellation); status != http.StatusOK {
		t.Fatalf("cancellation: status = %d", status)
	}
	if !cancellation.Allowed || cancellation.SecondsLeft != 120 {
		t.Fatalf("cancellation = %+v", cancellation)
	}

	var cancelled orderResponse
	if status := call(t, mux, http.MethodPost, "/v1/orders/"+placed.ID+"/cancel",
		`{"reason":"changed my mind"}`, &cancelled); status != http.StatusOK {
		t.Fatalf("cancel: status = %d", status)
	}
	if cancelled.Status != "cancelled" || cancelled.Live {
		t.Fatalf("cancelled = %+v", cancelled)
	}
}

// Cancelling with no body at all is the common case: the button sends nothing.
func TestCancelWithNoBody(t *testing.T) {
	r := newRig()
	mux := server(r, "USR-1", nil)
	var placed orderResponse
	call(t, mux, http.MethodPost, "/v1/orders", `{"address_id":"ADR-1","payment_method":"cash"}`, &placed)

	if status := call(t, mux, http.MethodPost, "/v1/orders/"+placed.ID+"/cancel", "", nil); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
}

func TestTheShopRoutes(t *testing.T) {
	r := newRig()
	customerMux := server(r, "USR-1", nil)
	var placed orderResponse
	call(t, customerMux, http.MethodPost, "/v1/orders", `{"address_id":"ADR-1","payment_method":"cash"}`, &placed)

	shopMux := server(r, "USR-OWNER", []string{"MER-1"})
	base := "/v1/merchants/MER-1/orders"

	var queue struct {
		Total int `json:"total"`
	}
	if status := call(t, shopMux, http.MethodGet, base+"?live=true", "", &queue); status != http.StatusOK {
		t.Fatalf("queue: status = %d", status)
	}
	if queue.Total != 1 {
		t.Fatalf("queue = %+v", queue)
	}

	var one orderResponse
	if status := call(t, shopMux, http.MethodGet, base+"/"+placed.ID, "", &one); status != http.StatusOK {
		t.Fatalf("read: status = %d", status)
	}

	// A rejection with no reason is refused; the customer would have nothing
	// to go on.
	if status := call(t, shopMux, http.MethodPost, base+"/"+placed.ID+"/reject", `{}`, nil); status != http.StatusBadRequest {
		t.Fatalf("reject with no reason: status = %d", status)
	}

	var accepted orderResponse
	if status := call(t, shopMux, http.MethodPost, base+"/"+placed.ID+"/accept", "", &accepted); status != http.StatusOK {
		t.Fatalf("accept: status = %d", status)
	}
	if accepted.Status != "accepted" {
		t.Fatalf("accepted = %+v", accepted)
	}

	for _, step := range []struct{ path, want string }{
		{"/preparing", "preparing"},
		{"/ready", "ready"},
	} {
		var got orderResponse
		if status := call(t, shopMux, http.MethodPost, base+"/"+placed.ID+step.path, "", &got); status != http.StatusOK {
			t.Fatalf("%s: status = %d", step.path, status)
		}
		if got.Status != step.want {
			t.Fatalf("%s = %+v", step.path, got)
		}
	}

	// And the shop cannot go further: the rider owns the next step.
	if status := call(t, shopMux, http.MethodPost, base+"/"+placed.ID+"/accept", "", nil); status != http.StatusConflict {
		t.Fatalf("a second accept returned %d", status)
	}
}

// A shop reaching for another shop's queue gets a not-found, not a forbidden.
func TestAShopCannotReachAnotherShopsQueue(t *testing.T) {
	r := newRig()
	customerMux := server(r, "USR-1", nil)
	var placed orderResponse
	call(t, customerMux, http.MethodPost, "/v1/orders", `{"address_id":"ADR-1","payment_method":"cash"}`, &placed)

	otherMux := server(r, "USR-OTHER", []string{"MER-2"})
	for _, target := range []struct{ method, path string }{
		{http.MethodGet, "/v1/merchants/MER-1/orders"},
		{http.MethodGet, "/v1/merchants/MER-1/orders/" + placed.ID},
		{http.MethodPost, "/v1/merchants/MER-1/orders/" + placed.ID + "/accept"},
	} {
		if status := call(t, otherMux, target.method, target.path, "", nil); status != http.StatusNotFound {
			t.Errorf("%s %s returned %d, want 404", target.method, target.path, status)
		}
	}
}

func TestTheAdminRoute(t *testing.T) {
	r := newRig()
	customerMux := server(r, "USR-1", nil)
	var placed orderResponse
	call(t, customerMux, http.MethodPost, "/v1/orders", `{"address_id":"ADR-1","payment_method":"cash"}`, &placed)

	adminMux := server(r, "USR-ADMIN", nil)
	path := "/v1/admin/orders/" + placed.ID + "/transition"

	if status := call(t, adminMux, http.MethodPost, path, `{"status":"shipped"}`, nil); status != http.StatusBadRequest {
		t.Fatalf("an unknown status returned %d", status)
	}

	var cancelled orderResponse
	if status := call(t, adminMux, http.MethodPost, path,
		`{"status":"cancelled","reason":"customer phoned"}`, &cancelled); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if cancelled.Status != "cancelled" {
		t.Fatalf("cancelled = %+v", cancelled)
	}
	last := cancelled.Events[len(cancelled.Events)-1]
	if last.Status != "cancelled" {
		t.Errorf("history = %+v", cancelled.Events)
	}
}

func TestEveryRouteRefusesAnAnonymousCaller(t *testing.T) {
	mux := server(newRig(), "", nil)
	for _, pattern := range orderhttp.Patterns() {
		method, path, _ := strings.Cut(pattern, " ")
		target := strings.ReplaceAll(path, "{orderId}", "ORD-1")
		target = strings.ReplaceAll(target, "{merchantId}", "MER-1")
		if status := call(t, mux, method, target, "{}", nil); status != http.StatusUnauthorized {
			t.Errorf("%s returned %d, want 401", pattern, status)
		}
	}
}

func TestMalformedRequestsAreRefused(t *testing.T) {
	r := newRig()
	mux := server(r, "USR-1", []string{"MER-1"})
	cases := []struct {
		name, method, target, body string
	}{
		{"place", http.MethodPost, "/v1/orders", `{"address_id":`},
		{"cancel", http.MethodPost, "/v1/orders/ORD-1/cancel", `not json`},
		{"admin transition", http.MethodPost, "/v1/admin/orders/ORD-1/transition", `[]`},
		{"shop reject", http.MethodPost, "/v1/merchants/MER-1/orders/ORD-1/reject", `{`},
		{"a bad limit", http.MethodGet, "/v1/orders?limit=0", ""},
		{"a bad offset", http.MethodGet, "/v1/orders?offset=x", ""},
		{"a bad shop limit", http.MethodGet, "/v1/merchants/MER-1/orders?limit=0", ""},
		{"a bad shop offset", http.MethodGet, "/v1/merchants/MER-1/orders?offset=x", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if status := call(t, mux, tc.method, tc.target, tc.body, nil); status != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", status)
			}
		})
	}
}

func TestDownstreamFailuresReachTheClient(t *testing.T) {
	r := newRig()
	r.repo.readErr = errBoom
	mux := server(r, "USR-1", []string{"MER-1"})

	cases := []struct{ name, method, target string }{
		{"read", http.MethodGet, "/v1/orders/ORD-1"},
		{"cancellation", http.MethodGet, "/v1/orders/ORD-1/cancellation"},
		{"cancel", http.MethodPost, "/v1/orders/ORD-1/cancel"},
		{"shop read", http.MethodGet, "/v1/merchants/MER-1/orders/ORD-1"},
		{"shop accept", http.MethodPost, "/v1/merchants/MER-1/orders/ORD-1/accept"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if status := call(t, mux, tc.method, tc.target, "", nil); status != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want 503", status)
			}
		})
	}

	listing := newRig()
	listing.repo.listErr = errBoom
	listMux := server(listing, "USR-1", []string{"MER-1"})
	for _, target := range []string{"/v1/orders", "/v1/merchants/MER-1/orders"} {
		if status := call(t, listMux, http.MethodGet, target, "", nil); status != http.StatusServiceUnavailable {
			t.Errorf("%s returned %d, want 503", target, status)
		}
	}
}

// A merchant lookup that fails is an outage, not a missing shop.
func TestAFailedOwnershipLookupIsReported(t *testing.T) {
	r := newRig()
	mux := http.NewServeMux()
	open := func(next http.Handler) http.Handler { return next }
	orderhttp.NewHandler(
		r.place, r.transitions, r.reads, open, open,
		func(*http.Request) (string, bool) { return "USR-1", true },
		func(*http.Request, string) ([]string, error) { return nil, errBoom },
	).Register(mux)

	if status := call(t, mux, http.MethodGet, "/v1/merchants/MER-1/orders", "", nil); status != http.StatusInternalServerError {
		t.Errorf("status = %d", status)
	}
}

func TestNewHandlerRefusesToRunWithoutItsGuards(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a handler with no guards was built")
		}
	}()
	orderhttp.NewHandler(nil, nil, nil, nil, nil, nil, nil)
}

func TestPatternsMatchWhatIsMounted(t *testing.T) {
	patterns := orderhttp.Patterns()
	if len(patterns) != 12 {
		t.Fatalf("Patterns() = %v", patterns)
	}
	mux := server(newRig(), "USR-1", []string{"MER-1"})
	for _, pattern := range patterns {
		method, path, ok := strings.Cut(pattern, " ")
		if !ok {
			t.Fatalf("pattern %q is not %q", pattern, "METHOD /path")
		}
		target := strings.ReplaceAll(path, "{orderId}", "ORD-1")
		target = strings.ReplaceAll(target, "{merchantId}", "MER-1")
		_, matched := mux.Handler(httptest.NewRequest(method, target, bytes.NewBufferString("{}")))
		if matched != pattern {
			t.Errorf("%s is in Patterns() but the mux resolved %q", pattern, matched)
		}
	}
}
