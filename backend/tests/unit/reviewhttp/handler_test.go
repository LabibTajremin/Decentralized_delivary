package reviewhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ordercontract "github.com/rootlogic-lab/delivery/backend/internal/modules/order/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/domain"
	orderx "github.com/rootlogic-lab/delivery/backend/internal/modules/review/external/order"
	reviewhttp "github.com/rootlogic-lab/delivery/backend/internal/modules/review/transport/http"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

var errBoom = errors.New("boom")

// memoryRepo is review's storage, in memory — one type implementing both
// repository interfaces, the way the real Postgres repository does.
type memoryRepo struct {
	reviews []domain.Review
	tickets []domain.Ticket
	loadErr error
}

func (m *memoryRepo) SaveReview(_ context.Context, r domain.Review) error {
	if m.loadErr != nil {
		return m.loadErr
	}
	m.reviews = append(m.reviews, r)
	return nil
}

func (m *memoryRepo) ExistsForRater(_ context.Context, orderID, raterID string, subject domain.Subject, subjectID string) (bool, error) {
	for _, r := range m.reviews {
		if r.OrderID == orderID && r.RaterID == raterID && r.Subject == subject && r.SubjectID == subjectID {
			return true, nil
		}
	}
	return false, nil
}

func (m *memoryRepo) RatingFor(_ context.Context, subject domain.Subject, subjectID string) (domain.Rating, error) {
	if m.loadErr != nil {
		return domain.Rating{}, m.loadErr
	}
	rating := domain.Rating{Subject: subject, SubjectID: subjectID}
	var total int
	for _, r := range m.reviews {
		if r.Subject == subject && r.SubjectID == subjectID {
			total += r.Rating
			rating.Count++
		}
	}
	if rating.Count > 0 {
		rating.Average = float64(total) / float64(rating.Count)
	}
	return rating, nil
}

