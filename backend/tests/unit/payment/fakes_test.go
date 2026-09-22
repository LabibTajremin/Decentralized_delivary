package payment

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/domain"
	dispatchx "github.com/rootlogic-lab/delivery/backend/internal/modules/payment/external/dispatch"
	orderx "github.com/rootlogic-lab/delivery/backend/internal/modules/payment/external/order"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

var errBoom = errors.New("boom")

// fakeRepo is an in-memory payment store with the same guards the real one
// has: one open checkout per order, one collection per order, and a
// compare-and-set on every write that can race.
type fakeRepo struct {
	payments    map[string]domain.Payment
	collections map[string]domain.Collection

	createPaymentErr    error
	paymentErr          error
	byReferenceErr      error
	pendingForOrderErr  error
	latestForOrderErr   error
	savePaymentErr      error
	createCollectionErr error
	collectionErr       error
	forPartnerErr       error
	remitErr            error

	// latestForOrderErrAfter, when nonzero, delays latestForOrderErr until
	// the call count passes it — so a test can let one caller's read
	// succeed and the next one fail, the way two calls a millisecond apart
	// can find a healthy database and then a dropped connection.
	latestForOrderErrAfter int
	latestForOrderCalls    int

	// pendingMissOnce makes the next PendingForOrder miss, which is how the
	// loser of a checkout race sees the world: it checks, finds nothing, and
	// writes — and the unique index catches it instead.
	pendingMissOnce bool
}

func newRepo() *fakeRepo {
	return &fakeRepo{
		payments:    map[string]domain.Payment{},
		collections: map[string]domain.Collection{},
	}
}

func (r *fakeRepo) CreatePayment(_ context.Context, p domain.Payment) error {
	if r.createPaymentErr != nil {
		return r.createPaymentErr
	}
	for _, existing := range r.payments {
		if existing.OrderID == p.OrderID && existing.Status == domain.StatusPending {
			return errs.New(errs.KindConflict, "checkout_in_progress",
				"A checkout for that order is already in progress.")
		}
	}
	r.payments[p.ID] = p
	return nil
}

func (r *fakeRepo) Payment(_ context.Context, id string) (domain.Payment, error) {
	if r.paymentErr != nil {
		return domain.Payment{}, r.paymentErr
	}
	p, ok := r.payments[id]
	if !ok {
		return domain.Payment{}, errs.New(errs.KindNotFound, "payment_not_found", "No such payment.")
	}
	return p, nil
}

func (r *fakeRepo) ByReference(_ context.Context, reference string) (domain.Payment, error) {
	if r.byReferenceErr != nil {
		return domain.Payment{}, r.byReferenceErr
	}
	for _, p := range r.payments {
		if p.Reference == reference {
			return p, nil
		}
	}
	return domain.Payment{}, errs.New(errs.KindNotFound, "payment_not_found", "No such payment.")
}

func (r *fakeRepo) PendingForOrder(_ context.Context, orderID string) (domain.Payment, bool, error) {
	if r.pendingMissOnce {
		r.pendingMissOnce = false
		return domain.Payment{}, false, nil
	}
	if r.pendingForOrderErr != nil {
		return domain.Payment{}, false, r.pendingForOrderErr
	}
	for _, p := range r.payments {
		if p.OrderID == orderID && p.Status == domain.StatusPending {
			return p, true, nil
		}
	}
	return domain.Payment{}, false, nil
}

func (r *fakeRepo) LatestForOrder(_ context.Context, orderID string) (domain.Payment, bool, error) {
	r.latestForOrderCalls++
	if r.latestForOrderErr != nil && (r.latestForOrderErrAfter == 0 || r.latestForOrderCalls > r.latestForOrderErrAfter) {
		return domain.Payment{}, false, r.latestForOrderErr
	}
	var latest domain.Payment
	found := false
	for _, p := range r.payments {
		if p.OrderID != orderID {
			continue
		}
		if !found || p.CreatedAt.After(latest.CreatedAt) {
			latest, found = p, true
		}
	}
	return latest, found, nil
}

func (r *fakeRepo) Save(_ context.Context, p domain.Payment, expected domain.Status) error {
	if r.savePaymentErr != nil {
		return r.savePaymentErr
	}
	current, ok := r.payments[p.ID]
	if !ok || current.Status != expected {
		return errs.New(errs.KindConflict, "payment_moved", "That payment has already moved on.")
	}
	r.payments[p.ID] = p
	return nil
}

func (r *fakeRepo) CreateCollection(_ context.Context, c domain.Collection) error {
	if r.createCollectionErr != nil {
		return r.createCollectionErr
	}
	for _, existing := range r.collections {
		if existing.OrderID == c.OrderID {
			return errs.New(errs.KindConflict, "collection_exists",
				"That order's cash has already been recorded.")
		}
	}
	r.collections[c.ID] = c
	return nil
}

func (r *fakeRepo) Collection(_ context.Context, id string) (domain.Collection, error) {
	if r.collectionErr != nil {
		return domain.Collection{}, r.collectionErr
	}
	c, ok := r.collections[id]
	if !ok {
		return domain.Collection{}, errs.New(errs.KindNotFound, "collection_not_found", "No such collection.")
	}
	return c, nil
}

func (r *fakeRepo) ForPartner(_ context.Context, partnerID string) ([]domain.Collection, error) {
	if r.forPartnerErr != nil {
		return nil, r.forPartnerErr
	}
	var out []domain.Collection
	for _, c := range r.collections {
		if c.PartnerID == partnerID {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Status != out[j].Status {
			return out[i].Status == domain.StatusHeld
		}
		return out[i].CollectedAt.Before(out[j].CollectedAt)
	})
	return out, nil
}

