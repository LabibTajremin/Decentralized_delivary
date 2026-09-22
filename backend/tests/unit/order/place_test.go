package order

import (
	"context"
	"testing"

	cartcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/cart/contract"
	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
	discocontract "github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/contract"
	merchantcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/domain"
	pricingapp "github.com/rootlogic-lab/delivery/backend/internal/modules/pricing/application"
	usercontract "github.com/rootlogic-lab/delivery/backend/internal/modules/user/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// rig is one assembled order module with every collaborator reachable.
type rig struct {
	repo      *fakeRepo
	cart      *fakeCart
	merchant  *fakeMerchant
	user      *fakeUser
	discovery *fakeDiscovery
	config    *fakeConfig
	pricing   *fakeConfig
	dispatch  *fakeDispatch
	payment   *fakePayment
	notify    *fakeNotify
	clock     *fixedClock

	place       *application.PlaceUseCase
	transitions *application.TransitionUseCase
	reads       *application.ReadUseCase
	service     *application.Service
}

func newRig() *rig {
	repo := newRepo()
	cart := &fakeCart{found: true, cart: readyCart()}
	shop := &fakeMerchant{shop: openShop(), accepting: true, owned: []string{"MER-1"}}
	users := &fakeUser{address: homeAddress(), profile: profile()}
	disco := &fakeDiscovery{reach: discocontract.Reach{
		Reachable: true, DivisionCode: "DHA", AreaCode: "DHA-DHK-DHN",
		DistrictCode: "DHA-DHK", DistanceM: 2400,
	}}
	config := &fakeConfig{settings: appendixB()}
	prices := &fakeConfig{settings: appendixB()}
	clk := &fixedClock{at: placedAt}
	ids := &fakeIDs{}

	dispatcher := &fakeDispatch{}
	payer := &fakePayment{}
	notifier := &fakeNotify{}
	transitions := application.NewTransitionUseCase(repo, shop, config, dispatcher, clk, ids)
	transitions.UsePayment(payer)
	transitions.UseNotification(notifier)
	return &rig{
		repo: repo, cart: cart, merchant: shop, user: users,
		discovery: disco, config: config, pricing: prices,
		dispatch: dispatcher, payment: payer, notify: notifier, clock: clk,
		place: application.NewPlaceUseCase(
			repo, &fakeCodes{}, cart, pricingapp.NewService(prices),
			shop, users, disco, config, clk, ids,
		),
		transitions: transitions,
		reads:       application.NewReadUseCase(repo, config, clk),
		service:     application.NewService(repo, transitions),
	}
}

func readyCart() cartcontract.Cart {
	return cartcontract.Cart{
		ID: "CRT-1", UserID: "USR-1", MerchantID: "MER-1",
		AddressID: "ADR-1", Lat: 23.7, Lng: 90.4,
		Lines: []cartcontract.Line{{
			ID: "CLN-1", Kind: "item", TargetID: "ITM-1", Name: "Biryani",
			UnitPrice: cartcontract.Money{Minor: 25000, Currency: "BDT", Display: "৳ 250"},
			Options: []cartcontract.Option{{
				GroupID: "GRP-size", OptionID: "OPT-large", Name: "Large",
				Price: cartcontract.Money{Minor: 5000, Currency: "BDT", Display: "৳ 50"},
			}},
			Quantity: 2, Note: "no chilli",
		}},
		Subtotal:  cartcontract.Money{Minor: 60000, Currency: "BDT", Display: "৳ 600"},
		Orderable: true,
	}
}

func openShop() merchantcontract.Merchant {
	return merchantcontract.Merchant{
		ID: "MER-1", Name: "Star Kabab", Phone: "+8801711000001",
		SingleLine: "House 32, Road 27, Dhanmondi", Lat: 23.7455, Lng: 90.3738,
		IsListed: true, IsOpenNow: true, DivisionCode: "DHA",
	}
}

func homeAddress() usercontract.Address {
	return usercontract.Address{
		ID: "ADR-1", Label: "Home", RecipientName: "Fardin",
		RecipientPhone: "+8801711111111", Line1: "House 5, Road 3",
		SingleLine: "House 5, Road 3, Dhanmondi", Lat: 23.746, Lng: 90.375,
		AreaCode: "DHA-DHK-DHN", AreaName: "Dhanmondi",
		DistrictCode: "DHA-DHK", DivisionCode: "DHA",
		Instructions: "Second gate",
	}
}

func profile() usercontract.Profile {
	return usercontract.Profile{UserID: "USR-1", Name: "Fardin", DisplayName: "Fardin"}
}

func place(t *testing.T, r *rig, key string) application.View {
	t.Helper()
	view, err := r.place.Execute(context.Background(), "USR-1", application.PlaceRequest{
		AddressID: "ADR-1", Payment: "cash", IdempotencyKey: key, Lang: "en",
	})
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	return view
}

func TestPlacingAnOrderFreezesEverything(t *testing.T) {
	r := newRig()
	view := place(t, r, "")

	if view.Status != "placed" || !view.Live || view.Payment != "cash" {
		t.Fatalf("view = %+v", view)
	}
	if view.Code == "" {
		t.Error("no order code")
	}
	// The lines were copied off the cart, options and note included.
	if len(view.Lines) != 1 || view.Lines[0].Name != "Biryani" ||
		view.Lines[0].Quantity != 2 || view.Lines[0].Note != "no chilli" {
		t.Fatalf("lines = %+v", view.Lines)
	}
	if len(view.Lines[0].Options) != 1 || view.Lines[0].Options[0].Name != "Large" {
		t.Errorf("options = %+v", view.Lines[0].Options)
	}
	// (250 + 50) × 2 = ৳600.
	if view.Lines[0].LineTotal.Minor != 60000 || view.Subtotal.Minor != 60000 {
		t.Fatalf("money = %+v", view)
	}
	// ৳600 clears the ৳500 free-delivery threshold, so delivery is nothing and
	// the total is the goods.
	if view.Delivery.Minor != 0 || view.Total.Minor != 60000 {
		t.Fatalf("charges = %+v", view)
	}
	// The receipt is rebuilt from what was frozen.
	if len(view.Receipt) < 3 || view.Receipt[0].Key != "subtotal" ||
		view.Receipt[len(view.Receipt)-1].Key != "total" {
		t.Fatalf("receipt = %+v", view.Receipt)
	}
	// Both ends of the delivery were copied.
	if view.Pickup.Name != "Star Kabab" || view.Destination.Name != "Fardin" ||
		view.Destination.SingleLine == "" {
		t.Fatalf("places = %+v / %+v", view.Pickup, view.Destination)
	}
	// The customer can still change their mind, and is told for how long.
	if !view.Cancel.Allowed || view.Cancel.SecondsLeft != 120 {
		t.Errorf("cancel = %+v", view.Cancel)
	}
	// And what they may do next comes from the state machine.
	if len(view.NextActions) != 1 || view.NextActions[0] != "cancelled" {
		t.Errorf("next actions = %v", view.NextActions)
	}
	// The cart is emptied once the order exists.
	if r.cart.clears != 1 {
		t.Errorf("the cart was cleared %d times", r.cart.clears)
	}
}

// The phase's second acceptance criterion. A customer on a village 2G
// connection taps "Place order", sees nothing happen, and taps again.
func TestOneIdempotencyKeyMakesOneOrder(t *testing.T) {
	r := newRig()
	first := place(t, r, "attempt-1")
	second := place(t, r, "attempt-1")

	if first.ID != second.ID {
		t.Fatalf("two orders: %s and %s", first.ID, second.ID)
	}
	if r.repo.created != 1 {
		t.Fatalf("%d orders were written", r.repo.created)
	}
	// The second attempt did not touch anything else either — no second cart
	// clear, and no second read of the shop.
	if r.cart.clears != 1 {
		t.Errorf("the cart was cleared %d times", r.cart.clears)
	}

	// A different key is a different order, which is what a customer placing a
	// second order five minutes later means.
	r.cart.cart = readyCart()
	third := place(t, r, "attempt-2")
	if third.ID == first.ID || r.repo.created != 2 {
		t.Fatalf("a new key reused an order: %s, %d written", third.ID, r.repo.created)
	}
}

// Without a key there is no protection, which is a real risk and worth having
// the test say so out loud.
func TestWithoutAKeyEveryRequestIsANewOrder(t *testing.T) {
	r := newRig()
	first := place(t, r, "")
	second := place(t, r, "")
	if first.ID == second.ID || r.repo.created != 2 {
		t.Fatalf("expected two orders, got %s and %s", first.ID, second.ID)
	}
}

// The prices are read at the moment of writing, not taken from the cart. Here
// the cart claims one subtotal and the order is written with what pricing says
// about the lines — which is the cart's own subtotal, but arrived at by the
// server rather than accepted from the request.
func TestTheOrderIsPricedOnTheWayIn(t *testing.T) {
	r := newRig()
	// A cheaper cart: ৳250 of goods, below the free-delivery threshold.
	cart := readyCart()
	cart.Subtotal = cartcontract.Money{Minor: 25000, Currency: "BDT", Display: "৳ 250"}
	r.cart.cart = cart

	view := place(t, r, "")
	// ৳40 + 3 × ৳10 = ৳70 over 2.4 km.
	if view.Delivery.Minor != 7000 || view.Total.Minor != 32000 {
		t.Fatalf("charges = %+v", view)
	}
	if view.DistanceM != 2400 {
		t.Errorf("distance = %v, want the one discovery measured", view.DistanceM)
	}
	// Pricing was asked about the address's placement, not the cart's.
	if len(r.pricing.seen) != 1 || r.pricing.seen[0].AreaCode != "DHA-DHK-DHN" {
		t.Errorf("pricing was asked about %+v", r.pricing.seen)
	}
}

// D2 reaching the order: a shop the customer widened their search to find is
// dearer to deliver from, and the receipt says by how much.
func TestAnExpandedOrderCarriesTheSurcharge(t *testing.T) {
	r := newRig()
	cart := readyCart()
	cart.Subtotal = cartcontract.Money{Minor: 25000}
	r.cart.cart = cart
	r.discovery.reach = discocontract.Reach{
		Reachable: true, AreaCode: "DHA-DHK-DHN", DistrictCode: "DHA-DHK", DivisionCode: "DHA",
		DistanceM: 8000, RequiredLevel: 1, Expanded: true,
	}

	view := place(t, r, "")
	if !view.Expanded {
		t.Fatalf("view = %+v", view)
	}
	found := false
	for _, row := range view.Receipt {
		if row.Key == "expansion_surcharge" && row.Amount.Minor == 6000 {
			found = true
		}
	}
	if !found {
		t.Errorf("receipt = %+v", view.Receipt)
	}
}

func TestPlacementRefusals(t *testing.T) {
	ctx := context.Background()
	request := application.PlaceRequest{AddressID: "ADR-1", Payment: "cash", Lang: "en"}

	t.Run("an unknown way to pay", func(t *testing.T) {
		r := newRig()
		_, err := r.place.Execute(ctx, "USR-1", application.PlaceRequest{AddressID: "ADR-1", Payment: "barter"})
		if errs.CodeOf(err) != "invalid_payment_method" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("no cart", func(t *testing.T) {
		r := newRig()
		r.cart.found = false
		if _, err := r.place.Execute(ctx, "USR-1", request); errs.CodeOf(err) != "empty_order" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("an empty cart", func(t *testing.T) {
		r := newRig()
		r.cart.cart = cartcontract.Cart{MerchantID: "MER-1", Orderable: true}
		if _, err := r.place.Execute(ctx, "USR-1", request); errs.CodeOf(err) != "empty_order" {
			t.Fatalf("err = %v", err)
		}
	})

	// The cart's own verdict, re-read rather than taken from the client.
	t.Run("a cart the cart module refuses", func(t *testing.T) {
		r := newRig()
		cart := readyCart()
		cart.Orderable, cart.Blocker = false, "shop_closed"
		r.cart.cart = cart
		_, err := r.place.Execute(ctx, "USR-1", request)
		if errs.CodeOf(err) != "cart_not_orderable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("a shop that is not taking orders", func(t *testing.T) {
		r := newRig()
		r.merchant.accepting = false
		if _, err := r.place.Execute(ctx, "USR-1", request); errs.CodeOf(err) != "shop_not_accepting" {
			t.Fatalf("err = %v", err)
		}
	})

	// D3, checked again at placement against the address actually being
	// ordered to — which may not be the one the cart was held against.
	t.Run("an address in another division", func(t *testing.T) {
		r := newRig()
		r.discovery.reach = discocontract.Reach{Reason: discocontract.ReasonOutsideDivision}
		_, err := r.place.Execute(ctx, "USR-1", request)
		if errs.CodeOf(err) != "not_deliverable" {
			t.Fatalf("err = %v", err)
		}
		if errs.MessageOf(err) == "" {
			t.Error("no sentence explaining it")
		}
	})

	t.Run("an address beyond the widest radius", func(t *testing.T) {
		r := newRig()
		r.discovery.reach = discocontract.Reach{Reason: discocontract.ReasonBeyondMaxRadius}
		_, err := r.place.Execute(ctx, "USR-1", application.PlaceRequest{AddressID: "ADR-1", Payment: "cash"})
		if errs.CodeOf(err) != "not_deliverable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("a cash order above the area's ceiling", func(t *testing.T) {
		r := newRig()
		settings := appendixB()
		settings.ints[cfgcontract.OrderCODLimit] = 10000 // ৳100
		r.config.settings = settings
		_, err := r.place.Execute(ctx, "USR-1", request)
		if errs.CodeOf(err) != "cod_limit_exceeded" {
			t.Fatalf("err = %v", err)
		}
	})

	// The same order online is fine — nobody is carrying the cash.
	t.Run("the same order paid online", func(t *testing.T) {
		r := newRig()
		settings := appendixB()
		settings.ints[cfgcontract.OrderCODLimit] = 10000
		r.config.settings = settings
		view, err := r.place.Execute(ctx, "USR-1", application.PlaceRequest{AddressID: "ADR-1", Payment: "online"})
		if err != nil {
			t.Fatalf("Place: %v", err)
		}
		if view.Status != string(domain.StatusPendingPayment) {
			t.Fatalf("status = %q", view.Status)
		}
	})
}

func TestPlacementDownstreamFailures(t *testing.T) {
	ctx := context.Background()
	request := application.PlaceRequest{AddressID: "ADR-1", Payment: "cash", IdempotencyKey: "k"}

	cases := []struct {
		name   string
		break_ func(*rig)
	}{
		{"the key cannot be looked up", func(r *rig) { r.repo.keyReadErr = errBoom }},
		{"the cart cannot be read", func(r *rig) { r.cart.err = errBoom }},
		{"the address cannot be read", func(r *rig) { r.user.addressErr = errBoom }},
		{"the profile cannot be read", func(r *rig) { r.user.profileErr = errBoom }},
		{"the shop cannot be read", func(r *rig) { r.merchant.shopErr = errBoom }},
		{"the shop's availability cannot be read", func(r *rig) { r.merchant.acceptErr = errBoom }},
		{"reachability cannot be checked", func(r *rig) { r.discovery.err = errBoom }},
		{"pricing is unreachable", func(r *rig) { r.pricing.err = errBoom }},
		{"the order's rules cannot be read", func(r *rig) { r.config.err = errBoom }},
		{"the order cannot be written", func(r *rig) { r.repo.createErr = errBoom }},
		{"the key cannot be claimed", func(r *rig) { r.repo.keyClaimErr = errBoom }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig()
			tc.break_(r)
			if _, err := r.place.Execute(ctx, "USR-1", request); err == nil {
				t.Fatal("an order was placed anyway")
			}
		})
	}
}

// A cart that will not empty is a nuisance, not a double charge. The order
// exists; failing the request would leave the customer thinking it does not.
func TestAFailedCartClearDoesNotFailTheOrder(t *testing.T) {
	r := newRig()
	r.cart.clrErr = errBoom
	view := place(t, r, "")
	if view.ID == "" || r.repo.created != 1 {
		t.Fatalf("view = %+v, created = %d", view, r.repo.created)
	}
}

// A window that cannot be read stops the order rather than placing one nobody
// can describe. It is read alongside the COD ceiling, before anything is
// written — an order that exists but cannot say how long the customer has to
// change their mind is worse than one that was refused.
func TestAnUnreadableWindowRefusesThePlacement(t *testing.T) {
	r := newRig()
	settings := appendixB()
	settings.failOn = cfgcontract.OrderCancellationWindow
	r.config.settings = settings

	_, err := r.place.Execute(context.Background(), "USR-1", application.PlaceRequest{
		AddressID: "ADR-1", Payment: "cash",
	})
	if errs.CodeOf(err) != "order_config_unavailable" {
		t.Fatalf("err = %v", err)
	}
	if r.repo.created != 0 {
		t.Errorf("%d orders were written anyway", r.repo.created)
	}
}
