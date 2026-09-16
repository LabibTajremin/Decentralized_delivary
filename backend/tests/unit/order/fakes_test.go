// Package order holds the unit tests for the order module.
//
// The two things this phase promises are a state machine that refuses illegal
// moves and an idempotency key that makes one order out of two identical
// requests. Both are pinned here, and again over a real database in the
// integration and E2E suites.
package order

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	cartcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/cart/contract"
	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
	discocontract "github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/contract"
	dispatchcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/contract"
	merchantcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/domain"
	usercontract "github.com/rootlogic-lab/delivery/backend/internal/modules/user/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

var errBoom = errors.New("boom")

// fakeRepo is an in-memory order store with the same concurrency guard the
// real one has: a transition is applied only if the order is still where the
// caller thought it was.
type fakeRepo struct {
	orders map[string]domain.Order
	keys   map[string]string

	createErr     error
	readErr       error
	listErr       error
	transitionErr error
	keyReadErr    error
	keyClaimErr   error

	created int
}

func newRepo() *fakeRepo {
	return &fakeRepo{orders: map[string]domain.Order{}, keys: map[string]string{}}
}

func (r *fakeRepo) Create(_ context.Context, o domain.Order) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created++
	r.orders[o.ID] = o
	return nil
}

func (r *fakeRepo) Order(_ context.Context, orderID string) (domain.Order, error) {
	if r.readErr != nil {
		return domain.Order{}, r.readErr
	}
	o, ok := r.orders[orderID]
	if !ok {
		return domain.Order{}, errs.New(errs.KindNotFound, "order_not_found", "No such order.")
	}
	return o, nil
}

