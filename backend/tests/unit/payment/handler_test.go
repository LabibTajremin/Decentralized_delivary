package payment

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/infrastructure/gateway/manual"
	paymenthttp "github.com/rootlogic-lab/delivery/backend/internal/modules/payment/transport/http"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
)

// realRig is one assembled module wired with the real manual gateway, so a
// webhook test can sign a payload the way the running binary actually would.
type realRig struct {
	repo    *fakeRepo
	order   *fakeOrder
	gateway *manual.Gateway
	rig
}

func newRealRig(t *testing.T) *realRig {
	t.Helper()
	gw, err := manual.New("test-secret", false)
	if err != nil {
		t.Fatalf("manual.New: %v", err)
	}
	repo := newRepo()
	order := newOrder()
	dispatch := newDispatch()
	clk := clock.NewFixed(at)
	ids := &fakeIDs{}

	collections := application.NewCollectionUseCase(repo, dispatch, clk, ids)
	refunds := application.NewRefundUseCase(repo, gw, clk)
	return &realRig{
		repo: repo, order: order, gateway: gw,
		rig: rig{
			repo: repo, order: order, dispatch: dispatch, clock: clk,
			checkout:    application.NewCheckoutUseCase(repo, gw, order, clk, ids),
			webhook:     application.NewWebhookUseCase(repo, gw, order, clk),
			collections: collections,
			refunds:     refunds,
			reads:       application.NewReadUseCase(repo),
			service:     application.NewService(repo, collections, refunds),
		},
	}
}

// server mounts the handler over a rig. The guards pass everything through —
// what is under test is what the handler does with a caller, not the
// middleware that identifies one.
func server(checkout *application.CheckoutUseCase, webhook *application.WebhookUseCase,
	collections *application.CollectionUseCase, refunds *application.RefundUseCase,
	reads *application.ReadUseCase, userID string, devTools bool,
) *http.ServeMux {
	mux := http.NewServeMux()
	open := func(next http.Handler) http.Handler { return next }
	paymenthttp.NewHandler(
		checkout, webhook, collections, refunds, reads,
		open, open,
		func(*http.Request) (string, bool) { return userID, userID != "" },
		devTools,
	).Register(mux)
	return mux
}

func call(t *testing.T, mux *http.ServeMux, method, target, body string, headers map[string]string, into any) int {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
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

type moneyResponse struct {
	Minor    int64  `json:"minor"`
	Currency string `json:"currency"`
	Display  string `json:"display"`
}

type checkoutResponse struct {
	PaymentID   string        `json:"payment_id"`
	OrderID     string        `json:"order_id"`
	Status      string        `json:"status"`
	StatusLabel string        `json:"status_label"`
	Amount      moneyResponse `json:"amount"`
	RedirectURL string        `json:"redirect_url"`
}

type paymentResponse struct {
	ID          string        `json:"id"`
	OrderID     string        `json:"order_id"`
	Status      string        `json:"status"`
	StatusLabel string        `json:"status_label"`
	Amount      moneyResponse `json:"amount"`
	Reason      string        `json:"reason"`
}

type collectionResponse struct {
	ID            string        `json:"id"`
	OrderID       string        `json:"order_id"`
	Amount        moneyResponse `json:"amount"`
	Status        string        `json:"status"`
	StatusLabel   string        `json:"status_label"`
	RemittanceRef string        `json:"remittance_ref"`
}

type ledgerResponse struct {
	PartnerID   string               `json:"partner_id"`
	Outstanding moneyResponse        `json:"outstanding"`
	Remitted    moneyResponse        `json:"remitted"`
	Held        []collectionResponse `json:"held"`
}

func TestCheckoutOverHTTP(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = pendingOrder("ord_1", "usr_1", 50000)
	mux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "usr_1", false)

	var body checkoutResponse
	if status := call(t, mux, http.MethodPost, "/v1/payments/checkout", `{"order_id":"ord_1"}`, nil, &body); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if body.Status != "pending" || body.Amount.Minor != 50000 || body.Amount.Currency != "BDT" {
		t.Fatalf("body = %+v", body)
	}
}

func TestMyPaymentOverHTTP(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = pendingOrder("ord_1", "usr_1", 50000)
	mux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "usr_1", false)

	if status := call(t, mux, http.MethodPost, "/v1/payments/checkout", `{"order_id":"ord_1"}`, nil, nil); status != http.StatusOK {
		t.Fatalf("checkout: status = %d", status)
	}
	var body paymentResponse
	if status := call(t, mux, http.MethodGet, "/v1/payments/ord_1", "", nil, &body); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if body.OrderID != "ord_1" || body.Status != "pending" {
		t.Fatalf("body = %+v", body)
	}

	// A stranger reaching for it gets a 404, not a 403 that would confirm the
	// order exists.
	strangerMux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "usr_stranger", false)
	if status := call(t, strangerMux, http.MethodGet, "/v1/payments/ord_1", "", nil, nil); status != http.StatusNotFound {
		t.Fatalf("a stranger's read returned %d, want 404", status)
	}
}

