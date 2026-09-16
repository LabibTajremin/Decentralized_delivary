package order

import (
	"context"
	"testing"
	"time"

	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

func customer() application.Caller {
	return application.Caller{Actor: domain.ActorCustomer, ID: "USR-1", Lang: "en"}
}

func shopkeeper() application.Caller {
	return application.Caller{
		Actor: domain.ActorMerchant, ID: "USR-OWNER",
		MerchantIDs: []string{"MER-1"}, Lang: "en",
	}
}

func operator() application.Caller {
	return application.Caller{Actor: domain.ActorAdmin, ID: "USR-ADMIN", Lang: "en"}
}

// move applies a transition and fails the test if it is refused.
func move(t *testing.T, r *rig, orderID string, to domain.Status, reason string, caller application.Caller) application.View {
	t.Helper()
	view, err := r.transitions.Execute(context.Background(), orderID, to, reason, caller)
	if err != nil {
		t.Fatalf("%s: %v", to, err)
	}
	return view
}

// The whole happy path, through the four parties who touch it.
func TestAnOrderGoesAllTheWay(t *testing.T) {
	r := newRig()
	placed := place(t, r, "")

	accepted := move(t, r, placed.ID, domain.StatusAccepted, "", shopkeeper())
	if accepted.Status != "accepted" || accepted.StatusLabel != "The shop has accepted" {
		t.Fatalf("view = %+v", accepted)
	}
	// From the shop's point of view, the next moves are the shop's.
	if len(accepted.NextActions) != 2 {
		t.Errorf("next actions = %v", accepted.NextActions)
	}

	move(t, r, placed.ID, domain.StatusPreparing, "", shopkeeper())
	move(t, r, placed.ID, domain.StatusReady, "", shopkeeper())

	rider := application.Caller{Actor: domain.ActorPartner, ID: "PTR-1"}
	move(t, r, placed.ID, domain.StatusPickedUp, "", rider)
	delivered := move(t, r, placed.ID, domain.StatusDelivered, "", rider)

	if delivered.Status != "delivered" || delivered.Live {
		t.Fatalf("view = %+v", delivered)
	}
	if len(delivered.NextActions) != 0 {
		t.Errorf("a delivered order still offers %v", delivered.NextActions)
	}
	// The history has every step, in order, starting with the placement.
	if len(delivered.Events) != 6 {
		t.Fatalf("events = %+v", delivered.Events)
	}
	if delivered.Events[0].Status != "placed" || delivered.Events[5].Status != "delivered" {
		t.Errorf("history = %+v", delivered.Events)
	}
	for _, e := range delivered.Events {
		if e.Label == "" {
			t.Errorf("event %q has no sentence", e.Status)
		}
	}
}

func TestIllegalMovesAreRefusedThroughTheUseCase(t *testing.T) {
	r := newRig()
	placed := place(t, r, "")
	ctx := context.Background()

	cases := []struct {
		name   string
		to     domain.Status
		caller application.Caller
		code   string
	}{
		{"a customer accepting their own order", domain.StatusAccepted, customer(), "not_your_transition"},
		{"a shop delivering it itself", domain.StatusDelivered, shopkeeper(), "illegal_transition"},
		{"a rider picking up what is not ready", domain.StatusPickedUp,
			application.Caller{Actor: domain.ActorPartner, ID: "PTR-1"}, "illegal_transition"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.transitions.Execute(ctx, placed.ID, tc.to, "", tc.caller)
			if errs.CodeOf(err) != tc.code {
				t.Fatalf("err = %v, want %s", err, tc.code)
			}
		})
	}

	// And nothing moved.
	if got, _ := r.repo.Order(ctx, placed.ID); got.Status != domain.StatusPlaced {
		t.Errorf("status = %q after three refusals", got.Status)
	}
}