// Remit mirrors the real repository's transaction: every id is validated
// before anything is written, so a batch either all moves or none of it does.
func (r *fakeRepo) Remit(_ context.Context, partnerID string, collectionIDs []string, reference string, now time.Time) error {
	if r.remitErr != nil {
		return r.remitErr
	}
	for _, id := range collectionIDs {
		c, ok := r.collections[id]
		if !ok || c.PartnerID != partnerID || c.Status != domain.StatusHeld {
			return errs.New(errs.KindConflict, "collection_not_remittable",
				"One of those collections is not yours to remit, or has already been remitted.").
				With("collection_id", id)
		}
	}
	for _, id := range collectionIDs {
		c := r.collections[id]
		c.Status = domain.StatusRemitted
		c.RemittanceRef = reference
		c.RemittedAt = now
		c.UpdatedAt = now
		r.collections[id] = c
	}
	return nil
}

// ------------------------------------------------------------------ gateway

// webhookPayload mirrors the manual gateway's own wire shape, so tests build
// realistic callbacks without reaching into that package.
type webhookPayload struct {
	Reference   string `json:"reference"`
	GatewayRef  string `json:"gateway_ref"`
	Succeeded   bool   `json:"succeeded"`
	Reason      string `json:"reason,omitempty"`
	AmountMinor int64  `json:"amount_minor"`
}

// fakeGateway is a scriptable ports.Gateway. VerifyWebhook actually decodes
// its payload rather than returning a canned event, so a test's payload is
// what the use case really sees — the same discipline as a real adapter.
type fakeGateway struct {
	name string

	checkoutErr    error
	checkoutResult ports.CheckoutResult
	checkoutCalls  []string

	validSignature string
	verifyErr      error

	refundErr    error
	refundResult ports.RefundResult
	refundCalls  []string
}

func newGateway() *fakeGateway {
	return &fakeGateway{name: "manual", validSignature: "valid-signature"}
}

func (g *fakeGateway) Name() string {
	if g.name == "" {
		return "manual"
	}
	return g.name
}

func (g *fakeGateway) Checkout(_ context.Context, reference string, _ int64, _ string) (ports.CheckoutResult, error) {
	g.checkoutCalls = append(g.checkoutCalls, reference)
	if g.checkoutErr != nil {
		return ports.CheckoutResult{}, g.checkoutErr
	}
	if g.checkoutResult.GatewayRef == "" {
		return ports.CheckoutResult{GatewayRef: reference}, nil
	}
	return g.checkoutResult, nil
}

func (g *fakeGateway) VerifyWebhook(_ context.Context, payload []byte, signature string) (ports.WebhookEvent, error) {
	if g.verifyErr != nil {
		return ports.WebhookEvent{}, g.verifyErr
	}
	if signature != g.validSignature {
		return ports.WebhookEvent{}, errors.New("bad signature")
	}
	var body webhookPayload
	if err := json.Unmarshal(payload, &body); err != nil {
		return ports.WebhookEvent{}, err
	}
	return ports.WebhookEvent{
		Reference: body.Reference, GatewayRef: body.GatewayRef,
		Succeeded: body.Succeeded, Reason: body.Reason, AmountMinor: body.AmountMinor,
	}, nil
}

func (g *fakeGateway) Refund(_ context.Context, gatewayRef string, _ int64) (ports.RefundResult, error) {
	g.refundCalls = append(g.refundCalls, gatewayRef)
	if g.refundErr != nil {
		return ports.RefundResult{}, g.refundErr
	}
	return ports.RefundResult{RefundRef: gatewayRef}, nil
}

// sign builds a payload+signature pair a test can hand to Handle.
func sign(secret string, body webhookPayload) ([]byte, string) {
	payload, _ := json.Marshal(body)
	return payload, secret
}

// ------------------------------------------------------------------ order

type fakeOrder struct {
	orders map[string]orderx.Order

	orderErr      error
	markPaidErr   error
	markFailedErr error

	paidCalls   []string
	failedCalls []struct{ orderID, reason string }
}

func newOrder() *fakeOrder {
	return &fakeOrder{orders: map[string]orderx.Order{}}
}

func (f *fakeOrder) Order(_ context.Context, orderID string) (orderx.Order, error) {
	if f.orderErr != nil {
		return orderx.Order{}, f.orderErr
	}
	o, ok := f.orders[orderID]
	if !ok {
		return orderx.Order{}, errs.New(errs.KindNotFound, "order_not_found", "We could not find that order.")
	}
	return o, nil
}

func (f *fakeOrder) MarkPaid(_ context.Context, orderID string) error {
	if f.markPaidErr != nil {
		return f.markPaidErr
	}
	f.paidCalls = append(f.paidCalls, orderID)
	return nil
}

func (f *fakeOrder) MarkPaymentFailed(_ context.Context, orderID, reason string) error {
	if f.markFailedErr != nil {
		return f.markFailedErr
	}
	f.failedCalls = append(f.failedCalls, struct{ orderID, reason string }{orderID, reason})
	return nil
}

// ------------------------------------------------------------------ dispatch

type fakeDispatch struct {
	partners map[string]string // userID -> partnerID
	err      error
}

func newDispatch() *fakeDispatch {
	return &fakeDispatch{partners: map[string]string{}}
}

func (f *fakeDispatch) PartnerOfUser(_ context.Context, userID string) (string, bool, error) {
	if f.err != nil {
		return "", false, f.err
	}
	id, ok := f.partners[userID]
	return id, ok, nil
}

var _ dispatchx.Service = (*fakeDispatch)(nil)
var _ orderx.Service = (*fakeOrder)(nil)

// fakeIDs hands out predictable, incrementing ids.
type fakeIDs struct{ n int }

func (g *fakeIDs) New(prefix string) string {
	g.n++
	return prefix + "-" + strconv.Itoa(g.n)
}