func TestAdminPaymentAndRefundOverHTTP(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = pendingOrder("ord_1", "usr_1", 50000)
	customerMux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "usr_1", false)
	adminMux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "usr_admin", false)

	var checkout checkoutResponse
	if status := call(t, customerMux, http.MethodPost, "/v1/payments/checkout", `{"order_id":"ord_1"}`, nil, &checkout); status != http.StatusOK {
		t.Fatalf("checkout: status = %d", status)
	}

	var seen paymentResponse
	if status := call(t, adminMux, http.MethodGet, "/v1/admin/payments/ord_1", "", nil, &seen); status != http.StatusOK {
		t.Fatalf("admin read: status = %d", status)
	}
	if seen.OrderID != "ord_1" {
		t.Fatalf("body = %+v", seen)
	}

	// A refund needs a reason.
	if status := call(t, adminMux, http.MethodPost, "/v1/admin/payments/ord_1/refund", `{"reason":""}`, nil, nil); status != http.StatusBadRequest {
		t.Fatalf("a reasonless refund returned %d, want 400", status)
	}
	// And it must be captured first.
	if status := call(t, adminMux, http.MethodPost, "/v1/admin/payments/ord_1/refund", `{"reason":"complaint"}`, nil, nil); status != http.StatusConflict {
		t.Fatalf("refunding a pending payment returned %d, want 409", status)
	}
}

func TestLedgerRoutesOverHTTP(t *testing.T) {
	r := newRig()
	r.dispatch.partners["usr_rider"] = "PTR-1"
	seedHeld(t, r, "COL-1", "ord_1", "PTR-1", 30000)
	partnerMux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "usr_rider", false)
	adminMux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "usr_admin", false)

	var mine ledgerResponse
	if status := call(t, partnerMux, http.MethodGet, "/v1/partner/cod", "", nil, &mine); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if mine.PartnerID != "PTR-1" || mine.Outstanding.Minor != 30000 || len(mine.Held) != 1 {
		t.Fatalf("body = %+v", mine)
	}

	var admin ledgerResponse
	if status := call(t, adminMux, http.MethodGet, "/v1/admin/payments/cod/PTR-1", "", nil, &admin); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if admin.Outstanding.Minor != 30000 {
		t.Fatalf("body = %+v", admin)
	}

	reconcileBody := `{"collection_ids":["COL-1"],"reference":"DEPOSIT-1"}`
	var reconciled ledgerResponse
	if status := call(t, adminMux, http.MethodPost, "/v1/admin/payments/cod/PTR-1/reconcile", reconcileBody, nil, &reconciled); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if reconciled.Outstanding.Minor != 0 || reconciled.Remitted.Minor != 30000 {
		t.Fatalf("body = %+v", reconciled)
	}

	// A customer, not a partner, has no ledger.
	strangerMux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "usr_customer", false)
	if status := call(t, strangerMux, http.MethodGet, "/v1/partner/cod", "", nil, nil); status != http.StatusNotFound {
		t.Fatalf("a non-partner's ledger returned %d, want 404", status)
	}
}