// A shop refusing an order owes the customer a reason. It is the only thing
// they will have to go on.
func TestARejectionNeedsAReason(t *testing.T) {
	r := newRig()
	placed := place(t, r, "")

	_, err := r.transitions.Execute(context.Background(), placed.ID, domain.StatusRejected, "", shopkeeper())
	if errs.CodeOf(err) != "reason_required" {
		t.Fatalf("err = %v", err)
	}

	view := move(t, r, placed.ID, domain.StatusRejected, "কাচ্চি শেষ হয়ে গেছে", shopkeeper())
	if view.Status != "rejected" || view.Live {
		t.Fatalf("view = %+v", view)
	}
	last := view.Events[len(view.Events)-1]
	if last.Reason != "কাচ্চি শেষ হয়ে গেছে" || last.Actor != "merchant" {
		t.Errorf("event = %+v", last)
	}
}

// The clock is the second question. The table says a cancellation is possible
// from `placed`; the window says whether it is still the customer's to make.
func TestTheCancellationWindowIsEnforcedOnTheWayThrough(t *testing.T) {
	r := newRig()
	placed := place(t, r, "")
	ctx := context.Background()

	inside, err := r.transitions.CancelStatus(ctx, placed.ID, customer())
	if err != nil {
		t.Fatalf("CancelStatus: %v", err)
	}
	if !inside.Allowed || inside.SecondsLeft != 120 {
		t.Fatalf("cancel = %+v", inside)
	}

	r.clock.at = placedAt.Add(3 * time.Minute)
	late, err := r.transitions.CancelStatus(ctx, placed.ID, customer())
	if err != nil {
		t.Fatalf("CancelStatus: %v", err)
	}
	if late.Allowed || late.Reason != domain.ReasonTooLate || late.Text == "" {
		t.Fatalf("cancel = %+v", late)
	}

	_, err = r.transitions.Execute(ctx, placed.ID, domain.StatusCancelled, "", customer())
	if errs.CodeOf(err) != "cannot_cancel" {
		t.Fatalf("err = %v", err)
	}

	// An admin is not bound by the window: clearing up after a customer who
	// phoned in is exactly what operations is for.
	view := move(t, r, placed.ID, domain.StatusCancelled, "customer phoned", operator())
	if view.Status != "cancelled" {
		t.Fatalf("view = %+v", view)
	}
}

func TestCancellingInsideTheWindow(t *testing.T) {
	r := newRig()
	placed := place(t, r, "")
	view := move(t, r, placed.ID, domain.StatusCancelled, "changed my mind", customer())
	if view.Status != "cancelled" || view.Live {
		t.Fatalf("view = %+v", view)
	}
	if view.Cancel.Allowed || view.Cancel.Reason != domain.ReasonAlreadyFinished {
		t.Errorf("a cancelled order still offers cancellation: %+v", view.Cancel)
	}
}

// An order is another shop's business or nobody's. The refusal is a not-found
// rather than a forbidden: "that order exists but is not yours" tells a prober
// which ids are real.
func TestAnOrderIsOnlyReachableByItsOwnParties(t *testing.T) {
	r := newRig()
	placed := place(t, r, "")
	ctx := context.Background()

	stranger := application.Caller{Actor: domain.ActorCustomer, ID: "USR-2"}
	if _, err := r.transitions.Execute(ctx, placed.ID, domain.StatusCancelled, "", stranger); !errs.Is(err, errs.KindNotFound) {
		t.Fatalf("another customer: err = %v", err)
	}
	if _, err := r.reads.One(ctx, placed.ID, stranger); !errs.Is(err, errs.KindNotFound) {
		t.Fatalf("another customer reading: err = %v", err)
	}

	otherShop := application.Caller{
		Actor: domain.ActorMerchant, ID: "USR-OTHER", MerchantIDs: []string{"MER-2"},
	}
	if _, err := r.transitions.Execute(ctx, placed.ID, domain.StatusAccepted, "", otherShop); !errs.Is(err, errs.KindNotFound) {
		t.Fatalf("another shop: err = %v", err)
	}
	if _, err := r.reads.One(ctx, placed.ID, otherShop); !errs.Is(err, errs.KindNotFound) {
		t.Fatalf("another shop reading: err = %v", err)
	}
}

func TestTransitionFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("no such order", func(t *testing.T) {
		r := newRig()
		if _, err := r.transitions.Execute(ctx, "ORD-nope", domain.StatusAccepted, "", shopkeeper()); !errs.Is(err, errs.KindNotFound) {
			t.Fatalf("err = %v", err)
		}
		if _, err := r.transitions.CancelStatus(ctx, "ORD-nope", customer()); !errs.Is(err, errs.KindNotFound) {
			t.Fatalf("cancel status: err = %v", err)
		}
		if _, err := r.reads.One(ctx, "ORD-nope", customer()); !errs.Is(err, errs.KindNotFound) {
			t.Fatalf("read: err = %v", err)
		}
	})

	t.Run("the order cannot be read", func(t *testing.T) {
		r := newRig()
		r.repo.readErr = errBoom
		if _, err := r.transitions.Execute(ctx, "ORD-1", domain.StatusAccepted, "", shopkeeper()); errs.CodeOf(err) != "order_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the transition cannot be written", func(t *testing.T) {
		r := newRig()
		placed := place(t, r, "")
		r.repo.transitionErr = errBoom
		if _, err := r.transitions.Execute(ctx, placed.ID, domain.StatusAccepted, "", shopkeeper()); errs.CodeOf(err) != "order_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the cancellation window cannot be read", func(t *testing.T) {
		r := newRig()
		placed := place(t, r, "")
		r.config.err = errBoom
		if _, err := r.transitions.Execute(ctx, placed.ID, domain.StatusCancelled, "", customer()); errs.CodeOf(err) != "order_config_unavailable" {
			t.Fatalf("err = %v", err)
		}
		if _, err := r.transitions.CancelStatus(ctx, placed.ID, customer()); errs.CodeOf(err) != "order_config_unavailable" {
			t.Fatalf("cancel status: err = %v", err)
		}
	})

	t.Run("the window setting is missing", func(t *testing.T) {
		r := newRig()
		placed := place(t, r, "")
		settings := appendixB()
		settings.failOn = cfgcontract.OrderCancellationWindow
		r.config.settings = settings
		if _, err := r.transitions.CancelStatus(ctx, placed.ID, customer()); errs.CodeOf(err) != "order_config_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})
}

// The contract four other modules will call.
func TestTheContract(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	var api contract.OrderContract = r.service

	t.Run("a prepaid order is placed when the money arrives", func(t *testing.T) {
		view, err := r.place.Execute(ctx, "USR-1", application.PlaceRequest{
			AddressID: "ADR-1", Payment: "online",
		})
		if err != nil {
			t.Fatalf("Place: %v", err)
		}
		if err := api.MarkPaid(ctx, view.ID); err != nil {
			t.Fatalf("MarkPaid: %v", err)
		}
		got, err := api.Order(ctx, view.ID)
		if err != nil {
			t.Fatalf("Order: %v", err)
		}
		if got.Status != "placed" || !got.Live {
			t.Fatalf("order = %+v", got)
		}
		// The contract carries what dispatch and tracking need and nothing
		// else: no option prices, no customer note.
		if got.Pickup.Name == "" || got.Destination.Name == "" || got.Count != 2 {
			t.Errorf("order = %+v", got)
		}
		if len(got.Events) != 2 {
			t.Errorf("events = %+v", got.Events)
		}
	})

	t.Run("a payment that never arrives cancels the order", func(t *testing.T) {
		view, err := r.place.Execute(ctx, "USR-1", application.PlaceRequest{
			AddressID: "ADR-1", Payment: "online",
		})
		if err != nil {
			t.Fatalf("Place: %v", err)
		}
		if err := api.MarkPaymentFailed(ctx, view.ID, "timed out"); err != nil {
			t.Fatalf("MarkPaymentFailed: %v", err)
		}
		got, _ := api.Order(ctx, view.ID)
		if got.Status != "cancelled" || got.Live {
			t.Fatalf("order = %+v", got)
		}
	})

	t.Run("Advance is a way in, not a way round", func(t *testing.T) {
		view := place(t, r, "")
		// The state machine still applies.
		if err := api.Advance(ctx, view.ID, "delivered", "partner", "PTR-1", ""); err == nil {
			t.Fatal("a rider delivered an order the shop had not made")
		}
		if err := api.Advance(ctx, view.ID, "accepted", "admin", "USR-ADMIN", ""); err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if err := api.Advance(ctx, view.ID, "shipped", "admin", "USR-ADMIN", ""); errs.CodeOf(err) != "unknown_status" {
			t.Fatalf("err = %v", err)
		}
		// Two actors a consumer may not claim. The customer's cancellation is
		// bound by a window checked on their own path; a shop's authority is
		// "this shop is yours", which only the HTTP layer can establish.
		for _, actor := range []string{"customer", "merchant"} {
			if err := api.Advance(ctx, view.ID, "cancelled", actor, "USR-1", ""); errs.CodeOf(err) != "unknown_actor" {
				t.Fatalf("%s: err = %v", actor, err)
			}
		}
	})

	t.Run("no such order", func(t *testing.T) {
		if _, err := api.Order(ctx, "ORD-nope"); !errs.Is(err, errs.KindNotFound) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the store is broken", func(t *testing.T) {
		broken := newRig()
		broken.repo.readErr = errBoom
		if _, err := broken.service.Order(ctx, "ORD-1"); errs.CodeOf(err) != "order_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})
}

// The order tells dispatch when a shop marks food ready, so a rider's phone
// buzzes while it is hot — and tells it again when an order goes away, so
// nobody rides to a shop for nothing.
func TestTheOrderTellsDispatch(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	placed := place(t, r, "")

	move(t, r, placed.ID, domain.StatusAccepted, "", shopkeeper())
	move(t, r, placed.ID, domain.StatusPreparing, "", shopkeeper())
	if len(r.dispatch.offered) != 0 {
		t.Fatalf("dispatch was told too early: %v", r.dispatch.offered)
	}

	move(t, r, placed.ID, domain.StatusReady, "", shopkeeper())
	if len(r.dispatch.offered) != 1 || r.dispatch.offered[0] != placed.ID {
		t.Fatalf("offered = %v", r.dispatch.offered)
	}

	// An admin calling it off takes the job back off the board.
	move(t, r, placed.ID, domain.StatusCancelled, "customer phoned", operator())
	if len(r.dispatch.withdrew) != 1 || r.dispatch.withdrew[0] != placed.ID {
		t.Fatalf("withdrew = %v", r.dispatch.withdrew)
	}

	// A rejection withdraws too — the shop said no, so there is nothing to
	// carry.
	second := place(t, r, "")
	move(t, r, second.ID, domain.StatusRejected, "কাচ্চি শেষ", shopkeeper())
	if len(r.dispatch.withdrew) != 2 {
		t.Fatalf("withdrew = %v", r.dispatch.withdrew)
	}
	_ = ctx
}

// A dispatch outage must not stop a shop marking food ready. An order sitting
// at `ready` with no rider is visible to an operator and recoverable by a
// sweep; an order the shop could not mark ready at all is a kitchen with
// cooling food and no way to say so.
func TestADispatchOutageDoesNotBlockTheShop(t *testing.T) {
	r := newRig()
	placed := place(t, r, "")
	r.dispatch.err = errBoom

	move(t, r, placed.ID, domain.StatusAccepted, "", shopkeeper())
	move(t, r, placed.ID, domain.StatusPreparing, "", shopkeeper())
	view := move(t, r, placed.ID, domain.StatusReady, "", shopkeeper())
	if view.Status != "ready" {
		t.Fatalf("view = %+v", view)
	}
}

// An order module wired without dispatch — which is how cmd/api builds it
// before the loop is closed — must not panic on a transition.
func TestATransitionWithNoDispatchWired(t *testing.T) {
	r := newRig()
	placed := place(t, r, "")
	r.transitions.UseDispatch(nil)

	move(t, r, placed.ID, domain.StatusAccepted, "", shopkeeper())
	move(t, r, placed.ID, domain.StatusPreparing, "", shopkeeper())
	if view := move(t, r, placed.ID, domain.StatusReady, "", shopkeeper()); view.Status != "ready" {
		t.Fatalf("view = %+v", view)
	}
}