func (r *fakeRepo) Orders(_ context.Context, f ports.Filter) ([]domain.Order, int, error) {
	if r.listErr != nil {
		return nil, 0, r.listErr
	}
	var out []domain.Order
	for _, o := range r.orders {
		switch {
		case f.CustomerID != "" && o.CustomerID != f.CustomerID:
			continue
		case f.MerchantID != "" && o.MerchantID != f.MerchantID:
			continue
		case f.CustomerID == "" && f.MerchantID == "":
			// The real repository answers an unscoped filter with a predicate
			// that matches nothing, rather than everybody's orders.
			continue
		case f.LiveOnly && !o.Status.IsLive():
			continue
		}
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	total := len(out)
	if f.Offset >= len(out) {
		return nil, total, nil
	}
	out = out[f.Offset:]
	if f.Limit > 0 && f.Limit < len(out) {
		out = out[:f.Limit]
	}
	return out, total, nil
}

func (r *fakeRepo) AppendTransition(_ context.Context, orderID string, from domain.Status, e domain.Event) error {
	if r.transitionErr != nil {
		return r.transitionErr
	}
	o, ok := r.orders[orderID]
	if !ok {
		return errs.New(errs.KindNotFound, "order_not_found", "No such order.")
	}
	if o.Status != from {
		return errs.New(errs.KindConflict, "order_moved", "That order has already changed.")
	}
	o.Status = e.Status
	o.UpdatedAt = e.At
	o.Events = append(o.Events, e)
	r.orders[orderID] = o
	return nil
}

func (r *fakeRepo) ByIdempotencyKey(ctx context.Context, customerID, key string) (domain.Order, bool, error) {
	if r.keyReadErr != nil {
		return domain.Order{}, false, r.keyReadErr
	}
	orderID, ok := r.keys[customerID+"/"+key]
	if !ok {
		return domain.Order{}, false, nil
	}
	o, err := r.Order(ctx, orderID)
	return o, err == nil, err
}

func (r *fakeRepo) ClaimIdempotencyKey(_ context.Context, customerID, key, orderID string) error {
	if r.keyClaimErr != nil {
		return r.keyClaimErr
	}
	r.keys[customerID+"/"+key] = orderID
	return nil
}

// fakeCodes hands out predictable references so an assertion can name one.
type fakeCodes struct{ n int }

func (c *fakeCodes) Code() string {
	c.n++
	return "CODE" + strings.Repeat("X", c.n%3)
}

// fakeIDs hands out predictable ids.
type fakeIDs struct{ n int }

func (g *fakeIDs) New(prefix string) string {
	g.n++
	return prefix + "-" + strings.Repeat("0", 1) + itoa(g.n)
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

// fakeCart stands in for the cart module.
type fakeCart struct {
	cart   cartcontract.Cart
	found  bool
	err    error
	clears int
	clrErr error
}

func (c *fakeCart) Current(context.Context, string) (cartcontract.Cart, bool, error) {
	if c.err != nil {
		return cartcontract.Cart{}, false, c.err
	}
	return c.cart, c.found, nil
}

func (c *fakeCart) Clear(context.Context, string) error {
	c.clears++
	return c.clrErr
}

// fakeMerchant stands in for the merchant module.
type fakeMerchant struct {
	shop      merchantcontract.Merchant
	shopErr   error
	accepting bool
	acceptErr error
	owned     []string
	ownedErr  error
}

func (m *fakeMerchant) Merchant(context.Context, string) (merchantcontract.Merchant, error) {
	return m.shop, m.shopErr
}

func (m *fakeMerchant) IsAcceptingOrders(context.Context, string) (bool, error) {
	return m.accepting, m.acceptErr
}

func (m *fakeMerchant) OwnedBy(context.Context, string) ([]string, error) {
	return m.owned, m.ownedErr
}

// fakeUser stands in for the user module.
type fakeUser struct {
	address    usercontract.Address
	addressErr error
	profile    usercontract.Profile
	profileErr error
}

func (u *fakeUser) Address(context.Context, string, string) (usercontract.Address, error) {
	return u.address, u.addressErr
}

func (u *fakeUser) Profile(context.Context, string) (usercontract.Profile, error) {
	return u.profile, u.profileErr
}

// fakeDiscovery stands in for the discovery module.
type fakeDiscovery struct {
	reach discocontract.Reach
	err   error
}

func (d *fakeDiscovery) Reach(_ context.Context, _ discocontract.Point, merchantID string) (discocontract.Reach, error) {
	if d.err != nil {
		return discocontract.Reach{}, d.err
	}
	reach := d.reach
	reach.MerchantID = merchantID
	return reach, nil
}

// fakeSettings and fakeConfig stand in for the config module.
type fakeSettings struct {
	ints   map[string]int64
	ratios map[string]float64
	failOn string
}

func (s fakeSettings) Int(key string) (int64, error) {
	if key == s.failOn {
		return 0, errBoom
	}
	v, ok := s.ints[key]
	if !ok {
		return 0, errBoom
	}
	return v, nil
}

func (s fakeSettings) Bool(string) (bool, error) { return false, errBoom }

func (s fakeSettings) Ratio(key string) (float64, error) {
	if key == s.failOn {
		return 0, errBoom
	}
	v, ok := s.ratios[key]
	if !ok {
		return 0, errBoom
	}
	return v, nil
}

type fakeConfig struct {
	settings cfgcontract.Settings
	err      error
	seen     []cfgcontract.Placement
}

func (f *fakeConfig) Settings(_ context.Context, p cfgcontract.Placement) (cfgcontract.Settings, error) {
	f.seen = append(f.seen, p)
	if f.err != nil {
		return nil, f.err
	}
	return f.settings, nil
}

// appendixB is the default configuration an order reads: ৳40 base, ৳10 a
// kilometre, 1.5× when expanded, free delivery above ৳500, a ৳5000 cash
// ceiling and a two-minute cancellation window.
func appendixB() fakeSettings {
	return fakeSettings{
		ints: map[string]int64{
			cfgcontract.PricingDeliveryBase:     4000,
			cfgcontract.PricingDeliveryPerKm:    1000,
			cfgcontract.PricingFreeDelivery:     50000,
			cfgcontract.OrderCODLimit:           500000,
			cfgcontract.OrderCancellationWindow: 120,
		},
		ratios: map[string]float64{cfgcontract.PricingExpansionMult: 1.5},
	}
}

// fixedClock is a clock a test can move.
type fixedClock struct{ at time.Time }

func (c *fixedClock) Now() time.Time { return c.at }

// placedAt is the instant every order in these tests is placed.
var placedAt = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// fakeDispatch stands in for the dispatch module.
//
// The order tells dispatch when an order becomes ready to collect, and when one
// goes away. Both calls are best-effort from the order's side, which is what
// these tests pin: a dispatch outage must not stop a shop marking food ready.
type fakeDispatch struct {
	err      error
	offered  []string
	withdrew []string
}

func (d *fakeDispatch) Offer(_ context.Context, req dispatchcontract.OfferRequest) (dispatchcontract.Job, error) {
	d.offered = append(d.offered, req.OrderID)
	if d.err != nil {
		return dispatchcontract.Job{}, d.err
	}
	return dispatchcontract.Job{ID: "JOB-1", OrderID: req.OrderID, Status: "waiting"}, nil
}

func (d *fakeDispatch) Withdraw(_ context.Context, orderID, _ string) error {
	d.withdrew = append(d.withdrew, orderID)
	return d.err
}