func (m *memoryRepo) ReviewsFor(_ context.Context, subject domain.Subject, subjectID string, limit int) ([]domain.Review, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	var out []domain.Review
	for i := len(m.reviews) - 1; i >= 0; i-- {
		r := m.reviews[i]
		if r.Subject == subject && r.SubjectID == subjectID {
			out = append(out, r)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (m *memoryRepo) SaveTicket(_ context.Context, t domain.Ticket) error {
	for i, existing := range m.tickets {
		if existing.ID == t.ID {
			m.tickets[i] = t
			return nil
		}
	}
	m.tickets = append(m.tickets, t)
	return nil
}

func (m *memoryRepo) Ticket(_ context.Context, id string) (domain.Ticket, bool, error) {
	if m.loadErr != nil {
		return domain.Ticket{}, false, m.loadErr
	}
	for _, t := range m.tickets {
		if t.ID == id {
			return t, true, nil
		}
	}
	return domain.Ticket{}, false, nil
}

func (m *memoryRepo) TicketsRaisedBy(_ context.Context, userID string) ([]domain.Ticket, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	var out []domain.Ticket
	for _, t := range m.tickets {
		if t.RaisedBy == userID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (m *memoryRepo) OpenTickets(_ context.Context, limit int) ([]domain.Ticket, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	var out []domain.Ticket
	for _, t := range m.tickets {
		if t.Status == domain.TicketOpen {
			out = append(out, t)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

var _ ports.ReviewRepository = (*memoryRepo)(nil)
var _ ports.TicketRepository = (*memoryRepo)(nil)

// memoryOrder is review's view of order, in memory.
type memoryOrder struct {
	orders map[string]orderx.Order
}

func newMemoryOrder() *memoryOrder { return &memoryOrder{orders: map[string]orderx.Order{}} }

func (m *memoryOrder) Order(_ context.Context, orderID string) (orderx.Order, error) {
	o, ok := m.orders[orderID]
	if !ok {
		return orderx.Order{}, errBoom
	}
	return o, nil
}

// memoryPayment is review's view of payment, in memory.
type memoryPayment struct {
	err     error
	refunds []string
}

func (m *memoryPayment) Refund(_ context.Context, orderID, _ string) error {
	if m.err != nil {
		return m.err
	}
	m.refunds = append(m.refunds, orderID)
	return nil
}

type fakeIDs struct{ n int }

func (f *fakeIDs) New(prefix string) string { f.n++; return prefix + "_test" }

var _ id.Generator = (*fakeIDs)(nil)

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// rig wires every piece a handler test needs, mirroring how cmd/api wires
// the module in production.
type rig struct {
	repo    *memoryRepo
	order   *memoryOrder
	payment *memoryPayment
}

func newRig() *rig {
	return &rig{repo: &memoryRepo{}, order: newMemoryOrder(), payment: &memoryPayment{}}
}

func server(r *rig, userID string, admin bool) *http.ServeMux {
	mux := http.NewServeMux()
	open := func(next http.Handler) http.Handler { return next }
	adminGuard := open
	if !admin {
		adminGuard = func(http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			})
		}
	}
	ratings := application.NewRatingsUseCase(r.repo)
	reviewhttp.NewHandler(
		application.NewSubmitReviewUseCase(r.repo, r.order, realClock{}, &fakeIDs{}),
		ratings,
		application.NewListReviewsUseCase(r.repo),
		application.NewRaiseTicketUseCase(r.repo, r.order, realClock{}, &fakeIDs{}),
		application.NewResolveTicketUseCase(r.repo, r.payment, realClock{}),
		application.NewMyTicketsUseCase(r.repo),
		application.NewOpenTicketsUseCase(r.repo),
		open, adminGuard,
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

func deliveredOrder() orderx.Order {
	return orderx.Order{
		ID: "ord_1", CustomerID: "usr_1", MerchantID: "mer_1", Status: "delivered",
		Lines:  []ordercontract.Line{{Name: "Biryani", ItemID: "itm_1"}},
		Events: []ordercontract.Event{{Status: "delivered", Actor: "partner", ActorID: "prt_1"}},
	}
}

func TestNewHandlerPanicsWithoutAGuard(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewHandler did not panic with a nil guard")
		}
	}()
	reviewhttp.NewHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
}

func TestNewHandlerPanicsWithoutAnAdminGuard(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewHandler did not panic with a nil admin guard")
		}
	}()
	open := func(next http.Handler) http.Handler { return next }
	reviewhttp.NewHandler(nil, nil, nil, nil, nil, nil, nil, open, nil, nil)
}

func TestNewHandlerPanicsWithoutAPrincipalReader(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewHandler did not panic with a nil principal reader")
		}
	}()
	open := func(next http.Handler) http.Handler { return next }
	reviewhttp.NewHandler(nil, nil, nil, nil, nil, nil, nil, open, open, nil)
}

func TestSubmitReviewOverHTTP(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = deliveredOrder()
	mux := server(r, "usr_1", false)

	var out struct {
		ID      string `json:"id"`
		Subject string `json:"subject"`
	}
	code := call(t, mux, http.MethodPost, "/v1/reviews",
		`{"order_id":"ord_1","subject":"merchant","subject_id":"mer_1","rating":5}`, &out)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if out.Subject != "merchant" {
		t.Errorf("out = %+v", out)
	}
}

func TestSubmitReviewRejectsAnUnauthenticatedCaller(t *testing.T) {
	r := newRig()
	mux := server(r, "", false)
	code := call(t, mux, http.MethodPost, "/v1/reviews", `{}`, nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", code)
	}
}

func TestSubmitReviewRejectsAMalformedBody(t *testing.T) {
	r := newRig()
	mux := server(r, "usr_1", false)
	code := call(t, mux, http.MethodPost, "/v1/reviews", `not json`, nil)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", code)
	}
}

func TestListReviewsForOverHTTP(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = deliveredOrder()
	mux := server(r, "usr_1", false)
	call(t, mux, http.MethodPost, "/v1/reviews",
		`{"order_id":"ord_1","subject":"merchant","subject_id":"mer_1","rating":5}`, nil)

	var out struct {
		Reviews []struct {
			SubjectID string `json:"subject_id"`
		} `json:"reviews"`
	}
	code := call(t, mux, http.MethodGet, "/v1/reviews?subject=merchant&subject_id=mer_1", "", &out)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if len(out.Reviews) != 1 || out.Reviews[0].SubjectID != "mer_1" {
		t.Errorf("out = %+v", out)
	}
}

func TestListReviewsForRejectsAMissingSubject(t *testing.T) {
	r := newRig()
	mux := server(r, "usr_1", false)
	code := call(t, mux, http.MethodGet, "/v1/reviews", "", nil)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", code)
	}
}

