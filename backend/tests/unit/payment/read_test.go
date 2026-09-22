package payment

import (
	"context"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

func TestReadingAPaymentAsItsCustomer(t *testing.T) {
	ctx := context.Background()
	r, paymentID := startedCheckout(t, 50000)
	_ = paymentID

	view, err := r.reads.ForOrder(ctx, "ord_1", "usr_1", false, "en")
	if err != nil {
		t.Fatalf("ForOrder: %v", err)
	}
	if view.OrderID != "ord_1" || view.Status != "pending" {
		t.Fatalf("view = %+v", view)
	}
}

// A stranger asking about somebody else's order gets the same answer as an
// order that does not exist — nothing that lets them learn it is real.
func TestReadingSomebodyElsesPaymentIsRefused(t *testing.T) {
	ctx := context.Background()
	r, _ := startedCheckout(t, 50000)

	if _, err := r.reads.ForOrder(ctx, "ord_1", "usr_stranger", false, "en"); errs.CodeOf(err) != "payment_not_found" {
		t.Fatalf("err = %v", err)
	}
}

func TestAnAdminReadsAnyPayment(t *testing.T) {
	ctx := context.Background()
	r, _ := startedCheckout(t, 50000)

	view, err := r.reads.ForOrder(ctx, "ord_1", "", true, "en")
	if err != nil {
		t.Fatalf("ForOrder: %v", err)
	}
	if view.OrderID != "ord_1" {
		t.Fatalf("view = %+v", view)
	}
}

func TestReadingAnOrderWithNoPayment(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	if _, err := r.reads.ForOrder(ctx, "ord_missing", "usr_1", false, "en"); errs.CodeOf(err) != "payment_not_found" {
		t.Fatalf("err = %v", err)
	}
}

func TestReadingAPaymentWhenStorageIsDown(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.repo.latestForOrderErr = errBoom
	if _, err := r.reads.ForOrder(ctx, "ord_1", "usr_1", false, "en"); errs.CodeOf(err) != "payment_unavailable" {
		t.Fatalf("err = %v", err)
	}
}
