package payment

import (
	"context"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// TestServiceImplementsThePublicContract is what order's external/payment
// seam, and review/support in P16, actually depend on — PaymentContract,
// not this package's own types.
func TestServiceImplementsThePublicContract(t *testing.T) {
	ctx := context.Background()
	r, paymentID := capturedPayment(t, 50000)
	var api contract.PaymentContract = r.service

	got, found, err := api.PaymentFor(ctx, "ord_1")
	if err != nil || !found {
		t.Fatalf("found = %v, err = %v", found, err)
	}
	if got.ID != paymentID || got.Status != "captured" || got.Amount.Minor != 50000 {
		t.Fatalf("payment = %+v", got)
	}

	if err := api.Refund(ctx, "ord_1", "a customer complaint"); err != nil {
		t.Fatalf("Refund: %v", err)
	}
	refunded, _, err := api.PaymentFor(ctx, "ord_1")
	if err != nil {
		t.Fatalf("PaymentFor: %v", err)
	}
	if refunded.Status != "refunded" || refunded.Reason != "a customer complaint" {
		t.Fatalf("payment = %+v", refunded)
	}
}

func TestPaymentForANonExistentOrder(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	var api contract.PaymentContract = r.service

	_, found, err := api.PaymentFor(ctx, "ord_missing")
	if err != nil || found {
		t.Fatalf("found = %v, err = %v", found, err)
	}
}

func TestPaymentForWhenStorageIsDown(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.repo.latestForOrderErr = errBoom
	var api contract.PaymentContract = r.service

	if _, _, err := api.PaymentFor(ctx, "ord_1"); errs.CodeOf(err) != "payment_unavailable" {
		t.Fatalf("err = %v", err)
	}
}

// RecordCashCollection is order's own delivery hook, reached the same way —
// through the contract, not this package's application type.
func TestRecordCashCollectionOverTheContract(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	var api contract.PaymentContract = r.service

	if err := api.RecordCashCollection(ctx, "ord_1", "PTR-1", 50000); err != nil {
		t.Fatalf("RecordCashCollection: %v", err)
	}
	if len(r.repo.collections) != 1 {
		t.Fatalf("%d collections written, want 1", len(r.repo.collections))
	}
}