func TestListReviewsForAcceptsALimit(t *testing.T) {
	r := newRig()
	mux := server(r, "usr_1", false)
	code := call(t, mux, http.MethodGet, "/v1/reviews?subject=merchant&subject_id=mer_1&limit=5", "", nil)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
}

func TestListReviewsForSurfacesAStorageFailure(t *testing.T) {
	r := newRig()
	r.repo.loadErr = errBoom
	mux := server(r, "usr_1", false)
	code := call(t, mux, http.MethodGet, "/v1/reviews?subject=merchant&subject_id=mer_1", "", nil)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", code)
	}
}

func TestListReviewsForIgnoresAMalformedLimit(t *testing.T) {
	r := newRig()
	mux := server(r, "usr_1", false)
	code := call(t, mux, http.MethodGet, "/v1/reviews?subject=merchant&subject_id=mer_1&limit=nope", "", nil)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
}

func TestRatingOverHTTP(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = deliveredOrder()
	mux := server(r, "usr_1", false)
	call(t, mux, http.MethodPost, "/v1/reviews",
		`{"order_id":"ord_1","subject":"merchant","subject_id":"mer_1","rating":4}`, nil)

	var out struct {
		Average float64 `json:"average"`
		Count   int     `json:"count"`
	}
	code := call(t, mux, http.MethodGet, "/v1/ratings?subject=merchant&subject_id=mer_1", "", &out)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if out.Average != 4 || out.Count != 1 {
		t.Errorf("out = %+v", out)
	}
}

func TestRatingRejectsAMissingSubject(t *testing.T) {
	r := newRig()
	mux := server(r, "usr_1", false)
	code := call(t, mux, http.MethodGet, "/v1/ratings?subject=merchant", "", nil)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", code)
	}
}

func TestRatingSurfacesAStorageFailure(t *testing.T) {
	r := newRig()
	r.repo.loadErr = errBoom
	mux := server(r, "usr_1", false)
	code := call(t, mux, http.MethodGet, "/v1/ratings?subject=merchant&subject_id=mer_1", "", nil)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", code)
	}
}

func TestRaiseTicketOverHTTP(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = deliveredOrder()
	mux := server(r, "usr_1", false)

	var out struct {
		Status string `json:"status"`
	}
	code := call(t, mux, http.MethodPost, "/v1/support/tickets",
		`{"order_id":"ord_1","subject":"item damaged"}`, &out)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if out.Status != "open" {
		t.Errorf("out = %+v", out)
	}
}

func TestRaiseTicketRejectsAnUnauthenticatedCaller(t *testing.T) {
	r := newRig()
	mux := server(r, "", false)
	code := call(t, mux, http.MethodPost, "/v1/support/tickets", `{}`, nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", code)
	}
}

func TestRaiseTicketRejectsAMalformedBody(t *testing.T) {
	r := newRig()
	mux := server(r, "usr_1", false)
	code := call(t, mux, http.MethodPost, "/v1/support/tickets", `not json`, nil)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", code)
	}
}

func TestMyTicketsOverHTTP(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = deliveredOrder()
	mux := server(r, "usr_1", false)
	call(t, mux, http.MethodPost, "/v1/support/tickets", `{"order_id":"ord_1","subject":"damaged"}`, nil)

	var out struct {
		Tickets []struct {
			OrderID string `json:"order_id"`
		} `json:"tickets"`
	}
	code := call(t, mux, http.MethodGet, "/v1/me/support/tickets", "", &out)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if len(out.Tickets) != 1 || out.Tickets[0].OrderID != "ord_1" {
		t.Errorf("out = %+v", out)
	}
}

