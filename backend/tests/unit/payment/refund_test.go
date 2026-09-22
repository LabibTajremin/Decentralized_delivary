package payment

import (
	"context"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// capturedPayment is a rig with a fully paid order, and the payment id.
func capturedPayment(t *testing.T, totalMinor int64) (*rig, string) {
	t.Helper()
	ctx := context.Background()
	r, paymentID := startedCheckout(t, totalMinor)
	payload, sig := sign(r.gateway.validSignature, webhookPayload{
		Reference: paymentID, GatewayRef: "gw_txn_1", Succeeded: true, AmountMinor: totalMinor,
	})
	if err := r.webhook.Handle(ctx, payload, sig); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	return r, paymentID
}

func TestRefundingACapturedPayment(t *testing.T) {
	ctx := context.Background()
	r, paymentID := capturedPayment(t, 50000)

	if err := r.refunds.Refund(ctx, "ord_1", "the shop had run out"); err != nil {
		t.Fatalf("Refund: %v", err)
	}
	stored := r.repo.payments[paymentID]
	if stored.Status != domain.StatusRefunded || stored.RefundReason != "the shop had run out" {
		t.Fatalf("payment = %+v", stored)
	}
	if len(r.gateway.refundCalls) != 1 || r.gateway.refundCalls[0] != "gw_txn_1" {
		t.Fatalf("gateway refund calls = %v", r.gateway.refundCalls)
	}
}

// Idempotent: a payment already refunded is a success without asking the
// gateway a second time.
func TestRefundingAnAlreadyRefundedPaymentIsIdempotent(t *testing.T) {
	ctx := context.Background()
	r, _ := capturedPayment(t, 50000)

	if err := r.refunds.Refund(ctx, "ord_1", "first"); err != nil {
		t.Fatalf("Refund: %v", err)
	}
	if err := r.refunds.Refund(ctx, "ord_1", "second"); err != nil {
		t.Fatalf("Refund again: %v", err)
	}
	if len(r.gateway.refundCalls) != 1 {
		t.Fatalf("the gateway was asked to refund twice: %v", r.gateway.refundCalls)
	}
}

// The reason is checked before the gateway is ever called: a refund that
// cannot be recorded must never have moved money in the first place.
func TestRefundNeedsAReasonBeforeTouchingTheGateway(t *testing.T) {
	ctx := context.Background()
	r, _ := capturedPayment(t, 50000)

	if err := r.refunds.Refund(ctx, "ord_1", ""); errs.CodeOf(err) != "reason_required" {
		t.Fatalf("err = %v", err)
	}
	if len(r.gateway.refundCalls) != 0 {
		t.Fatalf("the gateway was called despite a missing reason: %v", r.gateway.refundCalls)
	}
}

func TestRefundingAPendingPaymentIsRefused(t *testing.T) {
	ctx := context.Background()
	r, _ := startedCheckout(t, 50000)

	if err := r.refunds.Refund(ctx, "ord_1", "changed their mind"); errs.CodeOf(err) != "not_captured" {
		t.Fatalf("err = %v", err)
	}
	if len(r.gateway.refundCalls) != 0 {
		t.Fatalf("the gateway was called for an uncaptured payment: %v", r.gateway.refundCalls)
	}
}

func TestRefundingAnOrderWithNoPayment(t *testing.T) {
	ctx := context.Background()
	r := newRig()

	if err := r.refunds.Refund(ctx, "ord_missing", "changed their mind"); errs.CodeOf(err) != "payment_not_found" {
		t.Fatalf("err = %v", err)
	}
}

func TestRefundFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("storage unreachable on the read", func(t *testing.T) {
		r, _ := capturedPayment(t, 50000)
		r.repo.latestForOrderErr = errBoom
		if err := r.refunds.Refund(ctx, "ord_1", "reason"); errs.CodeOf(err) != "payment_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the gateway refuses", func(t *testing.T) {
		r, _ := capturedPayment(t, 50000)
		r.gateway.refundErr = errBoom
		if err := r.refunds.Refund(ctx, "ord_1", "reason"); errs.CodeOf(err) != "gateway_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the write fails", func(t *testing.T) {
		r, paymentID := capturedPayment(t, 50000)
		r.repo.savePaymentErr = errBoom
		if err := r.refunds.Refund(ctx, "ord_1", "reason"); errs.CodeOf(err) != "payment_unavailable" {
			t.Fatalf("err = %v", err)
		}
		if r.repo.payments[paymentID].Status != domain.StatusCaptured {
			t.Fatalf("a failed save still shows refunded in the store: %+v", r.repo.payments[paymentID])
		}
	})
}