// The webhook is the one route no signed-in caller reaches — it is
// authenticated by its own signature.
func TestWebhookOverHTTP(t *testing.T) {
	rr := newRealRig(t)
	rr.order.orders["ord_1"] = pendingOrder("ord_1", "usr_1", 50000)
	mux := server(rr.checkout, rr.webhook, rr.collections, rr.refunds, rr.reads, "usr_1", false)

	var checkout checkoutResponse
	if status := call(t, mux, http.MethodPost, "/v1/payments/checkout", `{"order_id":"ord_1"}`, nil, &checkout); status != http.StatusOK {
		t.Fatalf("checkout: status = %d", status)
	}

	body, err := json.Marshal(map[string]any{
		"reference": checkout.PaymentID, "gateway_ref": "gw_txn_1",
		"succeeded": true, "amount_minor": 50000,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	signature := manual.Sign("test-secret", body)

	status := call(t, mux, http.MethodPost, "/v1/payments/manual/webhook", string(body),
		map[string]string{"X-Webhook-Signature": signature}, nil)
	if status != http.StatusNoContent {
		t.Fatalf("status = %d", status)
	}

	var got paymentResponse
	if status := call(t, mux, http.MethodGet, "/v1/payments/ord_1", "", nil, &got); status != http.StatusOK || got.Status != "captured" {
		t.Fatalf("status = %d, body = %+v", status, got)
	}

	// The same route with no signature at all is refused, whoever asks — it
	// takes no Authorization header either way.
	badStatus := call(t, mux, http.MethodPost, "/v1/payments/manual/webhook", string(body), nil, nil)
	if badStatus != http.StatusBadRequest {
		t.Fatalf("an unsigned webhook returned %d, want 400", badStatus)
	}
}

func TestTheDevCompletionRouteOnlyExistsInDevTools(t *testing.T) {
	rr := newRealRig(t)
	rr.order.orders["ord_1"] = pendingOrder("ord_1", "usr_1", 50000)

	withDev := server(rr.checkout, rr.webhook, rr.collections, rr.refunds, rr.reads, "usr_1", true)
	var checkout checkoutResponse
	if status := call(t, withDev, http.MethodPost, "/v1/payments/checkout", `{"order_id":"ord_1"}`, nil, &checkout); status != http.StatusOK {
		t.Fatalf("checkout: status = %d", status)
	}
	completeBody := `{"payment_id":"` + checkout.PaymentID + `","succeeded":true}`
	if status := call(t, withDev, http.MethodPost, "/v1/payments/manual/complete", completeBody, nil, nil); status != http.StatusNoContent {
		t.Fatalf("status = %d", status)
	}

	withoutDev := server(rr.checkout, rr.webhook, rr.collections, rr.refunds, rr.reads, "usr_1", false)
	if status := call(t, withoutDev, http.MethodPost, "/v1/payments/manual/complete", completeBody, nil, nil); status != http.StatusNotFound {
		t.Fatalf("the dev route answered %d outside devTools, want 404", status)
	}
}

func TestEveryRouteRefusesAnAnonymousCaller(t *testing.T) {
	r := newRig()
	mux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "", false)
	for _, pattern := range paymenthttp.Patterns() {
		if pattern == "POST /v1/payments/manual/webhook" {
			continue // authenticated by signature, not by a caller
		}
		method, path, _ := strings.Cut(pattern, " ")
		if strings.Contains(path, "/admin/") {
			continue // the admin guard is identity's, not this handler's
		}
		target := path
		target = strings.ReplaceAll(target, "{orderId}", "X-1")
		target = strings.ReplaceAll(target, "{partnerId}", "X-1")
		if status := call(t, mux, method, target, "{}", nil, nil); status != http.StatusUnauthorized {
			t.Errorf("%s returned %d, want 401", pattern, status)
		}
	}
}

func TestMalformedRequestsAreRefused(t *testing.T) {
	r := newRig()
	mux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "usr_1", true)

	cases := []struct{ name, method, target, body string }{
		{"checkout", http.MethodPost, "/v1/payments/checkout", `{"order_id":`},
		{"refund", http.MethodPost, "/v1/admin/payments/ord_1/refund", `not json`},
		{"reconcile", http.MethodPost, "/v1/admin/payments/cod/PTR-1/reconcile", `{`},
		{"simulate", http.MethodPost, "/v1/payments/manual/complete", `not json`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if status := call(t, mux, tc.method, tc.target, tc.body, nil, nil); status != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", status)
			}
		})
	}
}

// Every admin read and write surfaces a downstream failure rather than a
// panic or an empty 200 — checked once per handler that has its own storage
// call beyond the ones TestDownstreamFailuresReachTheClient already covers.
func TestMoreDownstreamFailuresReachTheClient(t *testing.T) {
	t.Run("admin payment read", func(t *testing.T) {
		r := newRig()
		r.repo.latestForOrderErr = errBoom
		mux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "usr_1", false)
		if status := call(t, mux, http.MethodGet, "/v1/admin/payments/ord_1", "", nil, nil); status != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", status)
		}
	})

	t.Run("admin ledger read", func(t *testing.T) {
		r := newRig()
		r.repo.forPartnerErr = errBoom
		mux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "usr_1", false)
		if status := call(t, mux, http.MethodGet, "/v1/admin/payments/cod/PTR-1", "", nil, nil); status != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", status)
		}
	})

	t.Run("reconcile", func(t *testing.T) {
		r := newRig()
		r.repo.remitErr = errBoom
		mux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "usr_1", false)
		body := `{"collection_ids":["col_1"],"reference":"ref_1"}`
		if status := call(t, mux, http.MethodPost, "/v1/admin/payments/cod/PTR-1/reconcile", body, nil, nil); status != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", status)
		}
	})

	t.Run("simulate on an unknown payment", func(t *testing.T) {
		r := newRig()
		mux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "usr_1", true)
		body := `{"payment_id":"PAY-MISSING","succeeded":true}`
		if status := call(t, mux, http.MethodPost, "/v1/payments/manual/complete", body, nil, nil); status != http.StatusNotFound {
			t.Errorf("status = %d, want 404", status)
		}
	})

	// The refund itself succeeds, and only the read-back that composes the
	// response afterwards fails — proven with a call-counted failure, since
	// both calls go through the same repository method a millisecond apart.
	t.Run("refund succeeds but the read back fails", func(t *testing.T) {
		r, _ := capturedPayment(t, 50000)
		r.repo.latestForOrderErr = errBoom
		r.repo.latestForOrderErrAfter = 1
		mux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "usr_1", false)
		if status := call(t, mux, http.MethodPost, "/v1/admin/payments/ord_1/refund", `{"reason":"x"}`, nil, nil); status != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", status)
		}
	})
}