func TestMyTicketsRejectsAnUnauthenticatedCaller(t *testing.T) {
	r := newRig()
	mux := server(r, "", false)
	code := call(t, mux, http.MethodGet, "/v1/me/support/tickets", "", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", code)
	}
}

func TestMyTicketsSurfacesAStorageFailure(t *testing.T) {
	r := newRig()
	r.repo.loadErr = errBoom
	mux := server(r, "usr_1", false)
	code := call(t, mux, http.MethodGet, "/v1/me/support/tickets", "", nil)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", code)
	}
}

func TestOpenTicketsOverHTTP(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = deliveredOrder()
	mux := server(r, "usr_1", true)
	call(t, mux, http.MethodPost, "/v1/support/tickets", `{"order_id":"ord_1","subject":"damaged"}`, nil)

	var out struct {
		Tickets []struct {
			ID string `json:"id"`
		} `json:"tickets"`
	}
	code := call(t, mux, http.MethodGet, "/v1/admin/support/tickets", "", &out)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if len(out.Tickets) != 1 {
		t.Errorf("out = %+v", out)
	}
}

func TestOpenTicketsAcceptsALimit(t *testing.T) {
	r := newRig()
	mux := server(r, "adm_1", true)
	code := call(t, mux, http.MethodGet, "/v1/admin/support/tickets?limit=5", "", nil)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
}

func TestOpenTicketsIgnoresAMalformedLimit(t *testing.T) {
	r := newRig()
	mux := server(r, "adm_1", true)
	code := call(t, mux, http.MethodGet, "/v1/admin/support/tickets?limit=nope", "", nil)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
}

func TestOpenTicketsSurfacesAStorageFailure(t *testing.T) {
	r := newRig()
	r.repo.loadErr = errBoom
	mux := server(r, "adm_1", true)
	code := call(t, mux, http.MethodGet, "/v1/admin/support/tickets", "", nil)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", code)
	}
}

func TestOpenTicketsRejectsANonAdmin(t *testing.T) {
	r := newRig()
	mux := server(r, "usr_1", false)
	code := call(t, mux, http.MethodGet, "/v1/admin/support/tickets", "", nil)
	if code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", code)
	}
}

func TestResolveTicketOverHTTP(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = deliveredOrder()
	mux := server(r, "usr_1", true)

	var raised struct {
		ID string `json:"id"`
	}
	call(t, mux, http.MethodPost, "/v1/support/tickets", `{"order_id":"ord_1","subject":"damaged"}`, &raised)

	var out struct {
		Status     string `json:"status"`
		Resolution string `json:"resolution"`
	}
	code := call(t, mux, http.MethodPost, "/v1/admin/support/tickets/resolve",
		`{"ticket_id":"`+raised.ID+`","resolution":"refunded","note":"policy"}`, &out)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if out.Status != "resolved" || out.Resolution != "refunded" {
		t.Errorf("out = %+v", out)
	}
	if len(r.payment.refunds) != 1 || r.payment.refunds[0] != "ord_1" {
		t.Errorf("refunds = %+v", r.payment.refunds)
	}
}

func TestResolveTicketRejectsAnUnauthenticatedCaller(t *testing.T) {
	r := newRig()
	mux := server(r, "", true)
	code := call(t, mux, http.MethodPost, "/v1/admin/support/tickets/resolve", `{}`, nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", code)
	}
}

func TestResolveTicketRejectsANonAdmin(t *testing.T) {
	r := newRig()
	mux := server(r, "usr_1", false)
	code := call(t, mux, http.MethodPost, "/v1/admin/support/tickets/resolve", `{}`, nil)
	if code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", code)
	}
}

func TestResolveTicketRejectsAMalformedBody(t *testing.T) {
	r := newRig()
	mux := server(r, "adm_1", true)
	code := call(t, mux, http.MethodPost, "/v1/admin/support/tickets/resolve", `not json`, nil)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", code)
	}
}

