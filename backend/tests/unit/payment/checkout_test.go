package payment

import (
	"context"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/application"
	orderx "github.com/rootlogic-lab/delivery/backend/internal/modules/payment/external/order"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// rig is one assembled payment module with every collaborator reachable.
type rig struct {
	repo     *fakeRepo
	gateway  *fakeGateway
	order    *fakeOrder
	dispatch *fakeDispatch
	clock    *clock.Fixed

	checkout    *application.CheckoutUseCase
	webhook     *application.WebhookUseCase
	collections *application.CollectionUseCase
	refunds     *application.RefundUseCase
	reads       *application.ReadUseCase
	service     *application.Service
}

func newRig() *rig {
	repo := newRepo()
	gateway := newGateway()
	order := newOrder()
	dispatch := newDispatch()
	clk := clock.NewFixed(at)
	ids := &fakeIDs{}

	collections := application.NewCollectionUseCase(repo, dispatch, clk, ids)
	refunds := application.NewRefundUseCase(repo, gateway, clk)
	return &rig{
		repo: repo, gateway: gateway, order: order, dispatch: dispatch, clock: clk,
		checkout:    application.NewCheckoutUseCase(repo, gateway, order, clk, ids),
		webhook:     application.NewWebhookUseCase(repo, gateway, order, clk),
		collections: collections,
		refunds:     refunds,
		reads:       application.NewReadUseCase(repo),
		service:     application.NewService(repo, collections, refunds),
	}
}

// pendingOrder is an order ready for online checkout.
func pendingOrder(orderID, customerID string, totalMinor int64) orderx.Order {
	return orderx.Order{
		ID: orderID, CustomerID: customerID, Status: "pending_payment",
		PaymentMethod: "online", TotalMinor: totalMinor, Currency: "BDT",
	}
}

func TestStartingACheckout(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.order.orders["ord_1"] = pendingOrder("ord_1", "usr_1", 50000)

	view, err := r.checkout.Start(ctx, "usr_1", "ord_1", "en")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if view.Status != "pending" || view.Amount.Minor != 50000 {
		t.Fatalf("view = %+v", view)
	}
	if len(r.gateway.checkoutCalls) != 1 {
		t.Fatalf("gateway was called %d times", len(r.gateway.checkoutCalls))
	}
	if len(r.repo.payments) != 1 {
		t.Fatalf("%d payments written, want 1", len(r.repo.payments))
	}
}

// Idempotent: a second checkout for the same order, before the first is
// answered, returns the same attempt rather than starting a second one.
func TestResumingAnOpenCheckout(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.order.orders["ord_1"] = pendingOrder("ord_1", "usr_1", 50000)

	first, err := r.checkout.Start(ctx, "usr_1", "ord_1", "en")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	second, err := r.checkout.Start(ctx, "usr_1", "ord_1", "en")
	if err != nil {
		t.Fatalf("Start again: %v", err)
	}
	if first.PaymentID != second.PaymentID {
		t.Fatalf("two payments: %s and %s", first.PaymentID, second.PaymentID)
	}
	if len(r.gateway.checkoutCalls) != 1 {
		t.Fatalf("the gateway was asked twice for one open checkout")
	}
}

// A checkout the unique index catches as a race is not an error — the one
// that won is the answer.
func TestACheckoutRaceIsIdempotent(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.order.orders["ord_1"] = pendingOrder("ord_1", "usr_1", 50000)

	won, err := r.checkout.Start(ctx, "usr_1", "ord_1", "en")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// The loser's own PendingForOrder came back empty — won's attempt is
	// still there — so it wrote, and the unique index refused it. The
	// re-read that follows finds won's attempt and returns it.
	r.repo.pendingMissOnce = true
	lost, err := r.checkout.Start(ctx, "usr_1", "ord_1", "en")
	if err != nil {
		t.Fatalf("the losing attempt failed: %v", err)
	}
	if lost.PaymentID != won.PaymentID {
		t.Fatalf("the loser got %q, want the winner's %q", lost.PaymentID, won.PaymentID)
	}
}

// If the re-read after losing a race also fails, there is nothing honest left
// to return but an outage — the loser cannot fabricate an answer it never saw.
func TestACheckoutRaceWhoseRereadAlsoFails(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.order.orders["ord_1"] = pendingOrder("ord_1", "usr_1", 50000)

	if _, err := r.checkout.Start(ctx, "usr_1", "ord_1", "en"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	r.repo.pendingMissOnce = true
	r.repo.pendingForOrderErr = errBoom
	if _, err := r.checkout.Start(ctx, "usr_1", "ord_1", "en"); errs.CodeOf(err) != "payment_unavailable" {
		t.Fatalf("err = %v", err)
	}
}

func TestCheckoutRefusesWhatItMust(t *testing.T) {
	ctx := context.Background()

	t.Run("no such order", func(t *testing.T) {
		r := newRig()
		if _, err := r.checkout.Start(ctx, "usr_1", "ord_missing", "en"); errs.CodeOf(err) != "payment_not_found" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("somebody else's order", func(t *testing.T) {
		r := newRig()
		r.order.orders["ord_1"] = pendingOrder("ord_1", "usr_owner", 50000)
		if _, err := r.checkout.Start(ctx, "usr_stranger", "ord_1", "en"); errs.CodeOf(err) != "payment_not_found" {
			t.Fatalf("a stranger's checkout was not refused as not-found: %v", err)
		}
	})

	t.Run("a cash order", func(t *testing.T) {
		r := newRig()
		order := pendingOrder("ord_1", "usr_1", 50000)
		order.PaymentMethod = "cash"
		r.order.orders["ord_1"] = order
		if _, err := r.checkout.Start(ctx, "usr_1", "ord_1", "en"); errs.CodeOf(err) != "not_online_payment" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("an order not awaiting payment", func(t *testing.T) {
		r := newRig()
		order := pendingOrder("ord_1", "usr_1", 50000)
		order.Status = "placed"
		r.order.orders["ord_1"] = order
		if _, err := r.checkout.Start(ctx, "usr_1", "ord_1", "en"); errs.CodeOf(err) != "not_awaiting_payment" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("order module unreachable", func(t *testing.T) {
		r := newRig()
		r.order.orderErr = errBoom
		if _, err := r.checkout.Start(ctx, "usr_1", "ord_1", "en"); errs.CodeOf(err) != "payment_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the pending lookup fails", func(t *testing.T) {
		r := newRig()
		r.order.orders["ord_1"] = pendingOrder("ord_1", "usr_1", 50000)
		r.repo.pendingForOrderErr = errBoom
		if _, err := r.checkout.Start(ctx, "usr_1", "ord_1", "en"); errs.CodeOf(err) != "payment_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the gateway is unreachable", func(t *testing.T) {
		r := newRig()
		r.order.orders["ord_1"] = pendingOrder("ord_1", "usr_1", 50000)
		r.gateway.checkoutErr = errBoom
		if _, err := r.checkout.Start(ctx, "usr_1", "ord_1", "en"); errs.CodeOf(err) != "gateway_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	// An order with nothing to collect — a zero total is not something a
	// checkout can ever legitimately be for.
	t.Run("nothing to collect", func(t *testing.T) {
		r := newRig()
		r.order.orders["ord_1"] = pendingOrder("ord_1", "usr_1", 0)
		if _, err := r.checkout.Start(ctx, "usr_1", "ord_1", "en"); errs.CodeOf(err) != "invalid_payment" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the write fails for a reason other than the race", func(t *testing.T) {
		r := newRig()
		r.order.orders["ord_1"] = pendingOrder("ord_1", "usr_1", 50000)
		r.repo.createPaymentErr = errBoom
		if _, err := r.checkout.Start(ctx, "usr_1", "ord_1", "en"); errs.CodeOf(err) != "payment_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})
}
