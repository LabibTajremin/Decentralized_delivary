package order

import (
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/domain"
)

// The free cancellation window. Two questions, kept apart on purpose: the
// transition table says whether a cancellation is *possible* from where the
// order is, and this says whether it is still *free*.

const window = 2 * time.Minute

func placedOrder(t *testing.T, status domain.Status) domain.Order {
	t.Helper()
	order, err := domain.NewOrder(draft())
	if err != nil {
		t.Fatalf("NewOrder: %v", err)
	}
	order.Status = status
	return order
}

func TestTheWindowIsOpenThenClosed(t *testing.T) {
	order := placedOrder(t, domain.StatusPlaced)

	justPlaced := order.MayCustomerCancel(window, placedAt)
	if !justPlaced.Allowed || justPlaced.SecondsLeft != 120 {
		t.Fatalf("at placement: %+v", justPlaced)
	}
	if !justPlaced.FreeUntil.Equal(placedAt.Add(window)) {
		t.Errorf("free until %v", justPlaced.FreeUntil)
	}

	halfway := order.MayCustomerCancel(window, placedAt.Add(time.Minute))
	if !halfway.Allowed || halfway.SecondsLeft != 60 {
		t.Fatalf("halfway: %+v", halfway)
	}

	// Exactly at the boundary the window is shut. A customer who taps at the
	// instant it expires gets the same answer as one who taps a second later,
	// which is the only version a countdown can be honest about.
	atTheEdge := order.MayCustomerCancel(window, placedAt.Add(window))
	if atTheEdge.Allowed || atTheEdge.Reason != domain.ReasonTooLate {
		t.Fatalf("at the edge: %+v", atTheEdge)
	}
	if atTheEdge.SecondsLeft != 0 {
		t.Errorf("seconds left = %d, want 0", atTheEdge.SecondsLeft)
	}

	// And long afterwards the countdown does not go negative.
	late := order.MayCustomerCancel(window, placedAt.Add(time.Hour))
	if late.SecondsLeft != 0 {
		t.Errorf("seconds left = %d, want 0", late.SecondsLeft)
	}
}

// The window runs from placement rather than acceptance. A customer changes
// their mind about the decision they made, and a shop that takes six minutes to
// accept should not thereby shorten — or silently extend — the time they had.
func TestTheWindowRunsFromPlacementNotAcceptance(t *testing.T) {
	order := placedOrder(t, domain.StatusAccepted)

	inside := order.MayCustomerCancel(window, placedAt.Add(time.Minute))
	if !inside.Allowed {
		t.Fatalf("an accepted order inside the window: %+v", inside)
	}
	outside := order.MayCustomerCancel(window, placedAt.Add(3*time.Minute))
	if outside.Allowed || outside.Reason != domain.ReasonTooLate {
		t.Fatalf("an accepted order outside the window: %+v", outside)
	}
}

// Once the kitchen has started, the clock stops mattering: somebody has already
// paid for this in ingredients. The customer is told about the food rather than
// about a state machine.
func TestOnceItIsCookingTheWindowIsIrrelevant(t *testing.T) {
	for _, status := range []domain.Status{
		domain.StatusPreparing, domain.StatusReady, domain.StatusPickedUp,
	} {
		order := placedOrder(t, status)
		got := order.MayCustomerCancel(window, placedAt)
		if got.Allowed || got.Reason != domain.ReasonUnderway {
			t.Errorf("%s: %+v", status, got)
		}
	}
}

func TestAFinishedOrderCannotBeCancelled(t *testing.T) {
	for _, status := range []domain.Status{
		domain.StatusDelivered, domain.StatusCancelled,
		domain.StatusRejected, domain.StatusFailed,
	} {
		order := placedOrder(t, status)
		got := order.MayCustomerCancel(window, placedAt)
		if got.Allowed || got.Reason != domain.ReasonAlreadyFinished {
			t.Errorf("%s: %+v", status, got)
		}
	}
}

// An unpaid order is the customer's to abandon at any time. Nothing has been
// committed on their behalf — the shop has not even been told.
func TestAnUnpaidOrderCanAlwaysBeAbandoned(t *testing.T) {
	order := placedOrder(t, domain.StatusPendingPayment)
	if got := order.MayCustomerCancel(window, placedAt); !got.Allowed {
		t.Fatalf("%+v", got)
	}
	// Even long after the window, because the window is about a commitment
	// nobody has made yet.
	late := order.MayCustomerCancel(window, placedAt.Add(time.Hour))
	if late.Allowed || late.Reason != domain.ReasonTooLate {
		t.Fatalf("%+v", late)
	}
}