func TestResolveTicketSurfacesARefundFailure(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = deliveredOrder()
	r.payment.err = errBoom
	mux := server(r, "usr_1", true)

	var raised struct {
		ID string `json:"id"`
	}
	call(t, mux, http.MethodPost, "/v1/support/tickets", `{"order_id":"ord_1","subject":"damaged"}`, &raised)

	code := call(t, mux, http.MethodPost, "/v1/admin/support/tickets/resolve",
		`{"ticket_id":"`+raised.ID+`","resolution":"refunded"}`, nil)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", code)
	}
}

func TestResolveTicketRejectsAnAlreadyResolvedTicket(t *testing.T) {
	r := newRig()
	r.order.orders["ord_1"] = deliveredOrder()
	mux := server(r, "usr_1", true)

	var raised struct {
		ID string `json:"id"`
	}
	call(t, mux, http.MethodPost, "/v1/support/tickets", `{"order_id":"ord_1","subject":"damaged"}`, &raised)
	call(t, mux, http.MethodPost, "/v1/admin/support/tickets/resolve",
		`{"ticket_id":"`+raised.ID+`","resolution":"rejected"}`, nil)

	code := call(t, mux, http.MethodPost, "/v1/admin/support/tickets/resolve",
		`{"ticket_id":"`+raised.ID+`","resolution":"refunded"}`, nil)
	if code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", code)
	}
}

func TestResolveTicketSurfacesANotFoundTicket(t *testing.T) {
	r := newRig()
	mux := server(r, "adm_1", true)
	code := call(t, mux, http.MethodPost, "/v1/admin/support/tickets/resolve",
		`{"ticket_id":"tkt_missing","resolution":"rejected"}`, nil)
	if code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", code)
	}
}

// TestEveryReviewRouteIsBehindAGuard: every route in this module passes
// through either the authenticated or the admin guard before its handler
// runs — a route that slipped past either would let an anonymous caller
// submit reviews as anyone, or read the support queue.
func TestEveryReviewRouteIsBehindAGuard(t *testing.T) {
	var guarded []string
	counting := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			guarded = append(guarded, r.Method+" "+r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}
	repo := &memoryRepo{}
	ord := newMemoryOrder()
	pay := &memoryPayment{}
	mux := http.NewServeMux()
	reviewhttp.NewHandler(
		application.NewSubmitReviewUseCase(repo, ord, realClock{}, &fakeIDs{}),
		application.NewRatingsUseCase(repo),
		application.NewListReviewsUseCase(repo),
		application.NewRaiseTicketUseCase(repo, ord, realClock{}, &fakeIDs{}),
		application.NewResolveTicketUseCase(repo, pay, realClock{}),
		application.NewMyTicketsUseCase(repo),
		application.NewOpenTicketsUseCase(repo),
		counting, counting,
		func(*http.Request) (string, bool) { return "usr_1", true },
	).Register(mux)

	for _, pattern := range reviewhttp.Patterns() {
		guarded = nil
		method, path, _ := splitPattern(pattern)
		call(t, mux, method, path+queryFor(path), "{}", nil)
		if len(guarded) == 0 {
			t.Errorf("%s reached its handler without passing through a guard", pattern)
		}
	}
}

func splitPattern(pattern string) (method, path string, ok bool) {
	for i, c := range pattern {
		if c == ' ' {
			return pattern[:i], pattern[i+1:], true
		}
	}
	return "", pattern, false
}

// queryFor supplies the required query parameters a GET route needs so the
// guard test reaches the handler rather than a 400 from a missing subject.
func queryFor(path string) string {
	switch path {
	case "/v1/reviews", "/v1/ratings":
		return "?subject=merchant&subject_id=mer_1"
	default:
		return ""
	}
}

func TestPatternsMatchWhatIsMounted(t *testing.T) {
	want := map[string]bool{
		"POST /v1/reviews": true, "GET /v1/reviews": true, "GET /v1/ratings": true,
		"POST /v1/support/tickets": true, "GET /v1/me/support/tickets": true,
		"GET /v1/admin/support/tickets": true, "POST /v1/admin/support/tickets/resolve": true,
	}
	got := reviewhttp.Patterns()
	if len(got) != len(want) {
		t.Fatalf("Patterns() = %v", got)
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("unexpected pattern %q", p)
		}
	}
}