// The full, successful shape of a refund over HTTP: a captured payment goes
// to refunded and the response reflects it, not just the intermediate
// failure paths above.
func TestRefundOverHTTP(t *testing.T) {
	r, _ := capturedPayment(t, 50000)
	mux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "usr_1", false)

	var got paymentResponse
	status := call(t, mux, http.MethodPost, "/v1/admin/payments/ord_1/refund", `{"reason":"a customer complaint"}`, nil, &got)
	if status != http.StatusOK || got.Status != "refunded" {
		t.Fatalf("status = %d, body = %+v", status, got)
	}
}

// A checkout's own use case can fail after decoding cleanly — reached over
// HTTP, not just at the application layer.
func TestCheckoutUseCaseFailureOverHTTP(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = pendingOrder("ord_1", "usr_1", 50000)
	r.repo.pendingForOrderErr = errBoom
	mux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "usr_1", false)

	if status := call(t, mux, http.MethodPost, "/v1/payments/checkout", `{"order_id":"ord_1"}`, nil, nil); status != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", status)
	}
}

// A webhook body over the cap is refused before anything tries to verify a
// signature over it — the same defense every other JSON body on this
// service gets, applied by hand here because a webhook's signature must be
// checked over the exact bytes sent, which strict JSON decoding does not
// preserve.
func TestAnOversizedWebhookBodyIsRefused(t *testing.T) {
	r := newRig()
	mux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "usr_1", false)

	huge := strings.Repeat("a", (1<<20)+1)
	if status := call(t, mux, http.MethodPost, "/v1/payments/manual/webhook", huge, nil, nil); status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", status)
	}
}

func TestDownstreamFailuresReachTheClient(t *testing.T) {
	r := newRig()
	r.repo.latestForOrderErr = errBoom
	mux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "usr_1", false)

	if status := call(t, mux, http.MethodGet, "/v1/payments/ord_1", "", nil, nil); status != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", status)
	}
	if status := call(t, mux, http.MethodPost, "/v1/admin/payments/ord_1/refund", `{"reason":"x"}`, nil, nil); status != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", status)
	}
}

func TestNewHandlerRefusesToRunWithoutItsGuards(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a handler with no guards was built")
		}
	}()
	paymenthttp.NewHandler(nil, nil, nil, nil, nil, nil, nil, nil, false)
}

func TestPatternsMatchWhatIsMounted(t *testing.T) {
	patterns := paymenthttp.Patterns()
	if len(patterns) != 8 {
		t.Fatalf("Patterns() = %v", patterns)
	}
	sorted := append([]string(nil), patterns...)
	sort.Strings(sorted)
	for i := range patterns {
		if patterns[i] != sorted[i] {
			t.Fatalf("Patterns() is not sorted: %v", patterns)
		}
	}

	r := newRig()
	mux := server(r.checkout, r.webhook, r.collections, r.refunds, r.reads, "usr_1", false)
	for _, pattern := range patterns {
		method, path, ok := strings.Cut(pattern, " ")
		if !ok {
			t.Fatalf("pattern %q is not %q", pattern, "METHOD /path")
		}
		target := path
		target = strings.ReplaceAll(target, "{orderId}", "X-1")
		target = strings.ReplaceAll(target, "{partnerId}", "X-1")
		_, matched := mux.Handler(httptest.NewRequest(method, target, bytes.NewBufferString("{}")))
		if matched != pattern {
			t.Errorf("%s is in Patterns() but the mux resolved %q", pattern, matched)
		}
	}
}
